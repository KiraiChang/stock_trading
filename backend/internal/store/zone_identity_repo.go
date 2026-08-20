package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/trading/backend/pkg/timeutil"
)

// ZoneIdentityRepo 管理 zone 的跨交易日身分、一世與轉換（T-048 階段 B）。
//
// **本批只寫不讀**：沒有任何決策路徑會查這些表。既有的 market_event_states /
// market_event_detections 繼續並行寫入，等階段 C 有東西可比對之後再切換。
//
// 分三層的理由見 migration 067 的註解與 docs/sr-zone-scoring.md：身分跨越失效與角色翻轉，
// `INVALIDATED` 只是「這一世」的終態。
type ZoneIdentityRepo interface {
	// ListLive 撈這檔還有資格進 matcher 的身分，附帶當前這一世的角色——正好是
	// ZoneMatcher 需要的 `previous`。
	//
	// **只用次數軸過濾，刻意沒有時間下界。**
	//
	// 次數軸用 `<= maxObservedAbsences` 而**不是** `<`：用 `<` 的話，剛好累到上限的
	// 身分再也撈不出來 → 進不了 matcher → 不會出現在 expired_previous →
	// **沒有任何東西會把它收成 EXPIRED**，收攤流程整條變成不可達的死碼。
	// 放它進來一次，matcher 判失格、呼叫端收攤並把次數推過上限，下次就不會再進來。
	//
	// **時間軸完全交給 matcher**，這裡不加 last_seen_at 下界。加了的話同一個洞會從
	// 另一個軸出現：超過下界的身分被 SQL 擋在 matcher 之前，於是永遠不會被判失格、
	// 永遠不會收攤，就這樣以 state='ACTIVE' 與未結束的一世留在表裡。
	// 那不是理論問題——按需分析的標的隔超過三個月才再分析一次是常態。
	//
	// 不加下界也不會讓結果集變大：收攤時次數會被推過上限，所以死掉的身分下一次就被
	// 次數軸擋掉。單一 symbol 的活躍身分實測是十幾個。
	ListLive(ctx context.Context, symbol, timeframe string,
		maxObservedAbsences int) ([]LiveZone, error)
	// Apply 把一次分析的結果整批寫入，**單一交易**。
	//
	// 四張表必須一起成功：只寫了 instances 卻沒寫 relations，血緣圖就會出現無父的孤兒，
	// 而那與「這個 zone 是新生的」在資料上無法區分。
	Apply(ctx context.Context, w ZoneIdentityWrite) error
	// ListTradingDays 回傳最近 limit 個**市場交易日**（全庫 distinct 日期），由新到舊。
	//
	// matcher 的時間軸用交易日算距離，而**台股的休市日不是「週末 ＋ 固定國定假日」
	// 那麼簡單**（颱風假、補行交易日都會變動），所以不自己造日曆，
	// 從 candles 有資料的日子反推——與 Python 端 db.fetch_market_trading_days 同一個事實來源。
	//
	// **放在這個 repo 而不是 CandleRepo**：CandleRepo 有十處使用與多個測試替身，
	// 為單一消費者加方法要動一整圈。等到有第二個消費者再搬。
	//
	// 回傳 `YYYY-MM-DD` 字串而不是 time.Time：三個 engine 的日期型別掃描行為不一致
	// （sqlite 的 DATE() 回字串、postgres 的 ::date 回 time.Time），而下游是
	// Python 端點的 JSON 欄位、本來就要字串。在 SQL 就轉成文字，三邊行為才一致。
	ListTradingDays(ctx context.Context, timeframe string, limit int) ([]string, error)
	// ListKeyAliases 回傳這檔**還活著的身分歷來用過的 zone_key**，由新到舊
	// （T-048 階段 C 修法，F1 key 漂移）。
	//
	// 事件身上帶的 zone_key 是「上次那個 zone 長什麼樣」，而本次分析的 key 由這次的
	// ATR 邊界與 role 算出來——對不上是常態。實測 41 筆 ZONE scope 事件有 26 筆
	// 關聯失敗，兩個成因（role 進 AT_ZONE、邊界漂走）都是「身分還在，只是 key 到不了」。
	//
	// **只回 state='ACTIVE' 且 ended_at IS NULL 的身分**：把事件掛到收攤過的身分上
	// 比關聯失敗更糟——關聯失敗會進 warn 計數，掛錯身分不會有任何東西報錯。
	//
	// **次數軸與 ListLive 用同一個閘門**（`maxObservedAbsences`，呼叫端傳同一個常數）。
	// 少了它，這裡的「還活著」會比 matcher 寬：階段 B 的定案是失格只收掉「這一世」、
	// 身分本身仍是 `ACTIVE`，於是 matcher 早就放棄的身分照樣留在 alias 索引裡。
	// 只靠呼叫端排除本輪 `expired_previous` 補不起來——失格身分下一輪就被這裡的次數軸
	// 擋在 matcher 之前，所以它一生只會出現在 `expired_previous` **一次**，
	// 之後就永遠沉在索引裡（2026-08-19 每日階梯實測：77 筆 `alias_ambiguous`、
	// 16 個 key 撞號，加上這道過濾後歸零。見 docs/sr-zone-scoring.md「實測特性」）。
	//
	// 同一個 zone_key 對到多個活身分時**兩筆都回**，由呼叫端決定取捨並計數；
	// 在 SQL 裡靜靜挑一個會讓「有多少衝突」永遠問不出來。排序保證同一個 key 的
	// 最新者在前。
	ListKeyAliases(ctx context.Context, symbol, timeframe string,
		maxObservedAbsences int) ([]ZoneKeyAliasRef, error)
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
	ObservedAbsences int `db:"observed_absences"`
	// LastRole 是**上次觀測到的 role**，與「當前這一世的角色」是兩回事：
	// AT_ZONE 期間 LastRole 是 AT_ZONE，而一世的角色仍是上一個已解析的方向。
	// matcher 兩者都要——少了 LastRole 就分不出「這次才進 AT_ZONE」與
	// 「已經在 AT_ZONE 好幾次了」，前者該記 ROLE_UNRESOLVED，後者不該。
	LastRole string `db:"last_role"`
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
	IncarnationSeq  sql.NullInt64  `db:"incarnation_seq"`
	// IncarnationMaxSeq 是這個身分歷來的最大 seq（含已結束的）。
	// **不能用 IncarnationSeq 代替**：一世被 INVALIDATED 之後沒有未結束的一世，
	// 下一世要接在最大值之後，只看未結束的那筆會拿到 NULL 而重複用 seq=1，
	// 撞上 UNIQUE(zone_uid, seq)。
	IncarnationMaxSeq sql.NullInt64 `db:"incarnation_max_seq"`
}

// ZoneKeyAlias 是「這個身分曾經以這個 zone_key 被觀測到」的一筆歷史
// （T-048 階段 C 修法）。
//
// **刻意不做成 zone_instances 上的單一欄位**：那只記得住最後一次，而缺席容忍是
// 3 次、角色翻轉前後還要再回溯一段。更重要的是單一欄位會與這張表成為兩份事實，
// 分歧時沒有任何東西會報錯——那正是 T-048 一路在解的那類問題。
type ZoneKeyAlias struct {
	ZoneUID string `db:"zone_uid"`
	// ZoneKey 是 role:price_low:price_high，由 Python 的 zone_identity_key 產生，
	// 與 market_event_states.zone_key 同一個函數。Go 端只做字串比對，不重建它。
	ZoneKey     string    `db:"zone_key"`
	FirstSeenAt time.Time `db:"first_seen_at"`
	LastSeenAt  time.Time `db:"last_seen_at"`
}

// ZoneKeyAliasRef 是 ListKeyAliases 的回傳：一筆 zone_key → zone_uid 的對應。
type ZoneKeyAliasRef struct {
	ZoneKey    string    `db:"zone_key"`
	ZoneUID    string    `db:"zone_uid"`
	LastSeenAt time.Time `db:"last_seen_at"`
}

// ZoneKeyAliasLimit 是每個 zone_uid 保留的 alias 筆數上限。
//
// **一定要有上限**：zone 邊界每次分析都由 ATR 重算，不設限這張表會隨分析次數單調成長。
// 取 8 的理由是它要涵蓋得住回溯需求——缺席容忍 3 次（zone_matcher.MAX_OBSERVED_ABSENCES）
// 加上角色翻轉前後各一段，8 已經比實際需要寬。
const ZoneKeyAliasLimit = 8

// ZoneIdentityWrite 是一次分析要落地的全部異動。
type ZoneIdentityWrite struct {
	Instances    []ZoneInstance
	Incarnations []ZoneRoleIncarnation
	Transitions  []ZoneTransition
	Relations    []ZoneRelation
	// KeyAliases 是這次觀測到的 zone_key。與四張表在**同一個交易**內寫入：
	// alias 沒寫進去而身分寫了，下一次分析就少一把回溯的鑰匙，而那個缺失是靜默的。
	KeyAliases []ZoneKeyAlias
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

	// 交易日快取。key 是 timeframe，連同「算出來的那一天」一起存——
	// 跨日就失效重算。全市場共用的資料，不必每次分析都掃一次 candles。
	calendarMu    sync.Mutex
	calendarCache map[string]cachedCalendar
}

type cachedCalendar struct {
	day  string // YYYY-MM-DD（台北），跨日即失效
	days []string
}

func NewZoneIdentityRepo(db *sqlx.DB) ZoneIdentityRepo {
	return &zoneIdentityRepo{
		db: db, driver: db.DriverName(),
		calendarCache: map[string]cachedCalendar{},
	}
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
	       z.observed_absences, z.last_role, z.ended_at,
	       i.incarnation_uid AS incarnation_uid,
	       i.role            AS incarnation_role,
	       i.seq             AS incarnation_seq,
	       (SELECT MAX(i3.seq) FROM zone_role_incarnations i3
	         WHERE i3.zone_uid = z.zone_uid) AS incarnation_max_seq
	FROM zone_instances z
	LEFT JOIN zone_role_incarnations i
	       ON i.zone_uid = z.zone_uid AND i.ended_at IS NULL
	      AND i.seq = (SELECT MAX(i2.seq) FROM zone_role_incarnations i2
	                    WHERE i2.zone_uid = z.zone_uid AND i2.ended_at IS NULL)
	WHERE z.symbol = ? AND z.timeframe = ? AND z.state = 'ACTIVE'
	  AND z.observed_absences <= ?
	ORDER BY z.price_low`

func (r *zoneIdentityRepo) ListLive(
	ctx context.Context, symbol, timeframe string, maxObservedAbsences int,
) ([]LiveZone, error) {
	var out []LiveZone
	query := r.db.Rebind(listLiveZonesSQL)
	if err := r.db.SelectContext(ctx, &out, query,
		symbol, timeframe, maxObservedAbsences); err != nil {
		return nil, fmt.Errorf("zone identity: list live: %w", err)
	}
	return out, nil
}

// **時區一定要轉**：ts 存 UTC，而日 K 是以台北午夜寫入的（market/finmind.go 用
// TaipeiTZ 解析），也就是 UTC 前一日 16:00。直接取 DATE(ts) 會讓 2026-08-18 的盤
// 被報成 2026-08-17——`database-schema.md` 開頭就警告過這個「整整差一天」。
// postgres 的 to_char 還會依 session 的 TimeZone GUC 變動，同一句在不同伺服器設定下
// 結果不同，而 Python 端是釘死的，兩份「同一個事實來源」會分岔。
//
// 定義與 db.fetch_market_trading_days 逐字對齊：sqlite 的 ts 本來就是本地時間字串
// 所以直接取日期，postgres 明確轉 Asia/Taipei。
func (r *zoneIdentityRepo) tradingDaysSQL() string {
	switch r.driver {
	case "postgres", "pgx":
		return `SELECT DISTINCT to_char((ts AT TIME ZONE 'Asia/Taipei')::date, 'YYYY-MM-DD') AS d
		        FROM candles WHERE timeframe = ? AND ts >= ?
		        ORDER BY d DESC LIMIT ?`
	case "mysql":
		return `SELECT DISTINCT DATE_FORMAT(CONVERT_TZ(ts, '+00:00', '+08:00'), '%Y-%m-%d') AS d
		        FROM candles WHERE timeframe = ? AND ts >= ?
		        ORDER BY d DESC LIMIT ?`
	}
	return `SELECT DISTINCT DATE(ts) AS d FROM candles WHERE timeframe = ? AND ts >= ?
	        ORDER BY d DESC LIMIT ?`
}

// ListTradingDays 會**在行程內快取到當日結束**。
//
// `since` 幫不上效能：candles 唯一可用的索引是 (symbol, timeframe, ts DESC) 與
// UNIQUE(symbol, timeframe, ts)，兩者前導欄都是 symbol，所以 `timeframe = ?` 與
// `ts >= ?` 都無法 seek——DB 仍得掃過整張表再做 DISTINCT，`since` 只縮小了聚合的輸入。
// 而這條查詢跑在**每一次 POST /sr-zones** 裡。
//
// 市場交易日是全市場共用、一天最多變一次的東西，所以快取是最便宜的正解；
// 真要靠索引解決得加 (timeframe, ts)，那是對一張大表的 migration，不值得為這件事做。
func (r *zoneIdentityRepo) ListTradingDays(
	ctx context.Context, timeframe string, limit int,
) ([]string, error) {
	today := time.Now().In(timeutil.TaipeiTZ).Format("2006-01-02")
	r.calendarMu.Lock()
	if c, ok := r.calendarCache[timeframe]; ok && c.day == today && len(c.days) >= limit {
		defer r.calendarMu.Unlock()
		return append([]string(nil), c.days...), nil
	}
	r.calendarMu.Unlock()

	// 取 limit 個交易日最多需要回看多久：抓 3 倍日曆天當寬鬆上界（含週末與連假）。
	since := time.Now().UTC().AddDate(0, 0, -limit*3)
	var days []string
	query := r.db.Rebind(r.tradingDaysSQL())
	if err := r.db.SelectContext(ctx, &days, query, timeframe, since, limit); err != nil {
		return nil, fmt.Errorf("zone identity: list trading days: %w", err)
	}

	r.calendarMu.Lock()
	r.calendarCache[timeframe] = cachedCalendar{day: today, days: days}
	r.calendarMu.Unlock()
	return days, nil
}

func (r *zoneIdentityRepo) instanceUpsertSQL() string {
	const cols = `(zone_uid, symbol, timeframe, method, state, price_low, price_high,
		first_seen_at, last_seen_at, observed_absences, last_role, ended_at)
		VALUES (:zone_uid, :symbol, :timeframe, :method, :state, :price_low, :price_high,
		:first_seen_at, :last_seen_at, :observed_absences, :last_role, :ended_at)`
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
				observed_absences=VALUES(observed_absences), last_role=VALUES(last_role),
				ended_at=COALESCE(ended_at, VALUES(ended_at)),
				updated_at=CURRENT_TIMESTAMP`
	case "sqlite", "sqlite3":
		return `INSERT INTO zone_instances ` + cols + `
			ON CONFLICT(zone_uid) DO UPDATE SET
				state=excluded.state, price_low=excluded.price_low, price_high=excluded.price_high,
				last_seen_at=MAX(zone_instances.last_seen_at, excluded.last_seen_at),
				observed_absences=excluded.observed_absences, last_role=excluded.last_role, last_role=excluded.last_role,
				ended_at=COALESCE(zone_instances.ended_at, excluded.ended_at),
				updated_at=CURRENT_TIMESTAMP`
	}
	return `INSERT INTO zone_instances ` + cols + `
		ON CONFLICT(zone_uid) DO UPDATE SET
			state=excluded.state, price_low=excluded.price_low, price_high=excluded.price_high,
			last_seen_at=GREATEST(zone_instances.last_seen_at, excluded.last_seen_at),
			observed_absences=excluded.observed_absences, last_role=excluded.last_role,
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

// **只回活著的身分**，理由見介面註解。排序讓同一個 zone_key 的最新者在最前面，
// 呼叫端才能用「先到先得」的規則挑，而不是讓 SQL 的回傳順序決定。
const listZoneKeyAliasesSQL = `
	SELECT a.zone_key, a.zone_uid, a.last_seen_at
	FROM zone_key_aliases a
	JOIN zone_instances z ON z.zone_uid = a.zone_uid
	WHERE z.symbol = ? AND z.timeframe = ? AND z.state = 'ACTIVE' AND z.ended_at IS NULL
	  AND z.observed_absences <= ?
	ORDER BY a.zone_key, a.last_seen_at DESC, a.zone_uid`

func (r *zoneIdentityRepo) ListKeyAliases(
	ctx context.Context, symbol, timeframe string, maxObservedAbsences int,
) ([]ZoneKeyAliasRef, error) {
	var out []ZoneKeyAliasRef
	query := r.db.Rebind(listZoneKeyAliasesSQL)
	if err := r.db.SelectContext(ctx, &out, query,
		symbol, timeframe, maxObservedAbsences); err != nil {
		return nil, fmt.Errorf("zone identity: list key aliases: %w", err)
	}
	return out, nil
}

// alias 是「觀測到的事實」，重複觀測只推進 last_seen_at。
//
// **first_seen_at 不更新**：它回答「這個 key 從什麼時候開始代表這個身分」，
// 被覆寫就答不出來了。與 zone_instances.first_seen_at 同一個道理。
func (r *zoneIdentityRepo) keyAliasUpsertSQL() string {
	const cols = `(zone_uid, zone_key, first_seen_at, last_seen_at)
		VALUES (:zone_uid, :zone_key, :first_seen_at, :last_seen_at)`
	switch r.driver {
	case "mysql":
		return `INSERT INTO zone_key_aliases ` + cols + `
			ON DUPLICATE KEY UPDATE
				last_seen_at=GREATEST(last_seen_at, VALUES(last_seen_at))`
	case "sqlite", "sqlite3":
		return `INSERT INTO zone_key_aliases ` + cols + `
			ON CONFLICT(zone_uid, zone_key) DO UPDATE SET
				last_seen_at=MAX(zone_key_aliases.last_seen_at, excluded.last_seen_at)`
	}
	return `INSERT INTO zone_key_aliases ` + cols + `
		ON CONFLICT(zone_uid, zone_key) DO UPDATE SET
			last_seen_at=GREATEST(zone_key_aliases.last_seen_at, excluded.last_seen_at)`
}

// prune 只針對**本次寫過的 zone_uid**，不全表掃——沒動到的身分它的 alias 數本來就沒變。
//
// 子查詢多包一層 `t` 是 mysql 的要求（不能在同一句裡對目標表做 IN 子查詢）；
// postgres 與 sqlite 也吃得下這個寫法，所以三個 engine 共用一句。
// ORDER BY 補上 zone_key 是為了 last_seen_at 打平時仍然決定性——不然同一批資料
// 在不同 engine 上會 prune 掉不同的列。
const pruneZoneKeyAliasesSQL = `
	DELETE FROM zone_key_aliases
	WHERE zone_uid = ?
	  AND zone_key NOT IN (
	      SELECT t.zone_key FROM (
	          SELECT zone_key FROM zone_key_aliases
	          WHERE zone_uid = ?
	          ORDER BY last_seen_at DESC, zone_key
	          LIMIT ?
	      ) t
	  )`

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

	// alias 的主鍵是 (zone_uid, zone_key)，同一批重複時 postgres 一樣會炸
	// （ON CONFLICT DO UPDATE cannot affect row a second time），理由同上。
	// 這裡直接去重而不是報錯：兩個 method 算出完全一樣的區間並非不可能，
	// 那是合法輸入，不該讓整批寫入失敗。
	aliases := dedupeZoneKeyAliases(w.KeyAliases)

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
	// alias 排在身分之後：外鍵指向 zone_instances，新生的身分要先存在。
	if len(aliases) > 0 {
		if _, err := tx.NamedExecContext(ctx, r.keyAliasUpsertSQL(), aliases); err != nil {
			return fmt.Errorf("zone identity: upsert key aliases: %w", err)
		}
		prune := tx.Rebind(pruneZoneKeyAliasesSQL)
		for _, uid := range distinctAliasZoneUIDs(aliases) {
			if _, err := tx.ExecContext(ctx, prune, uid, uid, ZoneKeyAliasLimit); err != nil {
				return fmt.Errorf("zone identity: prune key aliases: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("zone identity: commit: %w", err)
	}
	return nil
}

// dedupeZoneKeyAliases 讓同一批裡的 (zone_uid, zone_key) 只留一筆，取 last_seen_at
// 較新的那個；first_seen_at 反過來取較舊的，因為它記的是起點。
func dedupeZoneKeyAliases(in []ZoneKeyAlias) []ZoneKeyAlias {
	if len(in) == 0 {
		return nil
	}
	idx := make(map[string]int, len(in))
	out := make([]ZoneKeyAlias, 0, len(in))
	for _, a := range in {
		if a.ZoneUID == "" || a.ZoneKey == "" {
			continue
		}
		k := a.ZoneUID + "|" + a.ZoneKey
		if i, dup := idx[k]; dup {
			if a.LastSeenAt.After(out[i].LastSeenAt) {
				out[i].LastSeenAt = a.LastSeenAt
			}
			if a.FirstSeenAt.Before(out[i].FirstSeenAt) {
				out[i].FirstSeenAt = a.FirstSeenAt
			}
			continue
		}
		idx[k] = len(out)
		out = append(out, a)
	}
	return out
}

// distinctAliasZoneUIDs 保持輸入順序，讓 prune 的執行順序在三個 engine 上一致。
func distinctAliasZoneUIDs(in []ZoneKeyAlias) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, a := range in {
		if _, dup := seen[a.ZoneUID]; dup {
			continue
		}
		seen[a.ZoneUID] = struct{}{}
		out = append(out, a.ZoneUID)
	}
	return out
}
