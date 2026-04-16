package contracts

import "testing"

func TestValidateCancelWithdrawIdentifier_TrimsValue(t *testing.T) {
	trimmed, err := ValidateCancelWithdrawIdentifier("query merchant cancel withdraw", "sub_mchid", "  1900001111  ")
	if err != nil {
		t.Fatalf("expected identifier to be valid, got %v", err)
	}
	if trimmed != "1900001111" {
		t.Fatalf("expected trimmed identifier, got %q", trimmed)
	}
}

func TestValidateCancelWithdrawCreateRequest_RejectsNonAlnumOutRequestNo(t *testing.T) {
	req := validCancelWithdrawRequest()
	req.OutRequestNo = "REQ-001"

	err := ValidateCancelWithdrawCreateRequest(req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if err.Error() != "create merchant cancel withdraw: out_request_no must contain only letters and digits" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCancelWithdrawCreateRequest_NormalizesValidRequest(t *testing.T) {
	req := validCancelWithdrawRequest()

	err := ValidateCancelWithdrawCreateRequest(req)
	if err != nil {
		t.Fatalf("expected request to be valid, got %v", err)
	}
	if req.SubMchID != "1900001111" {
		t.Fatalf("expected trimmed sub_mchid, got %q", req.SubMchID)
	}
	if req.OutRequestNo != "REQ001ABC" {
		t.Fatalf("expected trimmed out_request_no, got %q", req.OutRequestNo)
	}
	if req.Withdraw != CancelWithdrawModeApply {
		t.Fatalf("expected trimmed withdraw mode, got %q", req.Withdraw)
	}
	if req.PayeeInfo == nil || req.PayeeInfo.AccountType != CancelWithdrawAccountTypePersonal {
		t.Fatalf("expected payee_info.account_type to be normalized, got %+v", req.PayeeInfo)
	}
	if req.PayeeInfo.IdentityInfo == nil || req.PayeeInfo.IdentityInfo.IDDocType != CancelWithdrawIDDocTypeOverseaPassport {
		t.Fatalf("expected payee_info.identity_info.id_doc_type to be normalized, got %+v", req.PayeeInfo.IdentityInfo)
	}
	if req.Remark != "keep funds flowing" {
		t.Fatalf("expected trimmed remark, got %q", req.Remark)
	}
}

func TestValidateCancelWithdrawEligibilityResponse_RejectsUnsupportedBlockReasonType(t *testing.T) {
	resp := validCancelWithdrawEligibilityResponse()
	resp.BlockReasons = []CancelWithdrawBlockReason{{Type: "UNSUPPORTED_REASON"}}

	err := ValidateCancelWithdrawEligibilityResponse(resp)
	if err == nil {
		t.Fatal("expected contract error")
	}
	if err.Error() != "validate merchant cancel withdraw: block_reasons[0].type has unsupported value \"UNSUPPORTED_REASON\"" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCancelWithdrawQueryResponse_RejectsWithdrawStateBeforeWithdrawPhase(t *testing.T) {
	resp := validCancelWithdrawQueryResponse()
	resp.CancelState = CancelWithdrawCancelStateReviewing
	resp.WithdrawState = CancelWithdrawWithdrawStateProcessing

	err := ValidateCancelWithdrawQueryResponse("query merchant cancel withdraw", resp)
	if err == nil {
		t.Fatal("expected contract error")
	}
	if err.Error() != "query merchant cancel withdraw: withdraw_state is only allowed after the request reaches a withdraw-processing state" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCancelWithdrawQueryResponse_AcceptsWaitingMerchantConfirmURL(t *testing.T) {
	resp := validCancelWithdrawQueryResponse()
	resp.CancelState = CancelWithdrawCancelStateWaitingMerchantConfirm
	resp.WithdrawState = ""
	resp.ConfirmCancel = &CancelWithdrawConfirmCancel{ConfirmCancelURL: "https://example.com/confirm"}

	err := ValidateCancelWithdrawQueryResponse("query merchant cancel withdraw", resp)
	if err != nil {
		t.Fatalf("expected response to be valid, got %v", err)
	}
}

func validCancelWithdrawRequest() *CancelWithdrawRequest {
	return &CancelWithdrawRequest{
		SubMchID:     " 1900001111 ",
		OutRequestNo: " REQ001ABC ",
		Withdraw:     " " + CancelWithdrawModeApply + " ",
		PayeeInfo: &CancelWithdrawPayeeInfo{
			AccountType: " " + CancelWithdrawAccountTypePersonal + " ",
			IdentityInfo: &CancelWithdrawIdentityInfo{
				IDDocType: " " + CancelWithdrawIDDocTypeOverseaPassport + " ",
			},
		},
		AdditionalMaterials: []string{"media-1", "media-2"},
		Remark:              " keep funds flowing ",
	}
}

func validCancelWithdrawEligibilityResponse() *CancelWithdrawEligibilityResponse {
	return &CancelWithdrawEligibilityResponse{
		SubMchID:       "1900001111",
		MerchantState:  CancelWithdrawMerchantStateNormal,
		ValidateResult: CancelWithdrawValidateResultNotAllow,
		AccountInfo: []CancelWithdrawAccountInfo{{
			OutAccountType: CancelWithdrawOutAccountTypeBasic,
			Amount:         100,
		}},
		BlockReasons: []CancelWithdrawBlockReason{{
			Type:        CancelWithdrawBlockReasonTypeOtherReason,
			Description: "manual review",
		}},
	}
}

func validCancelWithdrawQueryResponse() *CancelWithdrawQueryResponse {
	return &CancelWithdrawQueryResponse{
		ApplymentID:            "20000001",
		OutRequestNo:           "REQ001ABC",
		CancelState:            CancelWithdrawCancelStateFundProcessing,
		CancelStateDescription: "funds are processing",
		Withdraw:               CancelWithdrawModeApply,
		WithdrawState:          CancelWithdrawWithdrawStateProcessing,
		ModifyTime:             "2026-04-16T10:00:00+08:00",
		SubMchID:               "1900001111",
		AccountInfo: []CancelWithdrawAccountInfo{{
			OutAccountType: CancelWithdrawOutAccountTypeBasic,
			Amount:         100,
		}},
		AccountWithdrawResult: []CancelWithdrawAccountWithdrawResult{{
			OutAccountType:   CancelWithdrawOutAccountTypeBasic,
			PayState:         CancelWithdrawPayStateProcessing,
			StateDescription: "processing",
		}},
	}
}
