package logic

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
)

func NormalizeMerchantCancelState(cancelState string) string {
	return strings.ToUpper(strings.TrimSpace(cancelState))
}

func MerchantCancelWithdrawIsTerminal(cancelState string) bool {
	switch NormalizeMerchantCancelState(cancelState) {
	case db.MerchantCancelStateRejected, db.MerchantCancelStateRevoked, db.MerchantCancelStateCanceled, db.MerchantCancelStateFinish:
		return true
	default:
		return false
	}
}

func BuildMerchantCancelWithdrawSyncParams(
	current db.MerchantCancelWithdrawApplication,
	query *wechat.EcommerceCancelWithdrawQueryResponse,
	localSyncState string,
	lastError string,
	markSubmitted bool,
	queryAt time.Time,
) (db.UpdateMerchantCancelWithdrawApplicationSyncParams, error) {
	params := db.UpdateMerchantCancelWithdrawApplicationSyncParams{
		ApplymentID:              current.ApplymentID,
		LocalSyncState:           localSyncState,
		CancelState:              current.CancelState,
		CancelStateDescription:   current.CancelStateDescription,
		WithdrawState:            current.WithdrawState,
		WithdrawStateDescription: current.WithdrawStateDescription,
		ConfirmCancelUrl:         current.ConfirmCancelUrl,
		AccountInfo:              current.AccountInfo,
		AccountWithdrawResult:    current.AccountWithdrawResult,
		LatestQueryResponse:      current.LatestQueryResponse,
		ClearLastError:           strings.TrimSpace(lastError) == "",
		LastError:                textParam(lastError),
		ModifyTime:               current.ModifyTime,
		MarkSubmitted:            markSubmitted,
		LastQueryAt:              pgtype.Timestamptz{Time: queryAt, Valid: true},
		ID:                       current.ID,
	}

	if query == nil {
		return params, nil
	}

	if query.ApplymentID != "" {
		params.ApplymentID = textParam(query.ApplymentID)
	}
	params.CancelState = textParam(query.CancelState)
	params.CancelStateDescription = textParam(query.CancelStateDescription)
	params.WithdrawState = textParam(query.WithdrawState)
	params.WithdrawStateDescription = textParam(query.WithdrawStateDescription)

	confirmCancelURL := ""
	if query.ConfirmCancel != nil {
		confirmCancelURL = query.ConfirmCancel.ConfirmCancelURL
	}
	params.ConfirmCancelUrl = textParam(confirmCancelURL)

	accountInfo, err := marshalJSON(query.AccountInfo)
	if err != nil {
		return params, fmt.Errorf("marshal account_info: %w", err)
	}
	params.AccountInfo = accountInfo

	accountWithdrawResult, err := marshalJSON(query.AccountWithdrawResult)
	if err != nil {
		return params, fmt.Errorf("marshal account_withdraw_result: %w", err)
	}
	params.AccountWithdrawResult = accountWithdrawResult

	latestQueryResponse, err := marshalJSON(query)
	if err != nil {
		return params, fmt.Errorf("marshal latest_query_response: %w", err)
	}
	params.LatestQueryResponse = latestQueryResponse

	if strings.TrimSpace(query.ModifyTime) == "" {
		params.ModifyTime = pgtype.Timestamptz{}
	} else {
		modifyTime, err := time.Parse(time.RFC3339, query.ModifyTime)
		if err != nil {
			return params, fmt.Errorf("parse modify_time: %w", err)
		}
		params.ModifyTime = pgtype.Timestamptz{Time: modifyTime, Valid: true}
	}

	return params, nil
}

func textParam(value string) pgtype.Text {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: trimmed, Valid: true}
}

func marshalJSON(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if string(data) == "null" {
		return nil, nil
	}
	return data, nil
}
