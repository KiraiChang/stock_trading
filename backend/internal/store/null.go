package store

import (
	"database/sql"
	"encoding/json"
)

// NullFloat64 / NullString / NullTime 包裝 database/sql 對應型別，
// 補上 JSON 序列化：預設 sql.NullFloat64 這類型別會被序列化成
// {"Float64":960.2,"Valid":true} 這種內部結構，而不是單純的數字或 null，
// 前端拿到後對它呼叫 .toFixed() 之類的方法會直接炸掉。這裡用內嵌
// （不是型別別名）保留 Scan/Value 供 sqlx 讀寫 DB，另外加上
// MarshalJSON/UnmarshalJSON 讓 API 回應是乾淨的 `123.45` 或 `null`。

type NullFloat64 struct {
	sql.NullFloat64
}

func (n NullFloat64) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Float64)
}

func (n *NullFloat64) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Valid = false
		n.Float64 = 0
		return nil
	}
	if err := json.Unmarshal(data, &n.Float64); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

type NullString struct {
	sql.NullString
}

func (n NullString) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.String)
}

func (n *NullString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Valid = false
		n.String = ""
		return nil
	}
	if err := json.Unmarshal(data, &n.String); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

// RawJSON 存的是一段已經是合法 JSON 的文字（例如 SRZone.TradingScoreBreakdown），
// DB 欄位型別是 TEXT。刻意用 string 當底層型別，不是 []byte/json.RawMessage——
// pgx（postgres driver）綁定 []byte 參數時會送成 bytea，寫入 TEXT 欄位會直接
// 報型別不符的錯誤（sqlite/mysql 對這個比較寬容，不會出錯，但 postgres 會），
// 用 string 才能讓 sqlx 在三種資料庫方言下都正常寫入/讀出這個欄位。
// MarshalJSON 把內容原樣嵌入 API 回應（變成巢狀 JSON object），不是逃逸成
// 一個 JSON 字串，前端才能用 z.trading_score_breakdown.expected_value 直接取值。
type RawJSON string

func (r RawJSON) MarshalJSON() ([]byte, error) {
	if r == "" {
		return []byte("null"), nil
	}
	return []byte(r), nil
}

type NullTime struct {
	sql.NullTime
}

func (n NullTime) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Time)
}

func (n *NullTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Valid = false
		return nil
	}
	if err := json.Unmarshal(data, &n.Time); err != nil {
		return err
	}
	n.Valid = true
	return nil
}
