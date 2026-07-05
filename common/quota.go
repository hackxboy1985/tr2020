package common

import "math"

func GetTrustQuota() int {
	return int(10 * QuotaPerUnit)
}

// YuanToQuota 将人民币金额（元）换算为本系统积分
// 1元 = 1$ = QuotaPerUnit 积分
func YuanToQuota(yuan float64) int {
	return int(math.Round(yuan * QuotaPerUnit))
}

// QuotaToYuan 将本系统积分换算为人民币金额（元）
func QuotaToYuan(quota int) float64 {
	return float64(quota) / QuotaPerUnit
}
