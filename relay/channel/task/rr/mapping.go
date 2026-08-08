package rr

import "strings"

// SizeConfig holds the mapped RR parameters for a given OpenAI size string.
type SizeConfig struct {
	AspectRatio string
	Resolution  string
}

// SizeMap maps OpenAI image size strings to RR aspectRatio + resolution.
// Edit this table directly to change mapping rules — no other code needs to change.
var SizeMap = map[string]SizeConfig{
	// 1:1
	"256x256":  {"1:1", "1k"},
	"512x512":  {"1:1", "1k"},
	"1024x1024": {"1:1", "1k"},
	"2048x2048": {"1:1", "2k"},
	"4096x4096": {"1:1", "4k"},
	// 16:9
	"1280x720":  {"16:9", "1k"},
	"1792x1024": {"16:9", "2k"},
	"1920x1080": {"16:9", "2k"},
	"3840x2160": {"16:9", "4k"},
	"4096x2304": {"16:9", "4k"},
	// 9:16
	"720x1280":  {"9:16", "1k"},
	"1024x1792": {"9:16", "2k"},
	"1080x1920": {"9:16", "2k"},
	"2160x3840": {"9:16", "4k"},
	"2304x4096": {"9:16", "4k"},
	// 4:3
	"1024x768":  {"4:3", "1k"},
	"1360x1024": {"4:3", "2k"},
	"2880x2160": {"4:3", "4k"},
	// 3:4
	"768x1024":  {"3:4", "1k"},
	"1024x1360": {"3:4", "2k"},
	"2160x2880": {"3:4", "4k"},
	// 2:1
	"2048x1024": {"2:1", "2k"},
	"4096x2048": {"2:1", "4k"},
	// 1:2
	"1024x2048": {"1:2", "2k"},
	"2048x4096": {"1:2", "4k"},
}

// DefaultSizeConfig is used when the input size is not in SizeMap.
var DefaultSizeConfig = SizeConfig{AspectRatio: "1:1", Resolution: "1k"}

// mapSize returns the SizeConfig for the given OpenAI size string.
// If size is in ratio format (e.g. "16:9"), it is used directly as AspectRatio
// and resolution defaults to "1k".
// Falls back to DefaultSizeConfig if the size is not recognized.
func mapSize(size string) SizeConfig {
	if cfg, ok := SizeMap[size]; ok {
		return cfg
	}
	// ratio format: contains ":" but no "x" (e.g. "16:9", "1:1", "9:16")
	if strings.Contains(size, ":") && !strings.Contains(size, "x") {
		return SizeConfig{AspectRatio: size, Resolution: "1k"}
	}
	return DefaultSizeConfig
}

// mapSizeWithResolution is like mapSize but allows an explicit resolution override.
// When size is in ratio format and resolution is provided, the resolution is used directly.
func mapSizeWithResolution(size, resolution string) SizeConfig {
	cfg := mapSize(size)
	if resolution != "" && strings.Contains(size, ":") && !strings.Contains(size, "x") {
		cfg.Resolution = resolution
	}
	return cfg
}

// QualityMap maps OpenAI quality strings to RR quality values.
var QualityMap = map[string]string{
	"standard": "medium",
	"hd":       "high",
	"":         "medium",
}

// mapQuality returns the RR quality string for a given OpenAI quality value.
// Known values are translated via QualityMap; unknown values are passed through as-is.
func mapQuality(quality string) string {
	if q, ok := QualityMap[quality]; ok {
		return q
	}
	return quality
}

// RR upstream status constants
const (
	StatusRunning = "RUNNING"
	StatusPending = "PENDING"
	StatusSuccess = "SUCCESS"
	StatusFailed  = "FAILED"
)

// Channel metadata
const (
	ChannelName          = "RR"
	DefaultSubmitPath    = "/openapi/v2/%s/text-to-image"
	QueryPath            = "/openapi/v2/query"
)

// ModelList contains the default supported models for this channel.
var ModelList = []string{
	"rhart-image-g-2-official",
}
