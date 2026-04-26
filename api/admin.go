package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dgraph-io/badger/v4"
)

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

var loginLimiter = newLoginAttemptLimiter()

const (
	maxAdminFormBodyBytes = 32 << 10 // 32 KiB for application/x-www-form-urlencoded
	maxSlugLen            = 64
)

func setAdminCORS(w http.ResponseWriter, corsAllow string) {
	if corsAllow == "" {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", corsAllow)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}

func adminPreflight(w http.ResponseWriter, corsAllow string) {
	if corsAllow == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	setAdminCORS(w, corsAllow)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.WriteHeader(http.StatusNoContent)
}

func withAdminCORS(corsAllow string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setAdminCORS(w, corsAllow)
		h.ServeHTTP(w, r)
	})
}

func registerAdminPreflightRoutes(mux *http.ServeMux, corsAllow string) {
	if corsAllow == "" {
		return
	}
	paths := []string{
		"/admin/login",
		"/admin/logout",
		"/admin/session",
		"/admin/contacts",
		"/admin/rsvps",
		"/admin/events",
	}
	for _, p := range paths {
		mux.HandleFunc("OPTIONS "+p, func(w http.ResponseWriter, r *http.Request) {
			adminPreflight(w, corsAllow)
		})
	}
}

func mountAdminRoutes(mux *http.ServeMux, db *badger.DB, sessionSecret []byte, passwordHash string, corsAllow string) {
	registerAdminPreflightRoutes(mux, corsAllow)

	mux.Handle("GET /admin/session", withAdminCORS(corsAllow, requireAdmin(sessionSecret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": true})
	}))))

	mux.HandleFunc("POST /admin/login", func(w http.ResponseWriter, r *http.Request) {
		setAdminCORS(w, corsAllow)
		if !loginLimiter.allow(requestRemoteAddr(r)) {
			writeTooManyLoginAttempts(w)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxAdminFormBodyBytes)
		if err := r.ParseForm(); err != nil {
			slog.WarnContext(r.Context(), "admin login bad form", "err", err, "remote_addr", requestRemoteAddr(r))
			writeJSON(w, http.StatusBadRequest, errResp{"invalid form"})
			return
		}
		pass := strings.TrimSpace(r.FormValue("password"))
		if !verifyAdminPassword(passwordHash, pass) {
			slog.WarnContext(r.Context(), "admin login failed", "reason", "invalid_credentials", "remote_addr", requestRemoteAddr(r))
			writeJSON(w, http.StatusUnauthorized, errResp{"invalid credentials"})
			return
		}
		if err := setSessionCookie(w, sessionSecret, time.Now().Add(adminSessionTTL())); err != nil {
			slog.ErrorContext(r.Context(), "admin session cookie", "err", err)
			http.Error(w, "session error", http.StatusInternalServerError)
			return
		}
		slog.InfoContext(r.Context(), "admin login ok", "remote_addr", requestRemoteAddr(r))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	mux.Handle("POST /admin/logout", withAdminCORS(corsAllow, requireAdmin(sessionSecret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clearSessionCookie(w)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}))))

	mux.Handle("GET /admin/contacts", withAdminCORS(corsAllow, requireAdmin(sessionSecret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := parseLimit(r.URL.Query().Get("limit"), 200, 500)
		rows, err := listContacts(db, limit)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin list contacts", "err", err)
			writeJSON(w, http.StatusInternalServerError, errResp{"could not load contacts"})
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}))))

	mux.Handle("GET /admin/rsvps", withAdminCORS(corsAllow, requireAdmin(sessionSecret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := parseLimit(r.URL.Query().Get("limit"), 200, 500)
		rows, err := listRSVPs(db, limit)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin list rsvps", "err", err)
			writeJSON(w, http.StatusInternalServerError, errResp{"could not load rsvps"})
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}))))

	mux.Handle("GET /admin/events", withAdminCORS(corsAllow, requireAdmin(sessionSecret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		events, err := listAllEvents(db)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin list events", "err", err)
			writeJSON(w, http.StatusInternalServerError, errResp{"could not load events"})
			return
		}
		writeJSON(w, http.StatusOK, events)
	}))))

	mux.Handle("POST /admin/events", withAdminCORS(corsAllow, requireAdmin(sessionSecret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxAdminFormBodyBytes)
		if err := r.ParseForm(); err != nil {
			writeJSON(w, http.StatusBadRequest, errResp{"invalid form"})
			return
		}
		slug := strings.TrimSpace(r.FormValue("slug"))
		title := strings.TrimSpace(r.FormValue("title"))
		published := r.FormValue("published") == "on" || r.FormValue("published") == "1"
		if slug == "" || title == "" {
			writeJSON(w, http.StatusBadRequest, errResp{"slug and title are required"})
			return
		}
		if !slugRe.MatchString(slug) || utf8.RuneCountInString(slug) > maxSlugLen {
			writeJSON(w, http.StatusBadRequest, errResp{"invalid slug"})
			return
		}
		if utf8.RuneCountInString(title) > 200 {
			writeJSON(w, http.StatusBadRequest, errResp{"title too long"})
			return
		}
		if err := insertEvent(db, slug, title, published); err != nil {
			if errors.Is(err, ErrEventExists) {
				writeJSON(w, http.StatusConflict, errResp{"event slug already exists"})
				return
			}
			slog.ErrorContext(r.Context(), "admin create event", "err", err, "slug", slug)
			writeJSON(w, http.StatusInternalServerError, errResp{"could not create event"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}))))
}

func adminSessionTTL() time.Duration {
	s := strings.TrimSpace(os.Getenv("ADMIN_SESSION_HOURS"))
	if s == "" {
		return 12 * time.Hour
	}
	h, err := strconv.ParseFloat(s, 64)
	if err != nil || h <= 0 || h > 720 {
		return 12 * time.Hour
	}
	return time.Duration(h * float64(time.Hour))
}

func parseLimit(raw string, def int, max int) int {
	s := strings.TrimSpace(raw)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		n = max
	}
	return n
}
