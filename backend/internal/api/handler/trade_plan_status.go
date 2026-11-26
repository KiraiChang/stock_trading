package handler

// T-089 D2：交易計畫的行情日期、來源確認與資料狀態（只加回應欄位，⛔ 不存 DB、⛔ 不改判斷邏輯）。
// 規格見 docs/plans/t089-first-usable-trade-plan.md 第三節 D2；現況說明歸檔在 docs/api-reference.md 的交易分析兩支端點。

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/internal/tradingday"
	"github.com/trading/backend/pkg/timeutil"
)

const (
	// tradePlanRuleVersion 是現行分析器的 rule_version；只有它的計畫能確認來源（038 搬過來的舊紀錄⛔ 不行）。
	tradePlanRuleVersion = "position_sr_zone_v1"
	tradePlanTimeframe   = "1d"
	// tradePlanSettledLookbackDays：往回找「今天之前最近一個交易日」最多找幾天（找不到就是 calendar = unconfirmed）。
	tradePlanSettledLookbackDays = 15
	// 13:30 是等待當日日 K 的起點（Q2）。
	tradePlanAwaitHour, tradePlanAwaitMinute = 13, 30
)

// tradeDayJudge 是共用的交易日判斷（*tradingday.Judge 滿足它）。
type tradeDayJudge interface {
	On(ctx context.Context, day time.Time) tradingday.Result
}

// srSourceRefReader 依 id 一次回查 SR 快照的來源欄位（store.SRZoneRepo 滿足它）。
type srSourceRefReader interface {
	GetSourceRefs(ctx context.Context, ids []uint64) ([]store.SRZoneAnalysisSourceRef, error)
}

// tradePlanRecord 是回應裡的一筆計畫：原本的欄位（內嵌，JSON 攤平）加兩個衍生欄位。
type tradePlanRecord struct {
	*store.PositionAnalysis
	// MarketDate：analyzed_at（這次分析實際用到的最後一根日 K）的台北日期；來源無法確認時是 null。
	MarketDate *string `json:"market_date"`
	// SourceConfirmed1d：確認這筆計畫是現行分析器用日 K 產生的（四條判定見 sourceConfirmed）。
	SourceConfirmed1d bool `json:"source_confirmed_1d"`
}

// tradeDataStatus 是回應頂層的資料狀態（針對這一個投組與標的）。
type tradeDataStatus struct {
	LatestCandle     string    `json:"latest_candle"` // ok／no_data／error
	LatestCandleDate *string   `json:"latest_candle_date"`
	ExpectedDate     *string   `json:"expected_date"`
	SettledDate      *string   `json:"settled_date"`
	AwaitingToday    bool      `json:"awaiting_today"`
	Calendar         string    `json:"calendar"` // confirmed／unconfirmed
	PositionVersion  *int64    `json:"position_version"`
	SourceLookup     string    `json:"source_lookup"` // ok／error
	AsOf             time.Time `json:"as_of"`
}

const (
	latestCandleOK     = "ok"
	latestCandleNoData = "no_data"
	latestCandleError  = "error"
	calendarConfirmed  = "confirmed"
	calendarUnconfirm  = "unconfirmed"
	sourceLookupOK     = "ok"
	sourceLookupError  = "error"
)

func taipeiDate(t time.Time) string { return t.In(timeutil.TaipeiTZ).Format("2006-01-02") }

func strPtr(s string) *string { return &s }

// dayConfirmed：推算用到的這一天，判斷依據有沒有確認過（fallback、或讀不到手動設定 → 沒有）。
func dayConfirmed(r tradingday.Result) bool {
	return r.Source != tradingday.SourceFallback && !r.OverrideUnavailable
}

// buildDataStatus 組 data_status。latest candle 與持倉版本各自查一次；任何一項查詢失敗只影響它自己的欄位，⛔ 不讓整個請求失敗。
func (h *TradeAnalysisHandler) buildDataStatus(ctx context.Context, portfolioID uint64, symbol string, sourceLookup string) tradeDataStatus {
	now := h.currentTime()
	st := tradeDataStatus{SourceLookup: sourceLookup, AsOf: now}

	// 最新一根日 K：查無資料（sql.ErrNoRows）與查詢失敗分開（⛔ 不把 DB 故障顯示成資料不足）
	var latest *time.Time
	switch {
	case h.candles == nil:
		st.LatestCandle = latestCandleError
	default:
		c, err := h.candles.GetLatest(ctx, symbol, tradePlanTimeframe)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			st.LatestCandle = latestCandleNoData
		case err != nil || c == nil:
			st.LatestCandle = latestCandleError
			if err != nil {
				h.logger().Warn("trade analyses: latest candle lookup failed", zap.String("symbol", symbol), zap.Error(err))
			}
		default:
			st.LatestCandle = latestCandleOK
			ts := c.Timestamp
			latest = &ts
			st.LatestCandleDate = strPtr(taipeiDate(ts))
		}
	}

	// 應有的行情日：用共用判斷由今天往回推
	local := now.In(timeutil.TaipeiTZ)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, timeutil.TaipeiTZ)
	afterClose := local.Hour() > tradePlanAwaitHour || (local.Hour() == tradePlanAwaitHour && local.Minute() >= tradePlanAwaitMinute)
	judge := h.tradeDayJudge()
	todayResult := judge.On(ctx, today)
	confirmed := dayConfirmed(todayResult)
	var settled *time.Time
	for i := 1; i <= tradePlanSettledLookbackDays; i++ {
		d := today.AddDate(0, 0, -i)
		r := judge.On(ctx, d)
		confirmed = confirmed && dayConfirmed(r)
		if r.IsTrading {
			settled = &d
			break
		}
	}
	if settled == nil {
		confirmed = false
	} else {
		st.SettledDate = strPtr(settled.Format("2006-01-02"))
	}
	if todayResult.IsTrading && afterClose {
		st.ExpectedDate = strPtr(today.Format("2006-01-02"))
		if latest != nil && taipeiDate(*latest) < today.Format("2006-01-02") {
			st.AwaitingToday = true
		}
	} else {
		st.ExpectedDate = st.SettledDate
	}
	if confirmed {
		st.Calendar = calendarConfirmed
	} else {
		st.Calendar = calendarUnconfirm
	}

	// 目前持倉的版本：沒有持倉列是 0；讀取失敗是 null
	pos, err := h.repo.Get(ctx, portfolioID, symbol)
	switch {
	case errors.Is(err, sql.ErrNoRows) || (err == nil && pos == nil):
		zero := int64(0)
		st.PositionVersion = &zero
	case err != nil:
		h.logger().Warn("trade analyses: position lookup failed", zap.String("symbol", symbol), zap.Error(err))
	default:
		v := pos.Version
		st.PositionVersion = &v
	}
	return st
}

// evidenceTimeframe 讀 evidence.decision_context.timeframe；沒有這個鍵（或不是字串）回 ok=false。
func evidenceTimeframe(ev store.RawJSON) (string, bool) {
	if ev == "" {
		return "", false
	}
	var parsed struct {
		DecisionContext map[string]json.RawMessage `json:"decision_context"`
	}
	if err := json.Unmarshal([]byte(ev), &parsed); err != nil || parsed.DecisionContext == nil {
		return "", false
	}
	raw, ok := parsed.DecisionContext["timeframe"]
	if !ok {
		return "", false
	}
	var tf string
	if err := json.Unmarshal(raw, &tf); err != nil {
		return "", false
	}
	return tf, true
}

// enrichPlans 加上 market_date 與 source_confirmed_1d。source_confirmed_1d 依序判定，第一個成立的決定結果：
//  1. rule_version ⛔ 不是現行分析器（例如 038 的 holding_sr_zone_v1_legacy，SR id 是原樣搬移的舊值）→ false；
//  2. evidence.decision_context.timeframe 有值 → 等於 1d 才是 true（本案之後的新紀錄）；
//  3. 回查 sr_zone_analysis_id 的快照：存在、timeframe 是 1d、analyzed_at 等於計畫的 analyzed_at → true（本案之前的現行紀錄）；
//  4. 其他（快照已刪、對不上）→ false。
//
// 第 3 條用一句 IN 查詢一次取完；回查失敗時需要回查的那幾筆都當 false，回傳 source_lookup = error。
func (h *TradeAnalysisHandler) enrichPlans(ctx context.Context, rows []store.PositionAnalysis) ([]tradePlanRecord, string) {
	out := make([]tradePlanRecord, len(rows))
	needLookup := map[int]uint64{}
	for i := range rows {
		out[i] = tradePlanRecord{PositionAnalysis: &rows[i]}
		r := &rows[i]
		if r.RuleVersion != tradePlanRuleVersion {
			continue
		}
		if tf, ok := evidenceTimeframe(r.Evidence); ok {
			out[i].SourceConfirmed1d = tf == tradePlanTimeframe
			continue
		}
		if r.SRZoneAnalysisID.Valid && r.SRZoneAnalysisID.Int64 > 0 {
			needLookup[i] = uint64(r.SRZoneAnalysisID.Int64)
		}
	}
	lookup := sourceLookupOK
	if len(needLookup) > 0 {
		ids := make([]uint64, 0, len(needLookup))
		seen := map[uint64]bool{}
		for _, id := range needLookup {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
		var refs []store.SRZoneAnalysisSourceRef
		var err error
		if h.srRefs == nil {
			err = errors.New("sr source ref reader is not configured")
		} else {
			refs, err = h.srRefs.GetSourceRefs(ctx, ids)
		}
		if err != nil {
			lookup = sourceLookupError
			h.logger().Warn("trade analyses: sr source lookup failed", zap.Int("ids", len(ids)), zap.Error(err))
		} else {
			byID := make(map[uint64]store.SRZoneAnalysisSourceRef, len(refs))
			for _, ref := range refs {
				byID[ref.ID] = ref
			}
			for i, id := range needLookup {
				ref, ok := byID[id]
				out[i].SourceConfirmed1d = ok && ref.Timeframe == tradePlanTimeframe && ref.AnalyzedAt.Equal(rows[i].AnalyzedAt)
			}
		}
	}
	for i := range out {
		if out[i].SourceConfirmed1d {
			out[i].MarketDate = strPtr(taipeiDate(rows[i].AnalyzedAt))
		}
	}
	return out, lookup
}

func (h *TradeAnalysisHandler) currentTime() time.Time {
	if h.now != nil {
		return h.now()
	}
	return time.Now()
}

func (h *TradeAnalysisHandler) logger() *zap.Logger {
	if h.log != nil {
		return h.log
	}
	return zap.NewNop()
}

// tradeDayJudge：請求時才取共用判斷（跟 intraday 相同的做法——handler 建立時 scheduler 可能還沒注入）；沒有時用 noJudge
// （週末休市、平日照 fallback → calendar = unconfirmed，⛔ 不會誤判成已確認）。
func (h *TradeAnalysisHandler) tradeDayJudge() tradeDayJudge {
	if h.judge != nil {
		if j := h.judge(); j != nil {
			return j
		}
	}
	return noJudge
}

// 編譯期確認依賴的型別滿足介面
var (
	_ tradeDayJudge               = (*tradingday.Judge)(nil)
	_ srSourceRefReader           = store.SRZoneRepo(nil)
	_ analysis.LatestCandleReader = store.CandleRepo(nil)
)
