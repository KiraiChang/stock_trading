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
