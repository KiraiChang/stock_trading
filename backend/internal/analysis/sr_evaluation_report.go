package analysis

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// SR evaluation job 的共用小工具。scheduler（cron）與 api/handler（手動觸發）走的是
// 同一條 job 生命週期，這些 helper 原本在兩個套件各有一份完全相同的副本
// 統一收在這裡。

// NewEvaluationJobID 產生 evaluation job 的識別碼。
//
// 時間戳只到毫秒，手動 API 與 cron 若在同一毫秒觸發會撞上 job_id 的 UNIQUE 約束，
// 所以後面補 4 bytes 隨機值。rand 讀取失敗時退回純時間戳——碰撞機率遠低於讓整個
// job 建立失敗的代價。
func NewEvaluationJobID() string {
	stamp := time.Now().UTC().Format("20060102_150405_000")
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "sr_eval_job_" + stamp
	}
	return "sr_eval_job_" + stamp + "_" + hex.EncodeToString(buf)
}

// StringFromReport 取 Python report 的字串欄位；缺 key 或型別不符一律回空字串，
// 不讓一個欄位問題中斷整個 job 的收尾。
func StringFromReport(report map[string]any, key string) string {
	value, ok := report[key].(string)
	if !ok {
		return ""
	}
	return value
}

// IntFromReport 取 Python report 的整數欄位。走 encoding/json 的來源一律是 float64，
// int 分支是給直接以 Go map 組出來的呼叫端（例如測試）用的。
func IntFromReport(report map[string]any, key string) int {
	switch value := report[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}
