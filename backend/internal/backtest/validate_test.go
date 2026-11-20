package backtest

import (
	"encoding/json"
	"os"
	"testing"
)

func TestValidateRequest(t *testing.T) {
	base := CreateRequest{Strategy: "breakout_swing_atr_v1", Symbols: []string{"2330"}, StartDate: "2025-01-01", EndDate: "2025-12-31"}
	with := func(f func(r *CreateRequest)) CreateRequest {
		r := base
		r.Symbols = append([]string(nil), base.Symbols...)
		f(&r)
		return r
	}
	cases := []struct {
		name string
		req  CreateRequest
		want string
	}{
		{"合法", base, ""},
		{"同一天", with(func(r *CreateRequest) { r.StartDate, r.EndDate = "2025-06-02", "2025-06-02" }), ""},
		{"最大日期", with(func(r *CreateRequest) { r.StartDate, r.EndDate = "2099-12-31", "2099-12-31" }), ""},
		{"格式錯", with(func(r *CreateRequest) { r.StartDate = "2025/01/01" }), "invalid_date:start_date"},
		{"少補零", with(func(r *CreateRequest) { r.StartDate = "2025-1-1" }), "invalid_date:start_date"},
		{"日期不存在", with(func(r *CreateRequest) { r.StartDate = "2025-02-30" }), "invalid_date:start_date"},
		{"9999-12-31 加一天會溢位", with(func(r *CreateRequest) { r.EndDate = "9999-12-31" }), "invalid_date:end_date"},
		{"晚於最大日期", with(func(r *CreateRequest) { r.EndDate = "2100-01-01" }), "invalid_date:end_date"},
		{"開始晚於結束", with(func(r *CreateRequest) { r.StartDate, r.EndDate = "2025-06-02", "2025-06-01" }), "start_after_end"},
		{"modular 多檔", with(func(r *CreateRequest) { r.Symbols = []string{"A", "B"} }), ""},
		{"標的數超過", with(func(r *CreateRequest) { r.Symbols = []string{"A", "B", "C"} }), "too_many_symbols:3:2"},
		{"backtrader 多檔", with(func(r *CreateRequest) { r.Strategy, r.Symbols = "breakout_v1", []string{"A", "B"} }), "backtrader_single_symbol:2"},
		{"backtrader 單檔", with(func(r *CreateRequest) { r.Strategy = "breakout_v1" }), ""},
	}
	for _, c := range cases {
		if got := ValidateRequest(c.req, 2); got != c.want {
			t.Errorf("%s：ValidateRequest = %q，預期 %q", c.name, got, c.want)
		}
	}
}

// 跟 python 的 tests/data/backtest_job_error_cases.json 內容相同（scripts/check-backtest-contract.py 比對）。
// Go 的 error 欄位是字串（NULL 讀成空字串），所以 null 的案例當成空字串。
func TestPublicJobErrorSharedCases(t *testing.T) {
	raw, err := os.ReadFile("testdata/job_error_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Input *string `json:"input"`
		Want  *string `json:"want"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("案例檔是空的")
	}
	deref := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	for _, c := range cases {
		if got := PublicJobError(deref(c.Input)); got != deref(c.Want) {
			t.Errorf("PublicJobError(%q) = %q，預期 %q", deref(c.Input), got, deref(c.Want))
		}
	}
}

// 跟 python 的 tests/data/backtest_request_validation_cases.json 內容相同（scripts/check-backtest-contract.py 比對）。
func TestValidateRequestSharedCases(t *testing.T) {
	raw, err := os.ReadFile("testdata/request_validation_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name       string   `json:"name"`
		Strategy   string   `json:"strategy"`
		Symbols    []string `json:"symbols"`
		StartDate  string   `json:"start_date"`
		EndDate    string   `json:"end_date"`
		MaxSymbols int      `json:"max_symbols"`
		Want       string   `json:"want"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("案例檔是空的")
	}
	for _, c := range cases {
		got := ValidateRequest(CreateRequest{Strategy: c.Strategy, Symbols: c.Symbols, StartDate: c.StartDate, EndDate: c.EndDate}, c.MaxSymbols)
		if got != c.Want {
			t.Errorf("%s：ValidateRequest = %q，預期 %q", c.Name, got, c.Want)
		}
	}
}
