package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// version is set with -X main.version at release build time.
var version = "dev"

const (
	defaultHostStatusPath = "/var/lib/personal-site/host-status.json"
	maxHostStatusBytes    = 32 << 10
)

var (
	processStarted = time.Now().UTC()
	requestsTotal  atomic.Uint64
	responses5xx   atomic.Uint64
)

func healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

func withRequestMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		requestsTotal.Add(1)
		if rec.status >= 500 {
			responses5xx.Add(1)
		}
	})
}

type opsSnapshot struct {
	Version       string         `json:"version"`
	StartedAt     string         `json:"started_at"`
	UptimeSeconds int64          `json:"uptime_seconds"`
	RequestsTotal uint64         `json:"requests_total"`
	Responses5xx  uint64         `json:"responses_5xx"`
	LogLevel      string         `json:"log_level"`
	Host          hostStatusView `json:"host"`
}

type hostStatusView struct {
	Configured           bool     `json:"configured"`
	Hint                 string   `json:"hint,omitempty"`
	CollectedAt          string   `json:"collected_at,omitempty"`
	OS                   string   `json:"os,omitempty"`
	Kernel               string   `json:"kernel,omitempty"`
	Nginx                string   `json:"nginx,omitempty"`
	NginxPackage         string   `json:"nginx_package,omitempty"`
	OpenSSHPackage       string   `json:"openssh_package,omitempty"`
	Docker               string   `json:"docker,omitempty"`
	DockerSocket         string   `json:"docker_socket,omitempty"`
	UnattendedUpgrades   bool     `json:"unattended_upgrades"`
	RebootRequired       bool     `json:"reboot_required"`
	SecurityUpgradeCount int      `json:"security_upgrade_count"`
	SecurityUpgrades     []string `json:"security_upgrades,omitempty"`
	OtherUpgradeCount    int      `json:"other_upgrade_count"`
	Listeners            []string `json:"listeners,omitempty"`
	UnexpectedListeners  []string `json:"unexpected_listeners,omitempty"`
	APILocalHealth       string   `json:"api_local_health,omitempty"`
	Warnings             []string `json:"warnings,omitempty"`
	Notes                []string `json:"notes,omitempty"`
}

func adminOps(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentOpsSnapshot())
}

func currentOpsSnapshot() opsSnapshot {
	return opsSnapshot{
		Version:       version,
		StartedAt:     processStarted.Format(time.RFC3339),
		UptimeSeconds: int64(time.Since(processStarted).Seconds()),
		RequestsTotal: requestsTotal.Load(),
		Responses5xx:  responses5xx.Load(),
		LogLevel:      apiLogLevel,
		Host:          readHostStatus(),
	}
}

func hostStatusFile() string {
	p := strings.TrimSpace(os.Getenv("HOST_STATUS_PATH"))
	if p == "" {
		return defaultHostStatusPath
	}
	return p
}

func validHostStatusPath(p string) bool {
	if p == "" || !filepath.IsAbs(p) || strings.Contains(p, "..") {
		return false
	}
	return true
}

func readHostStatus() hostStatusView {
	p := hostStatusFile()
	if !validHostStatusPath(p) {
		return hostStatusView{Configured: false, Hint: "HOST_STATUS_PATH must be an absolute path without .."}
	}
	f, err := os.OpenFile(p, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return hostStatusView{
				Configured: false,
				Hint:       "Host status is not on this machine yet. After this API is deployed, run the Host maintenance workflow on main with mode configure.",
			}
		}
		slog.Error("host status open", "path", p, "err", err)
		return hostStatusView{Configured: false, Hint: "Host status file could not be read."}
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return hostStatusView{Configured: false, Hint: "Host status file could not be read."}
	}
	body, err := io.ReadAll(io.LimitReader(f, maxHostStatusBytes+1))
	if err != nil {
		slog.Error("host status read", "path", p, "err", err)
		return hostStatusView{Configured: false, Hint: "Host status file could not be read."}
	}
	if len(body) > maxHostStatusBytes {
		return hostStatusView{Configured: false, Hint: "Host status file is too large."}
	}
	var parsed hostStatusView
	if err := json.Unmarshal(body, &parsed); err != nil {
		slog.Error("host status json", "path", p, "err", err)
		return hostStatusView{Configured: false, Hint: "Host status file is not valid JSON."}
	}
	if strings.TrimSpace(parsed.CollectedAt) == "" {
		return hostStatusView{Configured: false, Hint: "Host status file is missing collected_at."}
	}
	return sanitizeHostStatus(parsed)
}

func sanitizeHostStatus(in hostStatusView) hostStatusView {
	in.Configured = true
	in.Hint = ""
	in.CollectedAt = clipText(in.CollectedAt, 40)
	in.OS = clipText(in.OS, 120)
	in.Kernel = clipText(in.Kernel, 80)
	in.Nginx = clipText(in.Nginx, 160)
	in.NginxPackage = clipText(in.NginxPackage, 80)
	in.OpenSSHPackage = clipText(in.OpenSSHPackage, 80)
	in.Docker = clipText(in.Docker, 80)
	in.DockerSocket = clipText(in.DockerSocket, 40)
	in.APILocalHealth = clipText(in.APILocalHealth, 16)
	in.SecurityUpgrades = clipList(in.SecurityUpgrades, 40, 80)
	in.Listeners = clipList(in.Listeners, 30, 80)
	in.UnexpectedListeners = clipList(in.UnexpectedListeners, 30, 80)
	in.Warnings = clipList(in.Warnings, 8, 200)
	in.Notes = clipList(in.Notes, 4, 500)
	if in.SecurityUpgradeCount < 0 {
		in.SecurityUpgradeCount = 0
	}
	if in.SecurityUpgradeCount > 10000 {
		in.SecurityUpgradeCount = 10000
	}
	if in.OtherUpgradeCount < 0 {
		in.OtherUpgradeCount = 0
	}
	if in.OtherUpgradeCount > 10000 {
		in.OtherUpgradeCount = 10000
	}
	return in
}

func clipText(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "")
}

func clipList(items []string, maxItems, maxLen int) []string {
	if len(items) == 0 {
		return nil
	}
	if len(items) > maxItems {
		items = items[:maxItems]
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = clipText(item, maxLen)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
