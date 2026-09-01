package store

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// SRIdentityStats 是**一次分析**的身分關聯決策拆解（表設計見 docs/database-schema.md
// 的 sr_identity_stats）。
//
// 這些數字同時以結構化 log 輸出，但 log 答不出趨勢問題——「alias 命中率從 2% 爬到 30%」
// 代表 zone 邊界漂移在惡化，而沒有人會每天 grep。本表是那個問題的答案。
//
// **只存原始計數，不存比率**：比率的分母隨「要看哪個區間」而變，先算好等於把一個決定
// 寫死在資料裡。聚合由查詢端做。
type SRIdentityStats struct {
	ID         uint64 `db:"id"          json:"id"`
	AnalysisID uint64 `db:"analysis_id" json:"analysis_id"`
	Symbol     string `db:"symbol"      json:"symbol"`
	Timeframe  string `db:"timeframe"   json:"timeframe"`

	// 三段關聯決策各自命中幾次。優先序不可調換，見 sr-zone-scoring.md。
	MatchedByChain   int `db:"matched_by_chain"   json:"matched_by_chain"`
	MatchedByCurrent int `db:"matched_by_current" json:"matched_by_current"`
	MatchedByAlias   int `db:"matched_by_alias"   json:"matched_by_alias"`

	UnmatchedKeys     int `db:"unmatched_keys"      json:"unmatched_keys"`
	CarriedNoop       int `db:"carried_noop"        json:"carried_noop"`
	ZoneEndedSkipped  int `db:"zone_ended_skipped"  json:"zone_ended_skipped"`
	ChainConflicts    int `db:"chain_conflicts"     json:"chain_conflicts"`
	ChainKeyAmbiguous int `db:"chain_key_ambiguous" json:"chain_key_ambiguous"`
	AliasAmbiguous    int `db:"alias_ambiguous"     json:"alias_ambiguous"`
	CarriedParseFail  int `db:"carried_parse_fail"  json:"carried_parse_fail"`

	// InvariantViolations 與上面幾欄**語意不同**：上面問的是「分佈正不正常」，
	// 這一欄問的是「不變式有沒有被違反」，而它**必須恆為零**。不要混進同一組比率。
	InvariantViolations int `db:"invariant_violations" json:"invariant_violations"`

	// ZoneIdentityDegraded 為 true 時，這次分析的 zone 身分比對整個沒跑成，
	// 事件層也會跟著跳過——**其餘計數會全為 0，別把它讀成「這次很乾淨」**。
	ZoneIdentityDegraded  bool `db:"zone_identity_degraded"  json:"zone_identity_degraded"`
	EventIdentityDegraded bool `db:"event_identity_degraded" json:"event_identity_degraded"`
	// ZoneLiveCandidates 是這次進入 matcher 的既有身分數，供比率當分母參考。
	ZoneLiveCandidates int `db:"zone_live_candidates" json:"zone_live_candidates"`
	ZoneEnded          int `db:"zone_ended"           json:"zone_ended"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// SRIdentityStatsAggregate 是**整個查詢區間**的原始計數，供
// GET /sr-zones/identity-stats 的 summary 使用。
//
// **與 List 的關鍵差別：不套 limit。** limit 只限制明細列數，聚合一律涵蓋
// 完整的 WHERE 區間——否則 alias_hit_rate 的分母會由 limit 決定而不是由 days 決定，
// 那正是 2026-08-31 review 打回的那個 bug。
//
// **只放 SQL 直接算得出的原始計數**：matched_total 與 alias_hit_rate 是查詢端的
// derived view，由 handler 算。分母隨「要看哪個區間」而變，是查詢端的決定；資料層一旦
// 開始回傳 derived view，下一個人就會想把它存進表。**型別名刻意叫 Aggregate 不叫
// Summary**：後者與 handler 的 identityStatsSummary 只差大小寫，光靠註解擋不住
// 後人往這裡塞比率。
type SRIdentityStatsAggregate struct {
	Analyses int `db:"analyses"`
	// Degraded 是 zone 身分整段沒跑成的次數。**先看這個再看比率**——降級的那幾次
	// 其餘計數全是 0，會把比率稀釋成看起來很健康。
	Degraded int `db:"degraded_analyses"`

	MatchedByChain   int `db:"matched_by_chain"`
	MatchedByCurrent int `db:"matched_by_current"`
	MatchedByAlias   int `db:"matched_by_alias"`

	UnmatchedKeys     int `db:"unmatched_keys"`
	ChainConflicts    int `db:"chain_conflicts"`
	ChainKeyAmbiguous int `db:"chain_key_ambiguous"`
	AliasAmbiguous    int `db:"alias_ambiguous"`
	CarriedParseFail  int `db:"carried_parse_fail"`

	// InvariantViolations **必須是 0**。非零不是「比較差」而是不變式被違反，
	// 與其他欄位的語意完全不同，不要混進同一組比率。
	InvariantViolations int `db:"invariant_violations"`
}

// SRIdentityStatsQuery 是列表查詢條件。Symbol 為空代表全部標的。
type SRIdentityStatsQuery struct {
	Symbol string
	From   time.Time
	To     time.Time
	Limit  int
}

type SRIdentityStatsRepo interface {
	// Insert 寫入一次分析的拆解。呼叫端一律 fail-open：寫入失敗只記 log，
	// 分析本身照常成立——統計缺一列比分析失敗好。
	Insert(ctx context.Context, s *SRIdentityStats) error
	// List 依條件取回，新的在前。**Limit 只截斷明細**，不影響 Summarize。
	List(ctx context.Context, q SRIdentityStatsQuery) ([]SRIdentityStats, error)
	// Summarize 回傳同一組條件下**未套 Limit** 的聚合。與 List 分成兩次查詢，
	// 所以兩者可能落在微幅不同的 snapshot——這是刻意接受的取捨，見 api-reference.md。
	Summarize(ctx context.Context, q SRIdentityStatsQuery) (SRIdentityStatsAggregate, error)
}

type srIdentityStatsRepo struct {
	db *sqlx.DB
}

func NewSRIdentityStatsRepo(db *sqlx.DB) SRIdentityStatsRepo {
	return &srIdentityStatsRepo{db: db}
}

func (r *srIdentityStatsRepo) Insert(ctx context.Context, s *SRIdentityStats) error {
	_, err := r.db.NamedExecContext(ctx, r.db.Rebind(`
		INSERT INTO sr_identity_stats (
			analysis_id, symbol, timeframe,
			matched_by_chain, matched_by_current, matched_by_alias,
			unmatched_keys, carried_noop, zone_ended_skipped,
			chain_conflicts, chain_key_ambiguous, alias_ambiguous,
			carried_parse_fail, invariant_violations,
			zone_identity_degraded, event_identity_degraded,
			zone_live_candidates, zone_ended
		) VALUES (
			:analysis_id, :symbol, :timeframe,
			:matched_by_chain, :matched_by_current, :matched_by_alias,
			:unmatched_keys, :carried_noop, :zone_ended_skipped,
			:chain_conflicts, :chain_key_ambiguous, :alias_ambiguous,
			:carried_parse_fail, :invariant_violations,
			:zone_identity_degraded, :event_identity_degraded,
			:zone_live_candidates, :zone_ended
		)
	`), s)
	return err
}

const srIdentityStatsColumns = `
	id, analysis_id, symbol, timeframe,
	matched_by_chain, matched_by_current, matched_by_alias,
	unmatched_keys, carried_noop, zone_ended_skipped,
	chain_conflicts, chain_key_ambiguous, alias_ambiguous,
	carried_parse_fail, invariant_violations,
	zone_identity_degraded, event_identity_degraded,
	zone_live_candidates, zone_ended, created_at`

// srIdentityStatsWhere 是 List 與 Summarize **唯一**的過濾條件來源。
//
// ⛔ **兩邊不准各自拼一份 SQL。** 之後有人加一個過濾條件只改一邊，rows 與 summary
// 就會悄悄對應到不同母體——那與這個修法要解的 bug 是同一類「不會報錯的失真」。
func srIdentityStatsWhere(q SRIdentityStatsQuery) (string, []any) {
	where := []string{"1=1"}
	args := []any{}
	if q.Symbol != "" {
		where = append(where, "symbol = ?")
		args = append(args, q.Symbol)
	}
	if !q.From.IsZero() {
		where = append(where, "created_at >= ?")
		args = append(args, q.From)
	}
	if !q.To.IsZero() {
		where = append(where, "created_at <= ?")
		args = append(args, q.To)
	}
	return strings.Join(where, " AND "), args
}

func (r *srIdentityStatsRepo) List(ctx context.Context, q SRIdentityStatsQuery) ([]SRIdentityStats, error) {
	where, args := srIdentityStatsWhere(q)
	limit := q.Limit
	if limit <= 0 {
		limit = 200
	}
	args = append(args, limit)

	var rows []SRIdentityStats
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT `+srIdentityStatsColumns+`
		FROM sr_identity_stats
		WHERE `+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`), args...)
	return rows, err
}

// Summarize 走與 List 同一組 WHERE，但**沒有 LIMIT**。
//
// 只用 COUNT(*) 與 COALESCE(SUM(...), 0)，布林計數走 CASE WHEN——postgres 的 BOOLEAN、
// sqlite 的 0/1、mysql 的 TINYINT 三者都成立。**但只有 sqlite 被實際測過**（issue.md I-054）。
//
// 空集合時 COUNT(*) 回 0、SUM 回 NULL 由 COALESCE 收成 0，整個 aggregate 全零。
func (r *srIdentityStatsRepo) Summarize(ctx context.Context, q SRIdentityStatsQuery) (SRIdentityStatsAggregate, error) {
	where, args := srIdentityStatsWhere(q)

	var agg SRIdentityStatsAggregate
	err := r.db.GetContext(ctx, &agg, r.db.Rebind(`
		SELECT
			COUNT(*) AS analyses,
			COALESCE(SUM(CASE WHEN zone_identity_degraded THEN 1 ELSE 0 END), 0) AS degraded_analyses,
			COALESCE(SUM(matched_by_chain), 0)     AS matched_by_chain,
			COALESCE(SUM(matched_by_current), 0)   AS matched_by_current,
			COALESCE(SUM(matched_by_alias), 0)     AS matched_by_alias,
			COALESCE(SUM(unmatched_keys), 0)       AS unmatched_keys,
			COALESCE(SUM(chain_conflicts), 0)      AS chain_conflicts,
			COALESCE(SUM(chain_key_ambiguous), 0)  AS chain_key_ambiguous,
			COALESCE(SUM(alias_ambiguous), 0)      AS alias_ambiguous,
			COALESCE(SUM(carried_parse_fail), 0)   AS carried_parse_fail,
			COALESCE(SUM(invariant_violations), 0) AS invariant_violations
		FROM sr_identity_stats
		WHERE `+where), args...)
	return agg, err
}
