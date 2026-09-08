package market

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
)

// TWSE「終止上市公司」名單的抓取與解析（計畫書 docs/todo.md T-071）。
//
// ⚠️ **這份名單只補日期、不驅動 is_listed**：判定「還在不在交易」的權責只屬於
// stock_symbol_sync 的 ISIN 缺席邏輯。理由是這份 CSV 含**歷史**紀錄，
// 代號會被重用——2002 年的「光寶電子(2301)」與今天的「光寶科(2301)」是兩家公司。

// DelistingSourceName 是寫進 delisting_events.source / snapshots.source 的來源識別。
const DelistingSourceName = "twse_suspend_listing"

// maxSuspendListingBodyBytes：實測回應約 8 KB，1 MiB 已是數量級的餘裕。
const maxSuspendListingBodyBytes = 1 << 20

// ErrEmptySuspendListing 代表來源回了東西但解析不出任何一筆事件。
// ⛔ 空名單一律失敗，不能當成「今天沒有下市公司」——那會讓所有事件被標記消失。
var ErrEmptySuspendListing = errors.New("twse suspend listing parsed empty")

// ErrUnexpectedSuspendListingHeader 代表兩行 header 不是預期的形狀（欄位被插入、
// 重排或改名）。
//
// ⛔ **必須整輪失敗，不能靠「反正解析不出來就略過」**：欄位重排之後每一列的
// 日期／名稱／代號都會錯位，而**筆數不變**——縮水防護攔不住，
// 舊事件會被標成消失並撤銷正確的投影。
var ErrUnexpectedSuspendListingHeader = errors.New("twse suspend listing header changed")

// ErrMalformedSuspendListingRow 代表 header 之後出現無法解析的資料列。
//
// ⛔ **不得靜默略過**：略過一列就是「少一筆事件」，而事件消失的處置是標記 missing
// ＋撤銷投影。少數幾列壞掉時筆數只少一點，縮水防護不一定攔得住。
var ErrMalformedSuspendListingRow = errors.New("twse suspend listing has a malformed row")

// ErrConflictingSuspendListingRows 代表同一批 CSV 內出現同 (symbol, date) 但**不同名稱**。
//
// ⛔ 整輪解析失敗，不猜哪個名稱是對的：名稱是投影身分比對的唯一輸入，
// 猜錯就是誤投影到別家公司。
var ErrConflictingSuspendListingRows = errors.New("twse suspend listing has conflicting rows")

// DelistingEventRow 是解析出來的一筆事件。
type DelistingEventRow struct {
	Symbol       string
	CompanyName  string
	DelistedDate time.Time
}

// DelistingSourceResult 是一次成功抓取的完整結果。
type DelistingSourceResult struct {
	Rows []DelistingEventRow
	// RowCount 是**去重後**的 canonical event 數，縮水防護比的就是它。
	// ⛔ 不是 CSV 原始列數——否則去重會被誤判成縮水。
	RowCount int
	// ContentHash 讓「來源完全沒變」看得出來（僅供觀察，不影響流程）。
	ContentHash string
	// DuplicateRows 是被去重掉的完全重複列數（Info 等級，不是錯誤）。
	DuplicateRows int
	// DistinctSymbols 是 job_runs.symbols_total 的值。
	//
	// ⚠️ **不是事件筆數**：身分鍵是 (source, symbol, date)，同一代號可以有多筆
	// 不同日期的事件（代號重用時必然如此）。而 symbols_total 的單位由全域契約
	// 定死是「標的數」（見 docs/api-reference.md，原記於 issue.md I-092）。
	DistinctSymbols int
}

type TWSESuspendListingClient struct {
	url  string
	http *http.Client
	log  *zap.Logger
}

func NewTWSESuspendListingClient(url string, timeout time.Duration, log *zap.Logger) *TWSESuspendListingClient {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &TWSESuspendListingClient{url: url, http: &http.Client{Timeout: timeout}, log: log}
}

// FetchDelistings 抓取並解析名單。任何一步失敗都回錯——⛔ all-or-nothing，不做部分寫入。
func (c *TWSESuspendListingClient) FetchDelistings(ctx context.Context) (DelistingSourceResult, error) {
	var out DelistingSourceResult
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return out, err
	}
	// 誠實表明來源身分，不偽裝成瀏覽器（與 twse_isin.go 同一原則）：
	// 這是公開名單，抓取本身正當，站方要限流或聯絡時也才有辨識依據。
	req.Header.Set("User-Agent", "stock-trading/1.0 (personal market data sync)")
	req.Header.Set("Accept", "text/csv,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en;q=0.8")

	resp, err := c.http.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, fmt.Errorf("twse suspend listing: unexpected status %d", resp.StatusCode)
	}

	// 多讀 1 byte 用來判斷是否真的超過上限（而不是剛好等於上限）。
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSuspendListingBodyBytes+1))
	if err != nil {
		return out, err
	}
	if len(body) > maxSuspendListingBodyBytes {
		return out, fmt.Errorf("twse suspend listing: response body exceeds %d bytes", maxSuspendListingBodyBytes)
	}

	// 實測 Content-Type 是 `text/csv;charset=ms950`，複用 ISIN 那邊的判斷與解碼器。
	reader, err := newTWSEISINBodyReader(body, resp.Header.Get("Content-Type"))
	if err != nil {
		return out, err
	}
	return ParseSuspendListingCSV(reader)
}

// ParseSuspendListingCSV 解析已解碼成 UTF-8 的 CSV。
//
// 來源格式（2026-09-04 實測 265 筆）：
//
//	"終止上市公司"                      ← 第 1 行標題
//	"終止上市日期","公司名稱","上市編號",  ← 第 2 行欄位名
//	"115/09/01","三商壽","2867",         ← 民國 100 年以後
//	="099/11/15","友尚","2403",          ← ⚠️ 099 年以前多一個 `=` 前綴
//
// 三個要點：①**兩行 header**；②行尾多一個逗號（4 欄）；
// ③⚠️ **民國 099 年以前的列有 Excel 防轉型前綴 `=`**（實測 151/265 筆），解析要吃兩種。
func ParseSuspendListingCSV(r io.Reader) (DelistingSourceResult, error) {
	var out DelistingSourceResult

	raw, err := io.ReadAll(r)
	if err != nil {
		return out, err
	}
	sum := sha256.Sum256(raw)
	out.ContentHash = hex.EncodeToString(sum[:])

	cr := csv.NewReader(bytes.NewReader(raw))
	// 行尾多一個逗號讓欄位數是 4，而 header 只有 1～3 欄——關掉欄位數檢查。
	cr.FieldsPerRecord = -1
	// 名稱裡可能有引號等字元，寬鬆處理。
	cr.LazyQuotes = true

	records, err := cr.ReadAll()
	if err != nil {
		return out, fmt.Errorf("twse suspend listing: csv parse: %w", err)
	}

	// canonical 以身分鍵去重。⛔ 必須在解析階段做完，不能丟給 DB：
	// postgres 的批次 upsert 對**同一批內**重複命中同一個 conflict target 會直接報
	// `ON CONFLICT DO UPDATE command cannot affect row a second time`，整批失敗。
	type identity struct {
		symbol string
		date   string
	}
	canonical := map[identity]DelistingEventRow{}
	order := []identity{}

	// ⛔ **header 要明確辨識，資料列不得靜默略過。**
	// 舊版是「解析不出日期就 continue」——那讓 header 與**壞掉的資料列**走同一條路：
	// 只要還剩一筆有效資料，解析就「成功」。TWSE 若插入或重排欄位而筆數不變，
	// 縮水防護不會觸發，錯位的資料會把舊事件標成消失並撤銷正確投影。
	// **完全沒有內容**時回既有的「空名單」語意，而不是 header 錯誤：
	// 回應是空的與「header 變了」是兩種不同的上游狀況，錯誤訊息要分得開。
	// ⛔ 兩者都是整輪失敗——空名單不能當成「今天沒有下市公司」。
	nonBlank := 0
	for _, rec := range records {
		if !blankSuspendListingRecord(rec) {
			nonBlank++
		}
	}
	if nonBlank == 0 {
		return out, ErrEmptySuspendListing
	}

	seenHeader := 0
	for i, rec := range records {
		if blankSuspendListingRecord(rec) {
			continue // 真正的空行
		}
		if seenHeader < 2 {
			if err := checkSuspendListingHeader(seenHeader, rec); err != nil {
				return out, err
			}
			seenHeader++
			continue
		}

		if len(rec) < 3 {
			return out, fmt.Errorf("%w: 第 %d 列只有 %d 欄", ErrMalformedSuspendListingRow, i+1, len(rec))
		}
		dateCell := cleanSuspendListingCell(rec[0])
		name := cleanSuspendListingCell(rec[1])
		symbol := cleanSuspendListingCell(rec[2])
		if symbol == "" || dateCell == "" || name == "" {
			return out, fmt.Errorf("%w: 第 %d 列有空欄（date=%q name=%q symbol=%q）",
				ErrMalformedSuspendListingRow, i+1, dateCell, name, symbol)
		}
		// 複用 exchange_reference.go 的 parseROCDate：它已經帶 newStrictDate 的
		// 歸一化防護（`115/02/31` 不會被 time.Date 靜默轉成 3/3）。
		date, err := parseROCDate(dateCell)
		if err != nil {
			return out, fmt.Errorf("%w: 第 %d 列日期無法解析: %w", ErrMalformedSuspendListingRow, i+1, err)
		}

		id := identity{symbol: symbol, date: date.Format("2006-01-02")}
		if prev, ok := canonical[id]; ok {
			if prev.CompanyName != name {
				// ⛔ 來源自相矛盾：無法決定哪個名稱是對的，而名稱是身分比對的唯一輸入。
				return out, fmt.Errorf("%w: symbol=%s date=%s names=%q/%q",
					ErrConflictingSuspendListingRows, symbol, id.date, prev.CompanyName, name)
			}
			out.DuplicateRows++
			continue
		}
		canonical[id] = DelistingEventRow{Symbol: symbol, CompanyName: name, DelistedDate: date}
		order = append(order, id)
	}

	if seenHeader < 2 {
		return out, fmt.Errorf("%w: 只讀到 %d 行 header", ErrUnexpectedSuspendListingHeader, seenHeader)
	}
	if len(canonical) == 0 {
		return out, ErrEmptySuspendListing
	}

	out.Rows = make([]DelistingEventRow, 0, len(canonical))
	for _, id := range order {
		out.Rows = append(out.Rows, canonical[id])
	}
	out.RowCount = len(out.Rows)

	symbols := map[string]struct{}{}
	for _, row := range out.Rows {
		symbols[row.Symbol] = struct{}{}
	}
	out.DistinctSymbols = len(symbols)

	// 排序讓下游（log、測試）有穩定順序；DB 寫入順序不影響結果。
	sort.SliceStable(out.Rows, func(i, j int) bool {
		if out.Rows[i].DelistedDate.Equal(out.Rows[j].DelistedDate) {
			return out.Rows[i].Symbol < out.Rows[j].Symbol
		}
		return out.Rows[i].DelistedDate.After(out.Rows[j].DelistedDate)
	})
	return out, nil
}

func blankSuspendListingRecord(rec []string) bool {
	for _, cell := range rec {
		if cleanSuspendListingCell(cell) != "" {
			return false
		}
	}
	return true
}

// checkSuspendListingHeader 驗兩行 header 的形狀（2026-09-04 實測）：
//
//	第 1 行：「終止上市公司」（單欄標題）
//	第 2 行：「終止上市日期」,「公司名稱」,「上市編號」, ← 行尾多一個逗號
//
// ⚠️ **比對關鍵字而不是完整字串**：完整比對會被「終止上市日」這種無害改名打死；
// 但關鍵字的**位置**必須對，那正是欄位重排會壞掉的地方。
func checkSuspendListingHeader(index int, rec []string) error {
	cells := make([]string, 0, len(rec))
	for _, cell := range rec {
		cells = append(cells, cleanSuspendListingCell(cell))
	}
	if index == 0 {
		if !strings.Contains(strings.Join(cells, ""), "終止上市") {
			return fmt.Errorf("%w: 第 1 行不是標題列（得到 %q）",
				ErrUnexpectedSuspendListingHeader, cells)
		}
		return nil
	}
	want := [3]string{"日期", "名稱", "編號"}
	if len(cells) < 3 {
		return fmt.Errorf("%w: 第 2 行只有 %d 欄（得到 %q）",
			ErrUnexpectedSuspendListingHeader, len(cells), cells)
	}
	for i, keyword := range want {
		if !strings.Contains(cells[i], keyword) {
			return fmt.Errorf("%w: 第 2 行第 %d 欄應含 %q，得到 %q（欄位被插入或重排？）",
				ErrUnexpectedSuspendListingHeader, i+1, keyword, cells[i])
		}
	}
	return nil
}

// cleanSuspendListingCell 去掉 Excel 防轉型前綴 `=`、引號與前後空白。
func cleanSuspendListingCell(cell string) string {
	s := strings.TrimSpace(cell)
	s = strings.TrimPrefix(s, "=")
	s = strings.Trim(s, `"`)
	return strings.TrimSpace(s)
}
