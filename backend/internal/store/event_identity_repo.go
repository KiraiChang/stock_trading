package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// EventIdentityRepo 管理事件鏈的身分與生命週期（T-048 階段 C）。
//
// **本批只寫不讀**：沒有任何決策路徑會查這兩張表。既有的 market_event_states /
// market_event_detections 繼續並行寫入，逐欄比對是驗收條件。
//
// 與 market_event_states 的關鍵差別是**鍵**：那邊用 event_engine._zone_key()
// （`role:price_low:price_high`，邊界每次由 ATR 重算），這裡用階段 B 做出來的
// 穩定 zone_uid。次要差別是生命週期在這裡是**存下來的事實**，而不是像
// internal/analysis/event_timeline.go 那樣在讀取時從歷史快照摺疊出來。
type EventIdentityRepo interface {
	// ListLatestChains 每個 (zone_scope_key, event_family) 回**最新的一條鏈**，
	// 不論它終結了沒有。
	//
	// **不能只撈未終結的鏈**：前一條 RESOLVED／EXPIRED 之後再出現同家族事件，那是
	// 新的一條鏈（規則與 Python 的 build_event_state_summary 對稱），新鏈的 seq 要從
	// 歷史最大值往上加。只看未終結的鏈會一直算出 seq=1 而撞 uq_event_instance_seq，
	// 而那個失敗是**靜默的**——寫入失敗只記 log。
	//
	// 呼叫端據此分流：`EndedAt` 無值就延續這條鏈，有值就用 `Seq + 1` 開新的。
	//
	// 候選集合會隨時間單調成長（zone_uid churn 會不斷產生新的 key）。目前不加時間
	// 下界：加了就可能漏看歷史最大 seq 而撞唯一索引，而那個代價比多撈幾十列高。
	// 單一 symbol 實測是數十列的量級，等真的變成問題再處理。
	ListLatestChains(ctx context.Context, symbol, timeframe string) ([]LiveEvent, error)
	// ListChains 取某檔某 timeframe 的**所有**事件鏈，供 Event Timeline 顯示
	// （原記於 todo.md T-051，已收斂）。
	//
	// **不能用 ListLatestChains 代替**：那支每個 (zone_scope_key, family) 只回最新一條，
	// 因為它服務的是寫入端的 seq 分派。timeline 要看的是演進，`seq` 較小的舊鏈同樣要顯示。
	//
	// since 為視窗起點，**單位是 K 棒時間（stock_sr_zone_analyses.analyzed_at）**。
	// 零值代表不設下界。
	//
	// **不能拿 event_instances 自己的時間欄位來比。** first_seen_at / last_seen_at /
	// occurred_at 存的是 as_of 的 **wall clock**（身分層的已知限制之一：那個時間軸量的是
	// 「我們看了幾次」而不是資料日期），而呼叫端拿到的視窗來自分析的 K 棒日期。
	// 兩者混用時條件會恆真——過濾看起來有寫，實際上什麼都沒擋掉。
	//
	// 納入條件是「**有一步落在視窗內，或這條鏈還沒結束**」。後半是必要的：一條長壽而
	// 這段期間剛好沒有狀態變化的鏈，在 timeline 上正是最該看到的東西。
	//
	// **視窗選的是「哪些鏈」，不是「鏈的哪幾步」**：被選中的鏈一律回完整歷史。
	// 把鏈從中間切開會失去誕生那一步，而 `from_state IS NULL 恰好等於鏈誕生` 是
	// event_transitions 的不變式——切一半的鏈會讓讀者以為它是從中途冒出來的。
	//
	// 依 first_seen_at 由舊到新排序，讓呼叫端不必再排。
	ListChains(ctx context.Context, symbol, timeframe string, since time.Time) ([]EventInstance, error)
	// ListTransitions 取這些鏈的所有轉換，**附帶該次分析的 K 棒時間**。
	//
	// 空的 eventUIDs 回空集合而不是「全部」——後者會在鏈清單為空時掃出整張表。
	ListTransitions(ctx context.Context, eventUIDs []string) ([]EventTransitionView, error)
	// GetIdentitySince 回這檔在身分層**最早有紀錄的時間**，供 Event Timeline 的
	// `identity_since` 使用（原記於 todo.md T-051 R5，已收斂）。沒有任何鏈時 `Valid=false`。
	//
	// **刻意不吃視窗參數。** 它要回答的是「身分層何時開始有紀錄」，而不是「這次查了
	// 多久」——拿 ListChains 的結果推導就會答成後者：視窗只保證未終結的鏈不受限制，
	// 視窗之前就終結的鏈會被濾掉，於是畫面會宣告「更早的分析沒有事件鏈」，
	// 實際上只是這次沒查到。
	//
	// 時間軸與 ListTransitions 一致，是**分析的 K 棒時間**：每條鏈取第一步所屬分析的
	// analyzed_at，沒有 analysis_id 才退回 occurred_at，完全沒有轉換的鏈才退回
	// first_seen_at。不能直接取 MIN(first_seen_at)——那是 as_of 的 wall clock，
	// 與對外的 K 棒軸不同軸（T-045 踩過）。
	GetIdentitySince(ctx context.Context, symbol, timeframe string) (sql.NullTime, error)
	// Apply 把一次分析的結果整批寫入，**單一交易**。
	//
	// 兩張表必須一起成功：只寫了 instances 卻沒寫 transitions，鏈的狀態會變成
	// 「現在是什麼」有紀錄、「怎麼走到這裡」沒有——而那正是本階段要消滅的情況。
	Apply(ctx context.Context, w EventIdentityWrite) error
}

// SymbolScopeKey 是 SYMBOL scope 事件在 zone_scope_key 欄位的值。
//
// zone_uid 可為 NULL（SYMBOL scope 的事件不屬於任何 zone），但**三個 engine 的
// UNIQUE 都不擋多個 NULL**，直接拿可空的 zone_uid 進唯一鍵等於沒有唯一性：
// 同一個 SYMBOL 事件可以無限重複建立而不報錯。所以另存這個 NOT NULL 的投影鍵。
const SymbolScopeKey = "SYMBOL"

// EventInstance 是一條事件鏈。`Seq` 表達同一個 (zone, family) 上的第幾條鏈。
type EventInstance struct {
	EventUID  string `db:"event_uid"`
	Symbol    string `db:"symbol"`
	Timeframe string `db:"timeframe"`
	// ZoneUID 為 NULL 時代表 SYMBOL scope 的事件。
	ZoneUID sql.NullString `db:"zone_uid"`
	// ZoneScopeKey 是 ZoneUID 的 NOT NULL 投影，見 SymbolScopeKey。
	ZoneScopeKey string `db:"zone_scope_key"`
	EventScope   string `db:"event_scope"`
	EventFamily  string `db:"event_family"`
	Seq          int    `db:"seq"`
	// RootEventType 是這條鏈的起點。**不可被新偵測蓋掉**，否則欄位名叫 root 卻永遠
	// 等於 latest，鏈的起點無法還原（T-045 踩過一次）。
	RootEventType   string         `db:"root_event_type"`
	LatestEventType string         `db:"latest_event_type"`
	State           string         `db:"state"`
	Active          bool           `db:"active"`
	Direction       string         `db:"direction"`
	ResolvedBy      sql.NullString `db:"resolved_by"`
	FirstSeenAt     time.Time      `db:"first_seen_at"`
	LastSeenAt      time.Time      `db:"last_seen_at"`
	EndedAt         sql.NullTime   `db:"ended_at"`
	// LastZoneKey 是這條鏈最近一次被觀測到時，**事件身上帶的** zone_key。
	//
	// **鏈延續的第一把鑰匙**（T-048 階段 C 修法，F1 chain silent freeze）。zone 邊界
	// 每次由 ATR 重算、role 也會翻轉，所以「事件帶的 key → 本次分析的 zone」對不上
	// 是常態（實測 41 筆有 26 筆）。用 (last_zone_key, event_family) 直接找回活鏈，
	// 就完全不必解析 key——那才是根治，把解析做得更聰明只是把失敗率壓低。
	//
	// **每次寫入都要更新成本次事件帶的 key**：停在誕生那天的值會讓這把鑰匙往後永遠 miss。
	LastZoneKey sql.NullString `db:"last_zone_key"`
	// EndReason：RESOLVED / EXPIRED / ZONE_IDENTITY_ENDED。
	EndReason sql.NullString `db:"end_reason"`
	// DecisionVisible 是這條鏈能不能被決策看到（階段 D 的隔離旗標）。
	//
	// **值一路來自 Python 的 event_engine.EVENT_TYPE_META**（經 state_json），
	// Go 只搬不推導——依 event_family 自己判斷等於維護第二份型別清單，
	// 兩份分歧時沒有任何東西會報錯。缺值一律 true，理由見 handler 的
	// eventDecisionVisible。
	DecisionVisible bool `db:"decision_visible"`
}

// EventTransition 是事件鏈的狀態轉換流水。
//
// **FromState 留白恰好等於「鏈誕生」**，這條不變式沿用 067 的定案。
type EventTransition struct {
	EventUID         string         `db:"event_uid"`
	AnalysisID       sql.NullInt64  `db:"analysis_id"`
	FromState        sql.NullString `db:"from_state"`
	ToState          string         `db:"to_state"`
	TriggerEventType sql.NullString `db:"trigger_event_type"`
	ReasonCodes      RawJSON        `db:"reason_codes"`
	OccurredAt       time.Time      `db:"occurred_at"`
}

// LiveEvent 是 ListLatestChains 的回傳：某個 (zone_scope_key, family) 上最新的一條鏈。
// `Seq` 就是該組用過的最大值，`EndedAt` 有值代表這條鏈已經結束、下一次要開新的。
type LiveEvent struct {
	EventInstance
}

// EventIdentityWrite 是一次分析要寫的兩張表內容。
type EventIdentityWrite struct {
	Instances   []EventInstance
	Transitions []EventTransition
}

type eventIdentityRepo struct {
	db     *sqlx.DB
	driver string
}

func NewEventIdentityRepo(db *sqlx.DB) EventIdentityRepo {
	return &eventIdentityRepo{db: db, driver: db.DriverName()}
}

const listLatestEventChainsSQL = `
	SELECT e.event_uid, e.symbol, e.timeframe, e.zone_uid, e.zone_scope_key,
	       e.event_scope, e.event_family, e.seq,
	       e.root_event_type, e.latest_event_type, e.state, e.active, e.direction,
	       e.resolved_by, e.first_seen_at, e.last_seen_at, e.ended_at,
	       e.last_zone_key, e.end_reason, e.decision_visible
	FROM event_instances e
	WHERE e.symbol = ? AND e.timeframe = ?
	  AND e.seq = (SELECT MAX(e2.seq) FROM event_instances e2
	                WHERE e2.symbol = e.symbol AND e2.timeframe = e.timeframe
	                  AND e2.zone_scope_key = e.zone_scope_key
	                  AND e2.event_family = e.event_family)
	ORDER BY e.zone_scope_key, e.event_family`

func (r *eventIdentityRepo) ListLatestChains(
	ctx context.Context, symbol, timeframe string,
) ([]LiveEvent, error) {
	var out []LiveEvent
	query := r.db.Rebind(listLatestEventChainsSQL)
	if err := r.db.SelectContext(ctx, &out, query, symbol, timeframe); err != nil {
		return nil, fmt.Errorf("event identity: list latest chains: %w", err)
	}
	return out, nil
}

func (r *eventIdentityRepo) instanceUpsertSQL() string {
	const cols = `(event_uid, symbol, timeframe, zone_uid, zone_scope_key, event_scope,
		event_family, seq, root_event_type, latest_event_type, state, active, direction,
		resolved_by, first_seen_at, last_seen_at, ended_at, last_zone_key, end_reason,
		decision_visible)
		VALUES (:event_uid, :symbol, :timeframe, :zone_uid, :zone_scope_key, :event_scope,
		:event_family, :seq, :root_event_type, :latest_event_type, :state, :active, :direction,
		:resolved_by, :first_seen_at, :last_seen_at, :ended_at, :last_zone_key, :end_reason,
		:decision_visible)`
	// 幾個欄位的更新規則與 zone_instances 同樣是「錯了也不會有人發現」的那種：
	//
	//   * first_seen_at 不更新——鏈的起點是事實。
	//   * root_event_type 不更新——它是鏈的起點，被 latest 蓋掉就再也還原不回來。
	//   * ended_at 用 COALESCE 保護，忘了帶會讓已終結的鏈復活。
	//   * last_zone_key **要**更新——它記的是「最近一次」，停住就等於第一把鑰匙失效。
	//   * decision_visible 跟著最近一次觀測更新——它是 latest_event_type 的性質，
	//     值一路來自 Python 的旗標。收尾路徑（沒有本次事件）不會走到這裡，
	//     所以已終結的鏈不會被寫回預設值。
	//
	// **state / active 用 CASE 綁在 ended_at 上**（T-048 階段 C 的 F4 修法）：
	// 這兩個欄位原本是無條件覆寫，於是「已終結的鏈被寫入非終態」時 ended_at 靠
	// COALESCE 保住了、state 卻退回 ACTIVE——就是 F4 那種「ended_at 有值卻 active=true」
	// 的自相矛盾資料，只是從另一個方向產生。純函數那層已經不會產出這種列，
	// 這道是不變式的最後一關，成本只有一個 CASE。
	switch r.driver {
	case "mysql":
		return `INSERT INTO event_instances ` + cols + `
			ON DUPLICATE KEY UPDATE
				latest_event_type=VALUES(latest_event_type),
				state=CASE WHEN event_instances.ended_at IS NOT NULL
				           THEN event_instances.state ELSE VALUES(state) END,
				active=CASE WHEN event_instances.ended_at IS NOT NULL
				            THEN FALSE ELSE VALUES(active) END,
				direction=VALUES(direction),
				resolved_by=VALUES(resolved_by),
				last_seen_at=GREATEST(last_seen_at, VALUES(last_seen_at)),
				ended_at=COALESCE(ended_at, VALUES(ended_at)),
				last_zone_key=VALUES(last_zone_key),
				end_reason=COALESCE(end_reason, VALUES(end_reason)),
				decision_visible=VALUES(decision_visible),
				updated_at=CURRENT_TIMESTAMP`
	case "sqlite", "sqlite3":
		return `INSERT INTO event_instances ` + cols + `
			ON CONFLICT(event_uid) DO UPDATE SET
				latest_event_type=excluded.latest_event_type,
				state=CASE WHEN event_instances.ended_at IS NOT NULL
				           THEN event_instances.state ELSE excluded.state END,
				active=CASE WHEN event_instances.ended_at IS NOT NULL
				            THEN 0 ELSE excluded.active END,
				direction=excluded.direction,
				resolved_by=excluded.resolved_by,
				last_seen_at=MAX(event_instances.last_seen_at, excluded.last_seen_at),
				ended_at=COALESCE(event_instances.ended_at, excluded.ended_at),
				last_zone_key=excluded.last_zone_key,
				end_reason=COALESCE(event_instances.end_reason, excluded.end_reason),
				decision_visible=excluded.decision_visible,
				updated_at=CURRENT_TIMESTAMP`
	}
	return `INSERT INTO event_instances ` + cols + `
		ON CONFLICT(event_uid) DO UPDATE SET
			latest_event_type=excluded.latest_event_type,
			state=CASE WHEN event_instances.ended_at IS NOT NULL
			           THEN event_instances.state ELSE excluded.state END,
			active=CASE WHEN event_instances.ended_at IS NOT NULL
			            THEN FALSE ELSE excluded.active END,
			direction=excluded.direction,
			resolved_by=excluded.resolved_by,
			last_seen_at=GREATEST(event_instances.last_seen_at, excluded.last_seen_at),
			ended_at=COALESCE(event_instances.ended_at, excluded.ended_at),
			last_zone_key=excluded.last_zone_key,
			end_reason=COALESCE(event_instances.end_reason, excluded.end_reason),
			decision_visible=excluded.decision_visible,
			updated_at=CURRENT_TIMESTAMP`
}

const insertEventTransitionSQL = `
	INSERT INTO event_transitions
		(event_uid, analysis_id, from_state, to_state, trigger_event_type,
		 reason_codes, occurred_at)
	VALUES (:event_uid, :analysis_id, :from_state, :to_state, :trigger_event_type,
		 :reason_codes, :occurred_at)`

func (r *eventIdentityRepo) Apply(ctx context.Context, w EventIdentityWrite) error {
	// 同一批出現重複 event_uid 時**只有 postgres 會炸**（ON CONFLICT DO UPDATE cannot
	// affect row a second time）；sqlite 逐列處理、mysql 的 ON DUPLICATE KEY UPDATE
	// 都吞得下去。測試只跑 sqlite，所以這個分歧測不出來卻只在正式環境的 engine 失敗。
	seen := make(map[string]struct{}, len(w.Instances))
	for _, inst := range w.Instances {
		if _, dup := seen[inst.EventUID]; dup {
			return fmt.Errorf("event identity: duplicate event_uid in one batch: %s", inst.EventUID)
		}
		seen[inst.EventUID] = struct{}{}
	}

	// RawJSON 是純 string，沒有 driver.Valuer——零值會把 '' 寫進 NOT NULL DEFAULT '[]'
	// 的欄位（欄位被明確列在 INSERT 裡，DEFAULT 不會生效）。本批只寫不讀，
	// 這種 '' 會安靜累積，等有人 json.Unmarshal 才炸。比照 zone_identity_repo。
	transitions := make([]EventTransition, len(w.Transitions))
	copy(transitions, w.Transitions)
	for i := range transitions {
		if transitions[i].ReasonCodes == "" {
			transitions[i].ReasonCodes = RawJSON("[]")
		}
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("event identity: begin: %w", err)
	}
	defer tx.Rollback()

	// 順序由外鍵決定：鏈 → 轉換。
	if len(w.Instances) > 0 {
		if _, err := tx.NamedExecContext(ctx, r.instanceUpsertSQL(), w.Instances); err != nil {
			return fmt.Errorf("event identity: upsert instances: %w", err)
		}
	}
	if len(transitions) > 0 {
		if _, err := tx.NamedExecContext(ctx, tx.Rebind(insertEventTransitionSQL), transitions); err != nil {
			return fmt.Errorf("event identity: insert transitions: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("event identity: commit: %w", err)
	}
	return nil
}

const listEventChainsSQL = `
	SELECT e.event_uid, e.symbol, e.timeframe, e.zone_uid, e.zone_scope_key,
	       e.event_scope, e.event_family, e.seq,
	       e.root_event_type, e.latest_event_type, e.state, e.active, e.direction,
	       e.resolved_by, e.first_seen_at, e.last_seen_at, e.ended_at,
	       e.last_zone_key, e.end_reason, e.decision_visible
	FROM event_instances e
	WHERE e.symbol = ? AND e.timeframe = ?
`

func (r *eventIdentityRepo) ListChains(
	ctx context.Context, symbol, timeframe string, since time.Time,
) ([]EventInstance, error) {
	query := listEventChainsSQL
	args := []any{symbol, timeframe}
	// 視窗比的是**分析的 K 棒時間**，不是 event_instances 自己的 wall-clock 欄位。
	// 詳見介面上的說明——混用會讓條件恆真。
	if !since.IsZero() {
		query += `
			AND (
				e.ended_at IS NULL
				OR EXISTS (
					SELECT 1 FROM event_transitions t
					JOIN stock_sr_zone_analyses a ON a.id = t.analysis_id
					WHERE t.event_uid = e.event_uid AND a.analyzed_at >= ?
				)
			)`
		args = append(args, since)
	}
	query += " ORDER BY e.first_seen_at ASC, e.event_uid ASC"
	var rows []EventInstance
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		return nil, err
	}
	return rows, nil
}

// identitySince 的兩支查詢（見介面上的 GetIdentitySince）。
//
// **與 analysis.BuildEventTimeline 的 firstSeen 規則同構**：優先用該步所屬分析的
// K 棒時間、沒有 analysis_id 退回 occurred_at、完全沒有轉換的鏈才退回 first_seen_at。
// 同一個時間點不能有兩套推導，否則 identity_since 會與 chains[0].first_seen_at
// 對不起來，而兩者本來就該落在同一條軸上。
//
// **不套任何視窗**：這正是這兩支查詢存在的理由。
//
// **為什麼是兩支查詢、而且 SELECT 只挑真欄位**：本來寫成一支
// `SELECT MIN(COALESCE(a.analyzed_at, t.occurred_at, e.first_seen_at))`，
// 但 sqlite 的 driver 只對「宣告型別是 DATETIME 的欄位」回 time.Time，
// **聚合／COALESCE 這種運算式沒有宣告型別，會回字串**，掃進 sql.NullTime 直接炸
// （`unsupported Scan, storing driver.Value type string into type *time.Time`）。
// 把運算式留在 ORDER BY、SELECT 只挑真欄位，三個 engine 就都不必解析字串。
//
// 「每條鏈取最早一步再取全域最小」等於「全域最早的那一步」，所以第一支不需要分組。
const identitySinceStepSQL = `
	SELECT a.analyzed_at AS analyzed_at, t.occurred_at AS occurred_at
	  FROM event_transitions t
	  JOIN event_instances e ON e.event_uid = t.event_uid
	  LEFT JOIN stock_sr_zone_analyses a ON a.id = t.analysis_id
	 WHERE e.symbol = ? AND e.timeframe = ?
	 ORDER BY COALESCE(a.analyzed_at, t.occurred_at) ASC
	 LIMIT 1`

// 只有 instances 沒有 transitions 是寫入端的異常（Apply 的單一交易本該擋掉），
// 但這種鏈仍要算進來——靜靜漏掉會讓「身分層從哪天開始有紀錄」答錯。
const identitySinceOrphanSQL = `
	SELECT e.first_seen_at
	  FROM event_instances e
	 WHERE e.symbol = ? AND e.timeframe = ?
	   AND NOT EXISTS (SELECT 1 FROM event_transitions t WHERE t.event_uid = e.event_uid)
	 ORDER BY e.first_seen_at ASC
	 LIMIT 1`

func (r *eventIdentityRepo) GetIdentitySince(
	ctx context.Context, symbol, timeframe string,
) (sql.NullTime, error) {
	var out sql.NullTime

	var step struct {
		AnalyzedAt sql.NullTime `db:"analyzed_at"`
		OccurredAt time.Time    `db:"occurred_at"`
	}
	err := r.db.GetContext(ctx, &step, r.db.Rebind(identitySinceStepSQL), symbol, timeframe)
	switch {
	case err == nil:
		out = sql.NullTime{Time: step.OccurredAt, Valid: true}
		if step.AnalyzedAt.Valid {
			out.Time = step.AnalyzedAt.Time
		}
	case errors.Is(err, sql.ErrNoRows):
		// 這檔沒有任何轉換——可能一條鏈都沒有，也可能只有沒轉換的鏈，交給下一支。
	default:
		return sql.NullTime{}, fmt.Errorf("event identity: identity since: %w", err)
	}

	var orphan time.Time
	err = r.db.GetContext(ctx, &orphan, r.db.Rebind(identitySinceOrphanSQL), symbol, timeframe)
	switch {
	case err == nil:
		if !out.Valid || orphan.Before(out.Time) {
			out = sql.NullTime{Time: orphan, Valid: true}
		}
	case errors.Is(err, sql.ErrNoRows):
		// 正常情況：每條鏈都有轉換。
	default:
		return sql.NullTime{}, fmt.Errorf("event identity: identity since (no-transition chains): %w", err)
	}

	return out, nil
}

// EventTransitionView 是 ListTransitions 的回傳：轉換本身 ＋ 它所屬分析的 K 棒時間。
//
// **為什麼要多帶這一欄**：`occurred_at` 是 as_of 的 wall clock，而同一份 timeline 裡的
// `snapshots` 用的是 K 棒日期。兩個軸混在一起畫，整條鏈會擠在「跑分析的那一刻」，
// 而不是它實際發生的那幾天。顯示層一律用這一欄；`analysis_id` 為 NULL（排程收尾）時
// 才退回 `occurred_at`。
type EventTransitionView struct {
	EventTransition
	AnalyzedAt sql.NullTime `db:"analyzed_at"`
}

func (r *eventIdentityRepo) ListTransitions(
	ctx context.Context, eventUIDs []string,
) ([]EventTransitionView, error) {
	// **空集合直接回**：交給 sqlx.In 會產生 `IN ()`，在 postgres 是語法錯誤，
	// 在 sqlite 則會掃出整張表——兩種都不是呼叫端要的。
	if len(eventUIDs) == 0 {
		return nil, nil
	}
	query, args, err := sqlx.In(`
		SELECT t.event_uid, t.analysis_id, t.from_state, t.to_state,
		       t.trigger_event_type, t.reason_codes, t.occurred_at,
		       a.analyzed_at
		FROM event_transitions t
		LEFT JOIN stock_sr_zone_analyses a ON a.id = t.analysis_id
		WHERE t.event_uid IN (?)
		ORDER BY t.event_uid ASC, t.occurred_at ASC, t.id ASC
	`, eventUIDs)
	if err != nil {
		return nil, err
	}
	var rows []EventTransitionView
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		return nil, err
	}
	return rows, nil
}
