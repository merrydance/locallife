package api

import (
	"bytes"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
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
	selfCloudPrintCallbackHeaderEventID   = "X-Print-Callback-Event-Id"
	selfCloudPrintCallbackHeaderTimestamp = "X-Print-Callback-Timestamp"
	selfCloudPrintCallbackHeaderSignature = "X-Print-Callback-Signature"
)

type selfCloudPrintCallbackOKResponse struct {
	Data string `json:"data"`
}

type selfCloudPrintResultCallback struct {
	EventID      string            `json:"event_id"`
	PrintJobID   string            `json:"print_job_id"`
	AppID        string            `json:"app_id"`
	TenantRef    string            `json:"tenant_ref"`
	PrinterID    string            `json:"printer_id"`
	TaskKey      string            `json:"task_key"`
	Status       string            `json:"status"`
	Copies       int               `json:"copies"`
	Metadata     map[string]string `json:"metadata"`
	ErrorCode    string            `json:"error_code"`
	ErrorMessage string            `json:"error_message"`
}

func (server *Server) handleSelfCloudPrintResultNotify(ctx *gin.Context) {
	signingSecret := strings.TrimSpace(server.config.PrintServerCallbackSigningSecret)
	configuredAppID := strings.TrimSpace(server.config.PrintServerAppID)
	if signingSecret == "" || configuredAppID == "" {
		log.Error().
			Bool("has_signing_secret", signingSecret != "").
			Bool("has_app_id", configuredAppID != "").
			Msg("self-cloud print callback config is incomplete")
		ctx.JSON(http.StatusServiceUnavailable, selfCloudPrintCallbackOKResponse{Data: "FAIL"})
		return
	}

	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		log.Warn().Err(err).Msg("read self-cloud print callback body failed")
		ctx.JSON(http.StatusBadRequest, selfCloudPrintCallbackOKResponse{Data: "FAIL"})
		return
	}
	ctx.Request.Body = io.NopCloser(bytes.NewReader(body))

	headerEventID := strings.TrimSpace(ctx.GetHeader(selfCloudPrintCallbackHeaderEventID))
	timestampValue := strings.TrimSpace(ctx.GetHeader(selfCloudPrintCallbackHeaderTimestamp))
	signature := strings.TrimSpace(ctx.GetHeader(selfCloudPrintCallbackHeaderSignature))
	if headerEventID == "" || timestampValue == "" || signature == "" {
		log.Warn().
			Bool("has_event_id", headerEventID != "").
			Bool("has_timestamp", timestampValue != "").
			Bool("has_signature", signature != "").
			Msg("self-cloud print callback missing required signature header")
		ctx.JSON(http.StatusBadRequest, selfCloudPrintCallbackOKResponse{Data: "FAIL"})
		return
	}
	if !server.selfCloudCallbackTimestampFresh(timestampValue) {
		log.Warn().Str("event_id", headerEventID).Str("timestamp", timestampValue).Msg("self-cloud print callback outside freshness window")
		ctx.JSON(http.StatusUnauthorized, selfCloudPrintCallbackOKResponse{Data: "FAIL"})
		return
	}
	if !verifySelfCloudCallbackSignature(signingSecret, headerEventID, timestampValue, body, signature) {
		log.Warn().Str("event_id", headerEventID).Msg("self-cloud print callback signature verification failed")
		ctx.JSON(http.StatusUnauthorized, selfCloudPrintCallbackOKResponse{Data: "FAIL"})
		return
	}

	var payload selfCloudPrintResultCallback
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Warn().Err(err).Str("event_id", headerEventID).Msg("decode self-cloud print callback failed")
		ctx.JSON(http.StatusBadRequest, selfCloudPrintCallbackOKResponse{Data: "FAIL"})
		return
	}
	payload.EventID = strings.TrimSpace(payload.EventID)
	payload.PrintJobID = strings.TrimSpace(payload.PrintJobID)
	payload.AppID = strings.TrimSpace(payload.AppID)
	payload.Status = strings.ToLower(strings.TrimSpace(payload.Status))
	if payload.EventID == "" || payload.PrintJobID == "" || payload.AppID == "" || payload.Status == "" {
		log.Warn().
			Str("event_id", payload.EventID).
			Bool("has_print_job_id", payload.PrintJobID != "").
			Bool("has_app_id", payload.AppID != "").
			Bool("has_status", payload.Status != "").
			Msg("self-cloud print callback missing required payload field")
		ctx.JSON(http.StatusBadRequest, selfCloudPrintCallbackOKResponse{Data: "FAIL"})
		return
	}
	if payload.EventID != headerEventID {
		log.Warn().Str("header_event_id", headerEventID).Str("body_event_id", payload.EventID).Msg("self-cloud print callback event id mismatch")
		ctx.JSON(http.StatusBadRequest, selfCloudPrintCallbackOKResponse{Data: "FAIL"})
		return
	}
	if payload.AppID != configuredAppID {
		log.Warn().Str("event_id", payload.EventID).Str("app_id", payload.AppID).Msg("self-cloud print callback app id mismatch")
		ctx.JSON(http.StatusBadRequest, selfCloudPrintCallbackOKResponse{Data: "FAIL"})
		return
	}

	printLogStatus, errorMessage, ok := selfCloudCallbackTerminalMapping(payload)
	if !ok {
		log.Warn().Str("event_id", payload.EventID).Str("status", payload.Status).Msg("self-cloud print callback unsupported status")
		ctx.JSON(http.StatusBadRequest, selfCloudPrintCallbackOKResponse{Data: "FAIL"})
		return
	}

	printLogID, err := parseSelfCloudCallbackPrintLogID(payload.Metadata)
	if err != nil {
		log.Warn().Err(err).Str("event_id", payload.EventID).Msg("self-cloud print callback print log id is invalid")
		ctx.JSON(http.StatusBadRequest, selfCloudPrintCallbackOKResponse{Data: "FAIL"})
		return
	}

	result, err := server.store.ProcessSelfCloudPrintCallbackTx(ctx, db.ProcessSelfCloudPrintCallbackTxParams{
		EventID:        payload.EventID,
		PrintJobID:     payload.PrintJobID,
		PrintLogID:     printLogID,
		CallbackStatus: payload.Status,
		PrintLogStatus: printLogStatus,
		ErrorMessage:   errorMessage,
		RawPayload:     body,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			log.Warn().Err(err).Str("event_id", payload.EventID).Str("print_job_id", payload.PrintJobID).Msg("self-cloud print callback print log not found; ask provider to retry")
			ctx.JSON(http.StatusConflict, selfCloudPrintCallbackOKResponse{Data: "FAIL"})
			return
		}
		log.Error().Err(err).Str("event_id", payload.EventID).Str("print_job_id", payload.PrintJobID).Msg("process self-cloud print callback failed")
		ctx.JSON(http.StatusInternalServerError, selfCloudPrintCallbackOKResponse{Data: "FAIL"})
		return
	}

	log.Info().
		Str("event_id", payload.EventID).
		Str("print_job_id", payload.PrintJobID).
		Int64("print_log_id", result.PrintLog.ID).
		Str("status", printLogStatus).
		Bool("duplicate", result.Duplicate).
		Bool("already_terminal", result.AlreadyTerminal).
		Msg("self-cloud print callback acknowledged")
	ctx.JSON(http.StatusOK, selfCloudPrintCallbackOKResponse{Data: "OK"})
}

func (server *Server) selfCloudCallbackTimestampFresh(timestampValue string) bool {
	timestamp, err := time.Parse(time.RFC3339, timestampValue)
	if err != nil {
		return false
	}
	window := server.config.PrintServerCallbackFreshnessWindow
	if window <= 0 {
		window = 10 * time.Minute
	}
	now := time.Now().UTC()
	timestamp = timestamp.UTC()
	return !timestamp.Before(now.Add(-window)) && !timestamp.After(now.Add(window))
}

func verifySelfCloudCallbackSignature(secret, eventID, timestamp string, body []byte, signature string) bool {
	expected := cloudprint.BuildPrintServerCallbackSignature(secret, eventID, timestamp, body)
	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return false
	}
	actualBytes, err := hex.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(expectedBytes, actualBytes) == 1
}

func selfCloudCallbackTerminalMapping(payload selfCloudPrintResultCallback) (string, pgtype.Text, bool) {
	switch payload.Status {
	case cloudprint.PrintStateSuccess:
		return db.PrintLogStatusSuccess, pgtype.Text{}, true
	case cloudprint.PrintStateFailed, cloudprint.PrintStateTimeout, cloudprint.PrintStateCancelled:
		return db.PrintLogStatusFailed, pgtype.Text{String: sanitizeSelfCloudCallbackError(payload), Valid: true}, true
	default:
		return "", pgtype.Text{}, false
	}
}

func parseSelfCloudCallbackPrintLogID(metadata map[string]string) (pgtype.Int8, error) {
	raw := strings.TrimSpace(metadata["local_life_print_log_id"])
	if raw == "" {
		return pgtype.Int8{}, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return pgtype.Int8{}, errors.New("local_life_print_log_id is invalid")
	}
	return pgtype.Int8{Int64: id, Valid: true}, nil
}

func sanitizeSelfCloudCallbackError(payload selfCloudPrintResultCallback) string {
	code := strings.TrimSpace(payload.ErrorCode)
	message := strings.TrimSpace(payload.ErrorMessage)
	if code != "" && message != "" {
		return truncateSelfCloudCallbackError(code + ": " + message)
	}
	if message != "" {
		return truncateSelfCloudCallbackError(message)
	}
	if code != "" {
		return truncateSelfCloudCallbackError(code)
	}
	if payload.Status != "" {
		return truncateSelfCloudCallbackError(payload.Status)
	}
	return "provider_print_failed"
}

func truncateSelfCloudCallbackError(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= 500 {
		return message
	}
	return message[:500]
}
