package controller

import "testing"

// TestExtractFaceVerificationCallbackURL 下游请求体兼容性：
// 老下游用 return_url，n-axis/火山写法用 callback_url，三种大小写都要能取到。
func TestExtractFaceVerificationCallbackURL(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"return_url (legacy downstream)", `{"return_url":"https://app.example.com/cb"}`, "https://app.example.com/cb"},
		{"callback_url (n-axis REST)", `{"callback_url":"https://app.example.com/cb"}`, "https://app.example.com/cb"},
		{"CallbackURL (volcano top)", `{"CallbackURL":"https://app.example.com/cb"}`, "https://app.example.com/cb"},
		{"CallbackUrl (mixed case)", `{"CallbackUrl":"https://app.example.com/cb"}`, "https://app.example.com/cb"},
		{"empty body", ``, ""},
		{"invalid json", "not-json", ""},
		{"no callback", `{"foo":"bar"}`, ""},
	}
	for _, tc := range cases {
		if got := extractFaceVerificationCallbackURL([]byte(tc.body)); got != tc.want {
			t.Fatalf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
