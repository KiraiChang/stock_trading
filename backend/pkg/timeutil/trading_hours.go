package timeutil

import "time"

var TaipeiTZ *time.Location

func init() {
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	TaipeiTZ = loc
}

func IsMarketOpen(t time.Time) bool {
	local := t.In(TaipeiTZ)
	if local.Weekday() == time.Saturday || local.Weekday() == time.Sunday {
		return false
	}
	h, m, _ := local.Clock()
	totalMin := h*60 + m
	return totalMin >= 9*60 && totalMin <= 13*60+30
}

func TodayTaipei() time.Time {
	now := time.Now().In(TaipeiTZ)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, TaipeiTZ)
}
