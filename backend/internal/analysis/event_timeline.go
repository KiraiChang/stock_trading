package analysis

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/trading/backend/internal/store"
)

// Event Timeline：把 market_event_states 的**每次分析快照**摺疊成事件鏈（chain）。
//
// 背景見 docs/todo.md T-045。現在的事件模型以 (zone_key, event_family) 為鍵、
// 新事件直接覆寫舊的，所以 in-memory 只看得到「當前狀態」，沒有演進歷程。
// 但每次分析都把完整狀態寫進 market_event_states，**相鄰兩份快照的差異就是一次轉換**——
// 鏈其實一直都在 DB 裡，只是沒有人把它讀出來。
//
// **這份資料是 display_chain**：供前端 timeline 顯示與人工檢查用，
// 不是 Lifecycle Engine 的 runtime 輸入（那需要另補 Go→Python contract，見 T-045）。

// EventTimelineTransition 是鏈上的一次狀態轉換。
type EventTimelineTransition struct {
	AnalyzedAt      time.Time `json:"analyzed_at"`
	AnalysisID      uint64    `json:"analysis_id"`
	State           string    `json:"state"`
	Active          bool      `json:"active"`
	EventType       string    `json:"event_type"`
	LatestEventType string    `json:"latest_event_type"`
	ResolvedBy      string    `json:"resolved_by,omitempty"`
	ReasonCodes     []string  `json:"reason_codes,omitempty"`
	// Changed 列出這一步相對前一步改變了哪些欄位，讓前端不必自己比對。
	// 第一筆（鏈的起點）為空。
	Changed []string `json:"changed,omitempty"`
}

// EventTimelineChain 是一條事件鏈：同一個 (zone_key, event_family) 從首次出現到終結。
type EventTimelineChain struct {
	ZoneKey       string    `json:"zone_key"`
	EventFamily   string    `json:"event_family"`
	Direction     string    `json:"direction"`
	RootEventType string    `json:"root_event_type"`
	FirstSeenAt   time.Time `json:"first_seen_at"`
	LastSeenAt    time.Time `json:"last_seen_at"`
	// Closed 代表鏈已終結（RESOLVED / EXPIRED）。未終結的鏈仍可能繼續演進。
	Closed      bool                      `json:"closed"`
	FinalState  string                    `json:"final_state"`
	Transitions []EventTimelineTransition `json:"transitions"`
}

// EventTimeline 是一個標的的完整輸出。
type EventTimeline struct {
	Symbol    string               `json:"symbol"`
	Timeframe string               `json:"timeframe"`
	Chains    []EventTimelineChain `json:"chains"`
	// Snapshots 是這段期間實際存在的分析次數與時間點。
	//
	// **必須誠實揭露**：timeline 的解析度等於 SR 分析的執行頻率，而目前沒有任何排程
	// 會產生分析（見 T-045 前置條件），所以鏈上的空白**不代表那段期間沒有事件**，
	// 只代表那段期間沒有分析。少了這個欄位，空白會被讀成「風平浪靜」。
	Snapshots []EventTimelineSnapshot `json:"snapshots"`
}

type EventTimelineSnapshot struct {
	AnalysisID uint64    `json:"analysis_id"`
	AnalyzedAt time.Time `json:"analyzed_at"`
	// GapDays 是距離上一次分析的天數。第一筆為 0。
	// 大於 1 就代表中間有沒被觀測到的日子。
	GapDays int `json:"gap_days"`
}

// 終結狀態：到這兩個狀態之後，同一個 (zone_key, event_family) 再出現算新的一條鏈。
// 值與 python/backtest/modular/sr_scoring/event_engine.py 的 LIFECYCLE_* 對齊。
const (
	eventStateResolved = "RESOLVED"
	eventStateExpired  = "EXPIRED"
)

func isClosedEventState(state string) bool {
	return state == eventStateResolved || state == eventStateExpired
}

// BuildEventTimeline 把狀態快照摺疊成事件鏈。
//
// 輸入必須是同一個 (symbol, timeframe) 的列，順序不拘——函式會自己依
// (analyzed_at, analysis_id) 穩定排序，因為摺疊結果依賴順序決定性。
// analyses 是這段期間**所有**分析的時間點，用來產生 Snapshots 與 gap。
// 傳 nil 會退化成「由 rows 推導」——那會漏掉「跑了分析但沒有任何事件」的次數，
// **只供單元測試使用**；正式路徑一定要帶（handler 會查 ListAnalysisSnapshots）。
func BuildEventTimeline(
	symbol, timeframe string,
	rows []store.MarketEventState,
	analyses []store.AnalysisSnapshot,
) EventTimeline {
	out := EventTimeline{
		Symbol:    symbol,
		Timeframe: timeframe,
		Chains:    []EventTimelineChain{},
		Snapshots: []EventTimelineSnapshot{},
	}

	sorted := make([]store.MarketEventState, len(rows))
	copy(sorted, rows)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if !a.AnalyzedAt.Equal(b.AnalyzedAt) {
			return a.AnalyzedAt.Before(b.AnalyzedAt)
		}
		if a.AnalysisID != b.AnalysisID {
			return a.AnalysisID < b.AnalysisID
		}
		if a.ZoneKey != b.ZoneKey {
			return a.ZoneKey < b.ZoneKey
		}
		if a.EventFamily != b.EventFamily {
			return a.EventFamily < b.EventFamily
		}
		return a.ID < b.ID
	})

	// 依分析分組：一次分析 ＝ 一份完整快照。
	type snapshot struct {
		analysisID uint64
		analyzedAt time.Time
		states     []store.MarketEventState
	}
	var snapshots []snapshot
	for _, row := range sorted {
		if n := len(snapshots); n > 0 && snapshots[n-1].analysisID == row.AnalysisID {
			snapshots[n-1].states = append(snapshots[n-1].states, row)
			continue
		}
		snapshots = append(snapshots, snapshot{
			analysisID: row.AnalysisID,
			analyzedAt: row.AnalyzedAt,
			states:     []store.MarketEventState{row},
		})
	}

	// Snapshots 一律以「所有分析」為準，而不是有事件的那幾次。
	// **實測 0050 有 14 次分析但只有 11 次留下事件狀態列**——用 rows 推導會把
	// 那 3 次沒有事件的分析報成「沒有觀測」，而這個欄位正是 gap 揭露的唯一依據。
	type snapPoint struct {
		id uint64
		at time.Time
	}
	points := make([]snapPoint, 0, len(analyses))
	if len(analyses) > 0 {
		for _, a := range analyses {
			points = append(points, snapPoint{a.ID, a.AnalyzedAt})
		}
	} else {
		for _, s := range snapshots {
			points = append(points, snapPoint{s.analysisID, s.analyzedAt})
		}
	}
	sort.SliceStable(points, func(i, j int) bool {
		if !points[i].at.Equal(points[j].at) {
			return points[i].at.Before(points[j].at)
		}
		return points[i].id < points[j].id
	})

	var prevAt time.Time
	for i, pt := range points {
		gap := 0
		if i > 0 {
			gap = int(pt.at.Sub(prevAt).Hours() / 24)
		}
		out.Snapshots = append(out.Snapshots, EventTimelineSnapshot{
			AnalysisID: pt.id,
			AnalyzedAt: pt.at,
			GapDays:    gap,
		})
		prevAt = pt.at
	}

	type chainKey struct{ zoneKey, family string }
	// open 指向 chains 裡尚未終結的那一條；終結後從 open 移除。
	open := map[chainKey]int{}
	// last 指向該鍵最近一條鏈（不論是否已終結），用來辨識墓碑重複回報。
	//
	// **這是實測真實資料才發現的**：事件終結後，狀態表會把那筆 EXPIRED／RESOLVED
	// 一直帶在後續每一份快照裡（0050 的 SUPPORT:92.1361:100.5139 從 7/23 EXPIRED 之後，
	// 8/03～8/12 每份快照都還在回報同一筆）。若把「已終結的鍵再出現」一律當成新事件，
	// 每份快照都會生出一條只有一筆 EXPIRED 的垃圾鏈。
	// 規則因此是：**只有非終結狀態才會開新鏈**，終結狀態只是墓碑。
	last := map[chainKey]int{}
	chains := []EventTimelineChain{}

	for _, s := range snapshots {
		for _, st := range s.states {
			key := chainKey{st.ZoneKey, st.EventFamily}
			step := EventTimelineTransition{
				AnalyzedAt:      s.analyzedAt,
				AnalysisID:      s.analysisID,
				State:           st.State,
				Active:          st.Active,
				EventType:       st.EventType,
				LatestEventType: st.LatestEventType,
				ResolvedBy:      st.ResolvedBy.String,
				ReasonCodes:     decodeReasonCodes(st.ReasonCodes),
			}

			idx, ok := open[key]
			if !ok {
				// 已終結的鍵又回報終結狀態，多半是墓碑（狀態表會把終結狀態一直帶著）。
				// **但不能一律當墓碑**：終結之後仍可能有真實的狀態變化，最典型的是
				// RESOLVED 老化成 EXPIRED——`_normalize_previous_event_state` 在
				// age_bars 達門檻時會把 carried 的 RESOLVED 翻成 EXPIRED
				// （event_engine.py 的 expired 判定）。一律吞掉的話，鏈的 final_state
				// 會永遠停在 RESOLVED，與 DB 裡的最新狀態矛盾。
				// 所以只有**與前一步完全相同**才算墓碑。
				if isClosedEventState(st.State) {
					if prev, seen := last[key]; seen {
						prevChain := &chains[prev]
						lastStep := prevChain.Transitions[len(prevChain.Transitions)-1]
						if changed := diffTransition(lastStep, step); len(changed) == 0 {
							prevChain.LastSeenAt = s.analyzedAt
							continue
						} else {
							// 有變化：接在同一條鏈上，讓終結後的演進仍然看得見。
							step.Changed = changed
							prevChain.Transitions = append(prevChain.Transitions, step)
							prevChain.LastSeenAt = s.analyzedAt
							prevChain.FinalState = st.State
							continue
						}
					}
					// 從未見過這個鍵、第一次看到就已終結：我們是在事件結束後才開始觀測，
					// 仍要記一條（已終結的）鏈，否則這段歷史會完全消失。
				}
				chains = append(chains, EventTimelineChain{
					ZoneKey:       st.ZoneKey,
					EventFamily:   st.EventFamily,
					Direction:     st.Direction,
					RootEventType: st.RootEventType,
					FirstSeenAt:   s.analyzedAt,
					LastSeenAt:    s.analyzedAt,
					FinalState:    st.State,
					Transitions:   []EventTimelineTransition{step},
				})
				idx = len(chains) - 1
				last[key] = idx
				if isClosedEventState(st.State) {
					chains[idx].Closed = true
				} else {
					open[key] = idx
				}
				continue
			}

			chain := &chains[idx]
			last := chain.Transitions[len(chain.Transitions)-1]
			changed := diffTransition(last, step)
			chain.LastSeenAt = s.analyzedAt
			chain.FinalState = st.State
			// **同狀態不產生 transition**：分析每天跑一次，沒有變化的日子若都記一筆，
			// 鏈會被「什麼都沒發生」淹沒。
			if len(changed) == 0 {
				continue
			}
			step.Changed = changed
			chain.Transitions = append(chain.Transitions, step)
			if isClosedEventState(st.State) {
				chain.Closed = true
				delete(open, key)
			}
		}
	}

	out.Chains = chains
	return out
}

// decodeReasonCodes：reason_codes 存的是 JSON 陣列字串。解析失敗回 nil 而不是報錯——
// 它是輔助說明，壞掉的一筆不該讓整條 timeline 查不出來。
func decodeReasonCodes(raw store.RawJSON) []string {
	if raw == "" {
		return nil
	}
	var codes []string
	if err := json.Unmarshal([]byte(raw), &codes); err != nil {
		return nil
	}
	return codes
}

// diffTransition 回傳這一步相對前一步改變的欄位名。
// 只看語意上代表「事件推進」的欄位——confidence、price_level 這類數值每次分析都會微幅浮動，
// 納入比對會讓每一天都變成一次 transition，鏈就失去可讀性。
func diffTransition(prev, next EventTimelineTransition) []string {
	var changed []string
	if prev.State != next.State {
		changed = append(changed, "state")
	}
	if prev.Active != next.Active {
		changed = append(changed, "active")
	}
	if prev.LatestEventType != next.LatestEventType {
		changed = append(changed, "latest_event_type")
	}
	if prev.ResolvedBy != next.ResolvedBy {
		changed = append(changed, "resolved_by")
	}
	return changed
}
