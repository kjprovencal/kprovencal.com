package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dgraph-io/badger/v4"
)

func newTestDB(t *testing.T) *badger.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "badger")
	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestContactPOST_Success(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	mux := http.NewServeMux()
	mountPublicAPI(mux, db, "")

	body := `{"name":"Ada","email":"ada@example.com","message":"Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/contact", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestContactPOST_BodyTooLarge(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	mux := http.NewServeMux()
	mountPublicAPI(mux, db, "")

	msg := strings.Repeat("x", maxJSONBodyBytes+100)
	body := fmt.Sprintf(`{"name":"a","email":"a@b.co","message":%q}`, msg)
	req := httptest.NewRequest(http.MethodPost, "/api/contact", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d want 413", rec.Code)
	}
}

func TestWeddingRSVPPOST_GuestCountZero(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	mux := http.NewServeMux()
	mountPublicAPI(mux, db, "")

	body := `{"name":"a","email":"a@b.co","guest_count":0,"meals":[],"notes":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/rsvp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestWeddingRSVPPOST_Success(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	mux := http.NewServeMux()
	mountPublicAPI(mux, db, "")

	body := `{"name":"Ada","email":"ada@example.com","guest_count":1,"meals":["Guest 1: Chicken Alfredo"],"notes":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/rsvp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

