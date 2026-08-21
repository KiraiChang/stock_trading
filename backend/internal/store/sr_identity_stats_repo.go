package store

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// SRIdentityStats 是**一次分析**的身分關聯決策拆解（todo.md T-050）。
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

	UnmatchedKeys      int `db:"unmatched_keys"      json:"unmatched_keys"`
	CarriedNoop        int `db:"carried_noop"        json:"carried_noop"`
	ZoneEndedSkipped   int `db:"zone_ended_skipped"  json:"zone_ended_skipped"`
	ChainConflicts     int `db:"chain_conflicts"     json:"chain_conflicts"`
	ChainKeyAmbiguous  int `db:"chain_key_ambiguous" json:"chain_key_ambiguous"`
	AliasAmbiguous     int `db:"alias_ambiguous"     json:"alias_ambiguous"`
	CarriedParseFail   int `db:"carried_parse_fail"  json:"carried_parse_fail"`

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
	// List 依條件取回，新的在前。
	List(ctx context.Context, q SRIdentityStatsQuery) ([]SRIdentityStats, error)
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

func (r *srIdentityStatsRepo) List(ctx context.Context, q SRIdentityStatsQuery) ([]SRIdentityStats, error) {
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
	limit := q.Limit
	if limit <= 0 {
		limit = 200
	}
	args = append(args, limit)

	var rows []SRIdentityStats
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT `+srIdentityStatsColumns+`
		FROM sr_identity_stats
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`), args...)
	return rows, err
}
