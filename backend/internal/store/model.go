package store

import (
	"database/sql"
	"time"
)

type Candle struct {
	ID        uint64    `db:"id"`
	Symbol    string    `db:"symbol"`
	Timeframe string    `db:"timeframe"`
	Open      float64   `db:"open"`
	High      float64   `db:"high"`
	Low       float64   `db:"low"`
	Close     float64   `db:"close"`
	Volume    int64     `db:"volume"`
	Amount    float64   `db:"amount"`
	Timestamp time.Time `db:"ts"`
}

type IndicatorSnapshot struct {
	ID         uint64    `db:"id"`
	Symbol     string    `db:"symbol"`
	Timeframe  string    `db:"timeframe"`
	Timestamp  time.Time `db:"ts"`
	MA5        float64   `db:"ma5"`
	MA10       float64   `db:"ma10"`
	MA20       float64   `db:"ma20"`
	MA60       float64   `db:"ma60"`
	RSI14      float64   `db:"rsi14"`
	MACD       float64   `db:"macd"`
	MACDSignal float64   `db:"macd_signal"`
	MACDHist   float64   `db:"macd_hist"`
	BBUpper    float64   `db:"bb_upper"`
	BBMiddle   float64   `db:"bb_middle"`
	BBLower    float64   `db:"bb_lower"`
	ATR14      float64   `db:"atr14"`
	VWAP       float64   `db:"vwap"`
	VolMA20    int64     `db:"vol_ma20"`
	VolRatio   float64   `db:"vol_ratio"`
}

type Signal struct {
	ID         uint64    `db:"id"`
	Symbol     string    `db:"symbol"`
	SignalType  string    `db:"signal_type"`
	Direction  string    `db:"direction"`
	Price      float64   `db:"price"`
	Volume     int64     `db:"volume"`
	VolRatio   float64   `db:"vol_ratio"`
	Resistance float64   `db:"resistance"`
	Support    float64   `db:"support"`
	Trend      string    `db:"trend"`
	Note       string    `db:"note"`
	Timestamp  time.Time `db:"ts"`
}

type WatchlistItem struct {
	ID     uint32 `db:"id"`
	Symbol string `db:"symbol"`
	Name   string `db:"name"`
	Sector string `db:"sector"`
}

// ── Backtest models ───────────────────────────────────────────

type BacktestJob struct {
	ID         uint64       `db:"id"          json:"id"`
	JobID      string       `db:"job_id"      json:"job_id"`
	Type       string       `db:"type"        json:"type"`
	Strategy   string       `db:"strategy"    json:"strategy"`
	Symbols    string       `db:"symbols"     json:"symbols"`    // JSON array string
	Timeframe  string       `db:"timeframe"   json:"timeframe"`
	StartDate  string       `db:"start_date"  json:"start_date"`
	EndDate    string       `db:"end_date"    json:"end_date"`
	Status     string       `db:"status"      json:"status"`     // pending/running/done/failed
	Trigger    string       `db:"trigger"     json:"trigger"`    // manual/scheduler
	Error      string       `db:"error"       json:"error,omitempty"`
	CreatedAt  time.Time    `db:"created_at"  json:"created_at"`
	StartedAt  sql.NullTime `db:"started_at"  json:"started_at,omitempty"`
	FinishedAt sql.NullTime `db:"finished_at" json:"finished_at,omitempty"`
}

type BacktestResult struct {
	ID           uint64    `db:"id"            json:"id"`
	JobID        string    `db:"job_id"        json:"job_id"`
	Strategy     string    `db:"strategy"      json:"strategy"`
	TotalReturn  float64   `db:"total_return"  json:"total_return"`
	AnnualReturn float64   `db:"annual_return" json:"annual_return"`
	WinRate      float64   `db:"win_rate"      json:"win_rate"`
	MaxDrawdown  float64   `db:"max_drawdown"  json:"max_drawdown"`
	SharpeRatio  float64   `db:"sharpe_ratio"  json:"sharpe_ratio"`
	TotalTrades  int       `db:"total_trades"  json:"total_trades"`
	WinTrades    int       `db:"win_trades"    json:"win_trades"`
	LossTrades   int       `db:"loss_trades"   json:"loss_trades"`
	AvgPnL       float64   `db:"avg_pnl"       json:"avg_pnl"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
}

type BacktestTrade struct {
	ID         uint64       `db:"id"          json:"id"`
	JobID      string       `db:"job_id"      json:"job_id"`
	Symbol     string       `db:"symbol"      json:"symbol"`
	Direction  string       `db:"direction"   json:"direction"`
	EntryTime  sql.NullTime `db:"entry_time"  json:"entry_time,omitempty"`
	ExitTime   sql.NullTime `db:"exit_time"   json:"exit_time,omitempty"`
	EntryPrice float64      `db:"entry_price" json:"entry_price"`
	ExitPrice  float64      `db:"exit_price"  json:"exit_price"`
	Size       float64      `db:"size"        json:"size"`
	PnL        float64      `db:"pnl"         json:"pnl"`
	PnLPct     float64      `db:"pnl_pct"     json:"pnl_pct"`
	Commission float64      `db:"commission"  json:"commission"`
	CreatedAt  time.Time    `db:"created_at"  json:"created_at"`
}
