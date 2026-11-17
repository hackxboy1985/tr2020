package service

import (
	"fmt"
	"testing"
)

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

func TestIsUpstreamAssetNotFound(t *testing.T) {
	notFound := []error{
		fmt.Errorf("naxis upstream error 404: ResourceNotFound"),
		fmt.Errorf("asset not found"),
		fmt.Errorf("resource_not_found"),
	}
	for _, err := range notFound {
		if !isUpstreamAssetNotFound(err) {
			t.Fatalf("expected not-found for: %v", err)
		}
	}
	other := []error{
		nil,
		fmt.Errorf("naxis upstream error 401: invalid api key"),
		fmt.Errorf("naxis upstream error 500: internal error"),
	}
	for _, err := range other {
		if isUpstreamAssetNotFound(err) {
			t.Fatalf("must not treat as not-found: %v", err)
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
