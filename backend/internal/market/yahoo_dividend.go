package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// yahooDividendURL 的 symbol 在 path 裡，所以**只能逐檔查詢**——這是規模上的主要限制
// （見 docs/todo.md T-042 Phase 2）。分割那邊可以一次抓全市場，除權息不行。
const yahooDividendURL = "https://tw.stock.yahoo.com/_td-stock/api/resource/" +
	"StockServices.dividendsByYear;action=combineCashAndStock;handleUpcoming=true;" +
	"sortBy=-payYear;symbol=%s.TW?device=desktop&intl=tw&lang=zh-Hant-TW&region=TW" +
	"&site=finance&tz=Asia%%2FTaipei&returnMeta=true"

// YahooDividendClient 從 Yahoo 台股頁面的內部 API 取除權息資料。
//
// **這是非官方 API**（與 yahoo-intraday-integration.md 記載的盤中源同源），
// 格式可能無預警改變、也可能被限流。因此：解析失敗只跳過該檔並記 Error，
// 絕不讓整輪中斷；重算是冪等的，下一輪會自動補上。
type YahooDividendClient struct {
	http    *http.Client
	limiter *rateLimiter
	log     *zap.Logger
	// baseURLForTest 只給測試覆寫；為空時走真實的 Yahoo 端點。
	baseURLForTest string
}

func NewYahooDividendClient(ratePerMinute int, log *zap.Logger) *YahooDividendClient {
	return &YahooDividendClient{
		http: &http.Client{Timeout: 30 * time.Second},
		// 與盤中報價共用同一個節流器：兩者打的是同一個 host，各自節流會讓實際速率加倍
		// （2026-08-11 review）。
		limiter: sharedYahooLimiter(ratePerMinute),
		log:     log,
	}
}

// rawYahooDividend 只列出實際會用到的欄位。
type rawYahooDividend struct {
	ExDate              string          `json:"exDate"`
	RecordType          string          `json:"recordType"`
	Symbol              string          `json:"symbol"`
	ExDatePreviousClose *rawYahooNumber `json:"exDatePreviousClose"`
	ExDividend          *rawYahooCash   `json:"exDividend"`
	ExRight             *rawYahooStock  `json:"exRight"`
}

type rawYahooNumber struct {
	Raw json.RawMessage `json:"raw"`
}
type rawYahooCash struct {
	Cash string `json:"cash"`
}
type rawYahooStock struct {
	Stock string `json:"stock"`
}

type yahooDividendResponse struct {
	Data struct {
		DividendByYear []rawYahooDividend `json:"dividendByYear"`
	} `json:"data"`
}

// yahooNumber 解析 Yahoo 的數值欄位。
//
// **`raw` 不保證是數字**：實測會出現字串 `"-"`（無資料），`cash` / `stock` 也可能是
// `"-"` 或空字串。直接轉型會 panic 或炸掉整輪，所以一律回 (值, 是否有效)。
func yahooNumber(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, false
	}
	return parseYahooDecimal(s)
}

func parseYahooDecimal(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// FetchDividends 取單一標的的全部除權息事件並換算成還原係數。
//
// **判斷「這是一次事件」的依據是有沒有 `exDate`，不是 `recordType`。** 這一點實測踩過：
// `SUB` 只出現在一年配息多次的標的（2330、0050）；一年只配一次的（2317、1101）
// **全部是 `YEAR`**，而那些 YEAR 記錄自己帶 exDate。用 recordType 過濾會讓市場上
// 大多數標的變成「沒有除權息事件」——而且不會有任何錯誤，就是空的。
//
// 係數：
//
//	price  = (prevClose − cash) / (1 + stock/10) / prevClose
//	volume = 1 / (1 + stock/10)      ← 現金股利不改變股數，所以純現金時為 1
//
// 已知限制：來源把同年的現金與股票股利合併成一筆（URL 的 action=combineCashAndStock），
// 兩者除權息日不同天時係數會算錯。實測 146 筆中 1 筆受影響（2891 於 2016-10-12，差 4.1%），
// 已接受這個限制——其餘純現金案例與 FinMind 精確到浮點極限。
func (c *YahooDividendClient) FetchDividends(ctx context.Context, symbol string) ([]store.CorporateAction, error) {
	if err := c.limiter.wait(ctx); err != nil {
		return nil, err
	}

	url := fmt.Sprintf(yahooDividendURL, symbol)
	if c.baseURLForTest != "" {
		url = c.baseURLForTest
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// 不帶 User-Agent 時 Yahoo 可能回非預期內容；沿用一般瀏覽器字串。
	req.Header.Set("User-Agent",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo dividends %s: HTTP %d", symbol, resp.StatusCode)
	}

	var parsed yahooDividendResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("yahoo dividends %s: %w", symbol, err)
	}

	rows := parsed.Data.DividendByYear
	actions := make([]store.CorporateAction, 0, len(rows))
	for _, row := range rows {
		// 沒有 exDate 的是「年度彙總」列（多次配息的標的才有），不是一次事件。
		if row.ExDate == "" {
			continue
		}
		prev, ok := 0.0, false
		if row.ExDatePreviousClose != nil {
			prev, ok = yahooNumber(row.ExDatePreviousClose.Raw)
		}
		// 沒有前一日收盤就算不出係數。跳過而不是當成 0——0 會讓還原價變成 0。
		if !ok || prev <= 0 {
			continue
		}
		ts, err := time.Parse(time.RFC3339, row.ExDate)
		if err != nil {
			continue
		}

		var cash, stock float64
		if row.ExDividend != nil {
			cash, _ = parseYahooDecimal(row.ExDividend.Cash)
		}
		if row.ExRight != nil {
			stock, _ = parseYahooDecimal(row.ExRight.Stock)
		}
		if cash <= 0 && stock <= 0 {
			continue
		}

		// 配股以「每 1000 股配 n 股」的元為單位表示，除以 10 得到比率。
		shareRatio := 1 + stock/10.0
		reference := (prev - cash) / shareRatio
		if reference <= 0 {
			// 現金股利大於股價是不可能的，出現就是資料有問題，跳過。
			c.log.Warn("yahoo dividend 參考價非正，略過",
				zap.String("symbol", symbol), zap.String("ex_date", row.ExDate),
				zap.Float64("prev_close", prev), zap.Float64("cash", cash), zap.Float64("stock", stock))
			continue
		}

		actionType := store.CorporateActionDividendCash
		if stock > 0 && cash > 0 {
			actionType = store.CorporateActionDividendBoth
		} else if stock > 0 {
			actionType = store.CorporateActionDividendStock
		}

		actions = append(actions, store.CorporateAction{
			Symbol:      symbol,
			EventDate:   ts.In(timeutil.TaipeiTZ),
			ActionType:  actionType,
			BeforePrice: prev,
			AfterPrice:  reference,
			Factor:      reference / prev,
			// 只有配股改變股數；純現金為 1，成交量不調整。
			VolumeFactor: 1 / shareRatio,
			Source:       store.CorporateActionSourceDividend,
		})
	}
	return actions, nil
}
