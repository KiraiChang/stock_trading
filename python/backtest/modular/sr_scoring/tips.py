"""SR Zone report reading tips.

Tips are intentionally framed as technical-analysis reading guidance, not UI
instructions or implementation notes. The public response still returns a flat
list of strings to keep the API compatible with existing frontend clients.
"""
from __future__ import annotations

from typing import Optional

from .formatting import fmt_price

CHIP_SIGNAL_WEAK_THRESHOLD = 10.0
CHIP_SIGNAL_STRONG_THRESHOLD = 30.0

ANALYSIS_STATUS_TIPS: dict[str, list[dict[str, str]]] = {
    "事件語意": [
        {
            "code": "EXTREME_VOLUME",
            "name": "極端量能",
            "description": "最新量能明顯放大，只代表需要提高注意，不單獨等於偏多或偏空。",
        },
        {
            "code": "HIGH_VOLUME_BREAKDOWN",
            "name": "放量破位",
            "description": "支撐區被觸碰且量能放大：收盤跌破會升為已確認、防守優先；僅盤中跌破則先列為候選，需觀察是否收復。",
        },
        {
            "code": "INTRADAY_RECLAIM",
            "name": "收盤收復",
            "description": "價格測試支撐後收盤重新站回區間上緣，代表結構修復的候選訊號。",
        },
        {
            "code": "REVERSAL_CANDIDATE",
            "name": "反轉候選",
            "description": "支撐測試尚未失守，但還沒完成延續確認，只能先列入觀察。",
        },
    ],
    "事件生命週期": [
        {
            "code": "CANDIDATE",
            "name": "候選",
            "description": "事件剛出現或證據不足，不能直接拿來當進場或出場依據。",
        },
        {
            "code": "CONFIRMED",
            "name": "已確認",
            "description": "事件已符合收盤或規則確認，可以進入後續 market state 判讀。",
        },
        {
            "code": "ACTIVE",
            "name": "有效中",
            "description": "事件仍在有效期限內，會影響風險、路徑或進場 gate。",
        },
        {
            "code": "RESOLVED",
            "name": "已解除",
            "description": "原事件已被後續相反事件化解，保留紀錄但不再作為 active gate。",
        },
        {
            "code": "EXPIRED",
            "name": "已過期",
            "description": "事件超過有效 K 棒數，不能再用來解讀當前決策。",
        },
    ],
    "區間分級": [
        {
            "code": "SUPPORT",
            "name": "支撐",
            "description": "現價下方或價格已站回的承接區，只是觀察區，不是自動買點。",
        },
        {
            "code": "RESISTANCE",
            "name": "壓力",
            "description": "現價上方或價格尚未突破的供給區，只是觀察區，不是自動放空點。",
        },
        {
            "code": "AT_ZONE",
            "name": "區間內",
            "description": "價格位於區間內，方向未明，應等待收盤站回、跌破或突破確認。",
        },
        {
            "code": "TIER_1_MAIN_STRUCTURE",
            "name": "主結構",
            "description": "最主要的結構區間，常用於持倉防守或大方向判讀。",
        },
        {
            "code": "TIER_2_TRADING_ZONE",
            "name": "交易區",
            "description": "較接近實際交易決策的區間，需搭配 RR、事件與日 K gate。",
        },
        {
            "code": "TIER_3_SHORT_TERM",
            "name": "短期區",
            "description": "短線參考價位，容易受波動影響，不宜單獨當主要決策依據。",
        },
    ],
    "證據分級": [
        {
            "code": "VALIDATED_RECENTLY",
            "name": "近期驗證",
            "description": "區間近期被測試後守住或有效反應，參考性較高。",
        },
        {
            "code": "PENDING_VALIDATION",
            "name": "等待驗證",
            "description": "區間尚未有足夠後續資料驗證，需降低解讀權重。",
        },
        {
            "code": "NOT_TESTED_RECENTLY",
            "name": "近期未測試",
            "description": "過去有效但近期沒有新測試，仍可參考但需要重新確認。",
        },
        {
            "code": "CONFIRMED",
            "name": "量能確認",
            "description": "量能配合價格反應，但仍需確認方向，不代表所有訊號都可交易。",
        },
        {
            "code": "WEAK",
            "name": "量能偏弱",
            "description": "量能不足以支撐明確判斷，訊號需要更多 K 棒確認。",
        },
        {
            "code": "FAILED",
            "name": "量能失敗",
            "description": "量能放大但價格往不利方向走，通常代表原設定失效風險提高。",
        },
    ],
    "市場狀態": [
        {
            "code": "NORMAL",
            "name": "一般狀態",
            "description": "沒有明確短線事件主導，回到趨勢、區間與風險報酬判斷。",
        },
        {
            "code": "BREAKDOWN_RISK",
            "name": "跌破風險",
            "description": "有效偏空事件仍在，持有者需防守，未持有者不應急著進場。",
        },
        {
            "code": "RECLAIM_ATTEMPT",
            "name": "收復嘗試",
            "description": "價格已嘗試站回關鍵區，仍要看隔日或後續是否延續。",
        },
        {
            "code": "REVERSAL_CANDIDATE",
            "name": "反轉觀察",
            "description": "可能出現反轉，但還沒有足夠證據轉成趨勢延續。",
        },
        {
            "code": "RECOVERY",
            "name": "回復修復",
            "description": "屬短線 regime（short_term_regime），支撐收復已確認、偏向結構修復，但仍不等於可無條件進場。",
        },
        {
            "code": "EARLY_TREND",
            "name": "早期趨勢",
            "description": "屬短線 regime（short_term_regime），區間盤中出現偏多苗頭，適合觀察是否進一步轉強。",
        },
        {
            "code": "BULLISH_RECOVERY",
            "name": "偏多修復",
            "description": "收復後的修復狀態，重點是守住與延續，不是追價。",
        },
    ],
    "趨勢分級": [
        {
            "code": "TREND_UP",
            "name": "上升趨勢",
            "description": "中期結構偏多，但仍需避免在壓力前方追價。",
        },
        {
            "code": "RANGE_BOUND",
            "name": "區間整理",
            "description": "價格處於區間震盪，支撐與壓力都要等測試結果確認。",
        },
        {
            "code": "TREND_DOWN",
            "name": "下降趨勢",
            "description": "中期結構偏空，支撐需要更嚴格確認，反彈也可能只是修正。",
        },
    ],
    "多空傾向": [
        {
            "code": "BULLISH_BIAS",
            "name": "偏多觀察",
            "description": "整體條件偏多，但仍需看 entry gate，不代表立刻買進。",
        },
        {
            "code": "BULLISH_CONTINUATION",
            "name": "多頭延續",
            "description": "短線與結構同向偏多，若 entry gate 也放行才代表可進場。",
        },
        {
            "code": "REVERSAL_BIAS",
            "name": "反轉觀察",
            "description": "有反轉跡象但仍待確認，應偏觀察而不是直接判定翻多。",
        },
        {
            "code": "NEUTRAL_BIAS",
            "name": "中性觀察",
            "description": "多空沒有明確優勢，等待價格接近關鍵區或出現確認事件。",
        },
        {
            "code": "BEARISH_BIAS",
            "name": "偏空觀察",
            "description": "風險或結構偏空，支撐也要更嚴格確認，不能直接當買點。",
        },
    ],
    "市場行為": [
        {
            "code": "BUY",
            "name": "買進",
            "description": "市場條件完整，但仍需 final entry 與日 K gate 同步放行。",
        },
        {
            "code": "BUY_SMALL",
            "name": "小量試單",
            "description": "條件尚可但不完整，只適合小量或觀察性試探。",
        },
        {
            "code": "WATCH",
            "name": "觀察",
            "description": "尚未達到進場條件，重點是等待確認或更好的價位。",
        },
        {
            "code": "AVOID",
            "name": "避開",
            "description": "風險條件不合格，未持有者不應進場，持有者需看防守線。",
        },
        {
            "code": "HOLD",
            "name": "持有",
            "description": "持有者可續抱，但仍要搭配 position condition 看防守價。",
        },
        {
            "code": "CONDITIONAL_HOLD",
            "name": "條件式持有",
            "description": "屬 semantic action（semantic_pipeline.action_state）：可以持有但必須守住防守線或等待延續，條件失效就要重評估。",
        },
        {
            "code": "REDUCE_ON_BREAKDOWN",
            "name": "跌破減碼",
            "description": "出現跌破風險時先降低曝險，避免把支撐失效誤判成低接。",
        },
        {
            "code": "EXIT",
            "name": "退出",
            "description": "主要防守條件失效，持有者應優先處理風險。",
        },
    ],
    "進場權限": [
        {
            "code": "BLOCKED",
            "name": "禁止進場",
            "description": "硬性條件不通，例如跌破、RR 不足或模型不可用。",
        },
        {
            "code": "WAIT_CONFIRMATION",
            "name": "等待確認",
            "description": "目前不是買點，需要下一根 K 棒、量能或價格延續確認。",
        },
        {
            "code": "PROBE_ENTRY",
            "name": "觀察性試探",
            "description": "只允許很保守的試探，不代表正式加碼或完整進場。",
        },
        {
            "code": "PROBE_ALLOWED",
            "name": "允許觀察性試探",
            "description": "日 K gate 未阻擋，但仍要求後續確認，適合小心觀察。",
        },
        {
            "code": "SMALL_ENTRY",
            "name": "小量進場",
            "description": "條件普通但已確認，可小量參與，不適合重倉。",
        },
        {
            "code": "ACCUMULATE",
            "name": "分批累積",
            "description": "條件完整但仍建議分批，不等於一次全部買進。",
        },
        {
            "code": "ENTRY_READY",
            "name": "日 K 進場條件成立",
            "description": "entry 端條件就緒，仍要 final entry 檢查日 K gate。",
        },
        {
            "code": "ENTRY_ALLOWED",
            "name": "允許進場",
            "description": "市場、事件、行為與日 K gate 同步放行後才是明確進場語意。",
        },
        {
            "code": "BUY_READY",
            "name": "買進就緒",
            "description": "日 K 端最高等級確認，通常需要延續或突破後才會出現。",
        },
    ],
    "日 K Gate": [
        {
            "code": "WAIT_DAILY_CONFIRM",
            "name": "等待日 K 確認",
            "description": "尚缺收盤、延續或量能確認，不能把盤中碰到支撐當成進場。",
        },
        {
            "code": "INVALIDATED",
            "name": "設定失效",
            "description": "主要支撐或交易假設已被破壞，應重新計算區間與防守線。",
        },
        {
            "code": "CHASING_RISK",
            "name": "追價風險",
            "description": "價格離主要區間太遠，即使方向正確也可能沒有足夠風險報酬。",
        },
        {
            "code": "CLOSE_NEAR_HIGH",
            "name": "收近高點",
            "description": "收盤靠近日內高點，代表買盤收尾較強；這是收盤位置分級，與「向上延續」是不同判定，收近高點不等於延續確認，仍需看隔日延續。",
        },
        {
            "code": "CLOSE_NEAR_LOW",
            "name": "收近低點",
            "description": "收盤靠近日內低點，代表賣壓收尾較強；這是收盤位置分級，與「向下延續」是不同判定，收近低點不等於延續確認，支撐需要重新確認。",
        },
        {
            "code": "CLOSE_MID_RANGE",
            "name": "收在中段",
            "description": "收盤位置沒有明顯偏多或偏空，通常代表仍在觀察階段。",
        },
        {
            "code": "UPSIDE_FOLLOW_THROUGH",
            "name": "向上延續",
            "description": "價格相對前一日繼續轉強，是從修復走向延續的重要證據。",
        },
        {
            "code": "DOWNSIDE_FOLLOW_THROUGH",
            "name": "向下延續",
            "description": "價格相對前一日繼續轉弱，代表跌破或壓力訊號需要優先處理。",
        },
        {
            "code": "NO_FOLLOW_THROUGH",
            "name": "尚無延續",
            "description": "已出現候選事件但價格沒有繼續確認，應維持等待或試探層級。",
        },
        {
            "code": "MOMENTUM_CONFIRMED",
            "name": "動能確認",
            "description": "價格延續且波動放大，動能證據較完整，但仍需檢查上方阻力與 RR。",
        },
        {
            "code": "MOMENTUM_UNCONFIRMED",
            "name": "動能未確認",
            "description": "價格有延續但波動證據不足，不宜直接升級成強進場。",
        },
    ],
    "價格路徑": [
        {
            "code": "EVENT_RISK",
            "name": "事件風險",
            "description": "有效偏空事件仍在，應先處理風險，不適合把 RR 通過視為買點。",
        },
        {
            "code": "INVALIDATION_RISK",
            "name": "失效風險",
            "description": "關鍵支撐或收復條件失效，需要重評估原先區間。",
        },
        {
            "code": "RR_BLOCKED",
            "name": "RR 阻擋",
            "description": "風險報酬不符合最低門檻，即使方向偏多也不應進場。",
        },
        {
            "code": "WAIT_PRICE_FOLLOW_THROUGH",
            "name": "等待價格延續",
            "description": "已有收復或修復，但價格還沒繼續往上確認。",
        },
        {
            "code": "BLOCKING_ZONE_AHEAD",
            "name": "前方壓力",
            "description": "上方很快遇到壓力區，追價空間不足，需要等待突破或拉回。",
        },
        {
            "code": "DAILY_CANDIDATE_ONLY",
            "name": "僅日 K 候選",
            "description": "目前只有日 K 候選區，尚缺主要交易區支撐完整決策。",
        },
        {
            "code": "OPEN_PATH",
            "name": "路徑開放",
            "description": "前方沒有明確阻擋或硬性風險，但仍需看 entry 是否放行。",
        },
    ],
    "RR 與模型": [
        {
            "code": "RR_QUALIFIED",
            "name": "RR 合格",
            "description": "風險報酬通過最低門檻，但 RR 高不等於勝率高。",
        },
        {
            "code": "RR_INSUFFICIENT",
            "name": "RR 不足",
            "description": "預期報酬相對風險不足，應等待更好的價格或放棄。",
        },
        {
            "code": "RR_UNAVAILABLE",
            "name": "RR 無資料",
            "description": "缺少有效風險報酬估計，不能把它當成合格訊號。",
        },
        {
            "code": "MODEL_DEGRADED",
            "name": "模型降級",
            "description": "模型健康度偏弱，訊號要保守使用，進場層級需降級。",
        },
        {
            "code": "MODEL_UNRELIABLE",
            "name": "模型不可用",
            "description": "模型狀態不可靠，不能依機率模型進場。",
        },
        {
            "code": "HEALTHY",
            "name": "模型健康",
            "description": "模型資料與驗證狀態足夠，但仍只是輔助，不取代價格確認。",
        },
        {
            "code": "DEGRADED",
            "name": "模型降級",
            "description": "模型可參考但需要降低進場層級，避免過度相信機率輸出。",
        },
        {
            "code": "UNRELIABLE",
            "name": "模型不可靠",
            "description": "模型不應作為進場依據，決策要回到價格結構與風控。",
        },
    ],
}


def analysis_status_tips() -> list[str]:
    tips: list[str] = []
    for category, items in ANALYSIS_STATUS_TIPS.items():
        for item in items:
            tips.append(f"{category}｜{item['code']}：{item['name']}。{item['description']}")
    return tips


def _moving_average_tip(current_price: float, ma5: Optional[float]) -> str:
    if ma5 is None:
        return "判讀提醒｜均線：5日均線資料不足時，先以區間邊界與收盤確認判讀。"
    diff = (current_price - ma5) / ma5 if ma5 else 0.0
    if diff > 0.01:
        return f"判讀提醒｜均線：收盤站上5日均線（{fmt_price(ma5)}），短線動能偏穩，但仍需搭配量能確認。"
    if diff < -0.01:
        return f"判讀提醒｜均線：收盤跌破5日均線（{fmt_price(ma5)}），短線動能偏弱，靠近支撐也不代表可直接買進。"
    return f"判讀提醒｜均線：收盤接近5日均線（{fmt_price(ma5)}），方向仍在整理，應等待突破或跌破確認。"


def _chip_tip(chip_score: Optional[float]) -> str:
    if chip_score is None:
        return "判讀提醒｜籌碼：尚無籌碼分數時，籌碼項先以中性看待，不代表偏多或偏空。"
    if chip_score >= CHIP_SIGNAL_STRONG_THRESHOLD:
        return "判讀提醒｜籌碼：籌碼偏多可增加支撐參考性，但支撐仍不是買點，仍需價格與量能確認。"
    if chip_score >= CHIP_SIGNAL_WEAK_THRESHOLD:
        return "判讀提醒｜籌碼：籌碼略偏多只是輔助加分，不能取代區間突破、反彈或停損規劃。"
    if chip_score <= -CHIP_SIGNAL_STRONG_THRESHOLD:
        return "判讀提醒｜籌碼：籌碼偏空時，支撐被測試需要更嚴格確認，壓力也可能更容易形成壓制。"
    if chip_score <= -CHIP_SIGNAL_WEAK_THRESHOLD:
        return "判讀提醒｜籌碼：籌碼略偏空代表支撐需要更多確認，不等於所有壓力都適合放空。"
    return "判讀提醒｜籌碼：籌碼分數接近中性時，不應把它解讀成明確方向訊號。"


def _build_analysis_tips(
    period_summaries: list[dict], current_price: float, ma5: Optional[float], chip_score: Optional[float]
) -> list[str]:
    return [
        "指標小辭典｜RR：Risk/Reward，衡量預期報酬相對預期風險；RR 高不等於勝率高。",
        "指標小辭典｜EV：Expected Value，表示機率加權後的期望報酬，會同時考慮反彈與跌破情境。",
        "指標小辭典｜Confidence：信心分數反映樣本數、近期性與穩定度；低信心不是看空，而是需要更多確認。",
        "指標小辭典｜Trading Score：綜合 EV、RR、趨勢、量能、信心與籌碼後的交易參考分數，不是單一買賣訊號。",
        "指標小辭典｜Confluence：多方法共振，代表不同證據族群指向相近價位，但仍需後續 K 棒確認。",
        "價位語意｜Support：支撐是現價下方可能出現承接的區間，不是自動買點。",
        "價位語意｜Resistance：壓力是現價上方可能遇到賣壓的區間，不是自動放空點。",
        "價位語意｜AT_ZONE：價格正在區間內，應優先觀察收盤站回、跌破或量能變化。",
        "價位語意｜Primary Zone：主區是目前最具決策參考性的區間，但仍要搭配風險報酬與停損位置。",
        "事件語意｜Break：Break 指收盤有效穿越區間邊界；單根刺穿不等於有效突破或跌破。",
        "事件語意｜Bounce：Bounce 指價格測試支撐或壓力後出現反彈或壓回，需要收盤與量能確認。",
        "事件語意｜Reclaim：Reclaim 指跌破後重新站回關鍵區間，常用來觀察失而復得的結構修復。",
        "事件語意｜Invalidated：Invalidated 代表原區間參考性被破壞，應重新評估支撐壓力與停損。",
        "事件語意｜Pullback：Pullback 是趨勢中的回測，不等於反轉；要看是否守住關鍵區間。",
        *analysis_status_tips(),
        _moving_average_tip(current_price, ma5),
        _chip_tip(chip_score),
    ]
