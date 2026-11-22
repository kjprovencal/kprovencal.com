package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHealthzIsPlainOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	healthz(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("body %q", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "version") || strings.Contains(rec.Header().Get("Content-Type"), "json") {
		t.Fatalf("healthz leaked detail: %s %q", rec.Header().Get("Content-Type"), rec.Body.String())
	}
}

func TestWithRequestMetricsCounts5xx(t *testing.T) {
	requestsTotal.Store(0)
	responses5xx.Store(0)
	h := withRequestMetrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/boom" {
			http.Error(w, "nope", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ok", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))
	if requestsTotal.Load() != 2 || responses5xx.Load() != 1 {
		t.Fatalf("total=%d 5xx=%d", requestsTotal.Load(), responses5xx.Load())
	}
}

func TestAdminOpsRequiresSession(t *testing.T) {
	mux := newOpsMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/ops", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestAdminOpsReturnsSnapshotAndHostFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host-status.json")
	raw := `{
		"collected_at":"2026-11-22T00:00:00Z",
		"kernel":"<script>",
		"security_upgrade_count":2,
		"security_upgrades":["nginx"],
		"api_local_health":"ok",
		"notes":["kernel note"]
	}`
	if err := os.WriteFile(path, []byte(raw), 0o640); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOST_STATUS_PATH", path)
	requestsTotal.Store(0)
	responses5xx.Store(0)

	mux := newOpsMux(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/ops", nil)
	token, err := signSession([]byte("0123456789abcdef0123456789abcdef"), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var snap opsSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Version != "dev" {
		t.Fatalf("version %q", snap.Version)
	}
	if !snap.Host.Configured {
		t.Fatalf("host %#v", snap.Host)
	}
	if snap.Host.Kernel != "<script>" {
		t.Fatalf("kernel %q", snap.Host.Kernel)
	}
	if snap.Host.SecurityUpgradeCount != 2 || len(snap.Host.SecurityUpgrades) != 1 {
		t.Fatalf("upgrades %#v", snap.Host.SecurityUpgrades)
	}
	if strings.Contains(rec.Body.String(), "SESSION_SECRET") {
		t.Fatal("response included a secret field")
	}
}

func TestReadHostStatusRejectsSymlinkAndJunk(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.json")
	if err := os.WriteFile(real, []byte(`{"collected_at":"2026-11-22T00:00:00Z"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOST_STATUS_PATH", link)
	if got := readHostStatus(); got.Configured {
		t.Fatalf("symlink was accepted: %#v", got)
	}

	t.Setenv("HOST_STATUS_PATH", "relative.json")
	if got := readHostStatus(); got.Configured || !strings.Contains(got.Hint, "absolute") {
		t.Fatalf("relative path: %#v", got)
	}

	t.Setenv("HOST_STATUS_PATH", "/tmp/../etc/passwd")
	if got := readHostStatus(); got.Configured {
		t.Fatalf("dotdot path accepted: %#v", got)
	}

	big := filepath.Join(dir, "big.json")
	if err := os.WriteFile(big, []byte(strings.Repeat("a", maxHostStatusBytes+2)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOST_STATUS_PATH", big)
	if got := readHostStatus(); got.Configured || !strings.Contains(got.Hint, "too large") {
		t.Fatalf("oversize: %#v", got)
	}

	empty := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(empty, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOST_STATUS_PATH", empty)
	if got := readHostStatus(); got.Configured {
		t.Fatalf("missing collected_at accepted: %#v", got)
	}
}

func TestHostStatusFileDefault(t *testing.T) {
	t.Setenv("HOST_STATUS_PATH", "")
	if hostStatusFile() != defaultHostStatusPath {
		t.Fatalf("default %q", hostStatusFile())
	}
}

func newOpsMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mountAdminRoutes(mux, nil, []byte("0123456789abcdef0123456789abcdef"), "unused", "")
	return mux
}
