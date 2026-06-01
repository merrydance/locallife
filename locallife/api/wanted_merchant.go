package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
	"github.com/rs/zerolog/log"
)

const wantedMerchantDefaultPageSize int32 = 50

type wantedMerchantTextChecker struct {
	client wechatMsgSecChecker
}

type wechatMsgSecChecker interface {
	MsgSecCheck(ctx context.Context, openid string, scene int, content string) error
}

func (c wantedMerchantTextChecker) CheckWantedMerchantText(ctx context.Context, openID string, content string) error {
	if c.client == nil {
		return errors.New("wechat msg sec checker is not configured")
	}
	return c.client.MsgSecCheck(ctx, openID, 2, content)
}

type listWantedMerchantRequest struct {
	RegionID int64 `form:"region_id" binding:"required,min=1"`
	PageID   int32 `form:"page_id" binding:"omitempty,min=1"`
	PageSize int32 `form:"page_size" binding:"omitempty,min=1,max=100"`
}

type createWantedMerchantVoteRequest struct {
	RegionID  int64    `json:"region_id" binding:"required,min=1"`
	Source    string   `json:"source" binding:"omitempty,oneof=manual map"`
	Name      string   `json:"name" binding:"required,min=1,max=80"`
	Address   string   `json:"address" binding:"omitempty,max=200"`
	Latitude  *float64 `json:"latitude" binding:"omitempty"`
	Longitude *float64 `json:"longitude" binding:"omitempty"`
}

type voteExistingWantedMerchantRequest struct {
	RegionID int64 `json:"region_id" binding:"required,min=1"`
}

type wantedMerchantItemResponse struct {
	ID          int64      `json:"id"`
	RegionID    int64      `json:"region_id"`
	Name        string     `json:"name"`
	Address     string     `json:"address,omitempty"`
	Latitude    *float64   `json:"latitude,omitempty"`
	Longitude   *float64   `json:"longitude,omitempty"`
	Source      string     `json:"source"`
	WantCount   int32      `json:"want_count"`
	Rank        int64      `json:"rank"`
	HasVoted    bool       `json:"has_voted"`
	LastVotedAt *time.Time `json:"last_voted_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type wantedMerchantListResponse struct {
	Items      []wantedMerchantItemResponse `json:"items"`
	Total      int64                        `json:"total"`
	PageID     int32                        `json:"page_id"`
	PageSize   int32                        `json:"page_size"`
	TotalPages int32                        `json:"total_pages"`
}

type wantedMerchantVoteResponse struct {
	Result           string `json:"result"`
	WantedMerchantID int64  `json:"wanted_merchant_id,omitempty"`
	MerchantID       int64  `json:"merchant_id,omitempty"`
	Rank             int64  `json:"rank,omitempty"`
	WantCount        int32  `json:"want_count,omitempty"`
}

// listWantedMerchants godoc
// @Summary 获取想吃商户榜单
// @Description 获取当前区县的想吃商户榜单，按总想吃人数实时排序。
// @Tags 想吃商户榜单
// @Accept json
// @Produce json
// @Param region_id query int true "区县ID"
// @Param page_id query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} wantedMerchantListResponse "榜单"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/wanted-merchants [get]
// @Security BearerAuth
func (server *Server) listWantedMerchants(ctx *gin.Context) {
	var req listWantedMerchantRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if req.PageID == 0 {
		req.PageID = 1
	}
	if req.PageSize == 0 {
		req.PageSize = wantedMerchantDefaultPageSize
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	offset := (req.PageID - 1) * req.PageSize
	items, err := server.store.ListWantedMerchantLeaderboard(ctx, db.ListWantedMerchantLeaderboardParams{
		RegionID: req.RegionID,
		UserID:   authPayload.UserID,
		Limit:    req.PageSize,
		Offset:   offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	total, err := server.store.CountActiveWantedMerchantsByRegion(ctx, req.RegionID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	respItems := make([]wantedMerchantItemResponse, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, newWantedMerchantItemResponse(item))
	}
	ctx.JSON(http.StatusOK, wantedMerchantListResponse{
		Items:      respItems,
		Total:      total,
		PageID:     req.PageID,
		PageSize:   req.PageSize,
		TotalPages: int32((total + int64(req.PageSize) - 1) / int64(req.PageSize)),
	})
}

// createWantedMerchantVote godoc
// @Summary 提交想吃商户
// @Description 手动输入或地图选择想吃商户。手动输入会先经过微信文本内容审查；已入驻商户返回 merchant_available；已在榜单返回 found_in_rank。
// @Tags 想吃商户榜单
// @Accept json
// @Produce json
// @Param request body createWantedMerchantVoteRequest true "想吃商户"
// @Success 200 {object} wantedMerchantVoteResponse "提交结果"
// @Failure 400 {object} ErrorResponse "参数错误或内容审查未通过"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 502 {object} ErrorResponse "内容审查服务不可用"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/wanted-merchants/votes [post]
// @Security BearerAuth
func (server *Server) createWantedMerchantVote(ctx *gin.Context) {
	var req createWantedMerchantVoteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	user, err := server.store.GetUser(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrUserNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	service := logic.NewWantedMerchantService(server.store, wantedMerchantTextChecker{client: server.wechatClient})
	result, err := service.SubmitVote(ctx, logic.WantedMerchantVoteInput{
		UserID:       authPayload.UserID,
		WechatOpenID: strings.TrimSpace(user.WechatOpenid),
		RegionID:     req.RegionID,
		Source:       req.Source,
		Name:         req.Name,
		Address:      req.Address,
		Latitude:     optionalNumeric(req.Latitude),
		Longitude:    optionalNumeric(req.Longitude),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	ctx.JSON(http.StatusOK, newWantedMerchantVoteResponse(result))
}

// voteExistingWantedMerchant godoc
// @Summary 给榜单商户 +1
// @Description 给当前区县榜单中的商户 +1；同一用户对同一商户重复点击不会重复计数。
// @Tags 想吃商户榜单
// @Accept json
// @Produce json
// @Param id path int true "榜单商户ID"
// @Param request body voteExistingWantedMerchantRequest true "区县信息"
// @Success 200 {object} wantedMerchantVoteResponse "投票结果"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "榜单商户不存在"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/wanted-merchants/{id}/votes [post]
// @Security BearerAuth
func (server *Server) voteExistingWantedMerchant(ctx *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	var req voteExistingWantedMerchantRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	service := logic.NewWantedMerchantService(server.store, nil)
	result, err := service.VoteExisting(ctx, logic.WantedMerchantExistingVoteInput{
		UserID:           authPayload.UserID,
		RegionID:         req.RegionID,
		WantedMerchantID: uri.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	ctx.JSON(http.StatusOK, newWantedMerchantVoteResponse(result))
}

func newWantedMerchantItemResponse(item db.ListWantedMerchantLeaderboardRow) wantedMerchantItemResponse {
	resp := wantedMerchantItemResponse{
		ID:        item.ID,
		RegionID:  item.RegionID,
		Name:      item.DisplayName,
		Source:    item.Source,
		WantCount: item.WantCount,
		Rank:      item.Rank,
		HasVoted:  item.HasVoted,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
	if item.Address.Valid {
		resp.Address = item.Address.String
	}
	if lat, ok := numericToFloatPtr(item.Latitude); ok {
		resp.Latitude = &lat
	}
	if lng, ok := numericToFloatPtr(item.Longitude); ok {
		resp.Longitude = &lng
	}
	if item.LastVotedAt.Valid {
		resp.LastVotedAt = &item.LastVotedAt.Time
	}
	return resp
}

func newWantedMerchantVoteResponse(result logic.WantedMerchantVoteResult) wantedMerchantVoteResponse {
	resp := wantedMerchantVoteResponse{
		Result:           result.Result,
		WantedMerchantID: result.WantedMerchantID,
		Rank:             result.Rank,
		WantCount:        result.WantCount,
	}
	if result.MerchantID.Valid {
		resp.MerchantID = result.MerchantID.Int64
	}
	return resp
}

func optionalNumeric(value *float64) pgtype.Numeric {
	if value == nil {
		return pgtype.Numeric{}
	}
	return numericFromFloat(*value)
}

func numericToFloatPtr(value pgtype.Numeric) (float64, bool) {
	if !value.Valid {
		return 0, false
	}
	parsed, err := parseNumericToFloat(value)
	if err != nil {
		log.Warn().Err(err).Msg("wanted merchant: failed to parse numeric coordinate")
		return 0, false
	}
	return parsed, true
}
