package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	merchantcontracts "github.com/merrydance/locallife/baofu/merchantreport/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

func TestBaofuAccountOnboardingServiceStart_MissingProfileDoesNotCallBaofu(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	service := NewBaofuAccountOnboardingService(store, &fakeBaofuOnboardingAccountClient{}, nil, nil, BaofuAccountOnboardingConfig{
		VerifyFeeFen: 200,
		IndustryID:   "9931",
	})

	result, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   88,
		UserID:    99,
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateProfilePending, result.State)
	require.Zero(t, store.openAccountCalls)
	require.Len(t, store.flows, 1)
	require.Equal(t, db.BaofuAccountOpeningStateProfilePending, store.flows[0].State)
	require.ElementsMatch(t, []string{
		"legal_name",
		"business_license_number",
		"legal_person_name",
		"legal_person_id_number",
		"email",
		"bank_account_no",
		"bank_name",
		"deposit_bank_province",
		"deposit_bank_city",
		"deposit_bank_name",
	}, result.MissingFields)
	require.Contains(t, result.StatusDesc, "请补充开户资料")
	require.Contains(t, result.StatusDesc, "企业名称")
	require.Contains(t, result.StatusDesc, "开户支行")
}

func TestBaofuAccountOnboardingServiceStart_ProviderOpenErrorBecomesSafeRequestError(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	providerErr := &baofu.ProviderError{
		Operation:       "open_account",
		Capability:      "baofu_account_open",
		UpstreamCode:    "BF00061",
		UpstreamMessage: "raw upstream id card failure: 110101199001010011",
	}
	accountClient := &fakeBaofuOnboardingAccountClient{err: providerErr}
	service := NewBaofuAccountOnboardingService(store, accountClient, nil, nil, BaofuAccountOnboardingConfig{
		VerifyFeeFen: 200,
		IndustryID:   "9931",
	})

	_, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   88,
		UserID:    99,
		Profile: &BaofuAccountOpeningProfileInput{
			LegalName:           "测试商户",
			BusinessLicenseNo:   "91330100MA00000001",
			LegalPersonName:     "李四",
			LegalPersonIDNumber: "110101199001010011",
			Email:               "merchant@example.com",
			BankAccountNo:       "6222020202020202",
			BankName:            "招商银行",
			DepositBankProvince: "浙江省",
			DepositBankCity:     "杭州市",
			DepositBankName:     "招商银行杭州支行",
		},
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.EqualError(t, reqErr.Err, "身份或银行卡信息核验未通过，请核对后重新提交")
	require.Same(t, providerErr, LoggableError(err))
	require.Equal(t, 1, accountClient.openCalls)
}

func TestBaofuAccountOnboardingServiceStart_ProviderOpenErrorMarksFlowFailed(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	providerErr := baofu.NewProviderBusinessError("T-1001-013-01", "BF0001", "raw upstream missing field")
	accountClient := &fakeBaofuOnboardingAccountClient{err: providerErr}
	service := NewBaofuAccountOnboardingService(store, accountClient, nil, nil, BaofuAccountOnboardingConfig{
		VerifyFeeFen: 200,
		IndustryID:   "9931",
	})

	_, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   88,
		UserID:    99,
		Profile: &BaofuAccountOpeningProfileInput{
			LegalName:           "测试商户",
			BusinessLicenseNo:   "91330100MA00000001",
			LegalPersonName:     "李四",
			LegalPersonIDNumber: "110101199001010011",
			Email:               "merchant@example.com",
			BankAccountNo:       "6222020202020202",
			BankName:            "招商银行",
			DepositBankProvince: "浙江省",
			DepositBankCity:     "杭州市",
			DepositBankName:     "招商银行杭州支行",
		},
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, 1, accountClient.openCalls)
	require.Len(t, store.flows, 1)
	require.Equal(t, db.BaofuAccountOpeningStateFailed, store.flows[0].State)
	require.Equal(t, "BF0001", store.flows[0].FailureCode.String)
	require.Equal(t, "资料信息不完整，请核对后重新提交", store.flows[0].FailureMessage.String)
	require.Equal(t, db.BaofuAccountOpenStateFailed, store.activeBinding.OpenState)
}

func TestBaofuAccountOnboardingServiceStart_ProviderOpenErrorPersistsSafeDiagnosticSnapshot(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	providerErr := baofu.NewProviderBusinessError("T-1001-013-01", "BF0020", "通道返回身份证 110101199001010011 银行卡 6222020202020202 手机 13800138000")
	var baofuProviderErr *baofu.ProviderError
	require.ErrorAs(t, providerErr, &baofuProviderErr)
	baofuProviderErr.DiagnosticSnapshot = []byte(`{"provider":"baofu","capability":"account","operation":"T-1001-013-01","http_status":200,"sys_resp_code":"S_0000","business_failure":true,"source_path":"body.errorCode","ret_code":"0","top_error_code":"BF0020","top_error_message_present":true}`)
	accountClient := &fakeBaofuOnboardingAccountClient{err: providerErr}
	service := NewBaofuAccountOnboardingService(store, accountClient, nil, nil, BaofuAccountOnboardingConfig{
		VerifyFeeFen: 200,
		IndustryID:   "9931",
	})

	_, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   88,
		UserID:    99,
		Profile: &BaofuAccountOpeningProfileInput{
			LegalName:           "测试商户",
			BusinessLicenseNo:   "91330100MA00000001",
			LegalPersonName:     "李四",
			LegalPersonIDNumber: "110101199001010011",
			Email:               "merchant@example.com",
			BankAccountNo:       "6222020202020202",
			BankName:            "招商银行",
			DepositBankProvince: "浙江省",
			DepositBankCity:     "杭州市",
			DepositBankName:     "招商银行杭州支行",
		},
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusBadGateway, reqErr.Status)
	require.Len(t, store.flows, 1)
	require.Equal(t, db.BaofuAccountOpeningStateFailed, store.flows[0].State)
	require.Equal(t, "BF0020", store.flows[0].FailureCode.String)
	require.JSONEq(t, `{
		"state":"failed",
		"failure_code":"BF0020",
		"provider_diagnostic":{
			"provider":"baofu",
			"capability":"account",
			"operation":"T-1001-013-01",
			"http_status":200,
			"sys_resp_code":"S_0000",
			"business_failure":true,
			"source_path":"body.errorCode",
			"ret_code":"0",
			"top_error_code":"BF0020",
			"top_error_message_present":true
		}
	}`, string(store.flows[0].RawSnapshot))
	require.NotContains(t, string(store.flows[0].RawSnapshot), "110101199001010011")
	require.NotContains(t, string(store.flows[0].RawSnapshot), "6222020202020202")
	require.NotContains(t, string(store.flows[0].RawSnapshot), "13800138000")
}

func TestBaofuAccountOnboardingServiceStart_BusinessPrivateCardRequiresCorporateMobile(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	service := NewBaofuAccountOnboardingService(store, &fakeBaofuOnboardingAccountClient{}, nil, nil, BaofuAccountOnboardingConfig{
		VerifyFeeFen: 200,
		IndustryID:   "9931",
	})

	result, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   88,
		UserID:    99,
		Profile: &BaofuAccountOpeningProfileInput{
			LegalName:           "测试商户",
			BusinessLicenseNo:   "91330100MA00000001",
			LegalPersonName:     "李四",
			LegalPersonIDNumber: "110101199001010011",
			Email:               "merchant@example.com",
			BankAccountNo:       "6222020202020202",
			BankName:            "招商银行",
			DepositBankProvince: "浙江省",
			DepositBankCity:     "杭州市",
			DepositBankName:     "招商银行杭州支行",
			CardUserName:        "李四",
			SelfEmployed:        true,
		},
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateProfilePending, result.State)
	require.ElementsMatch(t, []string{"corporate_mobile"}, result.MissingFields)
	require.Zero(t, store.openAccountCalls)
}

func TestBaofuAccountOnboardingServiceStart_BusinessManualBankRequiresCity(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	service := NewBaofuAccountOnboardingService(store, &fakeBaofuOnboardingAccountClient{}, nil, nil, BaofuAccountOnboardingConfig{
		VerifyFeeFen: 200,
		IndustryID:   "9931",
	})

	result, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   88,
		UserID:    99,
		Profile: &BaofuAccountOpeningProfileInput{
			LegalName:           "宁晋县周鹏饭店",
			BusinessLicenseNo:   "92130528MA00000001",
			LegalPersonName:     "周松涛",
			LegalPersonIDNumber: "130528199001010011",
			Email:               "merchant@example.com",
			BankAccountNo:       "6222020202020202",
			BankName:            "邢台银行",
			DepositBankProvince: "河北省",
			DepositBankCity:     "北京市",
			DepositBankName:     "邢台银行宁晋支行",
			CardUserName:        "周松涛",
			CorporateMobile:     "13800138000",
			SelfEmployed:        true,
		},
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateProfilePending, result.State)
	require.ElementsMatch(t, []string{"deposit_bank_city"}, result.MissingFields)
	require.Zero(t, store.openAccountCalls)
}

func TestBaofuAccountOnboardingServiceStart_BusinessPrivateCardPassesOfficialFields(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	client := &fakeBaofuOnboardingAccountClient{
		openResult: &baofucontracts.AccountResult{
			ContractNo:    "CM202605080188",
			OpenState:     db.BaofuAccountOpenStateActive,
			UpstreamState: "1",
		},
	}
	service := NewBaofuAccountOnboardingService(store, client, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931", CollectMerchantID: "100000"})

	result, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   188,
		UserID:    99,
		Profile: &BaofuAccountOpeningProfileInput{
			LegalName:           "测试商户",
			BusinessLicenseNo:   "91330100MA00000001",
			LegalPersonName:     "李四",
			LegalPersonIDNumber: "110101199001010011",
			Email:               "merchant@example.com",
			BankAccountNo:       "6222020202020202",
			BankName:            "招商银行",
			DepositBankProvince: "浙江省",
			DepositBankCity:     "杭州市",
			DepositBankName:     "招商银行杭州支行",
			CardUserName:        "李四",
			CorporateMobile:     "13800138000",
			SelfEmployed:        true,
		},
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateMerchantReportProcessing, result.State)
	require.Equal(t, db.BaofuAccountTypeBusiness, client.lastOpen.AccountType)
	require.True(t, client.lastOpen.SelfEmployed)
	require.Equal(t, "李四", client.lastOpen.CardUserName)
	require.Equal(t, "13800138000", client.lastOpen.CorporateMobile)
}

func TestBaofuAccountOnboardingServiceStart_MerchantPersonalModeUsesPersonalFourFactorAndContinuesReport(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	client := &fakeBaofuOnboardingAccountClient{
		openResult: &baofucontracts.AccountResult{
			ContractNo:    "CP202606020001",
			SharingMerID:  "CP202606020001",
			OpenState:     db.BaofuAccountOpenStateActive,
			UpstreamState: "1",
		},
	}
	service := NewBaofuAccountOnboardingService(store, client, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931", CollectMerchantID: "100000"})

	result, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType:          db.BaofuAccountOwnerTypeMerchant,
		OwnerID:            188,
		UserID:             99,
		AccountOpeningMode: db.BaofuAccountTypePersonal,
		Profile: &BaofuAccountOpeningProfileInput{
			LegalName:     "李四",
			CertificateNo: "110101199001010011",
			BankAccountNo: "6222020202020202",
			BankMobile:    "13800138000",
			CardUserName:  "李四",
			ContactName:   "李四",
			ContactMobile: "13800138000",
		},
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateMerchantReportProcessing, result.State)
	require.Equal(t, db.BaofuAccountTypePersonal, result.Flow.AccountType)
	require.Equal(t, db.BaofuAccountTypePersonal, store.activeBinding.AccountType)
	require.Equal(t, db.BaofuAccountTypePersonal, client.lastOpen.AccountType)
	require.Equal(t, "李四", client.lastOpen.LegalName)
	require.Equal(t, "110101199001010011", client.lastOpen.CertificateNo)
	require.Equal(t, "6222020202020202", client.lastOpen.BankAccountNo)
	require.Equal(t, "13800138000", client.lastOpen.BankMobile)
	require.Empty(t, client.lastOpen.CorporateName)
	require.Empty(t, client.lastOpen.CorporateCertID)
	require.Empty(t, client.lastOpen.BankName)
}

func TestBaofuAccountOnboardingServiceStart_MerchantEmptyModeContinuesPersonalDraft(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	profile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypeMerchant, 188, db.BaofuAccountTypePersonal)
	flow := store.mustCreateFlow(t, db.BaofuAccountOwnerTypeMerchant, 188, db.BaofuAccountTypePersonal, profile.ID)
	flow.LoginNo = pgtype.Text{String: "LLBFOMP0000000188R1", Valid: true}
	store.flows[0] = flow
	client := &fakeBaofuOnboardingAccountClient{
		openResult: &baofucontracts.AccountResult{
			ContractNo:    "CP202606020188",
			OpenState:     db.BaofuAccountOpenStateProcessing,
			UpstreamState: "2",
		},
	}
	service := NewBaofuAccountOnboardingService(store, client, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931", CollectMerchantID: "100000"})

	result, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   188,
		UserID:    99,
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateOpeningProcessing, result.State)
	require.Len(t, store.flows, 1)
	require.Equal(t, flow.ID, result.Flow.ID)
	require.Equal(t, db.BaofuAccountTypePersonal, result.Flow.AccountType)
	require.Equal(t, db.BaofuAccountTypePersonal, store.activeBinding.AccountType)
	require.Equal(t, db.BaofuAccountTypePersonal, client.lastOpen.AccountType)
	require.Equal(t, "LLBFOMP0000000188R1", client.lastOpen.LoginNo)
	require.Equal(t, "张三", client.lastOpen.LegalName)
	require.Equal(t, "110101199001011234", client.lastOpen.CertificateNo)
	require.Equal(t, "6222020202020202", client.lastOpen.BankAccountNo)
	require.Equal(t, "13800138000", client.lastOpen.BankMobile)
	require.Empty(t, client.lastOpen.CorporateName)
}

func TestBaofuAccountOnboardingServiceStart_MerchantOpeningModeChangeVoidsDraftFlow(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	oldProfile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypeMerchant, 188, db.BaofuAccountTypeBusiness)
	oldFlow := store.mustCreateFlow(t, db.BaofuAccountOwnerTypeMerchant, 188, db.BaofuAccountTypeBusiness, oldProfile.ID)
	store.flows[0].State = db.BaofuAccountOpeningStateProfilePending
	service := NewBaofuAccountOnboardingService(store, &fakeBaofuOnboardingAccountClient{}, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931"})

	result, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType:          db.BaofuAccountOwnerTypeMerchant,
		OwnerID:            188,
		UserID:             99,
		AccountOpeningMode: db.BaofuAccountTypePersonal,
		Profile: &BaofuAccountOpeningProfileInput{
			LegalName:     "李四",
			CertificateNo: "110101199001010011",
			BankAccountNo: "6222020202020202",
			BankMobile:    "13800138000",
			CardUserName:  "李四",
			ContactName:   "李四",
			ContactMobile: "13800138000",
		},
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateOpeningProcessing, result.State)
	require.Len(t, store.flows, 2)
	require.Equal(t, db.BaofuAccountOpeningStateVoided, store.flows[0].State)
	require.Equal(t, oldFlow.ID, store.flows[0].ID)
	require.Equal(t, db.BaofuAccountTypePersonal, store.flows[1].AccountType)
	require.Equal(t, db.BaofuAccountTypePersonal, result.Flow.AccountType)
}

func TestBaofuAccountOnboardingServiceStart_MerchantOpeningModeChangeRejectsProcessingFlow(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	oldProfile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypeMerchant, 188, db.BaofuAccountTypeBusiness)
	oldFlow := store.mustCreateFlow(t, db.BaofuAccountOwnerTypeMerchant, 188, db.BaofuAccountTypeBusiness, oldProfile.ID)
	store.flows[0].State = db.BaofuAccountOpeningStateOpeningProcessing
	service := NewBaofuAccountOnboardingService(store, &fakeBaofuOnboardingAccountClient{}, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931"})

	_, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType:          db.BaofuAccountOwnerTypeMerchant,
		OwnerID:            188,
		UserID:             99,
		AccountOpeningMode: db.BaofuAccountTypePersonal,
		Profile: &BaofuAccountOpeningProfileInput{
			LegalName:     "李四",
			CertificateNo: "110101199001010011",
			BankAccountNo: "6222020202020202",
			BankMobile:    "13800138000",
			CardUserName:  "李四",
			ContactName:   "李四",
			ContactMobile: "13800138000",
		},
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusConflict, reqErr.Status)
	require.Equal(t, oldFlow.ID, store.flows[0].ID)
	require.Equal(t, db.BaofuAccountOpeningStateOpeningProcessing, store.flows[0].State)
	require.Len(t, store.flows, 1)
}

func TestBaofuAccountOnboardingServiceStart_MerchantOpeningModeChangeRejectsActiveBinding(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	store.activeBinding = db.BaofuAccountBinding{ID: 21, OwnerType: db.BaofuAccountOwnerTypeMerchant, OwnerID: 188, AccountType: db.BaofuAccountTypeBusiness, OpenState: db.BaofuAccountOpenStateActive}
	service := NewBaofuAccountOnboardingService(store, &fakeBaofuOnboardingAccountClient{}, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931"})

	_, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType:          db.BaofuAccountOwnerTypeMerchant,
		OwnerID:            188,
		UserID:             99,
		AccountOpeningMode: db.BaofuAccountTypePersonal,
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusConflict, reqErr.Status)
	require.Empty(t, store.flows)
}

func TestBaofuAccountOnboardingServiceStart_RiderRequiresVerifyFeeBeforeOpening(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	store.users[7] = db.User{ID: 7, WechatOpenid: "openid-rider"}
	service := NewBaofuAccountOnboardingService(store, &fakeBaofuOnboardingAccountClient{}, &fakeBaofuOnboardingDirectPaymentClient{
		prepayID: "prepay-verify-fee",
	}, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931"})

	result, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   12,
		UserID:    7,
		ClientIP:  "127.0.0.1",
		Profile: &BaofuAccountOpeningProfileInput{
			LegalName:     "张三",
			CertificateNo: "110101199001011234",
			BankAccountNo: "6222020202020202",
			BankMobile:    "13800138000",
			CardUserName:  "张三",
		},
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateVerifyFeeProcessing, result.State)
	require.NotNil(t, result.PayParams)
	require.Equal(t, int64(200), result.PaymentOrder.Amount)
	require.Equal(t, db.PaymentBusinessTypeBaofuAccountVerifyFee, result.PaymentOrder.BusinessType)
	require.Equal(t, db.PaymentChannelDirect, result.PaymentOrder.PaymentChannel)
	require.False(t, result.PaymentOrder.RequiresProfitSharing)
	require.Zero(t, store.openAccountCalls)
	require.Len(t, store.flows, 1)
	require.Equal(t, db.BaofuAccountOpeningStateVerifyFeeProcessing, store.flows[0].State)
}

func TestBaofuAccountOnboardingServiceStart_OperatorRequiresVerifyFeeBeforeOpening(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	store.users[17] = db.User{ID: 17, WechatOpenid: "openid-operator"}
	client := &fakeBaofuOnboardingAccountClient{}
	service := NewBaofuAccountOnboardingService(store, client, &fakeBaofuOnboardingDirectPaymentClient{
		prepayID: "prepay-operator-verify-fee",
	}, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931"})

	result, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType: db.BaofuAccountOwnerTypeOperator,
		OwnerID:   42,
		UserID:    17,
		ClientIP:  "127.0.0.1",
		Profile: &BaofuAccountOpeningProfileInput{
			LegalName:     "王五",
			CertificateNo: "110101199001015555",
			BankAccountNo: "6222020202020555",
			BankMobile:    "13900139000",
			CardUserName:  "王五",
		},
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateVerifyFeeProcessing, result.State)
	require.NotNil(t, result.PayParams)
	require.Equal(t, db.PaymentBusinessTypeBaofuAccountVerifyFee, result.PaymentOrder.BusinessType)
	require.Equal(t, int64(200), result.PaymentOrder.Amount)
	require.Equal(t, db.BaofuAccountTypePersonal, result.Flow.AccountType)
	require.Zero(t, store.openAccountCalls)
	require.Len(t, store.flows, 1)
	require.Equal(t, db.BaofuAccountOpeningStateVerifyFeeProcessing, store.flows[0].State)
	require.Contains(t, store.payments[0].Attach.String, "owner_type:operator;owner_id:42")
}

func TestBaofuAccountOnboardingServiceStart_BusinessOwnerOpensWithIndustry9931AndPlatformFeeLedger(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	client := &fakeBaofuOnboardingAccountClient{
		openResult: &baofucontracts.AccountResult{
			ContractNo:    "CM202605080001",
			SharingMerID:  "",
			OpenState:     db.BaofuAccountOpenStateActive,
			UpstreamState: "1",
		},
	}
	service := NewBaofuAccountOnboardingService(store, client, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931", CollectMerchantID: "100000"})

	result, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType: db.BaofuAccountOwnerTypePlatform,
		OwnerID:   0,
		UserID:    1,
		Profile: &BaofuAccountOpeningProfileInput{
			LegalName:           "本地生活科技有限公司",
			BusinessLicenseNo:   "91310000123456789A",
			LegalPersonName:     "李四",
			LegalPersonIDNumber: "110101199001019999",
			Email:               "finance@example.com",
			BankAccountNo:       "6222020202020202",
			BankName:            "招商银行",
			DepositBankProvince: "上海",
			DepositBankCity:     "上海",
			DepositBankName:     "招商银行上海分行",
			ContactName:         "财务",
			ContactMobile:       "13800138000",
		},
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateReady, result.State)
	require.Equal(t, db.BaofuAccountTypeBusiness, client.lastOpen.AccountType)
	require.Equal(t, "9931", client.lastOpen.IndustryID)
	require.Empty(t, client.lastOpen.QualificationTransSerialNo)
	require.Empty(t, client.lastOpen.PlatformNo)
	require.Empty(t, client.lastOpen.PlatformTerminalID)
	require.Equal(t, "CM202605080001", store.activeBinding.SharingMerID.String)
	require.Equal(t, db.BaofuFeeTypeAccountOpenVerifyFee, store.lastLedger.FeeType)
	require.Equal(t, db.BaofuFeePayerTypePlatform, store.lastLedger.PayerType)
	require.Equal(t, int64(200), store.lastLedger.Amount)
}

func TestBaofuAccountOnboardingServiceContinueAfterVerifyFeePaid_OpensRiderWithoutPlatformFeeLedger(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	client := &fakeBaofuOnboardingAccountClient{
		openResult: &baofucontracts.AccountResult{
			ContractNo:    "CP202605080001",
			OpenState:     db.BaofuAccountOpenStateActive,
			UpstreamState: "1",
		},
	}
	service := NewBaofuAccountOnboardingService(store, client, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931", CollectMerchantID: "100000"})
	profile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypeRider, 66, db.BaofuAccountTypePersonal)
	flow := store.mustCreateFlow(t, db.BaofuAccountOwnerTypeRider, 66, db.BaofuAccountTypePersonal, profile.ID)
	payment := db.PaymentOrder{ID: 501, UserID: 9, BusinessType: db.PaymentBusinessTypeBaofuAccountVerifyFee, PaymentChannel: db.PaymentChannelDirect, PaymentType: "miniprogram", Amount: 200, Status: "paid"}
	flow.VerifyFeePaymentOrderID = pgtype.Int8{Int64: payment.ID, Valid: true}
	store.flows[0] = flow

	err := service.ContinueAfterVerifyFeePaid(context.Background(), payment)

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateReady, store.flows[0].State)
	require.Equal(t, db.BaofuAccountTypePersonal, client.lastOpen.AccountType)
	require.False(t, store.platformFeeLedgerCreated)
}

func TestBaofuAccountOnboardingServiceContinueAfterVerifyFeePaid_ReadyFlowIsIdempotent(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	client := &fakeBaofuOnboardingAccountClient{
		openResult: &baofucontracts.AccountResult{
			ContractNo:    "CP202605170001",
			OpenState:     db.BaofuAccountOpenStateActive,
			UpstreamState: "1",
		},
	}
	service := NewBaofuAccountOnboardingService(store, client, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931", CollectMerchantID: "100000"})
	profile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypeRider, 1, db.BaofuAccountTypePersonal)
	flow := store.mustCreateFlow(t, db.BaofuAccountOwnerTypeRider, 1, db.BaofuAccountTypePersonal, profile.ID)
	payment := db.PaymentOrder{ID: 11, UserID: 2, BusinessType: db.PaymentBusinessTypeBaofuAccountVerifyFee, PaymentChannel: db.PaymentChannelDirect, PaymentType: "miniprogram", Amount: 200, Status: "paid"}
	flow.State = db.BaofuAccountOpeningStateReady
	flow.VerifyFeePaymentOrderID = pgtype.Int8{Int64: payment.ID, Valid: true}
	store.flows[0] = flow

	err := service.ContinueAfterVerifyFeePaid(context.Background(), payment)

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateReady, store.flows[0].State)
	require.Zero(t, client.openCalls)
}

func TestBaofuAccountOnboardingServiceContinueAfterVerifyFeePaid_ActiveBindingRecoversFailedFlow(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	service := NewBaofuAccountOnboardingService(store, &fakeBaofuOnboardingAccountClient{}, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931", CollectMerchantID: "100000"})
	profile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypeRider, 2, db.BaofuAccountTypePersonal)
	flow := store.mustCreateFlow(t, db.BaofuAccountOwnerTypeRider, 2, db.BaofuAccountTypePersonal, profile.ID)
	payment := db.PaymentOrder{ID: 24, UserID: 35, BusinessType: db.PaymentBusinessTypeBaofuAccountVerifyFee, PaymentChannel: db.PaymentChannelDirect, PaymentType: "miniprogram", Amount: 200, Status: "paid"}
	flow.State = db.BaofuAccountOpeningStateFailed
	flow.FailureCode = pgtype.Text{String: "BF0003", Valid: true}
	flow.FailureMessage = pgtype.Text{String: "支付通道异常，请联系平台处理", Valid: true}
	flow.VerifyFeePaymentOrderID = pgtype.Int8{Int64: payment.ID, Valid: true}
	flow.OpenTransSerialNo = pgtype.Text{String: "BFO202605271037580bee6bbc", Valid: true}
	flow.LoginNo = pgtype.Text{String: "LLBFOR0000000002", Valid: true}
	store.flows[0] = flow
	store.activeBinding = db.BaofuAccountBinding{
		ID:                    11,
		OwnerType:             flow.OwnerType,
		OwnerID:               flow.OwnerID,
		AccountType:           flow.AccountType,
		LoginNo:               flow.LoginNo,
		OpenState:             db.BaofuAccountOpenStateActive,
		ContractNo:            pgtype.Text{String: "CP660000000223785098", Valid: true},
		SharingMerID:          pgtype.Text{String: "CP660000000223785098", Valid: true},
		LastOpenTransSerialNo: flow.OpenTransSerialNo,
	}

	err := service.ContinueAfterVerifyFeePaid(context.Background(), payment)

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateReady, store.flows[0].State)
	require.Equal(t, pgtype.Int8{Int64: 11, Valid: true}, store.flows[0].AccountBindingID)
	require.False(t, store.flows[0].FailureCode.Valid)
	require.False(t, store.flows[0].FailureMessage.Valid)
}

func TestBaofuAccountOnboardingServiceContinueAfterVerifyFeePaid_OpensOperatorWithoutPlatformFeeLedgerOrMerchantReport(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	client := &fakeBaofuOnboardingAccountClient{
		openResult: &baofucontracts.AccountResult{
			ContractNo:    "CP202605080042",
			OpenState:     db.BaofuAccountOpenStateActive,
			UpstreamState: "1",
		},
	}
	reportClient := &fakeMerchantReportClient{}
	service := NewBaofuAccountOnboardingService(store, client, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931"}).
		WithMerchantReportContinuation(reportClient, BaofuAccountMerchantReportConfig{
			CollectMerchantID: "100000",
			CollectTerminalID: "200000",
			MiniProgramAppID:  "wx1234567890abcdef",
			ChannelID:         "CH001",
			ChannelName:       "LocalLife",
			Business:          "758-2",
		})
	profile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypeOperator, 42, db.BaofuAccountTypePersonal)
	flow := store.mustCreateFlow(t, db.BaofuAccountOwnerTypeOperator, 42, db.BaofuAccountTypePersonal, profile.ID)
	payment := db.PaymentOrder{ID: 601, UserID: 17, BusinessType: db.PaymentBusinessTypeBaofuAccountVerifyFee, PaymentChannel: db.PaymentChannelDirect, PaymentType: "miniprogram", Amount: 200, Status: "paid"}
	flow.VerifyFeePaymentOrderID = pgtype.Int8{Int64: payment.ID, Valid: true}
	store.flows[0] = flow

	err := service.ContinueAfterVerifyFeePaid(context.Background(), payment)

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateReady, store.flows[0].State)
	require.False(t, store.flows[0].MerchantReportID.Valid)
	require.Equal(t, db.BaofuAccountTypePersonal, client.lastOpen.AccountType)
	require.False(t, store.platformFeeLedgerCreated)
	require.Empty(t, store.merchantReports)
	require.Empty(t, reportClient.reportRequest.ReportNo)
}

func TestBaofuAccountOnboardingServiceStart_ReplacementFlowReusesPaidVerifyFeeWithoutChargingAgain(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	client := &fakeBaofuOnboardingAccountClient{
		openResult: &baofucontracts.AccountResult{
			ContractNo:    "CP202605080066",
			OpenState:     db.BaofuAccountOpenStateActive,
			UpstreamState: "1",
		},
	}
	service := NewBaofuAccountOnboardingService(store, client, &fakeBaofuOnboardingDirectPaymentClient{
		prepayID: "should-not-be-used",
	}, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931"})
	store.users[9] = db.User{ID: 9, WechatOpenid: "openid-rider"}
	store.payments = append(store.payments, db.PaymentOrder{
		ID:                    701,
		UserID:                9,
		PaymentType:           "miniprogram",
		PaymentChannel:        db.PaymentChannelDirect,
		RequiresProfitSharing: false,
		BusinessType:          db.PaymentBusinessTypeBaofuAccountVerifyFee,
		Amount:                200,
		OutTradeNo:            "BFV202605080701",
		Status:                "paid",
		Attach:                pgtype.Text{String: "business:baofu_account_verify_fee;owner_type:rider;owner_id:66;purpose:initial_open", Valid: true},
		ProcessedAt:           pgtype.Timestamptz{Time: time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC), Valid: true},
	})
	oldProfile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypeRider, 66, db.BaofuAccountTypePersonal)
	oldFlow := store.mustCreateFlow(t, db.BaofuAccountOwnerTypeRider, 66, db.BaofuAccountTypePersonal, oldProfile.ID)
	oldFlow.State = db.BaofuAccountOpeningStateFailed
	oldFlow.VerifyFeePaymentOrderID = pgtype.Int8{}
	store.flows[0] = oldFlow

	result, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   66,
		UserID:    9,
		ClientIP:  "127.0.0.1",
		Profile: &BaofuAccountOpeningProfileInput{
			LegalName:     "张三",
			CertificateNo: "110101199001011234",
			BankAccountNo: "6222020202020202",
			BankMobile:    "13800138000",
			CardUserName:  "张三",
		},
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateReady, result.State)
	require.Equal(t, int64(701), store.flows[1].VerifyFeePaymentOrderID.Int64)
	require.Len(t, store.payments, 1)
	require.Equal(t, 1, client.openCalls)
	require.Equal(t, db.BaofuAccountTypePersonal, client.lastOpen.AccountType)
	require.False(t, store.platformFeeLedgerCreated)
}

func TestBaofuAccountOnboardingServiceApplyAccountOpenResult_NonMerchantOwnersSkipMerchantReport(t *testing.T) {
	tests := []struct {
		name              string
		ownerType         string
		accountType       string
		expectPlatformFee bool
	}{
		{name: "platform", ownerType: db.BaofuAccountOwnerTypePlatform, accountType: db.BaofuAccountTypeBusiness, expectPlatformFee: true},
		{name: "rider", ownerType: db.BaofuAccountOwnerTypeRider, accountType: db.BaofuAccountTypePersonal},
		{name: "operator", ownerType: db.BaofuAccountOwnerTypeOperator, accountType: db.BaofuAccountTypePersonal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeBaofuAccountOnboardingStore()
			reportClient := &fakeMerchantReportClient{}
			service := NewBaofuAccountOnboardingService(store, nil, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931"}).
				WithMerchantReportContinuation(reportClient, BaofuAccountMerchantReportConfig{
					CollectMerchantID: "100000",
					CollectTerminalID: "200000",
					MiniProgramAppID:  "wx1234567890abcdef",
					ChannelID:         "CH001",
					ChannelName:       "LocalLife",
					Business:          "758-2",
				})
			flow := db.BaofuAccountOpeningFlow{
				ID:                11,
				OwnerType:         tt.ownerType,
				OwnerID:           42,
				AccountType:       tt.accountType,
				State:             db.BaofuAccountOpeningStateOpeningProcessing,
				OpenTransSerialNo: pgtype.Text{String: "BFO42", Valid: true},
				LoginNo:           pgtype.Text{String: "LLBFOO0000000042", Valid: true},
			}
			store.flows = append(store.flows, flow)
			store.activeBinding = db.BaofuAccountBinding{ID: 21, OwnerType: flow.OwnerType, OwnerID: flow.OwnerID, AccountType: flow.AccountType, OpenState: db.BaofuAccountOpenStateProcessing}

			applied, err := service.ApplyAccountOpenResult(context.Background(), flow, baofucontracts.AccountResult{
				ContractNo:    "CP202605080042",
				OpenState:     db.BaofuAccountOpenStateActive,
				UpstreamState: "1",
				Raw:           []byte(`{"state":"1"}`),
			})

			require.NoError(t, err)
			require.Equal(t, db.BaofuAccountOpeningStateReady, applied.Flow.State)
			require.False(t, applied.Flow.MerchantReportID.Valid)
			require.Equal(t, tt.expectPlatformFee, store.platformFeeLedgerCreated)
			require.Empty(t, store.merchantReports)
			require.Empty(t, reportClient.reportRequest.ReportNo)
		})
	}
}

func TestBaofuAccountOnboardingServiceApplyAccountOpenResult_ActiveCallbackRecoversFailedFlowWithMatchingBinding(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	service := NewBaofuAccountOnboardingService(store, nil, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931"})
	flow := db.BaofuAccountOpeningFlow{
		ID:                7101,
		OwnerType:         db.BaofuAccountOwnerTypeRider,
		OwnerID:           2,
		AccountType:       db.BaofuAccountTypePersonal,
		State:             db.BaofuAccountOpeningStateFailed,
		FailureCode:       pgtype.Text{String: "BF0003", Valid: true},
		FailureMessage:    pgtype.Text{String: "支付通道异常，请联系平台处理", Valid: true},
		OpenTransSerialNo: pgtype.Text{String: "BFO202605271037580bee6bbc", Valid: true},
		LoginNo:           pgtype.Text{String: "LLBFOR0000000002", Valid: true},
	}
	store.flows = append(store.flows, flow)
	store.activeBinding = db.BaofuAccountBinding{
		ID:                    6101,
		OwnerType:             flow.OwnerType,
		OwnerID:               flow.OwnerID,
		AccountType:           flow.AccountType,
		LoginNo:               flow.LoginNo,
		OpenState:             db.BaofuAccountOpenStateActive,
		ContractNo:            pgtype.Text{String: "CP660000000223785098", Valid: true},
		SharingMerID:          pgtype.Text{String: "CP660000000223785098", Valid: true},
		LastOpenTransSerialNo: flow.OpenTransSerialNo,
	}

	applied, err := service.ApplyAccountOpenResult(context.Background(), flow, baofucontracts.AccountResult{
		OutRequestNo:  "BFO202605271037580bee6bbc",
		ContractNo:    "CP660000000223785098",
		SharingMerID:  "CP660000000223785098",
		OpenState:     db.BaofuAccountOpenStateActive,
		UpstreamState: "1",
		Raw:           []byte(`{"transSerialNo":"BFO202605271037580bee6bbc","state":"1","contractNo":"CP660000000223785098"}`),
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateReady, applied.Flow.State)
	require.Equal(t, pgtype.Int8{Int64: 6101, Valid: true}, applied.Flow.AccountBindingID)
	require.False(t, applied.Flow.FailureCode.Valid)
	require.False(t, applied.Flow.FailureMessage.Valid)
	require.NotNil(t, applied.Binding)
	require.Equal(t, db.BaofuAccountOpenStateActive, applied.Binding.OpenState)
}

func TestBaofuAccountOnboardingServiceApplyAccountOpenResult_MerchantRecoveredFailureContinuesMerchantReport(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	reportClient := &fakeMerchantReportClient{
		reportResult: &merchantcontracts.MerchantReportResult{
			ReportNo:      "MR202605270088",
			ReportState:   "SUCCESS",
			SubMchID:      "1900000088",
			PlatformBizNo: "PB202605270088",
		},
		bindResult: &merchantcontracts.BindSubConfigResult{
			SubMchID:   "1900000088",
			AuthType:   merchantcontracts.AuthTypeApplet,
			ResultCode: "SUCCESS",
		},
	}
	service := NewBaofuAccountOnboardingService(store, nil, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931"}).
		WithMerchantReportContinuation(reportClient, BaofuAccountMerchantReportConfig{
			CollectMerchantID: "100000",
			CollectTerminalID: "200000",
			MiniProgramAppID:  "wx1234567890abcdef",
			ChannelID:         "CH001",
			ChannelName:       "LocalLife",
			Business:          "758-2",
		})
	profile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypeMerchant, 88, db.BaofuAccountTypeBusiness)
	profile.LegalName = pgtype.Text{String: "测试餐饮有限公司", Valid: true}
	profile.CertificateType = pgtype.Text{String: baofucontracts.OfficialBusinessCertificateTypeLicense, Valid: true}
	profile.CertificateNoCiphertext = pgtype.Text{String: "91330100MA00000001", Valid: true}
	profile.BankAccountNoCiphertext = pgtype.Text{String: "6222020202020202", Valid: true}
	profile.CardUserName = pgtype.Text{String: "测试餐饮有限公司", Valid: true}
	profile.DepositBankName = pgtype.Text{String: "招商银行杭州支行", Valid: true}
	store.profiles[0] = profile
	store.merchants[88] = db.Merchant{ID: 88, Status: db.MerchantStatusApproved, Name: "测试餐饮", Phone: "057112345678", Address: "测试路 1 号", RegionID: 330106}
	store.regions[330106] = db.Region{ID: 330106, Code: "330106", Level: 3, ParentID: pgtype.Int8{Int64: 330100, Valid: true}}
	store.regions[330100] = db.Region{ID: 330100, Code: "330100", Level: 2, ParentID: pgtype.Int8{Int64: 330000, Valid: true}}
	store.regions[330000] = db.Region{ID: 330000, Code: "330000", Level: 1}
	flow := db.BaofuAccountOpeningFlow{
		ID:                7102,
		OwnerType:         db.BaofuAccountOwnerTypeMerchant,
		OwnerID:           88,
		AccountType:       db.BaofuAccountTypeBusiness,
		ProfileID:         pgtype.Int8{Int64: profile.ID, Valid: true},
		State:             db.BaofuAccountOpeningStateFailed,
		FailureCode:       pgtype.Text{String: "BF0003", Valid: true},
		FailureMessage:    pgtype.Text{String: "支付通道异常，请联系平台处理", Valid: true},
		OpenTransSerialNo: pgtype.Text{String: "BFO20260527103758merchant", Valid: true},
		LoginNo:           pgtype.Text{String: "LLBFOM0000000088", Valid: true},
	}
	store.flows = append(store.flows, flow)
	store.activeBinding = db.BaofuAccountBinding{
		ID:                    6102,
		OwnerType:             flow.OwnerType,
		OwnerID:               flow.OwnerID,
		AccountType:           flow.AccountType,
		LoginNo:               flow.LoginNo,
		OpenState:             db.BaofuAccountOpenStateActive,
		ContractNo:            pgtype.Text{String: "CM660000000223785098", Valid: true},
		SharingMerID:          pgtype.Text{String: "CM660000000223785098", Valid: true},
		LastOpenTransSerialNo: flow.OpenTransSerialNo,
	}

	applied, err := service.ApplyAccountOpenResult(context.Background(), flow, baofucontracts.AccountResult{
		OutRequestNo:  "BFO20260527103758merchant",
		ContractNo:    "CM660000000223785098",
		SharingMerID:  "CM660000000223785098",
		OpenState:     db.BaofuAccountOpenStateActive,
		UpstreamState: "1",
		Raw:           []byte(`{"transSerialNo":"BFO20260527103758merchant","state":"1","contractNo":"CM660000000223785098"}`),
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateReady, applied.Flow.State)
	require.Equal(t, pgtype.Int8{Int64: 6102, Valid: true}, applied.Flow.AccountBindingID)
	require.Len(t, store.merchantReports, 1)
	require.Equal(t, "100000", reportClient.reportRequest.MerchantID)
	require.Equal(t, "1900000088", store.merchantPaymentConfigs[0].SubMchID)
	require.Equal(t, db.MerchantStatusActive, store.merchants[88].Status)
}

func TestBaofuAccountOnboardingServiceApplyAccountOpenResult_MerchantPersonalReportUsesIdentityCardAndBindsApplet(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	reportClient := &fakeMerchantReportClient{
		reportResult: &merchantcontracts.MerchantReportResult{
			ReportNo:      "MR202606020188",
			ReportState:   "SUCCESS",
			SubMchID:      "1900000188",
			PlatformBizNo: "PB202606020188",
		},
		bindResult: &merchantcontracts.BindSubConfigResult{
			SubMchID:   "1900000188",
			AuthType:   merchantcontracts.AuthTypeApplet,
			ResultCode: "SUCCESS",
		},
	}
	service := NewBaofuAccountOnboardingService(store, nil, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931"}).
		WithMerchantReportContinuation(reportClient, BaofuAccountMerchantReportConfig{
			CollectMerchantID: "100000",
			CollectTerminalID: "200000",
			MiniProgramAppID:  "wx1234567890abcdef",
			ChannelID:         "CH001",
			ChannelName:       "LocalLife",
			Business:          "758-2",
		})
	profile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypeMerchant, 188, db.BaofuAccountTypePersonal)
	profile.LegalName = pgtype.Text{String: "李四", Valid: true}
	profile.CertificateType = pgtype.Text{String: baofucontracts.OfficialCertificateTypeID, Valid: true}
	profile.CertificateNoCiphertext = pgtype.Text{String: "110101199001010011", Valid: true}
	profile.BankAccountNoCiphertext = pgtype.Text{String: "6222020202020202", Valid: true}
	profile.BankMobileCiphertext = pgtype.Text{String: "13800138000", Valid: true}
	profile.CardUserName = pgtype.Text{String: "李四", Valid: true}
	profile.ContactName = pgtype.Text{String: "李四", Valid: true}
	profile.ContactMobileCiphertext = pgtype.Text{String: "13800138000", Valid: true}
	store.profiles[0] = profile
	store.merchants[188] = db.Merchant{ID: 188, Status: db.MerchantStatusApproved, Name: "李四小吃店", Phone: "057112345678", Address: "测试路 1 号", RegionID: 330106}
	store.regions[330106] = db.Region{ID: 330106, Code: "330106", Level: 3, ParentID: pgtype.Int8{Int64: 330100, Valid: true}}
	store.regions[330100] = db.Region{ID: 330100, Code: "330100", Level: 2, ParentID: pgtype.Int8{Int64: 330000, Valid: true}}
	store.regions[330000] = db.Region{ID: 330000, Code: "330000", Level: 1}
	flow := db.BaofuAccountOpeningFlow{
		ID:                7202,
		OwnerType:         db.BaofuAccountOwnerTypeMerchant,
		OwnerID:           188,
		AccountType:       db.BaofuAccountTypePersonal,
		ProfileID:         pgtype.Int8{Int64: profile.ID, Valid: true},
		State:             db.BaofuAccountOpeningStateOpeningProcessing,
		OpenTransSerialNo: pgtype.Text{String: "BFO202606020188", Valid: true},
		LoginNo:           pgtype.Text{String: "LLBFOM0000000188", Valid: true},
	}
	store.flows = append(store.flows, flow)
	store.activeBinding = db.BaofuAccountBinding{
		ID:                    6202,
		OwnerType:             flow.OwnerType,
		OwnerID:               flow.OwnerID,
		AccountType:           flow.AccountType,
		LoginNo:               flow.LoginNo,
		OpenState:             db.BaofuAccountOpenStateProcessing,
		LastOpenTransSerialNo: flow.OpenTransSerialNo,
	}

	applied, err := service.ApplyAccountOpenResult(context.Background(), flow, baofucontracts.AccountResult{
		OutRequestNo:  "BFO202606020188",
		ContractNo:    "CP660000000223785188",
		SharingMerID:  "CP660000000223785188",
		OpenState:     db.BaofuAccountOpenStateActive,
		UpstreamState: "1",
		Raw:           []byte(`{"transSerialNo":"BFO202606020188","state":"1","contractNo":"CP660000000223785188"}`),
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateReady, applied.Flow.State)
	require.Len(t, store.merchantReports, 1)
	require.Equal(t, db.BaofuAccountTypePersonal, store.activeBinding.AccountType)
	require.Equal(t, merchantcontracts.WechatCertificateTypeIdentityCard, reportClient.reportRequest.ReportInfo.BusinessLicenseType)
	require.Equal(t, "110101199001010011", reportClient.reportRequest.ReportInfo.BusinessLicense)
	require.Equal(t, "CP660000000223785188", reportClient.reportRequest.BCTMerchantID)
	require.Equal(t, "1900000188", reportClient.bindRequest.SubMchID)
	require.Equal(t, merchantcontracts.AuthTypeApplet, reportClient.bindRequest.AuthType)
	require.Equal(t, "wx1234567890abcdef", reportClient.bindRequest.AuthContent)
}

func TestBaofuAccountOnboardingServiceApplyAccountOpenResult_FailedResultReturnsSafeGuidanceAndLogsCode(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	store := newFakeBaofuAccountOnboardingStore()
	service := NewBaofuAccountOnboardingService(store, nil, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931"})
	flow := db.BaofuAccountOpeningFlow{
		ID:                12,
		OwnerType:         db.BaofuAccountOwnerTypeOperator,
		OwnerID:           1,
		AccountType:       db.BaofuAccountTypePersonal,
		State:             db.BaofuAccountOpeningStateOpeningProcessing,
		OpenTransSerialNo: pgtype.Text{String: "BFO_DUPLICATE_CURRENT", Valid: true},
		LoginNo:           pgtype.Text{String: "LLBFOO0000000999", Valid: true},
	}
	store.flows = append(store.flows, flow)
	store.activeBinding = db.BaofuAccountBinding{ID: 22, OwnerType: flow.OwnerType, OwnerID: flow.OwnerID, AccountType: flow.AccountType, OpenState: db.BaofuAccountOpenStateProcessing}

	applied, err := service.ApplyAccountOpenResult(context.Background(), flow, baofucontracts.AccountResult{
		OpenState:   db.BaofuAccountOpenStateFailed,
		FailCode:    "BF00060",
		FailMessage: "该子商户已开户，请勿重复提交，login_no=LLBFOO0000000999",
		Raw:         []byte(`{"state":"0","errorCode":"BF00060"}`),
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateFailed, applied.Flow.State)
	require.Equal(t, "BF00060", applied.Flow.FailureCode.String)
	result := baofuOpeningResult(applied.Flow, db.BaofuAccountOpeningProfile{ProfileStatus: db.BaofuAccountOpeningProfileStatusComplete})
	require.Equal(t, "该主体已存在宝付开户记录，请联系平台核对账户状态", result.StatusDesc)
	require.Contains(t, logs.String(), "baofu account opening flow marked failed")
	require.Contains(t, logs.String(), `"failure_code":"BF00060"`)
	require.Contains(t, logs.String(), `"failure_category":"user_action_required"`)
	require.NotContains(t, logs.String(), "该子商户已开户")
	require.NotContains(t, logs.String(), "LLBFOO0000000999")
}

func TestBaofuAccountOnboardingServiceApplyAccountOpenResult_DoesNotPersistUnsafeRawSnapshot(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	service := NewBaofuAccountOnboardingService(store, nil, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931"})
	flow := db.BaofuAccountOpeningFlow{
		ID:                13,
		OwnerType:         db.BaofuAccountOwnerTypeOperator,
		OwnerID:           1,
		AccountType:       db.BaofuAccountTypePersonal,
		State:             db.BaofuAccountOpeningStateOpeningProcessing,
		OpenTransSerialNo: pgtype.Text{String: "BFO_SAFE_RAW_CURRENT", Valid: true},
		LoginNo:           pgtype.Text{String: "LLBFOO0000000999", Valid: true},
	}
	store.flows = append(store.flows, flow)
	store.activeBinding = db.BaofuAccountBinding{ID: 23, OwnerType: flow.OwnerType, OwnerID: flow.OwnerID, AccountType: flow.AccountType, OpenState: db.BaofuAccountOpenStateProcessing}

	applied, err := service.ApplyAccountOpenResult(context.Background(), flow, baofucontracts.AccountResult{
		OpenState:     db.BaofuAccountOpenStateFailed,
		UpstreamState: "0",
		FailCode:      "BF0020",
		FailMessage:   "通道返回身份证 110101199001010011 银行卡 6222020202020202 手机 13800138000",
		Raw:           []byte(`{"retCode":1,"result":[{"state":0,"errorCode":"BF0020","errorMsg":"通道返回身份证 110101199001010011 银行卡 6222020202020202 手机 13800138000","customerName":"测试用户","loginNo":"LLBFOO0000000999","contractNo":"CP_SECRET_123"}]}`),
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateFailed, applied.Flow.State)
	require.Equal(t, "BF0020", applied.Flow.FailureCode.String)
	require.Equal(t, "支付通道异常，请联系平台处理", applied.Flow.FailureMessage.String)
	require.JSONEq(t, `{
		"state":"failed",
		"failure_code":"BF0020",
		"provider_diagnostic":{
			"provider":"baofu",
			"capability":"account",
			"source_path":"body.result[0].errorCode",
			"ret_code":"1",
			"result_state":"0",
			"result_error_code":"BF0020",
			"result_error_message_present":true
		}
	}`, string(applied.Flow.RawSnapshot))
	require.NotContains(t, string(applied.Flow.RawSnapshot), "测试用户")
	require.NotContains(t, string(applied.Flow.RawSnapshot), "110101199001010011")
	require.NotContains(t, string(applied.Flow.RawSnapshot), "6222020202020202")
	require.NotContains(t, string(applied.Flow.RawSnapshot), "13800138000")
	require.NotContains(t, string(applied.Flow.RawSnapshot), "LLBFOO0000000999")
	require.NotContains(t, string(applied.Flow.RawSnapshot), "CP_SECRET_123")
}

func TestBaofuOpeningProviderFailureSnapshotDropsNestedDiagnosticValues(t *testing.T) {
	snapshot := baofuOpeningProviderFailureSnapshot("BF0020", []byte(`{
		"provider":"baofu",
		"capability":"account",
		"source_path":"body.result[0].errorCode",
		"ret_code":"1",
		"result_state":"0",
		"result_error_code":{"customerName":"测试用户","contractNo":"CP_SECRET_123"},
		"top_error_code":"身份证 110101199001010011",
		"result_error_message_present":true
	}`))

	require.JSONEq(t, `{
		"state":"failed",
		"failure_code":"BF0020",
		"provider_diagnostic":{
			"provider":"baofu",
			"capability":"account",
			"source_path":"body.result[0].errorCode",
			"ret_code":"1",
			"result_state":"0",
			"result_error_message_present":true
		}
	}`, string(snapshot))
	require.NotContains(t, string(snapshot), "测试用户")
	require.NotContains(t, string(snapshot), "CP_SECRET_123")
}

func TestBaofuAccountOnboardingServiceApplyAccountOpenResult_DuplicateOpenReconcilesByQuery(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	client := &fakeBaofuOnboardingAccountClient{
		queryResult: &baofucontracts.AccountResult{
			OutRequestNo:  "BFO_DUPLICATE_PREVIOUS",
			ContractNo:    "CP2026051600011958",
			SharingMerID:  "CP2026051600011958",
			OpenState:     db.BaofuAccountOpenStateActive,
			UpstreamState: "1",
			Raw:           []byte(`{"transSerialNo":"BFO_DUPLICATE_PREVIOUS","contractNo":"CP2026051600011958"}`),
		},
	}
	service := NewBaofuAccountOnboardingService(store, client, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931", CollectMerchantID: "1338125"})
	profile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypeOperator, 1, db.BaofuAccountTypePersonal)
	flow := store.mustCreateFlow(t, db.BaofuAccountOwnerTypeOperator, 1, db.BaofuAccountTypePersonal, profile.ID)
	flow.State = db.BaofuAccountOpeningStateOpeningProcessing
	flow.OpenTransSerialNo = pgtype.Text{String: "BFO_DUPLICATE_CURRENT", Valid: true}
	flow.LoginNo = pgtype.Text{String: "LLBFOO0000000999", Valid: true}
	store.flows[0] = flow
	store.activeBinding = db.BaofuAccountBinding{ID: 22, OwnerType: flow.OwnerType, OwnerID: flow.OwnerID, AccountType: flow.AccountType, LoginNo: flow.LoginNo, LastOpenTransSerialNo: flow.OpenTransSerialNo, OpenState: db.BaofuAccountOpenStateProcessing}

	applied, err := service.ApplyAccountOpenResult(context.Background(), flow, baofucontracts.AccountResult{
		OpenState:     db.BaofuAccountOpenStateFailed,
		UpstreamState: "0",
		FailCode:      "BF00060",
		FailMessage:   "该子商户已开户，请勿重复提交",
		Raw:           []byte(`{"state":"0","errorCode":"BF00060"}`),
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateReady, applied.Flow.State)
	require.NotNil(t, applied.Binding)
	require.Equal(t, db.BaofuAccountOpenStateActive, applied.Binding.OpenState)
	require.Equal(t, "CP2026051600011958", applied.Binding.ContractNo.String)
	require.Equal(t, "CP2026051600011958", applied.Binding.SharingMerID.String)
	require.Equal(t, "LLBFOO0000000999", client.lastQuery.LoginNo)
	require.Equal(t, "110101199001011234", client.lastQuery.CertificateNo)
	require.Equal(t, baofucontracts.OfficialCertificateTypeID, client.lastQuery.CertificateType)
	require.Equal(t, "1338125", client.lastQuery.PlatformNo)
	require.Equal(t, db.BaofuAccountOpeningStateReady, store.flows[0].State)
	require.Equal(t, pgtype.Text{}, store.flows[0].FailureCode)
	require.Equal(t, pgtype.Text{}, store.flows[0].FailureMessage)
}

func TestBaofuAccountOnboardingServiceStart_MerchantActiveBindingWaitsForAppletAuth(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	store.activeBinding = db.BaofuAccountBinding{
		ID:           21,
		OwnerType:    db.BaofuAccountOwnerTypeMerchant,
		OwnerID:      88,
		AccountType:  db.BaofuAccountTypeBusiness,
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: "CM202605080088", Valid: true},
		SharingMerID: pgtype.Text{String: "CM202605080088", Valid: true},
	}
	store.merchantReports = append(store.merchantReports, db.BaofuMerchantReport{
		ID:              31,
		OwnerType:       db.BaofuAccountOwnerTypeMerchant,
		OwnerID:         88,
		ReportType:      db.BaofuMerchantReportTypeWechat,
		ReportState:     db.BaofuMerchantReportStateSucceeded,
		AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending,
		SubMchID:        pgtype.Text{String: "1900000118", Valid: true},
	})
	client := &fakeBaofuOnboardingAccountClient{}
	service := NewBaofuAccountOnboardingService(store, client, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931", CollectMerchantID: "100000"})

	result, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   88,
		UserID:    9,
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateAppletAuthPending, result.State)
	require.Zero(t, store.openAccountCalls)
}

func TestBaofuAccountOnboardingServiceStart_MerchantAppletAuthFailedReturnsGuidance(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	store.activeBinding = db.BaofuAccountBinding{
		ID:           22,
		OwnerType:    db.BaofuAccountOwnerTypeMerchant,
		OwnerID:      88,
		AccountType:  db.BaofuAccountTypeBusiness,
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: "CM202605080088", Valid: true},
		SharingMerID: pgtype.Text{String: "CM202605080088", Valid: true},
	}
	store.merchantReports = append(store.merchantReports, db.BaofuMerchantReport{
		ID:              32,
		OwnerType:       db.BaofuAccountOwnerTypeMerchant,
		OwnerID:         88,
		ReportType:      db.BaofuMerchantReportTypeWechat,
		ReportState:     db.BaofuMerchantReportStateSucceeded,
		AppletAuthState: db.BaofuMerchantReportAppletAuthStateFailed,
		FailureMessage:  pgtype.Text{String: "raw upstream auth failure for appid wx123456", Valid: true},
	})
	service := NewBaofuAccountOnboardingService(store, &fakeBaofuOnboardingAccountClient{}, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931"})

	result, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   88,
		UserID:    9,
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateFailed, result.State)
	require.Equal(t, "微信支付授权目录绑定失败，请联系平台处理后重试", result.StatusDesc)
	require.NotContains(t, result.StatusDesc, "raw upstream")
	require.Zero(t, store.openAccountCalls)
}

func TestBaofuAccountOnboardingServiceApplyAccountOpenResult_MerchantActiveWaitsForReport(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	reportClient := &fakeMerchantReportClient{
		reportResult: &merchantcontracts.MerchantReportResult{
			ReportNo:      "MR202605080088",
			ReportState:   "SUCCESS",
			SubMchID:      "1900000088",
			PlatformBizNo: "PB202605080088",
		},
		bindResult: &merchantcontracts.BindSubConfigResult{
			SubMchID:   "1900000088",
			AuthType:   merchantcontracts.AuthTypeApplet,
			ResultCode: "SUCCESS",
		},
	}
	service := NewBaofuAccountOnboardingService(store, nil, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931"}).
		WithMerchantReportContinuation(reportClient, BaofuAccountMerchantReportConfig{
			CollectMerchantID: "100000",
			CollectTerminalID: "200000",
			MiniProgramAppID:  "wx1234567890abcdef",
			ChannelID:         "CH001",
			ChannelName:       "LocalLife",
			Business:          "758-2",
		})
	profile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypeMerchant, 88, db.BaofuAccountTypeBusiness)
	profile.LegalName = pgtype.Text{String: "测试餐饮有限公司", Valid: true}
	profile.CertificateType = pgtype.Text{String: baofucontracts.OfficialBusinessCertificateTypeLicense, Valid: true}
	profile.CertificateNoCiphertext = pgtype.Text{String: "91330100MA00000001", Valid: true}
	profile.BankAccountNoCiphertext = pgtype.Text{String: "6222020202020202", Valid: true}
	profile.CardUserName = pgtype.Text{String: "测试餐饮有限公司", Valid: true}
	profile.DepositBankName = pgtype.Text{String: "招商银行杭州支行", Valid: true}
	store.profiles[0] = profile
	store.merchants[88] = db.Merchant{ID: 88, Name: "测试餐饮", Phone: "057112345678", Address: "测试路 1 号", RegionID: 330106}
	store.regions[330106] = db.Region{ID: 330106, Code: "330106", Level: 3, ParentID: pgtype.Int8{Int64: 330100, Valid: true}}
	store.regions[330100] = db.Region{ID: 330100, Code: "330100", Level: 2, ParentID: pgtype.Int8{Int64: 330000, Valid: true}}
	store.regions[330000] = db.Region{ID: 330000, Code: "330000", Level: 1}
	flow := db.BaofuAccountOpeningFlow{
		ID:                11,
		OwnerType:         db.BaofuAccountOwnerTypeMerchant,
		OwnerID:           88,
		AccountType:       db.BaofuAccountTypeBusiness,
		ProfileID:         pgtype.Int8{Int64: profile.ID, Valid: true},
		State:             db.BaofuAccountOpeningStateOpeningProcessing,
		OpenTransSerialNo: pgtype.Text{String: "BFO88", Valid: true},
		LoginNo:           pgtype.Text{String: "LLBFOM0000000088", Valid: true},
	}
	store.flows = append(store.flows, flow)
	store.activeBinding = db.BaofuAccountBinding{ID: 21, OwnerType: flow.OwnerType, OwnerID: flow.OwnerID, AccountType: flow.AccountType, OpenState: db.BaofuAccountOpenStateProcessing}

	applied, err := service.ApplyAccountOpenResult(context.Background(), flow, baofucontracts.AccountResult{
		ContractNo:    "CM202605080088",
		OpenState:     db.BaofuAccountOpenStateActive,
		UpstreamState: "1",
		Raw:           []byte(`{"state":"1"}`),
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateReady, applied.Flow.State)
	require.NotNil(t, applied.Binding)
	require.Equal(t, "CM202605080088", applied.Binding.ContractNo.String)
	require.Equal(t, "CM202605080088", applied.Binding.SharingMerID.String)
	require.True(t, store.platformFeeLedgerCreated)
	require.Equal(t, int64(200), store.lastLedger.Amount)
	require.Equal(t, "100000", reportClient.reportRequest.MerchantID)
	require.Equal(t, "100000", reportClient.bindRequest.MerchantID)
	require.Equal(t, "200000", reportClient.bindRequest.TerminalID)
	require.Equal(t, "wx1234567890abcdef", reportClient.bindRequest.AuthContent)
	require.Equal(t, "1900000088", store.merchantReports[0].SubMchID.String)
	require.Equal(t, db.BaofuMerchantReportAppletAuthStateSucceeded, store.merchantReports[0].AppletAuthState)
}

func TestBaofuAccountMerchantReportServiceRecoverReadyActivatesApprovedMerchant(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	flow := db.BaofuAccountOpeningFlow{
		ID:               702,
		OwnerType:        db.BaofuAccountOwnerTypeMerchant,
		OwnerID:          88,
		AccountType:      db.BaofuAccountTypeBusiness,
		State:            db.BaofuAccountOpeningStateAppletAuthPending,
		AccountBindingID: pgtype.Int8{Int64: 22, Valid: true},
		MerchantReportID: pgtype.Int8{Int64: 902, Valid: true},
	}
	store.flows = append(store.flows, flow)
	store.merchants[88] = db.Merchant{ID: 88, Status: "approved"}
	store.merchantReports = append(store.merchantReports, db.BaofuMerchantReport{
		ID:              902,
		OwnerType:       db.BaofuAccountOwnerTypeMerchant,
		OwnerID:         88,
		ReportType:      db.BaofuMerchantReportTypeWechat,
		ReportNo:        "MR202605090702",
		ReportState:     db.BaofuMerchantReportStateSucceeded,
		AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending,
		SubMchID:        pgtype.Text{String: "1900000702", Valid: true},
	})
	service := NewBaofuAccountMerchantReportService(store, &fakeMerchantReportClient{
		bindResult: &merchantcontracts.BindSubConfigResult{
			SubMchID:   "1900000702",
			AuthType:   merchantcontracts.AuthTypeApplet,
			ResultCode: "SUCCESS",
		},
	}, nil, BaofuAccountMerchantReportConfig{
		CollectMerchantID: "100000",
		CollectTerminalID: "200000",
		MiniProgramAppID:  "wx1234567890abcdef",
		ChannelID:         "CH001",
		ChannelName:       "LocalLife",
		Business:          "758-2",
	})

	updated, err := service.RecoverMerchantReportFlow(context.Background(), flow)

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateReady, updated.State)
	require.Equal(t, db.MerchantStatusActive, store.merchants[88].Status)
	require.Len(t, store.merchantPaymentConfigs, 1)
	require.Equal(t, db.MerchantPaymentConfigStatusActive, store.merchantPaymentConfigs[0].Status)
}

func TestBaofuAccountMerchantReportServiceRecoverTreatsRepeatBindAsSucceededWhenCommandExists(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	flow := db.BaofuAccountOpeningFlow{
		ID:               703,
		OwnerType:        db.BaofuAccountOwnerTypeMerchant,
		OwnerID:          88,
		AccountType:      db.BaofuAccountTypeBusiness,
		State:            db.BaofuAccountOpeningStateAppletAuthPending,
		AccountBindingID: pgtype.Int8{Int64: 22, Valid: true},
		MerchantReportID: pgtype.Int8{Int64: 903, Valid: true},
	}
	store.flows = append(store.flows, flow)
	store.merchants[88] = db.Merchant{ID: 88, Status: "approved"}
	store.merchantReports = append(store.merchantReports, db.BaofuMerchantReport{
		ID:              903,
		OwnerType:       db.BaofuAccountOwnerTypeMerchant,
		OwnerID:         88,
		ReportType:      db.BaofuMerchantReportTypeWechat,
		ReportNo:        "MR202605090703",
		ReportState:     db.BaofuMerchantReportStateSucceeded,
		AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending,
		SubMchID:        pgtype.Text{String: "1900000703", Valid: true},
	})
	store.commands = append(store.commands, db.CreateExternalPaymentCommandParams{
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		Capability:         db.ExternalPaymentCapabilityBaofuMerchantReport,
		CommandType:        db.ExternalPaymentCommandTypeBaofuBindSubConfig,
		ExternalObjectType: "baofu_bind_sub_config",
		ExternalObjectKey:  "1900000703",
	})
	service := NewBaofuAccountMerchantReportService(store, &fakeMerchantReportClient{
		bindErr: baofu.NewProviderBusinessError("bind_sub_config", "BIND_REPEAT_ERROR", "绑定关系已存在"),
	}, nil, BaofuAccountMerchantReportConfig{
		CollectMerchantID: "100000",
		CollectTerminalID: "200000",
		MiniProgramAppID:  "wx1234567890abcdef",
		ChannelID:         "CH001",
		ChannelName:       "LocalLife",
		Business:          "758-2",
	})

	updated, err := service.RecoverMerchantReportFlow(context.Background(), flow)

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateReady, updated.State)
	require.Equal(t, db.BaofuMerchantReportAppletAuthStateSucceeded, store.merchantReports[0].AppletAuthState)
	require.Equal(t, db.MerchantStatusActive, store.merchants[88].Status)
}

func TestBaofuAccountMerchantReportServiceRecoverDoesNotTreatRepeatBindAsSucceededWithoutPreviousCommand(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	flow := db.BaofuAccountOpeningFlow{
		ID:               704,
		OwnerType:        db.BaofuAccountOwnerTypeMerchant,
		OwnerID:          88,
		AccountType:      db.BaofuAccountTypeBusiness,
		State:            db.BaofuAccountOpeningStateAppletAuthPending,
		AccountBindingID: pgtype.Int8{Int64: 22, Valid: true},
		MerchantReportID: pgtype.Int8{Int64: 904, Valid: true},
	}
	store.flows = append(store.flows, flow)
	store.merchants[88] = db.Merchant{ID: 88, Status: db.MerchantStatusApproved}
	store.merchantReports = append(store.merchantReports, db.BaofuMerchantReport{
		ID:              904,
		OwnerType:       db.BaofuAccountOwnerTypeMerchant,
		OwnerID:         88,
		ReportType:      db.BaofuMerchantReportTypeWechat,
		ReportNo:        "MR202605090704",
		ReportState:     db.BaofuMerchantReportStateSucceeded,
		AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending,
		SubMchID:        pgtype.Text{String: "1900000704", Valid: true},
	})
	service := NewBaofuAccountMerchantReportService(store, &fakeMerchantReportClient{
		bindErr: baofu.NewProviderBusinessError("bind_sub_config", "BIND_REPEAT_ERROR", "绑定关系已存在"),
	}, nil, BaofuAccountMerchantReportConfig{
		CollectMerchantID: "100000",
		CollectTerminalID: "200000",
		MiniProgramAppID:  "wx1234567890abcdef",
		ChannelID:         "CH001",
		ChannelName:       "LocalLife",
		Business:          "758-2",
	})

	_, err := service.RecoverMerchantReportFlow(context.Background(), flow)

	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusBadGateway, reqErr.Status)
	require.EqualError(t, reqErr.Err, "微信支付授权目录绑定失败，请联系平台处理后重试")
	require.Equal(t, db.BaofuMerchantReportAppletAuthStatePending, store.merchantReports[0].AppletAuthState)
	require.Equal(t, db.MerchantStatusApproved, store.merchants[88].Status)
}

func TestBaofuAccountMerchantReportServiceRecoverProviderErrorReturnsSafeContext(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	providerErr := &baofu.ProviderError{
		Operation:       "bind_sub_config",
		Capability:      "baofu_merchant_report",
		UpstreamCode:    "NO_AUTH",
		UpstreamMessage: "raw upstream auth failure for appid wx123456",
	}
	client := &fakeMerchantReportClient{bindErr: providerErr}
	flow := db.BaofuAccountOpeningFlow{
		ID:                701,
		OwnerType:         db.BaofuAccountOwnerTypeMerchant,
		OwnerID:           88,
		AccountType:       db.BaofuAccountTypeBusiness,
		State:             db.BaofuAccountOpeningStateAppletAuthPending,
		OpenTransSerialNo: pgtype.Text{String: "BFO202605090701", Valid: true},
		MerchantReportID:  pgtype.Int8{Int64: 901, Valid: true},
	}
	store.flows = append(store.flows, flow)
	store.merchantReports = append(store.merchantReports, db.BaofuMerchantReport{
		ID:              901,
		OwnerType:       db.BaofuAccountOwnerTypeMerchant,
		OwnerID:         88,
		ReportType:      db.BaofuMerchantReportTypeWechat,
		ReportNo:        "MR202605090701",
		ReportState:     db.BaofuMerchantReportStateSucceeded,
		AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending,
		SubMchID:        pgtype.Text{String: "1900000701", Valid: true},
	})
	service := NewBaofuAccountMerchantReportService(store, client, nil, BaofuAccountMerchantReportConfig{
		CollectMerchantID: "100000",
		CollectTerminalID: "200000",
		MiniProgramAppID:  "wx1234567890abcdef",
		ChannelID:         "CH001",
		ChannelName:       "LocalLife",
		Business:          "758-2",
	})

	_, err := service.RecoverMerchantReportFlow(context.Background(), flow)

	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, reqErr.Status)
	require.EqualError(t, reqErr.Err, "微信支付授权目录绑定失败，请联系平台处理后重试")
	require.NotContains(t, reqErr.Err.Error(), providerErr.UpstreamMessage)
	var provider *baofu.ProviderError
	require.True(t, errors.As(LoggableError(err), &provider))
	require.Same(t, providerErr, provider)
	ctx, ok := BaofuProviderErrorContextFromError(LoggableError(err))
	require.True(t, ok)
	require.Equal(t, flow.ID, ctx.FlowID)
	require.Equal(t, db.BaofuAccountOwnerTypeMerchant, ctx.OwnerType)
	require.Equal(t, int64(88), ctx.OwnerID)
	require.Equal(t, "BFO202605090701", ctx.OpenTransSerialNo)
	require.Equal(t, db.BaofuAccountOpeningStateAppletAuthPending, ctx.CurrentState)
	require.Equal(t, int64(901), ctx.MerchantReportID)
	require.Equal(t, "MR202605090701", ctx.MerchantReportNo)
	require.Equal(t, "bind_sub_config", ctx.ProviderOperation)
}

func TestBaofuAccountOnboardingServiceRecoverOpeningQueriesByLoginNoAndAppliesResult(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	client := &fakeBaofuOnboardingAccountClient{
		queryResult: &baofucontracts.AccountResult{
			ContractNo:    "CP202605080066",
			OpenState:     db.BaofuAccountOpenStateActive,
			UpstreamState: "1",
			Raw:           []byte(`{"state":"1"}`),
		},
	}
	service := NewBaofuAccountOnboardingService(store, client, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931", CollectMerchantID: "100000"})
	profile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypeRider, 66, db.BaofuAccountTypePersonal)
	flow := store.mustCreateFlow(t, db.BaofuAccountOwnerTypeRider, 66, db.BaofuAccountTypePersonal, profile.ID)
	flow.State = db.BaofuAccountOpeningStateOpeningProcessing
	flow.OpenTransSerialNo = pgtype.Text{String: "BFO66", Valid: true}
	flow.LoginNo = pgtype.Text{String: "LLBFOR0000000066", Valid: true}
	store.flows[0] = flow
	store.activeBinding = db.BaofuAccountBinding{ID: 31, OwnerType: flow.OwnerType, OwnerID: flow.OwnerID, AccountType: flow.AccountType, LoginNo: flow.LoginNo, OpenState: db.BaofuAccountOpenStateProcessing}

	applied, err := service.RecoverOpeningFlow(context.Background(), flow)

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateReady, applied.Flow.State)
	require.Equal(t, "LLBFOR0000000066", client.lastQuery.LoginNo)
	require.Equal(t, "110101199001011234", client.lastQuery.CertificateNo)
	require.Equal(t, baofucontracts.OfficialCertificateTypeID, client.lastQuery.CertificateType)
	require.Equal(t, "100000", client.lastQuery.PlatformNo)
	require.False(t, store.platformFeeLedgerCreated)
}

func TestBaofuAccountOnboardingServiceStartRecoversProviderProgressFlow(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	client := &fakeBaofuOnboardingAccountClient{
		queryResult: &baofucontracts.AccountResult{
			ContractNo:    "CP202605170099",
			SharingMerID:  "CP202605170099",
			OpenState:     db.BaofuAccountOpenStateActive,
			UpstreamState: "1",
			Raw:           []byte(`{"state":"1","contractNo":"CP202605170099"}`),
		},
	}
	service := NewBaofuAccountOnboardingService(store, client, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931", CollectMerchantID: "100000"})
	profile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypePlatform, platformBaofuOpeningOwnerID, db.BaofuAccountTypeBusiness)
	flow := store.mustCreateFlow(t, db.BaofuAccountOwnerTypePlatform, platformBaofuOpeningOwnerID, db.BaofuAccountTypeBusiness, profile.ID)
	flow.State = db.BaofuAccountOpeningStateOpeningProcessing
	flow.OpenTransSerialNo = pgtype.Text{String: "BFO_PLATFORM", Valid: true}
	flow.LoginNo = pgtype.Text{String: "LLBFOP0000000000", Valid: true}
	store.flows[0] = flow
	store.activeBinding = db.BaofuAccountBinding{ID: 88, OwnerType: flow.OwnerType, OwnerID: flow.OwnerID, AccountType: flow.AccountType, LoginNo: flow.LoginNo, OpenState: db.BaofuAccountOpenStateProcessing}

	result, err := service.StartOrRecoverOpening(context.Background(), BaofuAccountOpeningInput{
		OwnerType: db.BaofuAccountOwnerTypePlatform,
		OwnerID:   platformBaofuOpeningOwnerID,
	})

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateReady, result.State)
	require.NotNil(t, result.Binding)
	require.Equal(t, db.BaofuAccountOpenStateActive, result.Binding.OpenState)
	require.Equal(t, "LLBFOP0000000000", client.lastQuery.LoginNo)
	require.Equal(t, "100000", client.lastQuery.PlatformNo)
	require.Equal(t, 0, client.openCalls)
}

func TestBaofuAccountOnboardingServiceRecoverOpeningReconcilesDuplicateFailedFlow(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	client := &fakeBaofuOnboardingAccountClient{
		queryResult: &baofucontracts.AccountResult{
			OutRequestNo:  "BFO_DUPLICATE_PREVIOUS",
			ContractNo:    "CP2026051600011958",
			SharingMerID:  "CP2026051600011958",
			OpenState:     db.BaofuAccountOpenStateActive,
			UpstreamState: "1",
			Raw:           []byte(`{"transSerialNo":"BFO_DUPLICATE_PREVIOUS","contractNo":"CP2026051600011958"}`),
		},
	}
	service := NewBaofuAccountOnboardingService(store, client, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931", CollectMerchantID: "1338125"})
	profile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypeOperator, 1, db.BaofuAccountTypePersonal)
	flow := store.mustCreateFlow(t, db.BaofuAccountOwnerTypeOperator, 1, db.BaofuAccountTypePersonal, profile.ID)
	flow.State = db.BaofuAccountOpeningStateFailed
	flow.OpenTransSerialNo = pgtype.Text{String: "BFO_DUPLICATE_CURRENT", Valid: true}
	flow.LoginNo = pgtype.Text{String: "LLBFOO0000000999", Valid: true}
	flow.FailureCode = pgtype.Text{String: "BF00060", Valid: true}
	flow.FailureMessage = pgtype.Text{String: "该子商户已开户，请勿重复提交", Valid: true}
	store.flows[0] = flow
	store.activeBinding = db.BaofuAccountBinding{ID: 22, OwnerType: flow.OwnerType, OwnerID: flow.OwnerID, AccountType: flow.AccountType, LoginNo: flow.LoginNo, LastOpenTransSerialNo: flow.OpenTransSerialNo, OpenState: db.BaofuAccountOpenStateFailed}

	applied, err := service.RecoverOpeningFlow(context.Background(), flow)

	require.NoError(t, err)
	require.Equal(t, db.BaofuAccountOpeningStateReady, applied.Flow.State)
	require.NotNil(t, applied.Binding)
	require.Equal(t, db.BaofuAccountOpenStateActive, applied.Binding.OpenState)
	require.Equal(t, "CP2026051600011958", applied.Binding.ContractNo.String)
	require.Equal(t, "CP2026051600011958", applied.Binding.SharingMerID.String)
	require.Equal(t, "LLBFOO0000000999", client.lastQuery.LoginNo)
	require.Equal(t, "1338125", client.lastQuery.PlatformNo)
	require.Equal(t, db.BaofuAccountOpeningStateReady, store.flows[0].State)
}

func TestBaofuAccountOnboardingServiceRecoverOpeningAlertsAndRejectsContractOwnedByDifferentOwner(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	client := &fakeBaofuOnboardingAccountClient{
		queryResult: &baofucontracts.AccountResult{
			ContractNo:    "CP_CONFLICT",
			OpenState:     db.BaofuAccountOpenStateActive,
			UpstreamState: "1",
			Raw:           []byte(`{"state":"1","contractNo":"CP_CONFLICT"}`),
		},
	}
	service := NewBaofuAccountOnboardingService(store, client, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931", CollectMerchantID: "100000"})
	profile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypeRider, 166, db.BaofuAccountTypePersonal)
	flow := store.mustCreateFlow(t, db.BaofuAccountOwnerTypeRider, 166, db.BaofuAccountTypePersonal, profile.ID)
	flow.State = db.BaofuAccountOpeningStateOpeningProcessing
	flow.OpenTransSerialNo = pgtype.Text{String: "BFO166", Valid: true}
	flow.LoginNo = pgtype.Text{String: "LLBFOR0000000166", Valid: true}
	store.flows[0] = flow
	store.activeBinding = db.BaofuAccountBinding{ID: 131, OwnerType: flow.OwnerType, OwnerID: flow.OwnerID, AccountType: flow.AccountType, LoginNo: flow.LoginNo, OpenState: db.BaofuAccountOpenStateProcessing}
	store.bindingsByContract = map[string]db.BaofuAccountBinding{
		"CP_CONFLICT": {
			ID:          901,
			OwnerType:   db.BaofuAccountOwnerTypeOperator,
			OwnerID:     999,
			AccountType: db.BaofuAccountTypePersonal,
			ContractNo:  pgtype.Text{String: "CP_CONFLICT", Valid: true},
			OpenState:   db.BaofuAccountOpenStateActive,
		},
	}

	_, err := service.RecoverOpeningFlow(context.Background(), flow)

	require.Error(t, err)
	require.Contains(t, err.Error(), "contract owner mismatch")
	require.Equal(t, db.BaofuAccountOpeningStateOpeningProcessing, store.flows[0].State)
	require.False(t, store.platformFeeLedgerCreated)
	require.Len(t, store.alerts, 1)
	require.Equal(t, "SYSTEM_ERROR", store.alerts[0].AlertType)
	require.Equal(t, "critical", store.alerts[0].Level)
	require.Equal(t, flow.ID, store.alerts[0].RelatedID)
	require.Equal(t, "baofu_account_opening_flow", store.alerts[0].RelatedType)
	var extra map[string]any
	require.NoError(t, json.Unmarshal(store.alerts[0].Extra, &extra))
	require.Equal(t, "contract_owner_mismatch", extra["reason"])
	require.Equal(t, "*******LICT", extra["contract_no_mask"])
	require.NotContains(t, string(store.alerts[0].Extra), "CP_CONFLICT")
	require.Equal(t, db.BaofuAccountOwnerTypeRider, extra["flow_owner_type"])
	require.Equal(t, db.BaofuAccountOwnerTypeOperator, extra["binding_owner_type"])
}

func TestBaofuAccountOnboardingServiceRecoverOpeningAlertsAndRejectsMismatchedOutRequestNo(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	client := &fakeBaofuOnboardingAccountClient{
		queryResult: &baofucontracts.AccountResult{
			OutRequestNo:  "BFO_OTHER",
			ContractNo:    "CP_QUERY_SERIAL_MISMATCH",
			OpenState:     db.BaofuAccountOpenStateActive,
			UpstreamState: "1",
			Raw:           []byte(`{"transSerialNo":"BFO_OTHER","state":"1","contractNo":"CP_QUERY_SERIAL_MISMATCH"}`),
		},
	}
	service := NewBaofuAccountOnboardingService(store, client, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931", CollectMerchantID: "100000"})
	profile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypeRider, 167, db.BaofuAccountTypePersonal)
	flow := store.mustCreateFlow(t, db.BaofuAccountOwnerTypeRider, 167, db.BaofuAccountTypePersonal, profile.ID)
	flow.State = db.BaofuAccountOpeningStateOpeningProcessing
	flow.OpenTransSerialNo = pgtype.Text{String: "BFO167", Valid: true}
	flow.LoginNo = pgtype.Text{String: "LLBFOR0000000167", Valid: true}
	store.flows[0] = flow
	store.activeBinding = db.BaofuAccountBinding{ID: 132, OwnerType: flow.OwnerType, OwnerID: flow.OwnerID, AccountType: flow.AccountType, LoginNo: flow.LoginNo, LastOpenTransSerialNo: flow.OpenTransSerialNo, OpenState: db.BaofuAccountOpenStateProcessing}

	_, err := service.RecoverOpeningFlow(context.Background(), flow)

	require.Error(t, err)
	require.Contains(t, err.Error(), "out request no mismatch")
	require.Equal(t, db.BaofuAccountOpeningStateOpeningProcessing, store.flows[0].State)
	require.Len(t, store.alerts, 1)
	var extra map[string]any
	require.NoError(t, json.Unmarshal(store.alerts[0].Extra, &extra))
	require.Equal(t, "out_request_no_mismatch", extra["reason"])
	require.Equal(t, "BFO167", extra["flow_open_trans_serial_no"])
	require.Equal(t, "BFO_OTHER", extra["result_out_request_no"])
	require.Equal(t, "********************ATCH", extra["contract_no_mask"])
	require.NotContains(t, string(store.alerts[0].Extra), "CP_QUERY_SERIAL_MISMATCH")
}

func TestBaofuAccountOnboardingServiceRecoverOpeningKeepsProcessingWhenQueryHasNoAccountRecord(t *testing.T) {
	store := newFakeBaofuAccountOnboardingStore()
	providerErr := baofu.NewProviderBusinessError("T-1001-013-03", "BF00064", "raw upstream account not found")
	client := &fakeBaofuOnboardingAccountClient{err: providerErr}
	service := NewBaofuAccountOnboardingService(store, client, nil, nil, BaofuAccountOnboardingConfig{VerifyFeeFen: 200, IndustryID: "9931", CollectMerchantID: "100000"})
	profile := store.mustUpsertProfile(t, db.BaofuAccountOwnerTypeMerchant, 88, db.BaofuAccountTypeBusiness)
	flow := store.mustCreateFlow(t, db.BaofuAccountOwnerTypeMerchant, 88, db.BaofuAccountTypeBusiness, profile.ID)
	flow.State = db.BaofuAccountOpeningStateOpeningProcessing
	flow.OpenTransSerialNo = pgtype.Text{String: "BFO202605171059041b2b8998", Valid: true}
	flow.LoginNo = pgtype.Text{String: "LLBFOM0000000088", Valid: true}
	store.flows[0] = flow
	store.activeBinding = db.BaofuAccountBinding{ID: 31, OwnerType: flow.OwnerType, OwnerID: flow.OwnerID, AccountType: flow.AccountType, LoginNo: flow.LoginNo, OpenState: db.BaofuAccountOpenStateProcessing}

	_, err := service.RecoverOpeningFlow(context.Background(), flow)

	reqErr := assertRequestError(t, err)
	require.Equal(t, 503, reqErr.Status)
	require.Equal(t, db.BaofuAccountOpeningStateOpeningProcessing, store.flows[0].State)
	require.False(t, store.flows[0].FailureCode.Valid)
	require.False(t, store.flows[0].FailureMessage.Valid)
	require.Equal(t, db.BaofuAccountOpenStateProcessing, store.activeBinding.OpenState)
	require.Equal(t, "100000", client.lastQuery.PlatformNo)
}

type fakeBaofuAccountOnboardingStore struct {
	profiles                 []db.BaofuAccountOpeningProfile
	flows                    []db.BaofuAccountOpeningFlow
	users                    map[int64]db.User
	payments                 []db.PaymentOrder
	merchantReports          []db.BaofuMerchantReport
	merchantPaymentConfigs   []db.MerchantPaymentConfig
	merchants                map[int64]db.Merchant
	regions                  map[int64]db.Region
	bindingsByContract       map[string]db.BaofuAccountBinding
	commands                 []db.CreateExternalPaymentCommandParams
	alerts                   []db.PlatformAlertEvent
	activeBinding            db.BaofuAccountBinding
	lastLedger               db.CreateBaofuFeeLedgerParams
	platformFeeLedgerCreated bool
	openAccountCalls         int
	nextID                   int64
}

func newFakeBaofuAccountOnboardingStore() *fakeBaofuAccountOnboardingStore {
	return &fakeBaofuAccountOnboardingStore{
		users:     map[int64]db.User{},
		merchants: map[int64]db.Merchant{},
		regions:   map[int64]db.Region{},
		nextID:    1,
	}
}

func (s *fakeBaofuAccountOnboardingStore) GetBaofuAccountBindingByOwner(context.Context, db.GetBaofuAccountBindingByOwnerParams) (db.BaofuAccountBinding, error) {
	if s.activeBinding.ID == 0 {
		return db.BaofuAccountBinding{}, db.ErrRecordNotFound
	}
	return s.activeBinding, nil
}

func (s *fakeBaofuAccountOnboardingStore) GetBaofuAccountBindingByContractNo(_ context.Context, contractNo pgtype.Text) (db.BaofuAccountBinding, error) {
	if s.bindingsByContract == nil || !contractNo.Valid {
		return db.BaofuAccountBinding{}, db.ErrRecordNotFound
	}
	binding, ok := s.bindingsByContract[contractNo.String]
	if !ok {
		return db.BaofuAccountBinding{}, db.ErrRecordNotFound
	}
	return binding, nil
}

func (s *fakeBaofuAccountOnboardingStore) GetBaofuMerchantReportByOwner(_ context.Context, arg db.GetBaofuMerchantReportByOwnerParams) (db.BaofuMerchantReport, error) {
	for _, report := range s.merchantReports {
		if report.OwnerType == arg.OwnerType && report.OwnerID == arg.OwnerID && report.ReportType == arg.ReportType {
			return report, nil
		}
	}
	return db.BaofuMerchantReport{}, db.ErrRecordNotFound
}

func (s *fakeBaofuAccountOnboardingStore) GetMerchant(_ context.Context, id int64) (db.Merchant, error) {
	merchant, ok := s.merchants[id]
	if !ok {
		return db.Merchant{}, db.ErrRecordNotFound
	}
	return merchant, nil
}

func (s *fakeBaofuAccountOnboardingStore) GetRegion(_ context.Context, id int64) (db.Region, error) {
	region, ok := s.regions[id]
	if !ok {
		return db.Region{}, db.ErrRecordNotFound
	}
	return region, nil
}

func (s *fakeBaofuAccountOnboardingStore) GetBaofuAccountOpeningProfileByOwner(_ context.Context, arg db.GetBaofuAccountOpeningProfileByOwnerParams) (db.BaofuAccountOpeningProfile, error) {
	for _, profile := range s.profiles {
		if profile.OwnerType == arg.OwnerType && profile.OwnerID == arg.OwnerID {
			return profile, nil
		}
	}
	return db.BaofuAccountOpeningProfile{}, db.ErrRecordNotFound
}

func (s *fakeBaofuAccountOnboardingStore) GetBaofuAccountOpeningProfile(_ context.Context, id int64) (db.BaofuAccountOpeningProfile, error) {
	for _, profile := range s.profiles {
		if profile.ID == id {
			return profile, nil
		}
	}
	return db.BaofuAccountOpeningProfile{}, db.ErrRecordNotFound
}

func (s *fakeBaofuAccountOnboardingStore) UpsertBaofuAccountOpeningProfile(_ context.Context, arg db.UpsertBaofuAccountOpeningProfileParams) (db.BaofuAccountOpeningProfile, error) {
	for i, profile := range s.profiles {
		if profile.OwnerType == arg.OwnerType && profile.OwnerID == arg.OwnerID {
			profile.AccountType = arg.AccountType
			profile.ProfileStatus = arg.ProfileStatus
			profile.LegalName = arg.LegalName
			profile.CertificateType = arg.CertificateType
			profile.CertificateNoCiphertext = arg.CertificateNoCiphertext
			profile.CertificateNoMask = arg.CertificateNoMask
			profile.EmailCiphertext = arg.EmailCiphertext
			profile.EmailMask = arg.EmailMask
			profile.CustomerName = arg.CustomerName
			profile.CorporateName = arg.CorporateName
			profile.CorporateCertType = arg.CorporateCertType
			profile.CorporateCertIDCiphertext = arg.CorporateCertIDCiphertext
			profile.CorporateCertIDMask = arg.CorporateCertIDMask
			profile.CorporateMobileCiphertext = arg.CorporateMobileCiphertext
			profile.CorporateMobileMask = arg.CorporateMobileMask
			profile.IndustryID = arg.IndustryID
			profile.ContactName = arg.ContactName
			profile.ContactMobileCiphertext = arg.ContactMobileCiphertext
			profile.ContactMobileMask = arg.ContactMobileMask
			profile.BankAccountNoCiphertext = arg.BankAccountNoCiphertext
			profile.BankAccountNoMask = arg.BankAccountNoMask
			profile.BankMobileCiphertext = arg.BankMobileCiphertext
			profile.BankMobileMask = arg.BankMobileMask
			profile.BankName = arg.BankName
			profile.DepositBankProvince = arg.DepositBankProvince
			profile.DepositBankCity = arg.DepositBankCity
			profile.DepositBankName = arg.DepositBankName
			profile.CardUserName = arg.CardUserName
			profile.SourceSnapshot = arg.SourceSnapshot
			s.profiles[i] = profile
			return profile, nil
		}
	}
	profile := db.BaofuAccountOpeningProfile{
		ID:                        s.next(),
		OwnerType:                 arg.OwnerType,
		OwnerID:                   arg.OwnerID,
		AccountType:               arg.AccountType,
		ProfileStatus:             arg.ProfileStatus,
		LegalName:                 arg.LegalName,
		CertificateType:           arg.CertificateType,
		CertificateNoCiphertext:   arg.CertificateNoCiphertext,
		CertificateNoMask:         arg.CertificateNoMask,
		EmailCiphertext:           arg.EmailCiphertext,
		EmailMask:                 arg.EmailMask,
		CustomerName:              arg.CustomerName,
		CorporateName:             arg.CorporateName,
		CorporateCertType:         arg.CorporateCertType,
		CorporateCertIDCiphertext: arg.CorporateCertIDCiphertext,
		CorporateCertIDMask:       arg.CorporateCertIDMask,
		CorporateMobileCiphertext: arg.CorporateMobileCiphertext,
		CorporateMobileMask:       arg.CorporateMobileMask,
		IndustryID:                arg.IndustryID,
		ContactName:               arg.ContactName,
		ContactMobileCiphertext:   arg.ContactMobileCiphertext,
		ContactMobileMask:         arg.ContactMobileMask,
		BankAccountNoCiphertext:   arg.BankAccountNoCiphertext,
		BankAccountNoMask:         arg.BankAccountNoMask,
		BankMobileCiphertext:      arg.BankMobileCiphertext,
		BankMobileMask:            arg.BankMobileMask,
		BankName:                  arg.BankName,
		DepositBankProvince:       arg.DepositBankProvince,
		DepositBankCity:           arg.DepositBankCity,
		DepositBankName:           arg.DepositBankName,
		CardUserName:              arg.CardUserName,
		SourceSnapshot:            arg.SourceSnapshot,
		CreatedAt:                 time.Now(),
		UpdatedAt:                 time.Now(),
	}
	s.profiles = append(s.profiles, profile)
	return profile, nil
}

func (s *fakeBaofuAccountOnboardingStore) GetActiveBaofuAccountOpeningFlowByOwner(_ context.Context, arg db.GetActiveBaofuAccountOpeningFlowByOwnerParams) (db.BaofuAccountOpeningFlow, error) {
	for _, flow := range s.flows {
		if flow.OwnerType == arg.OwnerType && flow.OwnerID == arg.OwnerID && flow.State != db.BaofuAccountOpeningStateReady && flow.State != db.BaofuAccountOpeningStateFailed && flow.State != db.BaofuAccountOpeningStateVoided {
			return flow, nil
		}
	}
	return db.BaofuAccountOpeningFlow{}, db.ErrRecordNotFound
}

func (s *fakeBaofuAccountOnboardingStore) GetBaofuAccountOpeningFlowByPaymentOrder(_ context.Context, paymentOrderID pgtype.Int8) (db.BaofuAccountOpeningFlow, error) {
	for _, flow := range s.flows {
		if flow.VerifyFeePaymentOrderID.Valid && flow.VerifyFeePaymentOrderID.Int64 == paymentOrderID.Int64 {
			return flow, nil
		}
	}
	return db.BaofuAccountOpeningFlow{}, db.ErrRecordNotFound
}

func (s *fakeBaofuAccountOnboardingStore) CreateBaofuAccountOpeningFlow(_ context.Context, arg db.CreateBaofuAccountOpeningFlowParams) (db.BaofuAccountOpeningFlow, error) {
	flow := db.BaofuAccountOpeningFlow{
		ID:                      s.next(),
		OwnerType:               arg.OwnerType,
		OwnerID:                 arg.OwnerID,
		AccountType:             arg.AccountType,
		ProfileID:               arg.ProfileID,
		State:                   arg.State,
		VerifyFeeAmount:         arg.VerifyFeeAmount,
		VerifyFeePaymentOrderID: arg.VerifyFeePaymentOrderID,
		OpenTransSerialNo:       arg.OpenTransSerialNo,
		LoginNo:                 arg.LoginNo,
		ProviderRequestSnapshot: arg.ProviderRequestSnapshot,
		RawSnapshot:             arg.RawSnapshot,
		CreatedAt:               time.Now(),
		UpdatedAt:               time.Now(),
	}
	s.flows = append(s.flows, flow)
	return flow, nil
}

func (s *fakeBaofuAccountOnboardingStore) VoidBaofuAccountOpeningFlow(_ context.Context, arg db.VoidBaofuAccountOpeningFlowParams) (db.BaofuAccountOpeningFlow, error) {
	return s.updateFlow(arg.ID, func(flow *db.BaofuAccountOpeningFlow) {
		flow.State = db.BaofuAccountOpeningStateVoided
		flow.FailureCode = arg.FailureCode
		flow.FailureMessage = arg.FailureMessage
		flow.RawSnapshot = arg.RawSnapshot
	})
}

func (s *fakeBaofuAccountOnboardingStore) MarkBaofuAccountOpeningFlowVerifyFeePending(_ context.Context, arg db.MarkBaofuAccountOpeningFlowVerifyFeePendingParams) (db.BaofuAccountOpeningFlow, error) {
	return s.updateFlow(arg.ID, func(flow *db.BaofuAccountOpeningFlow) {
		flow.State = db.BaofuAccountOpeningStateVerifyFeePending
		flow.VerifyFeeAmount = arg.VerifyFeeAmount
		flow.VerifyFeePaymentOrderID = arg.VerifyFeePaymentOrderID
	})
}

func (s *fakeBaofuAccountOnboardingStore) MarkBaofuAccountOpeningFlowVerifyFeeProcessing(_ context.Context, arg db.MarkBaofuAccountOpeningFlowVerifyFeeProcessingParams) (db.BaofuAccountOpeningFlow, error) {
	return s.updateFlow(arg.ID, func(flow *db.BaofuAccountOpeningFlow) {
		flow.State = db.BaofuAccountOpeningStateVerifyFeeProcessing
		flow.VerifyFeePaymentOrderID = arg.VerifyFeePaymentOrderID
	})
}

func (s *fakeBaofuAccountOnboardingStore) MarkBaofuAccountOpeningFlowOpeningProcessing(_ context.Context, arg db.MarkBaofuAccountOpeningFlowOpeningProcessingParams) (db.BaofuAccountOpeningFlow, error) {
	return s.updateFlow(arg.ID, func(flow *db.BaofuAccountOpeningFlow) {
		flow.State = db.BaofuAccountOpeningStateOpeningProcessing
		flow.ProfileID = arg.ProfileID
		flow.VerifyFeePaymentOrderID = arg.VerifyFeePaymentOrderID
		flow.OpenTransSerialNo = arg.OpenTransSerialNo
		flow.LoginNo = arg.LoginNo
		flow.ProviderRequestSnapshot = arg.ProviderRequestSnapshot
		flow.RawSnapshot = arg.RawSnapshot
	})
}

func (s *fakeBaofuAccountOnboardingStore) SetBaofuAccountOpeningFlowProfilePending(_ context.Context, arg db.SetBaofuAccountOpeningFlowProfilePendingParams) (db.BaofuAccountOpeningFlow, error) {
	return s.updateFlow(arg.ID, func(flow *db.BaofuAccountOpeningFlow) {
		flow.State = db.BaofuAccountOpeningStateProfilePending
		flow.ProfileID = arg.ProfileID
		flow.FailureCode = pgtype.Text{}
		flow.FailureMessage = pgtype.Text{}
		flow.RawSnapshot = arg.RawSnapshot
	})
}

func (s *fakeBaofuAccountOnboardingStore) MarkBaofuAccountOpeningFlowMerchantReportProcessing(_ context.Context, arg db.MarkBaofuAccountOpeningFlowMerchantReportProcessingParams) (db.BaofuAccountOpeningFlow, error) {
	return s.updateFlow(arg.ID, func(flow *db.BaofuAccountOpeningFlow) {
		flow.State = db.BaofuAccountOpeningStateMerchantReportProcessing
		flow.AccountBindingID = arg.AccountBindingID
		flow.MerchantReportID = arg.MerchantReportID
		flow.RawSnapshot = arg.RawSnapshot
	})
}

func (s *fakeBaofuAccountOnboardingStore) MarkBaofuAccountOpeningFlowAppletAuthPending(_ context.Context, arg db.MarkBaofuAccountOpeningFlowAppletAuthPendingParams) (db.BaofuAccountOpeningFlow, error) {
	return s.updateFlow(arg.ID, func(flow *db.BaofuAccountOpeningFlow) {
		flow.State = db.BaofuAccountOpeningStateAppletAuthPending
		flow.MerchantReportID = arg.MerchantReportID
		flow.RawSnapshot = arg.RawSnapshot
	})
}

func (s *fakeBaofuAccountOnboardingStore) MarkBaofuAccountOpeningFlowReady(_ context.Context, arg db.MarkBaofuAccountOpeningFlowReadyParams) (db.BaofuAccountOpeningFlow, error) {
	return s.updateFlowMatching(arg.ID, func(flow db.BaofuAccountOpeningFlow) bool {
		state := strings.TrimSpace(flow.State)
		return state == db.BaofuAccountOpeningStateOpeningProcessing ||
			state == db.BaofuAccountOpeningStateMerchantReportProcessing ||
			state == db.BaofuAccountOpeningStateAppletAuthPending ||
			state == db.BaofuAccountOpeningStateReady ||
			(state == db.BaofuAccountOpeningStateFailed && baofuAccountDuplicateFailureCode(flow.FailureCode.String))
	}, func(flow *db.BaofuAccountOpeningFlow) {
		flow.State = db.BaofuAccountOpeningStateReady
		flow.AccountBindingID = arg.AccountBindingID
		flow.FailureCode = pgtype.Text{}
		flow.FailureMessage = pgtype.Text{}
		flow.RawSnapshot = arg.RawSnapshot
	})
}

func (s *fakeBaofuAccountOnboardingStore) RecoverFailedBaofuAccountOpeningFlowFromActiveBinding(_ context.Context, arg db.RecoverFailedBaofuAccountOpeningFlowFromActiveBindingParams) (db.BaofuAccountOpeningFlow, error) {
	return s.updateFlowMatching(arg.ID, func(flow db.BaofuAccountOpeningFlow) bool {
		if strings.TrimSpace(flow.State) != db.BaofuAccountOpeningStateFailed &&
			strings.TrimSpace(flow.State) != db.BaofuAccountOpeningStateReady {
			return false
		}
		if !arg.OpenTransSerialNo.Valid || strings.TrimSpace(flow.OpenTransSerialNo.String) != strings.TrimSpace(arg.OpenTransSerialNo.String) {
			return false
		}
		binding := s.activeBinding
		return binding.ID == arg.AccountBindingID.Int64 &&
			strings.TrimSpace(binding.OwnerType) == strings.TrimSpace(flow.OwnerType) &&
			binding.OwnerID == flow.OwnerID &&
			strings.TrimSpace(binding.AccountType) == strings.TrimSpace(flow.AccountType) &&
			strings.TrimSpace(binding.OpenState) == db.BaofuAccountOpenStateActive &&
			strings.TrimSpace(binding.LastOpenTransSerialNo.String) == strings.TrimSpace(flow.OpenTransSerialNo.String) &&
			arg.ContractNo.Valid &&
			strings.TrimSpace(binding.ContractNo.String) == strings.TrimSpace(arg.ContractNo.String)
	}, func(flow *db.BaofuAccountOpeningFlow) {
		if strings.TrimSpace(flow.OwnerType) == db.BaofuAccountOwnerTypeMerchant &&
			strings.TrimSpace(flow.State) == db.BaofuAccountOpeningStateFailed {
			flow.State = db.BaofuAccountOpeningStateMerchantReportProcessing
		} else {
			flow.State = db.BaofuAccountOpeningStateReady
		}
		flow.AccountBindingID = arg.AccountBindingID
		flow.FailureCode = pgtype.Text{}
		flow.FailureMessage = pgtype.Text{}
		flow.RawSnapshot = arg.RawSnapshot
	})
}

func (s *fakeBaofuAccountOnboardingStore) MarkMerchantBaofuAccountOpeningReadyTx(ctx context.Context, arg db.MarkMerchantBaofuAccountOpeningReadyTxParams) (db.MarkMerchantBaofuAccountOpeningReadyTxResult, error) {
	cfg, err := s.UpsertMerchantPaymentConfig(ctx, arg.PaymentConfig)
	if err != nil {
		return db.MarkMerchantBaofuAccountOpeningReadyTxResult{}, err
	}
	flow, err := s.MarkBaofuAccountOpeningFlowReady(ctx, arg.Flow)
	if err != nil {
		return db.MarkMerchantBaofuAccountOpeningReadyTxResult{}, err
	}
	merchant, ok := s.merchants[arg.PaymentConfig.MerchantID]
	if !ok {
		return db.MarkMerchantBaofuAccountOpeningReadyTxResult{}, db.ErrRecordNotFound
	}
	if merchant.Status == db.MerchantStatusApproved {
		merchant.Status = db.MerchantStatusActive
		s.merchants[merchant.ID] = merchant
	}
	return db.MarkMerchantBaofuAccountOpeningReadyTxResult{PaymentConfig: cfg, Flow: flow, Merchant: merchant}, nil
}

func (s *fakeBaofuAccountOnboardingStore) MarkBaofuAccountOpeningFlowFailed(_ context.Context, arg db.MarkBaofuAccountOpeningFlowFailedParams) (db.BaofuAccountOpeningFlow, error) {
	return s.updateFlow(arg.ID, func(flow *db.BaofuAccountOpeningFlow) {
		flow.State = db.BaofuAccountOpeningStateFailed
		flow.FailureCode = arg.FailureCode
		flow.FailureMessage = arg.FailureMessage
		flow.RawSnapshot = arg.RawSnapshot
	})
}

func (s *fakeBaofuAccountOnboardingStore) UpsertBaofuAccountBinding(_ context.Context, arg db.UpsertBaofuAccountBindingParams) (db.BaofuAccountBinding, error) {
	s.activeBinding = db.BaofuAccountBinding{ID: s.next(), OwnerType: arg.OwnerType, OwnerID: arg.OwnerID, AccountType: arg.AccountType, LoginNo: arg.LoginNo, OpenState: arg.OpenState, LastOpenTransSerialNo: arg.LastOpenTransSerialNo, RawSnapshot: arg.RawSnapshot, UpdatedAt: time.Now()}
	return s.activeBinding, nil
}

func (s *fakeBaofuAccountOnboardingStore) MarkBaofuAccountBindingActive(_ context.Context, arg db.MarkBaofuAccountBindingActiveParams) (db.BaofuAccountBinding, error) {
	s.activeBinding.OpenState = db.BaofuAccountOpenStateActive
	s.activeBinding.ContractNo = arg.ContractNo
	s.activeBinding.SharingMerID = arg.SharingMerID
	return s.activeBinding, nil
}

func (s *fakeBaofuAccountOnboardingStore) MarkBaofuAccountBindingActiveWithFeeLedgerTx(_ context.Context, arg db.MarkBaofuAccountBindingActiveWithFeeLedgerTxParams) (db.MarkBaofuAccountBindingActiveWithFeeLedgerTxResult, error) {
	s.platformFeeLedgerCreated = true
	s.lastLedger = arg.AccountOpenFeeLedger
	s.activeBinding.OpenState = db.BaofuAccountOpenStateActive
	s.activeBinding.ContractNo = arg.ActiveBinding.ContractNo
	s.activeBinding.SharingMerID = arg.ActiveBinding.SharingMerID
	return db.MarkBaofuAccountBindingActiveWithFeeLedgerTxResult{Binding: s.activeBinding, AccountOpenFeeLedger: db.BaofuFeeLedger{ID: s.next(), Amount: arg.AccountOpenFeeLedger.Amount}}, nil
}

func (s *fakeBaofuAccountOnboardingStore) MarkBaofuAccountBindingFailed(_ context.Context, arg db.MarkBaofuAccountBindingFailedParams) (db.BaofuAccountBinding, error) {
	s.activeBinding.ID = arg.ID
	s.activeBinding.OpenState = db.BaofuAccountOpenStateFailed
	s.activeBinding.RawSnapshot = arg.RawSnapshot
	return s.activeBinding, nil
}

func (s *fakeBaofuAccountOnboardingStore) MarkBaofuAccountBindingAbnormal(_ context.Context, arg db.MarkBaofuAccountBindingAbnormalParams) (db.BaofuAccountBinding, error) {
	s.activeBinding.ID = arg.ID
	s.activeBinding.OpenState = db.BaofuAccountOpenStateAbnormal
	s.activeBinding.RawSnapshot = arg.RawSnapshot
	return s.activeBinding, nil
}

func (s *fakeBaofuAccountOnboardingStore) GetReusableBaofuVerifyFeePayment(_ context.Context, arg db.GetReusableBaofuVerifyFeePaymentParams) (db.PaymentOrder, error) {
	for _, payment := range s.payments {
		if payment.Attach.Valid && arg.Attach.Valid && payment.Attach.String == arg.Attach.String && payment.UserID == arg.UserID && payment.Amount == arg.Amount {
			return payment, nil
		}
	}
	return db.PaymentOrder{}, db.ErrRecordNotFound
}

func (s *fakeBaofuAccountOnboardingStore) CreatePaymentOrder(_ context.Context, arg db.CreatePaymentOrderParams) (db.PaymentOrder, error) {
	payment := db.PaymentOrder{ID: s.next(), UserID: arg.UserID, PaymentType: arg.PaymentType, PaymentChannel: arg.PaymentChannel, RequiresProfitSharing: arg.RequiresProfitSharing, BusinessType: arg.BusinessType, Amount: arg.Amount, OutTradeNo: arg.OutTradeNo, ExpiresAt: arg.ExpiresAt, Attach: arg.Attach, Status: "pending", CreatedAt: time.Now()}
	s.payments = append(s.payments, payment)
	return payment, nil
}

func (s *fakeBaofuAccountOnboardingStore) UpdatePaymentOrderPrepayId(_ context.Context, arg db.UpdatePaymentOrderPrepayIdParams) (db.PaymentOrder, error) {
	for i := range s.payments {
		if s.payments[i].ID == arg.ID {
			s.payments[i].PrepayID = arg.PrepayID
			return s.payments[i], nil
		}
	}
	return db.PaymentOrder{}, db.ErrRecordNotFound
}

func (s *fakeBaofuAccountOnboardingStore) UpdatePaymentOrderToClosed(_ context.Context, id int64) (db.PaymentOrder, error) {
	return db.PaymentOrder{ID: id, Status: "closed"}, nil
}

func (s *fakeBaofuAccountOnboardingStore) GetUser(_ context.Context, id int64) (db.User, error) {
	user, ok := s.users[id]
	if !ok {
		return db.User{}, db.ErrRecordNotFound
	}
	return user, nil
}

func (s *fakeBaofuAccountOnboardingStore) CreateExternalPaymentCommand(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
	s.commands = append(s.commands, arg)
	return db.ExternalPaymentCommand{ID: s.next()}, nil
}

func (s *fakeBaofuAccountOnboardingStore) GetExternalPaymentCommandByExternalObject(_ context.Context, arg db.GetExternalPaymentCommandByExternalObjectParams) (db.ExternalPaymentCommand, error) {
	for i := len(s.commands) - 1; i >= 0; i-- {
		command := s.commands[i]
		if command.Provider == arg.Provider &&
			command.Channel == arg.Channel &&
			command.Capability == arg.Capability &&
			command.CommandType == arg.CommandType &&
			command.ExternalObjectType == arg.ExternalObjectType &&
			command.ExternalObjectKey == arg.ExternalObjectKey {
			return db.ExternalPaymentCommand{
				ID:                 int64(i + 1),
				Provider:           command.Provider,
				Channel:            command.Channel,
				Capability:         command.Capability,
				CommandType:        command.CommandType,
				ExternalObjectType: command.ExternalObjectType,
				ExternalObjectKey:  command.ExternalObjectKey,
			}, nil
		}
	}
	return db.ExternalPaymentCommand{}, db.ErrRecordNotFound
}

func (s *fakeBaofuAccountOnboardingStore) UpsertBaofuMerchantReportProcessing(_ context.Context, arg db.UpsertBaofuMerchantReportProcessingParams) (db.BaofuMerchantReport, error) {
	report := db.BaofuMerchantReport{
		ID:              s.next(),
		OwnerType:       arg.OwnerType,
		OwnerID:         arg.OwnerID,
		ReportType:      arg.ReportType,
		ReportNo:        arg.ReportNo,
		BctMerID:        arg.BctMerID,
		ReportState:     db.BaofuMerchantReportStateProcessing,
		AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending,
		RawSnapshot:     arg.RawSnapshot,
	}
	s.merchantReports = append(s.merchantReports, report)
	return report, nil
}

func (s *fakeBaofuAccountOnboardingStore) MarkBaofuMerchantReportSucceeded(_ context.Context, arg db.MarkBaofuMerchantReportSucceededParams) (db.BaofuMerchantReport, error) {
	for i := range s.merchantReports {
		if s.merchantReports[i].ID == arg.ID {
			s.merchantReports[i].ReportState = db.BaofuMerchantReportStateSucceeded
			s.merchantReports[i].SubMchID = arg.SubMchID
			s.merchantReports[i].PlatformBizNo = arg.PlatformBizNo
			s.merchantReports[i].RawSnapshot = arg.RawSnapshot
			return s.merchantReports[i], nil
		}
	}
	return db.BaofuMerchantReport{}, db.ErrRecordNotFound
}

func (s *fakeBaofuAccountOnboardingStore) MarkBaofuMerchantReportFailed(_ context.Context, arg db.MarkBaofuMerchantReportFailedParams) (db.BaofuMerchantReport, error) {
	for i := range s.merchantReports {
		if s.merchantReports[i].ID == arg.ID {
			s.merchantReports[i].ReportState = db.BaofuMerchantReportStateFailed
			s.merchantReports[i].FailureCode = arg.FailureCode
			s.merchantReports[i].FailureMessage = arg.FailureMessage
			s.merchantReports[i].RawSnapshot = arg.RawSnapshot
			return s.merchantReports[i], nil
		}
	}
	return db.BaofuMerchantReport{}, db.ErrRecordNotFound
}

func (s *fakeBaofuAccountOnboardingStore) MarkBaofuMerchantReportAppletAuthSucceeded(_ context.Context, id int64) (db.BaofuMerchantReport, error) {
	for i := range s.merchantReports {
		if s.merchantReports[i].ID == id {
			s.merchantReports[i].AppletAuthState = db.BaofuMerchantReportAppletAuthStateSucceeded
			return s.merchantReports[i], nil
		}
	}
	return db.BaofuMerchantReport{}, db.ErrRecordNotFound
}

func (s *fakeBaofuAccountOnboardingStore) MarkBaofuMerchantReportAppletAuthFailed(_ context.Context, arg db.MarkBaofuMerchantReportAppletAuthFailedParams) (db.BaofuMerchantReport, error) {
	for i := range s.merchantReports {
		if s.merchantReports[i].ID == arg.ID {
			s.merchantReports[i].AppletAuthState = db.BaofuMerchantReportAppletAuthStateFailed
			s.merchantReports[i].FailureCode = arg.FailureCode
			s.merchantReports[i].FailureMessage = arg.FailureMessage
			return s.merchantReports[i], nil
		}
	}
	return db.BaofuMerchantReport{}, db.ErrRecordNotFound
}

func (s *fakeBaofuAccountOnboardingStore) UpsertMerchantPaymentConfig(_ context.Context, arg db.UpsertMerchantPaymentConfigParams) (db.MerchantPaymentConfig, error) {
	for i := range s.merchantPaymentConfigs {
		if s.merchantPaymentConfigs[i].MerchantID == arg.MerchantID {
			s.merchantPaymentConfigs[i].SubMchID = arg.SubMchID
			s.merchantPaymentConfigs[i].Status = arg.Status
			s.merchantPaymentConfigs[i].UpdatedAt = time.Now()
			return s.merchantPaymentConfigs[i], nil
		}
	}
	cfg := db.MerchantPaymentConfig{
		ID:         s.next(),
		MerchantID: arg.MerchantID,
		SubMchID:   arg.SubMchID,
		Status:     arg.Status,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	s.merchantPaymentConfigs = append(s.merchantPaymentConfigs, cfg)
	return cfg, nil
}

func (s *fakeBaofuAccountOnboardingStore) CreatePlatformAlertEvent(_ context.Context, arg db.CreatePlatformAlertEventParams) (db.PlatformAlertEvent, error) {
	alert := db.PlatformAlertEvent{
		ID:          s.next(),
		AlertType:   arg.AlertType,
		Level:       arg.Level,
		Title:       arg.Title,
		Message:     arg.Message,
		RelatedID:   arg.RelatedID,
		RelatedType: arg.RelatedType,
		Extra:       arg.Extra,
		EmittedAt:   arg.EmittedAt,
	}
	s.alerts = append(s.alerts, alert)
	return alert, nil
}

func (s *fakeBaofuAccountOnboardingStore) mustUpsertProfile(t *testing.T, ownerType string, ownerID int64, accountType string) db.BaofuAccountOpeningProfile {
	t.Helper()
	profile, err := s.UpsertBaofuAccountOpeningProfile(context.Background(), db.UpsertBaofuAccountOpeningProfileParams{
		OwnerType:               ownerType,
		OwnerID:                 ownerID,
		AccountType:             accountType,
		ProfileStatus:           db.BaofuAccountOpeningProfileStatusComplete,
		LegalName:               pgtype.Text{String: "张三", Valid: true},
		CertificateType:         pgtype.Text{String: baofucontracts.OfficialCertificateTypeID, Valid: true},
		CertificateNoCiphertext: pgtype.Text{String: "110101199001011234", Valid: true},
		BankAccountNoCiphertext: pgtype.Text{String: "6222020202020202", Valid: true},
		BankMobileCiphertext:    pgtype.Text{String: "13800138000", Valid: true},
		CardUserName:            pgtype.Text{String: "张三", Valid: true},
	})
	require.NoError(t, err)
	return profile
}

func (s *fakeBaofuAccountOnboardingStore) mustCreateFlow(t *testing.T, ownerType string, ownerID int64, accountType string, profileID int64) db.BaofuAccountOpeningFlow {
	t.Helper()
	flow, err := s.CreateBaofuAccountOpeningFlow(context.Background(), db.CreateBaofuAccountOpeningFlowParams{
		OwnerType:               ownerType,
		OwnerID:                 ownerID,
		AccountType:             accountType,
		ProfileID:               pgtype.Int8{Int64: profileID, Valid: true},
		State:                   db.BaofuAccountOpeningStateVerifyFeeProcessing,
		VerifyFeeAmount:         200,
		ProviderRequestSnapshot: []byte(`{}`),
		RawSnapshot:             []byte(`{}`),
	})
	require.NoError(t, err)
	return flow
}

func (s *fakeBaofuAccountOnboardingStore) updateFlow(id int64, mutate func(*db.BaofuAccountOpeningFlow)) (db.BaofuAccountOpeningFlow, error) {
	for i := range s.flows {
		if s.flows[i].ID == id {
			mutate(&s.flows[i])
			s.flows[i].UpdatedAt = time.Now()
			return s.flows[i], nil
		}
	}
	return db.BaofuAccountOpeningFlow{}, db.ErrRecordNotFound
}

func (s *fakeBaofuAccountOnboardingStore) updateFlowMatching(id int64, match func(db.BaofuAccountOpeningFlow) bool, mutate func(*db.BaofuAccountOpeningFlow)) (db.BaofuAccountOpeningFlow, error) {
	for i := range s.flows {
		if s.flows[i].ID == id {
			if !match(s.flows[i]) {
				return db.BaofuAccountOpeningFlow{}, db.ErrRecordNotFound
			}
			mutate(&s.flows[i])
			s.flows[i].UpdatedAt = time.Now()
			return s.flows[i], nil
		}
	}
	return db.BaofuAccountOpeningFlow{}, db.ErrRecordNotFound
}

func (s *fakeBaofuAccountOnboardingStore) next() int64 {
	id := s.nextID
	s.nextID++
	return id
}

type fakeBaofuOnboardingAccountClient struct {
	lastOpen    baofucontracts.OpenAccountRequest
	lastQuery   baofucontracts.QueryAccountRequest
	openResult  *baofucontracts.AccountResult
	queryResult *baofucontracts.AccountResult
	openCalls   int
	err         error
}

func (c *fakeBaofuOnboardingAccountClient) OpenAccount(_ context.Context, req baofucontracts.OpenAccountRequest) (*baofucontracts.AccountResult, error) {
	c.openCalls++
	c.lastOpen = req
	if c.err != nil {
		return nil, c.err
	}
	if c.openResult != nil {
		result := *c.openResult
		if result.OutRequestNo == "" {
			result.OutRequestNo = req.OutRequestNo
		}
		return &result, nil
	}
	return &baofucontracts.AccountResult{OpenState: db.BaofuAccountOpenStateProcessing, UpstreamState: "2"}, nil
}

func (c *fakeBaofuOnboardingAccountClient) QueryAccount(_ context.Context, req baofucontracts.QueryAccountRequest) (*baofucontracts.AccountResult, error) {
	c.lastQuery = req
	if c.err != nil {
		return nil, c.err
	}
	if c.queryResult != nil {
		return c.queryResult, nil
	}
	return &baofucontracts.AccountResult{OpenState: db.BaofuAccountOpenStateProcessing, UpstreamState: "2"}, nil
}

type fakeBaofuOnboardingDirectPaymentClient struct {
	prepayID string
}

func (c *fakeBaofuOnboardingDirectPaymentClient) GetMchID() string { return "mch" }
func (c *fakeBaofuOnboardingDirectPaymentClient) GetAppID() string { return "app" }
func (c *fakeBaofuOnboardingDirectPaymentClient) CreateJSAPIOrder(_ context.Context, _ *wechatcontracts.DirectJSAPIOrderRequest) (*wechatcontracts.DirectJSAPIOrderResponse, *wechat.JSAPIPayParams, error) {
	return &wechatcontracts.DirectJSAPIOrderResponse{PrepayID: c.prepayID}, &wechat.JSAPIPayParams{Package: "prepay_id=" + c.prepayID}, nil
}
func (c *fakeBaofuOnboardingDirectPaymentClient) QueryOrderByOutTradeNo(context.Context, string) (*wechatcontracts.DirectOrderQueryResponse, error) {
	return nil, nil
}
func (c *fakeBaofuOnboardingDirectPaymentClient) CloseOrder(context.Context, string) error {
	return nil
}
func (c *fakeBaofuOnboardingDirectPaymentClient) CreateRefund(context.Context, *wechat.RefundRequest) (*wechat.RefundResponse, error) {
	return nil, nil
}
func (c *fakeBaofuOnboardingDirectPaymentClient) QueryRefund(context.Context, string) (*wechat.RefundResponse, error) {
	return nil, nil
}
func (c *fakeBaofuOnboardingDirectPaymentClient) DecryptPaymentNotification(*wechat.PaymentNotification) (*wechatcontracts.DirectPaymentNotificationResource, error) {
	return nil, nil
}
func (c *fakeBaofuOnboardingDirectPaymentClient) DecryptRefundNotification(*wechat.PaymentNotification) (*wechatcontracts.DirectRefundNotificationResource, error) {
	return nil, nil
}
func (c *fakeBaofuOnboardingDirectPaymentClient) DecryptNotificationRaw(*wechat.PaymentNotification) ([]byte, error) {
	return nil, nil
}
func (c *fakeBaofuOnboardingDirectPaymentClient) VerifyNotificationSignature(string, string, string, string, string) error {
	return nil
}
func (c *fakeBaofuOnboardingDirectPaymentClient) GenerateJSAPIPayParams(prepayID string) (*wechat.JSAPIPayParams, error) {
	return &wechat.JSAPIPayParams{Package: "prepay_id=" + prepayID}, nil
}
