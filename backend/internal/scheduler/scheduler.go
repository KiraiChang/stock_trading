package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/chip"
	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// defaultCorporateActionCron 是公司行動同步的預設排程（平日 06:30，台北時區）。
// 與 config 的 SetDefault 同值；兩邊都留是為了讓不經過 config.Load() 的呼叫端
// 也有可用的預設（見 corporateActionCron）。
const defaultCorporateActionCron = "30 6 * * 1-5"

// srZoneVerifyLimit 每次收盤驗證最多處理幾筆最近的 SR zone 分析，避免隨著
// 歷史分析越積越多，這個 job 的執行時間跟著無上限成長（見 RunDailyClose）。
const srZoneVerifyLimit = 50

// stockSymbolSyncTimeout 是整個 stock_symbol_sync job 的上限（抓取兩個來源 + 寫入快照）。
// 單次 HTTP 請求另有 client timeout（預設 300 秒／來源），這層是最後防線：來源異常慢或
// DB 卡住時，job 不會無限期停在 running（job_runs 的 stale 判定要 26 小時才會亮）。
// 20 分鐘 = 2 來源 × 300 秒 + 來源間隔 + 寫入快照，仍有餘裕。
const stockSymbolSyncTimeout = 20 * time.Minute

// defaultCorporateActionTimeout 是整輪公司行動同步的預設預算（config 沒給時的退路）。
//
// **45 分鐘是算出來的，不是拍的**（推導見 docs/architecture.md 的公司行動同步段）：逐檔同步的節奏由 FinMind 的
// 5 req/min 決定（每檔約 12 秒），當日名單 ≈ watchlist 11 檔 ＋ 每片約 170 檔 = 181 檔，
// 181 × 12 秒 ≈ 36 分鐘。舊值 10 分鐘只跑得完約 50 檔，正是覆蓋率破洞的直接成因。
const defaultCorporateActionTimeout = 45 * time.Minute

// defaultCorporateActionShardCount 是非 watchlist 標的的預設分片數。
// 5 片對應週一到週五，每檔**每週至少覆蓋一次**。
const defaultCorporateActionShardCount = 5

type Scheduler struct {
	fetcher          *market.Fetcher
	signalEng        *signal.Engine
	watchlist        store.WatchlistRepo
	jobRuns          store.JobRunRepo
	srZoneRepo       store.SRZoneRepo
	srZoneVerifier   *analysis.SRZoneVerifier
	chipSyncer       *chip.Syncer
	chipSyncCron     string
	stockSyncer      *market.StockSymbolSyncer
	stockSyncCron    string
	stockSyncEnabled bool
	analysisClient   *analysis.Client
	srEvaluationJobs store.SREvaluationJobRepo
	chipScores       store.ChipScoreRepo
	modelGovernance  store.SRModelGovernanceRepo
	srEvaluation     config.SREvaluationConfig
	intradayEnabled  bool
	// adjuster 為選填（見 docs/todo.md T-042）：未注入時不註冊還原係數同步排程，
	// 行為與導入前完全相同。
	adjuster *market.Adjuster
	// corporateActionCfg 只有 cron。cron 為空時退回 defaultCorporateActionCron——
	// config 沒設不該讓這支排程消失（漏跑會讓該檔整段歷史出現假跳空）。
	corporateActionCfg config.CorporateActionConfig
	// evaluationUniverse 為選填（見 docs/todo.md T-040 Step 5）：未注入或未啟用時
	// 不註冊該排程，行為與導入前完全相同。比照 adjuster 的處理。
	evaluationUniverse    store.EvaluationUniverseRepo
	evaluationUniverseCfg config.EvaluationUniverseConfig
	// universeSyncRunning 擋重複觸發。這個 job 要跑約 26 分鐘，cron 與人工觸發撞在一起
	// 會讓兩批請求共用同一個節流器互相拖慢，且 job_runs 出現兩筆難以判讀的紀錄。
	// **行程內旗標而非查 job_runs**：目前是單一 backend 實例，DB 層檢查要多一個 repo 方法
	// 卻只在多實例部署才有意義。
	universeSyncRunning atomic.Bool
	// srAnalysisRunner 為選填（todo.md T-052）：未注入或未啟用時不註冊排程，
	// 行為與導入前完全相同（比照 adjuster / evaluationUniverse）。
	//
	// **介面由這裡定義、由 main 注入 handler**：身分追蹤只存在於
	// `api/handler.SRZoneHandler`，而 `api/handler` 已經 import 本套件
	// （SchedulerHandler），反向 import 會是 cycle。介面由消費端宣告是 Go 的慣例，
	// 也讓「身分追蹤只有一份實作」這條不變式不必靠紀律維持。
	srAnalysisRunner SRAnalysisRunner
	srAnalysisCfg    config.SRAnalysisConfig
	// srAnalysisCandles 用來檢查「今天的 K 棒到了沒」。
	srAnalysisCandles store.CandleRepo
	// 兩個時段各一個併發旗標：17:00 那輪還在跑時 22:00 不該被它擋掉。
	srAnalysisRunning     atomic.Bool
	srAnalysisChipRunning atomic.Bool
	// registeredJobs 記錄 Start() 實際註冊了哪些 cron job。
	// **為什麼要記而不是讓呼叫端自己判斷**：註冊條件散在 Start() 各處
	// （config 開關、adjuster 是否注入、repo 是否注入），複製一份到 API 層
	// 遲早會與這裡不一致。由 scheduler 自己回報才有單一事實來源。
	//
	// **必須上鎖**：main.go 是 `go sched.Start()` 與 HTTP server 並行啟動的，
	// Start() 寫這個 map 的同時 /scheduler/status 可能正在讀。Go 的 map 不支援
	// 並發讀寫，撞上會是 `fatal error: concurrent map read and map write`——
	// 不可 recover，整個行程死掉。
	registeredJobsMu sync.RWMutex
	registeredJobs   map[string]bool
	log              *zap.Logger
	cron             *cron.Cron
}

// SRAnalysisRunner 是「跑一次帶身分追蹤的 SR zone 分析」。實作是
// `api/handler.SRZoneHandler.RunAnalysis`——`POST /sr-zones` 走的也是它。
//
// **不要在這裡另外實作一份。** `analysis.SRAnalysisProvider`（reuse_existing=true 那條）
// 不寫 zone_uid、不追身分，用它產生的分析在身分層完全沒有紀錄，而且不會報錯。
type SRAnalysisRunner interface {
	RunAnalysis(ctx context.Context, symbol, timeframe string, limit int) (uint64, error)
}

func New(
	fetcher *market.Fetcher,
	signalEng *signal.Engine,
	watchlist store.WatchlistRepo,
	jobRuns store.JobRunRepo,
	srZoneRepo store.SRZoneRepo,
	srZoneVerifier *analysis.SRZoneVerifier,
	chipSyncer *chip.Syncer,
	chipSyncCron string,
	stockSyncer *market.StockSymbolSyncer,
	stockSyncCron string,
	stockSyncEnabled bool,
	analysisClient *analysis.Client,
	srEvaluationJobs store.SREvaluationJobRepo,
	chipScores store.ChipScoreRepo,
	modelGovernance store.SRModelGovernanceRepo,
	srEvaluation config.SREvaluationConfig,
	intradayEnabled bool,
	log *zap.Logger,
) *Scheduler {
	return &Scheduler{
		fetcher:          fetcher,
		signalEng:        signalEng,
		watchlist:        watchlist,
		jobRuns:          jobRuns,
		srZoneRepo:       srZoneRepo,
		srZoneVerifier:   srZoneVerifier,
		chipSyncer:       chipSyncer,
		chipSyncCron:     chipSyncCron,
		stockSyncer:      stockSyncer,
		stockSyncCron:    stockSyncCron,
		stockSyncEnabled: stockSyncEnabled,
		analysisClient:   analysisClient,
		srEvaluationJobs: srEvaluationJobs,
		chipScores:       chipScores,
		modelGovernance:  modelGovernance,
		srEvaluation:     srEvaluation,
		intradayEnabled:  intradayEnabled,
		log:              log,
		cron:             cron.New(cron.WithLocation(timeutil.TaipeiTZ)),
		registeredJobs:   map[string]bool{},
	}
}

func (s *Scheduler) Start() {
	// 盤前初始化：補齊近 5 天日K 缺口 + 預熱日線指標
	s.cron.AddFunc("50 8 * * 1-5", func() {
		s.runPreMarket()
	})
	s.markRegistered("pre_market")

	// 盤中：每 5 分鐘拉取分K + 計算指標 + Signal 掃描（IsMarketOpen 守衛 13:30 收盤）
	s.cron.AddFunc("*/5 9-13 * * 1-5", func() {
		s.runIntradayJob()
	})
	s.markRegistered("intraday")

	// 收盤後：拉日K + 完整掃描。收盤是 13:30，這裡刻意等到 15:00 才拉，
	// 是因為 FinMind TaiwanStockPrice 當天日K不會在收盤當下就立刻發布——
	// 曾經在 14:00 整拉到 count=0（請求成功但資料還沒發布，BulkInsert 對
	// 空陣列直接視為成功，job_runs 也顯示 success，不會有任何錯誤訊號）。
	// 15:00 給 FinMind 更多緩衝時間，仍抓空的話可用 RunDailyClose 手動重拉
	// （見 handler.SchedulerHandler.RunDailyClose）。
	s.cron.AddFunc("0 15 * * 1-5", func() {
		s.RunDailyClose()
	})
	s.markRegistered("daily_close")
	// sr_zone_verify 沒有自己的 cron——它在 RunDailyClose 尾端無條件執行，
	// 所以「有沒有排程會觸發它」等同於 daily_close。
	s.markRegistered("sr_zone_verify")

	// 籌碼採集：與 15:00 收盤掃描解耦，改為傍晚獨立排程（預設 21:00，見
	// config chip.sync.cron）。FinMind 法人資料收盤後傍晚、融資融券更要晚間
	// 才由 TWSE 發布，若沿用 15:00 會抓到空資料、只能靠隔天 lookback 回補，
	// 造成資料庫永遠落後一天（見 docs/chip-analysis-design.md 第8節）。
	if _, err := s.cron.AddFunc(s.chipSyncCron, func() {
		s.runChipDailySync(context.Background())
	}); err != nil {
		s.log.Error("chip sync cron register failed", zap.String("cron", s.chipSyncCron), zap.Error(err))
	} else {
		s.markRegistered("chip_daily_sync")
	}

	if s.stockSyncEnabled && s.stockSyncer != nil {
		if _, err := s.cron.AddFunc(s.stockSyncCron, func() {
			s.RunStockSymbolSync()
		}); err != nil {
			s.log.Error("stock symbol sync cron register failed", zap.String("cron", s.stockSyncCron), zap.Error(err))
		} else {
			s.markRegistered("stock_symbol_sync")
		}
	}

	// 公司行動同步：分割罕見（全市場 11 年只有 33 筆），但漏掉一次就會讓該檔的整段歷史
	// 出現假跳空，所以每天跑一次。重算是冪等的，重複執行不會累積誤差。
	if s.adjuster != nil {
		cronSpec := s.corporateActionCron()
		if _, err := s.cron.AddFunc(cronSpec, func() {
			s.RunCorporateActionSync()
		}); err != nil {
			s.log.Error("corporate action sync cron register failed",
				zap.String("cron", cronSpec), zap.Error(err))
		} else {
			s.markRegistered("corporate_action_sync")
		}
	}

	// 評估標的池的每日日 K 維護。**只做這一件事**——不進盤中、不抓籌碼、不做 SR 分析，
	// 那是 T-040「新標的不能放進 watchlists」的核心約束。
	if s.evaluationUniverse != nil && s.evaluationUniverseCfg.Enabled {
		if _, err := s.cron.AddFunc(s.evaluationUniverseCfg.Cron, func() {
			s.runEvaluationUniverseSync(context.Background())
		}); err != nil {
			s.log.Error("evaluation universe cron register failed",
				zap.String("cron", s.evaluationUniverseCfg.Cron), zap.Error(err))
		} else {
			s.markRegistered("evaluation_universe_sync")
		}
	}

	// SR 分析排程（T-052）：兩輪都跑同一份 runner，差別只在「要不要求當日籌碼」。
	if s.srAnalysisRunner != nil && s.srAnalysisCfg.Enabled {
		if _, err := s.cron.AddFunc(s.srAnalysisCfg.Cron, func() {
			s.runSRAnalysis(context.Background(), false)
		}); err != nil {
			s.log.Error("sr analysis cron register failed",
				zap.String("cron", s.srAnalysisCfg.Cron), zap.Error(err))
		} else {
			s.markRegistered("sr_analysis")
		}
		if _, err := s.cron.AddFunc(s.srAnalysisCfg.ChipCron, func() {
			s.runSRAnalysis(context.Background(), true)
		}); err != nil {
			s.log.Error("sr analysis chip cron register failed",
				zap.String("cron", s.srAnalysisCfg.ChipCron), zap.Error(err))
		} else {
			s.markRegistered("sr_analysis_chip")
		}
	}

	if s.srEvaluation.Enabled {
		if _, err := s.cron.AddFunc(s.srEvaluation.Cron, func() {
			s.RunSREvaluation()
		}); err != nil {
			s.log.Error("sr evaluation cron register failed", zap.String("cron", s.srEvaluation.Cron), zap.Error(err))
		} else {
			s.markRegistered("sr_evaluation")
		}
	}

	s.cron.Start()
	s.log.Info("scheduler started")
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

// startRun 記錄一筆排程執行紀錄，失敗時只記 log（不影響排程本身執行）
func (s *Scheduler) startRun(ctx context.Context, jobName string) uint64 {
	runID, err := s.jobRuns.Start(ctx, jobName)
	if err != nil {
		s.log.Error("job_runs start failed", zap.String("job", jobName), zap.Error(err))
	}
	return runID
}

// finishRunWriteTimeout 是「寫回結束狀態」這一步自己的預算。只是一次 UPDATE，
// 短即可；它存在的目的是讓 finishRun 不會因為沒有上界而卡住排程 goroutine。
const finishRunWriteTimeout = 10 * time.Second

// finishRun 依失敗數量換算 status 並寫回執行紀錄。
//
// **刻意不沿用呼叫端的 ctx**（2026-08-24 起；語意見 docs/api-reference.md 的 /scheduler/status）：job 的 ctx
// 逾時之後，用它去寫 `job_runs` 一定會失敗，那筆紀錄就**永遠卡在 `running`**——
// 看起來像還在跑，實際早就結束了。這條**不只影響 corporate_action_sync**：任何 job
// 的 ctx 逾時都會踩到。用 context.WithoutCancel 切斷取消訊號、只保留 value，
// 再套自己的短 timeout，才能保證「跑失敗」與「沒回報」在資料上分得開。
//
// total / failed 的單位是**標的數**（欄位就叫 `symbols_total` / `symbols_failed`），
// 呼叫端不要傳事件筆數——單位混用會讓下面的 `failed >= total` 判定失準。
func (s *Scheduler) finishRun(ctx context.Context, runID uint64, jobName string, total, failed int, lastErr string) {
	s.finishRunDegraded(ctx, runID, jobName, total, failed, lastErr, false)
}

// finishRunDegraded 與 finishRun 相同，但多一個 degraded 旗標：**這輪跑完了、名單內的標的
// 也可能零失敗，但名單本身就不完整**——該同步的一批根本沒進去。
//
// 這種情況照 total/failed 推導會得到 `success`，而那是假的：數字只涵蓋得到「有跑的那些」，
// 沒進名單的標的不在分母裡，所以再怎麼算都不會失敗。degraded 為真時至少記 `partial`，
// 讓「跑得不完整」在 job_runs 上看得見（corporate_action_sync 讀不到 watchlist 時就是這樣）。
// 已經是 failed 的不會被降級——degraded 只補強，不會讓狀態變樂觀。
func (s *Scheduler) finishRunDegraded(ctx context.Context, runID uint64, jobName string, total, failed int, lastErr string, degraded bool) {
	status := "success"
	switch {
	case total > 0 && failed >= total:
		status = "failed"
	case failed > 0:
		status = "partial"
	}
	if degraded && status == "success" {
		status = "partial"
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finishRunWriteTimeout)
	defer cancel()
	if err := s.jobRuns.Finish(writeCtx, runID, status, total, failed, lastErr); err != nil {
		s.log.Error("job_runs finish failed", zap.String("job", jobName), zap.Error(err))
	}
}

func (s *Scheduler) runPreMarket() {
	ctx := context.Background()

	// 只保留當天的排程執行紀錄，開盤前先清掉前幾天的舊資料
	if n, err := s.jobRuns.DeleteBefore(ctx, timeutil.TodayTaipei()); err != nil {
		s.log.Warn("job_runs cleanup failed", zap.Error(err))
	} else if n > 0 {
		s.log.Info("job_runs cleanup done", zap.Int64("deleted", n))
	}

	runID := s.startRun(ctx, "pre_market")
	s.log.Info("pre-market job started")

	symbols, err := s.watchlist.Symbols(ctx)
	if err != nil {
		s.log.Error("watchlist fetch failed", zap.Error(err))
		s.finishRun(ctx, runID, "pre_market", 0, 0, err.Error())
		return
	}

	// 補齊近 5 天日K（涵蓋週末 / 假日缺口），BulkInsert 有 UNIQUE 保護不會重複
	failed := s.fetcher.BackfillHistory(ctx, symbols, 5, nil)

	// 預熱日線指標，讓第一根分K掃描前就有 MA / RSI / MACD 基準值
	for _, sym := range symbols {
		s.signalEng.Evaluate(ctx, sym, "1d")
	}
	s.log.Info("pre-market job completed", zap.Int("symbols", len(symbols)), zap.Int("failed", failed))

	lastErr := ""
	if failed > 0 {
		lastErr = "backfill failed for some symbols"
	}
	s.finishRun(ctx, runID, "pre_market", len(symbols), failed, lastErr)
}

func (s *Scheduler) runIntradayJob() {
	if !timeutil.IsMarketOpen(time.Now()) {
		return
	}
	// 有掛載批次盤中源（Yahoo）時優先走批次路徑（免 token）；未掛載才退回 FinMind 分K。
	if s.fetcher.HasIntradaySource() {
		s.runIntradayBatch()
		return
	}
	if !s.intradayEnabled {
		// finmind.intraday_enabled=false（預設）且無批次源：帳號等級不足以使用
		// TaiwanStockKBar dataset，不建立 job_run 紀錄，避免每 5 分鐘
		// 洗一筆「skipped」進資料庫；升級帳號或啟用 Yahoo 後即可恢復
		s.log.Debug("intraday job skipped: no batch source and finmind.intraday_enabled=false")
		return
	}

	ctx := context.Background()
	runID := s.startRun(ctx, "intraday")

	symbols, err := s.watchlist.Symbols(ctx)
	if err != nil {
		s.log.Error("watchlist fetch failed", zap.Error(err))
		s.finishRun(ctx, runID, "intraday", 0, 0, err.Error())
		return
	}

	today := timeutil.TodayTaipei()
	failed := 0
	lastErr := ""
	for i, sym := range symbols {
		if err := s.fetcher.FetchAndStoreMinute(ctx, sym, today); err != nil {
			if errors.Is(err, market.ErrInsufficientTier) {
				// 帳號等級不足是整個 token 的限制，對其他 symbol 重試也一定會失敗，
				// 記一次 log 後整輪跳過，避免每 5 分鐘對 watchlist 每檔股票都打一次注定失敗的請求
				s.log.Warn("intraday job skipped: finmind token tier insufficient", zap.Error(err))
				if ferr := s.jobRuns.Finish(ctx, runID, "skipped", len(symbols), len(symbols)-i, err.Error()); ferr != nil {
					s.log.Error("job_runs finish failed", zap.String("job", "intraday"), zap.Error(ferr))
				}
				return
			}
			s.log.Warn("intraday fetch failed", zap.String("symbol", sym), zap.Error(err))
			failed++
			lastErr = err.Error()
		}
		s.signalEng.Evaluate(ctx, sym, "1m")
	}
	s.finishRun(ctx, runID, "intraday", len(symbols), failed, lastErr)
}

// runIntradayBatch 為批次盤中源（Yahoo）版本的盤中 job：把 watchlist 依 batch_size
// 分批、每批一次請求拉多檔 1 分K 並寫入 candles，再逐檔跑 signal 掃描。
func (s *Scheduler) runIntradayBatch() {
	ctx := context.Background()
	runID := s.startRun(ctx, "intraday")

	symbols, err := s.watchlist.Symbols(ctx)
	if err != nil {
		s.log.Error("watchlist fetch failed", zap.Error(err))
		s.finishRun(ctx, runID, "intraday", 0, 0, err.Error())
		return
	}

	batchSize := s.fetcher.IntradayBatchSize()
	if batchSize <= 0 {
		batchSize = len(symbols)
	}

	failed := 0
	lastErr := ""
	for start := 0; start < len(symbols); start += batchSize {
		end := start + batchSize
		if end > len(symbols) {
			end = len(symbols)
		}
		batch := symbols[start:end]
		if _, err := s.fetcher.FetchAndStoreIntradayBatch(ctx, batch); err != nil {
			// 批次請求失敗（例如 Yahoo 被限流/封鎖）：記錄後續跑其他批次。
			// 未來的 Yahoo→FinMind fallback（T-008/T-031）會在此改為回退補資料。
			s.log.Warn("intraday batch fetch failed", zap.Int("from", start), zap.Int("size", len(batch)), zap.Error(err))
			failed += len(batch)
			lastErr = err.Error()
		}
	}

	for _, sym := range symbols {
		s.signalEng.Evaluate(ctx, sym, "1m")
	}
	s.finishRun(ctx, runID, "intraday", len(symbols), failed, lastErr)
}

// RunDailyClose 執行「收盤後拉日K + 完整掃描」的邏輯，供 cron 排程與
// 人工手動觸發（例如排程時間點 FinMind 剛好還沒發布資料時）共用同一份實作。
func (s *Scheduler) RunDailyClose() {
	ctx := context.Background()
	runID := s.startRun(ctx, "daily_close")

	symbols, err := s.watchlist.Symbols(ctx)
	if err != nil {
		s.log.Error("watchlist fetch failed", zap.Error(err))
		s.finishRun(ctx, runID, "daily_close", 0, 0, err.Error())
		return
	}

	today := timeutil.TodayTaipei()
	failed := 0
	lastErr := ""
	for _, sym := range symbols {
		if err := s.fetcher.FetchAndStoreDaily(ctx, sym, today); err != nil {
			s.log.Warn("daily fetch failed", zap.String("symbol", sym), zap.Error(err))
			failed++
			lastErr = err.Error()
		}
		s.signalEng.Evaluate(ctx, sym, "1d")
	}
	s.log.Info("daily close job completed", zap.Int("symbols", len(symbols)), zap.Int("failed", failed))
	s.finishRun(ctx, runID, "daily_close", len(symbols), failed, lastErr)

	// SR zone 驗證是獨立的 job_run 紀錄，失敗不影響上面已經完成的 daily_close
	// 結果——兩者依序執行但彼此獨立記錄，其中一個出問題不會讓另一個也跟著
	// 判定失敗。
	s.runSRZoneVerification(ctx)

	// 註：籌碼同步（chip_daily_sync）已從此處解耦，改由傍晚獨立 cron 觸發
	// （見 Start() 的 s.chipSyncCron）。法人/融資融券資料比日K更晚發布，
	// 15:00 收盤這批太早，沿用會抓到空資料造成落後一天。
}

// runChipDailySync 對 watchlist 全部股票執行當日籌碼同步（chip.Syncer.SyncDaily），
// 完全比照 runSRZoneVerification 的結構：單筆失敗只記錄、不中斷其他股票的同步。
func (s *Scheduler) runChipDailySync(ctx context.Context) {
	runID := s.startRun(ctx, "chip_daily_sync")

	symbols, err := s.watchlist.Symbols(ctx)
	if err != nil {
		s.log.Error("watchlist fetch failed", zap.Error(err))
		s.finishRun(ctx, runID, "chip_daily_sync", 0, 0, err.Error())
		return
	}

	today := timeutil.TodayTaipei()
	failed := 0
	lastErr := ""
	for _, sym := range symbols {
		if err := s.chipSyncer.SyncDaily(ctx, sym, today); err != nil {
			s.log.Warn("chip daily sync failed", zap.String("symbol", sym), zap.Error(err))
			failed++
			lastErr = err.Error()
		}
	}
	s.log.Info("chip daily sync job completed", zap.Int("symbols", len(symbols)), zap.Int("failed", failed))
	s.finishRun(ctx, runID, "chip_daily_sync", len(symbols), failed, lastErr)
}

func (s *Scheduler) RunStockSymbolSync() {
	s.runStockSymbolSync(context.Background())
}

func (s *Scheduler) runStockSymbolSync(ctx context.Context) {
	runID := s.startRun(ctx, "stock_symbol_sync")
	if s.stockSyncer == nil {
		errMsg := "stock symbol syncer is not configured"
		s.log.Warn(errMsg)
		s.finishRun(ctx, runID, "stock_symbol_sync", 0, 1, errMsg)
		return
	}

	// 只有同步本身套 timeout；startRun / finishRun 仍用原本的 ctx，
	// 否則 sync 逾時後連「把這次 job 標記成失敗」的 DB 寫入都會一起被取消。
	syncCtx, cancel := context.WithTimeout(ctx, stockSymbolSyncTimeout)
	defer cancel()

	result, err := s.stockSyncer.Sync(syncCtx, time.Now().In(timeutil.TaipeiTZ))
	if err != nil {
		s.log.Error("stock symbol sync failed", zap.Error(err))
		s.finishRun(ctx, runID, "stock_symbol_sync", 0, 1, err.Error())
		return
	}
	s.finishRun(ctx, runID, "stock_symbol_sync", result.Seen, 0, "")
}

// runSRZoneVerification 對最近 srZoneVerifyLimit 筆 SR zone 分析重新驗證
// zone 有沒有被突破（見 internal/analysis/sr_zone_verifier.go）。跟
// indicator/signal 排程一樣，單筆驗證失敗只記錄、不中斷其他分析的驗證。
func (s *Scheduler) runSRZoneVerification(ctx context.Context) {
	runID := s.startRun(ctx, "sr_zone_verify")

	analyses, err := s.srZoneRepo.List(ctx, "", srZoneVerifyLimit)
	if err != nil {
		s.log.Error("sr zone list failed", zap.Error(err))
		s.finishRun(ctx, runID, "sr_zone_verify", 0, 0, err.Error())
		return
	}

	failed := 0
	lastErr := ""
	for _, a := range analyses {
		if _, _, err := s.srZoneVerifier.Verify(ctx, a.ID); err != nil {
			s.log.Warn("sr zone verify failed", zap.Uint64("analysis_id", a.ID), zap.String("symbol", a.Symbol), zap.Error(err))
			failed++
			lastErr = err.Error()
		}
	}
	s.log.Info("sr zone verification job completed", zap.Int("analyses", len(analyses)), zap.Int("failed", failed))
	s.finishRun(ctx, runID, "sr_zone_verify", len(analyses), failed, lastErr)
}

func (s *Scheduler) RunSREvaluation() {
	s.runSREvaluation(context.Background())
}

func (s *Scheduler) runSREvaluation(ctx context.Context) {
	runID := s.startRun(ctx, "sr_evaluation")

	symbols, err := s.srEvaluationSymbols(ctx)
	if err != nil {
		s.log.Error("sr evaluation symbols failed", zap.Error(err))
		s.finishRun(ctx, runID, "sr_evaluation", 1, 1, err.Error())
		return
	}
	if len(symbols) == 0 {
		errMsg := "sr evaluation symbols is empty"
		s.log.Warn(errMsg)
		s.finishRun(ctx, runID, "sr_evaluation", 1, 1, errMsg)
		return
	}
	if s.analysisClient == nil {
		errMsg := "sr evaluation analysis client is not configured"
		s.log.Warn(errMsg)
		s.finishRun(ctx, runID, "sr_evaluation", len(symbols), len(symbols), errMsg)
		return
	}
	if s.srEvaluationJobs == nil {
		errMsg := "sr evaluation job repo is not configured"
		s.log.Warn(errMsg)
		s.finishRun(ctx, runID, "sr_evaluation", len(symbols), len(symbols), errMsg)
		return
	}

	request := s.srEvaluationRequest(symbols)
	analysis.PopulateSREvaluationReplayContext(ctx, &request, s.chipScores, s.modelGovernance, s.log)

	symbolsJSON, err := json.Marshal(symbols)
	if err != nil {
		s.log.Error("sr evaluation symbols marshal failed", zap.Error(err))
		s.finishRun(ctx, runID, "sr_evaluation", len(symbols), len(symbols), err.Error())
		return
	}

	jobID := analysis.NewEvaluationJobID()
	mode := "evaluation"
	if request.DecisionReplay {
		mode = "decision_replay"
	}
	if _, err := s.srEvaluationJobs.Create(ctx, &store.SREvaluationJob{
		JobID:         jobID,
		Status:        "pending",
		Symbols:       string(symbolsJSON),
		Timeframe:     request.Timeframe,
		FetchLimit:    request.Limit,
		Mode:          mode,
		WriteDB:       request.WriteDB,
		ReplayMaxRows: request.ReplayMaxRows,
	}); err != nil {
		s.log.Error("sr evaluation job create failed", zap.String("job_id", jobID), zap.Error(err))
		s.finishRun(ctx, runID, "sr_evaluation", len(symbols), len(symbols), err.Error())
		return
	}
	if err := s.srEvaluationJobs.MarkRunning(ctx, jobID); err != nil {
		s.log.Error("sr evaluation job mark running failed", zap.String("job_id", jobID), zap.Error(err))
	}

	report, err := s.analysisClient.RunSREvaluation(ctx, request)
	if err != nil {
		s.log.Error("sr evaluation failed", zap.String("job_id", jobID), zap.Error(err))
		if markErr := s.srEvaluationJobs.MarkFailed(ctx, jobID, err.Error()); markErr != nil {
			s.log.Error("sr evaluation job mark failed failed", zap.String("job_id", jobID), zap.Error(markErr))
		}
		s.finishRun(ctx, runID, "sr_evaluation", len(symbols), len(symbols), err.Error())
		return
	}

	reportJSON, err := json.Marshal(report)
	if err != nil {
		s.log.Error("sr evaluation report marshal failed", zap.String("job_id", jobID), zap.Error(err))
		reportJSON = []byte("null")
	}
	if err := s.srEvaluationJobs.MarkDone(
		ctx,
		jobID,
		store.RawJSON(reportJSON),
		analysis.StringFromReport(report, "run_id"),
		analysis.StringFromReport(report, "schema_version"),
		analysis.StringFromReport(report, "pipeline_version"),
		analysis.IntFromReport(report, "rows"),
		analysis.IntFromReport(report, "sources"),
	); err != nil {
		s.log.Error("sr evaluation job mark done failed", zap.String("job_id", jobID), zap.Error(err))
		s.finishRun(ctx, runID, "sr_evaluation", len(symbols), len(symbols), err.Error())
		return
	}

	s.log.Info("sr evaluation job completed", zap.String("job_id", jobID), zap.Int("symbols", len(symbols)))
	s.finishRun(ctx, runID, "sr_evaluation", len(symbols), 0, "")
}

func (s *Scheduler) srEvaluationSymbols(ctx context.Context) ([]string, error) {
	if len(s.srEvaluation.Symbols) > 0 {
		symbols := make([]string, 0, len(s.srEvaluation.Symbols))
		for _, symbol := range s.srEvaluation.Symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" {
				symbols = append(symbols, symbol)
			}
		}
		return symbols, nil
	}
	return s.watchlist.Symbols(ctx)
}

func (s *Scheduler) srEvaluationRequest(symbols []string) analysis.SREvaluationRequest {
	timeframe := strings.TrimSpace(s.srEvaluation.Timeframe)
	if timeframe == "" {
		timeframe = "1d"
	}
	limit := s.srEvaluation.Limit
	if limit <= 0 {
		limit = 1500
	}
	replayMaxRows := s.srEvaluation.ReplayMaxRows
	if s.srEvaluation.DecisionReplay && replayMaxRows <= 0 {
		replayMaxRows = 200
	}
	if !s.srEvaluation.DecisionReplay {
		replayMaxRows = 0
	}
	return analysis.SREvaluationRequest{
		Symbols:        symbols,
		Timeframe:      timeframe,
		Limit:          limit,
		WriteDB:        s.srEvaluation.WriteDB,
		DecisionReplay: s.srEvaluation.DecisionReplay,
		ReplayMaxRows:  replayMaxRows,
	}
}

// IsJobRegistered 回報 Start() 是否真的註冊了這個 cron job。
//
// **註冊失敗也算沒註冊**：cron 字串打錯時 AddFunc 只記 log 不中止，
// 那種情況與「刻意關閉」在行為上相同（都不會跑），但成因完全不同——
// 呼叫端要能分辨「沒開」與「該開卻沒開」，前者看這個回傳、後者看啟動 log。
func (s *Scheduler) IsJobRegistered(name string) bool {
	s.registeredJobsMu.RLock()
	defer s.registeredJobsMu.RUnlock()
	return s.registeredJobs[name]
}

// markRegistered 由 Start() 呼叫，是唯一的寫入點。
func (s *Scheduler) markRegistered(name string) {
	s.registeredJobsMu.Lock()
	defer s.registeredJobsMu.Unlock()
	s.registeredJobs[name] = true
}

// SetAdjuster 注入還原係數同步器與其排程設定。未呼叫時排程不註冊該 job。
// 比照 SetEvaluationUniverse：依賴與設定一起注入，避免兩者分開設定時漏掉其一。
func (s *Scheduler) SetAdjuster(a *market.Adjuster, cfg config.CorporateActionConfig) {
	s.adjuster = a
	s.corporateActionCfg = cfg
}

// corporateActionCron 取設定值，**空白或無法解析時都退回預設**。
//
// **為什麼不讓壞字串直接傳給 AddFunc**：那會註冊失敗（只記 log 不中止），
// 於是「config 打錯一個字」的後果是這支排程靜默消失，而它漏跑一次就會讓該檔的整段
// 歷史出現假跳空。viper 那層已經有 SetDefault，這裡是第二道防線——
// 測試或其他呼叫端不經過 config.Load() 時同樣要拿得到可用的預設。
//
// **為什麼這支特別要擋，其他排程不用**：其他三支 cron 走 config 的排程
// （chip / stock symbol / sr evaluation）都有各自的 enabled 開關或選填依賴，
// 註冊失敗時 /scheduler/status 顯示 disabled 是說得通的（見 docs/api-reference.md
// 「`status` 的三種『沒有執行紀錄』情形」）。**本 job 沒有開關**——它顯示成 disabled
// 等於一個不存在的狀態，操作者不會知道還原係數已經停止重算。所以寧可用預設時間繼續跑，
// 也不要靜默停掉；打錯的那個值會以 Error log 留下來。
func (s *Scheduler) corporateActionCron() string {
	spec := strings.TrimSpace(s.corporateActionCfg.Cron)
	if spec == "" {
		return defaultCorporateActionCron
	}
	if _, err := cron.ParseStandard(spec); err != nil {
		s.log.Error("corporate action cron 無法解析，改用預設值",
			zap.String("cron", spec),
			zap.String("fallback", defaultCorporateActionCron),
			zap.Error(err))
		return defaultCorporateActionCron
	}
	return spec
}

// corporateActionTimeout 取設定的整輪預算，非正值時退回預設（比照 corporateActionCron
// 的第二道防線：不經過 config.Load() 的呼叫端同樣要拿得到可用的值）。
func (s *Scheduler) corporateActionTimeout() time.Duration {
	if sec := s.corporateActionCfg.TimeoutSec; sec > 0 {
		return time.Duration(sec) * time.Second
	}
	return defaultCorporateActionTimeout
}

// corporateActionShardCount 取設定的分片數，非正值時退回預設。
func (s *Scheduler) corporateActionShardCount() int {
	if n := s.corporateActionCfg.ShardCount; n > 0 {
		return n
	}
	return defaultCorporateActionShardCount
}

// corporateActionEpochMonday 是片號週序號的起點——1970-01-05 是星期一。
//
// 用「自某個固定星期一起算的週數」而不是 ISO 週數，是因為 ISO 週數跨年會從 52 跳回 1，
// 那個不連續會讓某一片被跳過或連跑兩次（shardCount > 5 時才看得出來）。
var corporateActionEpochMonday = time.Date(1970, 1, 5, 0, 0, 0, 0, time.UTC)

// corporateActionShardOfDay 算出 t 這天要跑第幾片：`(週序號×5 ＋ 平日序號) % shardCount`。
//
// **shardCount = 5 時等於「週一片 0、週五片 4」**（因為 `(5w+d) % 5 = d`），
// 每檔每週覆蓋一次；shardCount = 10 則連續 10 個工作日走完一輪（兩週）；1 是每天全量。
// **不能直接寫 `weekday-1`**：那只會產生 0～4，shardCount 設 7 或 10 時片 5 以後永遠輪不到——
// 那正是分片要消滅的「永遠輪不到的尾段」。
//
// 週六日沿用**當週週五那一片**。排程是平日 cron，週末只會來自手動觸發；
// 與其另外生一片打亂輪替，不如重跑最近一次排程日的名單（重算是冪等的）。
func corporateActionShardOfDay(t time.Time, shardCount int) int {
	if shardCount <= 1 {
		return 0
	}
	local := t.In(timeutil.TaipeiTZ)
	// 只取日期部分再換算天數，避免時分秒與時區位移影響整除結果。
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	week := int(day.Sub(corporateActionEpochMonday)/(24*time.Hour)) / 7
	dayIdx := int(local.Weekday()) - 1 // 週一 = 0
	if dayIdx < 0 || dayIdx > 4 {
		dayIdx = 4 // 週六日：沿用當週週五那片
	}
	shard := (week*5 + dayIdx) % shardCount
	if shard < 0 {
		shard += shardCount
	}
	return shard
}

// corporateActionShardOf 算出 symbol 屬於哪一片。
//
// **用 symbol 的 hash 而不是「排序後的位置」**：清單來源是
// `SELECT DISTINCT symbol FROM candles ORDER BY symbol`，順序穩定，但位置會漂移——
// 新股上市或標的下架會讓它後面的所有標的整批位移一格，被推過當天那片的標的要再等一輪，
// 「每檔每週至少覆蓋一次」在清單變動的那一週就不成立。hash 讓既有標的的片別
// 不受清單增減影響。
func corporateActionShardOf(symbol string, shardCount int) int {
	if shardCount <= 1 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(symbol))
	return int(h.Sum32() % uint32(shardCount))
}

// corporateActionSymbols 決定今天要逐檔同步哪些標的：
// **watchlist 全量 ＋ 其餘標的的當日分片**（規則見 docs/architecture.md 的公司行動同步段）。
//
// **為什麼 watchlist 不分片**：它是實際拿來做交易決策的清單，還原係數過期會直接影響
// 訊號與 SR 分析，所以每個排程日都要是最新的。其餘標的（評估標的池、有歷史但沒在看的檔）
// 只維護資料完整性，晚幾天更新不會產生錯誤決策——分片換來的是「每檔都輪得到」，
// 取代原本「每天固定只跑排序最前的約 50 檔、8xxx/9xxx 永遠輪不到」的確定性破洞。
//
// 只從 all（有 K 棒的標的）裡挑：沒有價格歷史的 watchlist 標的抓了事件也無從重算。
//
// **watchlist 讀不到時降級成「只跑當日分片」，不整輪放棄**（2026-08-24 review 後改）：
// 分片那一批與 watchlist 無關，讓它們陪葬會多掉一整片，而片號由日期決定、沒有游標，
// 掉的那片要等下一輪（預設一週）才輪得回來。回傳的 error **不是致命錯誤**，
// 而是「這輪的名單不完整」的訊號——呼叫端要照樣跑分片，並把該輪記成 partial，
// 才不會變成「有跑，但跑的不是該跑的」還顯示成功。
func (s *Scheduler) corporateActionSymbols(ctx context.Context, all []string) ([]string, error) {
	shardCount := s.corporateActionShardCount()
	shard := corporateActionShardOfDay(time.Now(), shardCount)

	watched := map[string]bool{}
	var watchlistErr error
	if s.watchlist != nil {
		symbols, err := s.watchlist.Symbols(ctx)
		if err != nil {
			watchlistErr = fmt.Errorf("列出 watchlist 失敗: %w", err)
			s.log.Error("列出 watchlist 失敗，本輪降級為只跑當日分片", zap.Error(err))
		}
		for _, symbol := range symbols {
			watched[symbol] = true
		}
	}

	selected := make([]string, 0, len(all))
	for _, symbol := range all {
		if watched[symbol] || corporateActionShardOf(symbol, shardCount) == shard {
			selected = append(selected, symbol)
		}
	}
	s.log.Info("公司行動同步的當日名單",
		zap.Int("all", len(all)), zap.Int("selected", len(selected)),
		zap.Int("watchlist", len(watched)),
		// watchlist=0 有兩種可能（真的空 / 讀失敗降級），靠這個欄位分辨。
		zap.Bool("watchlist_degraded", watchlistErr != nil),
		zap.Int("shard", shard), zap.Int("shard_count", shardCount))
	return selected, watchlistErr
}

// SetEvaluationUniverse 注入評估標的池與其排程設定。未呼叫或 cfg.Enabled=false 時
// 不註冊該 job，行為與導入前完全相同（比照 SetAdjuster）。
func (s *Scheduler) SetEvaluationUniverse(repo store.EvaluationUniverseRepo, cfg config.EvaluationUniverseConfig) {
	s.evaluationUniverse = repo
	s.evaluationUniverseCfg = cfg
}

// RunEvaluationUniverseSync 供 API 手動觸發。
func (s *Scheduler) RunEvaluationUniverseSync() {
	s.runEvaluationUniverseSync(context.Background())
}

// runEvaluationUniverseSync 更新評估標的池的日 K。
//
// **這是 T-040 Step 5 的唯一目的**：池裡的標的沒有任何流程會碰它們，尾端會逐日落後
// （實測 2026-08-17：全庫只有 9 檔 watchlist 有當日資料，池內另外 122 檔停在三天前）。
// evaluation 取「最後 N 根」，尾端不齊會讓評估視窗錯開、同一份報告隔幾天重跑結果不同。
//
// 單檔失敗只累計不中斷——131 檔裡一檔抓不到不該讓其餘 130 檔也不更新。
func (s *Scheduler) runEvaluationUniverseSync(ctx context.Context) {
	if s.evaluationUniverse == nil {
		return
	}
	// 這個 job 約 26 分鐘，cron 與人工觸發撞在一起會共用同一個節流器互相拖慢。
	if !s.universeSyncRunning.CompareAndSwap(false, true) {
		s.log.Warn("evaluation universe sync already running, skipped")
		return
	}
	defer s.universeSyncRunning.Store(false)

	runID := s.startRun(ctx, "evaluation_universe_sync")

	entries, err := s.evaluationUniverse.ListActive(ctx)
	if err != nil {
		s.log.Error("evaluation universe list failed", zap.Error(err))
		s.finishRun(ctx, runID, "evaluation_universe_sync", 0, 1, err.Error())
		return
	}
	if len(entries) == 0 {
		// 空池不是錯誤：表建好但還沒匯入清單時就是這個狀態。
		s.finishRun(ctx, runID, "evaluation_universe_sync", 0, 0, "")
		return
	}

	symbols := make([]string, 0, len(entries))
	for i := range entries {
		symbols = append(symbols, entries[i].Symbol)
	}

	days := s.evaluationUniverseCfg.Days
	if days <= 0 {
		days = 10
	}
	// **一定要帶 onSymbol**：131 檔在 5 req/min 下約 26 分鐘，沒有中間訊號的話
	// 「卡在第 3 檔」與「跑到第 130 檔」在 log 上完全一樣，無法判斷該不該中斷。
	// 每 25 檔記一次進度，避免 131 行雜訊；失敗的逐檔記，那才是要追的東西。
	done := 0
	var firstErr string
	onSymbol := func(symbol string, err error) {
		done++
		if err != nil {
			if firstErr == "" {
				firstErr = symbol + ": " + err.Error()
			}
			s.log.Warn("evaluation universe symbol failed",
				zap.String("symbol", symbol), zap.Int("at", done), zap.Error(err))
			return
		}
		if done%25 == 0 || done == len(symbols) {
			s.log.Info("evaluation universe sync progress",
				zap.Int("done", done), zap.Int("total", len(symbols)))
		}
	}

	failed := s.fetcher.BackfillHistory(ctx, symbols, days, onSymbol)
	lastErr := ""
	if failed > 0 {
		// 帶上第一個實際錯誤而不是一句「詳見 log」——job_runs 是排程頁唯一看得到的地方。
		lastErr = firstErr
		if lastErr == "" {
			lastErr = "部分標的回補失敗，詳見 log"
		}
	}
	s.log.Info("evaluation universe sync done",
		zap.Int("symbols", len(symbols)), zap.Int("failed", failed), zap.Int("days", days))
	s.finishRun(ctx, runID, "evaluation_universe_sync", len(symbols), failed, lastErr)
}

// RunCorporateActionSync 抓取公司行動並重算還原係數。
//
// 抓取區間刻意從 2015 年起整段重抓，而不是只抓增量：全市場只有數十筆分割，
// 一次請求就抓得完，而「增量」需要維護游標、漏一次就永久缺一筆。
// 事件表是 upsert、重算是冪等的，所以整段重抓沒有副作用。
func (s *Scheduler) RunCorporateActionSync() {
	if s.adjuster == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.corporateActionTimeout())
	defer cancel()

	runID := s.startRun(ctx, "corporate_action_sync")
	start := time.Date(2015, 1, 1, 0, 0, 0, 0, timeutil.TaipeiTZ)
	end := timeutil.TodayTaipei()

	splitEvents, err := s.adjuster.SyncSplits(ctx, start, end)
	if err != nil {
		s.log.Error("corporate action sync failed", zap.Error(err))
		// total 傳 1 而不是 0：total=0 會落到 finishRun 的 `failed > 0` 分支被記成
		// partial，但分割批次失敗時整輪根本沒開始跑，那是 failed 不是 partial。
		s.finishRun(ctx, runID, "corporate_action_sync", 1, 1, err.Error())
		return
	}

	// 除權息與減資：**逐檔查詢**（沒有批次端點，與分割不同）。
	//
	// 標的來源是 **candles 內所有相異 symbol**，不是 watchlist：評估標的池（T-040）的
	// 標的不在 watchlist 裡，只跑 watchlist 會讓它們「分割有還原、除權息沒有」，
	// 而且不會有任何東西報錯（2026-08-11 review）。
	all, err := s.adjuster.SymbolsWithCandles(ctx)
	if err != nil {
		s.log.Error("列出有 K 棒的標的失敗", zap.Error(err))
		s.finishRun(ctx, runID, "corporate_action_sync", 1, 1, err.Error())
		return
	}

	// 當日名單 = watchlist 全量 ＋ 其餘標的的當日分片。
	// watchlistErr **不致命**：名單仍然可用，只是少了 watchlist 那批。這輪照跑分片，
	// 但下面要記成 partial——分片與 watchlist 無關，讓它們陪葬會多掉一整片（2026-08-24 review）。
	symbols, watchlistErr := s.corporateActionSymbols(ctx, all)

	// 狀態誠實（語意見 docs/api-reference.md 的 /scheduler/status）：`job_runs` 的 total/failed 單位是**標的數**，
	// 所以 total 傳「這輪計畫要跑的檔數」而不是事件筆數，failed 則是
	// **失敗檔數 ＋ 因逾時沒輪到的檔數**。少了後面那一項，逾時被 break 掉的那輪會因為
	// 「零失敗」被記成 success——跑了 50 檔就停掉卻顯示成功，正是修改前的主要症狀。
	processed, failed, syncErr := s.adjuster.SyncPerSymbolEvents(ctx, symbols)
	skipped := len(symbols) - processed
	if skipped < 0 {
		skipped = 0
	}

	// error 欄只有一格，但一輪可能同時撞到兩件事（名單降級 ＋ 逾時），所以串起來寫，
	// 不讓後發生的那個蓋掉前一個。
	var errParts []string
	if syncErr != nil {
		// 個別標的失敗已在 Adjuster 內記錄並跳過；這裡只處理整體性錯誤（目前只有 ctx 逾時／取消）。
		s.log.Error("逐檔事件同步中止", zap.Error(syncErr),
			zap.Int("processed", processed), zap.Int("planned", len(symbols)))
		errParts = append(errParts, syncErr.Error())
	} else if skipped > 0 {
		// 有 syncErr 時它已經交代了為什麼停；沒有 syncErr 卻有沒跑到的檔才需要另外說明。
		errParts = append(errParts, fmt.Sprintf("%d 檔未處理", skipped))
	}
	if watchlistErr != nil {
		errParts = append(errParts, watchlistErr.Error())
	}

	s.log.Info("corporate action sync done",
		zap.Int("split_events", splitEvents),
		zap.Int("all", len(all)),
		zap.Int("planned", len(symbols)),
		zap.Int("processed", processed),
		zap.Int("failed", failed),
		zap.Int("skipped", skipped),
		zap.Bool("watchlist_degraded", watchlistErr != nil))
	// degraded 傳 watchlistErr != nil：watchlist 那批沒進名單，所以它們不可能算進
	// failed，零失敗也不該記 success。
	s.finishRunDegraded(ctx, runID, "corporate_action_sync", len(symbols), failed+skipped,
		strings.Join(errParts, "; "), watchlistErr != nil)
}

// SetSRAnalysis 注入 SR 分析 runner 與其排程設定（todo.md T-052）。
// 未呼叫或 cfg.Enabled=false 時不註冊，行為與導入前完全相同（比照 SetEvaluationUniverse）。
func (s *Scheduler) SetSRAnalysis(runner SRAnalysisRunner, candles store.CandleRepo, cfg config.SRAnalysisConfig) {
	s.srAnalysisRunner = runner
	s.srAnalysisCandles = candles
	s.srAnalysisCfg = cfg
}

// RunSRAnalysis 供 API 手動觸發。withChip 決定是否要求「當日籌碼已入庫」。
func (s *Scheduler) RunSRAnalysis(withChip bool) {
	s.runSRAnalysis(context.Background(), withChip)
}

// runSRAnalysis 對 watchlist 逐檔跑一次帶身分追蹤的 SR zone 分析（todo.md T-052）。
//
// **為什麼是 watchlist 而不是 evaluation_universe**：後者的唯一職能是日 K 維護，
// 「不做任何分析，也不參與任何交易決策或狀態推導」——那是 T-040 的核心約束，
// 見 docs/architecture.md「兩個標的清單」。
//
// **為什麼一天跑兩輪**：SR 分析吃籌碼（trading_score 的 Chip 佔 15%），而籌碼要晚間才
// 發布。17:00 那輪拿到的是前一日籌碼，22:00 那輪才有當日的。兩輪站在同一根 K 棒上，
// 只有籌碼不同。
//
// **序列執行**：這台 host 只有 2GiB。逐檔的峰值等同使用者手動點一次分析；
// 真的撐不住時要降頻而不是加併發。
func (s *Scheduler) runSRAnalysis(ctx context.Context, withChip bool) {
	if s.srAnalysisRunner == nil {
		return
	}
	jobName, guard := "sr_analysis", &s.srAnalysisRunning
	if withChip {
		jobName, guard = "sr_analysis_chip", &s.srAnalysisChipRunning
	}
	// 併發守衛只擋同一輪自己；兩輪之間互不影響。
	if !guard.CompareAndSwap(false, true) {
		s.log.Warn("sr analysis already running, skipped", zap.String("job", jobName))
		return
	}
	defer guard.Store(false)

	runID := s.startRun(ctx, jobName)

	symbols, err := s.watchlist.Symbols(ctx)
	if err != nil {
		s.log.Error("sr analysis watchlist failed", zap.String("job", jobName), zap.Error(err))
		s.finishRun(ctx, runID, jobName, 0, 1, err.Error())
		return
	}
	if len(symbols) == 0 {
		s.finishRun(ctx, runID, jobName, 0, 0, "")
		return
	}

	timeframe := s.srAnalysisCfg.Timeframe
	if timeframe == "" {
		timeframe = "1d"
	}
	today := timeutil.TodayTaipei()

	var failed, skipped int
	var firstErr string
	for _, symbol := range symbols {
		reason, ok := s.srAnalysisSkipReason(ctx, symbol, timeframe, today, withChip)
		if !ok {
			skipped++
			s.log.Info("sr analysis skipped",
				zap.String("job", jobName), zap.String("symbol", symbol), zap.String("reason", reason))
			continue
		}
		if _, err := s.srAnalysisRunner.RunAnalysis(ctx, symbol, timeframe, s.srAnalysisCfg.Limit); err != nil {
			failed++
			if firstErr == "" {
				firstErr = symbol + ": " + err.Error()
			}
			// **單檔失敗不中斷整批**：一檔沒有 K 棒或 Python 逾時，不該讓其餘標的整天沒分析。
			s.log.Warn("sr analysis symbol failed",
				zap.String("job", jobName), zap.String("symbol", symbol), zap.Error(err))
		}
	}
	total := len(symbols) - skipped
	s.log.Info("sr analysis done",
		zap.String("job", jobName), zap.Int("total", len(symbols)),
		zap.Int("analyzed", total-failed), zap.Int("skipped", skipped), zap.Int("failed", failed))
	s.finishRun(ctx, runID, jobName, total, failed, firstErr)
}

// srAnalysisSkipReason 回傳 (跳過原因, 是否該跑)。
//
// 這裡的判斷全部**從資料推導**，不靠行程內狀態——重啟後行為要一致，而且兩輪的規則
// 刻意不同（否則 17:00 跑完就會把 22:00 整輪擋掉）。四個跳過條件：
//
//   - 兩輪共同：最新 K 棒的交易日必須是今天。一次處理掉假日、停牌、daily_close 尚未完成。
//   - 僅 17:00：今天這根 K 棒已經分析過就不必再算。
//   - 僅 22:00：**當日籌碼尚未入庫就跳過**（查 chip_scores 本身，不是查最新分析用了什麼）。
//     21:00 的採集失敗或還沒寫完時跑這一輪，結果會與 17:00 那輪一模一樣——白算一次，
//     還多推一次 observed_absences。
//   - 僅 22:00：當日籌碼已入庫、但最新那筆分析已經用過它了，同樣不必再算。
//
// **所有查詢都帶 timeframe。** 少了它，使用者手動跑過的 5m 分析會擋掉 1d 的排程。
func (s *Scheduler) srAnalysisSkipReason(
	ctx context.Context, symbol, timeframe string, today time.Time, withChip bool,
) (string, bool) {
	// 共同前置：今天的 K 棒必須已經入庫。假日、停牌、daily_close 尚未完成都會落在這裡。
	if s.srAnalysisCandles != nil {
		candle, err := s.srAnalysisCandles.GetLatest(ctx, symbol, timeframe)
		switch {
		case err != nil:
			s.log.Warn("sr analysis candle lookup failed", zap.String("symbol", symbol), zap.Error(err))
			return "查不到 K 棒", false
		case candle == nil:
			return "沒有任何 K 棒", false
		case !taipeiDate(candle.Timestamp).Equal(today):
			return "最新 K 棒不是今天", false
		}
	}

	// **必須帶 timeframe**：List(symbol, 1) 只按 symbol 取最新一筆，使用者今天手動跑過一次
	// 5m 分析就會讓 1d 的排程誤判「今天已經分析過」而整批跳過。
	latest, err := s.srZoneRepo.GetLatestByTimeframe(ctx, symbol, timeframe)
	if err != nil {
		// 查不到就照跑：漏跑一次分析比因為一個唯讀查詢失敗而整檔停擺糟。
		s.log.Warn("sr analysis latest lookup failed", zap.String("symbol", symbol), zap.Error(err))
		return "", true
	}

	if !withChip {
		// 17:00 那輪：今天這根 K 棒已經分析過就不必再算。
		if latest != nil && taipeiDate(latest.AnalyzedAt).Equal(today) {
			return "已分析過今日 K 棒", false
		}
		return "", true
	}

	// 22:00 那輪的前提是「當日籌碼已經入庫」。**光看最新分析用的是不是今日籌碼不夠**：
	// 21:00 的 chip sync 失敗或還沒寫完時，那個條件同樣成立（最新分析用的是昨日籌碼），
	// 於是這一輪會拿著昨日籌碼再產生一筆內容相同的分析——白算一次，還多推一次
	// observed_absences，污染的正是 T-049 要用的 production 母體。所以先查籌碼本身。
	if s.chipScores != nil {
		chip, err := s.chipScores.GetLatest(ctx, symbol)
		switch {
		// **沒有籌碼資料不是錯誤**，是這檔還沒被採集過（新標的、或籌碼來源沒有它）。
		// ChipScoreRepo.GetLatest 對此回 sql.ErrNoRows，不特別處理的話每個這種標的
		// 每天都會產生一筆 Warn，把真正的查詢失敗淹掉。
		case errors.Is(err, sql.ErrNoRows) || (err == nil && chip == nil):
			return "沒有任何籌碼資料", false
		case err != nil:
			s.log.Warn("sr analysis chip lookup failed", zap.String("symbol", symbol), zap.Error(err))
			return "籌碼查詢失敗", false
		case !taipeiDate(chip.TradeDate).Equal(today):
			return "當日籌碼尚未入庫", false
		}
	}
	// 籌碼是今天的，但最新那筆分析已經用過它了 → 再算一次結果相同。
	if latest != nil && taipeiDate(latest.AnalyzedAt).Equal(today) &&
		chipTradeDateIs(latest.ChipSummary, today) {
		return "已用今日籌碼分析過", false
	}
	return "", true
}

// taipeiDate 取 t 在台北時區的日曆日（時分秒歸零）。
//
// **不能用 t 的 UTC 日期**：日 K 的 candles.ts 存的是 16:00Z＝台北隔日 00:00，
// 所以 `2026-08-17T16:00:00Z` 那根的交易日是 **08-18**。用 UTC 判斷會整批差一天。
func taipeiDate(t time.Time) time.Time {
	local := t.In(timeutil.TaipeiTZ)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, timeutil.TaipeiTZ)
}

// chipTradeDateIs 判斷這筆分析用的籌碼是不是 day 當天的。
//
// chip_summary 是 Python 產生的 JSON，`trade_date` 缺席或為 null 是**正常**的
// （該檔當日沒有籌碼資料時就會這樣），一律當成「不是今天」——保守側會讓 22:00 那輪
// 照跑，而不是誤判成已經算過。
func chipTradeDateIs(chipSummary store.RawJSON, day time.Time) bool {
	if len(chipSummary) == 0 {
		return false
	}
	var payload struct {
		TradeDate string `json:"trade_date"`
	}
	if err := json.Unmarshal([]byte(chipSummary), &payload); err != nil || payload.TradeDate == "" {
		return false
	}
	parsed, err := time.ParseInLocation("2006-01-02", payload.TradeDate, timeutil.TaipeiTZ)
	if err != nil {
		return false
	}
	return parsed.Equal(day)
}
