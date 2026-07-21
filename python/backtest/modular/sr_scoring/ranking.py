"""Zone ranking and overlap grouping helpers for SR Zone analysis."""
from __future__ import annotations

from typing import Optional

from .types import EvidenceFamily, Zone, ZoneMethod, ZoneScore, ZoneTier


def _assign_tiers(widths: list[float]) -> list[str]:
    """Assign zone tiers by relative width, preserving input order."""
    n = len(widths)
    if n == 0:
        return []
    order = sorted(range(n), key=lambda i: widths[i], reverse=True)
    third = -(-n // 3)
    tiers = [""] * n
    for rank, idx in enumerate(order):
        if rank < third:
            tiers[idx] = ZoneTier.TIER_1_MAIN_STRUCTURE.value
        elif rank < 2 * third:
            tiers[idx] = ZoneTier.TIER_2_TRADING_ZONE.value
        else:
            tiers[idx] = ZoneTier.TIER_3_SHORT_TERM.value
    return tiers


_TIER_ORDER = {
    ZoneTier.TIER_1_MAIN_STRUCTURE.value: 1,
    ZoneTier.TIER_2_TRADING_ZONE.value: 2,
    ZoneTier.TIER_3_SHORT_TERM.value: 3,
}


def _sort_zone_scores(zone_scores: list[ZoneScore]) -> list[ZoneScore]:
    return sorted(
        zone_scores, key=lambda z: (
            _TIER_ORDER.get(z.tier, 99),
            -z.trading_score,
            -(z.confluence_family_count or 1),
        )
    )


OVERLAP_GROUP_THRESHOLD = 0.6


def _zone_overlap_ratio(a: Zone, b: Zone) -> float:
    overlap = min(a.price_high, b.price_high) - max(a.price_low, b.price_low)
    if overlap <= 0:
        return 0.0
    return overlap / min(a.width, b.width)


def _evidence_family(method: str | ZoneMethod) -> str:
    value = method.value if isinstance(method, ZoneMethod) else str(method)
    return {
        ZoneMethod.ATR.value: EvidenceFamily.STRUCTURAL_ATR.value,
        ZoneMethod.VOLUME_PROFILE.value: EvidenceFamily.VOLUME_PROFILE.value,
        ZoneMethod.RECENT_PIVOT.value: EvidenceFamily.RECENT_MICROSTRUCTURE.value,
        ZoneMethod.BREAKDOWN_RECLAIM.value: EvidenceFamily.RECENT_MICROSTRUCTURE.value,
        ZoneMethod.VWAP_RECLAIM.value: EvidenceFamily.VWAP_OR_AVERAGE_RECLAIM.value,
    }.get(value, value.upper())


def _group_overlapping_zones(zones: list[Zone]) -> tuple[list[Optional[int]], list[int], list[int], list[tuple[str, ...]]]:
    n = len(zones)
    parent = list(range(n))

    def find(i: int) -> int:
        while parent[i] != i:
            parent[i] = parent[parent[i]]
            i = parent[i]
        return i

    def union(i: int, j: int) -> None:
        ri, rj = find(i), find(j)
        if ri != rj:
            parent[rj] = ri

    for i in range(n):
        for j in range(i + 1, n):
            if zones[i].method == zones[j].method:
                continue
            if _zone_overlap_ratio(zones[i], zones[j]) >= OVERLAP_GROUP_THRESHOLD:
                union(i, j)

    members_by_root: dict[int, list[int]] = {}
    for i in range(n):
        members_by_root.setdefault(find(i), []).append(i)

    overlap_group: list[Optional[int]] = [None] * n
    confluence_count: list[int] = [1] * n
    confluence_family_count: list[int] = [1] * n
    confluence_families: list[tuple[str, ...]] = [tuple([_evidence_family(z.method)]) for z in zones]
    group_id = 0
    for members in members_by_root.values():
        confluence = len(members)
        families = tuple(sorted({_evidence_family(zones[i].method) for i in members}))
        for i in members:
            confluence_count[i] = confluence
            confluence_family_count[i] = len(families)
            confluence_families[i] = families
        if confluence > 1:
            for i in members:
                overlap_group[i] = group_id
            group_id += 1

    return overlap_group, confluence_count, confluence_family_count, confluence_families
