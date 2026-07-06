package store

import (
	"context"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

func newTestBrokerTradeRepo(t *testing.T) BrokerTradeRepo {
	t.Helper()
	tmp, err := os.CreateTemp("", "broker-trade-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })

	db, err := NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}
	return NewBrokerTradeRepo(db)
}

func TestBrokerTradeBulkUpsertIdempotentAndSortedByNetBuy(t *testing.T) {
	repo := newTestBrokerTradeRepo(t)
	ctx := context.Background()
	date := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)

	trades := []BrokerTrade{
		{Symbol: "2330", TradeDate: date, BrokerName: "凱基", BranchName: "台北", BuyVolume: 5000, SellVolume: 1000, NetBuy: 4000},
		{Symbol: "2330", TradeDate: date, BrokerName: "元大", BranchName: "台中", BuyVolume: 1000, SellVolume: 6000, NetBuy: -5000},
		{Symbol: "2330", TradeDate: date, BrokerName: "富邦", BranchName: "高雄", BuyVolume: 8000, SellVolume: 2000, NetBuy: 6000},
	}
	if err := repo.BulkUpsert(ctx, trades); err != nil {
		t.Fatalf("bulk upsert failed: %v", err)
	}

	// 同一天重跑（symbol+trade_date+broker_name+branch_name 相同）應覆蓋而非新增
	update := trades[0]
	update.NetBuy = 9999
	if err := repo.BulkUpsert(ctx, []BrokerTrade{update}); err != nil {
		t.Fatalf("bulk upsert (rerun) failed: %v", err)
	}

	rows, err := repo.GetByDate(ctx, "2330", date)
	if err != nil {
		t.Fatalf("get by date failed: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows (rerun should upsert, not duplicate), got %d", len(rows))
	}
	// 依 net_buy DESC 排序：9999(凱基) > 6000(富邦) > -5000(元大)
	if rows[0].BrokerName != "凱基" || rows[0].NetBuy != 9999 {
		t.Fatalf("expected first row to be 凱基 with net_buy=9999, got %+v", rows[0])
	}
	if rows[len(rows)-1].BrokerName != "元大" {
		t.Fatalf("expected last row (smallest net_buy) to be 元大, got %+v", rows[len(rows)-1])
	}
}
