package market

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/text/unicode/norm"

	"github.com/trading/backend/internal/store"
)

// 終止上市名單的對帳與投影（計畫書 docs/todo.md T-071）。
//
// **單向、全程唯讀 is_listed**：
//
//	CSV → 解析 → 縮水防護 → delisting_events（全部保存）
//	    → 身分可證者投影到 stock_symbols → 其餘只告警 → 記錄本次快照

// ErrSourceShrunk 代表本次筆數少於上次 accepted 快照，放棄本輪。
var ErrSourceShrunk = errors.New("delisting source row count shrank")

// ErrEmptySourceRows 代表解析出 0 筆。
//
// ⛔ **與 acceptShrink 完全無關**：放行 0 的後果是災難性的——所有事件會被標記消失，
// 而且縮水基準被降成 0，之後任何筆數都「不低於基準」，防護等於永久失效。
var ErrEmptySourceRows = errors.New("delisting source returned zero rows")

// ErrStockSymbolSyncNotReady 代表當日的 stock_symbol_sync 尚未成功。
//
// ⚠️ 這是 fail-closed：sync 沒跑成功時 is_listed 的狀態是舊的或不完整的，
// 拿它當投影前置沒有意義。只把 cron 排在它後面**不構成依賴**——
// sync 可能失敗、被停用、逾時（上限 20 分鐘）或與手動觸發重疊。
var ErrStockSymbolSyncNotReady = errors.New("stock_symbol_sync has not succeeded today")

// companyNameSuffixes 是正規化時可移除的**封閉** allowlist。
//
// ⛔ **不可放寬成「移除 `-` 之後的所有文字」**：名稱是唯一能辨識「代號重用」的訊號，
// 過度正規化會把兩家不同公司變成相等，反而製造誤匹配——那是拆掉最後一道防線。
// 新增項目要改計畫書。
var companyNameSuffixes = []string{"-創櫃", "-創", "-DR", "-KY"}

// NormalizeCompanyName 正規化公司名稱以供身分比對。
//
// 固定順序：①全半形統一 → ②去除所有空白 → ③**只從字串尾端**移除 allowlist 內的
// **完整** suffix，**最多移除一次**。
//
// ⛔ `-DR` 只在結尾才移除，出現在中間不動；不得用「切到第一個 `-`」這類規則。
//
// ⚠️ **名稱比對是啟發式，不是身分證明**（公司會改名；這份 CSV 沒有 ISIN）。
// 它的定位是**只准不放**：不符就不寫入、進 D 類等人工，
// **不會**因為名稱相符就放寬其他任何一條規則。
func NormalizeCompanyName(name string) string {
	// NFKC 把全形英數與相容字元摺疊成半形（「ＡＢＣ」→「ABC」）。
	s := norm.NFKC.String(name)
	s = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r', '　', ' ':
			return -1
		}
		return r
	}, s)
	// ⚠️ allowlist 依長度由長到短嘗試（`-創櫃` 要先於 `-創`），且**最多移除一次**。
	for _, suffix := range companyNameSuffixes {
		if strings.HasSuffix(s, suffix) {
			return strings.TrimSuffix(s, suffix)
		}
	}
	return s
}

// DelistingOutcome 是一輪對帳的完整計數。
//
// ⚠️ **只進結構化 log，不進 job_runs 的數值欄位**——job_runs 沒有自由欄位。
type DelistingOutcome struct {
	// 方向一（母體 = 本次解析出的每一筆事件），**每筆恰好一類**，
	// 所以 N+B+U+C+D+M+X+P 應等於去重後的事件數。
	NoMaster      int // N：主檔查無此代號（實測 261/265）
	StillListed   int // B：is_listed = true（代號重用或轉櫃，正常現象）
	Unresolvable  int // U：market/type 不符，或 listed_date IS NULL
	GenerationGap int // C：delisted_date < listed_date
	IdentityDoubt int // D：名稱不符／多筆歧義／人工值且日期不同
	ManualMatched int // M：人工值且日期相同（來源佐證一致，Info）
	Conflict      int // X：投影 CAS 落空
	Projected     int // P：寫入或維持投影

	// 方向二（母體 = 主檔裡已下市的上市股票）。
	// ⚠️ **是 outcome counters 不是窮盡分割**：成功投影且穩定的列不屬於任何一類，
	// 那是正常的。契約只要求 R／RX／A 三者彼此不重複。
	Revoked            int // R：job-owned 失去唯一 candidate，撤銷成功
	RevocationConflict int // RX：同上但撤銷 CAS 落空
	SourceMissing      int // A：非 job-owned、CSV 查無 candidate

	// 來源更正（改名／改日期／消失／重現）的 **distinct event 數**。
	// ⛔ 不是轉換次數：同一筆事件同時「重現」與「改名」算 1 不算 2。
	SourceCorrected int

	// 診斷用，不參與狀態推導。
	EventRows       int
	DistinctSymbols int
	DuplicateRows   int
}

// Degraded 表示這一輪要收成 partial。
//
// ⚠️ **M 不算**：「人工值與來源一致」是好事，不該讓整輪變 partial。
// ⚠️ **來源更正要算**：否則「日期被更正、新事件成功投影為 P」會出 Warn 卻收成
// success，排程頁上完全看不出來源動過。
func (o DelistingOutcome) Degraded() bool {
	return o.SourceMissing > 0 || o.Unresolvable > 0 || o.IdentityDoubt > 0 ||
		o.Revoked > 0 || o.RevocationConflict > 0 || o.Conflict > 0 || o.SourceCorrected > 0
}

// Summary 是寫進 job_runs.error 的安全摘要（只有類別名與數字）。
//
// ⚠️ partial 也必須寫得出原因，否則排程頁只看得到 partial、看不到為什麼。
// 內容沒有主機、DSN 或 SQL 片段，所以由 joberr.SafeMessenger 原樣通過。
func (o DelistingOutcome) Summary() string {
	return fmt.Sprintf(
		"delisting_reconcile: source_missing=%d unresolvable=%d identity_doubt=%d "+
			"source_corrected=%d projection_revoked=%d concurrency_conflict=%d revocation_conflict=%d",
		o.SourceMissing, o.Unresolvable, o.IdentityDoubt,
		o.SourceCorrected, o.Revoked, o.Conflict, o.RevocationConflict)
}

// Fields 是結構化 log 的完整計數。
func (o DelistingOutcome) Fields() []zap.Field {
	return []zap.Field{
		zap.Int("no_master", o.NoMaster),
		zap.Int("still_listed", o.StillListed),
		zap.Int("unresolvable", o.Unresolvable),
		zap.Int("generation_gap", o.GenerationGap),
		zap.Int("identity_doubt", o.IdentityDoubt),
		zap.Int("manual_matched", o.ManualMatched),
		zap.Int("concurrency_conflict", o.Conflict),
		zap.Int("projected", o.Projected),
		zap.Int("projection_revoked", o.Revoked),
		zap.Int("revocation_conflict", o.RevocationConflict),
		zap.Int("source_missing", o.SourceMissing),
		zap.Int("source_corrected", o.SourceCorrected),
		zap.Int("event_rows", o.EventRows),
		zap.Int("distinct_symbols", o.DistinctSymbols),
		zap.Int("duplicate_rows", o.DuplicateRows),
	}
}

// DelistingSource 是可替換的來源（測試用 stub）。
type DelistingSource interface {
	FetchDelistings(ctx context.Context) (DelistingSourceResult, error)
}

// SyncReadinessChecker 回答「當日的 stock_symbol_sync 成功了嗎」。
type SyncReadinessChecker interface {
	StockSymbolSyncSucceededToday(ctx context.Context, now time.Time) (bool, error)
}

type DelistingReconciler struct {
	source    DelistingSource
	repo      store.DelistingRepo
	readiness SyncReadinessChecker
	log       *zap.Logger
}

func NewDelistingReconciler(
	source DelistingSource, repo store.DelistingRepo,
	readiness SyncReadinessChecker, log *zap.Logger,
) *DelistingReconciler {
	if log == nil {
		log = zap.NewNop()
	}
	return &DelistingReconciler{source: source, repo: repo, readiness: readiness, log: log}
}

// Reconcile 跑完整一輪。
//
// acceptShrink 只由**手動端點**傳入；cron 路徑結構上拿不到它
// （scheduler 的 cron 入口不收這個參數）。
func (r *DelistingReconciler) Reconcile(
	ctx context.Context, runID uint64, acceptShrink bool, now time.Time,
) (DelistingOutcome, error) {
	var out DelistingOutcome

	// ① fail-closed 的前置：當日 stock_symbol_sync 必須已成功。
	ready, err := r.readiness.StockSymbolSyncSucceededToday(ctx, now)
	if err != nil {
		return out, err
	}
	if !ready {
		return out, ErrStockSymbolSyncNotReady
	}

	// ② 抓取＋解析。任何失敗都整輪 failed，⛔ 不做部分寫入。
	res, err := r.source.FetchDelistings(ctx)
	if err != nil {
		return out, err
	}
	if res.RowCount == 0 {
		return out, ErrEmptySourceRows
	}
	out.EventRows = res.RowCount
	out.DistinctSymbols = res.DistinctSymbols
	out.DuplicateRows = res.DuplicateRows

	err = r.repo.WithTx(ctx, func(tx store.DelistingTx) error {
		// ③ 縮水防護。基準是上一筆 accepted 快照，⛔ 不是累計事件數
		//（那會在來源更正一次之後永久鎖死）。
		last, err := tx.LastAcceptedSnapshot(DelistingSourceName)
		if err != nil {
			return err
		}
		if last != nil && res.RowCount < last.RowCount {
			if !acceptShrink {
				return fmt.Errorf("%w: got=%d last_accepted=%d",
					ErrSourceShrunk, res.RowCount, last.RowCount)
			}
			// 人工核可的縮水要留審計紀錄（含新舊筆數）。
			r.log.Warn("人工核可縮水：本次筆數低於上次 accepted 快照",
				zap.Int("row_count", res.RowCount),
				zap.Int("last_accepted_row_count", last.RowCount),
				zap.Uint64("job_run_id", runID))
		}

		// ④ 事件層：先讀既有狀態（改名／重現的計數來源），再 upsert，最後標記消失。
		prior, err := tx.LoadEvents(DelistingSourceName)
		if err != nil {
			return err
		}
		priorByKey := map[store.DelistingEventKey]store.DelistingEvent{}
		for _, ev := range prior {
			priorByKey[ev.Key()] = ev
		}

		present := map[store.DelistingEventKey]struct{}{}
		correctedKeys := map[store.DelistingEventKey]struct{}{}
		for _, row := range res.Rows {
			ev := store.DelistingEvent{
				Source: DelistingSourceName, Symbol: row.Symbol,
				CompanyName: row.CompanyName, DelistedDate: row.DelistedDate,
			}
			key := ev.Key()
			present[key] = struct{}{}
			// ⛔ 改名／重現的計數**不能**用 upsert 的 RowsAffected：正常路徑每輪都會
			// 更新 last_seen_at，一筆都沒更正也會被算成「全部都變了」→ 永久 partial。
			// 三種 engine 的 affected-row 語意還各不相同，拿它當語意來源本身就不可移植。
			// 所以在**記憶體裡**比對更新前後的狀態。
			if old, ok := priorByKey[key]; ok {
				// ⚠️ **逐項 Warn 是規格要求**（計畫書 #10d／#10f／#10i）：
				// 只留 aggregate 的話，upsert 之後舊名稱與重現前的缺席時間就消失了，
				// 事後完全查不出「來源動了什麼」。
				if old.CompanyName != row.CompanyName {
					r.log.Warn("來源更正：事件公司名稱已變更",
						zap.String("symbol", row.Symbol),
						zap.String("delisted_date", store.DateKey(row.DelistedDate)),
						zap.String("old_name", old.CompanyName),
						zap.String("new_name", row.CompanyName))
				}
				if old.MissingFromSourceAt.Valid {
					r.log.Warn("來源更正：曾消失的事件重新出現（標記清回 NULL，恢復投影資格）",
						zap.String("symbol", row.Symbol),
						zap.String("delisted_date", store.DateKey(row.DelistedDate)),
						zap.Time("missing_since", old.MissingFromSourceAt.Time))
				}
				if old.CompanyName != row.CompanyName || old.MissingFromSourceAt.Valid {
					// 同一筆事件同時「重現」與「改名」→ 用 set 收斂成 1。
					correctedKeys[key] = struct{}{}
				}
			}
			if err := tx.UpsertEvent(ev, now); err != nil {
				return err
			}
		}

		// 首次缺席的逐項 Warn（規格 #10d 第三態）。⚠️ **計數仍以 affected rows 為準**
		// （見下），這裡只是把「是哪幾筆」寫進 log——標記完之後就分不出來了。
		for _, ev := range prior {
			if ev.MissingFromSourceAt.Valid {
				continue // 持續缺席：⛔ 不重複 Warn，也不重複計數（#10h）
			}
			if _, still := present[ev.Key()]; still {
				continue
			}
			r.log.Warn("來源更正：事件已從官方名單消失（標記後不再參與投影，⛔ 不刪除）",
				zap.String("symbol", ev.Symbol),
				zap.String("delisted_date", store.DateKey(ev.DelistedDate)),
				zap.String("company_name", ev.CompanyName))
		}

		// ✅ 首次缺席**可以**用 affected rows：SQL 帶 `WHERE missing_from_source_at IS NULL`，
		// predicate 自己保證了只有真的發生轉換的列才會被更新。
		firstAbsent, err := tx.MarkMissing(DelistingSourceName, present, now)
		if err != nil {
			return err
		}
		out.SourceCorrected = len(correctedKeys) + int(firstAbsent)

		// ⑤ 主檔 snapshot。⛔ 一般 SELECT，不可 FOR UPDATE——鎖住之後並行更新
		// 只能等到本交易結束，CAS 就永遠不會落空，X／RX 變成不可達。
		symbols := make([]string, 0, len(res.Rows))
		seen := map[string]struct{}{}
		for _, row := range res.Rows {
			if _, ok := seen[row.Symbol]; ok {
				continue
			}
			seen[row.Symbol] = struct{}{}
			symbols = append(symbols, row.Symbol)
		}
		states, err := tx.SymbolStates(symbols)
		if err != nil {
			return err
		}
		// 方向二的母體也在**撤銷前**一次讀完：R／RX／A 三者都基於同一份 snapshot 判定。
		// ⛔ 先撤銷再統計 A 的話，剛被撤銷的列會在同一輪同時計進 R 和 A。
		delistedStocks, err := tx.DelistedListedStocks()
		if err != nil {
			return err
		}

		// ⑥ 逐事件分類（方向一）。
		eventIDs, err := r.loadEventIDs(tx, res.Rows)
		if err != nil {
			return err
		}
		candidates := r.candidatesBySymbol(res.Rows, states)

		for _, row := range res.Rows {
			if err := r.classifyEvent(tx, row, states, candidates, eventIDs, now, &out); err != nil {
				return err
			}
		}

		// ⑦ 方向二：撤銷與來源缺漏。
		if err := r.reconcileMaster(tx, delistedStocks, candidates, now, &out); err != nil {
			return err
		}

		// ⑧ 快照。⚠️ **必須在同一個交易內**——它是下一輪縮水判斷的基準，
		// 「事件更新了但快照沒寫」會讓下一輪拿舊基準比新事件。
		return tx.InsertSnapshot(store.DelistingSourceSnapshot{
			Source:      DelistingSourceName,
			JobRunID:    nullableRunID(runID),
			FetchedAt:   now,
			RowCount:    res.RowCount,
			ContentHash: res.ContentHash,
			Accepted:    true,
		})
	})
	if err != nil {
		return DelistingOutcome{EventRows: out.EventRows, DistinctSymbols: out.DistinctSymbols}, err
	}
	return out, nil
}

// candidate 是通過投影規則 1～5 的事件（active 由 MarkMissing 保證：
// 標記完之後，仍 active 的事件集合就等於本輪 present 的集合）。
type candidate struct {
	row     DelistingEventRow
	eventID uint64
}

// candidatesBySymbol 依代號分組候選。
//
// ⚠️ **規則 6（數量恰好為 1）不是 candidate 的 predicate，是對這個集合的仲裁結果。**
// ⛔ 把它寫回 candidate 的定義裡會讓「多筆」永遠算成 0——兩筆並存時兩筆都不通過
// 規則 6、數量歸零，歧義那條分支在定義上就永遠走不到。
func (r *DelistingReconciler) candidatesBySymbol(
	rows []DelistingEventRow, states map[string]store.SymbolProjectionState,
) map[string][]DelistingEventRow {
	out := map[string][]DelistingEventRow{}
	for _, row := range rows {
		state, ok := states[row.Symbol]
		if !ok {
			continue
		}
		if evaluateCandidateFilters(row, state) != filterPass {
			continue
		}
		out[row.Symbol] = append(out[row.Symbol], row)
	}
	return out
}

// filterVerdict 是投影規則 1～5（逐列 predicate）的判定結果。
//
// ⛔ **規則 1～5 只能有一份實作。** 一度寫成「candidate 過濾」與「分類」各一份，
// 結果兩份互相掩護：拿掉任一邊的名稱比對，另一邊仍會把它擋成 D，
// mutation 測不出來——那代表**沒有任何測試真的在守那條規則**。
type filterVerdict int

const (
	filterPass          filterVerdict = iota
	filterStillListed                 // 規則 1：目前還在交易的一律不碰
	filterUnresolvable                // 規則 2／3：市場別或證券類型不符、listed_date 為 NULL
	filterGenerationGap               // 規則 4：終止日早於上市日 → 必然是上一代證券
	filterNameMismatch                // 規則 5：正規化名稱不符 → 代號重用
)

// evaluateCandidateFilters 是規則 1～5 的**唯一**實作。
func evaluateCandidateFilters(row DelistingEventRow, state store.SymbolProjectionState) filterVerdict {
	if state.IsListed {
		return filterStillListed
	}
	if !isListedStock(state) || !state.ListedDate.Valid {
		return filterUnresolvable
	}
	if row.DelistedDate.Before(state.ListedDate.Time) {
		return filterGenerationGap
	}
	// 規則 5：正規化名稱相符——唯一能區辨「代號重用」的訊號。
	if NormalizeCompanyName(state.Name) != NormalizeCompanyName(row.CompanyName) {
		return filterNameMismatch
	}
	return filterPass
}

func isListedStock(state store.SymbolProjectionState) bool {
	return strings.HasPrefix(state.Market, "上市") && state.SecurityType == "股票"
}

// classifyEvent 依序判定一筆 CSV 事件的類別（方向一，每筆恰好一類）。
func (r *DelistingReconciler) classifyEvent(
	tx store.DelistingTx,
	row DelistingEventRow,
	states map[string]store.SymbolProjectionState,
	candidates map[string][]DelistingEventRow,
	eventIDs map[store.DelistingEventKey]uint64,
	now time.Time,
	out *DelistingOutcome,
) error {
	state, ok := states[row.Symbol]
	if !ok {
		out.NoMaster++ // N：主檔查無此代號，**不告警**（實測 261/265 都是這一類）
		return nil
	}
	// 規則 1～5 走同一份實作（見 evaluateCandidateFilters 的註解）。
	// ⚠️ 順序有意義：`2432` 在規則 1 就歸 B，**不會**再被算進 C——
	// 那正是修掉第一版重複計數的地方。
	// ⚠️ **等級依計畫書的分類表**：N 不告警、B／C／M 是 Info、U／D／X 是 Warn。
	// ⛔ 只留 aggregate 數字不夠——U／D 是「**等人工**」的類別，
	// 沒有 symbol 與新舊值的話，看到 `identity_doubt=2` 的人無從查起。
	switch evaluateCandidateFilters(row, state) {
	case filterStillListed:
		r.log.Info("官方名單有終止上市紀錄但該代號仍在交易，不動作（歸 B）",
			zap.String("symbol", row.Symbol), zap.String("master_name", state.Name),
			zap.String("source_name", row.CompanyName),
			zap.String("delisted_date", store.DateKey(row.DelistedDate)))
		out.StillListed++ // B：代號重用或轉櫃，不動作
		return nil
	case filterUnresolvable:
		r.log.Warn("缺身分依據，無法判定（歸 U，等人工）",
			zap.String("symbol", row.Symbol), zap.String("market", state.Market),
			zap.String("security_type", state.SecurityType),
			zap.Bool("listed_date_present", state.ListedDate.Valid),
			zap.String("delisted_date", store.DateKey(row.DelistedDate)))
		out.Unresolvable++ // U：缺身分依據，等人工
		return nil
	case filterGenerationGap:
		r.log.Info("終止日早於主檔上市日，必然是上一代證券（歸 C）",
			zap.String("symbol", row.Symbol),
			zap.String("delisted_date", store.DateKey(row.DelistedDate)),
			zap.String("listed_date", store.DateKey(state.ListedDate.Time)))
		out.GenerationGap++ // C：明確的代號重用
		return nil
	case filterNameMismatch:
		r.log.Warn("名稱不符，疑似代號重用（歸 D，不寫主檔）",
			zap.String("symbol", row.Symbol), zap.String("master_name", state.Name),
			zap.String("source_name", row.CompanyName),
			zap.String("delisted_date", store.DateKey(row.DelistedDate)))
		out.IdentityDoubt++ // D：名稱不符
		return nil
	}

	// 規則 6：candidate 數量仲裁。
	if n := len(candidates[row.Symbol]); n != 1 {
		r.log.Warn("同一代號有多筆候選事件，無法決定投影哪一筆（歸 D，等人工）",
			zap.String("symbol", row.Symbol), zap.Int("candidates", n),
			zap.String("delisted_date", store.DateKey(row.DelistedDate)))
		out.IdentityDoubt++ // D：多筆歧義
		return nil
	}

	// 規則 7：ownership 仲裁（四選一）。
	switch {
	case state.Manual():
		// ⛔ **人工所有權是單向的，永不「升級」成 job-owned。**
		// 日期即使完全相同也不得補上 provenance——那會把人工值轉成 job-owned，
		// 之後來源一消失就被撤銷規則清掉，使用者填的東西被系統靜默刪除。
		if state.DelistedDate.Valid && sameDay(state.DelistedDate.Time, row.DelistedDate) {
			r.log.Info("人工填的終止日與官方一致（歸 M，⛔ 不補 provenance）",
				zap.String("symbol", row.Symbol),
				zap.String("delisted_date", store.DateKey(row.DelistedDate)))
			out.ManualMatched++ // M：來源佐證與人工值一致（Info）
		} else {
			manual := ""
			if state.DelistedDate.Valid {
				manual = store.DateKey(state.DelistedDate.Time)
			}
			r.log.Warn("人工填的終止日與官方不同，不覆蓋（歸 D，等人工）",
				zap.String("symbol", row.Symbol), zap.String("manual_date", manual),
				zap.String("source_date", store.DateKey(row.DelistedDate)))
			out.IdentityDoubt++ // D：人工值且日期不同 → 不覆蓋，等人工
		}
		return nil

	case state.JobOwned():
		// job-owned：維持或**替換**（來源更正 D1 → D2 走這條）。
		eventID := eventIDs[eventKeyOf(row)]
		if state.DelistedDate.Valid && sameDay(state.DelistedDate.Time, row.DelistedDate) &&
			uint64(state.DelistedEventID.Int64) == eventID {
			// 目標值與現值相同 → ⛔ 不發 no-op UPDATE 再看 affected rows
			//（MySQL 更新成相同值回 0，穩定正確的投影會每天被誤判成衝突）。
			ok, err := tx.ConfirmProjection(state)
			return r.tally(ok, err, row.Symbol, "confirm", out)
		}
		ok, err := tx.Project(state, row.DelistedDate, eventID, now)
		return r.tally(ok, err, row.Symbol, "replace", out)

	default:
		// 空值狀態 → 首次投影，建立 ownership。
		eventID := eventIDs[eventKeyOf(row)]
		ok, err := tx.Project(state, row.DelistedDate, eventID, now)
		return r.tally(ok, err, row.Symbol, "create", out)
	}
}

// tally 把 CAS 結果記成 P 或 X。
//
// ⚠️ CAS 落空歸 **X**（concurrency_conflict）而**不是** U：後者是「缺身分資料」，
// 混在一起的話看 log 的人分不出該補資料還是該查排程重疊。
// ⛔ **不重試**：本 job 每日跑，下一輪自然會用新的主檔狀態重算；
// 重試只會在 ISIN sync 仍在寫的窗口內反覆落空。
//
// ⛔ **DB error 不是 CAS 落空，一律往上拋讓整輪 rollback。**
// X 的定義是「`UPDATE` 影響 0 列」；把 error 也算成 X 會同時違反兩件事：
// ①分類語意（看 log 的人分不出「被並行改動」與「寫入根本失敗」）；
// ②all-or-nothing——**MySQL／SQLite 的單句錯誤不會中止整個 transaction**，
// 於是事件與 accepted 快照照樣 commit、投影卻沒寫進去（PostgreSQL 會讓後續語句
// 全部失敗，結果是同一輪的其他寫入也一起壞掉，同樣不該假裝成功）。
func (r *DelistingReconciler) tally(ok bool, err error, symbol, phase string, out *DelistingOutcome) error {
	if err != nil {
		r.log.Error("投影寫入失敗，整輪中止並 rollback",
			zap.String("symbol", symbol), zap.String("phase", phase), zap.Error(err))
		return fmt.Errorf("delisting projection %s failed: %w", phase, err)
	}
	if !ok {
		r.log.Warn("投影 CAS 落空，主檔在讀取後被改動（歸 X，本輪跳過不重試）",
			zap.String("symbol", symbol), zap.String("phase", phase))
		out.Conflict++
		return nil
	}
	out.Projected++
	return nil
}

// reconcileMaster 是方向二：撤銷失去 candidate 的 job-owned 投影，並統計來源缺漏。
//
// ⚠️ **收斂靠「每輪重算」而不是列舉觸發條件**——列舉法必漏：改名後不再相符、
// 出現兩筆歧義、替代事件不合格、主檔自己變了（改名／listed_date／market／type），
// 每一種都會留下不再符合規則卻仍在主檔的舊投影。
// 重算是冪等的，上面四種全部自動涵蓋。
func (r *DelistingReconciler) reconcileMaster(
	tx store.DelistingTx,
	master []store.SymbolProjectionState,
	candidates map[string][]DelistingEventRow,
	now time.Time,
	out *DelistingOutcome,
) error {
	for _, state := range master {
		n := len(candidates[state.Symbol])

		if state.JobOwned() {
			if n == 1 {
				continue // 由方向一的投影／確認處理
			}
			// candidate 數為 0 或 >1 → 撤銷。
			ok, err := tx.Revoke(state, now)
			if err != nil {
				// ⛔ 與 tally 同一條理由：DB error 不是 CAS 落空，
				// 吞掉它會讓事件與快照 commit、撤銷卻沒做（見 tally 的註解）。
				r.log.Error("撤銷投影失敗，整輪中止並 rollback",
					zap.String("symbol", state.Symbol), zap.Error(err))
				return fmt.Errorf("delisting revoke failed: %w", err)
			}
			if !ok {
				// ⛔ 撤銷落空**不能**計 R（本 job 其實沒完成撤銷），
				// 也**不能**併進方向一的 X——X 的單位是 CSV 事件，而撤銷的觸發情境
				// 常常是「來源事件已經消失」、根本沒有 CSV event 可承載。
				r.log.Warn("撤銷 CAS 落空，主檔在讀取後被改動（歸 RX，本輪跳過不重試）",
					zap.String("symbol", state.Symbol))
				out.RevocationConflict++
				continue
			}
			r.log.Warn("已撤銷失去來源支持的投影",
				zap.String("symbol", state.Symbol), zap.Int("candidates", n))
			out.Revoked++
			continue
		}

		// 非 job-owned（空值或人工值）且 CSV 查無 candidate → A。
		//
		// ⚠️ 這是最有價值的一類：代表「**不是下市，而是清冊抓取異常／證券類型變更**」。
		// ⚠️ 順序讓 R 與 A 不會重複：已投影的事件從 CSV 消失時，**首輪是 R**；
		// 撤銷之後該列才變成非 job-owned，**下一輪**才會落到 A。
		// 兩者是同一件事的兩個階段，靠的是「三者都基於撤銷前的同一份 snapshot」。
		if n == 0 {
			r.log.Warn("主檔標示已下市但官方名單查無對應事件",
				zap.String("symbol", state.Symbol), zap.String("name", state.Name),
				zap.String("market", state.Market))
			out.SourceMissing++
		}
	}
	return nil
}

// loadEventIDs 取本輪每一筆事件在 DB 裡的 id（投影要寫進 provenance）。
func (r *DelistingReconciler) loadEventIDs(
	tx store.DelistingTx, rows []DelistingEventRow,
) (map[store.DelistingEventKey]uint64, error) {
	events, err := tx.LoadEvents(DelistingSourceName)
	if err != nil {
		return nil, err
	}
	out := make(map[store.DelistingEventKey]uint64, len(events))
	for _, ev := range events {
		out[ev.Key()] = ev.ID
	}
	return out, nil
}

func eventKeyOf(row DelistingEventRow) store.DelistingEventKey {
	return store.DelistingEventKey{
		Source:       DelistingSourceName,
		Symbol:       row.Symbol,
		DelistedDate: store.DateKey(row.DelistedDate),
	}
}

// sameDay 只比日期。⚠️ 三種 engine 對 DATE 欄位的往返時區處理不同，
// 直接用 time.Equal 會在某些組合上永遠不相等。
// ⛔ **也不能先轉 UTC**：MySQL 帶 `loc=Asia/Taipei` 時會整批差一天，理由見 store.DateKey。
func sameDay(a, b time.Time) bool { return store.SameCalendarDay(a, b) }

// nullableRunID：runID 為 0 代表 startRun 失敗。
//
// ⚠️ 實際上走不到——核心流程在 startRun 失敗時就中止了（scheduler 那邊擋掉）。
// 保留這個轉換只是讓型別誠實：job_run_id 是 nullable，0 不是有效 id。
func nullableRunID(runID uint64) sql.NullInt64 {
	if runID == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(runID), Valid: true}
}
