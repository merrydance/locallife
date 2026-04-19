package wechatdoc

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

type Extraction struct {
	Source        string         `json:"source,omitempty"`
	Sections      []Section      `json:"sections"`
	UnknownTables []UnknownTable `json:"unknown_tables,omitempty"`
	Warnings      []string       `json:"warnings,omitempty"`
	Summary       Summary        `json:"summary"`
}

type Summary struct {
	SectionCount      int `json:"section_count"`
	EndpointCount     int `json:"endpoint_count"`
	FieldCount        int `json:"field_count"`
	EnumSetCount      int `json:"enum_set_count"`
	EnumValueCount    int `json:"enum_value_count"`
	ErrorCodeCount    int `json:"error_code_count"`
	UnknownTableCount int `json:"unknown_table_count"`
	WarningCount      int `json:"warning_count"`
}

type Section struct {
	Heading    string      `json:"heading"`
	Path       []string    `json:"path"`
	Endpoints  []Endpoint  `json:"endpoints,omitempty"`
	Fields     []Field     `json:"fields,omitempty"`
	EnumSets   []EnumSet   `json:"enum_sets,omitempty"`
	ErrorCodes []ErrorCode `json:"error_codes,omitempty"`
	Warnings   []string    `json:"warnings,omitempty"`
}

type Endpoint struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Line   int    `json:"line"`
}

type Field struct {
	Name        string   `json:"name"`
	Type        string   `json:"type,omitempty"`
	Requirement string   `json:"requirement,omitempty"`
	Condition   string   `json:"condition,omitempty"`
	EnumValues  []string `json:"enum_values,omitempty"`
	Description string   `json:"description,omitempty"`
	Line        int      `json:"line"`
}

type EnumSet struct {
	Name   string      `json:"name"`
	Kind   string      `json:"kind"`
	Values []EnumValue `json:"values"`
	Line   int         `json:"line"`
}

type EnumValue struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	Line        int    `json:"line"`
}

type ErrorCode struct {
	Code        string `json:"code"`
	Description string `json:"description,omitempty"`
	Resolution  string `json:"resolution,omitempty"`
	Line        int    `json:"line"`
}

type UnknownTable struct {
	SectionPath []string `json:"section_path"`
	Headers     []string `json:"headers"`
	Line        int      `json:"line"`
}

type headingEntry struct {
	Level int
	Title string
}

type markdownTable struct {
	Headers []string
	Rows    [][]string
	Line    int
	Path    []string
}

type sectionBuilder struct {
	Heading    string
	Path       []string
	Endpoints  []Endpoint
	Fields     []Field
	EnumSets   []EnumSet
	ErrorCodes []ErrorCode
	Warnings   []string

	endpointSeen map[string]struct{}
	fieldSeen    map[string]struct{}
	enumSeen     map[string]struct{}
	errorSeen    map[string]struct{}
}

type pendingMethod struct {
	Method  string
	Line    int
	PathKey string
}

type headerRoles struct {
	Name        int
	Type        int
	Required    int
	Condition   int
	Description int
	Enum        int
	Method      int
	Path        int
	Code        int
	Resolution  int
	Value       int
	HasStatus   bool
	HasError    bool
	HasMethod   bool
	HasPath     bool
	HasField    bool
	HasType     bool
	HasRequired bool
	HasValue    bool
}

var (
	inlineMethodPathRe = regexp.MustCompile(`(?i)\b(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\s+(/v[0-9]/[^\s\)` + "`" + `"']+)`)
	methodLabelRe      = regexp.MustCompile(`(?i)(请求方式|http\s*method|method)\s*[:：]\s*[【\[]?\s*(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\s*[】\]]?`)
	pathLabelRe        = regexp.MustCompile(`(?i)(请求地址|请求url|接口地址|url|path|request\s*url)\s*[:：]\s*(/v[0-9]/[^\s\)` + "`" + `"']+)`)
	headingRe          = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
	standalonePathRe   = regexp.MustCompile("^`?(/v[0-9]/[^\\s`\"]+)`?$")
)

func ExtractMarkdownFile(path string) (*Extraction, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	result := ExtractMarkdown(string(b))
	result.Source = path
	return result, nil
}

func ExtractMarkdown(markdown string) *Extraction {
	lines := strings.Split(markdown, "\n")
	builders := make(map[string]*sectionBuilder)
	sectionOrder := make([]string, 0)
	unknownTables := make([]UnknownTable, 0)
	globalWarnings := make([]string, 0)
	headings := make([]headingEntry, 0)
	pending := pendingMethod{}
	inFence := false

	for index := 0; index < len(lines); index++ {
		lineNo := index + 1
		line := lines[index]
		trimmed := strings.TrimSpace(line)

		if isFenceDelimiter(trimmed) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}

		if level, title, ok := parseHeading(trimmed); ok {
			headings = updateHeadings(headings, level, title)
			pending = pendingMethod{}
			continue
		}

		currentPath := headingsToPath(headings)
		if trimmed == "" {
			continue
		}

		if isTableHeader(lines, index) {
			table, endIndex := parseTable(lines, index, currentPath)
			if !table.isEmpty() {
				if !applyTable(builders, &sectionOrder, table, &unknownTables) {
					globalWarnings = append(globalWarnings, fmt.Sprintf("unclassified table at line %d (%s)", table.Line, strings.Join(currentPath, " > ")))
				}
			}
			index = endIndex
			continue
		}

		section := ensureSection(builders, &sectionOrder, currentPath)
		captureInlineEndpoints(section, trimmed, lineNo)

		if methodMatch := methodLabelRe.FindStringSubmatch(trimmed); len(methodMatch) == 3 {
			pending = pendingMethod{Method: strings.ToUpper(methodMatch[2]), Line: lineNo, PathKey: sectionKey(currentPath)}
		}
		if pathMatch := pathLabelRe.FindStringSubmatch(trimmed); len(pathMatch) == 3 {
			path := normalizePath(pathMatch[2])
			if pending.Method != "" && pending.PathKey == sectionKey(currentPath) && lineNo-pending.Line <= 5 {
				section.addEndpoint(Endpoint{Method: pending.Method, Path: path, Line: pending.Line})
				pending = pendingMethod{}
			} else {
				section.Warnings = append(section.Warnings, fmt.Sprintf("unpaired path label at line %d: %s", lineNo, path))
			}
		}
		if pathMatch := standalonePathRe.FindStringSubmatch(trimmed); len(pathMatch) == 2 {
			path := normalizePath(pathMatch[1])
			if pending.Method != "" && pending.PathKey == sectionKey(currentPath) && lineNo-pending.Line <= 5 {
				section.addEndpoint(Endpoint{Method: pending.Method, Path: path, Line: pending.Line})
				pending = pendingMethod{}
			}
		}
	}

	sections := make([]Section, 0, len(sectionOrder))
	summary := Summary{}
	for _, key := range sectionOrder {
		builder := builders[key]
		if builder == nil {
			continue
		}
		sections = append(sections, builder.build())
	}

	sort.Slice(sections, func(i, j int) bool {
		left := strings.Join(sections[i].Path, " > ")
		right := strings.Join(sections[j].Path, " > ")
		if left == right {
			return sections[i].Heading < sections[j].Heading
		}
		return left < right
	})

	for _, section := range sections {
		summary.SectionCount++
		summary.EndpointCount += len(section.Endpoints)
		summary.FieldCount += len(section.Fields)
		summary.EnumSetCount += len(section.EnumSets)
		summary.ErrorCodeCount += len(section.ErrorCodes)
		summary.WarningCount += len(section.Warnings)
		for _, enumSet := range section.EnumSets {
			summary.EnumValueCount += len(enumSet.Values)
		}
	}
	summary.UnknownTableCount = len(unknownTables)
	summary.WarningCount += len(globalWarnings)

	return &Extraction{
		Sections:      sections,
		UnknownTables: unknownTables,
		Warnings:      uniqueSortedStrings(globalWarnings),
		Summary:       summary,
	}
}

func ensureSection(builders map[string]*sectionBuilder, order *[]string, path []string) *sectionBuilder {
	key := sectionKey(path)
	if builder, ok := builders[key]; ok {
		return builder
	}
	heading := "Document Root"
	if len(path) > 0 {
		heading = path[len(path)-1]
	}
	builder := &sectionBuilder{
		Heading:      heading,
		Path:         append([]string(nil), path...),
		endpointSeen: make(map[string]struct{}),
		fieldSeen:    make(map[string]struct{}),
		enumSeen:     make(map[string]struct{}),
		errorSeen:    make(map[string]struct{}),
	}
	builders[key] = builder
	*order = append(*order, key)
	return builder
}

func (builder *sectionBuilder) addEndpoint(endpoint Endpoint) {
	key := endpoint.Method + " " + endpoint.Path
	if _, ok := builder.endpointSeen[key]; ok {
		return
	}
	builder.endpointSeen[key] = struct{}{}
	builder.Endpoints = append(builder.Endpoints, endpoint)
}

func (builder *sectionBuilder) addField(field Field) {
	key := field.Name + "|" + field.Requirement + "|" + field.Type
	if _, ok := builder.fieldSeen[key]; ok {
		return
	}
	builder.fieldSeen[key] = struct{}{}
	builder.Fields = append(builder.Fields, field)
}

func (builder *sectionBuilder) addEnumSet(enumSet EnumSet) {
	key := enumSet.Kind + "|" + enumSet.Name
	if _, ok := builder.enumSeen[key]; ok {
		return
	}
	builder.enumSeen[key] = struct{}{}
	builder.EnumSets = append(builder.EnumSets, enumSet)
}

func (builder *sectionBuilder) addErrorCode(errorCode ErrorCode) {
	key := errorCode.Code
	if _, ok := builder.errorSeen[key]; ok {
		return
	}
	builder.errorSeen[key] = struct{}{}
	builder.ErrorCodes = append(builder.ErrorCodes, errorCode)
}

func (builder *sectionBuilder) build() Section {
	section := Section{
		Heading:    builder.Heading,
		Path:       append([]string(nil), builder.Path...),
		Endpoints:  append([]Endpoint(nil), builder.Endpoints...),
		Fields:     append([]Field(nil), builder.Fields...),
		EnumSets:   append([]EnumSet(nil), builder.EnumSets...),
		ErrorCodes: append([]ErrorCode(nil), builder.ErrorCodes...),
		Warnings:   uniqueSortedStrings(builder.Warnings),
	}

	sort.Slice(section.Endpoints, func(i, j int) bool {
		if section.Endpoints[i].Method == section.Endpoints[j].Method {
			return section.Endpoints[i].Path < section.Endpoints[j].Path
		}
		return section.Endpoints[i].Method < section.Endpoints[j].Method
	})
	sort.Slice(section.Fields, func(i, j int) bool {
		return section.Fields[i].Line < section.Fields[j].Line
	})
	sort.Slice(section.EnumSets, func(i, j int) bool {
		return section.EnumSets[i].Line < section.EnumSets[j].Line
	})
	sort.Slice(section.ErrorCodes, func(i, j int) bool {
		return section.ErrorCodes[i].Line < section.ErrorCodes[j].Line
	})

	return section
}

func captureInlineEndpoints(section *sectionBuilder, line string, lineNo int) {
	matches := inlineMethodPathRe.FindAllStringSubmatch(line, -1)
	for _, match := range matches {
		if len(match) != 3 {
			continue
		}
		section.addEndpoint(Endpoint{Method: strings.ToUpper(match[1]), Path: normalizePath(match[2]), Line: lineNo})
	}
}

func applyTable(builders map[string]*sectionBuilder, order *[]string, table markdownTable, unknownTables *[]UnknownTable) bool {
	roles := classifyHeaders(table.Headers)
	section := ensureSection(builders, order, table.Path)

	switch {
	case roles.HasMethod && roles.HasPath:
		for _, row := range table.Rows {
			method := strings.ToUpper(cellAt(row, roles.Method))
			path := normalizePath(cellAt(row, roles.Path))
			if method == "" || path == "" {
				continue
			}
			section.addEndpoint(Endpoint{Method: method, Path: path, Line: table.Line})
		}
		return true
	case roles.HasError:
		for rowIndex, row := range table.Rows {
			code := cleanCell(cellAt(row, roles.Code))
			if code == "" {
				continue
			}
			errorCode := ErrorCode{
				Code:        code,
				Description: cleanCell(cellAt(row, roles.Description)),
				Resolution:  cleanCell(cellAt(row, roles.Resolution)),
				Line:        table.Line + rowIndex + 2,
			}
			section.addErrorCode(errorCode)
		}
		return true
	case roles.HasField:
		for rowIndex, row := range table.Rows {
			name := cleanCell(cellAt(row, roles.Name))
			if name == "" {
				continue
			}
			description := cleanCell(cellAt(row, roles.Description))
			requirement, condition := parseRequirement(cellAt(row, roles.Required), cellAt(row, roles.Condition), description)
			enumValues := extractInlineValues(strings.TrimSpace(cellAt(row, roles.Enum) + " " + description))
			field := Field{
				Name:        name,
				Type:        cleanCell(cellAt(row, roles.Type)),
				Requirement: requirement,
				Condition:   condition,
				EnumValues:  enumValues,
				Description: description,
				Line:        table.Line + rowIndex + 2,
			}
			section.addField(field)
		}
		return true
	case roles.HasValue:
		kind := inferEnumKind(table.Path, table.Headers)
		name := inferEnumSetName(table.Path, table.Headers, kind)
		values := make([]EnumValue, 0, len(table.Rows))
		for rowIndex, row := range table.Rows {
			value := cleanCell(cellAt(row, roles.Value))
			if value == "" {
				continue
			}
			values = append(values, EnumValue{Value: value, Description: cleanCell(cellAt(row, roles.Description)), Line: table.Line + rowIndex + 2})
		}
		if len(values) == 0 {
			return false
		}
		section.addEnumSet(EnumSet{Name: name, Kind: kind, Values: values, Line: table.Line})
		return true
	default:
		*unknownTables = append(*unknownTables, UnknownTable{SectionPath: append([]string(nil), table.Path...), Headers: append([]string(nil), table.Headers...), Line: table.Line})
		return false
	}
}

func inferEnumSetName(path []string, headers []string, kind string) string {
	for index := len(path) - 1; index >= 0; index-- {
		heading := strings.TrimSpace(path[index])
		if heading == "" {
			continue
		}
		return heading
	}
	if len(headers) > 0 {
		return strings.TrimSpace(headers[0])
	}
	if kind == "status" {
		return "status"
	}
	return "enum"
}

func inferEnumKind(path []string, headers []string) string {
	for _, heading := range path {
		normalized := normalizeHeader(heading)
		if strings.Contains(normalized, "状态") || strings.Contains(normalized, "status") || strings.Contains(normalized, "result") {
			return "status"
		}
	}
	for _, header := range headers {
		normalized := normalizeHeader(header)
		if strings.Contains(normalized, "状态") || strings.Contains(normalized, "status") || strings.Contains(normalized, "result") {
			return "status"
		}
	}
	return "enum"
}

func parseRequirement(requiredCell, conditionCell, description string) (string, string) {
	rawRequired := strings.TrimSpace(requiredCell)
	rawCondition := strings.TrimSpace(conditionCell)
	normalizedRequired := normalizeText(rawRequired)
	normalizedCondition := normalizeText(rawCondition)
	normalizedDescription := normalizeText(description)

	if indicatesRequired(normalizedRequired) {
		condition := strings.TrimSpace(rawCondition)
		if condition == "" && containsAny(normalizedDescription, []string{"条件必填", "二选一必填", "选一必填", "当", "如果", "仅当", "时必填", "时必传"}) {
			condition = extractCondition(description)
			if condition == "" {
				condition = description
			}
			return "conditional", condition
		}
		if condition != "" && containsAny(normalizedCondition, []string{"当", "如果", "仅当"}) {
			return "conditional", condition
		}
		return "required", condition
	}
	if indicatesOptional(normalizedRequired) {
		if containsAny(normalizedDescription, []string{"条件必填", "二选一必填", "选一必填", "当", "如果", "仅当", "时必填", "时必传"}) {
			condition := extractCondition(description)
			if condition == "" {
				condition = description
			}
			return "conditional", condition
		}
		return "optional", strings.TrimSpace(rawCondition)
	}
	if containsAny(normalizedDescription, []string{"条件必填", "二选一必填", "选一必填", "当", "如果", "仅当", "时必填", "时必传"}) {
		condition := extractCondition(description)
		if condition == "" {
			condition = description
		}
		return "conditional", condition
	}
	return "unknown", strings.TrimSpace(rawCondition)
}

func indicatesRequired(normalizedRequired string) bool {
	if normalizedRequired == "" {
		return false
	}
	return normalizedRequired == "是" ||
		normalizedRequired == "true" ||
		normalizedRequired == "yes" ||
		normalizedRequired == "required" ||
		normalizedRequired == "必填"
}

func indicatesOptional(normalizedRequired string) bool {
	if normalizedRequired == "" {
		return false
	}
	return normalizedRequired == "否" ||
		normalizedRequired == "false" ||
		normalizedRequired == "no" ||
		normalizedRequired == "optional" ||
		normalizedRequired == "选填"
}

func extractCondition(description string) string {
	trimmed := strings.TrimSpace(description)
	if trimmed == "" {
		return ""
	}
	for _, token := range []string{"当", "如果", "仅当"} {
		if index := strings.Index(trimmed, token); index >= 0 {
			return strings.TrimSpace(trimmed[index:])
		}
	}
	return ""
}

func extractInlineValues(text string) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	markers := []string{"可选值", "枚举值", "取值", "状态", "结果"}
	for _, marker := range markers {
		index := strings.Index(trimmed, marker)
		if index < 0 {
			continue
		}
		remainder := strings.TrimSpace(trimmed[index+len(marker):])
		remainder = strings.TrimPrefix(remainder, "：")
		remainder = strings.TrimPrefix(remainder, ":")
		remainder = strings.TrimSpace(remainder)
		for _, terminator := range []string{"。", ";", "；"} {
			if cut := strings.Index(remainder, terminator); cut >= 0 {
				remainder = remainder[:cut]
				break
			}
		}
		return splitValues(remainder)
	}
	return nil
}

func splitValues(text string) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	replacer := strings.NewReplacer("/", " ", "、", " ", "，", " ", ",", " ", "|", " ", "；", " ", ";", " ")
	normalized := replacer.Replace(trimmed)
	parts := strings.Fields(normalized)
	seen := make(map[string]struct{}, len(parts))
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := cleanCell(part)
		if value == "" {
			continue
		}
		if !looksLikeEnumValue(value) {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func looksLikeEnumValue(value string) bool {
	if value == "" {
		return false
	}
	hasSignal := false
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			hasSignal = true
		case r >= '0' && r <= '9':
			hasSignal = true
		case r == '_' || r == '-':
			hasSignal = true
		case r >= 'a' && r <= 'z':
			hasSignal = true
		default:
		}
	}
	return hasSignal
}

func classifyHeaders(headers []string) headerRoles {
	roles := headerRoles{
		Name:        -1,
		Type:        -1,
		Required:    -1,
		Condition:   -1,
		Description: -1,
		Enum:        -1,
		Method:      -1,
		Path:        -1,
		Code:        -1,
		Resolution:  -1,
		Value:       -1,
	}

	for index, header := range headers {
		normalized := normalizeHeader(header)
		switch {
		case matchesHeader(normalized, []string{"错误码", "errorcode", "error", "code"}):
			if roles.Code == -1 {
				roles.Code = index
			}
			roles.HasError = true
		case matchesHeader(normalized, []string{"字段名", "字段", "参数名", "参数", "field", "name", "属性"}):
			if roles.Name == -1 {
				roles.Name = index
			}
			roles.HasField = true
		case matchesHeader(normalized, []string{"类型", "type", "格式"}):
			if roles.Type == -1 {
				roles.Type = index
			}
			roles.HasType = true
		case matchesHeader(normalized, []string{"必填", "required", "是否必填", "是否必须"}):
			if roles.Required == -1 {
				roles.Required = index
			}
			roles.HasRequired = true
		case matchesHeader(normalized, []string{"条件", "条件必填", "必填条件", "condition"}):
			if roles.Condition == -1 {
				roles.Condition = index
			}
		case matchesHeader(normalized, []string{"描述", "说明", "备注", "含义", "description", "desc"}):
			if roles.Description == -1 {
				roles.Description = index
			}
		case matchesHeader(normalized, []string{"可选值", "枚举值", "取值", "enum"}):
			if roles.Enum == -1 {
				roles.Enum = index
			}
		case matchesHeader(normalized, []string{"请求方式", "method", "httpmethod"}):
			if roles.Method == -1 {
				roles.Method = index
			}
			roles.HasMethod = true
		case matchesHeader(normalized, []string{"请求地址", "请求url", "接口地址", "url", "path", "requesturl"}):
			if roles.Path == -1 {
				roles.Path = index
			}
			roles.HasPath = true
		case matchesHeader(normalized, []string{"解决方案", "处理建议", "solution", "resolution"}):
			if roles.Resolution == -1 {
				roles.Resolution = index
			}
		case matchesHeader(normalized, []string{"状态", "结果", "status", "result", "value", "枚举"}):
			if roles.Value == -1 {
				roles.Value = index
			}
			roles.HasValue = true
			if strings.Contains(normalized, "状态") || strings.Contains(normalized, "status") || strings.Contains(normalized, "result") {
				roles.HasStatus = true
			}
		}
	}

	if roles.Value == -1 && roles.Enum != -1 && !roles.HasField && !roles.HasError {
		roles.Value = roles.Enum
		roles.HasValue = true
	}
	if roles.Description == -1 && len(headers) > 1 {
		roles.Description = 1
	}
	return roles
}

func matchesHeader(normalized string, candidates []string) bool {
	for _, candidate := range candidates {
		if normalized == normalizeHeader(candidate) {
			return true
		}
	}
	return false
}

func parseHeading(line string) (int, string, bool) {
	match := headingRe.FindStringSubmatch(line)
	if len(match) != 3 {
		return 0, "", false
	}
	return len(match[1]), strings.TrimSpace(match[2]), true
}

func updateHeadings(headings []headingEntry, level int, title string) []headingEntry {
	trimmed := make([]headingEntry, 0, len(headings)+1)
	for _, heading := range headings {
		if heading.Level < level {
			trimmed = append(trimmed, heading)
		}
	}
	trimmed = append(trimmed, headingEntry{Level: level, Title: title})
	return trimmed
}

func headingsToPath(headings []headingEntry) []string {
	path := make([]string, 0, len(headings))
	for _, heading := range headings {
		path = append(path, heading.Title)
	}
	return path
}

func isFenceDelimiter(line string) bool {
	return strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~")
}

func isTableHeader(lines []string, index int) bool {
	if index+1 >= len(lines) {
		return false
	}
	if !strings.Contains(lines[index], "|") {
		return false
	}
	return isSeparatorRow(lines[index+1])
}

func isSeparatorRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.Contains(trimmed, "|") {
		return false
	}
	parts := splitMarkdownRow(trimmed)
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		cell := strings.TrimSpace(part)
		cell = strings.Trim(cell, ":")
		if len(cell) < 3 || strings.Trim(cell, "-") != "" {
			return false
		}
	}
	return true
}

func parseTable(lines []string, start int, path []string) (markdownTable, int) {
	headers := splitMarkdownRow(lines[start])
	rows := make([][]string, 0)
	index := start + 2
	for index < len(lines) {
		line := strings.TrimSpace(lines[index])
		if line == "" || !strings.Contains(line, "|") || isSeparatorRow(line) {
			break
		}
		rows = append(rows, splitMarkdownRow(line))
		index++
	}
	return markdownTable{Headers: headers, Rows: rows, Line: start + 1, Path: append([]string(nil), path...)}, index - 1
}

func (table markdownTable) isEmpty() bool {
	return len(table.Headers) == 0 || len(table.Rows) == 0
}

func splitMarkdownRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := make([]string, 0)
	var builder strings.Builder
	escaped := false
	for _, r := range line {
		switch {
		case escaped:
			builder.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '|':
			parts = append(parts, strings.TrimSpace(builder.String()))
			builder.Reset()
		default:
			builder.WriteRune(r)
		}
	}
	parts = append(parts, strings.TrimSpace(builder.String()))
	return parts
}

func cellAt(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return row[index]
}

func normalizeHeader(header string) string {
	trimmed := cleanCell(header)
	trimmed = strings.ToLower(trimmed)
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", "（", "", "）", "", "(", "", ")", "", "?", "", "？", "", "*", "")
	return replacer.Replace(trimmed)
}

func normalizeText(text string) string {
	trimmed := strings.ToLower(strings.TrimSpace(text))
	replacer := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "")
	return replacer.Replace(trimmed)
}

func cleanCell(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.Trim(trimmed, "`")
	trimmed = strings.Trim(trimmed, "*")
	trimmed = strings.TrimSpace(trimmed)
	return trimmed
}

func normalizePath(path string) string {
	trimmed := strings.TrimSpace(path)
	trimmed = strings.Trim(trimmed, "`)")
	trimmed = strings.TrimRight(trimmed, ".,;:。；，")
	return trimmed
}

func sectionKey(path []string) string {
	if len(path) == 0 {
		return "__root__"
	}
	return strings.Join(path, " > ")
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func containsAny(haystack string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(haystack, normalizeText(needle)) {
			return true
		}
	}
	return false
}
