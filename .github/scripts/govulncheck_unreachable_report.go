package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type streamObject struct {
	OSV     *osvEntry `json:"osv"`
	Finding *finding  `json:"finding"`
}

type osvEntry struct {
	ID       string        `json:"id"`
	Summary  string        `json:"summary"`
	Details  string        `json:"details"`
	Affected []osvAffected `json:"affected"`
}

type osvAffected struct {
	Package *osvPackage `json:"package"`
}

type osvPackage struct {
	Name string `json:"name"`
}

type finding struct {
	OSV          string       `json:"osv"`
	FixedVersion string       `json:"fixed_version"`
	Trace        []traceFrame `json:"trace"`
}

type traceFrame struct {
	Module   string `json:"module"`
	Version  string `json:"version"`
	Package  string `json:"package"`
	Function string `json:"function"`
	Receiver string `json:"receiver"`
}

type moduleFinding struct {
	OSV             string
	Module          string
	Version         string
	FixedVersion    string
	AffectedPackage string
	Summary         string
	Registered      bool
}

func main() {
	var inputPath string
	var registerPath string
	var reportPath string
	var templatePath string
	var scanCommand string
	var scanDate string
	var failOnUnregistered bool

	flag.StringVar(&inputPath, "input", "", "Path to govulncheck JSON stream. Reads stdin when empty.")
	flag.StringVar(&registerPath, "register", "", "Path to unreachable dependency register for presence checks.")
	flag.StringVar(&reportPath, "report", "", "Path to write the markdown report.")
	flag.StringVar(&templatePath, "template", "", "Path to write the register entry template.")
	flag.StringVar(&scanCommand, "scan-command", `govulncheck -json ./...`, "Command used for the scan.")
	flag.StringVar(&scanDate, "scan-date", time.Now().UTC().Format("2006-01-02"), "UTC scan date in YYYY-MM-DD format.")
	flag.BoolVar(&failOnUnregistered, "fail-on-unregistered", false, "Exit non-zero when module-only findings are missing from the register.")
	flag.Parse()

	input, closeInput, err := openInput(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open input: %v\n", err)
		os.Exit(1)
	}
	defer closeInput()

	registerContent, err := readOptionalFile(registerPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read register: %v\n", err)
		os.Exit(1)
	}

	entries, reachableCount, err := collectModuleFindings(input, registerContent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse govulncheck stream: %v\n", err)
		os.Exit(1)
	}

	report := buildReport(entries, reachableCount, scanCommand, scanDate)
	template := buildTemplate(entries, scanCommand, scanDate)

	if reportPath != "" {
		if err := writeFile(reportPath, report); err != nil {
			fmt.Fprintf(os.Stderr, "write report: %v\n", err)
			os.Exit(1)
		}
	}

	if templatePath != "" {
		if err := writeFile(templatePath, template); err != nil {
			fmt.Fprintf(os.Stderr, "write template: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Print(report)

	if failOnUnregistered {
		missing := unregisteredFindings(entries)
		if len(missing) > 0 {
			fmt.Fprintf(os.Stderr, "unregistered module-only findings detected: %s\n", strings.Join(missing, ", "))
			os.Exit(2)
		}
	}
}

func openInput(path string) (io.Reader, func(), error) {
	if path == "" {
		return bufio.NewReader(os.Stdin), func() {}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}

	return bufio.NewReader(file), func() { _ = file.Close() }, nil
}

func readOptionalFile(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	return os.ReadFile(path)
}

func collectModuleFindings(input io.Reader, registerContent []byte) ([]moduleFinding, int, error) {
	decoder := json.NewDecoder(input)
	osvByID := map[string]osvEntry{}
	moduleOnlyByKey := map[string]moduleFinding{}
	registeredIDs := parseRegisteredOSVIDs(registerContent)
	reachableCount := 0

	for {
		var object streamObject
		if err := decoder.Decode(&object); err != nil {
			if err == io.EOF {
				break
			}
			return nil, 0, err
		}

		if object.OSV != nil && object.OSV.ID != "" {
			osvByID[object.OSV.ID] = *object.OSV
		}

		if object.Finding == nil {
			continue
		}

		if !isModuleOnlyFinding(*object.Finding) {
			reachableCount++
			continue
		}

		frame := object.Finding.Trace[0]
		osv := osvByID[object.Finding.OSV]
		entry := moduleFinding{
			OSV:             object.Finding.OSV,
			Module:          frame.Module,
			Version:         frame.Version,
			FixedVersion:    object.Finding.FixedVersion,
			AffectedPackage: detectAffectedPackage(osv, frame.Module),
			Summary:         summarizeOSV(osv),
			Registered:      registeredIDs[object.Finding.OSV],
		}
		key := entry.OSV + "|" + entry.Module + "|" + entry.Version + "|" + entry.FixedVersion
		moduleOnlyByKey[key] = entry
	}

	entries := make([]moduleFinding, 0, len(moduleOnlyByKey))
	for _, entry := range moduleOnlyByKey {
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].OSV == entries[j].OSV {
			return entries[i].Module < entries[j].Module
		}
		return entries[i].OSV < entries[j].OSV
	})

	return entries, reachableCount, nil
}

var registerOSVIDPattern = regexp.MustCompile(`(?m)^-\s*(?:漏洞 ID|Vulnerability ID)：?\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*$`)

func parseRegisteredOSVIDs(registerContent []byte) map[string]bool {
	registered := map[string]bool{}
	for _, match := range registerOSVIDPattern.FindAllSubmatch(registerContent, -1) {
		if len(match) < 2 {
			continue
		}
		osvID := strings.TrimSpace(string(match[1]))
		if osvID != "" {
			registered[osvID] = true
		}
	}
	return registered
}

func isModuleOnlyFinding(f finding) bool {
	if len(f.Trace) == 0 {
		return false
	}

	for _, frame := range f.Trace {
		if frame.Package != "" || frame.Function != "" || frame.Receiver != "" {
			return false
		}
	}

	return f.Trace[0].Module != ""
}

func detectAffectedPackage(entry osvEntry, modulePath string) string {
	selected := ""
	for _, affected := range entry.Affected {
		if affected.Package != nil && affected.Package.Name != "" {
			if len(affected.Package.Name) > len(selected) {
				selected = affected.Package.Name
			}
		}
	}

	if selected != "" && selected != modulePath {
		return selected
	}

	if modulePath == "" {
		return selected
	}

	for _, candidate := range strings.Fields(strings.Join([]string{entry.Summary, entry.Details}, " ")) {
		trimmed := strings.Trim(candidate, "`.,;:!?()[]{}<>\"'")
		if strings.HasPrefix(trimmed, modulePath+"/") && len(trimmed) > len(selected) {
			selected = trimmed
		}
	}

	return selected
}

func summarizeOSV(entry osvEntry) string {
	if entry.Summary != "" {
		return strings.TrimSpace(entry.Summary)
	}
	if entry.Details == "" {
		return ""
	}
	first := strings.Split(strings.TrimSpace(entry.Details), "\n")[0]
	return strings.TrimSpace(first)
}

func buildReport(entries []moduleFinding, reachableCount int, scanCommand, scanDate string) string {
	var builder strings.Builder
	missing := unregisteredFindings(entries)

	builder.WriteString("# Govulncheck Module-Only Risk Report\n\n")
	builder.WriteString(fmt.Sprintf("- Scan date: `%s`\n", scanDate))
	builder.WriteString(fmt.Sprintf("- Scan command: `%s`\n", scanCommand))
	builder.WriteString(fmt.Sprintf("- Reachable findings: `%d`\n", reachableCount))
	builder.WriteString(fmt.Sprintf("- Module-only findings: `%d`\n\n", len(entries)))

	if len(entries) == 0 {
		builder.WriteString("No module-only findings were reported in this scan.\n")
		return builder.String()
	}

	if len(missing) == 0 {
		builder.WriteString("- Register coverage: `complete`\n\n")
	} else {
		builder.WriteString("- Register coverage: `incomplete`\n")
		builder.WriteString(fmt.Sprintf("- Missing register entries: `%s`\n\n", strings.Join(missing, ", ")))
	}

	builder.WriteString("Use the companion template artifact to update the unreachable dependency register when a finding is new or changed.\n\n")

	for _, entry := range entries {
		builder.WriteString(fmt.Sprintf("## %s\n\n", entry.OSV))
		builder.WriteString(fmt.Sprintf("- Module: `%s`\n", entry.Module))
		builder.WriteString(fmt.Sprintf("- Current version: `%s`\n", fallback(entry.Version, "unknown")))
		builder.WriteString(fmt.Sprintf("- Fixed version: `%s`\n", fallback(entry.FixedVersion, "unknown")))
		if entry.AffectedPackage != "" {
			builder.WriteString(fmt.Sprintf("- Affected package: `%s`\n", entry.AffectedPackage))
		}
		if entry.Summary != "" {
			builder.WriteString(fmt.Sprintf("- Summary: %s\n", entry.Summary))
		}
		status := "not found in register"
		if entry.Registered {
			status = "already present in register"
		}
		builder.WriteString(fmt.Sprintf("- Register status: `%s`\n\n", status))
	}

	return builder.String()
}

func unregisteredFindings(entries []moduleFinding) []string {
	missing := make([]string, 0)
	for _, entry := range entries {
		if !entry.Registered {
			missing = append(missing, entry.OSV)
		}
	}
	return missing
}

func buildTemplate(entries []moduleFinding, scanCommand, scanDate string) string {
	var builder strings.Builder
	builder.WriteString("# Unreachable Dependency Register Template\n\n")
	builder.WriteString("Copy the relevant entry into `.github/standards/engineering/UNREACHABLE_DEPENDENCY_RISK_REGISTER.md` when the finding is new or materially changed.\n\n")

	if len(entries) == 0 {
		builder.WriteString("No module-only findings were reported in this scan.\n")
		return builder.String()
	}

	for index, entry := range entries {
		builder.WriteString(fmt.Sprintf("## Entry %s\n\n", entry.OSV))
		builder.WriteString(fmt.Sprintf("### Entry %s-%02d\n\n", scanDate, index+1))
		builder.WriteString(fmt.Sprintf("- 漏洞 ID：`%s`\n", entry.OSV))
		builder.WriteString(fmt.Sprintf("- 模块：`%s`\n", entry.Module))
		builder.WriteString(fmt.Sprintf("- 当前版本：`%s`\n", fallback(entry.Version, "待补充")))
		builder.WriteString(fmt.Sprintf("- 修复版本：`%s`\n", fallback(entry.FixedVersion, "待补充")))
		builder.WriteString(fmt.Sprintf("- 扫描命令：`%s`\n", scanCommand))
		builder.WriteString(fmt.Sprintf("- 扫描日期：`%s`\n", scanDate))
		builder.WriteString("- 扫描结果：`0 reachable vulnerabilities; module-only finding remains.`\n")
		builder.WriteString("- 当前判定：`不可达模块风险`\n")
		builder.WriteString(fmt.Sprintf("- 判定依据：`govulncheck` JSON finding 仅包含 module/version trace，未出现 package 或 symbol reachable trace。%s\n", renderAffectedPackageEvidence(entry.AffectedPackage)))
		builder.WriteString("- 暴露触发条件：\n")
		builder.WriteString(fmt.Sprintf("  - 新增或引入对 `%s` 的实际调用、解析或处理路径。\n", fallback(entry.AffectedPackage, entry.Module)))
		builder.WriteString("  - 新增不可信输入链路把该依赖接入当前服务。\n")
		builder.WriteString("  - 后续扫描结果从 module-only 升级为 package 或 symbol reachable。\n")
		builder.WriteString("- 计划动作：\n")
		builder.WriteString(fmt.Sprintf("  - 在下一次依赖收敛时优先升级到 `%s@%s` 或更高版本。\n", entry.Module, fallback(entry.FixedVersion, "待补充")))
		builder.WriteString("  - 每次 security-baseline 复跑后确认该条目是否仍为 module-only result。\n")
		builder.WriteString("- 状态：`open-monitored`\n\n")
	}

	return builder.String()
}

func renderAffectedPackageEvidence(name string) string {
	if name == "" {
		return ""
	}
	return fmt.Sprintf("受影响包为 `%s`。", name)
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}