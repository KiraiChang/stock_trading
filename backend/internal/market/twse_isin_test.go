package market

import (
	"strings"
	"testing"
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
