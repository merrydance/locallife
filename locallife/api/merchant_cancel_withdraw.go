package api

import (
	"context"
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
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/merrydance/locallife/wechat/errorcodes"
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
	errMerchantCancelWithdrawWechatParamError      = errors.New("WeChat rejected the cancel-withdraw request: check sub_mchid, out_request_no, payee info, proof materials, and additional materials before retrying")
	errMerchantCancelWithdrawWechatInvalidRequest  = errors.New("WeChat rejected the cancel-withdraw request in the current state: verify merchant configuration, current cancel-withdraw state, and signed request inputs before retrying")
	errMerchantCancelWithdrawWechatNoAuth          = ErrMerchantCancelWithdrawWechatNoAuth
	errMerchantCancelWithdrawWechatSignError       = ErrMerchantCancelWithdrawSignError
	errMerchantCancelWithdrawWechatRetryLater      = ErrMerchantCancelWithdrawWechatRetryLater
	errMerchantCancelWithdrawWechatInvalidResponse = ErrMerchantCancelWithdrawWechatInvalidResponse
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
	OutAccountType string `json:"out_account_type" enums:"BASIC_ACCOUNT,OPERATE_ACCOUNT,MARGIN_ACCOUNT,TRADE_FEE_ACCOUNT"`
	Amount         int64  `json:"amount"`
}

type merchantCancelWithdrawBlockReason struct {
	Type        string `json:"type" enums:"CONSUMER_COMPLAINT_UNPROCESSED,HAS_BLOCKING_CONTROL,FUNDS_PENDING_PROCESSING,OTHER_REASON"`
	Description string `json:"description"`
}

type merchantCancelWithdrawEligibilityItem struct {
	SubMchID       string                              `json:"sub_mchid"`
	MerchantState  string                              `json:"merchant_state" enums:"NORMAL,HAS_BEEN_CANCELLED"`
	ValidateResult string                              `json:"validate_result" enums:"ALLOW_CANCEL_WITHDRAW,NOT_ALLOW_CANCEL_WITHDRAW"`
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
	IDDocType          string `json:"id_doc_type,omitempty" enums:"IDENTIFICATION_TYPE_ID_CARD,IDENTIFICATION_TYPE_OVERSEA_PASSPORT,IDENTIFICATION_TYPE_HONGKONG_PASSPORT,IDENTIFICATION_TYPE_MACAO_PASSPORT,IDENTIFICATION_TYPE_TAIWAN_PASSPORT,IDENTIFICATION_TYPE_FOREIGN_RESIDENT,IDENTIFICATION_TYPE_HONGKONG_MACAO_RESIDENT,IDENTIFICATION_TYPE_TAIWAN_RESIDENT"`
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
	AccountType     string                                        `json:"account_type,omitempty" enums:"ACCOUNT_TYPE_CORPORATE,ACCOUNT_TYPE_PERSONAL"`
	BankAccountInfo *merchantCancelWithdrawBankAccountInfoRequest `json:"bank_account_info,omitempty"`
	IdentityInfo    *merchantCancelWithdrawIdentityInfoRequest    `json:"identity_info,omitempty"`
}

type createMerchantCancelWithdrawRequest struct {
	OutRequestNo                     string                                  `json:"out_request_no,omitempty" binding:"omitempty,max=32,alphanum"`
	Withdraw                         string                                  `json:"withdraw" binding:"required,oneof=NOT_APPLY_WITHDRAW APPLY_WITHDRAW" enums:"NOT_APPLY_WITHDRAW,APPLY_WITHDRAW"`
	BusinessLicenseStatusDeclaration string                                  `json:"business_license_status_declaration,omitempty" enums:"ACTIVE,CANCELED,REVOKED"`
	PayeeInfo                        *merchantCancelWithdrawPayeeInfoRequest `json:"payee_info,omitempty"`
	ProofMediaAssetIDs               []int64                                 `json:"proof_media_asset_ids,omitempty"`
	AdditionalMaterialAssetIDs       []int64                                 `json:"additional_material_asset_ids,omitempty"`
	Remark                           string                                  `json:"remark,omitempty" binding:"omitempty,max=32"`
}

type merchantCancelWithdrawAccountWithdrawResult struct {
	OutAccountType   string `json:"out_account_type"`
	PayState         string `json:"pay_state" enums:"PAY_PROCESSING,PAY_SUCCEED,PAY_FAIL,BANK_REFUNDED"`
	StateDescription string `json:"state_description"`
}

type merchantCancelWithdrawItem struct {
	ID                               int64                                         `json:"id"`
	OutRequestNo                     string                                        `json:"out_request_no"`
	ApplymentID                      string                                        `json:"applyment_id,omitempty"`
	SubMchID                         string                                        `json:"sub_mchid"`
	Withdraw                         string                                        `json:"withdraw" enums:"NOT_APPLY_WITHDRAW,APPLY_WITHDRAW"`
	BusinessLicenseStatusDeclaration string                                        `json:"business_license_status_declaration,omitempty" enums:"ACTIVE,CANCELED,REVOKED"`
	Remark                           string                                        `json:"remark,omitempty"`
	LocalSyncState                   string                                        `json:"local_sync_state" enums:"created,submit_succeeded,submit_unknown,sync_failed"`
	CancelState                      string                                        `json:"cancel_state,omitempty" enums:"ACCEPTED,REVIEWING,REJECTED,WAITING_MERCHANT_CONFIRM,REVOKED,SYSTEM_PROCESSING,CANCELED,FUND_PROCESSING,FINISH"`
	CancelStateDescription           string                                        `json:"cancel_state_description,omitempty"`
	WithdrawState                    string                                        `json:"withdraw_state,omitempty" enums:"WITHDRAW_PROCESSING,WITHDRAW_EXCEPTION,WITHDRAW_SUCCEED"`
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

func merchantCancelWithdrawConflictResponse(err error) ErrorResponse {
	return ErrorResponse{Code: ErrMerchantCancelWithdrawNotEligible.Code, Error: err.Error()}
}

// @Summary 查询商户注销提现资格
// @Description 查询当前商户的收付通账户状态与微信注销提现资格校验结果。若收付通账户未激活，则返回 200 且 eligible=false，不触发微信调用。
// @Tags 商户财务
// @Produce json
// @Success 200 {object} merchantCancelWithdrawEligibilityResponse
// @Failure 401 {object} ErrorResponse "认证失败"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 429 {object} ErrorResponse "微信限频"
// @Failure 500 {object} ErrorResponse "内部错误或微信服务异常"
// @Failure 502 {object} ErrorResponse "微信返回契约不合法"
// @Failure 503 {object} ErrorResponse "服务不可用"
// @Security BearerAuth
// @Router /v1/merchant/finance/account/cancel-withdraw/eligibility [get]
func (server *Server) getMerchantCancelWithdrawEligibility(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		err := errors.New(ErrMerchantCancelWithdrawServiceUnavailable.Message)
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, ErrMerchantCancelWithdrawServiceUnavailable.Message, "merchant cancel withdraw eligibility ecommerce client not configured"))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	_, paymentConfig, accountStatus, statusDesc, err := server.getFinanceViewerPaymentConfigState(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrMerchantNotFound))
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
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, ErrMerchantCancelWithdrawServiceUnavailable.Message, "merchant cancel withdraw eligibility validate cancel withdraw failed"))
		return
	}

	ctx.JSON(http.StatusOK, merchantCancelWithdrawEligibilityResponse{
		AccountStatus: accountStatus,
		StatusDesc:    statusDesc,
		Eligible:      strings.TrimSpace(eligibility.ValidateResult) == "ALLOW_CANCEL_WITHDRAW",
		Eligibility:   toMerchantCancelWithdrawEligibilityItem(eligibility),
	})
}

// @Summary 查询商户注销提现申请列表
// @Description 返回当前商户的注销提现申请列表。若收付通账户未激活，则返回空列表并带 account_status/status_desc。
// @Tags 商户财务
// @Produce json
// @Param page query int false "页码" minimum(1) default(1)
// @Param limit query int false "每页数量" minimum(1) maximum(100) default(20)
// @Success 200 {object} merchantCancelWithdrawListResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "认证失败"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Security BearerAuth
// @Router /v1/merchant/finance/account/cancel-withdraw/applications [get]
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
			ctx.JSON(http.StatusNotFound, errorResponse(ErrMerchantNotFound))
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
		item, err := toMerchantCancelWithdrawItem(row)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("decode merchant cancel withdraw application %d: %w", row.ID, err)))
			return
		}
		items = append(items, item)
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

// @Summary 查询商户注销提现申请详情
// @Description 查询当前商户的注销提现申请详情。若申请尚未终态，服务端会先尝试同步一次微信状态。
// @Tags 商户财务
// @Produce json
// @Param id path int true "本地申请 ID"
// @Success 200 {object} merchantCancelWithdrawItem
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "认证失败"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 404 {object} ErrorResponse "商户或申请不存在"
// @Failure 429 {object} ErrorResponse "微信限频"
// @Failure 500 {object} ErrorResponse "内部错误或微信服务异常"
// @Failure 502 {object} ErrorResponse "微信返回契约不合法"
// @Failure 503 {object} ErrorResponse "服务不可用"
// @Security BearerAuth
// @Router /v1/merchant/finance/account/cancel-withdraw/applications/{id} [get]
func (server *Server) getMerchantCancelWithdrawApplication(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		err := errors.New(ErrMerchantCancelWithdrawServiceUnavailable.Message)
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, ErrMerchantCancelWithdrawServiceUnavailable.Message, "merchant cancel withdraw application ecommerce client not configured"))
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
			ctx.JSON(http.StatusNotFound, errorResponse(ErrMerchantNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	record, err := server.store.GetMerchantCancelWithdrawApplication(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrMerchantCancelWithdrawApplicationNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if record.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(ErrMerchantCancelWithdrawPermissionDenied))
		return
	}

	record = server.syncMerchantCancelWithdrawApplicationIfNeeded(ctx, record)
	item, err := toMerchantCancelWithdrawItem(record)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("decode merchant cancel withdraw application %d: %w", record.ID, err)))
		return
	}
	ctx.JSON(http.StatusOK, item)
}

// @Summary 创建商户注销提现申请
// @Description 创建商户注销提现申请并提交微信注销提现接口。服务端会先校验主体状态、上传证明材料，并按微信错误码返回明确语义，不会把已知微信错误降级为“提交结果未知”。
// @Tags 商户财务
// @Accept json
// @Produce json
// @Param body body createMerchantCancelWithdrawRequest true "创建注销提现申请"
// @Success 200 {object} merchantCancelWithdrawCreateResponse "同 out_request_no 已存在本地申请，返回现有申请"
// @Success 201 {object} merchantCancelWithdrawCreateResponse "创建成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "微信签名失败"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 404 {object} ErrorResponse "商户支付账户、进件记录或申请不存在"
// @Failure 409 {object} ErrorResponse "当前不满足注销提现条件或申请已存在"
// @Failure 422 {object} ErrorResponse "商户支付账户未激活"
// @Failure 429 {object} ErrorResponse "微信限频"
// @Failure 500 {object} ErrorResponse "内部错误或微信服务异常"
// @Failure 502 {object} ErrorResponse "微信返回契约不合法"
// @Failure 503 {object} ErrorResponse "服务不可用或微信要求稍后重试"
// @Security BearerAuth
// @Router /v1/merchant/finance/account/cancel-withdraw/applications [post]
func (server *Server) createMerchantCancelWithdrawApplication(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		err := errors.New(ErrMerchantCancelWithdrawServiceUnavailable.Message)
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, ErrMerchantCancelWithdrawServiceUnavailable.Message, "merchant cancel withdraw create ecommerce client not configured"))
		return
	}
	if server.mediaStorage == nil {
		err := errors.New(ErrMerchantCancelWithdrawServiceUnavailable.Message)
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, ErrMerchantCancelWithdrawServiceUnavailable.Message, "merchant cancel withdraw create media storage not configured"))
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
			ctx.JSON(http.StatusNotFound, errorResponse(ErrMerchantCancelWithdrawPaymentConfigNotFound))
			return
		}
		if errors.Is(err, errMerchantOwnerRequired) {
			ctx.JSON(http.StatusForbidden, errorResponse(ErrPermissionDenied))
			return
		}
		if err.Error() == "merchant payment config is not active" {
			ctx.JSON(http.StatusUnprocessableEntity, errorResponse(ErrMerchantCancelWithdrawAccountInactive))
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
			ctx.JSON(http.StatusNotFound, errorResponse(ErrMerchantCancelWithdrawApplymentNotFound))
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
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, ErrMerchantCancelWithdrawServiceUnavailable.Message, "merchant cancel withdraw create validate cancel withdraw failed"))
		return
	}
	if strings.TrimSpace(eligibility.ValidateResult) != "ALLOW_CANCEL_WITHDRAW" {
		ctx.JSON(http.StatusConflict, merchantCancelWithdrawConflictResponse(merchantCancelWithdrawEligibilityBlockedError(eligibility)))
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
			ctx.JSON(http.StatusConflict, errorResponse(ErrMerchantCancelWithdrawApplicationExists))
			return
		}
		existing = server.syncMerchantCancelWithdrawApplicationIfNeeded(ctx, existing)
		item, err := toMerchantCancelWithdrawItem(existing)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("decode merchant cancel withdraw application %d: %w", existing.ID, err)))
			return
		}
		ctx.JSON(http.StatusOK, merchantCancelWithdrawCreateResponse{Application: item})
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
			ctx.JSON(http.StatusConflict, errorResponse(ErrMerchantCancelWithdrawApplicationExists))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	createResp, err := server.ecommerceClient.CreateEcommerceCancelWithdraw(ctx, wechatReq)
	if err != nil {
		queryResp, queryErr := server.ecommerceClient.QueryEcommerceCancelWithdrawByOutRequestNo(ctx, outRequestNo)
		if queryErr != nil {
			logMerchantCancelWithdrawSyncFailure(ctx, "query_cancel_withdraw_after_submit_failed", record, queryErr)
			if isMerchantCancelWithdrawSubmitAmbiguous(err) {
				record = server.markMerchantCancelWithdrawSyncState(ctx, record, db.MerchantCancelWithdrawLocalSyncStateSubmitUnknown, logic.MerchantCancelWithdrawSafeErrorMessage(err))
				server.enqueueMerchantCancelWithdrawPolling(ctx, record)
				recordMerchantCancelWithdrawCommandUnknown(ctx, server.store, record, err, queryErr)
			} else {
				record = server.markMerchantCancelWithdrawSyncState(ctx, record, db.MerchantCancelWithdrawLocalSyncStateSyncFailed, logic.MerchantCancelWithdrawSafeErrorMessage(err))
				recordMerchantCancelWithdrawCommandRejected(ctx, server.store, record, err)
			}
			if respondMerchantCancelWithdrawWechatError(ctx, "create_cancel_withdraw", merchant.ID, paymentConfig.SubMchID, outRequestNo, err) {
				return
			}
			if respondMerchantCancelWithdrawWechatError(ctx, "query_cancel_withdraw_after_submit", merchant.ID, paymentConfig.SubMchID, outRequestNo, queryErr) {
				return
			}
			ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, queryErr, ErrMerchantCancelWithdrawServiceUnavailable.Message, "merchant cancel withdraw create query after submit failed"))
			return
		}
		createResp = &wechatcontracts.CancelWithdrawCreateResponse{ApplymentID: queryResp.ApplymentID, OutRequestNo: queryResp.OutRequestNo}
	}

	if createResp != nil && strings.TrimSpace(createResp.ApplymentID) != "" {
		record.ApplymentID = optionalText(createResp.ApplymentID)
	}

	queryResp, queryErr := server.queryMerchantCancelWithdrawStatus(ctx, record)
	if queryErr != nil {
		logMerchantCancelWithdrawSyncFailure(ctx, "query_cancel_withdraw_after_submit", record, queryErr)
	} else {
		application, err := server.recordMerchantCancelWithdrawQueryFact(ctx, record, queryResp)
		if err != nil {
			log.Error().Err(err).
				Int64("merchant_cancel_withdraw_application_id", record.ID).
				Str("out_request_no", record.OutRequestNo).
				Msg("record merchant cancel withdraw query fact after submit failed")
		} else {
			preApplyParams, buildErr := logic.BuildMerchantCancelWithdrawSyncParams(record, nil, db.MerchantCancelWithdrawLocalSyncStateSubmitSucceeded, "", true, time.Now())
			if buildErr != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, buildErr))
				return
			}
			if createResp != nil && strings.TrimSpace(createResp.ApplymentID) != "" {
				preApplyParams.ApplymentID = optionalText(createResp.ApplymentID)
			} else if queryResp != nil && strings.TrimSpace(queryResp.ApplymentID) != "" {
				preApplyParams.ApplymentID = optionalText(queryResp.ApplymentID)
			}
			record, err = server.store.UpdateMerchantCancelWithdrawApplicationSync(ctx, preApplyParams)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}

			if updatedRecord, applied, err := server.applyMerchantCancelWithdrawFactApplication(ctx, application); err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			} else if applied {
				record = updatedRecord
			}
		}
	}
	if queryErr != nil {
		params, buildErr := logic.BuildMerchantCancelWithdrawSyncParams(record, queryResp, db.MerchantCancelWithdrawLocalSyncStateSubmitSucceeded, logic.MerchantCancelWithdrawSafeErrorMessage(queryErr), true, time.Now())
		if buildErr != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, buildErr))
			return
		}
		if createResp != nil && strings.TrimSpace(createResp.ApplymentID) != "" {
			params.ApplymentID = optionalText(createResp.ApplymentID)
		}
		record, err = server.store.UpdateMerchantCancelWithdrawApplicationSync(ctx, params)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}
	recordMerchantCancelWithdrawCommandAccepted(ctx, server.store, record)

	server.enqueueMerchantCancelWithdrawPolling(ctx, record)
	item, err := toMerchantCancelWithdrawItem(record)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("decode merchant cancel withdraw application %d: %w", record.ID, err)))
		return
	}
	ctx.JSON(http.StatusCreated, merchantCancelWithdrawCreateResponse{Application: item})
}

func recordMerchantCancelWithdrawCommandAccepted(ctx context.Context, store db.Store, record db.MerchantCancelWithdrawApplication) {
	secondaryKey := stringPtrIfNotEmpty(strings.TrimSpace(record.ApplymentID.String))
	if _, err := logic.NewPaymentCommandService(store).RecordExternalPaymentCommand(ctx, dbMerchantCancelWithdrawCommandInput(
		record,
		db.ExternalPaymentCommandStatusAccepted,
		secondaryKey,
		nil,
		nil,
		merchantCancelWithdrawCommandSnapshot(map[string]string{
			"out_request_no": strings.TrimSpace(record.OutRequestNo),
			"sub_mchid":      strings.TrimSpace(record.SubMchID),
			"applyment_id":   strings.TrimSpace(record.ApplymentID.String),
			"cancel_state":   strings.TrimSpace(record.CancelState.String),
		}),
	)); err != nil {
		log.Warn().Err(err).
			Int64("merchant_cancel_withdraw_application_id", record.ID).
			Str("out_request_no", strings.TrimSpace(record.OutRequestNo)).
			Msg("record merchant cancel withdraw command accepted failed")
	}
}

func recordMerchantCancelWithdrawCommandUnknown(ctx context.Context, store db.Store, record db.MerchantCancelWithdrawApplication, createErr, queryErr error) {
	if record.LocalSyncState != db.MerchantCancelWithdrawLocalSyncStateSubmitUnknown {
		return
	}
	lastErrorCode, lastErrorMessage := merchantCancelWithdrawCommandErrorFields(createErr)
	if _, err := logic.NewPaymentCommandService(store).RecordExternalPaymentCommand(ctx, dbMerchantCancelWithdrawCommandInput(
		record,
		db.ExternalPaymentCommandStatusUnknown,
		nil,
		lastErrorCode,
		lastErrorMessage,
		merchantCancelWithdrawCommandSnapshot(map[string]string{
			"out_request_no":      strings.TrimSpace(record.OutRequestNo),
			"sub_mchid":           strings.TrimSpace(record.SubMchID),
			"error_code":          stringValue(lastErrorCode),
			"error_message":       stringValue(lastErrorMessage),
			"query_error_message": strings.TrimSpace(merchantCancelWithdrawErrorString(queryErr)),
		}),
	)); err != nil {
		log.Warn().Err(err).
			Int64("merchant_cancel_withdraw_application_id", record.ID).
			Str("out_request_no", strings.TrimSpace(record.OutRequestNo)).
			Msg("record merchant cancel withdraw command unknown failed")
	}
}

func recordMerchantCancelWithdrawCommandRejected(ctx context.Context, store db.Store, record db.MerchantCancelWithdrawApplication, createErr error) {
	if record.LocalSyncState != db.MerchantCancelWithdrawLocalSyncStateSyncFailed {
		return
	}
	lastErrorCode, lastErrorMessage := merchantCancelWithdrawCommandErrorFields(createErr)
	if _, err := logic.NewPaymentCommandService(store).RecordExternalPaymentCommand(ctx, dbMerchantCancelWithdrawCommandInput(
		record,
		db.ExternalPaymentCommandStatusRejected,
		nil,
		lastErrorCode,
		lastErrorMessage,
		merchantCancelWithdrawCommandSnapshot(map[string]string{
			"out_request_no": strings.TrimSpace(record.OutRequestNo),
			"sub_mchid":      strings.TrimSpace(record.SubMchID),
			"error_code":     stringValue(lastErrorCode),
			"error_message":  stringValue(lastErrorMessage),
		}),
	)); err != nil {
		log.Warn().Err(err).
			Int64("merchant_cancel_withdraw_application_id", record.ID).
			Str("out_request_no", strings.TrimSpace(record.OutRequestNo)).
			Msg("record merchant cancel withdraw command rejected failed")
	}
}

func dbMerchantCancelWithdrawCommandInput(
	record db.MerchantCancelWithdrawApplication,
	commandStatus string,
	externalSecondaryKey *string,
	lastErrorCode *string,
	lastErrorMessage *string,
	responseSnapshot []byte,
) logic.RecordExternalPaymentCommandInput {
	businessObjectType := "merchant_cancel_withdraw_application"
	businessObjectID := record.ID
	return logic.RecordExternalPaymentCommandInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityCancelWithdraw,
		CommandType:          db.ExternalPaymentCommandTypeCreateCancelWithdraw,
		BusinessOwner:        db.ExternalPaymentBusinessOwnerMerchantFunds,
		BusinessObjectType:   &businessObjectType,
		BusinessObjectID:     &businessObjectID,
		ExternalObjectType:   db.ExternalPaymentObjectCancelWithdraw,
		ExternalObjectKey:    strings.TrimSpace(record.OutRequestNo),
		ExternalSecondaryKey: externalSecondaryKey,
		CommandStatus:        commandStatus,
		LastErrorCode:        lastErrorCode,
		LastErrorMessage:     lastErrorMessage,
		ResponseSnapshot:     responseSnapshot,
	}
}

func merchantCancelWithdrawCommandErrorFields(err error) (*string, *string) {
	if err == nil {
		return nil, nil
	}
	var wxErr *wechat.WechatPayError
	if errors.As(err, &wxErr) {
		return stringPtrIfNotEmpty(strings.TrimSpace(wxErr.Code)), stringPtrIfNotEmpty(strings.TrimSpace(wxErr.Message))
	}
	return nil, stringPtrIfNotEmpty(strings.TrimSpace(err.Error()))
}

func merchantCancelWithdrawCommandSnapshot(values map[string]string) []byte {
	filtered := make(map[string]string, len(values))
	for key, value := range values {
		if strings.TrimSpace(value) != "" {
			filtered[key] = strings.TrimSpace(value)
		}
	}
	if len(filtered) == 0 {
		return []byte(`{}`)
	}
	data, err := json.Marshal(filtered)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}

func merchantCancelWithdrawErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
	application, err := server.recordMerchantCancelWithdrawQueryFact(ctx, record, queryResp)
	if err != nil {
		log.Error().Err(err).
			Int64("merchant_cancel_withdraw_application_id", record.ID).
			Str("out_request_no", record.OutRequestNo).
			Msg("record merchant cancel withdraw query fact for read failed")
		return record
	}
	updated, applied, err := server.applyMerchantCancelWithdrawFactApplication(ctx, application)
	if err != nil {
		logMerchantCancelWithdrawSyncFailure(ctx, "apply_cancel_withdraw_query_fact", record, err)
		return record
	}
	if !applied {
		return record
	}
	return updated
}

func (server *Server) queryMerchantCancelWithdrawStatus(ctx *gin.Context, record db.MerchantCancelWithdrawApplication) (*wechatcontracts.CancelWithdrawQueryResponse, error) {
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
) (*wechatcontracts.CancelWithdrawRequest, error) {
	wechatReq := &wechatcontracts.CancelWithdrawRequest{
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

func (server *Server) encryptMerchantCancelWithdrawPayeeInfo(req *merchantCancelWithdrawPayeeInfoRequest) (*wechatcontracts.CancelWithdrawPayeeInfo, error) {
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

	payee := &wechatcontracts.CancelWithdrawPayeeInfo{
		AccountType: strings.TrimSpace(req.AccountType),
		BankAccountInfo: &wechatcontracts.CancelWithdrawBankAccountInfo{
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
		payee.IdentityInfo = &wechatcontracts.CancelWithdrawIdentityInfo{
			IDDocType:          strings.TrimSpace(req.IdentityInfo.IDDocType),
			IdentificationName: encryptedName,
			IdentificationNo:   encryptedNo,
		}
	}

	return payee, nil
}

func (server *Server) uploadMerchantCancelWithdrawProofMedias(ctx *gin.Context, userID int64, assetIDs []int64) ([]wechatcontracts.CancelWithdrawProofMedia, error) {
	mediaIDs, err := server.uploadMerchantCancelWithdrawMediaAssets(ctx, userID, assetIDs)
	if err != nil {
		return nil, err
	}
	items := make([]wechatcontracts.CancelWithdrawProofMedia, 0, len(mediaIDs))
	for _, mediaID := range mediaIDs {
		items = append(items, wechatcontracts.CancelWithdrawProofMedia{
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

func toMerchantCancelWithdrawEligibilityItem(resp *wechatcontracts.CancelWithdrawEligibilityResponse) *merchantCancelWithdrawEligibilityItem {
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

func merchantCancelWithdrawEligibilityBlockedError(resp *wechatcontracts.CancelWithdrawEligibilityResponse) error {
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

func toMerchantCancelWithdrawItem(record db.MerchantCancelWithdrawApplication) (merchantCancelWithdrawItem, error) {
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
	if len(record.ProofMediaAssetIds) > 0 {
		if err := json.Unmarshal(record.ProofMediaAssetIds, &item.ProofMediaAssetIDs); err != nil {
			return merchantCancelWithdrawItem{}, fmt.Errorf("unmarshal proof_media_asset_ids: %w", err)
		}
	}
	if len(record.AdditionalMaterialAssetIds) > 0 {
		if err := json.Unmarshal(record.AdditionalMaterialAssetIds, &item.AdditionalMaterialAssetIDs); err != nil {
			return merchantCancelWithdrawItem{}, fmt.Errorf("unmarshal additional_material_asset_ids: %w", err)
		}
	}
	if len(record.AccountInfo) > 0 {
		if err := json.Unmarshal(record.AccountInfo, &item.AccountInfo); err != nil {
			return merchantCancelWithdrawItem{}, fmt.Errorf("unmarshal account_info: %w", err)
		}
	}
	if len(record.AccountWithdrawResult) > 0 {
		if err := json.Unmarshal(record.AccountWithdrawResult, &item.AccountWithdrawResult); err != nil {
			return merchantCancelWithdrawItem{}, fmt.Errorf("unmarshal account_withdraw_result: %w", err)
		}
	}
	return item, nil
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
	switch errorcodes.CanonicalCancelWithdrawCode(wxErr.Code) {
	case errorcodes.CancelWithdrawCodeAlreadyExists, errorcodes.CancelWithdrawCodeBizErrNeedRetry, errorcodes.CancelWithdrawCodeSystemError:
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
		ctx.JSON(http.StatusServiceUnavailable, attachedServerError(ctx, err, ErrMerchantCancelWithdrawServiceUnavailable.Message))
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
		canonicalCode := merchantCancelWithdrawCanonicalWechatCode(operation, wxErr.Code)
		evt := log.Error()
		if wxErr.StatusCode < http.StatusInternalServerError && canonicalCode != errorcodes.CancelWithdrawCodeSignError && canonicalCode != errorcodes.ApplymentCodeSignError {
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

		writeClientError := func(status int, responseErr error) {
			_ = ctx.Error(err)
			ctx.JSON(status, errorResponse(responseErr))
		}

		if !merchantCancelWithdrawWechatCodeIsAccepted(operation, canonicalCode) {
			log.Error().
				Err(err).
				Str("request_id", GetRequestID(ctx)).
				Str("operation", operation).
				Int64("merchant_id", merchantID).
				Str("sub_mchid", strings.TrimSpace(subMchID)).
				Str("out_request_no", strings.TrimSpace(outRequestNo)).
				Str("wechat_error_code", canonicalCode).
				Msg("wechat merchant cancel withdraw returned undocumented error code")
			writeClientError(http.StatusBadGateway, errMerchantCancelWithdrawWechatInvalidResponse)
			return true
		}

		switch canonicalCode {
		case errorcodes.CancelWithdrawCodeParamError:
			writeClientError(http.StatusBadRequest, errMerchantCancelWithdrawWechatParamError)
		case errorcodes.CancelWithdrawCodeInvalidRequest:
			writeClientError(http.StatusBadRequest, errMerchantCancelWithdrawWechatInvalidRequest)
		case errorcodes.CancelWithdrawCodeNoAuth:
			writeClientError(http.StatusForbidden, errMerchantCancelWithdrawWechatNoAuth)
		case errorcodes.CancelWithdrawCodeSignError:
			writeClientError(http.StatusUnauthorized, errMerchantCancelWithdrawWechatSignError)
		case errorcodes.CancelWithdrawCodeAlreadyExists:
			writeClientError(http.StatusConflict, ErrMerchantCancelWithdrawApplicationExists)
		case errorcodes.CancelWithdrawCodeBizErrNeedRetry:
			writeClientError(http.StatusServiceUnavailable, errMerchantCancelWithdrawWechatRetryLater)
		case errorcodes.CancelWithdrawCodeRateLimitExceeded,
			errorcodes.CancelWithdrawCodeFrequencyLimited,
			errorcodes.CancelWithdrawCodeFrequencyLimit,
			errorcodes.ApplymentCodeFrequencyLimitExceed:
			writeClientError(http.StatusTooManyRequests, ErrMerchantCancelWithdrawWechatFrequencyLimit)
		case errorcodes.CancelWithdrawCodeSystemError:
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		default:
			if wxErr.StatusCode >= http.StatusInternalServerError {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			} else {
				writeClientError(http.StatusBadGateway, errMerchantCancelWithdrawWechatInvalidResponse)
			}
		}
		return true
	}

	var contractErr *wechatcontracts.CancelWithdrawQueryContractError
	if errors.As(err, &contractErr) {
		log.Error().
			Err(err).
			Str("request_id", GetRequestID(ctx)).
			Str("operation", operation).
			Int64("merchant_id", merchantID).
			Str("sub_mchid", strings.TrimSpace(subMchID)).
			Str("out_request_no", strings.TrimSpace(outRequestNo)).
			Msg("wechat merchant cancel withdraw response contract validation failed")
		ctx.JSON(http.StatusBadGateway, attachedServerError(ctx, err, errMerchantCancelWithdrawWechatInvalidResponse.Message))
		return true
	}

	var validationErr *wechatcontracts.CancelWithdrawRequestValidationError
	if errors.As(err, &validationErr) {
		log.Error().
			Err(err).
			Str("request_id", GetRequestID(ctx)).
			Str("operation", operation).
			Int64("merchant_id", merchantID).
			Str("sub_mchid", strings.TrimSpace(subMchID)).
			Str("out_request_no", strings.TrimSpace(outRequestNo)).
			Msg("merchant cancel withdraw request failed local contract validation before upstream call")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return true
	}

	return false
}

func merchantCancelWithdrawCanonicalWechatCode(operation string, code string) string {
	if operation == "prepare_cancel_withdraw_request" {
		return errorcodes.CanonicalApplymentCode(code)
	}
	return errorcodes.CanonicalCancelWithdrawCode(code)
}

func merchantCancelWithdrawWechatCodeIsAccepted(operation string, code string) bool {
	switch operation {
	case "validate_cancel_withdraw":
		return errorcodes.EcommerceCancelWithdrawValidateDocumentedCodes.Has(code)
	case "create_cancel_withdraw":
		return errorcodes.EcommerceCancelWithdrawCreateDocumentedCodes.Has(code)
	case "prepare_cancel_withdraw_request":
		return errorcodes.MerchantMediaUploadDocumentedCodes.Has(code)
	default:
		if strings.Contains(operation, "query_cancel_withdraw") {
			return errorcodes.EcommerceCancelWithdrawQueryDocumentedCodes.Has(code)
		}
		return true
	}
}
