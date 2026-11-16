package config

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/robfig/cron/v3"
)

// 設定契約（I-132 的 E；規格見 docs/plans/i132-redis-silent-disable.md 附錄 C）。
//
// Validate 在 Load() 之後、連 DB 之前執行；違反時只回報「環境變數名稱:規則代號」，
// ⛔ 不帶值、值的長度或片段。規則依 APP_ENV 分兩級：
//   - 「全部」：production／dev／test 都檢查（格式、範圍、條件必要的格式）。
//   - 「prod」：只在 production 檢查（必要的機密、公開範例值、JWT 長度、必須的服務）。
//
// 每一條都有適用條件：功能停用時，它的鍵就算是空值或範例值也放行
// （viper 會把空字串退回 config.yaml 的範例值，例如 fugle.api_key 的 YOUR_FUGLE_API_KEY）。

// 規則代號（封閉值域）。
const (
	RuleRequired      = "required"
	RulePublicExample = "public_example"
	RuleInvalidValue  = "invalid_value"
	RuleInvalidCron   = "invalid_cron"
	RuleInvalidURL    = "invalid_url"
	RuleOutOfRange    = "out_of_range"
	RuleTooShort      = "too_short"
	RuleInvalidType   = "invalid_type"
	RuleUnreadable    = "unreadable"
	RuleInvalid       = "invalid"
	// RuleRequiredWhenRedisAddr：有設定 REDIS_ADDR 時，REDIS_PASSWORD 是必要的。
	RuleRequiredWhenRedisAddr = "required_when_redis_addr"
)

// 環境類別（D9）。沒設定時當成 production（最嚴格）。
const (
	AppEnvProduction = "production"
	AppEnvDev        = "dev"
	AppEnvTest       = "test"
)

// JWTMinLength 是 production 的 AUTH_JWT_SECRET 最短長度（HS256；C.3 Q1）。
const JWTMinLength = 32

// 公開範例值：repo 裡任何一處寫過的範例或預設憑證（完整清單見計畫書 4.6；測試會掃描 repo 確認沒有漏）。
var (
	PublicJWTSecrets      = []string{"change-me-in-production", "change-me", "dev-only-change-me"}
	PublicFinMindKeys     = []string{"YOUR_FINMIND_API_KEY", "your-finmind-api-key"}
	PublicFugleKeys       = []string{"YOUR_FUGLE_API_KEY", "your-fugle-api-key"}
	PublicRedisPasswords  = []string{"dev-redis-only"}
	PublicDBUserPasswords = []string{"trading:trading", "user:password", "root:password"}
	PublicDBPasswords     = []string{"trading", "password"}
)

// defaultSQLiteDSN 是 config.yaml 的 database.dsn 預設值（本機 SQLite 檔）。
const defaultSQLiteDSN = "../../output/stock_trading/trading.db"

// Violation 是一筆契約違反：Key 是環境變數名稱，Rule 是規則代號。
type Violation struct {
	Key  string
	Rule string
}

func (v Violation) String() string { return v.Key + ":" + v.Rule }

// JoinViolations 把違反清單串成「KEY:rule, KEY:rule」。
func JoinViolations(vs []Violation) string {
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = v.String()
	}
	return strings.Join(parts, ", ")
}

// ViolationStrings 給 zap.Strings 用。
func ViolationStrings(vs []Violation) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.String()
	}
	return out
}

var timeframeRE = regexp.MustCompile(`^[1-9][0-9]*[mhdw]$`)

type checker struct {
	out  []Violation
	prod bool
}

func (c *checker) add(key, rule string) { c.out = append(c.out, Violation{Key: key, Rule: rule}) }

func (c *checker) cron(key, spec string) {
	if _, err := cron.ParseStandard(spec); err != nil {
		c.add(key, RuleInvalidCron)
	}
}

func (c *checker) atLeast(key string, v, min int) {
	if v < min {
		c.add(key, RuleOutOfRange)
	}
}

func (c *checker) url(key, raw string, schemes ...string) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		c.add(key, RuleInvalidURL)
		return
	}
	for _, s := range schemes {
		if u.Scheme == s {
			return
		}
	}
	c.add(key, RuleInvalidURL)
}

// secret：prod 時不能是空值、不能是公開範例值。
func (c *checker) secret(key, v string, examples []string) {
	if !c.prod {
		return
	}
	if v == "" {
		c.add(key, RuleRequired)
		return
	}
	if contains(examples, v) {
		c.add(key, RulePublicExample)
	}
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// IsProduction 回報 Validate 會不會套用 production 的規則（APP_ENV 沒設定也算）。
func (c *Config) IsProduction() bool {
	return c.AppEnv == "" || c.AppEnv == AppEnvProduction
}

// Validate 依設定契約檢查最終的設定值，回傳違反清單（空的代表通過）。
func Validate(cfg *Config) []Violation {
	c := &checker{prod: cfg.IsProduction()}

	switch cfg.AppEnv {
	case "", AppEnvProduction, AppEnvDev, AppEnvTest:
	default:
		// 打錯字⛔ 不能默默變成寬鬆模式：其他規則照 production 檢查。
		c.add("APP_ENV", RuleInvalidValue)
		c.prod = true
	}

	// ── DB ──
	if c.prod {
		if cfg.Database.Driver != "postgres" {
			c.add("DATABASE_DRIVER", RuleInvalidValue)
		}
		switch {
		case cfg.Database.DSN == "":
			c.add("DATABASE_DSN", RuleRequired)
		case cfg.Database.DSN == defaultSQLiteDSN:
			c.add("DATABASE_DSN", RuleInvalidValue)
		case cfg.Database.Driver != "postgres":
			// driver 已經判違反；帳密照 PostgreSQL 的規則解析沒有意義
		default:
			user, pass, ok := dsnCredentials(cfg.Database.DSN)
			switch {
			case !ok:
				c.add("DATABASE_DSN", RuleInvalidValue)
			case contains(PublicDBUserPasswords, user+":"+pass) || contains(PublicDBPasswords, pass):
				c.add("DATABASE_DSN", RulePublicExample)
			}
		}
	}

	// ── HTTP ──
	if p, err := strconv.Atoi(cfg.Server.Port); err != nil || p < 1 || p > 65535 {
		c.add("SERVER_PORT", RuleOutOfRange)
	}

	// ── Redis ──
	if c.prod && cfg.Redis.Addr == "" {
		c.add("REDIS_ADDR", RuleRequired)
	}
	if cfg.Redis.Addr != "" {
		if c.prod {
			switch {
			case cfg.Redis.Password == "":
				c.add("REDIS_PASSWORD", RuleRequiredWhenRedisAddr)
			case contains(PublicRedisPasswords, cfg.Redis.Password):
				c.add("REDIS_PASSWORD", RulePublicExample)
			}
		}
		c.atLeast("REDIS_DB", cfg.Redis.DB, 0)
	}

	// ── 登入 ──
	if cfg.Auth.JWTSecret == "" {
		c.add("AUTH_JWT_SECRET", RuleRequired)
	} else if c.prod {
		switch {
		case contains(PublicJWTSecrets, cfg.Auth.JWTSecret):
			c.add("AUTH_JWT_SECRET", RulePublicExample)
		case len(cfg.Auth.JWTSecret) < JWTMinLength:
			c.add("AUTH_JWT_SECRET", RuleTooShort)
		}
	}

	// ── FinMind（日 K、公司行動、籌碼都無條件用到）──
	c.secret("FINMIND_API_KEY", cfg.FinMind.APIKey, PublicFinMindKeys)
	c.url("FINMIND_BASE_URL", cfg.FinMind.BaseURL, "https")
	c.atLeast("FINMIND_RATE_LIMIT", cfg.FinMind.RateLimit, 1)

	// ── Fugle（只在啟用時用到）──
	if cfg.Fugle.Enabled {
		c.secret("FUGLE_API_KEY", cfg.Fugle.APIKey, PublicFugleKeys)
		c.url("FUGLE_REST_BASE_URL", cfg.Fugle.RESTBaseURL, "https")
		c.url("FUGLE_WS_ENDPOINT", cfg.Fugle.WSEndpoint, "wss")
		c.atLeast("FUGLE_QUOTE_RATE_LIMIT", cfg.Fugle.QuoteRateLimit, 1)
		c.atLeast("FUGLE_MAX_SUBSCRIPTIONS", cfg.Fugle.MaxSubscriptions, 1)
	}

	// ── Yahoo（RATE_LIMIT 另外給除權息來源用，無條件）──
	c.atLeast("YAHOO_RATE_LIMIT", cfg.Yahoo.RateLimit, 1)
	if cfg.Yahoo.Enabled {
		c.url("YAHOO_BASE_URL", cfg.Yahoo.BaseURL, "https")
		c.atLeast("YAHOO_BATCH_SIZE", cfg.Yahoo.BatchSize, 1)
	}

	// ── Python 服務 ──
	if cfg.Python.ServiceURL == "" {
		if c.prod {
			c.add("PYTHON_SERVICE_URL", RuleRequired)
		}
	} else {
		c.url("PYTHON_SERVICE_URL", cfg.Python.ServiceURL, "http", "https")
	}
	c.atLeast("PYTHON_SR_ZONES_TIMEOUT_SEC", cfg.Python.SRZonesTimeoutSec, 1)

	// ── 主檔同步 ──
	if cfg.StockSymbols.Enabled {
		c.cron("STOCK_SYMBOLS_CRON", cfg.StockSymbols.Cron)
		if len(cfg.StockSymbols.SyncURLs) == 0 {
			c.add("STOCK_SYMBOLS_SYNC_URLS", RuleRequired)
		}
		for _, u := range cfg.StockSymbols.SyncURLs {
			before := len(c.out)
			c.url("STOCK_SYMBOLS_SYNC_URLS", u, "http", "https")
			if len(c.out) > before {
				break // 同一個鍵只報一次
			}
		}
	}
	c.atLeast("STOCK_SYMBOLS_TIMEOUT_SEC", cfg.StockSymbols.TimeoutSec, 0)
	c.atLeast("STOCK_SYMBOLS_FETCH_DELAY_SEC", cfg.StockSymbols.FetchDelaySec, 0)

	// ── 籌碼（無條件註冊）──
	c.cron("CHIP_SYNC_CRON", cfg.Chip.Sync.Cron)
	c.atLeast("CHIP_SYNC_HISTORY_TRADING_DAYS", cfg.Chip.Sync.HistoryTradingDays, 1)
	c.atLeast("CHIP_SYNC_BATCH_SIZE", cfg.Chip.Sync.BatchSize, 1)

	// ── 公司行動（無條件註冊；空白沿用 scheduler 的預設，打錯字啟動就擋：C.3 Q4）──
	if strings.TrimSpace(cfg.CorporateAction.Cron) != "" {
		c.cron("CORPORATE_ACTION_CRON", cfg.CorporateAction.Cron)
	}

	// ── 評估標的池 ──
	if cfg.EvaluationUniverse.Enabled {
		c.cron("EVALUATION_UNIVERSE_CRON", cfg.EvaluationUniverse.Cron)
		c.atLeast("EVALUATION_UNIVERSE_DAYS", cfg.EvaluationUniverse.Days, 1)
	}

	// ── SR 分析 ──
	if cfg.SRAnalysis.Enabled {
		c.cron("SR_ANALYSIS_CRON", cfg.SRAnalysis.Cron)
		c.cron("SR_ANALYSIS_CHIP_CRON", cfg.SRAnalysis.ChipCron)
		if !timeframeRE.MatchString(cfg.SRAnalysis.Timeframe) {
			c.add("SR_ANALYSIS_TIMEFRAME", RuleInvalidValue)
		}
		c.atLeast("SR_ANALYSIS_LIMIT", cfg.SRAnalysis.Limit, 1)
	}

	// ── SR evaluation ──
	if cfg.SREvaluation.Enabled {
		c.cron("SR_EVALUATION_CRON", cfg.SREvaluation.Cron)
		if !timeframeRE.MatchString(cfg.SREvaluation.Timeframe) {
			c.add("SR_EVALUATION_TIMEFRAME", RuleInvalidValue)
		}
		c.atLeast("SR_EVALUATION_LIMIT", cfg.SREvaluation.Limit, 1)
		c.atLeast("SR_EVALUATION_REPLAY_MAX_ROWS", cfg.SREvaluation.ReplayMaxRows, 1)
	}

	// ── 終止上市對帳 ──
	if cfg.Delisting.Enabled {
		c.url("DELISTING_URL", cfg.Delisting.URL, "http", "https")
		if strings.TrimSpace(cfg.Delisting.Cron) != "" {
			c.cron("DELISTING_CRON", cfg.Delisting.Cron)
		}
		c.atLeast("DELISTING_TIMEOUT_SEC", cfg.Delisting.TimeoutSec, 0)
	}

	// ── 持倉分析（C.3 Q2；現在 ≤ 0 會在 portfolio.NewAnalyzer 靜默退回預設值）──
	pa := cfg.PositionAnalysis
	positive := func(key string, v float64) {
		if !(v > 0) {
			c.add(key, RuleOutOfRange)
		}
	}
	ratio := func(key string, v float64) {
		if !(v > 0 && v <= 1) {
			c.add(key, RuleOutOfRange)
		}
	}
	positive("POSITION_ANALYSIS_MAX_POSITION_VALUE", pa.MaxPositionValue)
	positive("POSITION_ANALYSIS_MAX_RISK_AMOUNT", pa.MaxRiskAmount)
	positive("POSITION_ANALYSIS_MIN_RISK_REWARD_RATIO", pa.MinRiskRewardRatio)
	positive("POSITION_ANALYSIS_BREAKOUT_TARGET_RISK_REWARD_RATIO", pa.BreakoutTargetRR)
	ratio("POSITION_ANALYSIS_ADD_ON_RATIO", pa.AddOnRatio)
	ratio("POSITION_ANALYSIS_TAKE_PROFIT_REDUCTION_RATIO", pa.TakeProfitReductionRatio)
	c.atLeast("POSITION_ANALYSIS_SR_REUSE_MAX_AGE_HOURS", pa.SRReuseMaxAgeHours, 1)

	return c.out
}

// dsnCredentials 取出 PostgreSQL driver **最後實際使用**的使用者與密碼。
//
// ⛔ 不自己解析字串：直接用 pgx 的 pgconn.ParseConfig（store 用 sqlx.Connect("pgx", dsn)，
// 走的就是這個解析），所以 URL 的 userinfo、query 參數（`?user=…&password=…`）、
// key=value 的引號寫法（`password='a b'`）、`PGUSER`／`PGPASSWORD`、passfile
// 的結果都跟 driver 一致。ParseConfig 不連線。
//
// 解析不了時 ok 為 false。⛔ 不回傳錯誤原文（可能含密碼）。
func dsnCredentials(dsn string) (user, pass string, ok bool) {
	pc, err := pgconn.ParseConfig(dsn)
	if err != nil {
		return "", "", false
	}
	return pc.User, pc.Password, true
}
