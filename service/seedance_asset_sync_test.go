package service

import "testing"

func TestNormalizeAssetRef(t *testing.T) {
	cases := map[string]string{
		"asset://ast_abc":   "ast_abc",
		" asset://asset-x ": "asset-x",
		"ast_abc":           "ast_abc",
		"asset://":          "",
		"   ":               "",
	}
	for input, want := range cases {
		if got := NormalizeAssetRef(input); got != want {
			t.Fatalf("NormalizeAssetRef(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestAssetStatusForStorage(t *testing.T) {
	cases := map[string]string{
		"active":     "Active",
		"Active":     "Active",
		"processing": "Processing",
		"failed":     "Failed",
		"archived":   "Archived",
		"":           "",
	}
	for input, want := range cases {
		if got := assetStatusForStorage(input); got != want {
			t.Fatalf("assetStatusForStorage(%q) = %q, want %q", input, got, want)
		}
	}
}
