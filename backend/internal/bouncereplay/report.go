package bouncereplay

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/trading/backend/internal/store"
)

// 覆蓋率門檻（第 7.3 節）。
const (
	CoverageTotalMin = 0.90
	CoverageDayMin   = 0.80
)

type policyStats struct {
	Candidates, Sent, Suppressed, Withheld int
	ShortPairs, SameSupportPairs           int
}

// Metrics 是第 7.3 節的衡量口徑（一種範圍：全部重播的評估，或只算可對帳的）。
type Metrics struct {
	Scope string
	A, B  map[string]*policyStats // 依日期；"合計" 是總和
	// 被合併的不同支撐：B 抑制、A 會送出的 SUPPORT_BOUNCE 候選。
	Merged, MergedDiff, MergedOver1 int
	MergedPct                       []float64
	// 非目標訊號 A、B 送出集合的差異；Withhold 的回歸違規。
	NonTargetDiff []string
	WithholdViol  []string
}

type sendKey struct {
	symbol, day string
	at          time.Time
	support     float64
}

func computeMetrics(recs []Record, scope string, include func(Record) bool) Metrics {
	m := Metrics{Scope: scope, A: map[string]*policyStats{}, B: map[string]*policyStats{}}
	sends := map[string][]sendKey{"A": nil, "B": nil}
	nonTarget := map[string]map[string]int{"A": {}, "B": {}}
	stat := func(mp map[string]*policyStats, d string) []*policyStats {
		for _, k := range []string{d, "合計"} {
			if mp[k] == nil {
				mp[k] = &policyStats{}
			}
		}
		return []*policyStats{mp[d], mp["合計"]}
	}
	for _, r := range recs {
		if !include(r) {
			continue
		}
		for _, pol := range []struct {
			name string
			o    Outcome
			mp   map[string]*policyStats
		}{{"A", r.A, m.A}, {"B", r.B, m.B}} {
			ss := stat(pol.mp, r.Day)
			if pol.o.Candidate == nil {
				continue
			}
			if pol.o.Candidate.SignalType != "SUPPORT_BOUNCE" {
				if pol.o.Sent != nil {
					s := pol.o.Sent
					nonTarget[pol.name][fmt.Sprintf("%s|%s|%s|%.2f|%s", s.Symbol, s.SignalType, s.Direction, round2(s.Price), s.Timestamp.Format(time.RFC3339))]++
				}
				continue
			}
			for _, s := range ss {
				s.Candidates++
				switch pol.o.Status {
				case "signal_sent":
					s.Sent++
				case "signal_suppressed":
					s.Suppressed++
				case "withheld_stale":
					s.Withheld++
				}
			}
			if pol.o.Sent != nil {
				sends[pol.name] = append(sends[pol.name], sendKey{r.Symbol, r.Day, pol.o.Sent.CreatedAt, round2(pol.o.Sent.Support)})
			}
		}
		if r.BAnchorSupport != nil && r.A.Candidate != nil {
			m.Merged++
			c, a := round2(r.A.Candidate.Support), round2(*r.BAnchorSupport)
			if c != a {
				m.MergedDiff++
				pct := math.Abs(c-a) / a * 100
				m.MergedPct = append(m.MergedPct, pct)
				if pct > 1 {
					m.MergedOver1++
				}
			}
		}
		// 資料延遲（Withhold）時 B ⛔ 不能送出任何訊號：不論 live 記的是什麼、B 的狀態是不是 degraded，只看 B 有沒有送出（複審 2026-11-24）。
		if r.Withhold && r.B.Sent != nil {
			m.WithholdViol = append(m.WithholdViol, fmt.Sprintf("run %d %s：資料已延遲（live %s），B 卻送出（%s %s）", r.RunID, r.Symbol, r.EvalStatus, r.B.Status, r.B.Sent.SignalType))
		}
	}
	for name, mp := range map[string]map[string]*policyStats{"A": m.A, "B": m.B} {
		ss := sends[name]
		sort.Slice(ss, func(i, j int) bool {
			if ss[i].symbol != ss[j].symbol {
				return ss[i].symbol < ss[j].symbol
			}
			return ss[i].at.Before(ss[j].at)
		})
		for i := 1; i < len(ss); i++ {
			p, c := ss[i-1], ss[i]
			if p.symbol != c.symbol || p.day != c.day || c.at.Sub(p.at) >= Cooldown {
				continue
			}
			for _, s := range []*policyStats{mp[c.day], mp["合計"]} {
				s.ShortPairs++
				if p.support == c.support {
					s.SameSupportPairs++
				}
			}
		}
	}
	for k, n := range nonTarget["A"] {
		if nonTarget["B"][k] != n {
			m.NonTargetDiff = append(m.NonTargetDiff, fmt.Sprintf("%s：A %d 次、B %d 次", k, n, nonTarget["B"][k]))
		}
	}
	for k, n := range nonTarget["B"] {
		if _, ok := nonTarget["A"][k]; !ok {
			m.NonTargetDiff = append(m.NonTargetDiff, fmt.Sprintf("%s：A 0 次、B %d 次", k, n))
		}
	}
	sort.Strings(m.NonTargetDiff)
	sort.Float64s(m.MergedPct)
	return m
}

func quantile(xs []float64, q float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	return xs[int(math.Round(q*float64(len(xs)-1)))]
}

// Summary 是重播的結論：覆蓋率、分群、對帳、能不能進入比較。
type Summary struct {
	Days        []string
	Total       map[string]int            // 每日 live 實際評估的筆數（覆蓋率分母）
	Groups      map[string]map[string]int // 日期 → 群 → 筆數
	Matches     map[string]int
	Reasons     map[string]int
	Counted     map[string]int // 每日的覆蓋率分子（可對帳而且 A 跟 live 一致）
	Unexplained []Record
	Explained   []Record
	Abnormal    []Record
	NonTarget   []Record // 非目標訊號 A、B 不同的評估（已解釋與未解釋都列）
	CoverageOK  bool
	// Comparable：覆蓋率達到門檻、⛔ 沒有原因不明的不一致——才輸出 A／B 比較。
	Comparable bool
	// Pass：Comparable，而且 B 的回歸檢查都通過（B 的短時間重複 0、Withhold 違規 0、非目標差異全部證實來自最近 20 筆的邊界）。
	Pass     bool
	Failures []string
}

func Summarize(recs []Record, expected []string) Summary {
	s := Summary{Total: map[string]int{}, Groups: map[string]map[string]int{}, Matches: map[string]int{}, Reasons: map[string]int{}, Counted: map[string]int{}}
	for _, r := range recs {
		if s.Groups[r.Day] == nil {
			s.Groups[r.Day] = map[string]int{}
			s.Days = append(s.Days, r.Day)
		}
		s.Total[r.Day]++
		s.Groups[r.Day][r.Group]++
		if r.counted() {
			s.Counted[r.Day]++
		}
		if r.NonTargetDiff != "" {
			s.NonTarget = append(s.NonTarget, r)
		}
		if r.Group != GroupReconcilable && r.GroupReason != "" {
			for _, x := range strings.Split(r.GroupReason, "；") {
				s.Reasons[strings.TrimPrefix(x, "待重新核對：")]++
			}
		}
		if r.Group == GroupReconcilable {
			s.Matches[r.AMatch]++
			switch r.AMatch {
			case MatchUnexplained:
				s.Unexplained = append(s.Unexplained, r)
			case MatchClockBoundary:
				s.Explained = append(s.Explained, r)
			case MatchLiveAbnormal:
				s.Abnormal = append(s.Abnormal, r)
			}
		}
	}
	completeness := dayCompleteness(expected, s.Total)
	for _, d := range expected {
		if s.Groups[d] == nil {
			s.Groups[d] = map[string]int{}
			s.Days = append(s.Days, d)
		}
	}
	sort.Strings(s.Days)
	tot, rec := 0, 0
	s.CoverageOK = len(completeness) == 0
	s.Failures = append(s.Failures, completeness...)
	for _, d := range s.Days {
		tot += s.Total[d]
		rec += s.Counted[d]
		if float64(s.Counted[d]) < CoverageDayMin*float64(s.Total[d]) {
			s.CoverageOK = false
		}
	}
	if tot == 0 || float64(rec) < CoverageTotalMin*float64(tot) {
		s.CoverageOK = false
	}
	if !s.CoverageOK {
		s.Failures = append(s.Failures, "覆蓋率沒有達到門檻（合計 ≥ 90%、每日 ≥ 80%；分子⛔ 不含時鐘邊界與 live 評估異常）")
	}
	if len(s.Unexplained) > 0 {
		s.Failures = append(s.Failures, fmt.Sprintf("A 對 live 有 %d 筆原因不明的不一致", len(s.Unexplained)))
	}
	s.Comparable = s.CoverageOK && len(s.Unexplained) == 0
	if s.Comparable {
		m := computeMetrics(recs, "", func(Record) bool { return true })
		if b := m.B["合計"]; b != nil && b.ShortPairs > 0 {
			s.Failures = append(s.Failures, fmt.Sprintf("B 有 %d 對 15 分鐘內的重複送出（B 應為 0）", b.ShortPairs))
		}
		if len(m.WithholdViol) > 0 {
			s.Failures = append(s.Failures, fmt.Sprintf("B 有 %d 筆在資料延遲時送出（應扣住）", len(m.WithholdViol)))
		}
		n := 0
		for _, r := range s.NonTarget {
			if !r.NonTargetExplained {
				n++
			}
		}
		if n > 0 {
			s.Failures = append(s.Failures, fmt.Sprintf("非目標訊號 A、B 有 %d 筆差異無法證實來自最近 20 筆的邊界", n))
		}
	}
	s.Pass = s.Comparable && len(s.Failures) == 0
	return s
}

func pct(n, d int) string {
	if d == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", float64(n)*100/float64(d))
}

// WriteReport 輸出 markdown 報告。原因不明的不一致存在時，⛔ 不輸出 A／B 比較（第 7.3 節：停下來）。
func WriteReport(w io.Writer, recs []Record, expected, notes, selfCheck []string) Summary {
	s := Summarize(recs, expected)
	fmt.Fprintf(w, "# I-140 A／B 重播報告\n\n")
	for _, x := range selfCheck {
		fmt.Fprintf(w, "* 自我檢查：%s\n", x)
	}
	fmt.Fprintf(w, "\n## 覆蓋率與分群\n\n覆蓋率＝可對帳而且 A 跟 live 一致 ÷ live 實際評估；時鐘邊界與 live 評估異常留在分母、⛔ 不算進分子。\n\n| 日期 | live 評估 | 可對帳 | 其中一致 | 輸入已變 | 歷史分歧 | 歷史未確認 | 覆蓋率 |\n|---|---|---|---|---|---|---|---|\n")
	tot, recn := 0, 0
	for _, d := range s.Days {
		g := s.Groups[d]
		tot += s.Total[d]
		recn += s.Counted[d]
		fmt.Fprintf(w, "| %s | %d | %d | %d | %d | %d | %d | %s |\n", d, s.Total[d], g[GroupReconcilable], s.Counted[d], g[GroupInputChanged], g[GroupDiverged], g[GroupUnconfirmed], pct(s.Counted[d], s.Total[d]))
	}
	fmt.Fprintf(w, "| 合計 | %d | | %d | | | | %s |\n\n", tot, recn, pct(recn, tot))
	fmt.Fprintf(w, "* 門檻（合計 ≥ 90%%、每日 ≥ 80%%）：%v\n", map[bool]string{true: "達到", false: "⛔ 沒有達到"}[s.CoverageOK])
	fmt.Fprintf(w, "* 可對帳的 A 對 live：一致 %d、時鐘邊界（已解釋）%d、live 評估異常 %d、原因不明 %d\n",
		s.Matches[MatchOK], s.Matches[MatchClockBoundary], s.Matches[MatchLiveAbnormal], s.Matches[MatchUnexplained])
	var reasons []string
	for k, n := range s.Reasons {
		reasons = append(reasons, fmt.Sprintf("%s：%d", k, n))
	}
	sort.Strings(reasons)
	fmt.Fprintf(w, "\n### 非可對帳的原因（一筆可能有多個）\n\n")
	for _, x := range reasons {
		fmt.Fprintf(w, "* %s\n", x)
	}
	if len(notes) > 0 {
		fmt.Fprintf(w, "\n### 訊號歸因的註記\n\n")
		for _, x := range notes {
			fmt.Fprintf(w, "* %s\n", x)
		}
	}
	for _, sec := range []struct {
		title string
		rs    []Record
	}{{"原因不明的不一致（⛔ 停下）", s.Unexplained}, {"時鐘邊界（已解釋）", s.Explained}, {"live 評估異常", s.Abnormal}} {
		if len(sec.rs) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n### %s\n\n", sec.title)
		for _, r := range sec.rs {
			fmt.Fprintf(w, "* run %d %s %s：%s\n", r.RunID, r.Symbol, r.EvalAt.Format(time.RFC3339), r.MatchDetail)
		}
	}
	if len(s.NonTarget) > 0 {
		fmt.Fprintf(w, "\n### 非目標訊號 A、B 的差異（逐筆）\n\n")
		for _, r := range s.NonTarget {
			ex := "⛔ 無法證實來自最近 20 筆的邊界"
			if r.NonTargetExplained {
				ex = "已證實：最近 20 筆的邊界"
			}
			fmt.Fprintf(w, "* run %d %s %s %s：%s（%s）\n", r.RunID, r.Symbol, r.EvalAt.Format(time.RFC3339), nonTargetType(r.A, r.B), r.NonTargetDiff, ex)
		}
	}
	fmt.Fprintf(w, "\n## 判定\n\n")
	if s.Pass {
		fmt.Fprintf(w, "✅ 通過：覆蓋率達到門檻、⛔ 沒有原因不明的不一致、B 的回歸檢查都通過。\n")
	} else {
		for _, f := range s.Failures {
			fmt.Fprintf(w, "* ⛔ 不通過：%s\n", f)
		}
	}
	if !s.Comparable {
		fmt.Fprintf(w, "\n⛔ 不進入 A／B 比較（覆蓋率沒有達到門檻、或有原因不明的不一致）。\n")
		return s
	}
	for _, sc := range []struct {
		name    string
		include func(Record) bool
	}{{"全部重播的評估", func(Record) bool { return true }}, {"只算可對帳而且一致的評估", Record.counted}} {
		writeMetrics(w, computeMetrics(recs, sc.name, sc.include), s.Days)
	}
	return s
}

func writeMetrics(w io.Writer, m Metrics, days []string) {
	fmt.Fprintf(w, "\n## A／B 比較：%s\n\n| 日期 | A 候選 | A 送出 | A 抑制率 | A 短時間重複（同支撐） | B 候選 | B 送出 | B 抑制率 | B 短時間重複（同支撐） | 扣住 A／B |\n|---|---|---|---|---|---|---|---|---|---|\n", m.Scope)
	for _, d := range append(append([]string{}, days...), "合計") {
		a, b := m.A[d], m.B[d]
		if a == nil || b == nil {
			continue
		}
		fmt.Fprintf(w, "| %s | %d | %d | %s | %d（%d） | %d | %d | %s | %d（%d） | %d／%d |\n", d,
			a.Candidates, a.Sent, pct(a.Suppressed, a.Candidates), a.ShortPairs, a.SameSupportPairs,
			b.Candidates, b.Sent, pct(b.Suppressed, b.Candidates), b.ShortPairs, b.SameSupportPairs, a.Withheld, b.Withheld)
	}
	dist := "相距：—（沒有支撐不同的）"
	if len(m.MergedPct) > 0 {
		dist = fmt.Sprintf("相距 p10 %.3f%%、p50 %.3f%%、p90 %.3f%%、最大 %.3f%%", quantile(m.MergedPct, 0.1), quantile(m.MergedPct, 0.5), quantile(m.MergedPct, 0.9), quantile(m.MergedPct, 1))
	}
	fmt.Fprintf(w, "\n* 被合併的不同支撐（B 抑制、A 會送出）：%d 筆，其中支撐不同 %d 筆（%s），%s，超過 1%% 的 %d 筆\n",
		m.Merged, m.MergedDiff, pct(m.MergedDiff, m.Merged), dist, m.MergedOver1)
	fmt.Fprintf(w, "* 非目標訊號 A、B 的送出差異：%d 筆\n", len(m.NonTargetDiff))
	for _, x := range m.NonTargetDiff {
		fmt.Fprintf(w, "  * %s\n", x)
	}
	fmt.Fprintf(w, "* Withhold 的回歸違規：%d 筆\n", len(m.WithholdViol))
	for _, x := range m.WithholdViol {
		fmt.Fprintf(w, "  * %s\n", x)
	}
}

// WritePrecheck 輸出覆蓋率預檢的報告。
func WritePrecheck(w io.Writer, rep PrecheckReport) bool {
	fmt.Fprintf(w, "# I-140 覆蓋率預檢（只算指紋）\n\n| 日期 | live 評估 | 輸入已變 | 指紋相同（覆蓋率上限） |\n|---|---|---|---|\n")
	ok := rep.OK()
	for _, d := range append(append([]DayCount{}, rep.Days...), rep.Total) {
		same := d.Evaluated - d.InputDiffs
		fmt.Fprintf(w, "| %s | %d | %d | %s |\n", d.Day, d.Evaluated, d.InputDiffs, pct(same, d.Evaluated))
	}
	var st []string
	for k, n := range rep.Total.Statuses {
		st = append(st, fmt.Sprintf("%s %d", k, n))
	}
	sort.Strings(st)
	fmt.Fprintf(w, "\n* live 的評估結果：%s\n", strings.Join(st, "、"))
	fmt.Fprintf(w, "* 指紋相同的比例是覆蓋率的上限；門檻（合計 ≥ 90%%、每日 ≥ 80%%）與預定交易日的完整性：%v\n", map[bool]string{true: "通過", false: "⛔ 不通過，停下來回報"}[ok])
	for _, f := range rep.Failures {
		fmt.Fprintf(w, "  * ⛔ %s\n", f)
	}
	if len(rep.Short) > 0 {
		fmt.Fprintf(w, "* ⚠️ K 棒不足 %d 根的評估 %d 筆（暖身不夠）：%s\n", Lookback, len(rep.Short), strings.Join(rep.Short[:min(5, len(rep.Short))], "、"))
	}
	for _, x := range rep.Notes {
		fmt.Fprintf(w, "* %s\n", x)
	}
	return ok
}

// WriteCSV 輸出逐筆紀錄。
func WriteCSV(w io.Writer, recs []Record) error {
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"run_id", "symbol", "day", "data_at", "eval_at", "last_bar_ts", "withhold", "live_status", "live_cause",
		"group", "group_reason", "a_status", "a_by", "a_type", "a_support", "b_status", "b_by", "b_anchor_support", "a_match", "match_detail"})
	sig := func(s *store.Signal) (string, string) {
		if s == nil {
			return "", ""
		}
		return s.SignalType, fmt.Sprintf("%.2f", s.Support)
	}
	for _, r := range recs {
		at, as := sig(r.A.Candidate)
		anc := ""
		if r.BAnchorSupport != nil {
			anc = fmt.Sprintf("%.2f", *r.BAnchorSupport)
		}
		_ = cw.Write([]string{fmt.Sprint(r.RunID), r.Symbol, r.Day, r.DataAt.Format(time.RFC3339Nano), r.EvalAt.Format(time.RFC3339Nano),
			r.LastBarTS.Format(time.RFC3339), fmt.Sprint(r.Withhold), r.EvalStatus, r.EvalCause, r.Group, r.GroupReason,
			r.A.Status, r.A.SuppressedBy, at, as, r.B.Status, r.B.SuppressedBy, anc, r.AMatch, r.MatchDetail})
	}
	cw.Flush()
	return cw.Error()
}
