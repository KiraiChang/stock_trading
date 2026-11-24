package bouncereplay

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// ── 自我檢查（實作補充第 1 點）──────────────────────────────────────────
//
// DB 判重沒有錨點（替身⛔ 不回任何列）、Redis 停用，用實際資料的同一組輸入送兩次相同身分的候選：
// 第一次要 BroadcastAttempted，第二次（15 分鐘內）要**被本機保留鎖擋下**（⛔ 不是被 DB 擋下）。

type noAnchorSignals struct{ store.SignalRepo }

func (noAnchorSignals) Insert(context.Context, *store.Signal) error { return nil }
func (noAnchorSignals) GetBySymbol(context.Context, string, int) ([]store.Signal, error) {
	return nil, nil
}
func (noAnchorSignals) LatestBySymbolTypeTimeframe(context.Context, string, string, string) (*store.Signal, error) {
	return nil, nil
}

type noopIndicators struct{ store.IndicatorRepo }

func (noopIndicators) Upsert(context.Context, *store.IndicatorSnapshot) error { return nil }

// SelfCheck 對一種規則做自我檢查，回傳用到的那一次評估的描述。
func SelfCheck(ctx context.Context, in *Input, policy signal.DedupPolicy, log *zap.Logger) (string, error) {
	cands := &cutoffCandles{all: in.Candles}
	var now time.Time
	eng := newEngine(cands, noAnchorSignals{}, noopIndicators{}, policy, func() time.Time { return now }, log)
	return selfCheck(ctx, in, eng, cands, &now, policy)
}

func selfCheck(ctx context.Context, in *Input, eng *signal.Engine, cands *cutoffCandles, nowp *time.Time, policy signal.DedupPolicy) (string, error) {
	now := nowp
	for _, e := range in.Evals {
		cands.cutoff, *now = e.LastBarTS, e.EvalAt
		first, err := eng.EvaluateWithOptions(ctx, e.Symbol, Timeframe, signal.EvaluateOptions{})
		if err != nil || !first.SignalGenerated {
			continue
		}
		if policy == signal.DedupIntradayBounceBySymbol && first.Signal.SignalType != "SUPPORT_BOUNCE" {
			continue // B 的保留鎖只在 SUPPORT_BOUNCE 走得到
		}
		desc := fmt.Sprintf("run %d %s %s（eval_at %s）", e.RunID, e.Symbol, first.Signal.SignalType, e.EvalAt.Format(time.RFC3339))
		if !first.BroadcastAttempted {
			return desc, fmt.Errorf("自我檢查失敗：第一次送出沒有 BroadcastAttempted（保留鎖會被釋放）")
		}
		*now = e.EvalAt.Add(time.Minute)
		second, err := eng.EvaluateWithOptions(ctx, e.Symbol, Timeframe, signal.EvaluateOptions{})
		if err != nil {
			return desc, fmt.Errorf("自我檢查失敗：第二次評估錯誤：%w", err)
		}
		if !second.Suppressed || second.SuppressedBy != signal.SuppressedByReservation {
			return desc, fmt.Errorf("自我檢查失敗：第二次（1 分鐘後）沒有被本機保留鎖擋下：suppressed=%v by=%q sent=%v",
				second.Suppressed, second.SuppressedBy, second.SignalGenerated)
		}
		return desc, nil
	}
	return "", fmt.Errorf("自我檢查做不了：輸入裡沒有任何一次評估會送出（規則 %d）", policy)
}

// CheckDatabaseName：三個資料庫的名稱都必須以 i140_ 開頭（隔離的 dev PostgreSQL 裡另建的 database），⛔ 不能指向 live 或一般 dev。
func CheckDatabaseName(name string) error {
	if !strings.HasPrefix(name, "i140_") {
		return fmt.Errorf("資料庫 %q 不是 i140_ 開頭，拒絕執行（⛔ 不能指向 live 或一般 dev 的資料庫）", name)
	}
	return nil
}

// ── 覆蓋率預檢（第 7.3 節）：只算指紋、⛔ 不跑引擎 ─────────────────────

type DayCount struct {
	Day                   string
	Evaluated, InputDiffs int
	Statuses              map[string]int
}

type PrecheckReport struct {
	Days     []DayCount
	Total    DayCount
	Short    []string // K 棒不足 120 根的評估
	Notes    []string // 訊號歸因的註記
	Failures []string // 不通過的原因（完整性與覆蓋率上限）
}

// OK：完整性通過，而且覆蓋率的上限達到門檻。
func (p PrecheckReport) OK() bool { return len(p.Failures) == 0 }

// dayCompleteness 依預定交易日清單檢查完整性：清單不能是空的、每一天都要有評估、清單外的日期⛔ 不能出現。
func dayCompleteness(expected []string, seen map[string]int) []string {
	var f []string
	if len(expected) == 0 {
		return append(f, "沒有預定交易日清單（-expected-days）")
	}
	want := map[string]bool{}
	for _, d := range expected {
		want[d] = true
		if seen[d] == 0 {
			f = append(f, "預定交易日 "+d+" 沒有任何評估（整天缺資料）")
		}
	}
	var extra []string
	for d := range seen {
		if !want[d] {
			extra = append(extra, d)
		}
	}
	sort.Strings(extra)
	for _, d := range extra {
		f = append(f, "評估出現在預定交易日清單之外的 "+d)
	}
	return f
}

// ParseExpectedDays 讀預定交易日清單：一行一個 YYYY-MM-DD（空行與 # 開頭的行略過），要在 [from, to] 之內、是平日、⛔ 不能重複、⛔ 不能是空的。
func ParseExpectedDays(r io.Reader, from, to time.Time) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		d, err := time.ParseInLocation("2006-01-02", line, timeutil.TaipeiTZ)
		if err != nil {
			return nil, fmt.Errorf("預定交易日清單格式不對：%q", line)
		}
		if d.Before(from) || d.After(to) {
			return nil, fmt.Errorf("預定交易日 %s 不在窗口 %s～%s 之內", line, from.Format("2006-01-02"), to.Format("2006-01-02"))
		}
		if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
			return nil, fmt.Errorf("預定交易日 %s 是週末", line)
		}
		if seen[line] {
			return nil, fmt.Errorf("預定交易日 %s 重複", line)
		}
		seen[line] = true
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("預定交易日清單是空的")
	}
	sort.Strings(out)
	return out, nil
}

func Precheck(in *Input) PrecheckReport {
	byDay := map[string]*DayCount{}
	rep := PrecheckReport{Total: DayCount{Day: "合計", Statuses: map[string]int{}}}
	for _, e := range in.Evals {
		d := byDay[day(e.DataAt)]
		if d == nil {
			d = &DayCount{Day: day(e.DataAt), Statuses: map[string]int{}}
			byDay[d.Day] = d
		}
		cs := latestUpTo(in.Candles[e.Symbol], e.LastBarTS, Lookback)
		if len(cs) < Lookback {
			rep.Short = append(rep.Short, fmt.Sprintf("run %d %s 只有 %d 根", e.RunID, e.Symbol, len(cs)))
		}
		for _, c := range []*DayCount{d, &rep.Total} {
			c.Evaluated++
			c.Statuses[e.EvalStatus]++
			if e.Fingerprint == "" || fingerprintAt(in.Candles[e.Symbol], e.LastBarTS) != e.Fingerprint {
				c.InputDiffs++
			}
		}
	}
	seen := map[string]int{}
	for _, d := range byDay {
		seen[d.Day] = d.Evaluated
	}
	for _, d := range in.ExpectedDays { // 缺的那一天也列出來（0 筆）
		if byDay[d] == nil {
			byDay[d] = &DayCount{Day: d, Statuses: map[string]int{}}
		}
	}
	for _, d := range byDay {
		rep.Days = append(rep.Days, *d)
	}
	sort.Slice(rep.Days, func(i, j int) bool { return rep.Days[i].Day < rep.Days[j].Day })
	rep.Notes = attribute(in).notes
	rep.Failures = dayCompleteness(in.ExpectedDays, seen)
	if rep.Total.Evaluated == 0 {
		rep.Failures = append(rep.Failures, "整個窗口沒有任何評估")
	}
	for _, d := range append(append([]DayCount{}, rep.Days...), rep.Total) {
		min := CoverageDayMin
		if d.Day == "合計" {
			min = CoverageTotalMin
		}
		if d.Evaluated > 0 && float64(d.Evaluated-d.InputDiffs) < min*float64(d.Evaluated) {
			rep.Failures = append(rep.Failures, fmt.Sprintf("%s 的覆蓋率上限（指紋相同 %d／%d）達不到門檻", d.Day, d.Evaluated-d.InputDiffs, d.Evaluated))
		}
	}
	return rep
}
