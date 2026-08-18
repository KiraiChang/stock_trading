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

## 尚未實作的判準維度

計畫書列的四個維度裡，**時間距離（超過 N 個交易日沒出現就不再匹配）刻意留到階段 B**：
它需要 `PreviousZone` 帶時間戳，而時間戳只有在有持久化之後才有意義的來源。

**這是呼叫端現在必須自己顧的風險**：`unmatched_previous` 的身分若被無限期留著等下次，
ATR 某天碰巧重算出相近的區間時，會把一個消失數月的 zone 復活。階段 B 接上持久化時
要一併補上這個維度。

完整數據與推導見 `docs/todo.md` T-048 階段 A。
"""
from __future__ import annotations

import uuid
from collections import defaultdict
from dataclasses import dataclass, field
from typing import Callable, Optional, Sequence

# ── 匹配判準（實測定案，改動前先看 docs/todo.md T-048 的門檻掃描表）──
MAX_CENTER_SHIFT_RATIO = 0.06
MAX_WIDTH_CHANGE_RATIO = 0.25

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

    def relations_by_child(self) -> dict[str, list[ZoneRelation]]:
        out: dict[str, list[ZoneRelation]] = defaultdict(list)
        for rel in self.relations:
            out[rel.child_zone_uid].append(rel)
        return dict(out)


def _center(zone) -> float:
    return (zone.price_low + zone.price_high) / 2.0


def _width(zone) -> float:
    return zone.price_high - zone.price_low


def is_same_zone(prev, cur) -> bool:
    """兩個 zone 是不是同一個。

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
) -> MatchResult:
    """把這次算出來的 zone 對應到既有身分，回傳身分指派與血緣關聯。

    **只有 `CONTINUE` 會延續身分。** SPLIT / MERGE / RESHAPE 的所有 child 都取得
    新的 `zone_uid`，parent 一律終止（列在 `terminated_previous`）——不讓某個 child
    「繼承」parent 的身分，那個選擇必然是任意的，而且會讓血緣圖說謊。

    `uid_factory` 可注入，讓測試不必面對隨機 UUID。
    """
    result = MatchResult(
        zone_uids=[""] * len(current),
        incarnation_roles=[None] * len(current),
    )
    accounted_prev: set[int] = set()

    for p_set, c_set, edges in _connected_components(previous, current):
        relation = _classify(len(p_set), len(c_set))

        if relation == RELATION_CONTINUE:
            (i,) = tuple(p_set)
            (j,) = tuple(c_set)
            prev, cur = previous[i], current[j]
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
        result.terminated_previous.extend(previous[i].zone_uid for i in sorted(p_set))

        # **只寫實際匹配上的邊**。元件連通不代表全交叉——鏈狀元件用笛卡爾積補邊，
        # 會產生 is_same_zone 明確否決過的父子關係，那正是「不猜血緣」要避免的事。
        for i, j in sorted(edges):
            result.relations.append(
                ZoneRelation(previous[i].zone_uid, result.zone_uids[j], relation)
            )

    result.unmatched_previous = [
        previous[i].zone_uid for i in range(len(previous)) if i not in accounted_prev
    ]
    return result
