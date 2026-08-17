"""baseline_check 的單元測試（計畫書階段 6）。

重點是**這些檢查該對什麼敏感、對什麼不敏感**：
資料前進幾天不該報警，pipeline 算錯必須報警。
"""
from __future__ import annotations

from baseline_check import build_baseline, compare, spearman


def _report(atr: dict[str, float], *, bucket=None) -> dict:
    def _b(a):
        if bucket:
            return bucket
        return "LOW_VOLATILITY" if a < 0.015 else ("HIGH_VOLATILITY" if a > 0.035 else "NORMAL_VOLATILITY")
    return {
        "schema_version": "sr_zone_evaluation_p0",
        "pipeline_version": "p0",
        "timeframe": "1d",
        # 真實的 report 是 dict（symbol 當 key），不是 list——實跑才發現
        "volatility_profiles": {
            s: {"symbol": s, "atr_pct": a, "average_range_pct": a * 0.9, "bucket": _b(a),
                "candle_count": 1500, "lookback_bars": 60}
            for s, a in atr.items()
        },
    }


# 2026-08-17 實測的 9 檔回歸基準形狀
LIVE = {"0050": 0.0260, "00830": 0.0300, "2330": 0.0265, "2399": 0.0552,
        "2454": 0.0599, "2478": 0.0838, "3630": 0.0446, "5490": 0.0637, "6243": 0.0860}


def _baseline(atr=LIVE):
    return build_baseline(_report(atr), atr.keys(), {"last_trading_day": "2026-08-17"})


# ── 對資料前進不敏感 ──────────────────────────────────────────
def test_uniform_drift_does_not_fail():
    """全體同比例下滑（近 60 根窗口滾動的典型形狀）不該報警。

    實測 6243 在 11 天內從 11.6% 掉到 8.60%——那是預期，不是壞掉。
    """
    drifted = {s: a * 0.78 for s, a in LIVE.items()}
    r = compare(_baseline(), _report(drifted))
    assert r["passed"], [c for c in r["checks"] if not c["passed"]]


def test_small_adjacent_swap_tolerated():
    """相鄰兩檔互換（2454 0.0599 與 5490 0.0637）在 0.9 門檻下應容許。"""
    swapped = dict(LIVE); swapped["2454"], swapped["5490"] = LIVE["5490"], LIVE["2454"]
    r = compare(_baseline(), _report(swapped))
    rho = [c for c in r["checks"] if "Spearman" in c["name"]][0]["detail"]["spearman"]
    assert rho >= 0.9, f"相鄰互換不該掉到 {rho}"


# ── 對 pipeline 出錯敏感 ──────────────────────────────────────
def test_reversed_ranking_fails():
    """ATR 公式改壞導致排序整體翻轉，必須抓到。"""
    vals = sorted(LIVE.values())
    reversed_map = dict(zip(LIVE.keys(), reversed([LIVE[s] for s in LIVE])))
    # 直接構造完全反序：最高變最低
    order = sorted(LIVE, key=lambda s: LIVE[s])
    flipped = {s: v for s, v in zip(order, reversed(vals))}
    r = compare(_baseline(), _report(flipped))
    assert not r["passed"]
    names = [c["name"] for c in r["checks"] if not c["passed"]]
    assert any("Spearman" in n for n in names)
    assert any("最高" in n for n in names), f"極值檢查沒抓到：{names}"
    assert reversed_map  # 保留變數以說明意圖，不參與斷言


def test_missing_symbol_fails():
    """基準有但 report 缺的標的必須報警，不能靜默略過。"""
    partial = {s: a for s, a in LIVE.items() if s != "6243"}
    r = compare(_baseline(), _report(partial))
    assert not r["passed"]
    chk = [c for c in r["checks"] if "都有 profile" in c["name"]][0]
    assert chk["detail"]["missing"] == ["6243"]


def test_bucket_crossing_warns_but_does_not_block():
    """bucket 跨越要被看見，但**不能讓階段 6 失敗**。

    絕對門檻與台股分佈差一個量級，標的常態貼在 3.5% 附近，普通資料漂移就會跨過去——
    實證是 2026-08-06 到 08-17 的 11 天內 HIGH 由 9 檔變 6 檔，pipeline 沒有任何改動。
    設成 blocking 會讓這道檢查常態失敗然後被當成雜訊忽略。
    """
    # 挑 3630（0.0446，新門檻下屬 LOW）推到 0.050 → NORMAL。
    # 刻意選中段的標的：排名與極值都不變，才能單獨驗「跨越不阻擋」而不誤觸其他 blocking 項。
    crossed = dict(LIVE); crossed["3630"] = 0.050
    r = compare(_baseline(), _report(crossed))
    chk = [c for c in r["checks"] if "bucket 跨越" in c["name"]][0]
    assert chk["blocking"] is False
    assert not chk["passed"]
    assert chk["detail"]["moved"]["3630"] == ["LOW_VOLATILITY", "NORMAL_VOLATILITY"]
    assert r["passed"], "bucket 跨越不該讓整體失敗"
    assert r["warnings"] == ["bucket 跨越（觀察項，不阻擋）"]


# ── Spearman 的邊界行為 ───────────────────────────────────────
def test_spearman_returns_none_when_uninformative():
    """樣本不足或全平手時回 None，**不可當成通過**。"""
    assert spearman({"A": 1.0}, {"A": 1.0}) is None, "1 檔算不出相關係數"
    assert spearman({"A": 1.0, "B": 1.0}, {"A": 2.0, "B": 3.0}) is None, "全平手沒有排序資訊"


def test_none_spearman_fails_the_check():
    """rho 為 None 時該項必須 FAIL，而不是因為 `None >= 0.9` 拋錯或被跳過。"""
    one = {"0050": 0.026}
    r = compare(build_baseline(_report(one), one.keys(), {}), _report(one))
    chk = [c for c in r["checks"] if "Spearman" in c["name"]][0]
    assert chk["passed"] is False
    assert chk["detail"]["spearman"] is None


# ── 基準檔必須帶資料快照 ──────────────────────────────────────
def test_baseline_records_snapshot():
    """少了快照就無法判斷差異來自 pipeline 還是資料——那正是原定義失敗的原因。"""
    b = _baseline()
    assert b["snapshot"] == {"last_trading_day": "2026-08-17"}
    assert b["symbols"] == sorted(LIVE)
    assert b["missing"] == []
    assert b["profiles"]["6243"]["atr_pct"] == 0.0860


def test_profiles_by_symbol_accepts_both_shapes():
    """實際 report 是 dict；list 形狀也要吃（第一版只處理 list，實跑才發現）。"""
    from baseline_check import profiles_by_symbol

    as_dict = {"2330": {"symbol": "2330", "atr_pct": 0.0265}}
    as_list = [{"symbol": "2330", "atr_pct": 0.0265}]
    assert profiles_by_symbol(as_dict) == profiles_by_symbol(as_list)
    assert profiles_by_symbol({}) == {} and profiles_by_symbol([]) == {}
    assert profiles_by_symbol(None) == {}


def test_baseline_records_thresholds_and_derives_bucket_from_them():
    """bucket 是門檻的函數，所以基準必須記下當時在位的門檻。

    2026-08-17 門檻由 1.5%/3.5% 重定為凍結的分位數。少了 `thresholds`，
    下次比對看到 bucket 變了會分不清是資料漂移還是門檻被改。
    也不能照抄 report 的 bucket——那份 report 可能是重定之前跑的。
    """
    from baseline_check import (HIGH_VOLATILITY_THRESHOLD, LOW_VOLATILITY_THRESHOLD,
                                build_baseline)

    # report 帶一個**過期**的 bucket 值，build_baseline 應以現行門檻重算而非照抄
    rep = {"volatility_profiles": {"2330": {"symbol": "2330", "atr_pct": 0.0265,
                                           "average_range_pct": 0.0196,
                                           "bucket": "STALE_FROM_OLD_RUN"}}}
    b = build_baseline(rep, ["2330"], {})
    assert b["thresholds"]["low_volatility_max"] == LOW_VOLATILITY_THRESHOLD
    assert b["thresholds"]["high_volatility_min"] == HIGH_VOLATILITY_THRESHOLD
    assert b["profiles"]["2330"]["bucket"] == "LOW_VOLATILITY", "0.0265 在新門檻下是 LOW"
    assert b["profiles"]["2330"]["bucket"] != "STALE_FROM_OLD_RUN"
