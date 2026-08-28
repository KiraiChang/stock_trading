package market

import (
	"testing"
	"time"
)

func TestSourceBreakerOpensAtThresholdAndIsPerSource(t *testing.T) {
	b := NewSourceBreaker(3, time.Hour)

	b.Fail(SourceTWSEStockDay)
	b.Fail(SourceTWSEStockDay)
	if b.IsOpen(SourceTWSEStockDay) {
		t.Fatal("未達門檻不該開啟")
	}
	b.Fail(SourceTWSEStockDay)
	if !b.IsOpen(SourceTWSEStockDay) {
		t.Fatal("達到門檻要開啟")
	}
	// **來源之間互不影響**：一個來源壞掉不該連累另一個。
	if b.IsOpen(SourceTPExStockDay) {
		t.Error("其他來源不該被連帶開啟")
	}
}

// 「連續」的語意靠 Succeed 維持——中間成功過一次就不算連續。
//
// 沒有這條的話，一整天零星的失敗累加起來也會打開 breaker，
// 那不是斷路器要偵測的狀況。
func TestSourceBreakerSuccessResetsConsecutiveCount(t *testing.T) {
	b := NewSourceBreaker(3, time.Hour)

	b.Fail(SourceTWSEStockDay)
	b.Fail(SourceTWSEStockDay)
	b.Succeed(SourceTWSEStockDay)
	if b.Failures(SourceTWSEStockDay) != 0 {
		t.Fatalf("成功要歸零，得到 %d", b.Failures(SourceTWSEStockDay))
	}
	b.Fail(SourceTWSEStockDay)
	b.Fail(SourceTWSEStockDay)
	if b.IsOpen(SourceTWSEStockDay) {
		t.Error("歸零後再失敗兩次仍未達門檻 3，不該開啟")
	}
}

// 冷卻到期**自動**恢復，恢復條件是時間到不是人工介入。
//
// 恢復時失敗計數要一起歸零，否則下一次失敗會立刻再度開啟，等於冷卻沒有意義。
func TestSourceBreakerRecoversAfterCooldown(t *testing.T) {
	b := NewSourceBreaker(1, 30*time.Minute)
	now := time.Date(2026, 8, 27, 16, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return now }

	b.Fail(SourceTWSEMarket)
	if !b.IsOpen(SourceTWSEMarket) {
		t.Fatal("門檻 1，失敗一次就該開啟")
	}

	now = now.Add(29 * time.Minute)
	if !b.IsOpen(SourceTWSEMarket) {
		t.Error("冷卻未到不該恢復")
	}

	// **邊界是 >=**：剛好到冷卻時間就要恢復。
	now = now.Add(time.Minute)
	if b.IsOpen(SourceTWSEMarket) {
		t.Error("冷卻到期要自動恢復")
	}
	if b.Failures(SourceTWSEMarket) != 0 {
		t.Error("恢復時失敗計數要一起歸零，否則下一次失敗立刻又開啟")
	}
}

// threshold <= 0 要兜底成 1：0 會讓 breaker 從第一次失敗就永遠開著，
// 偵測整條變成不可達。
func TestSourceBreakerClampsNonPositiveThreshold(t *testing.T) {
	b := NewSourceBreaker(0, time.Hour)
	if b.IsOpen(SourceTWSECalendar) {
		t.Fatal("還沒失敗就不該開啟")
	}
	b.Fail(SourceTWSECalendar)
	if !b.IsOpen(SourceTWSECalendar) {
		t.Error("threshold 應被兜底成 1")
	}
}
