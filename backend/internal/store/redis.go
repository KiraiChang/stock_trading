package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/trading/backend/internal/config"
)

type RedisClient struct {
	rdb *redis.Client // nil 表示 Redis 停用（開發模式）
}

// DisabledRedisConfig 回傳空設定，用於明確停用 Redis
func DisabledRedisConfig() config.RedisConfig {
	return config.RedisConfig{}
}

// NewRedis 建立 Redis 連線；addr 為空時回傳 no-op client（不連線）
func NewRedis(cfg config.RedisConfig) *RedisClient {
	if cfg.Addr == "" {
		return &RedisClient{rdb: nil}
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return &RedisClient{rdb: rdb}
}

func (r *RedisClient) Enabled() bool {
	return r.rdb != nil
}

func (r *RedisClient) Ping(ctx context.Context) error {
	if r.rdb == nil {
		return nil
	}
	return r.rdb.Ping(ctx).Err()
}

func (r *RedisClient) HSet(ctx context.Context, key string, values map[string]interface{}) error {
	if r.rdb == nil {
		return nil
	}
	return r.rdb.HSet(ctx, key, values).Err()
}

func (r *RedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	if r.rdb == nil {
		return nil, nil
	}
	return r.rdb.HGetAll(ctx, key).Result()
}

func (r *RedisClient) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if r.rdb == nil {
		return nil
	}
	return r.rdb.Expire(ctx, key, ttl).Err()
}

func (r *RedisClient) LPush(ctx context.Context, key string, value interface{}) error {
	if r.rdb == nil {
		return nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.rdb.LPush(ctx, key, b).Err()
}

func (r *RedisClient) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	if r.rdb == nil {
		return nil, nil
	}
	return r.rdb.LRange(ctx, key, start, stop).Result()
}

func (r *RedisClient) SAdd(ctx context.Context, key string, members ...interface{}) error {
	if r.rdb == nil {
		return nil
	}
	return r.rdb.SAdd(ctx, key, members...).Err()
}

func (r *RedisClient) SMembers(ctx context.Context, key string) ([]string, error) {
	if r.rdb == nil {
		return nil, nil
	}
	return r.rdb.SMembers(ctx, key).Result()
}

func (r *RedisClient) SRem(ctx context.Context, key string, members ...interface{}) error {
	if r.rdb == nil {
		return nil
	}
	return r.rdb.SRem(ctx, key, members...).Err()
}

func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if r.rdb == nil {
		return nil
	}
	return r.rdb.Set(ctx, key, value, ttl).Err()
}

func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	if r.rdb == nil {
		return "", nil
	}
	return r.rdb.Get(ctx, key).Result()
}
