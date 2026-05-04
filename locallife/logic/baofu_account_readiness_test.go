package logic

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestBaofuAccountReadinessStates(t *testing.T) {
	service := NewBaofuAccountService(nil, nil)

	tests := []struct {
		name                  string
		found                 bool
		binding               db.BaofuAccountBinding
		requireWechatSubMchID bool
		wantState             string
		wantLabel             string
		wantPaymentReady      bool
	}{
		{
			name:             "missing binding asks for profile submission",
			wantState:        BaofuOnboardingStateProfilePending,
			wantLabel:        "资料待提交",
			wantPaymentReady: false,
		},
		{
			name:  "processing binding asks user to wait",
			found: true,
			binding: db.BaofuAccountBinding{
				OpenState: db.BaofuAccountOpenStateProcessing,
			},
			wantState:        BaofuOnboardingStateOpeningProcessing,
			wantLabel:        "宝付开户处理中",
			wantPaymentReady: false,
		},
		{
			name:  "merchant active binding still waits for wechat channel identity",
			found: true,
			binding: db.BaofuAccountBinding{
				OwnerType:    db.BaofuAccountOwnerTypeMerchant,
				AccountType:  db.BaofuAccountTypeBusiness,
				OpenState:    db.BaofuAccountOpenStateActive,
				ContractNo:   pgtype.Text{String: "CM123", Valid: true},
				SharingMerID: pgtype.Text{String: "CM123", Valid: true},
			},
			requireWechatSubMchID: true,
			wantState:             BaofuOnboardingStateWechatChannelPending,
			wantLabel:             "微信支付通道待开通",
			wantPaymentReady:      false,
		},
		{
			name:  "active rider binding is ready without wechat channel identity",
			found: true,
			binding: db.BaofuAccountBinding{
				OwnerType:    db.BaofuAccountOwnerTypeRider,
				AccountType:  db.BaofuAccountTypePersonal,
				OpenState:    db.BaofuAccountOpenStateActive,
				ContractNo:   pgtype.Text{String: "CP123", Valid: true},
				SharingMerID: pgtype.Text{String: "CP123", Valid: true},
			},
			wantState:        BaofuOnboardingStateReady,
			wantLabel:        "结算账户可用",
			wantPaymentReady: true,
		},
		{
			name:  "active rider binding with non personal account is not ready",
			found: true,
			binding: db.BaofuAccountBinding{
				OwnerType:    db.BaofuAccountOwnerTypeRider,
				AccountType:  db.BaofuAccountTypeBusiness,
				OpenState:    db.BaofuAccountOpenStateActive,
				ContractNo:   pgtype.Text{String: "CB123", Valid: true},
				SharingMerID: pgtype.Text{String: "CB123", Valid: true},
			},
			wantState:        BaofuOnboardingStateOpenFailed,
			wantLabel:        "开通失败",
			wantPaymentReady: false,
		},
		{
			name:  "failed binding surfaces failed state",
			found: true,
			binding: db.BaofuAccountBinding{
				OpenState: db.BaofuAccountOpenStateFailed,
			},
			wantState:        BaofuOnboardingStateOpenFailed,
			wantLabel:        "开通失败",
			wantPaymentReady: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.ReadinessFromBinding(tt.binding, tt.found, tt.requireWechatSubMchID)

			require.Equal(t, tt.wantState, got.State)
			require.Equal(t, tt.wantLabel, got.Label)
			require.Equal(t, tt.wantPaymentReady, got.PaymentReady)
		})
	}
}
