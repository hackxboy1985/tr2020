package doubao

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

// applyOtherRatios 复刻 relay.RelayTaskSubmit 第 6 步的额度乘算逻辑。
// quotaYuanEqual 按 0.01 元（即分）精度比较最终价格，容忍整数截断误差。
func quotaYuanEqual(a, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < 0.01
}

func applyOtherRatios(baseQuota int, ratios map[string]float64) int {
	quota := baseQuota
	for _, ra := range ratios {
		if ra != 1.0 {
			quota = int(float64(quota) * ra)
		}
	}
	return quota
}

// TestQuotaEndToEnd 校验「模型-分辨率-是否带视频」到最终扣费额度的完整换算。
//
// 以 doubao-seedance-2.5 为例（基准倍率配置为 480p/720p 不含视频 = 70）：
//
//	480p/720p 不含视频 → 70（无倍率）
//	1080p     不含视频 → 77
//	480p/720p 含视频   → 42
//	1080p     含视频   → 46
func TestQuotaEndToEnd(t *testing.T) {
	cases := []struct {
		name       string
		model      string
		resolution string
		hasVideo   bool
		wantYuan   float64
	}{
		{"2.5/480p/不含视频", model25, "480p", false, 70.0},
		{"2.5/720p/不含视频", model25, "720p", false, 70.0},
		{"2.5/1080p/不含视频", model25, "1080p", false, 77.0},
		{"2.5/480p/含视频", model25, "480p", true, 42.0},
		{"2.5/720p/含视频", model25, "720p", true, 42.0},
		{"2.5/1080p/含视频", model25, "1080p", true, 46.0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baselineWithout, baselineWith, ok := GetVideoResolutionPrice(tc.model, ResolutionBaseline)
			if !ok {
				t.Fatalf("缺少基准档配置")
			}
			// 基准档不含视频价作为管理员配置的 ModelRatio
			baseQuota := int(baselineWithout * common.QuotaPerUnit)
			_ = baselineWith

			priceWithout, priceWith, ok := GetVideoResolutionPrice(tc.model, tc.resolution)
			if !ok {
				t.Fatalf("缺少价格配置")
			}

			// 复刻 EstimateBilling 的倍率产出
			ratios := map[string]float64{}
			if r := priceWithout / baselineWithout; r != 1.0 {
				ratios["resolution"] = r
			}
			if tc.hasVideo {
				ratios["video_input"] = priceWith / priceWithout
			}

			gotQuota := applyOtherRatios(baseQuota, ratios)
			gotYuan := float64(gotQuota) / common.QuotaPerUnit
			if !quotaYuanEqual(gotYuan, tc.wantYuan) {
				t.Errorf("最终价格 got %.4f 元, want %.4f 元 (ratios=%v)", gotYuan, tc.wantYuan, ratios)
			}
		})
	}
}

// TestQuotaEndToEndSeedance2 校验 seedance-2.0 系列（含 4k 档与 fast/mini 档）。
func TestQuotaEndToEndSeedance2(t *testing.T) {
	cases := []struct {
		name       string
		model      string
		resolution string
		hasVideo   bool
		wantYuan   float64
	}{
		{"2.0/480p/不含视频", model20, "480p", false, 46.0},
		{"2.0/720p/不含视频", model20, "720p", false, 46.0},
		{"2.0/1080p/不含视频", model20, "1080p", false, 51.0},
		{"2.0/4k/不含视频", model20, "4k", false, 26.0},
		{"2.0/1080p/含视频", model20, "1080p", true, 31.0},
		{"2.0/4k/含视频", model20, "4k", true, 16.0},
		{"2.0-fast/720p/不含视频", modelFast, "720p", false, 37.0},
		{"2.0-fast/720p/含视频", modelFast, "720p", true, 22.0},
		{"2.0-mini/720p/不含视频", modelMini, "720p", false, 23.0},
		{"2.0-mini/720p/含视频", modelMini, "720p", true, 14.0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baselineWithout, _, ok := GetVideoResolutionPrice(tc.model, ResolutionBaseline)
			if !ok {
				t.Fatalf("缺少基准档配置")
			}
			baseQuota := int(baselineWithout * common.QuotaPerUnit)

			priceWithout, priceWith, ok := GetVideoResolutionPrice(tc.model, tc.resolution)
			if !ok {
				t.Fatalf("缺少价格配置")
			}

			ratios := map[string]float64{}
			if r := priceWithout / baselineWithout; r != 1.0 {
				ratios["resolution"] = r
			}
			if tc.hasVideo {
				ratios["video_input"] = priceWith / priceWithout
			}

			gotYuan := float64(applyOtherRatios(baseQuota, ratios)) / common.QuotaPerUnit
			if !quotaYuanEqual(gotYuan, tc.wantYuan) {
				t.Errorf("最终价格 got %.4f 元, want %.4f 元 (ratios=%v)", gotYuan, tc.wantYuan, ratios)
			}
		})
	}
}
