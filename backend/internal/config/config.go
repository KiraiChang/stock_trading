package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	FinMind  FinMindConfig
	Python   PythonConfig
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
