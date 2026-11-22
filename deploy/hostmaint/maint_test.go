package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRejectsUnknownCommand(t *testing.T) {
	if _, err := parseArgs([]string{"bash", "-c", "true"}); err == nil {
		t.Fatal("accepted a free-form command")
	}
}

func TestStatusRejectsRebootFlag(t *testing.T) {
	if _, err := parseArgs([]string{"status", "--reboot"}); err == nil {
		t.Fatal("status accepted --reboot")
	}
}

func TestApplyRebootFlag(t *testing.T) {
	args, err := parseArgs([]string{"apply", "--reboot"})
	if err != nil {
		t.Fatal(err)
	}
	if args.command != commandApply || !args.reboot {
		t.Fatalf("args = %+v", args)
	}
}

func TestMainRefusesWhenNotRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root would run apt")
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = writer
	t.Cleanup(func() {
		os.Stderr = old
		writer.Close()
		_, _ = io.Copy(io.Discard, reader)
		reader.Close()
	})
	if code := runMain([]string{"status"}); code != 1 {
		t.Fatalf("code = %d", code)
	}
}

func TestSecurityOriginIsSplitFromUpdates(t *testing.T) {
	text := strings.Join([]string{
		"Inst nginx [1.24.0-2ubuntu7.3] (1.24.0-2ubuntu7.21 Ubuntu:24.04/noble-security [amd64])",
		"Inst bash [5.2] (5.2-2 Ubuntu:24.04/noble-updates [amd64])",
		"Inst not a package (1 local [amd64])",
		"Conf nginx (1.24.0-2ubuntu7.21 Ubuntu:24.04/noble-security [amd64])",
	}, "\n")
	security, other := classifyAptSimulation(text)
	if strings.Join(security, ",") != "nginx" {
		t.Fatalf("security = %#v", security)
	}
	if strings.Join(other, ",") != "bash" {
		t.Fatalf("other = %#v", other)
	}
}

func TestPackageNamesAreCappedInThePayload(t *testing.T) {
	names := make([]string, 50)
	for i := range names {
		names[i] = fmt.Sprintf("libpkg%d", i)
	}
	payload := buildPayload(statusInput{
		collectedAt:         "2026-11-22T00:00:00Z",
		osName:              "Ubuntu",
		kernel:              "6.8.0",
		nginx:               "nginx/1.24.0",
		nginxPackage:        "1.24.0-2ubuntu7.21",
		opensshPackage:      "1:9.6p1",
		docker:              "not-installed",
		dockerSocket:        "absent",
		unattendedUpgrades:  true,
		rebootRequired:      false,
		securityUpgrades:    names,
		otherUpgradeCount:   3,
		listeners:           []string{"0.0.0.0:22"},
		unexpectedListeners: []string{},
		apiLocalHealth:      "ok",
		warnings:            []string{},
	})
	if len(payload.SecurityUpgrades) != maxPackages {
		t.Fatalf("len = %d", len(payload.SecurityUpgrades))
	}
	if payload.SecurityUpgradeCount != 50 {
		t.Fatalf("count = %d", payload.SecurityUpgradeCount)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]any
	if err := json.Unmarshal(encoded, &generic); err != nil {
		t.Fatal(err)
	}
	if _, ok := generic["env"]; ok {
		t.Fatal("payload has an env key")
	}
	if !strings.Contains(payload.Notes[0], "CVE-2026-99288") {
		t.Fatal(payload.Notes[0])
	}
}

func TestSSAndUnexpectedPorts(t *testing.T) {
	text := strings.Join([]string{
		"LISTEN 0 128 0.0.0.0:22 0.0.0.0:*",
		"LISTEN 0 511 127.0.0.1:8080 0.0.0.0:*",
		"LISTEN 0 511 *:443 *:*",
		"LISTEN 0 128 [::]:22 [::]:*",
		"LISTEN 0 511 0.0.0.0:8080 0.0.0.0:*",
	}, "\n")
	parsed := parseListeners(text)
	want := []listener{
		{"0.0.0.0", 22},
		{"127.0.0.1", 8080},
		{"*", 443},
		{"::", 22},
		{"0.0.0.0", 8080},
	}
	if len(parsed) != len(want) {
		t.Fatalf("parsed = %#v", parsed)
	}
	for i := range want {
		if parsed[i] != want[i] {
			t.Fatalf("parsed = %#v", parsed)
		}
	}
	unexpected := unexpectedListeners(parsed)
	if len(unexpected) != 1 || unexpected[0] != "0.0.0.0:8080" {
		t.Fatalf("unexpected = %#v", unexpected)
	}
}

func TestWorldBits(t *testing.T) {
	mode660 := 0o660
	mode666 := 0o666
	link := 0o120777
	if socketStateFromMode(&mode660, true, false) != "present-not-world-accessible" {
		t.Fatal("0660")
	}
	if socketStateFromMode(&mode666, true, false) != "world-accessible" {
		t.Fatal("0666")
	}
	if socketStateFromMode(nil, false, false) != "absent" {
		t.Fatal("missing")
	}
	if socketStateFromMode(&link, true, true) != "unknown" {
		t.Fatal("symlink")
	}
}

func TestOnlyLoopbackHealthz(t *testing.T) {
	if !allowedHealthURL("http://127.0.0.1:8080/healthz") {
		t.Fatal("ipv4 loopback")
	}
	if !allowedHealthURL("http://localhost:8080/healthz") {
		t.Fatal("localhost")
	}
	if allowedHealthURL("http://169.254.169.254/latest/meta-data") {
		t.Fatal("metadata host")
	}
	if allowedHealthURL("https://127.0.0.1:8080/healthz") {
		t.Fatal("https")
	}
	if allowedHealthURL("http://127.0.0.1:8080/healthz?x=1") {
		t.Fatal("query")
	}
	if allowedHealthURL("http://user:pass@127.0.0.1/healthz") {
		t.Fatal("userinfo")
	}
}

func TestMaintUserIgnoresRootAndJunk(t *testing.T) {
	t.Setenv("HOST_MAINT_USER", "root")
	t.Setenv("SUDO_USER", "github-deploy")
	if got := maintUser(); got != "github-deploy" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("HOST_MAINT_USER", "bad user")
	t.Setenv("SUDO_USER", "ok;rm")
	if got := maintUser(); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderIsLimitedToTheProgram(t *testing.T) {
	text, err := renderSudoers("github-deploy")
	if err != nil {
		t.Fatal(err)
	}
	const want = "# Managed by personal-site-host-maint. This script only.\n" +
		"github-deploy ALL=(root) NOPASSWD: /usr/local/sbin/personal-site-host-maint status, /usr/local/sbin/personal-site-host-maint configure, /usr/local/sbin/personal-site-host-maint apply, /usr/local/sbin/personal-site-host-maint apply --reboot\n"
	if text != want {
		t.Fatalf("sudoers =\n%s", text)
	}
	if strings.Contains(text, "NOPASSWD: ALL") {
		t.Fatal("sudoers grants ALL")
	}
	for _, bad := range []string{"root", "github deploy", "ok;rm"} {
		if _, err := renderSudoers(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestRoundTripJSONHasNoSecretKeys(t *testing.T) {
	payload := buildPayload(statusInput{
		collectedAt:         "2026-11-22T00:00:00Z",
		osName:              "Ubuntu 24.04",
		kernel:              "<script>",
		nginx:               "nginx/1.24.0",
		nginxPackage:        "1.24.0-2ubuntu7.21",
		opensshPackage:      "",
		docker:              "not-installed",
		dockerSocket:        "nope",
		unattendedUpgrades:  false,
		rebootRequired:      true,
		securityUpgrades:    []string{"nginx", "bad name"},
		otherUpgradeCount:   1,
		listeners:           []string{"127.0.0.1:8080"},
		unexpectedListeners: []string{},
		apiLocalHealth:      "ok",
		warnings:            []string{"unattended-upgrades is not enabled"},
	})
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("SESSION_SECRET")) || bytes.Contains(encoded, []byte("BEGIN OPENSSH")) {
		t.Fatal("encoded payload contains secret-like text")
	}
	if payload.DockerSocket != "unknown" {
		t.Fatalf("socket = %s", payload.DockerSocket)
	}
	if len(payload.SecurityUpgrades) != 1 || payload.SecurityUpgrades[0] != "nginx" {
		t.Fatalf("upgrades = %#v", payload.SecurityUpgrades)
	}
	if payload.Kernel != "<script>" {
		t.Fatalf("kernel = %q", payload.Kernel)
	}
}

func TestWriteTextUsesRequestedMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host-status.json")
	if err := writeText(path, "{}\n", modeStatus); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != modeStatus {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	sudoers := filepath.Join(dir, "personal-site-host-maint")
	if err := writeText(sudoers, "user ALL=(root) NOPASSWD: /usr/local/sbin/personal-site-host-maint status\n", modeSudoers); err != nil {
		t.Fatal(err)
	}
	info, err = os.Stat(sudoers)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != modeSudoers {
		t.Fatalf("sudoers mode = %o", info.Mode().Perm())
	}
	conf := filepath.Join(dir, "52personal-site-unattended")
	if err := writeText(conf, unattendedConf, modeConfig); err != nil {
		t.Fatal(err)
	}
	info, err = os.Stat(conf)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != modeConfig {
		t.Fatalf("config mode = %o", info.Mode().Perm())
	}
}

func TestUnattendedConfigIsSecurityOnly(t *testing.T) {
	if !strings.Contains(unattendedConf, `"${distro_id}:${distro_codename}-security"`) {
		t.Fatal("security origin missing")
	}
	if !strings.Contains(unattendedConf, `Automatic-Reboot "false"`) {
		t.Fatal("automatic reboot is not disabled")
	}
	if strings.Contains(unattendedConf, "-updates") {
		t.Fatal("updates pocket is allowed")
	}
	if serverTokensConf != "server_tokens off;\n" {
		t.Fatalf("server tokens = %q", serverTokensConf)
	}
	if !strings.Contains(serviceUnit, "ExecStart=/usr/local/sbin/personal-site-host-maint status\n") {
		t.Fatal(serviceUnit)
	}
}
