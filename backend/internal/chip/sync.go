package chip

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
)

// lookbackDays 涵蓋連續買賣超天數、5日累積買賣超、20日均量計算所需的緩衝
// （30 個日曆天約等於 20 個交易日，含週末假日餘裕）。
const lookbackDays = 30

// scoreRecomputeWindow 為 daily 單日同步時，chip_scores 額外向前回算的日曆天數。
// 用途：當某個交易日的原始籌碼在當天排程時尚未發布（抓到空資料），會在隔天的
// lookback 區間 fetch 才被補進 DB；此時需要回頭替那天重算分數，否則 chip_scores
// 會永遠落後一天。取小值（不需涵蓋整個 lookbackDays），只為自我修復近日缺口。
const scoreRecomputeWindow = 5

// ErrChipDataNotPublished 表示目標日確認是交易日（有日K）但請求的籌碼類型
// （法人／融資融券）尚未發布資料。用於讓排程/手動同步把「當日資料還沒進來」
// 記成可見的失敗訊號（job_runs.failed / chip_sync_jobs failures），而非誤判為
// 成功——空資料若靜默視為成功，會讓資料庫停在昨日卻毫無錯誤跡象。
var ErrChipDataNotPublished = errors.New("chip data for target date not published yet")

// Syncer 是 chip 套件唯一有 IO 副作用的部分：從 market.ChipDataSource 抓取
// 原始籌碼資料、寫入 store，並呼叫 Calculate 計算 chip_score 後落地。
type Syncer struct {
	source            market.ChipDataSource
	institutionalRepo store.InstitutionalTradeRepo
	marginRepo        store.MarginTradeRepo
	brokerRepo        store.BrokerTradeRepo
	scoreRepo         store.ChipScoreRepo
	candleRepo        store.CandleRepo
	log               *zap.Logger
}

func NewSyncer(
	source market.ChipDataSource,
	institutionalRepo store.InstitutionalTradeRepo,
	marginRepo store.MarginTradeRepo,
	brokerRepo store.BrokerTradeRepo,
	scoreRepo store.ChipScoreRepo,
	candleRepo store.CandleRepo,
	log *zap.Logger,
) *Syncer {
	return &Syncer{
		source:            source,
		institutionalRepo: institutionalRepo,
		marginRepo:        marginRepo,
		brokerRepo:        brokerRepo,
		scoreRepo:         scoreRepo,
		candleRepo:        candleRepo,
		log:               log,
	}
}

// SyncDaily 對單一股票、單一日期執行完整籌碼同步流程，供排程 daily 模式使用。
func (s *Syncer) SyncDaily(ctx context.Context, symbol string, date time.Time) error {
	var syncErr error
	s.SyncRange(ctx, []string{symbol}, date, date, nil, func(_ string, err error) {
		syncErr = err
	})
	return syncErr
}

// SyncRange 對多檔股票、一段日期區間執行籌碼同步，供 backfill/manual 使用。
// 法人與融資融券資料在每檔股票只呼叫一次區間 API（涵蓋 lookbackDays 緩衝），
// 避免逐日呼叫在長區間回補時觸發 rate limit；分點資料與分數計算仍須逐日
// 進行（分點是單日排行語意，分數計算依賴當日成交量）。dataTypes 為空時
// 代表同步全部四種類型（institutional/margin/broker/scores）。onProgress
// 在每檔股票處理完後回呼一次（err 非 nil 代表該股票至少一項處理失敗），
// 供呼叫端（scheduler 或 handler）寫回 job_runs/chip_sync_jobs 進度。
func (s *Syncer) SyncRange(ctx context.Context, symbols []string, from, to time.Time, dataTypes []string, onProgress func(symbol string, err error)) {
	lookbackStart := from.AddDate(0, 0, -lookbackDays)
	syncInstitutional := hasDataType(dataTypes, "institutional")
	syncMargin := hasDataType(dataTypes, "margin")
	syncBroker := hasDataType(dataTypes, "broker")
	syncScores := hasDataType(dataTypes, "scores")

	for _, symbol := range symbols {
		var firstErr error
		recordErr := func(err error) {
			if err != nil && firstErr == nil {
				firstErr = err
			}
		}

		if syncInstitutional {
			rows, err := s.source.FetchInstitutionalTrades(ctx, symbol, lookbackStart, to)
			if err != nil {
				s.log.Warn("chip sync: institutional fetch failed", zap.String("symbol", symbol), zap.Error(err))
				recordErr(err)
			} else {
				recordErr(s.upsertInstitutional(ctx, rows))
			}
		}

		if syncMargin {
			rows, err := s.source.FetchMarginTrades(ctx, symbol, lookbackStart, to)
			if err != nil {
				s.log.Warn("chip sync: margin fetch failed", zap.String("symbol", symbol), zap.Error(err))
				recordErr(err)
			} else {
				recordErr(s.upsertMargin(ctx, rows))
			}
		}

		// 目標日資料是否已發布：確認 to 是交易日（有日K）卻抓不到請求的籌碼類型
		// 時，記成 ErrChipDataNotPublished，讓呼叫端能看見「當日還沒進來」而不是
		// 誤判成功。fetch 本身若已失敗，firstErr 會保留該錯誤（recordErr 取首個非 nil）。
		if syncInstitutional || syncMargin {
			published, err := s.targetDatePublished(ctx, symbol, to, syncInstitutional, syncMargin)
			if err != nil {
				recordErr(err)
			} else if !published {
				recordErr(ErrChipDataNotPublished)
			}
		}

		// 分點資料是單日排行語意，僅處理請求區間 [from, to]。
		if syncBroker {
			for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
				recordErr(s.syncBrokerForDate(ctx, symbol, d))
			}
		}

		// chip_scores：daily 單日模式（from==to）額外向前回算 scoreRecomputeWindow 天，
		// 讓「前一交易日原本抓空、這次被 lookback 區間補上」的日子自動重算分數，
		// 消除分數落後一天。computeAndStoreScore 具冪等性且會 skip 無資料的非交易日，
		// 故回算窗口涵蓋到週末/尚未發布的日子也不會產生錯誤紀錄。backfill（from<to）
		// 已逐日涵蓋整個區間，維持原樣。
		if syncScores {
			scoreFrom := from
			if from.Equal(to) {
				scoreFrom = to.AddDate(0, 0, -scoreRecomputeWindow)
			}
			for d := scoreFrom; !d.After(to); d = d.AddDate(0, 0, 1) {
				recordErr(s.computeAndStoreScore(ctx, symbol, d))
			}
		}

		if onProgress != nil {
			onProgress(symbol, firstErr)
		}
	}
}

func hasDataType(dataTypes []string, want string) bool {
	if len(dataTypes) == 0 {
		return true // 未指定 = 同步全部類型
	}
	for _, d := range dataTypes {
		if d == want {
			return true
		}
	}
	return false
}

// syncBrokerForDate 抓取單日分點資料並落地。market.ErrBrokerDataUnsupported
// 視為非致命（目前 provider 不支援此資料類型），不回傳錯誤。
func (s *Syncer) syncBrokerForDate(ctx context.Context, symbol string, date time.Time) error {
	rows, err := s.source.FetchBrokerTrades(ctx, symbol, date)
	if err != nil {
		if errors.Is(err, market.ErrBrokerDataUnsupported) {
			return nil
		}
		s.log.Warn("chip sync: broker fetch failed", zap.String("symbol", symbol), zap.Time("date", date), zap.Error(err))
		return err
	}
	return s.upsertBroker(ctx, rows)
}

// computeAndStoreScore 從 store 讀回已落地的原始資料（不直接用剛 fetch 到的
// 記憶體資料，確保 backfill 逐日計算時的歷史區間正確以 date 為界，不會誤
// 用到 date 之後才落地的資料）並計算/落地 chip_score。
//
// 寫入前先確認 date 當天真的有資料（candle 或法人/融資融券任一），沒有
// 就整天 skip、不寫 chip_scores——避免非交易日（週末/國定假日）或資料
// 尚未發布時，用前一交易日的資料「借用」出一筆看似屬於當天、實則沿用
// 舊資料的分數紀錄（GetLatest 依 trade_date DESC 取最新，會讓使用者看到
// 週末日期的籌碼摘要，訊號加權也可能套用這種 stale score）。
func (s *Syncer) computeAndStoreScore(ctx context.Context, symbol string, date time.Time) error {
	hasData, err := s.hasDataForDate(ctx, symbol, date)
	if err != nil {
		return err
	}
	if !hasData {
		return nil
	}

	lookbackStart := date.AddDate(0, 0, -lookbackDays)

	institutionalHist, err := s.institutionalRepo.GetRange(ctx, symbol, lookbackStart, date)
	if err != nil {
		return err
	}
	marginHist, err := s.marginRepo.GetRange(ctx, symbol, lookbackStart, date)
	if err != nil {
		return err
	}
	brokerTrades, err := s.brokerRepo.GetByDate(ctx, symbol, date)
	if err != nil {
		return err
	}
	dailyVolume, avgVolume20, priceChangePercent, err := s.loadCandleContext(ctx, symbol, date)
	if err != nil {
		return err
	}

	result := Calculate(ChipScoreInput{
		Symbol:               symbol,
		Date:                 date,
		InstitutionalHistory: institutionalHist,
		MarginHistory:        marginHist,
		BrokerTrades:         brokerTrades,
		DailyVolume:          dailyVolume,
		AvgVolume20:          avgVolume20,
		PriceChangePercent:   priceChangePercent,
	})

	reasonJSON, err := json.Marshal(result.Reasons)
	if err != nil {
		return err
	}

	score := store.ChipScore{
		Symbol:             symbol,
		TradeDate:          date,
		InstitutionalScore: result.InstitutionalScore,
		MarginScore:        result.MarginScore,
		BrokerScore:        result.BrokerScore,
		ConcentrationScore: result.ConcentrationScore,
		TotalScore:         result.TotalScore,
		Signal:             string(result.Signal),
		Reason:             store.RawJSON(reasonJSON),
	}
	return s.scoreRepo.Upsert(ctx, &score)
}

// hasDataForDate 檢查 date 當天是否有任一種原始資料落地（candle、法人
// 買賣超、融資融券），藉此判斷這天是不是真的有發布資料的交易日。任一種
//存在就視為有效，不要求三者都齊全——避免單一資料來源延遲就整天被跳過。
func (s *Syncer) hasDataForDate(ctx context.Context, symbol string, date time.Time) (bool, error) {
	candles, err := s.candleRepo.GetRange(ctx, symbol, "1d", date, date)
	if err != nil {
		return false, err
	}
	if len(candles) > 0 {
		return true, nil
	}

	if _, err := s.institutionalRepo.GetByDate(ctx, symbol, date); err == nil {
		return true, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}

	if _, err := s.marginRepo.GetByDate(ctx, symbol, date); err == nil {
		return true, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}

	return false, nil
}

// targetDatePublished 判斷「目標日的籌碼資料是否已發布」。只在 date 確認為交易日
// （有日K）時才做判斷，避免週末/國定假日或當日K尚未同步時被誤判為「尚未發布」。
// 交易日確認後，只要 checkInst/checkMargin 中任一被請求的類型在 date 當天已有資料
// 就視為已發布（true）；全部請求的類型都查無資料（sql.ErrNoRows）才回傳 false，
// 代表 FinMind 對這天的籌碼還沒發布。
func (s *Syncer) targetDatePublished(ctx context.Context, symbol string, date time.Time, checkInst, checkMargin bool) (bool, error) {
	candles, err := s.candleRepo.GetRange(ctx, symbol, "1d", date, date)
	if err != nil {
		return false, err
	}
	if len(candles) == 0 {
		// 無法確認 date 是交易日（可能是週末/假日，或當日K尚未同步）→ 不判定為
		// 「尚未發布」，避免非交易日一律被記成失敗。
		return true, nil
	}

	if checkInst {
		if _, err := s.institutionalRepo.GetByDate(ctx, symbol, date); err == nil {
			return true, nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return false, err
		}
	}
	if checkMargin {
		if _, err := s.marginRepo.GetByDate(ctx, symbol, date); err == nil {
			return true, nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return false, err
		}
	}

	return false, nil
}

// loadCandleContext 取當日成交量、近20日均量與當日漲跌幅（%）。candles 依
// date 為界（GetRange 上界為 date），若 date 當天尚無 candle（例如非交易日
// 或當日K線尚未同步），會退而取得 date 之前最近一根 K 棒的資料，可能造成
// 分數計算使用稍舊的成交量/價格，屬於可接受的降級行為。
func (s *Syncer) loadCandleContext(ctx context.Context, symbol string, date time.Time) (dailyVolume int64, avgVolume20, priceChangePercent float64, err error) {
	candles, err := s.candleRepo.GetRange(ctx, symbol, "1d", date.AddDate(0, 0, -lookbackDays), date)
	if err != nil {
		return 0, 0, 0, err
	}
	if len(candles) == 0 {
		return 0, 0, 0, nil
	}

	today := candles[len(candles)-1]
	dailyVolume = today.Volume

	history := candles[:len(candles)-1]
	if len(history) == 0 {
		return dailyVolume, 0, 0, nil
	}

	var sum int64
	for _, c := range history {
		sum += c.Volume
	}
	avgVolume20 = float64(sum) / float64(len(history))

	prev := history[len(history)-1]
	if prev.Close != 0 {
		priceChangePercent = (today.Close - prev.Close) / prev.Close * 100
	}
	return dailyVolume, avgVolume20, priceChangePercent, nil
}

func (s *Syncer) upsertInstitutional(ctx context.Context, rows []market.InstitutionalTrade) error {
	if len(rows) == 0 {
		return nil
	}
	out := make([]store.InstitutionalTrade, len(rows))
	for i, r := range rows {
		out[i] = store.InstitutionalTrade{
			Symbol:                r.Symbol,
			TradeDate:             r.Date,
			ForeignNetBuy:         r.ForeignNetBuy,
			InvestmentTrustNetBuy: r.InvestmentTrustNetBuy,
			DealerNetBuy:          r.DealerNetBuy,
			TotalNetBuy:           r.TotalNetBuy,
		}
	}
	return s.institutionalRepo.BulkUpsert(ctx, out)
}

func (s *Syncer) upsertMargin(ctx context.Context, rows []market.MarginTrade) error {
	if len(rows) == 0 {
		return nil
	}
	out := make([]store.MarginTrade, len(rows))
	for i, r := range rows {
		mt := store.MarginTrade{
			Symbol:        r.Symbol,
			TradeDate:     r.Date,
			MarginBalance: r.MarginBalance,
			MarginChange:  r.MarginChange,
			ShortBalance:  r.ShortBalance,
			ShortChange:   r.ShortChange,
		}
		if r.MarginUsageRate != nil {
			mt.MarginUsageRate = store.NullFloat64{NullFloat64: sql.NullFloat64{Float64: *r.MarginUsageRate, Valid: true}}
		}
		if r.ShortUsageRate != nil {
			mt.ShortUsageRate = store.NullFloat64{NullFloat64: sql.NullFloat64{Float64: *r.ShortUsageRate, Valid: true}}
		}
		out[i] = mt
	}
	return s.marginRepo.BulkUpsert(ctx, out)
}

func (s *Syncer) upsertBroker(ctx context.Context, rows []market.BrokerTrade) error {
	if len(rows) == 0 {
		return nil
	}
	out := make([]store.BrokerTrade, len(rows))
	for i, r := range rows {
		out[i] = store.BrokerTrade{
			Symbol:     r.Symbol,
			TradeDate:  r.Date,
			BrokerName: r.BrokerName,
			BranchName: r.BranchName,
			BuyVolume:  r.BuyVolume,
			SellVolume: r.SellVolume,
			NetBuy:     r.NetBuy,
		}
	}
	return s.brokerRepo.BulkUpsert(ctx, out)
}
