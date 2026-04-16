package wechatdoc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuditApplymentAlignment_FindsMissingEnumsAndMultipartFields(t *testing.T) {
	extraction := &Extraction{
		Sections: []Section{
			{
				Heading: "接口说明",
				Path:    []string{"商户进件组", "提交申请单", "接口说明"},
				Endpoints: []Endpoint{{
					Method: "POST",
					Path:   "/v3/ecommerce/applyments/",
				}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"商户进件组", "提交申请单", "Body 参数"},
				Fields: []Field{
					{Name: "organization_type", EnumValues: []string{"2401", "4", "2"}},
					{Name: "contact_info.contact_type", EnumValues: []string{"65", "66"}},
				},
			},
			{
				Heading:    "业务错误码",
				Path:       []string{"商户进件组", "提交申请单", "业务错误码"},
				ErrorCodes: []ErrorCode{{Code: "RESOURCE_ALREADY_EXISTS"}, {Code: "NO_AUTH"}},
			},
			{
				Heading: "接口说明",
				Path:    []string{"商户进件组", "图片上传", "接口说明"},
				Endpoints: []Endpoint{{
					Method: "POST",
					Path:   "/v3/merchant/media/upload",
				}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"商户进件组", "图片上传", "Body 参数"},
				Fields: []Field{
					{Name: "file"},
					{Name: "meta"},
					{Name: "meta.filename"},
					{Name: "meta.sha256"},
				},
			},
			{
				Heading:    "业务错误码",
				Path:       []string{"商户进件组", "图片上传", "业务错误码"},
				ErrorCodes: []ErrorCode{{Code: "FREQUENCY_LIMIT_EXCEED"}, {Code: "NEW_CODE"}},
			},
		},
	}

	report := AuditApplymentAlignment(extraction)
	require.Equal(t, "applyment", report.Scope)
	require.Equal(t, 2, report.Summary.DocumentedEndpointCount)
	require.Equal(t, 2, report.Summary.AuditedEndpointCount)
	require.Len(t, report.Endpoints, 1)
	require.Len(t, report.SuppressedGaps, 1)

	createAudit := findEndpointAudit(report.Endpoints, "POST", "/v3/ecommerce/applyments/")
	require.Nil(t, createAudit)
	suppressedCreateAudit := findEndpointAudit(report.SuppressedGaps, "POST", "/v3/ecommerce/applyments/")
	require.NotNil(t, suppressedCreateAudit)
	require.Len(t, suppressedCreateAudit.MissingRequestEnums, 1)
	require.Equal(t, "organization_type", suppressedCreateAudit.MissingRequestEnums[0].Field)
	require.Equal(t, []string{"2401"}, suppressedCreateAudit.MissingRequestEnums[0].MissingValues)

	uploadAudit := findEndpointAudit(report.Endpoints, "POST", "/v3/merchant/media/upload")
	require.NotNil(t, uploadAudit)
	require.Empty(t, uploadAudit.MissingRequestFields)
	require.Equal(t, []string{"NEW_CODE"}, uploadAudit.MissingErrorCodes)

	require.Equal(t, 0, report.Summary.MissingRequestFieldCount)
	require.Equal(t, 0, report.Summary.MissingRequestEnumCount)
	require.Equal(t, 1, report.Summary.MissingErrorCodeCount)
	require.Equal(t, 1, report.Summary.SuppressedRequestEnumCount)
}

func TestAuditApplymentAlignment_ReportsMissingEndpoint(t *testing.T) {
	extraction := &Extraction{
		Sections: []Section{
			{
				Heading:   "接口说明",
				Path:      []string{"商户进件组", "未知接口", "接口说明"},
				Endpoints: []Endpoint{{Method: "GET", Path: "/v3/unknown/applyment"}},
			},
			{
				Heading: "请求参数",
				Path:    []string{"商户进件组", "未知接口", "请求参数"},
				Fields:  []Field{{Name: "foo"}},
			},
			{
				Heading: "应答参数",
				Path:    []string{"商户进件组", "未知接口", "应答参数"},
				Fields:  []Field{{Name: "bar"}},
			},
			{
				Heading:    "业务错误码",
				Path:       []string{"商户进件组", "未知接口", "业务错误码"},
				ErrorCodes: []ErrorCode{{Code: "UNKNOWN"}},
			},
		},
	}

	report := AuditApplymentAlignment(extraction)
	require.Len(t, report.Endpoints, 1)
	require.True(t, report.Endpoints[0].MissingEndpoint)
	require.Equal(t, []string{"foo"}, report.Endpoints[0].MissingRequestFields)
	require.Equal(t, []string{"bar"}, report.Endpoints[0].MissingResponseFields)
	require.Equal(t, []string{"UNKNOWN"}, report.Endpoints[0].MissingErrorCodes)
	require.Equal(t, 1, report.Summary.MissingEndpointCount)
}

func TestAuditApplymentAlignment_IgnoresConditionalProseInEnumValues(t *testing.T) {
	extraction := &Extraction{
		Sections: []Section{
			{
				Heading: "接口说明",
				Path:    []string{"商户进件组", "查询申请状态", "接口说明"},
				Endpoints: []Endpoint{{
					Method: "GET",
					Path:   "/v3/ecommerce/applyments/out-request-no/{out_request_no}",
				}},
			},
			{
				Heading: "应答参数",
				Path:    []string{"商户进件组", "查询申请状态", "应答参数"},
				Fields: []Field{
					{Name: "applyment_state", EnumValues: []string{"CHECKING", "FINISH"}},
					{Name: "sign_url", Requirement: "conditional", EnumValues: []string{"为NEED_SIGN时才返回"}},
				},
			},
		},
	}

	report := AuditApplymentAlignment(extraction)
	require.Empty(t, report.Endpoints)
	require.Equal(t, 0, report.Summary.MissingResponseEnumCount)
}

func findEndpointAudit(endpoints []EndpointAlignmentAudit, method, path string) *EndpointAlignmentAudit {
	for index := range endpoints {
		endpoint := &endpoints[index]
		if endpoint.Method == method && endpoint.Path == path {
			return endpoint
		}
	}
	return nil
}
