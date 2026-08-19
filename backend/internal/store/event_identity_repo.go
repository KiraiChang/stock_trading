package store

import (
	"context"
	"database/sql"
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
	       e.last_zone_key, e.end_reason
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
		resolved_by, first_seen_at, last_seen_at, ended_at, last_zone_key, end_reason)
		VALUES (:event_uid, :symbol, :timeframe, :zone_uid, :zone_scope_key, :event_scope,
		:event_family, :seq, :root_event_type, :latest_event_type, :state, :active, :direction,
		:resolved_by, :first_seen_at, :last_seen_at, :ended_at, :last_zone_key, :end_reason)`
	// 幾個欄位的更新規則與 zone_instances 同樣是「錯了也不會有人發現」的那種：
	//
	//   * first_seen_at 不更新——鏈的起點是事實。
	//   * root_event_type 不更新——它是鏈的起點，被 latest 蓋掉就再也還原不回來。
	//   * ended_at 用 COALESCE 保護，忘了帶會讓已終結的鏈復活。
	//   * last_zone_key **要**更新——它記的是「最近一次」，停住就等於第一把鑰匙失效。
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
