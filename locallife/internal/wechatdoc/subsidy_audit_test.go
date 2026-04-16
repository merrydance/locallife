package wechatdoc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuditSubsidyAlignment_ReportsMissingSemanticConstraints(t *testing.T) {
	extraction := &Extraction{
		Sections: []Section{
			{
				Heading:   "接口说明",
				Path:      []string{"补差组", "请求补差", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v3/ecommerce/subsidies/create"}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"补差组", "请求补差", "Body 参数"},
				Fields: []Field{
					{Name: "sub_mchid"},
					{Name: "transaction_id"},
					{Name: "amount", Description: "补差金额"},
					{Name: "description"},
					{Name: "out_subsidy_no"},
				},
			},
			{
				Heading: "应答参数",
				Path:    []string{"补差组", "请求补差", "应答参数"},
				Fields: []Field{
					{Name: "sub_mchid"},
					{Name: "transaction_id"},
					{Name: "subsidy_id"},
					{Name: "description"},
					{Name: "amount", Description: "补差金额"},
					{Name: "result", EnumValues: []string{"SUCCESS", "FAIL", "REFUND"}},
					{Name: "success_time", Description: "补差完成时间"},
					{Name: "out_subsidy_no"},
				},
			},
			{
				Heading:    "业务错误码",
				Path:       []string{"补差组", "请求补差", "业务错误码"},
				ErrorCodes: []ErrorCode{{Code: "PARAM_ERROR"}, {Code: "INVALID_REQUEST"}, {Code: "SIGN_ERROR"}, {Code: "SYSTEM_ERROR"}, {Code: "FREQUENCY_LIMITED"}},
			},
		},
	}

	report := AuditSubsidyAlignment(extraction)
	require.Equal(t, "subsidy", report.Scope)
	require.Equal(t, 1, report.Summary.DocumentedEndpointCount)
	require.Equal(t, 1, report.Summary.AuditedEndpointCount)
	require.Len(t, report.Endpoints, 1)

	createAudit := findEndpointAudit(report.Endpoints, "POST", "/v3/ecommerce/subsidies/create")
	require.NotNil(t, createAudit)
	require.Equal(t, []FieldConstraintAudit{{Field: "amount", MissingConstraints: []string{"unit_fen"}}}, createAudit.MissingRequestConstraints)
	require.Equal(t, []FieldConstraintAudit{{Field: "amount", MissingConstraints: []string{"unit_fen"}}, {Field: "success_time", MissingConstraints: []string{"format_rfc3339"}}}, createAudit.MissingResponseConstraints)
}