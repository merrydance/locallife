package worker_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskBaofuAccountOpeningRejectsInvalidPayloadWithSkipRetry(t *testing.T) {
	processor := worker.NewTestTaskProcessor(nil, nil, nil, nil)

	err := processor.ProcessTaskBaofuAccountOpening(context.Background(), asynq.NewTask(worker.TaskProcessBaofuAccountOpening, []byte(`{"flow_id":0}`)))

	require.Error(t, err)
	require.ErrorIs(t, err, asynq.SkipRetry)
	require.Contains(t, err.Error(), "flow id is required")
}

func TestProcessTaskBaofuAccountOpeningExecutesPreparedFlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	accountClient := &baofuAccountOpeningWorkerClient{
		openResult: &baofucontracts.AccountResult{
			OpenState:     db.BaofuAccountOpenStateProcessing,
			UpstreamState: "2",
		},
	}
	flow := baofuAccountOpeningWorkerFlow(901, 66, 501)
	profile := baofuAccountOpeningWorkerProfile(flow)

	store.EXPECT().GetBaofuAccountOpeningFlow(gomock.Any(), flow.ID).Return(flow, nil)
	store.EXPECT().GetBaofuAccountOpeningProfile(gomock.Any(), profile.ID).Return(profile, nil)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: flow.OwnerType,
		OwnerID:   flow.OwnerID,
	}).Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)
	store.EXPECT().UpsertBaofuAccountBinding(gomock.Any(), gomock.AssignableToTypeOf(db.UpsertBaofuAccountBindingParams{})).
		DoAndReturn(func(_ context.Context, arg db.UpsertBaofuAccountBindingParams) (db.BaofuAccountBinding, error) {
			require.Equal(t, flow.OpenTransSerialNo, arg.LastOpenTransSerialNo)
			require.Equal(t, flow.LoginNo, arg.LoginNo)
			return db.BaofuAccountBinding{
				ID:                    9901,
				OwnerType:             arg.OwnerType,
				OwnerID:               arg.OwnerID,
				AccountType:           arg.AccountType,
				LoginNo:               arg.LoginNo,
				OpenState:             arg.OpenState,
				LastOpenTransSerialNo: arg.LastOpenTransSerialNo,
			}, nil
		})

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	processor.SetBaofuAccountClient(accountClient, nil)

	err := processor.ProcessTaskBaofuAccountOpening(context.Background(), asynq.NewTask(worker.TaskProcessBaofuAccountOpening, []byte(`{"flow_id":901}`)))

	require.NoError(t, err)
	require.Equal(t, 1, accountClient.openCalls)
	require.Equal(t, flow.OpenTransSerialNo.String, accountClient.lastOpen.OutRequestNo)
	require.Equal(t, flow.LoginNo.String, accountClient.lastOpen.LoginNo)
	require.Equal(t, db.BaofuAccountTypePersonal, accountClient.lastOpen.AccountType)
}

type baofuAccountOpeningWorkerClient struct {
	lastOpen    baofucontracts.OpenAccountRequest
	lastQuery   baofucontracts.QueryAccountRequest
	openResult  *baofucontracts.AccountResult
	queryResult *baofucontracts.AccountResult
	openErr     error
	queryErr    error
	openCalls   int
}

func (c *baofuAccountOpeningWorkerClient) OpenAccount(_ context.Context, req baofucontracts.OpenAccountRequest) (*baofucontracts.AccountResult, error) {
	c.openCalls++
	c.lastOpen = req
	if c.openErr != nil {
		return nil, c.openErr
	}
	return c.openResult, nil
}

func (c *baofuAccountOpeningWorkerClient) QueryAccount(_ context.Context, req baofucontracts.QueryAccountRequest) (*baofucontracts.AccountResult, error) {
	c.lastQuery = req
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	if c.queryResult != nil {
		return c.queryResult, nil
	}
	return nil, errors.New("unexpected query account call")
}

func baofuAccountOpeningWorkerFlow(flowID, ownerID, paymentOrderID int64) db.BaofuAccountOpeningFlow {
	return db.BaofuAccountOpeningFlow{
		ID:                      flowID,
		OwnerType:               db.BaofuAccountOwnerTypeRider,
		OwnerID:                 ownerID,
		AccountType:             db.BaofuAccountTypePersonal,
		ProfileID:               pgtype.Int8{Int64: 801, Valid: true},
		State:                   db.BaofuAccountOpeningStateOpeningProcessing,
		VerifyFeePaymentOrderID: pgtype.Int8{Int64: paymentOrderID, Valid: true},
		OpenTransSerialNo:       pgtype.Text{String: "BFO_PREPARED_901", Valid: true},
		LoginNo:                 pgtype.Text{String: "LLBFOR0000000066", Valid: true},
	}
}

func baofuAccountOpeningWorkerProfile(flow db.BaofuAccountOpeningFlow) db.BaofuAccountOpeningProfile {
	return db.BaofuAccountOpeningProfile{
		ID:                      flow.ProfileID.Int64,
		OwnerType:               flow.OwnerType,
		OwnerID:                 flow.OwnerID,
		AccountType:             flow.AccountType,
		ProfileStatus:           db.BaofuAccountOpeningProfileStatusComplete,
		LegalName:               pgtype.Text{String: "张三", Valid: true},
		CertificateType:         pgtype.Text{String: baofucontracts.OfficialCertificateTypeID, Valid: true},
		CertificateNoCiphertext: pgtype.Text{String: "110101199001011234", Valid: true},
		BankAccountNoCiphertext: pgtype.Text{String: "6222020202020202", Valid: true},
		BankMobileCiphertext:    pgtype.Text{String: "13800138000", Valid: true},
		CardUserName:            pgtype.Text{String: "张三", Valid: true},
	}
}
