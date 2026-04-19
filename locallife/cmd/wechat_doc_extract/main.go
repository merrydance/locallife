package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/merrydance/locallife/internal/wechatdoc"
)

type outputEnvelope struct {
	Extraction     *wechatdoc.Extraction     `json:"extraction"`
	AlignmentAudit *wechatdoc.AlignmentAudit `json:"alignment_audit,omitempty"`
}

func main() {
	docPath := flag.String("doc", "", "Path to the markdown document to extract")
	outPath := flag.String("out", "", "Optional JSON output path; defaults to stdout")
	auditScope := flag.String("audit", "", "Optional audit scope; supported values: applyment, ordering, cancel_withdraw, profit_sharing, subsidy, refund, fund_management, complaint, direct_payment, merchant_transfer")
	flag.Parse()

	if *docPath == "" {
		fmt.Fprintln(os.Stderr, "missing required -doc flag")
		os.Exit(2)
	}

	result, err := wechatdoc.ExtractMarkdownFile(*docPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "extract wechat doc failed:", err)
		os.Exit(2)
	}

	var payloadValue any = result
	var audit *wechatdoc.AlignmentAudit
	if scope := normalizeAuditScope(*auditScope); scope != "" {
		envelope := outputEnvelope{Extraction: result}
		switch scope {
		case "applyment":
			audit = wechatdoc.AuditApplymentAlignment(result)
		case "ordering":
			audit = wechatdoc.AuditOrderingAlignment(result)
		case "cancel_withdraw":
			audit = wechatdoc.AuditCancelWithdrawAlignment(result)
		case "profit_sharing":
			audit = wechatdoc.AuditProfitSharingAlignment(result)
		case "subsidy":
			audit = wechatdoc.AuditSubsidyAlignment(result)
		case "refund":
			audit = wechatdoc.AuditRefundAlignment(result)
		case "fund_management":
			audit = wechatdoc.AuditFundManagementAlignment(result)
		case "complaint":
			audit = wechatdoc.AuditComplaintAlignment(result)
		case "direct_payment":
			audit = wechatdoc.AuditDirectPaymentAlignment(result)
		case "merchant_transfer":
			audit = wechatdoc.AuditMerchantTransferAlignment(result)
		default:
			fmt.Fprintln(os.Stderr, "unsupported -audit value:", *auditScope)
			os.Exit(2)
		}
		envelope.AlignmentAudit = audit
		payloadValue = envelope
	}

	payload, err := json.MarshalIndent(payloadValue, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal extraction result failed:", err)
		os.Exit(2)
	}
	payload = append(payload, '\n')

	if *outPath == "" {
		if _, err := os.Stdout.Write(payload); err != nil {
			fmt.Fprintln(os.Stderr, "write extraction result failed:", err)
			os.Exit(2)
		}
	} else {
		if err := os.WriteFile(*outPath, payload, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "write extraction output failed:", err)
			os.Exit(2)
		}
	}

	fmt.Fprintf(os.Stderr,
		"extracted sections=%d endpoints=%d fields=%d enum_sets=%d enum_values=%d error_codes=%d unknown_tables=%d warnings=%d\n",
		result.Summary.SectionCount,
		result.Summary.EndpointCount,
		result.Summary.FieldCount,
		result.Summary.EnumSetCount,
		result.Summary.EnumValueCount,
		result.Summary.ErrorCodeCount,
		result.Summary.UnknownTableCount,
		result.Summary.WarningCount,
	)
	if audit != nil {
		fmt.Fprintf(os.Stderr,
			"audit scope=%s documented_endpoints=%d audited_endpoints=%d missing_endpoints=%d missing_request_fields=%d missing_response_fields=%d missing_request_constraints=%d missing_response_constraints=%d missing_request_enums=%d missing_response_enums=%d missing_error_codes=%d suppressed_request_fields=%d suppressed_response_fields=%d suppressed_request_enums=%d suppressed_response_enums=%d suppressed_error_codes=%d compatibility_endpoints=%d compatibility_error_codes=%d\n",
			audit.Scope,
			audit.Summary.DocumentedEndpointCount,
			audit.Summary.AuditedEndpointCount,
			audit.Summary.MissingEndpointCount,
			audit.Summary.MissingRequestFieldCount,
			audit.Summary.MissingResponseFieldCount,
			audit.Summary.MissingRequestConstraintCount,
			audit.Summary.MissingResponseConstraintCount,
			audit.Summary.MissingRequestEnumCount,
			audit.Summary.MissingResponseEnumCount,
			audit.Summary.MissingErrorCodeCount,
			audit.Summary.SuppressedRequestFieldCount,
			audit.Summary.SuppressedResponseFieldCount,
			audit.Summary.SuppressedRequestEnumCount,
			audit.Summary.SuppressedResponseEnumCount,
			audit.Summary.SuppressedErrorCodeCount,
			audit.Summary.CompatibilityEndpointCount,
			audit.Summary.CompatibilityErrorCodeCount,
		)
	}
}

func normalizeAuditScope(scope string) string {
	return strings.ToLower(strings.TrimSpace(scope))
}
