package cmd

import "testing"

func TestDeriveKVKey(t *testing.T) {
	tests := []struct {
		filePath string
		bucket   string
		want     string
	}{
		{"rules/router/basic.yaml", "rules", "router.basic"},
		{"rules/router/advanced.yaml", "rules", "router.advanced"},
		{"rules/http/webhooks.yaml", "rules", "http.webhooks"},
		{"rules/router/wildcard-examples.yaml", "rules", "router.wildcard-examples"},
		{"basic.yaml", "rules", "basic"},
		{"router/basic.yaml", "rules", "router.basic"},
		{"rules/basic.yml", "rules", "basic"},
		{"custom-bucket/router/basic.yaml", "custom-bucket", "router.basic"},
	}
	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			got := deriveKVKey(tt.filePath, tt.bucket)
			if got != tt.want {
				t.Errorf("deriveKVKey(%q, %q) = %q, want %q", tt.filePath, tt.bucket, got, tt.want)
			}
		})
	}
}

func TestSanitizeKVKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"basic.yaml", "basic"},
		{"basic.yml", "basic"},
		{"router/basic.yaml", "router.basic"},
		{"rules/router/basic.yaml", "rules.router.basic"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeKVKey(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeKVKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
