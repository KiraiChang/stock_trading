package scheduler

import (
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/market"
)

// 缺漏偵測參數的預設與界限。合法範圍與非法值處置見 `backend/config.yaml` 的
// `candle_gap_detection` 區段（原記於 issue.md I-091，已收斂）。
//
// **與 config.SetDefault 的那份是同一組數字，但職責不同**：viper 的預設處理「沒設」，
// 這裡處理「設了但不合法」。兩者都要有，因為 viper 對 `cap: 0` 這種值是照收的。
const (
	defaultGapAggregateRatio      = 0.5
	defaultGapAggregateMinSymbols = 5
	defaultGapCandidateCap        = 20
	maxGapCandidateCap            = 100
	defaultGapTimeoutSec          = 300
	maxGapTimeoutSec              = 900
	defaultGapLookbackDays        = 10
	maxGapLookbackDays            = 60
	defaultGapRequestIntervalMs   = 500
	minGapRequestIntervalMs       = 100
	defaultGapMarketStaleDays     = 2
	defaultGapCalendarTTLHours    = 24
	defaultGapBreakerFailures     = 5
	defaultGapBreakerCooldownMin  = 60
)

// NormalizeCandleGapDetectionConfig 把超出合法範圍的參數退回預設或截到界限，並逐項記
// Error log。**回傳的值可以被下游無條件信任**，之後的程式碼不再各自防禦。
//
// ⚠️ **匯出是刻意的，而且呼叫時機很重要**：`main.go` 必須**先**用它正規化，再拿
// 同一份結果去建 breaker 與 exchange client。原本只在 SetCandleGapDetection 裡正規化，
// 於是 client 拿到的是**原始非法值**——`request_interval_ms=0` 時 scheduler 顯示已
// 正規化成 100ms，實際上 client 完全不節流，直接違反「避免對交易所造成壓力」的風險對策。
//
// 本函式**冪等**（有測試釘住），所以 SetCandleGapDetection 再呼叫一次是安全的。
//
// **為什麼在這一層而不是 config 層**：`config.Load()` 在 `viper.Unmarshal` 失敗時直接
// 回 error，`main.go` 收到就 `os.Exit(1)`；而 `internal/config` 沒有 logger（實查零筆
// zap），記不了「已退回預設」這種必須被看見的訊息。所以型別解析失敗維持「啟動失敗」
// 的既有行為，超出範圍則在這裡正規化。形狀比照 `corporateActionCron()`：
// 非法值退回預設 ＋ 記 Error，再把已驗證的值交給下游。
//
// ⚠️ **記 Error 不是 Warn**：這些參數會直接改變告警結果，一個被悄悄改掉的
// `candidate_cap_per_run` 會讓偵測涵蓋率完全不同，而畫面上什麼都看不出來。
func NormalizeCandleGapDetectionConfig(
	cfg config.CandleGapDetectionConfig, log *zap.Logger,
) config.CandleGapDetectionConfig {
	// aggregate_ratio 合法範圍 (0, 1]：0 會讓任何缺口都短路成來源級告警（等於關掉逐檔
	// 核對），>1 則永遠不短路（全池缺漏時會展開 135 檔逐檔請求）。兩端都是嚴重後果。
	if cfg.AggregateRatio <= 0 || cfg.AggregateRatio > 1 {
		log.Error("candle_gap_detection.aggregate_ratio 超出 (0, 1]，退回預設",
			zap.Float64("got", cfg.AggregateRatio), zap.Float64("used", defaultGapAggregateRatio))
		cfg.AggregateRatio = defaultGapAggregateRatio
	}

	if cfg.AggregateMinSymbols < 1 {
		log.Error("candle_gap_detection.aggregate_min_symbols 須 >= 1，退回預設",
			zap.Int("got", cfg.AggregateMinSymbols), zap.Int("used", defaultGapAggregateMinSymbols))
		cfg.AggregateMinSymbols = defaultGapAggregateMinSymbols
	}

	// **0 不得放行**：一個候選都不驗卻仍回報 success，正是本筆要消滅的「看起來成功」。
	// **上限也不得省**：誤設大值會讓單輪請求量暴增，與「cap ＋ 間隔避免對交易所造成
	// 壓力」的風險對策直接衝突。
	if cfg.CandidateCapPerRun < 1 {
		log.Error("candle_gap_detection.candidate_cap_per_run 須 >= 1，退回預設",
			zap.Int("got", cfg.CandidateCapPerRun), zap.Int("used", defaultGapCandidateCap))
		cfg.CandidateCapPerRun = defaultGapCandidateCap
	} else if cfg.CandidateCapPerRun > maxGapCandidateCap {
		log.Error("candle_gap_detection.candidate_cap_per_run 超過上限，截到上限",
			zap.Int("got", cfg.CandidateCapPerRun), zap.Int("used", maxGapCandidateCap))
		cfg.CandidateCapPerRun = maxGapCandidateCap
	}

	if cfg.TimeoutSec <= 0 {
		log.Error("candle_gap_detection.timeout_sec 須為正，退回預設",
			zap.Int("got", cfg.TimeoutSec), zap.Int("used", defaultGapTimeoutSec))
		cfg.TimeoutSec = defaultGapTimeoutSec
	} else if cfg.TimeoutSec > maxGapTimeoutSec {
		log.Error("candle_gap_detection.timeout_sec 超過 hard cap，截到上限",
			zap.Int("got", cfg.TimeoutSec), zap.Int("used", maxGapTimeoutSec))
		cfg.TimeoutSec = maxGapTimeoutSec
	}

	// **0 會產生空視窗**：沒有預期日期＝沒有缺口＝永遠正常，是最徹底的靜默失效。
	if cfg.LookbackTradingDays < 1 {
		log.Error("candle_gap_detection.lookback_trading_days 須 >= 1，退回預設",
			zap.Int("got", cfg.LookbackTradingDays), zap.Int("used", defaultGapLookbackDays))
		cfg.LookbackTradingDays = defaultGapLookbackDays
	} else if cfg.LookbackTradingDays > maxGapLookbackDays {
		log.Error("candle_gap_detection.lookback_trading_days 超過上限，截到上限",
			zap.Int("got", cfg.LookbackTradingDays), zap.Int("used", maxGapLookbackDays))
		cfg.LookbackTradingDays = maxGapLookbackDays
	}

	// **截到下限而不是退回預設**：使用者的意圖是「快一點」，把 200 退回 500 等於反向
	// 調整。0 同樣截到 100——那不是合法值，取消對交易所的節流要改程式不是改設定。
	if cfg.RequestIntervalMs < minGapRequestIntervalMs {
		log.Error("candle_gap_detection.request_interval_ms 低於下限，截到下限",
			zap.Int("got", cfg.RequestIntervalMs), zap.Int("used", minGapRequestIntervalMs))
		cfg.RequestIntervalMs = minGapRequestIntervalMs
	}

	if cfg.MarketStaleDays < 1 {
		log.Error("candle_gap_detection.market_stale_days 須 >= 1，退回預設",
			zap.Int("got", cfg.MarketStaleDays), zap.Int("used", defaultGapMarketStaleDays))
		cfg.MarketStaleDays = defaultGapMarketStaleDays
	}

	if cfg.CalendarTTLHours < 1 {
		log.Error("candle_gap_detection.calendar_ttl_hours 須 >= 1，退回預設",
			zap.Int("got", cfg.CalendarTTLHours), zap.Int("used", defaultGapCalendarTTLHours))
		cfg.CalendarTTLHours = defaultGapCalendarTTLHours
	}

	// 0 會讓 breaker 永遠開著——所有來源從第一次就被跳過，偵測整條變成不可達。
	if cfg.BreakerFailures < 1 {
		log.Error("candle_gap_detection.breaker_failures 須 >= 1，退回預設",
			zap.Int("got", cfg.BreakerFailures), zap.Int("used", defaultGapBreakerFailures))
		cfg.BreakerFailures = defaultGapBreakerFailures
	}

	if cfg.BreakerCooldownMin < 1 {
		log.Error("candle_gap_detection.breaker_cooldown_min 須 >= 1，退回預設",
			zap.Int("got", cfg.BreakerCooldownMin), zap.Int("used", defaultGapBreakerCooldownMin))
		cfg.BreakerCooldownMin = defaultGapBreakerCooldownMin
	}

	return cfg
}

// NewCandleGapDependencies 正規化設定並用**同一份結果**建出 breaker 與 exchange client。
//
// ⚠️ **存在的理由就是把「正規化」與「建 client」綁在一起，讓兩者不可能再脫鉤。**
// 原本 main.go 是先用原始設定建 client、之後才在 SetCandleGapDetection 裡正規化，
// 於是 `request_interval_ms=0` 時 scheduler 顯示已正規化成 100ms、
// **client 卻完全不節流**；`breaker_failures=0` 時 client 用的是 NewSourceBreaker
// 兜底的 1，而不是規格預設的 5。
//
// 把它抽成一支函式之後，那個 bug 有地方可以測——直接斷言回傳的 breaker 行為，
// 而不是只斷言正規化函式的輸出（後者即使 main.go 改回用原始設定也照樣通過）。
func NewCandleGapDependencies(
	cfg config.CandleGapDetectionConfig, log *zap.Logger,
) (config.CandleGapDetectionConfig, *market.SourceBreaker, market.ExchangeReference) {
	normalized := NormalizeCandleGapDetectionConfig(cfg, log)
	breaker := market.NewSourceBreaker(
		normalized.BreakerFailures,
		time.Duration(normalized.BreakerCooldownMin)*time.Minute)
	reference := market.NewExchangeReference(breaker, market.ExchangeReferenceOptions{
		RequestInterval: time.Duration(normalized.RequestIntervalMs) * time.Millisecond,
		CalendarTTL:     time.Duration(normalized.CalendarTTLHours) * time.Hour,
	}, log)
	return normalized, breaker, reference
}
