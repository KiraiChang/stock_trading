package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server             ServerConfig
	Database           DatabaseConfig
	Redis              RedisConfig
	FinMind            FinMindConfig
	Fugle              FugleConfig
	Yahoo              YahooConfig
	StockSymbols       StockSymbolsConfig `mapstructure:"stock_symbols"`
	Python             PythonConfig
	Auth               AuthConfig
	Chip               ChipConfig
	SREvaluation       SREvaluationConfig       `mapstructure:"sr_evaluation"`
	EvaluationUniverse EvaluationUniverseConfig `mapstructure:"evaluation_universe"`
	CandleGapDetection CandleGapDetectionConfig `mapstructure:"candle_gap_detection"`
	SRAnalysis         SRAnalysisConfig         `mapstructure:"sr_analysis"`
	SRZoneVerify       SRZoneVerifyConfig       `mapstructure:"sr_zone_verify"`
	CorporateAction    CorporateActionConfig    `mapstructure:"corporate_action"`
	PositionAnalysis   PositionAnalysisConfig   `mapstructure:"position_analysis"`
}

type PositionAnalysisConfig struct {
	MaxPositionValue         float64 `mapstructure:"max_position_value"`
	MaxRiskAmount            float64 `mapstructure:"max_risk_amount"`
	AddOnRatio               float64 `mapstructure:"add_on_ratio"`
	MinRiskRewardRatio       float64 `mapstructure:"min_risk_reward_ratio"`
	BreakoutTargetRR         float64 `mapstructure:"breakout_target_risk_reward_ratio"`
	TakeProfitReductionRatio float64 `mapstructure:"take_profit_reduction_ratio"`
	SRReuseMaxAgeHours       int     `mapstructure:"sr_reuse_max_age_hours"`
}

type AuthConfig struct {
	JWTSecret string `mapstructure:"jwt_secret"`
}

type PythonConfig struct {
	// ServiceURL：Method B HTTP 端點；空白表示僅用 Method A（DB polling）
	ServiceURL        string `mapstructure:"service_url"`
	SRZonesTimeoutSec int    `mapstructure:"sr_zones_timeout_sec"`
}

type DatabaseConfig struct {
	Driver string `mapstructure:"driver"` // "sqlite" 或 "mysql"
	DSN    string `mapstructure:"dsn"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type FinMindConfig struct {
	APIKey    string `mapstructure:"api_key"`
	BaseURL   string `mapstructure:"base_url"`
	RateLimit int    `mapstructure:"rate_limit"`
	// IntradayEnabled 控制盤中分K排程（TaiwanStockKBar dataset）是否啟用；
	// 該 dataset 需要 FinMind Sponsor 級以上 token，帳號等級不足時預設關閉，
	// 避免排程每 5 分鐘都對 FinMind 發出注定失敗的請求、浪費 rate limit 額度。
	IntradayEnabled bool `mapstructure:"intraday_enabled"`
}

// FugleConfig 為富果 MarketData API 設定。免費公開方案額度（2026 查證）：
// WebSocket 1 連線／同時最多訂閱 5 檔，REST 即時行情與歷史行情各 60 次/分鐘。
type FugleConfig struct {
	APIKey           string `mapstructure:"api_key"`
	RESTBaseURL      string `mapstructure:"rest_base_url"`
	WSEndpoint       string `mapstructure:"ws_endpoint"`
	Enabled          bool   `mapstructure:"enabled"`
	QuoteRateLimit   int    `mapstructure:"quote_rate_limit"`  // 每分鐘 REST 呼叫上限，免費方案為 60
	MaxSubscriptions int    `mapstructure:"max_subscriptions"` // WebSocket 同時訂閱檔數上限，免費方案為 5
	ReconnectMaxSec  int    `mapstructure:"reconnect_max_sec"` // WebSocket 重連退避上限（秒）
	// MaxConnCooldownSec：撞到「Maximum number of connections reached」時的遞增冷卻
	// 起始值（秒）。名額被前一條 1006 幽靈連線佔著時，會從這個值開始、每次連續撞到
	// 就加倍（上限見 fugleMaxConnCooldownCap），確保安靜窗夠久讓伺服器釋放名額。0 沿用預設 60。
	MaxConnCooldownSec int `mapstructure:"max_conn_cooldown_sec"`
	// PingIntervalSec：WebSocket 認證成功後主動送 ping 的間隔（秒），用於保活、
	// 降低被 NAT／防火牆剪斷造成的 1006 異常斷線。未設定（0）沿用預設 30 並啟用；
	// 負值明確關閉主動 ping。
	PingIntervalSec int `mapstructure:"ping_interval_sec"`
}

// YahooConfig 為 Yahoo 股市盤中資料源設定（非官方 API，作為 Tier-1 批次盤中源，
// 與 Fugle 並存/擇一，見 docs/yahoo-intraday-integration.md）。
type YahooConfig struct {
	BaseURL   string `mapstructure:"base_url"`
	Enabled   bool   `mapstructure:"enabled"`
	RateLimit int    `mapstructure:"rate_limit"` // 每分鐘請求上限（批次請求計為一次），保守預設 20
	BatchSize int    `mapstructure:"batch_size"` // 單次請求最多帶入的 symbol 數，預設 40
}

type StockSymbolsConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// SyncURLs：TWSE ISIN 來源清單，需同時涵蓋上市（strMode=2）與上櫃（strMode=4），
	// 否則另一半市場會被誤判為不在名單內。
	SyncURLs []string `mapstructure:"sync_urls"`
	Cron     string   `mapstructure:"cron"`
	// TimeoutSec：單一 TWSE ISIN 請求（含讀 body）的上限秒數。
	// FetchDelaySec：來源之間的等待秒數，懷疑被限流時調大。
	// 兩者為 0 或未設定時，沿用 market 套件的 default 常數（300 秒 / 3 秒），
	// 讓預設值只有一個來源。
	TimeoutSec    int `mapstructure:"timeout_sec"`
	FetchDelaySec int `mapstructure:"fetch_delay_sec"`
}

// ChipConfig 為籌碼資料同步設定（三大法人、融資融券、券商分點、chip_scores）。
type ChipConfig struct {
	Sync ChipSyncConfig `mapstructure:"sync"`
}

type ChipSyncConfig struct {
	// HistoryTradingDays 為 backfill 預設回補天數（不指定 from 時反推起始日）。
	HistoryTradingDays int `mapstructure:"history_trading_days"`
	// BatchSize 為 DB 批次寫入大小，比照 candleBulkInsertBatchSize 的保守預設。
	BatchSize int `mapstructure:"batch_size"`
	// Cron 為籌碼每日採集的排程時間（robfig/cron 格式，台北時區）。籌碼採集與
	// 15:00 收盤掃描解耦：FinMind 的法人資料收盤後傍晚、融資融券更要晚間才由
	// TWSE 發布，故預設排在 21:00；需要自動重試時可設多時段（例如
	// "0 18,20,22 * * 1-5"），upsert 天生冪等，重跑安全。
	Cron string `mapstructure:"cron"`
}

type SREvaluationConfig struct {
	// Enabled 預設關閉，避免開發環境啟動後自動跑大量 Python decision replay。
	Enabled bool `mapstructure:"enabled"`
	// Cron 為 SR evaluation / decision replay 排程時間（台北時區）。
	Cron string `mapstructure:"cron"`
	// Symbols 空陣列代表使用 watchlist；非空時只跑指定股票。
	Symbols        []string `mapstructure:"symbols"`
	Timeframe      string   `mapstructure:"timeframe"`
	Limit          int      `mapstructure:"limit"`
	DecisionReplay bool     `mapstructure:"decision_replay"`
	ReplayMaxRows  int      `mapstructure:"replay_max_rows"`
	WriteDB        bool     `mapstructure:"write_db"`
}

// SRAnalysisConfig 是「定期對 watchlist 產生 SR zone 分析」的排程（todo.md T-052）。
//
// **兩個 cron 是刻意的，不是重複。** SR 分析吃籌碼（trading_score 的 Chip 佔 15%），
// 而 FinMind 的法人／融資券要晚間才發布（chip.sync.cron 預設 21:00）：
//
//   - Cron（預設 17:00）：當日 K 棒 ＋ **前一日**籌碼。收盤後盡快有一份可看的分析。
//   - ChipCron（預設 22:00）：當日 K 棒 ＋ **當日**籌碼。晚於籌碼採集、早於
//     sr_evaluation（22:30）。
//
// 兩輪各自有前置檢查，不符就跳過該檔而不是失敗；ChipCron 那輪額外要求籌碼
// trade_date 是今天，否則跑出來的東西與 17:00 那輪相同，白跑還多推一次
// observed_absences（見 todo.md T-052 的 S4）。
//
// **預設關閉**，比照 sr_evaluation / evaluation_universe：啟用是一個明確的部署動作。
type SRAnalysisConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// Cron 是不含當日籌碼的那一輪（台北時區）。
	Cron string `mapstructure:"cron"`
	// ChipCron 是含當日籌碼的那一輪（台北時區）。
	ChipCron string `mapstructure:"chip_cron"`
	// Timeframe / Limit 與使用者手動觸發分析時的參數同義。
	Timeframe string `mapstructure:"timeframe"`
	Limit     int    `mapstructure:"limit"`
}

// SRZoneVerifyConfig 是收盤後 SR zone 驗證排程的覆蓋窗口
// （現況規格見 docs/architecture.md 的排程說明段）。
//
// **窗口的單位是「天」而不是「筆數」**：原本寫死「最近 50 筆分析」，但那個數字是
// 分析還沒排程化的年代訂的（一天 1~3 筆，涵蓋約 20 個交易日）。watchlist 11 檔
// × 每日兩輪之後一天就 22 筆，50 筆只剩約 2.3 個交易日，watchlist 再擴大就不到一天，
// 更早的分析裡那些 PENDING 的 zone 會永遠停在 PENDING。改用天數之後，
// 覆蓋窗口與 watchlist 大小、與每日輪數都脫鉤。
type SRZoneVerifyConfig struct {
	// Days 是往回驗幾天的分析（依 created_at）。預設 30。
	//
	// 成本不是限制：**2026-08-25 在 dev postgres 實測 672 筆分析（10256 個 zone）
	// 整輪 20.0 秒**，平均每筆 29.8ms。驗證是 I/O 為主的本地 DB 往返，不是 CPU 密集
	// （單筆成本 = 5 次查詢 ＋ 每個 zone 一次 UpdateZoneStatus）。
	// 立案時的「45 筆／1 秒」推估 660 筆約 15 秒，實測偏高約 33%，但結論不變。
	Days int `mapstructure:"days"`
	// MaxAnalyses 是單輪處理筆數的硬上限，防止窗口拉長後無限成長。預設 2000。
	//
	// 比照 timeline 的 maxTimelineMaxAnalyses 慣例，scheduler 端是**兩段式**：
	// 非正值退回預設，超過 maxSRZoneVerifyMaxAnalyses（10000）則截到上限。
	// 上限在記憶體上站得住的前提是**清單只取 id/symbol**（store 的
	// ListRefsSince）。改回撈整份分析的話 10000 筆約 276 MB，會直接 OOM。
	// 也就是說這個值調得再大也不會超過那個上限——排程沒有 timeout，
	// 不能讓 env 的一個錯字決定單輪要跑多久。
	MaxAnalyses int `mapstructure:"max_analyses"`
}

// EvaluationUniverseConfig 是評估標的池的每日日 K 維護排程（T-040 Step 5）。
//
// **預設關閉**：這個 job 一次會對整個池（實測 131 檔）各發一個 FinMind 請求，
// 在 5 req/min 的節流下約 26 分鐘。不該讓它預設開著，比照 sr_evaluation 的處理。
type EvaluationUniverseConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// Cron 為每日維護時間（台北時區）。預設 16:00：晚於 daily_close（15:00，已確認
	// FinMind 當日日 K 已發布——14:00 曾抓到 count=0），且與 21:00 的籌碼採集有近 5 小時
	// 緩衝，26 分鐘的執行窗不會重疊。
	Cron string `mapstructure:"cron"`
	// Days 為每次回補往前幾個**日曆天**。10 而非 5 是為了容忍連假與國定假日；
	// 成本與天數無關（FetchDailyCandles 把日期區間塞在同一個請求裡，1 request/檔）。
	Days int `mapstructure:"days"`
}

// CandleGapDetectionConfig 是日 K 缺漏偵測的設定。
// 現況說明見 `docs/architecture.md`「日 K 缺漏偵測」（原記於 issue.md I-091，已收斂）。
//
// **這些是待調的初值，不是實測最佳值**——除了 MarketStaleDays 有一次 14:13 的觀測支撐，
// 其餘都是保守猜測。
//
// ⚠️ **合法範圍的檢查不在這一層。** `config.Load()` 在 `viper.Unmarshal` 失敗時直接回
// error、`main.go` 收到就結束行程，而 `internal/config` 沒有 logger（全套件零筆 zap），
// 記不了「已退回預設」這種需要被看見的訊息。所以：
//
//	型別解析失敗（ENABLED=abc）      → 維持既有行為：**啟動失敗**
//	解析成功但超出範圍（cap=0）      → 由 scheduler 的正規化函式退回預設／截到界限 ＋ Error log
//
// 正規化見 `scheduler.normalizeCandleGapDetectionConfig`。**正規化之後的值才可以被信任**，
// 下游不再各自防禦。
type CandleGapDetectionConfig struct {
	// Enabled 預設 false。新機制一律預設關閉，比照 evaluation_universe / sr_analysis。
	Enabled bool `mapstructure:"enabled"`
	// AggregateRatio 是單一 (market, date) 分組的缺漏比例達到（**>=**，不是 >）此值時
	// 短路成來源級告警，不展開逐檔核對。合法範圍 (0, 1]：0 會讓任何缺口都短路、
	// >1 則永不短路。
	AggregateRatio float64 `mapstructure:"aggregate_ratio"`
	// AggregateMinSymbols 是套用比例所需的**最小母體**。該市場當時有效池不足此數時
	// 強制走逐檔核對——否則單檔市場的合法停止買賣會被短路成來源級告警。合法範圍 >= 1。
	AggregateMinSymbols int `mapstructure:"aggregate_min_symbols"`
	// CandidateCapPerRun 的單位是**候選標的數，不是 HTTP 請求數**。
	// 視窗跨月時同一個候選要兩次請求，所以
	// 請求數上限 = CandidateCapPerRun × 該輪視窗涵蓋的月份數（**不是固定值**，
	// LookbackTradingDays 可設到 60，跨越月份數會跟著變）。
	//
	// 用候選數當單位是為了讓公平上界 ceil(N/cap) 成立——改用請求數的話每輪處理的候選數
	// 會浮動，那個上界就不再可論證。合法範圍 1~100：不得為 0（一個都不驗卻回報成功），
	// 也不得無上限（誤設大值會讓單輪請求量暴增，與「避免對交易所造成壓力」直接衝突）。
	CandidateCapPerRun int `mapstructure:"candidate_cap_per_run"`
	// TimeoutSec 是整輪偵測的上限，hard cap 900。
	TimeoutSec int `mapstructure:"timeout_sec"`
	// LookbackTradingDays 是往回檢查幾個**交易日**（不是日曆天）。
	//
	// **刻意與 evaluation_universe.days 解耦**：那個值控制「回補要抓多長」，
	// 耦合的話調整回補成本會意外改變偵測範圍。合法範圍 1~60：0 會產生空視窗
	// （沒有預期日期＝沒有缺口＝永遠正常），那是「看起來成功」的靜默失效。
	LookbackTradingDays int `mapstructure:"lookback_trading_days"`
	// RequestIntervalMs 是對交易所端點的節流間隔（預設 2 req/s），與 FinMind 的
	// 5 req/min 無關。合法範圍 >= 100：**0 不是合法值**，那等於完全取消節流。
	// 要壓測請改程式，不要靠設定關掉安全限制。
	RequestIntervalMs int `mapstructure:"request_interval_ms"`
	// MarketStaleDays 的單位是**預期交易日，不是日曆日**——跨週末時日曆日差 3 是誤導。
	// 市場層級端點實測當日 14:13 尚未更新，所以容忍一個交易日的發布延遲，不容忍數日。
	MarketStaleDays int `mapstructure:"market_stale_days"`
	// CalendarTTLHours：歷史年度的日曆不會變；當年度容忍年中補班補假修訂。
	CalendarTTLHours int `mapstructure:"calendar_ttl_hours"`
	// BreakerFailures 是**來源層級**的連續失敗門檻（不是逐 symbol，兩者是不同的計數）。
	// 合法範圍 >= 1：0 會讓 breaker 永遠開著。
	BreakerFailures int `mapstructure:"breaker_failures"`
	// BreakerCooldownMin 後**自動**恢復，恢復條件是時間到而不是人工介入。
	BreakerCooldownMin int `mapstructure:"breaker_cooldown_min"`
}

// CorporateActionConfig 是公司行動（分割／除權息／減資）同步排程的設定。
//
// **沒有 Enabled 開關**：是否註冊由「有沒有注入 adjuster」決定（`SetAdjuster`），
// 與 stock symbol／sr evaluation 那種 config 開關不同。多一個 enabled 會出現
// 「adjuster 有注入但 enabled=false」這種要另外解釋的組合，而目前沒有關掉它的需求——
// 漏跑一次就會讓該檔整段歷史出現假跳空。
type CorporateActionConfig struct {
	// Cron 為同步時間（robfig/cron 格式，台北時區）。預設 06:30 平日：
	// 早於 08:50 的 pre_market，讓當天開盤前的分析已經吃到最新係數。
	//
	// **多時段對覆蓋率沒有幫助**：逐檔清單依日期分片，同一天的第二輪跑的是
	// 同一片。要臨時全量補跑請把 ShardCount 設成 1。
	Cron string `mapstructure:"cron"`

	// TimeoutSec 是整輪同步的預算（秒），預設 2700（45 分鐘）。
	//
	// 預算怎麼來的：逐檔事件同步的節奏由 FinMind 的 5 req/min 決定（每檔約 12 秒），
	// 當日名單 ≈ watchlist 11 檔 ＋ 每片約 170 檔 = 181 檔 × 12 秒 ≈ 36 分鐘，
	// 45 分鐘留了餘裕，且 06:30 起跑會在 08:50 的 pre_market 之前結束。
	// 設 0 或負值時退回預設。
	TimeoutSec int `mapstructure:"timeout_sec"`

	// ShardCount 是非 watchlist 標的的分片數，預設 5（週一到週五各一片，每檔每週覆蓋一次）。
	//
	// **設 5 的倍數或 1**：片號由「週序號×5 ＋ 平日序號」推導，非 5 的倍數會讓覆蓋週期
	// 落在非整數週、不好推理。10 = 兩週一輪（單輪時間砍半，適合預算吃緊時）；
	// 1 = 每天全量（除權息旺季臨時補跑）。設 0 或負值時退回預設。
	ShardCount int `mapstructure:"shard_count"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	viper.SetDefault("server.port", "8080")
	viper.SetDefault("database.driver", "sqlite")
	viper.SetDefault("database.dsn", "./trading.db")
	viper.SetDefault("finmind.base_url", "https://api.finmindtrade.com/api/v4")
	viper.SetDefault("finmind.rate_limit", 5)
	viper.SetDefault("finmind.intraday_enabled", false)
	viper.SetDefault("fugle.rest_base_url", "https://api.fugle.tw/marketdata/v1.0/stock")
	viper.SetDefault("fugle.ws_endpoint", "wss://api.fugle.tw/marketdata/v1.0/stock/streaming")
	viper.SetDefault("fugle.enabled", false)
	viper.SetDefault("fugle.quote_rate_limit", 60)
	viper.SetDefault("fugle.max_subscriptions", 5)
	viper.SetDefault("fugle.reconnect_max_sec", 60)
	viper.SetDefault("fugle.max_conn_cooldown_sec", 60)
	viper.SetDefault("fugle.ping_interval_sec", 30)
	viper.SetDefault("yahoo.base_url", "https://tw.stock.yahoo.com/_td-stock/api/resource/FinanceChartService.ApacLibraCharts")
	viper.SetDefault("yahoo.enabled", false)
	viper.SetDefault("yahoo.rate_limit", 20)
	viper.SetDefault("yahoo.batch_size", 40)
	viper.SetDefault("stock_symbols.enabled", true)
	viper.SetDefault("stock_symbols.sync_urls", []string{
		"https://isin.twse.com.tw/isin/C_public.jsp?strMode=2", // 上市
		"https://isin.twse.com.tw/isin/C_public.jsp?strMode=4", // 上櫃
	})
	viper.SetDefault("stock_symbols.cron", "30 6 * * *")
	// timeout_sec / fetch_delay_sec 不在這裡給數字：預設值由 market 套件的常數決定
	// （見 market.defaultTWSEISINTimeout / defaultFetchDelayBetweenSources），
	// 這裡給 0 代表「沿用程式預設」。
	viper.SetDefault("stock_symbols.timeout_sec", 0)
	viper.SetDefault("stock_symbols.fetch_delay_sec", 0)
	viper.SetDefault("auth.jwt_secret", "change-me-in-production")
	viper.SetDefault("python.sr_zones_timeout_sec", 120)
	viper.SetDefault("chip.sync.history_trading_days", 500)
	viper.SetDefault("chip.sync.batch_size", 50)
	viper.SetDefault("chip.sync.cron", "0 21 * * 1-5")
	viper.SetDefault("sr_evaluation.enabled", false)
	viper.SetDefault("sr_evaluation.cron", "30 22 * * 1-5")
	viper.SetDefault("sr_evaluation.symbols", []string{})
	viper.SetDefault("sr_evaluation.timeframe", "1d")
	viper.SetDefault("sr_evaluation.limit", 1500)
	viper.SetDefault("sr_evaluation.decision_replay", true)
	viper.SetDefault("sr_evaluation.replay_max_rows", 200)
	viper.SetDefault("sr_evaluation.write_db", true)

	// 與搬進 config 之前的硬編碼值相同，行為不變（T-042）。
	viper.SetDefault("sr_zone_verify.days", 30)
	viper.SetDefault("sr_zone_verify.max_analyses", 2000)
	viper.SetDefault("corporate_action.cron", "30 6 * * 1-5")
	// 2026-08-24：預算由 10 分鐘改為 45 分鐘，並把非 watchlist 標的切成 5 片（週一到週五各一片）。
	viper.SetDefault("corporate_action.timeout_sec", 2700)
	viper.SetDefault("corporate_action.shard_count", 5)
	viper.SetDefault("evaluation_universe.enabled", false)
	viper.SetDefault("evaluation_universe.cron", "0 16 * * 1-5")
	viper.SetDefault("evaluation_universe.days", 10)
	// 缺漏偵測沒有自己的 cron，跟著 evaluation_universe 那輪跑。**預設關閉。**
	// 這裡只負責「有沒有設」，合法範圍由 scheduler 的正規化函式把關（config 層沒有 logger）。
	viper.SetDefault("candle_gap_detection.enabled", false)
	viper.SetDefault("candle_gap_detection.aggregate_ratio", 0.5)
	viper.SetDefault("candle_gap_detection.aggregate_min_symbols", 5)
	viper.SetDefault("candle_gap_detection.candidate_cap_per_run", 20)
	viper.SetDefault("candle_gap_detection.timeout_sec", 300)
	viper.SetDefault("candle_gap_detection.lookback_trading_days", 10)
	viper.SetDefault("candle_gap_detection.request_interval_ms", 500)
	viper.SetDefault("candle_gap_detection.market_stale_days", 2)
	viper.SetDefault("candle_gap_detection.calendar_ttl_hours", 24)
	viper.SetDefault("candle_gap_detection.breaker_failures", 5)
	viper.SetDefault("candle_gap_detection.breaker_cooldown_min", 60)
	// T-052：兩輪都預設關閉。17:00 那輪拿不到當日籌碼，22:00 那輪晚於 chip sync（21:00）
	// 且早於 sr_evaluation（22:30）。
	viper.SetDefault("sr_analysis.enabled", false)
	viper.SetDefault("sr_analysis.cron", "0 17 * * 1-5")
	viper.SetDefault("sr_analysis.chip_cron", "0 22 * * 1-5")
	viper.SetDefault("sr_analysis.timeframe", "1d")
	viper.SetDefault("sr_analysis.limit", 400)
	viper.SetDefault("position_analysis.max_position_value", 200000)
	viper.SetDefault("position_analysis.max_risk_amount", 10000)
	viper.SetDefault("position_analysis.add_on_ratio", 0.25)
	viper.SetDefault("position_analysis.min_risk_reward_ratio", 1.5)
	viper.SetDefault("position_analysis.breakout_target_risk_reward_ratio", 2.0)
	viper.SetDefault("position_analysis.take_profit_reduction_ratio", 0.5)
	viper.SetDefault("position_analysis.sr_reuse_max_age_hours", 24)

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
