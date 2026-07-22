package market

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/traditionalchinese"

	"github.com/trading/backend/internal/store"
)

// defaultTWSEISINTimeout / defaultFetchDelayBetweenSources 是這兩個參數的唯一真實來源：
// config 的 timeout_sec / fetch_delay_sec 留 0（或不設）時就會 fallback 到這裡，
// 避免同一個數字散落在 Go 常數、viper default 與 config.yaml 三處而各自漂移。
const (
	// defaultTWSEISINTimeout：2026-07-22 實測單一來源（strMode=2 上市）回應 7.5 MB、
	// 耗時約 251 秒，因此 30 秒與 90 秒都會在讀 body 時中斷（context deadline exceeded
	// while reading body）。300 秒是量測值加上約兩成餘裕；來源明顯變慢時調
	// stock_symbols.timeout_sec 即可，不需要改程式。
	defaultTWSEISINTimeout = 300 * time.Second
	// defaultFetchDelayBetweenSources：連續抓取多個 TWSE ISIN 來源（上市／上櫃）之間的
	// 間隔，避免短時間內連打同一站台觸發反爬／限流。
	defaultFetchDelayBetweenSources = 3 * time.Second
)

type TWSEISINClient struct {
	urls       []string
	http       *http.Client
	fetchDelay time.Duration
	log        *zap.Logger
}

type twseISINFetchMeta struct {
	StatusCode  int
	ContentType string
	BodyBytes   int
	ParseStats  twseISINParseStats
}

type twseISINParseStats struct {
	Rows                  int
	CandidateRows         int
	ParsedRows            int
	SkippedSymbolNameRows int
	SampleRows            []string
}

// TWSEISINClientOptions 的欄位皆為 0 值時沿用上面的 default 常數。
type TWSEISINClientOptions struct {
	// Timeout 是單一來源 HTTP 請求（含讀取 body）的上限。
	Timeout time.Duration
	// FetchDelay 是來源與來源之間的等待時間。懷疑被 TWSE 限流時，這是第一個該調大的參數。
	FetchDelay time.Duration
}

// NewTWSEISINClient 接受一或多個 TWSE ISIN 來源（例如 strMode=2 上市、strMode=4 上櫃），
// FetchStockSymbols 會全部抓取並合併，任一來源失敗即整體失敗（避免只拿到半個市場就把
// 另一半誤判為下市）。
//
// 刻意不做自動重試：股票主檔異動頻率低（新上市／下市），漏同步一天不影響既有標的的
// 分析與訊號，而重試會拉長連打 TWSE 的時間、增加被限流與「半套資料」的風險。失敗時
// 由人工按前端的手動同步按鈕（POST /api/v1/scheduler/stock-symbol-sync/run）補即可。
func NewTWSEISINClient(urls []string, opts TWSEISINClientOptions, log *zap.Logger) *TWSEISINClient {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTWSEISINTimeout
	}
	fetchDelay := opts.FetchDelay
	if fetchDelay <= 0 {
		fetchDelay = defaultFetchDelayBetweenSources
	}
	return &TWSEISINClient{
		urls: urls,
		http: &http.Client{
			Timeout: timeout,
		},
		fetchDelay: fetchDelay,
		log:        log,
	}
}

func (c *TWSEISINClient) FetchStockSymbols(ctx context.Context) ([]store.StockSymbol, error) {
	if len(c.urls) == 0 {
		return nil, fmt.Errorf("twse isin: no sync urls configured")
	}
	merged := make([]store.StockSymbol, 0, 2048)
	seen := make(map[string]struct{})
	for i, url := range c.urls {
		if i > 0 {
			// 來源之間間隔一段時間再打下一個，降低被 TWSE 限流／封鎖的風險；
			// context 取消或逾時時立即中止等待。
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.fetchDelay):
			}
		}
		startedAt := time.Now()
		symbols, meta, err := c.fetchOne(ctx, url)
		if err != nil {
			if c.log != nil {
				// 只記診斷用的 elapsed；錯誤本身往上拋，由 scheduler 統一記一次，
				// 避免同一次失敗在 log 出現兩筆重複的 error 訊息。
				c.log.Warn(
					"twse isin source fetch failed",
					zap.String("url", url),
					zap.Duration("elapsed", time.Since(startedAt)),
					zap.Int("status", meta.StatusCode),
					zap.String("content_type", meta.ContentType),
					zap.Int("body_bytes", meta.BodyBytes),
					zap.Int("rows", meta.ParseStats.Rows),
					zap.Int("candidate_rows", meta.ParseStats.CandidateRows),
					zap.Int("parsed_rows", meta.ParseStats.ParsedRows),
					zap.Int("skipped_symbol_name_rows", meta.ParseStats.SkippedSymbolNameRows),
					zap.Strings("sample_rows", meta.ParseStats.SampleRows),
				)
			}
			// all-or-nothing：任一來源失敗就整體失敗，避免部分快照觸發誤下市。
			return nil, fmt.Errorf("twse isin fetch %s: %w", url, err)
		}
		if c.log != nil {
			c.log.Info(
				"twse isin source fetched",
				zap.String("url", url),
				zap.Int("symbols", len(symbols)),
				zap.Duration("elapsed", time.Since(startedAt)),
				zap.Int("status", meta.StatusCode),
				zap.String("content_type", meta.ContentType),
				zap.Int("body_bytes", meta.BodyBytes),
				zap.Int("rows", meta.ParseStats.Rows),
				zap.Int("candidate_rows", meta.ParseStats.CandidateRows),
			)
		}
		for _, s := range symbols {
			if _, dup := seen[s.Symbol]; dup {
				continue
			}
			seen[s.Symbol] = struct{}{}
			merged = append(merged, s)
		}
	}
	if len(merged) == 0 {
		return nil, store.ErrEmptyStockSymbolSnapshot
	}
	return merged, nil
}

func (c *TWSEISINClient) fetchOne(ctx context.Context, url string) ([]store.StockSymbol, twseISINFetchMeta, error) {
	var meta twseISINFetchMeta
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, meta, err
	}
	// 誠實表明來源身分，不偽裝成瀏覽器：TWSE ISIN 是公開清單，抓取本身正當，
	// 站方要限流或聯絡時也才有辨識依據。Accept / Accept-Language 則是讓來源
	// 回傳預期的 HTML 與中文編碼。
	req.Header.Set("User-Agent", "stock-trading/1.0 (personal market data sync)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en;q=0.8")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, meta, err
	}
	defer resp.Body.Close()
	meta.StatusCode = resp.StatusCode
	meta.ContentType = resp.Header.Get("Content-Type")
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, meta, fmt.Errorf("twse isin: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, meta, err
	}
	meta.BodyBytes = len(body)

	reader, err := newTWSEISINBodyReader(body, meta.ContentType)
	if err != nil {
		return nil, meta, err
	}
	symbols, stats, err := parseTWSEISINSymbols(reader, c.log)
	meta.ParseStats = stats
	if err != nil {
		return nil, meta, err
	}
	if len(symbols) == 0 {
		return nil, meta, fmt.Errorf(
			"twse isin parsed empty: rows=%d candidate_rows=%d skipped_symbol_name_rows=%d body_bytes=%d content_type=%q",
			stats.Rows,
			stats.CandidateRows,
			stats.SkippedSymbolNameRows,
			meta.BodyBytes,
			meta.ContentType,
		)
	}
	return symbols, meta, nil
}

func newTWSEISINBodyReader(body []byte, contentType string) (io.Reader, error) {
	if isTWSEISINBig5Charset(contentType) {
		return traditionalchinese.Big5.NewDecoder().Reader(bytes.NewReader(body)), nil
	}
	return charset.NewReader(bytes.NewReader(body), contentType)
}

func isTWSEISINBig5Charset(contentType string) bool {
	_, params, err := mime.ParseMediaType(contentType)
	if err == nil {
		switch strings.ToLower(strings.TrimSpace(params["charset"])) {
		case "ms950", "cp950", "big5", "big-5", "windows-950":
			return true
		}
	}
	lower := strings.ToLower(contentType)
	return strings.Contains(lower, "charset=ms950") ||
		strings.Contains(lower, "charset=cp950") ||
		strings.Contains(lower, "charset=big5")
}

type StockSymbolSource interface {
	FetchStockSymbols(ctx context.Context) ([]store.StockSymbol, error)
}

type StockSymbolSyncer struct {
	source StockSymbolSource
	repo   store.StockSymbolRepo
	log    *zap.Logger
}

func NewStockSymbolSyncer(source StockSymbolSource, repo store.StockSymbolRepo, log *zap.Logger) *StockSymbolSyncer {
	return &StockSymbolSyncer{source: source, repo: repo, log: log}
}

func (s *StockSymbolSyncer) Sync(ctx context.Context, seenAt time.Time) (store.StockSymbolSyncResult, error) {
	symbols, err := s.source.FetchStockSymbols(ctx)
	if err != nil {
		return store.StockSymbolSyncResult{}, err
	}
	result, err := s.repo.UpsertSnapshot(ctx, symbols, seenAt)
	if err != nil {
		return store.StockSymbolSyncResult{}, err
	}
	if s.log != nil {
		s.log.Info("stock symbol sync completed", zap.Int("seen", result.Seen), zap.Int("delisted", result.Delisted))
	}
	return result, nil
}

func ParseTWSEISINSymbols(r io.Reader, log *zap.Logger) ([]store.StockSymbol, error) {
	out, _, err := parseTWSEISINSymbols(r, log)
	return out, err
}

func parseTWSEISINSymbols(r io.Reader, log *zap.Logger) ([]store.StockSymbol, twseISINParseStats, error) {
	var stats twseISINParseStats
	doc, err := html.Parse(r)
	if err != nil {
		return nil, stats, err
	}

	var rows [][]string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			if cells := rowCells(n); len(cells) > 0 {
				rows = append(rows, cells)
			}
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	stats.Rows = len(rows)

	currentType := ""
	malformedDates := 0
	out := make([]store.StockSymbol, 0, len(rows))
	for _, cells := range rows {
		if isTWSEISINHeader(cells) {
			continue
		}
		if len(cells) == 1 {
			currentType = normalizeSpace(cells[0])
			continue
		}
		if len(cells) < 6 {
			continue
		}
		stats.CandidateRows++
		if len(stats.SampleRows) < 5 {
			stats.SampleRows = append(stats.SampleRows, sampleTWSEISINRow(cells))
		}
		symbol, name, ok := parseSymbolName(cells[0])
		if !ok {
			stats.SkippedSymbolNameRows++
			continue
		}
		listedDate, malformed := parseListedDate(cells[2])
		if malformed {
			malformedDates++
		}
		out = append(out, store.StockSymbol{
			Symbol:       symbol,
			Name:         name,
			ISINCode:     normalizeSpace(cells[1]),
			Market:       normalizeSpace(cells[3]),
			SecurityType: currentType,
			Industry:     normalizeSpace(cells[4]),
			CFICode:      normalizeSpace(cells[5]),
			Remarks:      cellAt(cells, 6),
			ListedDate:   listedDate,
		})
	}
	stats.ParsedRows = len(out)
	// 非空但無法解析的上市日代表 TWSE 日期格式可能已變動——不讓單筆失敗中止整份解析，
	// 但要留下警示，避免格式漂移導致 listed_date 靜默全空。
	if malformedDates > 0 && log != nil {
		log.Warn("twse isin: unparseable listed-date cells", zap.Int("count", malformedDates), zap.Int("parsed", len(out)))
	}
	return out, stats, nil
}

func sampleTWSEISINRow(cells []string) string {
	sampleCells := cells
	if len(sampleCells) > 4 {
		sampleCells = sampleCells[:4]
	}
	s := strings.Join(sampleCells, " | ")
	if len(s) > 240 {
		return s[:240]
	}
	return s
}

func rowCells(row *html.Node) []string {
	cells := []string{}
	for child := row.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode || (child.Data != "td" && child.Data != "th") {
			continue
		}
		cells = append(cells, normalizeSpace(nodeText(child)))
	}
	return cells
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if cur.Type == html.TextNode {
			b.WriteString(cur.Data)
		}
		for child := cur.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return b.String()
}

func normalizeSpace(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, "\u3000", " ")
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

var symbolNamePattern = regexp.MustCompile(`^([0-9A-Z]+)\s+(.+)$`)

func parseSymbolName(raw string) (string, string, bool) {
	normalized := normalizeSpace(raw)
	matches := symbolNamePattern.FindStringSubmatch(normalized)
	if len(matches) != 3 {
		return "", "", false
	}
	return matches[1], matches[2], true
}

// parseListedDate 回傳 (解析結果, 是否為「非空但無法解析」的異常值)。空字串是合法的
// 「無上市日」，回 (零值, false)；非空卻解析失敗回 (零值, true) 供呼叫端計數告警。
func parseListedDate(raw string) (store.NullTime, bool) {
	normalized := normalizeSpace(raw)
	if normalized == "" {
		return store.NullTime{}, false
	}
	t, err := time.Parse("2006/01/02", normalized)
	if err != nil {
		return store.NullTime{}, true
	}
	return store.NullTime{NullTime: sql.NullTime{Time: t, Valid: true}}, false
}

func cellAt(cells []string, idx int) string {
	if idx >= len(cells) {
		return ""
	}
	return normalizeSpace(cells[idx])
}

func isTWSEISINHeader(cells []string) bool {
	if len(cells) < 2 {
		return false
	}
	first := normalizeSpace(cells[0])
	second := normalizeSpace(cells[1])
	return strings.Contains(first, "Security Code") ||
		strings.Contains(first, "有價證券") ||
		strings.Contains(second, "ISIN")
}
