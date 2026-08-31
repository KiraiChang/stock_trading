package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/trading/backend/internal/store"
)

// Event Timeline 對**真實資料**的驗證（todo.md T-045 P1 的驗收項目）。
//
// **為什麼單元測試不夠**：單元測試全部建立在我對資料長相的假設上。T-045 唯一抓到的 bug
// （終結後的墓碑被重複開鏈）正是那個假設錯了。**2026-08-20 改讀身分層之後那個 bug 不可能
// 再發生**——鏈的邊界是寫入端存下來的事實，不再是讀取時推導的。但實跑仍然必要，只是驗證
// 的對象換了：現在要驗的是「讀取端有沒有忠實反映身分層」，而身分層本身是否正確由
// T-048 的六條門檻負責（scripts/verify-event-timeline.sh 與階梯驗收）。
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
	identity := store.NewEventIdentityRepo(db)
	chains, err := identity.ListChains(ctx, symbol, "1d", time.Time{})
	if err != nil {
		t.Fatalf("ListChains 失敗: %v", err)
	}
	if len(chains) == 0 {
		t.Fatalf("%s 沒有任何事件鏈——換一檔有分析紀錄的標的（SR_TIMELINE_SYMBOL）", symbol)
	}
	uids := make([]string, 0, len(chains))
	for i := range chains {
		uids = append(uids, chains[i].EventUID)
	}
	transitions, err := identity.ListTransitions(ctx, uids)
	if err != nil {
		t.Fatalf("ListTransitions 失敗: %v", err)
	}

	// 另查所有分析：沒有事件的分析不會留下 state 列，只靠 rows 推導會漏掉它們。
	analyses, err := repo.ListAnalysisSnapshots(ctx, store.MarketEventStateHistoryOptions{
		Symbol: symbol, Timeframe: "1d",
	})
	if err != nil {
		t.Fatalf("ListAnalysisSnapshots 失敗: %v", err)
	}

	tl := BuildEventTimeline(symbol, "1d", chains, transitions, analyses, nil)

	// 印出來供人工核對：鏈的形狀是否合理，是這次驗證的主要目的。
	pretty, _ := json.MarshalIndent(tl, "", "  ")
	t.Logf("鏈 %d 條 / 轉換 %d 筆 / 分析 %d 次 → %d 條鏈 / %d 份快照\n%s",
		len(chains), len(transitions), len(analyses), len(tl.Chains), len(tl.Snapshots), pretty)

	// **核心驗收**：端點回傳的鏈數必須等於身分層的鏈數。
	// 不相等就是讀取端漏了或多生了鏈——舊作法（摺疊 zone_key）在這裡必然對不上，
	// 因為 key 漂移會把同一個身分拆成多條（現況規格見
	// docs/sr-zone-scoring.md「事件層：鏈的身分與三段關聯決策」；
	// 原記於 todo.md T-051 與 issue.md I-080，均已收斂）。
	if len(tl.Chains) != len(chains) {
		t.Errorf("輸出鏈數 %d != event_instances 鏈數 %d", len(tl.Chains), len(chains))
	}

	// 同一個 zone 身分 ＋ family ＋ seq 只能有一條鏈。
	seen := map[string]int{}
	for _, c := range tl.Chains {
		zone := "SYMBOL" // zone_uid 為 nil 代表 SYMBOL scope 的事件
		if c.ZoneUID != nil {
			zone = *c.ZoneUID
		}
		seen[fmt.Sprintf("%s|%s|%d", zone, c.EventFamily, c.Seq)]++
	}
	for key, n := range seen {
		if n > 1 {
			t.Errorf("%s 出現 %d 條鏈——同一個身分的同一條鏈被拆開了", key, n)
		}
	}

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
	// 每條鏈的 transition 必須時間遞增，且第一步是誕生（from_state 留白）。
	for _, c := range tl.Chains {
		if len(c.Transitions) == 0 {
			t.Errorf("%s(%s) 的鏈沒有任何 transition——寫入端的單一交易應該擋掉這種情況",
				c.EventUID, c.EventFamily)
			continue
		}
		if c.Transitions[0].FromState != "" {
			t.Errorf("%s 的第一步 from_state = %q，應留白（鏈誕生）",
				c.EventUID, c.Transitions[0].FromState)
		}
		if c.LastSeenAt.Before(c.Transitions[len(c.Transitions)-1].OccurredAt) {
			t.Errorf("%s 的 LastSeenAt 早於最後一筆 transition", c.EventUID)
		}
		for i := 1; i < len(c.Transitions); i++ {
			if c.Transitions[i].OccurredAt.Before(c.Transitions[i-1].OccurredAt) {
				t.Errorf("%s 的第 %d 筆 transition 時間倒退", c.EventUID, i)
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
