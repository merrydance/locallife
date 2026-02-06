package docaudit

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
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
	SuspiciousHandlers  []SuspiciousHandler
	HandlerAuditStats   HandlerAuditStats
}

type HandlerAuditStats struct {
	DocEndpoints              int
	DocActions                int
	MatchedRouteRegistrations int
	ResolvedHandlerNames      int
	ResolvedHandlerDefs       int
	AnalyzedHandlers          int
	SuspiciousFindings        int
	SkippedNoHandlerName      int
	SkippedNoHandlerDef       int
}

type DocAction struct {
	Raw      string
	Method   string
	Path     string
	Line     int
	IsPrefix bool
}

type RouteRegistration struct {
	Method  string
	Path    string // normalized, full path (incl group prefix)
	Handler string // best-effort handler name (e.g. getUserMe)
}

type SuspiciousHandler struct {
	Method   string
	Path     string
	Handler  string
	File     string
	Line     int
	Reason   string
	DocLine  int
	DocRaw   string
	Resolved bool
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
	docActions, err := ExtractDocActions(docPath)
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

	routesWithHandlers, err := ExtractServerRegisteredRoutesWithHandlers(filepath.Join(apiDir, "server.go"))
	if err != nil {
		return nil, err
	}
	handlersIndex, err := IndexAPIHandlers(apiDir)
	if err != nil {
		return nil, err
	}
	suspiciousActions := AuditDocActionsHandlers(docActions, routesWithHandlers, handlersIndex)
	suspiciousEndpoints, stats := AuditDocEndpointsHandlersWithStats(docEndpoints, docActions, routesWithHandlers, handlersIndex)
	suspicious := mergeSuspicious(suspiciousActions, suspiciousEndpoints)

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
		SuspiciousHandlers:  suspicious,
		HandlerAuditStats:   stats,
	}, nil
}

func mergeSuspicious(a, b []SuspiciousHandler) []SuspiciousHandler {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	seen := make(map[string]struct{}, len(a)+len(b))
	var out []SuspiciousHandler
	for _, s := range a {
		key := s.Method + " " + s.Path + "|" + s.Handler + "|" + s.Reason
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	for _, s := range b {
		key := s.Method + " " + s.Path + "|" + s.Handler + "|" + s.Reason
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			if out[i].Method == out[j].Method {
				return out[i].Reason < out[j].Reason
			}
			return out[i].Method < out[j].Method
		}
		return out[i].Path < out[j].Path
	})
	return out
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

var methodPathRe = regexp.MustCompile("\\b(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\\s+(/v1/[^\\s`\\\"']+)")

// ExtractDocActions extracts METHOD + /v1/path occurrences from markdown docPath.
// This is used to map doc steps to concrete handlers and detect "empty run" stubs.
func ExtractDocActions(docPath string) ([]DocAction, error) {
	b, err := os.ReadFile(docPath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(b), "\n")
	seen := make(map[string]DocAction)

	for i, line := range lines {
		lineNo := i + 1
		matches := methodPathRe.FindAllStringSubmatchIndex(line, -1)
		for _, idx := range matches {
			if len(idx) < 6 {
				continue
			}
			raw := strings.TrimSpace(line[idx[0]:idx[1]])
			method := strings.ToUpper(line[idx[2]:idx[3]])
			path := line[idx[4]:idx[5]]
			path = strings.TrimRight(path, ".,;:。；，)")
			path = strings.TrimRight(path, "）")
			act := DocAction{Raw: raw, Method: method, Path: NormalizePath(path), Line: lineNo}
			if strings.Contains(path, "*") {
				act.IsPrefix = true
				prefix := path[:strings.Index(path, "*")]
				act.Path = NormalizePath(prefix)
			}
			key := fmt.Sprintf("%s %s|%t", act.Method, act.Path, act.IsPrefix)
			if _, ok := seen[key]; !ok {
				seen[key] = act
			}
		}
	}

	var out []DocAction
	for _, a := range seen {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Method == out[j].Method {
			if out[i].Path == out[j].Path {
				return out[i].Line < out[j].Line
			}
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
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

// ExtractServerRegisteredRoutesWithHandlers extracts registered routes including handler names.
// Best-effort: only resolves handler names when the last handler is a selector (e.g. server.getUserMe).
func ExtractServerRegisteredRoutesWithHandlers(serverFile string) ([]RouteRegistration, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, serverFile, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	groupPrefix := map[string]string{"router": ""}

	ast.Inspect(f, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			return true
		}
		lhs, ok := as.Lhs[0].(*ast.Ident)
		if !ok {
			return true
		}
		call, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil || sel.Sel.Name != "Group" {
			return true
		}
		parentIdent, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if len(call.Args) < 1 {
			return true
		}
		gp, ok := call.Args[0].(*ast.BasicLit)
		if !ok || gp.Kind != token.STRING {
			return true
		}
		parentPrefix, ok := groupPrefix[parentIdent.Name]
		if !ok {
			return true
		}
		groupPath := strings.Trim(gp.Value, "\"")
		groupPrefix[lhs.Name] = joinPaths(parentPrefix, groupPath)
		return true
	})

	var routes []RouteRegistration
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil {
			return true
		}
		method := strings.ToUpper(sel.Sel.Name)
		switch method {
		case "GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD":
			// ok
		default:
			return true
		}
		groupIdent, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		prefix, ok := groupPrefix[groupIdent.Name]
		if !ok {
			return true
		}
		if len(call.Args) < 2 {
			return true
		}
		p0, ok := call.Args[0].(*ast.BasicLit)
		if !ok || p0.Kind != token.STRING {
			return true
		}
		rel := strings.Trim(p0.Value, "\"")
		full := NormalizePath(joinPaths(prefix, rel))

		hName := ""
		last := call.Args[len(call.Args)-1]
		switch v := last.(type) {
		case *ast.SelectorExpr:
			if v.Sel != nil {
				hName = v.Sel.Name
			}
		case *ast.Ident:
			hName = v.Name
		default:
			// leave empty
		}

		routes = append(routes, RouteRegistration{Method: method, Path: full, Handler: hName})
		return true
	})

	// de-dup
	seen := make(map[string]struct{}, len(routes))
	var out []RouteRegistration
	for _, r := range routes {
		key := r.Method + " " + r.Path
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Method == out[j].Method {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out, nil
}

type HandlerDef struct {
	Name         string
	File         string
	Line         int
	CtxParamName string
	Func         *ast.FuncDecl
}

// IndexAPIHandlers builds a best-effort index of Server methods (handlers) under apiDir.
func IndexAPIHandlers(apiDir string) (map[string]*HandlerDef, error) {
	index := make(map[string]*HandlerDef)
	err := filepath.WalkDir(apiDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		fset := token.NewFileSet()
		f, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Name == nil {
				continue
			}
			// only methods on *Server
			if len(fd.Recv.List) != 1 {
				continue
			}
			star, ok := fd.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			ident, ok := star.X.(*ast.Ident)
			if !ok || ident.Name != "Server" {
				continue
			}

			ctxName := ""
			if fd.Type != nil && fd.Type.Params != nil && len(fd.Type.Params.List) > 0 {
				if len(fd.Type.Params.List[0].Names) > 0 {
					ctxName = fd.Type.Params.List[0].Names[0].Name
				}
			}
			pos := fset.Position(fd.Pos())
			index[fd.Name.Name] = &HandlerDef{
				Name:         fd.Name.Name,
				File:         path,
				Line:         pos.Line,
				CtxParamName: ctxName,
				Func:         fd,
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return index, nil
}

func AuditDocActionsHandlers(actions []DocAction, routes []RouteRegistration, handlers map[string]*HandlerDef) []SuspiciousHandler {
	byKey := make(map[string]RouteRegistration, len(routes))
	for _, r := range routes {
		byKey[r.Method+" "+NormalizePath(r.Path)] = r
	}

	writeMethods := map[string]struct{}{
		"JSON": {}, "Status": {}, "String": {}, "Data": {}, "Redirect": {},
		"Abort": {}, "AbortWithStatus": {}, "AbortWithStatusJSON": {},
		"Render": {}, "HTML": {}, "XML": {}, "YAML": {}, "ProtoBuf": {},
		"File": {}, "FileAttachment": {},
	}

	var suspicious []SuspiciousHandler
	for _, a := range actions {
		if a.Method == "" || a.Path == "" {
			continue
		}

		var reg RouteRegistration
		found := false
		if !a.IsPrefix {
			reg, found = byKey[a.Method+" "+a.Path]
		} else {
			// prefix match within same method
			for k, r := range byKey {
				if !strings.HasPrefix(k, a.Method+" ") {
					continue
				}
				if strings.HasPrefix(r.Path, a.Path) {
					reg = r
					found = true
					break
				}
			}
		}
		if !found {
			continue
		}
		if reg.Handler == "" {
			// can't resolve handler; skip (not enough signal)
			continue
		}
		hd, ok := handlers[reg.Handler]
		if !ok || hd.Func == nil {
			continue
		}
		if hd.Func.Body == nil || len(hd.Func.Body.List) == 0 {
			suspicious = append(suspicious, SuspiciousHandler{
				Method:   a.Method,
				Path:     a.Path,
				Handler:  reg.Handler,
				File:     hd.File,
				Line:     hd.Line,
				Reason:   "empty handler body",
				DocLine:  a.Line,
				DocRaw:   a.Raw,
				Resolved: true,
			})
			continue
		}

		ctxName := hd.CtxParamName
		wroteResponse := false
		delegatedCtx := false
		ast.Inspect(hd.Func.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			// ctx.WriteMethod(...)
			if ctxName != "" {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if x, ok := sel.X.(*ast.Ident); ok && x.Name == ctxName {
						if _, ok := writeMethods[sel.Sel.Name]; ok {
							wroteResponse = true
							return false
						}
					}
				}
				for _, arg := range call.Args {
					if id, ok := arg.(*ast.Ident); ok && id.Name == ctxName {
						delegatedCtx = true
						break
					}
				}
			}
			return true
		})

		if wroteResponse || delegatedCtx {
			continue
		}

		// mark suspicious: no obvious response write and no ctx delegation
		loc := SuspiciousHandler{
			Method:   a.Method,
			Path:     a.Path,
			Handler:  reg.Handler,
			File:     hd.File,
			Line:     hd.Line,
			Reason:   "no ctx response write (JSON/Status/Abort/...) and no ctx delegation detected",
			DocLine:  a.Line,
			DocRaw:   a.Raw,
			Resolved: true,
		}
		// include a tiny best-effort handler snippet for debugging when needed (kept internal)
		suspicious = append(suspicious, loc)
	}

	sort.Slice(suspicious, func(i, j int) bool {
		if suspicious[i].Path == suspicious[j].Path {
			return suspicious[i].Method < suspicious[j].Method
		}
		return suspicious[i].Path < suspicious[j].Path
	})
	return suspicious
}

func AuditDocEndpointsHandlersWithStats(endpoints []DocEndpoint, actions []DocAction, routes []RouteRegistration, handlers map[string]*HandlerDef) ([]SuspiciousHandler, HandlerAuditStats) {
	// index routes by normalized path
	byPath := make(map[string][]RouteRegistration, len(routes))
	for _, r := range routes {
		byPath[NormalizePath(r.Path)] = append(byPath[NormalizePath(r.Path)], r)
	}

	writeMethods := map[string]struct{}{
		"JSON": {}, "Status": {}, "String": {}, "Data": {}, "Redirect": {},
		"Abort": {}, "AbortWithStatus": {}, "AbortWithStatusJSON": {},
		"Render": {}, "HTML": {}, "XML": {}, "YAML": {}, "ProtoBuf": {},
		"File": {}, "FileAttachment": {},
	}

	stats := HandlerAuditStats{DocEndpoints: len(endpoints), DocActions: len(actions)}
	var suspicious []SuspiciousHandler
	for _, ep := range endpoints {
		if ep.Path == "" {
			continue
		}
		if !ep.IsPrefix {
			regs := byPath[ep.Path]
			for _, reg := range regs {
				stats.MatchedRouteRegistrations++
				if reg.Handler == "" {
					stats.SkippedNoHandlerName++
					continue
				}
				stats.ResolvedHandlerNames++
				hd, ok := handlers[reg.Handler]
				if !ok || hd.Func == nil {
					stats.SkippedNoHandlerDef++
					continue
				}
				stats.ResolvedHandlerDefs++
				before := len(suspicious)
				appendIfSuspicious(&suspicious, reg, hd, ep.Line, ep.Raw, writeMethods)
				stats.AnalyzedHandlers++
				if len(suspicious) > before {
					stats.SuspiciousFindings++
				}
			}
			continue
		}

		// prefix match
		for _, reg := range routes {
			if !strings.HasPrefix(NormalizePath(reg.Path), ep.Path) {
				continue
			}
			stats.MatchedRouteRegistrations++
			if reg.Handler == "" {
				stats.SkippedNoHandlerName++
				continue
			}
			stats.ResolvedHandlerNames++
			hd, ok := handlers[reg.Handler]
			if !ok || hd.Func == nil {
				stats.SkippedNoHandlerDef++
				continue
			}
			stats.ResolvedHandlerDefs++
			before := len(suspicious)
			appendIfSuspicious(&suspicious, reg, hd, ep.Line, ep.Raw, writeMethods)
			stats.AnalyzedHandlers++
			if len(suspicious) > before {
				stats.SuspiciousFindings++
			}
		}
	}

	sort.Slice(suspicious, func(i, j int) bool {
		if suspicious[i].Path == suspicious[j].Path {
			return suspicious[i].Method < suspicious[j].Method
		}
		return suspicious[i].Path < suspicious[j].Path
	})
	return suspicious, stats
}

func appendIfSuspicious(out *[]SuspiciousHandler, reg RouteRegistration, hd *HandlerDef, docLine int, docRaw string, writeMethods map[string]struct{}) {
	if hd.Func.Body == nil || len(hd.Func.Body.List) == 0 {
		*out = append(*out, SuspiciousHandler{
			Method:   reg.Method,
			Path:     NormalizePath(reg.Path),
			Handler:  reg.Handler,
			File:     hd.File,
			Line:     hd.Line,
			Reason:   "empty handler body",
			DocLine:  docLine,
			DocRaw:   docRaw,
			Resolved: true,
		})
		return
	}

	ctxName := hd.CtxParamName
	wroteResponse := false
	delegatedCtx := false
	ast.Inspect(hd.Func.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if ctxName != "" {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == ctxName {
					if _, ok := writeMethods[sel.Sel.Name]; ok {
						wroteResponse = true
						return false
					}
				}
			}
			for _, arg := range call.Args {
				if id, ok := arg.(*ast.Ident); ok && id.Name == ctxName {
					delegatedCtx = true
					break
				}
			}
		}
		return true
	})

	if wroteResponse || delegatedCtx {
		return
	}
	*out = append(*out, SuspiciousHandler{
		Method:   reg.Method,
		Path:     NormalizePath(reg.Path),
		Handler:  reg.Handler,
		File:     hd.File,
		Line:     hd.Line,
		Reason:   "no ctx response write (JSON/Status/Abort/...) and no ctx delegation detected",
		DocLine:  docLine,
		DocRaw:   docRaw,
		Resolved: true,
	})
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
