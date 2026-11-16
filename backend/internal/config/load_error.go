package config

import (
	"errors"
	"io/fs"
	"regexp"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

// LoadError 是 Load() 失敗時回傳的錯誤，只帶「鍵名:代號」，⛔ 不帶輸入值或設定檔內容。
//
// **為什麼不能直接回傳原始錯誤**（I-132）：mapstructure 的型別轉換錯誤會帶出輸入值
// （例如 `cannot parse 'job_retry.enabled' as bool: strconv.ParseBool: parsing "…"`），
// viper 讀設定檔失敗的錯誤也可能帶出檔案內容的片段；main.go 會把錯誤寫進持久化 log。
type LoadError struct {
	Violations []Violation
}

func (e *LoadError) Error() string {
	return "config load failed: " + JoinViolations(e.Violations)
}

// mapstructure 錯誤訊息裡第一個單引號包住的是鍵名（可能帶 slice 的索引）。
// 抓到的名稱還要在 viper 已知的鍵裡才採用，避免把引號包住的**值**誤當鍵名輸出。
var mapstructureKeyRE = regexp.MustCompile(`'([A-Za-z0-9_.]+)(?:\[[0-9]+\])*'`)

func safeReadError(err error) error {
	var notFound viper.ConfigFileNotFoundError
	if errors.As(err, &notFound) {
		return &LoadError{Violations: []Violation{{Key: "config_file", Rule: RuleUnreadable}}}
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return &LoadError{Violations: []Violation{{Key: "config_file", Rule: RuleUnreadable}}}
	}
	return &LoadError{Violations: []Violation{{Key: "config_file", Rule: RuleInvalid}}}
}

func safeDecodeError(err error, knownKeys []string) error {
	known := make(map[string]bool, len(knownKeys))
	for _, k := range knownKeys {
		known[strings.ToLower(k)] = true
	}
	var me *mapstructure.Error
	if !errors.As(err, &me) {
		return &LoadError{Violations: []Violation{{Key: "config", Rule: RuleInvalidType}}}
	}
	seen := map[string]bool{}
	var out []Violation
	for _, msg := range me.Errors {
		key := "config"
		if m := mapstructureKeyRE.FindStringSubmatch(msg); m != nil && known[strings.ToLower(m[1])] {
			key = envName(m[1])
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, Violation{Key: key, Rule: RuleInvalidType})
		}
	}
	if len(out) == 0 {
		out = []Violation{{Key: "config", Rule: RuleInvalidType}}
	}
	return &LoadError{Violations: out}
}

// envName 把 viper 的鍵（`job_retry.enabled`）換成環境變數的名稱（`JOB_RETRY_ENABLED`）。
func envName(key string) string {
	return strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
}
