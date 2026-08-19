"""Zone 跨交易日的身分匹配（T-048 階段 A）。

## 為什麼需要這個

現行的 zone 身分是 `event_engine._zone_key()`：

    f"{role}:{price_low:.4f}:{price_high:.4f}"

**身分綁在浮點邊界與角色上**，而 zone 邊界每次分析都由 ATR 重算、角色又會隨價格穿越
重新解析（見 `zone_builder` 模組 docstring）。2026-08-18 對 live 的盤點抓到兩種後果：

1. **同一個支撐分裂成兩條事件鏈**。0050 從 2026-08-05 起，`SUPPORT:102.4916:103.1084`
   與 `SUPPORT:102.5414:103.1585` 每天並存，價格區間重疊 99%，各自帶一條 reclaim 鏈。
   下游會重複計數，而且沒有任何東西會報錯。
2. **角色翻轉必然斷鏈**。實測有 IoU = 1.000 的翻轉——邊界一動也沒動，
   只因為 role 從 RESISTANCE 變 SUPPORT，key 就完全不同。

本模組給 zone 一個 **opaque 的 `zone_uid`**，讓身分跨越邊界漂移與角色翻轉。

## 判準為什麼不是 IoU

純 IoU 會被 zone 寬度污染：寬 zone 漂一點點 IoU 就掉很多。實測 543 組相鄰配對後改用
兩個**尺度無關**的量：

* 中心位移 ÷ 平均寬度 < 0.06
* 寬度變化率 < 0.25

**`IoU >= 0.90` 是它的嚴格子集**（實測：兩者交集 145 = `IoU >= 0.90` 的全部，
沒有任何一組是 IoU 收而位移判準不收）。位移判準在此門檻下**另外多收 16 組**。

0.06 是拐點而不是「越鬆越好」：放寬到 0.10 會多收到 38 組，但乾淨的一對一反而
從 144 掉到 136——它們被吸進無法解析血緣的 N→M 糾纏裡（N→M 從 3 組暴增到 10 組）。

## 資格閘門與幾何比對是兩件事

持久化之後，`previous` 不再是「上一次分析的 zone」而是**所有還活著的身分**，
所以需要一道閘門擋住「消失很久之後被 ATR 碰巧重算出相近區間而復活」。

**閘門只決定「這個身分還有沒有資格進入 matcher」，不參與幾何比對。**
`is_same_zone()` 是純幾何的，混進時間條件會讓「形狀像不像」與「還算不算數」
變成同一個判斷，兩者的失敗方式完全不同。

閘門有**兩個獨立的軸**，缺一不可：

| 軸 | 量什麼 | 與分析頻率的關係 |
|---|---|---|
| `MAX_ABSENCE_TRADING_DAYS` | wall-clock 陳舊度 | 無關 |
| `MAX_OBSERVED_ABSENCES` | 「我們看了幾次都沒看到」 | 相關 |

**為什麼兩個都要**：先前只有時間軸時，實測「缺席後又找回」的間隔一路拉到 22 天，
但那反映的是**分析頻率**而不是 zone 真的消失——2330 全期只有 4 次分析、橫跨 5 週。
單一時間軸分不出「zone 消失了」與「我們根本沒看」，加上觀測次數才分得出來。

**距離用交易日算，不是日曆天**：週末與假日不交易，週五→週一是 1 個交易日而不是 3 天。
交易日曆由呼叫端注入（`TradingCalendar`），沿用專案既有的做法——交易日從 candles 的
distinct 日期推得（見 `db.fetch_market_trading_days`），不引入外部假日表。

`as_of` 或 `last_seen_at` 缺一時不套用時間軸，讓還沒接上持久化的呼叫端行為不變；
`observed_absences` 預設 0，所以次數軸對它們也不會誤擋。

完整數據與推導見 `docs/todo.md` T-048 階段 A。
"""
from __future__ import annotations

import bisect
import uuid
from collections import defaultdict
from dataclasses import dataclass, field
from datetime import date
from typing import Callable, Optional, Sequence

# ── 幾何判準（實測定案，改動前先看 docs/sr-zone-scoring.md 的「門檻的來源」）──
MAX_CENTER_SHIFT_RATIO = 0.06
MAX_WIDTH_CHANGE_RATIO = 0.25

# ── 資格閘門。兩個軸都要通過才進得了候選集合 ──
# 缺席超過這麼多**交易日**就失格。20 個交易日約一個月，與先前的 30 日曆天同一個意圖，
# 但改用交易日之後不會被連假拉長。**這個值不是實測出來的**，理由見模組 docstring。
MAX_ABSENCE_TRADING_DAYS = 20
# 連續幾次「觀測到它不存在」就失格。**小於這個值才有資格**，所以 3 代表缺席第 3 次收攤。
MAX_OBSERVED_ABSENCES = 3

# 一世因為長期缺席而收攤時，寫進 zone_transitions 的原因碼。
# 與 INVALIDATED（被跌破/突破）不同：那是市場事件，這是「我們不再認得它」。
EXPIRED_BY_ABSENCE = "EXPIRED_BY_ABSENCE"

# ── 血緣關聯 ──
RELATION_CONTINUE = "CONTINUE"   # 1→1，身分延續（**不寫進 relations**，見 MatchResult）
RELATION_SPLIT = "SPLIT"         # 1→N
RELATION_MERGE = "MERGE"         # N→1
RELATION_RESHAPE = "RESHAPE"     # N→M，血緣無法解析

# ── role 轉換的三種語意。混為一談會讓真正的翻轉被 AT_ZONE 的雜訊淹沒 ──
ROLE_RESOLVED = "ROLE_RESOLVED"        # AT_ZONE → 有向
ROLE_UNRESOLVED = "ROLE_UNRESOLVED"    # 有向 → AT_ZONE
ROLE_FLIPPED = "ROLE_FLIPPED"          # SUPPORT ↔ RESISTANCE，真正的翻轉

ROLE_AT_ZONE = "AT_ZONE"

DIRECTIONAL_ROLES = frozenset({"SUPPORT", "RESISTANCE"})


@dataclass(frozen=True)
class TradingCalendar:
    """已知的交易日，升冪。

    **不自己判斷週末與假日**：台股的休市日不是「週末 ＋ 固定國定假日」那麼簡單
    （颱風假、補行交易日都會變動）。專案既有的做法是從 candles 的 distinct 日期推得
    真實交易日（`db.fetch_market_trading_days`），這裡沿用同一個事實來源。
    """

    days: tuple[date, ...]

    def __post_init__(self) -> None:
        """**必須升冪且是 date**——`sessions_between` 用 bisect 取序數差。

        這道檢查不是潔癖：本 class 指名的事實來源 `db.fetch_market_trading_days`
        回傳的是 **`ORDER BY d DESC` 的字串清單**。照著文件直接接上去有兩種壞法，
        而且都不會有人發現：
          * 丟字串進來 → bisect 比較 str 與 date 會 raise（這種還算好，至少會炸）
          * 轉成 date 但忘了 reverse → `hi - lo` 恆為 0 或負數，
            時間軸閘門**被靜默關掉**，正好是這一批要修的「消失數月的 zone 被接回來」。
        用 `from_iterable()` 建構就不必自己顧這件事。
        """
        for d in self.days:
            if not isinstance(d, date):
                raise TypeError(f"TradingCalendar.days 只收 date，收到 {type(d).__name__}")
        if list(self.days) != sorted(self.days):
            raise ValueError("TradingCalendar.days 必須升冪；fetch_market_trading_days 是降冪的")

    @classmethod
    def from_iterable(cls, days) -> "TradingCalendar":
        """從任意順序的 `date` 或 `YYYY-MM-DD` 字串建構，去重並升冪排序。

        `db.fetch_market_trading_days()` 的輸出可以直接丟進來。
        """
        parsed = {
            d if isinstance(d, date) else date.fromisoformat(str(d))
            for d in days
        }
        return cls(tuple(sorted(parsed)))

    def sessions_between(self, start: date, end: date) -> int:
        """`start` 之後到 `end`（含）之間有幾個交易日。

        兩個日期本身不必是交易日——分析可能落在盤後任何時間點。
        用二分搜尋取序數差，所以與曆法無關。
        """
        if end <= start:
            return 0
        lo = bisect.bisect_right(self.days, start)
        hi = bisect.bisect_right(self.days, end)
        return hi - lo


@dataclass(frozen=True)
class PreviousZone:
    """上一次分析的 zone，帶著已經確立的身分。

    `incarnation_role` 是**當前這一世的角色**，不是上次觀測到的 role：
    `AT_ZONE` 期間沿用上一個已解析的方向。這個區分讓「穿過 AT_ZONE 的翻轉」
    偵測得到——實測有一筆 `RESISTANCE → AT_ZONE → SUPPORT`，
    兩個有向角色並不相鄰，只比對相鄰兩次的 role 會漏掉它。

    **不必自己維護這個欄位**：`MatchResult.incarnation_roles` 已經算好下一輪該帶的值。
    """

    zone_uid: str
    price_low: float
    price_high: float
    method: str
    role: str
    incarnation_role: Optional[str] = None
    # 這個身分最後一次被觀測到的日期。`None` 代表呼叫端沒有持久化來源，
    # 此時不套用時間軸的閘門（行為與階段 A 相同）。
    last_seen_at: Optional[date] = None
    # 已經連續幾次「觀測到它不存在」。由 MatchResult.next_observed_absences 維護，
    # 呼叫端只負責存回去、不要自己加減——那種 bookkeeping 錯了是靜默的。
    observed_absences: int = 0


@dataclass(frozen=True)
class CandidateZone:
    """這次分析新算出來的 zone，還沒有身分。"""

    price_low: float
    price_high: float
    method: str
    role: str


@dataclass(frozen=True)
class ZoneRelation:
    parent_zone_uid: str
    child_zone_uid: str
    relation: str


@dataclass(frozen=True)
class RoleTransition:
    zone_uid: str
    kind: str
    from_role: Optional[str]
    to_role: str


@dataclass
class MatchResult:
    """`zone_uids[i]` 是 `current[i]` 這個候選 zone 被指派到的身分。

    `relations` **只含真實的血緣邊**（SPLIT / MERGE / RESHAPE），三個性質：

    * **沒有 CONTINUE 自環**。身分延續由「`zone_uids[i]` 是既有 uid」表達即可；
      寫成 `parent == child` 的邊會讓沿 parent 遞迴回溯祖先的查詢無法終止。
    * **只寫實際匹配上的邊**。元件常是鏈狀（P0–C0、P1–C0、P1–C1）而不是全交叉，
      對整個 `p_set × c_set` 做笛卡爾積會**憑空產生 `is_same_zone` 明確否決過的父子關係**。
    * parent 與 child 都用得到的終止資訊在 `terminated_previous`。
    """

    zone_uids: list[str] = field(default_factory=list)
    # incarnation_roles[i] 是 current[i] 下一輪該帶的 incarnation_role。
    # **由本模組算，不要讓每個呼叫端各自重寫**：漏掉「翻轉後要前進」會讓
    # RESISTANCE → SUPPORT → RESISTANCE 的第二次翻轉靜默消失。
    incarnation_roles: list[Optional[str]] = field(default_factory=list)
    relations: list[ZoneRelation] = field(default_factory=list)
    role_transitions: list[RoleTransition] = field(default_factory=list)
    # 這次沒有任何 child 的舊身分——由呼叫端決定是收掉還是留著等下次
    unmatched_previous: list[str] = field(default_factory=list)
    # 因為分裂／合併／重整而終止的舊身分。它們**有** child，但身分本身結束了，
    # 所以不會出現在 unmatched_previous；階段 B 要把這些寫成身分終態。
    terminated_previous: list[str] = field(default_factory=list)
    # 沒通過資格閘門、根本沒進候選集合的身分。**與 unmatched_previous 語意不同**：
    # unmatched 是「這次沒配到，下次還有機會」，expired 是「不再認得它了」。
    # 呼叫端要把這些的一世收成 EXPIRED、記 expired_at、寫一筆 EXPIRED_BY_ABSENCE。
    expired_previous: list[str] = field(default_factory=list)
    # 下一輪該存回 PreviousZone.observed_absences 的值。配到歸零、沒配到 +1。
    next_observed_absences: dict[str, int] = field(default_factory=dict)

    def relations_by_child(self) -> dict[str, list[ZoneRelation]]:
        out: dict[str, list[ZoneRelation]] = defaultdict(list)
        for rel in self.relations:
            out[rel.child_zone_uid].append(rel)
        return dict(out)


def _center(zone) -> float:
    return (zone.price_low + zone.price_high) / 2.0


def _width(zone) -> float:
    return zone.price_high - zone.price_low


def is_eligible(
    prev: PreviousZone,
    as_of: Optional[date] = None,
    calendar: Optional[TradingCalendar] = None,
    max_absence_trading_days: int = MAX_ABSENCE_TRADING_DAYS,
    max_observed_absences: int = MAX_OBSERVED_ABSENCES,
) -> bool:
    """這個既有身分**還有沒有資格進入 matcher 的候選集合**。

    **這裡不做任何形狀比對**——資格與相似度是兩件事，混在一起會讓
    「形狀像不像」與「還算不算數」變成同一個判斷。

    兩個軸任一超標就失格：

    * 缺席交易日數 > `max_absence_trading_days`
    * 連續觀測缺席次數 >= `max_observed_absences`（**小於才有資格**）

    缺 `as_of`／`last_seen_at`／`calendar` 任一項時不套用時間軸——
    還沒接上持久化的呼叫端行為與階段 A 相同。
    """
    if prev.observed_absences >= max_observed_absences:
        return False

    if as_of is None or prev.last_seen_at is None or calendar is None:
        return True

    return calendar.sessions_between(prev.last_seen_at, as_of) <= max_absence_trading_days


def is_same_zone(prev, cur) -> bool:
    """兩個 zone 的**形狀**是不是同一個。純幾何，不看時間也不看資格。

    **不比 role**：角色翻轉要能保持同一身分，那正是這個模組要解的問題之一。
    **比 method**：ATR zone 與 volume profile zone 是不同方法算出來的東西，
    價格碰巧重疊不代表是同一個結構。
    """
    if prev.method != cur.method:
        return False

    avg_width = (_width(prev) + _width(cur)) / 2.0
    if avg_width <= 0:
        # 退化成一個點的 zone：只有完全相同才算同一個，避免除以零時誤配。
        return prev.price_low == cur.price_low and prev.price_high == cur.price_high

    shift_ratio = abs(_center(prev) - _center(cur)) / avg_width
    width_change_ratio = abs(_width(cur) - _width(prev)) / avg_width
    return shift_ratio < MAX_CENTER_SHIFT_RATIO and width_change_ratio < MAX_WIDTH_CHANGE_RATIO


def _connected_components(
    previous: Sequence[PreviousZone], current: Sequence[CandidateZone]
) -> list[tuple[set[int], set[int], set[tuple[int, int]]]]:
    """把匹配關係當二部圖，取連通元件，**並保留元件內真實存在的邊**。

    **為什麼要取元件而不是逐對判斷**：血緣型別是元件的性質，不是單一邊的性質。
    同一條邊在 1→1 裡是 CONTINUE、在 2→2 裡是 RESHAPE。

    **為什麼要把邊帶出來**：元件是連通的，不代表元件內每個 parent 都連到每個 child。
    鏈狀元件（P0–C0、P1–C0、P1–C1）很常見，用笛卡爾積補邊會捏造 P0–C1。
    """
    adj: dict[int, set[int]] = {i: set() for i in range(len(previous))}
    radj: dict[int, set[int]] = {j: set() for j in range(len(current))}
    for i, prev in enumerate(previous):
        for j, cur in enumerate(current):
            if is_same_zone(prev, cur):
                adj[i].add(j)
                radj[j].add(i)

    components: list[tuple[set[int], set[int], set[tuple[int, int]]]] = []
    seen_prev: set[int] = set()
    seen_cur: set[int] = set()

    for start in range(len(previous)):
        if start in seen_prev:
            continue
        p_set = {start}
        c_set: set[int] = set()
        frontier_p = {start}
        while frontier_p:
            new_c: set[int] = set()
            for i in frontier_p:
                new_c |= adj[i] - c_set
            c_set |= new_c
            new_p: set[int] = set()
            for j in new_c:
                new_p |= radj[j] - p_set
            p_set |= new_p
            frontier_p = new_p
        seen_prev |= p_set
        seen_cur |= c_set
        edges = {(i, j) for i in p_set for j in adj[i]}
        components.append((p_set, c_set, edges))

    # 沒有連到任何舊 zone 的新 zone，各自成為一個「新生」元件
    for j in range(len(current)):
        if j not in seen_cur:
            components.append((set(), {j}, set()))

    return components


def _classify(p_count: int, c_count: int) -> Optional[str]:
    if p_count == 0 or c_count == 0:
        return None          # 新生或消失，沒有關聯邊
    if p_count == 1 and c_count == 1:
        return RELATION_CONTINUE
    if p_count == 1:
        return RELATION_SPLIT
    if c_count == 1:
        return RELATION_MERGE
    return RELATION_RESHAPE


def _incarnation_role_of(prev: PreviousZone) -> Optional[str]:
    if prev.incarnation_role:
        return prev.incarnation_role
    return prev.role if prev.role in DIRECTIONAL_ROLES else None


def _role_transition(
    zone_uid: str, prev: PreviousZone, cur: CandidateZone
) -> Optional[RoleTransition]:
    """身分延續時的 role 轉換分類。

    **翻轉的判定是「新解析出的方向 ≠ 當前這一世的 role」**，不是「相鄰兩次的 role 不同」。
    後者會漏掉穿過 AT_ZONE 的翻轉（實測有一筆 `RESISTANCE → AT_ZONE → SUPPORT`）。
    """
    incarnation_role = _incarnation_role_of(prev)

    if cur.role == ROLE_AT_ZONE:
        # 只有「從一個明確方向」進入 AT_ZONE 才算 UNRESOLVED。prev.role 是未知值時
        # 不臆測——`role` 從 DB 讀回來是自由字串，這一層要擋得住髒資料。
        if prev.role not in DIRECTIONAL_ROLES:
            return None
        return RoleTransition(zone_uid, ROLE_UNRESOLVED, prev.role, ROLE_AT_ZONE)

    if cur.role not in DIRECTIONAL_ROLES:
        return None          # 未知角色，不臆測

    if incarnation_role is None:
        # 這個身分第一次解析出方向
        return RoleTransition(zone_uid, ROLE_RESOLVED, prev.role, cur.role)

    if cur.role != incarnation_role:
        return RoleTransition(zone_uid, ROLE_FLIPPED, incarnation_role, cur.role)

    if prev.role == ROLE_AT_ZONE:
        # 從 AT_ZONE 回到原本的方向：是解析，不是翻轉
        return RoleTransition(zone_uid, ROLE_RESOLVED, ROLE_AT_ZONE, cur.role)

    return None


def match_zones(
    previous: Sequence[PreviousZone],
    current: Sequence[CandidateZone],
    uid_factory: Callable[[], str] = lambda: str(uuid.uuid4()),
    as_of: Optional[date] = None,
    calendar: Optional[TradingCalendar] = None,
    max_absence_trading_days: int = MAX_ABSENCE_TRADING_DAYS,
    max_observed_absences: int = MAX_OBSERVED_ABSENCES,
) -> MatchResult:
    """把這次算出來的 zone 對應到既有身分，回傳身分指派與血緣關聯。

    流程是**先過資格閘門、再做幾何比對**：失格的身分根本不進候選集合，
    列在 `expired_previous` 由呼叫端收攤（見 MatchResult）。

    **只有 `CONTINUE` 會延續身分。** SPLIT / MERGE / RESHAPE 的所有 child 都取得
    新的 `zone_uid`，parent 一律終止（列在 `terminated_previous`）——不讓某個 child
    「繼承」parent 的身分，那個選擇必然是任意的，而且會讓血緣圖說謊。

    `uid_factory` 可注入，讓測試不必面對隨機 UUID。
    """
    result = MatchResult(
        zone_uids=[""] * len(current),
        incarnation_roles=[None] * len(current),
    )

    # ── 資格閘門。這一步之後 previous 就只剩「還算數」的身分 ──
    eligible: list[PreviousZone] = []
    for prev in previous:
        if is_eligible(prev, as_of, calendar, max_absence_trading_days, max_observed_absences):
            eligible.append(prev)
        else:
            result.expired_previous.append(prev.zone_uid)

    accounted_prev: set[int] = set()

    for p_set, c_set, edges in _connected_components(eligible, current):
        relation = _classify(len(p_set), len(c_set))

        if relation == RELATION_CONTINUE:
            (i,) = tuple(p_set)
            (j,) = tuple(c_set)
            prev, cur = eligible[i], current[j]
            accounted_prev.add(i)
            result.zone_uids[j] = prev.zone_uid
            # 身分延續**不寫 relations**：那會是一條 parent == child 的自環邊。
            transition = _role_transition(prev.zone_uid, prev, cur)
            if transition is not None:
                result.role_transitions.append(transition)
            # 翻轉時前進到新角色；AT_ZONE 期間沿用這一世原本的角色。
            result.incarnation_roles[j] = (
                cur.role if cur.role in DIRECTIONAL_ROLES else _incarnation_role_of(prev)
            )
            continue

        # 新生、分裂、合併、重整：child 一律拿新身分，這一世從頭開始
        for j in sorted(c_set):
            result.zone_uids[j] = uid_factory()
            cur = current[j]
            result.incarnation_roles[j] = (
                cur.role if cur.role in DIRECTIONAL_ROLES else None
            )

        if relation is None:
            continue

        accounted_prev |= p_set
        result.terminated_previous.extend(eligible[i].zone_uid for i in sorted(p_set))

        # **只寫實際匹配上的邊**。元件連通不代表全交叉——鏈狀元件用笛卡爾積補邊，
        # 會產生 is_same_zone 明確否決過的父子關係，那正是「不猜血緣」要避免的事。
        for i, j in sorted(edges):
            result.relations.append(
                ZoneRelation(eligible[i].zone_uid, result.zone_uids[j], relation)
            )

    result.unmatched_previous = [
        eligible[i].zone_uid for i in range(len(eligible)) if i not in accounted_prev
    ]

    # 下一輪的缺席次數。**由本模組算**——讓呼叫端自己加減的話，算錯是靜默的
    # （比照 incarnation_roles 的教訓）。三種來源，缺一不可：
    #
    #   * 延續的身分 → 歸零
    #   * 這次沒配到但仍有資格 → +1
    #   * **失格的身分 → 也要 +1**。少了這一條，`ListLive` 的 `observed_absences <= 上限`
    #     會讓它每次分析都再被撈出來、再失格一次，於是 EXPIRED_BY_ABSENCE 會被重複寫入。
    #     +1 之後它就越過上限，從此不再進候選集合——這是與 repo 端的握手。
    #
    # **終止的 parent（SPLIT / MERGE / RESHAPE）刻意不列入**：它們的身分已經結束、
    # child 另取新 uid，呼叫端若照欄位說明遍歷整個 dict 去 upsert，會把剛判定終止的
    # 身分又寫回一筆 observed_absences=0 的活躍狀態。
    terminated = set(result.terminated_previous)
    matched_uids = {eligible[i].zone_uid for i in accounted_prev}
    for prev in eligible:
        if prev.zone_uid in terminated:
            continue
        result.next_observed_absences[prev.zone_uid] = (
            0 if prev.zone_uid in matched_uids else prev.observed_absences + 1
        )
    for prev in previous:
        if prev.zone_uid in result.expired_previous:
            result.next_observed_absences[prev.zone_uid] = prev.observed_absences + 1
    return result
