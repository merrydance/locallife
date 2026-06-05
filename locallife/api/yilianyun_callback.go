package api

import (
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/cloudprint"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

const (
	yilianyunPrintCallbackCmdFinish = "oauth_finish"
	yilianyunPrintCallbackStateOK   = "1"
	yilianyunPrintCallbackStateFail = "2"
)

type yilianyunPushOKResponse struct {
	Data string `json:"data"`
}

func (server *Server) handleYilianyunPrintResultHealth(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, yilianyunPushOKResponse{Data: "OK"})
}

func (server *Server) handleYilianyunPrintResultNotify(ctx *gin.Context) {
	if strings.TrimSpace(server.config.YilianyunAppID) == "" || strings.TrimSpace(server.config.YilianyunAppSecret) == "" {
		log.Error().Msg("yilianyun callback credential is not configured")
		ctx.JSON(http.StatusServiceUnavailable, yilianyunPushOKResponse{Data: "FAIL"})
		return
	}

	if err := ctx.Request.ParseForm(); err != nil {
		log.Warn().Err(err).Msg("parse yilianyun print callback form failed")
		ctx.JSON(http.StatusBadRequest, yilianyunPushOKResponse{Data: "FAIL"})
		return
	}

	form := ctx.Request.PostForm
	cmd := strings.TrimSpace(form.Get("cmd"))
	machineCode := strings.TrimSpace(form.Get("machine_code"))
	orderID := strings.TrimSpace(form.Get("order_id"))
	state := strings.TrimSpace(form.Get("state"))
	printTime := strings.TrimSpace(form.Get("print_time"))
	originID := strings.TrimSpace(form.Get("origin_id"))
	pushTime := strings.TrimSpace(form.Get("push_time"))
	sign := strings.TrimSpace(form.Get("sign"))
	if cmd == "" || machineCode == "" || orderID == "" || state == "" || printTime == "" || originID == "" || pushTime == "" || sign == "" {
		log.Warn().
			Bool("has_cmd", cmd != "").
			Bool("has_machine_code", machineCode != "").
			Bool("has_order_id", orderID != "").
			Bool("has_state", state != "").
			Bool("has_print_time", printTime != "").
			Bool("has_origin_id", originID != "").
			Bool("has_push_time", pushTime != "").
			Bool("has_sign", sign != "").
			Msg("yilianyun print callback missing required field")
		ctx.JSON(http.StatusBadRequest, yilianyunPushOKResponse{Data: "FAIL"})
		return
	}
	if cmd != yilianyunPrintCallbackCmdFinish {
		log.Warn().Str("cmd", cmd).Msg("yilianyun print callback unsupported command")
		ctx.JSON(http.StatusBadRequest, yilianyunPushOKResponse{Data: "FAIL"})
		return
	}
	if state != yilianyunPrintCallbackStateOK && state != yilianyunPrintCallbackStateFail {
		log.Warn().Str("state", state).Str("provider_origin_id", originID).Msg("yilianyun print callback unsupported state")
		ctx.JSON(http.StatusBadRequest, yilianyunPushOKResponse{Data: "FAIL"})
		return
	}
	if _, err := strconv.ParseInt(printTime, 10, 64); err != nil {
		log.Warn().Err(err).Str("provider_origin_id", originID).Msg("yilianyun print callback print_time is invalid")
		ctx.JSON(http.StatusBadRequest, yilianyunPushOKResponse{Data: "FAIL"})
		return
	}
	pushUnix, err := strconv.ParseInt(pushTime, 10, 64)
	if err != nil {
		log.Warn().Err(err).Str("provider_origin_id", originID).Msg("yilianyun print callback push_time is invalid")
		ctx.JSON(http.StatusBadRequest, yilianyunPushOKResponse{Data: "FAIL"})
		return
	}
	if !server.yilianyunPushTimeFresh(pushUnix) {
		log.Warn().Str("provider_origin_id", originID).Int64("push_time", pushUnix).Msg("yilianyun print callback outside freshness window")
		ctx.JSON(http.StatusUnauthorized, yilianyunPushOKResponse{Data: "FAIL"})
		return
	}
	if !server.verifyYilianyunPushSignature(pushTime, sign) {
		log.Warn().Str("provider_origin_id", originID).Msg("yilianyun print callback signature verification failed")
		ctx.JSON(http.StatusUnauthorized, yilianyunPushOKResponse{Data: "FAIL"})
		return
	}

	printLog, err := server.store.GetPrintLogByProviderAndOriginID(ctx, db.GetPrintLogByProviderAndOriginIDParams{
		PrinterType:      string(cloudprint.ProviderYilianyun),
		ProviderOriginID: pgtype.Text{String: originID, Valid: true},
	})
	if err != nil {
		if err == db.ErrRecordNotFound {
			log.Warn().Str("provider_origin_id", originID).Str("provider_order_id", orderID).Msg("yilianyun print callback references unknown origin; ask provider to retry")
			ctx.JSON(http.StatusConflict, yilianyunPushOKResponse{Data: "FAIL"})
			return
		}
		log.Error().Err(err).Str("provider_origin_id", originID).Str("provider_order_id", orderID).Msg("get print log for yilianyun callback failed")
		ctx.JSON(http.StatusInternalServerError, yilianyunPushOKResponse{Data: "FAIL"})
		return
	}

	if !yilianyunVendorOrderMatches(printLog, orderID) {
		log.Warn().
			Int64("print_log_id", printLog.ID).
			Str("provider_origin_id", originID).
			Str("provider_order_id", orderID).
			Bool("has_local_vendor_order_id", printLog.VendorOrderID.Valid && strings.TrimSpace(printLog.VendorOrderID.String) != "").
			Msg("yilianyun print callback vendor order id mismatch; ask provider to retry")
		ctx.JSON(http.StatusConflict, yilianyunPushOKResponse{Data: "FAIL"})
		return
	}

	if printLog.Status != db.PrintLogStatusPending {
		log.Info().
			Int64("print_log_id", printLog.ID).
			Str("provider_origin_id", originID).
			Str("status", printLog.Status).
			Msg("yilianyun print callback duplicate terminal delivery acknowledged")
		ctx.JSON(http.StatusOK, yilianyunPushOKResponse{Data: "OK"})
		return
	}

	status := db.PrintLogStatusSuccess
	var errorMessage pgtype.Text
	if state == yilianyunPrintCallbackStateFail {
		status = db.PrintLogStatusCancelled
		errorMessage = pgtype.Text{String: "yilianyun_print_exception", Valid: true}
	}
	if _, err := server.store.MarkProviderStatusPrintLogTerminal(ctx, db.MarkProviderStatusPrintLogTerminalParams{
		ID:           printLog.ID,
		Status:       status,
		ErrorMessage: errorMessage,
	}); err != nil {
		if err == db.ErrRecordNotFound {
			log.Info().
				Int64("print_log_id", printLog.ID).
				Str("provider_origin_id", originID).
				Msg("yilianyun print callback terminal race acknowledged")
			ctx.JSON(http.StatusOK, yilianyunPushOKResponse{Data: "OK"})
			return
		}
		log.Error().Err(err).Int64("print_log_id", printLog.ID).Str("provider_origin_id", originID).Msg("update print log from yilianyun callback failed")
		ctx.JSON(http.StatusInternalServerError, yilianyunPushOKResponse{Data: "FAIL"})
		return
	}

	log.Info().
		Int64("print_log_id", printLog.ID).
		Int64("order_id", printLog.OrderID).
		Int64("printer_id", printLog.PrinterID).
		Str("provider_origin_id", originID).
		Str("provider_order_id", orderID).
		Str("status", status).
		Msg("yilianyun print callback marked print log terminal")
	ctx.JSON(http.StatusOK, yilianyunPushOKResponse{Data: "OK"})
}

func (server *Server) yilianyunPushTimeFresh(pushUnix int64) bool {
	window := server.config.YilianyunPrintCallbackFreshnessWindow
	if window <= 0 {
		window = 10 * time.Minute
	}
	pushAt := time.Unix(pushUnix, 0)
	now := time.Now()
	return !pushAt.Before(now.Add(-window)) && !pushAt.After(now.Add(window))
}

func (server *Server) verifyYilianyunPushSignature(pushTime string, sign string) bool {
	expected := buildYilianyunPushSignature(
		strings.TrimSpace(server.config.YilianyunAppID),
		strings.TrimSpace(pushTime),
		strings.TrimSpace(server.config.YilianyunAppSecret),
	)
	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return false
	}
	actualBytes, err := hex.DecodeString(strings.TrimSpace(sign))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(expectedBytes, actualBytes) == 1
}

func buildYilianyunPushSignature(clientID, pushTime, clientSecret string) string {
	sum := md5.Sum([]byte(clientID + pushTime + clientSecret))
	return hex.EncodeToString(sum[:])
}

func yilianyunVendorOrderMatches(printLog db.PrintLog, callbackOrderID string) bool {
	if !printLog.VendorOrderID.Valid {
		return false
	}
	return strings.TrimSpace(printLog.VendorOrderID.String) == strings.TrimSpace(callbackOrderID)
}
