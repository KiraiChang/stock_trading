package store

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/trading/backend/internal/config"
)

// 啟動時的 Redis 探測（I-132 的 C；設計見 docs/plans/i132-redis-silent-disable.md 4.4、D3、D4）。
//
// 有設定 REDIS_ADDR 時，Redis 是必要的：連不上就重試，仍然失敗由 main.go 在 HTTP 與排程器
// 啟動之前以非 0 結束，⛔ 不再靜默停用。
//
// 探測用**啟動專用**的 client（probeOptions），每次嘗試新建、用完立刻 Close；
// runtime 的 client 照舊由 NewRedis 建立，選項與重連行為⛔ 不變。
const (
	// RedisProbeAttempts 是總嘗試次數（含第一次）。
	RedisProbeAttempts = 5
	// RedisProbeInterval 是失敗後的等待；最後一次失敗之後⛔ 不再等。
	RedisProbeInterval = 2 * time.Second
	// RedisProbeTimeout 是單次探測的上限。理論總上限 5 × 3 + 4 × 2 = 23 秒。
	RedisProbeTimeout = 3 * time.Second
)

// 原因類別（封閉值域）。分類只看錯誤型別與 Redis 錯誤碼的前綴，⛔ 不輸出錯誤原文。
const (
	RedisCauseNoAuth    = "noauth"
	RedisCauseWrongPass = "wrongpass"
	RedisCauseTimeout   = "timeout"
	RedisCauseRefused   = "refused"
	RedisCauseDNS       = "dns"
	RedisCauseCanceled  = "canceled" // 父 context 被取消（例如啟動途中收到停止訊號）
	RedisCauseOther     = "other"
)

// RedisProbeError 是探測失敗的結果：最後一次嘗試的原因類別與嘗試次數。⛔ 不帶錯誤原文。
type RedisProbeError struct {
	Cause    string
	Attempts int
}

func (e *RedisProbeError) Error() string {
	return fmt.Sprintf("redis probe failed: cause=%s attempts=%d", e.Cause, e.Attempts)
}

// sleepFunc 等待 d；ctx 被取消時提早回傳 ctx 的錯誤。測試時注入，不必真的等。
type sleepFunc func(ctx context.Context, d time.Duration) error

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// probeRedis 是可測試的核心：最多嘗試 attempts 次，每次用 timeout 包住 ping，失敗之後等 interval
// （最後一次失敗之後⛔ 不等）。成功回傳嘗試次數；失敗回傳 *RedisProbeError。
// onFail 在每次嘗試失敗時呼叫（可為 nil），只拿得到次數與原因類別。
func probeRedis(ctx context.Context, ping func(context.Context) error, attempts int,
	interval, timeout time.Duration, sleep sleepFunc, onFail func(attempt int, cause string)) (int, error) {
	for i := 1; i <= attempts; i++ {
		if ctx.Err() != nil {
			return i - 1, &RedisProbeError{Cause: RedisCauseCanceled, Attempts: i - 1}
		}
		actx, cancel := context.WithTimeout(ctx, timeout)
		err := ping(actx)
		cancel()
		if err == nil {
			return i, nil
		}
		cause := classifyRedisErr(err)
		if ctx.Err() != nil {
			cause = RedisCauseCanceled
		}
		if onFail != nil {
			onFail(i, cause)
		}
		if cause == RedisCauseCanceled || i == attempts {
			return i, &RedisProbeError{Cause: cause, Attempts: i}
		}
		if err := sleep(ctx, interval); err != nil {
			return i, &RedisProbeError{Cause: RedisCauseCanceled, Attempts: i}
		}
	}
	return 0, &RedisProbeError{Cause: RedisCauseOther, Attempts: 0} // attempts <= 0
}

// classifyRedisErr 把錯誤歸到封閉值域的原因類別。DNS 先判（DNSError 也可能是逾時）。
func classifyRedisErr(err error) string {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return RedisCauseDNS
	}
	var rerr redis.Error
	if errors.As(err, &rerr) {
		// HasErrorPrefix 會先去掉開頭的 "ERR "
		switch {
		case redis.HasErrorPrefix(err, "NOAUTH"):
			return RedisCauseNoAuth
		case redis.HasErrorPrefix(err, "WRONGPASS"),
			redis.HasErrorPrefix(err, "invalid password"), // Redis 6 以前的密碼錯誤
			redis.HasErrorPrefix(err, "AUTH"):             // 伺服器沒設密碼、client 卻帶了密碼
			return RedisCauseWrongPass
		}
		return RedisCauseOther
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return RedisCauseRefused
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return RedisCauseTimeout
	}
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		return RedisCauseTimeout
	}
	return RedisCauseOther
}

// probeOptions 是啟動專用 client 的選項：關掉 client 內建的重試，讓單次探測的上限真的是
// RedisProbeTimeout（go-redis v9.5.4 預設 MaxRetries 3、DialTimeout 5 秒，而且
// ContextTimeoutEnabled 為 false 時 socket 讀寫⛔ 不看 context 的 deadline）。
func probeOptions(cfg config.RedisConfig) *redis.Options {
	return &redis.Options{
		Addr:                  cfg.Addr,
		Password:              cfg.Password,
		DB:                    cfg.DB,
		MaxRetries:            -1,
		DialTimeout:           RedisProbeTimeout,
		ReadTimeout:           RedisProbeTimeout,
		WriteTimeout:          RedisProbeTimeout,
		PoolSize:              1,
		PoolTimeout:           RedisProbeTimeout,
		ContextTimeoutEnabled: true,
	}
}

// ProbeRedis 用啟動專用的 client 探測 Redis（每次嘗試新建、用完立刻 Close）。
// 成功回傳嘗試次數；失敗回傳 *RedisProbeError。cfg.Addr 為空時⛔ 不要呼叫（由 main.go 判斷）。
func ProbeRedis(ctx context.Context, cfg config.RedisConfig, onFail func(attempt int, cause string)) (int, error) {
	return probeRedisClient(ctx, cfg, RedisProbeAttempts, RedisProbeInterval, RedisProbeTimeout, sleepCtx, onFail)
}

// probeRedisClient 是 ProbeRedis 的本體，次數與時間可以注入（測試用假 Redis 伺服器時不必等 23 秒）。
func probeRedisClient(ctx context.Context, cfg config.RedisConfig, attempts int, interval, timeout time.Duration,
	sleep sleepFunc, onFail func(attempt int, cause string)) (int, error) {
	ping := func(ctx context.Context) error {
		// ⛔ 每次都要新的 Options：NewClient 會就地改寫它（MaxRetries -1 → 0），
		// 重用的話第二次 init 會把 0 當成「沒設定」而變回預設的 3 次重試。
		c := redis.NewClient(probeOptions(cfg))
		defer c.Close()
		return c.Ping(ctx).Err()
	}
	return probeRedis(ctx, ping, attempts, interval, timeout, sleep, onFail)
}
