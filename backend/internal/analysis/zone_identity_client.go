package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/trading/backend/internal/store"
)

// ZoneIdentityZone 是 matcher 需要的最小 zone 形狀。
// price_low/price_high **不是身分**，只是形狀——身分是 ZoneUID。
type ZoneIdentityZone struct {
	PriceLow  float64 `json:"price_low"`
	PriceHigh float64 `json:"price_high"`
	Method    string  `json:"method"`
	Role      string  `json:"role"`
	// 以下只有 previous 側會帶
	ZoneUID          string `json:"zone_uid,omitempty"`
	IncarnationRole  string `json:"incarnation_role,omitempty"`
	LastSeenAt       string `json:"last_seen_at,omitempty"` // YYYY-MM-DD
	ObservedAbsences int    `json:"observed_absences,omitempty"`
}

type zoneIdentityMatchRequest struct {
	AsOf string `json:"as_of,omitempty"` // YYYY-MM-DD
	// 市場交易日。**可以是降冪**——Python 端用 from_iterable 排序去重，
	// 所以 store.ListTradingDays 的輸出可直接放進來。
	TradingDays []string           `json:"trading_days,omitempty"`
	Previous    []ZoneIdentityZone `json:"previous"`
	Current     []ZoneIdentityZone `json:"current"`
}

// ZoneIdentityRelation / ZoneIdentityRoleTransition 對應 Python 的回應欄位。
type ZoneIdentityRelation struct {
	ParentZoneUID string `json:"parent_zone_uid"`
	ChildZoneUID  string `json:"child_zone_uid"`
	Relation      string `json:"relation"`
}

type ZoneIdentityRoleTransition struct {
	ZoneUID  string `json:"zone_uid"`
	Kind     string `json:"kind"`
	FromRole string `json:"from_role"`
	ToRole   string `json:"to_role"`
}

// ZoneIdentityMatchResult 是 /zone-identity/match 的回應。
//
// `ZoneUIDs[i]` 對應請求裡 `Current[i]` 被指派到的身分；
// `IncarnationRoles[i]` 是**下一輪**該帶回來的值——不要自己推導，
// 漏掉「翻轉後要前進」會讓連續翻轉的第二次靜默消失。
type ZoneIdentityMatchResult struct {
	ZoneUIDs             []string                     `json:"zone_uids"`
	IncarnationRoles     []*string                    `json:"incarnation_roles"`
	Relations            []ZoneIdentityRelation       `json:"relations"`
	RoleTransitions      []ZoneIdentityRoleTransition `json:"role_transitions"`
	UnmatchedPrevious    []string                     `json:"unmatched_previous"`
	TerminatedPrevious   []string                     `json:"terminated_previous"`
	ExpiredPrevious      []string                     `json:"expired_previous"`
	NextObservedAbsences map[string]int               `json:"next_observed_absences"`
}

// MatchZoneIdentities 呼叫 Python 的 /zone-identity/match（T-048 階段 B 接線）。
//
// **刻意是獨立端點而不是併進 /sr-zones**：階段 B 只寫不讀，沒有任何決策依賴它的輸出；
// 併進去就得動 scoring.py / pipeline.py 那條有決策責任的核心路徑，
// 為一個還沒有讀者的功能去動它，風險與收益不成比例。
func (c *Client) MatchZoneIdentities(
	ctx context.Context,
	asOf string,
	tradingDays []string,
	previous, current []ZoneIdentityZone,
) (*ZoneIdentityMatchResult, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("python service url not configured")
	}

	body, err := json.Marshal(zoneIdentityMatchRequest{
		AsOf:        asOf,
		TradingDays: tradingDays,
		Previous:    previous,
		Current:     current,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/zone-identity/match", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.srZonesHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("python zone-identity request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("python zone-identity returned %d", resp.StatusCode)
	}

	var out ZoneIdentityMatchResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode zone-identity response: %w", err)
	}
	if len(out.ZoneUIDs) != len(current) {
		// 長度不符代表 contract 對不上，硬吃下去會讓 zone 與身分錯位——
		// 那種錯誤在資料裡看起來完全正常。
		return nil, fmt.Errorf("zone-identity: 回傳 %d 個身分但送出 %d 個 zone",
			len(out.ZoneUIDs), len(current))
	}
	return &out, nil
}

// ZoneIdentityZonesFromLive 把 repo 撈出來的既有身分轉成請求形狀。
func ZoneIdentityZonesFromLive(live []store.LiveZone) []ZoneIdentityZone {
	out := make([]ZoneIdentityZone, 0, len(live))
	for _, z := range live {
		out = append(out, ZoneIdentityZone{
			PriceLow:         z.PriceLow,
			PriceHigh:        z.PriceHigh,
			Method:           z.Method,
			Role:             lastObservedRole(z),
			ZoneUID:          z.ZoneUID,
			IncarnationRole:  z.IncarnationRole.String,
			LastSeenAt:       z.LastSeenAt.Format("2006-01-02"),
			ObservedAbsences: z.ObservedAbsences,
		})
	}
	return out
}

// **`role` 與 `incarnation_role` 是兩個不同的東西，不能拿後者代替前者。**
//
// `role` 是上次觀測到的角色，`incarnation_role` 是當前這一世的角色。用一世的角色
// 當 role 的話，一個已經在 AT_ZONE 好幾次的 zone 每次都會被看成「這次才從 SUPPORT
// 進 AT_ZONE」，於是每次分析都重複記一筆 ROLE_UNRESOLVED（migration 067 提到的
// 那條連續 16 次 AT_ZONE 的鏈會變成 16 筆相同紀錄）；而它回到 SUPPORT 時，
// matcher 的「prev.role == AT_ZONE」永遠不成立，對應的 ROLE_RESOLVED 也就永遠不會寫。
// 轉換流水會變成沒有配對的重複雜訊——正好與「分開記才不會讓真正的翻轉被淹沒」相反。
func lastObservedRole(z store.LiveZone) string {
	if z.LastRole != "" {
		return z.LastRole
	}
	return "AT_ZONE"
}
