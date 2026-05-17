package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	baofunotification "github.com/merrydance/locallife/baofu/account/notification"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog/log"
)

func (server *Server) recordBaofuAccountOpenCallbackFact(ctx context.Context, notification *baofunotification.AccountNotification) (db.ExternalPaymentFact, error) {
	if notification == nil {
		return db.ExternalPaymentFact{}, fmt.Errorf("baofu account callback notification is required")
	}
	outRequestNo := strings.TrimSpace(notification.OutRequestNo)
	upstreamState := strings.TrimSpace(notification.UpstreamState)
	terminalStatus, isTerminal := baofuAccountTerminalStatus(notification.OpenState)
	observedAt := baofuObservedAt(notification)
	flow, err := server.resolveBaofuAccountOpeningFlowForCallback(ctx, notification)
	if err != nil {
		return db.ExternalPaymentFact{}, err
	}
	externalObjectKey := outRequestNo
	if externalObjectKey == "" {
		externalObjectKey = strings.TrimSpace(notification.ContractNo)
	}
	createParams := db.CreateExternalPaymentFactParams{
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuAccount,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventID:        baofuText(externalObjectKey),
		SourceEventType:      pgtype.Text{String: "BAOFU_ACCOUNT_OPEN", Valid: true},
		ExternalObjectType:   "baofu_account",
		ExternalObjectKey:    externalObjectKey,
		ExternalSecondaryKey: baofuText(strings.TrimSpace(notification.ContractNo)),
		BusinessOwner:        pgtype.Text{String: strings.TrimSpace(flow.OwnerType), Valid: strings.TrimSpace(flow.OwnerType) != ""},
		BusinessObjectType:   pgtype.Text{String: "baofu_account_opening_flow", Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: flow.ID, Valid: flow.ID > 0},
		UpstreamState:        upstreamState,
		TerminalStatus:       terminalStatus,
		IsTerminal:           isTerminal,
		Currency:             "CNY",
		OccurredAt:           pgtype.Timestamptz{Time: observedAt, Valid: true},
		ObservedAt:           time.Now().UTC(),
		RawResource:          notification.Raw,
		DedupeKey:            fmt.Sprintf("baofu:callback:account:%s:%s", externalObjectKey, upstreamState),
		ProcessingStatus:     db.ExternalPaymentFactProcessingStatusReceived,
	}
	fact, err := server.store.CreateExternalPaymentFact(ctx, createParams)
	if err != nil {
		fact, err = server.baofuAccountOpenCallbackExistingFact(ctx, createParams, err)
		if err != nil {
			return db.ExternalPaymentFact{}, err
		}
	}
	if err := server.applyBaofuAccountOpenCallbackState(ctx, notification, flow); err != nil {
		return db.ExternalPaymentFact{}, err
	}
	return fact, nil
}

func (server *Server) baofuAccountOpenCallbackExistingFact(ctx context.Context, params db.CreateExternalPaymentFactParams, createErr error) (db.ExternalPaymentFact, error) {
	if !errors.Is(createErr, db.ErrRecordNotFound) {
		return db.ExternalPaymentFact{}, createErr
	}
	fact, err := server.store.GetExternalPaymentFactByDedupeKey(ctx, params.DedupeKey)
	if err != nil {
		return db.ExternalPaymentFact{}, createErr
	}
	if !baofuAccountOpenCallbackFactMatches(params, fact) {
		return db.ExternalPaymentFact{}, createErr
	}
	return fact, nil
}

func baofuAccountOpenCallbackFactMatches(params db.CreateExternalPaymentFactParams, fact db.ExternalPaymentFact) bool {
	if fact.Provider != params.Provider ||
		fact.Channel != params.Channel ||
		fact.Capability != params.Capability ||
		fact.FactSource != params.FactSource ||
		fact.ExternalObjectType != params.ExternalObjectType ||
		fact.ExternalObjectKey != params.ExternalObjectKey ||
		fact.UpstreamState != params.UpstreamState ||
		fact.TerminalStatus != params.TerminalStatus ||
		fact.IsTerminal != params.IsTerminal ||
		fact.Currency != params.Currency ||
		fact.DedupeKey != params.DedupeKey {
		return false
	}
	if !pgTextEqual(fact.SourceEventID, params.SourceEventID) ||
		!pgTextEqual(fact.SourceEventType, params.SourceEventType) ||
		!pgTextEqual(fact.ExternalSecondaryKey, params.ExternalSecondaryKey) ||
		!pgTextEqual(fact.BusinessOwner, params.BusinessOwner) ||
		!pgTextEqual(fact.BusinessObjectType, params.BusinessObjectType) {
		return false
	}
	return pgInt8Equal(fact.BusinessObjectID, params.BusinessObjectID)
}

func pgTextEqual(left pgtype.Text, right pgtype.Text) bool {
	if left.Valid != right.Valid {
		return false
	}
	if !left.Valid {
		return true
	}
	return strings.TrimSpace(left.String) == strings.TrimSpace(right.String)
}

func pgInt8Equal(left pgtype.Int8, right pgtype.Int8) bool {
	if left.Valid != right.Valid {
		return false
	}
	if !left.Valid {
		return true
	}
	return left.Int64 == right.Int64
}

func (server *Server) resolveBaofuAccountOpeningFlowForCallback(ctx context.Context, notification *baofunotification.AccountNotification) (db.BaofuAccountOpeningFlow, error) {
	if notification == nil {
		return db.BaofuAccountOpeningFlow{}, fmt.Errorf("baofu account callback notification is required")
	}
	outRequestNo := strings.TrimSpace(notification.OutRequestNo)
	if outRequestNo != "" {
		flow, err := server.store.GetBaofuAccountOpeningFlowByOpenTransSerialNo(ctx, pgtype.Text{String: outRequestNo, Valid: true})
		if err == nil || !errors.Is(err, db.ErrRecordNotFound) {
			return flow, err
		}
		server.persistUnmatchedBaofuAccountCallbackAlert(ctx, notification, "callback_serial_no_unmatched")
		return db.BaofuAccountOpeningFlow{}, fmt.Errorf("baofu account callback out request no %q does not match a local opening flow", outRequestNo)
	}
	contractNo := strings.TrimSpace(notification.ContractNo)
	if contractNo == "" {
		server.persistUnmatchedBaofuAccountCallbackAlert(ctx, notification, "callback_unmatched")
		return db.BaofuAccountOpeningFlow{}, fmt.Errorf("baofu account callback out request no or contract no is required")
	}
	binding, err := server.store.GetBaofuAccountBindingByContractNo(ctx, pgtype.Text{String: contractNo, Valid: true})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			server.persistUnmatchedBaofuAccountCallbackAlert(ctx, notification, "callback_unmatched")
		}
		return db.BaofuAccountOpeningFlow{}, fmt.Errorf("resolve baofu account callback contract %q: %w", contractNo, err)
	}
	flow, err := server.store.GetLatestBaofuAccountOpeningFlowByOwner(ctx, db.GetLatestBaofuAccountOpeningFlowByOwnerParams{
		OwnerType: binding.OwnerType,
		OwnerID:   binding.OwnerID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			server.persistUnmatchedBaofuAccountCallbackAlert(ctx, notification, "callback_owner_flow_missing")
		}
		return db.BaofuAccountOpeningFlow{}, fmt.Errorf("resolve baofu account callback flow for contract %q: %w", contractNo, err)
	}
	return flow, nil
}

func (server *Server) persistUnmatchedBaofuAccountCallbackAlert(ctx context.Context, notification *baofunotification.AccountNotification, reason string) {
	if notification == nil {
		return
	}
	outRequestNo := strings.TrimSpace(notification.OutRequestNo)
	contractNo := strings.TrimSpace(notification.ContractNo)
	upstreamState := strings.TrimSpace(notification.UpstreamState)
	openState := strings.TrimSpace(notification.OpenState)
	err := worker.SavePlatformAlertEvent(
		ctx,
		server.store,
		string(worker.AlertTypeSystemError),
		string(worker.AlertLevelCritical),
		"宝付开户回调未匹配本地流程",
		fmt.Sprintf("宝付开户回调未匹配到本地开户流程，流水号 %s，合约号 %s。系统未创建账户绑定，请人工核对宝付侧状态和本地流程。", outRequestNo, contractNo),
		0,
		"baofu_account_opening_flow",
		map[string]any{
			"out_request_no": outRequestNo,
			"contract_no":    contractNo,
			"upstream_state": upstreamState,
			"open_state":     openState,
			"reason":         strings.TrimSpace(reason),
		},
		time.Now(),
	)
	if err != nil {
		log.Error().Err(err).
			Str("out_request_no", outRequestNo).
			Str("contract_no", contractNo).
			Str("upstream_state", upstreamState).
			Str("open_state", openState).
			Str("reason", strings.TrimSpace(reason)).
			Msg("persist unmatched baofu account callback alert failed")
	}
}

func (server *Server) applyBaofuAccountOpenCallbackState(ctx context.Context, notification *baofunotification.AccountNotification, flow db.BaofuAccountOpeningFlow) error {
	if notification == nil {
		return fmt.Errorf("baofu account callback notification is required")
	}
	result := baofuAccountResultFromNotification(notification)
	service := server.newBaofuAccountOnboardingService()
	_, err := service.ApplyAccountOpenResult(ctx, flow, result)
	return err
}

func baofuAccountResultFromNotification(notification *baofunotification.AccountNotification) baofucontracts.AccountResult {
	if notification == nil {
		return baofucontracts.AccountResult{}
	}
	sharingMerID := strings.TrimSpace(notification.SharingMerID)
	contractNo := strings.TrimSpace(notification.ContractNo)
	if sharingMerID == "" {
		sharingMerID = contractNo
	}
	return baofucontracts.AccountResult{
		OutRequestNo:  strings.TrimSpace(notification.OutRequestNo),
		ContractNo:    contractNo,
		SharingMerID:  sharingMerID,
		OpenState:     strings.TrimSpace(notification.OpenState),
		UpstreamState: strings.TrimSpace(notification.UpstreamState),
		FailCode:      strings.TrimSpace(notification.FailCode),
		FailMessage:   strings.TrimSpace(notification.FailMessage),
		Raw:           notification.Raw,
	}
}
