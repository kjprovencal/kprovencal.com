package main

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// apiLogPath is set when LOG_PATH is configured (used by GET /admin/logs).
var apiLogPath string

// initLogging configures the default slog logger. LOG_LEVEL: debug, info (default), warn, error.
// When LOG_PATH is set, logs are appended to that file as well as stderr (systemd journal).
func initLogging() {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	out := io.Writer(os.Stderr)
	logPath := strings.TrimSpace(os.Getenv("LOG_PATH"))
	if logPath != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
		if err != nil {
			h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
			slog.SetDefault(slog.New(h))
			slog.Error("cannot open LOG_PATH; logging to stderr only", "path", logPath, "err", err)
			return
		}
		apiLogPath = logPath
		out = io.MultiWriter(os.Stderr, f)
	}

	h := slog.NewTextHandler(out, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}

// requestRemoteAddr returns a client address for logs (first X-Forwarded-For hop if present).
func requestRemoteAddr(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = strings.TrimSpace(xff[:i])
		}
		return xff
	}
	return r.RemoteAddr
}

type responseRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (rec *responseRecorder) WriteHeader(code int) {
	if !rec.wroteHeader {
		rec.status = code
		rec.wroteHeader = true
	}
	rec.ResponseWriter.WriteHeader(code)
}

func (rec *responseRecorder) Write(b []byte) (int, error) {
	if !rec.wroteHeader {
		rec.status = http.StatusOK
		rec.wroteHeader = true
	}
	return rec.ResponseWriter.Write(b)
}

func withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if !rec.wroteHeader {
			rec.status = http.StatusOK
		}
		slog.InfoContext(r.Context(), "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", requestRemoteAddr(r),
		)
	})
}
