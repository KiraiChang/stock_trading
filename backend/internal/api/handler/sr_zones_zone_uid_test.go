package handler

import (
	"context"
	"database/sql"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/store"
)

// T-048 階段 E：zone 身分要跟著 zones 一次寫入 stock_sr_zones。
//
// applyZoneUIDs 的錯誤在資料裡看起來完全正常——錯位的身分只是「這個 zone 對到另一個
// 身分」，任何查詢都不會報錯，而血緣圖會開始說謊。所以它是純函數，在這裡逐條釘住。

func TestApplyZoneUIDsAssignsByIndex(t *testing.T) {
	zones := []store.SRZone{
		srZone(100.0, 101.0, "SUPPORT"),
		srZone(110.0, 111.0, "RESISTANCE"),
	}
	m := &zoneIdentityMatch{matched: &analysis.ZoneIdentityMatchResult{
		ZoneUIDs: []string{"Z-1", "Z-2"},
	}}

	applyZoneUIDs(zones, m)

	for i, want := range []string{"Z-1", "Z-2"} {
		if !zones[i].ZoneUID.Valid || zones[i].ZoneUID.String != want {
			t.Fatalf("zones[%d].ZoneUID = %+v, want %s", i, zones[i].ZoneUID, want)
		}
	}
}

// 比對失敗（matchZoneIdentity 回 nil）時 zone_uid 必須留空，而不是沿用上一次或猜一個。
// 這是階段 E 的 fail-open：身分掛掉不影響分析本身。
func TestApplyZoneUIDsLeavesZonesEmptyWhenMatchMissing(t *testing.T) {
	cases := []struct {
		name  string
		match *zoneIdentityMatch
	}{
		{"沒有比對結果", nil},
		{"有結構但 matcher 沒回東西", &zoneIdentityMatch{}},
		{"uid 是空字串", &zoneIdentityMatch{matched: &analysis.ZoneIdentityMatchResult{
			ZoneUIDs: []string{""},
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			zones := []store.SRZone{srZone(100.0, 101.0, "SUPPORT")}

			applyZoneUIDs(zones, tc.match)

			if zones[0].ZoneUID.Valid {
				t.Fatalf("zone_uid 應留空，得到 %q", zones[0].ZoneUID.String)
			}
		})
	}
}

// **長度對不上時寧可留空也不猜**：matcher 的輸出與 zones 是索引對齊的，
// 一旦錯位，後面每個 zone 都會掛到別人的身分上。
func TestApplyZoneUIDsStopsWhenMatchIsShorterThanZones(t *testing.T) {
	zones := []store.SRZone{
		srZone(100.0, 101.0, "SUPPORT"),
		srZone(110.0, 111.0, "RESISTANCE"),
		srZone(120.0, 121.0, "RESISTANCE"),
	}
	m := &zoneIdentityMatch{matched: &analysis.ZoneIdentityMatchResult{
		ZoneUIDs: []string{"Z-1", "Z-2"},
	}}

	applyZoneUIDs(zones, m)

	if !zones[0].ZoneUID.Valid || !zones[1].ZoneUID.Valid {
		t.Fatalf("前兩個應照常指派，得到 %+v / %+v", zones[0].ZoneUID, zones[1].ZoneUID)
	}
	if zones[2].ZoneUID.Valid {
		t.Fatalf("超出 matcher 輸出的 zone 必須留空，得到 %q", zones[2].ZoneUID.String)
	}
}

// 比對失敗時寫入段要安靜地什麼都不做——不能 panic，也不能回一個空的 outcome
// 讓事件層以為身分寫成功了。
func TestPersistZoneIdentityIsNoOpWithoutMatch(t *testing.T) {
	h := &SRZoneHandler{}
	zones := []store.SRZone{srZone(100.0, 101.0, "SUPPORT")}

	for _, m := range []*zoneIdentityMatch{nil, {}} {
		if got := h.persistZoneIdentity(context.Background(), "0050", 1, zones, m); got != nil {
			t.Fatalf("沒有比對結果時應回 nil，得到 %+v", got)
		}
	}
}

// 對外 payload 也要看得到身分（T-048 階段 E 目標 2）。
//
// **這條測的是白名單有沒有漏欄位。** zones[].data 是手工組的 gin.H，不是把 SRZone
// marshal 出去——所以 struct 上的 `json:"zone_uid"` tag 完全擋不住漏掉這一鍵，而漏掉的
// 外觀是「這個 zone 沒有身分」，與降級後的 NULL 一模一樣，從回應上看不出差別。
func TestSRZonePipelineResponseExposesZoneUID(t *testing.T) {
	withUID := srZone(100.0, 101.0, "SUPPORT")
	withUID.ZoneUID = store.NullString{NullString: sql.NullString{String: "Z-abc", Valid: true}}

	resp := srZonePipelineResponse(&srZonePipelineSnapshot{
		Analysis: &store.SRZoneAnalysis{ID: 1, Symbol: "0050", Timeframe: "1d"},
		// 第二個 zone 不帶身分：降級與早於 069 的舊資料都長這樣，要回 JSON null
		// 而不是讓整個鍵消失。
		Zones:  []store.SRZone{withUID, srZone(110.0, 111.0, "RESISTANCE")},
		Status: gin.H{},
	})

	items, ok := resp["zones"].([]gin.H)
	if !ok || len(items) != 2 {
		t.Fatalf("zones = %#v, want 2 筆", resp["zones"])
	}
	for i, want := range []store.NullString{
		{NullString: sql.NullString{String: "Z-abc", Valid: true}},
		{},
	} {
		got, ok := items[i]["data"].(gin.H)["zone_uid"]
		if !ok {
			t.Fatalf("zones[%d].data 沒有 zone_uid 這個鍵", i)
		}
		if got != any(want) {
			t.Errorf("zones[%d].data.zone_uid = %+v, want %+v", i, got, want)
		}
	}
}
