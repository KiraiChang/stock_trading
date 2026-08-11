package market

import (
	"testing"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
)

// TestYahooClientsShareRateLimiter 鎖住 2026-08-11 review 的修正：
// 盤中報價與除權息打的是同一個 Yahoo host，各自持有節流器時實際速率會是設定值的兩倍。
//
// 設定裡的 rate_limit 語意是「對 Yahoo 的總速率」，不是「每個用戶端」——
// 這件事沒有測試就只是「看起來共用」，因為兩個 client 各自能跑、也不會報錯。
func TestYahooClientsShareRateLimiter(t *testing.T) {
	quote := NewYahooQuoteClient(config.YahooConfig{RateLimit: 20})
	dividend := NewYahooDividendClient(20, zap.NewNop())

	if quote.limiter == nil || dividend.limiter == nil {
		t.Fatal("節流器不該是 nil")
	}
	if quote.limiter != dividend.limiter {
		t.Error("兩個 Yahoo 用戶端持有不同的節流器——合計速率會是設定值的兩倍")
	}
}
