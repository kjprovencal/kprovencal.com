package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/dgraph-io/badger/v4"
)

const (
	maxNameLen            = 120
	maxEmailLen           = 254
	maxMessageLen         = 8000
	maxNotesLen           = 2000
	maxSlugLen            = 64
	maxWeddingMealLines   = 8
	maxWeddingMealLineLen = 500
)

type errResp struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writePlainError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(msg))
}

func corsHeaders(w http.ResponseWriter, corsAllow string, methods []string) {
	if corsAllow == "" {
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", corsAllow)

	// If only GET, just set the origin
	if len(methods) == 1 && methods[0] == "GET" {
		return
	}

	// If POST, OPTIONS, etc., set the methods and headers
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func mountPublicAPI(mux *http.ServeMux, db *badger.DB, corsAllow string) {
	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w, corsAllow, []string{"GET"})
		events, err := listPublishedEvents(db)
		if err != nil {
			slog.ErrorContext(r.Context(), "list published events", "err", err)
			writeJSON(w, http.StatusInternalServerError, errResp{"could not load events"})
			return
		}
		type out struct {
			Slug  string `json:"slug"`
			Title string `json:"title"`
		}
		list := make([]out, 0, len(events))
		for _, e := range events {
			list = append(list, out{Slug: e.Slug, Title: e.Title})
		}
		writeJSON(w, http.StatusOK, list)
	})
	mux.HandleFunc("POST /api/contact", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w, corsAllow, []string{"POST", "OPTIONS"})
		handleContact(w, r, db)
	})
	mux.HandleFunc("OPTIONS /api/contact", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w, corsAllow, []string{"POST", "OPTIONS"})
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/rsvp", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w, corsAllow, []string{"POST", "OPTIONS"})
		handleRSVP(w, r, db)
	})
	mux.HandleFunc("OPTIONS /api/rsvp", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w, corsAllow, []string{"POST", "OPTIONS"})
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/wedding-rsvp", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w, corsAllow, []string{"POST", "OPTIONS"})
		handleWeddingRSVP(w, r, db)
	})
	mux.HandleFunc("OPTIONS /api/wedding-rsvp", func(w http.ResponseWriter, r *http.Request) {
		corsHeaders(w, corsAllow, []string{"POST", "OPTIONS"})
		w.WriteHeader(http.StatusNoContent)
	})
}

type contactIn struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

func handleContact(w http.ResponseWriter, r *http.Request, db *badger.DB) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in contactIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp{"invalid JSON"})
		return
	}
	name := strings.TrimSpace(in.Name)
	email := strings.TrimSpace(in.Email)
	msg := strings.TrimSpace(in.Message)
	if name == "" || email == "" || msg == "" {
		writeJSON(w, http.StatusBadRequest, errResp{"name, email, and message are required"})
		return
	}
	if utf8.RuneCountInString(name) > maxNameLen || utf8.RuneCountInString(email) > maxEmailLen || utf8.RuneCountInString(msg) > maxMessageLen {
		writeJSON(w, http.StatusBadRequest, errResp{"field too long"})
		return
	}
	if err := insertContact(db, name, email, msg); err != nil {
		slog.ErrorContext(r.Context(), "insert contact", "err", err)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not save"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type rsvpIn struct {
	EventSlug  string `json:"event_slug"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	GuestCount int    `json:"guest_count"`
	Notes      string `json:"notes"`
}

func handleRSVP(w http.ResponseWriter, r *http.Request, db *badger.DB) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in rsvpIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp{"invalid JSON"})
		return
	}
	slug := strings.TrimSpace(in.EventSlug)
	name := strings.TrimSpace(in.Name)
	email := strings.TrimSpace(in.Email)
	notes := strings.TrimSpace(in.Notes)
	guests := in.GuestCount
	if guests < 1 {
		guests = 1
	}
	if guests > 50 {
		writeJSON(w, http.StatusBadRequest, errResp{"guest_count out of range"})
		return
	}
	if slug == "" || name == "" || email == "" {
		writeJSON(w, http.StatusBadRequest, errResp{"event_slug, name, and email are required"})
		return
	}
	if utf8.RuneCountInString(slug) > maxSlugLen || utf8.RuneCountInString(name) > maxNameLen || utf8.RuneCountInString(email) > maxEmailLen || utf8.RuneCountInString(notes) > maxNotesLen {
		writeJSON(w, http.StatusBadRequest, errResp{"field too long"})
		return
	}
	ev, err := getPublishedEventBySlug(db, slug)
	if err != nil {
		if errors.Is(err, ErrEventNotFound) {
			writeJSON(w, http.StatusNotFound, errResp{"unknown or unpublished event"})
			return
		}
		slog.ErrorContext(r.Context(), "get event by slug", "err", err, "slug", slug)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not look up event"})
		return
	}
	if err := insertRSVP(db, ev, name, email, guests, notes); err != nil {
		slog.ErrorContext(r.Context(), "insert rsvp", "err", err, "event_slug", slug)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not save"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type weddingRsvpIn struct {
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	GuestCount     int      `json:"guest_count"`
	Meals          []string `json:"meals"`
	Notes          string   `json:"notes"`
	TurnstileToken string   `json:"turnstile_token"`
}

func handleWeddingRSVP(w http.ResponseWriter, r *http.Request, db *badger.DB) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	turnstileSecret := strings.TrimSpace(os.Getenv("TURNSTILE_SECRET_KEY"))

	var in weddingRsvpIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writePlainError(w, http.StatusBadRequest, "Invalid request body.")
		return
	}

	name := strings.TrimSpace(in.Name)
	email := strings.TrimSpace(in.Email)
	notes := strings.TrimSpace(in.Notes)
	guests := in.GuestCount

	if guests > maxWeddingMealLines {
		writePlainError(w, http.StatusBadRequest, "Guest count cannot exceed 8.")
		return
	}
	if name == "" || email == "" {
		writePlainError(w, http.StatusBadRequest, "Name and email are required.")
		return
	}
	if utf8.RuneCountInString(name) > maxNameLen || utf8.RuneCountInString(email) > maxEmailLen || utf8.RuneCountInString(notes) > maxNotesLen {
		writePlainError(w, http.StatusBadRequest, "A field is too long.")
		return
	}

	meals := in.Meals
	if meals == nil {
		meals = []string{}
	}
	if len(meals) != guests {
		writePlainError(w, http.StatusBadRequest, "Meal choices must match the number of guests.")
		return
	}
	normalized := make([]string, 0, len(meals))
	for _, line := range meals {
		line = strings.TrimSpace(line)
		if line == "" {
			writePlainError(w, http.StatusBadRequest, "Each guest needs a meal choice.")
			return
		}
		if utf8.RuneCountInString(line) > maxWeddingMealLineLen {
			writePlainError(w, http.StatusBadRequest, "A meal line is too long.")
			return
		}
		normalized = append(normalized, line)
	}

	if turnstileSecret != "" {
		token := strings.TrimSpace(in.TurnstileToken)
		if token == "" {
			writePlainError(w, http.StatusBadRequest, "Security check is required.")
			return
		}
		if err := verifyTurnstile(r.Context(), turnstileSecret, token, clientIP(r)); err != nil {
			slog.WarnContext(r.Context(), "turnstile verify failed", "err", err, "remote_addr", requestRemoteAddr(r))
			writePlainError(w, http.StatusForbidden, "Security verification failed. Please try again.")
			return
		}
	}

	if err := insertWeddingRSVP(db, name, email, guests, normalized, notes); err != nil {
		slog.ErrorContext(r.Context(), "insert wedding rsvp", "err", err)
		writePlainError(w, http.StatusInternalServerError, "Could not save your RSVP. Please try again later.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
