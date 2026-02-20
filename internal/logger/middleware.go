package logger

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}

func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

type HTTPLoggerConfig struct {
	QuietRoutes        []QuietRoute
	SkipPaths          []string
	Concise            bool
	LogRequestHeaders  bool
	LogResponseHeaders bool
}

type QuietRoute struct {
	Path   string
	Period time.Duration
}

type quietRouteTracker struct {
	mu     sync.Mutex
	routes map[string]QuietRoute
	last   map[string]time.Time
}

func newQuietRouteTracker(routes []QuietRoute) *quietRouteTracker {
	rm := make(map[string]QuietRoute, len(routes))
	for _, r := range routes {
		rm[r.Path] = r
	}
	return &quietRouteTracker{
		routes: rm,
		last:   make(map[string]time.Time),
	}
}

func (q *quietRouteTracker) shouldLog(path string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	route, ok := q.routes[path]
	if !ok {
		return true
	}

	last, exists := q.last[path]
	now := time.Now()
	if !exists || now.Sub(last) >= route.Period {
		q.last[path] = now
		return true
	}
	return false
}

var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"proxy-authorization": true,
	"cookie":              true,
	"set-cookie":          true,
	"x-api-key":           true,
	"x-auth-token":        true,
	"x-csrf-token":        true,
	"x-session-token":     true,
	"api-key":             true,
	"apikey":              true,
}

func sanitizeHeaders(h http.Header) map[string][]string {
	out := make(map[string][]string, len(h))
	for k, v := range h {
		if sensitiveHeaders[strings.ToLower(k)] {
			out[k] = []string{"REDACTED"}
		} else {
			cp := make([]string, len(v))
			copy(cp, v)
			out[k] = cp
		}
	}
	return out
}

func RequestLogger(logger *slog.Logger, cfg HTTPLoggerConfig) func(http.Handler) http.Handler {
	tracker := newQuietRouteTracker(cfg.QuietRoutes)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, sp := range cfg.SkipPaths {
				if strings.HasPrefix(r.URL.Path, sp) {
					next.ServeHTTP(w, r)
					return
				}
			}

			start := time.Now()
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			if !tracker.shouldLog(r.URL.Path) {
				return
			}

			duration := time.Since(start)

			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", wrapped.status),
				slog.Duration("duration", duration),
				slog.Int("bytes", wrapped.bytes),
				slog.String("remote_addr", r.RemoteAddr),
			}

			if !cfg.Concise {
				attrs = append(attrs,
					slog.String("user_agent", r.UserAgent()),
					slog.String("proto", r.Proto),
					slog.String("query", r.URL.RawQuery),
				)
			}

			if cfg.LogRequestHeaders {
				attrs = append(attrs, slog.Any("request_headers", sanitizeHeaders(r.Header)))
			}

			if cfg.LogResponseHeaders {
				attrs = append(attrs, slog.Any("response_headers", sanitizeHeaders(wrapped.Header())))
			}

			level := slog.LevelInfo
			switch {
			case wrapped.status >= 500:
				level = slog.LevelError
			case wrapped.status >= 400:
				level = slog.LevelWarn
			}

			logger.LogAttrs(r.Context(), level, "http request", attrs...)
		})
	}
}
