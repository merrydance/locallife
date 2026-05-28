package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/cloudprint"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

const feieyunPrintStatusSuccess = 1

func (server *Server) handleFeieyunPrintResultNotify(ctx *gin.Context) {
	if strings.TrimSpace(server.config.FeieyunCallbackPublicKeyPEM) == "" {
		log.Error().Msg("feieyun callback public key is not configured")
		ctx.String(http.StatusServiceUnavailable, "FAIL")
		return
	}

	if err := ctx.Request.ParseForm(); err != nil {
		log.Warn().Err(err).Msg("parse feieyun print callback form failed")
		ctx.String(http.StatusBadRequest, "FAIL")
		return
	}

	orderID := strings.TrimSpace(ctx.Request.PostForm.Get("orderId"))
	statusValue := strings.TrimSpace(ctx.Request.PostForm.Get("status"))
	stime := strings.TrimSpace(ctx.Request.PostForm.Get("stime"))
	sign := strings.TrimSpace(ctx.Request.PostForm.Get("sign"))
	if orderID == "" || statusValue == "" || stime == "" || sign == "" {
		log.Warn().
			Bool("has_order_id", orderID != "").
			Bool("has_status", statusValue != "").
			Bool("has_stime", stime != "").
			Bool("has_sign", sign != "").
			Msg("feieyun print callback missing required field")
		ctx.String(http.StatusBadRequest, "FAIL")
		return
	}

	status, err := strconv.Atoi(statusValue)
	if err != nil {
		log.Warn().Err(err).Str("status", statusValue).Msg("feieyun print callback status is invalid")
		ctx.String(http.StatusBadRequest, "FAIL")
		return
	}
	if _, err := parseFeieyunCallbackStime(stime); err != nil {
		log.Warn().Err(err).Str("stime", stime).Msg("feieyun print callback stime is invalid")
		ctx.String(http.StatusBadRequest, "FAIL")
		return
	}

	if err := cloudprint.VerifyFeieyunCallbackSignature(ctx.Request.PostForm, server.config.FeieyunCallbackPublicKeyPEM); err != nil {
		log.Warn().Err(err).Str("vendor_order_id", orderID).Msg("feieyun print callback signature verification failed")
		ctx.String(http.StatusUnauthorized, "FAIL")
		return
	}

	if status != feieyunPrintStatusSuccess {
		log.Warn().
			Str("vendor_order_id", orderID).
			Int("status", status).
			Msg("feieyun print callback received unsupported status")
		ctx.String(http.StatusOK, "SUCCESS")
		return
	}

	printLog, err := server.store.GetPrintLogByVendorOrderID(ctx, pgtype.Text{String: orderID, Valid: true})
	if err != nil {
		if err == db.ErrRecordNotFound {
			log.Warn().Str("vendor_order_id", orderID).Msg("feieyun print callback references unknown vendor order; ask provider to retry")
			ctx.String(http.StatusConflict, "FAIL")
			return
		}
		log.Error().Err(err).Str("vendor_order_id", orderID).Msg("get print log for feieyun callback failed")
		ctx.String(http.StatusInternalServerError, "FAIL")
		return
	}

	if _, err := server.store.UpdatePrintLogStatus(ctx, db.UpdatePrintLogStatusParams{
		ID:     printLog.ID,
		Status: "success",
	}); err != nil {
		log.Error().Err(err).
			Int64("print_log_id", printLog.ID).
			Str("vendor_order_id", orderID).
			Msg("update print log from feieyun callback failed")
		ctx.String(http.StatusInternalServerError, "FAIL")
		return
	}

	log.Info().
		Int64("print_log_id", printLog.ID).
		Int64("order_id", printLog.OrderID).
		Int64("printer_id", printLog.PrinterID).
		Str("vendor_order_id", orderID).
		Msg("feieyun print callback marked print log success")
	ctx.String(http.StatusOK, "SUCCESS")
}

func parseFeieyunCallbackStime(value string) (int64, error) {
	if len(value) != 10 {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseInt(value, 10, 64)
}
