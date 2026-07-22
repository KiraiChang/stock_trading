package market

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"

	"github.com/trading/backend/internal/store"
)

const defaultTWSEISINTimeout = 30 * time.Second

type TWSEISINClient struct {
	urls []string
	http *http.Client
	log  *zap.Logger
}

// NewTWSEISINClient 接受一或多個 TWSE ISIN 來源（例如 strMode=2 上市、strMode=4 上櫃），
// FetchStockSymbols 會全部抓取並合併，任一來源失敗即整體失敗（避免只拿到半個市場就把
// 另一半誤判為下市）。
func NewTWSEISINClient(urls []string, log *zap.Logger) *TWSEISINClient {
	return &TWSEISINClient{
		urls: urls,
		http: &http.Client{
			Timeout: defaultTWSEISINTimeout,
		},
		log: log,
	}
}

func (c *TWSEISINClient) FetchStockSymbols(ctx context.Context) ([]store.StockSymbol, error) {
	if len(c.urls) == 0 {
		return nil, fmt.Errorf("twse isin: no sync urls configured")
	}
	merged := make([]store.StockSymbol, 0, 2048)
	seen := make(map[string]struct{})
	for _, url := range c.urls {
		symbols, err := c.fetchOne(ctx, url)
		if err != nil {
			// all-or-nothing：任一來源失敗就整體失敗，避免部分快照觸發誤下市。
			return nil, fmt.Errorf("twse isin fetch %s: %w", url, err)
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

func (c *TWSEISINClient) fetchOne(ctx context.Context, url string) ([]store.StockSymbol, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("twse isin: unexpected status %d", resp.StatusCode)
	}

	reader, err := charset.NewReader(resp.Body, resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	}
	return ParseTWSEISINSymbols(reader, c.log)
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
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
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
		symbol, name, ok := parseSymbolName(cells[0])
		if !ok {
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
	// 非空但無法解析的上市日代表 TWSE 日期格式可能已變動——不讓單筆失敗中止整份解析，
	// 但要留下警示，避免格式漂移導致 listed_date 靜默全空。
	if malformedDates > 0 && log != nil {
		log.Warn("twse isin: unparseable listed-date cells", zap.Int("count", malformedDates), zap.Int("parsed", len(out)))
	}
	return out, nil
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
