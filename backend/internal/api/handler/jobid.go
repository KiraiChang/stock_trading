package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// newJobID 產生背景任務的識別碼：<prefix>_<UTC 時間到毫秒>_<4 位隨機碼>。
//
// 隨機碼是必要的：先前只有時間戳（精度到毫秒），同一毫秒內進來的兩個請求會產生
// 完全相同的 job_id，撞上 job 表的 UNIQUE constraint 讓後到的那筆回 500。
// 機率低但不是零（前端連點、或排程與手動觸發同時發生），而且失敗方式很難查。
func newJobID(prefix string) string {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand 失敗在實務上等同系統故障；退回純時間戳仍可用，
		// 只是回到「同毫秒會撞號」的舊行為，不值得讓建立任務整個失敗。
		return fmt.Sprintf("%s_%s", prefix, time.Now().UTC().Format("20060102_150405_000"))
	}
	return fmt.Sprintf("%s_%s_%s", prefix, time.Now().UTC().Format("20060102_150405_000"), hex.EncodeToString(b[:]))
}
