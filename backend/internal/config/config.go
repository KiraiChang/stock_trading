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
	viper.SetDefault("fugle.rest_base_url", "https://api.fugle.tw/marketdata/v1.0/stock")
	viper.SetDefault("fugle.ws_endpoint", "wss://api.fugle.tw/marketdata/v1.0/stock/streaming")
	viper.SetDefault("fugle.enabled", false)
	viper.SetDefault("fugle.quote_rate_limit", 60)
	viper.SetDefault("fugle.max_subscriptions", 5)
	viper.SetDefault("fugle.reconnect_max_sec", 60)
	viper.SetDefault("auth.jwt_secret", "change-me-in-production")

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
