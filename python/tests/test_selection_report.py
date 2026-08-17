"""selection_report 的單元測試（T-040 Step 3）。

案例形狀取自 live 實測的真實標的（見 docs/evaluation-universe-selection-plan.md
「階段 1：已知答案測試」），而不是憑空造的合成資料——這批工作的經驗是
**單元測試只會驗證「我以為的行為」**，真 bug 都是實跑抓到的。
用真實形狀當 fixture 至少讓假設有依據。
"""
from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest

from backtest.modular.sr_scoring.zone_builder import (
    HIGH_VOLATILITY_THRESHOLD,
    LOW_VOLATILITY_THRESHOLD,
)
from selection_report import (
    BUCKET_STALE,
    MIN_CANDLES_FOR_PROFILE,
    assign_quantile_bucket,
    build_report,
    build_threshold_matrix,
    classify_liquidity,
    bucket_basis,
    compute_symbol_metrics,
    evaluate_exclusion,
    quantile_bucket_edges,
)

MARKET_DAYS = [f"2026-08-{d:02d}" for d in range(13, 0, -1)] + [
    f"2026-07-{d:02d}" for d in range(31, 0, -1)
] + [f"2026-06-{d:02d}" for d in range(30, 12, -1)]
LATEST = MARKET_DAYS[0]


def _ts(day: str) -> int:
    """把台北日期轉成 epoch 秒（取當日 09:00 台北，避免落在時區邊界上）。"""
    dt = datetime.strptime(day, "%Y-%m-%d").replace(tzinfo=timezone(timedelta(hours=8)), hour=9)
    return int(dt.timestamp())


def _candles(days: list[str], *, close: float = 100.0, rng: float = 1.0, amount: float = 5e7):
    out = []
    for d in reversed(days):  # fetch_candles 回傳時間遞增
        out.append({
            "timestamp": _ts(d),
            "high": close + rng / 2,
            "low": close - rng / 2,
            "close": close,
            "amount": amount,
        })
    return out


def _meta(symbol="2330", security_type="股票", industry="半導體業"):
    return {"symbol": symbol, "name": symbol, "security_type": security_type,
            "industry": industry, "listed_date": "2010-01-01"}


# ── 缺陷一：traded_days_60 的分母是市場交易日 ──────────────────
def test_traded_days_counts_market_days_not_own_rows():
    """`6236 中湛` 的形狀：K 線根數不少，但多數市場交易日根本沒成交。

    用錯的定義（自己近 60 根裡 amount>0 的天數）會得到「全部有成交」，
    因為 candles 只有有成交的日子才有列——那個過濾器永遠不會觸發。
    """
    traded = MARKET_DAYS[:7]  # 只有 7 天有成交
    m = compute_symbol_metrics(_meta("6236"), _candles(traded), MARKET_DAYS)

    assert m["candle_count"] == 7
    assert m["traded_days_60"] == 7, "分母用錯了——應該是市場交易日而不是自己的根數"
    status, reason = evaluate_exclusion(m, min_amount=1e7)
    assert (status, reason) == ("excluded", "short_history"), (
        "7 根連波動都算不出來，應先被 short_history 擋下"
    )


def test_thin_trading_detected_when_history_is_long_enough():
    """根數夠算波動，但市場交易日覆蓋率低 → thin_trading。"""
    days = MARKET_DAYS[:MIN_CANDLES_FOR_PROFILE]
    # 只有前 40 天落在最近 60 個市場交易日內，其餘來自更早（不在 window）
    older = [f"2026-05-{d:02d}" for d in range(31, 11, -1)]
    m = compute_symbol_metrics(_meta("X"), _candles(days[:40] + older), MARKET_DAYS)

    assert m["traded_days_60"] == 40
    assert evaluate_exclusion(m, min_amount=1e7)[1] == "thin_trading"


# ── 缺陷二：不新鮮的資料不算波動 ──────────────────────────────
def test_stale_symbol_reports_null_atr_not_a_computed_value():
    """`4804 大略-KY` 的形狀：資料停在四個月前。

    照算會得到「四個月前的波動率」，而只看 bucket 分佈統計的人不會知道。
    所以 atr_pct 必須是 None、bucket 為 STALE。
    """
    stale_days = MARKET_DAYS[20:20 + MIN_CANDLES_FOR_PROFILE]
    m = compute_symbol_metrics(_meta("4804"), _candles(stale_days), MARKET_DAYS)

    assert m["is_stale"] is True
    assert m["atr_pct"] is None, "不新鮮的資料不該產出波動數字"
    assert m["current_bucket"] == BUCKET_STALE
    assert evaluate_exclusion(m, min_amount=1e7) == ("excluded", "stale_candle")


def test_fresh_symbol_computes_bucket():
    days = MARKET_DAYS[:MIN_CANDLES_FOR_PROFILE]
    m = compute_symbol_metrics(_meta("2330"), _candles(days, close=100.0, rng=8.0), MARKET_DAYS)

    assert m["is_stale"] is False
    assert m["atr_pct"] is not None
    assert m["current_bucket"] == "HIGH_VOLATILITY", "日內振幅 8% 應落在 HIGH"


# ── 流動性 ────────────────────────────────────────────────────
def test_low_volatility_but_illiquid_is_excluded():
    """`3067 全域` 的形狀：ATR 低，但日均成交只有 10 萬。

    **低波動與低流動性高度重疊**——不動不是因為穩定，是因為沒人交易。
    這種標的進了 LOW bucket，調參學到的會是「沒有成交所以價格不動」。
    """
    days = MARKET_DAYS[:MIN_CANDLES_FOR_PROFILE]
    m = compute_symbol_metrics(
        _meta("3067"), _candles(days, close=100.0, rng=1.2, amount=1e5), MARKET_DAYS
    )
    assert m["current_bucket"] == "LOW_VOLATILITY"
    assert evaluate_exclusion(m, min_amount=2e7) == ("excluded", "low_liquidity")


def test_low_volatility_and_liquid_is_selected():
    """`2633 台灣高鐵` 的形狀：ATR 1.45%、日均 3.34 億 → 唯一該進 LOW 的類型。"""
    days = MARKET_DAYS[:MIN_CANDLES_FOR_PROFILE]
    m = compute_symbol_metrics(
        _meta("2633", industry="航運業"), _candles(days, close=100.0, rng=1.2, amount=3.34e8),
        MARKET_DAYS,
    )
    assert m["current_bucket"] == "LOW_VOLATILITY"
    assert m["liquidity_tier"] == "T1_1E"
    assert evaluate_exclusion(m, min_amount=5e7) == ("selected", None)


def test_liquidity_tiers_are_ordered():
    assert classify_liquidity(3.34e8) == "T1_1E"
    assert classify_liquidity(6e7) == "T2_5000W"
    assert classify_liquidity(3e7) == "T3_2000W"
    assert classify_liquidity(1.5e7) == "T4_1000W"
    assert classify_liquidity(1e5) == "T5_BELOW_1000W"


# ── 排除原因必須唯一且有優先序 ────────────────────────────────
def test_exclusion_reason_is_single_and_data_problems_win():
    """同時 stale 又低流動性時，回報 stale——資料不可用優先於條件不合格。

    多重原因會讓排除統計無法加總，所以只給一個。
    """
    stale_days = MARKET_DAYS[20:20 + MIN_CANDLES_FOR_PROFILE]
    m = compute_symbol_metrics(_meta("X"), _candles(stale_days, amount=1e4), MARKET_DAYS)
    assert evaluate_exclusion(m, min_amount=5e7) == ("excluded", "stale_candle")


# ── 分位數母體純度 ────────────────────────────────────────────
def test_quantile_edges_need_at_least_three_values():
    assert quantile_bucket_edges([0.01, 0.02]) is None
    edges = quantile_bucket_edges([0.01, 0.02, 0.03, 0.04, 0.05, 0.06])
    assert edges is not None and edges[0] < edges[1]


def test_quantile_bucket_uses_only_liquid_stocks_population():
    """**這是最容易錯的地方**：分位數的母體必須只含流動性合格的股票。

    混進 ETF（債券 ETF 全在低波動端）或低流動性股票，切出來的門檻沒有意義。
    這裡直接斷言 build_report 用來算切點的集合。
    """
    days = MARKET_DAYS[:MIN_CANDLES_FOR_PROFILE]
    metrics = [
        # 合格股票：三檔不同波動
        compute_symbol_metrics(_meta("A"), _candles(days, rng=1.2, amount=1e8), MARKET_DAYS),
        compute_symbol_metrics(_meta("B"), _candles(days, rng=3.0, amount=1e8), MARKET_DAYS),
        compute_symbol_metrics(_meta("C"), _candles(days, rng=8.0, amount=1e8), MARKET_DAYS),
        # 債券 ETF：極低波動、流動性足 → 不該影響股票切點
        compute_symbol_metrics(
            _meta("00679B", security_type="ETF", industry=""),
            _candles(days, rng=0.3, amount=1e8), MARKET_DAYS),
        # 低流動性股票：極低波動但沒人交易 → 不該影響股票切點
        compute_symbol_metrics(_meta("Z"), _candles(days, rng=0.4, amount=1e5), MARKET_DAYS),
    ]
    report = build_report(metrics, snapshot={}, min_amount=2e7)

    # 期望值必須用**與 pipeline 相同的基準**（max(atr, range)），不是單獨的 atr_pct——
    # 兩者在半數標的上不同，見 bucket_basis 的說明。
    stock_bases = sorted(
        bucket_basis(m) for m in metrics
        if m["security_type"] == "股票" and m["avg_amount_60"] >= 2e7
    )
    assert report["quantile_edges"] == list(quantile_bucket_edges(stock_bases)), (
        "切點的母體混進了 ETF 或低流動性股票，或基準與 pipeline 不同源"
    )


# ── 門檻矩陣的性質 ────────────────────────────────────────────
def test_threshold_matrix_is_monotonic_and_nested():
    """門檻遞增時合格集合必須遞減，且**後者是前者的子集合**。

    這是不論資料如何都必須成立的性質，違反即為邏輯錯誤（計畫書「階段 3」）。
    """
    days = MARKET_DAYS[:MIN_CANDLES_FOR_PROFILE]
    metrics = [
        compute_symbol_metrics(_meta(f"S{i}"), _candles(days, rng=1.0 + i, amount=amt), MARKET_DAYS)
        for i, amt in enumerate([5e6, 1.5e7, 3e7, 6e7, 2e8])
    ]
    matrix = build_threshold_matrix(metrics)

    counts = [row["qualified_stocks"] for row in matrix]
    assert counts == sorted(counts, reverse=True), f"合格數沒有隨門檻遞減：{counts}"
    for prev, nxt in zip(matrix, matrix[1:]):
        assert set(nxt["stock_symbols"]) <= set(prev["stock_symbols"]), (
            f"{nxt['threshold']} 的合格集合不是 {prev['threshold']} 的子集合"
        )


def test_report_lists_are_exhaustive_and_disjoint():
    """selected ∪ excluded ＝ 全體，且互斥。"""
    days = MARKET_DAYS[:MIN_CANDLES_FOR_PROFILE]
    metrics = [
        compute_symbol_metrics(_meta("A"), _candles(days, amount=1e8), MARKET_DAYS),
        compute_symbol_metrics(_meta("B"), _candles(days, amount=1e5), MARKET_DAYS),
        compute_symbol_metrics(_meta("C"), _candles(MARKET_DAYS[:5]), MARKET_DAYS),
    ]
    report = build_report(metrics, snapshot={}, min_amount=2e7)

    s = report["summary"]
    assert s["selected"] + s["excluded"] == s["total"] == len(metrics)
    assert sum(s["exclusion_reasons"].values()) == s["excluded"], "排除原因總和對不上"


def test_report_records_snapshot():
    """沒有快照，日後重跑拿到不同數字時無法判斷是資料變了還是程式錯了。"""
    report = build_report([], snapshot={"symbols_with_candles": 857}, min_amount=2e7)
    assert report["snapshot"]["symbols_with_candles"] == 857
    assert report["schema_version"] == "selection-report-p2"


@pytest.mark.parametrize("atr,expected", [
    (0.005, "LOW_VOLATILITY"), (0.03, "NORMAL_VOLATILITY"), (0.09, "HIGH_VOLATILITY"),
])
def test_assign_quantile_bucket(atr, expected):
    assert assign_quantile_bucket(atr, (0.02, 0.05)) == expected


def test_assign_quantile_bucket_without_edges_is_stale():
    assert assign_quantile_bucket(0.03, None) == BUCKET_STALE
    assert assign_quantile_bucket(None, (0.02, 0.05)) == BUCKET_STALE


# ── universe_role：債券／槓桿 ETF 不得參與股票決策 ────────────
@pytest.mark.parametrize("symbol,security_type,expected", [
    ("2330", "股票", "primary"),
    ("0050", "ETF", "primary"),          # 純數字代號＝股票型
    ("006208", "ETF", "primary"),
    ("00679B", "ETF", "supplemental"),   # B＝債券
    ("00631L", "ETF", "supplemental"),   # L＝槓桿正2
    ("00632R", "ETF", "supplemental"),   # R＝反向反1
    ("00635U", "ETF", "supplemental"),   # U＝期貨商品
    ("00400A", "ETF", "supplemental"),   # A＝主動式
    ("00625K", "ETF", "supplemental"),   # K＝特殊計價級別
])
def test_universe_role_uses_twse_suffix_convention(symbol, security_type, expected):
    """代號後綴比名稱比對可靠，且規則刻意保守——帶字母後綴一律 supplemental。

    誤標成 supplemental 只是少一檔交叉觀察；誤標成 primary 會讓債券或槓桿商品
    去影響股票的 zone builder 參數。
    """
    from selection_report import classify_universe_role
    assert classify_universe_role(symbol, security_type) == expected


def test_report_counts_primary_and_supplemental_separately():
    """最終 universe 要看得出「有多少檔真的能參與股票決策」。"""
    days = MARKET_DAYS[:MIN_CANDLES_FOR_PROFILE]
    metrics = [
        compute_symbol_metrics(_meta("2330"), _candles(days, amount=1e8), MARKET_DAYS),
        compute_symbol_metrics(
            _meta("0050", security_type="ETF", industry=""),
            _candles(days, amount=1e8), MARKET_DAYS),
        compute_symbol_metrics(
            _meta("00679B", security_type="ETF", industry=""),
            _candles(days, rng=0.3, amount=1e8), MARKET_DAYS),
    ]
    report = build_report(metrics, snapshot={}, min_amount=2e7)
    assert report["summary"]["selected_primary"] == 2      # 2330 + 0050
    assert report["summary"]["selected_supplemental"] == 1  # 00679B


# ── 最終 universe 選取 ────────────────────────────────────────
def _row(symbol, bucket, industry, amount, listed="2010-01-01", role="primary", status="selected"):
    return {"symbol": symbol, "selection_bucket": bucket, "industry": industry,
            "avg_amount_60": amount, "listed_date": listed,
            "universe_role": role, "selection_status": status}


def test_select_universe_caps_single_industry_per_bucket():
    """半導體業有 201 檔，不設上限會直接主導整個 bucket。"""
    from selection_report import select_universe
    rows = [_row(f"S{i:03d}", "HIGH_VOLATILITY", "半導體業", 1e9 - i) for i in range(50)]
    rows += [_row(f"T{i:03d}", "HIGH_VOLATILITY", "航運業", 5e8 - i) for i in range(50)]
    out = select_universe(rows, "2026-08-17", per_bucket_max=40, industry_cap_ratio=0.25)

    inds = out["per_bucket"]["HIGH_VOLATILITY"]["industries"]
    assert inds["半導體業"] == 10, f"單一產業超過上限：{inds}"
    assert inds["航運業"] == 10


def test_select_universe_keeps_watchlist_unconditionally():
    """watchlist 是回歸檢查的基準，無條件保留且不佔產業配額。"""
    from selection_report import select_universe
    rows = [_row("2330", "NORMAL_VOLATILITY", "半導體業", 1e5)]  # 成交額極低也要留
    rows += [_row(f"X{i:03d}", "NORMAL_VOLATILITY", "航運業", 1e9 - i) for i in range(5)]
    out = select_universe(rows, "2026-08-17", keep_symbols=["2330"])

    assert "2330" in out["selected_symbols"]
    assert out["selection_reason"]["2330"] == "watchlist_baseline"


def test_select_universe_keeps_unqualified_watchlist_but_not_as_baseline():
    """分級保留：算得出 bucket 的排除原因仍留在 universe，只是不當基準。

    以前這裡是靜默丟掉——watchlist 少一檔完全看不出來。
    """
    from selection_report import select_universe
    rows = [_row("6243", "HIGH_VOLATILITY", "電子零組件業", 1e6,
                 status="excluded") | {"exclusion_reason": "low_liquidity",
                                       "total_candle_count": 3000}]
    out = select_universe(rows, "2026-08-17", keep_symbols=["6243"])

    assert out["selected_symbols"] == ["6243"], "被排除的 watchlist 仍應留在 universe"
    assert out["selection_reason"]["6243"] == "watchlist_kept", "只是留著，不是基準"
    assert out["baseline_excluded"] == {"6243": "low_liquidity"}
    assert out["regression_baseline_symbols"] == []


def test_select_universe_drops_stale_watchlist():
    """`stale_candle` 是唯一「保留也沒意義」的原因：沒有 bucket 也產不出 profile。"""
    from selection_report import select_universe
    rows = [_row("DEAD", "STALE", "半導體業", 0.0, status="excluded")
            | {"exclusion_reason": "stale_candle", "total_candle_count": 3000}]
    out = select_universe(rows, "2026-08-17", keep_symbols=["DEAD"])

    assert out["selected_symbols"] == [], "無桶標的不該塞進 universe"
    assert out["keep_symbols_dropped"] == {"DEAD": "stale_candle"}
    assert out["regression_baseline_symbols"] == []


def test_select_universe_reports_missing_keep_symbols():
    """主檔／K 線裡找不到的 keep symbol 不能靜默忽略。"""
    from selection_report import select_universe
    out = select_universe([], "2026-08-17", keep_symbols=["9999"])
    assert out["keep_symbols_missing"] == ["9999"]
    assert out["selected_symbols"] == []


def test_select_universe_flags_watchlist_without_walk_forward_depth():
    """watchlist 繞過年限檢查，所以會放進深度不足的標的：留在池內但不算回歸基準。

    實例是 00981A（299 根）與 00947（529 根）——`--limit 1500` 的 walk-forward 撐不起來。
    """
    from selection_report import select_universe
    rows = [_row("00981A", "NORMAL_VOLATILITY", "", 1e8, listed="2025-05-27") | {"total_candle_count": 299},
            _row("0050", "LOW_VOLATILITY", "", 1e9, listed="2003-06-25") | {"total_candle_count": 4871}]
    out = select_universe(rows, "2026-08-17", keep_symbols=["00981A", "0050"])

    assert set(out["selected_symbols"]) == {"00981A", "0050"}, "深度不足者仍應留在 universe"
    assert out["insufficient_depth"] == ["00981A"]
    assert out["insufficient_depth_detail"]["00981A"]["total_candle_count"] == 299
    assert out["insufficient_depth_detail"]["00981A"]["kind"] == "listing_age", (
        "上市才一年，299 根已是全部歷史——不該被當成可以回補的"
    )
    assert out["regression_baseline_symbols"] == ["0050"]
    assert out["regression_baseline_count"] == 1


def test_select_universe_distinguishes_backfillable_shortfall():
    """1569 濱川的形狀：2005 年上市，庫內只有 158 列——深補就會好。

    `min_listed_years` 用 `listed_date`，結構上抓不到這種「上市很久但沒抓」的標的，
    所以它是靠 bucket 名額正常選進來的，不是 watchlist 保留。
    """
    from selection_report import select_universe
    rows = [_row("1569", "HIGH_VOLATILITY", "電子零組件業", 1e9, listed="2005-07-29")
            | {"total_candle_count": 158}]
    out = select_universe(rows, "2026-08-17")

    assert out["selection_reason"]["1569"] == "bucket:HIGH_VOLATILITY"
    assert out["insufficient_depth_detail"]["1569"]["kind"] == "backfillable"


def test_depth_shortfall_slack_does_not_excuse_long_listed_shallow_symbols():
    """10% 餘裕只適用於「天花板是上市年數」的情況。

    上市 8 年、庫內 1,350 列（該有約 1,944 列）必須是 `backfillable`——
    若對 `min(ceiling, 1500)` 無條件留 10%，它會被標成 `listing_age` 而躲過
    「階段 5 前補完」的檢查，evaluation 就拿殘缺資料跑了。
    """
    from selection_report import _depth_shortfall_kind

    assert _depth_shortfall_kind(1350, "2018-08-17", "2026-08-17", 1500) == "backfillable"
    assert _depth_shortfall_kind(1400, "2011-01-01", "2026-08-17", 1500) == "backfillable"
    # 天花板低於門檻時仍要容忍停牌與交易日估算誤差
    assert _depth_shortfall_kind(1476, "2020-07-22", "2026-08-17", 1500) == "listing_age"
    assert _depth_shortfall_kind(299, "2025-05-27", "2026-08-17", 1500) == "listing_age"


def test_select_universe_depth_flag_skips_rows_without_candle_count():
    """精簡 row 沒有 total_candle_count 時是「未知」，不能當成深度不足。"""
    from selection_report import select_universe
    rows = [_row("2330", "NORMAL_VOLATILITY", "半導體業", 1e9)]
    out = select_universe(rows, "2026-08-17", keep_symbols=["2330"])
    assert out["insufficient_depth"] == []
    assert out["regression_baseline_symbols"] == ["2330"]


def test_supplemental_symbols_cover_etf_suffix_and_degraded_watchlist():
    """在 universe 內但不得影響股票 builder 決策者，兩個來源都要列出。"""
    from selection_report import select_universe
    rows = [
        _row("00981A", "NORMAL_VOLATILITY", "", 1e8, role="supplemental")
        | {"total_candle_count": 3000},
        _row("6243", "HIGH_VOLATILITY", "電子零組件業", 1e6, status="excluded")
        | {"exclusion_reason": "low_liquidity", "total_candle_count": 3000},
        _row("2330", "LOW_VOLATILITY", "半導體業", 1e9) | {"total_candle_count": 3000},
    ]
    out = select_universe(rows, "2026-08-17", keep_symbols=["00981A", "6243", "2330"])

    assert out["supplemental_symbols"] == {"00981A": "etf_suffix", "6243": "low_liquidity"}
    assert out["primary_count"] == 1


def test_keep_symbols_drift_detects_both_directions():
    """凍結的 --keep-symbols 與 DB watchlist 分歧時必須看得見。"""
    from selection_report import _keep_symbols_drift

    same = _keep_symbols_drift(["0050", "2330"], ["2330", "0050"])
    assert same["in_sync"] is True
    assert same["only_in_watchlist"] == [] and same["only_in_keep_symbols"] == []

    drift = _keep_symbols_drift(["0050", "2330"], ["2330", "6505"])
    assert drift["in_sync"] is False
    assert drift["only_in_watchlist"] == ["6505"], "watchlist 新增的標的沒被看見"
    assert drift["only_in_keep_symbols"] == ["0050"], "已移出 watchlist 的標的沒被看見"


def test_select_universe_excludes_recent_listings():
    """上市未滿 5 年無法支撐 walk-forward 深度。"""
    from selection_report import select_universe
    rows = [_row("NEW", "HIGH_VOLATILITY", "半導體業", 1e9, listed="2024-01-01"),
            _row("OLD", "HIGH_VOLATILITY", "半導體業", 1e8, listed="2010-01-01")]
    out = select_universe(rows, "2026-08-17")
    assert out["selected_symbols"] == ["OLD"]


def test_select_universe_excludes_supplemental():
    """債券／槓桿 ETF 不進主池。"""
    from selection_report import select_universe
    rows = [_row("00679B", "LOW_VOLATILITY", "", 1e9, role="supplemental"),
            _row("2633", "LOW_VOLATILITY", "航運業", 1e8)]
    out = select_universe(rows, "2026-08-17")
    assert out["selected_symbols"] == ["2633"]


def test_select_universe_flags_underfilled_bucket_without_backfilling():
    """配額填不滿時要標示出來，**不可用低流動性股票硬補**。"""
    from selection_report import select_universe
    rows = [_row(f"L{i}", "LOW_VOLATILITY", f"產業{i}", 1e8) for i in range(3)]
    out = select_universe(rows, "2026-08-17", per_bucket_min=35)
    assert out["per_bucket"]["LOW_VOLATILITY"]["underfilled"] is True
    assert out["per_bucket"]["LOW_VOLATILITY"]["picked"] == 3


def test_select_universe_is_deterministic():
    """同輸入必得同輸出——同額時以代號打破平手。"""
    from selection_report import select_universe
    rows = [_row(f"S{i:03d}", "HIGH_VOLATILITY", "半導體業", 1e8) for i in range(20)]
    a = select_universe(rows, "2026-08-17", per_bucket_max=5)
    b = select_universe(list(reversed(rows)), "2026-08-17", per_bucket_max=5)
    assert a["selected_symbols"] == b["selected_symbols"]


def test_select_universe_gives_etf_its_own_quota():
    """ETF 不是一個產業，是不同的商品類別——不該和產業共用配額。

    實測發現 11 檔股票型 ETF（industry 為空字串）被歸成「(未分類)」這一個假產業，
    吃掉 LOW bucket 整個產業名額。
    """
    from selection_report import select_universe
    rows = [dict(_row(f"E{i:03d}", "LOW_VOLATILITY", "", 1e9 - i), security_type="ETF")
            for i in range(20)]
    rows += [dict(_row(f"S{i:03d}", "LOW_VOLATILITY", f"產業{i}", 1e8 - i), security_type="股票")
             for i in range(20)]
    out = select_universe(rows, "2026-08-17", per_bucket_max=20, etf_per_bucket=2)

    st = out["per_bucket"]["LOW_VOLATILITY"]
    assert st["etf_picked"] == 2, "ETF 配額沒生效"
    assert "(未分類)" not in st["industries"], "ETF 仍在佔用產業配額"
    assert st["picked"] == 20


def test_report_exposes_threshold_gap_for_t003():
    """報告要並列 pipeline 絕對門檻與實際分位數切點——那是 T-003 的輸入。"""
    days = MARKET_DAYS[:MIN_CANDLES_FOR_PROFILE]
    metrics = [
        compute_symbol_metrics(_meta(f"S{i}"), _candles(days, rng=1.0 + i * 2, amount=1e8), MARKET_DAYS)
        for i in range(4)
    ]
    report = build_report(metrics, snapshot={}, min_amount=2e7)
    gap = report["threshold_gap"]
    # **不要斷言字面數值**：原本這裡寫 {"low_max": 0.015, "high_min": 0.035}，
    # 於是 2026-08-17 重定門檻時它「通過」了，卻沒發現報告開始宣稱 pipeline 用舊值。
    # 斷言的對象要是「與 zone_builder 同源」這個性質，不是某一組數字。
    assert gap["pipeline_absolute"]["low_max"] == LOW_VOLATILITY_THRESHOLD
    assert gap["pipeline_absolute"]["high_min"] == HIGH_VOLATILITY_THRESHOLD
    assert gap["liquid_stock_quantile"]["p33"] < gap["liquid_stock_quantile"]["p67"]


def test_threshold_gap_reports_live_pipeline_constants_not_copies():
    """`threshold_gap` 必須反映 zone_builder 的**當下**常數，不能是手抄的數值。

    原本硬編 0.015 / 0.035，2026-08-17 重定門檻後報告就開始宣稱 pipeline 用舊值——
    而「常數的手抄鏡像會過期」正是重定門檻要消滅的那類問題。
    """
    days = MARKET_DAYS[:MIN_CANDLES_FOR_PROFILE]
    metrics = [
        compute_symbol_metrics(_meta(f"S{i}"), _candles(days, rng=1.0 + i, amount=1e8), MARKET_DAYS)
        for i in range(4)
    ]
    gap = build_report(metrics, snapshot={}, min_amount=2e7)["threshold_gap"]

    assert gap["pipeline_absolute"]["low_max"] == LOW_VOLATILITY_THRESHOLD
    assert gap["pipeline_absolute"]["high_min"] == HIGH_VOLATILITY_THRESHOLD
    # 合成資料的切點不會等於凍結值，所以 aligned 應為 False——它是訊號不是錯誤
    assert gap["aligned"] is False


# ── 匯入 payload ──────────────────────────────────────────────
def _full_report():
    return {
        "schema_version": "selection-report-p2",
        "snapshot": {"last_trading_day": "2026-08-17"},
        "quantile_edges": [0.0461, 0.0628],
        "threshold_gap": {"aligned": True, "pipeline_absolute": {"low_max": 0.0461, "high_min": 0.0628}},
        "symbols": [
            {"symbol": "2330", "selection_bucket": "LOW_VOLATILITY", "universe_role": "primary",
             "atr_pct": 0.026, "avg_amount_60": 1e10},
            {"symbol": "6243", "selection_bucket": "HIGH_VOLATILITY", "universe_role": "primary"},
            {"symbol": "9999", "selection_bucket": "LOW_VOLATILITY", "universe_role": "primary"},
        ],
        "universe": {
            "selected_symbols": ["2330", "6243"],
            "supplemental_symbols": {"6243": "low_liquidity", "9999": "etf_suffix"},
            "insufficient_depth_detail": {"6243": {"total_candle_count": 299, "kind": "listing_age"}},
        },
    }


def test_import_payload_keeps_only_what_the_importer_reads():
    """完整報告 672KB / 857 檔，匯入只需要選池那幾檔的三個欄位。"""
    from selection_report import build_import_payload

    imp = build_import_payload(_full_report())

    assert [r["symbol"] for r in imp["symbols"]] == ["2330", "6243"]
    # 逐檔只留 importer 會讀的欄位，不夾帶 atr_pct 等判讀用資料
    assert set(imp["symbols"][0]) == {"symbol", "selection_bucket", "universe_role"}
    # universe 的三個欄位也要依成員過濾：9999 不在池內，它的 supplemental 標記不該跟著來
    assert imp["universe"]["supplemental_symbols"] == {"6243": "low_liquidity"}
    assert set(imp["universe"]["insufficient_depth_detail"]) == {"6243"}
    # 邊界與 threshold_gap 原樣帶過去——那是 bucket_hint 的判定依據
    assert imp["quantile_edges"] == [0.0461, 0.0628]
    assert imp["threshold_gap"]["aligned"] is True
    assert imp["missing_symbols"] == []


def test_import_payload_pins_membership_but_takes_current_buckets():
    """釘住 membership、bucket 取本次重算值。

    選池是人工決策且已深補完，但重跑時資料與分桶基準都可能已變
    （實測 131 → 126）。兩者必須能分開指定。
    """
    from selection_report import build_import_payload

    # 釘住 9999（本次報告沒把它選進池），排除 6243
    imp = build_import_payload(_full_report(), ["2330", "9999"])

    assert imp["universe"]["selected_symbols"] == ["2330", "9999"]
    assert [r["symbol"] for r in imp["symbols"]] == ["2330", "9999"]
    # bucket 取的是本次報告算出來的值，不是釘住時的舊值
    assert imp["symbols"][1]["selection_bucket"] == "LOW_VOLATILITY"


def test_import_payload_reports_pinned_symbols_missing_from_report():
    """釘住的成員在本次報告裡找不到時要顯式列出，不能靜默少幾檔。"""
    from selection_report import build_import_payload

    imp = build_import_payload(_full_report(), ["2330", "NOPE"])

    assert imp["missing_symbols"] == ["NOPE"]
    # 找不到的不會出現在 selected_symbols，否則前端 parser 會因缺 bucket 整份拒絕
    assert imp["universe"]["selected_symbols"] == ["2330"]
