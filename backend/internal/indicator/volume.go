package indicator

const VolumeSpikeMultiplier = 2.0

type VolumeResult struct {
	MA20    int64
	Ratio   float64
	IsSpike bool
}

func CalcVolumeSpike(volumes []int64, period int) VolumeResult {
	if len(volumes) < period+1 {
		return VolumeResult{}
	}

	// 計算前 period 根的成交量平均（不含最新一根）
	slice := volumes[len(volumes)-period-1 : len(volumes)-1]
	var sum int64
	for _, v := range slice {
		sum += v
	}
	ma := sum / int64(period)
	current := volumes[len(volumes)-1]

	ratio := 0.0
	if ma > 0 {
		ratio = float64(current) / float64(ma)
	}

	return VolumeResult{
		MA20:    ma,
		Ratio:   ratio,
		IsSpike: ratio >= VolumeSpikeMultiplier,
	}
}
