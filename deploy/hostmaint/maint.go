package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

const (
	statusPath       = "/var/lib/personal-site/host-status.json"
	scriptName       = "personal-site-host-maint"
	maxPackages      = 40
	maxListeners     = 30
	defaultHealthURL = "http://127.0.0.1:8080/healthz"

	modeStatus  os.FileMode = 0o640
	modeConfig  os.FileMode = 0o644
	modeSudoers os.FileMode = 0o440
	modeDir     os.FileMode = 0o750
)

type command string

const (
	commandStatus    command = "status"
	commandConfigure command = "configure"
	commandApply     command = "apply"
)

var (
	packageRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+.~:-]{0,120}$`)
	userRE    = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

	publicPorts   = map[int]struct{}{22: {}, 80: {}, 443: {}}
	loopbackHosts = map[string]struct{}{"127.0.0.1": {}, "::1": {}}
)

const kernelNote = "A zero security-upgrade count does not mean every kernel CVE is patched. " +
	"CVE-2026-99288 (Linux TCP, remote denial of service) had no Ubuntu fix as of 20 Nov 2026. " +
	"Unattended-upgrades installs a kernel when Ubuntu publishes one and reboots at 04:15 local time when a reboot is required."

const nginxNote = "CVE-2026-123007 and CVE-2026-123633 apply when HTTP/2 is enabled, which certbot usually turns on. " +
	"CVE-2026-118899 can hang a worker while it reads a TLS request body. " +
	"The fix is the Ubuntu security package (USN-9159-1 / USN-9182-1 or newer), not a version string of 1.30.7."

const unattendedConf = `// Managed by personal-site-host-maint. Security pockets only. Reboot at 04:15 local time when required.
Unattended-Upgrade::Allowed-Origins {
        "${distro_id}:${distro_codename}-security";
        "${distro_id}ESMApps:${distro_codename}-apps-security";
        "${distro_id}ESM:${distro_codename}-infra-security";
};
Unattended-Upgrade::Automatic-Reboot "true";
Unattended-Upgrade::Automatic-Reboot-Time "04:15";
Unattended-Upgrade::Automatic-Reboot-WithUsers "true";
Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
`

const periodicConf = `// Managed by personal-site-host-maint.
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::Download-Upgradeable-Packages "1";
APT::Periodic::AutocleanInterval "7";
`

const serverTokensConf = "server_tokens off;\n"

const serviceUnit = `[Unit]
Description=Write personal-site host status
After=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/personal-site-host-maint status
Nice=10
NoNewPrivileges=yes

[Install]
WantedBy=multi-user.target
`

const timerUnit = `[Unit]
Description=Daily personal-site host status

[Timer]
OnCalendar=daily
Persistent=true
RandomizedDelaySec=30m
OnBootSec=5min

[Install]
WantedBy=timers.target
`

type parsedArgs struct {
	command command
	reboot  bool
}

func parseArgs(argv []string) (parsedArgs, error) {
	if len(argv) == 0 {
		return parsedArgs{}, errors.New("a command is required: status, configure, or apply")
	}
	switch command(argv[0]) {
	case commandStatus, commandConfigure:
		if len(argv) != 1 {
			return parsedArgs{}, fmt.Errorf("unrecognized arguments for %s", argv[0])
		}
		return parsedArgs{command: command(argv[0])}, nil
	case commandApply:
		reboot := false
		for _, arg := range argv[1:] {
			if arg != "--reboot" {
				return parsedArgs{}, errors.New("unrecognized arguments for apply")
			}
			reboot = true
		}
		return parsedArgs{command: commandApply, reboot: reboot}, nil
	default:
		return parsedArgs{}, fmt.Errorf("invalid choice %q", argv[0])
	}
}

func runMain(argv []string) int {
	args, err := parseArgs(argv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 2
	}
	if os.Geteuid() != 0 {
		fmt.Fprintf(os.Stderr, "%s must run as root\n", scriptName)
		return 1
	}
	switch args.command {
	case commandStatus:
		return cmdStatus()
	case commandConfigure:
		return cmdConfigure()
	case commandApply:
		return cmdApply(args.reboot)
	default:
		panic("unhandled command: " + string(args.command))
	}
}

type listener struct {
	addr string
	port int
}

func classifyAptSimulation(text string) (security []string, other []string) {
	security = []string{}
	other = []string{}
	for _, line := range strings.Split(text, "\n") {
		name, origin, ok := parseInstLine(line)
		if !ok {
			continue
		}
		if strings.Contains(strings.ToLower(origin), "security") {
			security = append(security, name)
			continue
		}
		other = append(other, name)
	}
	return security, other
}

func parseInstLine(line string) (name string, origin string, ok bool) {
	stripped := strings.TrimSpace(line)
	if !strings.HasPrefix(stripped, "Inst ") {
		return "", "", false
	}
	rest := stripped[len("Inst "):]
	name, tail, found := strings.Cut(rest, " ")
	if !found {
		tail = ""
	}
	tail = strings.TrimLeft(tail, " ")
	if name == "" || !packageRE.MatchString(name) {
		return "", "", false
	}
	if !strings.HasPrefix(tail, "[") && !strings.HasPrefix(tail, "(") {
		return "", "", false
	}
	left := strings.Index(stripped, "(")
	right := strings.LastIndex(stripped, ")")
	if left < 0 || right <= left {
		return "", "", false
	}
	return name, stripped[left+1 : right], true
}

func parseListeners(ssText string) []listener {
	found := []listener{}
	for _, line := range strings.Split(ssText, "\n") {
		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}
		addr, port, ok := splitLocalAddress(parts[3])
		if !ok {
			continue
		}
		found = append(found, listener{addr: addr, port: port})
	}
	return found
}

func splitLocalAddress(local string) (string, int, bool) {
	host, sep, portText := rpartition(local, ":")
	if sep == "" {
		return local, 0, false
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1]
	}
	port, err := atoiPort(portText)
	if err != nil || port < 0 || port > 65535 {
		return local, 0, false
	}
	return host, port, true
}

func rpartition(value, sep string) (head string, found string, tail string) {
	index := strings.LastIndex(value, sep)
	if index < 0 {
		return value, "", ""
	}
	return value[:index], sep, value[index+len(sep):]
}

func atoiPort(text string) (int, error) {
	if text == "" {
		return 0, errors.New("empty port")
	}
	n := 0
	for _, r := range text {
		if r < '0' || r > '9' {
			return 0, errors.New("bad port")
		}
		n = n*10 + int(r-'0')
		if n > 65535 {
			return 0, errors.New("port out of range")
		}
	}
	return n, nil
}

func unexpectedListeners(listeners []listener) []string {
	bad := []string{}
	for _, item := range listeners {
		if _, loopback := loopbackHosts[item.addr]; loopback {
			continue
		}
		if _, allowed := publicPorts[item.port]; allowed {
			continue
		}
		bad = append(bad, fmt.Sprintf("%s:%d", item.addr, item.port))
		if len(bad) >= maxListeners {
			break
		}
	}
	return bad
}

func socketStateFromMode(mode *int, exists bool, isLink bool) string {
	if !exists {
		return "absent"
	}
	if mode == nil || isLink {
		return "unknown"
	}
	if *mode&0o007 != 0 {
		return "world-accessible"
	}
	return "present-not-world-accessible"
}

func allowedHealthURL(raw string) bool {
	parts, err := parseURL(raw)
	if err != nil {
		return false
	}
	if parts.scheme != "http" {
		return false
	}
	if parts.username != "" || parts.password != "" || parts.query != "" || parts.fragment != "" {
		return false
	}
	if parts.hostname != "127.0.0.1" && parts.hostname != "localhost" {
		return false
	}
	return parts.path == "/healthz"
}

type urlParts struct {
	scheme   string
	username string
	password string
	hostname string
	path     string
	query    string
	fragment string
}

func parseURL(raw string) (urlParts, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return urlParts{}, err
	}
	username := ""
	password := ""
	if parsed.User != nil {
		username = parsed.User.Username()
		password, _ = parsed.User.Password()
	}
	return urlParts{
		scheme:   parsed.Scheme,
		username: username,
		password: password,
		hostname: parsed.Hostname(),
		path:     parsed.Path,
		query:    parsed.RawQuery,
		fragment: parsed.Fragment,
	}, nil
}

func renderSudoers(user string) (string, error) {
	if user == "root" || !userRE.MatchString(user) {
		return "", errors.New("refusing sudoers user")
	}
	commands := strings.Join([]string{
		"/usr/local/sbin/personal-site-host-maint status",
		"/usr/local/sbin/personal-site-host-maint configure",
		"/usr/local/sbin/personal-site-host-maint apply",
		"/usr/local/sbin/personal-site-host-maint apply --reboot",
	}, ", ")
	return "# Managed by personal-site-host-maint. This script only.\n" +
		user + " ALL=(root) NOPASSWD: " + commands + "\n", nil
}

func maintUser() string {
	for _, key := range []string{"HOST_MAINT_USER", "SUDO_USER"} {
		user := strings.TrimSpace(os.Getenv(key))
		if user != "" && user != "root" && userRE.MatchString(user) {
			return user
		}
	}
	return ""
}

type statusPayload struct {
	Configured           bool     `json:"configured"`
	CollectedAt          string   `json:"collected_at"`
	OS                   string   `json:"os"`
	Kernel               string   `json:"kernel"`
	Nginx                string   `json:"nginx"`
	NginxPackage         string   `json:"nginx_package"`
	OpenSSHPackage       string   `json:"openssh_package"`
	Docker               string   `json:"docker"`
	DockerSocket         string   `json:"docker_socket"`
	UnattendedUpgrades   bool     `json:"unattended_upgrades"`
	RebootRequired       bool     `json:"reboot_required"`
	SecurityUpgradeCount int      `json:"security_upgrade_count"`
	SecurityUpgrades     []string `json:"security_upgrades"`
	OtherUpgradeCount    int      `json:"other_upgrade_count"`
	Listeners            []string `json:"listeners"`
	UnexpectedListeners  []string `json:"unexpected_listeners"`
	APILocalHealth       string   `json:"api_local_health"`
	Warnings             []string `json:"warnings"`
	Notes                []string `json:"notes"`
}

type statusInput struct {
	collectedAt         string
	osName              string
	kernel              string
	nginx               string
	nginxPackage        string
	opensshPackage      string
	docker              string
	dockerSocket        string
	unattendedUpgrades  bool
	rebootRequired      bool
	securityUpgrades    []string
	otherUpgradeCount   int
	listeners           []string
	unexpectedListeners []string
	apiLocalHealth      string
	warnings            []string
}

func buildPayload(in statusInput) statusPayload {
	security := make([]string, 0, len(in.securityUpgrades))
	for _, name := range in.securityUpgrades {
		if packageRE.MatchString(name) {
			security = append(security, name)
		}
	}
	count := len(security)
	if count > 10000 {
		count = 10000
	}
	if len(security) > maxPackages {
		security = security[:maxPackages]
	}
	socketState := in.dockerSocket
	switch socketState {
	case "absent", "unknown", "world-accessible", "present-not-world-accessible":
	default:
		socketState = "unknown"
	}
	health := in.apiLocalHealth
	if health != "ok" && health != "down" {
		health = "down"
	}
	docker := clip(in.docker, 80)
	if docker == "" {
		docker = "not-installed"
	}
	other := in.otherUpgradeCount
	if other < 0 {
		other = 0
	}
	if other > 10000 {
		other = 10000
	}
	return statusPayload{
		Configured:           true,
		CollectedAt:          clip(in.collectedAt, 40),
		OS:                   clip(in.osName, 120),
		Kernel:               clip(in.kernel, 80),
		Nginx:                clip(in.nginx, 160),
		NginxPackage:         clip(in.nginxPackage, 80),
		OpenSSHPackage:       clip(in.opensshPackage, 80),
		Docker:               docker,
		DockerSocket:         socketState,
		UnattendedUpgrades:   in.unattendedUpgrades,
		RebootRequired:       in.rebootRequired,
		SecurityUpgradeCount: count,
		SecurityUpgrades:     security,
		OtherUpgradeCount:    other,
		Listeners:            clipList(in.listeners, maxListeners, 80),
		UnexpectedListeners:  clipList(in.unexpectedListeners, maxListeners, 80),
		APILocalHealth:       health,
		Warnings:             clipList(in.warnings, 8, 200),
		Notes:                []string{kernelNote, nginxNote},
	}
}

func clipList(items []string, limit int, each int) []string {
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, clip(item, each))
	}
	return out
}

func clip(value string, limit int) string {
	text := strings.TrimSpace(value)
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

func tail(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[len(runes)-limit:])
}
