package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dgraph-io/badger/v4"
)

var ContactTestCases = []struct {
	name         string
	body         string
	expectedCode int
	expectedBody string
}{
	{
		name:         "success",
		body:         `{"name":"Ada","email":"ada@example.com","message":"Hello"}`,
		expectedCode: http.StatusNoContent,
		expectedBody: "",
	},
	{
		name:         "missing name",
		body:         `{"name":"","email":"a@b.co","message":"x"}`,
		expectedCode: http.StatusBadRequest,
		expectedBody: "name, email, and message are required",
	},
	{
		name:         "missing email",
		body:         `{"name":"a","email":"","message":"x"}`,
		expectedCode: http.StatusBadRequest,
		expectedBody: "name, email, and message are required",
	},
	{
		name:         "missing message",
		body:         `{"name":"a","email":"a@b.co","message":""}`,
		expectedCode: http.StatusBadRequest,
		expectedBody: "name, email, and message are required",
	},
	{
		name:         "name too long",
		body:         `{"name":"` + strings.Repeat("x", maxNameLen+1) + `","email":"a@b.co","message":"x"}`,
		expectedCode: http.StatusBadRequest,
		expectedBody: "field too long",
	},
	{
		name:         "email too long",
		body:         `{"name":"a","email":"` + strings.Repeat("x", maxEmailLen+1) + `","message":"x"}`,
		expectedCode: http.StatusBadRequest,
		expectedBody: "field too long",
	},
	{
		name:         "message too long",
		body:         `{"name":"a","email":"a@b.co","message":"` + strings.Repeat("x", maxMessageLen+1) + `"}`,
		expectedCode: http.StatusBadRequest,
		expectedBody: "field too long",
	},
}

var RSVPTestCases = []struct {
	name         string
	body         string
	expectedCode int
	expectedBody string
}{
	{
		name:         "no guests",
		body:         `{"name":"Ada","email":"ada@example.com","guest_count":0,"meals":[],"notes":""}`,
		expectedCode: http.StatusNoContent,
		expectedBody: "",
	},
	{
		name:         "one guest",
		body:         `{"name":"Ada","email":"ada@example.com","guest_count":1,"meals":["Guest 1: Chicken Alfredo"],"notes":""}`,
		expectedCode: http.StatusNoContent,
		expectedBody: "",
	},
	{
		name:         "eight guests",
		body:         `{"name":"Ada","email":"ada@example.com","guest_count":8,"meals":["Guest 1: Chicken Alfredo","Guest 2: Scampi","Guest 3: Verdura al Napoleon","Guest 4: Scampi","Guest 5: Chicken Alfredo","Guest 6: Scampi","Guest 7: Verdura al Napoleon","Guest 8: Scampi"],"notes":""}`,
		expectedCode: http.StatusNoContent,
		expectedBody: "",
	},
	{
		name:         "guest count negative",
		body:         `{"name":"a","email":"a@b.co","guest_count":-1,"meals":[],"notes":""}`,
		expectedCode: http.StatusBadRequest,
		expectedBody: "How did you get a negative number of guests?",
	},
	{
		name:         "guest count too large",
		body:         `{"name":"a","email":"a@b.co","guest_count":9,"meals":[],"notes":""}`,
		expectedCode: http.StatusBadRequest,
		expectedBody: "Guest count cannot exceed 8.",
	},
	{
		name:         "name and email required",
		body:         `{"name":"a","email":"","guest_count":1,"meals":[],"notes":""}`,
		expectedCode: http.StatusBadRequest,
		expectedBody: "Name and email are required.",
	},
	{
		name:         "field too long",
		body:         `{"name":"a","email":"a@b.co","guest_count":1,"meals":["Guest 1: Chicken Alfredo"],"notes":"` + strings.Repeat("x", maxNotesLen+1) + `"}`,
		expectedCode: http.StatusBadRequest,
		expectedBody: "A field is too long.",
	},
	{
		name:         "meal choices must match the number of guests",
		body:         `{"name":"a","email":"a@b.co","guest_count":1,"meals":["Guest 1: Chicken Alfredo","Guest 2: Scampi"],"notes":""}`,
		expectedCode: http.StatusBadRequest,
		expectedBody: "Meal choices must match the number of guests.",
	},
	{
		name:         "invalid meal choices",
		body:         `{"name":"a","email":"a@b.co","guest_count":2,"meals":["Guest 1: Chicken Alfredo","Guest 2: Mac and Cheese"],"notes":""}`,
		expectedCode: http.StatusBadRequest,
		expectedBody: "Invalid meal choices.",
	},
}

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

func TestContactPOST(t *testing.T) {
	for _, tc := range ContactTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := newTestDB(t)
			mux := http.NewServeMux()
			mountPublicAPI(mux, db, "")

			req := httptest.NewRequest(http.MethodPost, "/api/contact", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tc.expectedCode {
				t.Fatalf("status %d want %d", rec.Code, tc.expectedCode)
			}

			if !strings.Contains(rec.Body.String(), tc.expectedBody) {
				t.Fatalf("body %s does not contain %s", rec.Body.String(), tc.expectedBody)
			}
		})
	}
}

func TestWeddingRSVPPOST(t *testing.T) {
	for _, tc := range RSVPTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := newTestDB(t)
			mux := http.NewServeMux()
			mountPublicAPI(mux, db, "")

			req := httptest.NewRequest(http.MethodPost, "/api/rsvp", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tc.expectedCode {
				t.Fatalf("status %d want %d", rec.Code, tc.expectedCode)
			}
			if rec.Body.String() != tc.expectedBody {
				t.Fatalf("body %s want %s", rec.Body.String(), tc.expectedBody)
			}
		})
	}
}
