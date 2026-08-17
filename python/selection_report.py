"""Evaluation Universe Selection Report（T-040 Step 3）。

依 `docs/evaluation-universe-selection-plan.md` 產出選池分析報告：每檔標的的波動、
流動性、bucket 與排除原因，外加多組流動性門檻的比較矩陣。

**這是研究輸出，不是 production config。** 報告本身不決定任何事，它提供決策依據；
門檻與最終清單由人看過報告後決定。

## 兩個定義上的坑（計畫書「已知缺陷」記錄的）

1. **`traded_days_60` 的分母是「市場交易日」，不是「自己的 K 線根數」。**
   candles 只有有成交的日子才有列，所以「近 60 根裡 amount > 0 的天數」恆等於 60，
   量不出任何東西。實測 `6236 中湛` 在 60 個市場交易日裡只成交 7 天，
   但它有 38 根 K 線——用錯的定義完全看不出來。

2. **資料不新鮮的標的不算波動。** 停止交易者的「近 60 根」可能橫跨數月
   （`4804 大略-KY` 停在 2026-04-13），照算會得到四個月前的波動率。
   這種標的的 `atr_pct` 一律輸出 None、bucket 標 `STALE`，而不是先算完再排除——
   否則只看 bucket 分佈統計的人會被污染。
"""
from __future__ import annotations

import argparse
import json
import sys
from typing import Any, Iterable, Optional

# 直接重用 pipeline 的計算與分類，**不自己重寫一份**。
# 這兩個是私有函式，但重寫的代價是「報告說 LOW、pipeline 說 NORMAL」這種
# 沒有任何東西會報錯的分歧，比耦合一個私有函式嚴重得多。
from backtest.modular.sr_scoring.evaluation import _atr_pct, VOLATILITY_PROFILE_LOOKBACK
from backtest.modular.sr_scoring.zone_builder import (
    HIGH_VOLATILITY_THRESHOLD,
    LOW_VOLATILITY_THRESHOLD,
    volatility_bucket_from_profile,
)

BUCKET_STALE = "STALE"

# universe_role：`primary` 參與股票 builder 決策，`supplemental` 只作交叉觀察。
#
# 分類依據是 **TWSE 的 ETF 代號後綴慣例**，比名稱比對可靠：
#   B=債券（104 檔中 103 檔名稱含「債」）、L=槓桿正2、R=反向反1、
#   U=期貨商品、K=特殊計價級別、A=主動式、C/D/T=其他級別或平衡型。
# 純數字代號的 ETF（158 檔，名稱含「債」者 0）才是股票型。
#
# **規則刻意保守**：任何帶字母後綴的一律 supplemental。誤標成 supplemental 的代價只是
# 少一檔交叉觀察；誤標成 primary 的代價是讓債券或槓桿商品去影響股票的 zone builder 參數。
ROLE_PRIMARY = "primary"
ROLE_SUPPLEMENTAL = "supplemental"

# 流動性層級（日均成交金額，新台幣元）。與計畫書的門檻矩陣一致。
LIQUIDITY_TIERS: list[tuple[str, float]] = [
    ("T1_1E", 100_000_000.0),
    ("T2_5000W", 50_000_000.0),
    ("T3_2000W", 20_000_000.0),
    ("T4_1000W", 10_000_000.0),
]
THRESHOLD_MATRIX: list[tuple[str, float]] = [
    ("1000W", 10_000_000.0),
    ("2000W", 20_000_000.0),
    ("5000W", 50_000_000.0),
    ("1E", 100_000_000.0),
]

# 允許落後幾個市場交易日仍視為新鮮。1 是為了容忍當日尚未收盤入庫。
DEFAULT_STALE_TOLERANCE_DAYS = 3
# 近 60 個市場交易日內至少要有幾天成交。
DEFAULT_MIN_TRADED_DAYS = 45
# 少於這麼多根就算不出 60 根的波動側寫。
MIN_CANDLES_FOR_PROFILE = VOLATILITY_PROFILE_LOOKBACK


def _candle_date(candle: dict[str, Any]) -> str:
    """K 棒的台北日期字串。

    `fetch_candles` 回傳的 `timestamp` 是 epoch 秒（UTC）。台北是 UTC+8，
    直接用 UTC 日期會在收盤後的凌晨時段差一天。
    """
    from datetime import datetime, timedelta, timezone

    ts = int(candle["timestamp"])
    return (datetime.fromtimestamp(ts, tz=timezone.utc) + timedelta(hours=8)).strftime("%Y-%m-%d")


def compute_symbol_metrics(
    meta: dict[str, Any],
    candles: list[dict[str, Any]],
    market_days: Iterable[str],
    *,
    stale_tolerance_days: int = DEFAULT_STALE_TOLERANCE_DAYS,
) -> dict[str, Any]:
    """算出單一標的的報告欄位。純函數，不碰 DB。

    `candles` 需依時間遞增（`fetch_candles` 已如此）；`market_days` 是最近 N 個
    市場交易日（順序不拘）。
    """
    days = sorted(set(market_days), reverse=True)
    window = set(days)
    recent_days = set(days[:stale_tolerance_days])

    dates = [_candle_date(c) for c in candles]
    last_candle_at = dates[-1] if dates else None

    # **分母是市場交易日**，不是自己的根數（見模組 docstring）。
    traded_days = sum(1 for d in dates if d in window)
    is_stale = last_candle_at is None or last_candle_at not in recent_days

    recent = candles[-MIN_CANDLES_FOR_PROFILE:]
    enough_history = len(recent) >= MIN_CANDLES_FOR_PROFILE

    atr_pct: Optional[float] = None
    avg_range_pct: Optional[float] = None
    bucket = BUCKET_STALE
    if enough_history and not is_stale:
        import pandas as pd

        df = pd.DataFrame(recent)[["high", "low", "close"]].astype(float)
        atr_pct = _atr_pct(df)
        rng = ((df["high"] - df["low"]) / df["close"]).replace(
            [float("inf"), float("-inf")], float("nan")
        ).dropna()
        avg_range_pct = float(rng.mean()) if not rng.empty else None
        bucket = volatility_bucket_from_profile(atr_pct, avg_range_pct)

    amounts = [float(c.get("amount") or 0.0) for c in recent]
    avg_amount = sum(amounts) / len(amounts) if amounts else 0.0
    median_amount = 0.0
    if amounts:
        ordered = sorted(amounts)
        mid = len(ordered) // 2
        median_amount = (
            ordered[mid] if len(ordered) % 2 else (ordered[mid - 1] + ordered[mid]) / 2
        )

    return {
        "symbol": meta["symbol"],
        "name": meta.get("name", ""),
        "security_type": meta.get("security_type", ""),
        "industry": meta.get("industry", ""),
        "listed_date": meta.get("listed_date"),
        "candle_count": len(candles),
        "last_candle_at": last_candle_at,
        "is_stale": is_stale,
        "atr_pct": atr_pct,
        "average_range_pct": avg_range_pct,
        "current_bucket": bucket,
        "avg_amount_60": avg_amount,
        "median_amount_60": median_amount,
        "traded_days_60": traded_days,
        "liquidity_tier": classify_liquidity(avg_amount),
        "universe_role": classify_universe_role(meta["symbol"], meta.get("security_type", "")),
    }


def classify_universe_role(symbol: str, security_type: str) -> str:
    """判定標的在最終 universe 裡的角色（見上方 ROLE_* 的說明）。"""
    if security_type != "ETF":
        return ROLE_PRIMARY
    # 代號結尾是字母＝債券／槓桿／反向／期貨／特殊級別／主動式，一律 supplemental。
    return ROLE_SUPPLEMENTAL if symbol[-1:].isalpha() else ROLE_PRIMARY


def classify_liquidity(avg_amount: float) -> str:
    for name, floor in LIQUIDITY_TIERS:
        if avg_amount >= floor:
            return name
    return "T5_BELOW_1000W"


def evaluate_exclusion(
    m: dict[str, Any],
    *,
    min_amount: float,
    min_traded_days: int = DEFAULT_MIN_TRADED_DAYS,
) -> tuple[str, Optional[str]]:
    """回傳 (selection_status, exclusion_reason)。

    **排除原因有優先序且只給一個**——多重原因會讓統計無法加總。
    順序是「資料不可用」優先於「流動性不足」：前者代表這筆數字本身不可信。
    """
    if m["is_stale"]:
        return "excluded", "stale_candle"
    if m["candle_count"] < MIN_CANDLES_FOR_PROFILE:
        return "excluded", "short_history"
    if m["traded_days_60"] < min_traded_days:
        return "excluded", "thin_trading"
    if m["avg_amount_60"] < min_amount or m["median_amount_60"] <= 0:
        return "excluded", "low_liquidity"
    return "selected", None


def bucket_basis(m: dict[str, Any]) -> Optional[float]:
    """分桶用的波動基準，**必須與 pipeline 的 volatility_bucket_from_profile 相同**。

    那個函式取 `max(atr_pct, average_range_pct)`。報告一開始只取 `atr_pct`，
    造成選池的 `selection_bucket` 與 runtime 的 bucket 在 131 檔裡差了 20 檔——
    319 檔流動性合格股票中有 156 檔的 `average_range_pct` 比 `atr_pct` 大。
    **門檻、切點與判定基準三者必須同源**，否則重定門檻也修不好對不上的問題。
    """
    values = [v for v in (m.get("atr_pct"), m.get("average_range_pct")) if v is not None]
    return max(values) if values else None


def quantile_bucket_edges(values: list[float]) -> Optional[tuple[float, float]]:
    """由**流動性合格的股票**算 P33 / P67 切點。

    母體純度是這裡最容易錯的地方：混進 ETF（債券 ETF 全在低波動端）或低流動性股票
    （不動不是因為穩定，是因為沒人交易），切出來的門檻就沒有意義。
    呼叫端必須先過濾，本函式不負責。
    """
    if len(values) < 3:
        return None
    ordered = sorted(values)

    def _q(p: float) -> float:
        pos = p * (len(ordered) - 1)
        lo = int(pos)
        hi = min(lo + 1, len(ordered) - 1)
        return ordered[lo] + (ordered[hi] - ordered[lo]) * (pos - lo)

    return _q(1 / 3), _q(2 / 3)


def assign_quantile_bucket(basis: Optional[float], edges: Optional[tuple[float, float]]) -> str:
    """`basis` 是 `bucket_basis()` 的輸出，不是單獨的 atr_pct。"""
    if basis is None or edges is None:
        return BUCKET_STALE
    low_edge, high_edge = edges
    if basis < low_edge:
        return "LOW_VOLATILITY"
    if basis > high_edge:
        return "HIGH_VOLATILITY"
    return "NORMAL_VOLATILITY"


def build_threshold_matrix(
    metrics: list[dict[str, Any]],
    *,
    thresholds: list[tuple[str, float]] = THRESHOLD_MATRIX,
    min_traded_days: int = DEFAULT_MIN_TRADED_DAYS,
) -> list[dict[str, Any]]:
    """每組門檻的合格數與 bucket 分佈。

    合格集合必須**隨門檻遞增而單調縮小且互為子集**——這是性質檢查的依據
    （見計畫書「階段 3：性質檢查」）。
    """
    out = []
    for name, floor in sorted(thresholds, key=lambda t: t[1]):
        qualified = [
            m for m in metrics
            if evaluate_exclusion(m, min_amount=floor, min_traded_days=min_traded_days)[0] == "selected"
        ]
        stocks = [m for m in qualified if m["security_type"] == "股票"]
        etfs = [m for m in qualified if m["security_type"] == "ETF"]
        bases = [b for b in (bucket_basis(m) for m in stocks) if b is not None]
        edges = quantile_bucket_edges(bases)

        def _count(rows: list[dict[str, Any]], key: str) -> dict[str, int]:
            acc: dict[str, int] = {}
            for r in rows:
                acc[r[key]] = acc.get(r[key], 0) + 1
            return acc

        out.append({
            "threshold": name,
            "min_amount": floor,
            "qualified_stocks": len(stocks),
            "qualified_etfs": len(etfs),
            "stock_symbols": sorted(m["symbol"] for m in stocks),
            "current_bucket_stocks": _count(stocks, "current_bucket"),
            "current_bucket_etfs": _count(etfs, "current_bucket"),
            "quantile_edges": list(edges) if edges else None,
            "quantile_bucket_stocks": _count(
                [{"b": assign_quantile_bucket(bucket_basis(m), edges)} for m in stocks], "b"
            ),
            "industry_by_bucket": _industry_by_bucket(stocks),
        })
    return out


def _industry_by_bucket(rows: list[dict[str, Any]]) -> dict[str, dict[str, int]]:
    acc: dict[str, dict[str, int]] = {}
    for r in rows:
        acc.setdefault(r["current_bucket"], {})
        ind = r["industry"] or "(未分類)"
        acc[r["current_bucket"]][ind] = acc[r["current_bucket"]].get(ind, 0) + 1
    return acc


def build_report(
    metrics: list[dict[str, Any]],
    snapshot: dict[str, Any],
    *,
    min_amount: float,
    min_traded_days: int = DEFAULT_MIN_TRADED_DAYS,
) -> dict[str, Any]:
    rows = []
    for m in metrics:
        status, reason = evaluate_exclusion(
            m, min_amount=min_amount, min_traded_days=min_traded_days
        )
        rows.append({**m, "selection_status": status, "exclusion_reason": reason})

    selected_stocks = [
        r for r in rows if r["selection_status"] == "selected" and r["security_type"] == "股票"
    ]
    edges = quantile_bucket_edges(
        [b for b in (bucket_basis(r) for r in selected_stocks) if b is not None]
    )
    for r in rows:
        r["bucket_basis"] = bucket_basis(r)
        r["selection_bucket"] = assign_quantile_bucket(r["bucket_basis"], edges)

    reasons: dict[str, int] = {}
    for r in rows:
        if r["exclusion_reason"]:
            reasons[r["exclusion_reason"]] = reasons.get(r["exclusion_reason"], 0) + 1

    # **這是給 T-003 的輸入，不是報告的裝飾。**
    # pipeline 用的是絕對門檻 1.5% / 3.5%，而流動性合格股票的實際分位數切點差了一個量級——
    # 用分位數選出的「平均分佈」在 pipeline 眼中仍會擠在 HIGH。
    # 兩組數字並列，讓「門檻是否該重定」有依據而不是印象。
    # **import 真正的常數，不要複製數值。** 這裡原本把 0.015 / 0.035 硬編在報告裡，
    # 2026-08-17 重定門檻後就變成「報告宣稱 pipeline 用舊值」的假訊息——
    # 而「常數的手抄鏡像會過期」正是重定門檻要消滅的那類問題。
    threshold_gap = {
        "pipeline_absolute": {
            "low_max": LOW_VOLATILITY_THRESHOLD,
            "high_min": HIGH_VOLATILITY_THRESHOLD,
        },
        "liquid_stock_quantile": {"p33": edges[0], "p67": edges[1]} if edges else None,
        # 重定後兩者應該相等（新常數就是凍結的分位數）。**不相等代表母體已經漂離凍結點**，
        # 那是「要不要升 universe_version」的訊號，不是錯誤。
        "aligned": (
            edges is not None
            and edges[0] == LOW_VOLATILITY_THRESHOLD
            and edges[1] == HIGH_VOLATILITY_THRESHOLD
        ),
        "note": (
            "pipeline 門檻已於 2026-08-17 重定為凍結的分位數（見 docs/sr-zone-scoring.md）。"
            "aligned=false 代表當下母體算出的切點已偏離凍結值，需評估是否升 universe_version；"
            "不是錯誤。"
        ),
    }

    return {
        "schema_version": "selection-report-p2",
        "threshold_gap": threshold_gap,
        # 快照：沒有它，日後重跑拿到不同數字時無法判斷是資料變了還是程式錯了。
        "snapshot": snapshot,
        "params": {"min_amount": min_amount, "min_traded_days": min_traded_days},
        "quantile_edges": list(edges) if edges else None,
        "summary": {
            "total": len(rows),
            "selected": sum(1 for r in rows if r["selection_status"] == "selected"),
            "selected_primary": sum(
                1 for r in rows
                if r["selection_status"] == "selected" and r["universe_role"] == ROLE_PRIMARY
            ),
            "selected_supplemental": sum(
                1 for r in rows
                if r["selection_status"] == "selected" and r["universe_role"] == ROLE_SUPPLEMENTAL
            ),
            "excluded": sum(1 for r in rows if r["selection_status"] == "excluded"),
            "exclusion_reasons": reasons,
        },
        "threshold_matrix": build_threshold_matrix(metrics, min_traded_days=min_traded_days),
        "symbols": sorted(rows, key=lambda r: r["symbol"]),
    }


# ── 最終 universe 選取 ────────────────────────────────────────
DEFAULT_TARGET_TOTAL = 135
DEFAULT_PER_BUCKET_MIN = 35
DEFAULT_PER_BUCKET_MAX = 45
# 單一產業在單一 bucket 內的佔比上限。半導體業有 201 檔，不設限會直接主導。
DEFAULT_INDUSTRY_CAP_RATIO = 0.25
DEFAULT_MIN_LISTED_YEARS = 5
# ETF 的 industry 是空字串，若與股票共用產業配額，它們會被歸成「(未分類)」這一個
# 假產業並吃掉整個名額（實測 LOW bucket 有 11 檔 ETF 佔位）。
# ETF 不是一個產業，是**不同的商品類別**，所以給獨立配額。
DEFAULT_ETF_PER_BUCKET = 2
# walk-forward evaluation 的 `--limit 1500`。watchlist 是**分級保留**的（見 select_universe
# 的 docstring），所以年限篩選擋不住它——實測 00947（299 根）、00981A（529 根）就是這樣進池的。
# 它們留著沒有壞處，但**不能算進回歸基準的檔數**，否則「11 檔 watchlist 基準」名不副實。
WALK_FORWARD_MIN_CANDLES = 1500
# watchlist 保留是**分級**的，不是一律無條件（規格見
# docs/evaluation-universe-selection-plan.md「watchlist 的分級保留」）。
# 這個集合列的是「連保留都沒意義」的排除原因：`stale_candle` 的標的 `atr_pct` 是 None、
# bucket 是 BUCKET_STALE，既拿不到 ATR config，也產不出 `volatility_profiles`——
# 而「當回歸基準」正是保留 watchlist 的唯一理由，所以留著只會給下游一個無桶標的。
# 其餘原因（short_history / thin_trading / low_liquidity）算得出 bucket 與 profile，
# 留在 universe 有意義，只是不列入回歸基準。
KEEP_FATAL_EXCLUSIONS = frozenset({"stale_candle"})
# 台股一年約 243 個交易日，用來估「這檔理論上最多能有幾根」，
# 以區分深度不足的兩種成因（見 _depth_shortfall_kind）。
TRADING_DAYS_PER_YEAR = 243


def _listed_years(listed_date: Optional[str], today: str) -> Optional[float]:
    if not listed_date:
        return None
    try:
        from datetime import date
        d = date.fromisoformat(str(listed_date)[:10])
        t = date.fromisoformat(today)
    except ValueError:
        return None
    return (t - d).days / 365.25


def _depth_shortfall_kind(
    total: int, listed_date: Optional[str], today: str, threshold: int
) -> str:
    """深度不足的兩種成因——**處置完全不同，所以必須分開**。

    * `backfillable`：上市夠久但庫裡沒資料，深補就會好。實例是 1569 濱川（2005 年上市、
      庫內只有 158 列，起點 2025-12-16）——`min_listed_years` 用的是 `listed_date`，
      結構上抓不到「上市很久但我們沒抓」這種情況。
    * `listing_age`：庫裡已經是全部歷史，再怎麼抓也不會變多（00981A 上市才一年）。

    判準先看「上市至今理論上最多幾根」有沒有超過 threshold：

    * **超過** → 上市夠久，缺的部分本來就抓得到，一律 `backfillable`。
    * **沒超過** → 天花板是上市年數而不是我們的抓取，比對實際列數是否接近天花板；
      留 10% 餘裕吸收停牌與「一年 243 個交易日」這個估算的誤差。

    餘裕**只能用在後者**。若無條件對 `min(ceiling, threshold)` 留 10%，長期上市但資料淺的
    標的會被誤判：上市 8 年、庫內 1,350 列（實際該有約 1,944 列）會因為
    `1350 >= 0.9 × 1500` 而被標成 `listing_age`，於是「`backfillable` 必須在階段 5 前補完」
    這條檢查放它過關，evaluation 照樣拿殘缺資料跑。
    """
    years = _listed_years(listed_date, today)
    if years is None:
        return "unknown"
    ceiling = int(years * TRADING_DAYS_PER_YEAR)
    if ceiling >= threshold:
        return "backfillable"
    return "listing_age" if total >= 0.9 * ceiling else "backfillable"


def select_universe(
    rows: list[dict[str, Any]],
    today: str,
    *,
    keep_symbols: Iterable[str] = (),
    per_bucket_min: int = DEFAULT_PER_BUCKET_MIN,
    per_bucket_max: int = DEFAULT_PER_BUCKET_MAX,
    industry_cap_ratio: float = DEFAULT_INDUSTRY_CAP_RATIO,
    min_listed_years: int = DEFAULT_MIN_LISTED_YEARS,
    etf_per_bucket: int = DEFAULT_ETF_PER_BUCKET,
    walk_forward_min_candles: int = WALK_FORWARD_MIN_CANDLES,
) -> dict[str, Any]:
    """從流動性合格池挑出最終 universe。**決定性**：同輸入必得同輸出。

    實測發現**流動性門檻不是瓶頸**（2000 萬就有 319 檔合格，遠超 120～150 的目標），
    真正的約束是**產業分散與 bucket 配額**——所以這裡的重點不是「怎麼湊滿」，
    而是「怎麼從 300 檔裡挑 135 檔而不讓半導體業主導」。

    排序依據是**日均成交金額由高到低**。這與 Step 1 候選抽樣刻意避開的「取代號最小的前 N」
    不同：那裡的偏斜會扭曲**分佈量測**，這裡流動性高低本身就是**資料品質**的代理——
    zone 的觸價統計在成交稀疏的標的上不可信。但它仍是一種偏斜（偏大型股），
    所以用產業上限把它壓住。

    `keep_symbols`（watchlist）**分級保留**且不佔產業配額，理由是它們是回歸檢查的基準，
    見計畫書「最終 Universe 選取規則」第 1 點。分級的意思是：

    * 合格、或雖被排除但仍算得出 bucket 與 profile（`short_history` / `thin_trading` /
      `low_liquidity`）→ **留在 universe**；不合格者另記 `baseline_excluded`。
    * `KEEP_FATAL_EXCLUSIONS`（`stale_candle`）→ **不留**，記入 `keep_symbols_dropped`。
    * 主檔／K 線裡找不到 → 記入 `keep_symbols_missing`，不再靜默忽略。

    深度撐不起 walk-forward 者（`insufficient_depth`）也留在 universe，但同樣不算基準。
    """
    keep = set(keep_symbols)
    pool = [r for r in rows if r["selection_status"] == "selected" and r["universe_role"] == ROLE_PRIMARY]
    by_symbol = {r["symbol"]: r for r in rows}

    picked: dict[str, str] = {}
    keep_missing: list[str] = []
    keep_dropped: dict[str, str] = {}
    # 留在 universe 但不能當回歸基準者，理由記在這裡（不要靜默）。
    keep_degraded: dict[str, str] = {}
    for sym in sorted(keep):
        r = by_symbol.get(sym)
        if r is None:
            # 沒有 K 線被主流程跳過、`security_type` 不在查詢範圍、或已從 stock_symbols 移除。
            # 以前這裡是靜默 continue，watchlist 少一檔完全看不出來。
            keep_missing.append(sym)
            continue
        if r["selection_status"] != "selected":
            reason = r.get("exclusion_reason") or "unknown"
            if reason in KEEP_FATAL_EXCLUSIONS:
                keep_dropped[sym] = reason
                continue
            keep_degraded[sym] = reason
        # 標籤區分「是基準」與「只是留著」——後者叫 baseline 會與 baseline_excluded 打對台。
        picked[sym] = "watchlist_kept" if sym in keep_degraded else "watchlist_baseline"

    industry_cap = max(1, int(per_bucket_max * industry_cap_ratio))
    buckets: dict[str, list[dict[str, Any]]] = {}
    for r in pool:
        if r["symbol"] in picked:
            continue
        years = _listed_years(r.get("listed_date"), today)
        if min_listed_years and (years is None or years < min_listed_years):
            continue  # 深度不足 5 年，無法支撐 walk-forward
        buckets.setdefault(r["selection_bucket"], []).append(r)

    per_bucket_stats = {}
    for bucket, candidates in sorted(buckets.items()):
        # 決定性排序：成交金額由高到低，同額時以代號排序打破平手
        candidates.sort(key=lambda r: (-r["avg_amount_60"], r["symbol"]))
        industry_used: dict[str, int] = {}
        taken = 0
        etf_taken = 0
        for r in candidates:
            if taken >= per_bucket_max:
                break
            is_etf = r.get("security_type") == "ETF"
            if is_etf:
                # ETF 走獨立配額，不佔產業名額（見 DEFAULT_ETF_PER_BUCKET 的說明）。
                if etf_taken >= etf_per_bucket:
                    continue
                picked[r["symbol"]] = f"bucket:{bucket}:etf"
                etf_taken += 1
                taken += 1
                continue
            ind = r["industry"] or "(未分類)"
            if industry_used.get(ind, 0) >= industry_cap:
                continue
            picked[r["symbol"]] = f"bucket:{bucket}"
            industry_used[ind] = industry_used.get(ind, 0) + 1
            taken += 1
        per_bucket_stats[bucket] = {
            "candidates": len(candidates),
            "picked": taken,
            "etf_picked": etf_taken,
            "industry_cap": industry_cap,
            "industries": dict(sorted(industry_used.items(), key=lambda kv: -kv[1])),
            # 配額沒填滿代表「合格且產業分散後就是不夠」，**不可用低流動性股票硬補**
            "underfilled": taken < per_bucket_min,
        }

    # 深度不足的標的仍留在 universe（照樣跑 evaluation），但回歸基準要把它們扣掉。
    # 用 `total_candle_count`（全歷史）而**不是** `candle_count`——後者是波動 profile 的
    # 抓取窗口，被 VOLATILITY_PROFILE_LOOKBACK 夾在 60，拿來判深度會把每一檔都標成不足。
    # 欄位缺席時不標記：代表呼叫端給的是精簡 row，「不知道深度」不等於「深度不足」。
    def _depth(sym: str) -> Optional[int]:
        v = by_symbol.get(sym, {}).get("total_candle_count")
        return v if isinstance(v, int) else None

    insufficient_depth = sorted(
        s for s in picked
        if (d := _depth(s)) is not None and d < walk_forward_min_candles
    )
    # 回歸基準的排除原因有兩個來源：深度撐不起 walk-forward，或標的本身被 selection filter
    # 判為不合格（但仍保留在 universe）。合併成一個欄位，`insufficient_depth` 維持「只講深度」。
    watchlist_picked = [s for s in picked if picked[s].startswith("watchlist_")]
    baseline_excluded = dict(keep_degraded)
    for s in watchlist_picked:
        if s in insufficient_depth:
            baseline_excluded.setdefault(s, "insufficient_depth")
    baseline = sorted(s for s in watchlist_picked if s not in baseline_excluded)

    # 留在 universe 但**不得影響股票 builder 決策**者（選取原則第 6 點的機制）。
    # 兩個來源：代號後綴判定的 supplemental ETF，以及分級保留下來的不合格 watchlist——
    # 後者若參與 builder 決策，等於用低流動性／稀疏成交的標的去調參，
    # 正是原則第 3 點「不足時不要用低流動性股票硬補」要避免的。
    supplemental: dict[str, str] = {}
    for s in sorted(picked):
        if s in keep_degraded:
            supplemental[s] = keep_degraded[s]
        elif by_symbol.get(s, {}).get("universe_role") == ROLE_SUPPLEMENTAL:
            supplemental[s] = "etf_suffix"

    return {
        "today": today,
        "params": {
            "per_bucket_min": per_bucket_min, "per_bucket_max": per_bucket_max,
            "industry_cap": industry_cap, "min_listed_years": min_listed_years,
            "etf_per_bucket": etf_per_bucket,
            "walk_forward_min_candles": walk_forward_min_candles,
            "keep_symbols": sorted(keep),
        },
        "selected_symbols": sorted(picked),
        "selected_count": len(picked),
        "selection_reason": dict(sorted(picked.items())),
        "per_bucket": per_bucket_stats,
        # 留在 universe、但 K 棒數撐不起 walk-forward 的標的
        "insufficient_depth": insufficient_depth,
        "insufficient_depth_detail": {
            s: {
                "total_candle_count": _depth(s),
                "kind": _depth_shortfall_kind(
                    _depth(s) or 0, by_symbol.get(s, {}).get("listed_date"),
                    today, walk_forward_min_candles,
                ),
            }
            for s in insufficient_depth
        },
        # keep_symbols 的三種非正常結局，全部顯式輸出——靜默是這裡最危險的失敗模式
        "keep_symbols_missing": sorted(keep_missing),      # 主檔／K 線裡根本找不到
        "keep_symbols_dropped": dict(sorted(keep_dropped.items())),  # 保留也沒意義（stale）
        "baseline_excluded": dict(sorted(baseline_excluded.items())),  # 留在池內但不當基準
        # 在 universe 內但不得影響股票 builder 決策（symbol → 原因）
        "supplemental_symbols": supplemental,
        "primary_count": len(picked) - len(supplemental),
        # 實際可用的回歸基準
        "regression_baseline_symbols": baseline,
        "regression_baseline_count": len(baseline),
    }


def _keep_symbols_drift(keep: Iterable[str], watchlist: Iterable[str]) -> dict[str, Any]:
    """`--keep-symbols` 與 DB `watchlists` 的差異。

    **為什麼要比**：`--keep-symbols` 的預設值是**刻意凍結**的（凍結才能重現定案那次的選池），
    但凍結的代價是它不會跟著 watchlist 變動。少了這個比對，日後在前端加一檔 watchlist
    後重跑報告，那檔不會進池且**不會有任何提示**——與 watchlist 分級保留要消除的靜默失敗同一類
    （見 docs/evaluation-universe-selection-plan.md「watchlist 的分級保留」）。

    純函數，不查 DB；呼叫端負責取 watchlist。
    """
    k, w = set(keep), set(watchlist)
    return {
        "watchlist_count": len(w),
        "keep_symbols_count": len(k),
        "only_in_watchlist": sorted(w - k),     # watchlist 新增但沒進 keep_symbols
        "only_in_keep_symbols": sorted(k - w),  # keep_symbols 有但已不在 watchlist
        "in_sync": k == w,
    }


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Evaluation universe selection report（唯讀）")
    parser.add_argument("--timeframe", default="1d")
    parser.add_argument("--min-amount", type=float, default=20_000_000.0,
                        help="日均成交金額下限（預設 2000 萬）")
    parser.add_argument("--min-traded-days", type=int, default=DEFAULT_MIN_TRADED_DAYS)
    parser.add_argument("--security-type", default="股票,ETF")
    parser.add_argument("--output", help="輸出 JSON 路徑；未指定則印到 stdout")
    parser.add_argument("--keep-symbols", default="",
                        help="無條件保留的代號（逗號分隔），通常是 watchlist")
    parser.add_argument("--today", default=None, help="YYYY-MM-DD，用於算上市年數；預設今天")
    args = parser.parse_args(argv)

    from db import (fetch_candle_depths, fetch_candles, fetch_market_trading_days,
                    fetch_symbol_universe, fetch_watchlist_symbols)

    market_days = fetch_market_trading_days(args.timeframe, VOLATILITY_PROFILE_LOOKBACK)
    if not market_days:
        print("ERROR: 查不到任何市場交易日——candles 是空的？", file=sys.stderr)
        return 1

    wanted = {t.strip() for t in args.security_type.split(",") if t.strip()}
    universe = [u for u in fetch_symbol_universe() if u["security_type"] in wanted]

    # 全歷史深度：一次聚合拿完，不能用下面 fetch_candles 的長度（那個被 limit 夾在 60）。
    depths = fetch_candle_depths(args.timeframe)

    metrics = []
    for i, meta in enumerate(universe, 1):
        candles = fetch_candles(meta["symbol"], args.timeframe, limit=VOLATILITY_PROFILE_LOOKBACK)
        if not candles:
            continue
        m = compute_symbol_metrics(meta, candles, market_days)
        m["total_candle_count"] = depths.get(meta["symbol"], 0)
        metrics.append(m)
        if i % 100 == 0:
            print(f"  …{i}/{len(universe)}", file=sys.stderr)

    report = build_report(
        metrics,
        snapshot={
            "timeframe": args.timeframe,
            "symbols_in_universe": len(universe),
            "symbols_with_candles": len(metrics),
            "market_days": len(market_days),
            "market_day_range": [market_days[-1], market_days[0]],
        },
        min_amount=args.min_amount,
        min_traded_days=args.min_traded_days,
    )

    from datetime import date
    keep = [x.strip() for x in args.keep_symbols.split(",") if x.strip()]
    report["universe"] = select_universe(
        report["symbols"],
        args.today or date.today().isoformat(),
        keep_symbols=keep,
    )
    report["universe"]["keep_symbols_drift"] = _keep_symbols_drift(keep, fetch_watchlist_symbols())

    drift = report["universe"]["keep_symbols_drift"]
    if not drift["in_sync"]:
        print(
            f"WARNING: --keep-symbols 與 DB watchlists 不一致"
            f"（watchlist 獨有 {drift['only_in_watchlist']}、"
            f"keep_symbols 獨有 {drift['only_in_keep_symbols']}）。"
            f"預設值是刻意凍結的，若要納入新 watchlist 需明確傳 KEEP_SYMBOLS。",
            file=sys.stderr,
        )

    payload = json.dumps(report, ensure_ascii=False, indent=2, default=str)
    if args.output:
        with open(args.output, "w", encoding="utf-8") as fh:
            fh.write(payload)
        print(f"==> 已寫入 {args.output}", file=sys.stderr)
    else:
        print(payload)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
