package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

func TestStockSymbolRepoUpsertSnapshotMarksMissingDelisted(t *testing.T) {
	tmp, err := os.CreateTemp("", "stock-symbol-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	db, err := NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer db.Close()
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	repo := NewStockSymbolRepo(db)
	ctx := context.Background()
	day1 := time.Date(2026, 7, 20, 6, 30, 0, 0, time.UTC)
	day2 := day1.Add(24 * time.Hour)

	result, err := repo.UpsertSnapshot(ctx, []StockSymbol{
		stockSymbolForTest("1101", "TCC", "Cement"),
		stockSymbolForTest("2330", "TSMC", "Semiconductor"),
	}, day1)
	if err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}
	if result.Seen != 2 || result.Delisted != 0 {
		t.Fatalf("unexpected first result: %+v", result)
	}

	result, err = repo.UpsertSnapshot(ctx, []StockSymbol{
		stockSymbolForTest("2330", "TSMC", "Semiconductor"),
		stockSymbolForTest("00981A", "ACTIVE ETF", ""),
	}, day2)
	if err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}
	if result.Seen != 2 || result.Delisted != 1 {
		t.Fatalf("unexpected second result: %+v", result)
	}

	oldSymbol, err := repo.Get(ctx, "1101")
	if err != nil {
		t.Fatalf("get 1101 failed: %v", err)
	}
	if oldSymbol.IsListed {
		t.Fatalf("expected 1101 to be marked delisted: %+v", oldSymbol)
	}

	newSymbol, err := repo.Get(ctx, "00981A")
	if err != nil {
		t.Fatalf("get 00981A failed: %v", err)
	}
	if !newSymbol.IsListed || newSymbol.Name != "ACTIVE ETF" {
		t.Fatalf("unexpected new symbol: %+v", newSymbol)
	}

	listed, err := repo.List(ctx, true)
	if err != nil {
		t.Fatalf("list listed failed: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 listed symbols, got %d: %+v", len(listed), listed)
	}
}

func TestStockSymbolRepoUpsertSnapshotRejectsEmptySnapshot(t *testing.T) {
	tmp, err := os.CreateTemp("", "stock-symbol-empty-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	db, err := NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer db.Close()
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	repo := NewStockSymbolRepo(db)
	_, err = repo.UpsertSnapshot(context.Background(), nil, time.Now())
	if !errors.Is(err, ErrEmptyStockSymbolSnapshot) {
		t.Fatalf("expected ErrEmptyStockSymbolSnapshot, got %v", err)
	}
}

func TestStockSymbolRepoSearch(t *testing.T) {
	tmp, err := os.CreateTemp("", "stock-symbol-search-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	db, err := NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer db.Close()
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	repo := NewStockSymbolRepo(db)
	ctx := context.Background()
	seenAt := time.Date(2026, 7, 22, 6, 30, 0, 0, time.UTC)
	if _, err := repo.UpsertSnapshot(ctx, []StockSymbol{
		stockSymbolForTest("2330", "台積電", "半導體業"),
		stockSymbolForTest("2317", "鴻海", "其他電子業"),
	}, seenAt); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	rows, err := repo.Search(ctx, StockSymbolSearchOptions{Query: "台積", OnlyListed: true, Limit: 10})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(rows) != 1 || rows[0].Symbol != "2330" {
		t.Fatalf("unexpected search result: %+v", rows)
	}

	if _, err := repo.UpsertSnapshot(ctx, []StockSymbol{
		stockSymbolForTest("2317", "鴻海", "其他電子業"),
	}, seenAt.Add(24*time.Hour)); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}
	rows, err = repo.Search(ctx, StockSymbolSearchOptions{Query: "2330", OnlyListed: true, Limit: 10})
	if err != nil {
		t.Fatalf("search listed failed: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected delisted 2330 to be hidden when OnlyListed=true, got %+v", rows)
	}
	rows, err = repo.Search(ctx, StockSymbolSearchOptions{Query: "2330", OnlyListed: false, Limit: 10})
	if err != nil {
		t.Fatalf("search all failed: %v", err)
	}
	if len(rows) != 1 || rows[0].IsListed {
		t.Fatalf("expected delisted 2330 when OnlyListed=false, got %+v", rows)
	}
}

func stockSymbolForTest(symbol, name, industry string) StockSymbol {
	return StockSymbol{
		Symbol:       symbol,
		Name:         name,
		ISINCode:     "TW000" + symbol,
		Market:       "TWSE LISTED",
		SecurityType: "Stocks",
		Industry:     industry,
		CFICode:      "ESVUFR",
		ListedDate:   NullTime{NullTime: sql.NullTime{Time: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}},
	}
}

// candidateFixtureRows 是 ListCandidates 各項驗證共用的小母體。
// 抽成函式而非寫死在 candidateTestRepo 裡，是為了讓「讓某一檔下市」的測試能重報其餘標的
// ——UpsertSnapshot 有 minSnapshotListedRatio 的截斷偵測，手寫一個縮小的子集很容易誤觸。
func candidateFixtureRows() []StockSymbol {
	old := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	withDate := func(s StockSymbol, at time.Time) StockSymbol {
		s.ListedDate = NullTime{NullTime: sql.NullTime{Time: at, Valid: true}}
		return s
	}
	withType := func(s StockSymbol, kind string) StockSymbol {
		s.SecurityType = kind
		return s
	}
	noDate := func(s StockSymbol) StockSymbol {
		s.ListedDate = NullTime{}
		return s
	}

	return []StockSymbol{
		// 半導體 4 檔：驗 per-industry 等距取樣。代號遞增大致等於上市資歷遞增，
		// 取 2 檔時等距取樣應拿到第 1、3 名（2330、3034）而不是相鄰的前兩檔。
		withDate(stockSymbolForTest("2330", "TSMC", "Semiconductor"), old),
		withDate(stockSymbolForTest("2454", "MTK", "Semiconductor"), old),
		withDate(stockSymbolForTest("3034", "NOVATEK", "Semiconductor"), old),
		withDate(stockSymbolForTest("6243", "IST", "Semiconductor"), old),
		// 航運 2 檔。
		withDate(stockSymbolForTest("2603", "EMC", "Shipping"), old),
		withDate(stockSymbolForTest("2609", "YMTC", "Shipping"), old),
		// 上市未滿門檻，應被 ListedBefore 濾掉。
		withDate(stockSymbolForTest("9999", "NEWCO", "Shipping"), recent),
		// listed_date 為 NULL 且**沒有產業分類**：同時驗兩件事——證不出上市夠久要被濾掉，
		// 以及空產業不該被 per_industry 上限約束（真實資料裡 ETF 與權證都是空字串）。
		noDate(stockSymbolForTest("8888", "UNKNOWN", "")),
		// 非股票且無產業分類，驗 security_type 過濾與空產業豁免。
		withDate(withType(stockSymbolForTest("0050", "TW50", ""), "ETF"), old),
	}
}

// candidateTestRepo 準備一個帶產業／上市日／security_type 差異的小母體，
// 供 ListCandidates 的各項條件驗證使用。
func candidateTestRepo(t *testing.T) (StockSymbolRepo, context.Context) {
	t.Helper()

	tmp, err := os.CreateTemp("", "stock-symbol-candidates-*.db")
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
	ctx := context.Background()
	if err := database.RunMigrations(ctx, db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	repo := NewStockSymbolRepo(db)
	if _, err := repo.UpsertSnapshot(ctx, candidateFixtureRows(),
		time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	return repo, ctx
}

func candidateSymbols(rows []StockSymbol) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Symbol)
	}
	return out
}

// TestListCandidatesPerIndustryLimit 鎖住 T-040 的分層抽樣：半導體業有 201 檔，
// 不限制單一產業佔比的話「隨機抽 300 檔」會被它主導（見 todo.md T-040 選股方法）。
func TestListCandidatesPerIndustryLimit(t *testing.T) {
	repo, ctx := candidateTestRepo(t)

	res, err := repo.ListCandidates(ctx, StockSymbolCandidateOptions{
		SecurityTypes:    []string{"Stocks"},
		PerIndustryLimit: 2,
	})
	if err != nil {
		t.Fatalf("ListCandidates failed: %v", err)
	}

	perIndustry := map[string]int{}
	for _, r := range res.Symbols {
		if r.Industry == "" {
			continue // 沒有產業分類的列刻意豁免於上限，見下一支測試
		}
		perIndustry[r.Industry]++
	}
	for industry, n := range perIndustry {
		if n > 2 {
			t.Errorf("產業 %q 取到 %d 檔，超過 per_industry=2——分層抽樣沒生效", industry, n)
		}
	}

	// **等距取樣，不是取代號最小的前 N 檔**：台股代號大致依上市資歷編配，取前 N 會讓
	// 每個產業永遠只拿到最老最大的那幾檔，Step 1 量到的 ATR% 分佈會整個偏向低波動端。
	// 半導體 4 檔（2330/2454/3034/6243）取 2 檔時，等距取樣拿到的是第 1、3 名＝2330、3034。
	got := candidateSymbols(res.Symbols)
	for _, want := range []string{"2330", "3034"} {
		if !slices.Contains(got, want) {
			t.Errorf("缺少 %s——不是等距取樣（取前 N 會拿到 2330、2454）：%v", want, got)
		}
	}
	if slices.Contains(got, "2454") {
		t.Errorf("取到 2454——等距取樣不該拿相鄰的前兩檔，那正是資歷偏斜的來源：%v", got)
	}
}

// TestListCandidatesPerIndustryLimitKeepsUnclassified 鎖住一個會靜靜砍掉 97% ETF 的 bug：
// industry 是 NOT NULL DEFAULT ''，ETF 與權證全部落在空字串。若把空字串當成一個產業，
// per_industry=9 會讓 354 檔 ETF 只剩 9 檔——而 ETF 是目前唯一填得進 LOW bucket 的類型。
func TestListCandidatesPerIndustryLimitKeepsUnclassified(t *testing.T) {
	repo, ctx := candidateTestRepo(t)

	res, err := repo.ListCandidates(ctx, StockSymbolCandidateOptions{PerIndustryLimit: 1})
	if err != nil {
		t.Fatalf("ListCandidates failed: %v", err)
	}

	got := candidateSymbols(res.Symbols)
	// fixture 裡沒有產業分類的是 0050（ETF）與 8888（listed_date 為 NULL 那筆刻意也留空）。
	unclassified := 0
	for _, r := range res.Symbols {
		if r.Industry == "" {
			unclassified++
		}
	}
	if unclassified < 2 {
		t.Errorf("沒有產業分類的標的只剩 %d 檔——空字串被當成單一產業套用上限了：%v", unclassified, got)
	}
	if !slices.Contains(got, "0050") {
		t.Errorf("ETF 0050 被 per_industry 砍掉了：%v", got)
	}
}

// TestListCandidatesTruncatedFlag：呼叫端光看筆數分不出「母體剛好等於上限」與「被砍掉」，
// 而截斷是依代號順序、會整批砍掉高代號的產業。
func TestListCandidatesTruncatedFlag(t *testing.T) {
	repo, ctx := candidateTestRepo(t)

	full, err := repo.ListCandidates(ctx, StockSymbolCandidateOptions{})
	if err != nil {
		t.Fatalf("ListCandidates failed: %v", err)
	}
	if full.Truncated {
		t.Errorf("沒有超過上限卻回報 truncated=true（%d 筆）", len(full.Symbols))
	}

	cut, err := repo.ListCandidates(ctx, StockSymbolCandidateOptions{Limit: 2})
	if err != nil {
		t.Fatalf("ListCandidates failed: %v", err)
	}
	if len(cut.Symbols) != 2 {
		t.Errorf("limit=2 應回 2 筆，實得 %d 筆", len(cut.Symbols))
	}
	if !cut.Truncated {
		t.Error("被 limit 砍掉卻回報 truncated=false——呼叫端會以為拿到完整清單")
	}
}

// TestListCandidatesListedBeforeExcludesNullListedDate 鎖住一個容易寫錯的地方：
// listed_date 為 NULL 時「證不出上市夠久」，必須排除而不是放行。
func TestListCandidatesListedBeforeExcludesNullListedDate(t *testing.T) {
	repo, ctx := candidateTestRepo(t)

	res, err := repo.ListCandidates(ctx, StockSymbolCandidateOptions{
		ListedBefore: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("ListCandidates failed: %v", err)
	}

	got := candidateSymbols(res.Symbols)
	if slices.Contains(got, "8888") {
		t.Errorf("listed_date 為 NULL 的 8888 竟然入選——證不出上市夠久就不該進研究母體：%v", got)
	}
	if slices.Contains(got, "9999") {
		t.Errorf("上市日晚於門檻的 9999 竟然入選：%v", got)
	}
	if !slices.Contains(got, "2330") {
		t.Errorf("上市夠久的 2330 反而被濾掉：%v", got)
	}
}

// TestListCandidatesFiltersAndDelisted 驗 security_type 過濾與「預設不含下市」。
func TestListCandidatesFiltersAndDelisted(t *testing.T) {
	repo, ctx := candidateTestRepo(t)

	etf, err := repo.ListCandidates(ctx, StockSymbolCandidateOptions{SecurityTypes: []string{"ETF"}})
	if err != nil {
		t.Fatalf("ListCandidates failed: %v", err)
	}
	if got := candidateSymbols(etf.Symbols); len(got) != 1 || got[0] != "0050" {
		t.Errorf("security_type=ETF 應只回 0050，實得 %v", got)
	}

	// 讓 9999 下市：下一次快照只少掉它、其餘全部重報。
	//
	// **刻意重報所有其他標的**，而不是只送一個縮小的子集：UpsertSnapshot 有
	// minSnapshotListedRatio = 0.5 的截斷偵測，快照涵蓋數低於現有上市數的一半就整批放棄。
	// fixture 有 9 筆，門檻是 4.5——只送 5 筆的話餘裕只剩一列，日後有人為了測其他條件
	// 往 fixture 多加兩筆，這裡就會以 "second snapshot failed" 失敗，而錯誤訊息完全
	// 看不出是比例護欄造成的。
	fixture := candidateFixtureRows()
	remaining := make([]StockSymbol, 0, len(fixture)-1)
	for _, row := range fixture {
		if row.Symbol != "9999" {
			remaining = append(remaining, row)
		}
	}
	if _, err := repo.UpsertSnapshot(ctx, remaining, time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("second snapshot failed: %v", err)
	}

	res, err := repo.ListCandidates(ctx, StockSymbolCandidateOptions{})
	if err != nil {
		t.Fatalf("ListCandidates failed: %v", err)
	}
	if got := candidateSymbols(res.Symbols); slices.Contains(got, "9999") {
		t.Errorf("已下市的 9999 出現在預設結果中——研究母體不該混入下市標的：%v", got)
	}

	all, err := repo.ListCandidates(ctx, StockSymbolCandidateOptions{IncludeDelisted: true})
	if err != nil {
		t.Fatalf("ListCandidates failed: %v", err)
	}
	if got := candidateSymbols(all.Symbols); !slices.Contains(got, "9999") {
		t.Errorf("IncludeDelisted=true 仍看不到 9999：%v", got)
	}
}
