package ordinaryserviceprovider

import (
	"testing"

	"github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
)

var ordinaryClientEndpointCoverage = map[contracts.EndpointID]string{
	contracts.EndpointApplymentSubmit:                    "Client.SubmitApplyment",
	contracts.EndpointApplymentQueryByID:                 "Client.QueryApplymentByID",
	contracts.EndpointApplymentQueryByBusinessCode:       "Client.QueryApplymentByBusinessCode",
	contracts.EndpointSettlementModify:                   "Client.ModifySettlement",
	contracts.EndpointSettlementQuery:                    "Client.QuerySettlement",
	contracts.EndpointSettlementModificationQuery:        "Client.QuerySettlementModification",
	contracts.EndpointMerchantMediaUpload:                "Client.UploadImage",
	contracts.EndpointViolationNotificationConfigQuery:   "Client.QueryViolationNotificationConfig",
	contracts.EndpointViolationNotificationConfigUpdate:  "Client.UpdateViolationNotificationConfig",
	contracts.EndpointViolationNotificationConfigCreate:  "Client.CreateViolationNotificationConfig",
	contracts.EndpointViolationNotificationConfigDelete:  "Client.DeleteViolationNotificationConfig",
	contracts.EndpointMerchantLimitationQuery:            "Client.QueryMerchantLimitation",
	contracts.EndpointInactiveMerchantVerificationCreate: "Client.CreateInactiveMerchantIdentityVerification",
	contracts.EndpointInactiveMerchantVerificationQuery:  "Client.QueryInactiveMerchantIdentityVerification",
	contracts.EndpointPaymentPrepay:                      "Client.CreatePayment",
	contracts.EndpointPaymentQueryByTransactionID:        "Client.QueryPayment",
	contracts.EndpointPaymentQueryByOutTradeNo:           "Client.QueryPayment",
	contracts.EndpointPaymentClose:                       "Client.ClosePayment",
	contracts.EndpointPaymentRefundCreate:                "Client.CreateRefund",
	contracts.EndpointPaymentRefundQuery:                 "Client.QueryRefund",
	contracts.EndpointCombinePrepay:                      "Client.CreateCombinePayment",
	contracts.EndpointCombineJSAPIPayParams:              "Client.GenerateJSAPIPayParams",
	contracts.EndpointCombineQuery:                       "Client.QueryCombinePayment",
	contracts.EndpointCombineClose:                       "Client.CloseCombinePayment",
	contracts.EndpointCombineRefundCreate:                "Client.CreateRefund",
	contracts.EndpointCombineRefundQuery:                 "Client.QueryRefund",
	contracts.EndpointRefundCreate:                       "Client.CreateRefund",
	contracts.EndpointRefundQuery:                        "Client.QueryRefund",
	contracts.EndpointProfitSharingCreate:                "Client.CreateProfitSharingOrder",
	contracts.EndpointProfitSharingQuery:                 "Client.QueryProfitSharingOrder",
	contracts.EndpointProfitSharingReturnCreate:          "Client.CreateProfitSharingReturn",
	contracts.EndpointProfitSharingReturnQuery:           "Client.QueryProfitSharingReturn",
	contracts.EndpointProfitSharingUnfreeze:              "Client.UnfreezeProfitSharing",
	contracts.EndpointProfitSharingRemainingAmount:       "Client.QueryProfitSharingRemainingAmount",
	contracts.EndpointProfitSharingReceiverAdd:           "Client.AddProfitSharingReceiver",
	contracts.EndpointProfitSharingReceiverDelete:        "Client.DeleteProfitSharingReceiver",
}

var ordinaryCallbackEndpointCoverage = map[contracts.EndpointID]string{
	contracts.EndpointMerchantViolationNotification: "Client.ParseNotification + api.HandleOrdinaryViolationNotification",
	contracts.EndpointPaymentNotify:                 "Client.ParseNotification + api.HandleWechatOrdinaryPaymentNotify",
	contracts.EndpointPaymentRefundNotify:           "Client.ParseNotification + api.HandleWechatOrdinaryRefundNotify",
	contracts.EndpointCombineNotify:                 "Client.ParseNotification + api.HandleWechatOrdinaryCombineNotify",
	contracts.EndpointCombineRefundNotify:           "Client.ParseNotification + api.HandleWechatOrdinaryRefundNotify",
	contracts.EndpointRefundNotify:                  "Client.ParseNotification + api.HandleWechatOrdinaryRefundNotify",
	contracts.EndpointProfitSharingNotify:           "Client.ParseNotification + api.HandleWechatOrdinaryProfitSharingNotify",
}

func TestOrdinaryServiceProviderEndpointContractsHaveRuntimeCoverage(t *testing.T) {
	for id, contract := range contracts.EndpointContracts() {
		switch contract.Method {
		case "CALLBACK":
			if ordinaryCallbackEndpointCoverage[id] == "" {
				t.Fatalf("%s callback endpoint has no runtime callback coverage", id)
			}
		default:
			if ordinaryClientEndpointCoverage[id] == "" {
				t.Fatalf("%s endpoint has no client runtime coverage", id)
			}
		}
	}
	for id, owner := range ordinaryClientEndpointCoverage {
		if owner == "" {
			t.Fatalf("%s client coverage owner is empty", id)
		}
		if _, ok := contracts.EndpointContractByID(id); !ok {
			t.Fatalf("%s client coverage references missing contract", id)
		}
	}
	for id, owner := range ordinaryCallbackEndpointCoverage {
		if owner == "" {
			t.Fatalf("%s callback coverage owner is empty", id)
		}
		if _, ok := contracts.EndpointContractByID(id); !ok {
			t.Fatalf("%s callback coverage references missing contract", id)
		}
	}
}
