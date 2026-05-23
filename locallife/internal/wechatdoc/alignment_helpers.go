package wechatdoc

import (
	"reflect"
	"sort"
	"strings"
	"unicode"
)

type AlignmentAudit struct {
	Scope          string                   `json:"scope"`
	Summary        AlignmentSummary         `json:"summary"`
	Endpoints      []EndpointAlignmentAudit `json:"endpoints,omitempty"`
	SuppressedGaps []EndpointAlignmentAudit `json:"suppressed_gaps,omitempty"`
}

type AlignmentSummary struct {
	DocumentedEndpointCount          int `json:"documented_endpoint_count"`
	AuditedEndpointCount             int `json:"audited_endpoint_count"`
	MissingEndpointCount             int `json:"missing_endpoint_count"`
	MissingRequestFieldCount         int `json:"missing_request_field_count"`
	MissingResponseFieldCount        int `json:"missing_response_field_count"`
	MissingRequestConstraintCount    int `json:"missing_request_constraint_count"`
	MissingResponseConstraintCount   int `json:"missing_response_constraint_count"`
	MissingRequestEnumCount          int `json:"missing_request_enum_count"`
	MissingResponseEnumCount         int `json:"missing_response_enum_count"`
	MissingErrorCodeCount            int `json:"missing_error_code_count"`
	SuppressedMissingRequestFields   int `json:"suppressed_missing_request_fields,omitempty"`
	SuppressedMissingResponseFields  int `json:"suppressed_missing_response_fields,omitempty"`
	SuppressedMissingRequestEnums    int `json:"suppressed_missing_request_enums,omitempty"`
	SuppressedMissingResponseEnums   int `json:"suppressed_missing_response_enums,omitempty"`
	SuppressedMissingErrorCodeCount  int `json:"suppressed_missing_error_code_count,omitempty"`
	SuppressedMissingConstraintCount int `json:"suppressed_missing_constraint_count,omitempty"`
}

type EndpointAlignmentAudit struct {
	Method                     string                 `json:"method"`
	Path                       string                 `json:"path"`
	MissingEndpoint            bool                   `json:"missing_endpoint,omitempty"`
	MissingRequestFields       []string               `json:"missing_request_fields,omitempty"`
	MissingResponseFields      []string               `json:"missing_response_fields,omitempty"`
	MissingRequestConstraints  []FieldConstraintAudit `json:"missing_request_constraints,omitempty"`
	MissingResponseConstraints []FieldConstraintAudit `json:"missing_response_constraints,omitempty"`
	MissingRequestEnums        []FieldEnumAudit       `json:"missing_request_enums,omitempty"`
	MissingResponseEnums       []FieldEnumAudit       `json:"missing_response_enums,omitempty"`
	MissingErrorCodes          []string               `json:"missing_error_codes,omitempty"`
}

type FieldEnumAudit struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}

type FieldConstraintAudit struct {
	Field              string   `json:"field"`
	Constraint         string   `json:"constraint,omitempty"`
	Missing            []string `json:"missing,omitempty"`
	MissingConstraints []string `json:"missing_constraints,omitempty"`
}

type fieldEnumDiff struct {
	Missing []FieldEnumAudit
	Extra   []FieldEnumAudit
}

type endpointDoc struct {
	Method         string
	Path           string
	RequestFields  map[string]Field
	ResponseFields map[string]Field
	ErrorCodes     map[string]struct{}
}

func collectDocEndpoints(extraction *Extraction, canonicalizeCode func(string) string) map[string]*endpointDoc {
	index := make(map[string]*endpointDoc)
	if extraction == nil {
		return index
	}
	for _, section := range extraction.Sections {
		for _, endpoint := range section.Endpoints {
			ensureDocEndpoint(index, endpoint)
		}
	}
	for _, section := range extraction.Sections {
		key := nearestEndpointKey(index, section.Path)
		if key == "" {
			continue
		}
		doc := index[key]
		heading := normalizeText(section.Heading)
		switch {
		case containsAny(heading, []string{"path 参数", "query 参数", "body 参数", "请求参数", "通知参数"}):
			for _, field := range section.Fields {
				doc.RequestFields[field.Name] = field
			}
		case containsAny(heading, []string{"应答参数", "响应参数", "返回参数", "资源数据"}):
			for _, field := range section.Fields {
				doc.ResponseFields[field.Name] = field
			}
		case containsAny(heading, []string{"错误码"}):
			for _, code := range section.ErrorCodes {
				canonical := canonicalizeCode(code.Code)
				if canonical != "" {
					doc.ErrorCodes[canonical] = struct{}{}
				}
			}
		}
	}
	return index
}

func nearestEndpointKey(index map[string]*endpointDoc, path []string) string {
	if len(index) == 0 || len(path) == 0 {
		return ""
	}
	bestKey := ""
	bestLen := -1
	for key, doc := range index {
		if doc == nil {
			continue
		}
		docPath := pathWithoutLeaf(path)
		score := commonPrefixLength(docPath, path)
		if score > bestLen {
			bestKey = key
			bestLen = score
		}
	}
	return bestKey
}

func ensureDocEndpoint(index map[string]*endpointDoc, endpoint Endpoint) *endpointDoc {
	key := endpointAuditKey(endpoint.Method, endpoint.Path)
	if existing := index[key]; existing != nil {
		return existing
	}
	doc := &endpointDoc{
		Method:         strings.ToUpper(endpoint.Method),
		Path:           endpoint.Path,
		RequestFields:  make(map[string]Field),
		ResponseFields: make(map[string]Field),
		ErrorCodes:     make(map[string]struct{}),
	}
	index[key] = doc
	return doc
}

func pathWithoutLeaf(path []string) []string {
	if len(path) <= 1 {
		return path
	}
	return path[:len(path)-1]
}

func commonPrefixLength(left, right []string) int {
	max := len(left)
	if len(right) < max {
		max = len(right)
	}
	count := 0
	for count < max && left[count] == right[count] {
		count++
	}
	return count
}

func accumulateAlignmentSummary(summary *AlignmentSummary, audit EndpointAlignmentAudit) {
	if audit.MissingEndpoint {
		summary.MissingEndpointCount++
	}
	summary.MissingRequestFieldCount += len(audit.MissingRequestFields)
	summary.MissingResponseFieldCount += len(audit.MissingResponseFields)
	summary.MissingRequestConstraintCount += len(audit.MissingRequestConstraints)
	summary.MissingResponseConstraintCount += len(audit.MissingResponseConstraints)
	summary.MissingRequestEnumCount += countMissingEnumValues(audit.MissingRequestEnums)
	summary.MissingResponseEnumCount += countMissingEnumValues(audit.MissingResponseEnums)
	summary.MissingErrorCodeCount += len(audit.MissingErrorCodes)
}

func findEndpointAudit(audits []EndpointAlignmentAudit, method, path string) *EndpointAlignmentAudit {
	for i := range audits {
		if strings.EqualFold(strings.TrimSpace(audits[i].Method), strings.TrimSpace(method)) &&
			normalizePath(audits[i].Path) == normalizePath(path) {
			return &audits[i]
		}
	}
	return nil
}

func countMissingEnumValues(audits []FieldEnumAudit) int {
	total := 0
	for _, audit := range audits {
		total += len(audit.Values)
	}
	return total
}

func endpointHasMissingCoverage(audit EndpointAlignmentAudit) bool {
	return audit.MissingEndpoint ||
		len(audit.MissingRequestFields) > 0 ||
		len(audit.MissingResponseFields) > 0 ||
		len(audit.MissingRequestConstraints) > 0 ||
		len(audit.MissingResponseConstraints) > 0 ||
		len(audit.MissingRequestEnums) > 0 ||
		len(audit.MissingResponseEnums) > 0 ||
		len(audit.MissingErrorCodes) > 0
}

func structJSONFields(structType reflect.Type) map[string]struct{} {
	fields := make(map[string]struct{})
	collectJSONFields(structType, "", fields)
	return fields
}

func collectJSONFields(fieldType reflect.Type, prefix string, dest map[string]struct{}) {
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}
	if fieldType.Kind() == reflect.Slice || fieldType.Kind() == reflect.Array {
		fieldType = fieldType.Elem()
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
	}
	if fieldType.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < fieldType.NumField(); i++ {
		field := fieldType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := jsonFieldName(field)
		if name == "" {
			continue
		}
		fullName := name
		if prefix != "" {
			fullName = prefix + "." + name
		}
		dest[fullName] = struct{}{}
		collectJSONFields(field.Type, fullName, dest)
	}
}

func jsonFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return ""
	}
	name := strings.Split(tag, ",")[0]
	if name == "" {
		name = field.Name
	}
	return name
}

func diffFieldNames(documented map[string]Field, known map[string]struct{}) []string {
	var missing []string
	for name := range documented {
		if _, ok := known[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

func diffFieldEnums(documented map[string]Field, known map[string]map[string]struct{}, ignored map[string]map[string]struct{}) fieldEnumDiff {
	var diff fieldEnumDiff
	for name, field := range documented {
		if len(field.EnumValues) == 0 {
			continue
		}
		knownValues := known[name]
		ignoredValues := ignored[name]
		var missing []string
		for _, value := range field.EnumValues {
			if !isComparableEnumValue(value) {
				continue
			}
			if _, ok := knownValues[value]; ok {
				continue
			}
			if _, ok := ignoredValues[value]; ok {
				continue
			}
			missing = append(missing, value)
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			diff.Missing = append(diff.Missing, FieldEnumAudit{Field: name, Values: missing})
		}
	}
	for name, values := range known {
		documentedField, ok := documented[name]
		if !ok || len(values) == 0 {
			continue
		}
		documentedValues := make(map[string]struct{}, len(documentedField.EnumValues))
		for _, value := range documentedField.EnumValues {
			documentedValues[value] = struct{}{}
		}
		var extra []string
		for value := range values {
			if _, ok := documentedValues[value]; !ok {
				extra = append(extra, value)
			}
		}
		if len(extra) > 0 {
			sort.Strings(extra)
			diff.Extra = append(diff.Extra, FieldEnumAudit{Field: name, Values: extra})
		}
	}
	sort.Slice(diff.Missing, func(i, j int) bool { return diff.Missing[i].Field < diff.Missing[j].Field })
	sort.Slice(diff.Extra, func(i, j int) bool { return diff.Extra[i].Field < diff.Extra[j].Field })
	return diff
}

func diffFieldConstraints(documented map[string]Field, known map[string]map[string]struct{}) []FieldConstraintAudit {
	var missing []FieldConstraintAudit
	for name, field := range documented {
		knownConstraints := known[name]
		for _, constraint := range inferFieldConstraints(field) {
			if _, ok := knownConstraints[constraint]; !ok {
				missing = append(missing, FieldConstraintAudit{Field: name, Constraint: constraint, Missing: []string{constraint}, MissingConstraints: []string{constraint}})
			}
		}
	}
	sort.Slice(missing, func(i, j int) bool {
		if missing[i].Field == missing[j].Field {
			return missing[i].Constraint < missing[j].Constraint
		}
		return missing[i].Field < missing[j].Field
	})
	return missing
}

func inferFieldConstraints(field Field) []string {
	text := normalizeText(field.Description + " " + field.Requirement + " " + field.Condition)
	var constraints []string
	if strings.Contains(text, "单位为分") || strings.Contains(text, "单位（分）") || strings.Contains(text, "单位:分") || strings.Contains(text, "单位：分") {
		constraints = append(constraints, "unit_fen")
	}
	if strings.Contains(strings.ToLower(text), "rfc3339") {
		constraints = append(constraints, "format_rfc3339")
	}
	return constraints
}

func isComparableEnumValue(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if unicode.IsLetter(r) && !isCJKRune(r) {
			return true
		}
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func isCJKRune(r rune) bool {
	return r >= '\u4e00' && r <= '\u9fff'
}

func diffSet(documented map[string]struct{}, known map[string]struct{}) []string {
	var missing []string
	for value := range documented {
		if _, ok := known[value]; !ok {
			missing = append(missing, value)
		}
	}
	sort.Strings(missing)
	return missing
}

func sortedFieldNames(fields map[string]Field) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedSetKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return keys
}

func setOf(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func sortedEndpointKeys(endpoints map[string]*endpointDoc) []string {
	keys := make([]string, 0, len(endpoints))
	for key := range endpoints {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func endpointAuditKey(method, path string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + normalizePath(path)
}
