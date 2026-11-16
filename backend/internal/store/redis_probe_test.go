package store

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/trading/backend/internal/config"
)

// ── probeRedis：次數、等待、停止條件 ──

type sleepRecorder struct{ waits []time.Duration }

func (s *sleepRecorder) sleep(ctx context.Context, d time.Duration) error {
	s.waits = append(s.waits, d)
	return ctx.Err()
}

func failN(n int, err error) (func(context.Context) error, *int) {
	calls := 0
	return func(context.Context) error {
		calls++
		if calls <= n {
			return err
		}
		return nil
	}, &calls
}

var errRefused = &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}

func TestProbeRedis_FirstAttemptSucceeds(t *testing.T) {
	ping, calls := failN(0, nil)
	sr := &sleepRecorder{}
	n, err := probeRedis(context.Background(), ping, 5, 2*time.Second, 3*time.Second, sr.sleep, nil)
	if err != nil || n != 1 || *calls != 1 || len(sr.waits) != 0 {
		t.Fatalf("第一次就成功：n=%d err=%v calls=%d waits=%v", n, err, *calls, sr.waits)
	}
}

func TestProbeRedis_SucceedsOnKthAttempt(t *testing.T) {
	ping, calls := failN(2, errRefused)
	sr := &sleepRecorder{}
	var fails []string
	n, err := probeRedis(context.Background(), ping, 5, 2*time.Second, 3*time.Second, sr.sleep,
		func(a int, c string) { fails = append(fails, fmt.Sprintf("%d:%s", a, c)) })
	if err != nil || n != 3 || *calls != 3 || len(sr.waits) != 2 {
		t.Fatalf("第 3 次才成功：n=%d err=%v calls=%d waits=%v", n, err, *calls, sr.waits)
	}
	if strings.Join(fails, ",") != "1:refused,2:refused" {
		t.Fatalf("每次失敗都要回報次數與原因類別，得到 %v", fails)
	}
}

func TestProbeRedis_AllFail(t *testing.T) {
	ping, calls := failN(99, errRefused)
	sr := &sleepRecorder{}
	n, err := probeRedis(context.Background(), ping, 5, 2*time.Second, 3*time.Second, sr.sleep, nil)
	var pe *RedisProbeError
	if !errors.As(err, &pe) || pe.Cause != RedisCauseRefused || pe.Attempts != 5 || n != 5 {
		t.Fatalf("全部失敗要回傳 RedisProbeError{refused, 5}，得到 n=%d err=%v", n, err)
	}
	if *calls != 5 {
		t.Fatalf("總共要嘗試 5 次，得到 %d", *calls)
	}
	// 等了 4 次、每次 2 秒；最後一次失敗之後⛔ 不再等
	if len(sr.waits) != 4 {
		t.Fatalf("要等 4 次（最後一次失敗之後不等），得到 %v", sr.waits)
	}
	for _, w := range sr.waits {
		if w != 2*time.Second {
			t.Fatalf("間隔要是 2 秒，得到 %v", sr.waits)
		}
	}
}

func TestProbeRedis_EachAttemptHasTimeout(t *testing.T) {
	var deadlines []time.Duration
	ping := func(ctx context.Context) error {
		d, ok := ctx.Deadline()
		if !ok {
			t.Fatal("每次嘗試都要有 deadline")
		}
		deadlines = append(deadlines, time.Until(d))
		return context.DeadlineExceeded
	}
	sr := &sleepRecorder{}
	_, err := probeRedis(context.Background(), ping, 3, time.Second, 3*time.Second, sr.sleep, nil)
	var pe *RedisProbeError
	if !errors.As(err, &pe) || pe.Cause != RedisCauseTimeout {
		t.Fatalf("要回報 timeout，得到 %v", err)
	}
	for _, d := range deadlines {
		if d <= 0 || d > 3*time.Second {
			t.Fatalf("單次探測的 deadline 要在 3 秒以內，得到 %v", deadlines)
		}
	}
}

func TestProbeRedis_ParentCanceledStopsImmediately(t *testing.T) {
	// ①嘗試途中被取消：⛔ 不再等、⛔ 不再試
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	ping := func(context.Context) error {
		calls++
		if calls == 2 {
			cancel()
		}
		return errRefused
	}
	sr := &sleepRecorder{}
	_, err := probeRedis(ctx, ping, 5, 2*time.Second, 3*time.Second, sr.sleep, nil)
	var pe *RedisProbeError
	if !errors.As(err, &pe) || pe.Cause != RedisCauseCanceled || pe.Attempts != 2 || calls != 2 || len(sr.waits) != 1 {
		t.Fatalf("取消後要立刻停止：err=%v calls=%d waits=%v", err, calls, sr.waits)
	}

	// ②等待途中被取消
	ctx2, cancel2 := context.WithCancel(context.Background())
	calls = 0
	sleepCancel := func(context.Context, time.Duration) error { cancel2(); return context.Canceled }
	_, err = probeRedis(ctx2, func(context.Context) error { calls++; return errRefused }, 5, 2*time.Second, 3*time.Second, sleepCancel, nil)
	if !errors.As(err, &pe) || pe.Cause != RedisCauseCanceled || calls != 1 {
		t.Fatalf("等待途中取消要立刻停止：err=%v calls=%d", err, calls)
	}

	// ③一開始就已經取消：⛔ 一次都不試
	ctx3, cancel3 := context.WithCancel(context.Background())
	cancel3()
	calls = 0
	_, err = probeRedis(ctx3, func(context.Context) error { calls++; return nil }, 5, 2*time.Second, 3*time.Second, sr.sleep, nil)
	if !errors.As(err, &pe) || pe.Cause != RedisCauseCanceled || calls != 0 {
		t.Fatalf("已取消的 context ⛔ 不能嘗試：err=%v calls=%d", err, calls)
	}
}

func TestSleepCtx(t *testing.T) {
	if err := sleepCtx(context.Background(), time.Millisecond); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if err := sleepCtx(ctx, time.Hour); !errors.Is(err, context.Canceled) || time.Since(start) > time.Second {
		t.Fatalf("取消時要立刻回傳：%v", err)
	}
}

// ── 原因類別 ──

type fakeRedisErr string

func (e fakeRedisErr) Error() string { return string(e) }
func (fakeRedisErr) RedisError()     {}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestClassifyRedisErr(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{fakeRedisErr("NOAUTH Authentication required."), RedisCauseNoAuth},
		{fakeRedisErr("WRONGPASS invalid username-password pair or user is disabled."), RedisCauseWrongPass},
		{fakeRedisErr("ERR invalid password"), RedisCauseWrongPass},
		{fakeRedisErr("ERR AUTH <password> called without any password configured for the default user."), RedisCauseWrongPass},
		{fakeRedisErr("LOADING Redis is loading the dataset in memory"), RedisCauseOther},
		{fmt.Errorf("wrapped: %w", fakeRedisErr("NOAUTH x")), RedisCauseNoAuth},
		{errRefused, RedisCauseRefused},
		{&net.OpError{Op: "dial", Err: &net.DNSError{Err: "no such host", Name: "redis", IsNotFound: true}}, RedisCauseDNS},
		{&net.OpError{Op: "dial", Err: &net.DNSError{Err: "timeout", Name: "redis", IsTimeout: true}}, RedisCauseDNS},
		{context.DeadlineExceeded, RedisCauseTimeout},
		{&net.OpError{Op: "read", Err: timeoutErr{}}, RedisCauseTimeout},
		{io.EOF, RedisCauseOther},
		{errors.New("something else"), RedisCauseOther},
	}
	for _, c := range cases {
		if got := classifyRedisErr(c.err); got != c.want {
			t.Errorf("classifyRedisErr(%v) = %s，要 %s", c.err, got, c.want)
		}
	}
}

// ── 啟動專用 client 與 runtime client 的選項 ──

func TestProbeOptions(t *testing.T) {
	cfg := config.RedisConfig{Addr: "redis:6379", Password: "p", DB: 2}
	o := probeOptions(cfg)
	if o.MaxRetries != -1 || o.DialTimeout != RedisProbeTimeout || o.ReadTimeout != RedisProbeTimeout ||
		o.WriteTimeout != RedisProbeTimeout || o.PoolSize != 1 || o.PoolTimeout != RedisProbeTimeout ||
		!o.ContextTimeoutEnabled || o.Addr != cfg.Addr || o.Password != cfg.Password || o.DB != cfg.DB {
		t.Fatalf("探測的選項不對：%+v", o)
	}
	if probeOptions(cfg) == o {
		t.Fatal("每次都要回傳新的 Options（NewClient 會就地改寫它）")
	}
}

// runtime 的 contract：NewRedis 的選項⛔ 不能被 C 改到（照舊是 go-redis 的預設值）。
func TestNewRedis_RuntimeOptionsUnchanged(t *testing.T) {
	r := NewRedis(config.RedisConfig{Addr: "redis:6379", Password: "p", DB: 1})
	defer r.rdb.Close()
	o := r.rdb.Options()
	if o.MaxRetries != 3 || o.DialTimeout != 5*time.Second || o.ReadTimeout != 3*time.Second ||
		o.WriteTimeout != 3*time.Second || o.PoolTimeout != 4*time.Second ||
		o.PoolSize != 10*runtime.GOMAXPROCS(0) || o.ContextTimeoutEnabled {
		t.Fatalf("runtime client 的選項變了：%+v", o)
	}
	if NewRedis(config.RedisConfig{}).Enabled() {
		t.Fatal("沒有位址時要停用")
	}
}

// ── 用假的 Redis 伺服器跑真的 go-redis client ──

// fakeRedis 是極簡的 RESP 伺服器：每條連線用 newHandler 建一個自己的 handler（AUTH 狀態是連線層級的），
// 每收到一個指令就交給它回應（回傳完整的 RESP 字串；空字串表示直接關閉連線、"hang" 表示不回應）。
// 回傳位址與累計的連線數。
func fakeRedis(t *testing.T, newHandler func() func(cmd []string) string) (string, *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var conns atomic.Int32
	var wg sync.WaitGroup
	done := make(chan struct{})
	t.Cleanup(func() { close(done); ln.Close(); wg.Wait() })
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			conns.Add(1)
			wg.Add(1)
			go func(c net.Conn) {
				defer wg.Done()
				defer c.Close()
				go func() { <-done; c.Close() }()
				handler := newHandler()
				r := bufio.NewReader(c)
				for {
					cmd, err := readRESPArray(r)
					if err != nil {
						return
					}
					resp := handler(cmd)
					switch resp {
					case "":
						return
					case "hang":
						<-done
						return
					}
					if _, err := io.WriteString(c, resp); err != nil {
						return
					}
				}
			}(c)
		}
	}()
	return ln.Addr().String(), &conns
}

func readRESPArray(r *bufio.Reader) ([]string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(line, "*") {
		return nil, fmt.Errorf("unexpected %q", line)
	}
	n, err := strconv.Atoi(strings.TrimSpace(line[1:]))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		hdr, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		size, err := strconv.Atoi(strings.TrimSpace(hdr[1:]))
		if err != nil {
			return nil, err
		}
		buf := make([]byte, size+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		out = append(out, string(buf[:size]))
	}
	return out, nil
}

// redisWithPassword 模擬設了 requirepass 的 Redis 7（HELLO 回錯誤，讓 client 退回 RESP2 + AUTH）。
func redisWithPassword(password string) func([]string) string {
	authed := false
	return func(cmd []string) string {
		switch strings.ToUpper(cmd[0]) {
		case "HELLO":
			return "-ERR unknown command 'HELLO'\r\n"
		case "AUTH":
			if cmd[len(cmd)-1] == password {
				authed = true
				return "+OK\r\n"
			}
			return "-WRONGPASS invalid username-password pair or user is disabled.\r\n"
		case "CLIENT":
			return "+OK\r\n"
		case "PING":
			if !authed {
				return "-NOAUTH Authentication required.\r\n"
			}
			return "+PONG\r\n"
		}
		return "-ERR unknown\r\n"
	}
}

const fakeRedisSecret = "S3cr3t-Rd$9#q"

func probeFake(t *testing.T, addr, password string, attempts int, timeout time.Duration) (int, error, []string) {
	t.Helper()
	var fails []string
	sr := &sleepRecorder{}
	n, err := probeRedisClient(context.Background(), config.RedisConfig{Addr: addr, Password: password},
		attempts, time.Millisecond, timeout, sr.sleep, func(a int, c string) { fails = append(fails, c) })
	return n, err, fails
}

func stateless(h func([]string) string) func() func([]string) string {
	return func() func([]string) string { return h }
}

func TestProbeRedisClient_RealClientCauses(t *testing.T) {
	newHandler := func() func([]string) string { return redisWithPassword(fakeRedisSecret) }

	t.Run("ok", func(t *testing.T) {
		addr, _ := fakeRedis(t, newHandler)
		n, err, _ := probeFake(t, addr, fakeRedisSecret, 3, 2*time.Second)
		if err != nil || n != 1 {
			t.Fatalf("密碼正確要第一次就成功：n=%d err=%v", n, err)
		}
	})
	t.Run("noauth", func(t *testing.T) {
		addr, _ := fakeRedis(t, newHandler)
		_, err, fails := probeFake(t, addr, "", 3, 2*time.Second)
		assertProbeErr(t, err, RedisCauseNoAuth, 3, fails)
	})
	t.Run("wrongpass", func(t *testing.T) {
		addr, _ := fakeRedis(t, newHandler)
		_, err, fails := probeFake(t, addr, "wrong-"+fakeRedisSecret, 3, 2*time.Second)
		assertProbeErr(t, err, RedisCauseWrongPass, 3, fails)
		if strings.Contains(err.Error(), fakeRedisSecret) || strings.Contains(err.Error(), "S3cr3t") {
			t.Fatalf("錯誤⛔ 不能帶出密碼：%s", err)
		}
	})
	t.Run("refused", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := ln.Addr().String()
		ln.Close()
		_, perr, fails := probeFake(t, addr, "", 2, 2*time.Second)
		assertProbeErr(t, perr, RedisCauseRefused, 2, fails)
	})
	// 伺服器接受連線但不回應：單次探測要在 context 的 deadline 結束（ContextTimeoutEnabled），
	// ⛔ 不能等到 ReadTimeout（3 秒）或被 client 重試拉長。
	t.Run("timeout bounded by context", func(t *testing.T) {
		addr, _ := fakeRedis(t, stateless(func([]string) string { return "hang" }))
		start := time.Now()
		_, err, fails := probeFake(t, addr, "", 2, 300*time.Millisecond)
		elapsed := time.Since(start)
		assertProbeErr(t, err, RedisCauseTimeout, 2, fails)
		if elapsed > 2*time.Second {
			t.Fatalf("2 次各 300ms 的探測花了 %v：單次上限沒有生效", elapsed)
		}
	})
	// client 內建的重試要關掉：連線一收到指令就被關閉（EOF 是 go-redis 會重試的錯誤），
	// 每次嘗試⛔ 只能開一條連線。重用 Options 的話，第二次起 MaxRetries 會變回 3。
	t.Run("no client retries", func(t *testing.T) {
		addr, conns := fakeRedis(t, stateless(func([]string) string { return "" }))
		_, err, fails := probeFake(t, addr, "", 4, 2*time.Second)
		assertProbeErr(t, err, RedisCauseOther, 4, fails)
		if got := conns.Load(); got != 4 {
			t.Fatalf("4 次嘗試要剛好開 4 條連線，得到 %d（client 內建的重試沒有關掉）", got)
		}
	})
}

func assertProbeErr(t *testing.T, err error, cause string, attempts int, fails []string) {
	t.Helper()
	var pe *RedisProbeError
	if !errors.As(err, &pe) || pe.Cause != cause || pe.Attempts != attempts {
		t.Fatalf("要回傳 RedisProbeError{%s, %d}，得到 %v（每次：%v）", cause, attempts, err, fails)
	}
	if len(fails) != attempts {
		t.Fatalf("每次失敗都要回報，得到 %v", fails)
	}
	for _, f := range fails {
		if f != cause {
			t.Fatalf("每次的原因類別都要是 %s，得到 %v", cause, fails)
		}
	}
}
