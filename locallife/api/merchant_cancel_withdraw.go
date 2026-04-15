package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog/log"
)

const merchantCancelWithdrawPollDelay = 15 * time.Second

var merchantCancelWithdrawAllowedAccountTypes = map[string]struct{}{
	"ACCOUNT_TYPE_CORPORATE": {},
	"ACCOUNT_TYPE_PERSONAL":  {},
}

var merchantCancelWithdrawAllowedIDDocTypes = map[string]struct{}{
	"IDENTIFICATION_TYPE_ID_CARD":                 {},
	"IDENTIFICATION_TYPE_OVERSEA_PASSPORT":        {},
	"IDENTIFICATION_TYPE_HONGKONG_PASSPORT":       {},
	"IDENTIFICATION_TYPE_MACAO_PASSPORT":          {},
	"IDENTIFICATION_TYPE_TAIWAN_PASSPORT":         {},
	"IDENTIFICATION_TYPE_FOREIGN_RESIDENT":        {},
	"IDENTIFICATION_TYPE_HONGKONG_MACAO_RESIDENT": {},
	"IDENTIFICATION_TYPE_TAIWAN_RESIDENT":         {},
}

var (
	errMerchantCancelWithdrawWechatParamError         = errors.New("WeChat rejected the cancel-withdraw request: check sub_mchid, out_request_no, payee info, proof materials, and additional materials before retrying")
	errMerchantCancelWithdrawWechatInvalidRequest     = errors.New("WeChat rejected the cancel-withdraw request in the current state: verify merchant configuration, current cancel-withdraw state, and signed request inputs before retrying")
	errMerchantCancelWithdrawWechatNoAuth             = errors.New("WeChat rejected the cancel-withdraw request because the current merchant configuration has no permission to operate on this sub-merchant")
	errMerchantCancelWithdrawWechatSignError          = errors.New("WeChat rejected the cancel-withdraw request because signature verification failed: verify merchant credentials and signing inputs")
	errMerchantCancelWithdrawWechatServiceUnavailable = errors.New("WeChat cancel-withdraw service is temporarily unavailable; retry later")
	errMerchantCancelWithdrawWechatInvalidResponse    = errors.New("WeChat returned a cancel-withdraw response that does not match the documented contract")
)

type merchantCancelWithdrawRequestPreparationValidationError struct {
	Message string
}

func (e *merchantCancelWithdrawRequestPreparationValidationError) Error() string {
	return strings.TrimSpace(e.Message)
}

type merchantCancelWithdrawUpstreamPreparationError struct {
	Operation string
	Err       error
}

func (e *merchantCancelWithdrawUpstreamPreparationError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Operation) == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s: %v", e.Operation, e.Err)
}

func (e *merchantCancelWithdrawUpstreamPreparationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type merchantCancelWithdrawAccountInfo struct {
	OutAccountType string `json:"out_account_type"`
	Amount         int64  `json:"amount"`
}

type merchantCancelWithdrawBlockReason struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type merchantCancelWithdrawEligibilityItem struct {
	SubMchID       string                              `json:"sub_mchid"`
	MerchantState  string                              `json:"merchant_state"`
	ValidateResult string                              `json:"validate_result"`
	AccountInfo    []merchantCancelWithdrawAccountInfo `json:"account_info,omitempty"`
	BlockReasons   []merchantCancelWithdrawBlockReason `json:"block_reasons,omitempty"`
}

type merchantCancelWithdrawEligibilityResponse struct {
	AccountStatus string                                 `json:"account_status"`
	StatusDesc    string                                 `json:"status_desc"`
	Eligible      bool                                   `json:"eligible"`
	Eligibility   *merchantCancelWithdrawEligibilityItem `json:"eligibility,omitempty"`
}

type merchantCancelWithdrawIdentityInfoRequest struct {
	IDDocType          string `json:"id_doc_type,omitempty"`
	IdentificationName string `json:"identification_name,omitempty"`
	IdentificationNo   string `json:"identification_no,omitempty"`
}

type merchantCancelWithdrawBankAccountInfoRequest struct {
	AccountName    string `json:"account_name,omitempty"`
	AccountBank    string `json:"account_bank,omitempty"`
	BankBranchID   string `json:"bank_branch_id,omitempty"`
	BankBranchName string `json:"bank_branch_name,omitempty"`
	AccountNumber  string `json:"account_number,omitempty"`
}

type merchantCancelWithdrawPayeeInfoRequest struct {
	AccountType     string                                        `json:"account_type,omitempty"`
	BankAccountInfo *merchantCancelWithdrawBankAccountInfoRequest `json:"bank_account_info,omitempty"`
	IdentityInfo    *merchantCancelWithdrawIdentityInfoRequest    `json:"identity_info,omitempty"`
}

type createMerchantCancelWithdrawRequest struct {
	OutRequestNo                     string                                  `json:"out_request_no,omitempty" binding:"omitempty,max=32,alphanum"`
	Withdraw                         string                                  `json:"withdraw" binding:"required,oneof=NOT_APPLY_WITHDRAW APPLY_WITHDRAW"`
	BusinessLicenseStatusDeclaration string                                  `json:"business_license_status_declaration,omitempty"`
	PayeeInfo                        *merchantCancelWithdrawPayeeInfoRequest `json:"payee_info,omitempty"`
	ProofMediaAssetIDs               []int64                                 `json:"proof_media_asset_ids,omitempty"`
	AdditionalMaterialAssetIDs       []int64                                 `json:"additional_material_asset_ids,omitempty"`
	Remark                           string                                  `json:"remark,omitempty" binding:"omitempty,max=32"`
}

type merchantCancelWithdrawAccountWithdrawResult struct {
	OutAccountType   string `json:"out_account_type"`
	PayState         string `json:"pay_state"`
	StateDescription string `json:"state_description"`
}

type merchantCancelWithdrawItem struct {
	ID                               int64                                         `json:"id"`
	OutRequestNo                     string                                        `json:"out_request_no"`
	ApplymentID                      string                                        `json:"applyment_id,omitempty"`
	SubMchID                         string                                        `json:"sub_mchid"`
	Withdraw                         string                                        `json:"withdraw"`
	BusinessLicenseStatusDeclaration string                                        `json:"business_license_status_declaration,omitempty"`
	Remark                           string                                        `json:"remark,omitempty"`
	LocalSyncState                   string                                        `json:"local_sync_state"`
	CancelState                      string                                        `json:"cancel_state,omitempty"`
	CancelStateDescription           string                                        `json:"cancel_state_description,omitempty"`
	WithdrawState                    string                                        `json:"withdraw_state,omitempty"`
	WithdrawStateDescription         string                                        `json:"withdraw_state_description,omitempty"`
	ConfirmCancelURL                 string                                        `json:"confirm_cancel_url,omitempty"`
	AccountInfo                      []merchantCancelWithdrawAccountInfo           `json:"account_info,omitempty"`
	AccountWithdrawResult            []merchantCancelWithdrawAccountWithdrawResult `json:"account_withdraw_result,omitempty"`
	ProofMediaAssetIDs               []int64                                       `json:"proof_media_asset_ids,omitempty"`
	AdditionalMaterialAssetIDs       []int64                                       `json:"additional_material_asset_ids,omitempty"`
	LastError                        string                                        `json:"last_error,omitempty"`
	ModifyTime                       string                                        `json:"modify_time,omitempty"`
	SubmittedAt                      string                                        `json:"submitted_at,omitempty"`
	LastQueryAt                      string                                        `json:"last_query_at,omitempty"`
	CreatedAt                        time.Time                                     `json:"created_at"`
	UpdatedAt                        time.Time                                     `json:"updated_at"`
}

type merchantCancelWithdrawListResponse struct {
	Applications  []merchantCancelWithdrawItem `json:"applications"`
	Total         int64                        `json:"total"`
	Page          int32                        `json:"page"`
	Limit         int32                        `json:"limit"`
	TotalPages    int64                        `json:"total_pages"`
	AccountStatus string                       `json:"account_status"`
	StatusDesc    string                       `json:"status_desc"`
}

type merchantCancelWithdrawCreateResponse struct {
	Application merchantCancelWithdrawItem `json:"application"`
}

type listMerchantCancelWithdrawApplicationsRequest struct {
	Page  int32 `form:"page,default=1" binding:"min=1"`
	Limit int32 `form:"limit,default=20" binding:"min=1,max=100"`
}

type getMerchantCancelWithdrawApplicationRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

func (server *Server) getMerchantCancelWithdrawEligibility(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	_, paymentConfig, accountStatus, statusDesc, err := server.getFinanceViewerPaymentConfigState(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if accountStatus != "active" || paymentConfig == nil {
		ctx.JSON(http.StatusOK, merchantCancelWithdrawEligibilityResponse{
			AccountStatus: accountStatus,
			StatusDesc:    statusDesc,
			Eligible:      false,
		})
		return
	}

	eligibility, err := server.ecommerceClient.ValidateEcommerceCancelWithdraw(ctx, paymentConfig.SubMchID)
	if err != nil {
		if respondMerchantCancelWithdrawWechatError(ctx, "validate_cancel_withdraw", 0, paymentConfig.SubMchID, "", err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, merchantCancelWithdrawEligibilityResponse{
		AccountStatus: accountStatus,
		StatusDesc:    statusDesc,
		Eligible:      strings.TrimSpace(eligibility.ValidateResult) == "ALLOW_CANCEL_WITHDRAW",
		Eligibility:   toMerchantCancelWithdrawEligibilityItem(eligibility),
	})
}

func (server *Server) listMerchantCancelWithdrawApplications(ctx *gin.Context) {
	var req listMerchantCancelWithdrawApplicationsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, _, accountStatus, statusDesc, err := server.getFinanceViewerPaymentConfigState(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if accountStatus != "active" {
		ctx.JSON(http.StatusOK, merchantCancelWithdrawListResponse{
			Applications:  []merchantCancelWithdrawItem{},
			Total:         0,
			Page:          req.Page,
			Limit:         req.Limit,
			TotalPages:    0,
			AccountStatus: accountStatus,
			StatusDesc:    statusDesc,
		})
		return
	}

	offset := pageOffset(req.Page, req.Limit)
	rows, err := server.store.ListMerchantCancelWithdrawApplicationsByMerchant(ctx, db.ListMerchantCancelWithdrawApplicationsByMerchantParams{
		MerchantID: merchant.ID,
		Limit:      req.Limit,
		Offset:     offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	totalCount, err := server.store.CountMerchantCancelWithdrawApplicationsByMerchant(ctx, merchant.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	items := make([]merchantCancelWithdrawItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, toMerchantCancelWithdrawItem(row))
	}

	ctx.JSON(http.StatusOK, merchantCancelWithdrawListResponse{
		Applications:  items,
		Total:         totalCount,
		Page:          req.Page,
		Limit:         req.Limit,
		TotalPages:    (totalCount + int64(req.Limit) - 1) / int64(req.Limit),
		AccountStatus: accountStatus,
		StatusDesc:    statusDesc,
	})
}

func (server *Server) getMerchantCancelWithdrawApplication(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	var req getMerchantCancelWithdrawApplicationRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, _, _, _, err := server.getFinanceViewerPaymentConfigState(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	record, err := server.store.GetMerchantCancelWithdrawApplication(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("cancel withdraw application not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if record.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("no permission to access this cancel withdraw application")))
		return
	}

	record = server.syncMerchantCancelWithdrawApplicationIfNeeded(ctx, record)
	ctx.JSON(http.StatusOK, toMerchantCancelWithdrawItem(record))
}

func (server *Server) createMerchantCancelWithdrawApplication(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}
	if server.mediaStorage == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("media storage not configured")))
		return
	}

	var req createMerchantCancelWithdrawRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if err := validateCreateMerchantCancelWithdrawRequest(req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, paymentConfig, err := server.getOwnerMerchantWithActivePaymentConfig(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant or payment config not found")))
			return
		}
		if errors.Is(err, errMerchantOwnerRequired) {
			ctx.JSON(http.StatusForbidden, errorResponse(err))
			return
		}
		if err.Error() == "merchant payment config is not active" {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	applyment, err := server.store.GetLatestEcommerceApplymentBySubject(ctx, db.GetLatestEcommerceApplymentBySubjectParams{
		SubjectType: "merchant",
		SubjectID:   merchant.ID,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("merchant applyment not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	organizationType := strings.TrimSpace(applyment.OrganizationType)
	if !isMerchantApplymentOrganizationTypeSupported(organizationType) {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrMerchantApplymentOrganizationUnsupported))
		return
	}
	if err := logic.ValidateMerchantCancelWithdrawBusinessLicenseDeclaration(organizationType, req.BusinessLicenseStatusDeclaration, len(req.ProofMediaAssetIDs)); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	eligibility, err := server.ecommerceClient.ValidateEcommerceCancelWithdraw(ctx, paymentConfig.SubMchID)
	if err != nil {
		if respondMerchantCancelWithdrawWechatError(ctx, "validate_cancel_withdraw", merchant.ID, paymentConfig.SubMchID, "", err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if strings.TrimSpace(eligibility.ValidateResult) != "ALLOW_CANCEL_WITHDRAW" {
		ctx.JSON(http.StatusConflict, errorResponse(merchantCancelWithdrawEligibilityBlockedError(eligibility)))
		return
	}

	outRequestNo := strings.TrimSpace(req.OutRequestNo)
	if outRequestNo == "" {
		generated, genErr := generateMerchantCancelWithdrawOutRequestNo(merchant.ID)
		if genErr != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, genErr))
			return
		}
		outRequestNo = generated
	}

	existing, err := server.store.GetMerchantCancelWithdrawApplicationByOutRequestNo(ctx, outRequestNo)
	if err == nil {
		if existing.MerchantID != merchant.ID {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("duplicate out_request_no: application already exists")))
			return
		}
		existing = server.syncMerchantCancelWithdrawApplicationIfNeeded(ctx, existing)
		ctx.JSON(http.StatusOK, merchantCancelWithdrawCreateResponse{Application: toMerchantCancelWithdrawItem(existing)})
		return
	}
	if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	proofAssetIDsJSON, err := json.Marshal(req.ProofMediaAssetIDs)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("marshal proof_media_asset_ids: %w", err)))
		return
	}
	additionalAssetIDsJSON, err := json.Marshal(req.AdditionalMaterialAssetIDs)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("marshal additional_material_asset_ids: %w", err)))
		return
	}

	wechatReq, err := server.buildMerchantCancelWithdrawWechatRequest(ctx, authPayload.UserID, paymentConfig.SubMchID, outRequestNo, req)
	if err != nil {
		if respondMerchantCancelWithdrawRequestPreparationError(ctx, merchant.ID, paymentConfig.SubMchID, outRequestNo, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	record, err := server.store.CreateMerchantCancelWithdrawApplication(ctx, db.CreateMerchantCancelWithdrawApplicationParams{
		MerchantID:                       merchant.ID,
		CreatedByUserID:                  authPayload.UserID,
		SubMchID:                         paymentConfig.SubMchID,
		OutRequestNo:                     outRequestNo,
		Withdraw:                         req.Withdraw,
		BusinessLicenseStatusDeclaration: optionalText(req.BusinessLicenseStatusDeclaration),
		ProofMediaAssetIds:               proofAssetIDsJSON,
		AdditionalMaterialAssetIds:       additionalAssetIDsJSON,
		Remark:                           optionalText(req.Remark),
		LocalSyncState:                   db.MerchantCancelWithdrawLocalSyncStateCreated,
	})
	if err != nil {
		if db.ErrorCode(err) == db.UniqueViolation {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("duplicate out_request_no: application already exists")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	createResp, err := server.ecommerceClient.CreateEcommerceCancelWithdraw(ctx, wechatReq)
	if err != nil {
		queryResp, queryErr := server.ecommerceClient.QueryEcommerceCancelWithdrawByOutRequestNo(ctx, outRequestNo)
		if queryErr != nil {
			if isMerchantCancelWithdrawSubmitAmbiguous(err) {
				record = server.markMerchantCancelWithdrawSyncState(ctx, record, db.MerchantCancelWithdrawLocalSyncStateSubmitUnknown, logic.MerchantCancelWithdrawSafeErrorMessage(err))
				server.enqueueMerchantCancelWithdrawPolling(ctx, record)
				ctx.JSON(http.StatusAccepted, merchantCancelWithdrawCreateResponse{Application: toMerchantCancelWithdrawItem(record)})
				return
			}

			record = server.markMerchantCancelWithdrawSyncState(ctx, record, db.MerchantCancelWithdrawLocalSyncStateSyncFailed, logic.MerchantCancelWithdrawSafeErrorMessage(err))
			if respondMerchantCancelWithdrawWechatError(ctx, "create_cancel_withdraw", merchant.ID, paymentConfig.SubMchID, outRequestNo, err) {
				return
			}
			ctx.JSON(http.StatusBadGateway, errorResponse(errMerchantCancelWithdrawWechatServiceUnavailable))
			return
		}
		createResp = &wechat.EcommerceCancelWithdrawCreateResponse{ApplymentID: queryResp.ApplymentID, OutRequestNo: queryResp.OutRequestNo}
	}

	if createResp != nil && strings.TrimSpace(createResp.ApplymentID) != "" {
		record.ApplymentID = optionalText(createResp.ApplymentID)
	}

	queryResp, queryErr := server.queryMerchantCancelWithdrawStatus(ctx, record)
	if queryErr != nil {
		logMerchantCancelWithdrawSyncFailure(ctx, "query_cancel_withdraw_after_submit", record, queryErr)
	}
	params, buildErr := logic.BuildMerchantCancelWithdrawSyncParams(record, queryResp, db.MerchantCancelWithdrawLocalSyncStateSubmitSucceeded, logic.MerchantCancelWithdrawSafeErrorMessage(queryErr), true, time.Now())
	if buildErr != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, buildErr))
		return
	}
	if queryErr == nil && queryResp != nil && strings.TrimSpace(queryResp.ApplymentID) != "" {
		params.ApplymentID = optionalText(queryResp.ApplymentID)
	} else if createResp != nil && strings.TrimSpace(createResp.ApplymentID) != "" {
		params.ApplymentID = optionalText(createResp.ApplymentID)
	}
	record, err = server.store.UpdateMerchantCancelWithdrawApplicationSync(ctx, params)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	server.enqueueMerchantCancelWithdrawPolling(ctx, record)
	ctx.JSON(http.StatusCreated, merchantCancelWithdrawCreateResponse{Application: toMerchantCancelWithdrawItem(record)})
}

func (server *Server) syncMerchantCancelWithdrawApplicationIfNeeded(ctx *gin.Context, record db.MerchantCancelWithdrawApplication) db.MerchantCancelWithdrawApplication {
	if logic.MerchantCancelWithdrawIsTerminal(record.CancelState.String) {
		return record
	}

	queryResp, err := server.queryMerchantCancelWithdrawStatus(ctx, record)
	if err != nil {
		logMerchantCancelWithdrawSyncFailure(ctx, "query_cancel_withdraw_for_read", record, err)
		return record
	}

	params, err := logic.BuildMerchantCancelWithdrawSyncParams(record, queryResp, db.MerchantCancelWithdrawLocalSyncStateSubmitSucceeded, "", false, time.Now())
	if err != nil {
		logMerchantCancelWithdrawSyncFailure(ctx, "build_cancel_withdraw_sync_params", record, err)
		return record
	}
	updated, err := server.store.UpdateMerchantCancelWithdrawApplicationSync(ctx, params)
	if err != nil {
		logMerchantCancelWithdrawSyncFailure(ctx, "update_cancel_withdraw_sync_state", record, err)
		return record
	}
	return updated
}

func (server *Server) queryMerchantCancelWithdrawStatus(ctx *gin.Context, record db.MerchantCancelWithdrawApplication) (*wechat.EcommerceCancelWithdrawQueryResponse, error) {
	if record.ApplymentID.Valid && strings.TrimSpace(record.ApplymentID.String) != "" {
		resp, err := server.ecommerceClient.QueryEcommerceCancelWithdrawByApplymentID(ctx, record.ApplymentID.String)
		if err == nil {
			return resp, nil
		}
		logMerchantCancelWithdrawSyncFailure(ctx, "query_cancel_withdraw_by_applyment_id_fallback", record, err)
	}
	return server.ecommerceClient.QueryEcommerceCancelWithdrawByOutRequestNo(ctx, record.OutRequestNo)
}

func (server *Server) buildMerchantCancelWithdrawWechatRequest(
	ctx *gin.Context,
	userID int64,
	subMchID string,
	outRequestNo string,
	req createMerchantCancelWithdrawRequest,
) (*wechat.EcommerceCancelWithdrawRequest, error) {
	wechatReq := &wechat.EcommerceCancelWithdrawRequest{
		SubMchID:     subMchID,
		OutRequestNo: outRequestNo,
		Withdraw:     req.Withdraw,
		Remark:       strings.TrimSpace(req.Remark),
	}

	if req.Withdraw != db.MerchantCancelWithdrawModeWithdraw {
		return wechatReq, nil
	}

	payeeInfo, err := server.encryptMerchantCancelWithdrawPayeeInfo(req.PayeeInfo)
	if err != nil {
		return nil, err
	}
	wechatReq.PayeeInfo = payeeInfo

	proofMedias, err := server.uploadMerchantCancelWithdrawProofMedias(ctx, userID, req.ProofMediaAssetIDs)
	if err != nil {
		return nil, err
	}
	wechatReq.ProofMedias = proofMedias

	additionalMaterials, err := server.uploadMerchantCancelWithdrawMediaAssets(ctx, userID, req.AdditionalMaterialAssetIDs)
	if err != nil {
		return nil, err
	}
	wechatReq.AdditionalMaterials = additionalMaterials

	return wechatReq, nil
}

func (server *Server) encryptMerchantCancelWithdrawPayeeInfo(req *merchantCancelWithdrawPayeeInfoRequest) (*wechat.EcommerceCancelWithdrawPayeeInfo, error) {
	if req == nil || req.BankAccountInfo == nil {
		return nil, &merchantCancelWithdrawRequestPreparationValidationError{Message: "payee_info is required when withdraw=APPLY_WITHDRAW"}
	}
	if strings.TrimSpace(req.AccountType) == "" {
		return nil, &merchantCancelWithdrawRequestPreparationValidationError{Message: "payee_info.account_type is required"}
	}
	if strings.TrimSpace(req.BankAccountInfo.AccountName) == "" || strings.TrimSpace(req.BankAccountInfo.AccountBank) == "" || strings.TrimSpace(req.BankAccountInfo.AccountNumber) == "" {
		return nil, &merchantCancelWithdrawRequestPreparationValidationError{Message: "payee_info.bank_account_info.account_name/account_bank/account_number are required"}
	}

	encryptedAccountName, err := server.ecommerceClient.EncryptSensitiveData(strings.TrimSpace(req.BankAccountInfo.AccountName))
	if err != nil {
		return nil, fmt.Errorf("encrypt account_name: %w", err)
	}
	encryptedAccountNumber, err := server.ecommerceClient.EncryptSensitiveData(strings.TrimSpace(req.BankAccountInfo.AccountNumber))
	if err != nil {
		return nil, fmt.Errorf("encrypt account_number: %w", err)
	}

	payee := &wechat.EcommerceCancelWithdrawPayeeInfo{
		AccountType: strings.TrimSpace(req.AccountType),
		BankAccountInfo: &wechat.EcommerceCancelWithdrawBankAccountInfo{
			AccountName:    encryptedAccountName,
			AccountBank:    strings.TrimSpace(req.BankAccountInfo.AccountBank),
			BankBranchID:   strings.TrimSpace(req.BankAccountInfo.BankBranchID),
			BankBranchName: strings.TrimSpace(req.BankAccountInfo.BankBranchName),
			AccountNumber:  encryptedAccountNumber,
		},
	}

	if strings.TrimSpace(req.AccountType) == "ACCOUNT_TYPE_PERSONAL" {
		if req.IdentityInfo == nil || strings.TrimSpace(req.IdentityInfo.IDDocType) == "" || strings.TrimSpace(req.IdentityInfo.IdentificationName) == "" || strings.TrimSpace(req.IdentityInfo.IdentificationNo) == "" {
			return nil, &merchantCancelWithdrawRequestPreparationValidationError{Message: "payee_info.identity_info is required for personal account"}
		}
		encryptedName, err := server.ecommerceClient.EncryptSensitiveData(strings.TrimSpace(req.IdentityInfo.IdentificationName))
		if err != nil {
			return nil, fmt.Errorf("encrypt identification_name: %w", err)
		}
		encryptedNo, err := server.ecommerceClient.EncryptSensitiveData(strings.TrimSpace(req.IdentityInfo.IdentificationNo))
		if err != nil {
			return nil, fmt.Errorf("encrypt identification_no: %w", err)
		}
		payee.IdentityInfo = &wechat.EcommerceCancelWithdrawIdentityInfo{
			IDDocType:          strings.TrimSpace(req.IdentityInfo.IDDocType),
			IdentificationName: encryptedName,
			IdentificationNo:   encryptedNo,
		}
	}

	return payee, nil
}

func (server *Server) uploadMerchantCancelWithdrawProofMedias(ctx *gin.Context, userID int64, assetIDs []int64) ([]wechat.EcommerceCancelWithdrawProofMedia, error) {
	mediaIDs, err := server.uploadMerchantCancelWithdrawMediaAssets(ctx, userID, assetIDs)
	if err != nil {
		return nil, err
	}
	items := make([]wechat.EcommerceCancelWithdrawProofMedia, 0, len(mediaIDs))
	for _, mediaID := range mediaIDs {
		items = append(items, wechat.EcommerceCancelWithdrawProofMedia{
			ProofMediaType: "WITHDRAWAL_APPLICATION",
			ProofMedia:     mediaID,
		})
	}
	return items, nil
}

func (server *Server) uploadMerchantCancelWithdrawMediaAssets(ctx *gin.Context, userID int64, assetIDs []int64) ([]string, error) {
	mediaIDs := make([]string, 0, len(assetIDs))
	for _, assetID := range assetIDs {
		asset, err := server.store.GetMediaAssetByID(ctx, assetID)
		if err != nil {
			if isNotFoundError(err) {
				return nil, &merchantCancelWithdrawRequestPreparationValidationError{Message: fmt.Sprintf("media asset %d not found", assetID)}
			}
			return nil, fmt.Errorf("get media asset %d: %w", assetID, err)
		}
		if asset.UploadedBy != userID {
			return nil, &merchantCancelWithdrawRequestPreparationValidationError{Message: fmt.Sprintf("media asset %d is not owned by current user", assetID)}
		}
		if strings.TrimSpace(asset.UploadStatus) != "confirmed" {
			return nil, &merchantCancelWithdrawRequestPreparationValidationError{Message: fmt.Sprintf("media asset %d is not confirmed", assetID)}
		}

		bucket := server.mediaStorage.PrivateBucket()
		if asset.Visibility == string(media.VisibilityPublic) {
			bucket = server.mediaStorage.PublicBucket()
		}

		reader, err := server.mediaStorage.ReadObject(ctx, bucket, asset.ObjectKey)
		if err != nil {
			return nil, fmt.Errorf("read media asset %d: %w", assetID, err)
		}
		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return nil, fmt.Errorf("read media asset bytes %d: %w", assetID, err)
		}

		filename := path.Base(asset.ObjectKey)
		if filename == "." || filename == "/" || filename == "" {
			filename = fmt.Sprintf("asset-%d", assetID)
		}
		uploadResp, err := server.ecommerceClient.UploadImage(ctx, filename, data)
		if err != nil {
			return nil, &merchantCancelWithdrawUpstreamPreparationError{
				Operation: fmt.Sprintf("upload media asset %d to wechat", assetID),
				Err:       err,
			}
		}
		mediaIDs = append(mediaIDs, uploadResp.MediaID)
	}
	return mediaIDs, nil
}

func (server *Server) enqueueMerchantCancelWithdrawPolling(ctx *gin.Context, record db.MerchantCancelWithdrawApplication) {
	if server.taskDistributor == nil {
		return
	}
	_ = server.taskDistributor.DistributeTaskProcessMerchantCancelWithdrawResult(
		ctx,
		&worker.MerchantCancelWithdrawResultPayload{ApplicationID: record.ID, RetryCount: 0},
		asynq.ProcessIn(merchantCancelWithdrawPollDelay),
		asynq.Queue(worker.QueueDefault),
	)
}

func validateCreateMerchantCancelWithdrawRequest(req createMerchantCancelWithdrawRequest) error {
	trimmedRemark := strings.TrimSpace(req.Remark)
	trimmedBusinessLicenseStatusDeclaration := strings.ToUpper(strings.TrimSpace(req.BusinessLicenseStatusDeclaration))
	switch trimmedBusinessLicenseStatusDeclaration {
	case "", db.MerchantCancelWithdrawBusinessLicenseStatusActive, db.MerchantCancelWithdrawBusinessLicenseStatusCanceled, db.MerchantCancelWithdrawBusinessLicenseStatusRevoked:
	default:
		return errors.New("business_license_status_declaration must be ACTIVE, CANCELED, or REVOKED")
	}
	if trimmedRemark != "" && !isMerchantCancelWithdrawRemarkAllowed(trimmedRemark) {
		return errors.New("remark may only contain digits, letters, and Chinese characters")
	}
	if len(req.ProofMediaAssetIDs) > 1 {
		return errors.New("proof_media_asset_ids must not exceed 1 item")
	}
	if len(req.AdditionalMaterialAssetIDs) > 10 {
		return errors.New("additional_material_asset_ids must not exceed 10 items")
	}
	for _, assetID := range req.ProofMediaAssetIDs {
		if assetID <= 0 {
			return errors.New("proof_media_asset_ids must contain positive asset IDs")
		}
	}
	for _, assetID := range req.AdditionalMaterialAssetIDs {
		if assetID <= 0 {
			return errors.New("additional_material_asset_ids must contain positive asset IDs")
		}
	}
	if req.Withdraw == db.MerchantCancelWithdrawModeWithdraw {
		if req.PayeeInfo == nil {
			return errors.New("payee_info is required when withdraw=APPLY_WITHDRAW")
		}
		trimmedAccountType := strings.TrimSpace(req.PayeeInfo.AccountType)
		if _, ok := merchantCancelWithdrawAllowedAccountTypes[trimmedAccountType]; !ok {
			return errors.New("payee_info.account_type must be ACCOUNT_TYPE_CORPORATE or ACCOUNT_TYPE_PERSONAL")
		}
		if trimmedAccountType != "ACCOUNT_TYPE_PERSONAL" && req.PayeeInfo.IdentityInfo != nil {
			return errors.New("payee_info.identity_info is only allowed for personal account")
		}
		if trimmedAccountType == "ACCOUNT_TYPE_PERSONAL" && req.PayeeInfo.IdentityInfo != nil {
			if idDocType := strings.TrimSpace(req.PayeeInfo.IdentityInfo.IDDocType); idDocType != "" {
				if _, ok := merchantCancelWithdrawAllowedIDDocTypes[idDocType]; !ok {
					return errors.New("payee_info.identity_info.id_doc_type is unsupported")
				}
			}
		}
		return nil
	}
	if trimmedBusinessLicenseStatusDeclaration != "" {
		return errors.New("business_license_status_declaration must be empty when withdraw=NOT_APPLY_WITHDRAW")
	}
	if req.PayeeInfo != nil {
		return errors.New("payee_info must be empty when withdraw=NOT_APPLY_WITHDRAW")
	}
	if len(req.ProofMediaAssetIDs) > 0 || len(req.AdditionalMaterialAssetIDs) > 0 {
		return errors.New("proof media assets are only allowed when withdraw=APPLY_WITHDRAW")
	}
	return nil
}

func toMerchantCancelWithdrawEligibilityItem(resp *wechat.EcommerceCancelWithdrawEligibilityResponse) *merchantCancelWithdrawEligibilityItem {
	if resp == nil {
		return nil
	}
	item := &merchantCancelWithdrawEligibilityItem{
		SubMchID:       resp.SubMchID,
		MerchantState:  resp.MerchantState,
		ValidateResult: resp.ValidateResult,
		AccountInfo:    make([]merchantCancelWithdrawAccountInfo, 0, len(resp.AccountInfo)),
		BlockReasons:   make([]merchantCancelWithdrawBlockReason, 0, len(resp.BlockReasons)),
	}
	for _, account := range resp.AccountInfo {
		item.AccountInfo = append(item.AccountInfo, merchantCancelWithdrawAccountInfo{OutAccountType: account.OutAccountType, Amount: account.Amount})
	}
	for _, reason := range resp.BlockReasons {
		item.BlockReasons = append(item.BlockReasons, merchantCancelWithdrawBlockReason{Type: reason.Type, Description: reason.Description})
	}
	return item
}

func merchantCancelWithdrawEligibilityBlockedError(resp *wechat.EcommerceCancelWithdrawEligibilityResponse) error {
	const baseMessage = "merchant is not eligible for cancel withdraw"
	if resp == nil {
		return errors.New(baseMessage)
	}
	reasons := make([]string, 0, len(resp.BlockReasons))
	for _, reason := range resp.BlockReasons {
		description := strings.TrimSpace(reason.Description)
		if description != "" {
			reasons = append(reasons, description)
			continue
		}
		reasonType := strings.TrimSpace(reason.Type)
		if reasonType != "" {
			reasons = append(reasons, reasonType)
		}
	}
	if len(reasons) == 0 {
		return errors.New(baseMessage)
	}
	return fmt.Errorf("%s: %s", baseMessage, strings.Join(reasons, "; "))
}

func toMerchantCancelWithdrawItem(record db.MerchantCancelWithdrawApplication) merchantCancelWithdrawItem {
	item := merchantCancelWithdrawItem{
		ID:                               record.ID,
		OutRequestNo:                     record.OutRequestNo,
		ApplymentID:                      record.ApplymentID.String,
		SubMchID:                         record.SubMchID,
		Withdraw:                         record.Withdraw,
		BusinessLicenseStatusDeclaration: record.BusinessLicenseStatusDeclaration.String,
		Remark:                           record.Remark.String,
		LocalSyncState:                   record.LocalSyncState,
		CancelState:                      record.CancelState.String,
		CancelStateDescription:           record.CancelStateDescription.String,
		WithdrawState:                    record.WithdrawState.String,
		WithdrawStateDescription:         record.WithdrawStateDescription.String,
		ConfirmCancelURL:                 record.ConfirmCancelUrl.String,
		LastError:                        record.LastError.String,
		ModifyTime:                       formatPgTimestamptz(record.ModifyTime),
		SubmittedAt:                      formatPgTimestamptz(record.SubmittedAt),
		LastQueryAt:                      formatPgTimestamptz(record.LastQueryAt),
		CreatedAt:                        record.CreatedAt,
		UpdatedAt:                        record.UpdatedAt,
	}
	_ = json.Unmarshal(record.ProofMediaAssetIds, &item.ProofMediaAssetIDs)
	_ = json.Unmarshal(record.AdditionalMaterialAssetIds, &item.AdditionalMaterialAssetIDs)
	_ = json.Unmarshal(record.AccountInfo, &item.AccountInfo)
	_ = json.Unmarshal(record.AccountWithdrawResult, &item.AccountWithdrawResult)
	return item
}

func formatPgTimestamptz(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.Format(time.RFC3339)
}

func optionalText(value string) pgtype.Text {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: trimmed, Valid: true}
}

func generateMerchantCancelWithdrawOutRequestNo(merchantID int64) (string, error) {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate out_request_no: %w", err)
	}
	return fmt.Sprintf("MCW%d%s", merchantID, hex.EncodeToString(b)), nil
}

func isMerchantCancelWithdrawRemarkAllowed(value string) bool {
	for _, r := range value {
		switch {
		case unicode.IsDigit(r), unicode.IsLetter(r), unicode.Is(unicode.Han, r):
			continue
		default:
			return false
		}
	}
	return true
}

func isMerchantCancelWithdrawSubmitAmbiguous(err error) bool {
	var wxErr *wechat.WechatPayError
	if !errors.As(err, &wxErr) || wxErr == nil {
		return true
	}
	switch strings.TrimSpace(wxErr.Code) {
	case "ALREADY_EXISTS", "BIZ_ERR_NEED_RETRY", "SYSTEM_ERROR":
		return true
	default:
		return false
	}
}

func (server *Server) markMerchantCancelWithdrawSyncState(ctx *gin.Context, record db.MerchantCancelWithdrawApplication, localSyncState string, lastError string) db.MerchantCancelWithdrawApplication {
	params, err := logic.BuildMerchantCancelWithdrawSyncParams(record, nil, localSyncState, lastError, false, time.Now())
	if err != nil {
		logMerchantCancelWithdrawSyncFailure(ctx, "mark_cancel_withdraw_sync_state", record, err)
		return record
	}
	updated, err := server.store.UpdateMerchantCancelWithdrawApplicationSync(ctx, params)
	if err != nil {
		logMerchantCancelWithdrawSyncFailure(ctx, "persist_cancel_withdraw_sync_state", record, err)
		return record
	}
	return updated
}

func respondMerchantCancelWithdrawRequestPreparationError(ctx *gin.Context, merchantID int64, subMchID string, outRequestNo string, err error) bool {
	if respondMerchantCancelWithdrawWechatError(ctx, "prepare_cancel_withdraw_request", merchantID, subMchID, outRequestNo, err) {
		return true
	}

	var validationErr *merchantCancelWithdrawRequestPreparationValidationError
	if errors.As(err, &validationErr) || wechat.IsUploadImageValidationError(err) {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return true
	}

	var upstreamErr *merchantCancelWithdrawUpstreamPreparationError
	if errors.As(err, &upstreamErr) {
		log.Error().
			Err(err).
			Str("request_id", GetRequestID(ctx)).
			Str("operation", strings.TrimSpace(upstreamErr.Operation)).
			Int64("merchant_id", merchantID).
			Str("sub_mchid", strings.TrimSpace(subMchID)).
			Str("out_request_no", strings.TrimSpace(outRequestNo)).
			Msg("merchant cancel withdraw request preparation failed before upstream create call")
		_ = ctx.Error(err)
		ctx.JSON(http.StatusBadGateway, errorResponse(errMerchantCancelWithdrawWechatServiceUnavailable))
		return true
	}

	return false
}

func logMerchantCancelWithdrawSyncFailure(ctx *gin.Context, operation string, record db.MerchantCancelWithdrawApplication, err error) {
	requestID := ""
	if ctx != nil {
		requestID = GetRequestID(ctx)
	}

	evt := log.Error().
		Str("request_id", requestID).
		Str("operation", operation).
		Int64("application_id", record.ID).
		Int64("merchant_id", record.MerchantID).
		Str("sub_mchid", strings.TrimSpace(record.SubMchID)).
		Str("out_request_no", strings.TrimSpace(record.OutRequestNo)).
		Str("applyment_id", strings.TrimSpace(record.ApplymentID.String))

	var wxErr *wechat.WechatPayError
	if errors.As(err, &wxErr) && wxErr != nil {
		if wxErr.StatusCode < http.StatusInternalServerError && strings.TrimSpace(wxErr.Code) != "SIGN_ERROR" {
			evt = log.Warn().
				Str("request_id", requestID).
				Str("operation", operation).
				Int64("application_id", record.ID).
				Int64("merchant_id", record.MerchantID).
				Str("sub_mchid", strings.TrimSpace(record.SubMchID)).
				Str("out_request_no", strings.TrimSpace(record.OutRequestNo)).
				Str("applyment_id", strings.TrimSpace(record.ApplymentID.String))
		}
		evt = evt.
			Int("wechat_status_code", wxErr.StatusCode).
			Str("wechat_error_code", strings.TrimSpace(wxErr.Code)).
			Str("wechat_error_message", strings.TrimSpace(wxErr.Message)).
			Str("wechat_error_detail", strings.TrimSpace(wxErr.Detail))
	}

	evt.Err(err).Msg("merchant cancel withdraw sync failed")
}

func respondMerchantCancelWithdrawWechatError(ctx *gin.Context, operation string, merchantID int64, subMchID string, outRequestNo string, err error) bool {
	var wxErr *wechat.WechatPayError
	if errors.As(err, &wxErr) && wxErr != nil {
		evt := log.Error()
		if wxErr.StatusCode < http.StatusInternalServerError && wxErr.Code != "SIGN_ERROR" {
			evt = log.Warn()
		}
		evt.
			Err(err).
			Str("request_id", GetRequestID(ctx)).
			Str("operation", operation).
			Int64("merchant_id", merchantID).
			Str("sub_mchid", strings.TrimSpace(subMchID)).
			Str("out_request_no", strings.TrimSpace(outRequestNo)).
			Int("wechat_status_code", wxErr.StatusCode).
			Str("wechat_error_code", strings.TrimSpace(wxErr.Code)).
			Str("wechat_error_message", strings.TrimSpace(wxErr.Message)).
			Str("wechat_error_detail", strings.TrimSpace(wxErr.Detail)).
			Msg("wechat merchant cancel withdraw request failed")

		_ = ctx.Error(err)
		switch strings.TrimSpace(wxErr.Code) {
		case "PARAM_ERROR":
			ctx.JSON(http.StatusBadRequest, errorResponse(errMerchantCancelWithdrawWechatParamError))
		case "INVALID_REQUEST":
			ctx.JSON(http.StatusBadRequest, errorResponse(errMerchantCancelWithdrawWechatInvalidRequest))
		case "NO_AUTH":
			ctx.JSON(http.StatusForbidden, errorResponse(errMerchantCancelWithdrawWechatNoAuth))
		case "SIGN_ERROR":
			ctx.JSON(http.StatusUnauthorized, errorResponse(errMerchantCancelWithdrawWechatSignError))
		case "SYSTEM_ERROR":
			ctx.JSON(http.StatusInternalServerError, errorResponse(errMerchantCancelWithdrawWechatServiceUnavailable))
		default:
			if wxErr.StatusCode >= http.StatusInternalServerError {
				ctx.JSON(http.StatusInternalServerError, errorResponse(errMerchantCancelWithdrawWechatServiceUnavailable))
			} else {
				ctx.JSON(http.StatusBadGateway, errorResponse(errMerchantCancelWithdrawWechatInvalidRequest))
			}
		}
		return true
	}

	var contractErr *wechat.MerchantCancelWithdrawContractError
	if errors.As(err, &contractErr) {
		log.Error().
			Err(err).
			Str("request_id", GetRequestID(ctx)).
			Str("operation", operation).
			Int64("merchant_id", merchantID).
			Str("sub_mchid", strings.TrimSpace(subMchID)).
			Str("out_request_no", strings.TrimSpace(outRequestNo)).
			Msg("wechat merchant cancel withdraw response contract validation failed")
		_ = ctx.Error(err)
		ctx.JSON(http.StatusBadGateway, errorResponse(errMerchantCancelWithdrawWechatInvalidResponse))
		return true
	}

	var validationErr *wechat.MerchantCancelWithdrawValidationError
	if errors.As(err, &validationErr) {
		log.Error().
			Err(err).
			Str("request_id", GetRequestID(ctx)).
			Str("operation", operation).
			Int64("merchant_id", merchantID).
			Str("sub_mchid", strings.TrimSpace(subMchID)).
			Str("out_request_no", strings.TrimSpace(outRequestNo)).
			Msg("merchant cancel withdraw request failed local contract validation before upstream call")
		_ = ctx.Error(err)
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return true
	}

	return false
}
