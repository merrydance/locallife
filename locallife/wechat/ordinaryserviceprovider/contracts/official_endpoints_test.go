package contracts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	osperrorcodes "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/errorcodes"
)

func TestOfficialContractCapabilityGroupsMatchOrdinaryServiceProviderCapabilities(t *testing.T) {
	tests := map[string]int{
		"特约商户进件与结算账户":         7,
		"商户管控、商户平台处置通知与不活跃核实": 8,
		"小程序支付":   8,
		"小程序合单支付": 8,
		"订单退款":    3,
		"分账":      9,
	}

	if len(officialContractCapabilityGroups) != len(tests) {
		t.Fatalf("capability group count = %d, want %d", len(officialContractCapabilityGroups), len(tests))
	}
	for _, group := range officialContractCapabilityGroups {
		wantCount, ok := tests[group.Name]
		if !ok {
			t.Fatalf("unexpected capability group %s", group.Name)
		}
		if len(group.Bindings) != wantCount {
			t.Fatalf("%s endpoint count = %d, want %d", group.Name, len(group.Bindings), wantCount)
		}
		for _, endpoint := range group.Bindings {
			if len(endpoint.RequestContracts) == 0 && len(endpoint.ResponseContracts) == 0 {
				t.Fatalf("%s/%s must map at least one request or response contract", group.Name, endpoint.Title)
			}
		}
	}
}

func TestOfficialContractBindingsCoverImplementedOrdinaryDocs(t *testing.T) {
	if len(officialContractBindings) != 43 {
		t.Fatalf("official contract binding count = %d, want 43", len(officialContractBindings))
	}

	seenTitles := map[string]struct{}{}
	for _, endpoint := range officialContractBindings {
		if endpoint.Title == "" || endpoint.URL == "" || endpoint.Method == "" || endpoint.Path == "" {
			t.Fatalf("official contract binding entries must include title, url, method and path: %+v", endpoint)
		}
		if !strings.HasPrefix(endpoint.URL, "https://pay.weixin.qq.com/doc/v3/partner/") {
			t.Fatalf("%s must point at the official ordinary service provider doc, got %s", endpoint.Title, endpoint.URL)
		}
		if _, exists := seenTitles[endpoint.Title]; exists {
			t.Fatalf("duplicate official endpoint title %s", endpoint.Title)
		}
		seenTitles[endpoint.Title] = struct{}{}
		if endpoint.Method != methodCallback && endpoint.Method != methodLocalContract && endpoint.ErrorCodeSet == "" {
			t.Fatalf("%s must reference its endpoint-specific documented error-code set", endpoint.Title)
		}
		if endpoint.ErrorCodeSet != "" {
			if _, ok := ordinaryEndpointErrorCodeSetNames()[endpoint.ErrorCodeSet]; !ok {
				t.Fatalf("%s references missing error-code set %s", endpoint.Title, endpoint.ErrorCodeSet)
			}
		}
		for _, name := range append(endpoint.RequestContracts, endpoint.ResponseContracts...) {
			if _, ok := ordinaryContractTypesByName()[name]; !ok {
				t.Fatalf("%s references missing contract type %s", endpoint.Title, name)
			}
		}
	}
}

func TestOfficialContractBindingsNameStateOwnersForStatefulDocs(t *testing.T) {
	tests := map[string]string{
		"特约商户进件-申请单号查询申请状态":       "ApplymentState",
		"不活跃商户身份核实-查询不活跃商户身份核实结果": "InactiveMerchantIdentityVerificationState",
		"小程序支付-微信支付订单号查询订单":       "PaymentTradeState",
		"小程序合单支付-查询合单订单":          "PaymentTradeState",
		"订单退款-查询单笔退款（通过商户退款单号）":   "RefundStatus",
		"分账-查询分账结果":               "ProfitSharingOrderState",
		"分账-查询分账回退结果":             "ProfitSharingReturnState",
	}

	for title, wantStatus := range tests {
		endpoint, ok := officialContractBindingByTitle(title)
		if !ok {
			t.Fatalf("missing endpoint %s", title)
		}
		if !contains(endpoint.StatusConstants, wantStatus) {
			t.Fatalf("%s status constants = %#v, want %s", title, endpoint.StatusConstants, wantStatus)
		}
	}
}

func TestOfficialContractBindingsCoverSnapshotFieldPaths(t *testing.T) {
	snapshot := loadOfficialSnapshot(t)
	typesByName := ordinaryContractTypesByName()

	for _, endpoint := range officialContractBindings {
		entry, ok := snapshot[endpoint.Title]
		if !ok {
			t.Fatalf("missing snapshot entry for %s", endpoint.Title)
		}
		contractFields := map[string]struct{}{}
		for _, name := range append(endpoint.RequestContracts, endpoint.ResponseContracts...) {
			typ, ok := typesByName[name]
			if !ok {
				t.Fatalf("%s references missing contract type %s", endpoint.Title, name)
			}
			for field := range collectJSONFieldPaths(typ, "") {
				contractFields[field] = struct{}{}
			}
		}

		var missing []string
		for _, field := range entry.Fields {
			if field == "" {
				continue
			}
			if _, ok := contractFields[field]; !ok {
				missing = append(missing, field)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			t.Fatalf("%s contract fields missing official paths: %v", endpoint.Title, missing)
		}
	}
}

func TestOfficialRequestContractsDoNotExposeUnofficialJSONFields(t *testing.T) {
	snapshot := loadOfficialSnapshot(t)
	typesByName := ordinaryContractTypesByName()

	for _, endpoint := range officialContractBindings {
		if endpoint.Method == methodGet || endpoint.Method == methodDelete || endpoint.Method == methodCallback || endpoint.Method == methodLocalContract || len(endpoint.RequestContracts) == 0 {
			continue
		}
		entry, ok := snapshot[endpoint.Title]
		if !ok {
			t.Fatalf("missing snapshot entry for %s", endpoint.Title)
		}
		officialFields := map[string]struct{}{}
		for _, field := range entry.Fields {
			officialFields[field] = struct{}{}
		}

		for _, name := range endpoint.RequestContracts {
			typ := typesByName[name]
			var extra []string
			for field := range collectJSONFieldPaths(typ, "") {
				if _, ok := officialFields[field]; !ok {
					extra = append(extra, field)
				}
			}
			if len(extra) > 0 {
				sort.Strings(extra)
				t.Fatalf("%s request contract %s exposes unofficial JSON fields: %v", endpoint.Title, name, extra)
			}
		}
	}
}

func TestOfficialSnapshotMapsFieldMetadataFromWechatDocs(t *testing.T) {
	snapshot := loadOfficialSnapshot(t)

	for _, endpoint := range officialContractBindings {
		entry, ok := snapshot[endpoint.Title]
		if !ok {
			t.Fatalf("missing snapshot entry for %s", endpoint.Title)
		}
		if entry.Method == "" || entry.Path == "" {
			t.Fatalf("%s snapshot must keep the official method/path or callback/local path", endpoint.Title)
		}
		if !entry.Audit.LegacyFieldsCoverAllNonHeaderPaths {
			t.Fatalf("%s legacy fields must cover every non-header official request/response/notification path", endpoint.Title)
		}
		if !entry.Audit.LegacyErrorCodesCoverOfficialCodes {
			t.Fatalf("%s legacy error_codes must cover every official error code", endpoint.Title)
		}

		metadataByPath := map[string]officialSnapshotField{}
		for _, field := range entry.nonHeaderMetadataFields() {
			if field.Path == "" || field.Type == "" || field.Description == "" {
				t.Fatalf("%s has incomplete official field metadata: %+v", endpoint.Title, field)
			}
			if field.ConditionalRequired != "" && field.Required {
				t.Fatalf("%s marks required field %s as conditionally required", endpoint.Title, field.Path)
			}
			metadataByPath[field.Path] = field
		}
		if len(metadataByPath) != entry.Audit.OfficialNonHeaderPathCount {
			t.Fatalf("%s official metadata path count = %d, want audit count %d", endpoint.Title, len(metadataByPath), entry.Audit.OfficialNonHeaderPathCount)
		}

		for _, field := range entry.Fields {
			if _, ok := metadataByPath[field]; !ok {
				t.Fatalf("%s legacy snapshot field %s lacks official type/required metadata", endpoint.Title, field)
			}
		}
		if uniqueStringCount(entry.Fields) != entry.Audit.LegacyFieldCount {
			t.Fatalf("%s legacy unique field count = %d, want audit count %d", endpoint.Title, uniqueStringCount(entry.Fields), entry.Audit.LegacyFieldCount)
		}

		errorDetailsByCode := map[string]struct{}{}
		for _, detail := range entry.ErrorCodeDetails {
			if detail.Code == "" || detail.StatusCode == "" || detail.Description == "" || detail.Solution == "" {
				t.Fatalf("%s has incomplete official error-code metadata: %+v", endpoint.Title, detail)
			}
			errorDetailsByCode[detail.Code] = struct{}{}
		}
		for _, code := range entry.ErrorCodes {
			if _, ok := errorDetailsByCode[code]; !ok {
				t.Fatalf("%s legacy error code %s lacks official status/description/solution metadata", endpoint.Title, code)
			}
		}
	}
}

func TestOfficialSnapshotFieldTypesMatchGoContractTypes(t *testing.T) {
	snapshot := loadOfficialSnapshot(t)
	typesByName := ordinaryContractTypesByName()

	for _, endpoint := range officialContractBindings {
		entry, ok := snapshot[endpoint.Title]
		if !ok {
			t.Fatalf("missing snapshot entry for %s", endpoint.Title)
		}
		contractTypes := map[string][]reflect.Type{}
		for _, name := range append(endpoint.RequestContracts, endpoint.ResponseContracts...) {
			typ, ok := typesByName[name]
			if !ok {
				t.Fatalf("%s references missing contract type %s", endpoint.Title, name)
			}
			for path, fieldTypes := range collectJSONFieldTypes(typ, "") {
				contractTypes[path] = append(contractTypes[path], fieldTypes...)
			}
		}

		var mismatches []string
		for _, official := range entry.nonHeaderMetadataFields() {
			goTypes, ok := contractTypes[official.Path]
			if !ok {
				mismatches = append(mismatches, official.Path+" missing Go field")
				continue
			}
			if !anyGoTypeMatchesOfficialType(official.Type, goTypes) {
				mismatches = append(mismatches, official.Path+" official "+official.Type+" != Go "+formatReflectTypes(goTypes))
			}
		}
		if len(mismatches) > 0 {
			sort.Strings(mismatches)
			t.Fatalf("%s official field types do not match Go contracts: %v", endpoint.Title, mismatches)
		}
	}
}

type officialSnapshotEntry struct {
	Title            string                        `json:"title"`
	Method           string                        `json:"method"`
	Path             string                        `json:"path"`
	Request          officialSnapshotRequest       `json:"request"`
	Response         officialSnapshotResponse      `json:"response"`
	Notification     officialSnapshotNotification  `json:"notification"`
	Fields           []string                      `json:"fields"`
	ErrorCodes       []string                      `json:"error_codes"`
	ErrorCodeDetails []officialSnapshotErrorDetail `json:"error_code_details"`
	Audit            officialSnapshotAudit         `json:"audit"`
}

type officialSnapshotRequest struct {
	Header []officialSnapshotField `json:"header"`
	Path   []officialSnapshotField `json:"path"`
	Query  []officialSnapshotField `json:"query"`
	Body   []officialSnapshotField `json:"body"`
}

type officialSnapshotResponse struct {
	Body []officialSnapshotField `json:"body"`
}

type officialSnapshotNotification struct {
	SignatureHeaders []officialSnapshotField `json:"signature_headers"`
	Envelope         []officialSnapshotField `json:"envelope"`
	DecryptedPayload []officialSnapshotField `json:"decrypted_resource"`
	Response         []officialSnapshotField `json:"response"`
}

type officialSnapshotField struct {
	Path                string `json:"path"`
	Type                string `json:"type"`
	Required            bool   `json:"required"`
	ConditionalRequired string `json:"conditional_required"`
	ConditionalReturn   string `json:"conditional_return"`
	Description         string `json:"description"`
}

type officialSnapshotErrorDetail struct {
	StatusCode  string `json:"status_code"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Solution    string `json:"solution"`
}

type officialSnapshotAudit struct {
	Source                             string `json:"source"`
	FetchedOn                          string `json:"fetched_on"`
	LegacyFieldsCoverAllNonHeaderPaths bool   `json:"legacy_fields_cover_all_non_header_official_paths"`
	LegacyErrorCodesCoverOfficialCodes bool   `json:"legacy_error_codes_cover_official_codes"`
	OfficialNonHeaderPathCount         int    `json:"official_non_header_path_count"`
	LegacyFieldCount                   int    `json:"legacy_field_count"`
	OfficialErrorCodeCount             int    `json:"official_error_code_count"`
	LegacyErrorCodeCount               int    `json:"legacy_error_code_count"`
}

func (entry officialSnapshotEntry) nonHeaderMetadataFields() []officialSnapshotField {
	var fields []officialSnapshotField
	fields = append(fields, entry.Request.Path...)
	fields = append(fields, entry.Request.Query...)
	fields = append(fields, entry.Request.Body...)
	fields = append(fields, entry.Response.Body...)
	fields = append(fields, entry.Notification.Envelope...)
	fields = append(fields, entry.Notification.DecryptedPayload...)
	fields = append(fields, entry.Notification.Response...)
	return fields
}

func uniqueStringCount(values []string) int {
	seen := map[string]struct{}{}
	for _, value := range values {
		seen[value] = struct{}{}
	}
	return len(seen)
}

func loadOfficialSnapshot(t *testing.T) map[string]officialSnapshotEntry {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file for official snapshot path")
	}
	snapshotPath := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../artifacts/wechat-ordinary-service-provider-official-contract-snapshot-2026-05-02.json"))
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("read official contract snapshot %s: %v", snapshotPath, err)
	}
	var entries []officialSnapshotEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("decode official contract snapshot: %v", err)
	}
	byTitle := make(map[string]officialSnapshotEntry, len(entries))
	for _, entry := range entries {
		byTitle[entry.Title] = entry
	}
	return byTitle
}

func collectJSONFieldPaths(typ reflect.Type, prefix string) map[string]struct{} {
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		typ = typ.Elem()
	}
	paths := map[string]struct{}{}
	if typ.Kind() != reflect.Struct || typ == reflect.TypeOf(Currency("")) {
		return paths
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := jsonFieldName(field)
		if name == "" || name == "-" {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		paths[path] = struct{}{}
		for child := range collectJSONFieldPaths(field.Type, path) {
			paths[child] = struct{}{}
		}
	}
	return paths
}

func collectJSONFieldTypes(typ reflect.Type, prefix string) map[string][]reflect.Type {
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		typ = typ.Elem()
	}
	paths := map[string][]reflect.Type{}
	if typ.Kind() != reflect.Struct || typ == reflect.TypeOf(Currency("")) {
		return paths
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := jsonFieldName(field)
		if name == "" || name == "-" {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		paths[path] = append(paths[path], field.Type)
		for child, childTypes := range collectJSONFieldTypes(field.Type, path) {
			paths[child] = append(paths[child], childTypes...)
		}
	}
	return paths
}

func anyGoTypeMatchesOfficialType(officialType string, goTypes []reflect.Type) bool {
	for _, goType := range goTypes {
		if goTypeMatchesOfficialType(officialType, goType) {
			return true
		}
	}
	return false
}

func goTypeMatchesOfficialType(officialType string, goType reflect.Type) bool {
	officialType = strings.ToLower(strings.TrimSpace(officialType))
	for goType.Kind() == reflect.Pointer {
		goType = goType.Elem()
	}
	switch {
	case strings.HasPrefix(officialType, "string"):
		return goType.Kind() == reflect.String
	case officialType == "integer" || officialType == "int" || officialType == "int64":
		return goType.Kind() >= reflect.Int && goType.Kind() <= reflect.Int64
	case officialType == "boolean":
		return goType.Kind() == reflect.Bool
	case officialType == "object":
		return goType.Kind() == reflect.Struct || goType.Kind() == reflect.Map
	case strings.HasPrefix(officialType, "array"):
		return goType.Kind() == reflect.Slice || goType.Kind() == reflect.Array
	case officialType == "message":
		return (goType.Kind() == reflect.Slice || goType.Kind() == reflect.Array) && goType.Elem().Kind() == reflect.Uint8
	default:
		return false
	}
}

func formatReflectTypes(types []reflect.Type) string {
	names := make([]string, 0, len(types))
	for _, typ := range types {
		names = append(names, typ.String())
	}
	sort.Strings(names)
	return strings.Join(names, "|")
}

func jsonFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" {
		return field.Name
	}
	name, _, _ := strings.Cut(tag, ",")
	return name
}

func ordinaryContractTypesByName() map[string]reflect.Type {
	types := []reflect.Type{
		reflect.TypeOf((*NoRequestBody)(nil)).Elem(),
		reflect.TypeOf((*NoResponseBody)(nil)).Elem(),
		reflect.TypeOf((*ApplymentSubmitRequest)(nil)).Elem(),
		reflect.TypeOf((*ApplymentSubmitResponse)(nil)).Elem(),
		reflect.TypeOf((*ApplymentQueryByIDRequest)(nil)).Elem(),
		reflect.TypeOf((*ApplymentQueryByBusinessCodeRequest)(nil)).Elem(),
		reflect.TypeOf((*ApplymentQueryResponse)(nil)).Elem(),
		reflect.TypeOf((*SettlementQueryRequest)(nil)).Elem(),
		reflect.TypeOf((*SettlementQueryResponse)(nil)).Elem(),
		reflect.TypeOf((*SettlementModifyRequest)(nil)).Elem(),
		reflect.TypeOf((*SettlementModifyResponse)(nil)).Elem(),
		reflect.TypeOf((*SettlementModificationQueryRequest)(nil)).Elem(),
		reflect.TypeOf((*SettlementModificationQueryResponse)(nil)).Elem(),
		reflect.TypeOf((*MerchantLimitationQueryRequest)(nil)).Elem(),
		reflect.TypeOf((*MerchantLimitationQueryResponse)(nil)).Elem(),
		reflect.TypeOf((*ViolationNotificationConfigRequest)(nil)).Elem(),
		reflect.TypeOf((*ViolationNotificationConfigResponse)(nil)).Elem(),
		reflect.TypeOf((*InactiveMerchantIdentityVerificationCreateRequest)(nil)).Elem(),
		reflect.TypeOf((*InactiveMerchantIdentityVerificationCreateResponse)(nil)).Elem(),
		reflect.TypeOf((*InactiveMerchantIdentityVerificationQueryRequest)(nil)).Elem(),
		reflect.TypeOf((*InactiveMerchantIdentityVerificationQueryResponse)(nil)).Elem(),
		reflect.TypeOf((*MediaUploadRequestMultipart)(nil)).Elem(),
		reflect.TypeOf((*MediaUploadResponse)(nil)).Elem(),
		reflect.TypeOf((*NotificationRequest)(nil)).Elem(),
		reflect.TypeOf((*NotificationResource)(nil)).Elem(),
		reflect.TypeOf((*WechatErrorResponse)(nil)).Elem(),
		reflect.TypeOf((*MerchantViolationNotificationPayload)(nil)).Elem(),
		reflect.TypeOf((*PaymentPrepayRequest)(nil)).Elem(),
		reflect.TypeOf((*PaymentPrepayResponse)(nil)).Elem(),
		reflect.TypeOf((*PaymentQueryRequest)(nil)).Elem(),
		reflect.TypeOf((*PaymentQueryResponse)(nil)).Elem(),
		reflect.TypeOf((*PaymentNotificationPayload)(nil)).Elem(),
		reflect.TypeOf((*PaymentCloseRequest)(nil)).Elem(),
		reflect.TypeOf((*JSAPIPayParams)(nil)).Elem(),
		reflect.TypeOf((*CombinePrepayRequest)(nil)).Elem(),
		reflect.TypeOf((*CombinePrepayResponse)(nil)).Elem(),
		reflect.TypeOf((*CombineQueryRequest)(nil)).Elem(),
		reflect.TypeOf((*CombineQueryResponse)(nil)).Elem(),
		reflect.TypeOf((*CombinePaymentNotificationPayload)(nil)).Elem(),
		reflect.TypeOf((*CombineCloseRequest)(nil)).Elem(),
		reflect.TypeOf((*RefundCreateRequest)(nil)).Elem(),
		reflect.TypeOf((*RefundQueryRequest)(nil)).Elem(),
		reflect.TypeOf((*RefundResponse)(nil)).Elem(),
		reflect.TypeOf((*RefundNotificationPayload)(nil)).Elem(),
		reflect.TypeOf((*ProfitSharingReceiverAddRequest)(nil)).Elem(),
		reflect.TypeOf((*ProfitSharingReceiverDeleteRequest)(nil)).Elem(),
		reflect.TypeOf((*ProfitSharingReceiverResponse)(nil)).Elem(),
		reflect.TypeOf((*ProfitSharingOrderRequest)(nil)).Elem(),
		reflect.TypeOf((*ProfitSharingQueryRequest)(nil)).Elem(),
		reflect.TypeOf((*ProfitSharingOrderResponse)(nil)).Elem(),
		reflect.TypeOf((*ProfitSharingReturnRequest)(nil)).Elem(),
		reflect.TypeOf((*ProfitSharingReturnQueryRequest)(nil)).Elem(),
		reflect.TypeOf((*ProfitSharingReturnResponse)(nil)).Elem(),
		reflect.TypeOf((*ProfitSharingUnfreezeRequest)(nil)).Elem(),
		reflect.TypeOf((*ProfitSharingUnfreezeResponse)(nil)).Elem(),
		reflect.TypeOf((*ProfitSharingRemainingAmountRequest)(nil)).Elem(),
		reflect.TypeOf((*ProfitSharingRemainingAmountResponse)(nil)).Elem(),
		reflect.TypeOf((*ProfitSharingNotificationPayload)(nil)).Elem(),
	}

	byName := make(map[string]reflect.Type, len(types))
	for _, typ := range types {
		byName[typ.Name()] = typ
	}
	return byName
}

func ordinaryEndpointErrorCodeSetNames() map[string]struct{} {
	sets := []osperrorcodes.DocumentedCodeSet{
		osperrorcodes.ApplymentSubmitDocumentedCodes,
		osperrorcodes.ApplymentQueryDocumentedCodes,
		osperrorcodes.SettlementModifyDocumentedCodes,
		osperrorcodes.SettlementQueryDocumentedCodes,
		osperrorcodes.SettlementModificationQueryDocumentedCodes,
		osperrorcodes.MerchantMediaUploadDocumentedCodes,
		osperrorcodes.ViolationNotificationConfigQueryDocumentedCodes,
		osperrorcodes.ViolationNotificationConfigUpdateDocumentedCodes,
		osperrorcodes.ViolationNotificationConfigCreateDocumentedCodes,
		osperrorcodes.ViolationNotificationConfigDeleteDocumentedCodes,
		osperrorcodes.MerchantLimitationQueryDocumentedCodes,
		osperrorcodes.InactiveMerchantIdentityVerificationCreateDocumentedCodes,
		osperrorcodes.InactiveMerchantIdentityVerificationQueryDocumentedCodes,
		osperrorcodes.PaymentPrepayDocumentedCodes,
		osperrorcodes.PaymentQueryDocumentedCodes,
		osperrorcodes.PaymentCloseDocumentedCodes,
		osperrorcodes.RefundCreateDocumentedCodes,
		osperrorcodes.RefundQueryDocumentedCodes,
		osperrorcodes.CombinePrepayDocumentedCodes,
		osperrorcodes.CombineQueryDocumentedCodes,
		osperrorcodes.CombineCloseDocumentedCodes,
		osperrorcodes.ProfitSharingCreateDocumentedCodes,
		osperrorcodes.ProfitSharingQueryDocumentedCodes,
		osperrorcodes.ProfitSharingReturnCreateDocumentedCodes,
		osperrorcodes.ProfitSharingReturnQueryDocumentedCodes,
		osperrorcodes.ProfitSharingUnfreezeDocumentedCodes,
		osperrorcodes.ProfitSharingRemainingAmountDocumentedCodes,
		osperrorcodes.ProfitSharingReceiverAddDocumentedCodes,
		osperrorcodes.ProfitSharingReceiverDeleteDocumentedCodes,
	}
	names := make(map[string]struct{}, len(sets))
	for _, set := range sets {
		names[set.Name] = struct{}{}
	}
	return names
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
