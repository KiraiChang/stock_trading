package tradingday

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

// T-089：讀手動設定失敗或逾時時，判斷結果照舊（Source 仍是 calendar／fallback、IsTrading 照日曆），
// 只多一個內部旗標 OverrideUnavailable，讓呼叫端知道那一天⛔ 沒有確認過手動設定。
func TestJudgeOverrideUnavailableFlag(t *testing.T) {
	ctx := context.Background()
	cal := &fakeCal{closed: map[string]bool{"2026-12-25": true}}

	t.Run("讀取失敗：照日曆、旗標 true", func(t *testing.T) {
		ov := &fakeOverrides{err: errors.New("db down")}
		j, _ := newJudge(cal, ov, monday)
		r := j.On(ctx, monday)
		if r.Source != SourceCalendar || !r.IsTrading || !r.OverrideUnavailable {
			t.Fatalf("交易日：要 calendar/true/旗標 true，得到 %+v", r)
		}
		r = j.On(ctx, holiday)
		if r.Source != SourceCalendar || r.IsTrading || !r.OverrideUnavailable {
			t.Fatalf("日曆休市日：要 calendar/false/旗標 true，得到 %+v", r)
		}
	})

	t.Run("讀取逾時：照日曆、旗標 true", func(t *testing.T) {
		release := make(chan struct{})
		defer close(release)
		j := New(cal, stuckOverrides{honorCtx: true, release: release}, zap.NewNop())
		j.SetTimingForTest(time.Second, 5*time.Minute, func() time.Time { return monday })
		j.SetOverrideTimeoutForTest(30 * time.Millisecond)
		r := j.On(ctx, monday)
		if r.Source != SourceCalendar || !r.IsTrading || !r.OverrideUnavailable {
			t.Fatalf("要 calendar/true/旗標 true，得到 %+v", r)
		}
	})

	t.Run("讀取失敗而且日曆取不到：fallback、旗標 true", func(t *testing.T) {
		ov := &fakeOverrides{err: errors.New("db down")}
		j, _ := newJudge(&fakeCal{err: errors.New("twse down")}, ov, monday)
		r := j.On(ctx, monday)
		if r.Source != SourceFallback || !r.IsTrading || !r.OverrideUnavailable {
			t.Fatalf("要 fallback/true/旗標 true，得到 %+v", r)
		}
	})

	t.Run("讀到（沒有設定、cleared）：旗標 false", func(t *testing.T) {
		ov := &fakeOverrides{}
		j, _ := newJudge(cal, ov, monday)
		if r := j.On(ctx, monday); r.Source != SourceCalendar || r.OverrideUnavailable {
			t.Fatalf("沒有手動設定：要 calendar、旗標 false，得到 %+v", r)
		}
		ov.set("2026-11-23", store.MarketDayCleared)
		if r := j.On(ctx, monday); r.Source != SourceCalendar || r.OverrideUnavailable {
			t.Fatalf("cleared：要 calendar、旗標 false，得到 %+v", r)
		}
	})

	t.Run("手動設定生效：manual、旗標 false", func(t *testing.T) {
		ov := &fakeOverrides{}
		ov.set("2026-11-23", store.MarketDayClosed)
		j, _ := newJudge(cal, ov, monday)
		if r := j.On(ctx, monday); r.Source != SourceManual || r.IsTrading || r.OverrideUnavailable {
			t.Fatalf("要 manual/false/旗標 false，得到 %+v", r)
		}
	})

	t.Run("沒有手動設定的來源、週末：旗標 false", func(t *testing.T) {
		j, _ := newJudge(cal, nil, monday)
		if r := j.On(ctx, monday); r.OverrideUnavailable {
			t.Fatalf("overrides 是 nil（視為沒有手動設定）：旗標要是 false，得到 %+v", r)
		}
		jj, _ := newJudge(cal, &fakeOverrides{err: errors.New("db down")}, saturday)
		if r := jj.On(ctx, saturday); r.Source != SourceWeekend || r.OverrideUnavailable {
			t.Fatalf("週末⛔ 不讀手動設定：要 weekend、旗標 false，得到 %+v", r)
		}
	})
}

// 旗標是 `json:"-"`：既有的 /market-day-overrides、/intraday/status 回應直接序列化 Result，⛔ 不能多出欄位。
func TestResultJSONOmitsOverrideUnavailable(t *testing.T) {
	b, err := json.Marshal(Result{IsTrading: true, Source: SourceCalendar, OverrideUnavailable: true})
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if got != `{"is_trading":true,"source":"calendar"}` {
		t.Fatalf("JSON 只能有既有的鍵，得到 %s", got)
	}
	if strings.Contains(strings.ToLower(got), "unavailable") {
		t.Fatalf("旗標⛔ 不能出現在 JSON：%s", got)
	}
}
