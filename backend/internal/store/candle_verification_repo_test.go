package store

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

func newVerificationTestRepo(t *testing.T) CandleVerificationRepo {
	t.Helper()
	tmp, err := os.CreateTemp("", "candle-verification-*.db")
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
	return NewCandleVerificationRepo(db)
}

// 矩陣 #28：LoadStates **不得截斷**。
//
// 這是整個公平排序的地基：repo 若少回傳任何一筆既有 state，呼叫端會把它誤認成
// 「從未出現的候選」而排到最前面——排序壞掉，而且完全不會報錯。
func TestCandleVerificationLoadStatesReturnsEveryExistingRow(t *testing.T) {
	repo := newVerificationTestRepo(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)

	attempts := make([]VerificationAttempt, 0, 30)
	symbols := make([]string, 0, 40)
	for i := 0; i < 30; i++ {
		symbol := "S" + string(rune('A'+i/26)) + string(rune('A'+i%26))
		symbols = append(symbols, symbol)
		attempts = append(attempts, VerificationAttempt{
			Symbol: symbol, Timeframe: "1d",
			LastAttemptedAt: now, LastVerifiedAt: now,
			LastResult: VerificationVerified,
		})
	}
	// 另外 10 個從沒被驗過——它們**不該**出現在結果裡（沒有列 ≠ 欄位為 NULL）。
	for i := 0; i < 10; i++ {
		symbols = append(symbols, "NEW"+string(rune('A'+i)))
	}
	if err := repo.RecordAttempts(ctx, attempts); err != nil {
		t.Fatalf("record failed: %v", err)
	}

	got, err := repo.LoadStates(ctx, "1d", symbols)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(got) != 30 {
		t.Fatalf("應回傳全部 30 筆既有 state，得到 %d（截斷會讓排序靜默壞掉）", len(got))
	}
	if _, ok := got["NEWA"]; ok {
		t.Error("從未驗過的候選不該有列——它是「沒有列」不是「NULL 欄位」")
	}

	// timeframe 要有效：同一個 symbol 在別的 timeframe 不該被撈進來。
	other, err := repo.LoadStates(ctx, "5m", symbols)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(other) != 0 {
		t.Errorf("timeframe 不符不該回傳，得到 %d 筆", len(other))
	}

	// 空清單回空 map 而不是 nil——呼叫端會直接對它取值。
	empty, err := repo.LoadStates(ctx, "1d", nil)
	if err != nil {
		t.Fatalf("empty load failed: %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Errorf("空清單應回空 map，得到 %#v", empty)
	}
}

// 矩陣 #27：同一批不得有重複鍵。
//
// postgres 的 `INSERT … ON CONFLICT DO UPDATE` 不允許同一 statement 更新同一列兩次，
// 會直接讓**整批**失敗。同一個 symbol 落在兩個 aggregate 日期時就會產生重複鍵，
// 所以這裡要擋下來並說清楚是哪一檔。
func TestCandleVerificationRecordAttemptsRejectsDuplicateKeys(t *testing.T) {
	repo := newVerificationTestRepo(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)

	err := repo.RecordAttempts(ctx, []VerificationAttempt{
		{Symbol: "2330", Timeframe: "1d", LastAttemptedAt: now, LastResult: VerificationGap},
		{Symbol: "2330", Timeframe: "1d", LastAttemptedAt: now, LastResult: VerificationUnavailable},
	})
	if err == nil {
		t.Fatal("重複鍵必須被擋下——放行的話 postgres 會整批失敗")
	}
	// 錯誤訊息要指出是哪一檔，否則整批失敗時無從追查。
	if !strings.Contains(err.Error(), "2330") {
		t.Errorf("錯誤訊息要帶上 symbol，得到：%v", err)
	}

	// 不同 timeframe 的同一個 symbol 是不同鍵，不該被誤擋。
	if err := repo.RecordAttempts(ctx, []VerificationAttempt{
		{Symbol: "2330", Timeframe: "1d", LastAttemptedAt: now, LastResult: VerificationVerified},
		{Symbol: "2330", Timeframe: "5m", LastAttemptedAt: now, LastResult: VerificationVerified},
	}); err != nil {
		t.Errorf("不同 timeframe 是不同鍵，不該被擋：%v", err)
	}
}

// last_verified_at 用 COALESCE 保護：本輪沒有任何成功驗證時**不得**覆寫掉先前的成功時間。
//
// 沒有這個保護，一次失敗就會讓「上次驗成功是什麼時候」永遠遺失，
// 公平排序與陳舊判斷都會失準。
func TestCandleVerificationRecordAttemptsKeepsLastVerifiedOnFailure(t *testing.T) {
	repo := newVerificationTestRepo(t)
	ctx := context.Background()
	verifiedAt := time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)
	failedAt := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)

	if err := repo.RecordAttempts(ctx, []VerificationAttempt{{
		Symbol: "2330", Timeframe: "1d",
		LastAttemptedAt: verifiedAt, LastVerifiedAt: verifiedAt,
		LastResult: VerificationVerified,
	}}); err != nil {
		t.Fatalf("first record failed: %v", err)
	}

	// 本輪失敗：LastVerifiedAt 留零值代表「這次沒驗成」。
	if err := repo.RecordAttempts(ctx, []VerificationAttempt{{
		Symbol: "2330", Timeframe: "1d",
		LastAttemptedAt: failedAt,
		LastResult:      VerificationUnavailable, ConsecutiveFailures: 1,
	}}); err != nil {
		t.Fatalf("second record failed: %v", err)
	}

	got, err := repo.LoadStates(ctx, "1d", []string{"2330"})
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	state := got["2330"]
	if !state.LastVerifiedAt.Valid {
		t.Fatal("失敗不得把先前的 last_verified_at 抹成 NULL")
	}
	if !state.LastVerifiedAt.Time.UTC().Equal(verifiedAt) {
		t.Errorf("last_verified_at 應保留 %v，得到 %v", verifiedAt, state.LastVerifiedAt.Time)
	}
	// 其餘欄位是本輪結論，要覆寫。
	if !state.LastAttemptedAt.Time.UTC().Equal(failedAt) {
		t.Errorf("last_attempted_at 應更新為 %v，得到 %v", failedAt, state.LastAttemptedAt.Time)
	}
	if state.LastResult != VerificationUnavailable || state.ConsecutiveFailures != 1 {
		t.Errorf("本輪結論應覆寫，得到 %+v", state)
	}
}

// 首次就 deferred 的 symbol 必須寫得進去。
//
// last_result 是 NOT NULL ＋ CHECK，而 deferred 沒有舊列可保留——把它漏出值域
// 會讓「對照來源還沒發布到那一天」這種正常情況變成寫入失敗。
func TestCandleVerificationRecordAttemptsAcceptsFirstTimeDeferred(t *testing.T) {
	repo := newVerificationTestRepo(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)

	if err := repo.RecordAttempts(ctx, []VerificationAttempt{{
		Symbol: "6182", Timeframe: "1d",
		LastAttemptedAt: now, LastResult: VerificationDeferred,
	}}); err != nil {
		t.Fatalf("首次 deferred 必須寫得進去：%v", err)
	}
	got, err := repo.LoadStates(ctx, "1d", []string{"6182"})
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	state := got["6182"]
	if state.LastResult != VerificationDeferred {
		t.Errorf("last_result = %q, 期望 deferred", state.LastResult)
	}
	// deferred 不是成功驗證，last_verified_at 應維持 NULL。
	if state.LastVerifiedAt.Valid {
		t.Error("deferred 不是成功驗證，last_verified_at 不該有值")
	}
}

// CHECK constraint 要擋掉值域外的字串。
//
// 值域靠 CHECK、長度靠欄寬，兩者是不同的守門。沒有 CHECK 的話，一個拼錯的
// last_result 會被寫進去，而所有讀它的地方都會靜默把它當成「不是任何已知狀態」。
func TestCandleVerificationRejectsUnknownResult(t *testing.T) {
	repo := newVerificationTestRepo(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)

	err := repo.RecordAttempts(ctx, []VerificationAttempt{{
		Symbol: "2330", Timeframe: "1d",
		LastAttemptedAt: now, LastResult: "verifed", // 拼錯
	}})
	if err == nil {
		t.Fatal("值域外的 last_result 必須被 CHECK 擋下")
	}
}
