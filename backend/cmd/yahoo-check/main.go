// yahoo-check 是驗證 Yahoo 股市盤中資料源（非官方 API）欄位、覆蓋率與延遲用的
// 獨立工具，不掛載到主服務。
//
// 用法（於 backend/ 目錄下執行，沿用與 cmd/server 相同的 config.yaml）：
//
//	go run ./cmd/yahoo-check -symbols 2330,0050
//
// 會呼叫一次批次端點，對每檔印出：解析後的 1 分K 數量、涵蓋的時間範圍、
// 最新一根 K 棒與現在的時間差，供人工在盤中時段比對覆蓋率（特別是 ETF 的
// null 陣列問題）與延遲。詳見 docs/yahoo-intraday-integration.md。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/market"
)

func main() {
	symbolsFlag := flag.String("symbols", "2330,0050", "要測試的股票代碼，逗號分隔（系統格式，如 2330；上櫃可帶 .TWO）")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config load failed:", err)
		os.Exit(1)
	}

	symbols := splitSymbols(*symbolsFlag)
	if len(symbols) == 0 {
		fmt.Fprintln(os.Stderr, "請以 -symbols 指定至少一檔股票")
		os.Exit(1)
	}

	client := market.NewYahooQuoteClient(cfg.Yahoo)
	fmt.Printf("===== Yahoo 盤中批次 1 分K（symbols=%v，rate_limit=%d/min，batch_size=%d） =====\n",
		symbols, client.RateLimit(), client.BatchSize())

	ctx := context.Background()
	t0 := time.Now()
	bySymbol, err := client.FetchIntradayCandlesBatch(ctx, symbols)
	elapsed := time.Since(t0)
	if err != nil {
		fmt.Println("fetch failed:", err)
		os.Exit(1)
	}
	fmt.Printf("本地耗時: %s\n\n", elapsed)

	for _, sym := range symbols {
		candles := bySymbol[sym]
		if len(candles) == 0 {
			fmt.Printf("[%s] 0 根 K 棒（非交易時間、盤後 null 陣列、或代碼/尾碼不正確）\n", sym)
			continue
		}
		first := candles[0]
		last := candles[len(candles)-1]
		fmt.Printf(
			"[%s] %d 根 K 棒\n  時間範圍: %s ~ %s\n  最新一根（距現在 %s）: O=%.2f H=%.2f L=%.2f C=%.2f V=%d\n",
			sym, len(candles),
			first.Timestamp.Format("15:04"), last.Timestamp.Format("15:04"),
			time.Since(last.Timestamp).Truncate(time.Second),
			last.Open, last.High, last.Low, last.Close, last.Volume,
		)
	}
	fmt.Println("\n驗證結束")
}

func splitSymbols(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
