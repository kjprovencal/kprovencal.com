package main

import (
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/dgraph-io/badger/v4"
)

const maxRsvpAbandonReason = 32

type rsvpAbandonIn struct {
	Name       string   `json:"name"`
	Email      string   `json:"email"`
	GuestCount *int     `json:"guest_count"`
	Meals      []string `json:"meals"`
	Notes      string   `json:"notes"`
	Reason     string   `json:"reason"`
}

func handleRsvpAbandon(w http.ResponseWriter, r *http.Request, db *badger.DB) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var in rsvpAbandonIn
	if !decodeJSONBody(w, r, &in, false) {
		return
	}

	name := strings.TrimSpace(in.Name)
	email := strings.TrimSpace(in.Email)
	notes := strings.TrimSpace(in.Notes)
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		reason = "leave"
	}

	meals := in.Meals
	if meals == nil {
		meals = []string{}
	}

	guestCount := -1
	if in.GuestCount != nil {
		guestCount = *in.GuestCount
	}

	if !rsvpAbandonHasStarted(name, email, notes, guestCount, meals) {
		writeJSON(w, http.StatusBadRequest, errResp{"no form data to record"})
		return
	}

	if utf8.RuneCountInString(name) > maxNameLen ||
		utf8.RuneCountInString(email) > maxEmailLen ||
		utf8.RuneCountInString(notes) > maxNotesLen ||
		utf8.RuneCountInString(reason) > maxRsvpAbandonReason ||
		len(meals) > maxWeddingMealLines {
		writeJSON(w, http.StatusBadRequest, errResp{"field too long"})
		return
	}
	for _, line := range meals {
		if utf8.RuneCountInString(strings.TrimSpace(line)) > maxWeddingMealLineLen {
			writeJSON(w, http.StatusBadRequest, errResp{"field too long"})
			return
		}
	}
	if guestCount < -1 || guestCount > maxWeddingMealLines {
		writeJSON(w, http.StatusBadRequest, errResp{"invalid guest count"})
		return
	}

	if err := insertRsvpAbandon(db, name, email, guestCount, meals, notes, reason); err != nil {
		slog.ErrorContext(r.Context(), "insert rsvp abandon", "err", err)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not save"})
		return
	}

	slog.InfoContext(r.Context(), "rsvp abandon",
		"reason", reason,
		"guest_count", guestCount,
		"meals_filled", len(meals),
		"has_name", name != "",
		"has_email", email != "",
		"remote_addr", requestRemoteAddr(r),
	)

	w.WriteHeader(http.StatusNoContent)
}

func rsvpAbandonHasStarted(name, email, notes string, guestCount int, meals []string) bool {
	if name != "" || email != "" || notes != "" {
		return true
	}
	if guestCount >= 0 {
		return true
	}
	return len(meals) > 0
}
