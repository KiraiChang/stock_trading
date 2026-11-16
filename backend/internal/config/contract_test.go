package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// validConfig 回傳一份能通過 production 契約的設定，各測試只改要驗的那一項。
func validConfig(appEnv string) *Config {
	return &Config{
		AppEnv:   appEnv,
		Server:   ServerConfig{Port: "8080"},
		Database: DatabaseConfig{Driver: "postgres", DSN: "postgres://app:Zq8-generated@db:5432/trading?sslmode=disable"},
		Redis:    RedisConfig{Addr: "redis:6379", Password: "a1b2c3-generated"},
		FinMind:  FinMindConfig{APIKey: "real-finmind-key", BaseURL: "https://api.finmindtrade.com/api/v4", RateLimit: 5},
		Fugle: FugleConfig{APIKey: "YOUR_FUGLE_API_KEY", RESTBaseURL: "https://api.fugle.tw/x",
			WSEndpoint: "wss://api.fugle.tw/y", QuoteRateLimit: 60, MaxSubscriptions: 5},
		Yahoo:              YahooConfig{BaseURL: "https://tw.stock.yahoo.com/x", RateLimit: 20, BatchSize: 40},
		StockSymbols:       StockSymbolsConfig{Enabled: true, Cron: "30 6 * * *", SyncURLs: []string{"https://isin.twse.com.tw/a"}},
		Python:             PythonConfig{ServiceURL: "http://python-server:8001", SRZonesTimeoutSec: 120},
		Auth:               AuthConfig{JWTSecret: strings.Repeat("k", 64)},
		Chip:               ChipConfig{Sync: ChipSyncConfig{HistoryTradingDays: 500, BatchSize: 50, Cron: "0 21 * * 1-5"}},
		SREvaluation:       SREvaluationConfig{Cron: "30 22 * * 1-5", Timeframe: "1d", Limit: 1500, ReplayMaxRows: 200},
		EvaluationUniverse: EvaluationUniverseConfig{Enabled: true, Cron: "0 16 * * 1-5", Days: 10},
		SRAnalysis: SRAnalysisConfig{Enabled: true, Cron: "0 17 * * 1-5", ChipCron: "0 22 * * 1-5",
			Timeframe: "1d", Limit: 400},
		CorporateAction: CorporateActionConfig{Cron: "30 6 * * 1-5", TimeoutSec: 2700, ShardCount: 5},
		PositionAnalysis: PositionAnalysisConfig{MaxPositionValue: 200000, MaxRiskAmount: 10000, AddOnRatio: 0.25,
			MinRiskRewardRatio: 1.5, BreakoutTargetRR: 2, TakeProfitReductionRatio: 0.5, SRReuseMaxAgeHours: 24},
		Delisting: DelistingConfig{Enabled: true, URL: "https://www.twse.com.tw/x", Cron: "0 7 * * *"},
	}
}

func has(vs []Violation, key, rule string) bool {
	for _, v := range vs {
		if v.Key == key && v.Rule == rule {
			return true
		}
	}
	return false
}

func TestValidate_ValidProductionPasses(t *testing.T) {
	for _, env := range []string{"", AppEnvProduction, AppEnvDev, AppEnvTest} {
		if v := Validate(validConfig(env)); len(v) != 0 {
			t.Errorf("APP_ENV=%q：合法的設定不該違反契約，得到 %s", env, JoinViolations(v))
		}
	}
}

func TestValidate_AppEnvTypoIsStrict(t *testing.T) {
	cfg := validConfig("prod") // 打錯字
	cfg.Auth.JWTSecret = "dev-only-change-me"
	v := Validate(cfg)
	if !has(v, "APP_ENV", RuleInvalidValue) {
		t.Fatalf("APP_ENV 打錯字要擋下，得到 %s", JoinViolations(v))
	}
	if !has(v, "AUTH_JWT_SECRET", RulePublicExample) {
		t.Fatalf("APP_ENV 打錯字時其他規則要照 production 檢查，得到 %s", JoinViolations(v))
	}
}

// 公開範例值：清單裡的每一個值都要 production 擋下、dev／test 放行。
func TestValidate_PublicExamples(t *testing.T) {
	isolatePGEnv(t)
	type tc struct {
		name, key, rule string
		mut             func(*Config)
	}
	var cases []tc
	for _, s := range PublicJWTSecrets {
		s := s
		cases = append(cases, tc{"jwt " + s, "AUTH_JWT_SECRET", RulePublicExample, func(c *Config) { c.Auth.JWTSecret = s }})
	}
	for _, s := range PublicFinMindKeys {
		s := s
		cases = append(cases, tc{"finmind " + s, "FINMIND_API_KEY", RulePublicExample, func(c *Config) { c.FinMind.APIKey = s }})
	}
	for _, s := range PublicFugleKeys {
		s := s
		cases = append(cases, tc{"fugle " + s, "FUGLE_API_KEY", RulePublicExample, func(c *Config) {
			c.Fugle.Enabled = true
			c.Fugle.APIKey = s
		}})
	}
	for _, s := range PublicRedisPasswords {
		s := s
		cases = append(cases, tc{"redis " + s, "REDIS_PASSWORD", RulePublicExample, func(c *Config) { c.Redis.Password = s }})
	}
	for _, up := range PublicDBUserPasswords {
		up := up
		cases = append(cases,
			tc{"dsn url " + up, "DATABASE_DSN", RulePublicExample, func(c *Config) { c.Database.DSN = "postgres://" + up + "@db:5432/trading" }},
			tc{"dsn query " + up, "DATABASE_DSN", RulePublicExample, func(c *Config) {
				u, pw, _ := strings.Cut(up, ":")
				c.Database.DSN = "postgres://db/trading?user=" + u + "&password=" + pw
			}},
			// MySQL 寫法 pgx 解析不了：driver 是 postgres 時一樣擋下（只是代號不同）
			tc{"dsn mysql " + up, "DATABASE_DSN", RuleInvalidValue, func(c *Config) { c.Database.DSN = up + "@tcp(db:3306)/trading" }})
	}
	for _, p := range PublicDBPasswords {
		p := p
		cases = append(cases,
			tc{"dsn pass " + p, "DATABASE_DSN", RulePublicExample, func(c *Config) { c.Database.DSN = "postgres://someone:" + p + "@db/trading" }},
			tc{"dsn kv " + p, "DATABASE_DSN", RulePublicExample, func(c *Config) { c.Database.DSN = "host=db user=someone password=" + p }},
			tc{"dsn kv quoted " + p, "DATABASE_DSN", RulePublicExample, func(c *Config) { c.Database.DSN = "host=db user=someone password='" + p + "'" }},
			// URL 的 userinfo 是產生的密碼，query 參數覆寫成範例密碼：driver 用的是 query 的值
			tc{"dsn query override " + p, "DATABASE_DSN", RulePublicExample, func(c *Config) {
				c.Database.DSN = "postgres://app:Zq8-generated@db/trading?password=" + p
			}})
	}
	for _, c := range cases {
		prod := validConfig(AppEnvProduction)
		c.mut(prod)
		if v := Validate(prod); !has(v, c.key, c.rule) {
			t.Errorf("%s：production 要擋下 %s:%s，得到 %s", c.name, c.key, c.rule, JoinViolations(v))
		}
		for _, env := range []string{AppEnvDev, AppEnvTest} {
			cfg := validConfig(env)
			c.mut(cfg)
			if v := Validate(cfg); len(v) != 0 {
				t.Errorf("%s：%s 要放行，得到 %s", c.name, env, JoinViolations(v))
			}
		}
	}
}

func TestValidate_FugleDisabledAllowsPlaceholder(t *testing.T) {
	cfg := validConfig("")
	cfg.Fugle.Enabled = false
	cfg.Fugle.APIKey = "YOUR_FUGLE_API_KEY" // viper 把空值退回 config.yaml 的範例值
	if v := Validate(cfg); len(v) != 0 {
		t.Fatalf("Fugle 停用時 key 是範例值要放行，得到 %s", JoinViolations(v))
	}
	cfg.Fugle.APIKey = ""
	if v := Validate(cfg); len(v) != 0 {
		t.Fatalf("Fugle 停用時 key 是空值要放行，得到 %s", JoinViolations(v))
	}
	cfg.Fugle.Enabled = true
	if v := Validate(cfg); !has(v, "FUGLE_API_KEY", RuleRequired) {
		t.Fatalf("Fugle 啟用時 key 是空值要擋下，得到 %s", JoinViolations(v))
	}
}

func TestValidate_ProductionRules(t *testing.T) {
	cases := []struct {
		name, key, rule string
		mut             func(*Config)
	}{
		{"driver sqlite", "DATABASE_DRIVER", RuleInvalidValue, func(c *Config) { c.Database.Driver = "sqlite" }},
		{"dsn empty", "DATABASE_DSN", RuleRequired, func(c *Config) { c.Database.DSN = "" }},
		{"dsn sqlite default", "DATABASE_DSN", RuleInvalidValue, func(c *Config) { c.Database.DSN = defaultSQLiteDSN }},
		{"dsn unparsable", "DATABASE_DSN", RuleInvalidValue, func(c *Config) { c.Database.DSN = "not a dsn" }},
		{"redis addr empty", "REDIS_ADDR", RuleRequired, func(c *Config) { c.Redis.Addr = "" }},
		{"redis password empty", "REDIS_PASSWORD", RuleRequiredWhenRedisAddr, func(c *Config) { c.Redis.Password = "" }},
		{"jwt short", "AUTH_JWT_SECRET", RuleTooShort, func(c *Config) { c.Auth.JWTSecret = strings.Repeat("k", JWTMinLength-1) }},
		{"finmind empty", "FINMIND_API_KEY", RuleRequired, func(c *Config) { c.FinMind.APIKey = "" }},
		{"python url empty", "PYTHON_SERVICE_URL", RuleRequired, func(c *Config) { c.Python.ServiceURL = "" }},
	}
	for _, c := range cases {
		prod := validConfig("")
		c.mut(prod)
		if v := Validate(prod); !has(v, c.key, c.rule) {
			t.Errorf("%s：production 要擋下 %s:%s，得到 %s", c.name, c.key, c.rule, JoinViolations(v))
		}
		dev := validConfig(AppEnvDev)
		c.mut(dev)
		if v := Validate(dev); len(v) != 0 {
			t.Errorf("%s：dev 要放行，得到 %s", c.name, JoinViolations(v))
		}
	}
	// JWT 在 32 bytes 剛好通過
	cfg := validConfig("")
	cfg.Auth.JWTSecret = strings.Repeat("k", JWTMinLength)
	if v := Validate(cfg); len(v) != 0 {
		t.Errorf("JWT 剛好 %d bytes 要通過，得到 %s", JWTMinLength, JoinViolations(v))
	}
}

func TestValidate_AllEnvRules(t *testing.T) {
	cases := []struct {
		name, key, rule string
		mut             func(*Config)
	}{
		{"jwt empty", "AUTH_JWT_SECRET", RuleRequired, func(c *Config) { c.Auth.JWTSecret = "" }},
		{"port", "SERVER_PORT", RuleOutOfRange, func(c *Config) { c.Server.Port = "70000" }},
		{"port text", "SERVER_PORT", RuleOutOfRange, func(c *Config) { c.Server.Port = "http" }},
		{"python url bad", "PYTHON_SERVICE_URL", RuleInvalidURL, func(c *Config) { c.Python.ServiceURL = "python-server:8001" }},
		{"chip cron", "CHIP_SYNC_CRON", RuleInvalidCron, func(c *Config) { c.Chip.Sync.Cron = "0 25 * * *" }},
		{"corporate cron", "CORPORATE_ACTION_CRON", RuleInvalidCron, func(c *Config) { c.CorporateAction.Cron = "30 6 * *" }},
		{"stock cron", "STOCK_SYMBOLS_CRON", RuleInvalidCron, func(c *Config) { c.StockSymbols.Cron = "bad" }},
		{"sr analysis cron", "SR_ANALYSIS_CHIP_CRON", RuleInvalidCron, func(c *Config) { c.SRAnalysis.ChipCron = "x" }},
		{"sr timeframe", "SR_ANALYSIS_TIMEFRAME", RuleInvalidValue, func(c *Config) { c.SRAnalysis.Timeframe = "daily" }},
		{"eval days", "EVALUATION_UNIVERSE_DAYS", RuleOutOfRange, func(c *Config) { c.EvaluationUniverse.Days = 0 }},
		{"delisting url", "DELISTING_URL", RuleInvalidURL, func(c *Config) { c.Delisting.URL = "" }},
		{"yahoo rate", "YAHOO_RATE_LIMIT", RuleOutOfRange, func(c *Config) { c.Yahoo.RateLimit = 0 }},
		{"stock timeout negative", "STOCK_SYMBOLS_TIMEOUT_SEC", RuleOutOfRange, func(c *Config) { c.StockSymbols.TimeoutSec = -1 }},
		{"position value zero", "POSITION_ANALYSIS_MAX_POSITION_VALUE", RuleOutOfRange, func(c *Config) { c.PositionAnalysis.MaxPositionValue = 0 }},
		{"take profit > 1", "POSITION_ANALYSIS_TAKE_PROFIT_REDUCTION_RATIO", RuleOutOfRange, func(c *Config) { c.PositionAnalysis.TakeProfitReductionRatio = 1.5 }},
		{"add on zero", "POSITION_ANALYSIS_ADD_ON_RATIO", RuleOutOfRange, func(c *Config) { c.PositionAnalysis.AddOnRatio = 0 }},
		{"sr reuse zero", "POSITION_ANALYSIS_SR_REUSE_MAX_AGE_HOURS", RuleOutOfRange, func(c *Config) { c.PositionAnalysis.SRReuseMaxAgeHours = 0 }},
	}
	for _, c := range cases {
		for _, env := range []string{"", AppEnvDev, AppEnvTest} {
			cfg := validConfig(env)
			c.mut(cfg)
			if v := Validate(cfg); !has(v, c.key, c.rule) {
				t.Errorf("%s（APP_ENV=%q）：要擋下 %s:%s，得到 %s", c.name, env, c.key, c.rule, JoinViolations(v))
			}
		}
	}
}

func TestValidate_DisabledFeaturesNotChecked(t *testing.T) {
	cfg := validConfig("")
	cfg.SRAnalysis = SRAnalysisConfig{Enabled: false, Cron: "bad", Timeframe: "?"}
	cfg.EvaluationUniverse = EvaluationUniverseConfig{Enabled: false, Cron: "bad"}
	cfg.Delisting = DelistingConfig{Enabled: false}
	cfg.StockSymbols = StockSymbolsConfig{Enabled: false, Cron: "bad"}
	cfg.SREvaluation = SREvaluationConfig{Enabled: false, Cron: "bad"}
	cfg.CorporateAction.Cron = "" // 空白沿用 scheduler 的預設
	if v := Validate(cfg); len(v) != 0 {
		t.Fatalf("停用的功能與空白的公司行動 cron 都要放行，得到 %s", JoinViolations(v))
	}
}

// 違反清單⛔ 不能帶出值、長度或片段。
func TestValidate_ViolationsNeverContainValues(t *testing.T) {
	const marker = "Mk$9#'\"q"
	cfg := validConfig("")
	cfg.Auth.JWTSecret = marker                       // too_short
	cfg.Database.DSN = "postgres://u:" + marker + "x" // 解析不了（密碼含特殊字元）或通過，兩種都⛔ 不能帶出 marker
	cfg.Python.ServiceURL = marker
	cfg.SRAnalysis.Cron = marker
	cfg.Server.Port = marker
	out := JoinViolations(Validate(cfg))
	if strings.Contains(out, marker) || strings.Contains(out, "Mk$9") {
		t.Fatalf("違反清單帶出了值：%s", out)
	}
	if strings.ContainsAny(out, "0123456789") {
		t.Fatalf("違反清單⛔ 不能帶出長度之類的數字：%s", out)
	}
}

// isolatePGEnv 讓 pgconn.ParseConfig 不受執行環境的 PG* 變數與 ~/.pgpass 影響。
func isolatePGEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"PGUSER", "PGPASSWORD", "PGHOST", "PGDATABASE", "PGSERVICE", "PGSERVICEFILE"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	t.Setenv("PGPASSFILE", filepath.Join(t.TempDir(), "no-passfile"))
}

// dsnCredentials 要跟 pgx 解析的結果一致（I-132 E 的 review：自行解析會跟 driver 不一致）。
func TestDSNCredentials(t *testing.T) {
	isolatePGEnv(t)
	cases := []struct {
		dsn, user, pass string
		ok              bool
	}{
		{"postgres://a:b@h:5432/d?sslmode=disable", "a", "b", true},
		{"postgres://db/trading?user=app&password=trading", "app", "trading", true},
		{"postgres://a:gen@db/trading?password=trading", "a", "trading", true},
		{"host=h user=u password='generated passphrase' dbname=d", "u", "generated passphrase", true},
		{`host=h user=u password='it\'s' dbname=d`, "u", "it's", true},
		{"host=h user=u password=p dbname=d", "u", "p", true},
		// MySQL 寫法：沒有 `=` 時 pgx 解析不了。帶 `?parseTime=true` 時 pgx 會當成 key=value、
		// 把整段當成未知的 runtime 參數而解析成功（帳密不是 root:pw）；這裡跟 driver 一致、⛔ 不另外擋，
		// 那種 DSN 在連線時才失敗。
		{"root:pw@tcp(127.0.0.1:3306)/d", "", "", false},
		{"garbage", "", "", false},
	}
	for _, c := range cases {
		u, p, ok := dsnCredentials(c.dsn)
		if ok != c.ok || (ok && (u != c.user || p != c.pass)) {
			t.Errorf("dsnCredentials(%q) = %q %q %v，要 %q %q %v", c.dsn, u, p, ok, c.user, c.pass, c.ok)
		}
	}
}

// driver 最終用的帳密也包含環境變數與 passfile：DSN 沒寫密碼時，這兩處的範例密碼也要擋下。
func TestDSNCredentials_EnvAndPassfile(t *testing.T) {
	isolatePGEnv(t)
	t.Setenv("PGPASSWORD", "trading")
	if _, p, ok := dsnCredentials("host=db user=someone dbname=d"); !ok || p != "trading" {
		t.Fatalf("要取到 PGPASSWORD，得到 %q %v", p, ok)
	}
	os.Unsetenv("PGPASSWORD")
	pf := filepath.Join(t.TempDir(), "pgpass")
	if err := os.WriteFile(pf, []byte("db:5432:d:someone:password\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PGPASSFILE", pf)
	if _, p, ok := dsnCredentials("host=db user=someone dbname=d"); !ok || p != "password" {
		t.Fatalf("要取到 passfile 的密碼，得到 %q %v", p, ok)
	}
	cfg := validConfig(AppEnvProduction)
	cfg.Database.DSN = "host=db user=someone dbname=d"
	if !has(Validate(cfg), "DATABASE_DSN", RulePublicExample) {
		t.Fatal("passfile 的範例密碼要擋下")
	}
}

// review 的第二個案例：driver 接受的引號寫法、產生的密碼 ⛔ 不能被判違反而擋住啟動。
func TestValidate_QuotedGeneratedPasswordPasses(t *testing.T) {
	isolatePGEnv(t)
	cfg := validConfig(AppEnvProduction)
	cfg.Database.DSN = "host=db user=app password='generated passphrase' dbname=trading"
	if vs := Validate(cfg); len(vs) != 0 {
		t.Fatalf("合法的引號寫法要通過，得到 %s", JoinViolations(vs))
	}
}

// backend/config.yaml 裡寫過的範例值都要在清單裡（repo 其他檔案由 scripts/check-config-examples.py 檢查）。
func TestPublicExamples_CoverConfigYAML(t *testing.T) {
	b, err := os.ReadFile("../../config.yaml")
	if err != nil {
		t.Fatalf("讀不到 backend/config.yaml：%v", err)
	}
	src := string(b)
	for _, m := range regexp.MustCompile(`jwt_secret:\s*"([^"]*)"`).FindAllStringSubmatch(src, -1) {
		if !contains(PublicJWTSecrets, m[1]) {
			t.Errorf("config.yaml 的 jwt_secret 範例值 %q 不在 PublicJWTSecrets", m[1])
		}
	}
	keys := append(append([]string{}, PublicFinMindKeys...), PublicFugleKeys...)
	for _, m := range regexp.MustCompile(`api_key:\s*"([^"]*)"`).FindAllStringSubmatch(src, -1) {
		if m[1] != "" && !contains(keys, m[1]) {
			t.Errorf("config.yaml 的 api_key 範例值 %q 不在清單", m[1])
		}
	}
	for _, m := range regexp.MustCompile(`://([^:/@\s"]+):([^@\s"]+)@|"([A-Za-z0-9_]+):([^@\s"]+)@tcp\(`).FindAllStringSubmatch(src, -1) {
		user, pass := m[1]+m[3], m[2]+m[4]
		if !contains(PublicDBUserPasswords, user+":"+pass) && !contains(PublicDBPasswords, pass) {
			t.Errorf("config.yaml 的 DSN 範例帳密 %s:%s 不在清單", user, pass)
		}
	}
}
