package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/trading/backend/internal/store"
)

// 這一組守的是 docs/issue.md I-105：`StockTradedDates` 失敗時**成因要記下來**，
// 不能只把它收進 unavailable 計數。
//
// **為什麼這件事值得一支測試**：計數本身照樣會加、job_runs 照樣收 partial，
// 所以「有沒有記成因」不會讓任何既有斷言變紅——漏掉就是靜默漏掉。

func newWarnObservedLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.WarnLevel)
	return zap.New(core), logs
}

func TestVerifyCandidatesLogsUnavailableCause(t *testing.T) {
	cause := errors.New("verification unavailable: twse stat=\"很抱歉，沒有符合條件的資料!\" (symbol=2867)")
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{traded: map[string]map[string]bool{}, tradedErr: map[string]error{"2867": cause}}

	s, _ := newGapScheduler(verification, reference, &gapCandleStub{}, nil, nil)
	log, logs := newWarnObservedLogger()
	s.log = log

	attempts := map[string]*gapAttemptAgg{}
	_, unavailable, _ := s.verifyCandidates(context.Background(), []gapCandidate{
		{Symbol: "2867", Market: "上市", Date: "2026-09-01"},
	}, time.Now(), attempts, nil)

	if unavailable != 1 {
		t.Fatalf("前提：應計入 unavailable，得到 %d", unavailable)
	}

	entries := logs.FilterMessage("candle gap verification unavailable").All()
	if len(entries) != 1 {
		t.Fatalf("成因必須記下來，不能只有計數；實際 log 筆數 %d", len(entries))
	}

	fields := entries[0].ContextMap()
	for key, want := range map[string]any{
		"symbol": "2867",
		"market": "上市",
		"year":   int64(2026),
		"month":  "September",
	} {
		if got := fields[key]; got != want {
			t.Errorf("log 欄位 %s = %v, want %v", key, got, want)
		}
	}
	// **cause 本身要在**——沒有它就只知道「驗不了」，不知道是暫時性還是結構性。
	if got, ok := fields["error"].(string); !ok || got == "" {
		t.Errorf("log 必須帶 error，得到 %v", fields["error"])
	} else if got != cause.Error() {
		t.Errorf("log 的 error 應為原始 cause，得到 %q", got)
	}
}

// TestVerifyCandidatesLogsEachFailedMonth：兩個月份都失敗時**各記一次**。
func TestVerifyCandidatesLogsEachFailedMonth(t *testing.T) {
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{
		traded:    map[string]map[string]bool{},
		tradedErr: map[string]error{"2867": errors.New("boom")},
	}

	s, _ := newGapScheduler(verification, reference, &gapCandleStub{}, nil, nil)
	log, logs := newWarnObservedLogger()
	s.log = log

	attempts := map[string]*gapAttemptAgg{}
	// 同一檔跨兩個月：(symbol, month) 去重後是兩組。
	_, unavailable, _ := s.verifyCandidates(context.Background(), []gapCandidate{
		{Symbol: "2867", Market: "上市", Date: "2026-08-31"},
		{Symbol: "2867", Market: "上市", Date: "2026-09-01"},
	}, time.Now(), attempts, nil)

	if unavailable != 2 {
		t.Fatalf("兩個月份都失敗應計 2，得到 %d", unavailable)
	}
	entries := logs.FilterMessage("candle gap verification unavailable").All()
	if len(entries) != 2 {
		t.Fatalf("兩個月份各要記一次，實際 %d 筆", len(entries))
	}
	months := map[string]bool{}
	for _, e := range entries {
		months[e.ContextMap()["month"].(string)] = true
	}
	if !months["August"] || !months["September"] {
		t.Errorf("兩個月份都要出現，得到 %v", months)
	}
}

// TestVerifyCandidatesCrossMonthOneSucceedsOneFails 重現 **2026-09-02 live 的實際形狀**：
// `2867` 跨月後 8 月那組核對成功、9 月那組失敗。
//
// ⚠️ **這才是 I-105 要解的情境**。前一版的跨月測試讓兩個月份都失敗
// （stub 的 tradedErr 只以 symbol 為 key，做不出一成一敗），
// 於是「能不能指出是哪一個月失敗」根本沒被驗到——兩筆 Warn 蓋住了整個問題。
func TestVerifyCandidatesCrossMonthOneSucceedsOneFails(t *testing.T) {
	cause := errors.New(`verification unavailable: twse stat="很抱歉，沒有符合條件的資料!" (symbol=2867)`)
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{
		// 8 月成功（回空集合＝交易所那幾天也沒成交），9 月失敗。
		traded: map[string]map[string]bool{"2867": {}},
		tradedErrByMonth: map[gapStubMonthKey]error{
			{"2867", 2026, time.September}: cause,
		},
	}

	s, _ := newGapScheduler(verification, reference, &gapCandleStub{}, nil, nil)
	log, logs := newWarnObservedLogger()
	s.log = log

	attempts := map[string]*gapAttemptAgg{}
	_, unavailable, _ := s.verifyCandidates(context.Background(), []gapCandidate{
		// 8 月三天、9 月一天——順便讓 missing_dates 兩組不同，才驗得出它有沒有帶對。
		{Symbol: "2867", Market: "上市", Date: "2026-08-28"},
		{Symbol: "2867", Market: "上市", Date: "2026-08-29"},
		{Symbol: "2867", Market: "上市", Date: "2026-08-31"},
		{Symbol: "2867", Market: "上市", Date: "2026-09-01"},
	}, time.Now(), attempts, nil)

	if unavailable != 1 {
		t.Fatalf("只有 9 月失敗應計 1，得到 %d", unavailable)
	}

	entries := logs.FilterMessage("candle gap verification unavailable").All()
	if len(entries) != 1 {
		t.Fatalf("只有失敗的那個月份要記 log，實際 %d 筆", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["month"] != "September" {
		t.Errorf("log 必須指出是 9 月失敗，得到 %v", fields["month"])
	}
	// missing_dates 是新增欄位，沒有斷言就沒被鎖住。9 月只有 09-01 一天。
	if fields["missing_dates"] != int64(1) {
		t.Errorf("missing_dates = %v, want 1（9 月只缺 09-01）", fields["missing_dates"])
	}

	// **coalesce 契約**：有任何一個月成功 → consecutive_failures 歸零、
	// last_verified_at 有值。2026-09-02 live 的 consecutive_failures=0 就是這條的結果，
	// 它本身是「一成一敗」的證據，不是異常。
	agg, ok := attempts["2867"]
	if !ok {
		t.Fatal("應有 2867 的 attempt 彙總")
	}
	if !agg.anySuccess {
		t.Error("8 月成功了，anySuccess 必須為 true")
	}
	if !agg.anyUnavailable {
		t.Error("9 月失敗了，anyUnavailable 必須為 true")
	}
	if agg.result != store.VerificationUnavailable {
		t.Errorf("result 取最嚴重應為 unavailable，得到 %q", agg.result)
	}
}

// TestVerifyCandidatesDoesNotLogWhenSuccessful 是對照組——沒有它，
// 上面兩支可能是「永遠會記」而不是「失敗才記」。
func TestVerifyCandidatesDoesNotLogWhenSuccessful(t *testing.T) {
	verification := &gapVerificationStub{}
	reference := &gapReferenceStub{traded: map[string]map[string]bool{"2867": {}}}

	s, _ := newGapScheduler(verification, reference, &gapCandleStub{}, nil, nil)
	log, logs := newWarnObservedLogger()
	s.log = log

	attempts := map[string]*gapAttemptAgg{}
	_, unavailable, _ := s.verifyCandidates(context.Background(), []gapCandidate{
		{Symbol: "2867", Market: "上市", Date: "2026-09-01"},
	}, time.Now(), attempts, nil)

	if unavailable != 0 {
		t.Fatalf("前提：核對成功不該計入 unavailable，得到 %d", unavailable)
	}
	if n := logs.FilterMessage("candle gap verification unavailable").Len(); n != 0 {
		t.Errorf("成功時不該記這行，實際 %d 筆", n)
	}
}
