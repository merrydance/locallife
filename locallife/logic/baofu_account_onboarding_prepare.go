package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

func (s *BaofuAccountOnboardingService) PrepareOpening(ctx context.Context, input BaofuAccountOpeningInput) (BaofuAccountOpeningPrepareResult, error) {
	return s.openingFromInput(ctx, input, baofuAccountOpeningExecutionPrepare)
}

func (s *BaofuAccountOnboardingService) PrepareOpeningAfterVerifyFeePaid(ctx context.Context, paymentOrder db.PaymentOrder) (BaofuAccountOpeningPrepareResult, error) {
	if s == nil || s.store == nil {
		return BaofuAccountOpeningPrepareResult{}, ErrBaofuAccountOnboardingNotConfigured
	}
	flow, err := s.store.GetBaofuAccountOpeningFlowByPaymentOrder(ctx, pgtype.Int8{Int64: paymentOrder.ID, Valid: paymentOrder.ID > 0})
	if err != nil {
		return BaofuAccountOpeningPrepareResult{}, err
	}
	if !baofuOpeningRequiresUserFee(flow.OwnerType) {
		return BaofuAccountOpeningPrepareResult{BaofuAccountOpeningResult: BaofuAccountOpeningResult{State: flow.State, Label: baofuOnboardingStateLabel(flow.State), Flow: flow}}, nil
	}
	switch strings.TrimSpace(flow.State) {
	case db.BaofuAccountOpeningStateOpeningProcessing:
		profile, profileErr := s.profileForFlow(ctx, flow)
		if profileErr != nil {
			return BaofuAccountOpeningPrepareResult{}, profileErr
		}
		return BaofuAccountOpeningPrepareResult{BaofuAccountOpeningResult: baofuOpeningResult(flow, profile), ShouldEnqueueOpening: true}, nil
	case db.BaofuAccountOpeningStateMerchantReportProcessing,
		db.BaofuAccountOpeningStateAppletAuthPending,
		db.BaofuAccountOpeningStateReady,
		db.BaofuAccountOpeningStateVoided:
		profile, _ := s.profileForFlow(ctx, flow)
		return BaofuAccountOpeningPrepareResult{BaofuAccountOpeningResult: baofuOpeningResult(flow, profile)}, nil
	}
	if strings.TrimSpace(flow.State) == db.BaofuAccountOpeningStateFailed {
		recovered, err := s.recoverFailedFlowFromActiveBinding(ctx, flow, nil, true)
		if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
			return BaofuAccountOpeningPrepareResult{}, err
		}
		if err == nil && strings.TrimSpace(recovered.Flow.State) == db.BaofuAccountOpeningStateReady {
			return BaofuAccountOpeningPrepareResult{BaofuAccountOpeningResult: BaofuAccountOpeningResult{State: recovered.Flow.State, Label: baofuOnboardingStateLabel(recovered.Flow.State), Flow: recovered.Flow, Binding: recovered.Binding}}, nil
		}
	}
	profile, err := s.profileForFlow(ctx, flow)
	if err != nil {
		return BaofuAccountOpeningPrepareResult{}, err
	}
	if strings.TrimSpace(profile.ProfileStatus) != db.BaofuAccountOpeningProfileStatusComplete {
		flow, err = s.markProfilePending(ctx, flow, profile)
		if err != nil {
			return BaofuAccountOpeningPrepareResult{}, err
		}
		return BaofuAccountOpeningPrepareResult{BaofuAccountOpeningResult: baofuOpeningResult(flow, profile)}, nil
	}
	prepared, err := s.prepareFlowForOpen(ctx, flow, profile, s.config.normalized())
	if err != nil {
		return BaofuAccountOpeningPrepareResult{}, err
	}
	return BaofuAccountOpeningPrepareResult{BaofuAccountOpeningResult: baofuOpeningResult(prepared, profile), ShouldEnqueueOpening: true}, nil
}

func (s *BaofuAccountOnboardingService) ExecutePreparedOpening(ctx context.Context, flowID int64) (BaofuAccountOpenApplyResult, error) {
	if s == nil || s.store == nil {
		return BaofuAccountOpenApplyResult{}, ErrBaofuAccountOnboardingNotConfigured
	}
	if flowID <= 0 {
		return BaofuAccountOpenApplyResult{}, errors.New("baofu account opening flow id is required")
	}
	flow, err := s.store.GetBaofuAccountOpeningFlow(ctx, flowID)
	if err != nil {
		return BaofuAccountOpenApplyResult{}, err
	}
	if strings.TrimSpace(flow.State) != db.BaofuAccountOpeningStateOpeningProcessing {
		return BaofuAccountOpenApplyResult{Flow: flow}, nil
	}
	profile, err := s.profileForFlow(ctx, flow)
	if err != nil {
		return BaofuAccountOpenApplyResult{}, err
	}
	if strings.TrimSpace(profile.ProfileStatus) != db.BaofuAccountOpeningProfileStatusComplete {
		updated, err := s.markProfilePending(ctx, flow, profile)
		if err != nil {
			return BaofuAccountOpenApplyResult{}, err
		}
		return BaofuAccountOpenApplyResult{Flow: updated}, nil
	}
	if baofuOpeningRequiresUserFee(flow.OwnerType) && !flow.VerifyFeePaymentOrderID.Valid {
		return BaofuAccountOpenApplyResult{Flow: flow}, nil
	}
	if binding, ok, err := s.activeBindingForPreparedFlow(ctx, flow); err != nil {
		return BaofuAccountOpenApplyResult{}, err
	} else if ok {
		return BaofuAccountOpenApplyResult{Flow: flow, Binding: &binding}, nil
	}
	flow, binding, err := s.openFromPreparedProfile(ctx, flow, profile, s.config.normalized())
	if err != nil {
		return BaofuAccountOpenApplyResult{}, err
	}
	return BaofuAccountOpenApplyResult{Flow: flow, Binding: binding}, nil
}

func (s *BaofuAccountOnboardingService) activeBindingForPreparedFlow(ctx context.Context, flow db.BaofuAccountOpeningFlow) (db.BaofuAccountBinding, bool, error) {
	binding, err := s.store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{OwnerType: flow.OwnerType, OwnerID: flow.OwnerID})
	if errors.Is(err, db.ErrRecordNotFound) {
		return db.BaofuAccountBinding{}, false, nil
	}
	if err != nil {
		return db.BaofuAccountBinding{}, false, err
	}
	return binding, strings.TrimSpace(binding.OpenState) == db.BaofuAccountOpenStateActive, nil
}

func (s *BaofuAccountOnboardingService) profileForFlow(ctx context.Context, flow db.BaofuAccountOpeningFlow) (db.BaofuAccountOpeningProfile, error) {
	if !flow.ProfileID.Valid {
		return db.BaofuAccountOpeningProfile{}, errors.New("baofu account opening profile is required")
	}
	return s.store.GetBaofuAccountOpeningProfile(ctx, flow.ProfileID.Int64)
}
