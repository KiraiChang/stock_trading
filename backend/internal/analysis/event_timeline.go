package analysis

import (
	"database/sql"
	"encoding/json"
	"sort"
	"time"

	"github.com/trading/backend/internal/store"
)

// Event Timeline：把身分層的事件鏈（event_instances ＋ event_transitions）整理成
// 前端與人工檢查看得懂的形狀。
//
// 現況規格見 docs/sr-zone-scoring.md「事件層：鏈的身分與三段關聯決策」與
// 「第一個讀者：Event Timeline」（原記於 todo.md T-045 / T-048 / T-051 與
// issue.md I-080，均已收斂）。
//
// **2026-08-20 起改讀身分層，不再摺疊 market_event_states 的快照。** 舊作法以
// `(zone_key, event_family)` 為鍵，而 zone 邊界每次由 ATR 重算、role 也會翻轉，
// 於是同一個 zone 的鏈會被拆成好幾條（實測 329 個身分裡有 102 個漂移過 key）。
//
// **不能改成「在讀取時把 zone_key 換算成 zone_uid」**：`market_event_states` 沒有
// `zone_uid` 欄位，唯一的換算路徑是 `zone_key_aliases`，而它每個身分只留最近 8 筆
// （實測已有 23 個身分撞頂，見 issue.md I-079）——換算是有損的。寫入端在三段關聯決策裡
// 已經把這件事做對了，讀取端重算一次只會產生第二份會漂移的事實。
//
// **這份資料是 display_chain**：供前端 timeline 顯示與人工檢查用，
// 不是 Lifecycle Engine 的 runtime 輸入（那需要另補 Go→Python contract，見 T-045）。

// EventTimelineTransition 是鏈上的一次狀態轉換，直接對應 event_transitions 的一列。
//
// 這裡**沒有** `active` 與 `changed`，兩者都是刻意拿掉的：
//   - `active` 不等於 state，還要通過該 family 的 gating 規則才算數。要逐步重建它，
//     Go 就得複製一份 gating_states——那是第二份判準，正是 T-048 一路在避免的東西。
//     鏈層仍然有 `active`（寫入端算好的）。
//   - `changed` 是舊作法比對相鄰快照推導出來的。現在 `from_state → to_state` 加上
//     `event_type` 本身就說明了這一步改了什麼，而且是存下來的事實而不是推導。
type EventTimelineTransition struct {
	// OccurredAt 是這一步所屬分析的 **K 棒時間**，不是跑分析的 wall clock。
	//
	// **這件事必須與 snapshots 同軸**：身分層存的 occurred_at 是 as_of 的 wall clock
	// （已知限制：那個軸量的是「我們看了幾次」），而 snapshots 用的是 K 棒日期。
	// 兩軸混用時整條鏈會擠在「跑分析的那一刻」——實測 2330 的 28 條鏈會全部顯示成
	// 在同一秒內發生，而 snapshots 橫跨一個月。
	// `analysis_id` 為 NULL（排程收尾）時才退回身分層的 occurred_at。
	OccurredAt time.Time `json:"occurred_at"`
	// AnalysisID 可為 0：鏈的終結有可能由排程收尾而不是某次分析。
	AnalysisID uint64 `json:"analysis_id,omitempty"`
	// FromState 為空代表**這是鏈的誕生**（event_transitions 的不變式）。
	FromState string `json:"from_state,omitempty"`
	State     string `json:"state"`
	// EventType 是觸發這次轉換的事件型別。鏈誕生與 carried 事件過期時可能沒有觸發事件。
	EventType   string   `json:"event_type,omitempty"`
	ReasonCodes []string `json:"reason_codes,omitempty"`
}

// EventTimelineChain 是一條事件鏈：一個 zone 身分 × 一個 family × 一個 seq。
type EventTimelineChain struct {
	EventUID string `json:"event_uid"`
	// ZoneUID 為 nil 代表 SYMBOL scope 的事件（不屬於任何 zone）。
	//
	// **是指標而不是 string + omitempty**：後者會讓 SYMBOL scope 時整個鍵消失，
	// 而消費端很自然會寫「欄位存在但為 null ＝ SYMBOL scope」——鍵直接不見會讓那個
	// 判斷靜默地走到 undefined 分支。DB 那一層本來就是 nullable，對外就照實表達成 null。
	ZoneUID *string `json:"zone_uid"`
	// ZoneKey 是**最近一次觀測到時事件帶的 key**（last_zone_key），只供人工比對。
	// **它不再是鏈的身分**——那是 zone_uid 的工作。同樣可為 null（SYMBOL scope 沒有 key）。
	ZoneKey         *string   `json:"zone_key"`
	EventScope      string    `json:"event_scope"`
	EventFamily     string    `json:"event_family"`
	Seq             int       `json:"seq"`
	Direction       string    `json:"direction"`
	RootEventType   string    `json:"root_event_type"`
	LatestEventType string    `json:"latest_event_type"`
	FirstSeenAt     time.Time `json:"first_seen_at"`
	LastSeenAt      time.Time `json:"last_seen_at"`
	// Closed 代表鏈已終結。未終結的鏈仍可能繼續演進。
	Closed     bool   `json:"closed"`
	Active     bool   `json:"active"`
	FinalState string `json:"final_state"`
	ResolvedBy string `json:"resolved_by,omitempty"`
	// EndReason：RESOLVED / EXPIRED / ZONE_IDENTITY_ENDED。
	//
	// **ZONE_IDENTITY_ENDED 要看得出來**：那是「zone 因 SPLIT/MERGE/RESHAPE 終止，
	// 所以鏈跟著收攤」，不是事件自己走完生命週期。畫成一般結束會誤導。
	EndReason string `json:"end_reason,omitempty"`
	// DecisionVisible=false 是「只寫不讀的事實紀錄」（階段 D 的隔離旗標）：
	// SUPPORT_RETEST / RESISTANCE_BREAKOUT 的鏈會回傳，但它們不進任何決策桶，
	// 不影響 Bias 或進場。顯示端要把它與決策事件分開，否則會被讀成買賣訊號。
	//
	// **刻意沒有 omitempty**：那會把 false 整個吃掉，而 false 正是這個欄位唯一
	// 有資訊量的值。缺值一律視為 true（既有事件都是決策可見的）。
	DecisionVisible bool                      `json:"decision_visible"`
	Transitions     []EventTimelineTransition `json:"transitions"`
}

// EventTimeline 是一個標的的完整輸出。
type EventTimeline struct {
	Symbol    string `json:"symbol"`
	Timeframe string `json:"timeframe"`
	// IdentitySince 是身分層對這檔最早的觀測時間。
	//
	// **不受 max_analyses 影響**（原記於 todo.md T-051 R5，已收斂）：它問的是「身分層何時開始有
	// 紀錄」，不是「這次查了多久」。值由 EventIdentityRepo.GetIdentitySince 對
	// **全歷史**算出來，與被視窗篩過的 Chains 無關——早期由 Chains 推導時，
	// 視窗之前就終結的鏈會被濾掉，畫面會把「這次沒查到」說成「更早沒有事件鏈」。
	//
	// **必須揭露**：事件鏈是從 migration 068 才開始寫的，更早的分析沒有鏈資料，而且
	// 刻意不回填（回填要解的正是「兩個舊 key 是不是同一個 zone」，那是身分層本身要建的
	// 能力）。少了這個欄位，「這段期間沒有鏈」與「這段期間沒有事件」在畫面上分不開。
	// 沒有任何鏈時為 nil。
	IdentitySince *time.Time           `json:"identity_since"`
	Chains        []EventTimelineChain `json:"chains"`
	// Snapshots 是這段期間實際存在的分析次數與時間點。
	//
	// **必須誠實揭露**：timeline 的解析度等於 SR 分析的執行頻率，所以鏈上的空白
	// **不代表那段期間沒有事件**，只代表那段期間沒有分析。分析排程見
	// docs/architecture.md「SR 分析的兩個時段共用一個執行所有權」
	// （平日 17:00 與 22:00 各一次，預設關閉）。
	Snapshots []EventTimelineSnapshot `json:"snapshots"`
}

type EventTimelineSnapshot struct {
	AnalysisID uint64    `json:"analysis_id"`
	AnalyzedAt time.Time `json:"analyzed_at"`
	// GapDays 是距離上一次分析的天數。第一筆為 0。
	// 大於 1 就代表中間有沒被觀測到的日子。
	GapDays int `json:"gap_days"`
}

// BuildEventTimeline 把身分層的鏈與轉換整理成對外形狀。
//
// chains / transitions 來自 EventIdentityRepo 的 ListChains / ListTransitions，
// 順序不拘——函式會自己排序，因為輸出的決定性不該依賴 SQL 的排序。
// analyses 是這段期間**所有**分析的時間點，用來產生 Snapshots 與 gap。
//
// identitySince 是身分層對這檔最早的紀錄時間，來自 EventIdentityRepo.GetIdentitySince
// （**全歷史、不套視窗**）。非 nil 時直接採用；nil 時退回由 chains 推導——那條路徑只給
// 未接身分層的呼叫端，理由見迴圈內的說明。
func BuildEventTimeline(
	symbol, timeframe string,
	chains []store.EventInstance,
	transitions []store.EventTransitionView,
	analyses []store.AnalysisSnapshot,
	identitySince *time.Time,
) EventTimeline {
	out := EventTimeline{
		Symbol:    symbol,
		Timeframe: timeframe,
		Chains:    []EventTimelineChain{},
		Snapshots: []EventTimelineSnapshot{},
	}
	if identitySince != nil {
		at := *identitySince
		out.IdentitySince = &at
	}

	out.Snapshots = buildTimelineSnapshots(analyses)

	byUID := map[string][]EventTimelineTransition{}
	for _, t := range transitions {
		byUID[t.EventUID] = append(byUID[t.EventUID], EventTimelineTransition{
			OccurredAt:  transitionTime(t),
			AnalysisID:  uint64(t.AnalysisID.Int64),
			FromState:   t.FromState.String,
			State:       t.ToState,
			EventType:   t.TriggerEventType.String,
			ReasonCodes: decodeReasonCodes(t.ReasonCodes),
		})
	}
	for uid := range byUID {
		steps := byUID[uid]
		sort.SliceStable(steps, func(i, j int) bool {
			return steps[i].OccurredAt.Before(steps[j].OccurredAt)
		})
	}

	sorted := make([]store.EventInstance, len(chains))
	copy(sorted, chains)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if !a.FirstSeenAt.Equal(b.FirstSeenAt) {
			return a.FirstSeenAt.Before(b.FirstSeenAt)
		}
		return a.EventUID < b.EventUID
	})

	for _, c := range sorted {
		steps := byUID[c.EventUID]
		if steps == nil {
			// 鏈存在但沒有任何轉換，代表寫入端只寫了 instances 沒寫 transitions——
			// 那是 Apply 的單一交易應該擋掉的情況。這裡不吞掉：鏈照樣輸出（空 transitions），
			// 讓它在畫面上看得見，而不是靜靜消失。
			steps = []EventTimelineTransition{}
		}
		// 鏈的起訖同樣要落在 K 棒軸上，否則鏈的長度會變成「跑了幾秒分析」。
		// 沒有任何步驟時只好退回身分層存的 wall clock——那是上面說的降級情況。
		firstSeen, lastSeen := c.FirstSeenAt, c.LastSeenAt
		if len(steps) > 0 {
			firstSeen, lastSeen = steps[0].OccurredAt, steps[len(steps)-1].OccurredAt
		}
		out.Chains = append(out.Chains, EventTimelineChain{
			EventUID:        c.EventUID,
			ZoneUID:         nullableString(c.ZoneUID),
			ZoneKey:         nullableString(c.LastZoneKey),
			EventScope:      c.EventScope,
			EventFamily:     c.EventFamily,
			Seq:             c.Seq,
			Direction:       c.Direction,
			RootEventType:   c.RootEventType,
			LatestEventType: c.LatestEventType,
			FirstSeenAt:     firstSeen,
			LastSeenAt:      lastSeen,
			Closed:          c.EndedAt.Valid,
			Active:          c.Active,
			FinalState:      c.State,
			ResolvedBy:      c.ResolvedBy.String,
			EndReason:       c.EndReason.String,
			DecisionVisible: c.DecisionVisible,
			Transitions:     steps,
		})
		// **只在沒有注入全域值時才由鏈推導**（原記於 todo.md T-051 R5，已收斂）。推導出來的是
		// 「本次回傳的鏈裡最早的起點」，而 chains 已先被 max_analyses 的視窗篩過，
		// 視窗之前就終結的鏈不在裡面——當成 identity_since 就會說謊。
		// 呼叫端有身分層時一律注入 EventIdentityRepo.GetIdentitySince 的全域值；
		// 這條降級路徑留給未注入身分層的呼叫端，那時 chains 必為空、結果同樣是 nil。
		if identitySince == nil {
			if out.IdentitySince == nil || firstSeen.Before(*out.IdentitySince) {
				at := firstSeen
				out.IdentitySince = &at
			}
		}
	}
	return out
}

// transitionTime 取這一步該顯示的時間：優先用所屬分析的 K 棒時間。
func transitionTime(t store.EventTransitionView) time.Time {
	if t.AnalyzedAt.Valid {
		return t.AnalyzedAt.Time
	}
	return t.OccurredAt
}

// buildTimelineSnapshots 產生分析次數與 gap。
//
// **一律以「所有分析」為準**，而不是有事件的那幾次：實測 0050 有 14 次分析但只有 11 次
// 留下事件列，用事件推導會把那 3 次報成「沒有觀測」，而這正是 gap 揭露的唯一依據。
func buildTimelineSnapshots(analyses []store.AnalysisSnapshot) []EventTimelineSnapshot {
	points := make([]store.AnalysisSnapshot, len(analyses))
	copy(points, analyses)
	sort.SliceStable(points, func(i, j int) bool {
		if !points[i].AnalyzedAt.Equal(points[j].AnalyzedAt) {
			return points[i].AnalyzedAt.Before(points[j].AnalyzedAt)
		}
		return points[i].ID < points[j].ID
	})

	out := make([]EventTimelineSnapshot, 0, len(points))
	var prevAt time.Time
	for i, pt := range points {
		gap := 0
		if i > 0 {
			gap = int(pt.AnalyzedAt.Sub(prevAt).Hours() / 24)
		}
		out = append(out, EventTimelineSnapshot{
			AnalysisID: pt.ID,
			AnalyzedAt: pt.AnalyzedAt,
			GapDays:    gap,
		})
		prevAt = pt.AnalyzedAt
	}
	return out
}

// nullableString 把 DB 的可空字串照實對應成「值或 null」，不折成空字串。
func nullableString(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	out := v.String
	return &out
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
