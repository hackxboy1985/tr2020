package doubao

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestStripBillingField(t *testing.T) {
	raw := []byte(`{"status":"succeeded","usage":{"completion_tokens":40594,"total_tokens":40594,"SR":"480p","audio":true,"duration":4,"ratio":"9:16"},"billing":{"discount_multiplier":0.65,"discounted_tokens":26386},"extension":{"keep":true}}`)
	got, err := stripBillingField(raw)
	require.NoError(t, err)
	require.NotContains(t, string(got), `"billing"`)
	require.Contains(t, string(got), `"total_tokens":40594`)
	require.Contains(t, string(got), `"extension"`)
	require.Contains(t, string(got), `"completion_tokens":40594`)
	require.NotContains(t, string(got), `"SR"`)
	require.NotContains(t, string(got), `"audio"`)
	require.NotContains(t, string(got), `"discounted_tokens"`)
	require.Contains(t, string(raw), `"billing"`)
	require.Contains(t, string(raw), `"SR"`)
	unchanged, err := stripBillingField([]byte(`{"status":"running"}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"status":"running"}`, string(unchanged))
}

func TestSubmitResponseBillingIsNotExposed(t *testing.T) {
	raw := `{"id":"task","billing":{"discount_multiplier":0.65}}`
	sanitized, err := stripBillingField([]byte(raw))
	require.NoError(t, err)
	require.False(t, strings.Contains(string(sanitized), `"billing"`))
}
