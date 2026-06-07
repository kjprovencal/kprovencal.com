package main

import (
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/dgraph-io/badger/v4"
)

const (
	maxClientErrorMessage = 2000
	maxClientErrorStack   = 8000
	maxClientErrorPage    = 500
	maxClientErrorSource  = 500
	maxClientErrorKind    = 64
	maxClientErrorUA      = 500
)

type clientErrorIn struct {
	Kind      string `json:"kind"`
	Message   string `json:"message"`
	Page      string `json:"page"`
	Source    string `json:"source"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Stack     string `json:"stack"`
	UserAgent string `json:"user_agent"`
}

func handleClientError(w http.ResponseWriter, r *http.Request, db *badger.DB) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var in clientErrorIn
	if !decodeJSONBody(w, r, &in, false) {
		return
	}

	kind := strings.TrimSpace(in.Kind)
	if kind == "" {
		kind = "error"
	}
	message := strings.TrimSpace(in.Message)
	page := strings.TrimSpace(in.Page)
	if page == "" {
		page = strings.TrimSpace(r.Header.Get("Referer"))
	}
	source := strings.TrimSpace(in.Source)
	stack := strings.TrimSpace(in.Stack)
	ua := strings.TrimSpace(in.UserAgent)
	if ua == "" {
		ua = strings.TrimSpace(r.UserAgent())
	}

	if message == "" {
		writeJSON(w, http.StatusBadRequest, errResp{"message is required"})
		return
	}
	if utf8.RuneCountInString(message) > maxClientErrorMessage ||
		utf8.RuneCountInString(page) > maxClientErrorPage ||
		utf8.RuneCountInString(source) > maxClientErrorSource ||
		utf8.RuneCountInString(stack) > maxClientErrorStack ||
		utf8.RuneCountInString(kind) > maxClientErrorKind ||
		utf8.RuneCountInString(ua) > maxClientErrorUA {
		writeJSON(w, http.StatusBadRequest, errResp{"field too long"})
		return
	}

	if err := insertClientError(db, kind, message, page, source, in.Line, in.Column, stack, ua); err != nil {
		slog.ErrorContext(r.Context(), "insert client error", "err", err)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not save"})
		return
	}

	slog.WarnContext(r.Context(), "client error",
		"kind", kind,
		"page", page,
		"message", truncateForLog(message, 240),
		"source", source,
		"line", in.Line,
		"remote_addr", requestRemoteAddr(r),
	)

	w.WriteHeader(http.StatusNoContent)
}

func truncateForLog(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes]) + "…"
}
