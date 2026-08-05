<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import Layout from '../components/layout/Layout.svelte'
  import SRChipPanel from '../components/chip/SRChipPanel.svelte'
  import { ApiError } from '../lib/api/client'
  import {
    createSRZoneAnalysis,
    listSRZoneAnalyses,
    getSRZoneAnalysis,
    deleteSRZoneAnalysis,
    verifySRZoneAnalysis,
    triggerSRScoringTrain,
    getTrainJob,
    listTrainJobs,
    pruneTrainJobs,
    getModelStatus,
    runSREvaluation,
    getSREvaluationJob,
    listSREvaluationJobs,
    listSRRegressionResults,
    derivedReasonLabel,
    type SRZoneAnalysis,
    type SRZone,
    type SRZoneSummaryItem,
    type ZoneTier,
    type ConfidenceLevel,
    type TradingRecommendation,
    type SRScoringTrainJob,
    type TrainJobStatus,
    type ModelStatus,
    type SREvaluationReport,
    type SREvaluationJob,
    type SREvaluationJobStatus,
    type SRDecisionReplayGovernance,
    type SRReplayCoverage,
    type SRRegressionResult,
  } from '../lib/api/srZones'

  let symbol = ''
  let fetchLimit = 250
  let reuseExisting = false
  let submitting = false
  let submitError = ''

  let modelStatus: ModelStatus | null = null
  async function loadModelStatus() {
    try {
      modelStatus = await getModelStatus()
    } catch {
      modelStatus = null // Python service 沒開或連不上；分析按鈕仍可嘗試，錯誤訊息由分析本身處理
    }
  }

  let current: SRZoneAnalysis | null = null
  let currentZones: SRZone[] = []
  $: tierGroups = groupByTier(currentZones)
  $: periodSummaries = current?.period_summaries ?? []
  $: analysisTips = current?.analysis_tips ?? []
  $: chipSummary = current?.chip_summary ?? null


  $: decisionSummary = current?.decision_summary ?? null
  $: hasNormalizedDecision = current?.normalized_status?.decision === 'normalized'
  $: missingNormalizedDecision = !!current && !decisionSummary && current.normalized_status?.decision === 'missing'
  $: missingModelGovernance = !!current && !current.probability_context && current.normalized_status?.model_governance === 'missing'
  $: hasDecisionDetail = !!decisionSummary?.market_regime && !!decisionSummary?.confidence_explanation
  $: analysisExplanation = current?.explanation ?? null
  $: explanationSummary = analysisExplanation?.summary
    ?? analysisTips[0]
    ?? ''
  $: explanationActionReason = analysisExplanation?.action_reason
    ?? (decisionSummary?.primary_zone?.reason ?? '')
  $: explanationDrivers = analysisExplanation?.market_drivers ?? decisionSummary?.market_regime?.reasons ?? []
  $: explanationRisks = analysisExplanation?.risk_notes ?? decisionSummary?.risk_notes ?? []
  $: globalEvidence = current?.evidence ?? null
  let verifying = false
  let verifyError = ''

  let history: SRZoneAnalysis[] = []
  let historyLoading = true
  let confirmDeleteId: number | null = null
  let deletingId: number | null = null

  // ── 訓練/更新機率模型 ──────────────────────────────────────
  let trainSymbols = ''
  let trainLimit = 1500
  type TrainModelType = 'gradient_boosting' | 'hist_gradient_boosting' | 'lightgbm' | 'logistic_regression'
  type TrainSplitMethod = 'time' | 'random'
  type TrainCalibrationMethod = 'sigmoid' | 'isotonic' | 'none'
  let trainModelType: TrainModelType = 'gradient_boosting'
  let trainSplitMethod: TrainSplitMethod = 'time'
  let trainCalibrationMethod: TrainCalibrationMethod = 'sigmoid'
  let training = false
  let trainError = ''
  let trainMessage = ''
  let pruningTrainJobs = false
  let activeJob: SRScoringTrainJob | null = null
  let recentTrainJobs: SRScoringTrainJob[] = []
  type EvaluationMode = 'decision_replay' | 'evaluation'
  let evaluationSymbols = ''
  let evaluationLimit = 1500
  let evaluationReplayMaxRows = 100
  let evaluationMode: EvaluationMode = 'decision_replay'
  // 預設不寫入：寫進 stock_sr_regression_results 會成為該 model_config_hash 的 production
  // entry gate 依據（第一筆寫入就讓原本 no-op 的 gate 生效），不該是點兩下就發生的副作用。
  let evaluationWriteDB = false
  let evaluating = false
  let evaluationError = ''
  let evaluationMessage = ''
  let evaluationReport: SREvaluationReport | null = null
  let activeEvaluationJob: SREvaluationJob | null = null
  let recentEvaluationJobs: SREvaluationJob[] = []
  let regressionResults: SRRegressionResult[] = []
  let regressionSchemaVersion = 'sr_zone_decision_replay_p0'
  let pollTimer: ReturnType<typeof setInterval> | null = null
  let evaluationPollTimer: ReturnType<typeof setInterval> | null = null
  let tipTimer: ReturnType<typeof setInterval> | null = null
  let activeTipIndex = 0
  let showDetailedZones = false

  $: if (analysisTips.length > 0 && activeTipIndex >= analysisTips.length) activeTipIndex = 0

  const trainStatusLabel: Record<TrainJobStatus, string> = {
    pending: '排隊中', running: '訓練中', done: '完成', failed: '失敗',
  }
  const trainStatusClass: Record<TrainJobStatus, string> = {
    pending: 'bg-gray-700/60 text-gray-400',
    running: 'bg-blue-900/40 text-blue-400',
    done: 'bg-green-900/40 text-green-400',
    failed: 'bg-red-900/40 text-red-400',
  }
  const evaluationStatusLabel: Record<SREvaluationJobStatus, string> = {
    pending: '排隊中', running: '執行中', done: '完成', failed: '失敗',
  }
  const governanceStateClass: Record<string, string> = {
    HEALTHY: 'bg-green-900/40 text-green-400',
    DEGRADED: 'bg-yellow-900/40 text-yellow-400',
    UNRELIABLE: 'bg-red-900/40 text-red-400',
  }

  function governanceFlags(governance: SRDecisionReplayGovernance): string {
    return [...(governance.blocking_flags ?? []), ...(governance.warning_flags ?? [])].join(', ')
  }

  function coverageLabel(coverage: SRReplayCoverage): string {
    const covered = coverage.symbols_covered ?? 0
    const requested = coverage.symbols_requested ?? 0
    const ratio = coverage.coverage_ratio
    const pct = ratio === null || ratio === undefined ? '—' : `${Math.round(ratio * 100)}%`
    return `${covered}/${requested}（${pct}）`
  }

  const evaluationStatusClass: Record<SREvaluationJobStatus, string> = {
    pending: 'bg-gray-700/60 text-gray-400',
    running: 'bg-blue-900/40 text-blue-400',
    done: 'bg-green-900/40 text-green-400',
    failed: 'bg-red-900/40 text-red-400',
  }
  const modelTypeNotes: Record<TrainModelType, string> = {
    gradient_boosting: '預設建議；小到中型資料穩定，能處理非線性，訓練速度中等。',
    hist_gradient_boosting: '資料量較大時速度較好；小資料不一定比預設模型穩定。',
    lightgbm: '大量資料時通常表現強，但 Python 環境需安裝 lightgbm，否則訓練會失敗。',
    logistic_regression: '可解釋的基準模型；輸出保守，適合拿來檢查複雜模型是否過擬合。',
  }
  const splitMethodNotes: Record<TrainSplitMethod, string> = {
    time: '正式評估建議；每檔股票用較新的 touch 事件當 holdout，避免未來資料混入訓練。',
    random: '只建議做比較；金融時間序列容易高估表現，不適合作為正式模型評估。',
  }
  const calibrationNotes: Record<TrainCalibrationMethod, string> = {
    sigmoid: '預設建議；校準機率較穩，樣本不足時後端會自動降級為未校準。',
    isotonic: '資料足夠時較有彈性；小資料容易過擬合。',
    none: '不做校準；適合診斷 estimator 原始輸出，不建議直接當最終機率模型。',
  }

  onMount(() => {
    loadHistory()
    loadRecentTrainJobs()
    loadRecentEvaluationJobs()
    loadRegressionResults()
    loadModelStatus()
    tipTimer = setInterval(() => {
      if (analysisTips.length > 0) activeTipIndex = (activeTipIndex + 1) % analysisTips.length
    }, 4500)
  })
  onDestroy(() => {
    stopPolling()
    stopEvaluationPolling()
    if (tipTimer) clearInterval(tipTimer)
  })

  const roleClass: Record<string, string> = {
    SUPPORT: 'bg-green-900/40 text-rise',
    RESISTANCE: 'bg-red-900/40 text-fall',
    AT_ZONE: 'bg-gray-700/60 text-gray-300',
  }
  const methodLabel: Record<string, string> = {
    atr: 'ATR 通道', volume_profile: '成交量分布',
  }
  const statusLabel: Record<string, string> = {
    PENDING: '尚未驗證', HELD_SO_FAR: '目前守住', BROKEN: '已被突破',
  }
  const statusClass: Record<string, string> = {
    PENDING: 'bg-gray-700/60 text-gray-400',
    HELD_SO_FAR: 'bg-green-900/40 text-green-400',
    BROKEN: 'bg-red-900/40 text-red-400',
  }

  const netScoreLabelText: Record<string, string> = {
    STRONG_SUPPORT: '強力支撐', NEUTRAL: '勢均力敵', STRONG_RESISTANCE: '強力壓力',
  }
  const netScoreLabelClass: Record<string, string> = {
    STRONG_SUPPORT: 'bg-green-900/40 text-rise',
    NEUTRAL: 'bg-gray-700/60 text-gray-300',
    STRONG_RESISTANCE: 'bg-red-900/40 text-fall',
  }

  const confidenceLevelText: Record<string, string> = {
    LOW: '低', MEDIUM: '中', HIGH: '高', VERY_HIGH: '極高',
  }
  const confidenceLevelClass: Record<string, string> = {
    LOW: 'bg-gray-700/60 text-gray-400',
    MEDIUM: 'bg-yellow-900/40 text-yellow-400',
    HIGH: 'bg-green-900/40 text-green-400',
    VERY_HIGH: 'bg-green-900/60 text-green-300',
  }

  const recentValidationText: Record<string, string> = {
    VALIDATED_RECENTLY: '最近已驗證', PENDING_VALIDATION: '尚待驗證',
    NOT_TESTED_RECENTLY: '近期未測試', EXPIRED: '可能已失效',
  }
  const recentValidationClass: Record<string, string> = {
    VALIDATED_RECENTLY: 'bg-green-900/40 text-green-400',
    PENDING_VALIDATION: 'bg-gray-700/60 text-gray-400',
    NOT_TESTED_RECENTLY: 'bg-yellow-900/40 text-yellow-400',
    EXPIRED: 'bg-red-900/40 text-red-400',
  }

  const volumeConfirmationText: Record<string, string> = {
    CONFIRMED: '量能確認', WEAK: '量能不足', NEUTRAL: '量能普通', FAILED: '量能確認失敗',
  }
  const volumeConfirmationClass: Record<string, string> = {
    CONFIRMED: 'bg-green-900/40 text-green-400',
    WEAK: 'bg-yellow-900/40 text-yellow-400',
    NEUTRAL: 'bg-gray-700/60 text-gray-400',
    FAILED: 'bg-red-900/40 text-red-400',
  }

  const summarySides = ['support', 'resistance'] as const

  const zoneDirectionText: Record<string, string> = { UP: '↑ 上升', DOWN: '↓ 下降', FLAT: '→ 持平' }
  const zoneDirectionClass: Record<string, string> = {
    UP: 'text-rise', DOWN: 'text-fall', FLAT: 'text-muted',
  }

  const recommendationText: Record<string, string> = {
    STRONG_BUY: '強力買進', BUY: '買進', WATCH: '觀察', NEUTRAL: '中性', AVOID: '避開', STRONG_SELL: '強力放空',
  }
  const recommendationClass: Record<string, string> = {
    STRONG_BUY: 'bg-green-900/60 text-green-300',
    BUY: 'bg-green-900/40 text-green-400',
    WATCH: 'bg-blue-900/40 text-blue-400',
    NEUTRAL: 'bg-gray-700/60 text-gray-400',
    AVOID: 'bg-yellow-900/40 text-yellow-400',
    STRONG_SELL: 'bg-red-900/60 text-red-300',
  }

  // Tier（可排序）：主結構（寬）→ 交易區 → 短期（窄），見後端
  // scoring.py::_assign_tiers。
  const tierOrder: ZoneTier[] = ['TIER_1_MAIN_STRUCTURE', 'TIER_2_TRADING_ZONE', 'TIER_3_SHORT_TERM']
  const tierDescription: Record<ZoneTier, string> = {
    TIER_1_MAIN_STRUCTURE: '寬幅區間，反映長期／宏觀的關鍵價位',
    TIER_2_TRADING_ZONE: '中等寬度，適合作為進出場的操作區間',
    TIER_3_SHORT_TERM: '窄幅區間，貼近盤中操作的精確價位',
  }

  // trading_score 的六個可拆解分量，依權重大到小排列，方便閱讀「總分裡
  // 佔最大比重的是哪一項」（見 十三、Score 必須可拆解）。【2026-07 籌碼
  // 分析整合】新增 chip 分量後，其餘五個分量依原比例縮小。
  const scoreBreakdownFields: { key: keyof SRZone['trading_score_breakdown']; label: string; weight: number }[] = [
    { key: 'expected_value', label: 'EV', weight: 34 },
    { key: 'risk_reward', label: 'RR', weight: 17 },
    { key: 'chip', label: '籌碼', weight: 15 },
    { key: 'trend', label: 'Trend', weight: 12.75 },
    { key: 'volume', label: 'Volume', weight: 12.75 },
    { key: 'confidence', label: 'Zone Confidence', weight: 8.5 },
  ]

  interface TierGroup {
    tier: ZoneTier
    label: string
    zones: SRZone[]
  }

  // zones 必須「可排序」：依 tier 由粗到細分組（後端已經照這個順序排好，
  // 這裡再依 trading_score 保險排序一次，避免前端顯示順序跟資料來源脫鉤）。
  function groupByTier(zones: SRZone[]): TierGroup[] {
    return tierOrder
      .map((tier) => ({
        tier,
        label: zones.find((z) => z.tier === tier)?.tier_label ?? tier,
        zones: zones.filter((z) => z.tier === tier).sort((a, b) => b.trading_score - a.trading_score),
      }))
      .filter((g) => g.zones.length > 0)
  }

  // ── 新手優先的閱讀層級 ──────────────────────────────────────
  // 目標：不展開任何「進階」區塊，也能看懂「哪個區間最重要、該觀察支撐
  // 還是壓力、什麼條件代表判斷失效、可信度高不高」。所有細節（EV/RR/
  // net_score/confidence 原始數字/score breakdown/觸碰統計）都還在，只是
  // 收在「進階」裡，不刪除任何既有欄位或計算。

  let showAdvancedGlobal = false
  let expandedZones: Record<number, boolean> = {}
  function toggleZoneAdvanced(id: number) {
    expandedZones = { ...expandedZones, [id]: !expandedZones[id] }
  }

  // resolved_role 是驗證後解析出的方向（只有原本 role='AT_ZONE' 的 zone 才
  // 可能有值，見 lib/api/srZones.ts 說明）。判斷「這個 zone 現在算什麼角色」
  // 一律要用這個 helper，不要直接讀 z.role——否則 AT_ZONE 驗證後即使已經
  // 解析出方向，UI 仍會顯示「方向還不明確」，但 status 卻已經是
  // HELD_SO_FAR/BROKEN，兩者會互相矛盾（見 docs/sr-zone-scoring.md 十五）。
  function effectiveRole(z: SRZone): 'SUPPORT' | 'RESISTANCE' | 'AT_ZONE' {
    return z.resolved_role ?? z.role
  }

  function roleLabel(role: string): string {
    if (role === 'SUPPORT') return '支撐'
    if (role === 'RESISTANCE') return '壓力'
    if (role === 'AT_ZONE') return '區間內'
    return role
  }

  function zoneDisplayLabel(z: SRZone): string {
    return z.display_label ?? `${z.tier_label}${roleLabel(effectiveRole(z))}`
  }

  function summaryDisplayLabel(item: SRZoneSummaryItem | null, side: 'support' | 'resistance'): string {
    if (!item) return summaryTitle(side)
    return item.display_label ?? `${item.tier_label}${item.role_label ?? summaryTitle(side)}`
  }

  function confluenceFamilyCount(item: { confluence_count?: number; confluence_family_count?: number | null } | null | undefined): number {
    return item?.confluence_family_count ?? item?.confluence_count ?? 1
  }

  // role（現價位置下目前扮演的角色）跟 net_score_label（這個價位帶過去
  // 更像支撐還是壓力）是兩個不同概念，方向相反不代表演算法錯，但 UI 不
  // 解釋的話使用者會覺得同一張卡片自相矛盾（role=SUPPORT 卻顯示「強力
  // 壓力」）。AT_ZONE 沒有明確角色可比較，NEUTRAL net_score 也不構成
  // 「相反」，兩者都不算衝突（見 docs/sr-zone-scoring.md 中 role 與 net_score_label 語意差異一節）。
  function roleNetScoreConflicts(z: SRZone): boolean {
    const role = effectiveRole(z)
    if (role === 'SUPPORT' && z.net_score_label === 'STRONG_RESISTANCE') return true
    if (role === 'RESISTANCE' && z.net_score_label === 'STRONG_SUPPORT') return true
    return false
  }

  // 主要觀察區間：優先挑已經解析出方向（SUPPORT/RESISTANCE）裡 trading_score
  // 最高的一個；如果全部都還是 AT_ZONE（現價卡在每個區間內），退而求其次
  // 挑 AT_ZONE 裡分數最高的。
  $: mainZone = pickMainZone(currentZones)
  function pickMainZone(zones: SRZone[]): SRZone | null {
    if (zones.length === 0) return null
    const directional = zones.filter((z) => effectiveRole(z) !== 'AT_ZONE')
    const pool = directional.length > 0 ? directional : zones
    return pool.reduce((best, z) => (z.trading_score > best.trading_score ? z : best), pool[0])
  }


  const marketBiasText: Record<string, string> = {
    BULLISH_BIAS: '偏多觀察',
    NEUTRAL_BIAS: '中性觀察',
    BEARISH_BIAS: '偏空觀察',
    REVERSAL_BIAS: '反轉觀察',
    BULLISH_CONTINUATION: '多頭延續',
  }
  const marketBiasClass: Record<string, string> = {
    BULLISH_BIAS: 'bg-green-900/40 text-green-300 border-green-700/60',
    NEUTRAL_BIAS: 'bg-gray-700/60 text-gray-300 border-border',
    BEARISH_BIAS: 'bg-red-900/40 text-red-300 border-red-700/60',
    REVERSAL_BIAS: 'bg-indigo-900/40 text-indigo-300 border-indigo-700/60',
    BULLISH_CONTINUATION: 'bg-emerald-900/40 text-emerald-300 border-emerald-700/60',
  }
  // decision_derived_view 的 reason code → 人類可讀說明；查無對照時回退顯示原始 code。
  const marketActionText: Record<string, string> = {
    BUY: '偏多觀察', BUY_SMALL: '偏多觀察', WATCH: '中性觀察', AVOID: '偏空觀察',
    Buy: '買進', BuySmall: '小量試單', Hold: '觀察', Avoid: '避開',
  }
  const marketActionClass: Record<string, string> = {
    BUY: 'bg-green-900/50 text-green-300 border-green-700/60',
    BUY_SMALL: 'bg-emerald-900/40 text-emerald-300 border-emerald-700/60',
    WATCH: 'bg-yellow-900/40 text-yellow-300 border-yellow-700/60',
    AVOID: 'bg-red-900/40 text-red-300 border-red-700/60',
    Buy: 'bg-green-900/50 text-green-300 border-green-700/60',
    BuySmall: 'bg-emerald-900/40 text-emerald-300 border-emerald-700/60',
    Hold: 'bg-yellow-900/40 text-yellow-300 border-yellow-700/60',
    Avoid: 'bg-red-900/40 text-red-300 border-red-700/60',
  }
  const entryActionText: Record<string, string> = {
    WAIT_CONFIRMATION: '等待確認',
    PROBE_ENTRY: '觀察性試探',
    SMALL_ENTRY: '小量進場',
    ACCUMULATE: '分批累積',
    BUY: '買進',
  }
  const entryActionClass: Record<string, string> = {
    WAIT_CONFIRMATION: 'bg-gray-700/60 text-gray-300 border-border',
    PROBE_ENTRY: 'bg-yellow-900/40 text-yellow-300 border-yellow-700/60',
    SMALL_ENTRY: 'bg-emerald-900/40 text-emerald-300 border-emerald-700/60',
    ACCUMULATE: 'bg-green-900/40 text-green-300 border-green-700/60',
    BUY: 'bg-green-900/50 text-green-300 border-green-700/60',
  }
  // final_entry_permission 是 entry 與日 K 兩端的保守仲裁結果，前端顯示「是否允許進場」以此為準
  const finalEntryText: Record<string, string> = {
    BLOCKED: '禁止進場',
    WAIT_CONFIRMATION: '等待確認',
    PROBE_ENTRY: '觀察性試探',
    SMALL_ENTRY: '小量進場',
    ACCUMULATE: '分批累積',
    BUY: '買進',
  }
  const finalEntryClass: Record<string, string> = {
    BLOCKED: 'bg-red-900/40 text-red-300 border-red-700/60',
    WAIT_CONFIRMATION: 'bg-gray-700/60 text-gray-300 border-border',
    PROBE_ENTRY: 'bg-yellow-900/40 text-yellow-300 border-yellow-700/60',
    SMALL_ENTRY: 'bg-emerald-900/40 text-emerald-300 border-emerald-700/60',
    ACCUMULATE: 'bg-green-900/40 text-green-300 border-green-700/60',
    BUY: 'bg-green-900/50 text-green-300 border-green-700/60',
  }
  const positionActionText: Record<string, string> = {
    HOLD: '條件式持有', REDUCE_ON_BREAKDOWN: '跌破減碼', REDUCE: '減碼', EXIT: '出場',
  }
  const positionActionClass: Record<string, string> = {
    HOLD: 'bg-blue-900/40 text-blue-300 border-blue-700/60',
    REDUCE_ON_BREAKDOWN: 'bg-yellow-900/40 text-yellow-300 border-yellow-700/60',
    REDUCE: 'bg-orange-900/40 text-orange-300 border-orange-700/60',
    EXIT: 'bg-red-900/50 text-red-300 border-red-700/60',
  }
  const regimePrimaryText: Record<string, string> = {
    TREND_UP: '偏多趨勢', TREND_DOWN: '偏空趨勢', RANGE_BOUND: '區間盤',
  }
  const structureStateText: Record<string, string> = {
    NORMAL: '結構正常',
    RECOVERY_CANDIDATE: '短線測試支撐',
    RECOVERY: '短線收回支撐',
    RECOVERY_INVALIDATED: '短線結構轉弱',
    SUPPORT_RECLAIM_CANDIDATE: '支撐收復候選',
    SUPPORT_RECLAIM_CONFIRMED: '支撐收復確認',
    SUPPORT_RECLAIM_INVALIDATED: '支撐收復失效',
    BREAKDOWN: '短線結構跌破',
  }
  const regimeFlagText: Record<string, string> = {
    HIGH_VOLATILITY: '高波動', LOW_CONFIDENCE: '低信心',
  }
  const shortTermRegimeText: Record<string, string> = {
    NORMAL: '短線正常',
    BREAKDOWN_RISK: '跌破風險',
    RECLAIM_ATTEMPT: '收復嘗試',
    REVERSAL_CANDIDATE: '反轉候選',
    RECOVERY: '短線收復',
    EARLY_TREND: '初升段',
  }
  const noviceRoleText: Record<string, string> = {
    SUPPORT: '比較接近支撐', RESISTANCE: '比較接近壓力', AT_ZONE: '現價卡在區間內，方向還不明確',
  }
  const dailyActionText: Record<string, string> = {
    CLOSE_NEAR_HIGH: '收近高點',
    CLOSE_MID_RANGE: '收在中段',
    CLOSE_NEAR_LOW: '收近低點',
    RANGE_EXPANSION: '波動擴大',
    NORMAL_RANGE: '一般波動',
    NARROW_RANGE: '窄幅整理',
    GAP_UP_APPROX: '近似跳空上行',
    GAP_DOWN_APPROX: '近似跳空下行',
    NO_GAP: '無明顯跳空',
    UPSIDE_FOLLOW_THROUGH: '向上延續',
    DOWNSIDE_FOLLOW_THROUGH: '向下延續',
    NO_FOLLOW_THROUGH: '未延續',
    PREVIOUS_CLOSE_RECLAIM: '收復昨收',
    PREVIOUS_CLOSE_REJECTION: '跌回昨收下方',
    PRICE_UPSIDE_FOLLOW_THROUGH: '價格向上延續',
    PRICE_DOWNSIDE_FOLLOW_THROUGH: '價格向下延續',
    NO_PRICE_FOLLOW_THROUGH: '價格未延續',
    MOMENTUM_CONFIRMED: '動能確認',
    MOMENTUM_UNCONFIRMED: '動能未確認',
    NO_MOMENTUM_CONFIRMATION: '無動能確認',
    NONE: '無',
    UNKNOWN: '無資料',
  }
  const pricePathText: Record<string, string> = {
    EVENT_RISK: '事件風險',
    INVALIDATION_RISK: '失效風險',
    RR_BLOCKED: 'RR 阻擋',
    WAIT_PRICE_FOLLOW_THROUGH: '等待價格延續',
    BLOCKING_ZONE_AHEAD: '前方壓力',
    DAILY_CANDIDATE_ONLY: '僅日 K 候選',
    OPEN_PATH: '路徑開放',
  }

  function positionReasonCodesText(codes?: string[]): string {
    return codes && codes.length > 0 ? codes.map(derivedReasonLabel).join(' / ') : '—'
  }

  // 白話交易建議：保持「輔助判斷」語氣，不寫成保證獲利或自動交易指令，
  // 跟 recommendationText（進階區用的英文術語中文對照）分開。
  const noviceRecommendationText: Record<TradingRecommendation, string> = {
    STRONG_BUY: '訊號偏強，可留意是否持續守住（僅供參考，不是買進指令）',
    BUY: '訊號尚可，可以持續觀察',
    WATCH: '訊號還不明確，建議先觀察',
    NEUTRAL: '目前沒有明顯訊號',
    AVOID: '訊號偏強，追高風險較高，建議觀望',
    STRONG_SELL: '訊號很強，追高風險高，建議觀望',
  }

  // 三級（低/中/高），VERY_HIGH 併入「高」——新手不需要分四級，完整四級
  // 徽章留在進階區（confidenceLevelText/confidenceLevelClass）。
  const noviceConfidenceText: Record<ConfidenceLevel, string> = {
    LOW: '低', MEDIUM: '中', HIGH: '高', VERY_HIGH: '高',
  }
  const noviceConfidenceClass: Record<ConfidenceLevel, string> = {
    LOW: 'text-fall', MEDIUM: 'text-yellow-400', HIGH: 'text-rise', VERY_HIGH: 'text-rise',
  }

  function watchRangeText(z: SRZone): string {
    return `可以觀察價格是否回到 ${fmt(z.price_low)} ~ ${fmt(z.price_high)}`
  }

  // 已經 BROKEN 就顯示實際結果，而不是假設性的「若跌破/突破」條件——
  // 判斷已經失效，不需要再用未來式提醒使用者。
  function invalidationText(z: SRZone): string {
    const role = effectiveRole(z)
    if (z.status === 'BROKEN') {
      return `已於 ${formatDateTime(z.broken_at)}（@ ${fmt(z.broken_price)}）${role === 'RESISTANCE' ? '突破' : '跌破'}，這個判斷已經失效`
    }
    if (role === 'SUPPORT') return `若跌破 ${fmt(z.price_low)}，這個判斷就失效`
    if (role === 'RESISTANCE') return `若突破 ${fmt(z.price_high)}，這個判斷就失效`
    return '現價還在區間內，方向未定，暫不適用'
  }

  const probabilityOutcomeText: Record<string, string> = {
    BOUNCE: '反彈/守住',
    BREAK: '跌破/突破',
    NEUTRAL: '盤整/不明確',
    NO_DIRECTION: '方向未定',
  }

  const probabilityFlagText: Record<string, string> = {
    HOLD_NOT_CALIBRATED: 'hold 未校準',
    BREAK_NOT_CALIBRATED: 'break 未校準',
    HOLD_LOW_TEST_ROWS: 'hold 測試樣本少',
    BREAK_LOW_TEST_ROWS: 'break 測試樣本少',
    NO_DIRECTION: '方向未定',
    LOW_CONFIDENCE: '信心偏低',
    LOW_PROBABILITY_EDGE: '機率差距小',
    MISSING_DIRECTIONAL_PROBABILITY: '缺方向機率',
    LOW_AVERAGE_EDGE: '平均機率差偏低',
    NO_DIRECTIONAL_ZONES: '無可用方向機率',
  }

  function probabilityFlagLabel(flag: string): string {
    return probabilityFlagText[flag] ?? flag
  }

  function summaryPriceText(item: SRZoneSummaryItem | null): string {
    return item ? `${fmt(item.price_low)} ~ ${fmt(item.price_high)}` : '暫無合理價位'
  }

  function summaryTitle(side: 'support' | 'resistance'): string {
    return side === 'support' ? '支撐' : '壓力'
  }

  function summaryAccent(side: 'support' | 'resistance'): string {
    return side === 'support' ? 'text-rise' : 'text-fall'
  }

  function summaryNote(item: SRZoneSummaryItem | null, fallback?: string): string {
    if (!item) return fallback ?? '目前沒有符合條件的價位，先看完整明細確認。'
    return item.reasons.slice(0, 3).join('、')
  }

  const chipDirectionText: Record<string, string> = {
    bullish: '籌碼偏多', bearish: '籌碼偏空', neutral: '籌碼中性', none: '籌碼無資料',
  }

  // 角色化：籌碼對「這個角色」是加分還是扣分，用直接加權貢獻是否高於中性
  // （15 分的一半 = 7.5）判斷；支撐與壓力的加/扣分方向後端已翻號。
  function chipRoleEffect(item: SRZoneSummaryItem): 'plus' | 'minus' | 'flat' {
    const c = item.chip?.contribution
    if (c === null || c === undefined) return 'flat'
    if (c > 7.9) return 'plus'
    if (c < 7.1) return 'minus'
    return 'flat'
  }

  function chipEffectText(item: SRZoneSummaryItem): string {
    const effect = chipRoleEffect(item)
    const roleWord = item.side === 'support' ? '支撐' : '壓力'
    if (effect === 'plus') return `對此${roleWord}加分`
    if (effect === 'minus') return `對此${roleWord}扣分`
    return '影響中性'
  }

  function chipEffectClass(item: SRZoneSummaryItem): string {
    const effect = chipRoleEffect(item)
    if (effect === 'plus') return 'text-rise'
    if (effect === 'minus') return 'text-fall'
    return 'text-muted'
  }

  // 籌碼對機率的邊際貢獻（百分點），依角色顯示不同事件的機率：支撐卡看
  // bounce_delta（hold＝反彈守住），壓力卡看 break_delta（突破壓力）。兩者是
  // 不同事件，不能共用「反彈機率」文案，否則會把壓力區的籌碼影響誤讀成看多。
  function chipDeltaText(item: SRZoneSummaryItem): string | null {
    const chip = item.chip
    if (!chip) return null
    const d = item.side === 'support' ? chip.bounce_delta_pp : chip.break_delta_pp
    if (d === null || d === undefined) return null
    const sign = d > 0 ? '+' : ''
    const label = item.side === 'support' ? '反彈機率' : '突破壓力機率'
    return `${label} ${sign}${d.toFixed(1)}pp`
  }

  async function submit() {
    if (!symbol.trim()) {
      submitError = '請輸入股票代號'
      return
    }
    if (fetchLimit < 35) {
      submitError = '抓取根數至少要 35 根，分析才有足夠資料可用'
      return
    }
    submitting = true
    submitError = ''
    try {
      const { analysis, zones } = await createSRZoneAnalysis(symbol.trim(), '1d', fetchLimit, reuseExisting)
      current = analysis
      currentZones = zones
      await loadHistory()
    } catch (err) {
      // 後端已經依實際狀況（404 沒有歷史資料/503 模型未訓練/502 Python
      // service 沒開/400 輸入錯誤）組好對應訊息，這裡直接顯示，不用自己
      // 再猜一句「大概是這幾種情況之一」的通用文字。
      submitError = err instanceof ApiError ? err.message : '分析失敗，請確認後端服務是否正常'
    } finally {
      submitting = false
    }
  }

  // 手動重新驗證：比對這筆分析之後的實際 K 棒，更新每個 zone 的
  // status（PENDING/HELD_SO_FAR/BROKEN）。可重複呼叫，不是一次性判定。
  async function runVerify() {
    if (!current) return
    verifying = true
    verifyError = ''
    try {
      const { analysis, zones } = await verifySRZoneAnalysis(current.id)
      current = analysis
      currentZones = zones
    } catch (err) {
      verifyError = err instanceof ApiError ? err.message : '驗證失敗，請確認後端服務是否正常'
    } finally {
      verifying = false
    }
  }

  async function runTrain() {
    training = true
    trainError = ''
    trainMessage = ''
    activeJob = null
    try {
      const symbols = trainSymbols
        .split(',')
        .map((s) => s.trim())
        .filter((s) => s.length > 0)
      const res = await triggerSRScoringTrain({
        symbols: symbols.length > 0 ? symbols : undefined,
        limit: trainLimit,
        modelType: trainModelType,
        splitMethod: trainSplitMethod,
        calibrationMethod: trainCalibrationMethod,
      })
      pollTrainJob(res.job_id)
    } catch (err) {
      trainError = err instanceof ApiError ? err.message : '觸發失敗，請確認 Python service 是否已啟動'
      training = false
    }
  }

  // 每 3 秒查一次任務狀態，直到 done/failed 才停止——訓練可能耗時數十秒到
  // 數分鐘，讓使用者不用一直盯著頁面，也不用只靠伺服器 log 才知道結果。
  function pollTrainJob(jobId: string) {
    stopPolling()
    pollTimer = setInterval(async () => {
      try {
        const job = await getTrainJob(jobId)
        activeJob = job
        if (job.status === 'done' || job.status === 'failed') {
          stopPolling()
          loadRecentTrainJobs()
          if (job.status === 'done') loadModelStatus()
        }
      } catch {
        trainError = '查詢訓練狀態失敗'
        stopPolling()
      }
    }, 3000)
  }

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
    training = false
  }

  async function loadRecentTrainJobs() {
    try {
      recentTrainJobs = await listTrainJobs(5)
    } catch {
      // 沉默失敗，不影響主要分析功能的呈現
    }
  }

  async function pruneOldTrainJobs() {
    pruningTrainJobs = true
    trainError = ''
    trainMessage = ''
    try {
      const result = await pruneTrainJobs(20)
      trainMessage = `已清理 ${result.deleted} 筆舊訓練紀錄，保留最近 ${result.keep} 筆完成/失敗紀錄`
      await loadRecentTrainJobs()
    } catch (err) {
      trainError = err instanceof ApiError ? err.message : '清理訓練紀錄失敗'
    } finally {
      pruningTrainJobs = false
    }
  }

  function parseSymbolList(input: string): string[] {
    return input
      .split(',')
      .map((s) => s.trim())
      .filter((s) => s.length > 0)
  }

  async function runEvaluation() {
    const symbols = parseSymbolList(evaluationSymbols || symbol)
    if (symbols.length === 0) {
      evaluationError = '請輸入 evaluation 股票代號'
      return
    }
    if (evaluationLimit < 80) {
      evaluationError = 'evaluation 抓取根數至少要 80 根'
      return
    }
    if (evaluationMode === 'decision_replay' && evaluationReplayMaxRows <= 0) {
      evaluationError = 'replay rows 必須大於 0'
      return
    }

    evaluating = true
    evaluationError = ''
    evaluationMessage = ''
    evaluationReport = null
    activeEvaluationJob = null
    try {
      const res = await runSREvaluation({
        symbols,
        timeframe: '1d',
        limit: evaluationLimit,
        decisionReplay: evaluationMode === 'decision_replay',
        replayMaxRows: evaluationReplayMaxRows,
        writeDb: evaluationWriteDB,
      })
      evaluationMessage = res.message
      activeEvaluationJob = {
        id: 0,
        job_id: res.job_id,
        status: res.status,
        symbols: JSON.stringify(symbols),
        timeframe: '1d',
        fetch_limit: evaluationLimit,
        mode: evaluationMode,
        write_db: evaluationWriteDB,
        replay_max_rows: evaluationMode === 'decision_replay' ? evaluationReplayMaxRows : 0,
        run_id: null,
        schema_version: null,
        pipeline_version: null,
        rows: null,
        sources: null,
        report: null,
        error: null,
        started_at: null,
        finished_at: null,
        created_at: new Date().toISOString(),
      }
      pollEvaluationJob(res.job_id)
    } catch (err) {
      evaluationError = err instanceof ApiError ? err.message : 'evaluation 失敗，請確認 Python service 是否已啟動'
      evaluating = false
    }
  }

  function pollEvaluationJob(jobId: string) {
    clearEvaluationPollTimer()
    evaluationPollTimer = setInterval(async () => {
      try {
        const job = await getSREvaluationJob(jobId)
        activeEvaluationJob = job
        if (job.status === 'done' || job.status === 'failed') {
          stopEvaluationPolling()
          loadRecentEvaluationJobs()
          if (job.status === 'done') {
            evaluationReport = job.report
            evaluationMessage = job.run_id ? `已完成 ${job.run_id}` : '已完成 evaluation'
            if (job.write_db) {
              regressionSchemaVersion = job.schema_version ?? regressionSchemaVersion
              loadRegressionResults()
            }
          } else {
            evaluationError = job.error ?? 'evaluation job failed'
          }
        }
      } catch {
        evaluationError = '查詢 evaluation 狀態失敗'
        stopEvaluationPolling()
      }
    }, 3000)
  }

  function clearEvaluationPollTimer() {
    if (evaluationPollTimer) {
      clearInterval(evaluationPollTimer)
      evaluationPollTimer = null
    }
  }

  function stopEvaluationPolling() {
    clearEvaluationPollTimer()
    evaluating = false
  }

  async function loadRecentEvaluationJobs() {
    try {
      recentEvaluationJobs = await listSREvaluationJobs(5)
    } catch {
      // 沉默失敗，不影響分析與訓練功能
    } finally {
    }
  }

  async function loadRegressionResults() {
    try {
      regressionResults = await listSRRegressionResults(regressionSchemaVersion || undefined, 8)
    } catch {
      // 沉默失敗，不影響分析與訓練功能
    }
  }

  function evaluationSchemaLabel(schema?: string | null): string {
    if (schema === 'sr_zone_decision_replay_p0') return 'Decision Replay'
    if (schema === 'sr_zone_evaluation_p0') return 'Evaluation'
    if (schema === 'decision_replay') return 'Decision Replay'
    if (schema === 'evaluation') return 'Evaluation'
    return schema || '—'
  }

  function regressionRows(result: SRRegressionResult): number | null {
    const value = result.metrics_json?.rows
    return typeof value === 'number' ? value : null
  }

  function regressionSymbols(result: SRRegressionResult): string {
    const symbols = result.metrics_json?.symbols
    return Array.isArray(symbols) && symbols.length > 0 ? symbols.join(', ') : '—'
  }

  function regressionWarnings(result: SRRegressionResult): string {
    const warnings = result.metrics_json?.warnings
    return Array.isArray(warnings) && warnings.length > 0 ? warnings.join('；') : '—'
  }

  function metricValue(job: SRScoringTrainJob | null, model: 'hold' | 'break', field: string): string {
    const v = job?.metrics?.[model]?.[field]
    return v === undefined || v === null ? '—' : v.toFixed(3)
  }

  function modelStatusMetric(model: 'hold' | 'break', field: string): string {
    const v = modelStatus?.metrics?.[model]?.[field]
    return v === undefined || v === null ? '—' : v.toFixed(3)
  }

  // calibrated 是 1/0（見 model.py::_fit_with_optional_calibration），資料
  // 太少時會自動降級為不校準，這裡讓使用者知道「這次的機率有沒有真的校準過」，
  // 而不是只看到一個看似可信的 AUC 數字。
  function calibratedLabel(job: SRScoringTrainJob | null, model: 'hold' | 'break'): string {
    const v = job?.metrics?.[model]?.calibrated
    if (v === undefined || v === null) return '—'
    return v === 1 ? '已校準' : '未校準（樣本不足）'
  }

  function splitMethodLabel(job: SRScoringTrainJob | null): string {
    if (!job?.split_method) return '—'
    return job.split_method === 'time' ? '時間序列切分' : '隨機切分'
  }

  function symbolCount(job: SRScoringTrainJob | null): number {
    return job?.dataset_summary ? Object.keys(job.dataset_summary.rows_by_symbol).length : 0
  }

  // symbol 留空時列出「所有股票」最近的分析紀錄，方便一進頁面就有內容可看；
  // symbol 有值時才篩選成該股票的歷史紀錄
  async function loadHistory() {
    historyLoading = true
    try {
      history = await listSRZoneAnalyses(symbol.trim() || undefined, 20)
    } catch {
      // 沉默失敗，不影響主要分析結果的呈現
    } finally {
      historyLoading = false
    }
  }

  async function selectHistory(h: SRZoneAnalysis) {
    try {
      const { analysis, zones } = await getSRZoneAnalysis(h.id)
      current = analysis
      currentZones = zones
      if (symbol.trim() !== h.symbol) {
        symbol = h.symbol
        await loadHistory()
      }
    } catch {
      // ignore
    }
  }

  async function doDelete(id: number) {
    deletingId = id
    try {
      await deleteSRZoneAnalysis(id)
      history = history.filter((h) => h.id !== id)
      if (current?.id === id) {
        current = null
        currentZones = []
      }
    } catch {
      // ignore，列表維持原狀讓使用者可以重試
    } finally {
      deletingId = null
      confirmDeleteId = null
    }
  }

  function formatDateTime(ts?: string | null): string {
    if (!ts) return '—'
    return new Date(ts).toLocaleString('zh-TW', { hour12: false })
  }

  function fmt(v?: number | null): string {
    return v === undefined || v === null ? '—' : v.toFixed(2)
  }

  function fmtPct(v?: number | null): string {
    return v === undefined || v === null ? '—' : `${(v * 100).toFixed(1)}%`
  }

  function fmtSignedPct(v?: number | null): string {
    if (v === undefined || v === null) return '—'
    const pct = (v * 100).toFixed(2)
    return v > 0 ? `+${pct}%` : `${pct}%`
  }

  function fmtRatio(v?: number | null): string {
    return v === undefined || v === null ? '—' : `${v.toFixed(2)}R`
  }

  function fmtScore100(v?: number | null): string {
    return v === undefined || v === null ? '—' : v.toFixed(0)
  }

  function signedClass(v?: number | null): string {
    if (v === undefined || v === null) return 'text-muted'
    if (v > 0) return 'text-rise'
    if (v < 0) return 'text-fall'
    return 'text-muted'
  }

  const featureLabel: Record<string, string> = {
    touch_count: '觸碰次數',
    rejection_count: '守住次數',
    breakout_count: '突破次數',
    average_bounce_return: '平均反彈',
    average_break_return: '平均跌破',
    relative_volume: '相對量能',
    volatility: '波動',
    trend_strength: '趨勢',
    is_support: '支撐角色',
    chip_total_score: '籌碼總分',
    chip_institutional_score: '法人',
    chip_margin_score: '融資',
    chip_broker_score: '券商',
    chip_concentration_score: '集中度',
    chip_missing: '籌碼缺漏',
  }

  function activeEvidence(z: SRZone) {
    if (!z.evidence) return null
    return effectiveRole(z) === 'RESISTANCE' ? z.evidence.resistance : z.evidence.support
  }
</script>

<Layout>
  <div class="max-w-5xl mx-auto space-y-4">
    <h1 class="text-white font-semibold">支撐/壓力機率分析</h1>

    <!-- ── 模型狀態：分析前先知道模型準備好了沒，不用等失敗才知道 ──── -->
    {#if modelStatus}
      <div class="flex flex-wrap items-center gap-2 text-xs px-3 py-2 rounded-lg border
                  {modelStatus.exists ? 'bg-green-900/10 border-green-900/40' : 'bg-yellow-900/10 border-yellow-900/40'}">
        {#if modelStatus.exists}
          <span class="inline-flex items-center px-2 py-0.5 rounded-full font-medium bg-green-900/40 text-green-400">模型可用</span>
          <span class="text-white">{modelStatus.version}</span>
          <span class="text-muted">訓練於 {formatDateTime(modelStatus.trained_at)}</span>
          <span class="text-muted">hold AUC {modelStatusMetric('hold', 'auc')} / break AUC {modelStatusMetric('break', 'auc')}</span>
          {#if modelStatus.config_hash}
            <span class="text-muted font-mono" title="訓練設定快照的短 hash，重訓改參數後會不一樣">設定 {modelStatus.config_hash}</span>
          {/if}
        {:else}
          <span class="inline-flex items-center px-2 py-0.5 rounded-full font-medium bg-yellow-900/40 text-yellow-400">模型尚未訓練</span>
          <span class="text-muted">請先在下方「訓練/更新機率模型」區塊訓練，才能開始分析</span>
        {/if}
      </div>
    {/if}

    <!-- ── 輸入表單 ──────────────────────────────────────────── -->
    <div class="bg-panel border border-border rounded-xl px-5 py-4">
      {#if submitError}
        <p class="text-rise text-sm mb-3">{submitError}</p>
      {/if}
      <div class="flex gap-3">
        <input
          bind:value={symbol}
          placeholder="輸入股票代號，例如 2330"
          on:keydown={(e) => e.key === 'Enter' && submit()}
          class="flex-1 bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 placeholder:text-muted focus:outline-none focus:border-indigo-500 transition-colors"
        />
        <input
          type="number"
          min="35"
          step="10"
          bind:value={fetchLimit}
          title="抓取的歷史K棒根數"
          on:keydown={(e) => e.key === 'Enter' && submit()}
          class="w-28 bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 focus:outline-none focus:border-indigo-500 transition-colors"
        />
        <button
          class="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm
                 font-medium px-5 py-2 rounded-lg transition-colors"
          disabled={submitting}
          on:click={submit}
        >
          {submitting ? '分析中...' : '分析'}
        </button>
      </div>
      <label class="mt-3 inline-flex items-center gap-2 text-xs text-muted">
        <input
          type="checkbox"
          bind:checked={reuseExisting}
          class="rounded border-border bg-surface text-indigo-600 focus:ring-indigo-500"
        />
        <span>重用 24 小時內同週期分析；未勾選時會建立新的分析快照。</span>
      </label>
      <p class="text-muted text-xs mt-2">
        用 ATR 通道與成交量分布兩種方法建立價格區間（zone），依歷史觸碰事件訓練的機率模型算出反彈/跌破機率，
        支撐/壓力強度分數由該機率依區間辨識信心收縮而來（觸碰次數越少越保守），兩者不會互相矛盾。抓取根數指分析用的
        歷史K棒數量（預設 250，至少 35 根）。需要先在下方訓練過機率模型才能分析；若勾選重用且找到近期快照，
        本次不會重新呼叫模型。
      </p>
    </div>

    <!-- ── 訓練/更新機率模型 ────────────────────────────────── -->
    <div class="bg-panel border border-border rounded-xl px-5 py-4">
      <h2 class="text-sm font-semibold text-white mb-1">訓練/更新機率模型</h2>
      <p class="text-muted text-xs mb-3">
        用歷史觸碰事件重新訓練 bounce_probability / break_probability 模型，在背景執行（視股票數與資料長度可能耗時數十秒到數分鐘）。
        目前系統只維持一個現行模型；每次訓練成功都會覆蓋現行模型，下面的訓練紀錄是 job history，不是可切換的模型清單。
      </p>
      {#if trainError}
        <p class="text-rise text-sm mb-3">{trainError}</p>
      {/if}
      {#if trainMessage}
        <p class="text-green-400 text-sm mb-3">{trainMessage}</p>
      {/if}
      <div class="flex flex-wrap gap-3 items-center">
        <input
          bind:value={trainSymbols}
          placeholder="股票代號，逗號分隔（留空 = 整個監控清單）"
          class="flex-1 min-w-[220px] bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 placeholder:text-muted focus:outline-none focus:border-indigo-500 transition-colors"
        />
        <select
          bind:value={trainModelType}
          class="bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 focus:outline-none focus:border-indigo-500 transition-colors"
        >
          <option value="gradient_boosting">Gradient Boosting</option>
          <option value="hist_gradient_boosting">Hist Gradient Boosting</option>
          <option value="lightgbm">LightGBM</option>
          <option value="logistic_regression">Logistic Regression</option>
        </select>
        <select
          bind:value={trainSplitMethod}
          title="訓練/驗證資料切分方式"
          class="bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 focus:outline-none focus:border-indigo-500 transition-colors"
        >
          <option value="time">時間序列切分</option>
          <option value="random">隨機切分</option>
        </select>
        <select
          bind:value={trainCalibrationMethod}
          title="機率校準方式"
          class="bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 focus:outline-none focus:border-indigo-500 transition-colors"
        >
          <option value="sigmoid">Sigmoid 校準</option>
          <option value="isotonic">Isotonic 校準</option>
          <option value="none">不校準</option>
        </select>
        <input
          type="number"
          min="35"
          step="100"
          bind:value={trainLimit}
          title="訓練用的歷史K棒根數（每檔股票）"
          class="w-32 bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 focus:outline-none focus:border-indigo-500 transition-colors"
        />
        <button
          class="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm
                 font-medium px-5 py-2 rounded-lg transition-colors"
          disabled={training}
          on:click={runTrain}
        >
          {training ? '訓練中...' : '開始訓練'}
        </button>
      </div>
      <div class="mt-3 grid gap-2 md:grid-cols-3 text-xs">
        <div class="bg-surface/50 border border-border rounded-lg p-3">
          <p class="text-white font-medium mb-1">模型方式</p>
          <p class="text-muted">{modelTypeNotes[trainModelType]}</p>
        </div>
        <div class="bg-surface/50 border border-border rounded-lg p-3">
          <p class="text-white font-medium mb-1">切分方式</p>
          <p class="text-muted">{splitMethodNotes[trainSplitMethod]}</p>
        </div>
        <div class="bg-surface/50 border border-border rounded-lg p-3">
          <p class="text-white font-medium mb-1">機率校準</p>
          <p class="text-muted">{calibrationNotes[trainCalibrationMethod]}</p>
        </div>
      </div>

      <!-- 目前這次觸發的任務狀態：每 3 秒輪詢一次，直到 done/failed -->
      {#if activeJob}
        <div class="mt-3 px-3 py-2 bg-surface/60 rounded-lg text-xs flex flex-wrap items-center gap-2">
          <span class="inline-flex items-center px-2 py-0.5 rounded-full font-medium {trainStatusClass[activeJob.status]}">
            {trainStatusLabel[activeJob.status]}
          </span>
          <span class="text-muted font-mono">{activeJob.job_id}</span>
          {#if activeJob.status === 'done'}
            <span class="text-white">rows={activeJob.rows} sources={activeJob.sources} model={activeJob.model_version}</span>
            <span class="text-muted">{splitMethodLabel(activeJob)} · 來自 {symbolCount(activeJob)} 檔股票</span>
            <span class="text-muted">hold AUC {metricValue(activeJob, 'hold', 'auc')}（{calibratedLabel(activeJob, 'hold')}） / break AUC {metricValue(activeJob, 'break', 'auc')}（{calibratedLabel(activeJob, 'break')}）</span>
            <span class="text-muted">brier: hold {metricValue(activeJob, 'hold', 'brier_score')} / break {metricValue(activeJob, 'break', 'brier_score')}</span>
          {:else if activeJob.status === 'failed'}
            <span class="text-rise">{activeJob.error}</span>
          {/if}
        </div>
      {/if}

      <!-- 最近訓練紀錄：不用只靠伺服器 log 才知道之前訓練成功了沒 -->
      {#if recentTrainJobs.length > 0}
        <div class="mt-4">
          <div class="flex items-center justify-between gap-3 mb-1.5">
            <p class="text-muted text-xs">最近訓練紀錄</p>
            <button
              class="text-xs text-muted hover:text-white px-2 py-1 border border-border rounded transition-colors disabled:opacity-50"
              disabled={pruningTrainJobs || training}
              on:click={pruneOldTrainJobs}
            >
              {pruningTrainJobs ? '清理中...' : '清理舊紀錄'}
            </button>
          </div>
          <table class="w-full text-xs">
            <thead>
              <tr class="text-muted border-b border-border/60">
                <th class="text-left py-1">狀態</th>
                <th class="text-left py-1">模型</th>
                <th class="text-left py-1">切分方式</th>
                <th class="text-right py-1">rows/股票數</th>
                <th class="text-right py-1">hold/break AUC</th>
                <th class="text-left py-1">校準</th>
                <th class="text-left py-1">時間</th>
              </tr>
            </thead>
            <tbody>
              {#each recentTrainJobs as job (job.job_id)}
                <tr class="border-b border-border/30">
                  <td class="py-1">
                    <span class="inline-flex items-center px-1.5 py-0 rounded-full font-medium {trainStatusClass[job.status]}">
                      {trainStatusLabel[job.status]}
                    </span>
                  </td>
                  <td class="py-1 text-white">{job.model_type}{job.model_version ? ` (${job.model_version})` : ''}</td>
                  <td class="py-1 text-white">{splitMethodLabel(job)}</td>
                  <td class="py-1 text-right text-white">{job.rows ?? '—'} / {symbolCount(job)}</td>
                  <td class="py-1 text-right text-white">{metricValue(job, 'hold', 'auc')} / {metricValue(job, 'break', 'auc')}</td>
                  <td class="py-1 text-muted">{calibratedLabel(job, 'hold')}</td>
                  <td class="py-1 text-muted font-mono">{formatDateTime(job.created_at)}</td>
                </tr>
                {#if job.status === 'failed' && job.error}
                  <tr class="border-b border-border/30">
                    <td colspan="7" class="py-1 text-rise">{job.error}</td>
                  </tr>
                {/if}
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>

    <!-- ── 模型驗證 / Decision Replay ────────────────────────── -->
    <div class="bg-panel border border-border rounded-xl px-5 py-4">
      <div class="flex flex-wrap items-start justify-between gap-3 mb-3">
        <div>
          <h2 class="text-sm font-semibold text-white mb-1">模型驗證 / Decision Replay</h2>
        </div>
        <button
          class="text-xs text-muted hover:text-white px-2 py-1 border border-border rounded transition-colors"
          on:click={loadRegressionResults}
        >
          重新整理
        </button>
      </div>

      {#if evaluationError}
        <p class="text-rise text-sm mb-3">{evaluationError}</p>
      {/if}
      {#if evaluationMessage}
        <p class="text-green-400 text-sm mb-3">{evaluationMessage}</p>
      {/if}

      <div class="flex flex-wrap gap-3 items-center">
        <select
          bind:value={evaluationMode}
          class="bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 focus:outline-none focus:border-indigo-500 transition-colors"
        >
          <option value="decision_replay">Decision Replay</option>
          <option value="evaluation">Zone Evaluation</option>
        </select>
        <input
          bind:value={evaluationSymbols}
          placeholder="股票代號，逗號分隔（留空 = 上方股票代號）"
          class="flex-1 min-w-[220px] bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 placeholder:text-muted focus:outline-none focus:border-indigo-500 transition-colors"
        />
        <input
          type="number"
          min="80"
          step="100"
          bind:value={evaluationLimit}
          title="evaluation 用的歷史K棒根數（每檔股票）"
          class="w-32 bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                 focus:outline-none focus:border-indigo-500 transition-colors"
        />
        {#if evaluationMode === 'decision_replay'}
          <input
            type="number"
            min="1"
            step="25"
            bind:value={evaluationReplayMaxRows}
            title="Decision Replay 輸出 rows 上限"
            class="w-32 bg-surface border border-border rounded-lg px-3 py-2 text-sm text-white
                   focus:outline-none focus:border-indigo-500 transition-colors"
          />
        {/if}
        <label class="inline-flex items-center gap-2 text-xs text-muted">
          <input
            type="checkbox"
            bind:checked={evaluationWriteDB}
            class="rounded border-border bg-surface text-indigo-600 focus:ring-indigo-500"
          />
          <span>寫入結果</span>
        </label>
        <button
          class="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm
                 font-medium px-5 py-2 rounded-lg transition-colors"
          disabled={evaluating}
          on:click={runEvaluation}
        >
          {evaluating ? '執行中...' : '開始驗證'}
        </button>
      </div>

      {#if evaluationWriteDB}
        <p class="text-yellow-400 text-xs mt-2">
          ⚠ 結果會寫入 stock_sr_regression_results，並成為此模型（model_config_hash）
          production entry gate 的判定依據。第一次寫入會讓原本未生效的 gate 開始作用；
          若判定為 UNRELIABLE，正式分析會保守阻擋進場。只想看報告請取消勾選。
        </p>
      {/if}

      {#if activeEvaluationJob}
        <div class="mt-3 px-3 py-2 bg-surface/60 rounded-lg text-xs flex flex-wrap items-center gap-2">
          <span class="inline-flex items-center px-2 py-0.5 rounded-full font-medium {evaluationStatusClass[activeEvaluationJob.status]}">
            {evaluationStatusLabel[activeEvaluationJob.status]}
          </span>
          <span class="text-muted font-mono">{activeEvaluationJob.job_id}</span>
          <span class="text-white">{evaluationSchemaLabel(activeEvaluationJob.schema_version ?? activeEvaluationJob.mode)}</span>
          <span class="text-muted">rows={activeEvaluationJob.rows ?? '—'} sources={activeEvaluationJob.sources ?? '—'}</span>
          {#if activeEvaluationJob.run_id}
            <span class="text-muted font-mono">{activeEvaluationJob.run_id}</span>
          {/if}
          {#if activeEvaluationJob.status === 'failed' && activeEvaluationJob.error}
            <span class="text-rise">{activeEvaluationJob.error}</span>
          {/if}
        </div>
      {/if}

      {#if evaluationReport}
        <div class="mt-3 px-3 py-2 bg-surface/60 rounded-lg text-xs flex flex-wrap items-center gap-2">
          <span class="inline-flex items-center px-2 py-0.5 rounded-full font-medium bg-blue-900/40 text-blue-400">
            {evaluationSchemaLabel(evaluationReport.schema_version)}
          </span>
          <span class="text-muted font-mono">{evaluationReport.run_id}</span>
          <span class="text-white">rows={evaluationReport.rows ?? '—'} sources={evaluationReport.sources ?? '—'}</span>
          {#if evaluationReport.decision_replay_available !== undefined}
            <span class="text-muted">decision={evaluationReport.decision_replay_available ? 'available' : 'unavailable'}</span>
          {/if}
          {#if evaluationReport.event_lifecycle_replay_available !== undefined}
            <span class="text-muted">lifecycle={evaluationReport.event_lifecycle_replay_available ? 'available' : 'unavailable'}</span>
          {/if}
        </div>

        <!-- 治理判定：write_db=false 的試跑也看得到，不必為了看結論而寫 DB 啟動 gate。 -->
        {#if evaluationReport.governance_evaluation}
          {@const governance = evaluationReport.governance_evaluation}
          {@const gate = governance.confidence_gate ?? {}}
          {@const flags = governanceFlags(governance)}
          <div class="mt-2 px-3 py-2 bg-surface/60 rounded-lg text-xs flex flex-wrap items-center gap-2">
            <span class="text-muted">模型治理</span>
            <span class="inline-flex items-center px-2 py-0.5 rounded-full font-medium
              {governanceStateClass[governance.health_state ?? ''] ?? 'bg-gray-700/60 text-gray-400'}">
              {governance.health_state ?? 'UNKNOWN'}
            </span>
            <span class={gate.allow_entry === false ? 'text-rise' : 'text-white'}>
              allow_entry={gate.allow_entry === undefined ? '—' : String(gate.allow_entry)}
            </span>
            <span class="text-white">max_entry_state={gate.max_entry_state ?? '—'}</span>
            {#if flags}
              <span class="text-muted max-w-[320px] truncate" title={flags}>{flags}</span>
            {/if}
            {#if evaluationReport.replay_coverage}
              <span class="text-muted">
                覆蓋 {coverageLabel(evaluationReport.replay_coverage)}
              </span>
              {#if (evaluationReport.replay_coverage.symbols_skipped ?? []).length > 0}
                <span
                  class="text-yellow-400 max-w-[240px] truncate"
                  title={(evaluationReport.replay_coverage.symbols_skipped ?? []).join(', ')}
                >
                  略過 {(evaluationReport.replay_coverage.symbols_skipped ?? []).join(', ')}
                </span>
              {/if}
            {/if}
          </div>
        {/if}
      {/if}

      <div class="mt-4">
        {#if recentEvaluationJobs.length > 0}
          <div class="mb-4">
            <p class="text-muted text-xs mb-1.5">最近 evaluation jobs</p>
            <table class="w-full text-xs">
              <thead>
                <tr class="text-muted border-b border-border/60">
                  <th class="text-left py-1">狀態</th>
                  <th class="text-left py-1">類型</th>
                  <th class="text-left py-1">run id</th>
                  <th class="text-right py-1">rows</th>
                  <th class="text-left py-1">時間</th>
                </tr>
              </thead>
              <tbody>
                {#each recentEvaluationJobs as job (job.job_id)}
                  <tr class="border-b border-border/30">
                    <td class="py-1">
                      <span class="inline-flex items-center px-1.5 py-0 rounded-full font-medium {evaluationStatusClass[job.status]}">
                        {evaluationStatusLabel[job.status]}
                      </span>
                    </td>
                    <td class="py-1 text-white">{evaluationSchemaLabel(job.schema_version ?? job.mode)}</td>
                    <td class="py-1 text-muted font-mono">{job.run_id ?? job.job_id}</td>
                    <td class="py-1 text-right text-white">{job.rows ?? '—'}</td>
                    <td class="py-1 text-muted font-mono">{formatDateTime(job.created_at)}</td>
                  </tr>
                  {#if job.status === 'failed' && job.error}
                    <tr class="border-b border-border/30">
                      <td colspan="5" class="py-1 text-rise">{job.error}</td>
                    </tr>
                  {/if}
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
        <div class="flex flex-wrap items-center justify-between gap-3 mb-1.5">
          <p class="text-muted text-xs">最近 regression results</p>
          <select
            bind:value={regressionSchemaVersion}
            on:change={loadRegressionResults}
            class="bg-surface border border-border rounded px-2 py-1 text-xs text-white
                   focus:outline-none focus:border-indigo-500 transition-colors"
          >
            <option value="sr_zone_decision_replay_p0">Decision Replay</option>
            <option value="sr_zone_evaluation_p0">Zone Evaluation</option>
            <option value="">全部</option>
          </select>
        </div>
        {#if regressionResults.length > 0}
          <table class="w-full text-xs">
            <thead>
              <tr class="text-muted border-b border-border/60">
                <th class="text-left py-1">類型</th>
                <th class="text-left py-1">run id</th>
                <th class="text-left py-1">股票</th>
                <th class="text-right py-1">rows</th>
                <th class="text-right py-1">hold/break AUC</th>
                <th class="text-left py-1">治理</th>
                <th class="text-left py-1">警告</th>
                <th class="text-left py-1">時間</th>
              </tr>
            </thead>
            <tbody>
              {#each regressionResults as result (result.id)}
                <tr class="border-b border-border/30">
                  <td class="py-1 text-white">{evaluationSchemaLabel(result.schema_version || result.metrics_json?.schema_version)}</td>
                  <td class="py-1 text-muted font-mono">{result.run_id}</td>
                  <td class="py-1 text-white">{regressionSymbols(result)}</td>
                  <td class="py-1 text-right text-white">{result.rows ?? regressionRows(result) ?? '—'}</td>
                  <td class="py-1 text-right text-white">{fmt(result.hold_auc)} / {fmt(result.break_auc)}</td>
                  <td class="py-1 text-white">{result.governance_health_state || '—'}</td>
                  <td class="py-1 text-muted max-w-[260px] truncate" title={regressionWarnings(result)}>{regressionWarnings(result)}</td>
                  <td class="py-1 text-muted font-mono">{formatDateTime(result.created_at)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {:else}
          <p class="text-muted text-xs py-2">尚無 regression result。</p>
        {/if}
      </div>
    </div>

    <!-- ── 目前分析結果 ──────────────────────────────────────── -->
    {#if current}
      <div class="bg-panel border border-border rounded-xl overflow-hidden">
        <div class="px-5 py-4 border-b border-border flex items-center justify-between">
          <div>
            <h2 class="text-white font-semibold">{current.symbol}</h2>
            <p class="text-muted text-xs mt-0.5">分析時間：{formatDateTime(current.analyzed_at)}</p>
            {#if verifyError}
              <p class="text-rise text-xs mt-0.5">{verifyError}</p>
            {/if}
          </div>
          <div class="text-right">
            <p class="text-white font-mono text-lg">{fmt(current.current_price)}</p>
            <p class="text-muted text-xs mb-1.5">{currentZones.length} 個區間</p>
            <button
              class="text-xs px-2.5 py-1 border border-border text-muted hover:text-white rounded transition-colors disabled:opacity-50"
              disabled={verifying}
              on:click={runVerify}
            >
              {verifying ? '驗證中...' : '重新驗證'}
            </button>
          </div>
        </div>

        {#if analysisTips.length > 0}
          <div class="px-5 py-3 border-b border-border bg-surface/50 overflow-hidden">
            <div class="flex items-center gap-3 text-xs">
              <span class="text-muted shrink-0">分析提示</span>
              <p class="text-white whitespace-nowrap animate-marquee">{analysisTips[activeTipIndex]}</p>
            </div>
          </div>
        {/if}

        <div class="px-5 py-4 border-b border-border">
          <SRChipPanel summary={chipSummary} />
        </div>
        {#if explanationSummary || explanationActionReason || explanationDrivers.length > 0 || explanationRisks.length > 0}
          <div class="px-5 py-4 border-b border-border bg-surface/35">
            <div class="flex items-start justify-between gap-4 flex-wrap mb-3">
              <div class="min-w-0">
                <p class="text-muted text-xs mb-1">解釋</p>
                {#if explanationSummary}
                  <h3 class="text-white text-sm font-semibold leading-relaxed">{explanationSummary}</h3>
                {/if}
                {#if explanationActionReason}
                  <p class="text-muted text-xs mt-1 leading-relaxed">{explanationActionReason}</p>
                {/if}
              </div>
              {#if analysisExplanation?.model_context}
                <div class="text-right text-[11px] text-muted font-mono">
                  <p>{analysisExplanation.model_context.version}{analysisExplanation.model_context.config_hash ? ` / ${analysisExplanation.model_context.config_hash}` : ''}</p>
                  <p>{analysisExplanation.model_context.uses_shap_evidence ? 'SHAP evidence' : 'rules only'}</p>
                </div>
              {/if}
            </div>
            <div class="grid md:grid-cols-2 gap-4 text-xs">
              {#if explanationDrivers.length > 0}
                <div>
                  <p class="text-white font-medium mb-2">主要因素</p>
                  <div class="space-y-1">
                    {#each explanationDrivers as item}
                      <p class="text-muted leading-relaxed">{item}</p>
                    {/each}
                  </div>
                </div>
              {/if}
              {#if explanationRisks.length > 0}
                <div>
                  <p class="text-white font-medium mb-2">風險提醒</p>
                  <div class="space-y-1">
                    {#each explanationRisks as item}
                      <p class="text-yellow-300 leading-relaxed">{item}</p>
                    {/each}
                  </div>
                </div>
              {/if}
            </div>
          </div>
        {/if}
        {#if globalEvidence}
          <div class="px-5 py-3 border-b border-border bg-indigo-950/20">
            <div class="flex items-center justify-between gap-3 flex-wrap">
              <div>
                <p class="text-white text-sm font-medium">模型證據</p>
                {#if globalEvidence.model.explainer}
                  <p class="text-muted text-xs">
                    {globalEvidence.model.explainer} · 解釋校準、正規化後的最終機率
                  </p>
                {:else}
                  <p class="text-muted text-xs">規則式評分（本次未產生 SHAP evidence）</p>
                {/if}
              </div>
              <div class="text-xs text-muted font-mono">
                pipeline {current.pipeline_version} · model {globalEvidence.model.version}/{globalEvidence.model.config_hash}
              </div>
            </div>
          </div>
        {/if}
        {#if decisionSummary && hasDecisionDetail}
          <div class="px-5 py-4 border-b border-border bg-surface/40 decision-summary-panel">
            <div class="flex items-start justify-between gap-4 flex-wrap mb-4">
              <div>
                <p class="text-muted text-xs mb-1">Market Regime</p>
                <div class="flex items-center gap-2 flex-wrap">
                  <h3 class="text-white font-semibold">{decisionSummary.market_regime?.label || regimePrimaryText[decisionSummary.market_regime?.primary ?? ''] || decisionSummary.market_regime?.primary}</h3>
                  {#if decisionSummary.market_regime?.trend_regime}
                    <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] bg-blue-900/40 text-blue-300">{regimePrimaryText[decisionSummary.market_regime.trend_regime] ?? decisionSummary.market_regime.trend_regime}</span>
                  {/if}
                  {#if decisionSummary.market_regime?.structure_state}
                    <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] bg-surface text-muted border border-border">{structureStateText[decisionSummary.market_regime.structure_state] ?? decisionSummary.market_regime.structure_state}</span>
                  {/if}
                  {#if decisionSummary.market_regime?.short_term_regime && decisionSummary.market_regime.short_term_regime !== 'NORMAL'}
                    <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] bg-purple-900/40 text-purple-300">{shortTermRegimeText[decisionSummary.market_regime.short_term_regime] ?? decisionSummary.market_regime.short_term_regime}</span>
                  {/if}
                  {#each decisionSummary.market_regime?.flags ?? [] as flag}
                    <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] bg-yellow-900/40 text-yellow-300">{regimeFlagText[flag] ?? flag}</span>
                  {/each}
                </div>
                {#if (decisionSummary.market_regime?.reasons ?? []).length > 0}
                  <p class="text-muted text-xs mt-1">{decisionSummary.market_regime?.reasons?.join('、')}</p>
                {/if}
              </div>
              <div class="flex gap-2 text-right flex-wrap justify-end">
                <div>
                  <p class="text-muted text-xs mb-1">Market Bias</p>
                  <span class="inline-flex items-center px-3 py-1 rounded-lg border text-sm font-semibold {marketBiasClass[decisionSummary.market_bias ?? 'NEUTRAL_BIAS'] ?? 'bg-gray-700/60 text-gray-300 border-border'}">
                    {decisionSummary.market_bias_label ?? marketBiasText[decisionSummary.market_bias ?? 'NEUTRAL_BIAS'] ?? decisionSummary.market_bias ?? 'NEUTRAL_BIAS'}
                  </span>
                </div>
                {#if decisionSummary.final_entry_permission}
                  <div>
                    <p class="text-muted text-xs mb-1">Final Entry</p>
                    <span class="inline-flex items-center px-3 py-1 rounded-lg border text-sm font-semibold {finalEntryClass[decisionSummary.final_entry_permission.state] ?? 'bg-gray-700/60 text-gray-300 border-border'}">
                      {decisionSummary.final_entry_permission.label ?? finalEntryText[decisionSummary.final_entry_permission.state] ?? decisionSummary.final_entry_permission.state}
                    </span>
                  </div>
                {/if}
                {#if decisionSummary.entry_action_state}
                  <div>
                    <p class="text-muted text-xs mb-1">Entry State（明細）</p>
                    <span class="inline-flex items-center px-3 py-1 rounded-lg border text-sm font-semibold {entryActionClass[decisionSummary.entry_action_state] ?? 'bg-gray-700/60 text-gray-300 border-border'}">
                      {decisionSummary.entry_action_label ?? entryActionText[decisionSummary.entry_action_state] ?? decisionSummary.entry_action_state}
                    </span>
                  </div>
                {/if}
                <div>
                  <p class="text-muted text-xs mb-1">Position Action</p>
                  <span class="inline-flex items-center px-3 py-1 rounded-lg border text-sm font-semibold {positionActionClass[decisionSummary.position_action ?? 'HOLD'] ?? 'bg-gray-700/60 text-gray-300 border-border'}">
                    {positionActionText[decisionSummary.position_action ?? 'HOLD'] ?? decisionSummary.position_action ?? 'HOLD'}
                  </span>
                </div>
              </div>
            </div>

            {#if (decisionSummary.decision_derived_view?.authority_reason_codes ?? []).length > 0}
              <div class="mb-4">
                <p class="text-muted text-xs mb-1">判定依據（Event Lifecycle 推導）</p>
                <div class="flex gap-1.5 flex-wrap">
                  {#each decisionSummary.decision_derived_view?.authority_reason_codes ?? [] as code}
                    <span class="inline-flex items-center px-2 py-0.5 rounded border border-border/70 bg-panel/50 text-muted text-xs font-mono">{derivedReasonLabel(code)}</span>
                  {/each}
                </div>
              </div>
            {/if}

            {#if decisionSummary.final_entry_permission || decisionSummary.entry_action_state}
              <p class="text-muted text-xs mb-4">
                是否允許進場以 <span class="font-semibold">Final Entry</span> 為準（entry 與日 K 兩端的保守仲裁）；Entry State 為進場階段明細，Market Bias 只表示整體盤勢傾向，未確認的設定（如「觀察性試探」）不代表可直接進場。
              </p>
            {/if}

            {#if decisionSummary.data_quality}
              <div class="border border-border/70 rounded-lg p-3 bg-panel/50 mb-4 text-xs">
                <div class="flex items-center justify-between gap-3 flex-wrap">
                  <p class="text-muted">資料模式：<span class="text-white font-mono">{decisionSummary.data_quality.data_mode ?? decisionSummary.data_mode ?? 'END_OF_DAY'}</span></p>
                  <p class="text-muted">完整度 <span class="text-white font-mono">{fmtPct(decisionSummary.data_quality.overall_completeness)}</span></p>
                </div>
                {#if decisionSummary.data_quality.notes.length > 0}
                  <p class="text-muted mt-1">{decisionSummary.data_quality.notes.join('、')}</p>
                {/if}
                {#if decisionSummary.data_quality.missing_features.length > 0}
                  <p class="text-yellow-300 mt-1">缺資料：{decisionSummary.data_quality.missing_features.join(' / ')}</p>
                {/if}
                {#if decisionSummary.data_quality.stale_features && decisionSummary.data_quality.stale_features.length > 0}
                  <p class="text-yellow-300 mt-1">資料過期：{decisionSummary.data_quality.stale_features.join(' / ')}</p>
                {/if}
                {#if decisionSummary.data_quality.invalid_features && decisionSummary.data_quality.invalid_features.length > 0}
                  <p class="text-red-300 mt-1">資料無效：{decisionSummary.data_quality.invalid_features.join(' / ')}</p>
                {/if}
                {#if decisionSummary.data_quality.negative_features && decisionSummary.data_quality.negative_features.length > 0}
                  <p class="text-red-300 mt-1">負向資料：{decisionSummary.data_quality.negative_features.join(' / ')}</p>
                {/if}
                {#if decisionSummary.data_quality.positive_features && decisionSummary.data_quality.positive_features.length > 0}
                  <p class="text-rise mt-1">正向資料：{decisionSummary.data_quality.positive_features.join(' / ')}</p>
                {/if}
                {#if decisionSummary.data_quality.neutral_features && decisionSummary.data_quality.neutral_features.length > 0}
                  <p class="text-muted mt-1">中性資料：{decisionSummary.data_quality.neutral_features.join(' / ')}</p>
                {/if}
              </div>
            {/if}

            {#if decisionSummary.event_sequence && decisionSummary.event_sequence.length > 0}
              <div class="border border-border/70 rounded-lg p-3 bg-panel/50 mb-4 text-xs">
                <p class="text-muted mb-2">Event Sequence</p>
                <div class="flex items-center gap-2 flex-wrap">
                  {#each decisionSummary.event_sequence as event, i}
                    {#if i > 0}<span class="text-muted">→</span>{/if}
                    <span class="inline-flex items-center px-2 py-0.5 rounded-full bg-surface text-white border border-border">{event.label}</span>
                  {/each}
                </div>
              </div>
            {/if}

            {#if decisionSummary.daily_price_action?.available}
              <div class="grid md:grid-cols-3 lg:grid-cols-5 gap-3 mb-4 text-xs">
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">Close Location</p>
                  <p class="text-white">{dailyActionText[decisionSummary.daily_price_action.close_location_state] ?? decisionSummary.daily_price_action.close_location_state}</p>
                </div>
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">Daily Range</p>
                  <p class="text-white">{dailyActionText[decisionSummary.daily_price_action.range_state] ?? decisionSummary.daily_price_action.range_state}</p>
                </div>
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">Follow Through</p>
                  <p class="text-white">{dailyActionText[decisionSummary.daily_price_action.price_follow_through_state ?? decisionSummary.daily_price_action.follow_through_state] ?? (decisionSummary.daily_price_action.price_follow_through_state ?? decisionSummary.daily_price_action.follow_through_state)}</p>
                </div>
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">Momentum</p>
                  <p class="text-white">{decisionSummary.daily_price_action.momentum_confirmation_state ? (dailyActionText[decisionSummary.daily_price_action.momentum_confirmation_state] ?? decisionSummary.daily_price_action.momentum_confirmation_state) : '—'}</p>
                </div>
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">Reclaim / Rejection</p>
                  <p class="text-white">{dailyActionText[decisionSummary.daily_price_action.reclaim_rejection_state] ?? decisionSummary.daily_price_action.reclaim_rejection_state}</p>
                </div>
              </div>
            {/if}

            {#if decisionSummary.daily_confirmation}
              <div class="grid md:grid-cols-3 gap-3 mb-4 text-xs">
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">Daily Confirmation</p>
                  <p class="text-white font-semibold">{decisionSummary.daily_confirmation.label}</p>
                </div>
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">Daily Entry State</p>
                  <p class="text-white font-mono">{decisionSummary.daily_entry_state ?? decisionSummary.daily_confirmation.state}</p>
                </div>
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">Reason Codes</p>
                  <p class="text-white font-mono break-words">{positionReasonCodesText(decisionSummary.daily_confirmation.reason_codes)}</p>
                </div>
              </div>
            {/if}

            {#if decisionSummary.position_action_condition}
              <div class="grid sm:grid-cols-3 gap-3 mb-4 text-xs">
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">防守線</p>
                  <p class="text-fall font-mono">{fmt(decisionSummary.position_action_condition.invalidation_price)}</p>
                </div>
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">回穩線</p>
                  <p class="text-rise font-mono">{fmt(decisionSummary.position_action_condition.recovery_price)}</p>
                </div>
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">Reason Codes</p>
                  <p class="text-white font-mono break-words">{positionReasonCodesText(decisionSummary.position_action_condition.reason_codes)}</p>
                </div>
              </div>
            {/if}

            {#if decisionSummary.rr_gate}
              <div class="grid sm:grid-cols-3 gap-3 mb-4 text-xs">
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">RR Gate</p>
                  <p class="{decisionSummary.rr_gate.qualified ? 'text-rise' : 'text-yellow-300'} font-semibold">{decisionSummary.rr_gate.qualified ? '通過' : '未通過'}</p>
                </div>
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">最低 RR</p>
                  <p class="text-white font-mono">{fmtRatio(decisionSummary.rr_gate.minimum_rr)}</p>
                </div>
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">Reason Code</p>
                  <p class="text-white font-mono break-words">{decisionSummary.rr_gate.reason_code}</p>
                </div>
              </div>
            {/if}

            {#if decisionSummary.rr_context}
              <div class="grid sm:grid-cols-2 gap-3 mb-4 text-xs">
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">進場 RR (Entry)</p>
                  <p class="text-white font-mono">{fmtRatio(decisionSummary.rr_context.entry_rr)}</p>
                  <p class="text-muted mt-1">{decisionSummary.rr_context.entry_rr_source}</p>
                </div>
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">持股 RR (Position)</p>
                  <p class="text-white font-mono">{fmtRatio(decisionSummary.rr_context.position_rr)}</p>
                  <p class="text-muted mt-1">{decisionSummary.rr_context.position_rr_source === 'UNAVAILABLE' ? 'SR 層不計持股 RR，請看 Position 分析' : decisionSummary.rr_context.position_rr_source}</p>
                </div>
              </div>
            {/if}

            {#if decisionSummary.price_path}
              <div class="grid md:grid-cols-4 gap-3 mb-4 text-xs">
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">Price Path</p>
                  <p class="text-white">{pricePathText[decisionSummary.price_path.path_state] ?? decisionSummary.price_path.path_state}</p>
                </div>
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">失效 / 回穩</p>
                  <p class="text-white font-mono">{fmt(decisionSummary.price_path.invalidation_price)} / {fmt(decisionSummary.price_path.recovery_price)}</p>
                </div>
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">Next Decision</p>
                  <p class="text-white font-mono">{fmt(decisionSummary.price_path.next_decision_price)}</p>
                </div>
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">Blocking Zone</p>
                  <p class="text-white font-mono">{decisionSummary.price_path.blocking_zone?.label ?? '—'}</p>
                  {#if decisionSummary.price_path.blocking_zone}
                    <p class="text-muted mt-1">
                      {decisionSummary.price_path.blocking_zone.method ?? decisionSummary.price_path.blocking_zone.source}
                      {#if decisionSummary.price_path.blocking_zone.tier_label}
                        · {decisionSummary.price_path.blocking_zone.tier_label}
                      {/if}
                      {#if decisionSummary.price_path.blocking_zone.confidence_level}
                        · {decisionSummary.price_path.blocking_zone.confidence_level}
                      {/if}
                    </p>
                    {#if decisionSummary.price_path.blocking_zone.selected_summary_zone === false}
                      <p class="text-yellow-300 mt-1">來源：完整 zone pool</p>
                    {/if}
                  {/if}
                </div>
              </div>
            {/if}

            {#if decisionSummary.primary_zone}
              <div class="grid lg:grid-cols-[1.2fr_1fr] gap-4 mb-4">
                <div class="border border-border/70 rounded-lg p-3 bg-panel/60">
                  <div class="flex items-center justify-between gap-3 mb-2">
                    <p class="text-muted text-xs">Primary Zone</p>
                    <span class="text-[11px] text-muted">距離 {decisionSummary.primary_zone.zone_interaction?.distance_label ?? decisionSummary.primary_zone.distance_label}</span>
                  </div>
                  <p class="font-mono text-white text-base">{decisionSummary.primary_zone.label}</p>
                  <p class="text-xs text-muted mt-1">{noviceRoleText[decisionSummary.primary_zone.role] ?? decisionSummary.primary_zone.role} · {decisionSummary.primary_zone.reason}</p>
                  <div class="flex flex-wrap gap-2 mt-3 text-[11px]">
                    <span class="text-white">{decisionSummary.primary_zone.zone_interaction?.state_label ?? '尚未測試'}</span>
                    <span class="text-muted">區間辨識信心 {decisionSummary.confidence_explanation?.label ?? '—'}</span>
                    <span class="text-muted">Score {fmtScore100(decisionSummary.primary_zone.trading_score)}</span>
                    <span class="text-muted">EV {fmtSignedPct(decisionSummary.primary_zone.expected_value)}</span>
                    <span class="text-muted">RR {fmtRatio(decisionSummary.primary_zone.risk_reward_ratio)}</span>
                    {#if (decisionSummary.primary_zone.confluence_family_count ?? decisionSummary.primary_zone.confluence_count) > 1}
                      <span class="text-indigo-300">證據族群 ×{decisionSummary.primary_zone.confluence_family_count ?? decisionSummary.primary_zone.confluence_count}</span>
                    {/if}
                  </div>
                </div>

                <div class="border border-border/70 rounded-lg p-3 bg-panel/60">
                  <p class="text-muted text-xs mb-2">Market Context</p>
                  <div class="grid grid-cols-2 gap-2 text-xs">
                    {#each decisionSummary.market_context ?? [] as ctx}
                      <div>
                        <p class="text-muted">{ctx.label}</p>
                        <p class="text-white font-mono">{ctx.value}</p>
                      </div>
                    {/each}
                  </div>
                  {#if (decisionSummary.risk_notes ?? []).length > 0}
                    <div class="mt-3 space-y-1">
                      {#each decisionSummary.risk_notes ?? [] as note}
                        <p class="text-yellow-300 text-xs">{note}</p>
                      {/each}
                    </div>
                  {/if}
                </div>
              </div>
            {/if}

            {#if decisionSummary.best_trade_zone || decisionSummary.nearest_support_zone || decisionSummary.nearest_resistance_zone || decisionSummary.primary_structural_zone}
              <div class="grid md:grid-cols-2 lg:grid-cols-4 gap-3 mb-4 text-xs">
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">Best Trade Zone</p>
                  <p class="text-white font-mono">{decisionSummary.best_trade_zone?.label ?? '尚未成立'}</p>
                  {#if decisionSummary.best_trade_zone?.lifecycle}
                    <p class="text-muted mt-1">{decisionSummary.best_trade_zone.source} · {decisionSummary.best_trade_zone.lifecycle}</p>
                  {/if}
                </div>
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">Nearest Support Zone</p>
                  <p class="text-white font-mono">{decisionSummary.nearest_support_zone?.label ?? '—'}</p>
                  {#if decisionSummary.nearest_support_zone}
                    <p class="text-muted mt-1">距離 {decisionSummary.nearest_support_zone.distance_label}{#if decisionSummary.nearest_support_zone.lifecycle} · {decisionSummary.nearest_support_zone.lifecycle}{/if}</p>
                  {/if}
                </div>
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">Nearest Resistance Zone</p>
                  <p class="text-white font-mono">{decisionSummary.nearest_resistance_zone?.label ?? '—'}</p>
                  {#if decisionSummary.nearest_resistance_zone}
                    <p class="text-muted mt-1">距離 {decisionSummary.nearest_resistance_zone.distance_label}{#if decisionSummary.nearest_resistance_zone.lifecycle} · {decisionSummary.nearest_resistance_zone.lifecycle}{/if}</p>
                  {/if}
                </div>
                <div class="border border-border/70 rounded-lg p-3 bg-panel/50">
                  <p class="text-muted mb-1">Primary Structural Zone</p>
                  <p class="text-white font-mono">{decisionSummary.primary_structural_zone?.label ?? '—'}</p>
                  {#if decisionSummary.primary_structural_zone?.lifecycle}
                    <p class="text-muted mt-1">{decisionSummary.primary_structural_zone.source} · {decisionSummary.primary_structural_zone.lifecycle}</p>
                  {/if}
                </div>
              </div>
            {/if}

            {#if decisionSummary.daily_candidate_zones && decisionSummary.daily_candidate_zones.length > 0}
              <div class="border border-border/70 rounded-lg p-3 bg-panel/50 mb-4 text-xs">
                <p class="text-muted mb-2">Daily Candidate Zones</p>
                <div class="grid md:grid-cols-2 gap-3">
                  {#each decisionSummary.daily_candidate_zones as z}
                    <div>
                      <p class="font-mono text-white">{z.label}</p>
                      <p class="text-muted mt-1">{noviceRoleText[z.role] ?? z.role} · {z.source} · {z.lifecycle}</p>
                      <p class="text-muted mt-1">{z.reason}</p>
                    </div>
                  {/each}
                </div>
              </div>
            {/if}

            <details class="text-xs">
              <summary class="cursor-pointer text-muted hover:text-white">區間辨識信心來源與次要區間</summary>
              <div class="mt-3 grid md:grid-cols-2 gap-3">
                <div class="space-y-2">
                  <p class="text-white font-medium">Zone Confidence {decisionSummary.confidence_explanation?.label ?? '—'}</p>
                  {#each decisionSummary.confidence_explanation?.formula_factors ?? [] as f}
                    <p class="text-muted">{f.label}：{f.description}</p>
                  {/each}
                  {#each decisionSummary.confidence_explanation?.context_factors ?? [] as f}
                    <p class="text-muted">{f.label}</p>
                  {/each}
                </div>
                <div class="space-y-2">
                  <p class="text-white font-medium">Secondary Zones</p>
                  {#each (decisionSummary.secondary_zones ?? []).slice(0, 3) as z}
                    <p class="text-muted"><span class="font-mono text-white">{z.label}</span> · {noviceRoleText[z.role] ?? z.role} · 距離 {z.distance_label}</p>
                  {/each}
                </div>
              </div>
            </details>
          </div>
        {:else if decisionSummary && hasNormalizedDecision}
          <div class="px-5 py-4 border-b border-border bg-surface/30">
            <p class="text-white text-sm font-medium">Decision Snapshot</p>
            <p class="text-muted text-xs mt-1">此分析有 normalized decision authority fields，但缺少完整 decision detail snapshot。</p>
          </div>
        {:else if missingNormalizedDecision}
          <div class="px-5 py-4 border-b border-border bg-surface/30">
            <p class="text-white text-sm font-medium">Decision Snapshot</p>
            <p class="text-muted text-xs mt-1">此分析缺少 normalized decision snapshot；請重新分析以產生新版決策資料。</p>
          </div>
        {/if}

        <div class="divide-y divide-border border-b border-border">
          {#if periodSummaries.length > 0}
            {#each periodSummaries as period (period.key)}
              <div class="px-5 py-4">
                <div class="flex items-center justify-between gap-3 mb-3">
                  <h3 class="text-sm font-semibold text-white">{period.label}</h3>
                  <span class="text-muted text-xs">每期保留一個支撐、一個壓力</span>
                </div>
                <div class="grid md:grid-cols-2 gap-3">
                  {#each summarySides as side}
                    {@const item = side === 'support' ? period.support : period.resistance}
                    {@const note = side === 'support' ? period.support_note : period.resistance_note}
                    <div class="border border-border/70 rounded-lg p-3 bg-surface/30">
                      <div class="flex items-center justify-between gap-2 mb-2">
                        <p class="text-muted text-xs">{summaryDisplayLabel(item, side)}</p>
                        {#if item}
                          <span class="text-[11px] text-muted">區間辨識信心 {noviceConfidenceText[item.confidence_level] ?? item.confidence_level}</span>
                        {/if}
                      </div>
                      <p class="font-mono text-sm {summaryAccent(side)}">{summaryPriceText(item)}</p>
                      {#if item?.chip}
                        <div class="flex items-center flex-wrap gap-x-2 gap-y-1 mt-2 text-[11px]">
                          <span class="text-muted">{chipDirectionText[item.chip.direction] ?? '籌碼'}</span>
                          {#if item.chip.direction !== 'none'}
                            <span class={chipEffectClass(item)}>· {chipEffectText(item)}</span>
                            {#if item.chip.contribution !== null}
                              <span class="font-mono text-muted">· 貢獻 {item.chip.contribution.toFixed(1)}/15</span>
                            {/if}
                            {#if chipDeltaText(item)}
                              <span class="font-mono {chipEffectClass(item)}">· {chipDeltaText(item)}</span>
                            {/if}
                          {/if}
                        </div>
                      {/if}
                      <p class="text-xs text-muted mt-2 leading-relaxed">{summaryNote(item, note)}</p>
                      {#if confluenceFamilyCount(item) > 1}
                        <p class="text-[11px] text-indigo-300 mt-1">證據族群 ×{confluenceFamilyCount(item)}</p>
                      {/if}
                    </div>
                  {/each}
                </div>
              </div>
            {/each}
          {:else}
            <p class="text-muted text-xs px-5 py-6 text-center">尚無短中長期摘要，請重新分析以取得收斂後資料。</p>
          {/if}
        </div>

        <button
          class="w-full px-5 py-3 text-xs text-muted hover:text-white transition-colors text-left border-b border-border"
          on:click={() => (showDetailedZones = !showDetailedZones)}
        >
          {showDetailedZones ? '▾ 收合完整明細' : '▸ 展開完整明細（原始 Global 指標、所有 zone、機率、EV/RR、籌碼拆解）'}
        </button>

        {#if showDetailedZones}
        <!-- ── 新手總結卡：不展開任何進階區塊也能看懂的重點 ──────── -->
        {#if mainZone}
          <div class="px-5 py-4 border-b border-border bg-indigo-950/20">
            <p class="text-white text-sm font-medium mb-2">
              {noviceRoleText[effectiveRole(mainZone)] ?? effectiveRole(mainZone)}，{noviceRecommendationText[mainZone.trading_recommendation] ?? mainZone.trading_recommendation}
            </p>
            <p class="text-muted text-xs mb-1">主要觀察區間：{fmt(mainZone.price_low)} ~ {fmt(mainZone.price_high)}（{watchRangeText(mainZone)}）</p>
            <p class="text-muted text-xs mb-2">{invalidationText(mainZone)}</p>
            <p class="text-xs">
              <span class="text-muted">區間辨識信心：</span>
              <span class="{noviceConfidenceClass[mainZone.confidence_level] ?? 'text-white'} font-medium">
                {noviceConfidenceText[mainZone.confidence_level] ?? mainZone.confidence_level}
              </span>
              {#if mainZone.confidence_level === 'LOW'}
                <span class="text-muted">（樣本少或太久沒測試，先觀察就好）</span>
              {/if}
            </p>
            {#if effectiveRole(mainZone) === 'AT_ZONE'}
              <p class="text-muted text-xs mt-1">現在在區間內，方向還不明確，不是確定的買賣訊號。</p>
            {/if}
            {#if roleNetScoreConflicts(mainZone)}
              <p class="text-yellow-400 text-xs mt-1">⚠ 這個價位帶過去的表現跟目前角色方向不一致，建議降低信心，展開進階細節查看歷史強弱。</p>
            {/if}
          </div>
        {/if}

        {#if current.scenario}
          <div class="px-5 py-4 border-b border-border bg-surface/40">
            <div class="flex items-start justify-between gap-3 flex-wrap">
              <div>
                <p class="text-white text-sm font-medium">{current.scenario.title}</p>
                <p class="text-muted text-xs mt-1 leading-relaxed">{current.scenario.summary}</p>
              </div>
              <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs bg-indigo-900/40 text-indigo-300">
                {current.scenario.state}
              </span>
            </div>
            <div class="grid md:grid-cols-2 gap-3 mt-3 text-[11px]">
              <div>
                <p class="text-rise font-medium mb-1">觸發條件</p>
                <div class="space-y-1">
                  {#each current.scenario.trigger_conditions as item}
                    <p class="text-muted">{item}</p>
                  {/each}
                </div>
              </div>
              <div>
                <p class="text-yellow-300 font-medium mb-1">失效條件</p>
                <div class="space-y-1">
                  {#each current.scenario.invalidation_conditions as item}
                    <p class="text-muted">{item}</p>
                  {/each}
                </div>
              </div>
            </div>
          </div>
        {/if}

        {#if current.probability_context?.health}
          <div class="px-5 py-4 border-b border-border bg-surface/30">
            <div class="flex items-start justify-between gap-3 flex-wrap">
              <div>
                <p class="text-white text-sm font-medium">機率品質</p>
                <p class="text-muted text-xs mt-1">
                  {#if current.probability_context.health.health_state}
                    狀態 {current.probability_context.health.health_state} ·
                  {/if}
                  可用方向機率 {current.probability_context.health.directional_zone_count}/{current.probability_context.health.zone_count}
                  {#if current.probability_context.health.average_edge_pp !== null}
                    · 平均機率差 {current.probability_context.health.average_edge_pp.toFixed(1)}pp
                  {/if}
                </p>
              </div>
              {#if current.probability_context.health.quality_flags.length > 0}
                <div class="flex flex-wrap gap-1 justify-end">
                  {#each current.probability_context.health.quality_flags as flag}
                    <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] bg-yellow-900/40 text-yellow-300">
                      {probabilityFlagLabel(flag)}
                    </span>
                  {/each}
                </div>
              {/if}
            </div>
          </div>
        {/if}

        <!-- 只有一個 Global Model：整體評估區塊的原始數字，收在進階裡 -->
        <div class="border-b border-border">
          <button
            class="w-full px-5 py-2 text-xs text-muted hover:text-white transition-colors text-left"
            on:click={() => (showAdvancedGlobal = !showAdvancedGlobal)}
          >
            {showAdvancedGlobal ? '▾' : '▸'} 進階：整體指標原始數字
          </button>
          {#if showAdvancedGlobal}
            <div class="px-5 py-3 bg-surface/40 grid grid-cols-2 sm:grid-cols-5 gap-3 text-xs">
              <div>
                <p class="text-muted mb-1">Global Trend</p>
                <p class="font-mono {signedClass(current.global_trend)}">{fmtSignedPct(current.global_trend)}</p>
              </div>
              <div>
                <p class="text-muted mb-1">Global Volatility</p>
                <p class="font-mono text-white">{fmtPct(current.global_volatility)}</p>
              </div>
              <div>
                <p class="text-muted mb-1">Global EV</p>
                <p class="font-mono {signedClass(current.global_expected_value)}">{fmtSignedPct(current.global_expected_value)}</p>
              </div>
              <div>
                        <p class="text-muted mb-1">Global Zone Confidence</p>
                <p class="font-mono text-white">{fmtPct(current.global_confidence)}</p>
              </div>
              <div>
                <p class="text-muted mb-1">Global RR</p>
                <p class="font-mono text-white">{fmtRatio(current.global_risk_reward_ratio)}</p>
              </div>
              <div>
                <p class="text-muted mb-1">模型版本 / 設定 Hash</p>
                <p class="font-mono text-white">{current.model_version}{current.model_config_hash ? ` / ${current.model_config_hash}` : ''}</p>
              </div>
            </div>
          {/if}
        </div>

        <div class="divide-y divide-border">
          {#each tierGroups as group (group.tier)}
            <div>
              <div class="px-5 py-2 bg-surface/60 flex items-baseline gap-2">
                <h3 class="text-sm font-semibold text-white">Tier {tierOrder.indexOf(group.tier) + 1}（{group.label}）</h3>
                <span class="text-muted text-xs">{tierDescription[group.tier]}</span>
              </div>
              <div class="divide-y divide-border/60">
                {#each group.zones as z (z.id)}
                  <div class="px-5 py-4">
                    <!-- 簡化預設檢視：價格區間、白話角色、白話建議、信心、失效條件 -->
                    <div class="flex items-start justify-between gap-3 flex-wrap mb-1">
                      <div>
                        <div class="flex items-center gap-2 flex-wrap mb-1">
                          <span class="font-mono text-white text-sm">{fmt(z.price_low)} ~ {fmt(z.price_high)}</span>
                          <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-gray-700/50 text-gray-200">
                            {zoneDisplayLabel(z)}
                          </span>
                          <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {roleClass[effectiveRole(z)] ?? 'bg-gray-700/60 text-gray-400'}">
                            {noviceRoleText[effectiveRole(z)] ?? effectiveRole(z)}
                            {#if z.resolved_role}
                              <span class="opacity-60 ml-0.5" title="分析當下是 AT_ZONE，後續驗證解析出方向">（已解析）</span>
                            {/if}
                          </span>
                          <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs {statusClass[z.status] ?? 'bg-gray-700/60 text-gray-400'}">
                            {statusLabel[z.status] ?? z.status}
                          </span>
                          {#if confluenceFamilyCount(z) > 1}
                            <span
                              class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-indigo-900/40 text-indigo-300"
                              title="不同證據族群指向這個價位帶，多一分交叉驗證"
                            >證據族群 ×{confluenceFamilyCount(z)}</span>
                          {/if}
                        </div>
                        <p class="text-white text-sm">{noviceRecommendationText[z.trading_recommendation] ?? z.trading_recommendation}</p>
                        {#if z.explanation?.role_summary}
                          <p class="text-muted text-xs mt-1 leading-relaxed">{z.explanation.role_summary}</p>
                        {/if}
                        <p class="text-muted text-xs mt-1">{invalidationText(z)}</p>
                      </div>
                      <div class="text-right shrink-0">
                        <p class="text-muted text-xs mb-1">區間辨識信心</p>
                        <p class="{noviceConfidenceClass[z.confidence_level] ?? 'text-white'} font-medium text-sm">
                          {noviceConfidenceText[z.confidence_level] ?? z.confidence_level}
                        </p>
                      </div>
                    </div>

                    <button
                      class="text-xs text-muted hover:text-white transition-colors mt-1"
                      on:click={() => toggleZoneAdvanced(z.id)}
                    >
                      {expandedZones[z.id] ? '▾ 收合進階細節' : '▸ 展開進階細節（機率、期望值、風險報酬比等原始數字）'}
                    </button>

                    {#if expandedZones[z.id]}
                      <div class="mt-3">
                        <!-- Net Score 分類、方法、英文交易建議徽章、突破時間 -->
                        <div class="flex items-center gap-2 flex-wrap mb-3">
                          <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {netScoreLabelClass[z.net_score_label] ?? ''}">
                            {netScoreLabelText[z.net_score_label] ?? z.net_score_label}
                          </span>
                          <span class="text-muted text-xs">{methodLabel[z.method] ?? z.method}</span>
                          <span class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-semibold {recommendationClass[z.trading_recommendation] ?? ''}">
                            {recommendationText[z.trading_recommendation] ?? z.trading_recommendation}
                          </span>
                          {#if z.overlap_group !== null}
                            <span class="text-muted text-xs">overlap_group #{z.overlap_group}</span>
                          {/if}
                        </div>

                        {#if roleNetScoreConflicts(z)}
                          <p class="text-yellow-400 text-xs mb-3">
                            ⚠ 角色與歷史強弱不一致：目前角色是「{noviceRoleText[effectiveRole(z)] ?? effectiveRole(z)}」，
                            但這個價位帶過去的觸碰歷史比較像「{netScoreLabelText[z.net_score_label] ?? z.net_score_label}」，
                            建議降低信心、多觀察一段時間再判斷。
                          </p>
                        {/if}

                        {#if z.scenario}
                          <div class="mb-3 border border-sky-900/50 rounded-lg p-3 bg-sky-950/20">
                            <div class="flex items-start justify-between gap-3 mb-2">
                              <div>
                                <p class="text-white text-xs font-medium">{z.scenario.title}</p>
                                <p class="text-muted text-xs mt-1 leading-relaxed">{z.scenario.summary}</p>
                              </div>
                              <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] bg-sky-900/50 text-sky-300">
                                {z.scenario.state}
                              </span>
                            </div>
                            <div class="grid md:grid-cols-2 gap-3 text-[11px]">
                              <div>
                                <p class="text-rise font-medium mb-1">觸發條件</p>
                                <div class="space-y-1">
                                  {#each z.scenario.trigger_conditions as item}
                                    <p class="text-muted">{item}</p>
                                  {/each}
                                </div>
                              </div>
                              <div>
                                <p class="text-yellow-300 font-medium mb-1">失效條件</p>
                                <div class="space-y-1">
                                  {#each z.scenario.invalidation_conditions as item}
                                    <p class="text-muted">{item}</p>
                                  {/each}
                                </div>
                              </div>
                            </div>
                          </div>
                        {/if}

                        {#if z.explanation}
                          <div class="mb-3 border border-border/70 rounded-lg p-3 bg-surface/40">
                            <p class="text-white text-xs font-medium mb-2">白話解釋</p>
                            <div class="space-y-1 text-xs">
                              <p class="text-muted leading-relaxed">{z.explanation.score_reason}</p>
                              <p class="text-muted leading-relaxed">{z.explanation.probability_reason}</p>
                              <p class="text-muted leading-relaxed">{z.explanation.confidence_reason}</p>
                            </div>
                            <div class="grid md:grid-cols-3 gap-3 mt-3 text-[11px]">
                              <div>
                                <p class="text-rise font-medium mb-1">加分因素</p>
                                <div class="space-y-1">
                                  {#each z.explanation.positive_factors as item}
                                    <p class="text-muted">{item}</p>
                                  {/each}
                                </div>
                              </div>
                              <div>
                                <p class="text-yellow-300 font-medium mb-1">扣分/風險</p>
                                <div class="space-y-1">
                                  {#each z.explanation.negative_factors as item}
                                    <p class="text-muted">{item}</p>
                                  {/each}
                                </div>
                              </div>
                              <div>
                                <p class="text-white font-medium mb-1">觀察條件</p>
                                <div class="space-y-1">
                                  {#each z.explanation.watch_conditions as item}
                                    <p class="text-muted">{item}</p>
                                  {/each}
                                </div>
                              </div>
                            </div>
                          </div>
                        {/if}

                        {#if activeEvidence(z)?.targets}
                          <div class="mb-3 border border-indigo-900/50 rounded-lg p-3 bg-indigo-950/20">
                            <div class="flex items-center justify-between gap-2 mb-2">
                              <p class="text-white text-xs font-medium">SHAP 局部模型解釋</p>
                              <p class="text-muted text-[11px]">
                                Hold {fmtPct(activeEvidence(z)?.targets.hold.baseline_probability)}
                                → {fmtPct(activeEvidence(z)?.targets.hold.final_probability)}
                              </p>
                            </div>
                            <div class="grid md:grid-cols-2 gap-x-4 gap-y-1">
                              {#each activeEvidence(z)?.targets.hold.contributions.slice(0, 6) ?? [] as contribution}
                                <div class="flex items-center justify-between gap-2 text-[11px]">
                                  <span class="text-muted">{featureLabel[contribution.feature] ?? contribution.feature}</span>
                                  <span class="{signedClass(contribution.contribution)} font-mono">
                                    {fmtSignedPct(contribution.contribution)}
                                  </span>
                                </div>
                              {/each}
                            </div>
                          </div>
                        {/if}

                        <!-- 風險旗標為純規則產出，evidence 降級（未產生 SHAP）時仍要顯示 -->
                        {#if z.evidence?.risk_flags?.length}
                          <p class="text-yellow-300 text-[11px] mb-3">風險旗標：{z.evidence.risk_flags.join('、')}</p>
                        {/if}

                        <!-- 分數列：Support/Resistance/Net Score、Confidence、Trading Score -->
                        <div class="grid grid-cols-2 sm:grid-cols-5 gap-3 text-xs mb-3">
                          <div>
                            <p class="text-muted mb-1">支撐強度分數</p>
                            <p class="text-rise font-mono">{fmtPct(z.support_score)}</p>
                          </div>
                          <div>
                            <p class="text-muted mb-1">壓力強度分數</p>
                            <p class="text-fall font-mono">{fmtPct(z.resistance_score)}</p>
                          </div>
                          <div>
                            <p class="text-muted mb-1">Net Score</p>
                            <p class="{signedClass(z.net_score)} font-mono">{fmtSignedPct(z.net_score)}</p>
                          </div>
                          <div>
                            <p class="text-muted mb-1 flex items-center gap-1">
                              區間辨識信心
                              <span class="inline-flex items-center px-1.5 py-0 rounded-full text-[10px] font-medium {confidenceLevelClass[z.confidence_level] ?? ''}">
                                {confidenceLevelText[z.confidence_level] ?? z.confidence_level}
                              </span>
                            </p>
                            <p class="text-white font-mono">{fmtPct(z.confidence)}</p>
                          </div>
                          <div>
                            <p class="text-muted mb-1">Trading Score</p>
                            <p class="text-white font-mono">{fmtScore100(z.trading_score)} / 100</p>
                          </div>
                        </div>

                        <!-- Trading Score 拆解：EV(34%)+RR(17%)+籌碼(15%)+Trend(12.75%)+Volume(12.75%)+Confidence(8.5%) -->
                        <div class="mb-3">
                          <div class="flex h-2 rounded-full overflow-hidden bg-surface">
                            {#each scoreBreakdownFields as f}
                              <div
                                class="bg-indigo-500 border-r border-panel last:border-r-0"
                                style="width: {(z.trading_score_breakdown[f.key] / (z.trading_score || 1)) * 100}%"
                                title="{f.label}: {z.trading_score_breakdown[f.key].toFixed(1)}"
                              ></div>
                            {/each}
                          </div>
                          <div class="flex flex-wrap gap-x-3 gap-y-0.5 mt-1 text-[11px] text-muted">
                            {#each scoreBreakdownFields as f}
                              <span>{f.label} {z.trading_score_breakdown[f.key].toFixed(1)}<span class="opacity-60">/{f.weight}</span></span>
                            {/each}
                          </div>
                        </div>

                        <!-- 交易數字列：機率、期望報酬、期望值、風險報酬比 -->
                        <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs mb-3">
                          <div>
                            <p class="text-muted mb-1">反彈/守住機率</p>
                            <p class="text-white font-mono">{fmtPct(z.bounce_probability)}</p>
                          </div>
                          <div>
                            <p class="text-muted mb-1">跌破/突破機率</p>
                            <p class="text-white font-mono">{fmtPct(z.break_probability)}</p>
                          </div>
                          <div>
                            <p class="text-muted mb-1">Expected Gain / Loss</p>
                            <p class="font-mono"><span class="text-rise">{fmtSignedPct(z.expected_gain)}</span> / <span class="text-fall">{fmtSignedPct(z.expected_loss)}</span></p>
                          </div>
                          <div>
                            <p class="text-muted mb-1">Expected Value</p>
                            <p class="{signedClass(z.expected_value)} font-mono">{fmtSignedPct(z.expected_value)}</p>
                          </div>
                          <div>
                            <p class="text-muted mb-1">Risk Reward{z.reward_risk_percentile !== null ? ` (${z.reward_risk_percentile.toFixed(0)}百分位)` : ''}</p>
                            <p class="text-white font-mono">{fmtRatio(z.risk_reward_ratio)}</p>
                          </div>
                        </div>

                        {#if z.probability_context}
                          <div class="mb-3 border border-border/70 rounded-lg p-3 bg-surface/30 text-xs">
                            <div class="flex items-start justify-between gap-3 flex-wrap mb-2">
                              <div>
                                <p class="text-white font-medium">機率解讀</p>
                                <p class="text-muted mt-1">
                                  主要結果：{probabilityOutcomeText[z.probability_context.dominant_outcome ?? ''] ?? z.probability_context.dominant_outcome}
                                  {#if z.probability_context.edge_pp !== null && z.probability_context.edge_pp !== undefined}
                                    · 差距 {z.probability_context.edge_pp.toFixed(1)}pp
                                  {/if}
                                </p>
                              </div>
                              {#if z.probability_context.quality_flags && z.probability_context.quality_flags.length > 0}
                                <div class="flex flex-wrap gap-1 justify-end">
                                  {#each z.probability_context.quality_flags as flag}
                                    <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] bg-yellow-900/40 text-yellow-300">
                                      {probabilityFlagLabel(flag)}
                                    </span>
                                  {/each}
                                </div>
                              {/if}
                            </div>
                            <div class="grid grid-cols-3 gap-3">
                              <div>
                                <p class="text-muted mb-1">反彈/守住</p>
                                <p class="text-white font-mono">{fmtPct(z.probability_context.bounce_probability ?? null)}</p>
                              </div>
                              <div>
                                <p class="text-muted mb-1">跌破/突破</p>
                                <p class="text-white font-mono">{fmtPct(z.probability_context.break_probability ?? null)}</p>
                              </div>
                              <div>
                                <p class="text-muted mb-1">盤整/不明確</p>
                                <p class="text-white font-mono">{fmtPct(z.probability_context.neutral_probability ?? null)}</p>
                              </div>
                            </div>
                          </div>
                        {/if}

                        <!-- 量能與驗證狀態列 -->
                        <div class="flex flex-wrap gap-2 mb-3">
                          {#if z.volume_confirmation}
                            <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {volumeConfirmationClass[z.volume_confirmation] ?? ''}">
                              {volumeConfirmationText[z.volume_confirmation] ?? z.volume_confirmation}
                            </span>
                          {/if}
                          <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {recentValidationClass[z.recent_validation] ?? ''}">
                            {recentValidationText[z.recent_validation] ?? z.recent_validation}
                          </span>
                          <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-gray-700/60 {zoneDirectionClass[z.zone_direction] ?? 'text-muted'}">
                            區間動能 {zoneDirectionText[z.zone_direction] ?? z.zone_direction}
                          </span>
                        </div>

                        <!-- 觸碰統計列 -->
                        <div class="grid grid-cols-3 sm:grid-cols-5 gap-3 text-xs text-muted">
                          <div><p class="mb-1">觸碰次數（支撐/壓力）</p><p class="text-white">{z.touch_count}（{z.support_touch_count}/{z.resistance_touch_count}）</p></div>
                          <div><p class="mb-1">拒絕次數</p><p class="text-white">{z.reject_count}</p></div>
                          <div><p class="mb-1">突破次數</p><p class="text-white">{z.break_count}</p></div>
                          <div><p class="mb-1">相對量能</p><p class="text-white">{z.relative_volume === null ? '—' : `${z.relative_volume.toFixed(2)}x`}</p></div>
                          <div><p class="mb-1">區間動能值</p><p class="{signedClass(z.zone_momentum)}">{fmtSignedPct(z.zone_momentum)}</p></div>
                        </div>
                        <p class="text-[11px] text-muted mt-1">區間辨識信心只用目前角色（{noviceRoleText[effectiveRole(z)] ?? effectiveRole(z)}）方向的觸碰樣本計算，不含另一方向；不是反彈勝率</p>
                      </div>
                    {/if}
                  </div>
                {/each}
              </div>
            </div>
          {:else}
            <p class="text-muted text-xs px-5 py-6 text-center">這次分析沒有偵測到任何價格區間</p>
          {/each}
        </div>
        {/if}
      </div>
    {/if}

    <!-- ── 歷史分析紀錄：一進頁面就顯示（未指定股票時顯示所有股票最近紀錄）── -->
    <div class="bg-panel border border-border rounded-xl overflow-hidden">
      <div class="px-5 py-3 border-b border-border flex items-center justify-between">
        <h2 class="text-sm font-semibold text-white">
          {symbol.trim() ? `${symbol} 歷史分析紀錄` : '最近分析紀錄（所有股票）'}
        </h2>
        {#if symbol.trim()}
          <button
            class="text-xs text-muted hover:text-white transition-colors"
            on:click={() => { symbol = ''; loadHistory() }}
          >清除篩選</button>
        {/if}
      </div>
      <table class="w-full text-sm">
        <thead>
          <tr class="text-muted text-xs border-b border-border">
            <th class="text-left px-5 py-2">股票</th>
            <th class="text-left px-3 py-2">分析時間</th>
            <th class="text-right px-3 py-2">現價</th>
            <th class="text-right px-5 py-2">操作</th>
          </tr>
        </thead>
        <tbody>
          {#if historyLoading}
            <tr><td colspan="4" class="px-5 py-6 text-center text-muted">載入中...</td></tr>
          {:else if history.length === 0}
            <tr><td colspan="4" class="px-5 py-6 text-center text-muted">尚無歷史紀錄，輸入股票代號分析看看</td></tr>
          {:else}
            {#each history as h (h.id)}
              {#if confirmDeleteId === h.id}
                <tr class="border-b border-border/50 bg-red-900/20">
                  <td class="px-5 py-2 text-xs text-gray-300" colspan="2">
                    確定刪除 <span class="font-semibold text-white">{h.symbol}（{formatDateTime(h.analyzed_at)}）</span> 這筆分析嗎？
                  </td>
                  <td class="px-5 py-2 text-right" colspan="2">
                    <div class="flex gap-2 justify-end">
                      <button
                        class="text-xs px-2.5 py-1 border border-border text-muted hover:text-white rounded transition-colors"
                        on:click={() => (confirmDeleteId = null)}
                      >取消</button>
                      <button
                        class="text-xs px-2.5 py-1 bg-red-600 hover:bg-red-500 text-white rounded transition-colors disabled:opacity-50"
                        disabled={deletingId === h.id}
                        on:click={() => doDelete(h.id)}
                      >{deletingId === h.id ? '刪除中...' : '刪除'}</button>
                    </div>
                  </td>
                </tr>
              {:else}
                <tr
                  class="border-b border-border/50 hover:bg-border/20 cursor-pointer transition-colors group
                         {current?.id === h.id ? 'bg-indigo-900/20' : ''}"
                  on:click={() => selectHistory(h)}
                >
                  <td class="px-5 py-2 text-white font-medium">{h.symbol}</td>
                  <td class="px-3 py-2 text-muted text-xs font-mono">{formatDateTime(h.analyzed_at)}</td>
                  <td class="px-3 py-2 text-right font-mono">{fmt(h.current_price)}</td>
                  <td class="px-5 py-2 text-right">
                    <div class="flex gap-2 justify-end">
                      <button
                        class="text-xs px-2.5 py-1 border border-fall/40 text-fall hover:bg-fall/10 rounded transition-colors"
                        on:click|stopPropagation={() => (confirmDeleteId = h.id)}
                      >刪除</button>
                    </div>
                  </td>
                </tr>
              {/if}
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  </div>
</Layout>
<style>
  :global(.animate-marquee) {
    display: inline-block;
    min-width: 100%;
    animation: sr-tip-marquee 14s linear infinite;
  }

  @keyframes sr-tip-marquee {
    0% { transform: translateX(0); }
    20% { transform: translateX(0); }
    100% { transform: translateX(-35%); }
  }
</style>
