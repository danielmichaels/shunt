package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSanitizeHeaders(t *testing.T) {
	h := http.Header{
		"Authorization":   {"Bearer secret-token"},
		"Content-Type":    {"application/json"},
		"X-Api-Key":       {"my-api-key"},
		"X-Custom-Header": {"safe-value"},
		"Cookie":          {"session=abc123"},
	}

	got := sanitizeHeaders(h)

	if got["Authorization"][0] != "REDACTED" {
		t.Errorf("Authorization should be REDACTED, got %q", got["Authorization"][0])
	}
	if got["X-Api-Key"][0] != "REDACTED" {
		t.Errorf("X-Api-Key should be REDACTED, got %q", got["X-Api-Key"][0])
	}
	if got["Cookie"][0] != "REDACTED" {
		t.Errorf("Cookie should be REDACTED, got %q", got["Cookie"][0])
	}
	if got["Content-Type"][0] != "application/json" {
		t.Errorf("Content-Type should pass through, got %q", got["Content-Type"][0])
	}
	if got["X-Custom-Header"][0] != "safe-value" {
		t.Errorf("X-Custom-Header should pass through, got %q", got["X-Custom-Header"][0])
	}
}

func TestSanitizeHeaders_CaseInsensitive(t *testing.T) {
	h := http.Header{
		"authorization": {"Bearer token"},
		"COOKIE":        {"session=xyz"},
	}

	got := sanitizeHeaders(h)

	if got["authorization"][0] != "REDACTED" {
		t.Errorf("lowercase authorization should be REDACTED, got %q", got["authorization"][0])
	}
	if got["COOKIE"][0] != "REDACTED" {
		t.Errorf("uppercase COOKIE should be REDACTED, got %q", got["COOKIE"][0])
	}
}

func TestQuietRouteTracker(t *testing.T) {
	tracker := newQuietRouteTracker([]QuietRoute{
		{Path: "/metrics", Period: 100 * time.Millisecond},
	})

	if !tracker.shouldLog("/metrics") {
		t.Error("first call should log")
	}

	if tracker.shouldLog("/metrics") {
		t.Error("immediate second call should be suppressed")
	}

	if !tracker.shouldLog("/other") {
		t.Error("non-quiet route should always log")
	}

	time.Sleep(110 * time.Millisecond)

	if !tracker.shouldLog("/metrics") {
		t.Error("should log after period elapsed")
	}
}

type logEntry struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

func parseLogEntries(t *testing.T, buf *bytes.Buffer) []logEntry {
	t.Helper()
	var entries []logEntry
	dec := json.NewDecoder(buf)
	for dec.More() {
		var e logEntry
		if err := dec.Decode(&e); err != nil {
			t.Fatalf("failed to decode log entry: %v", err)
		}
		entries = append(entries, e)
	}
	return entries
}

func TestRequestLogger_InfoLevel(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := RequestLogger(log, HTTPLoggerConfig{Concise: true})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	entries := parseLogEntries(t, &buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0].Level != "INFO" {
		t.Errorf("expected INFO level, got %q", entries[0].Level)
	}
}

func TestRequestLogger_WarnOn4xx(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := RequestLogger(log, HTTPLoggerConfig{})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	entries := parseLogEntries(t, &buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0].Level != "WARN" {
		t.Errorf("expected WARN level, got %q", entries[0].Level)
	}
}

func TestRequestLogger_ErrorOn5xx(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := RequestLogger(log, HTTPLoggerConfig{})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	entries := parseLogEntries(t, &buf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0].Level != "ERROR" {
		t.Errorf("expected ERROR level, got %q", entries[0].Level)
	}
}

func TestRequestLogger_SkipPaths(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := RequestLogger(log, HTTPLoggerConfig{
		SkipPaths: []string{"/health", "/healthz"},
	})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	for _, path := range []string{"/health", "/healthz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	if buf.Len() != 0 {
		t.Errorf("expected no log output for skip paths, got %q", buf.String())
	}
}

func TestRequestLogger_RedactsHeaders(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := RequestLogger(log, HTTPLoggerConfig{
		LogRequestHeaders: true,
	})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("REDACTED")) {
		t.Error("expected REDACTED in log output for Authorization header")
	}
	if bytes.Contains([]byte(output), []byte("Bearer secret")) {
		t.Error("sensitive header value should not appear in log output")
	}
}
