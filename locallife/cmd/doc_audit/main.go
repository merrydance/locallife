package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/merrydance/locallife/internal/docaudit"
)

func main() {
	docPath := flag.String("doc", "docs/phase0/user_journey_mermaid.md", "Path to markdown doc")
	apiDir := flag.String("api", "api", "API handlers directory to scan for @Router annotations")
	reportPath := flag.String("report", "", "Optional markdown report output path")
	failOnMissing := flag.Bool("fail-on-missing", true, "Exit non-zero if missing endpoints are found")
	flag.Parse()

	result, err := docaudit.AuditDocEndpoints(*docPath, *apiDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "doc audit failed:", err)
		os.Exit(2)
	}

	missing := result.MissingDocEndpoints
	if *reportPath != "" {
		if err := writeReport(*reportPath, *docPath, *apiDir, result); err != nil {
			fmt.Fprintln(os.Stderr, "write report failed:", err)
			os.Exit(2)
		}
	}

	if len(missing) == 0 {
		if len(result.SuspiciousHandlers) == 0 {
			s := result.HandlerAuditStats
			fmt.Printf("OK: %d doc endpoints all implemented (scanned %d implemented paths via @Router + api/server.go); handler-audit analyzed=%d matched_routes=%d resolved_handlers=%d skipped(no_name=%d,no_def=%d)\n",
				len(result.DocEndpoints),
				len(result.ImplementedPaths),
				s.AnalyzedHandlers,
				s.MatchedRouteRegistrations,
				s.ResolvedHandlerDefs,
				s.SkippedNoHandlerName,
				s.SkippedNoHandlerDef,
			)
		} else {
			fmt.Printf("OK: %d doc endpoints all implemented (scanned %d implemented paths via @Router + api/server.go); WARN: %d suspicious handlers\n", len(result.DocEndpoints), len(result.ImplementedPaths), len(result.SuspiciousHandlers))
		}
		return
	}

	fmt.Fprintf(os.Stderr, "MISSING: %d endpoints referenced in doc but not found in code (@Router + api/server.go)\n", len(missing))
	for _, m := range missing {
		fmt.Fprintf(os.Stderr, "- %s (line %d, raw: %s)\n", m.Path, m.Line, m.Raw)
	}

	if *failOnMissing {
		os.Exit(1)
	}
}

func writeReport(path, docPath, apiDir string, result *docaudit.AuditResult) error {
	var b strings.Builder
	b.WriteString("# 文档端点覆盖审计报告\n\n")
	b.WriteString(fmt.Sprintf("- doc: `%s`\n", docPath))
	b.WriteString(fmt.Sprintf("- api: `%s`（扫描 @Router 注解 + api/server.go 路由注册）\n", apiDir))
	b.WriteString(fmt.Sprintf("- doc endpoints: %d\n", len(result.DocEndpoints)))
	b.WriteString(fmt.Sprintf("- implemented paths: %d\n\n", len(result.ImplementedPaths)))

	if len(result.MissingDocEndpoints) == 0 {
		b.WriteString("## 结论\n\n- ✅ 文档引用的所有 `/v1/...` 端点在代码中都能找到对应实现（`@Router` 注解或 `api/server.go` 路由注册）。\n")
	} else {
		b.WriteString("## 缺失项（文档有，代码未找到）\n\n")
		missing := append([]docaudit.DocEndpoint(nil), result.MissingDocEndpoints...)
		sort.Slice(missing, func(i, j int) bool {
			if missing[i].Path == missing[j].Path {
				return missing[i].Line < missing[j].Line
			}
			return missing[i].Path < missing[j].Path
		})
		for _, m := range missing {
			b.WriteString(fmt.Sprintf("- `%s`（line %d，raw: `%s`）\n", m.Path, m.Line, m.Raw))
		}
	}

	b.WriteString("\n## 备注\n\n")
	b.WriteString("- 这是**文档 → 代码**的单向约束：确保文档里提到的端点不会‘空跑’。\n")
	b.WriteString("- 若需要反向约束（代码新增端点必须补文档），可以在后续版本加白名单/标签体系。\n")

	// Always include handler-audit coverage so WARN=0 is interpretable.
	s := result.HandlerAuditStats
	b.WriteString("\n## Handler 静态审计覆盖度\n\n")
	b.WriteString(fmt.Sprintf("- doc endpoints: %d\n", s.DocEndpoints))
	b.WriteString(fmt.Sprintf("- doc actions (METHOD + /v1/... occurrences): %d\n", s.DocActions))
	b.WriteString(fmt.Sprintf("- matched route registrations: %d\n", s.MatchedRouteRegistrations))
	b.WriteString(fmt.Sprintf("- resolved handler names: %d\n", s.ResolvedHandlerNames))
	b.WriteString(fmt.Sprintf("- resolved handler defs (*Server methods): %d\n", s.ResolvedHandlerDefs))
	b.WriteString(fmt.Sprintf("- analyzed handlers: %d\n", s.AnalyzedHandlers))
	b.WriteString(fmt.Sprintf("- suspicious findings: %d\n", s.SuspiciousFindings))
	b.WriteString(fmt.Sprintf("- skipped: no handler name=%d, no handler def=%d\n", s.SkippedNoHandlerName, s.SkippedNoHandlerDef))

	if len(result.SuspiciousHandlers) > 0 {
		b.WriteString("\n## 可疑空跑实现（需要人工确认）\n\n")
		b.WriteString("以下项是静态分析的 **WARN**：文档里出现了 `METHOD /v1/...`，能映射到 `api/server.go` 的 handler，但 handler 里没检测到明显的响应写入（`ctx.JSON/Status/Abort...`）也没把 `ctx` 传给下游函数。它不一定错误，但很可能是 stub/空跑或缺少返回。\n\n")
		for _, s := range result.SuspiciousHandlers {
			b.WriteString(fmt.Sprintf("- `%s %s` → `%s`（%s:%d） reason=%s doc_line=%d raw=`%s`\n", s.Method, s.Path, s.Handler, s.File, s.Line, s.Reason, s.DocLine, s.DocRaw))
		}
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}
