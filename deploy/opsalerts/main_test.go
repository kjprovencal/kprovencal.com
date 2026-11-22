package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHighNpmAndCleanGovulnIsActionable(t *testing.T) {
	code, body := summarizeAudit(0, "No vulnerabilities found.", 1, map[string]any{
		"vulnerabilities": map[string]any{
			"marked": map[string]any{
				"severity": "high",
				"via":      []any{map[string]any{"title": "bad parse <x>"}},
			},
		},
		"metadata": map[string]any{
			"vulnerabilities": map[string]any{"moderate": 2, "high": 1},
		},
	})
	if code != 2 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(body, "high: marked") {
		t.Fatal(body)
	}
	if !strings.Contains(body, "Moderate npm findings (not alerting): 2.") {
		t.Fatal(body)
	}
}

func TestModerateOnlyIsQuiet(t *testing.T) {
	code, _ := summarizeAudit(0, "No vulnerabilities found.", 1, map[string]any{
		"vulnerabilities": map[string]any{
			"vite": map[string]any{"severity": "moderate", "via": []any{"vite"}},
		},
		"metadata": map[string]any{
			"vulnerabilities": map[string]any{"moderate": 1},
		},
	})
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
}

func TestToolFailure(t *testing.T) {
	code, body := summarizeAudit(1, "boom", 1, map[string]any{"error": "nope"})
	if code != 1 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(body, "failed") {
		t.Fatal(body)
	}
}

func TestPendingSecurityUpdate(t *testing.T) {
	reasons := hostReasons(map[string]any{
		"security_upgrade_count": 1,
		"security_upgrades":      []any{"nginx"},
		"api_local_health":       "ok",
		"reboot_required":        false,
		"warnings":               []any{},
	})
	if len(reasons) != 1 || reasons[0] != "1 security update(s) pending: nginx" {
		t.Fatalf("%#v", reasons)
	}
}

func TestJSONNumberCountStillAlerts(t *testing.T) {
	doc, err := decodeJSON([]byte(`{"security_upgrade_count":1,"security_upgrades":["nginx"],"api_local_health":"ok","reboot_required":false,"warnings":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	reasons := hostReasons(doc)
	if len(reasons) != 1 || reasons[0] != "1 security update(s) pending: nginx" {
		t.Fatalf("%#v", reasons)
	}
}

func TestClearHostHasNoReasons(t *testing.T) {
	reasons := hostReasons(map[string]any{
		"security_upgrade_count": 0,
		"api_local_health":       "ok",
		"reboot_required":        false,
		"docker_socket":          "absent",
		"unexpected_listeners":   []any{},
		"warnings":               []any{},
	})
	if len(reasons) != 0 {
		t.Fatalf("%#v", reasons)
	}
}

func TestCLIExitCodes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.json")
	if err := os.WriteFile(path, []byte(`{"api_local_health":"down","warnings":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	restore := discardStdio(t)
	defer restore()
	if code := run([]string{"host-reasons", path}); code != 2 {
		t.Fatalf("down code = %d", code)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"host-reasons", path}); code != 1 {
		t.Fatalf("invalid code = %d", code)
	}
}

func discardStdio(t *testing.T) func() {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	return func() {
		os.Stdout, os.Stderr = oldOut, oldErr
		outW.Close()
		errW.Close()
		_, _ = io.Copy(io.Discard, outR)
		_, _ = io.Copy(io.Discard, errR)
		outR.Close()
		errR.Close()
	}
}

func TestHTMLHealthzIsNotAnOutage(t *testing.T) {
	status, detail := classifyUptime(200, 200, "<!DOCTYPE html>")
	if status != "ok" {
		t.Fatalf("status = %s", status)
	}
	if !strings.Contains(detail, "does not proxy") {
		t.Fatal(detail)
	}
}

func TestPlainOK(t *testing.T) {
	status, _ := classifyUptime(200, 200, "ok")
	if status != "ok" {
		t.Fatalf("status = %s", status)
	}
}

func TestBadGatewayFails(t *testing.T) {
	status, detail := classifyUptime(200, 502, "<html>bad gateway")
	if status != "fail" {
		t.Fatalf("status = %s", status)
	}
	if !strings.Contains(detail, "502") {
		t.Fatal(detail)
	}
}

func TestRootDownFails(t *testing.T) {
	status, _ := classifyUptime(503, 200, "ok")
	if status != "fail" {
		t.Fatalf("status = %s", status)
	}
}
