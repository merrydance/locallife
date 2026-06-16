package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
)

func platformRiderCardFromRow(row db.ListPlatformRiderCardsRow) platformRiderCard {
	return platformRiderCard{
		ID:             row.ID,
		Name:           row.RealName,
		RegionID:       platformRegionIDFromPgInt8(row.RegionID),
		RegionName:     stringFromPgText(row.RegionName),
		Status:         row.Status,
		Active:         activeRiderByPlatformRule(row.AcceptedIn3d),
		ComplaintCount: row.ComplaintCount,
	}
}

func platformComplaintCategoriesFromRiderRows(rows []db.ListPlatformRiderComplaintCategoriesRow) []platformComplaintCategory {
	items := make([]platformComplaintCategory, 0, len(rows))
	for _, row := range rows {
		items = append(items, platformComplaintCategory{Category: row.ClaimType, Count: row.Count})
	}
	return items
}

func platformRiderAgeAndGender(idCardOCR []byte) (*int32, string, error) {
	if len(idCardOCR) == 0 {
		return nil, "", nil
	}

	var raw struct {
		Gender    string `json:"gender"`
		BirthDate string `json:"birth_date"`
		IDNumber  string `json:"id_number"`
	}
	if err := json.Unmarshal(idCardOCR, &raw); err != nil {
		return nil, "", err
	}

	birthDate := strings.TrimSpace(raw.BirthDate)
	if birthDate == "" {
		idNumber := strings.TrimSpace(raw.IDNumber)
		if len(idNumber) >= 14 {
			birthDate = idNumber[6:10] + "-" + idNumber[10:12] + "-" + idNumber[12:14]
		}
	}

	var age *int32
	if birthDate != "" {
		birthday, err := time.Parse("2006-01-02", birthDate)
		if err == nil {
			now := time.Now()
			years := int32(now.Year() - birthday.Year())
			if now.YearDay() < birthday.YearDay() {
				years--
			}
			if years >= 0 {
				age = &years
			}
		}
	}

	return age, strings.TrimSpace(raw.Gender), nil
}

// listPlatformRiders godoc
// @Summary 获取平台骑手列表
// @Description 管理员分页获取平台骑手列表，返回平台管理页使用的骑手卡片信息
// @Tags 平台实体管理
// @Accept json
// @Produce json
// @Param page query int false "页码" minimum(1) default(1)
// @Param limit query int false "每页数量" minimum(1) maximum(100) default(20)
// @Success 200 {object} platformRiderListResponse "骑手列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/admin/riders [get]
// @Security BearerAuth
func (server *Server) listPlatformRiders(ctx *gin.Context) {
	var req platformEntityListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	normalizePlatformPagination(&req)

	rows, err := server.store.ListPlatformRiderCards(ctx, db.ListPlatformRiderCardsParams{
		Limit:  req.Limit,
		Offset: pageOffset(req.Page, req.Limit),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total, err := server.store.CountPlatformRiders(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	riders := make([]platformRiderCard, 0, len(rows))
	for _, row := range rows {
		riders = append(riders, platformRiderCardFromRow(row))
	}

	ctx.JSON(http.StatusOK, platformRiderListResponse{
		Riders:  riders,
		Total:   total,
		Page:    req.Page,
		Limit:   req.Limit,
		HasMore: platformHasMore(req.Page, req.Limit, len(riders), total),
	})
}

func (server *Server) getPlatformRiderDetail(ctx *gin.Context) {
	var uri platformRiderIDRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	row, err := server.store.GetPlatformRiderDetail(ctx, uri.RiderID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRiderNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	categoryRows, err := server.store.ListPlatformRiderComplaintCategories(ctx, uri.RiderID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	age, gender, err := platformRiderAgeAndGender(row.IDCardOcr)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var locationUpdatedAt *time.Time
	if row.LocationUpdatedAt.Valid {
		t := row.LocationUpdatedAt.Time
		locationUpdatedAt = &t
	}

	active := activeRiderByPlatformRule(row.AcceptedIn3d)
	ctx.JSON(http.StatusOK, platformRiderDetailResponse{
		ID:   row.ID,
		Name: row.RealName,
		Basic: platformRiderBasicInfo{
			Name:       row.RealName,
			RegionID:   platformRegionIDFromPgInt8(row.RegionID),
			RegionName: stringFromPgText(row.RegionName),
			Age:        age,
			Gender:     gender,
			Status:     row.Status,
			Active:     active,
		},
		OrderStats: platformOrderStats{
			TotalOrders:     row.TotalOrders,
			TotalIncome:     row.TotalEarnings,
			LastMonthOrders: row.MonthOrders,
			LastMonthIncome: row.MonthIncome,
		},
		Service: platformServiceStats{
			ComplaintCount:      row.ComplaintCount,
			ComplaintCategories: platformComplaintCategoriesFromRiderRows(categoryRows),
		},
		CreatedAt:          row.CreatedAt,
		LocationUpdatedAt:  locationUpdatedAt,
		CanPauseAccepting:  row.Status == db.RiderStatusActive,
		CanResumeAccepting: row.Status == db.RiderStatusSuspended,
	})
}

func (server *Server) updatePlatformRiderAcceptingStatus(ctx *gin.Context, targetStatus string) {
	var uri platformRiderIDRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	rider, err := server.store.GetRider(ctx, uri.RiderID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRiderNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if targetStatus == db.RiderStatusSuspended && rider.Status != db.RiderStatusActive {
		ctx.JSON(http.StatusConflict, errorResponse(errPlatformEntityStatusConflict))
		return
	}
	if targetStatus == db.RiderStatusActive && rider.Status != db.RiderStatusSuspended {
		ctx.JSON(http.StatusConflict, errorResponse(errPlatformEntityStatusConflict))
		return
	}

	updatedRider, err := server.store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{
		ID:     uri.RiderID,
		Status: targetStatus,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, platformRiderStatusResponse{ID: updatedRider.ID, Status: updatedRider.Status})
}

func (server *Server) pausePlatformRiderAccepting(ctx *gin.Context) {
	server.updatePlatformRiderAcceptingStatus(ctx, db.RiderStatusSuspended)
}

func (server *Server) resumePlatformRiderAccepting(ctx *gin.Context) {
	server.updatePlatformRiderAcceptingStatus(ctx, db.RiderStatusActive)
}
