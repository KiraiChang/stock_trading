package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/store"
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
	log       *zap.Logger
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

	previousEventStates, err := h.repo.GetLatestMarketEventStates(c.Request.Context(), body.Symbol, body.Timeframe)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: load previous event states")
		return
	}
	result, err := h.client.ScoreZonesWithPreviousEvents(c.Request.Context(), body.Symbol, body.Timeframe, body.Limit, previousEventStates)
	if err != nil {
		mapScoreZonesError(c, h.log, err)
		return
	}

	a, zones, projections, err := result.ToStore()
	if err != nil {
		serverError(c, h.log, err, "sr-zones: convert result to store")
		return
	}

	id, err := h.repo.Create(c.Request.Context(), a, zones, projections)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: create analysis")
		return
	}

	snapshot, err := h.loadSRZonePipelineSnapshot(c.Request.Context(), id)
	if err != nil {
		serverError(c, h.log, err, "sr-zones: load saved snapshot")
		return
	}

	c.JSON(http.StatusCreated, srZonePipelineResponse(snapshot))
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
