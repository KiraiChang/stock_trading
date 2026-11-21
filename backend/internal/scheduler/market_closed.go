package scheduler

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/tradingday"
)

// 休市日的閘門（I-139）。現況規格見 docs/architecture.md 的「休市日的行為」。
//
// 只放在 cron 入口：盤中每一輪、盤前的回補與預熱、收盤的當日取數與掃描在休市日記 skipped（`market_closed (…)`）；
// 盤前清理、收盤尾端的 sr_zone_verify、會回補或核對過去幾天的排程（評估標的池、缺漏偵測、籌碼）照原排程。
// 手動端點與重跑走原本的函式，⛔ 不經過閘門。日曆取不到時照平日跑（fail-open）。

// SetTradingDay 注入跟 API 共用的交易日判斷。**在 Start() 之前**注入。
func (s *Scheduler) SetTradingDay(j *tradingday.Judge) { s.tradingDay = j }

// TradingDay 回傳共用的判斷（API handler 用同一個實例）；沒有注入時是 nil。
func (s *Scheduler) TradingDay() *tradingday.Judge { return s.tradingDay }

// marketClosed 回報 at 那一天是不是休市日，以及要寫進 job_runs.error 的原因（`market_closed (calendar)`／`(manual)`）。
// 沒有注入判斷、週末以外的日曆取不到（fallback）→ 當交易日。
func (s *Scheduler) marketClosed(ctx context.Context, at time.Time) (string, bool) {
	if s.tradingDay == nil {
		return "", false
	}
	r := s.tradingDay.On(ctx, at)
	if r.IsTrading {
		return "", false
	}
	return "market_closed (" + r.Source + ")", true
}

// recordMarketClosed 記一筆 skipped（symbols_total=0、error 只有原因，⛔ 不含其他文字）。
func (s *Scheduler) recordMarketClosed(ctx context.Context, job, reason string) {
	runID := s.startRun(ctx, job)
	s.log.Info("job skipped: market closed", zap.String("job", job), zap.String("reason", reason), zap.Uint64("run_id", runID))
	s.finishRunStatus(ctx, runID, job, "skipped", 0, 0, reason)
}

// runPreMarketCron 是 pre_market 的 cron 入口：清理照跑，休市日⛔ 不回補、⛔ 不預熱。
func (s *Scheduler) runPreMarketCron() {
	ctx := context.Background()
	s.cleanupRetention(ctx)
	if reason, closed := s.marketClosed(ctx, s.intradayNow()); closed {
		s.recordMarketClosed(ctx, "pre_market", reason)
		return
	}
	s.runPreMarketWork(ctx)
}

// runDailyCloseCron 是 daily_close 的 cron 入口：休市日⛔ 不取數、⛔ 不掃描，記 skipped 之後 sr_zone_verify 照跑。
// 手動端點（POST /scheduler/daily-close/run）照舊呼叫完整的 RunDailyClose。
func (s *Scheduler) runDailyCloseCron() {
	ctx := context.Background()
	if reason, closed := s.marketClosed(ctx, s.intradayNow()); closed {
		s.recordMarketClosed(ctx, "daily_close", reason)
		s.runDailyCloseTail(ctx)
		return
	}
	s.RunDailyClose()
}
