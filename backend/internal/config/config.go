package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	FinMind  FinMindConfig
	Fugle    FugleConfig
	Python   PythonConfig
	Auth     AuthConfig
	Chip     ChipConfig
}

type AuthConfig struct {
	JWTSecret string `mapstructure:"jwt_secret"`
}

type PythonConfig struct {
	// ServiceURL：Method B HTTP 端點；空白表示僅用 Method A（DB polling）
	ServiceURL string `mapstructure:"service_url"`
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
	viper.SetDefault("auth.jwt_secret", "change-me-in-production")
	viper.SetDefault("chip.sync.history_trading_days", 500)
	viper.SetDefault("chip.sync.batch_size", 50)

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
