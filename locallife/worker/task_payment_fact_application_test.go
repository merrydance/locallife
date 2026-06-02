package worker_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type paymentFactApplicationReleaseDistributor struct {
	worker.NoopTaskDistributor
	payloads []worker.ClaimBehaviorActionPayload
}

func (d *paymentFactApplicationReleaseDistributor) DistributeTaskClaimBehaviorAction(ctx context.Context, payload *worker.ClaimBehaviorActionPayload, opts ...asynq.Option) error {
	if payload != nil {
		d.payloads = append(d.payloads, *payload)
	}
	return nil
}

func TestProcessTaskPaymentFactApplication_SkipsUnclaimableApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), int64(901)).Return(db.ExternalPaymentFactApplication{}, db.ErrRecordNotFound)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payload, err := json.Marshal(worker.PaymentFactApplicationPayload{ApplicationID: 901})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentFactApplication(context.Background(), asynq.NewTask(worker.TaskProcessPaymentFactApplication, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentFactApplication_EnqueuesClaimRecoveryReleaseAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &paymentFactApplicationReleaseDistributor{}
	now := time.Now().UTC()
	application := db.ExternalPaymentFactApplication{
		ID:                 902,
		FactID:             802,
		Consumer:           "claim_recovery_domain",
		BusinessObjectType: "payment_order",
		BusinessObjectID:   702,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}
	processingApplication := application
	processingApplication.Status = db.ExternalPaymentFactApplicationStatusProcessing
	fact := db.ExternalPaymentFact{
		ID:                 application.FactID,
		Provider:           db.ExternalPaymentProviderWechat,
		Channel:            db.PaymentChannelDirect,
		Capability:         db.ExternalPaymentCapabilityDirectJSAPIPayment,
		FactSource:         db.ExternalPaymentFactSourceCallback,
		ExternalObjectType: db.ExternalPaymentObjectPayment,
		ExternalObjectKey:  "CR_PAY_702",
		BusinessOwner:      pgtype.Text{String: db.ExternalPaymentBusinessOwnerClaimRecovery, Valid: true},
		BusinessObjectType: pgtype.Text{String: "payment_order", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: application.BusinessObjectID, Valid: true},
		TerminalStatus:     db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:         true,
		RawResource:        []byte(`{}`),
	}
	paymentOrder := db.PaymentOrder{
		ID:           application.BusinessObjectID,
		BusinessType: db.ExternalPaymentBusinessOwnerClaimRecovery,
		Status:       "paid",
	}
	releaseAction := db.BehaviorAction{ID: 9902, ActionType: "release", TargetEntity: "merchant", Status: "created"}

	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), application.ID).Return(processingApplication, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), application.FactID).Return(fact, nil)
	store.EXPECT().ProcessPaymentSuccessTx(gomock.Any(), db.ProcessPaymentSuccessTxParams{PaymentOrderID: application.BusinessObjectID}).Return(db.ProcessPaymentSuccessTxResult{
		PaymentOrder:  paymentOrder,
		Processed:     true,
		ReleaseAction: &releaseAction,
	}, nil)
	store.EXPECT().UpdateExternalPaymentFactProcessingStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateExternalPaymentFactProcessingStatusParams{})).
		DoAndReturn(func(_ context.Context, arg db.UpdateExternalPaymentFactProcessingStatusParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, fact.ID, arg.ID)
			require.Equal(t, db.ExternalPaymentFactProcessingStatusTerminalized, arg.ProcessingStatus)
			return fact, nil
		})
	store.EXPECT().MarkExternalPaymentFactApplicationApplied(gomock.Any(), gomock.AssignableToTypeOf(db.MarkExternalPaymentFactApplicationAppliedParams{})).
		DoAndReturn(func(_ context.Context, arg db.MarkExternalPaymentFactApplicationAppliedParams) (db.ExternalPaymentFactApplication, error) {
			require.Equal(t, application.ID, arg.ID)
			require.WithinDuration(t, now, arg.AppliedAt.Time, time.Minute)
			applied := application
			applied.Status = db.ExternalPaymentFactApplicationStatusApplied
			return applied, nil
		})

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentFactApplicationPayload{ApplicationID: application.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentFactApplication(context.Background(), asynq.NewTask(worker.TaskProcessPaymentFactApplication, payload))
	require.NoError(t, err)
	require.Len(t, distributor.payloads, 1)
	require.Equal(t, releaseAction.ID, distributor.payloads[0].ActionID)
}

func TestProcessTaskPaymentFactApplication_RejectsMissingApplicationID(t *testing.T) {
	processor := worker.NewTestTaskProcessor(nil, nil, nil, nil)
	payload, err := json.Marshal(worker.PaymentFactApplicationPayload{})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentFactApplication(context.Background(), asynq.NewTask(worker.TaskProcessPaymentFactApplication, payload))
	require.Error(t, err)
	require.Contains(t, err.Error(), "application id is required")
}
