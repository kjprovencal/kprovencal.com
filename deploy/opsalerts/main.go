package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) int {
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "usage: opsalerts summarize-audit|host-reasons|uptime ...")
		return 1
	}
	command, rest := argv[0], argv[1:]
	switch command {
	case "summarize-audit":
		return cmdSummarize(rest)
	case "host-reasons":
		return cmdHost(rest)
	case "uptime":
		return cmdUptime(rest)
	default:
		fmt.Fprintln(os.Stderr, "usage: opsalerts summarize-audit|host-reasons|uptime ...")
		return 1
	}
}

func oneLine(value any, limit int) string {
	return clip(strings.Join(strings.Fields(displayText(value)), " "), limit)
}

func displayText(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "True"
		}
		return "False"
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case nil:
		return "None"
	default:
		return fmt.Sprint(v)
	}
}

func clip(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
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

func npmHighFindings(doc any) []string {
	object, ok := doc.(map[string]any)
	if !ok {
		return []string{}
	}
	vulns, ok := object["vulnerabilities"].(map[string]any)
	if !ok {
		return []string{}
	}
	names := make([]string, 0, len(vulns))
	for name := range vulns {
		names = append(names, name)
	}
	sort.Strings(names)
	lines := []string{}
	for _, name := range names {
		info, ok := vulns[name].(map[string]any)
		if !ok {
			continue
		}
		severity := strings.ToLower(asString(info["severity"]))
		if severity != "high" && severity != "critical" {
			continue
		}
		title := viaTitle(info["via"])
		suffix := ""
		if title != "" {
			suffix = " — " + oneLine(title, 180)
		}
		lines = append(lines, severity+": "+oneLine(name, 80)+suffix)
	}
	return lines
}

func viaTitle(via any) string {
	items, ok := via.([]any)
	if !ok {
		return ""
	}
	for _, item := range items {
		switch value := item.(type) {
		case map[string]any:
			title, present := value["title"]
			if !present || !truthy(title) {
				continue
			}
			return asString(title)
		case string:
			return value
		}
	}
	return ""
}

func truthy(value any) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != ""
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		return true
	}
}

func asString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return displayText(v)
	}
}

func moderateCount(doc any) int {
	object, ok := doc.(map[string]any)
	if !ok {
		return 0
	}
	meta, ok := object["metadata"].(map[string]any)
	if !ok {
		return 0
	}
	vulns, ok := meta["vulnerabilities"].(map[string]any)
	if !ok {
		return 0
	}
	raw, ok := vulns["moderate"]
	if !ok || raw == nil {
		return 0
	}
	n, err := asInt(raw)
	if err != nil {
		return 0
	}
	return n
}

func asInt(value any) (int, error) {
	switch n := value.(type) {
	case bool:
		if n {
			return 1, nil
		}
		return 0, nil
	case int:
		return n, nil
	case int64:
		return int(n), nil
	case float64:
		return int(n), nil
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i), nil
		}
		f, err := n.Float64()
		if err != nil {
			return 0, err
		}
		return int(f), nil
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(n))
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, errors.New("not an int")
	}
}

func npmFailed(code int, doc any) bool {
	object, ok := doc.(map[string]any)
	if (code != 0 && code != 1) || !ok {
		return true
	}
	_, hasError := object["error"]
	_, hasVulns := object["vulnerabilities"]
	return hasError && !hasVulns
}

func govulnKind(code int) string {
	switch code {
	case 0:
		return "clean"
	case 3:
		return "findings"
	default:
		return "error"
	}
}

func summarizeAudit(govulnCode int, govulnLog string, npmCode int, npmDoc any) (int, string) {
	govuln := govulnKind(govulnCode)
	findings := npmHighFindings(npmDoc)
	if govuln == "error" || npmFailed(npmCode, npmDoc) {
		body := strings.Join([]string{
			"## Dependency CVE audit failed",
			"",
			fmt.Sprintf("govulncheck exit %d. npm audit exit %d.", govulnCode, npmCode),
			"",
			"```",
			tail(govulnLog, 4000),
			"```",
			"",
		}, "\n")
		return 1, body
	}
	actionable := govuln == "findings" || len(findings) > 0
	lines := []string{"## Dependency CVE audit", ""}
	if len(findings) > 0 {
		lines = append(lines, "npm high or critical:")
		for _, item := range findings {
			lines = append(lines, "- "+item)
		}
	} else {
		lines = append(lines, "No high or critical npm findings.")
	}
	lines = append(lines, fmt.Sprintf("Moderate npm findings (not alerting): %d.", moderateCount(npmDoc)), "")
	if govuln == "findings" {
		lines = append(lines, "govulncheck reported vulnerabilities:")
	} else {
		lines = append(lines, "govulncheck reported no vulnerabilities.")
	}
	lines = append(lines, "", "```", tail(govulnLog, 8000), "```", "")
	code := 0
	if actionable {
		code = 2
	}
	return code, strings.Join(lines, "\n")
}

func hostReasons(doc any) []string {
	object, ok := doc.(map[string]any)
	if !ok {
		return []string{"host status was not a JSON object"}
	}
	reasons := []string{}
	count := securityCount(object["security_upgrade_count"])
	if count > 0 {
		shown := ""
		if names, ok := asList(object["security_upgrades"]); ok {
			parts := make([]string, 0, min(len(names), 40))
			for _, name := range names {
				if len(parts) >= 40 {
					break
				}
				parts = append(parts, oneLine(name, 80))
			}
			shown = strings.Join(parts, ", ")
		}
		suffix := ""
		if shown != "" {
			suffix = ": " + shown
		}
		reasons = append(reasons, fmt.Sprintf("%d security update(s) pending%s", count, suffix))
	}
	if reboot, ok := object["reboot_required"].(bool); ok && reboot {
		reasons = append(reasons, "reboot required")
	}
	if object["api_local_health"] == "down" {
		reasons = append(reasons, "loopback API /healthz is down")
	}
	if object["docker_socket"] == "world-accessible" {
		reasons = append(reasons, "Docker socket is world-accessible")
	}
	if unexpected, ok := asList(object["unexpected_listeners"]); ok && len(unexpected) > 0 {
		parts := make([]string, 0, min(len(unexpected), 30))
		for _, item := range unexpected {
			if len(parts) >= 30 {
				break
			}
			parts = append(parts, oneLine(item, 80))
		}
		reasons = append(reasons, "unexpected listeners: "+strings.Join(parts, ", "))
	}
	if warnings, ok := asList(object["warnings"]); ok {
		for i, warning := range warnings {
			if i >= 8 {
				break
			}
			reasons = append(reasons, oneLine(warning, 200))
		}
	}
	return reasons
}

func securityCount(value any) int {
	if value == nil || value == false || value == "" {
		return 0
	}
	switch n := value.(type) {
	case int:
		if n > 0 {
			return n
		}
	case int64:
		if n > 0 {
			return int(n)
		}
	case json.Number:
		parsed, err := n.Int64()
		if err == nil && parsed > 0 {
			return int(parsed)
		}
	default:
		return 0
	}
	return 0
}

func asList(value any) ([]any, bool) {
	switch list := value.(type) {
	case []any:
		return list, true
	case []string:
		out := make([]any, len(list))
		for i, item := range list {
			out[i] = item
		}
		return out, true
	default:
		return nil, false
	}
}

func formatHostBody(doc any, reasons []string) string {
	var b strings.Builder
	b.WriteString("## Host watch\n\n")
	if len(reasons) == 0 {
		b.WriteString("Nothing actionable.\n")
	} else {
		b.WriteString("Actionable:\n")
		for _, reason := range reasons {
			b.WriteString("- ")
			b.WriteString(reason)
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	if object, ok := doc.(map[string]any); ok {
		for _, key := range []string{
			"collected_at",
			"os",
			"kernel",
			"nginx",
			"nginx_package",
			"openssh_package",
			"docker",
			"docker_socket",
			"api_local_health",
			"reboot_required",
			"unattended_upgrades",
			"security_upgrade_count",
			"other_upgrade_count",
		} {
			value, present := object[key]
			if !present {
				continue
			}
			b.WriteString("- ")
			b.WriteString(key)
			b.WriteString(": ")
			b.WriteString(oneLine(value, 300))
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	b.WriteString("On main, run Host maintenance. `configure` enables unattended security upgrades. ")
	b.WriteString("`apply` installs them now. Check reboot only when reboot_required is true.\n")
	return b.String()
}

func classifyUptime(rootCode int, healthCode int, healthBody string) (string, string) {
	if rootCode != 200 {
		return "fail", fmt.Sprintf("GET / returned HTTP %d.", rootCode)
	}
	stripped := strings.TrimSpace(healthBody)
	if healthCode == 200 && stripped == "ok" {
		return "ok", "GET / returned 200 and GET /healthz returned ok."
	}
	if healthCode == 200 && strings.HasPrefix(stripped, "<") {
		return "ok", "GET / returned 200. GET /healthz returned HTML, so the live vhost does not proxy /healthz yet."
	}
	sample := oneLine(stripped, 80)
	return "fail", fmt.Sprintf("GET / returned 200 but GET /healthz returned HTTP %d (%s).", healthCode, sample)
}

func readJSON(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeJSON(data)
}

func decodeJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("extra data")
		}
		return nil, err
	}
	return normalize(value), nil
}

func normalize(value any) any {
	switch n := value.(type) {
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i)
		}
		if f, err := n.Float64(); err == nil {
			return f
		}
		return n.String()
	case map[string]any:
		for key, child := range n {
			n[key] = normalize(child)
		}
		return n
	case []any:
		for i, child := range n {
			n[i] = normalize(child)
		}
		return n
	default:
		return value
	}
}

func readText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.ToValidUTF8(string(data), "\uFFFD"), nil
}

func cmdSummarize(argv []string) int {
	fs := flag.NewFlagSet("summarize-audit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	govulnExitText := fs.String("govuln-exit", "", "govulncheck exit code")
	govulnLog := fs.String("govuln-log", "", "govulncheck log")
	npmExitText := fs.String("npm-exit", "", "npm audit exit code")
	npmAudit := fs.String("npm-audit", "", "npm audit JSON")
	if err := fs.Parse(argv); err != nil {
		return 1
	}
	if *govulnExitText == "" || *govulnLog == "" || *npmExitText == "" || *npmAudit == "" {
		fmt.Fprintln(os.Stderr, "summarize-audit requires --govuln-exit, --govuln-log, --npm-exit, and --npm-audit")
		return 1
	}
	govulnExit, err := strconv.Atoi(*govulnExitText)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not read govuln exit code")
		return 1
	}
	npmExit, err := strconv.Atoi(*npmExitText)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not read npm exit code")
		return 1
	}
	log, err := readText(*govulnLog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not read govuln log: %s\n", err.Error())
		return 1
	}
	doc, err := readJSON(*npmAudit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not read %s: %s\n", baseName(*npmAudit), err.Error())
		code, body := summarizeAudit(govulnExit, log, npmExit, map[string]any{"error": "unreadable"})
		fmt.Fprint(os.Stdout, body)
		if code == 0 {
			return 1
		}
		return code
	}
	code, body := summarizeAudit(govulnExit, log, npmExit, doc)
	fmt.Fprint(os.Stdout, body)
	return code
}

func cmdHost(argv []string) int {
	fs := flag.NewFlagSet("host-reasons", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(argv); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "host-reasons requires a status file")
		return 1
	}
	path := fs.Arg(0)
	doc, err := readJSON(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not read %s: %s\n", baseName(path), err.Error())
		fmt.Fprintln(os.Stdout, "Host status JSON could not be read.")
		return 1
	}
	reasons := hostReasons(doc)
	fmt.Fprint(os.Stdout, formatHostBody(doc, reasons))
	if len(reasons) > 0 {
		return 2
	}
	return 0
}

func cmdUptime(argv []string) int {
	fs := flag.NewFlagSet("uptime", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	rootText := fs.String("root-code", "", "GET / status")
	healthText := fs.String("health-code", "", "GET /healthz status")
	bodyFile := fs.String("health-body-file", "", "GET /healthz body")
	if err := fs.Parse(argv); err != nil {
		return 1
	}
	if *rootText == "" || *healthText == "" || *bodyFile == "" {
		fmt.Fprintln(os.Stderr, "uptime requires --root-code, --health-code, and --health-body-file")
		return 1
	}
	rootCode, err := strconv.Atoi(*rootText)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not read root code")
		return 1
	}
	healthCode, err := strconv.Atoi(*healthText)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not read health code")
		return 1
	}
	body, err := readText(*bodyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not read health body: %s\n", err.Error())
		return 1
	}
	status, detail := classifyUptime(rootCode, healthCode, clip(body, 4000))
	fmt.Fprintln(os.Stdout, detail)
	if status == "ok" {
		return 0
	}
	return 2
}

func baseName(path string) string {
	slash := strings.LastIndex(path, "/")
	if slash < 0 {
		return path
	}
	return path[slash+1:]
}
