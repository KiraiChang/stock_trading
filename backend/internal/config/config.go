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

	viper.SetDefault("evaluation_universe.enabled", false)
	viper.SetDefault("evaluation_universe.cron", "0 16 * * 1-5")
	viper.SetDefault("evaluation_universe.days", 10)
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
