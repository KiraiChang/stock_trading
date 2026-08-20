package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// mapScoreZonesError 依 Python /sr-zones 回傳的實際狀態碼，回給前端對應的
// 通用訊息——不透漏原始錯誤文字（細節只寫 log），但至少讓前端能分辨是
// 「該補資料」「該去訓練模型」還是「Python service 沒開」，不是每種情況
// 都顯示同一句「Python service 錯誤」。
func mapScoreZonesError(c *gin.Context, log *zap.Logger, err error) {
	var upstreamErr *analysis.UpstreamStatusError
	if errors.As(err, &upstreamErr) {
		switch upstreamErr.StatusCode {
		case http.StatusNotFound:
			log.Warn("sr-zones: score zones (no candles)", zap.Error(err))
			c.JSON(http.StatusNotFound, gin.H{"error": "找不到歷史資料，請確認股票代號是否正確，或先用「歷史資料回補」補資料"})
			return
		case http.StatusServiceUnavailable:
			log.Warn("sr-zones: score zones (model not trained)", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "機率模型尚未訓練，請先在下方「訓練/更新機率模型」區塊訓練"})
			return
		}
	}
	// 逾時同時判斷 context.DeadlineExceeded 與 net.Error.Timeout()：
	// Go 1.23+ 的 http.Client.Timeout 逾時會 unwrap 成前者，但 net.Error.Timeout()
	// 作為 belt-and-suspenders，避免依賴特定 Go 版本的 unwrap 行為。
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		log.Warn("sr-zones: score zones timeout", zap.Error(err))
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "SR Zone 分析逾時，請稍後重試；若經常發生，可降低 evidence 計算數量或延長逾時設定"})
		return
	}
	log.Error("sr-zones: score zones", zap.Error(err))
	c.JSON(http.StatusBadGateway, gin.H{"error": "Python 服務無法連線，請確認服務是否已啟動"})
}

type SRZoneHandler struct {
	client    *analysis.Client
	repo      store.SRZoneRepo
	provider  *analysis.SRAnalysisProvider
	watchlist store.WatchlistRepo
	trainJobs store.SRScoringTrainJobRepo
	verifier  *analysis.SRZoneVerifier
	// zoneIdentity 為**選填**（T-048 階段 B）：未注入時整條身分追蹤不執行，
	// 行為與導入前完全相同。比照 scheduler 對 adjuster 的處理。
	zoneIdentity store.ZoneIdentityRepo
	// eventIdentity 同樣是**選填**（T-048 階段 C）：未注入時事件身分追蹤不執行。
	eventIdentity store.EventIdentityRepo
	log           *zap.Logger
}

// SetZoneIdentity 注入 zone 身分追蹤（T-048 階段 B）。**只寫不讀**：
// 寫入失敗不影響分析本身，見 persistZoneIdentity。
func (h *SRZoneHandler) SetZoneIdentity(repo store.ZoneIdentityRepo) {
	h.zoneIdentity = repo
}

// SetEventIdentity 注入事件身分追蹤（T-048 階段 C）。**只寫不讀**，
// 寫入失敗不影響分析本身，見 persistEventIdentity。
func (h *SRZoneHandler) SetEventIdentity(repo store.EventIdentityRepo) {
	h.eventIdentity = repo
}

// srZoneScoreExcludedKeys 是 zone 的 "score" 區塊要排除的欄位——它們已經在
// item.data（id/price_low…）、item.lifecycle（status/broken_at…）或兄弟鍵
// （features/evidence/explanation/scenario/probability_context）各自提供，不需要在 score 裡再帶
// 一份。其餘欄位（評分相關）維持在 score，且未來 SRZone 新增的評分欄位會自動
// 進 score，不必同步維護一份欄位清單。
var srZoneScoreExcludedKeys = []string{
	"id", "analysis_id", "price_low", "price_high", "method", "role",
	"status", "broken_at", "broken_price", "resolved_role",
	"features", "evidence", "explanation", "scenario", "probability_context",
}

// srZonePipelineScore 把整筆 SRZone 序列化後，移除 srZoneScoreExcludedKeys，
// 讓 "score" 只保留真正的評分欄位，避免同一份資料在 response 裡序列化兩次。
type srZonePipelineScore struct {
	zone store.SRZone
}

func (s srZonePipelineScore) MarshalJSON() ([]byte, error) {
	raw, err := json.Marshal(s.zone)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	for _, k := range srZoneScoreExcludedKeys {
		delete(m, k)
	}
	return json.Marshal(m)
}

type srZonePipelineSnapshot struct {
	Analysis        *store.SRZoneAnalysis
	Zones           []store.SRZone
	Decision        *store.SRDecision
	EventDetections []store.MarketEventDetection
	EventStates     []store.MarketEventState
	DailyCandidates []store.SRDailyCandidate
	ModelGovernance *store.SRModelGovernance
	Status          gin.H
}

func (h *SRZoneHandler) loadSRZonePipelineSnapshot(ctx context.Context, id uint64) (*srZonePipelineSnapshot, error) {
	a, err := h.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	zones, err := h.repo.GetZones(ctx, id)
	if err != nil {
		return nil, err
	}
	snapshot := &srZonePipelineSnapshot{
		Analysis: a,
		Zones:    zones,
		Status: gin.H{
			"decision":         "missing",
			"events":           "missing",
			"daily_candidates": "missing",
			"model_governance": "missing",
		},
	}
	if decision, err := h.repo.GetDecision(ctx, id); err == nil {
		snapshot.Decision = decision
		snapshot.Status["decision"] = "normalized"
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if events, err := h.repo.GetMarketEventDetections(ctx, id); err == nil {
		snapshot.EventDetections = events
	} else {
		return nil, err
	}
	if states, err := h.repo.GetMarketEventStates(ctx, id); err == nil {
		snapshot.EventStates = states
	} else {
		return nil, err
	}
	// events 與 decision 在同一個 Create transaction 寫入：只要該筆有 normalized
	// decision，events 即視為 normalized（空集合是合法的「無事件」，不是缺正規化資料）。
	if snapshot.Decision != nil {
		snapshot.Status["events"] = "normalized"
	}
	if candidates, err := h.repo.GetDailyCandidates(ctx, id); err == nil {
		snapshot.DailyCandidates = candidates
		if len(candidates) > 0 {
			snapshot.Status["daily_candidates"] = "normalized"
		}
	} else {
		return nil, err
	}
	if governance, err := h.repo.GetModelGovernance(ctx, id); err == nil {
		snapshot.ModelGovernance = governance
		snapshot.Status["model_governance"] = "normalized"
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return snapshot, nil
}

func srZonePipelineResponse(snapshot *srZonePipelineSnapshot) gin.H {
	a := snapshot.Analysis
	zones := snapshot.Zones
	items := make([]gin.H, 0, len(zones))
	for _, z := range zones {
		items = append(items, gin.H{
			"data": gin.H{
				"id": z.ID, "analysis_id": z.AnalysisID,
				"price_low": z.PriceLow, "price_high": z.PriceHigh,
				"method": z.Method, "role": z.Role,
				// zone_uid：跨交易日的 zone 身分（T-048 階段 E）。這個 map 是**白名單**，
				// 不是 SRZone 整個 marshal，所以 struct 上的 json tag 不會自己把它帶出來。
				// 沒值時是 JSON null——三種語意見 069 migration 的註解。
				"zone_uid": z.ZoneUID,
			},
			"features":            z.Features,
			"score":               srZonePipelineScore{zone: z},
			"evidence":            z.Evidence,
			"explanation":         z.Explanation,
			"scenario":            z.Scenario,
			"probability_context": z.ProbabilityContext,
			"lifecycle": gin.H{
				"status": z.Status, "broken_at": z.BrokenAt,
				"broken_price": z.BrokenPrice, "resolved_role": z.ResolvedRole,
			},
		})
	}
	return gin.H{
		"pipeline_version": a.PipelineVersion,
		"analysis": gin.H{
			"id": a.ID, "symbol": a.Symbol, "timeframe": a.Timeframe,
			"analyzed_at": a.AnalyzedAt, "current_price": a.CurrentPrice,
			"model_version": a.ModelVersion, "model_config_hash": a.ModelConfigHash,
			"period_summaries": a.PeriodSummaries, "analysis_tips": a.AnalysisTips,
			"chip_summary": a.ChipSummary, "created_at": a.CreatedAt,
			"zone_builder_runtime_config": a.ZoneBuilderRuntimeConfig,
		},
		"features": gin.H{
			"global_trend": a.GlobalTrend, "global_volatility": a.GlobalVolatility,
		},
		"score": gin.H{
			"global_expected_value":    a.GlobalExpectedValue,
			"global_confidence":        a.GlobalConfidence,
			"global_risk_reward_ratio": a.GlobalRiskRewardRatio,
		},
		"evidence":            a.Evidence,
		"decision":            decisionFromSnapshot(snapshot),
		"explanation":         a.Explanation,
		"scenario":            a.Scenario,
		"probability_context": probabilityContextFromSnapshot(snapshot.ModelGovernance),
		"normalized_status":   snapshot.Status,
		"zones":               items,
	}
}

func decisionFromSnapshot(snapshot *srZonePipelineSnapshot) store.RawJSON {
	if snapshot.Decision == nil {
		return store.RawJSON("null")
	}
	obj := map[string]any{}
	applyDecisionDetailJSON(obj, snapshot.Decision)
	obj["market_bias"] = snapshot.Decision.MarketBias
	obj["position_action"] = snapshot.Decision.PositionAction
	obj["reason_codes"] = rawArray(snapshot.Decision.ReasonCodes)
	setNestedObjectValue(obj, "final_entry_permission", "state", snapshot.Decision.EntryPermissionState)
	setNestedObjectValue(obj, "price_path", "path_state", snapshot.Decision.PricePathState)
	setNestedObjectValue(obj, "model_governance", "health_state", snapshot.Decision.ModelHealthState)
	setNestedObjectValue(obj, "event_state_summary", "market_state", snapshot.Decision.EventMarketState)
	if len(snapshot.EventDetections) > 0 {
		obj["market_events"] = eventDetectionsJSON(snapshot.EventDetections)
	} else {
		obj["market_events"] = []any{}
	}
	if len(snapshot.EventStates) > 0 {
		obj["event_state_summary"] = eventStateSummaryJSON(obj["event_state_summary"], snapshot.EventStates)
	}
	if len(snapshot.DailyCandidates) > 0 {
		obj["daily_candidate_zones"] = dailyCandidatesJSON(snapshot.DailyCandidates)
	} else {
		obj["daily_candidate_zones"] = []any{}
	}
	return marshalRawObject(obj)
}

func applyDecisionDetailJSON(obj map[string]any, decision *store.SRDecision) {
	setRawObjectIfPresent(obj, "market_regime", decision.MarketRegimeJSON)
	setRawObjectIfPresent(obj, "data_quality", decision.DataQualityJSON)
	setRawObjectIfPresent(obj, "decision_derived_view", decision.DecisionDerivedViewJSON)
	setRawArrayIfPresent(obj, "event_sequence", decision.EventSequenceJSON)
	setRawObjectIfPresent(obj, "daily_price_action", decision.DailyPriceActionJSON)
	setRawObjectIfPresent(obj, "price_path", decision.PricePathJSON)
	setRawObjectIfPresent(obj, "daily_confirmation", decision.DailyConfirmationJSON)
	setRawObjectIfPresent(obj, "defense_lines", decision.DefenseLinesJSON)
	setRawObjectIfPresent(obj, "entry_executability", decision.EntryExecutabilityJSON)
	setRawObjectIfPresent(obj, "entry_blocking_zone", decision.EntryBlockingZoneJSON)
	setRawObjectIfPresent(obj, "rr_context", decision.RRContextJSON)
	setRawObjectIfPresent(obj, "rr_gate", decision.RRGateJSON)
	setRawObjectIfPresent(obj, "position_action_condition", decision.PositionActionConditionJSON)
	setRawArrayIfPresent(obj, "market_context", decision.MarketContextJSON)
	setRawObjectIfPresent(obj, "confidence_explanation", decision.ConfidenceExplanationJSON)
	setRawArrayIfPresent(obj, "risk_notes", decision.RiskNotesJSON)
	applyDecisionZoneSummariesJSON(obj, decision.ZoneSummariesJSON)
}

func setRawObjectIfPresent(obj map[string]any, key string, raw store.RawJSON) {
	if isRawNull(raw) {
		return
	}
	obj[key] = rawAny(raw)
}

func setRawArrayIfPresent(obj map[string]any, key string, raw store.RawJSON) {
	if isRawNull(raw) {
		return
	}
	obj[key] = rawArray(raw)
}

func applyDecisionZoneSummariesJSON(obj map[string]any, raw store.RawJSON) {
	if isRawNull(raw) {
		return
	}
	summaries := rawObjectOrEmpty(raw)
	for _, key := range []string{
		"nearest_decision_zone",
		"nearest_support_zone",
		"nearest_resistance_zone",
		"primary_structural_zone",
		"best_trade_zone",
		"primary_zone",
	} {
		if value, ok := summaries[key]; ok && value != nil {
			obj[key] = value
		}
	}
	if value, ok := summaries["secondary_zones"]; ok {
		raw, err := json.Marshal(value)
		if err == nil {
			values := rawArray(store.RawJSON(raw))
			obj["secondary_zones"] = values
		}
	}
}

func probabilityContextFromSnapshot(governance *store.SRModelGovernance) store.RawJSON {
	if governance == nil {
		return store.RawJSON("null")
	}
	obj := map[string]any{"schema_version": "sr_probability_context_v1"}
	health := map[string]any{}
	health["health_state"] = governance.HealthState
	health["average_edge_pp"] = nullableFloat(governance.AverageEdgePP)
	health["directional_zone_count"] = nullableInt(governance.DirectionalZoneCount)
	health["zone_count"] = nullableInt(governance.ZoneCount)
	health["quality_flags"] = rawArray(governance.QualityFlags)
	health["warning_flags"] = rawArray(governance.WarningFlags)
	health["blocking_flags"] = rawArray(governance.BlockingFlags)
	health["confidence_gate"] = rawAny(governance.ConfidenceGateJSON)
	obj["health"] = health

	reports := map[string]any{}
	reports["calibration_report"] = rawAny(governance.CalibrationReportJSON)
	reports["walk_forward_report"] = rawAny(governance.WalkForwardReportJSON)
	reports["dataset_diagnostics"] = rawAny(governance.DatasetDiagnosticsJSON)
	obj["model_reports"] = reports
	return marshalRawObject(obj)
}

func eventDetectionsJSON(events []store.MarketEventDetection) []any {
	out := make([]any, 0, len(events))
	for _, event := range events {
		item := rawObjectOrEmpty(event.EventJSON)
		item["event_key"] = event.EventKey
		item["type"] = event.EventType
		item["event_family"] = event.EventFamily
		item["event_scope"] = event.EventScope
		item["zone_key"] = event.ZoneKey
		item["direction"] = event.Direction
		item["state"] = event.State
		item["active"] = event.Active
		item["confidence"] = nullableFloat(event.Confidence)
		item["price_level"] = nullableFloat(event.PriceLevel)
		item["reason_codes"] = rawArray(event.ReasonCodes)
		out = append(out, item)
	}
	return out
}

func eventStateSummaryJSON(base any, states []store.MarketEventState) map[string]any {
	summary := rawObjectFromAny(base)
	items := make([]any, 0, len(states))
	active := make([]any, 0)
	candidates := make([]any, 0)
	confirmed := make([]any, 0)
	resolved := make([]any, 0)
	expired := make([]any, 0)
	activeBearish := make([]any, 0)
	activeBullish := make([]any, 0)
	var latestType any
	for _, state := range states {
		item := rawObjectOrEmpty(state.StateJSON)
		item["event_key"] = state.EventKey
		item["type"] = state.EventType
		item["event_family"] = state.EventFamily
		item["event_scope"] = state.EventScope
		item["zone_key"] = state.ZoneKey
		item["root_event_type"] = state.RootEventType
		item["latest_event_type"] = state.LatestEventType
		item["direction"] = state.Direction
		item["state"] = state.State
		item["active"] = state.Active
		item["resolved_by"] = nullableString(state.ResolvedBy)
		item["confidence"] = nullableFloat(state.Confidence)
		item["price_level"] = nullableFloat(state.PriceLevel)
		item["reason_codes"] = rawArray(state.ReasonCodes)
		items = append(items, item)
		// 階段 D：decision_visible=false 的事件只進 states，不進任何決策桶，
		// 也不參與 latest_event_type。**carry-forward 的回程走的就是這裡**——
		// Python 端的桶構建濾了、Go 端沒濾，等於完全沒有隔離。
		if !eventDecisionVisible(state.StateJSON) {
			continue
		}
		switch state.State {
		case "CANDIDATE":
			candidates = append(candidates, item)
		case "CONFIRMED":
			confirmed = append(confirmed, item)
		case "RESOLVED":
			resolved = append(resolved, item)
		case "EXPIRED":
			expired = append(expired, item)
		}
		if state.Active {
			active = append(active, item)
			switch state.Direction {
			case "BEARISH":
				activeBearish = append(activeBearish, item)
			case "BULLISH":
				activeBullish = append(activeBullish, item)
			}
		}
		latestType = state.LatestEventType
	}
	summary["states"] = items
	summary["candidates"] = candidates
	summary["confirmed"] = confirmed
	summary["active"] = active
	summary["resolved"] = resolved
	summary["expired"] = expired
	summary["active_bearish_events"] = activeBearish
	summary["active_bullish_events"] = activeBullish
	summary["latest_event_type"] = latestType
	return summary
}

func dailyCandidatesJSON(candidates []store.SRDailyCandidate) []any {
	out := make([]any, 0, len(candidates))
	for _, candidate := range candidates {
		item := rawObjectOrEmpty(candidate.CandidateJSON)
		item["price_low"] = candidate.PriceLow
		item["price_high"] = candidate.PriceHigh
		item["label"] = candidate.Label
		item["role"] = candidate.Role
		item["source"] = candidate.Source
		item["lifecycle"] = candidate.Lifecycle
		item["decision_role"] = candidate.DecisionRole
		item["distance_pct"] = nullableFloat(candidate.DistancePct)
		item["distance_label"] = candidate.DistanceLabel
		item["reason"] = candidate.Reason
		item["event_refs"] = rawArray(candidate.EventRefs)
		out = append(out, item)
	}
	return out
}

func rawObjectOrEmpty(raw store.RawJSON) map[string]any {
	if raw == "" || !json.Valid([]byte(raw)) || string(raw) == "null" {
		return map[string]any{}
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return map[string]any{}
	}
	return obj
}

func isRawNull(raw store.RawJSON) bool {
	return raw == "" || !json.Valid([]byte(raw)) || string(raw) == "null"
}

func rawObjectFromAny(value any) map[string]any {
	if obj, ok := value.(map[string]any); ok {
		return obj
	}
	return map[string]any{}
}

func rawAny(raw store.RawJSON) any {
	if raw == "" || !json.Valid([]byte(raw)) {
		return nil
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil
	}
	return value
}

func rawArray(raw store.RawJSON) []any {
	value := rawAny(raw)
	if arr, ok := value.([]any); ok {
		return arr
	}
	return []any{}
}

func setNestedObjectValue(obj map[string]any, objectKey, valueKey string, value any) {
	nested := rawObjectFromAny(obj[objectKey])
	nested[valueKey] = value
	obj[objectKey] = nested
}

func marshalRawObject(obj map[string]any) store.RawJSON {
	data, err := json.Marshal(obj)
	if err != nil {
		return store.RawJSON("{}")
	}
	return store.RawJSON(data)
}

func nullableFloat(value store.NullFloat64) any {
	if !value.Valid {
		return nil
	}
	return value.Float64
}

func nullableInt(value store.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

func nullableString(value store.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}

func NewSRZoneHandler(
	client *analysis.Client, repo store.SRZoneRepo, watchlist store.WatchlistRepo,
	trainJobs store.SRScoringTrainJobRepo, verifier *analysis.SRZoneVerifier,
	provider *analysis.SRAnalysisProvider, log *zap.Logger,
) *SRZoneHandler {
	return &SRZoneHandler{
		client: client, repo: repo, provider: provider, watchlist: watchlist,
		trainJobs: trainJobs, verifier: verifier, log: log,
	}
}

// newTrainJobID 比照 backtest.newJobID 的時間戳格式，不同前綴以便從 job_id
// 一眼分辨來源。
func newTrainJobID() string {
	return "sr_train_" + time.Now().UTC().Format("20060102_150405_000")
}

// POST /api/v1/sr-zones
// Body: { "symbol": "2330", "timeframe": "1d", "limit": 250, "reuse_existing": false }
// limit 省略或為 0 時使用 Python 端的預設值（DEFAULT_FETCH_LIMIT）
// reuse_existing 預設 false，維持舊契約：每次呼叫都觸發一次新分析並寫入 DB。
// 只有明確傳 true 時才允許重用近期同 timeframe 的既有快照。
func (h *SRZoneHandler) Create(c *gin.Context) {
	var body struct {
		Symbol        string `json:"symbol"`
		Timeframe     string `json:"timeframe"`
		Limit         int    `json:"limit"`
		ReuseExisting bool   `json:"reuse_existing"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}
	if body.Timeframe == "" {
		body.Timeframe = "1d"
	}
	if body.Limit < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be >= 0"})
		return
	}
	if body.ReuseExisting {
		if h.provider == nil {
			serverError(c, h.log, errors.New("sr analysis provider is not configured"), "sr-zones: provider")
			return
		}
		result, err := h.provider.Analyze(c.Request.Context(), body.Symbol, analysis.SRAnalysisOptions{
			Timeframe: body.Timeframe, Limit: body.Limit, ForceRefresh: false,
		})
		if err != nil {
			var upstreamErr *analysis.UpstreamStatusError
			if errors.As(err, &upstreamErr) {
				mapScoreZonesError(c, h.log, err)
				return
			}
			serverError(c, h.log, err, "sr-zones: provider analyze")
			return
		}
		snapshot, err := h.loadSRZonePipelineSnapshot(c.Request.Context(), result.Analysis.ID)
		if err != nil {
			serverError(c, h.log, err, "sr-zones: load reusable snapshot")
			return
		}
		c.JSON(http.StatusCreated, srZonePipelineResponse(snapshot))
		return
	}

	id, err := h.RunAnalysis(c.Request.Context(), body.Symbol, body.Timeframe, body.Limit)
	if err != nil {
		// **只有 scoring 那一段能走 mapScoreZonesError。** 它的 fallback 是 502
		// 「Python 服務無法連線」，把 repo.Create 的失敗餵進去會變成完全誤導的訊息。
		var scoreErr *srScoreError
		if errors.As(err, &scoreErr) {
			mapScoreZonesError(c, h.log, err)
			return
		}
		serverError(c, h.log, err, "sr-zones: run analysis")
		return
	}

	snapshot, err := h.loadSRZonePipelineSnapshot(c.Request.Context(), id)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: load saved snapshot")
		return
	}

	c.JSON(http.StatusCreated, srZonePipelineResponse(snapshot))
}

// srScoreError 標記「錯在呼叫 Python scoring 那一段」。
//
// 抽出 RunAnalysis 之後這個標記是**必要的**：HTTP 層用它決定要不要套
// mapScoreZonesError（404 沒資料 / 503 模型未訓練 / 504 逾時 / 502 連不上），
// 而那組對照的 fallback 是「Python 服務無法連線」——把 repo.Create 或身分寫入的
// 失敗餵進去，使用者會看到一個與真正原因無關的訊息。
type srScoreError struct{ err error }

func (e *srScoreError) Error() string { return e.err.Error() }
func (e *srScoreError) Unwrap() error { return e.err }

// RunAnalysis 跑一次完整的 SR zone 分析並落地，回傳 analysis id。
//
// **這是帶身分追蹤的那條路徑，而且全系統只有這一份。** `POST /sr-zones` 與排程
// （docs/todo.md T-052）都呼叫它。不要為了排程另外複製一份：
// `analysis.SRAnalysisProvider`（`reuse_existing=true` 那條）**不寫 zone_uid、不追身分**，
// 兩者不可互換——用錯的那條，分析會產生但身分層完全沒有紀錄，而且不會報錯。
//
// 錯誤一律包上階段名；scoring 那段另外包 srScoreError 供 HTTP 層做狀態碼對照。
func (h *SRZoneHandler) RunAnalysis(ctx context.Context, symbol, timeframe string, limit int) (uint64, error) {
	previousEventStates, err := h.repo.GetLatestMarketEventStates(ctx, symbol, timeframe)
	if err != nil {
		return 0, fmt.Errorf("load previous event states: %w", err)
	}
	result, err := h.client.ScoreZonesWithPreviousEvents(ctx, symbol, timeframe, limit, previousEventStates)
	if err != nil {
		return 0, &srScoreError{err}
	}

	a, zones, projections, err := result.ToStore()
	if err != nil {
		return 0, fmt.Errorf("convert result to store: %w", err)
	}

	// **比對在寫入之前**（T-048 階段 E）：zones 一次寫入就帶著 zone_uid，
	// 分析快照與 zone_instances 才有 join 路徑。失敗一律降級（zoneMatch == nil →
	// zone_uid 留空、事件層跳過），分析本身照常成立。
	zoneMatch := h.matchZoneIdentity(ctx, symbol, timeframe, zones)
	applyZoneUIDs(zones, zoneMatch)

	id, err := h.repo.Create(ctx, a, zones, projections)
	if err != nil {
		return 0, fmt.Errorf("create analysis: %w", err)
	}

	zoneOutcome := h.persistZoneIdentity(ctx, symbol, id, zones, zoneMatch)
	h.persistEventIdentity(ctx, symbol, timeframe, id, projections.EventStates, zoneOutcome)
	return id, nil
}

// GET /api/v1/sr-zones?symbol=2330&limit=20
func (h *SRZoneHandler) List(c *gin.Context) {
	symbol := c.Query("symbol")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	rows, err := h.repo.List(c.Request.Context(), symbol, limit)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: list analyses")
		return
	}
	c.JSON(http.StatusOK, gin.H{"analyses": rows, "total": len(rows)})
}

// GET /api/v1/sr-zones/:id
func (h *SRZoneHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	a, err := h.repo.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sr zone analysis not found"})
		return
	}
	snapshot, err := h.loadSRZonePipelineSnapshot(c.Request.Context(), a.ID)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: load snapshot")
		return
	}
	c.JSON(http.StatusOK, srZonePipelineResponse(snapshot))
}

// POST /api/v1/sr-zones/:id/verify
// 手動重新驗證：比對這筆分析之後的實際 K 棒，更新每個 zone 的 status（是否
// 被突破）。可重複呼叫，每次都用目前為止最新的資料重新計算，不是一次性
// 判定（見 internal/analysis/sr_zone_verifier.go）。沒有自動排程，需要主動
// 呼叫這支 API 才會更新（但 daily_close 排程會自動對近期分析跑一次，見
// scheduler）。
func (h *SRZoneHandler) Verify(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	a, _, err := h.verifier.Verify(c.Request.Context(), id)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: verify")
		return
	}
	snapshot, err := h.loadSRZonePipelineSnapshot(c.Request.Context(), a.ID)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: load verified snapshot")
		return
	}
	c.JSON(http.StatusOK, srZonePipelineResponse(snapshot))
}

// POST /api/v1/sr-zones/train
// Body: { "symbols": ["2330","2454"], "timeframe": "1d", "limit": 1500, "model_type": "gradient_boosting" }
// symbols 省略時自動使用 watchlist 全部股票；立即建立一筆 sr_scoring_train_jobs
// 紀錄（status=pending）並回傳 job_id，實際訓練在背景 goroutine 執行（可能
// 耗時數十秒到數分鐘，見 analysis.Client.TrainModel 的說明），呼叫端可用
// job_id 輪詢 GET /sr-zones/train-jobs/:job_id 查詢進度，不需要只靠伺服器
// log 才知道訓練有沒有成功。
func (h *SRZoneHandler) Train(c *gin.Context) {
	var body struct {
		Symbols           []string `json:"symbols"`
		Timeframe         string   `json:"timeframe"`
		Limit             int      `json:"limit"`
		ModelType         string   `json:"model_type"`
		SplitMethod       string   `json:"split_method"`
		CalibrationMethod string   `json:"calibration_method"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Timeframe == "" {
		body.Timeframe = "1d"
	}
	if body.ModelType == "" {
		body.ModelType = "gradient_boosting"
	}
	if body.SplitMethod == "" {
		body.SplitMethod = "time"
	}
	if body.CalibrationMethod == "" {
		body.CalibrationMethod = "sigmoid"
	}

	symbols := body.Symbols
	if len(symbols) == 0 {
		var err error
		symbols, err = h.watchlist.Symbols(c.Request.Context())
		if err != nil {
			serverError(c, h.log, err, "sr-zones: list watchlist symbols for train")
			return
		}
	}
	if len(symbols) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "watchlist 為空；請先新增股票或在 request body 中指定 symbols"})
		return
	}

	symbolsJSON, err := json.Marshal(symbols)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: marshal train symbols")
		return
	}

	jobID := newTrainJobID()
	job := &store.SRScoringTrainJob{
		JobID:      jobID,
		Symbols:    string(symbolsJSON),
		Timeframe:  body.Timeframe,
		FetchLimit: body.Limit,
		ModelType:  body.ModelType,
	}
	if _, err := h.trainJobs.Create(c.Request.Context(), job); err != nil {
		serverError(c, h.log, err, "sr-zones: create train job")
		return
	}

	go h.runTrainJob(jobID, symbols, body.Timeframe, body.Limit, body.ModelType, body.SplitMethod, body.CalibrationMethod)

	c.JSON(http.StatusAccepted, gin.H{
		"job_id":  jobID,
		"status":  "pending",
		"message": "模型訓練已在背景啟動",
		"symbols": len(symbols),
	})
}

// runTrainJob 在背景 goroutine 執行，不使用 request context（request 結束後
// context 就會被取消，但訓練要繼續跑到完成）。狀態更新失敗只記 log——訓練
// 本身的成敗不應該因為「記錄狀態」這個次要動作失敗而受影響。
func (h *SRZoneHandler) runTrainJob(jobID string, symbols []string, timeframe string, limit int, modelType, splitMethod, calibrationMethod string) {
	ctx := context.Background()
	if err := h.trainJobs.MarkRunning(ctx, jobID); err != nil {
		h.log.Error("sr_scoring train job: mark running failed", zap.String("job_id", jobID), zap.Error(err))
	}

	result, err := h.client.TrainModel(ctx, symbols, timeframe, limit, modelType, splitMethod, calibrationMethod)
	if err != nil {
		h.log.Error("sr_scoring train failed", zap.String("job_id", jobID), zap.Int("symbols", len(symbols)), zap.Error(err))
		if markErr := h.trainJobs.MarkFailed(ctx, jobID, err.Error()); markErr != nil {
			h.log.Error("sr_scoring train job: mark failed failed", zap.String("job_id", jobID), zap.Error(markErr))
		}
		return
	}

	metricsJSON, err := json.Marshal(result.Metrics)
	if err != nil {
		h.log.Error("sr_scoring train job: marshal metrics failed", zap.String("job_id", jobID), zap.Error(err))
		metricsJSON = []byte("{}")
	}
	datasetSummaryJSON, err := json.Marshal(result.DatasetSummary)
	if err != nil {
		h.log.Error("sr_scoring train job: marshal dataset summary failed", zap.String("job_id", jobID), zap.Error(err))
		datasetSummaryJSON = []byte("{}")
	}
	if err := h.trainJobs.MarkDone(
		ctx, jobID, result.Rows, result.Sources, store.RawJSON(metricsJSON), result.ModelPath, result.Version,
		result.SplitMethod, store.RawJSON(datasetSummaryJSON),
	); err != nil {
		h.log.Error("sr_scoring train job: mark done failed", zap.String("job_id", jobID), zap.Error(err))
	}
	h.log.Info("sr_scoring train completed",
		zap.String("job_id", jobID), zap.Int("rows", result.Rows), zap.Int("sources", result.Sources),
		zap.String("model_path", result.ModelPath), zap.Any("metrics", result.Metrics))
}

// GET /api/v1/sr-zones/train-jobs?limit=20
func (h *SRZoneHandler) ListTrainJobs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	jobs, err := h.trainJobs.List(c.Request.Context(), limit)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: list train jobs")
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs, "total": len(jobs)})
}

// PruneTrainJobs 刪除舊的 terminal 訓練任務紀錄（done/failed），保留最近 keep
// 筆。pending/running 永遠不刪，避免清掉仍在執行或尚未開始的任務狀態。
func (h *SRZoneHandler) PruneTrainJobs(c *gin.Context) {
	keep, _ := strconv.Atoi(c.DefaultQuery("keep", "20"))
	if keep < 5 {
		keep = 5
	}
	if keep > 200 {
		keep = 200
	}

	deleted, err := h.trainJobs.PruneTerminal(c.Request.Context(), keep)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: prune train jobs")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted, "keep": keep})
}

// GET /api/v1/sr-zones/train-jobs/:job_id
func (h *SRZoneHandler) GetTrainJob(c *gin.Context) {
	jobID := c.Param("job_id")

	job, err := h.trainJobs.Get(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "train job not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"job": job})
}

// GET /api/v1/sr-zones/model-status
// 讓前端在觸發分析前就能知道模型準備好了沒，不用先按分析失敗才知道
// （見 sr-zone-scoring.md「模型可追蹤性」）。永遠回 200，用 body 裡的
// exists 欄位表示模型存不存在。
func (h *SRZoneHandler) ModelStatus(c *gin.Context) {
	status, err := h.client.GetModelStatus(c.Request.Context())
	if err != nil {
		badGatewayError(c, h.log, err, "sr-zones: get model status")
		return
	}
	c.JSON(http.StatusOK, status)
}

// DELETE /api/v1/sr-zones/:id
func (h *SRZoneHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if _, err := h.repo.Get(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sr zone analysis not found"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		serverError(c, h.log, err, "sr-zones: delete analysis")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// EventTimeline 回傳事件鏈（GET /api/v1/sr-zones/event-timeline?symbol=&timeframe=）。
//
// **為什麼是 query 而不是 path param**：同一層已有 `GET /sr-zones/:id`（server.go），
// 再放一個 `/sr-zones/:symbol/...` 會與它衝突——gin 不允許同一位置有兩個不同名的 wildcard。
// 而 `evaluate`、`train-jobs`、`model-status` 等靜態同層路由與 `:id` 並存無礙，
// 所以靜態路徑 ＋ query 才是與既有慣例一致的作法。
//
// 回傳的是 **display_chain**：由 DB 的分析快照序列重建，供前端 timeline 顯示與人工檢查，
// 不是 Lifecycle Engine 的 runtime 輸入（見 docs/todo.md T-045）。
//
// Query：
//
//	symbol       必填
//	timeframe    預設 1d
//	max_analyses 回溯幾次分析，預設 60、上限 500
func (h *SRZoneHandler) EventTimeline(c *gin.Context) {
	symbol := strings.TrimSpace(c.Query("symbol"))
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}
	timeframe := strings.TrimSpace(c.DefaultQuery("timeframe", "1d"))

	opts := store.MarketEventStateHistoryOptions{Symbol: symbol, Timeframe: timeframe}
	if raw := strings.TrimSpace(c.Query("max_analyses")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "max_analyses must be a positive integer"})
			return
		}
		opts.MaxAnalyses = n
	}

	rows, err := h.repo.ListMarketEventStateHistory(c.Request.Context(), opts)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: event timeline")
		return
	}
	// 另外查「所有分析」：沒有事件的分析不會在 market_event_states 留下任何列，
	// 只靠 rows 推導 snapshots 會把它們報成「沒有觀測」（實測 0050 有 14 次分析、
	// 只有 11 次留下事件列）。gap 揭露的正確性依賴這一查。
	analyses, err := h.repo.ListAnalysisSnapshots(c.Request.Context(), opts)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: event timeline analyses")
		return
	}

	timeline := analysis.BuildEventTimeline(symbol, timeframe, rows, analyses)
	c.JSON(http.StatusOK, timeline)
}

// persistZoneIdentity 把這次分析的 zone 對應到跨交易日的身分並落地（T-048 階段 B）。
//
// **失敗只記 log，不影響分析本身。** 這四張表目前沒有任何讀者——沒有決策依賴它們，
// 卻讓一次身分匹配的失敗把整個 SR 分析變成 500，風險與收益完全不成比例。
// 等階段 C／T-049 真的有東西讀它，再把錯誤處理收緊。
//
// **只接在這一條路徑上（reuse_existing=false）。** SRAnalysisProvider.Analyze
// 也會 repo.Create，被 portfolio/analyzer 與本 handler 的 reuse 分支使用——
// 那些跑法會產生 zone 但不算一次「觀測」，所以 observed_absences 統計的是
// **分析的一個子集**。這是刻意的取捨：那條路徑的目的是重用既有分析，把它也算成
// 觀測會讓「我們看了幾次」失真。等 T-049 需要完整母體時要重新評估，
// 已記在 docs/sr-zone-scoring.md「Zone 身分與 ZoneMatcher」的已知限制。
// zoneIdentityOutcome 是階段 B 的結果裡**階段 C 需要的部分**。
//
// 事件要掛到 zone 的穩定身分上，就得知道「這次分析的哪個 zone 對應哪個 zone_uid」，
// 而那個對應只有這裡算得出來（matcher 的輸出與 zones 是索引對齊的）。
type zoneIdentityOutcome struct {
	// UIDByZoneKey：Python 產生的 zone_key → zone_uid。key 由 Python 的
	// event_engine.zone_identity_key 產生，事件身上帶的是同一個函數的輸出。
	UIDByZoneKey map[string]string
	// AliasUIDByZoneKey：**還活著的身分歷來用過的** zone_key → zone_uid
	// （T-048 階段 C 修法，F1 key 漂移）。
	//
	// UIDByZoneKey 只認得「這次分析算出來的 key」，而事件身上帶的是上次那個 zone 的 key。
	// role 進 AT_ZONE（role 編在 key 裡）或邊界被 ATR 重算，兩者就對不上——實測 41 筆
	// ZONE scope 事件有 26 筆卡在這裡。這份 alias 是第二次機會，**只服務全新關聯**：
	// 既有鏈條由 event_instances.last_zone_key 直接接住，不必再解析 key。
	AliasUIDByZoneKey map[string]string
	// AliasAmbiguous 是同一個 zone_key 對到多個活身分的那些 key，呼叫端要記 warn。
	AliasAmbiguous []string
	// EndedZoneUIDs 是這次因 SPLIT / MERGE / RESHAPE 而**身分終止**的 zone。
	// 它們身上的事件鏈要一併收掉（D4：不傳給 child），見 buildEventIdentityWrite。
	EndedZoneUIDs map[string]struct{}
}

// zoneIdentityMatch 是「比對完、但還沒寫入」的中間結果（T-048 階段 E）。
//
// 階段 E 之前，取資料／比對／寫入是同一個函數，而它整段跑在 repo.Create **之後**——
// zones 因此不可能帶著 zone_uid 入庫，「這次分析的第 N 個 zone 是哪個身分」只活在
// 回傳的 UIDByZoneKey 裡。現在把前兩段前移到 repo.Create 之前，zones 一次寫入就帶著
// 身分；寫入那段仍在 analysisID 拿到之後跑（zone_role_incarnations 需要它）。
//
// 前移是安全的：buildZoneIdentityWrite 只用 zones 的 ZoneKey / Method / PriceLow /
// PriceHigh / Role，**沒有用到 DB id**。
//
// **nil 代表這次沒有身分可用**（沒接 repo、沒有 zone，或取資料／比對失敗）。呼叫端一律
// 當降級處理：zone_uid 留空、事件層跳過、**分析本身照常成立**——這條語意在階段 E 之前
// 就是這樣（每一步失敗都是 warn ＋ return nil），前移之後更要守住，因為比對現在發生在
// 分析還沒落地的時點。
type zoneIdentityMatch struct {
	// timeframe 是正規化過的值，寫入段直接沿用，避免兩段各自正規化而分歧。
	timeframe string
	now       time.Time
	live      []store.LiveZone
	aliasRefs []store.ZoneKeyAliasRef
	matched   *analysis.ZoneIdentityMatchResult
}

// matchZoneIdentity 取身分比對需要的資料並呼叫 matcher。**在 repo.Create 之前跑。**
func (h *SRZoneHandler) matchZoneIdentity(
	ctx context.Context, symbol, timeframe string, zones []store.SRZone,
) *zoneIdentityMatch {
	if h.zoneIdentity == nil || len(zones) == 0 {
		return nil
	}
	if timeframe == "" {
		timeframe = "1d"
	}
	// **用台北日期**：trading_days 是 (ts AT TIME ZONE 'Asia/Taipei')::date，
	// 市場也是台北時區。用 UTC 的話，台北 00:00～08:00 執行的分析會回報成前一個日曆日，
	// sessions_between 就少算一個交易日——兩份「同一個日期空間」其實不同。
	now := time.Now().In(timeutil.TaipeiTZ)

	// notSeenSince 刻意放得比 matcher 的交易日上限寬：SQL 裡沒有交易日的概念，
	// 在這裡用日曆天硬算會比 matcher 嚴，把身分靜默丟掉而不收攤。
	// 精確判定由 matcher 用交易日曆做。
	live, err := h.zoneIdentity.ListLive(ctx, symbol, timeframe, zoneIdentityMaxAbsences)
	if err != nil {
		h.log.Warn("zone identity: list live failed", zap.Error(err))
		return nil
	}
	tradingDays, err := h.zoneIdentity.ListTradingDays(ctx, timeframe, zoneIdentityCalendarDays)
	if err != nil {
		h.log.Warn("zone identity: list trading days failed", zap.Error(err))
		return nil
	}
	// **在 Apply 之前讀**：讀到的是先前各次分析留下的 key，正好是「上次那個 zone
	// 長什麼樣」。本次的 key 由 UIDByZoneKey 覆蓋，不需要（也不該）混進來。
	//
	// 失敗只降級不中止：沒有 alias 就退回本次 map 單段查找，也就是修法前的行為。
	// 為了一份輔助索引把整條身分追蹤停掉，代價與收益不成比例。
	// 次數軸傳與 ListLive 同一個常數：alias 索引與 matcher 的候選集合必須由同一個
	// 地方定義，否則「還活著」會有兩套判準（T-048 F5）。
	aliasRefs, err := h.zoneIdentity.ListKeyAliases(ctx, symbol, timeframe,
		zoneIdentityMaxAbsences)
	if err != nil {
		h.log.Warn("zone identity: list key aliases failed", zap.Error(err))
	}

	current := make([]analysis.ZoneIdentityZone, 0, len(zones))
	for _, z := range zones {
		current = append(current, analysis.ZoneIdentityZone{
			PriceLow: z.PriceLow, PriceHigh: z.PriceHigh,
			Method: z.Method, Role: z.Role,
		})
	}

	matched, err := h.client.MatchZoneIdentities(ctx, now.Format("2006-01-02"),
		tradingDays, analysis.ZoneIdentityZonesFromLive(live), current)
	if err != nil {
		h.log.Warn("zone identity: match failed", zap.Error(err))
		return nil
	}
	return &zoneIdentityMatch{
		timeframe: timeframe,
		now:       now,
		live:      live,
		aliasRefs: aliasRefs,
		matched:   matched,
	}
}

// applyZoneUIDs 把比對結果填進 zones，讓 repo.Create 一次寫入就帶著身分（T-048 階段 E）。
//
// matcher 的輸出與 zones 是**索引對齊**的（current 就是照 zones 的順序組的）。
// 長度對不上時寧可留空也不猜——錯位的身分比沒有身分更難發現。
func applyZoneUIDs(zones []store.SRZone, m *zoneIdentityMatch) {
	if m == nil || m.matched == nil {
		return
	}
	for i := range zones {
		if i >= len(m.matched.ZoneUIDs) {
			return
		}
		uid := m.matched.ZoneUIDs[i]
		if uid == "" {
			continue
		}
		zones[i].ZoneUID = store.NullString{NullString: sql.NullString{String: uid, Valid: true}}
	}
}

// persistZoneIdentity 把比對結果寫進四張身分表。**在 repo.Create 之後跑**，
// 因為 zone_role_incarnations 要 analysisID。
func (h *SRZoneHandler) persistZoneIdentity(
	ctx context.Context, symbol string, analysisID uint64, zones []store.SRZone,
	m *zoneIdentityMatch,
) *zoneIdentityOutcome {
	if m == nil || m.matched == nil {
		return nil
	}
	write := buildZoneIdentityWrite(symbol, m.timeframe, analysisID, m.now, zones, m.live, m.matched,
		func() string { return uuid.NewString() })
	if err := h.zoneIdentity.Apply(ctx, write); err != nil {
		h.log.Warn("zone identity: apply failed", zap.Error(err))
		// **這裡要 return nil**：身分沒寫進去，事件就沒有可以指的 zone_uid。
		// 讓階段 C 照跑會寫出指向不存在身分的事件鏈（sqlite 不擋外鍵時尤其安靜）。
		//
		// 注意 zones 的 zone_uid 這時**已經寫進 stock_sr_zones 了**（階段 E 的前移）。
		// 那是刻意的取捨：欄位可空、無外鍵，留著一個指不到 zone_instances 的 uid，
		// 比為了它把整筆分析退掉好。判讀時見 069 migration 註解的三種 NULL 語意。
		return nil
	}
	aliasByKey, aliasAmbiguous := aliasUIDByZoneKey(m.aliasRefs, m.matched.ExpiredPrevious)
	return &zoneIdentityOutcome{
		UIDByZoneKey:      zoneUIDByZoneKey(zones, m.matched.ZoneUIDs),
		AliasUIDByZoneKey: aliasByKey,
		AliasAmbiguous:    aliasAmbiguous,
		EndedZoneUIDs:     uidSet(m.matched.TerminatedPrevious),
	}
}

// zoneUIDByZoneKey 把「這次分析的 zone」對應到它拿到的身分。
//
// **同一個 zone_key 出現兩次時保留第一個**：key 是 role ＋ 邊界，兩個 method 算出
// 完全一樣的區間並非不可能。事件身上只帶 zone_key，本來就分不出是哪一個，
// 這裡選一個穩定的規則，而不是讓 map 的寫入順序決定。
func zoneUIDByZoneKey(zones []store.SRZone, uids []string) map[string]string {
	out := make(map[string]string, len(zones))
	for i, z := range zones {
		if i >= len(uids) || z.ZoneKey == "" {
			continue
		}
		if _, dup := out[z.ZoneKey]; dup {
			continue
		}
		out[z.ZoneKey] = uids[i]
	}
	return out
}

func uidSet(uids []string) map[string]struct{} {
	out := make(map[string]struct{}, len(uids))
	for _, u := range uids {
		out[u] = struct{}{}
	}
	return out
}

const (
	// 與 zone_matcher.MAX_OBSERVED_ABSENCES 同值。用 `<=` 撈，讓剛達上限的身分
	// 還能進來一次完成收攤（見 ZoneIdentityRepo.ListLive 的說明）。
	zoneIdentityMaxAbsences = 3
	// 交易日曆取多久。要涵蓋得住 lookback，否則 matcher 算距離時會低估。
	zoneIdentityCalendarDays = 120
)

// persistEventIdentity 把這次分析的事件狀態掛到 zone 的穩定身分上並落地（T-048 階段 C）。
//
// **失敗只記 log，不影響分析本身**，理由同 persistZoneIdentity：這兩張表沒有讀者。
//
// **依賴階段 B 先成功**：沒有 zone_uid 對應表就沒有可以指的身分，此時整段跳過，
// 而不是寫出 zone_uid 為 NULL 的 ZONE scope 事件——那與「這是 SYMBOL 事件」
// 在資料上無法區分。
func (h *SRZoneHandler) persistEventIdentity(
	ctx context.Context, symbol, timeframe string, analysisID uint64,
	states []store.MarketEventState, zoneOutcome *zoneIdentityOutcome,
) {
	if h.eventIdentity == nil || zoneOutcome == nil {
		return
	}
	if timeframe == "" {
		timeframe = "1d"
	}
	now := time.Now().In(timeutil.TaipeiTZ)

	latest, err := h.eventIdentity.ListLatestChains(ctx, symbol, timeframe)
	if err != nil {
		h.log.Warn("event identity: list latest chains failed", zap.Error(err))
		return
	}

	write, stats := buildEventIdentityWrite(symbol, timeframe, analysisID, now,
		states, latest, zoneOutcome, func() string { return uuid.NewString() })
	stats.AliasAmbiguous = append(stats.AliasAmbiguous, zoneOutcome.AliasAmbiguous...)

	h.logEventIdentityStats(symbol, timeframe, stats)

	if len(write.Instances) == 0 && len(write.Transitions) == 0 {
		return
	}
	if err := h.eventIdentity.Apply(ctx, write); err != nil {
		h.log.Warn("event identity: apply failed", zap.Error(err))
	}
}

// logEventIdentityStats 以**單筆**結構化 log 輸出整次分析的關聯決策拆解。
//
// 拆成多筆 log 會讓「這一次分析發生了什麼」要靠時間戳拼回去；一筆帶齊全部欄位，
// 日誌聚合可以直接對欄位做計數與趨勢。升級成可查詢的 metric 是 todo T-050。
//
// **三個級別是刻意分開的**：
//
//	Error — 終態不變式被違反。純函數只有一個終態寫入點，這裡非空代表有人繞過了它。
//	Warn  — 關聯失敗、衝突、carried 被擋下。都是「應該要有人看一眼」但不影響正確性。
//	Debug — 一切正常時的逐段命中數。每次分析都印 Info 會把日誌淹掉。
func (h *SRZoneHandler) logEventIdentityStats(symbol, timeframe string, s eventIdentityStats) {
	fields := []zap.Field{
		zap.String("symbol", symbol), zap.String("timeframe", timeframe),
		zap.Int("matched_by_chain", s.MatchedByChain),
		zap.Int("matched_by_current", s.MatchedByCurrent),
		zap.Int("matched_by_alias", s.MatchedByAlias),
		zap.Strings("unmatched_zone_keys", s.UnmatchedKeys),
		zap.Strings("carried_noop", s.CarriedNoop),
		zap.Strings("zone_ended_skipped", s.ZoneEndedSkipped),
		zap.Strings("chain_conflicts", s.ChainConflicts),
		zap.Strings("chain_key_ambiguous", s.ChainKeyAmbiguous),
		zap.Strings("alias_ambiguous", s.AliasAmbiguous),
		zap.Int("carried_parse_failed", s.CarriedParseFail),
		zap.Strings("invariant_violations", s.Invariant),
	}
	switch {
	case len(s.Invariant) > 0:
		// 這是「不該發生」的那一類：ended_at 有值卻 state / active 沒跟上（F4）。
		h.log.Error("event identity: terminal state invariant violated", fields...)
	case s.hasWarning():
		h.log.Warn("event identity: zone association needs attention", fields...)
	default:
		h.log.Debug("event identity: zone association", fields...)
	}
}

// eventZoneMatchSource 記錄 zone_uid 是從哪一段解析出來的，只為了計數。
type eventZoneMatchSource int

const (
	eventZoneMatchNone eventZoneMatchSource = iota
	eventZoneMatchCurrent
	eventZoneMatchAlias
)

// eventIdentityStats 是一次分析的關聯決策拆解（T-048 階段 C 修法，P1 可觀測性）。
//
// **為什麼要拆到這個顆粒度**：F1 在 live 存在了兩週沒有人發現，因為它的症狀是
// 「鏈靜默凍結」——資料表面上完全正常，只有把「事件是怎麼找到身分的」逐段數出來
// 才看得見。單一個「成功／失敗」布林答不出「alias 命中率在上升」這種趨勢問題。
//
// backend 目前沒有 metrics 依賴，所以這份計數以單筆結構化 log 輸出；
// 升級成可查詢的 metric 是 todo T-050。
type eventIdentityStats struct {
	// MatchedByChain：第一把鑰匙命中，直接沿用既有鏈的 zone_uid，完全不解析 key。
	MatchedByChain int
	// MatchedByCurrent：本次分析的 zone_key → zone_uid map 命中。
	MatchedByCurrent int
	// MatchedByAlias：本次 map miss，退到 zone_key alias history 才命中。
	MatchedByAlias int
	// UnmatchedKeys：三段都到不了身分的 zone_key。**必須歸零**，非零代表還有
	// 第四種成因沒找出來，不能當已知限制帶過。
	UnmatchedKeys []string
	// CarriedNoop：carried 事件找不到活鏈，依定案不建立新 occurrence。
	CarriedNoop []string
	// ZoneEndedSkipped：身分本次因 SPLIT / MERGE / RESHAPE 終止，交給 D4 收攤。
	ZoneEndedSkipped []string
	// ChainConflicts：第一把鑰匙命中的鏈，其 zone_uid 與本次 map 給的不同。
	// 以鏈為準（使用者定案的優先序），但衝突本身要看得見。
	ChainConflicts []string
	// ChainKeyAmbiguous：同一個 (last_zone_key, family) 有兩條活鏈。
	ChainKeyAmbiguous []string
	// AliasAmbiguous：同一個 zone_key 對到多個仍活著的身分。
	AliasAmbiguous []string
	// CarriedParseFail：state_json 解析不出 carried_from_previous。
	// **一律當成 false**——當成 true 會靜默吃掉真實的新事件。
	CarriedParseFail int
	// Invariant：寫入前掃出來的終態矛盾。**恆應為空**，非空是 Error 級別。
	Invariant []string
}

// hasWarning 決定這一筆拆解要不要升到 Warn。
//
// **CarriedNoop 刻意不在裡面**（2026-08-19 階梯實跑後修正，與計畫書一致）。
// Python 的 carry forward 會把**終態**的事件狀態每次分析都重報一次，所以每一條
// 走完生命週期的鏈都會在此後的每一次分析各貢獻一筆 CarriedNoop——七階 0050 的實測是
// 單調累積到 5 筆、每一階都觸發。那是護欄**正常運作**的樣子，不是要人看一眼的事；
// 讓它升 Warn 等於保證 warn 永遠不會歸零，真正的異常（UnmatchedKeys）就被淹在裡面。
// 它仍然每次都出現在 log 欄位裡，只是不再自己決定級別。
//
// AliasAmbiguous 則留在清單裡：同一個 zone_key 對到多個活身分是真的該有人看一眼，
// 性質與 ChainKeyAmbiguous 相同（計畫書列舉時漏了它，這裡刻意補上）。
func (s eventIdentityStats) hasWarning() bool {
	return len(s.UnmatchedKeys) > 0 || len(s.ChainConflicts) > 0 ||
		len(s.ChainKeyAmbiguous) > 0 || len(s.AliasAmbiguous) > 0 ||
		s.CarriedParseFail > 0
}

// applyEventTerminalState 是**唯一**的事件鏈終態寫入點（T-048 階段 C 的 F4 修法）。
//
// 原本的兩條終態路徑各自設一部分欄位，D4 那條只設了 ended_at / end_reason，
// state 與 active 沿用舊值——寫出來就是「ended_at 有值、transition 說已 EXPIRED，
// 但 state 仍是 ACTIVE 且 active=true」的自相矛盾資料。這在只寫不讀的階段沒有後果，
// 但 T-049 / T-041 讀新表時會把它當成還可用的事件。
//
// 四個欄位綁在一個函數裡，就沒有「只改了一半」的寫法可選。
func applyEventTerminalState(inst *store.EventInstance, state, reason string, now time.Time) {
	inst.State = state
	inst.Active = false
	inst.EndedAt = sql.NullTime{Time: now, Valid: true}
	inst.EndReason = sql.NullString{String: reason, Valid: true}
}

// eventCarriedFromPrevious 從 state_json 讀出「這次沒有新偵測，只是把上次的狀態抄過來」。
//
// Python 在 `_normalize_previous_event_state` 無條件把它設成 true，所以它恰好等於
// 「這一筆是重報」。**旗標只從 state_json 讀，不動 market_event_states 的欄位**——
// 「既有欄位逐欄相同」是階段 C 的驗收條件。
//
// 第二個回傳值是「有沒有讀到」。讀不到時呼叫端一律當 false 並計數：當成 true 會讓
// 真實的新事件被護欄吃掉，而那個損失是靜默的；當成 false 最多讓一筆重報開了新鏈，
// 會被階梯門檻②抓到。
func eventCarriedFromPrevious(raw store.RawJSON) (carried bool, parsed bool) {
	if len(raw) == 0 {
		return false, false
	}
	var payload struct {
		Carried *bool `json:"carried_from_previous"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil || payload.Carried == nil {
		return false, false
	}
	return *payload.Carried, true
}

// eventDecisionVisible 讀 state_json 的 decision_visible：這個事件能不能被決策看到。
//
// **缺鍵一律當 true**，與 eventCarriedFromPrevious 的「缺鍵是異常」刻意相反：
// 既有的四個事件型別都是決策可見的，而階段 D 之前寫進 market_event_states 的列
// 根本不會有這個鍵。當成 false 會讓既有事件整批從決策桶消失——那是最嚴重的行為改變。
//
// 旗標由 Python 單一產生（event_engine.EVENT_TYPE_META），Go 只讀不推導，
// 理由與 carried_from_previous 相同：兩份型別清單分歧時沒有東西會報錯。
func eventDecisionVisible(raw store.RawJSON) bool {
	if len(raw) == 0 {
		return true
	}
	var payload struct {
		Visible *bool `json:"decision_visible"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil || payload.Visible == nil {
		return true
	}
	return *payload.Visible
}

// buildEventIdentityWrite 把這次分析的事件狀態組成要寫的兩張表內容。
//
// 拆成純函數的理由與 buildZoneIdentityWrite 相同：這段的錯誤（事件掛錯 zone、
// 鏈斷掉、root 被 latest 蓋掉）在資料裡看起來都很正常。
//
// **關聯決策是三段的**（2026-08-19 定案，現況見 docs/sr-zone-scoring.md
// 「事件層：鏈的身分與三段關聯決策」）：
//
//	① 既有活鏈以 (last_zone_key, family) 直接命中 → 沿用 chain.zone_uid，不解析 key
//	② 沒有活鏈 ＋ carried_from_previous == true → WARN / NOOP，**不建立新 occurrence**
//	③ 沒有活鏈 ＋ 非 carried → 解析 zone：本次 key map → miss 退到 alias history
//
// 順序的含義：**instance 延續性 > carried 護欄 > key 解析**。①解的是 F1
// 「鏈靜默凍結」——身分還活著、只是 key 到不了它；②解的是 F2「終態每次分析重生」。
//
// ③之後**還要再查一次活鏈**（第二把鑰匙，`(zone_uid, family)`）才決定延續或開新鏈。
// 兩把鑰匙不等價：①用的是事件帶的 key，③產出的是 zone_uid。少了第二把，
// 「zone 解析得出來、但鏈記的 key 是舊的」會在同一個身分上開出第二條活鏈，
// 舊鏈被 ListLatestChains 的 MAX(seq) 蓋掉——F1 原樣重現，只是換了成因。
//
// 第四種寫法在迴圈之外：
//
//	zone 身分終止 → 鏈收掉，end_reason = ZONE_IDENTITY_ENDED（D4：不傳給 child）
func buildEventIdentityWrite(
	symbol, timeframe string, analysisID uint64, now time.Time,
	states []store.MarketEventState, latest []store.LiveEvent,
	zoneOutcome *zoneIdentityOutcome, uidFactory func() string,
) (store.EventIdentityWrite, eventIdentityStats) {
	analysisRef := sql.NullInt64{Int64: int64(analysisID), Valid: true}
	w := store.EventIdentityWrite{}
	stats := eventIdentityStats{}

	// 兩把鑰匙各一個索引。
	//
	// byScopeKey 含**已終結**的鏈：開新鏈時 seq 要從歷史最大值往上加，只看活鏈會一直
	// 算出 seq=1 而撞 uq_event_instance_seq，且那個失敗是靜默的（寫入只記 log）。
	// byZoneKey **只含活鏈**：已終結的鏈不該被延續。
	byScopeKey := make(map[string]store.LiveEvent, len(latest))
	byZoneKey := make(map[string]store.LiveEvent, len(latest))
	for _, e := range latest {
		byScopeKey[e.ZoneScopeKey+"|"+e.EventFamily] = e
		if e.EndedAt.Valid || !e.LastZoneKey.Valid || e.LastZoneKey.String == "" {
			continue
		}
		k := e.LastZoneKey.String + "|" + e.EventFamily
		// **保留第一個**：兩個身分曾用過同一個 key 並非不可能（同區間不同 method）。
		// latest 的順序由 SQL 的 ORDER BY 決定，所以這是穩定的規則，
		// 不是讓 map 的寫入順序做決定。
		if _, dup := byZoneKey[k]; dup {
			stats.ChainKeyAmbiguous = append(stats.ChainKeyAmbiguous, k)
			continue
		}
		byZoneKey[k] = e
	}
	written := make(map[string]struct{}, len(states))

	for _, st := range states {
		// SYMBOL scope 的事件不屬於任何 zone，鍵一律是 'SYMBOL'。
		// 但①②兩段照樣適用——SYMBOL 的鏈也會重生。
		lookupKey := st.ZoneKey
		isZoneScope := st.EventScope != "SYMBOL"
		if !isZoneScope {
			lookupKey = store.SymbolScopeKey
		}

		var (
			chain    store.LiveEvent
			hasChain bool
			zoneUID  sql.NullString
			scopeKey string
		)

		if c, ok := byZoneKey[lookupKey+"|"+st.EventFamily]; ok {
			// ── ① 既有活鏈優先 ──
			//
			// **守門**：這條鏈掛的身分若本次終止，不能延續它。放行的話 D4 的迴圈會因為
			// written 去重而跳過它，鏈就永遠收不掉——事件留在一個已經不存在的 zone 上。
			if c.ZoneUID.Valid {
				if _, ended := zoneOutcome.EndedZoneUIDs[c.ZoneUID.String]; ended {
					stats.ZoneEndedSkipped = append(stats.ZoneEndedSkipped, c.EventUID)
					continue
				}
				// 本次 map 對同一個 key 給出不同身分。**以鏈為準**（定案的優先序），
				// 但要看得見：這代表 zone 匹配出現了新的成因。
				if uid, ok := zoneOutcome.UIDByZoneKey[st.ZoneKey]; ok && uid != c.ZoneUID.String {
					stats.ChainConflicts = append(stats.ChainConflicts, c.EventUID)
				}
			}
			chain, hasChain = c, true
			zoneUID, scopeKey = c.ZoneUID, c.ZoneScopeKey
			if isZoneScope {
				stats.MatchedByChain++
			}
		} else {
			// ── ② carried 護欄 ──
			//
			// **carried_from_previous == false 是建立新 occurrence 的必要條件。**
			// carried 代表「這次重報了上次的狀態」，不代表「這次發生了事件」；
			// 找不到活鏈就表示這條鏈已經被收掉或從未建立，此時開新鏈是把重報寫成新事件
			// （F2：同一筆終態每次分析都誕生一條 seq+1 的新鏈）。狀態是不是終態
			// 不參與這個判斷。
			carried, parsed := eventCarriedFromPrevious(st.StateJSON)
			if !parsed {
				stats.CarriedParseFail++
			}
			if carried {
				stats.CarriedNoop = append(stats.CarriedNoop, lookupKey+"|"+st.EventFamily)
				continue
			}

			// ── ③ 解析 zone ──
			if !isZoneScope {
				scopeKey = store.SymbolScopeKey
			} else {
				uid, src := resolveEventZoneUID(st.ZoneKey, zoneOutcome)
				if src == eventZoneMatchNone {
					// 三段都到不了身分。**不寫成 NULL zone_uid**——那與「這是 SYMBOL
					// scope 的事件」在資料上長得一模一樣，之後只會看到「事件變少了」。
					stats.UnmatchedKeys = append(stats.UnmatchedKeys, st.ZoneKey)
					continue
				}
				if _, ended := zoneOutcome.EndedZoneUIDs[uid]; ended {
					stats.ZoneEndedSkipped = append(stats.ZoneEndedSkipped, st.ZoneKey)
					continue
				}
				if src == eventZoneMatchAlias {
					stats.MatchedByAlias++
				} else {
					stats.MatchedByCurrent++
				}
				zoneUID = sql.NullString{String: uid, Valid: true}
				scopeKey = uid
			}

			// ── 匯流點：第二把鑰匙 ──
			//
			// zone_uid 到這裡才第一次已知。同一個身分上這個 family 若已有活鏈，
			// **要沿用它而不是開第二條**：兩條活鏈時 ListLatestChains 只回 MAX(seq)，
			// 舊鏈從此撈不出來、last_seen_at 不再更新——正是 F1 的靜默凍結，
			// 只是成因從「key 到不了身分」換成「身分到不了鏈」。
			if c, ok := byScopeKey[scopeKey+"|"+st.EventFamily]; ok && !c.EndedAt.Valid {
				chain, hasChain = c, true
			}
		}

		key := scopeKey + "|" + st.EventFamily
		if _, dup := written[key]; dup {
			continue
		}
		written[key] = struct{}{}

		eventUID := chain.EventUID
		seq := chain.Seq
		firstSeen := chain.FirstSeenAt
		rootType := chain.RootEventType
		var fromState sql.NullString
		if hasChain {
			fromState = sql.NullString{String: chain.State, Valid: true}
		} else {
			// seq 從**歷史最大值**往上加，所以查含已終結鏈的那個索引。
			eventUID = uidFactory()
			seq = byScopeKey[key].Seq + 1
			if seq < 1 {
				seq = 1
			}
			firstSeen = now
			// **新鏈的 root 是這次的事件**，不是沿用上一條鏈的——上一條已經 RESOLVED
			// 或 EXPIRED，接下去等於宣稱它復活了。規則與 Python 的
			// build_event_state_summary 對稱。
			rootType = st.RootEventType
		}

		inst := store.EventInstance{
			EventUID: eventUID, Symbol: symbol, Timeframe: timeframe,
			ZoneUID: zoneUID, ZoneScopeKey: scopeKey,
			EventScope: st.EventScope, EventFamily: st.EventFamily, Seq: seq,
			RootEventType: rootType, LatestEventType: st.LatestEventType,
			State: st.State, Active: st.Active, Direction: st.Direction,
			ResolvedBy:  st.ResolvedBy.NullString,
			FirstSeenAt: firstSeen, LastSeenAt: now,
			// **每次都更新成本次事件帶的 key**：停在誕生那天的值會讓第一把鑰匙
			// 往後永遠 miss，整個①段等於沒做。
			LastZoneKey: sql.NullString{String: lookupKey, Valid: lookupKey != ""},
		}
		// RESOLVED / EXPIRED 是鏈的終態。**要在這裡收掉**，否則它會永遠停在未終結，
		// 下次分析把它當成「還活著」而延續下去，同一條鏈就吃掉了兩段本該分開的歷史。
		if st.State == "RESOLVED" || st.State == "EXPIRED" {
			applyEventTerminalState(&inst, st.State, st.State, now)
		}
		w.Instances = append(w.Instances, inst)

		// 只在狀態真的變了（或鏈剛誕生）才留痕。每次分析都寫一筆會讓流水變成
		// 「分析次數的紀錄」而不是「狀態轉換的紀錄」。
		if !hasChain || chain.State != st.State {
			w.Transitions = append(w.Transitions, store.EventTransition{
				EventUID: eventUID, AnalysisID: analysisRef,
				FromState: fromState, ToState: st.State,
				TriggerEventType: sqlNullString(st.LatestEventType),
				ReasonCodes:      store.RawJSON(`[]`),
				OccurredAt:       now,
			})
		}
	}

	// ── zone 身分終止 → 鏈跟著收掉（D4）──
	// parent 的事件**不傳給 child**：那條鏈的前提（這個 zone 存在）已經消失，
	// 接到 child 等於宣稱鏈延續了，而 RESHAPE 的定義正是「血緣無法解析」。
	// 血緣留在 zone_relations，日後要沿 parent 回溯補得回來；反過來拆不回來。
	for _, e := range latest {
		if e.EndedAt.Valid || !e.ZoneUID.Valid {
			continue
		}
		if _, ended := zoneOutcome.EndedZoneUIDs[e.ZoneUID.String]; !ended {
			continue
		}
		if _, dup := written[e.ZoneScopeKey+"|"+e.EventFamily]; dup {
			continue
		}
		inst := e.EventInstance
		inst.LastSeenAt = e.LastSeenAt // 不是 now：這次並沒有看到它
		// 終態四個欄位一起設。舊版只設了 ended_at / end_reason，state 沿用舊值
		// （必然是 ACTIVE / CONFIRMED 且 active=true，因為進得來這個分支就代表鏈還活著），
		// 寫出來就是「ended_at 有值卻 active=true」——F4。
		applyEventTerminalState(&inst, "EXPIRED", "ZONE_IDENTITY_ENDED", now)
		w.Instances = append(w.Instances, inst)
		w.Transitions = append(w.Transitions, store.EventTransition{
			EventUID: e.EventUID, AnalysisID: analysisRef,
			// **明寫舊狀態**：留白等同「鏈誕生」，那條不變式沿用 067 的定案。
			FromState:   sql.NullString{String: e.State, Valid: true},
			ToState:     "EXPIRED",
			ReasonCodes: store.RawJSON(`["ZONE_IDENTITY_ENDED"]`),
			OccurredAt:  now,
		})
	}

	// 寫入前的最後一道：終態四個欄位必須一致。純函數已經只有 applyEventTerminalState
	// 一個終態寫入點，所以這裡**恆應為空**——非空代表有人繞過了它。
	for _, inst := range w.Instances {
		if !inst.EndedAt.Valid {
			continue
		}
		if inst.Active || (inst.State != "RESOLVED" && inst.State != "EXPIRED") {
			stats.Invariant = append(stats.Invariant, inst.EventUID)
		}
	}

	return w, stats
}

// resolveEventZoneUID 是決策樹第三段：本次分析的 key map → miss 退到 alias history。
//
// **alias 只服務全新關聯**。既有鏈條由第一把鑰匙直接接住，連 key 都不必解析——
// 把 alias 也拿來救既有鏈條會讓「鏈延續」重新依賴 key 的正確性，那正是 F1 的成因。
func resolveEventZoneUID(zoneKey string, o *zoneIdentityOutcome) (string, eventZoneMatchSource) {
	if uid, ok := o.UIDByZoneKey[zoneKey]; ok {
		return uid, eventZoneMatchCurrent
	}
	if uid, ok := o.AliasUIDByZoneKey[zoneKey]; ok {
		return uid, eventZoneMatchAlias
	}
	return "", eventZoneMatchNone
}

// aliasUIDByZoneKey 把 ListKeyAliases 的列摺成 `zone_key → zone_uid`，並回報撞號的 key。
//
// SQL 已經按 (zone_key, last_seen_at DESC) 排好，所以**第一個就是最新的那個身分**。
// 同一個 key 對到多個活身分時取最新並計數——在 SQL 裡靜靜挑一個會讓「有多少衝突」
// 永遠問不出來。
//
// `expired` 是 matcher 這一輪判定**失格**的身分（`ZoneIdentityMatchResult.ExpiredPrevious`），
// 一律排除。這是**兩道過濾中的第二道，只負責「本輪」**：
//
//	ListKeyAliases  observed_absences <= zoneIdentityMaxAbsences（與 ListLive 同一個閘門）
//	這裡            再扣掉 matcher 這一輪剛判失格、但次數還沒推過上限的那些
//
// 為什麼兩道都要：階段 B 的定案是**失格只收掉「這一世」，身分本身仍是 ACTIVE**，
// 所以 alias 的 SQL 若只看 `state='ACTIVE' AND ended_at IS NULL`，matcher 早就放棄的
// 身分照樣是候選；而只靠這裡的 `expired` 又補不起來——失格身分下一輪就被次數軸擋在
// matcher 之前，一生只會出現在 `expired_previous` 一次。2026-08-19 每日階梯實測：
// 只做這一道時仍有 77 筆 `alias_ambiguous`、16 個 key 撞號
// （見 docs/sr-zone-scoring.md「實測特性」）。
//
// **`TerminatedPrevious` 刻意不排除**：那些身分是這一輪因 SPLIT / MERGE / RESHAPE
// 終止的，它們身上的事件要走 D4 收攤（`EndedZoneUIDs` → `zone_ended_skipped`）。
// 把它們從 alias 拿掉，那些事件會變成「關聯不到身分」，等於把一個有解釋的收攤
// 偽裝成 F1 那種必須歸零的失敗訊號。
func aliasUIDByZoneKey(
	refs []store.ZoneKeyAliasRef, expired []string,
) (map[string]string, []string) {
	skip := uidSet(expired)
	out := make(map[string]string, len(refs))
	var ambiguous []string
	for _, r := range refs {
		if _, gone := skip[r.ZoneUID]; gone {
			continue
		}
		if _, dup := out[r.ZoneKey]; dup {
			ambiguous = append(ambiguous, r.ZoneKey)
			continue
		}
		out[r.ZoneKey] = r.ZoneUID
	}
	return out, ambiguous
}

// buildZoneIdentityWrite 把 matcher 的結果組成一次交易要寫的四張表內容。
//
// 拆成獨立函數是為了讓它可以在沒有 HTTP 與 DB 的情況下測——這段的錯誤
// （身分與 zone 錯位、缺席次數沒存回去、終止的身分沒收攤）在資料裡看起來都很正常。
//
// 四種身分各有各的寫法，**漏掉任何一種都會讓身分無法穩定**：
//
//	這次看到的     → ACTIVE、缺席歸零、必要時開新的一世
//	分裂／合併的   → 終態 ＋ ended_at。漏掉的話它會永遠停在 ACTIVE 並重新進入
//	                 candidate set，每次分析都再分裂一次，child 每次拿到全新 uid
//	失格的         → 一世收成 EXPIRED，缺席次數推過上限
//	沒看到但仍有效 → 只加缺席次數，**不動 last_seen_at**
func buildZoneIdentityWrite(
	symbol, timeframe string, analysisID uint64, now time.Time,
	zones []store.SRZone, live []store.LiveZone, m *analysis.ZoneIdentityMatchResult,
	uidFactory func() string,
) store.ZoneIdentityWrite {
	byUID := make(map[string]store.LiveZone, len(live))
	for _, z := range live {
		byUID[z.ZoneUID] = z
	}
	analysisRef := sql.NullInt64{Int64: int64(analysisID), Valid: true}

	w := store.ZoneIdentityWrite{}
	written := make(map[string]struct{}, len(zones)+len(live))
	openedIncarnation := make(map[string]string, len(zones))

	// 翻轉由 matcher 判定，這裡不重推——它比對的是「當前這一世的角色」，
	// 而那個規則跨 AT_ZONE 才成立（見 zone_matcher 的 _role_transition）。
	flipped := make(map[string]string, len(m.RoleTransitions))
	for _, t := range m.RoleTransitions {
		if t.Kind == "ROLE_FLIPPED" {
			flipped[t.ZoneUID] = t.ToRole
		}
	}

	// ── 一、這次觀測到的 zone ──
	for i, z := range zones {
		uid := m.ZoneUIDs[i]
		// alias 記在 dup 檢查**之前**：兩個 zone 匹配到同一個身分時，兩個 key 都是
		// 這個身分被觀測到的樣子，兩筆都該留。repo 的 dedupe 會處理完全相同的那對。
		if z.ZoneKey != "" {
			w.KeyAliases = append(w.KeyAliases, store.ZoneKeyAlias{
				ZoneUID: uid, ZoneKey: z.ZoneKey,
				FirstSeenAt: now, LastSeenAt: now,
			})
		}
		if _, dup := written[uid]; dup {
			continue
		}
		written[uid] = struct{}{}

		prev, existing := byUID[uid]
		first := now
		if existing {
			first = prev.FirstSeenAt
		}
		w.Instances = append(w.Instances, store.ZoneInstance{
			ZoneUID: uid, Symbol: symbol, Timeframe: timeframe, Method: z.Method,
			State: "ACTIVE", PriceLow: z.PriceLow, PriceHigh: z.PriceHigh,
			FirstSeenAt: first, LastSeenAt: now, ObservedAbsences: 0,
			// 這次觀測到的 role。與一世的角色不同——AT_ZONE 也要如實記下來，
			// 否則下次分不出「這次才進 AT_ZONE」與「已經在裡面好幾次了」。
			LastRole: z.Role,
		})

		if inc := incarnationForObservedZone(uid, z.Role, now, prev, existing,
			flipped, uidFactory); len(inc) > 0 {
			w.Incarnations = append(w.Incarnations, inc...)
			// 翻轉開出來的新一世要能被 role transition 指到（見下方 RoleTransitions 迴圈）。
			openedIncarnation[uid] = inc[len(inc)-1].IncarnationUID
		}

		// ── 誕生也是一次轉換 ──
		// 少了這一筆，zone_transitions 就不是完整的事件流：下游要問「這個身分何時出現」
		// 得改查 zone_instances.first_seen_at，於是同一個問題要跨兩張表用兩種語意回答。
		// **誕生是唯一 from_state 為 NULL 的 STATE_CHANGE**——失格與終態都從 ACTIVE 出發。
		// SPLIT / MERGE / RESHAPE 的 child 也是走這條路徑拿新 uid，所以這裡一併涵蓋，
		// 不需要在血緣那幾段各補一次。不變式見 docs/database-schema.md。
		if !existing {
			w.Transitions = append(w.Transitions, store.ZoneTransition{
				ZoneUID: uid,
				// AT_ZONE 誕生時不開一世，這裡就是 NULL——不是漏帶。
				IncarnationUID: sqlNullString(openedIncarnation[uid]),
				AnalysisID:     analysisRef,
				TransitionKind: "STATE_CHANGE",
				ToState:        sql.NullString{String: "ACTIVE", Valid: true},
				ReasonCodes:    store.RawJSON(`["IDENTITY_CREATED"]`),
				OccurredAt:     now,
			})
		}
	}

	// ── 二、因分裂／合併／重整而終止的身分 ──
	// **這一段先前整個漏掉**，後果是 parent 永遠停在 ACTIVE：下次分析它照樣被
	// ListLive 撈出來、幾何上仍配得到自己的 child，於是變成 N→M 重整，
	// child 又拿到全新 uid——身分永遠不會穩定，正是這個功能要消滅的 churn。
	terminalState := terminalStateByParent(m.Relations)
	for _, uid := range m.TerminatedPrevious {
		if _, dup := written[uid]; dup {
			continue
		}
		prev, ok := byUID[uid]
		if !ok {
			continue
		}
		written[uid] = struct{}{}
		state := terminalState[uid]
		if state == "" {
			state = "RESHAPED"
		}
		w.Instances = append(w.Instances, instanceFrom(prev, symbol, timeframe, state, now,
			prev.ObservedAbsences))
		if prev.IncarnationUID.Valid {
			w.Incarnations = append(w.Incarnations, closeIncarnation(prev, now,
				"IDENTITY_ENDED", false))
		}
		w.Transitions = append(w.Transitions, store.ZoneTransition{
			ZoneUID: uid, IncarnationUID: prev.IncarnationUID, AnalysisID: analysisRef,
			TransitionKind: "STATE_CHANGE",
			FromState:      sql.NullString{String: "ACTIVE", Valid: true},
			ToState:        sql.NullString{String: state, Valid: true},
			ReasonCodes:    store.RawJSON(`["IDENTITY_ENDED"]`),
			OccurredAt:     now,
		})
	}

	// ── 三、失格的身分 ──
	for _, uid := range m.ExpiredPrevious {
		if _, dup := written[uid]; dup {
			continue
		}
		prev, ok := byUID[uid]
		if !ok {
			continue
		}
		written[uid] = struct{}{}

		// **一定要推過上限**，否則時間軸造成的失格會每次分析重複收攤：
		// 那條路徑觸發時 observed_absences 可能還很小（例如 1），+1 之後仍在
		// ListLive 的範圍內，下次又被撈出來、又失格一次，留下重複的
		// EXPIRED_BY_ABSENCE 紀錄。次數軸自然會越界，時間軸不會。
		absences := zoneIdentityMaxAbsences + 1
		if next, ok := m.NextObservedAbsences[uid]; ok && next > absences {
			absences = next
		}
		// **用 prev.LastSeenAt 而不是 now**：upsert 對 last_seen_at 取大，
		// 傳 now 等於宣告「這個剛被判定不再認得的身分今天有被看到」，
		// 而 idx_zone_instances_live 正是 (symbol, timeframe, state, last_seen_at)——
		// 階段 C 沿著它查會撈到一堆看起來很新鮮的鬼魂。
		w.Instances = append(w.Instances, instanceFrom(prev, symbol, timeframe, "ACTIVE",
			prev.LastSeenAt, absences))

		if prev.IncarnationUID.Valid {
			w.Incarnations = append(w.Incarnations, closeIncarnation(prev, now,
				"EXPIRED_BY_ABSENCE", true))
		}
		w.Transitions = append(w.Transitions, store.ZoneTransition{
			ZoneUID: uid,
			// 帶上 incarnation_uid：少了它，事後問「這筆 EXPIRED 收掉的是哪一世」
			// 只能靠時間戳去猜。
			IncarnationUID: prev.IncarnationUID,
			AnalysisID:     analysisRef,
			TransitionKind: "STATE_CHANGE",
			// **明寫 ACTIVE**：失格前這個身分一定是 ACTIVE（ListLive 只撈 ACTIVE）。
			// 留白會讓它與誕生那筆一樣都是 NULL，`from_state IS NULL` 就分不出兩者。
			FromState:   sql.NullString{String: "ACTIVE", Valid: true},
			ToState:     sql.NullString{String: "EXPIRED", Valid: true},
			ReasonCodes: store.RawJSON(`["EXPIRED_BY_ABSENCE"]`),
			OccurredAt:  now,
		})
	}

	// ── 四、沒看到但仍有資格：只加缺席次數 ──
	for _, z := range live {
		next, ok := m.NextObservedAbsences[z.ZoneUID]
		if !ok || next == 0 {
			continue
		}
		if _, dup := written[z.ZoneUID]; dup {
			continue
		}
		written[z.ZoneUID] = struct{}{}
		// **不動 last_seen_at**：填成本次時間等於宣告「它剛被看到」，
		// 時間軸閘門從此永遠不會觸發。
		w.Instances = append(w.Instances, instanceFrom(z, symbol, timeframe, "ACTIVE",
			z.LastSeenAt, next))
	}

	for _, rel := range m.Relations {
		w.Relations = append(w.Relations, store.ZoneRelation{
			ParentZoneUID: rel.ParentZoneUID, ChildZoneUID: rel.ChildZoneUID,
			Relation: rel.Relation, AnalysisID: analysisRef, OccurredAt: now,
		})
	}

	for _, t := range m.RoleTransitions {
		// 翻轉是**唯一會關掉一世又開一世**的事件，所以最需要這條連結。
		// 新開的優先（翻轉後這筆屬於新的一世），否則沿用既有未結束的那筆。
		inc := sqlNullString(openedIncarnation[t.ZoneUID])
		if !inc.Valid {
			if prev, ok := byUID[t.ZoneUID]; ok {
				inc = prev.IncarnationUID
			}
		}
		w.Transitions = append(w.Transitions, store.ZoneTransition{
			ZoneUID: t.ZoneUID, IncarnationUID: inc, AnalysisID: analysisRef,
			TransitionKind: t.Kind,
			FromRole:       sqlNullString(t.FromRole),
			ToRole:         sqlNullString(t.ToRole),
			OccurredAt:     now,
		})
	}

	return w
}

// incarnationForObservedZone 維護「一世」。
//
// **先前完全沒有人建立一世**，於是 zone_role_incarnations 永遠是空的、
// ListLive 的 incarnation_role 永遠是 NULL，matcher 因此把每個有向 zone 都當成
// 「第一次解析出方向」——ROLE_UNRESOLVED 永遠不會發生，而 ROLE_FLIPPED
// （這整個功能的動機）**永遠偵測不到**。
//
// 回傳可能是 0、1（新開）或 2（翻轉：關舊的＋開新的）筆。
func incarnationForObservedZone(
	uid, role string, now time.Time, prev store.LiveZone, existing bool,
	flipped map[string]string, uidFactory func() string,
) []store.ZoneRoleIncarnation {
	// AT_ZONE 不開一世：它是「方向暫時無法解析」，不是角色。
	if role != "SUPPORT" && role != "RESISTANCE" {
		return nil
	}

	openSameRole := existing && prev.IncarnationUID.Valid &&
		prev.IncarnationRole.String == role
	_, isFlip := flipped[uid]
	if openSameRole && !isFlip {
		return nil // 這一世繼續，沒有要寫的
	}

	out := make([]store.ZoneRoleIncarnation, 0, 2)
	if existing && prev.IncarnationUID.Valid {
		out = append(out, closeIncarnation(prev, now, "ROLE_FLIPPED", false))
	}
	seq := int(prev.IncarnationMaxSeq.Int64) + 1
	if seq < 1 {
		seq = 1
	}
	out = append(out, store.ZoneRoleIncarnation{
		IncarnationUID: uidFactory(), ZoneUID: uid, Seq: seq, Role: role,
		State: "ACTIVE", StartedAt: now,
	})
	return out
}

func closeIncarnation(
	prev store.LiveZone, now time.Time, reason string, expired bool,
) store.ZoneRoleIncarnation {
	inc := store.ZoneRoleIncarnation{
		IncarnationUID: prev.IncarnationUID.String,
		ZoneUID:        prev.ZoneUID,
		Seq:            int(prev.IncarnationSeq.Int64),
		Role:           prev.IncarnationRole.String,
		State:          "INVALIDATED",
		StartedAt:      prev.FirstSeenAt,
		EndedAt:        sql.NullTime{Time: now, Valid: true},
		EndReason:      sql.NullString{String: reason, Valid: true},
	}
	if expired {
		inc.State = "EXPIRED"
		// expired_at 與 ended_at 分開：後者答「這一世何時結束」，
		// 前者答「何時被判定為不再認得」，是資格閘門的稽核依據。
		inc.ExpiredAt = sql.NullTime{Time: now, Valid: true}
	}
	return inc
}

func instanceFrom(
	prev store.LiveZone, symbol, timeframe, state string, lastSeen time.Time, absences int,
) store.ZoneInstance {
	inst := store.ZoneInstance{
		ZoneUID: prev.ZoneUID, Symbol: symbol, Timeframe: timeframe, Method: prev.Method,
		State: state, PriceLow: prev.PriceLow, PriceHigh: prev.PriceHigh,
		FirstSeenAt: prev.FirstSeenAt, LastSeenAt: lastSeen, ObservedAbsences: absences,
		// 沒觀測到就沿用舊值——這次沒看到它，不代表它變成別的角色。
		LastRole: prev.LastRole,
	}
	if state != "ACTIVE" {
		inst.EndedAt = sql.NullTime{Time: lastSeen, Valid: true}
	}
	return inst
}

// terminalStateByParent 依血緣型別決定 parent 的終態。
// 一個 parent 在同一次分析裡只會屬於一個元件，所以不會有衝突。
func terminalStateByParent(relations []analysis.ZoneIdentityRelation) map[string]string {
	out := make(map[string]string, len(relations))
	for _, rel := range relations {
		switch rel.Relation {
		case "SPLIT":
			out[rel.ParentZoneUID] = "SPLIT"
		case "MERGE":
			out[rel.ParentZoneUID] = "MERGED"
		case "RESHAPE":
			out[rel.ParentZoneUID] = "RESHAPED"
		}
	}
	return out
}

// 空字串代表「這個轉換沒有這一側的角色」，要存 NULL 而不是 ''——
// 之後查 from_role IS NULL 才問得出「哪些是第一次解析出方向」。
// 本檔已有一個語意不同的 nullableString（處理 store.NullString），故另取名。
func sqlNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
