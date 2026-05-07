package cmd

import "testing"

func TestDeriveKVKey(t *testing.T) {
	tests := []struct {
		pushPath  string
		filePath  string
		bucket    string
		pushIsDir bool
		want      string
	}{
		{"rules/router/basic.yaml", "rules/router/basic.yaml", "rules", false, "router.basic"},
		{"rules/router/advanced.yaml", "rules/router/advanced.yaml", "rules", false, "router.advanced"},
		{"rules/http/webhooks.yaml", "rules/http/webhooks.yaml", "rules", false, "http.webhooks"},
		{"rules/router/wildcard-examples.yaml", "rules/router/wildcard-examples.yaml", "rules", false, "router.wildcard-examples"},
		{"basic.yaml", "basic.yaml", "rules", false, "basic"},
		{"router/basic.yaml", "router/basic.yaml", "rules", false, "router.basic"},
		{"rules/basic.yml", "rules/basic.yml", "rules", false, "basic"},
		{"custom-bucket/router/basic.yaml", "custom-bucket/router/basic.yaml", "custom-bucket", false, "router.basic"},
		{"./rules/basic.yaml", "./rules/basic.yaml", "rules", false, "basic"},
		{"../example-app/shunt/rules/basic-notify.yaml", "../example-app/shunt/rules/basic-notify.yaml", "rules", false, "basic-notify"},
		{"/home/dev/projects/example-app/shunt/rules/basic-notify.yaml", "/home/dev/projects/example-app/shunt/rules/basic-notify.yaml", "rules", false, "basic-notify"},
		{"/home/dev/projects/example-app/shunt/rules/router/basic.yaml", "/home/dev/projects/example-app/shunt/rules/router/basic.yaml", "rules", false, "router.basic"},
		{"/tmp/router", "/tmp/router/basic.yaml", "rules", true, "router.basic"},
		{"/tmp/basic.yaml", "/tmp/basic.yaml", "rules", false, "basic"},
	}
	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			got := deriveKVKey(tt.pushPath, tt.filePath, tt.bucket, tt.pushIsDir)
			if got != tt.want {
				t.Errorf("deriveKVKey(%q, %q, %q, %t) = %q, want %q", tt.pushPath, tt.filePath, tt.bucket, tt.pushIsDir, got, tt.want)
			}
		})
	}
}

func TestKVPathSegments(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"basic.yaml", []string{"basic"}},
		{"basic.yml", []string{"basic"}},
		{"router/basic.yaml", []string{"router", "basic"}},
		{"rules/router/basic.yaml", []string{"rules", "router", "basic"}},
		{"./rules/basic.yaml", []string{"rules", "basic"}},
		{"../example-app/shunt/rules/basic.yaml", []string{"example-app", "shunt", "rules", "basic"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := kvPathSegments(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("kvPathSegments(%q) length = %d, want %d (%v)", tt.input, len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("kvPathSegments(%q)[%d] = %q, want %q (%v)", tt.input, i, got[i], tt.want[i], got)
				}
			}
		})
	}
}
