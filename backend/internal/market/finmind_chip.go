package market

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/trading/backend/pkg/timeutil"
)

// marginLotToShares：TaiwanStockMarginPurchaseShortSale 的餘額/限額欄位單位是
// 「張」（=仟股），實測 2330／2317 的 MarginPurchaseLimit 換算後恰等於約當
// 流通股數的 25%（法定融資限額比例），確認整個 dataset 一致以「張」為單位。
// 內部一律用「股」，進入 store 前必須乘以 1000 換算（見
// docs/chip-analysis-design.md 第2節單位規則）。
const marginLotToShares = 1000

// RawInstitutionalRow 為 TaiwanStockInstitutionalInvestorsBuySell 單一列原始
// 資料，buy/sell 單位為股（已用真實 API 呼叫驗證，非「張」，跟
// TaiwanStockMarginPurchaseShortSale 不同）。Name 依身份分列：
// Foreign_Investor / Foreign_Dealer_Self / Investment_Trust / Dealer_self /
// Dealer_Hedging。
type RawInstitutionalRow struct {
	Date string `json:"date"`
	Name string `json:"name"`
	Buy  int64  `json:"buy"`
	Sell int64  `json:"sell"`
}

// RawMarginRow 為 TaiwanStockMarginPurchaseShortSale 單一列原始資料，
// 餘額/限額欄位單位為「張」（見 marginLotToShares 說明）。
type RawMarginRow struct {
	Date                           string `json:"date"`
	MarginPurchaseTodayBalance     int64  `json:"MarginPurchaseTodayBalance"`
	MarginPurchaseYesterdayBalance int64  `json:"MarginPurchaseYesterdayBalance"`
	MarginPurchaseLimit            int64  `json:"MarginPurchaseLimit"`
	ShortSaleTodayBalance          int64  `json:"ShortSaleTodayBalance"`
	ShortSaleYesterdayBalance      int64  `json:"ShortSaleYesterdayBalance"`
	ShortSaleLimit                 int64  `json:"ShortSaleLimit"`
}

// FetchInstitutionalTrades 拉取三大法人買賣超（dataset=
// TaiwanStockInstitutionalInvestorsBuySell），依日期分組聚合五個身份別：
// 外資=Foreign_Investor+Foreign_Dealer_Self，投信=Investment_Trust，
// 自營商=Dealer_self+Dealer_Hedging。
func (c *FinMindClient) FetchInstitutionalTrades(ctx context.Context, symbol string, start, end time.Time) ([]InstitutionalTrade, error) {
	params := url.Values{
		"dataset":    {"TaiwanStockInstitutionalInvestorsBuySell"},
		"data_id":    {symbol},
		"start_date": {start.Format("2006-01-02")},
		"end_date":   {end.Format("2006-01-02")},
	}

	rawRows, err := c.fetch(ctx, params)
	if err != nil {
		return nil, err
	}

	type accumulator struct {
		date   time.Time
		values InstitutionalTrade
	}
	byDate := make(map[string]*accumulator)
	order := make([]string, 0)

	for _, row := range rawRows {
		var raw RawInstitutionalRow
		if err := json.Unmarshal(row, &raw); err != nil {
			continue
		}
		ts, err := time.ParseInLocation("2006-01-02", raw.Date, timeutil.TaipeiTZ)
		if err != nil {
			continue
		}

		acc, ok := byDate[raw.Date]
		if !ok {
			acc = &accumulator{date: ts, values: InstitutionalTrade{Symbol: symbol, Date: ts}}
			byDate[raw.Date] = acc
			order = append(order, raw.Date)
		}

		net := raw.Buy - raw.Sell
		switch raw.Name {
		case "Foreign_Investor", "Foreign_Dealer_Self":
			acc.values.ForeignNetBuy += net
		case "Investment_Trust":
			acc.values.InvestmentTrustNetBuy += net
		case "Dealer_self", "Dealer_Hedging":
			acc.values.DealerNetBuy += net
		}
		acc.values.TotalNetBuy += net
	}

	trades := make([]InstitutionalTrade, 0, len(order))
	for _, d := range order {
		trades = append(trades, byDate[d].values)
	}
	return trades, nil
}

// FetchMarginTrades 拉取融資融券（dataset=TaiwanStockMarginPurchaseShortSale）。
// MarginChange/ShortChange 用當日與前一日餘額差值計算（比單純加總買賣還原
// 更穩定，現金償還等情況下兩種算法會不同）。餘額/限額欄位需乘以
// marginLotToShares 換算成股；UsageRate 為餘額/限額的比例，換算前後單位
// 一致所以不需再乘 1000，限額為 0（不太可能發生，防禦性處理）時回傳 nil。
func (c *FinMindClient) FetchMarginTrades(ctx context.Context, symbol string, start, end time.Time) ([]MarginTrade, error) {
	params := url.Values{
		"dataset":    {"TaiwanStockMarginPurchaseShortSale"},
		"data_id":    {symbol},
		"start_date": {start.Format("2006-01-02")},
		"end_date":   {end.Format("2006-01-02")},
	}

	rawRows, err := c.fetch(ctx, params)
	if err != nil {
		return nil, err
	}

	trades := make([]MarginTrade, 0, len(rawRows))
	for _, row := range rawRows {
		var raw RawMarginRow
		if err := json.Unmarshal(row, &raw); err != nil {
			continue
		}
		ts, err := time.ParseInLocation("2006-01-02", raw.Date, timeutil.TaipeiTZ)
		if err != nil {
			continue
		}

		var marginUsageRate, shortUsageRate *float64
		if raw.MarginPurchaseLimit > 0 {
			rate := float64(raw.MarginPurchaseTodayBalance) / float64(raw.MarginPurchaseLimit)
			marginUsageRate = &rate
		}
		if raw.ShortSaleLimit > 0 {
			rate := float64(raw.ShortSaleTodayBalance) / float64(raw.ShortSaleLimit)
			shortUsageRate = &rate
		}

		trades = append(trades, MarginTrade{
			Symbol:          symbol,
			Date:            ts,
			MarginBalance:   raw.MarginPurchaseTodayBalance * marginLotToShares,
			MarginChange:    (raw.MarginPurchaseTodayBalance - raw.MarginPurchaseYesterdayBalance) * marginLotToShares,
			ShortBalance:    raw.ShortSaleTodayBalance * marginLotToShares,
			ShortChange:     (raw.ShortSaleTodayBalance - raw.ShortSaleYesterdayBalance) * marginLotToShares,
			MarginUsageRate: marginUsageRate,
			ShortUsageRate:  shortUsageRate,
		})
	}
	return trades, nil
}

// FetchBrokerTrades 目前是 stub：FinMind 沒有提供個股「券商分點進出」的
// dataset（三大法人與融資融券皆有標準 dataset，分點資料官方上須另外爬
// TWSE 分點統計頁或串接付費第三方）。呼叫端（chip.Syncer）應把
// ErrBrokerDataUnsupported 視為「此資料類型不支援」而非抓取失敗，
// broker_score fallback 為中性，不阻塞其他分數計算。
func (c *FinMindClient) FetchBrokerTrades(ctx context.Context, symbol string, date time.Time) ([]BrokerTrade, error) {
	return nil, ErrBrokerDataUnsupported
}
