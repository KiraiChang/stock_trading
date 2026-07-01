package store

import (
	"database/sql"
	"time"
)

type Candle struct {
	ID        uint64    `db:"id"        json:"id"`
	Symbol    string    `db:"symbol"    json:"symbol"`
	Timeframe string    `db:"timeframe" json:"timeframe"`
	Open      float64   `db:"open"      json:"open"`
	High      float64   `db:"high"      json:"high"`
	Low       float64   `db:"low"       json:"low"`
	Close     float64   `db:"close"     json:"close"`
	Volume    int64     `db:"volume"    json:"volume"`
	Amount    float64   `db:"amount"    json:"amount"`
	Timestamp time.Time `db:"ts"        json:"ts"`
}

type IndicatorSnapshot struct {
	ID         uint64    `db:"id"          json:"id"`
	Symbol     string    `db:"symbol"      json:"symbol"`
	Timeframe  string    `db:"timeframe"   json:"timeframe"`
	Timestamp  time.Time `db:"ts"          json:"ts"`
	MA5        float64   `db:"ma5"         json:"ma5"`
	MA10       float64   `db:"ma10"        json:"ma10"`
	MA20       float64   `db:"ma20"        json:"ma20"`
	MA60       float64   `db:"ma60"        json:"ma60"`
	RSI14      float64   `db:"rsi14"       json:"rsi14"`
	MACD       float64   `db:"macd"        json:"macd"`
	MACDSignal float64   `db:"macd_signal" json:"macd_signal"`
	MACDHist   float64   `db:"macd_hist"   json:"macd_hist"`
	BBUpper    float64   `db:"bb_upper"    json:"bb_upper"`
	BBMiddle   float64   `db:"bb_middle"   json:"bb_middle"`
	BBLower    float64   `db:"bb_lower"    json:"bb_lower"`
	ATR14      float64   `db:"atr14"       json:"atr14"`
	VWAP       float64   `db:"vwap"        json:"vwap"`
	VolMA20    int64     `db:"vol_ma20"    json:"vol_ma20"`
	VolRatio   float64   `db:"vol_ratio"   json:"vol_ratio"`
}

type Signal struct {
	ID         uint64    `db:"id"          json:"id"`
	Symbol     string    `db:"symbol"      json:"symbol"`
	SignalType string    `db:"signal_type" json:"signal_type"`
	Direction  string    `db:"direction"   json:"direction"`
	Price      float64   `db:"price"       json:"price"`
	Volume     int64     `db:"volume"      json:"volume"`
	VolRatio   float64   `db:"vol_ratio"   json:"vol_ratio"`
	Resistance float64   `db:"resistance"  json:"resistance"`
	Support    float64   `db:"support"     json:"support"`
	Trend      string    `db:"trend"       json:"trend"`
	Note       string    `db:"note"        json:"note"`
	Timestamp  time.Time `db:"ts"          json:"ts"`
}

type JobRun struct {
	ID            uint64       `db:"id"             json:"id"`
	JobName       string       `db:"job_name"       json:"job_name"`
	Status        string       `db:"status"         json:"status"` // running/success/partial/failed
	SymbolsTotal  int          `db:"symbols_total"  json:"symbols_total"`
	SymbolsFailed int          `db:"symbols_failed" json:"symbols_failed"`
	Error         string       `db:"error"          json:"error,omitempty"`
	StartedAt     time.Time    `db:"started_at"     json:"started_at"`
	FinishedAt    sql.NullTime `db:"finished_at"    json:"finished_at,omitempty"`
}

type WatchlistItem struct {
	ID     uint32 `db:"id"     json:"id"`
	Symbol string `db:"symbol" json:"symbol"`
	Name   string `db:"name"   json:"name"`
	Sector string `db:"sector" json:"sector"`
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
