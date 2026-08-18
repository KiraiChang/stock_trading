package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// ZoneIdentityRepo 管理 zone 的跨交易日身分、一世與轉換（T-048 階段 B）。
//
// **本批只寫不讀**：沒有任何決策路徑會查這些表。既有的 market_event_states /
// market_event_detections 繼續並行寫入，等階段 C 有東西可比對之後再切換。
//
// 分三層的理由見 migration 067 的註解與 docs/todo.md T-048：身分跨越失效與角色翻轉，
// `INVALIDATED` 只是「這一世」的終態。
type ZoneIdentityRepo interface {
	// ListLive 撈這檔還有資格進 matcher 的身分，附帶當前這一世的角色——正好是
	// ZoneMatcher 需要的 `previous`。
	//
	// **這裡做的是粗篩，不是資格判定。** 兩個都刻意比 matcher 寬：
	//
	//   * 次數軸用 `<= maxObservedAbsences` 而**不是** `<`。用 `<` 的話，剛好累到上限的
	//     身分再也撈不出來 → 進不了 matcher → 不會出現在 expired_previous →
	//     **沒有任何東西會把它收成 EXPIRED**，收攤流程整條變成不可達的死碼。
	//     放它進來一次，matcher 判失格、呼叫端收攤並把次數 +1，下次就越過上限了。
	//   * 時間軸只用 notSeenSince 做寬鬆下界。精確的交易日距離必須由 matcher 用注入的
	//     交易日曆算——**SQL 裡沒有交易日的概念**，硬算會退回日曆天，週五→週一變 3 天。
	//     `notSeenSince` 傳得比 20 個交易日還緊的話，SQL 會比 matcher 嚴，
	//     同樣會讓身分被靜默丟掉而不收攤。
	ListLive(ctx context.Context, symbol, timeframe string, notSeenSince time.Time,
		maxObservedAbsences int) ([]LiveZone, error)
	// Apply 把一次分析的結果整批寫入，**單一交易**。
	//
	// 四張表必須一起成功：只寫了 instances 卻沒寫 relations，血緣圖就會出現無父的孤兒，
	// 而那與「這個 zone 是新生的」在資料上無法區分。
	Apply(ctx context.Context, w ZoneIdentityWrite) error
}

// ZoneInstance 是身分本身。`PriceLow`/`PriceHigh` 是最近一次觀測值，**不是身分**。
type ZoneInstance struct {
	ZoneUID     string       `db:"zone_uid"`
	Symbol      string       `db:"symbol"`
	Timeframe   string       `db:"timeframe"`
	Method      string       `db:"method"`
	State       string       `db:"state"`
	PriceLow    float64      `db:"price_low"`
	PriceHigh   float64      `db:"price_high"`
	FirstSeenAt time.Time    `db:"first_seen_at"`
	LastSeenAt  time.Time    `db:"last_seen_at"`
	// ObservedAbsences 是資格閘門的次數軸：連續幾次「觀測到它不存在」。
	// 由 matcher 的 next_observed_absences 維護，這裡只負責存回去。
	ObservedAbsences int          `db:"observed_absences"`
	EndedAt          sql.NullTime `db:"ended_at"`
}

// ZoneRoleIncarnation 是「一世」。`Role` 只會是 SUPPORT / RESISTANCE——
// AT_ZONE 是方向暫時無法解析，不開一世。
type ZoneRoleIncarnation struct {
	IncarnationUID string         `db:"incarnation_uid"`
	ZoneUID        string         `db:"zone_uid"`
	Seq            int            `db:"seq"`
	Role           string         `db:"role"`
	State          string         `db:"state"`
	StartedAt      time.Time      `db:"started_at"`
	EndedAt        sql.NullTime   `db:"ended_at"`
	// ExpiredAt 只在因長期缺席收攤時有值。與 EndedAt 分開：EndedAt 回答
	// 「這一世何時結束」，ExpiredAt 回答「何時被判定為不再認得」。
	ExpiredAt sql.NullTime   `db:"expired_at"`
	EndReason sql.NullString `db:"end_reason"`
}

// ZoneTransition 是狀態與角色轉換的流水。`IsIllegal` 為 true 時仍會寫入——
// 階段 B 沒有決策依賴這些資料，先看清楚現實會發生什麼比用猜想的規則擋掉證據重要。
type ZoneTransition struct {
	ZoneUID        string         `db:"zone_uid"`
	IncarnationUID sql.NullString `db:"incarnation_uid"`
	AnalysisID     sql.NullInt64  `db:"analysis_id"`
	TransitionKind string         `db:"transition_kind"`
	FromState      sql.NullString `db:"from_state"`
	ToState        sql.NullString `db:"to_state"`
	FromRole       sql.NullString `db:"from_role"`
	ToRole         sql.NullString `db:"to_role"`
	IsIllegal      bool           `db:"is_illegal"`
	ReasonCodes    RawJSON        `db:"reason_codes"`
	OccurredAt     time.Time      `db:"occurred_at"`
}

// ZoneRelation 是分裂／合併的血緣邊。**沒有 CONTINUE**：身分延續由 zone_uid 不變表達，
// 寫成自環會讓沿 parent 遞迴回溯祖先的查詢無法終止。
type ZoneRelation struct {
	ParentZoneUID string        `db:"parent_zone_uid"`
	ChildZoneUID  string        `db:"child_zone_uid"`
	Relation      string        `db:"relation"`
	AnalysisID    sql.NullInt64 `db:"analysis_id"`
	OccurredAt    time.Time     `db:"occurred_at"`
}

// LiveZone 是 ListLive 的回傳形狀：身分 ＋ 當前這一世的角色。
//
// `IncarnationRole` 為 NULL 代表這個身分還沒解析出方向（一直是 AT_ZONE）。
// matcher 靠它偵測穿過 AT_ZONE 的翻轉，所以**不能**用最近一次觀測到的 role 代替。
type LiveZone struct {
	ZoneInstance
	IncarnationUID  sql.NullString `db:"incarnation_uid"`
	IncarnationRole sql.NullString `db:"incarnation_role"`
}

// ZoneIdentityWrite 是一次分析要落地的全部異動。
type ZoneIdentityWrite struct {
	Instances    []ZoneInstance
	Incarnations []ZoneRoleIncarnation
	Transitions  []ZoneTransition
	Relations    []ZoneRelation
}

var (
	errZoneRelationSelfLoop = errors.New("zone relation: parent and child must differ")
	// 同一批出現重複 zone_uid 時，**只有 postgres 會炸**（ON CONFLICT DO UPDATE cannot
	// affect row a second time）；sqlite 逐列處理、mysql 的 ON DUPLICATE KEY UPDATE
	// 都吞得下去。測試只跑 sqlite，所以這個分歧測不出來卻只在正式環境的 engine 失敗——
	// 在 Go 這層明確擋掉，三個 engine 行為才一致。
	errZoneInstanceDuplicate = errors.New("zone identity: duplicate zone_uid in one batch")
)

type zoneIdentityRepo struct {
	db     *sqlx.DB
	driver string
}

func NewZoneIdentityRepo(db *sqlx.DB) ZoneIdentityRepo {
	return &zoneIdentityRepo{db: db, driver: db.DriverName()}
}

// 只撈身分還在（state='ACTIVE'）且最近仍出現過的。
//
// **一世取「未結束者中 seq 最大的那一筆」，不假設只有一筆。** schema 只有
// UNIQUE(zone_uid, seq)，**沒有**「每個身分最多一筆 ended_at IS NULL」的約束——
// 那種 partial unique index 在 mysql 上沒有對等寫法，三個 engine 給不起同一個保證。
// 少了這個子查詢，一旦出現兩筆未結束的一世（例如翻轉時漏關舊的），LEFT JOIN 會把同一個
// zone_uid 放大成兩列 → matcher 的 previous 出現同一身分兩次 → 本來 1→1 的 CONTINUE
// 被判成 2→1 的 MERGE → 身分被終止、child 另取新 uid，正是這個功能要防的斷鏈。
const listLiveZonesSQL = `
	SELECT z.zone_uid, z.symbol, z.timeframe, z.method, z.state,
	       z.price_low, z.price_high, z.first_seen_at, z.last_seen_at,
	       z.observed_absences, z.ended_at,
	       i.incarnation_uid AS incarnation_uid,
	       i.role            AS incarnation_role
	FROM zone_instances z
	LEFT JOIN zone_role_incarnations i
	       ON i.zone_uid = z.zone_uid AND i.ended_at IS NULL
	      AND i.seq = (SELECT MAX(i2.seq) FROM zone_role_incarnations i2
	                    WHERE i2.zone_uid = z.zone_uid AND i2.ended_at IS NULL)
	WHERE z.symbol = ? AND z.timeframe = ? AND z.state = 'ACTIVE'
	  AND z.last_seen_at >= ?
	  AND z.observed_absences <= ?
	ORDER BY z.price_low`

func (r *zoneIdentityRepo) ListLive(
	ctx context.Context, symbol, timeframe string, notSeenSince time.Time,
	maxObservedAbsences int,
) ([]LiveZone, error) {
	var out []LiveZone
	query := r.db.Rebind(listLiveZonesSQL)
	if err := r.db.SelectContext(ctx, &out, query,
		symbol, timeframe, notSeenSince, maxObservedAbsences); err != nil {
		return nil, fmt.Errorf("zone identity: list live: %w", err)
	}
	return out, nil
}

func (r *zoneIdentityRepo) instanceUpsertSQL() string {
	const cols = `(zone_uid, symbol, timeframe, method, state, price_low, price_high,
		first_seen_at, last_seen_at, observed_absences, ended_at)
		VALUES (:zone_uid, :symbol, :timeframe, :method, :state, :price_low, :price_high,
		:first_seen_at, :last_seen_at, :observed_absences, :ended_at)`
	// 三個欄位刻意**不是**單純覆寫，因為它們的錯誤都是靜默的：
	//
	//   * first_seen_at 完全不更新——身分第一次出現的時間是事實，重寫會讓身分壽命失真。
	//   * last_seen_at 取大的（**單調**）。一個「這次沒看到、只想把 absences +1」的寫入
	//     若順手填了本次分析時間，就等於宣告它剛被看到，**時間軸閘門從此永遠不會觸發**。
	//   * ended_at 用 COALESCE 保護既有值——忘了帶 EndedAt 的重複 upsert 會讓一個
	//     已終止的身分復活。
	//
	// last_seen_at 的取大寫法三個 engine 不同：mysql/postgres 是 GREATEST，
	// sqlite 沒有 GREATEST、用純量 MAX。這是唯一需要三分支的地方。
	switch r.driver {
	case "mysql":
		return `INSERT INTO zone_instances ` + cols + `
			ON DUPLICATE KEY UPDATE
				state=VALUES(state), price_low=VALUES(price_low), price_high=VALUES(price_high),
				last_seen_at=GREATEST(last_seen_at, VALUES(last_seen_at)),
				observed_absences=VALUES(observed_absences),
				ended_at=COALESCE(ended_at, VALUES(ended_at)),
				updated_at=CURRENT_TIMESTAMP`
	case "sqlite", "sqlite3":
		return `INSERT INTO zone_instances ` + cols + `
			ON CONFLICT(zone_uid) DO UPDATE SET
				state=excluded.state, price_low=excluded.price_low, price_high=excluded.price_high,
				last_seen_at=MAX(zone_instances.last_seen_at, excluded.last_seen_at),
				observed_absences=excluded.observed_absences,
				ended_at=COALESCE(zone_instances.ended_at, excluded.ended_at),
				updated_at=CURRENT_TIMESTAMP`
	}
	return `INSERT INTO zone_instances ` + cols + `
		ON CONFLICT(zone_uid) DO UPDATE SET
			state=excluded.state, price_low=excluded.price_low, price_high=excluded.price_high,
			last_seen_at=GREATEST(zone_instances.last_seen_at, excluded.last_seen_at),
			observed_absences=excluded.observed_absences,
			ended_at=COALESCE(zone_instances.ended_at, excluded.ended_at),
			updated_at=CURRENT_TIMESTAMP`
}

func (r *zoneIdentityRepo) incarnationUpsertSQL() string {
	const cols = `(incarnation_uid, zone_uid, seq, role, state, started_at,
		ended_at, expired_at, end_reason)
		VALUES (:incarnation_uid, :zone_uid, :seq, :role, :state, :started_at,
		:ended_at, :expired_at, :end_reason)`
	if r.driver == "mysql" {
		return `INSERT INTO zone_role_incarnations ` + cols + `
			ON DUPLICATE KEY UPDATE
				state=VALUES(state), ended_at=VALUES(ended_at),
				expired_at=VALUES(expired_at), end_reason=VALUES(end_reason)`
	}
	return `INSERT INTO zone_role_incarnations ` + cols + `
		ON CONFLICT(incarnation_uid) DO UPDATE SET
			state=excluded.state, ended_at=excluded.ended_at,
			expired_at=excluded.expired_at, end_reason=excluded.end_reason`
}

const insertZoneTransitionSQL = `
	INSERT INTO zone_transitions
		(zone_uid, incarnation_uid, analysis_id, transition_kind,
		 from_state, to_state, from_role, to_role, is_illegal, reason_codes, occurred_at)
	VALUES (:zone_uid, :incarnation_uid, :analysis_id, :transition_kind,
		:from_state, :to_state, :from_role, :to_role, :is_illegal, :reason_codes, :occurred_at)`

// 血緣邊是 append-only 的觀測紀錄，但同一次分析重跑要冪等——主鍵是
// (parent, child, occurred_at)，所以用 DO NOTHING 讓重跑不會撞主鍵。
func (r *zoneIdentityRepo) relationInsertSQL() string {
	const cols = `(parent_zone_uid, child_zone_uid, relation, analysis_id, occurred_at)
		VALUES (:parent_zone_uid, :child_zone_uid, :relation, :analysis_id, :occurred_at)`
	if r.driver == "mysql" {
		return `INSERT IGNORE INTO zone_relations ` + cols
	}
	return `INSERT INTO zone_relations ` + cols + ` ON CONFLICT DO NOTHING`
}

func (r *zoneIdentityRepo) Apply(ctx context.Context, w ZoneIdentityWrite) error {
	// 先擋自環：schema 有 CHECK，但那會在交易中途才炸，錯誤訊息也看不出是誰寫的。
	for _, rel := range w.Relations {
		if rel.ParentZoneUID == rel.ChildZoneUID {
			return fmt.Errorf("%w: %s", errZoneRelationSelfLoop, rel.ParentZoneUID)
		}
	}

	// 呼叫端要同時寫「這次匹配到的身分」與「這次缺席、次數要 +1 的身分」兩份清單，
	// 重疊很容易發生。與其讓它只在 postgres 上炸，不如在這裡明說。
	seen := make(map[string]struct{}, len(w.Instances))
	for _, inst := range w.Instances {
		if _, dup := seen[inst.ZoneUID]; dup {
			return fmt.Errorf("%w: %s", errZoneInstanceDuplicate, inst.ZoneUID)
		}
		seen[inst.ZoneUID] = struct{}{}
	}

	// RawJSON 是純 string，沒有 driver.Valuer——零值會把 '' 寫進 NOT NULL DEFAULT '[]'
	// 的欄位（DEFAULT 不會生效，因為欄位有被明確列在 INSERT 裡）。本批只寫不讀，
	// 這種 '' 會安靜累積，等階段 C 有人 json.Unmarshal 才炸。比照 sr_zone_repo 的做法。
	transitions := make([]ZoneTransition, len(w.Transitions))
	copy(transitions, w.Transitions)
	for i := range transitions {
		if transitions[i].ReasonCodes == "" {
			transitions[i].ReasonCodes = RawJSON("[]")
		}
	}
	w.Transitions = transitions

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("zone identity: begin: %w", err)
	}
	defer tx.Rollback()

	// 順序由外鍵決定：身分 → 一世 → 轉換／血緣。
	if len(w.Instances) > 0 {
		if _, err := tx.NamedExecContext(ctx, r.instanceUpsertSQL(), w.Instances); err != nil {
			return fmt.Errorf("zone identity: upsert instances: %w", err)
		}
	}
	if len(w.Incarnations) > 0 {
		if _, err := tx.NamedExecContext(ctx, r.incarnationUpsertSQL(), w.Incarnations); err != nil {
			return fmt.Errorf("zone identity: upsert incarnations: %w", err)
		}
	}
	if len(w.Transitions) > 0 {
		if _, err := tx.NamedExecContext(ctx, insertZoneTransitionSQL, w.Transitions); err != nil {
			return fmt.Errorf("zone identity: insert transitions: %w", err)
		}
	}
	if len(w.Relations) > 0 {
		if _, err := tx.NamedExecContext(ctx, r.relationInsertSQL(), w.Relations); err != nil {
			return fmt.Errorf("zone identity: insert relations: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("zone identity: commit: %w", err)
	}
	return nil
}
