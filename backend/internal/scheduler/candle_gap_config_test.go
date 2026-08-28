package scheduler

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/market"
)

// validGapConfig 是一組全部合法的設定，用來當「只改一個欄位」的基準。
// **不用 config 的預設值當基準**：那樣會分不出「正規化把它改回預設」與「它本來就是預設」。
func validGapConfig() config.CandleGapDetectionConfig {
	return config.CandleGapDetectionConfig{
		Enabled:             true,
		AggregateRatio:      0.4,
		AggregateMinSymbols: 3,
		CandidateCapPerRun:  15,
		TimeoutSec:          200,
		LookbackTradingDays: 7,
		RequestIntervalMs:   250,
		MarketStaleDays:     3,
		CalendarTTLHours:    12,
		BreakerFailures:     4,
		BreakerCooldownMin:  30,
	}
}

func newObservedLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.ErrorLevel)
	return zap.New(core), logs
}

// 合法值一律原樣通過——正規化不得「順手」改動使用者設定的合法值。
func TestNormalizeCandleGapDetectionKeepsValidValues(t *testing.T) {
	log, logs := newObservedLogger()
	in := validGapConfig()

	got := NormalizeCandleGapDetectionConfig(in, log)

	if got != in {
		t.Errorf("合法設定不該被改動\n got=%+v\nwant=%+v", got, in)
	}
	if logs.Len() != 0 {
		t.Errorf("合法設定不該產生 Error log，得到 %d 筆", logs.Len())
	}
}

// 矩陣 #36：解析成功但超出範圍 → 正規化 ＋ Error log，**且正規化後的行為與合法值一致**。
//
// 四個「必測」的值（cap=0、lookback=0、interval=0、ratio=1.5）都在表裡：
// 前三個都是「一個都不驗／空視窗／取消節流」這種**看起來成功**的靜默失效，
// 最後一個會讓 aggregate 永遠不短路。
func TestNormalizeCandleGapDetectionRejectsOutOfRangeValues(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*config.CandleGapDetectionConfig)
		check  func(*testing.T, config.CandleGapDetectionConfig)
	}{
		{"aggregate_ratio 為 0（任何缺口都會短路）",
			func(c *config.CandleGapDetectionConfig) { c.AggregateRatio = 0 },
			func(t *testing.T, c config.CandleGapDetectionConfig) {
				if c.AggregateRatio != defaultGapAggregateRatio {
					t.Errorf("aggregate_ratio = %v, 期望退回 %v", c.AggregateRatio, defaultGapAggregateRatio)
				}
			}},
		{"aggregate_ratio 為 1.5（永遠不短路）",
			func(c *config.CandleGapDetectionConfig) { c.AggregateRatio = 1.5 },
			func(t *testing.T, c config.CandleGapDetectionConfig) {
				if c.AggregateRatio != defaultGapAggregateRatio {
					t.Errorf("aggregate_ratio = %v, 期望退回 %v", c.AggregateRatio, defaultGapAggregateRatio)
				}
			}},
		{"aggregate_ratio 為 1（邊界，合法）",
			func(c *config.CandleGapDetectionConfig) { c.AggregateRatio = 1 },
			func(t *testing.T, c config.CandleGapDetectionConfig) {
				if c.AggregateRatio != 1 {
					t.Errorf("1 是合法上界（範圍是 (0, 1]），不該被改成 %v", c.AggregateRatio)
				}
			}},
		{"aggregate_min_symbols 為 0",
			func(c *config.CandleGapDetectionConfig) { c.AggregateMinSymbols = 0 },
			func(t *testing.T, c config.CandleGapDetectionConfig) {
				if c.AggregateMinSymbols != defaultGapAggregateMinSymbols {
					t.Errorf("got %d", c.AggregateMinSymbols)
				}
			}},
		{"candidate_cap_per_run 為 0（一個候選都不驗卻回報成功）",
			func(c *config.CandleGapDetectionConfig) { c.CandidateCapPerRun = 0 },
			func(t *testing.T, c config.CandleGapDetectionConfig) {
				if c.CandidateCapPerRun != defaultGapCandidateCap {
					t.Errorf("got %d, 期望退回 %d", c.CandidateCapPerRun, defaultGapCandidateCap)
				}
			}},
		{"candidate_cap_per_run 超過上限（單輪請求量暴增）",
			func(c *config.CandleGapDetectionConfig) { c.CandidateCapPerRun = 500 },
			func(t *testing.T, c config.CandleGapDetectionConfig) {
				if c.CandidateCapPerRun != maxGapCandidateCap {
					t.Errorf("got %d, 期望截到 %d", c.CandidateCapPerRun, maxGapCandidateCap)
				}
			}},
		{"timeout_sec 為 0",
			func(c *config.CandleGapDetectionConfig) { c.TimeoutSec = 0 },
			func(t *testing.T, c config.CandleGapDetectionConfig) {
				if c.TimeoutSec != defaultGapTimeoutSec {
					t.Errorf("got %d", c.TimeoutSec)
				}
			}},
		{"timeout_sec 超過 hard cap",
			func(c *config.CandleGapDetectionConfig) { c.TimeoutSec = 5000 },
			func(t *testing.T, c config.CandleGapDetectionConfig) {
				if c.TimeoutSec != maxGapTimeoutSec {
					t.Errorf("got %d, 期望截到 %d", c.TimeoutSec, maxGapTimeoutSec)
				}
			}},
		{"lookback_trading_days 為 0（空視窗＝永遠正常）",
			func(c *config.CandleGapDetectionConfig) { c.LookbackTradingDays = 0 },
			func(t *testing.T, c config.CandleGapDetectionConfig) {
				if c.LookbackTradingDays != defaultGapLookbackDays {
					t.Errorf("got %d", c.LookbackTradingDays)
				}
			}},
		{"lookback_trading_days 超過上限",
			func(c *config.CandleGapDetectionConfig) { c.LookbackTradingDays = 999 },
			func(t *testing.T, c config.CandleGapDetectionConfig) {
				if c.LookbackTradingDays != maxGapLookbackDays {
					t.Errorf("got %d, 期望截到 %d", c.LookbackTradingDays, maxGapLookbackDays)
				}
			}},
		{"request_interval_ms 為 0（等於取消對交易所的節流）",
			func(c *config.CandleGapDetectionConfig) { c.RequestIntervalMs = 0 },
			func(t *testing.T, c config.CandleGapDetectionConfig) {
				// **截到下限而不是退回預設**：使用者的意圖是「快一點」，
				// 退回 500 等於反向調整。
				if c.RequestIntervalMs != minGapRequestIntervalMs {
					t.Errorf("got %d, 期望截到下限 %d", c.RequestIntervalMs, minGapRequestIntervalMs)
				}
			}},
		{"market_stale_days 為 0",
			func(c *config.CandleGapDetectionConfig) { c.MarketStaleDays = 0 },
			func(t *testing.T, c config.CandleGapDetectionConfig) {
				if c.MarketStaleDays != defaultGapMarketStaleDays {
					t.Errorf("got %d", c.MarketStaleDays)
				}
			}},
		{"calendar_ttl_hours 為 0",
			func(c *config.CandleGapDetectionConfig) { c.CalendarTTLHours = 0 },
			func(t *testing.T, c config.CandleGapDetectionConfig) {
				if c.CalendarTTLHours != defaultGapCalendarTTLHours {
					t.Errorf("got %d", c.CalendarTTLHours)
				}
			}},
		{"breaker_failures 為 0（breaker 永遠開著，偵測整條不可達）",
			func(c *config.CandleGapDetectionConfig) { c.BreakerFailures = 0 },
			func(t *testing.T, c config.CandleGapDetectionConfig) {
				if c.BreakerFailures != defaultGapBreakerFailures {
					t.Errorf("got %d", c.BreakerFailures)
				}
			}},
		{"breaker_cooldown_min 為 0",
			func(c *config.CandleGapDetectionConfig) { c.BreakerCooldownMin = 0 },
			func(t *testing.T, c config.CandleGapDetectionConfig) {
				if c.BreakerCooldownMin != defaultGapBreakerCooldownMin {
					t.Errorf("got %d", c.BreakerCooldownMin)
				}
			}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log, logs := newObservedLogger()
			in := validGapConfig()
			tc.mutate(&in)

			got := NormalizeCandleGapDetectionConfig(in, log)
			tc.check(t, got)

			// **正規化之後的行為要與合法值一致**：再跑一次不得再改動、也不得再記 Error。
			// 沒有這條的話，一個「每次都退回預設卻每次都記 Error」的實作也會通過。
			log2, logs2 := newObservedLogger()
			again := NormalizeCandleGapDetectionConfig(got, log2)
			if again != got {
				t.Errorf("正規化不是冪等的：%+v → %+v", got, again)
			}

			// 邊界合法值（ratio=1）那筆本來就不該有 log，其餘每筆都要有。
			wantErrorLog := tc.name != "aggregate_ratio 為 1（邊界，合法）"
			if wantErrorLog && logs.Len() == 0 {
				t.Error("超出範圍必須記 Error log——悄悄改掉設定會讓告警結果變了卻無跡可循")
			}
			if !wantErrorLog && logs.Len() != 0 {
				t.Errorf("合法邊界值不該記 Error，得到 %d 筆", logs.Len())
			}
			if logs2.Len() != 0 {
				t.Errorf("已正規化的值不該再記 Error，得到 %d 筆", logs2.Len())
			}
		})
	}
}

// 零值 config（例如未注入時）要能被正規化成一組可用的值，而不是留著一堆 0。
//
// 這條守的是「下游可以無條件信任正規化後的值」那個承諾。
func TestNormalizeCandleGapDetectionFillsZeroValueConfig(t *testing.T) {
	log, _ := newObservedLogger()

	got := NormalizeCandleGapDetectionConfig(config.CandleGapDetectionConfig{}, log)

	if got.AggregateRatio != defaultGapAggregateRatio ||
		got.AggregateMinSymbols != defaultGapAggregateMinSymbols ||
		got.CandidateCapPerRun != defaultGapCandidateCap ||
		got.TimeoutSec != defaultGapTimeoutSec ||
		got.LookbackTradingDays != defaultGapLookbackDays ||
		got.RequestIntervalMs != minGapRequestIntervalMs ||
		got.MarketStaleDays != defaultGapMarketStaleDays ||
		got.CalendarTTLHours != defaultGapCalendarTTLHours ||
		got.BreakerFailures != defaultGapBreakerFailures ||
		got.BreakerCooldownMin != defaultGapBreakerCooldownMin {
		t.Errorf("零值 config 應被填成可用的值，得到 %+v", got)
	}
	// Enabled 是唯一不該被正規化動到的欄位——它沒有「超出範圍」的情形。
	if got.Enabled {
		t.Error("Enabled 不該被正規化改動")
	}
}

// ── Review 回歸：正規化必須發生在**建 client 之前**（#1） ────────────────────

// **這條測的是 wiring 的行為，不是函式的輸出。**
//
// 前一版只斷言 NormalizeCandleGapDetectionConfig 的回傳值——那守不住原始 bug：
// main.go 就算改回「用原始設定建 client」，那支測試照樣會通過。
//
// 現在直接斷言 NewCandleGapDependencies 回傳的 **breaker 實際行為**：
// 餵 breaker_failures=0 進去，若 helper 用的是原始值，NewSourceBreaker 會兜底成 1、
// **第一次失敗就開啟**；用正規化後的值才會是規格預設的 5。
func TestCandleGapDependenciesUseNormalisedValues(t *testing.T) {
	log, _ := newObservedLogger()

	cfg, breaker, reference := NewCandleGapDependencies(config.CandleGapDetectionConfig{
		Enabled:            true,
		RequestIntervalMs:  0, // 等於取消對交易所的節流
		BreakerFailures:    0, // 會讓 breaker 永遠開著
		BreakerCooldownMin: 0,
		CalendarTTLHours:   0,
	}, log)

	if reference == nil {
		t.Fatal("應建出 exchange client")
	}
	// 回傳的 cfg 要是正規化後的——scheduler 那邊會用它。
	if cfg.RequestIntervalMs != minGapRequestIntervalMs {
		t.Errorf("回傳的節流間隔仍非法：%d", cfg.RequestIntervalMs)
	}

	// **關鍵斷言**：breaker 的門檻要是 5，不是兜底的 1。
	for i := 0; i < defaultGapBreakerFailures-1; i++ {
		breaker.Fail(market.SourceTWSEStockDay)
		if breaker.IsOpen(market.SourceTWSEStockDay) {
			t.Fatalf("第 %d 次失敗就開啟——client 用的是原始設定而非正規化後的值", i+1)
		}
	}
	breaker.Fail(market.SourceTWSEStockDay)
	if !breaker.IsOpen(market.SourceTWSEStockDay) {
		t.Errorf("達到規格預設門檻 %d 應開啟", defaultGapBreakerFailures)
	}
}

// 合法設定要原樣流到 breaker，正規化不得「順手」改動它。
func TestCandleGapDependenciesKeepValidBreakerThreshold(t *testing.T) {
	log, _ := newObservedLogger()
	cfg := validGapConfig()
	cfg.BreakerFailures = 2

	got, breaker, _ := NewCandleGapDependencies(cfg, log)

	if got.BreakerFailures != 2 {
		t.Errorf("合法值不該被改動，得到 %d", got.BreakerFailures)
	}
	breaker.Fail(market.SourceTPExStockDay)
	if breaker.IsOpen(market.SourceTPExStockDay) {
		t.Error("門檻 2，第一次失敗不該開啟")
	}
	breaker.Fail(market.SourceTPExStockDay)
	if !breaker.IsOpen(market.SourceTPExStockDay) {
		t.Error("門檻 2，第二次失敗應開啟")
	}
}
