package docaudit

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

type DocEndpoint struct {
	Raw      string
	Path     string
	Line     int
	IsPrefix bool // wildcard match (e.g. /v1/kitchen/)
	Source   string
}

type AuditResult struct {
	DocEndpoints        []DocEndpoint
	ImplementedPaths    []string
	MissingDocEndpoints []DocEndpoint
}

var (
	routerLineRe  = regexp.MustCompile(`@Router\s+([^\s]+)\s+\[[^\]]+\]`)
	paramBracesRe = regexp.MustCompile(`\{[^}]+\}`)
	paramColonRe  = regexp.MustCompile(`:[A-Za-z0-9_]+`)
	groupRe       = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*:=\s*([A-Za-z_][A-Za-z0-9_]*)\.Group\("([^"]*)"\)`)
	routeRe       = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\("([^"]+)"`)
)

// AuditDocEndpoints compares endpoints referenced in docPath against implemented
// endpoints found via swagger-style @Router annotations under apiDir plus routes
// registered in api/server.go. It is intentionally strict: any doc endpoint not
// found in code is flagged.
func AuditDocEndpoints(docPath, apiDir string) (*AuditResult, error) {
	docEndpoints, err := ExtractDocEndpoints(docPath)
	if err != nil {
		return nil, err
	}

	implemented, err := ExtractImplementedRouterPaths(apiDir)
	if err != nil {
		return nil, err
	}
	serverRoutes, err := ExtractServerRegisteredPaths(filepath.Join(apiDir, "server.go"))
	if err != nil {
		return nil, err
	}
	implemented = mergeUniqueSorted(implemented, serverRoutes)

	implementedSet := make(map[string]struct{}, len(implemented))
	for _, p := range implemented {
		implementedSet[NormalizePath(p)] = struct{}{}
	}

	var missing []DocEndpoint
	for _, ep := range docEndpoints {
		if ep.IsPrefix {
			if !anyHasPrefix(implementedSet, ep.Path) {
				missing = append(missing, ep)
			}
			continue
		}
		if _, ok := implementedSet[ep.Path]; !ok {
			missing = append(missing, ep)
		}
	}

	return &AuditResult{
		DocEndpoints:        docEndpoints,
		ImplementedPaths:    implemented,
		MissingDocEndpoints: missing,
	}, nil
}

func mergeUniqueSorted(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	var out []string
	for _, s := range a {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, s := range b {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func anyHasPrefix(implementedSet map[string]struct{}, prefix string) bool {
	for p := range implementedSet {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// ExtractImplementedRouterPaths walks apiDir and extracts all @Router paths.
func ExtractImplementedRouterPaths(apiDir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(apiDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		scanner := bufio.NewScanner(bytes.NewReader(b))
		for scanner.Scan() {
			line := scanner.Text()
			m := routerLineRe.FindStringSubmatch(line)
			if len(m) != 2 {
				continue
			}
			paths = append(paths, m[1])
		}
		return scanner.Err()
	})
	if err != nil {
		return nil, err
	}

	// de-dup + stable sort
	uniq := make(map[string]struct{}, len(paths))
	var out []string
	for _, p := range paths {
		if _, ok := uniq[p]; ok {
			continue
		}
		uniq[p] = struct{}{}
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

// ExtractDocEndpoints extracts /v1 endpoints from markdown docPath.
// It supports patterns like `/v1/a/b\|c` (escaped pipes) and wildcards like `/v1/kitchen/*`.
func ExtractDocEndpoints(docPath string) ([]DocEndpoint, error) {
	b, err := os.ReadFile(docPath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(b), "\n")
	seen := make(map[string]DocEndpoint)

	for i, line := range lines {
		lineNo := i + 1
		for _, raw := range findRawEndpointsInLine(line) {
			for _, expanded := range expandAlternatives(raw) {
				ep := makeDocEndpoint(expanded, lineNo)
				key := fmt.Sprintf("%s|%t", ep.Path, ep.IsPrefix)
				if _, ok := seen[key]; !ok {
					seen[key] = ep
				}
			}
		}
	}

	var out []DocEndpoint
	for _, ep := range seen {
		out = append(out, ep)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Line < out[j].Line
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

func findRawEndpointsInLine(line string) []string {
	var out []string
	idx := 0
	for {
		pos := strings.Index(line[idx:], "/v1/")
		if pos < 0 {
			break
		}
		start := idx + pos
		end := start
		for end < len(line) {
			r, size := nextRune(line, end)
			if isEndpointTerminator(r) {
				break
			}
			end += size
		}
		raw := strings.TrimSpace(line[start:end])
		raw = strings.TrimRight(raw, ".,;:。；，)")
		raw = strings.TrimRight(raw, "）")
		if raw != "" {
			out = append(out, raw)
		}
		idx = end
	}
	return out
}

func nextRune(s string, i int) (rune, int) {
	r := s[i]
	if r < 0x80 {
		return rune(r), 1
	}
	decoded, size := utf8.DecodeRuneInString(s[i:])
	if decoded == utf8.RuneError && size == 1 {
		// invalid encoding, treat as single byte to avoid infinite loop
		return rune(r), 1
	}
	return decoded, size
}

func isEndpointTerminator(r rune) bool {
	switch r {
	case ' ', '\t', '\r', '\n', '`', '"', '\'', '(', ')', '[', ']', '<', '>', ',', '.', ';', '，', '。', '；', '）':
		return true
	default:
		return false
	}
}

func expandAlternatives(raw string) []string {
	// Normalize markdown-escaped table pipes "\|" into "|" so we can split.
	s := strings.ReplaceAll(raw, "\\|", "|")
	parts := splitPipesOutsideBraces(s)
	baseDir := ""
	if len(parts) > 0 {
		p0 := strings.TrimSpace(parts[0])
		if strings.HasPrefix(p0, "/v1/") {
			if idx := strings.LastIndex(p0, "/"); idx >= 0 {
				baseDir = p0[:idx+1]
			}
		}
	}
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !strings.HasPrefix(p, "/v1/") && !strings.HasPrefix(p, "/") && baseDir != "" {
			p = baseDir + p
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return []string{raw}
	}
	return out
}

func splitPipesOutsideBraces(s string) []string {
	var parts []string
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case '|':
			if depth == 0 {
				parts = append(parts, b.String())
				b.Reset()
				continue
			}
		}
		b.WriteRune(r)
	}
	parts = append(parts, b.String())
	return parts
}

// ExtractServerRegisteredPaths extracts concrete registered routes from api/server.go.
// It reconstructs group prefixes in a best-effort way (sufficient for /v1/* routes).
func ExtractServerRegisteredPaths(serverFile string) ([]string, error) {
	b, err := os.ReadFile(serverFile)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(b), "\n")
	prefix := map[string]string{
		"router": "",
	}

	// first pass: build group prefixes top-down
	for _, line := range lines {
		m := groupRe.FindStringSubmatch(line)
		if len(m) != 4 {
			continue
		}
		child, parent, groupPath := m[1], m[2], m[3]
		parentPrefix, ok := prefix[parent]
		if !ok {
			continue
		}
		prefix[child] = joinPaths(parentPrefix, groupPath)
	}

	var routes []string
	for _, line := range lines {
		m := routeRe.FindStringSubmatch(line)
		if len(m) != 4 {
			continue
		}
		groupVar, relPath := m[1], m[3]
		groupPrefix, ok := prefix[groupVar]
		if !ok {
			continue
		}
		routes = append(routes, joinPaths(groupPrefix, relPath))
	}

	return mergeUniqueSorted(nil, routes), nil
}

func joinPaths(a, b string) string {
	if a == "" {
		return NormalizePath(b)
	}
	if b == "" {
		return NormalizePath(a)
	}
	aHas := strings.HasSuffix(a, "/")
	bHas := strings.HasPrefix(b, "/")
	switch {
	case aHas && bHas:
		return NormalizePath(a + b[1:])
	case !aHas && !bHas:
		return NormalizePath(a + "/" + b)
	default:
		return NormalizePath(a + b)
	}
}

func makeDocEndpoint(raw string, line int) DocEndpoint {
	n := NormalizePath(raw)
	ep := DocEndpoint{
		Raw:    raw,
		Path:   n,
		Line:   line,
		Source: "doc",
	}
	if strings.Contains(raw, "*") {
		ep.IsPrefix = true
		// prefix rule: everything up to first '*'
		prefix := raw[:strings.Index(raw, "*")]
		ep.Path = NormalizePath(prefix)
	}
	return ep
}

// NormalizePath normalizes a path for comparison:
// - drops query strings
// - converts swagger `{id}` to gin-style `:param`
// - converts `:id` to `:param`
// - trims trailing slash (except root)
func NormalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	if q := strings.IndexByte(p, '?'); q >= 0 {
		p = p[:q]
	}
	p = paramBracesRe.ReplaceAllString(p, ":param")
	p = paramColonRe.ReplaceAllString(p, ":param")
	p = strings.ReplaceAll(p, "//", "/")
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	return p
}
