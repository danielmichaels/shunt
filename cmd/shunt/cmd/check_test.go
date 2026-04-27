package cmd

import (
	"reflect"
	"testing"
)

func TestParseHeaders_Success(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want map[string]string
	}{
		{
			name: "single header",
			in:   []string{"X-GitHub-Event: push"},
			want: map[string]string{"X-GitHub-Event": "push"},
		},
		{
			name: "multiple headers preserved",
			in: []string{
				"X-GitHub-Event: push",
				"X-Hub-Signature-256: sha256=abc",
				"Content-Type: application/json",
			},
			want: map[string]string{
				"X-GitHub-Event":      "push",
				"X-Hub-Signature-256": "sha256=abc",
				"Content-Type":        "application/json",
			},
		},
		{
			name: "leading space on value optional",
			in:   []string{"X:y", "Y: y"},
			want: map[string]string{"X": "y", "Y": "y"},
		},
		{
			name: "trailing whitespace on value trimmed",
			in:   []string{"X-Foo: bar   "},
			want: map[string]string{"X-Foo": "bar"},
		},
		{
			name: "value containing colon preserved verbatim",
			in:   []string{"X-Sig: sha256=abc:def:ghi"},
			want: map[string]string{"X-Sig": "sha256=abc:def:ghi"},
		},
		{
			name: "duplicate key last wins",
			in: []string{
				"X-Env: staging",
				"X-Env: production",
			},
			want: map[string]string{"X-Env": "production"},
		},
		{
			name: "empty value allowed",
			in:   []string{"X-Empty:"},
			want: map[string]string{"X-Empty": ""},
		},
		{
			name: "empty value with space allowed",
			in:   []string{"X-Empty: "},
			want: map[string]string{"X-Empty": ""},
		},
		{
			name: "key whitespace trimmed both sides",
			in:   []string{"  X-Foo  : bar"},
			want: map[string]string{"X-Foo": "bar"},
		},
		{
			name: "nil input returns nil",
			in:   nil,
			want: nil,
		},
		{
			name: "empty slice returns nil",
			in:   []string{},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHeaders(tt.in)
			if err != nil {
				t.Fatalf("parseHeaders(%v) returned unexpected error: %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseHeaders(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseHeaders_Errors(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		wantSub string
	}{
		{
			name:    "missing colon",
			in:      []string{"no colon here"},
			wantSub: `invalid --header "no colon here"`,
		},
		{
			name:    "empty key with colon",
			in:      []string{":value"},
			wantSub: `invalid --header ":value"`,
		},
		{
			name:    "whitespace-only key",
			in:      []string{"   : value"},
			wantSub: `invalid --header "   : value"`,
		},
		{
			name:    "fail fast on first bad header",
			in:      []string{"no colon", "X-Good: ok"},
			wantSub: `invalid --header "no colon"`,
		},
		{
			name:    "fail fast preserves first error when later header is also bad",
			in:      []string{"X-Good: ok", "first bad", "second bad"},
			wantSub: `invalid --header "first bad"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHeaders(tt.in)
			if err == nil {
				t.Fatalf("parseHeaders(%v) = %v, want error", tt.in, got)
			}
			if got != nil {
				t.Errorf("parseHeaders returned non-nil map alongside error: %v", got)
			}
			if !contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantSub)
			}
		})
	}
}

func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
