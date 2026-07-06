package chip

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
)

// fakeChipDataSource 是手寫的 market.ChipDataSource 假實作（不用 mock 框架，
// 比照 store 層測試的風格），可控制回傳資料與錯誤，用來驗證 Syncer 的行為。
type fakeChipDataSource struct {
	institutional      []market.InstitutionalTrade
	institutionalErr   error
	margin             []market.MarginTrade
	marginErr          error
	brokerErr          error
	institutionalCalls int
	marginCalls        int
}

func (f *fakeChipDataSource) FetchInstitutionalTrades(ctx context.Context, symbol string, start, end time.Time) ([]market.InstitutionalTrade, error) {
	f.institutionalCalls++
	if f.institutionalErr != nil {
		return nil, f.institutionalErr
	}
	return f.institutional, nil
}

func (f *fakeChipDataSource) FetchMarginTrades(ctx context.Context, symbol string, start, end time.Time) ([]market.MarginTrade, error) {
	f.marginCalls++
	if f.marginErr != nil {
		return nil, f.marginErr
	}
	return f.margin, nil
}

func (f *fakeChipDataSource) FetchBrokerTrades(ctx context.Context, symbol string, date time.Time) ([]market.BrokerTrade, error) {
	if f.brokerErr != nil {
		return nil, f.brokerErr
	}
	return nil, market.ErrBrokerDataUnsupported
}

type syncerTestEnv struct {
	syncer            *Syncer
	institutionalRepo store.InstitutionalTradeRepo
	scoreRepo         store.ChipScoreRepo
	candleRepo        store.CandleRepo
}

func newSyncerTestEnv(t *testing.T, source market.ChipDataSource) *syncerTestEnv {
	t.Helper()
	tmp, err := os.CreateTemp("", "chip-sync-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })

	db, err := store.NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	candleRepo := store.NewCandleRepo(db)
	syncer := NewSyncer(
		source,
		store.NewInstitutionalTradeRepo(db),
		store.NewMarginTradeRepo(db),
		store.NewBrokerTradeRepo(db),
		store.NewChipScoreRepo(db),
		candleRepo,
		zap.NewNop(),
	)
	return &syncerTestEnv{
		syncer:            syncer,
		institutionalRepo: store.NewInstitutionalTradeRepo(db),
		scoreRepo:         store.NewChipScoreRepo(db),
		candleRepo:        candleRepo,
	}
}

func seedCandles(t *testing.T, repo store.CandleRepo, symbol string, date time.Time) {
	t.Helper()
	candles := make([]store.Candle, 0, 21)
	for i := 20; i >= 0; i-- {
		d := date.AddDate(0, 0, -i)
		candles = append(candles, store.Candle{
			Symbol: symbol, Timeframe: "1d",
			Open: 100, High: 105, Low: 95, Close: 100 + float64(20-i),
			Volume: 1_000_000, Timestamp: d,
		})
	}
	if err := repo.BulkInsert(context.Background(), candles); err != nil {
		t.Fatalf("seed candles failed: %v", err)
	}
}

func TestSyncDaily_BrokerUnsupportedFallsBackWithoutFailing(t *testing.T) {
	date := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)
	source := &fakeChipDataSource{
		institutional: []market.InstitutionalTrade{
			{Symbol: "2330", Date: date, ForeignNetBuy: 10000, TotalNetBuy: 10000},
		},
		margin: []market.MarginTrade{
			{Symbol: "2330", Date: date, MarginBalance: 1000, MarginChange: 100},
		},
	}
	env := newSyncerTestEnv(t, source)
	seedCandles(t, env.candleRepo, "2330", date)

	if err := env.syncer.SyncDaily(context.Background(), "2330", date); err != nil {
		t.Fatalf("expected SyncDaily to succeed despite broker being unsupported, got: %v", err)
	}

	score, err := env.scoreRepo.GetByDate(context.Background(), "2330", date)
	if err != nil {
		t.Fatalf("expected a chip_score row to be stored, got error: %v", err)
	}
	if score.BrokerScore != 0 {
		t.Errorf("expected broker_score=0 fallback, got %v", score.BrokerScore)
	}
}

func TestSyncDaily_IdempotentOnRerun(t *testing.T) {
	date := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)
	source := &fakeChipDataSource{
		institutional: []market.InstitutionalTrade{
			{Symbol: "2330", Date: date, ForeignNetBuy: 5000, TotalNetBuy: 5000},
		},
	}
	env := newSyncerTestEnv(t, source)
	seedCandles(t, env.candleRepo, "2330", date)

	ctx := context.Background()
	if err := env.syncer.SyncDaily(ctx, "2330", date); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}
	if err := env.syncer.SyncDaily(ctx, "2330", date); err != nil {
		t.Fatalf("second sync (rerun) failed: %v", err)
	}

	rows, err := env.institutionalRepo.GetRange(ctx, "2330", date, date)
	if err != nil {
		t.Fatalf("get range failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected rerun to upsert (not duplicate) institutional_trades, got %d rows", len(rows))
	}
}

func TestSyncDaily_InstitutionalFetchErrorPropagates(t *testing.T) {
	date := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)
	wantErr := errors.New("finmind: boom")
	source := &fakeChipDataSource{institutionalErr: wantErr}
	env := newSyncerTestEnv(t, source)

	err := env.syncer.SyncDaily(context.Background(), "2330", date)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error to propagate, got: %v", err)
	}
}

func TestSyncDaily_NoDataForDateSkipsChipScoreWrite(t *testing.T) {
	// 【review 修復】完全沒有 candle/法人/融資融券資料的日期（例如週末、
	// 國定假日）不應該寫入 chip_scores，避免留下一筆「借用」前一交易日
	// 資料、卻掛在非交易日 trade_date 上的紀錄。
	date := time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC) // 2026-07-04 為週六
	source := &fakeChipDataSource{}                     // institutional/margin 皆回傳空陣列，broker unsupported
	env := newSyncerTestEnv(t, source)
	// 刻意不 seedCandles：模擬非交易日完全沒有任何原始資料落地

	if err := env.syncer.SyncDaily(context.Background(), "2330", date); err != nil {
		t.Fatalf("expected SyncDaily to succeed (no-op) when there's no data at all, got: %v", err)
	}

	if _, err := env.scoreRepo.GetByDate(context.Background(), "2330", date); err == nil {
		t.Fatal("expected no chip_scores row to be written for a date with zero underlying data")
	}
}

func TestSyncRange_WeekendWithoutDataIsSkippedButWeekdayIsScored(t *testing.T) {
	friday := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)
	saturday := friday.AddDate(0, 0, 1)
	source := &fakeChipDataSource{
		institutional: []market.InstitutionalTrade{
			{Symbol: "2330", Date: friday, ForeignNetBuy: 1000, TotalNetBuy: 1000},
		},
	}
	env := newSyncerTestEnv(t, source)
	seedCandles(t, env.candleRepo, "2330", friday) // 只有週五有 K 線，週六完全沒有任何資料

	env.syncer.SyncRange(context.Background(), []string{"2330"}, friday, saturday, nil, func(_ string, err error) {
		if err != nil {
			t.Fatalf("unexpected sync error: %v", err)
		}
	})

	if _, err := env.scoreRepo.GetByDate(context.Background(), "2330", friday); err != nil {
		t.Fatalf("expected a chip_score row for the trading day (Friday), got error: %v", err)
	}
	if _, err := env.scoreRepo.GetByDate(context.Background(), "2330", saturday); err == nil {
		t.Fatal("expected no chip_score row for the non-trading day (Saturday) with zero underlying data")
	}
}

func TestSyncRange_DataTypesFilterSkipsUnrequestedFetches(t *testing.T) {
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	source := &fakeChipDataSource{
		institutional: []market.InstitutionalTrade{{Symbol: "2330", Date: from, TotalNetBuy: 1000}},
	}
	env := newSyncerTestEnv(t, source)

	var progressErrs []error
	env.syncer.SyncRange(context.Background(), []string{"2330"}, from, to, []string{"institutional"}, func(_ string, err error) {
		progressErrs = append(progressErrs, err)
	})

	if source.institutionalCalls != 1 {
		t.Errorf("expected institutional fetch to be called once, got %d", source.institutionalCalls)
	}
	if source.marginCalls != 0 {
		t.Errorf("expected margin fetch to be skipped (not in dataTypes), got %d calls", source.marginCalls)
	}
	if len(progressErrs) != 1 || progressErrs[0] != nil {
		t.Errorf("expected a single successful progress callback, got %v", progressErrs)
	}
}
