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
		"商户开户意愿确认":            5,
		"商户管控、商户平台处置通知与不活跃核实": 8,
		"小程序支付":               8,
		"小程序合单支付":             8,
		"订单退款":                3,
		"分账":                  9,
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
	if len(officialContractBindings) != 48 {
		t.Fatalf("official contract binding count = %d, want 48", len(officialContractBindings))
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
		"商户开户意愿确认-查询申请单审核结果":      "AccountWillingnessState",
		"商户开户意愿确认-获取商户开户意愿确认状态":   "AccountAuthorizeState",
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

type officialSnapshotEntry struct {
	Title      string   `json:"title"`
	Fields     []string `json:"fields"`
	ErrorCodes []string `json:"error_codes"`
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

func jsonFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" {
		return field.Name
	}
	name, _, _ := strings.Cut(tag, ",")
	return name
}

func ordinaryContractTypeNames() map[string]struct{} {
	names := make(map[string]struct{}, len(ordinaryContractTypesByName()))
	for name := range ordinaryContractTypesByName() {
		names[name] = struct{}{}
	}
	return names
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
		reflect.TypeOf((*AccountWillingnessSubmitRequest)(nil)).Elem(),
		reflect.TypeOf((*AccountWillingnessSubmitResponse)(nil)).Elem(),
		reflect.TypeOf((*AccountWillingnessCancelRequest)(nil)).Elem(),
		reflect.TypeOf((*AccountWillingnessCancelResponse)(nil)).Elem(),
		reflect.TypeOf((*AccountWillingnessQueryRequest)(nil)).Elem(),
		reflect.TypeOf((*AccountWillingnessQueryResponse)(nil)).Elem(),
		reflect.TypeOf((*AccountAuthorizeStateRequest)(nil)).Elem(),
		reflect.TypeOf((*AccountAuthorizeStateResponse)(nil)).Elem(),
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
		reflect.TypeOf((*PaymentCloseRequest)(nil)).Elem(),
		reflect.TypeOf((*JSAPIPayParams)(nil)).Elem(),
		reflect.TypeOf((*CombinePrepayRequest)(nil)).Elem(),
		reflect.TypeOf((*CombinePrepayResponse)(nil)).Elem(),
		reflect.TypeOf((*CombineQueryRequest)(nil)).Elem(),
		reflect.TypeOf((*CombineQueryResponse)(nil)).Elem(),
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
		osperrorcodes.AccountWillingnessSubmitDocumentedCodes,
		osperrorcodes.AccountWillingnessCancelDocumentedCodes,
		osperrorcodes.AccountWillingnessQueryDocumentedCodes,
		osperrorcodes.AccountAuthorizeStateDocumentedCodes,
		osperrorcodes.AccountWillingnessMediaUploadDocumentedCodes,
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
