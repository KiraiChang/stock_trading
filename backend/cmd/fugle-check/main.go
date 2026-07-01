// fugle-check 是驗證 Fugle 即時行情延遲與推送訊息格式用的獨立工具，不掛載到主服務。
//
// 用法（於 backend/ 目錄下執行，沿用與 cmd/server 相同的 config.yaml）：
//
//	go run ./cmd/fugle-check -symbol 2330 -duration 60s
//
// 會依序：
//  1. 呼叫 REST /intraday/quote/{symbol}，印出回應與本地耗時
//  2. 呼叫 REST /intraday/candles/{symbol}，印出最新一根K棒時間與延遲
//  3. 連上 WebSocket，訂閱 candles channel，在指定時間內印出每一筆原始推送
//     訊息（含本地收到時間）與嘗試解析後的 Candle，供人工比對延遲、確認
//     實際 payload 欄位格式是否與 internal/market/fugle_model.go 的假設一致
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/market"
)

func main() {
	symbol := flag.String("symbol", "2330", "要測試的股票代碼")
	duration := flag.Duration("duration", 60*time.Second, "WebSocket 監聽時間")
	flag.Parse()

	log, _ := zap.NewDevelopment()
	defer log.Sync()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config load failed:", err)
		os.Exit(1)
	}
	if cfg.Fugle.APIKey == "" || cfg.Fugle.APIKey == "YOUR_FUGLE_API_KEY" {
		fmt.Fprintln(os.Stderr, "請先在 config.yaml 或環境變數 FUGLE_API_KEY 設定 Fugle API Key")
		os.Exit(1)
	}

	ctx := context.Background()
	quoteClient := market.NewFugleQuoteClient(cfg.Fugle)

	fmt.Println("===== REST /intraday/quote =====")
	t0 := time.Now()
	quote, err := quoteClient.FetchQuote(ctx, *symbol)
	elapsed := time.Since(t0)
	if err != nil {
		fmt.Println("quote fetch failed:", err)
	} else {
		raw, _ := json.MarshalIndent(quote, "", "  ")
		fmt.Printf("本地耗時: %s\n%s\n", elapsed, raw)
	}

	fmt.Println("\n===== REST /intraday/candles (timeframe=1) =====")
	t1 := time.Now()
	candles, err := quoteClient.FetchIntradayCandles(ctx, *symbol)
	elapsed = time.Since(t1)
	switch {
	case err != nil:
		fmt.Println("candles fetch failed:", err)
	case len(candles) == 0:
		fmt.Println("回傳 0 根K棒（可能非交易時間，或帳號無此資料權限）")
	default:
		last := candles[len(candles)-1]
		fmt.Printf(
			"本地耗時: %s\n最新一根K棒時間: %s（距現在 %s）\nOHLCV: open=%.2f high=%.2f low=%.2f close=%.2f volume=%d\n",
			elapsed, last.Timestamp.Format(time.RFC3339), time.Since(last.Timestamp),
			last.Open, last.High, last.Low, last.Close, last.Volume,
		)
	}

	fmt.Printf("\n===== WebSocket candles channel（監聽 %s，Ctrl+C 可提前結束） =====\n", *duration)
	streamCtx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	streamClient := market.NewFugleStreamClient(cfg.Fugle, log)
	streamClient.OnRawMessage = func(raw []byte) {
		fmt.Printf("[%s] raw: %s\n", time.Now().Format("15:04:05.000"), raw)
	}
	streamClient.Start(streamCtx)

	// Start() 內部會非同步連線+認證，這裡先等待固定秒數再送訂閱，避免連線
	// 尚未就緒時 Subscribe 因 conn 為 nil 而失敗（僅為手動驗證工具的簡化處理，
	// 正式整合到 Fetcher/Hotlist manager 時應改為等待「已認證」事件的訊號）
	time.Sleep(3 * time.Second)
	if err := streamClient.Subscribe(streamCtx, *symbol, func(c market.Candle) {
		fmt.Printf(
			"[%s] parsed candle: symbol=%s ts=%s（距現在 %s）O=%.2f H=%.2f L=%.2f C=%.2f V=%d\n",
			time.Now().Format("15:04:05.000"), c.Symbol, c.Timestamp.Format(time.RFC3339), time.Since(c.Timestamp),
			c.Open, c.High, c.Low, c.Close, c.Volume,
		)
	}); err != nil {
		fmt.Println("subscribe failed:", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	select {
	case <-streamCtx.Done():
	case <-sigCh:
	}
	_ = streamClient.Close()
	fmt.Println("\n驗證結束")
}
