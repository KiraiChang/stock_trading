package signal

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/indicator"
	"github.com/trading/backend/internal/store"
)

// I-140：盤中 1 分 K 的 SUPPORT_BOUNCE 用 B 規則去重——同一檔上一筆送出起算 15 分鐘內不再送出、⛔ 不比對支撐價位；
// DB、本機、Redis 三條路徑共用同一個 now（DB 判重前取樣一次）與同一個到期時刻（送出時間截到整秒＋15 分鐘）。
// 規格見 docs/signal-spec.md「訊號去重」與 docs/plans/i140-intraday-bounce-dedup.md。

// t0 是錨點的送出時間（整秒）。
var t0 = time.Date(2026, 11, 24, 1, 1, 0, 0, time.UTC) // 台北 09:01:00

// clock 是可控、可在查詢途中推進的時鐘。
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time  { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) set(t time.Time) { c.mu.Lock(); c.t = t; c.mu.Unlock() }

// supportBounceOnlyCandles 跟 bounceCandles 同形狀，但最後一根量能正常（⛔ 不爆量）：
// bounceCandles 的最後一根量比 3 倍，CheckBreakout 會先判成 VOLUME_SPIKE，走不到 SUPPORT_BOUNCE。
func supportBounceOnlyCandles() []store.Candle {
	cs := bounceCandles()
	cs[len(cs)-1].Volume = 1000
	return cs
}

// newBounceEngine 是 B 規則的 Engine（fixture 只會觸發 SUPPORT_BOUNCE）。
func newBounceEngine(t *testing.T, repo *dedupSignalRepo, redis emissionStore, c *clock) *Engine {
	t.Helper()
	candles := &dedupCandleRepo{candles: supportBounceOnlyCandles()}
	ind := indicator.NewEngine(candles, dedupIndicatorRepo{}, &store.RedisClient{}, zap.NewNop())
	e := NewEngine(candles, repo, &store.RedisClient{}, ind, dedupChipRepo{}, zap.NewNop())
	e.SetEmissionStoreForTest(redis)
	e.SetClockForTest(c.now)
	e.SetDedupPolicy(DedupIntradayBounceBySymbol)
	return e
}

func evalBounce(t *testing.T, e *Engine) *EvaluateResult {
	t.Helper()
	res, err := e.EvaluateWithResult(context.Background(), dedupSymbol, "1m")
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if res.Signal == nil && !res.Suppressed && !res.Withheld {
		t.Fatal("fixture 應觸發 SUPPORT_BOUNCE（送出、抑制或扣住其中之一）")
	}
	if res.Signal != nil && res.Signal.SignalType != "SUPPORT_BOUNCE" {
		t.Fatalf("fixture 失效：觸發的是 %s，⛔ 不是 SUPPORT_BOUNCE（B 規則的測試會變成空測試）", res.Signal.SignalType)
	}
	return res
}

// TestSupportBounceOnlyFixtureFires 是本檔所有測試的前提：fixture 要真的觸發 SUPPORT_BOUNCE。
func TestSupportBounceOnlyFixtureFires(t *testing.T) {
	res := evalBounce(t, newBounceEngine(t, &dedupSignalRepo{}, newFakeRedis(), &clock{t: t0}))
	if !res.SignalGenerated || res.Signal.SignalType != "SUPPORT_BOUNCE" {
		t.Fatalf("fixture 要觸發並送出 SUPPORT_BOUNCE：%+v", res)
	}
}

func tf(s string) store.NullString {
	return store.NullString{NullString: sql.NullString{String: s, Valid: true}}
}

// anchor 是 DB 裡一筆已寫入的盤中 SUPPORT_BOUNCE（支撐 95，跟 fixture 的 100 不同）。
func anchor(createdAt time.Time) store.Signal {
	return store.Signal{Symbol: dedupSymbol, SignalType: "SUPPORT_BOUNCE", Direction: "WATCH", Support: 95,
		Timeframe: tf("1m"), CreatedAt: createdAt, Timestamp: createdAt.Add(-time.Minute)}
}

func disabledRedis() *fakeRedis {
	r := newFakeRedis()
	r.reserveS = &store.ReserveOutcome{Status: store.ReserveDisabled}
	return r
}

// ── 規則本身 ───────────────────────────────────────────────────────

func TestIntradayBounceSuppressesDifferentSupportWithin15Min(t *testing.T) {
	repo := &dedupSignalRepo{inserted: []store.Signal{anchor(t0)}}
	e := newBounceEngine(t, repo, newFakeRedis(), &clock{t: t0.Add(10 * time.Minute)})
	if res := evalBounce(t, e); !res.Suppressed {
		t.Fatalf("15 分鐘內不同支撐（95 → 100）也要抑制：%+v", res)
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("被抑制⛔ 不寫 signals：%d 列", len(repo.inserted))
	}
}

func TestIntradayBounceWritesTimeframeAndTruncatedSentAt(t *testing.T) {
	repo := &dedupSignalRepo{}
	r := newFakeRedis()
	e := newBounceEngine(t, repo, r, &clock{t: t0.Add(700 * time.Millisecond)})
	if res := evalBounce(t, e); !res.SignalGenerated {
		t.Fatalf("沒有錨點時要送出：%+v", res)
	}
	got := repo.inserted[0]
	if !got.CreatedAt.Equal(t0) || got.Timeframe.String != "1m" || !got.Timeframe.Valid {
		t.Fatalf("created_at 要截到整秒（%v）、timeframe=1m：%+v", t0, got)
	}
	key := emissionKey(intradayBounceKey(&got))
	if v := r.keys[key]; !strings.HasSuffix(v, "|"+itoa(t0.Add(15*time.Minute).UnixMilli())) {
		t.Fatalf("Redis 的到期時刻要是送出時間＋15 分鐘（整秒）：%q", v)
	}
	if !strings.HasSuffix(key, "|tf=1m") || strings.Contains(key, "100") {
		t.Fatalf("B 的鍵⛔ 不帶價位、帶 tf=1m：%q", key)
	}
}

func TestIntradayBouncePolicyOnlyAppliesTo1mBounce(t *testing.T) {
	t.Run("日 K 的 SUPPORT_BOUNCE 仍走 A（價位鍵）", func(t *testing.T) {
		repo := &dedupSignalRepo{inserted: []store.Signal{anchor(t0)}}
		r := newFakeRedis()
		e := newBounceEngine(t, repo, r, &clock{t: t0.Add(time.Minute)})
		res, err := e.EvaluateWithResult(context.Background(), dedupSymbol, "1d")
		if err != nil || !res.SignalGenerated {
			t.Fatalf("1d 不受 B 影響（錨點是 1m、支撐不同），要送出：%+v %v", res, err)
		}
		if tfv := repo.inserted[1].Timeframe; tfv.String != "1d" {
			t.Fatalf("1d 的列要記 timeframe=1d：%+v", tfv)
		}
		for k := range r.keys {
			if strings.Contains(k, "tf=") {
				t.Fatalf("1d ⛔ 不用 B 的鍵：%q", k)
			}
		}
	})
	t.Run("A 規則下 1m 的 SUPPORT_BOUNCE 照現行", func(t *testing.T) {
		repo := &dedupSignalRepo{inserted: []store.Signal{anchor(t0)}}
		e := newBounceEngine(t, repo, newFakeRedis(), &clock{t: t0.Add(time.Minute)})
		e.SetDedupPolicy(DedupPriceKey)
		if res := evalBounce(t, e); !res.SignalGenerated {
			t.Fatalf("A 只比價位，支撐不同要送出：%+v", res)
		}
	})
}

// ── 邊界：三條路徑各自在到期前 1 秒抑制、剛好到期放行 ───────────────

func TestIntradayBounceBoundaryDBPath(t *testing.T) {
	for _, tc := range []struct {
		name     string
		now      time.Time
		suppress bool
	}{
		{"到期前 1 秒", t0.Add(15*time.Minute - time.Second), true},
		{"剛好到期", t0.Add(15 * time.Minute), false},
		{"時鐘倒退", t0.Add(-time.Minute), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &dedupSignalRepo{inserted: []store.Signal{anchor(t0)}}
			// 新的 Engine、空的 Redis：只剩 DB 錨點。
			res := evalBounce(t, newBounceEngine(t, repo, newFakeRedis(), &clock{t: tc.now}))
			if res.Suppressed != tc.suppress {
				t.Fatalf("suppressed=%v，要 %v", res.Suppressed, tc.suppress)
			}
		})
	}
}

func TestIntradayBounceBoundaryLocalPath(t *testing.T) {
	for _, tc := range []struct {
		name     string
		after    time.Duration
		suppress bool
	}{
		{"到期前 1 秒", 15*time.Minute - time.Second, true},
		{"剛好到期", 15 * time.Minute, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DB 寫失敗（沒有錨點）＋Redis 停用：只剩本機保留。
			repo := &dedupSignalRepo{insertErr: errors.New("db down")}
			c := &clock{t: t0.Add(700 * time.Millisecond)}
			e := newBounceEngine(t, repo, disabledRedis(), c)
			if res := evalBounce(t, e); !res.SignalGenerated {
				t.Fatalf("第一筆要送出：%+v", res)
			}
			c.set(t0.Add(tc.after))
			if res := evalBounce(t, e); res.Suppressed != tc.suppress {
				t.Fatalf("suppressed=%v，要 %v（到期時刻是 %v，⛔ 不是取樣時刻＋15 分鐘）", res.Suppressed, tc.suppress, t0.Add(15*time.Minute))
			}
		})
	}
}

func TestIntradayBounceBoundaryRedisPath(t *testing.T) {
	for _, tc := range []struct {
		name     string
		after    time.Duration
		suppress bool
	}{
		{"到期前 1 秒", 15*time.Minute - time.Second, true},
		{"剛好到期", 15 * time.Minute, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DB 寫失敗＋每次新的 Engine（本機清空）：只剩 Redis。
			repo := &dedupSignalRepo{insertErr: errors.New("db down")}
			r := newFakeRedis()
			if res := evalBounce(t, newBounceEngine(t, repo, r, &clock{t: t0.Add(700 * time.Millisecond)})); !res.SignalGenerated {
				t.Fatalf("第一筆要送出：%+v", res)
			}
			res := evalBounce(t, newBounceEngine(t, repo, r, &clock{t: t0.Add(tc.after)}))
			if res.Suppressed != tc.suppress {
				t.Fatalf("suppressed=%v，要 %v", res.Suppressed, tc.suppress)
			}
		})
	}
}

// ── 評估延遲（review D1）：09:00 的 K 棒 09:01 送出，09:14 的 K 棒延到 09:16 才評估 ───

func TestIntradayBounceDelayedEvaluationUsesSentTime(t *testing.T) {
	for _, tc := range []struct {
		name     string
		evalAt   time.Time
		suppress bool
	}{
		{"09:16:00 評估（已過 15 分鐘）", t0.Add(15 * time.Minute), false},
		{"09:15:59 評估", t0.Add(15*time.Minute - time.Second), true},
	} {
		t.Run(tc.name+"／有 DB 錨點", func(t *testing.T) {
			repo := &dedupSignalRepo{inserted: []store.Signal{anchor(t0)}}
			if res := evalBounce(t, newBounceEngine(t, repo, newFakeRedis(), &clock{t: tc.evalAt})); res.Suppressed != tc.suppress {
				t.Fatalf("suppressed=%v，要 %v（看送出時間、⛔ 不看 K 棒時間）", res.Suppressed, tc.suppress)
			}
		})
		t.Run(tc.name+"／DB 寫失敗、只剩本機", func(t *testing.T) {
			repo := &dedupSignalRepo{insertErr: errors.New("db down")}
			c := &clock{t: t0}
			e := newBounceEngine(t, repo, disabledRedis(), c)
			evalBounce(t, e)
			c.set(tc.evalAt)
			if res := evalBounce(t, e); res.Suppressed != tc.suppress {
				t.Fatalf("suppressed=%v，要 %v", res.Suppressed, tc.suppress)
			}
		})
		t.Run(tc.name+"／DB 寫失敗、只剩 Redis", func(t *testing.T) {
			repo := &dedupSignalRepo{insertErr: errors.New("db down")}
			r := newFakeRedis()
			evalBounce(t, newBounceEngine(t, repo, r, &clock{t: t0}))
			if res := evalBounce(t, newBounceEngine(t, repo, r, &clock{t: tc.evalAt})); res.Suppressed != tc.suppress {
				t.Fatalf("suppressed=%v，要 %v", res.Suppressed, tc.suppress)
			}
		})
	}
}

// ── DB 查詢橫跨到期秒（review D1）：now 在 DB 判重前取樣一次，三條路徑共用 ─────────

func TestIntradayBounceNowSampledBeforeDBQuery(t *testing.T) {
	expiry := t0.Add(15 * time.Minute)
	t.Run("有 DB 錨點：查詢途中跨過到期仍抑制", func(t *testing.T) {
		c := &clock{t: expiry.Add(-200 * time.Millisecond)}
		repo := &dedupSignalRepo{inserted: []store.Signal{anchor(t0)}}
		repo.latestHook = func() { c.set(expiry.Add(500 * time.Millisecond)) }
		if res := evalBounce(t, newBounceEngine(t, repo, newFakeRedis(), c)); !res.Suppressed {
			t.Fatalf("用查詢前取樣的 now 判斷，要抑制：%+v", res)
		}
	})
	t.Run("只剩保留鎖：本機與 Redis 也用查詢前的 now", func(t *testing.T) {
		repo := &dedupSignalRepo{insertErr: errors.New("db down")}
		r := newFakeRedis()
		c := &clock{t: t0}
		e := newBounceEngine(t, repo, r, c)
		evalBounce(t, e) // 09:01:00 送出，本機與 Redis 都到 09:16:00
		c.set(expiry.Add(-200 * time.Millisecond))
		repo.latestHook = func() { c.set(expiry.Add(500 * time.Millisecond)) }
		if res := evalBounce(t, e); !res.Suppressed {
			t.Fatalf("本機要用查詢前的 now（到期前 0.2 秒）判斷，要抑制：%+v", res)
		}
		repo.latestHook = nil
		res := evalBounce(t, newBounceEngine(t, repo, r, &clock{t: expiry.Add(-200 * time.Millisecond)}))
		if !res.Suppressed {
			t.Fatalf("Redis 也要抑制：%+v", res)
		}
		if n := r.untilNows[len(r.untilNows)-1]; !n.Equal(expiry.Add(-200 * time.Millisecond)) {
			t.Fatalf("傳給 Redis 的 now 要是查詢前取樣的那一個：%v", n)
		}
		if res := evalBounce(t, newBounceEngine(t, repo, r, &clock{t: expiry})); !res.SignalGenerated {
			t.Fatalf("到期後取樣的下一個候選三條都放行：%+v", res)
		}
	})
}

// 複審 P2-1：候選在到期前取樣、DB 查詢途中別檔觸發本機整掃——鎖要還在（B 的鎖到期後再留 EmissionUntilGrace 才可清掃）。
func TestIntradayBounceSweepDuringDBQueryKeepsLock(t *testing.T) {
	expiry := t0.Add(15 * time.Minute)
	repo := &dedupSignalRepo{insertErr: errors.New("db down")} // 沒有 DB 錨點
	c := &clock{t: t0}
	e := newBounceEngine(t, repo, disabledRedis(), c) // 只剩本機
	if res := evalBounce(t, e); !res.SignalGenerated {
		t.Fatalf("09:01:00 第一筆要送出：%+v", res)
	}
	c.set(expiry.Add(-200 * time.Millisecond)) // 09:15:59.8 取樣
	other := "2454"
	repo.latestHook = func() {
		repo.latestHook = nil
		c.set(expiry.Add(500 * time.Millisecond)) // 09:16:00.5：另一檔觸發、做整掃（距上次整掃已超過 1 分鐘）
		res, err := e.EvaluateWithResult(context.Background(), other, "1m")
		if err != nil || res.Signal == nil || res.Signal.Symbol != other || !res.SignalGenerated {
			t.Fatalf("另一檔要送出（順便整掃）：%+v %v", res, err)
		}
	}
	if res := evalBounce(t, e); !res.Suppressed {
		t.Fatalf("取樣在到期前 0.2 秒，整掃⛔ 不能先刪掉 09:16 到期的鎖：%+v", res)
	}
	if repo.latestHook != nil {
		t.Fatal("hook 沒跑到，交錯沒有發生（空測試）")
	}
	// 到期後取樣的下一個候選照常放行。
	c.set(expiry.Add(time.Second))
	if res := evalBounce(t, e); !res.SignalGenerated {
		t.Fatalf("到期後要放行：%+v", res)
	}
}

// B 的本機鎖到期後再留 EmissionUntilGrace 才被整掃刪掉（map ⛔ 不會無限成長）；A 到期就可刪。
func TestLocalReservationsSweepGrace(t *testing.T) {
	l := newLocalReservations()
	expiry := t0.Add(15 * time.Minute)
	if !l.reserveUntil("b", "tok", t0, expiry) || !l.reserve("a", "tok", time.Minute, t0) {
		t.Fatal("首次保留要成功")
	}
	l.reserve("x1", "tok", time.Second, expiry.Add(store.EmissionUntilGrace-time.Second)) // 觸發整掃
	if l.size() != 2 {                                                                    // a 已刪；b 還在寬限內；x1 新增
		t.Fatalf("寬限內 b 要保留、a 要刪：size=%d", l.size())
	}
	if l.reserveUntil("b", "tok2", expiry.Add(-time.Millisecond), expiry.Add(15*time.Minute)) {
		t.Fatal("寬限內鎖還在：到期前取樣的候選要被擋")
	}
	l.reserve("x2", "tok", time.Second, expiry.Add(store.EmissionUntilGrace+time.Minute)) // 下一次整掃
	if l.size() != 1 {
		t.Fatalf("過了寬限 b 要被刪：size=%d", l.size())
	}
}

// 複審 P2-2：DB 錨點依 created_at 選，⛔ 不依寫入順序——09:20 那筆先寫、09:04 那筆後寫，09:22 的候選仍要被 09:20 擋。
func TestIntradayBounceDBAnchorByCreatedAtNotWriteOrder(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 11, 24, h-8, m, 0, 0, time.UTC) }
	repo := &dedupSignalRepo{inserted: []store.Signal{anchor(at(9, 20)), anchor(at(9, 4))}}
	// Redis 與本機都清空（新 Engine、Redis 停用），只剩 DB。
	if res := evalBounce(t, newBounceEngine(t, repo, disabledRedis(), &clock{t: at(9, 22)})); !res.Suppressed {
		t.Fatalf("09:22 在 09:20 錨點的冷卻內，要抑制：%+v", res)
	}
	if res := evalBounce(t, newBounceEngine(t, repo, disabledRedis(), &clock{t: at(9, 35)})); !res.SignalGenerated {
		t.Fatalf("09:35 已過 09:20＋15 分鐘，要放行：%+v", res)
	}
}

// ── 被抑制⛔ 不延長冷卻：09:01 送出、09:11 被抑制、09:16 送出 ──────────────────

func TestIntradayBounceSuppressedDoesNotExtendCooldown(t *testing.T) {
	steps := []time.Time{t0, t0.Add(10 * time.Minute), t0.Add(15 * time.Minute)}
	want := []bool{true, false, true} // 送出與否
	check := func(t *testing.T, got []bool) {
		t.Helper()
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("第 %d 步送出=%v，要 %v（錨點是 09:01，⛔ 不是 09:11）", i+1, got[i], want[i])
			}
		}
	}
	t.Run("DB", func(t *testing.T) {
		repo := &dedupSignalRepo{}
		got := []bool{}
		for _, s := range steps {
			got = append(got, evalBounce(t, newBounceEngine(t, repo, newFakeRedis(), &clock{t: s})).SignalGenerated)
		}
		check(t, got)
	})
	t.Run("本機", func(t *testing.T) {
		repo := &dedupSignalRepo{insertErr: errors.New("db down")}
		c := &clock{}
		e := newBounceEngine(t, repo, disabledRedis(), c)
		got := []bool{}
		for _, s := range steps {
			c.set(s)
			got = append(got, evalBounce(t, e).SignalGenerated)
		}
		check(t, got)
	})
	t.Run("Redis", func(t *testing.T) {
		repo := &dedupSignalRepo{insertErr: errors.New("db down")}
		r := newFakeRedis()
		got := []bool{}
		for _, s := range steps {
			got = append(got, evalBounce(t, newBounceEngine(t, repo, r, &clock{t: s})).SignalGenerated)
		}
		check(t, got)
	})
}

// ── 送出證據與錨點（計畫 3.3 的情境 2、4、5）───────────────────────────

func TestIntradayBounceEvidenceAndAnchor(t *testing.T) {
	t.Run("情境 2：DB 寫失敗、佇列送出 → 下一輪由保留鎖擋", func(t *testing.T) {
		repo := &dedupSignalRepo{insertErr: errors.New("db down")}
		c := &clock{t: t0}
		e := newBounceEngine(t, repo, newFakeRedis(), c)
		if res := evalBounce(t, e); !res.SignalGenerated || res.DBPersisted || !res.QueueEnqueued {
			t.Fatalf("要送出（degraded）：%+v", res)
		}
		c.set(t0.Add(5 * time.Minute))
		if res := evalBounce(t, e); !res.Suppressed {
			t.Fatalf("保留鎖要擋：%+v", res)
		}
	})
	t.Run("情境 4：DB 有寫、通道都沒送出 → 釋放保留鎖，下一輪由 DB 錨點擋", func(t *testing.T) {
		repo := &dedupSignalRepo{}
		r := newFakeRedis()
		r.enqueueS = &store.EnqueueOutcome{Status: store.EnqueueDisabled}
		c := &clock{t: t0}
		e := newBounceEngine(t, repo, r, c) // fixture 沒有 BroadcastFn
		if res := evalBounce(t, e); !res.DBPersisted || res.QueueEnqueued || res.BroadcastAttempted {
			t.Fatalf("前提：DB 有寫、通道都沒送出：%+v", res)
		}
		if len(r.keys) != 0 {
			t.Fatalf("通道都沒送出要釋放 Redis 保留：%v", r.keys)
		}
		c.set(t0.Add(5 * time.Minute))
		if res := evalBounce(t, e); !res.Suppressed {
			t.Fatalf("DB 錨點要擋：%+v", res)
		}
	})
	t.Run("情境 5：全部失敗 → ⛔ 沒有錨點，下一輪可再發", func(t *testing.T) {
		repo := &dedupSignalRepo{insertErr: errors.New("db down")}
		r := newFakeRedis()
		r.enqueueS = &store.EnqueueOutcome{Status: store.EnqueueDisabled}
		c := &clock{t: t0}
		e := newBounceEngine(t, repo, r, c)
		evalBounce(t, e)
		c.set(t0.Add(time.Minute))
		if res := evalBounce(t, e); !res.SignalGenerated {
			t.Fatalf("未送出⛔ 不是錨點，要能再發：%+v", res)
		}
	})
	t.Run("Withhold：⛔ 不寫、⛔ 不保留，⛔ 不是錨點", func(t *testing.T) {
		repo := &dedupSignalRepo{}
		r := newFakeRedis()
		c := &clock{t: t0}
		e := newBounceEngine(t, repo, r, c)
		res, err := e.EvaluateWithOptions(context.Background(), dedupSymbol, "1m", EvaluateOptions{Withhold: true})
		if err != nil || !res.Withheld || len(repo.inserted) != 0 || len(r.keys) != 0 {
			t.Fatalf("扣住⛔ 不寫、⛔ 不保留：%+v %v keys=%v", res, err, r.keys)
		}
		c.set(t0.Add(time.Minute))
		if res := evalBounce(t, e); !res.SignalGenerated {
			t.Fatalf("扣住的那筆⛔ 不是錨點：%+v", res)
		}
	})
	t.Run("Withhold 遇到 DB 錨點：先判重，記成抑制", func(t *testing.T) {
		repo := &dedupSignalRepo{inserted: []store.Signal{anchor(t0)}}
		e := newBounceEngine(t, repo, newFakeRedis(), &clock{t: t0.Add(5 * time.Minute)})
		res, err := e.EvaluateWithOptions(context.Background(), dedupSymbol, "1m", EvaluateOptions{Withhold: true})
		if err != nil || !res.Suppressed || res.Withheld {
			t.Fatalf("引擎先做 DB 判重再處理 Withhold：%+v %v", res, err)
		}
	})
}

// ── Redis 與重啟（計畫 3.5）─────────────────────────────────────────

func TestIntradayBounceRedisAndRestart(t *testing.T) {
	t.Run("Redis 鍵消失（Redis 重啟）→ DB 錨點擋", func(t *testing.T) {
		repo := &dedupSignalRepo{}
		r := newFakeRedis()
		evalBounce(t, newBounceEngine(t, repo, r, &clock{t: t0}))
		r.keys = map[string]string{}
		if res := evalBounce(t, newBounceEngine(t, repo, r, &clock{t: t0.Add(5 * time.Minute)})); !res.Suppressed {
			t.Fatalf("要由 DB 擋：%+v", res)
		}
	})
	t.Run("backend 重啟＋Redis 停用 → 只靠 DB 錨點", func(t *testing.T) {
		repo := &dedupSignalRepo{}
		evalBounce(t, newBounceEngine(t, repo, disabledRedis(), &clock{t: t0}))
		if res := evalBounce(t, newBounceEngine(t, repo, disabledRedis(), &clock{t: t0.Add(5 * time.Minute)})); !res.Suppressed {
			t.Fatalf("要由 DB 擋：%+v", res)
		}
	})
	t.Run("Redis 退避 → 放行並標 dedup_degraded，本機仍保留", func(t *testing.T) {
		repo := &dedupSignalRepo{insertErr: errors.New("db down")}
		r := newFakeRedis()
		r.reserveS = &store.ReserveOutcome{Status: store.ReserveBackoff, Err: store.ErrRedisWriteBackoff}
		c := &clock{t: t0}
		e := newBounceEngine(t, repo, r, c)
		res := evalBounce(t, e)
		if !res.SignalGenerated || !hasStage(res, StageDedupDegraded) {
			t.Fatalf("要放行且標降級：%+v", res)
		}
		c.set(t0.Add(5 * time.Minute))
		if res := evalBounce(t, e); !res.Suppressed {
			t.Fatalf("本機保留要擋：%+v", res)
		}
	})
	t.Run("回應遺失後重試（同一 token）→ 照常送出，⛔ 不被自己的鎖擋", func(t *testing.T) {
		repo := &dedupSignalRepo{}
		r := newFakeRedis()
		r.lostResponses = 1
		if res := evalBounce(t, newBounceEngine(t, repo, r, &clock{t: t0})); !res.SignalGenerated {
			t.Fatalf("重試看到自己的 token 要回已取得：%+v", res)
		}
	})
}

// ── 非目標訊號的「最近 20 筆」邊界（review D4）─────────────────────────
//
// A 的 DB 判重只看同一檔最近 20 筆。B 少寫幾筆 SUPPORT_BOUNCE 時，原本排第 21 筆、跟候選同身分的 BREAKDOWN
// 會落進 20 筆內——這是 B 對 A 唯一的間接影響（要同一檔 15 分鐘內超過 20 筆訊號才會發生）。這裡鎖住兩種結果。

func TestPriceKeyDedupRecent20Boundary(t *testing.T) {
	ts := t0
	breakdown := store.Signal{Symbol: dedupSymbol, SignalType: "BREAKDOWN", Direction: "SELL", Support: 90, Timestamp: ts}
	candidate := &store.Signal{Symbol: dedupSymbol, SignalType: "BREAKDOWN", Direction: "SELL", Support: 90, Timestamp: ts.Add(10 * time.Minute)}
	build := func(bounces int) *dedupSignalRepo {
		repo := &dedupSignalRepo{inserted: []store.Signal{breakdown}}
		for i := 0; i < bounces; i++ {
			repo.inserted = append(repo.inserted, store.Signal{Symbol: dedupSymbol, SignalType: "SUPPORT_BOUNCE", Direction: "WATCH",
				Support: 95, Timeframe: tf("1m"), Timestamp: ts.Add(time.Duration(i) * 20 * time.Second)})
		}
		return repo
	}
	e := &Engine{}
	e.signals = build(20) // BREAKDOWN 是第 21 筆
	if s, _ := e.shouldSuppressDuplicate(context.Background(), dedupSymbol, candidate); s {
		t.Fatal("第 21 筆在最近 20 筆之外，A 放行（現行行為）")
	}
	e.signals = build(15) // B 少寫之後落進 20 筆內
	if s, _ := e.shouldSuppressDuplicate(context.Background(), dedupSymbol, candidate); !s {
		t.Fatal("落進最近 20 筆之後，A 會擋下同身分的 BREAKDOWN")
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func hasStage(res *EvaluateResult, s Stage) bool {
	for _, x := range res.DegradedStages {
		if x == s {
			return true
		}
	}
	return false
}
