package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBaofuSettlementAccountMerchantOwnerCanReadSafeSummary(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	binding := db.BaofuAccountBinding{
		ID:             91,
		OwnerType:      db.BaofuAccountOwnerTypeMerchant,
		OwnerID:        merchant.ID,
		AccountType:    db.BaofuAccountTypeBusiness,
		ContractNo:     pgtype.Text{String: "CONTRACT-NO-SHOULD-NOT-LEAK", Valid: true},
		SharingMerID:   pgtype.Text{String: "SHARING-MER-ID-SHOULD-NOT-LEAK", Valid: true},
		OpenState:      db.BaofuAccountOpenStateActive,
		BankCardLast4:  pgtype.Text{String: "1234", Valid: true},
		WechatSubMchID: pgtype.Text{String: "wx_sub_mch_123456789", Valid: true},
		UpdatedAt:      time.Now(),
	}
	profile := db.BaofuAccountOpeningProfile{
		OwnerType:         db.BaofuAccountOwnerTypeMerchant,
		OwnerID:           merchant.ID,
		AccountType:       db.BaofuAccountTypeBusiness,
		ProfileStatus:     baofuSettlementProfileStatusDraft,
		BankAccountNoMask: pgtype.Text{String: "6222********1234", Valid: true},
		BankMobileMask:    pgtype.Text{String: "138****0000", Valid: true},
		EmailMask:         pgtype.Text{String: "a***@example.com", Valid: true},
		UpdatedAt:         time.Now(),
	}
	flow := db.BaofuAccountOpeningFlow{
		ID:          77,
		OwnerType:   db.BaofuAccountOwnerTypeMerchant,
		OwnerID:     merchant.ID,
		AccountType: db.BaofuAccountTypeBusiness,
		State:       "opening_processing",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		})).
		Return(binding, nil)
	store.EXPECT().
		GetBaofuAccountOpeningProfileByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountOpeningProfileByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		})).
		Return(profile, nil)
	store.EXPECT().
		GetLatestBaofuAccountOpeningFlowByOwner(gomock.Any(), gomock.Eq(db.GetLatestBaofuAccountOpeningFlowByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		})).
		Return(flow, nil)
	store.EXPECT().
		GetBaofuMerchantReportByOwner(gomock.Any(), gomock.Eq(db.GetBaofuMerchantReportByOwnerParams{
			OwnerType:  db.BaofuAccountOwnerTypeMerchant,
			OwnerID:    merchant.ID,
			ReportType: db.BaofuMerchantReportTypeWechat,
		})).
		Return(db.BaofuMerchantReport{
			OwnerType:       db.BaofuAccountOwnerTypeMerchant,
			OwnerID:         merchant.ID,
			ReportType:      db.BaofuMerchantReportTypeWechat,
			ReportState:     db.BaofuMerchantReportStateSucceeded,
			AppletAuthState: db.BaofuMerchantReportAppletAuthStateSucceeded,
			SubMchID:        pgtype.Text{String: "wx_sub_mch_123456789", Valid: true},
		}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "CONTRACT-NO-SHOULD-NOT-LEAK")
	require.NotContains(t, recorder.Body.String(), "SHARING-MER-ID-SHOULD-NOT-LEAK")
	var response baofuSettlementAccountResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, db.BaofuAccountOwnerTypeMerchant, response.OwnerType)
	require.Equal(t, merchant.ID, response.OwnerID)
	require.Equal(t, db.BaofuAccountTypeBusiness, response.AccountType)
	require.True(t, response.PaymentReady)
	require.Equal(t, "1234", response.BankCardLast4)
	require.Equal(t, "***6789", response.WechatSubMchIDMask)
	require.Empty(t, response.MissingFields)
}

func TestBaofuSettlementAccountRiderActiveBindingDoesNotReturnProfileMissingFields(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.Status = "approved"
	binding := db.BaofuAccountBinding{
		ID:          93,
		OwnerType:   db.BaofuAccountOwnerTypeRider,
		OwnerID:     rider.ID,
		AccountType: db.BaofuAccountTypePersonal,
		OpenState:   db.BaofuAccountOpenStateActive,
		UpdatedAt:   time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
		Return(rider, nil)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(binding, nil)
	store.EXPECT().
		GetBaofuAccountOpeningProfileByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountOpeningProfileByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(db.BaofuAccountOpeningProfile{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetLatestBaofuAccountOpeningFlowByOwner(gomock.Any(), gomock.Eq(db.GetLatestBaofuAccountOpeningFlowByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(db.BaofuAccountOpeningFlow{}, db.ErrRecordNotFound)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/rider/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response baofuSettlementAccountResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, db.BaofuAccountOpeningStateReady, response.Status)
	require.True(t, response.PaymentReady)
	require.Empty(t, response.MissingFields)
	require.NotContains(t, recorder.Body.String(), "missing_fields")
}

func TestBaofuSettlementAccountRiderPostActiveBindingDoesNotReturnProfileMissingFields(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.Status = "approved"
	binding := db.BaofuAccountBinding{
		ID:          94,
		OwnerType:   db.BaofuAccountOwnerTypeRider,
		OwnerID:     rider.ID,
		AccountType: db.BaofuAccountTypePersonal,
		OpenState:   db.BaofuAccountOpenStateActive,
		UpdatedAt:   time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
		Return(rider, nil)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(binding, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/rider/settlement-account", bytes.NewBufferString(`{}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusAccepted, recorder.Code)
	var response baofuSettlementAccountResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, db.BaofuAccountOpeningStateReady, response.Status)
	require.True(t, response.PaymentReady)
	require.Empty(t, response.MissingFields)
	require.NotContains(t, recorder.Body.String(), "missing_fields")
}

func TestBaofuSettlementAccountMerchantActiveBindingWaitsForAppletAuth(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	binding := db.BaofuAccountBinding{
		ID:           92,
		OwnerType:    db.BaofuAccountOwnerTypeMerchant,
		OwnerID:      merchant.ID,
		AccountType:  db.BaofuAccountTypeBusiness,
		ContractNo:   pgtype.Text{String: "CM202605080001", Valid: true},
		SharingMerID: pgtype.Text{String: "CM202605080001", Valid: true},
		OpenState:    db.BaofuAccountOpenStateActive,
		UpdatedAt:    time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		})).
		Return(binding, nil)
	store.EXPECT().
		GetBaofuAccountOpeningProfileByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountOpeningProfileByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		})).
		Return(db.BaofuAccountOpeningProfile{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetLatestBaofuAccountOpeningFlowByOwner(gomock.Any(), gomock.Eq(db.GetLatestBaofuAccountOpeningFlowByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		})).
		Return(db.BaofuAccountOpeningFlow{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetBaofuMerchantReportByOwner(gomock.Any(), gomock.Eq(db.GetBaofuMerchantReportByOwnerParams{
			OwnerType:  db.BaofuAccountOwnerTypeMerchant,
			OwnerID:    merchant.ID,
			ReportType: db.BaofuMerchantReportTypeWechat,
		})).
		Return(db.BaofuMerchantReport{
			OwnerType:       db.BaofuAccountOwnerTypeMerchant,
			OwnerID:         merchant.ID,
			ReportType:      db.BaofuMerchantReportTypeWechat,
			ReportState:     db.BaofuMerchantReportStateSucceeded,
			AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending,
			SubMchID:        pgtype.Text{String: "1900000118", Valid: true},
		}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response baofuSettlementAccountResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, db.BaofuAccountOpeningStateAppletAuthPending, response.Status)
	require.False(t, response.PaymentReady)
	require.Equal(t, "***0118", response.WechatSubMchIDMask)
}

func TestBaofuSettlementAccountMerchantReportFailedReturnsSafeGuidance(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	binding := db.BaofuAccountBinding{
		ID:           95,
		OwnerType:    db.BaofuAccountOwnerTypeMerchant,
		OwnerID:      merchant.ID,
		AccountType:  db.BaofuAccountTypeBusiness,
		ContractNo:   pgtype.Text{String: "CM202605090095", Valid: true},
		SharingMerID: pgtype.Text{String: "CM202605090095", Valid: true},
		OpenState:    db.BaofuAccountOpenStateActive,
		UpdatedAt:    time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		})).
		Return(binding, nil)
	store.EXPECT().
		GetBaofuAccountOpeningProfileByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountOpeningProfileByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		})).
		Return(db.BaofuAccountOpeningProfile{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetLatestBaofuAccountOpeningFlowByOwner(gomock.Any(), gomock.Eq(db.GetLatestBaofuAccountOpeningFlowByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		})).
		Return(db.BaofuAccountOpeningFlow{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetBaofuMerchantReportByOwner(gomock.Any(), gomock.Eq(db.GetBaofuMerchantReportByOwnerParams{
			OwnerType:  db.BaofuAccountOwnerTypeMerchant,
			OwnerID:    merchant.ID,
			ReportType: db.BaofuMerchantReportTypeWechat,
		})).
		Return(db.BaofuMerchantReport{
			ID:              901,
			OwnerType:       db.BaofuAccountOwnerTypeMerchant,
			OwnerID:         merchant.ID,
			ReportType:      db.BaofuMerchantReportTypeWechat,
			ReportNo:        "MR202605090095",
			ReportState:     db.BaofuMerchantReportStateFailed,
			AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending,
			FailureCode:     pgtype.Text{String: "MERCHANT_REPORT_LIMIT", Valid: true},
			FailureMessage:  pgtype.Text{String: "raw upstream report failure: license 91330100MA00000001", Valid: true},
		}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response baofuSettlementAccountResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, db.BaofuAccountOpeningStateFailed, response.Status)
	require.Equal(t, "开通失败", response.Label)
	require.Equal(t, "微信支付商户报备失败，请核对商户资料后重试；如持续失败请联系平台处理", response.StatusDesc)
	require.False(t, response.PaymentReady)
	require.NotContains(t, recorder.Body.String(), "raw upstream report failure")
	require.NotContains(t, recorder.Body.String(), "91330100MA00000001")
	require.NotContains(t, recorder.Body.String(), "MERCHANT_REPORT_LIMIT")
}

func TestBaofuSettlementAccountMerchantAppletAuthFailedReturnsSafeGuidance(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	binding := db.BaofuAccountBinding{
		ID:           96,
		OwnerType:    db.BaofuAccountOwnerTypeMerchant,
		OwnerID:      merchant.ID,
		AccountType:  db.BaofuAccountTypeBusiness,
		ContractNo:   pgtype.Text{String: "CM202605090096", Valid: true},
		SharingMerID: pgtype.Text{String: "CM202605090096", Valid: true},
		OpenState:    db.BaofuAccountOpenStateActive,
		UpdatedAt:    time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		})).
		Return(binding, nil)
	store.EXPECT().
		GetBaofuAccountOpeningProfileByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountOpeningProfileByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		})).
		Return(db.BaofuAccountOpeningProfile{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetLatestBaofuAccountOpeningFlowByOwner(gomock.Any(), gomock.Eq(db.GetLatestBaofuAccountOpeningFlowByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		})).
		Return(db.BaofuAccountOpeningFlow{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetBaofuMerchantReportByOwner(gomock.Any(), gomock.Eq(db.GetBaofuMerchantReportByOwnerParams{
			OwnerType:  db.BaofuAccountOwnerTypeMerchant,
			OwnerID:    merchant.ID,
			ReportType: db.BaofuMerchantReportTypeWechat,
		})).
		Return(db.BaofuMerchantReport{
			ID:              902,
			OwnerType:       db.BaofuAccountOwnerTypeMerchant,
			OwnerID:         merchant.ID,
			ReportType:      db.BaofuMerchantReportTypeWechat,
			ReportNo:        "MR202605090096",
			ReportState:     db.BaofuMerchantReportStateSucceeded,
			AppletAuthState: db.BaofuMerchantReportAppletAuthStateFailed,
			SubMchID:        pgtype.Text{String: "1900000196", Valid: true},
			FailureCode:     pgtype.Text{String: "NO_AUTH", Valid: true},
			FailureMessage:  pgtype.Text{String: "raw upstream auth failure for appid wx123456", Valid: true},
		}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response baofuSettlementAccountResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, db.BaofuAccountOpeningStateFailed, response.Status)
	require.Equal(t, "微信支付授权目录绑定失败，请联系平台处理后重试", response.StatusDesc)
	require.Equal(t, "***0196", response.WechatSubMchIDMask)
	require.False(t, response.PaymentReady)
	require.NotContains(t, recorder.Body.String(), "raw upstream auth failure")
	require.NotContains(t, recorder.Body.String(), "wx123456")
	require.NotContains(t, recorder.Body.String(), "NO_AUTH")
}

func TestBaofuSettlementAccountMerchantPostAppletAuthFailedReturnsSafeGuidance(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	binding := db.BaofuAccountBinding{
		ID:           97,
		OwnerType:    db.BaofuAccountOwnerTypeMerchant,
		OwnerID:      merchant.ID,
		AccountType:  db.BaofuAccountTypeBusiness,
		ContractNo:   pgtype.Text{String: "CM202605090097", Valid: true},
		SharingMerID: pgtype.Text{String: "CM202605090097", Valid: true},
		OpenState:    db.BaofuAccountOpenStateActive,
		UpdatedAt:    time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		})).
		Return(binding, nil)
	store.EXPECT().
		GetBaofuMerchantReportByOwner(gomock.Any(), gomock.Eq(db.GetBaofuMerchantReportByOwnerParams{
			OwnerType:  db.BaofuAccountOwnerTypeMerchant,
			OwnerID:    merchant.ID,
			ReportType: db.BaofuMerchantReportTypeWechat,
		})).
		Return(db.BaofuMerchantReport{
			ID:              903,
			OwnerType:       db.BaofuAccountOwnerTypeMerchant,
			OwnerID:         merchant.ID,
			ReportType:      db.BaofuMerchantReportTypeWechat,
			ReportNo:        "MR202605090097",
			ReportState:     db.BaofuMerchantReportStateSucceeded,
			AppletAuthState: db.BaofuMerchantReportAppletAuthStateFailed,
			SubMchID:        pgtype.Text{String: "1900000197", Valid: true},
			FailureCode:     pgtype.Text{String: "NO_AUTH", Valid: true},
			FailureMessage:  pgtype.Text{String: "raw upstream auth failure for appid wx123456", Valid: true},
		}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/settlement-account", bytes.NewBufferString(`{}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusAccepted, recorder.Code)
	var response baofuSettlementAccountResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, db.BaofuAccountOpeningStateFailed, response.Status)
	require.Equal(t, "微信支付授权目录绑定失败，请联系平台处理后重试", response.StatusDesc)
	require.False(t, response.PaymentReady)
	require.NotContains(t, recorder.Body.String(), "raw upstream auth failure")
	require.NotContains(t, recorder.Body.String(), "NO_AUTH")
}

func TestBaofuSettlementAccountRequestErrorLogIncludesMerchantReportContext(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	providerErr := &baofu.ProviderError{
		Operation:       "bind_sub_config",
		Capability:      "baofu_merchant_report",
		UpstreamCode:    "NO_AUTH",
		UpstreamMessage: "raw upstream auth failure for appid wx123456",
	}
	err := logic.NewBaofuProviderContextError(providerErr, logic.BaofuProviderErrorContext{
		FlowID:             777,
		OwnerType:          db.BaofuAccountOwnerTypeMerchant,
		OwnerID:            88,
		OpenTransSerialNo:  "BFO202605090777",
		CurrentState:       db.BaofuAccountOpeningStateAppletAuthPending,
		MerchantReportID:   999,
		MerchantReportNo:   "MR202605090777",
		ProviderOperation:  "bind_sub_config",
		ProviderCapability: "baofu_merchant_report",
	})
	err = logic.NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("微信支付授权目录绑定失败，请联系平台处理后重试"), err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/merchant/settlement-account", nil)
	ctx.Set(RequestIDKey, "req-baofu-report-context")

	ok := writeBaofuSettlementAccountLogicRequestError(ctx, err, baofuSettlementAccountScope{
		OwnerType:   db.BaofuAccountOwnerTypeMerchant,
		OwnerID:     88,
		AccountType: db.BaofuAccountTypeBusiness,
		Audience:    "merchant",
	})

	require.True(t, ok)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.NotContains(t, recorder.Body.String(), providerErr.UpstreamMessage)
	require.Contains(t, recorder.Body.String(), "微信支付授权目录绑定失败")
	require.Contains(t, logs.String(), `"flow_id":777`)
	require.Contains(t, logs.String(), `"merchant_report_id":999`)
	require.Contains(t, logs.String(), `"merchant_report_no":"MR202605090777"`)
	require.Contains(t, logs.String(), `"provider_operation":"bind_sub_config"`)
	require.Contains(t, logs.String(), `"current_state":"applet_auth_pending"`)
	require.Contains(t, logs.String(), `"owner_type":"merchant"`)
	require.Contains(t, logs.String(), `"owner_id":88`)
}

func TestBaofuSettlementAccountMerchantManagerCannotPost(t *testing.T) {
	owner, _ := randomUser(t)
	manager, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleStaffMerchant(store, manager.ID, merchant)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/settlement-account", bytes.NewBufferString(`{"out_request_no":"REQ-1"}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, manager.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestBaofuSettlementAccountMerchantManagerCannotRead(t *testing.T) {
	owner, _ := randomUser(t)
	manager, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleStaffMerchant(store, manager.ID, merchant)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/merchant/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, manager.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestBaofuSettlementAccountRejectsClientControlledFieldsBeforeServiceCall(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.Status = "approved"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
		Return(rider, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/rider/settlement-account", bytes.NewBufferString(`{"owner_type":"platform"}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "服务端生成")
	require.NotContains(t, recorder.Body.String(), "owner_type is controlled by server")
}

func TestBaofuSettlementAccountRiderRejectsBusinessProfileFieldsBeforeServiceCall(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.Status = "approved"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
		Return(rider, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/rider/settlement-account", bytes.NewBufferString(`{"profile":{"business_license_number":"91330100MA00000001"}}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "开户资料字段不适用于当前角色")
	require.NotContains(t, recorder.Body.String(), "business_license_number is not allowed")
}

func TestBaofuSettlementAccountMerchantRejectsPersonalProfileAliasFieldsBeforeServiceCall(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/settlement-account", bytes.NewBufferString(`{"profile":{"id_card_number":"110101199001010011"}}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "开户资料字段不适用于当前角色")
	require.NotContains(t, recorder.Body.String(), "id_card_number is not allowed")
}

func TestBaofuSettlementAccountOperatorCanReadActiveBinding(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)
	binding := db.BaofuAccountBinding{
		ID:          195,
		OwnerType:   db.BaofuAccountOwnerTypeOperator,
		OwnerID:     operator.ID,
		AccountType: db.BaofuAccountTypePersonal,
		OpenState:   db.BaofuAccountOpenStateActive,
		UpdatedAt:   time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeOperator,
			OwnerID:   operator.ID,
		})).
		Return(binding, nil)
	store.EXPECT().
		GetBaofuAccountOpeningProfileByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountOpeningProfileByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeOperator,
			OwnerID:   operator.ID,
		})).
		Return(db.BaofuAccountOpeningProfile{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetLatestBaofuAccountOpeningFlowByOwner(gomock.Any(), gomock.Eq(db.GetLatestBaofuAccountOpeningFlowByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeOperator,
			OwnerID:   operator.ID,
		})).
		Return(db.BaofuAccountOpeningFlow{}, db.ErrRecordNotFound)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/operators/me/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response baofuSettlementAccountResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, db.BaofuAccountOwnerTypeOperator, response.OwnerType)
	require.Equal(t, operator.ID, response.OwnerID)
	require.Equal(t, db.BaofuAccountTypePersonal, response.AccountType)
	require.Equal(t, db.BaofuAccountOpeningStateReady, response.Status)
	require.True(t, response.PaymentReady)
	require.Empty(t, response.WechatSubMchIDMask)
}

func TestBaofuSettlementAccountOperatorRejectsClientControlledFieldsBeforeServiceCall(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/operators/me/settlement-account", bytes.NewBufferString(`{"profile":{"platformNo":"P1"}}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "服务端生成")
	require.NotContains(t, recorder.Body.String(), "platformNo is controlled by server")
}

func TestBaofuSettlementAccountPlatformCanReadActiveBinding(t *testing.T) {
	admin, _ := randomUser(t)
	binding := db.BaofuAccountBinding{
		ID:          196,
		OwnerType:   db.BaofuAccountOwnerTypePlatform,
		OwnerID:     platformBaofuAccountOwnerID,
		AccountType: db.BaofuAccountTypeBusiness,
		OpenState:   db.BaofuAccountOpenStateActive,
		UpdatedAt:   time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypePlatform,
			OwnerID:   platformBaofuAccountOwnerID,
		})).
		Return(binding, nil)
	store.EXPECT().
		GetBaofuAccountOpeningProfileByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountOpeningProfileByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypePlatform,
			OwnerID:   platformBaofuAccountOwnerID,
		})).
		Return(db.BaofuAccountOpeningProfile{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetLatestBaofuAccountOpeningFlowByOwner(gomock.Any(), gomock.Eq(db.GetLatestBaofuAccountOpeningFlowByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypePlatform,
			OwnerID:   platformBaofuAccountOwnerID,
		})).
		Return(db.BaofuAccountOpeningFlow{}, db.ErrRecordNotFound)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/platform/finance/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response baofuSettlementAccountResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, db.BaofuAccountOwnerTypePlatform, response.OwnerType)
	require.Equal(t, platformBaofuAccountOwnerID, response.OwnerID)
	require.Equal(t, db.BaofuAccountTypeBusiness, response.AccountType)
	require.Equal(t, db.BaofuAccountOpeningStateReady, response.Status)
	require.True(t, response.PaymentReady)
	require.Empty(t, response.WechatSubMchIDMask)
}

func TestBaofuSettlementAccountPlatformRejectsClientControlledFieldsBeforeServiceCall(t *testing.T) {
	admin, _ := randomUser(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), admin.ID).
		Return([]db.UserRole{{UserID: admin.ID, Role: RoleAdmin, Status: "active"}}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodPost, "/v1/platform/finance/settlement-account", bytes.NewBufferString(`{"owner_type":"merchant"}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, admin.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "服务端生成")
	require.NotContains(t, recorder.Body.String(), "owner_type is controlled by server")
}

func TestBaofuSettlementAccountPaymentLookupFailureIsLoggedAndNotDowngraded(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.Status = "approved"
	rawErr := errors.New("database secret payment lookup failure")
	flowUpdated := time.Now()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
		Return(rider, nil)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetBaofuAccountOpeningProfileByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountOpeningProfileByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(db.BaofuAccountOpeningProfile{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetLatestBaofuAccountOpeningFlowByOwner(gomock.Any(), gomock.Eq(db.GetLatestBaofuAccountOpeningFlowByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(db.BaofuAccountOpeningFlow{
			ID:                      403,
			OwnerType:               db.BaofuAccountOwnerTypeRider,
			OwnerID:                 rider.ID,
			AccountType:             db.BaofuAccountTypePersonal,
			State:                   db.BaofuAccountOpeningStateVerifyFeeProcessing,
			VerifyFeePaymentOrderID: pgtype.Int8{Int64: 503, Valid: true},
			CreatedAt:               flowUpdated,
			UpdatedAt:               flowUpdated,
		}, nil)
	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(503)).
		Return(db.PaymentOrder{}, rawErr)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/rider/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "宝付结算账户状态暂不可用，请稍后刷新")
	require.NotContains(t, recorder.Body.String(), rawErr.Error())
	require.Contains(t, logs.String(), rawErr.Error())
	require.Contains(t, logs.String(), `"payment_order_id":503`)
}

func TestBaofuSettlementAccountPayParamsFailureIsLoggedAndNotDowngraded(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.Status = "approved"
	rawErr := errors.New("wechat signing secret failure")
	flowUpdated := time.Now()
	payment := db.PaymentOrder{
		ID:           504,
		UserID:       user.ID,
		BusinessType: db.PaymentBusinessTypeBaofuAccountVerifyFee,
		Amount:       200,
		OutTradeNo:   "BFV202605090504",
		Status:       "pending",
		PrepayID:     pgtype.Text{String: "prepay-baofu-504", Valid: true},
		CreatedAt:    flowUpdated,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	store.EXPECT().
		GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
		Return(rider, nil)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetBaofuAccountOpeningProfileByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountOpeningProfileByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(db.BaofuAccountOpeningProfile{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetLatestBaofuAccountOpeningFlowByOwner(gomock.Any(), gomock.Eq(db.GetLatestBaofuAccountOpeningFlowByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(db.BaofuAccountOpeningFlow{
			ID:                      404,
			OwnerType:               db.BaofuAccountOwnerTypeRider,
			OwnerID:                 rider.ID,
			AccountType:             db.BaofuAccountTypePersonal,
			State:                   db.BaofuAccountOpeningStateVerifyFeeProcessing,
			VerifyFeePaymentOrderID: pgtype.Int8{Int64: payment.ID, Valid: true},
			CreatedAt:               flowUpdated,
			UpdatedAt:               flowUpdated,
		}, nil)
	store.EXPECT().
		GetPaymentOrder(gomock.Any(), payment.ID).
		Return(payment, nil)
	paymentClient.EXPECT().
		GenerateJSAPIPayParams("prepay-baofu-504").
		Return(nil, rawErr)

	server := newTestServer(t, store)
	server.SetDirectPaymentClientForTest(paymentClient)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/rider/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "宝付结算账户状态暂不可用，请稍后刷新")
	require.NotContains(t, recorder.Body.String(), rawErr.Error())
	require.NotContains(t, recorder.Body.String(), "BFV202605090504")
	require.Contains(t, logs.String(), rawErr.Error())
	require.Contains(t, logs.String(), `"flow_id":404`)
	require.Contains(t, logs.String(), `"payment_order_id":504`)
}

func TestBaofuSettlementAccountRiderVerifyFeePendingReturnsRetryGuidance(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.Status = "approved"
	flowCreated := time.Now().Add(-time.Minute)
	flowUpdated := time.Now()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
		Return(rider, nil)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetBaofuAccountOpeningProfileByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountOpeningProfileByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(db.BaofuAccountOpeningProfile{
			ID:                301,
			OwnerType:         db.BaofuAccountOwnerTypeRider,
			OwnerID:           rider.ID,
			AccountType:       db.BaofuAccountTypePersonal,
			ProfileStatus:     db.BaofuAccountOpeningProfileStatusComplete,
			BankAccountNoMask: pgtype.Text{String: "6222********1234", Valid: true},
			BankMobileMask:    pgtype.Text{String: "138****0000", Valid: true},
			CertificateNoMask: pgtype.Text{String: "***1234", Valid: true},
			SourceSnapshot:    []byte(`{}`),
			CreatedAt:         flowCreated,
			UpdatedAt:         flowUpdated,
		}, nil)
	store.EXPECT().
		GetLatestBaofuAccountOpeningFlowByOwner(gomock.Any(), gomock.Eq(db.GetLatestBaofuAccountOpeningFlowByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(db.BaofuAccountOpeningFlow{
			ID:                      401,
			OwnerType:               db.BaofuAccountOwnerTypeRider,
			OwnerID:                 rider.ID,
			AccountType:             db.BaofuAccountTypePersonal,
			ProfileID:               pgtype.Int8{Int64: 301, Valid: true},
			State:                   db.BaofuAccountOpeningStateVerifyFeePending,
			VerifyFeeAmount:         200,
			VerifyFeePaymentOrderID: pgtype.Int8{},
			ProviderRequestSnapshot: []byte(`{}`),
			RawSnapshot:             []byte(`{"retry_guidance":"支付未完成，请重新支付开户核验费。"}`),
			CreatedAt:               flowCreated,
			UpdatedAt:               flowUpdated,
		}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/rider/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response baofuSettlementAccountResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, db.BaofuAccountOpeningStateVerifyFeePending, response.Status)
	require.Equal(t, "核验费待确认", response.Label)
	require.Equal(t, "支付 2 元核验费后继续开户，支付未完成可重新发起支付", response.StatusDesc)
	require.False(t, response.PaymentReady)
	require.Equal(t, int64(200), response.VerifyFeeAmount)
	require.Nil(t, response.Payment)
}

func TestBaofuSettlementAccountRiderProfilePendingReturnsMissingFieldGuidance(t *testing.T) {
	user, _ := randomUser(t)
	rider := randomRider(user.ID)
	rider.Status = "approved"
	updatedAt := time.Now()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
		Return(rider, nil)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetBaofuAccountOpeningProfileByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountOpeningProfileByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(db.BaofuAccountOpeningProfile{
			ID:                      302,
			OwnerType:               db.BaofuAccountOwnerTypeRider,
			OwnerID:                 rider.ID,
			AccountType:             db.BaofuAccountTypePersonal,
			ProfileStatus:           db.BaofuAccountOpeningProfileStatusIncomplete,
			LegalName:               pgtype.Text{String: "张三", Valid: true},
			CertificateNoCiphertext: pgtype.Text{String: "110101199001010011", Valid: true},
			BankAccountNoCiphertext: pgtype.Text{},
			BankMobileCiphertext:    pgtype.Text{},
			UpdatedAt:               updatedAt,
		}, nil)
	store.EXPECT().
		GetLatestBaofuAccountOpeningFlowByOwner(gomock.Any(), gomock.Eq(db.GetLatestBaofuAccountOpeningFlowByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(db.BaofuAccountOpeningFlow{
			ID:        402,
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
			State:     db.BaofuAccountOpeningStateProfilePending,
			CreatedAt: updatedAt,
			UpdatedAt: updatedAt,
		}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/rider/settlement-account", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response baofuSettlementAccountResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, db.BaofuAccountOpeningStateProfilePending, response.Status)
	require.ElementsMatch(t, []string{"bank_account_no", "bank_mobile"}, response.MissingFields)
	require.Contains(t, response.StatusDesc, "请补充开户资料")
	require.Contains(t, response.StatusDesc, "银行卡/对公账号")
	require.Contains(t, response.StatusDesc, "银行预留手机号")
	require.NotContains(t, recorder.Body.String(), "110101199001010011")
}

func TestBaofuSettlementAccountRiderPostCreatesVerifyFeeBeforeBaofuOpening(t *testing.T) {
	user, _ := randomUser(t)
	user.WechatOpenid = "openid-rider-baofu"
	rider := randomRider(user.ID)
	rider.Status = "approved"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	store.EXPECT().
		GetRiderByUserID(gomock.Any(), gomock.Eq(user.ID)).
		Return(rider, nil)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)
	store.EXPECT().
		UpsertBaofuAccountOpeningProfile(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.UpsertBaofuAccountOpeningProfileParams) (db.BaofuAccountOpeningProfile, error) {
			require.Equal(t, db.BaofuAccountOwnerTypeRider, arg.OwnerType)
			require.Equal(t, rider.ID, arg.OwnerID)
			require.Equal(t, db.BaofuAccountTypePersonal, arg.AccountType)
			require.Equal(t, db.BaofuAccountOpeningProfileStatusComplete, arg.ProfileStatus)
			require.Equal(t, "张三", arg.LegalName.String)
			require.Equal(t, "110101199001010011", arg.CertificateNoCiphertext.String)
			require.Equal(t, "6222020202020202", arg.BankAccountNoCiphertext.String)
			require.Equal(t, "13800138000", arg.BankMobileCiphertext.String)
			return db.BaofuAccountOpeningProfile{
				ID:                      301,
				OwnerType:               arg.OwnerType,
				OwnerID:                 arg.OwnerID,
				AccountType:             arg.AccountType,
				ProfileStatus:           arg.ProfileStatus,
				LegalName:               arg.LegalName,
				CertificateType:         arg.CertificateType,
				CertificateNoCiphertext: arg.CertificateNoCiphertext,
				BankAccountNoCiphertext: arg.BankAccountNoCiphertext,
				BankMobileCiphertext:    arg.BankMobileCiphertext,
				CardUserName:            arg.CardUserName,
				CreatedAt:               time.Now(),
				UpdatedAt:               time.Now(),
			}, nil
		})
	store.EXPECT().
		GetActiveBaofuAccountOpeningFlowByOwner(gomock.Any(), gomock.Eq(db.GetActiveBaofuAccountOpeningFlowByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeRider,
			OwnerID:   rider.ID,
		})).
		Return(db.BaofuAccountOpeningFlow{}, db.ErrRecordNotFound)
	createdFlow := db.BaofuAccountOpeningFlow{
		ID:          401,
		OwnerType:   db.BaofuAccountOwnerTypeRider,
		OwnerID:     rider.ID,
		AccountType: db.BaofuAccountTypePersonal,
		ProfileID:   pgtype.Int8{Int64: 301, Valid: true},
		State:       db.BaofuAccountOpeningStateProfilePending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	store.EXPECT().
		CreateBaofuAccountOpeningFlow(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateBaofuAccountOpeningFlowParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, db.BaofuAccountOwnerTypeRider, arg.OwnerType)
			require.Equal(t, rider.ID, arg.OwnerID)
			require.Equal(t, db.BaofuAccountTypePersonal, arg.AccountType)
			require.Equal(t, db.BaofuAccountOpeningStateProfilePending, arg.State)
			return createdFlow, nil
		})
	verifyPendingFlow := createdFlow
	verifyPendingFlow.State = db.BaofuAccountOpeningStateVerifyFeePending
	verifyPendingFlow.VerifyFeeAmount = 200
	store.EXPECT().
		MarkBaofuAccountOpeningFlowVerifyFeePending(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowVerifyFeePendingParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, createdFlow.ID, arg.ID)
			require.Equal(t, int64(200), arg.VerifyFeeAmount)
			return verifyPendingFlow, nil
		})
	store.EXPECT().
		GetReusableBaofuVerifyFeePayment(gomock.Any(), gomock.Any()).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetUser(gomock.Any(), gomock.Eq(user.ID)).
		Return(user, nil)
	payment := db.PaymentOrder{
		ID:                    501,
		UserID:                user.ID,
		PaymentType:           "miniprogram",
		PaymentChannel:        db.PaymentChannelDirect,
		RequiresProfitSharing: false,
		BusinessType:          db.PaymentBusinessTypeBaofuAccountVerifyFee,
		Amount:                200,
		OutTradeNo:            "BFV202605080001",
		Status:                "pending",
		Attach:                pgtype.Text{String: "business:baofu_account_verify_fee;owner_type:rider;owner_id:" + strconv.FormatInt(rider.ID, 10) + ";purpose:initial_open", Valid: true},
		CreatedAt:             time.Now(),
		ExpiresAt:             pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	}
	store.EXPECT().
		CreatePaymentOrder(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreatePaymentOrderParams) (db.PaymentOrder, error) {
			require.Equal(t, user.ID, arg.UserID)
			require.Equal(t, db.PaymentBusinessTypeBaofuAccountVerifyFee, arg.BusinessType)
			require.Equal(t, db.PaymentChannelDirect, arg.PaymentChannel)
			require.Equal(t, int64(200), arg.Amount)
			payment.OutTradeNo = arg.OutTradeNo
			payment.ExpiresAt = arg.ExpiresAt
			payment.Attach = arg.Attach
			return payment, nil
		})
	paymentClient.EXPECT().
		CreateJSAPIOrder(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechatcontracts.DirectJSAPIOrderRequest) (*wechatcontracts.DirectJSAPIOrderResponse, *wechat.JSAPIPayParams, error) {
			require.Equal(t, db.PaymentBusinessTypeBaofuAccountVerifyFee, "baofu_account_verify_fee")
			require.Equal(t, payment.OutTradeNo, req.OutTradeNo)
			require.Equal(t, int64(200), req.TotalAmount)
			require.Equal(t, "openid-rider-baofu", req.PayerOpenID)
			return &wechatcontracts.DirectJSAPIOrderResponse{PrepayID: "prepay-baofu-verify"}, &wechat.JSAPIPayParams{Package: "prepay_id=prepay-baofu-verify"}, nil
		})
	updatedPayment := payment
	updatedPayment.PrepayID = pgtype.Text{String: "prepay-baofu-verify", Valid: true}
	store.EXPECT().
		UpdatePaymentOrderPrepayId(gomock.Any(), gomock.Eq(db.UpdatePaymentOrderPrepayIdParams{ID: payment.ID, PrepayID: pgtype.Text{String: "prepay-baofu-verify", Valid: true}})).
		Return(updatedPayment, nil)
	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).
		Return(db.ExternalPaymentCommand{ID: 601}, nil)
	processingFlow := verifyPendingFlow
	processingFlow.State = db.BaofuAccountOpeningStateVerifyFeeProcessing
	processingFlow.VerifyFeePaymentOrderID = pgtype.Int8{Int64: updatedPayment.ID, Valid: true}
	store.EXPECT().
		MarkBaofuAccountOpeningFlowVerifyFeeProcessing(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkBaofuAccountOpeningFlowVerifyFeeProcessingParams) (db.BaofuAccountOpeningFlow, error) {
			require.Equal(t, verifyPendingFlow.ID, arg.ID)
			require.Equal(t, updatedPayment.ID, arg.VerifyFeePaymentOrderID.Int64)
			return processingFlow, nil
		})

	server := newTestServer(t, store)
	server.SetDirectPaymentClientForTest(paymentClient)
	server.baofuAccountClient = &fakeBaofuSettlementAccountClient{}
	recorder := httptest.NewRecorder()
	body := map[string]any{
		"profile": map[string]any{
			"real_name":           "张三",
			"id_card_number":      "110101199001010011",
			"bank_account_number": "6222020202020202",
			"mobile":              "13800138000",
			"bank_name":           "招商银行",
		},
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, "/v1/rider/settlement-account", bytes.NewReader(rawBody))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusAccepted, recorder.Code)
	var response baofuSettlementAccountResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, db.BaofuAccountOwnerTypeRider, response.OwnerType)
	require.Equal(t, rider.ID, response.OwnerID)
	require.Equal(t, db.BaofuAccountTypePersonal, response.AccountType)
	require.Equal(t, db.BaofuAccountOpeningStateVerifyFeeProcessing, response.Status)
	require.Equal(t, updatedPayment.ID, response.PaymentOrderID)
	require.NotNil(t, response.PayParams)
	require.Equal(t, "prepay_id=prepay-baofu-verify", response.PayParams.Package)
}

func TestBaofuSettlementAccountMerchantPostBaofooProviderFailureReturnsSafeGuidance(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	providerErr := &baofu.ProviderError{
		Operation:       "open_account",
		Capability:      "baofu_account_open",
		StatusCode:      http.StatusBadGateway,
		UpstreamCode:    "BF00061",
		UpstreamMessage: "raw upstream id card failure: 110101199001010011",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		})).
		Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)
	store.EXPECT().
		UpsertBaofuAccountOpeningProfile(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.UpsertBaofuAccountOpeningProfileParams) (db.BaofuAccountOpeningProfile, error) {
			require.Equal(t, db.BaofuAccountOwnerTypeMerchant, arg.OwnerType)
			require.Equal(t, merchant.ID, arg.OwnerID)
			require.Equal(t, db.BaofuAccountTypeBusiness, arg.AccountType)
			require.Equal(t, db.BaofuAccountOpeningProfileStatusComplete, arg.ProfileStatus)
			return db.BaofuAccountOpeningProfile{
				ID:                        311,
				OwnerType:                 arg.OwnerType,
				OwnerID:                   arg.OwnerID,
				AccountType:               arg.AccountType,
				ProfileStatus:             arg.ProfileStatus,
				LegalName:                 arg.LegalName,
				CertificateType:           arg.CertificateType,
				CertificateNoCiphertext:   arg.CertificateNoCiphertext,
				EmailCiphertext:           arg.EmailCiphertext,
				CorporateName:             arg.CorporateName,
				CorporateCertType:         arg.CorporateCertType,
				CorporateCertIDCiphertext: arg.CorporateCertIDCiphertext,
				IndustryID:                arg.IndustryID,
				BankAccountNoCiphertext:   arg.BankAccountNoCiphertext,
				BankName:                  arg.BankName,
				DepositBankProvince:       arg.DepositBankProvince,
				DepositBankCity:           arg.DepositBankCity,
				DepositBankName:           arg.DepositBankName,
				CreatedAt:                 time.Now(),
				UpdatedAt:                 time.Now(),
			}, nil
		})
	store.EXPECT().
		GetActiveBaofuAccountOpeningFlowByOwner(gomock.Any(), gomock.Eq(db.GetActiveBaofuAccountOpeningFlowByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		})).
		Return(db.BaofuAccountOpeningFlow{}, db.ErrRecordNotFound)
	createdFlow := db.BaofuAccountOpeningFlow{
		ID:          411,
		OwnerType:   db.BaofuAccountOwnerTypeMerchant,
		OwnerID:     merchant.ID,
		AccountType: db.BaofuAccountTypeBusiness,
		ProfileID:   pgtype.Int8{Int64: 311, Valid: true},
		State:       db.BaofuAccountOpeningStateProfilePending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	store.EXPECT().
		CreateBaofuAccountOpeningFlow(gomock.Any(), gomock.Any()).
		Return(createdFlow, nil)
	processingFlow := createdFlow
	processingFlow.State = db.BaofuAccountOpeningStateOpeningProcessing
	processingFlow.OpenTransSerialNo = pgtype.Text{String: "BFO202605090001", Valid: true}
	processingFlow.LoginNo = pgtype.Text{String: "LLBFOM0000000001", Valid: true}
	store.EXPECT().
		MarkBaofuAccountOpeningFlowOpeningProcessing(gomock.Any(), gomock.Any()).
		Return(processingFlow, nil)
	store.EXPECT().
		UpsertBaofuAccountBinding(gomock.Any(), gomock.Any()).
		Return(db.BaofuAccountBinding{
			ID:          511,
			OwnerType:   db.BaofuAccountOwnerTypeMerchant,
			OwnerID:     merchant.ID,
			AccountType: db.BaofuAccountTypeBusiness,
			OpenState:   db.BaofuAccountOpenStateProcessing,
		}, nil)

	server := newTestServer(t, store)
	server.baofuAccountClient = &fakeBaofuSettlementAccountClient{openErr: providerErr}
	recorder := httptest.NewRecorder()
	rawBody, err := json.Marshal(completeMerchantBaofuSettlementProfileBody())
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/settlement-account", bytes.NewReader(rawBody))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.NotContains(t, recorder.Body.String(), providerErr.UpstreamMessage)
	require.NotContains(t, recorder.Body.String(), providerErr.UpstreamCode)
	require.NotContains(t, recorder.Body.String(), "110101199001010011")
	require.Contains(t, recorder.Body.String(), "身份或银行卡信息核验未通过，请核对后重新提交")
}

func TestBaofuSettlementAccountMerchantPostUnexpectedOpenFailureReturnsSafeGuidance(t *testing.T) {
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	rawErr := errors.New("internal baofu transport secret failure")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, owner.ID, merchant)
	store.EXPECT().
		GetBaofuAccountBindingByOwner(gomock.Any(), gomock.Eq(db.GetBaofuAccountBindingByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		})).
		Return(db.BaofuAccountBinding{}, db.ErrRecordNotFound)
	store.EXPECT().
		UpsertBaofuAccountOpeningProfile(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.UpsertBaofuAccountOpeningProfileParams) (db.BaofuAccountOpeningProfile, error) {
			return db.BaofuAccountOpeningProfile{
				ID:                        312,
				OwnerType:                 arg.OwnerType,
				OwnerID:                   arg.OwnerID,
				AccountType:               arg.AccountType,
				ProfileStatus:             arg.ProfileStatus,
				LegalName:                 arg.LegalName,
				CertificateType:           arg.CertificateType,
				CertificateNoCiphertext:   arg.CertificateNoCiphertext,
				EmailCiphertext:           arg.EmailCiphertext,
				CorporateName:             arg.CorporateName,
				CorporateCertType:         arg.CorporateCertType,
				CorporateCertIDCiphertext: arg.CorporateCertIDCiphertext,
				IndustryID:                arg.IndustryID,
				BankAccountNoCiphertext:   arg.BankAccountNoCiphertext,
				BankName:                  arg.BankName,
				DepositBankProvince:       arg.DepositBankProvince,
				DepositBankCity:           arg.DepositBankCity,
				DepositBankName:           arg.DepositBankName,
				CreatedAt:                 time.Now(),
				UpdatedAt:                 time.Now(),
			}, nil
		})
	store.EXPECT().
		GetActiveBaofuAccountOpeningFlowByOwner(gomock.Any(), gomock.Eq(db.GetActiveBaofuAccountOpeningFlowByOwnerParams{
			OwnerType: db.BaofuAccountOwnerTypeMerchant,
			OwnerID:   merchant.ID,
		})).
		Return(db.BaofuAccountOpeningFlow{}, db.ErrRecordNotFound)
	createdFlow := db.BaofuAccountOpeningFlow{
		ID:          412,
		OwnerType:   db.BaofuAccountOwnerTypeMerchant,
		OwnerID:     merchant.ID,
		AccountType: db.BaofuAccountTypeBusiness,
		ProfileID:   pgtype.Int8{Int64: 312, Valid: true},
		State:       db.BaofuAccountOpeningStateProfilePending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	store.EXPECT().
		CreateBaofuAccountOpeningFlow(gomock.Any(), gomock.Any()).
		Return(createdFlow, nil)
	processingFlow := createdFlow
	processingFlow.State = db.BaofuAccountOpeningStateOpeningProcessing
	processingFlow.OpenTransSerialNo = pgtype.Text{String: "BFO202605090002", Valid: true}
	processingFlow.LoginNo = pgtype.Text{String: "LLBFOM0000000002", Valid: true}
	store.EXPECT().
		MarkBaofuAccountOpeningFlowOpeningProcessing(gomock.Any(), gomock.Any()).
		Return(processingFlow, nil)
	store.EXPECT().
		UpsertBaofuAccountBinding(gomock.Any(), gomock.Any()).
		Return(db.BaofuAccountBinding{
			ID:          512,
			OwnerType:   db.BaofuAccountOwnerTypeMerchant,
			OwnerID:     merchant.ID,
			AccountType: db.BaofuAccountTypeBusiness,
			OpenState:   db.BaofuAccountOpenStateProcessing,
		}, nil)

	server := newTestServer(t, store)
	server.baofuAccountClient = &fakeBaofuSettlementAccountClient{openErr: rawErr}
	recorder := httptest.NewRecorder()
	rawBody, err := json.Marshal(completeMerchantBaofuSettlementProfileBody())
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, "/v1/merchant/settlement-account", bytes.NewReader(rawBody))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.NotContains(t, recorder.Body.String(), rawErr.Error())
	require.Contains(t, recorder.Body.String(), "宝付开户服务暂不可用，请稍后重试")
}

func completeMerchantBaofuSettlementProfileBody() map[string]any {
	return map[string]any{
		"profile": map[string]any{
			"legal_name":              "测试商户",
			"business_license_number": "91330100MA00000001",
			"legal_person_name":       "李四",
			"legal_person_id_number":  "110101199001010011",
			"email":                   "merchant@example.com",
			"bank_account_number":     "6222020202020202",
			"bank_name":               "招商银行",
			"deposit_bank_province":   "浙江省",
			"deposit_bank_city":       "杭州市",
			"deposit_bank_name":       "招商银行杭州支行",
		},
	}
}

type fakeBaofuSettlementAccountClient struct {
	openCalls int
	openErr   error
}

func (c *fakeBaofuSettlementAccountClient) OpenAccount(context.Context, baofucontracts.OpenAccountRequest) (*baofucontracts.AccountResult, error) {
	c.openCalls++
	if c.openErr != nil {
		return nil, c.openErr
	}
	return nil, nil
}

func (c *fakeBaofuSettlementAccountClient) QueryAccount(context.Context, baofucontracts.QueryAccountRequest) (*baofucontracts.AccountResult, error) {
	return nil, nil
}
