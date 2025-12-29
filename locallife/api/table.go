package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 桌台管理 ====================

type createTableRequest struct {
	TableNo      string  `json:"table_no" binding:"required,max=50"`
	TableType    string  `json:"table_type" binding:"required,oneof=table room"`
	Capacity     int16   `json:"capacity" binding:"required,min=1,max=100"`
	Description  *string `json:"description" binding:"omitempty,max=500"`
	MinimumSpend *int64  `json:"minimum_spend,omitempty" binding:"omitempty,min=0,max=100000000"`
	QrCodeUrl    *string `json:"qr_code_url,omitempty" binding:"omitempty,url,max=500"`
	TagIds       []int64 `json:"tag_ids,omitempty"` // 标签ID列表
}

type tableResponse struct {
	ID                   int64      `json:"id"`
	MerchantID           int64      `json:"merchant_id"`
	TableNo              string     `json:"table_no"`
	TableType            string     `json:"table_type"`
	Capacity             int16      `json:"capacity"`
	Description          *string    `json:"description,omitempty"`
	MinimumSpend         *int64     `json:"minimum_spend,omitempty"`
	QrCodeUrl            *string    `json:"qr_code_url,omitempty"`
	Status               string     `json:"status"`
	CurrentReservationID *int64     `json:"current_reservation_id,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            *time.Time `json:"updated_at,omitempty"`
	Tags                 []tagInfo  `json:"tags,omitempty"`
}

// tableTagInfo 桌台标签信息（包含类型）
type tableTagInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func newTableResponse(t db.Table) tableResponse {
	resp := tableResponse{
		ID:         t.ID,
		MerchantID: t.MerchantID,
		TableNo:    t.TableNo,
		TableType:  t.TableType,
		Capacity:   t.Capacity,
		Status:     t.Status,
		CreatedAt:  t.CreatedAt,
	}

	if t.Description.Valid {
		resp.Description = &t.Description.String
	}
	if t.MinimumSpend.Valid {
		resp.MinimumSpend = &t.MinimumSpend.Int64
	}
	if t.QrCodeUrl.Valid {
		resp.QrCodeUrl = &t.QrCodeUrl.String
	}
	if t.CurrentReservationID.Valid {
		resp.CurrentReservationID = &t.CurrentReservationID.Int64
	}
	if t.UpdatedAt.Valid {
		resp.UpdatedAt = &t.UpdatedAt.Time
	}

	return resp
}

// createTable godoc
// @Summary 创建桌台/包间
// @Description 商户创建新的桌台或包间
// @Tags 桌台管理
// @Accept json
// @Produce json
// @Param request body createTableRequest true "桌台信息"
// @Success 200 {object} tableResponse "创建成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 409 {object} ErrorResponse "桌号已存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/tables [post]
// @Security BearerAuth
func (server *Server) createTable(ctx *gin.Context) {
	var req createTableRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取商户ID
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查桌号是否重复
	_, err = server.store.GetTableByMerchantAndNo(ctx, db.GetTableByMerchantAndNoParams{
		MerchantID: merchant.ID,
		TableNo:    req.TableNo,
	})
	if err == nil {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("table number already exists")))
		return
	}
	if !errors.Is(err, db.ErrRecordNotFound) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 构建参数
	arg := db.CreateTableParams{
		MerchantID: merchant.ID,
		TableNo:    req.TableNo,
		TableType:  req.TableType,
		Capacity:   req.Capacity,
		Status:     "available",
	}

	if req.Description != nil {
		arg.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.MinimumSpend != nil {
		arg.MinimumSpend = pgtype.Int8{Int64: *req.MinimumSpend, Valid: true}
	}
	if req.QrCodeUrl != nil {
		arg.QrCodeUrl = pgtype.Text{String: *req.QrCodeUrl, Valid: true}
	}

	table, err := server.store.CreateTable(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 处理标签关联
	if len(req.TagIds) > 0 {
		for _, tagID := range req.TagIds {
			_, err = server.store.AddTableTag(ctx, db.AddTableTagParams{
				TableID: table.ID,
				TagID:   tagID,
			})
			if err != nil {
				// 忽略重复或无效的标签ID，继续处理其他标签
				continue
			}
		}
	}

	ctx.JSON(http.StatusOK, newTableResponse(table))
}

type getTableRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// getTable godoc
// @Summary 获取桌台详情
// @Description 商户获取自己的桌台/包间详细信息
// @Tags 桌台管理
// @Produce json
// @Param id path int true "桌台ID"
// @Success 200 {object} tableResponse "桌台详情"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非桌台所有者"
// @Failure 404 {object} ErrorResponse "桌台不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/tables/{id} [get]
// @Security BearerAuth
func (server *Server) getTable(ctx *gin.Context) {
	var req getTableRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	table, err := server.store.GetTable(ctx, req.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("table not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证桌台所有权
	if table.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("table does not belong to your merchant")))
		return
	}

	// 获取标签
	resp := newTableResponse(table)
	tags, err := server.store.ListTableTags(ctx, table.ID)
	if err == nil && len(tags) > 0 {
		tableTagInfos := make([]tableTagInfo, len(tags))
		for i, t := range tags {
			tableTagInfos[i] = tableTagInfo{
				ID:   t.TagID,
				Name: t.TagName,
				Type: t.TagType,
			}
		}
		// 转换为 tagInfo 佛展示
		resp.Tags = make([]tagInfo, len(tableTagInfos))
		for i, ti := range tableTagInfos {
			resp.Tags[i] = tagInfo{ID: ti.ID, Name: ti.Name}
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

type listTablesRequest struct {
	TableType string `form:"table_type" binding:"omitempty,oneof=table room"`
}

type listTablesResponse struct {
	Tables []tableResponse `json:"tables"`
	Count  int64           `json:"count"`
}

// listTables godoc
// @Summary 获取桌台列表
// @Description 商户列出自己的所有桌台/包间
// @Tags 桌台管理
// @Produce json
// @Param table_type query string false "桌台类型" Enums(table, room)
// @Success 200 {object} listTablesResponse "桌台列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/tables [get]
// @Security BearerAuth
func (server *Server) listTables(ctx *gin.Context) {
	var req listTablesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限并获取商户ID
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var tables []db.Table

	if req.TableType != "" {
		tables, err = server.store.ListTablesByMerchantAndType(ctx, db.ListTablesByMerchantAndTypeParams{
			MerchantID: merchant.ID,
			TableType:  req.TableType,
		})
	} else {
		tables, err = server.store.ListTablesByMerchant(ctx, merchant.ID)
	}

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := listTablesResponse{
		Tables: make([]tableResponse, len(tables)),
		Count:  int64(len(tables)),
	}
	for i, t := range tables {
		resp.Tables[i] = newTableResponse(t)

		// 加载每个桌台的标签
		tags, err := server.store.ListTableTags(ctx, t.ID)
		if err == nil && len(tags) > 0 {
			resp.Tables[i].Tags = make([]tagInfo, len(tags))
			for j, tag := range tags {
				resp.Tables[i].Tags[j] = tagInfo{
					ID:   tag.TagID,
					Name: tag.TagName,
				}
			}
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

// listAvailableRooms godoc
// @Summary 获取可用包间列表
// @Description C端用户获取商户的可用包间列表（含主图）
// @Tags 包间浏览
// @Produce json
// @Param id path int true "商户ID"
// @Success 200 {object} listRoomsForCustomerResponse "包间列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/{id}/rooms [get]
func (server *Server) listAvailableRooms(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 使用联合查询获取包间+主图
	rooms, err := server.store.ListAvailableRoomsForCustomer(ctx, uriReq.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := listRoomsForCustomerResponse{
		Rooms: make([]roomListItemResponse, len(rooms)),
		Count: int64(len(rooms)),
	}
	for i, r := range rooms {
		item := roomListItemResponse{
			ID:         r.ID,
			MerchantID: r.MerchantID,
			RoomNo:     r.TableNo,
			Capacity:   r.Capacity,
			Status:     r.Status,
		}
		if r.Description.Valid {
			item.Description = r.Description.String
		}
		if r.MinimumSpend.Valid {
			item.MinimumSpend = r.MinimumSpend.Int64
		}
		if r.PrimaryImage != "" {
			item.PrimaryImage = normalizeUploadURLForClient(r.PrimaryImage)
		}
		resp.Rooms[i] = item
	}

	ctx.JSON(http.StatusOK, resp)
}

// listMerchantRoomsForCustomer godoc
// @Summary 获取商户全部包间列表
// @Description C端用户获取商户的全部包间列表（含主图、月销量，包括不可用的）
// @Tags 包间浏览
// @Produce json
// @Param id path int true "商户ID"
// @Success 200 {object} listRoomsForCustomerResponse "包间列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/{id}/rooms/all [get]
func (server *Server) listMerchantRoomsForCustomer(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 使用联合查询获取所有包间+主图+月销量
	rooms, err := server.store.ListMerchantRoomsForCustomer(ctx, uriReq.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := listRoomsForCustomerResponse{
		Rooms: make([]roomListItemResponse, len(rooms)),
		Count: int64(len(rooms)),
	}
	for i, r := range rooms {
		item := roomListItemResponse{
			ID:                  r.ID,
			MerchantID:          r.MerchantID,
			RoomNo:              r.TableNo,
			Capacity:            r.Capacity,
			Status:              r.Status,
			MonthlyReservations: r.MonthlyReservations,
		}
		if r.Description.Valid {
			item.Description = r.Description.String
		}
		if r.MinimumSpend.Valid {
			item.MinimumSpend = r.MinimumSpend.Int64
		}
		if r.PrimaryImage != "" {
			item.PrimaryImage = normalizeUploadURLForClient(r.PrimaryImage)
		}
		resp.Rooms[i] = item
	}

	ctx.JSON(http.StatusOK, resp)
}

type updateTableRequest struct {
	TableNo      *string `json:"table_no,omitempty" binding:"omitempty,max=50"`
	Capacity     *int16  `json:"capacity,omitempty" binding:"omitempty,min=1,max=100"`
	Description  *string `json:"description,omitempty" binding:"omitempty,max=500"`
	MinimumSpend *int64  `json:"minimum_spend,omitempty" binding:"omitempty,min=0,max=100000000"`
	QrCodeUrl    *string `json:"qr_code_url,omitempty" binding:"omitempty,url,max=500"`
	Status       *string `json:"status,omitempty" binding:"omitempty,oneof=available occupied disabled"`
	TagIds       []int64 `json:"tag_ids,omitempty"` // 标签ID列表
}

// updateTable godoc
// @Summary 更新桌台信息
// @Description 商户更新桌台/包间的信息
// @Tags 桌台管理
// @Accept json
// @Produce json
// @Param id path int true "桌台ID"
// @Param request body updateTableRequest true "更新内容"
// @Success 200 {object} tableResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非桌台所有者"
// @Failure 404 {object} ErrorResponse "桌台不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/tables/{id} [patch]
// @Security BearerAuth
func (server *Server) updateTable(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateTableRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证桌台所有权
	table, err := server.store.GetTable(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("table not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if table.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("table does not belong to your merchant")))
		return
	}

	// 构建更新参数
	arg := db.UpdateTableParams{
		ID: uriReq.ID,
	}

	if req.TableNo != nil {
		arg.TableNo = pgtype.Text{String: *req.TableNo, Valid: true}
	}
	if req.Capacity != nil {
		arg.Capacity = pgtype.Int2{Int16: *req.Capacity, Valid: true}
	}
	if req.Description != nil {
		arg.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.MinimumSpend != nil {
		arg.MinimumSpend = pgtype.Int8{Int64: *req.MinimumSpend, Valid: true}
	}
	if req.QrCodeUrl != nil {
		arg.QrCodeUrl = pgtype.Text{String: *req.QrCodeUrl, Valid: true}
	}
	if req.Status != nil {
		arg.Status = pgtype.Text{String: *req.Status, Valid: true}
	}

	updatedTable, err := server.store.UpdateTable(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 处理标签关联
	if req.TagIds != nil {
		// 删除现有标签关联
		err = server.store.RemoveAllTableTags(ctx, uriReq.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		// 添加新的标签关联
		for _, tagID := range req.TagIds {
			_, err = server.store.AddTableTag(ctx, db.AddTableTagParams{
				TableID: uriReq.ID,
				TagID:   tagID,
			})
			if err != nil {
				// 忽略重复或无效的标签ID
				continue
			}
		}
	}

	ctx.JSON(http.StatusOK, newTableResponse(updatedTable))
}

type updateTableStatusRequest struct {
	Status               string `json:"status" binding:"required,oneof=available occupied disabled"`
	CurrentReservationID *int64 `json:"current_reservation_id,omitempty" binding:"omitempty,min=1"`
}

// updateTableStatus godoc
// @Summary 更新桌台状态
// @Description 商户更新桌台状态（空闲/占用/禁用）
// @Tags 桌台管理
// @Accept json
// @Produce json
// @Param id path int true "桌台ID"
// @Param request body updateTableStatusRequest true "状态更新"
// @Success 200 {object} tableResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非桌台所有者"
// @Failure 404 {object} ErrorResponse "桌台不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/tables/{id}/status [patch]
// @Security BearerAuth
func (server *Server) updateTableStatus(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateTableStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证桌台所有权
	table, err := server.store.GetTable(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("table not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if table.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("table does not belong to your merchant")))
		return
	}

	// 构建更新参数
	var reservationID pgtype.Int8
	if req.CurrentReservationID != nil {
		reservationID = pgtype.Int8{Int64: *req.CurrentReservationID, Valid: true}
	}

	updatedTable, err := server.store.UpdateTableStatus(ctx, db.UpdateTableStatusParams{
		ID:                   uriReq.ID,
		Status:               req.Status,
		CurrentReservationID: reservationID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newTableResponse(updatedTable))
}

type deleteTableRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// deleteTable godoc
// @Summary 删除桌台
// @Description 商户删除桌台/包间（不能有进行中的预定）
// @Tags 桌台管理
// @Produce json
// @Param id path int true "桌台ID"
// @Success 200 {object} map[string]string "message: table deleted successfully"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非桌台所有者"
// @Failure 404 {object} ErrorResponse "桌台不存在"
// @Failure 409 {object} ErrorResponse "有进行中的预定"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/tables/{id} [delete]
// @Security BearerAuth
func (server *Server) deleteTable(ctx *gin.Context) {
	var req deleteTableRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证桌台所有权
	table, err := server.store.GetTable(ctx, req.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("table not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if table.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("table does not belong to your merchant")))
		return
	}

	// 检查是否有当前进行中的预定
	if table.CurrentReservationID.Valid {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("cannot delete table with active reservation")))
		return
	}

	// 使用事务删除桌台（检查未来预定、删除标签、删除桌台）
	_, err = server.store.DeleteTableTx(ctx, db.DeleteTableParams{
		TableID: req.ID,
	})
	if err != nil {
		if err.Error() == "cannot delete table with future reservations" {
			ctx.JSON(http.StatusConflict, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "table deleted successfully"})
}

// ==================== 桌台图片管理 ====================

type tableImageResponse struct {
	ID        int64  `json:"id"`
	TableID   int64  `json:"table_id"`
	ImageURL  string `json:"image_url"`
	SortOrder int32  `json:"sort_order"`
	IsPrimary bool   `json:"is_primary"`
}

type addTableImageRequest struct {
	ImageURL  string `json:"image_url" binding:"required,min=1,max=500"`
	SortOrder int32  `json:"sort_order" binding:"omitempty,min=0,max=100"`
	IsPrimary bool   `json:"is_primary"`
}

// addTableImage godoc
// @Summary 添加桌台图片
// @Description 商户为桌台/包间添加图片
// @Tags 桌台管理
// @Accept json
// @Produce json
// @Param id path int true "桌台ID"
// @Param request body addTableImageRequest true "图片信息"
// @Success 201 {object} tableImageResponse "添加成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非桌台所有者"
// @Failure 404 {object} ErrorResponse "桌台不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/tables/{id}/images [post]
// @Security BearerAuth
func (server *Server) addTableImage(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req addTableImageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证桌台所有权
	table, err := server.store.GetTable(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("table not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if table.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("table does not belong to your merchant")))
		return
	}

	// 桌台/包间图片必须先审后存：仅允许使用 uploads/public/... 本地路径
	normalized := normalizeStoredUploadPath(req.ImageURL)
	prefix := fmt.Sprintf("uploads/public/merchants/%d/tables/", merchant.ID)
	if normalized == "" || !strings.HasPrefix(normalized, prefix) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("image_url 仅允许使用通过桌台图片上传接口生成的本地路径")))
		return
	}
	req.ImageURL = normalized

	// 如果设置为主图，先清除其他主图标记
	if req.IsPrimary {
		err = server.store.SetPrimaryTableImage(ctx, uriReq.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	// 添加图片
	image, err := server.store.AddTableImage(ctx, db.AddTableImageParams{
		TableID:   uriReq.ID,
		ImageUrl:  normalizeImageURLForStorage(req.ImageURL),
		SortOrder: req.SortOrder,
		IsPrimary: req.IsPrimary,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, tableImageResponse{
		ID:        image.ID,
		TableID:   image.TableID,
		ImageURL:  normalizeUploadURLForClient(image.ImageUrl),
		SortOrder: image.SortOrder,
		IsPrimary: image.IsPrimary,
	})
}

// listTableImages godoc
// @Summary 获取桌台图片列表
// @Description 获取桌台/包间的所有图片
// @Tags 桌台管理
// @Produce json
// @Param id path int true "桌台ID"
// @Success 200 {object} map[string][]tableImageResponse "images: 图片列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/tables/{id}/images [get]
// @Security BearerAuth
func (server *Server) listTableImages(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	images, err := server.store.ListTableImages(ctx, uriReq.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]tableImageResponse, len(images))
	for i, img := range images {
		resp[i] = tableImageResponse{
			ID:        img.ID,
			TableID:   img.TableID,
			ImageURL:  normalizeUploadURLForClient(img.ImageUrl),
			SortOrder: img.SortOrder,
			IsPrimary: img.IsPrimary,
		}
	}

	ctx.JSON(http.StatusOK, gin.H{"images": resp})
}

// setTablePrimaryImage godoc
// @Summary 设置桌台主图
// @Description 商户设置桌台/包间的主图
// @Tags 桌台管理
// @Produce json
// @Param id path int true "桌台ID"
// @Param image_id path int true "图片ID"
// @Success 200 {object} tableImageResponse "设置成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非桌台所有者"
// @Failure 404 {object} ErrorResponse "桌台或图片不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/tables/{id}/images/{image_id}/primary [put]
// @Security BearerAuth
func (server *Server) setTablePrimaryImage(ctx *gin.Context) {
	var uriReq struct {
		ID      int64 `uri:"id" binding:"required,min=1"`
		ImageID int64 `uri:"image_id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证桌台所有权
	table, err := server.store.GetTable(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("table not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if table.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("table does not belong to your merchant")))
		return
	}

	// 先清除所有主图标记
	err = server.store.SetPrimaryTableImage(ctx, uriReq.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 设置新主图
	image, err := server.store.SetTableImagePrimary(ctx, uriReq.ImageID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("image not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, tableImageResponse{
		ID:        image.ID,
		TableID:   image.TableID,
		ImageURL:  normalizeUploadURLForClient(image.ImageUrl),
		SortOrder: image.SortOrder,
		IsPrimary: image.IsPrimary,
	})
}

// deleteTableImage godoc
// @Summary 删除桌台图片
// @Description 商户删除桌台/包间的图片
// @Tags 桌台管理
// @Produce json
// @Param id path int true "桌台ID"
// @Param image_id path int true "图片ID"
// @Success 200 {object} map[string]string "message: image deleted successfully"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非桌台所有者"
// @Failure 404 {object} ErrorResponse "桌台不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/tables/{id}/images/{image_id} [delete]
// @Security BearerAuth
func (server *Server) deleteTableImage(ctx *gin.Context) {
	var uriReq struct {
		ID      int64 `uri:"id" binding:"required,min=1"`
		ImageID int64 `uri:"image_id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证桌台所有权
	table, err := server.store.GetTable(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("table not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if table.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("table does not belong to your merchant")))
		return
	}

	// 删除图片
	err = server.store.DeleteTableImage(ctx, uriReq.ImageID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "image deleted successfully"})
}

// ==================== 桌台标签管理 ====================

type addTableTagRequest struct {
	TagID int64 `json:"tag_id" binding:"required,min=1"`
}

// addTableTag godoc
// @Summary 添加桌台标签
// @Description 商户为桌台添加标签
// @Tags 桌台管理
// @Accept json
// @Produce json
// @Param id path int true "桌台ID"
// @Param request body addTableTagRequest true "标签信息"
// @Success 200 {object} map[string]string "message: tag added successfully"
// @Failure 400 {object} ErrorResponse "参数错误或标签类型不正确"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非桌台所有者"
// @Failure 404 {object} ErrorResponse "桌台或标签不存在"
// @Failure 409 {object} ErrorResponse "标签已存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/tables/{id}/tags [post]
// @Security BearerAuth
func (server *Server) addTableTag(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req addTableTagRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证桌台所有权
	table, err := server.store.GetTable(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("table not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if table.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("table does not belong to your merchant")))
		return
	}

	// 验证标签存在且类型正确
	tag, err := server.store.GetTag(ctx, req.TagID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("tag not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if tag.Type != "table" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("tag is not a table tag")))
		return
	}

	// 添加标签
	_, err = server.store.AddTableTag(ctx, db.AddTableTagParams{
		TableID: uriReq.ID,
		TagID:   req.TagID,
	})
	if err != nil {
		// 可能是重复添加
		if db.ErrorCode(err) == db.UniqueViolation {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("tag already added")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "tag added successfully"})
}

type removeTableTagRequest struct {
	ID    int64 `uri:"id" binding:"required,min=1"`
	TagID int64 `uri:"tag_id" binding:"required,min=1"`
}

// removeTableTag godoc
// @Summary 移除桌台标签
// @Description 商户移除桌台的标签
// @Tags 桌台管理
// @Produce json
// @Param id path int true "桌台ID"
// @Param tag_id path int true "标签ID"
// @Success 200 {object} map[string]string "message: tag removed successfully"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非桌台所有者"
// @Failure 404 {object} ErrorResponse "桌台不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/tables/{id}/tags/{tag_id} [delete]
// @Security BearerAuth
func (server *Server) removeTableTag(ctx *gin.Context) {
	var req removeTableTagRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证桌台所有权
	table, err := server.store.GetTable(ctx, req.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("table not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if table.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("table does not belong to your merchant")))
		return
	}

	// 移除标签
	err = server.store.RemoveTableTag(ctx, db.RemoveTableTagParams{
		TableID: req.ID,
		TagID:   req.TagID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "tag removed successfully"})
}

// listTableTags godoc
// @Summary 获取桌台标签列表
// @Description 获取桌台的所有标签
// @Tags 桌台管理
// @Produce json
// @Param id path int true "桌台ID"
// @Success 200 {object} map[string][]tableTagInfo "tags: 标签列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/tables/{id}/tags [get]
// @Security BearerAuth
func (server *Server) listTableTags(ctx *gin.Context) {
	var req getTableRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	tags, err := server.store.ListTableTags(ctx, req.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]tableTagInfo, len(tags))
	for i, t := range tags {
		resp[i] = tableTagInfo{
			ID:   t.TagID,
			Name: t.TagName,
			Type: t.TagType,
		}
	}

	ctx.JSON(http.StatusOK, gin.H{"tags": resp})
}

// roomDetailResponse 包间详情响应（C端顾客使用）
type roomDetailResponse struct {
	ID                  int64          `json:"id"`
	MerchantID          int64          `json:"merchant_id"`
	RoomNo              string         `json:"room_no"`
	Capacity            int16          `json:"capacity"`
	Description         string         `json:"description,omitempty"`
	MinimumSpend        int64          `json:"minimum_spend,omitempty"`
	Status              string         `json:"status"`
	Tags                []tableTagInfo `json:"tags"`
	Images              []string       `json:"images"`
	PrimaryImage        string         `json:"primary_image,omitempty"`
	MonthlyReservations int64          `json:"monthly_reservations"`
	// 商户信息
	MerchantName      string   `json:"merchant_name"`
	MerchantLogo      string   `json:"merchant_logo,omitempty"`
	MerchantAddress   string   `json:"merchant_address,omitempty"`
	MerchantPhone     string   `json:"merchant_phone,omitempty"`
	MerchantLatitude  *float64 `json:"merchant_latitude,omitempty"`
	MerchantLongitude *float64 `json:"merchant_longitude,omitempty"`
}

// roomListItemResponse 包间列表项响应（C端顾客使用）
type roomListItemResponse struct {
	ID                  int64  `json:"id"`
	MerchantID          int64  `json:"merchant_id"`
	RoomNo              string `json:"room_no"`
	Capacity            int16  `json:"capacity"`
	Description         string `json:"description,omitempty"`
	MinimumSpend        int64  `json:"minimum_spend,omitempty"`
	Status              string `json:"status"`
	PrimaryImage        string `json:"primary_image,omitempty"`
	MonthlyReservations int64  `json:"monthly_reservations,omitempty"`
}

// listRoomsForCustomerResponse 顾客端包间列表响应
type listRoomsForCustomerResponse struct {
	Rooms []roomListItemResponse `json:"rooms"`
	Count int64                  `json:"count"`
}

// getRoomDetail godoc
// @Summary 获取包间详情
// @Description C端用户获取包间详细信息（含商户信息、图集、月销量、标签）
// @Tags 包间浏览
// @Produce json
// @Param id path int true "包间ID"
// @Success 200 {object} roomDetailResponse "包间详情"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "包间不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rooms/{id} [get]
func (server *Server) getRoomDetail(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 使用联合查询获取包间+商户信息+月销量
	room, err := server.store.GetRoomDetailForCustomer(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("room not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取标签
	tags, err := server.store.ListTableTags(ctx, room.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	tagList := make([]tableTagInfo, len(tags))
	for i, t := range tags {
		tagList[i] = tableTagInfo{
			ID:   t.TagID,
			Name: t.TagName,
			Type: t.TagType,
		}
	}

	// 获取所有图片（图集）
	images, err := server.store.ListTableImages(ctx, room.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	imageUrls := make([]string, len(images))
	for i, img := range images {
		imageUrls[i] = normalizeUploadURLForClient(img.ImageUrl)
	}

	resp := roomDetailResponse{
		ID:                  room.ID,
		MerchantID:          room.MerchantID,
		RoomNo:              room.TableNo,
		Capacity:            room.Capacity,
		Status:              room.Status,
		Tags:                tagList,
		Images:              imageUrls,
		MonthlyReservations: room.MonthlyReservations,
		MerchantName:        room.MerchantName,
		MerchantAddress:     room.MerchantAddress,
		MerchantPhone:       room.MerchantPhone,
	}

	if room.Description.Valid {
		resp.Description = room.Description.String
	}
	if room.MinimumSpend.Valid {
		resp.MinimumSpend = room.MinimumSpend.Int64
	}
	if room.PrimaryImage != "" {
		resp.PrimaryImage = normalizeUploadURLForClient(room.PrimaryImage)
	}
	if room.MerchantLogo.Valid {
		resp.MerchantLogo = normalizeUploadURLForClient(room.MerchantLogo.String)
	}
	if room.MerchantLatitude.Valid {
		lat, _ := room.MerchantLatitude.Float64Value()
		if lat.Valid {
			resp.MerchantLatitude = &lat.Float64
		}
	}
	if room.MerchantLongitude.Valid {
		lng, _ := room.MerchantLongitude.Float64Value()
		if lng.Valid {
			resp.MerchantLongitude = &lng.Float64
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

// timeSlot 时间段
type timeSlot struct {
	Time      string `json:"time"`      // 时间如 "11:00", "11:30"
	Available bool   `json:"available"` // 是否可预约
}

// roomAvailabilityResponse 包间可用性响应
type roomAvailabilityResponse struct {
	RoomID    int64      `json:"room_id"`
	RoomNo    string     `json:"room_no"`
	Date      string     `json:"date"`
	TimeSlots []timeSlot `json:"time_slots"`
}

// getRoomAvailability godoc
// @Summary 获取包间可预约时段
// @Description C端用户获取包间的可预约时间段
// @Tags 桌台管理
// @Produce json
// @Param id path int true "包间ID"
// @Param date query string true "日期(YYYY-MM-DD)"
// @Success 200 {object} roomAvailabilityResponse "可用时段"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "包间不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rooms/{id}/availability [get]
// @Security BearerAuth
func (server *Server) getRoomAvailability(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var queryReq struct {
		Date string `form:"date" binding:"required"` // 格式 YYYY-MM-DD
	}
	if err := ctx.ShouldBindQuery(&queryReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 解析日期
	date, err := time.Parse("2006-01-02", queryReq.Date)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid date format, expected YYYY-MM-DD")))
		return
	}

	// 获取包间信息
	room, err := server.store.GetTable(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("room not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if room.TableType != "room" {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("room not found")))
		return
	}

	// 获取该日期的所有预约
	reservations, err := server.store.ListReservationsByTableAndDate(ctx, db.ListReservationsByTableAndDateParams{
		TableID: room.ID,
		ReservationDate: pgtype.Date{
			Time:  date,
			Valid: true,
		},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 构建已预约时间集合 (用分钟表示)
	reservedTimes := make(map[string]bool)
	for _, r := range reservations {
		// 只考虑有效状态的预约
		if r.Status == "pending" || r.Status == "paid" || r.Status == "confirmed" {
			if r.ReservationTime.Valid {
				timeStr := fmt.Sprintf("%02d:%02d", r.ReservationTime.Microseconds/3600000000, (r.ReservationTime.Microseconds%3600000000)/60000000)
				reservedTimes[timeStr] = true
			}
		}
	}

	// 生成时间段列表（11:00-21:00，每30分钟一个时段）
	slots := []timeSlot{}
	for hour := 11; hour <= 21; hour++ {
		for _, minute := range []int{0, 30} {
			if hour == 21 && minute == 30 {
				continue // 21:30 不提供预约
			}
			timeStr := fmt.Sprintf("%02d:%02d", hour, minute)
			slots = append(slots, timeSlot{
				Time:      timeStr,
				Available: !reservedTimes[timeStr],
			})
		}
	}

	ctx.JSON(http.StatusOK, roomAvailabilityResponse{
		RoomID:    room.ID,
		RoomNo:    room.TableNo,
		Date:      queryReq.Date,
		TimeSlots: slots,
	})
}
