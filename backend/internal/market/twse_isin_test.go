package market

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/traditionalchinese"

	"github.com/trading/backend/internal/store"
)

const twseISINFixture = `
<html><body><table>
<tr><td>Security Code & Security Name</td><td>ISIN Code</td><td>Date Listed</td><td>Market</td><td>Industrial Group</td><td>CFICode</td><td>Remarks</td></tr>
<tr><td colspan="7">Stocks</td></tr>
<tr><td>1101　TCC</td><td>TW0001101004</td><td>1962/02/09</td><td>TWSE LISTED</td><td>Cement</td><td>ESVUFR</td><td></td></tr>
<tr><td>0050　YUANTA TAIWAN 50</td><td>TW0000050004</td><td>2003/06/30</td><td>TWSE LISTED</td><td></td><td>CEOJEU</td><td></td></tr>
<tr><td colspan="7">ETFs</td></tr>
<tr><td>00981A　ACTIVE ETF</td><td>TW00000981A1</td><td>2025/05/05</td><td>TWSE LISTED</td><td></td><td>CEOIEU</td><td>note</td></tr>
</table></body></html>`

func TestNewTWSEISINClientUsesConfiguredTimeout(t *testing.T) {
	client := NewTWSEISINClient([]string{"http://example.test"}, TWSEISINClientOptions{Timeout: 12 * time.Second}, nil)
	if client.http.Timeout != 12*time.Second {
		t.Fatalf("expected configured timeout, got %s", client.http.Timeout)
	}
}

// 0 值（config 未設 timeout_sec / fetch_delay_sec）必須 fallback 到 market 套件常數，
// 這是「預設值只有一個來源」的保證。
func TestNewTWSEISINClientUsesDefaultTimeout(t *testing.T) {
	client := NewTWSEISINClient([]string{"http://example.test"}, TWSEISINClientOptions{}, nil)
	if client.http.Timeout != defaultTWSEISINTimeout {
		t.Fatalf("expected default timeout %s, got %s", defaultTWSEISINTimeout, client.http.Timeout)
	}
	if client.fetchDelay != defaultFetchDelayBetweenSources {
		t.Fatalf("expected default fetch delay %s, got %s", defaultFetchDelayBetweenSources, client.fetchDelay)
	}
}

func TestNewTWSEISINClientUsesConfiguredFetchDelay(t *testing.T) {
	client := NewTWSEISINClient([]string{"http://example.test"}, TWSEISINClientOptions{FetchDelay: 7 * time.Second}, nil)
	if client.fetchDelay != 7*time.Second {
		t.Fatalf("expected configured fetch delay, got %s", client.fetchDelay)
	}
}

func TestFetchStockSymbolsSendsIdentifyingHeaders(t *testing.T) {
	var gotUserAgent string
	var gotAccept string
	var gotAcceptLanguage string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		gotAcceptLanguage = r.Header.Get("Accept-Language")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(twseISINFixture))
	}))
	defer srv.Close()

	client := NewTWSEISINClient([]string{srv.URL}, TWSEISINClientOptions{Timeout: time.Second}, nil)
	rows, err := client.FetchStockSymbols(context.Background())
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	// 刻意不偽裝成瀏覽器：UA 要能辨識出是本系統，方便來源站方限流或聯絡。
	if !strings.Contains(gotUserAgent, "stock-trading/") {
		t.Fatalf("expected self-identifying User-Agent, got %q", gotUserAgent)
	}
	if strings.Contains(gotUserAgent, "Mozilla") {
		t.Fatalf("User-Agent should not masquerade as a browser, got %q", gotUserAgent)
	}
	if !strings.Contains(gotAccept, "text/html") {
		t.Fatalf("expected text/html Accept header, got %q", gotAccept)
	}
	if !strings.Contains(gotAcceptLanguage, "zh-TW") {
		t.Fatalf("expected zh-TW Accept-Language header, got %q", gotAcceptLanguage)
	}
}

func TestFetchStockSymbolsRejectsUnexpectedHTMLWithDiagnostics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><table><tr><td>temporarily unavailable</td></tr></table></body></html>`))
	}))
	defer srv.Close()

	client := NewTWSEISINClient([]string{srv.URL}, TWSEISINClientOptions{Timeout: time.Second}, nil)
	_, err := client.FetchStockSymbols(context.Background())
	if err == nil {
		t.Fatal("expected empty parse error")
	}
	if !strings.Contains(err.Error(), "twse isin parsed empty") {
		t.Fatalf("expected parsed-empty diagnostic, got %v", err)
	}
	if !strings.Contains(err.Error(), "rows=1") {
		t.Fatalf("expected row count diagnostic, got %v", err)
	}
	// 呼叫端要能用 errors.Is 區分「來源有回應但沒有半筆有價證券」與網路／狀態碼失敗。
	if !errors.Is(err, store.ErrEmptyStockSymbolSnapshot) {
		t.Fatalf("expected wrapped ErrEmptyStockSymbolSnapshot, got %v", err)
	}
}

// 來源改版或回錯誤頁時 candidate_rows 會是 0，SampleRows 因此為空；診斷要靠
// SkippedRowSamples 才看得到頁面實際長相。
func TestParseTWSEISINSymbolsKeepsSkippedRowSamples(t *testing.T) {
	const unexpected = `<html><body><table>
<tr><td>temporarily unavailable</td></tr>
<tr><td>please try again later</td><td>2026/07/22</td></tr>
</table></body></html>`
	rows, stats, err := parseTWSEISINSymbols(strings.NewReader(unexpected), nil)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(rows) != 0 || stats.CandidateRows != 0 {
		t.Fatalf("expected no candidates, got rows=%d stats=%+v", len(rows), stats)
	}
	if len(stats.SkippedRowSamples) == 0 {
		t.Fatal("expected skipped-row samples for diagnostics")
	}
	if !strings.Contains(strings.Join(stats.SkippedRowSamples, " "), "try again later") {
		t.Fatalf("expected skipped samples to carry page content, got %+v", stats.SkippedRowSamples)
	}
}

// 樣本截斷必須落在 rune 邊界，否則中文會被切成半個字、在 JSON log 變成亂碼。
func TestSampleTWSEISINRowTruncatesOnRuneBoundary(t *testing.T) {
	long := strings.Repeat("台泥水泥", 40) // 遠超過 240 bytes 的純中文
	got := sampleTWSEISINRow([]string{long})
	if len(got) > maxTWSEISINSampleBytes {
		t.Fatalf("expected truncation to %d bytes, got %d", maxTWSEISINSampleBytes, len(got))
	}
	if !utf8.ValidString(got) {
		t.Fatalf("truncated sample is not valid UTF-8: %q", got)
	}
	if !strings.HasPrefix(long, got) {
		t.Fatalf("truncated sample should be a prefix of the input, got %q", got)
	}
}

func TestFetchStockSymbolsDecodesMS950Body(t *testing.T) {
	html := `<html><body><table>
<tr><td>有價證券代號及名稱</td><td>國際證券辨識號碼(ISIN Code)</td><td>上市日</td><td>市場別</td><td>產業別</td><td>CFICode</td><td>備註</td></tr>
<tr><td colspan="7">股票</td></tr>
<tr><td>1101　台泥</td><td>TW0001101004</td><td>1962/02/09</td><td>上市</td><td>水泥工業</td><td>ESVUFR</td><td></td></tr>
</table></body></html>`
	encoded, err := traditionalchinese.Big5.NewEncoder().String(html)
	if err != nil {
		t.Fatalf("encode fixture failed: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html;charset=MS950")
		_, _ = w.Write([]byte(encoded))
	}))
	defer srv.Close()

	client := NewTWSEISINClient([]string{srv.URL}, TWSEISINClientOptions{Timeout: time.Second}, nil)
	rows, err := client.FetchStockSymbols(context.Background())
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Symbol != "1101" || rows[0].Name != "台泥" || rows[0].Market != "上市" {
		t.Fatalf("unexpected decoded row: %+v", rows[0])
	}
}

func TestParseTWSEISINSymbolStats(t *testing.T) {
	rows, stats, err := parseTWSEISINSymbols(strings.NewReader(twseISINFixture), nil)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if stats.Rows == 0 || stats.CandidateRows != 3 || stats.ParsedRows != 3 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if len(stats.SampleRows) == 0 || !strings.Contains(stats.SampleRows[0], "1101") {
		t.Fatalf("expected sample rows to include parsed candidates, got %+v", stats.SampleRows)
	}
}

func TestParseTWSEISINSymbols(t *testing.T) {
	rows, err := ParseTWSEISINSymbols(strings.NewReader(twseISINFixture), nil)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}

	first := rows[0]
	if first.Symbol != "1101" || first.Name != "TCC" || first.ISINCode != "TW0001101004" {
		t.Fatalf("unexpected first row: %+v", first)
	}
	if first.SecurityType != "Stocks" || first.Industry != "Cement" || !first.ListedDate.Valid {
		t.Fatalf("unexpected first metadata: %+v", first)
	}

	etf := rows[2]
	if etf.Symbol != "00981A" || etf.SecurityType != "ETFs" || etf.Remarks != "note" {
		t.Fatalf("unexpected ETF row: %+v", etf)
	}
}

func TestParseListedDate(t *testing.T) {
	if nt, malformed := parseListedDate("1962/02/09"); malformed || !nt.Valid {
		t.Fatalf("valid date should parse: malformed=%v valid=%v", malformed, nt.Valid)
	}
	if nt, malformed := parseListedDate(""); malformed || nt.Valid {
		t.Fatalf("empty date is a legit null, not malformed: malformed=%v valid=%v", malformed, nt.Valid)
	}
	// 非空但格式不符 → 標記 malformed 供呼叫端告警，而非靜默吞掉。
	if nt, malformed := parseListedDate("2025-05-05"); !malformed || nt.Valid {
		t.Fatalf("unparseable non-empty date should be flagged malformed: malformed=%v valid=%v", malformed, nt.Valid)
	}
}
