package analysis

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/trading/backend/internal/store"
)

// Event Timeline 對**真實資料**的驗證（todo.md T-045 P1 的驗收項目）。
//
// **為什麼單元測試不夠**：摺疊邏輯的單元測試全部建立在我對資料長相的假設上，
// 而 P1 唯一抓到的 bug（終結後的墓碑被重複回報，導致每份快照各開一條垃圾鏈）
// 正是那個假設錯了——事件終結後，狀態表會把 EXPIRED／RESOLVED 一直帶在後續快照裡。
// 這類錯誤不是邏輯錯誤而是**對資料實際長相的誤解**，只有實跑才驗得到。
//
// **唯讀**：只跑 SELECT，不寫入任何資料。CLAUDE.md 禁止的是拿 live 做測試資料、
// migration 驗證與清空資料；唯讀讀取不在此列（理由同 scripts/run-evaluation.sh）。
//
// 以 SR_TIMELINE_LIVE_DSN gate 住，沒設就 skip，一般的 backend/scripts/test.sh 不受影響。
// 跑法：scripts/verify-event-timeline.sh

func TestEventTimelineAgainstLiveData(t *testing.T) {
	dsn := os.Getenv("SR_TIMELINE_LIVE_DSN")
	if dsn == "" {
		t.Skip("未設 SR_TIMELINE_LIVE_DSN，跳過 live 驗證（用 scripts/verify-event-timeline.sh 執行）")
	}
	symbol := os.Getenv("SR_TIMELINE_SYMBOL")
	if symbol == "" {
		symbol = "0050"
	}

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("連不上 live DB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()

	repo := store.NewSRZoneRepo(db)
	rows, err := repo.ListMarketEventStateHistory(ctx, store.MarketEventStateHistoryOptions{
		Symbol: symbol, Timeframe: "1d",
	})
	if err != nil {
		t.Fatalf("ListMarketEventStateHistory 失敗: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("%s 沒有任何事件狀態列——換一檔有分析紀錄的標的（SR_TIMELINE_SYMBOL）", symbol)
	}

	// 另查所有分析：沒有事件的分析不會留下 state 列，只靠 rows 推導會漏掉它們。
	analyses, err := repo.ListAnalysisSnapshots(ctx, store.MarketEventStateHistoryOptions{
		Symbol: symbol, Timeframe: "1d",
	})
	if err != nil {
		t.Fatalf("ListAnalysisSnapshots 失敗: %v", err)
	}

	tl := BuildEventTimeline(symbol, "1d", rows, analyses)

	// 印出來供人工核對：鏈的形狀是否合理，是這次驗證的主要目的。
	pretty, _ := json.MarshalIndent(tl, "", "  ")
	t.Logf("狀態列 %d 筆 / 分析 %d 次 → %d 條鏈 / %d 份快照\n%s",
		len(rows), len(analyses), len(tl.Chains), len(tl.Snapshots), pretty)

	// snapshots 必須等於**所有**分析次數，不是只有留下事件列的那幾次。
	if len(tl.Snapshots) != len(analyses) {
		t.Errorf("快照數 %d != 分析次數 %d——沒有事件的分析被漏掉了，gap 會虛報",
			len(tl.Snapshots), len(analyses))
	}

	// ── 以下是不論資料內容都必須成立的性質 ──────────────────────

	if len(tl.Snapshots) == 0 {
		t.Fatal("有狀態列卻沒有任何快照")
	}
	if len(tl.Chains) == 0 {
		t.Fatal("有狀態列卻摺不出任何鏈")
	}

	// **這是 P1 抓到的那個 bug 的回歸檢查**：垃圾鏈的特徵是「只有一筆 transition
	// 且狀態是終結態」。真實資料裡首見即終結是合理的（我們在事件結束後才開始觀測），
	// 但**同一個 (zone, family) 不該有兩條以上這種鏈**——那就是墓碑被重複開鏈。
	tombstoneChains := map[string]int{}
	for _, c := range tl.Chains {
		if len(c.Transitions) == 1 && isClosedEventState(c.FinalState) {
			tombstoneChains[c.ZoneKey+"|"+c.EventFamily]++
		}
	}
	for key, n := range tombstoneChains {
		if n > 1 {
			t.Errorf("%s 有 %d 條「單筆且終結」的鏈——墓碑被當成新事件重複開鏈了", key, n)
		}
	}

	// 每條鏈的 transition 必須時間遞增，且 first/last 與 transition 一致。
	for _, c := range tl.Chains {
		if len(c.Transitions) == 0 {
			t.Errorf("%s|%s 的鏈沒有任何 transition", c.ZoneKey, c.EventFamily)
			continue
		}
		if !c.FirstSeenAt.Equal(c.Transitions[0].AnalyzedAt) {
			t.Errorf("%s|%s 的 FirstSeenAt 與第一筆 transition 不一致", c.ZoneKey, c.EventFamily)
		}
		if c.LastSeenAt.Before(c.Transitions[len(c.Transitions)-1].AnalyzedAt) {
			t.Errorf("%s|%s 的 LastSeenAt 早於最後一筆 transition", c.ZoneKey, c.EventFamily)
		}
		for i := 1; i < len(c.Transitions); i++ {
			if c.Transitions[i].AnalyzedAt.Before(c.Transitions[i-1].AnalyzedAt) {
				t.Errorf("%s|%s 的第 %d 筆 transition 時間倒退", c.ZoneKey, c.EventFamily, i)
			}
			if len(c.Transitions[i].Changed) == 0 {
				t.Errorf("%s|%s 的第 %d 筆 transition 沒有標明改變了什麼——"+
					"沒有變化就不該產生 transition", c.ZoneKey, c.EventFamily, i)
			}
		}
	}

	// 快照必須時間遞增；gap_days 是揭露「這段期間沒有分析」的唯一依據。
	for i := 1; i < len(tl.Snapshots); i++ {
		if tl.Snapshots[i].AnalyzedAt.Before(tl.Snapshots[i-1].AnalyzedAt) {
			t.Errorf("第 %d 份快照時間倒退", i)
		}
	}
}
