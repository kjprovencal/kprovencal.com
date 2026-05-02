package main

import (
	"encoding/json"
	"errors"
	"fmt"
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
	maxWeddingMealLines   = 8
	maxWeddingMealLineLen = 500
	maxJSONBodyBytes      = 64 << 10 // 64 KiB for POST JSON bodies
)

// allowedWeddingMealLabels must match `label` in src/mount-rsvp.ts (MEAL_OPTIONS).
var allowedWeddingMealLabels = map[string]struct{}{
	"Chicken Alfredo":     {},
	"Scampi":              {},
	"Verdura al Napoleon": {},
}

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

// decodeJSONBody reads JSON from r.Body with a size cap. plainErr selects plain-text vs JSON error responses.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, v any, plainErr bool) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	err := json.NewDecoder(r.Body).Decode(v)
	if err != nil {
		// http.MaxBytesReader returns errors.New("http: request body too large") (no stable exported sentinel in all Go versions).
		if err.Error() == "http: request body too large" {
			if plainErr {
				writePlainError(w, http.StatusRequestEntityTooLarge, "Request body too large.")
			} else {
				writeJSON(w, http.StatusRequestEntityTooLarge, errResp{"request body too large"})
			}
			return false
		}
		if plainErr {
			writePlainError(w, http.StatusBadRequest, "Invalid request body.")
		} else {
			writeJSON(w, http.StatusBadRequest, errResp{"invalid JSON"})
		}
		return false
	}
	return true
}

func normalizeWeddingMealLines(meals []string) ([]string, error) {
	normalized := make([]string, 0, len(meals))
	for i, line := range meals {
		line = strings.TrimSpace(line)
		if line == "" {
			return nil, errors.New("empty meal line")
		}
		prefix := fmt.Sprintf("Guest %d: ", i+1)
		if !strings.HasPrefix(line, prefix) {
			return nil, fmt.Errorf("invalid meal line format for guest %d", i+1)
		}
		label := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		if _, ok := allowedWeddingMealLabels[label]; !ok {
			return nil, fmt.Errorf("invalid meal choice for guest %d", i+1)
		}
		if utf8.RuneCountInString(line) > maxWeddingMealLineLen {
			return nil, errors.New("meal line too long")
		}
		normalized = append(normalized, line)
	}
	return normalized, nil
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
	if !decodeJSONBody(w, r, &in, false) {
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
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	GuestCount     int      `json:"guest_count"`
	Meals          []string `json:"meals"`
	Notes          string   `json:"notes"`
	TurnstileToken string   `json:"turnstile_token"`
}

func handleRSVP(w http.ResponseWriter, r *http.Request, db *badger.DB) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	turnstileSecret := strings.TrimSpace(os.Getenv("TURNSTILE_SECRET_KEY"))

	var in rsvpIn
	if !decodeJSONBody(w, r, &in, true) {
		return
	}

	name := strings.TrimSpace(in.Name)
	email := strings.TrimSpace(in.Email)
	notes := strings.TrimSpace(in.Notes)
	guests := in.GuestCount

	if guests < 0 {
		writePlainError(w, http.StatusBadRequest, "How did you get a negative number of guests?")
		return
	}
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
	normalized, err := normalizeWeddingMealLines(meals)
	if err != nil {
		writePlainError(w, http.StatusBadRequest, "Invalid meal choices.")
		return
	}

	if turnstileSecret != "" {
		token := strings.TrimSpace(in.TurnstileToken)
		if token == "" {
			writePlainError(w, http.StatusBadRequest, "Security check is required.")
			return
		}
		if err := verifyTurnstile(r.Context(), turnstileSecret, token, turnstileClientIP(r)); err != nil {
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
