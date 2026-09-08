package market

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding/traditionalchinese"
)

// realFixture 讀 2026-09-04 從 TWSE 實抓的那份 CSV（Big5／CRLF／265 筆）。
//
// ⚠️ **刻意用真實回應而不是手編字串**：兩行 header、行尾逗號、`=` 前綴、
// 六碼 TDR 代號這些都是來源的真實怪癖，手編很容易編出一份「自己看得懂」的假資料。
func realFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/twse_suspend_listing_20260904.csv")
	if err != nil {
		t.Fatalf("讀 fixture: %v", err)
	}
	utf8, err := traditionalchinese.Big5.NewDecoder().Bytes(raw)
	if err != nil {
		t.Fatalf("Big5 解碼: %v", err)
	}
	return utf8
}

// #1｜兩種日期前綴都要解析成正確的西元日期。
//
// 民國 100 年以後是 `"115/09/01"`，099 年以前是 `="099/11/15"`
// （Excel 防轉型前綴，實測 151/265 筆）。只吃一種的話會漏掉一半以上的資料。
func TestParseSuspendListingHandlesBothDatePrefixes(t *testing.T) {
	res, err := ParseSuspendListingCSV(bytes.NewReader(realFixture(t)))
	if err != nil {
		t.Fatalf("解析: %v", err)
	}

	byString := map[string]DelistingEventRow{}
	for _, row := range res.Rows {
		byString[row.Symbol] = row
	}

	cases := []struct {
		symbol string
		name   string
		want   time.Time
		note   string
	}{
		{"2867", "三商壽", time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), `無前綴 "115/09/01"`},
		{"2403", "友尚", time.Date(2010, 11, 15, 0, 0, 0, 0, time.UTC), `有 = 前綴 ="099/11/15"`},
		{"2301", "光寶電子", time.Date(2002, 11, 4, 0, 0, 0, 0, time.UTC), `有 = 前綴（代號重用案例）`},
	}
	for _, c := range cases {
		got, ok := byString[c.symbol]
		if !ok {
			t.Errorf("%s：解析結果裡找不到 %s", c.note, c.symbol)
			continue
		}
		if !got.DelistedDate.Equal(c.want) {
			t.Errorf("%s：%s 的日期得到 %s，期望 %s",
				c.note, c.symbol, got.DelistedDate.Format("2006-01-02"), c.want.Format("2006-01-02"))
		}
		if got.CompanyName != c.name {
			t.Errorf("%s：%s 的名稱得到 %q，期望 %q", c.note, c.symbol, got.CompanyName, c.name)
		}
	}
}

// #2｜真實 fixture 的整體形狀：兩行 header 被略過、行尾逗號不影響、六碼 TDR 代號照收。
func TestParseSuspendListingRealFixtureShape(t *testing.T) {
	res, err := ParseSuspendListingCSV(bytes.NewReader(realFixture(t)))
	if err != nil {
		t.Fatalf("解析: %v", err)
	}

	// 2026-09-04 實測 265 筆、無重複代號。
	if res.RowCount != 265 {
		t.Errorf("RowCount 得到 %d，期望 265", res.RowCount)
	}
	if res.DuplicateRows != 0 {
		t.Errorf("這份 fixture 無重複列，DuplicateRows 得到 %d", res.DuplicateRows)
	}
	// ⚠️ 這份 fixture 剛好 265 == 265 是**巧合不是保證**：身分鍵是 (symbol, date)，
	// 同一代號可以有多筆事件。這裡分開斷言，避免日後有人以為兩者恆等。
	if res.DistinctSymbols != 265 {
		t.Errorf("DistinctSymbols 得到 %d，期望 265", res.DistinctSymbols)
	}

	// 兩行 header 不得混進資料。
	for _, row := range res.Rows {
		if strings.Contains(row.Symbol, "編號") || strings.Contains(row.CompanyName, "公司名稱") {
			t.Errorf("header 混進資料了：%+v", row)
		}
	}

	// 六碼 TDR（91xxxx）不特別處理，但要收進來——主檔沒有就自然不匹配。
	var tdr int
	for _, row := range res.Rows {
		if len(row.Symbol) == 6 && strings.HasPrefix(row.Symbol, "91") {
			tdr++
		}
	}
	if tdr != 19 {
		t.Errorf("六碼 TDR 得到 %d 筆，實測應為 19", tdr)
	}

	if res.ContentHash == "" {
		t.Error("ContentHash 不得為空")
	}
}

// #2｜空檔要明確報錯。⛔ 不能當成「今天沒有下市公司」——那會讓所有事件被標記消失。
func TestParseSuspendListingEmptyIsError(t *testing.T) {
	for _, body := range []string{"", "\"終止上市公司\"\n\"終止上市日期\",\"公司名稱\",\"上市編號\",\n"} {
		_, err := ParseSuspendListingCSV(strings.NewReader(body))
		if !errors.Is(err, ErrEmptySuspendListing) {
			t.Errorf("空內容應回 ErrEmptySuspendListing，得到 %v", err)
		}
	}
}

// #2｜非 Big5 的亂碼：解析不出任何日期 → 視為空，明確報錯而不是靜默回 0 筆。
func TestParseSuspendListingGarbageIsError(t *testing.T) {
	_, err := ParseSuspendListingCSV(bytes.NewReader([]byte{0xff, 0xfe, 0x00, 0x01, 0x02}))
	if err == nil {
		t.Error("亂碼應該報錯")
	}
}

// #2b｜同批重複 key。
//
// ⛔ 必須在解析階段處理完：postgres 的批次 upsert 對同一批內重複命中同一個
// conflict target 會直接報 `cannot affect row a second time`，整批失敗。
func TestParseSuspendListingDuplicateKeys(t *testing.T) {
	header := "\"終止上市公司\"\n\"終止上市日期\",\"公司名稱\",\"上市編號\",\n"

	t.Run("同名重複：去重且 RowCount 採去重後數量", func(t *testing.T) {
		body := header +
			"\"115/09/01\",\"三商壽\",\"2867\",\n" +
			"\"115/09/01\",\"三商壽\",\"2867\",\n" +
			"\"114/10/01\",\"京城銀\",\"2809\",\n"
		res, err := ParseSuspendListingCSV(strings.NewReader(body))
		if err != nil {
			t.Fatalf("同名重複應該去重而不是報錯: %v", err)
		}
		if res.RowCount != 2 {
			t.Errorf("RowCount 應為去重後的 2，得到 %d", res.RowCount)
		}
		if res.DuplicateRows != 1 {
			t.Errorf("DuplicateRows 應為 1，得到 %d", res.DuplicateRows)
		}
	})

	t.Run("異名衝突：整輪解析失敗", func(t *testing.T) {
		// ⛔ 不猜哪個名稱是對的——名稱是投影身分比對的唯一輸入，猜錯就是誤投影。
		body := header +
			"\"115/09/01\",\"三商壽\",\"2867\",\n" +
			"\"115/09/01\",\"另一家公司\",\"2867\",\n"
		_, err := ParseSuspendListingCSV(strings.NewReader(body))
		if !errors.Is(err, ErrConflictingSuspendListingRows) {
			t.Errorf("同 key 異名應回 ErrConflictingSuspendListingRows，得到 %v", err)
		}
	})
}

// 民國年換算與不存在的日期。
//
// ⚠️ parseROCDate 是 exchange_reference.go 既有的實作（複用而非重寫）；
// 這裡驗的是**本來源會遇到的輸入**——尤其是 `099/...` 這種補零的三位數年份。
func TestParseROCDateForSuspendListing(t *testing.T) {
	ok := []struct {
		in   string
		want time.Time
	}{
		{"115/09/01", time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)},
		{"099/11/15", time.Date(2010, 11, 15, 0, 0, 0, 0, time.UTC)},
		{"090/01/20", time.Date(2001, 1, 20, 0, 0, 0, 0, time.UTC)},
		{"1/1/1", time.Date(1912, 1, 1, 0, 0, 0, 0, time.UTC)}, // 民國元年
	}
	for _, c := range ok {
		got, err := parseROCDate(c.in)
		if err != nil {
			t.Errorf("%q 應可解析: %v", c.in, err)
			continue
		}
		if !got.Equal(c.want) {
			t.Errorf("%q 得到 %s，期望 %s", c.in, got.Format("2006-01-02"), c.want.Format("2006-01-02"))
		}
	}

	bad := []string{
		"",
		"115-09-01",
		"115/09",
		"abc/09/01",
		"115/13/01", // 月份超範圍
		"115/00/01",
		"115/02/31", // ⚠️ 不存在的日期：time.Date 會歸一化成 3/3，必須被擋下
	}
	for _, in := range bad {
		if got, err := parseROCDate(in); err == nil {
			t.Errorf("%q 應該報錯，卻得到 %s", in, got.Format("2006-01-02"))
		}
	}

	// ⛔ **年份下界**（2026-09-08 修，原記於 issue.md I-109，已收斂）：民國元年是西元 1912，
	// 所以 0 年與負數年都不存在。舊版直接 `+1911`，把 `0/01/01` 靜默算成
	// 1911-01-01、`-5/01/01` 算成 1906——**不存在的日期被轉成一個合法日期**。
	for _, in := range []string{"0/01/01", "-5/01/01", "000/12/31"} {
		if got, err := parseROCDate(in); err == nil {
			t.Errorf("%q 是不存在的民國年，應該報錯，卻得到 %s", in, got.Format("2006-01-02"))
		}
	}
	// 邊界的另一側：民國元年（西元 1912）必須照常解析。
	if got, err := parseROCDate("1/01/01"); err != nil {
		t.Errorf("民國元年應可解析：%v", err)
	} else if got.Year() != 1912 {
		t.Errorf("民國元年應是西元 1912，得到 %d", got.Year())
	}
}

// 三個真實的代號重用案例都要被解析出來——它們是投影規則測試的 fixture 來源。
func TestParseSuspendListingKeepsSymbolReuseCases(t *testing.T) {
	res, err := ParseSuspendListingCSV(bytes.NewReader(realFixture(t)))
	if err != nil {
		t.Fatalf("解析: %v", err)
	}
	want := map[string]string{
		"2867": "三商壽",   // 主檔同名 → 應投影
		"6423": "億而得-創", // 轉上櫃，主檔 is_listed=true → 不投影
		"2432": "倚天資訊",  // 代號重用，主檔是「倚天酷碁-創」→ 不投影
		"2301": "光寶電子",  // 代號重用，主檔是「光寶科」→ 不投影
	}
	got := map[string]string{}
	for _, row := range res.Rows {
		if _, ok := want[row.Symbol]; ok {
			got[row.Symbol] = row.CompanyName
		}
	}
	for symbol, name := range want {
		if got[symbol] != name {
			t.Errorf("%s 的名稱得到 %q，期望 %q", symbol, got[symbol], name)
		}
	}
}

// ── review 修正的回歸測試（2026-09-08）─────────────────────────────────────

const suspendListingHeader = "\"終止上市公司\"\n\"終止上市日期\",\"公司名稱\",\"上市編號\",\n"

// ⛔ **header 變了要整輪失敗，不能靠「解析不出來就略過」。**
//
// 這是最危險的一種格式變動：欄位重排之後每一列都錯位，而**筆數不變**——
// 縮水防護攔不住，錯位的資料會把舊事件標成消失並撤銷正確的投影。
func TestParseSuspendListingRejectsChangedHeader(t *testing.T) {
	cases := map[string]string{
		"欄位重排（名稱與日期對調）": "\"終止上市公司\"\n\"公司名稱\",\"終止上市日期\",\"上市編號\",\n" +
			"\"三商壽\",\"115/09/01\",\"2867\",\n",
		"插入新欄位": "\"終止上市公司\"\n\"終止上市日期\",\"市場別\",\"公司名稱\",\"上市編號\",\n" +
			"\"115/09/01\",\"上市\",\"三商壽\",\"2867\",\n",
		"少一行 header": "\"終止上市日期\",\"公司名稱\",\"上市編號\",\n" +
			"\"115/09/01\",\"三商壽\",\"2867\",\n",
		"第二行只有兩欄": "\"終止上市公司\"\n\"終止上市日期\",\"公司名稱\",\n",
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseSuspendListingCSV(bytes.NewReader([]byte(body)))
			if !errors.Is(err, ErrUnexpectedSuspendListingHeader) {
				t.Fatalf("應回 ErrUnexpectedSuspendListingHeader，得到 %v", err)
			}
		})
	}
}

// ⛔ **資料列壞掉不得靜默略過**：略過一列＝少一筆事件，而事件消失的處置是
// 標記 missing ＋撤銷投影。少數幾列壞掉時筆數只少一點，縮水防護不一定攔得住。
func TestParseSuspendListingRejectsMalformedRows(t *testing.T) {
	cases := map[string]string{
		"日期無法解析":  suspendListingHeader + "\"11X/09/01\",\"三商壽\",\"2867\",\n",
		"日期不存在":   suspendListingHeader + "\"115/02/31\",\"三商壽\",\"2867\",\n",
		"欄位數不足":   suspendListingHeader + "\"115/09/01\",\"三商壽\"\n",
		"公司名稱是空的": suspendListingHeader + "\"115/09/01\",\"\",\"2867\",\n",
		"代號是空的":   suspendListingHeader + "\"115/09/01\",\"三商壽\",\"\",\n",
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseSuspendListingCSV(bytes.NewReader([]byte(body)))
			if !errors.Is(err, ErrMalformedSuspendListingRow) {
				t.Fatalf("應回 ErrMalformedSuspendListingRow，得到 %v", err)
			}
		})
	}
}

// **有效資料仍在時也不得放行**：舊版只要還剩一筆有效資料就算解析成功。
func TestParseSuspendListingRejectsMalformedRowAmongValidRows(t *testing.T) {
	body := suspendListingHeader +
		"\"115/09/01\",\"三商壽\",\"2867\",\n" +
		"\"11X/06/23\",\"壞掉的列\",\"6806\",\n" +
		"\"115/03/27\",\"晶采\",\"3454\",\n"

	if _, err := ParseSuspendListingCSV(bytes.NewReader([]byte(body))); !errors.Is(
		err, ErrMalformedSuspendListingRow) {
		t.Fatalf("⛔ 只要有一列壞掉就要整輪失敗，得到 %v", err)
	}
}

// 空行（含只有逗號的行）仍要容忍——那是 CSV 的正常結尾形態。
func TestParseSuspendListingToleratesBlankLines(t *testing.T) {
	body := suspendListingHeader +
		"\"115/09/01\",\"三商壽\",\"2867\",\n" +
		"\n" +
		",,,\n"

	res, err := ParseSuspendListingCSV(bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("空行不該讓解析失敗: %v", err)
	}
	if res.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", res.RowCount)
	}
}
