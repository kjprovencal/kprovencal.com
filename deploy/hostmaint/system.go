package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type cmdResult struct {
	code   int
	stdout string
	stderr string
}

func run(argv []string, timeout time.Duration) cmdResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = commandEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		errText := stderr.String()
		if errText == "" {
			errText = "timed out"
		}
		return cmdResult{code: 124, stdout: stdout.String(), stderr: errText}
	}
	if err == nil {
		return cmdResult{code: 0, stdout: stdout.String(), stderr: stderr.String()}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return cmdResult{code: exitErr.ExitCode(), stdout: stdout.String(), stderr: stderr.String()}
	}
	return cmdResult{code: 1, stdout: stdout.String(), stderr: err.Error()}
}

func commandEnv() []string {
	drop := map[string]struct{}{
		"DEBIAN_FRONTEND":          {},
		"NEEDRESTART_MODE":         {},
		"APT_LISTCHANGES_FRONTEND": {},
	}
	env := make([]string, 0, len(os.Environ())+3)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, skip := drop[key]; skip {
			continue
		}
		env = append(env, entry)
	}
	return append(env,
		"DEBIAN_FRONTEND=noninteractive",
		"NEEDRESTART_MODE=a",
		"APT_LISTCHANGES_FRONTEND=none",
	)
}

func probeLocalAPI(rawURL string) string {
	if !allowedHealthURL(rawURL) {
		return "down"
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			Proxy: nil,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "down"
	}
	resp, err := client.Do(req)
	if err != nil {
		return "down"
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32))
	if err != nil {
		return "down"
	}
	if resp.StatusCode == 200 && string(body) == "ok" {
		return "ok"
	}
	return "down"
}

func dpkgVersion(pkg string) string {
	if !packageRE.MatchString(pkg) {
		return ""
	}
	result := run([]string{"dpkg-query", "-W", "-f", "${Version}", pkg}, 30*time.Second)
	if result.code != 0 {
		return ""
	}
	return strings.TrimSpace(result.stdout)
}

func packageInstalled(pkg string) bool {
	if !packageRE.MatchString(pkg) {
		return false
	}
	result := run([]string{"dpkg-query", "-W", "-f", "${Status}", pkg}, 30*time.Second)
	return result.code == 0 && strings.Contains(result.stdout, "install ok installed")
}

func collectStatus() statusPayload {
	warnings := []string{}
	updated := run([]string{"apt-get", "update", "-qq"}, 300*time.Second)
	if updated.code != 0 {
		warnings = append(warnings, "apt-get update failed")
	}
	simulated := run([]string{"apt-get", "-s", "upgrade"}, 180*time.Second)
	var security, other []string
	if simulated.code != 0 {
		warnings = append(warnings, "apt-get simulation failed")
		security, other = []string{}, []string{}
	} else {
		security, other = classifyAptSimulation(simulated.stdout)
	}

	nginxText := ""
	nginxBin, err := exec.LookPath("nginx")
	if err != nil {
		warnings = append(warnings, "nginx binary not found")
	} else {
		version := run([]string{nginxBin, "-v"}, 15*time.Second)
		nginxText = strings.TrimSpace(version.stderr)
		if nginxText == "" {
			nginxText = strings.TrimSpace(version.stdout)
		}
	}

	openssh := dpkgVersion("openssh-server")
	if openssh == "" {
		warnings = append(warnings, "openssh-server package not installed")
	}
	unattended := unattendedEnabled()
	if !unattended {
		warnings = append(warnings, "unattended-upgrades is not enabled")
	}

	healthURL := strings.TrimSpace(os.Getenv("API_HEALTH_URL"))
	if healthURL == "" {
		healthURL = defaultHealthURL
	}
	health := "down"
	if !allowedHealthURL(healthURL) {
		warnings = append(warnings, "API health URL is not a loopback /healthz address")
	} else {
		health = probeLocalAPI(healthURL)
	}

	ss := run([]string{"ss", "-H", "-ltn"}, 15*time.Second)
	listeners := []string{}
	unexpected := []string{}
	if ss.code != 0 {
		warnings = append(warnings, "ss failed")
	} else {
		parsed := parseListeners(ss.stdout)
		listeners = make([]string, 0, len(parsed))
		for _, item := range parsed {
			listeners = append(listeners, fmt.Sprintf("%s:%d", item.addr, item.port))
		}
		unexpected = unexpectedListeners(parsed)
	}

	return buildPayload(statusInput{
		collectedAt:         time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		osName:              readPrettyName(),
		kernel:              kernelRelease(),
		nginx:               nginxText,
		nginxPackage:        dpkgVersion("nginx"),
		opensshPackage:      openssh,
		docker:              dockerPackage(),
		dockerSocket:        classifySocket("/var/run/docker.sock"),
		unattendedUpgrades:  unattended,
		rebootRequired:      pathExists("/var/run/reboot-required"),
		securityUpgrades:    security,
		otherUpgradeCount:   len(other),
		listeners:           listeners,
		unexpectedListeners: unexpected,
		apiLocalHealth:      health,
		warnings:            warnings,
	})
}

func unattendedEnabled() bool {
	if !packageInstalled("unattended-upgrades") {
		return false
	}
	result := run([]string{"systemctl", "is-enabled", "apt-daily-upgrade.timer"}, 20*time.Second)
	state := strings.TrimSpace(result.stdout)
	return result.code == 0 && (state == "enabled" || state == "static")
}

func dockerPackage() string {
	for _, pkg := range []string{"docker-ce", "docker.io"} {
		if version := dpkgVersion(pkg); version != "" {
			return version
		}
	}
	return "not-installed"
}

func readPrettyName() string {
	text, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(text), "\n") {
		if !strings.HasPrefix(line, "PRETTY_NAME=") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "PRETTY_NAME="))
		return strings.Trim(value, `"`)
	}
	return ""
}

func kernelRelease() string {
	var uname syscall.Utsname
	if err := syscall.Uname(&uname); err != nil {
		return ""
	}
	return cstring(uname.Release[:])
}

func cstring(buf []int8) string {
	raw := make([]byte, 0, len(buf))
	for _, c := range buf {
		if c == 0 {
			break
		}
		raw = append(raw, byte(c))
	}
	return string(raw)
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func classifySocket(path string) string {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "absent"
		}
		return "unknown"
	}
	if info.Mode()&os.ModeSymlink != 0 {
		info, err = os.Stat(path)
		if err != nil {
			return "unknown"
		}
	}
	mode := int(info.Mode().Perm())
	return socketStateFromMode(&mode, true, false)
}

func writeText(path string, data string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(filepath.Dir(path), filepath.Base(path)+".tmp")
	file, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, writeErr := file.Write([]byte(data))
	closeErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(tmp)
		return writeErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Chmod(path, mode)
}

func marshalStatus(payload statusPayload) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeStatus(payload statusPayload, path string) error {
	encoded, err := marshalStatus(payload)
	if err != nil {
		return err
	}
	if err := writeText(path, string(encoded), modeStatus); err != nil {
		return err
	}
	group, err := user.LookupGroup("www-data")
	if err != nil {
		var unknown user.UnknownGroupError
		if errors.As(err, &unknown) {
			fmt.Fprintln(os.Stderr, "group www-data does not exist; status file stays root-owned")
			return nil
		}
		return err
	}
	gid, err := strconv.Atoi(group.Gid)
	if err != nil {
		return err
	}
	if err := os.Chown(path, 0, gid); err != nil {
		if os.IsPermission(err) && os.Geteuid() != 0 {
			return nil
		}
		return err
	}
	parent := filepath.Dir(path)
	if err := os.Chown(parent, 0, gid); err != nil {
		if os.IsPermission(err) && os.Geteuid() != 0 {
			return nil
		}
		return err
	}
	if err := os.Chmod(parent, modeDir); err != nil {
		if os.IsPermission(err) && os.Geteuid() != 0 {
			return nil
		}
		return err
	}
	return nil
}

func installServerTokens() (string, error) {
	if _, err := exec.LookPath("nginx"); err != nil {
		return "", nil
	}
	path := "/etc/nginx/conf.d/personal-site-server-tokens.conf"
	if err := writeText(path, serverTokensConf, modeConfig); err != nil {
		return "", err
	}
	test := run([]string{"nginx", "-t"}, 30*time.Second)
	if test.code != 0 {
		_ = os.Remove(path)
		_ = run([]string{"nginx", "-t"}, 30*time.Second)
		detail := strings.TrimSpace(test.stderr)
		if detail == "" {
			detail = strings.TrimSpace(test.stdout)
		}
		if detail == "" {
			detail = "nginx -t failed"
		}
		lines := strings.Split(detail, "\n")
		tailLine := lines[len(lines)-1]
		runes := []rune(tailLine)
		if len(runes) > 180 {
			tailLine = string(runes[:180])
		}
		return "nginx -t failed; removed server_tokens drop-in: " + tailLine, nil
	}
	reload := run([]string{"systemctl", "reload", "nginx"}, 30*time.Second)
	if reload.code != 0 {
		return "nginx reload failed after server_tokens change", nil
	}
	return "", nil
}

func installSudoers(userName string) string {
	if userName == "" || userName == "root" {
		return ""
	}
	if !userRE.MatchString(userName) {
		return "HOST_MAINT_USER is not a safe username; sudoers not installed"
	}
	if _, err := exec.LookPath("visudo"); err != nil {
		return "visudo is not installed; sudoers not installed"
	}
	text, err := renderSudoers(userName)
	if err != nil {
		return "HOST_MAINT_USER is not a safe username; sudoers not installed"
	}
	path := "/etc/sudoers.d/personal-site-host-maint"
	tmp := path + ".tmp"
	if err := writeText(tmp, text, modeSudoers); err != nil {
		return "visudo rejected the host-maint sudoers drop-in; not installed"
	}
	check := run([]string{"visudo", "-c", "-f", tmp}, 20*time.Second)
	if check.code != 0 {
		_ = os.Remove(tmp)
		return "visudo rejected the host-maint sudoers drop-in; not installed"
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "visudo rejected the host-maint sudoers drop-in; not installed"
	}
	if err := os.Chmod(path, modeSudoers); err != nil {
		return "visudo rejected the host-maint sudoers drop-in; not installed"
	}
	return ""
}

func cmdStatus() int {
	payload := collectStatus()
	if err := writeStatus(payload, statusPath); err != nil {
		fmt.Fprintf(os.Stderr, "could not write status file: %s\n", err.Error())
		return 1
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		fmt.Fprintf(os.Stderr, "could not write status file: %s\n", err.Error())
		return 1
	}
	fmt.Fprint(os.Stdout, buf.String())
	return 0
}

func cmdConfigure() int {
	updated := run([]string{"apt-get", "update", "-qq"}, 300*time.Second)
	if updated.code != 0 {
		fmt.Fprintln(os.Stderr, "apt-get update failed")
		fmt.Fprintln(os.Stderr, tail(updated.stderr, 1000))
		return 1
	}
	installed := run([]string{"apt-get", "install", "-y", "unattended-upgrades"}, 600*time.Second)
	if installed.code != 0 {
		fmt.Fprintln(os.Stderr, "could not install unattended-upgrades")
		fmt.Fprintln(os.Stderr, tail(installed.stderr, 1000))
		return 1
	}
	if err := writeText("/etc/apt/apt.conf.d/52personal-site-unattended", unattendedConf, modeConfig); err != nil {
		fmt.Fprintf(os.Stderr, "could not write unattended config: %s\n", err.Error())
		return 1
	}
	if err := writeText("/etc/apt/apt.conf.d/52personal-site-periodic", periodicConf, modeConfig); err != nil {
		fmt.Fprintf(os.Stderr, "could not write periodic config: %s\n", err.Error())
		return 1
	}
	for _, timer := range []string{"apt-daily.timer", "apt-daily-upgrade.timer"} {
		enabled := run([]string{"systemctl", "enable", "--now", timer}, 30*time.Second)
		if enabled.code != 0 {
			fmt.Fprintf(os.Stderr, "could not enable %s\n", timer)
			fmt.Fprintln(os.Stderr, tail(enabled.stderr, 500))
			return 1
		}
	}
	if err := writeText("/etc/systemd/system/personal-site-host-status.service", serviceUnit, modeConfig); err != nil {
		fmt.Fprintf(os.Stderr, "could not write host status service: %s\n", err.Error())
		return 1
	}
	if err := writeText("/etc/systemd/system/personal-site-host-status.timer", timerUnit, modeConfig); err != nil {
		fmt.Fprintf(os.Stderr, "could not write host status timer: %s\n", err.Error())
		return 1
	}
	reloaded := run([]string{"systemctl", "daemon-reload"}, 30*time.Second)
	if reloaded.code != 0 {
		fmt.Fprintln(os.Stderr, "systemctl daemon-reload failed")
		return 1
	}
	timer := run([]string{"systemctl", "enable", "--now", "personal-site-host-status.timer"}, 30*time.Second)
	if timer.code != 0 {
		fmt.Fprintln(os.Stderr, "could not enable personal-site-host-status.timer")
		return 1
	}
	tokenWarning, tokenErr := installServerTokens()
	if tokenErr != nil {
		fmt.Fprintf(os.Stderr, "could not write server_tokens config: %s\n", tokenErr.Error())
		return 1
	}
	if tokenWarning != "" {
		fmt.Fprintln(os.Stderr, tokenWarning)
		if strings.HasPrefix(tokenWarning, "nginx reload failed") {
			return 1
		}
	}
	if sudoWarning := installSudoers(maintUser()); sudoWarning != "" {
		fmt.Fprintln(os.Stderr, sudoWarning)
	}
	return cmdStatus()
}

func cmdApply(reboot bool) int {
	if !packageInstalled("unattended-upgrades") {
		fmt.Fprintln(os.Stderr, "unattended-upgrades is not installed; run configure first")
		return 1
	}
	upgraded := run([]string{"unattended-upgrade", "-v"}, 1800*time.Second)
	if upgraded.stdout != "" {
		fmt.Fprintln(os.Stderr, tail(upgraded.stdout, 4000))
	}
	if upgraded.code != 0 {
		fmt.Fprintln(os.Stderr, tail(upgraded.stderr, 2000))
		return 1
	}
	if _, err := exec.LookPath("nginx"); err == nil {
		test := run([]string{"nginx", "-t"}, 30*time.Second)
		if test.code != 0 {
			fmt.Fprintln(os.Stderr, "nginx -t failed; did not reload")
			return 1
		}
		reload := run([]string{"systemctl", "reload", "nginx"}, 30*time.Second)
		if reload.code != 0 {
			fmt.Fprintln(os.Stderr, "nginx reload failed")
			return 1
		}
	}
	if code := cmdStatus(); code != 0 {
		return code
	}
	if !reboot {
		return 0
	}
	scheduled := run([]string{"systemd-run", "--on-active=8s", "systemctl", "reboot"}, 20*time.Second)
	if scheduled.code != 0 {
		fmt.Fprintln(os.Stderr, "could not schedule reboot")
		fmt.Fprintln(os.Stderr, tail(scheduled.stderr, 500))
		return 1
	}
	fmt.Fprintln(os.Stderr, "reboot scheduled in about 8 seconds")
	return 0
}
