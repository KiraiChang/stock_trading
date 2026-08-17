"""SR volatility 回歸基準的比對（計畫書階段 6）。

**為什麼不比絕對值**：`atr_pct` 取近 60 根，時間前進窗口就滾動；`adj_factor` 重算
（例如 2026-08-11／12 的股價還原）也會改變還原價。實測 `6243` 在 11 天內從 11.6%
掉到 8.60%——那是預期，不是壞掉。所以「數值與上次完全相同」是**不可檢驗的命題**，
原本的階段 6 定義就是這樣失敗的（見 docs/evaluation-universe-selection-plan.md 階段 6）。

**改比序數性質**：對「多了幾天資料」不敏感，但對「pipeline 算錯」很敏感——
ATR 公式改壞、還原係數套錯方向、bucket 判定寫反，都會打亂排序。

純函數，不碰 DB、不碰檔案系統（CLI 才讀檔）。
"""
from __future__ import annotations

import argparse
import json
import sys
from typing import Any, Iterable, Optional

# Spearman 相關的下限。1.0 太嚴（相鄰兩檔本來就可能因幾天資料互換），
# 0.9 在 9 檔的規模下容許約一組相鄰互換，但擋得住整體重排。
DEFAULT_MIN_SPEARMAN = 0.9


def _rank(values: dict[str, float]) -> dict[str, float]:
    """由小到大的排名，平手取平均名次（Spearman 要求）。"""
    ordered = sorted(values.items(), key=lambda kv: kv[1])
    ranks: dict[str, float] = {}
    i = 0
    while i < len(ordered):
        j = i
        while j + 1 < len(ordered) and ordered[j + 1][1] == ordered[i][1]:
            j += 1
        avg = (i + j) / 2 + 1
        for k in range(i, j + 1):
            ranks[ordered[k][0]] = avg
        i = j + 1
    return ranks


def spearman(a: dict[str, float], b: dict[str, float]) -> Optional[float]:
    """兩組數值在**共同 symbol** 上的 Spearman 相關係數。

    少於 2 檔共同 symbol 時回 None——相關係數在那個規模上沒有意義，
    不要回 0 或 1 讓呼叫端誤判。
    """
    common = sorted(set(a) & set(b))
    if len(common) < 2:
        return None
    ra, rb = _rank({s: a[s] for s in common}), _rank({s: b[s] for s in common})
    n = len(common)
    mean = (n + 1) / 2
    num = sum((ra[s] - mean) * (rb[s] - mean) for s in common)
    da = sum((ra[s] - mean) ** 2 for s in common) ** 0.5
    db = sum((rb[s] - mean) ** 2 for s in common) ** 0.5
    if da == 0 or db == 0:  # 全部平手，排序無資訊
        return None
    return num / (da * db)


def profiles_by_symbol(profiles: Any) -> dict[str, dict[str, Any]]:
    """`volatility_profiles` 正規化成 symbol → profile。

    **實際的 report 是 dict（symbol 當 key）**，不是 list——這裡兩種都接。
    第一版只處理 list，是照著我想像的形狀寫的，實跑才發現；
    留下兩種都吃是因為 dict 的 key 與 `profile["symbol"]` 理論上可能不一致（以 key 為準）。
    """
    if isinstance(profiles, dict):
        return {str(s): p for s, p in profiles.items() if isinstance(p, dict)}
    return {p["symbol"]: p for p in (profiles or []) if isinstance(p, dict) and p.get("symbol")}


def build_baseline(
    report: dict[str, Any], symbols: Iterable[str], snapshot: dict[str, Any]
) -> dict[str, Any]:
    """從 evaluation report 取出指定標的的 profile，組成基準檔。

    `snapshot` 必須記下產生當時的資料狀態（最後交易日、每檔列數…）。
    **少了它，下次比對無法判斷差異來自 pipeline 還是來自資料**——那正是這次踩到的坑。
    """
    by_sym = profiles_by_symbol(report.get("volatility_profiles") or [])
    wanted = sorted(set(symbols))
    return {
        "schema_version": "sr_volatility_baseline_p0",
        "source_schema_version": report.get("schema_version"),
        "pipeline_version": report.get("pipeline_version"),
        "timeframe": report.get("timeframe"),
        "snapshot": snapshot,
        "symbols": wanted,
        "missing": [s for s in wanted if s not in by_sym],
        "profiles": {
            s: {
                "atr_pct": by_sym[s].get("atr_pct"),
                "average_range_pct": by_sym[s].get("average_range_pct"),
                "bucket": by_sym[s].get("bucket"),
                "candle_count": by_sym[s].get("candle_count"),
                "lookback_bars": by_sym[s].get("lookback_bars"),
            }
            for s in wanted
            if s in by_sym
        },
    }


def compare(
    baseline: dict[str, Any],
    report: dict[str, Any],
    *,
    min_spearman: float = DEFAULT_MIN_SPEARMAN,
) -> dict[str, Any]:
    """比對序數性質。回傳 `checks`（每項含 passed）與 `passed` 總結。"""
    base = baseline.get("profiles") or {}
    cur = {
        s: p for s, p in profiles_by_symbol(report.get("volatility_profiles") or []).items()
        if s in base
    }

    def _atr(d: dict[str, Any]) -> dict[str, float]:
        return {s: v["atr_pct"] for s, v in d.items() if isinstance(v.get("atr_pct"), (int, float))}

    b_atr, c_atr = _atr(base), _atr(cur)
    checks: list[dict[str, Any]] = []

    missing = sorted(set(base) - set(cur))
    checks.append({
        "name": "所有基準標的都有 profile",
        "blocking": True,
        "passed": not missing,
        "detail": {"missing": missing},
    })

    def _extreme(d: dict[str, float], *, highest: bool, k: int = 1) -> list[str]:
        return sorted(sorted(d, key=lambda s: (-d[s] if highest else d[s]))[:k])

    for label, highest, k in (("波動最高者不變", True, 1), ("波動最低兩檔不變", False, 2)):
        if len(b_atr) >= k and len(c_atr) >= k:
            bx, cx = _extreme(b_atr, highest=highest, k=k), _extreme(c_atr, highest=highest, k=k)
            checks.append({"name": label, "blocking": True, "passed": bx == cx,
                           "detail": {"baseline": bx, "current": cx}})

    rho = spearman(b_atr, c_atr)
    checks.append({
        "name": f"atr_pct 排名 Spearman >= {min_spearman}",
        "blocking": True,
        # rho 為 None 代表樣本不足或全平手——**不能當成通過**
        "passed": rho is not None and rho >= min_spearman,
        "detail": {"spearman": rho, "min": min_spearman, "n": len(set(b_atr) & set(c_atr))},
    })

    # bucket 跨越是**觀察項而非失敗條件**。
    # 絕對門檻（1.5% / 3.5%）與台股分佈差一個量級（見 todo.md T-003「門檻重定」），
    # 標的常態貼在 3.5% 附近，普通的資料漂移就會跨過去。實證：2026-08-06 到 08-17 的 11 天內
    # HIGH 由 9 檔變 6 檔，期間 pipeline 沒有任何改動。
    # 把它設成 blocking 會讓階段 6 常態失敗，然後被當成雜訊忽略——那比沒有檢查更糟。
    moved = {s: [base[s].get("bucket"), cur[s].get("bucket")]
             for s in sorted(base) if s in cur and base[s].get("bucket") != cur[s].get("bucket")}
    checks.append({
        "name": "bucket 跨越（觀察項，不阻擋）",
        "blocking": False,
        "passed": not moved,
        "detail": {"moved": moved},
    })

    blocking = [c for c in checks if c["blocking"]]
    return {
        "baseline_snapshot": baseline.get("snapshot"),
        "compared_symbols": sorted(set(base) & set(cur)),
        "checks": checks,
        # 只有 blocking 項決定成敗
        "passed": all(c["passed"] for c in blocking),
        "warnings": [c["name"] for c in checks if not c["blocking"] and not c["passed"]],
    }


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description="SR volatility 回歸基準（階段 6）")
    sub = ap.add_subparsers(dest="cmd", required=True)

    b = sub.add_parser("build", help="從 evaluation report 產生基準檔")
    b.add_argument("--report", required=True)
    b.add_argument("--symbols", required=True, help="逗號分隔，通常是 regression_baseline_symbols")
    b.add_argument("--snapshot", default="{}", help="資料快照 JSON")
    b.add_argument("--output", required=True)

    c = sub.add_parser("compare", help="比對 evaluation report 與基準檔")
    c.add_argument("--report", required=True)
    c.add_argument("--baseline", required=True)
    c.add_argument("--min-spearman", type=float, default=DEFAULT_MIN_SPEARMAN)

    args = ap.parse_args(argv)
    with open(args.report, encoding="utf-8") as fh:
        report = json.load(fh)

    if args.cmd == "build":
        out = build_baseline(
            report,
            [x.strip() for x in args.symbols.split(",") if x.strip()],
            json.loads(args.snapshot),
        )
        with open(args.output, "w", encoding="utf-8") as fh:
            json.dump(out, fh, ensure_ascii=False, indent=2, sort_keys=True)
            fh.write("\n")
        print(f"==> 基準已寫入 {args.output}（{len(out['profiles'])} 檔）", file=sys.stderr)
        if out["missing"]:
            print(f"WARNING: report 裡找不到 {out['missing']}", file=sys.stderr)
        return 0

    with open(args.baseline, encoding="utf-8") as fh:
        baseline = json.load(fh)
    result = compare(baseline, report, min_spearman=args.min_spearman)
    print(json.dumps(result, ensure_ascii=False, indent=2))
    for chk in result["checks"]:
        tag = ("PASS" if chk["passed"] else "FAIL") if chk["blocking"] else \
              ("ok  " if chk["passed"] else "WARN")
        print(f"  {tag}  {chk['name']}", file=sys.stderr)
    return 0 if result["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
