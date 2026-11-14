package service

import "testing"

// TestLegacyAdaptersDoNotImplementFaceVerification 守护老上游行为：
// 只有 n-axis 适配器实现 FaceVerificationAdapter，gateway/kwjm/sdos2 不实现，
// 从而 controller 里的人脸核验老逻辑（网关透传）保持原样。
func TestLegacyAdaptersDoNotImplementFaceVerification(t *testing.T) {
	adapters := []AssetAdapter{
		NewGatewayAssetAdapter("https://example.com", "k", false),
		NewKwjmAssetAdapter("https://example.com", "k", "sd-video-v2"),
		NewSdos2AssetAdapter("https://example.com", "k"),
	}
	for _, adapter := range adapters {
		if faceAdapter, ok := adapter.(FaceVerificationAdapter); ok {
			t.Fatalf("%T must not implement FaceVerificationAdapter, got %T", adapter, faceAdapter)
		}
	}

	// n-axis 两个协议入口都必须实现
	for _, adapter := range []AssetAdapter{
		NewNaxisAssetAdapter("https://example.com", "k"),
		NewNaxisActionAssetAdapter("https://example.com", "k"),
	} {
		if _, ok := adapter.(FaceVerificationAdapter); !ok {
			t.Fatalf("%T must implement FaceVerificationAdapter", adapter)
		}
	}
}
