from backtest.modular.sr_scoring.tips import _build_analysis_tips, analysis_status_tips


def test_analysis_status_tips_include_fixed_enum_catalog():
    tips = analysis_status_tips()
    tips_text = "\n".join(tips)

    assert "事件生命週期｜ACTIVE：有效中。事件仍在有效期限內" in tips_text
    assert "區間分級｜TIER_2_TRADING_ZONE：交易區。較接近實際交易決策的區間" in tips_text
    assert "證據分級｜PENDING_VALIDATION：等待驗證。區間尚未有足夠後續資料驗證" in tips_text
    assert "多空傾向｜BULLISH_CONTINUATION：多頭延續。短線與結構同向偏多" in tips_text
    assert "趨勢分級｜TREND_DOWN：下降趨勢。中期結構偏空" in tips_text
    assert "市場行為｜CONDITIONAL_HOLD：條件式持有。屬部位防守 gate（position_gate_state）" in tips_text
    assert "進場權限｜PROBE_ALLOWED：允許觀察性試探。日 K gate 未阻擋" in tips_text
    assert "日 K Gate｜WAIT_DAILY_CONFIRM：等待日 K 確認。尚缺收盤" in tips_text
    assert "RR 與模型｜RR_QUALIFIED：RR 合格。風險報酬通過最低門檻" in tips_text


def test_build_analysis_tips_includes_status_catalog():
    tips = _build_analysis_tips([], 100.0, None, None)
    tips_text = "\n".join(tips)

    assert "指標小辭典｜RR：Risk/Reward" in tips_text
    assert "價位語意｜Support：支撐是現價下方可能出現承接的區間" in tips_text
    assert "事件語意｜Reclaim：Reclaim 指跌破後重新站回關鍵區間" in tips_text
    assert "進場權限｜ENTRY_ALLOWED：允許進場。市場、事件、行為與日 K gate 同步放行" in tips_text
    assert "判讀提醒｜均線：5日均線資料不足" in tips_text
