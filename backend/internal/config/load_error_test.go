package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellh/mapstructure"
)

// 含特殊字元的假機密：任何輸出都⛔ 不能帶出它或它的片段。
const fakeSecret = "S3cr3t-Mk$9#q"

func inTempDir(t *testing.T, configYAML string) {
	t.Helper()
	dir := t.TempDir()
	if configYAML != "" {
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configYAML), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
}

func asLoadError(t *testing.T, err error) *LoadError {
	t.Helper()
	var le *LoadError
	if !errors.As(err, &le) {
		t.Fatalf("要回傳 *LoadError，得到 %T", err)
	}
	return le
}

func assertNoSecret(t *testing.T, s string) {
	t.Helper()
	for _, frag := range []string{fakeSecret, "S3cr3t", "Mk$9"} {
		if strings.Contains(s, frag) {
			t.Fatalf("輸出帶出了假機密（%q）：%s", frag, s)
		}
	}
}

func TestLoad_TypeErrorIsSanitized(t *testing.T) {
	inTempDir(t, "server:\n  port: \"8080\"\n")
	t.Setenv("JOB_RETRY_ENABLED", fakeSecret)
	_, err := Load()
	le := asLoadError(t, err)
	if !has(le.Violations, "JOB_RETRY_ENABLED", RuleInvalidType) {
		t.Fatalf("要回報 JOB_RETRY_ENABLED:invalid_type，得到 %s", JoinViolations(le.Violations))
	}
	assertNoSecret(t, err.Error())
	assertNoSecret(t, JoinViolations(le.Violations))
}

func TestLoad_MissingFileIsSanitized(t *testing.T) {
	inTempDir(t, "")
	_, err := Load()
	le := asLoadError(t, err)
	if !has(le.Violations, "config_file", RuleUnreadable) {
		t.Fatalf("要回報 config_file:unreadable，得到 %s", JoinViolations(le.Violations))
	}
}

func TestLoad_BrokenYAMLIsSanitized(t *testing.T) {
	inTempDir(t, "auth:\n  jwt_secret: \""+fakeSecret+"\"\nserver: [\n  port: x\n")
	_, err := Load()
	le := asLoadError(t, err)
	if !has(le.Violations, "config_file", RuleInvalid) {
		t.Fatalf("要回報 config_file:invalid，得到 %s", JoinViolations(le.Violations))
	}
	assertNoSecret(t, err.Error())
}

func TestLoad_OKAndAppEnv(t *testing.T) {
	inTempDir(t, "server:\n  port: \"8080\"\n")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("合法的設定檔要載入成功：%v", err)
	}
	if cfg.AppEnv != "" || !cfg.IsProduction() {
		t.Fatalf("沒設定 APP_ENV 要當成 production，得到 %q", cfg.AppEnv)
	}
	t.Setenv("APP_ENV", "dev")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppEnv != AppEnvDev || cfg.IsProduction() {
		t.Fatalf("APP_ENV=dev 要讀得到，得到 %q", cfg.AppEnv)
	}
}

// safeDecodeError 抓不到已知的鍵時，⛔ 不能把引號包住的值當成鍵名輸出。
func TestSafeDecodeError_UnknownQuotedTokenIsNotEchoed(t *testing.T) {
	err := safeDecodeError(errors.New("x"), []string{"job_retry.enabled"})
	le := asLoadError(t, err)
	if JoinViolations(le.Violations) != "config:invalid_type" {
		t.Fatalf("非 mapstructure 的錯誤要回報 config:invalid_type，得到 %s", JoinViolations(le.Violations))
	}
}

func TestSafeDecodeError_QuotedValueNotTreatedAsKey(t *testing.T) {
	// 第一個引號包住的是值而⛔ 不是已知的鍵：⛔ 不能輸出它
	me := &mapstructure.Error{Errors: []string{"'hunter2value' expected type 'bool', got 'x'"}}
	le := asLoadError(t, safeDecodeError(me, []string{"job_retry.enabled"}))
	out := JoinViolations(le.Violations)
	if out != "config:invalid_type" || strings.Contains(strings.ToLower(out), "hunter2") {
		t.Fatalf("未知的引號內容要回報 config:invalid_type，得到 %s", out)
	}
	// 已知的鍵（含 slice 索引）要轉成環境變數名稱
	me = &mapstructure.Error{Errors: []string{"'stock_symbols.sync_urls[0]' expected type 'string'"}}
	le = asLoadError(t, safeDecodeError(me, []string{"stock_symbols.sync_urls"}))
	if JoinViolations(le.Violations) != "STOCK_SYMBOLS_SYNC_URLS:invalid_type" {
		t.Fatalf("已知鍵要轉成環境變數名稱，得到 %s", JoinViolations(le.Violations))
	}
}
