package db

import "testing"

func TestPaymentOrderUsesOrdinaryServiceProviderChannel(t *testing.T) {
	if PaymentChannelOrdinaryServiceProvider != "ordinary_service_provider" {
		t.Fatalf("ordinary service provider channel = %q", PaymentChannelOrdinaryServiceProvider)
	}

	if !PaymentOrderUsesOrdinaryServiceProviderChannel(PaymentOrder{PaymentChannel: PaymentChannelOrdinaryServiceProvider}) {
		t.Fatal("expected ordinary service provider channel to be recognized")
	}

	if PaymentOrderUsesOrdinaryServiceProviderChannel(PaymentOrder{PaymentChannel: PaymentChannelEcommerce}) {
		t.Fatal("did not expect ecommerce channel to be recognized as ordinary service provider")
	}

	if PaymentOrderUsesOrdinaryServiceProviderChannel(PaymentOrder{PaymentChannel: PaymentChannelDirect}) {
		t.Fatal("did not expect direct channel to be recognized as ordinary service provider")
	}
}

func TestPaymentOrderRequiresProfitSharingIncludesOrdinaryServiceProvider(t *testing.T) {
	if !PaymentOrderRequiresProfitSharing(PaymentOrder{PaymentChannel: PaymentChannelOrdinaryServiceProvider, RequiresProfitSharing: true}) {
		t.Fatal("expected ordinary service provider profit sharing payment order to require profit sharing")
	}

	if PaymentOrderRequiresProfitSharing(PaymentOrder{PaymentChannel: PaymentChannelDirect, RequiresProfitSharing: true}) {
		t.Fatal("did not expect direct payment order to require main-business profit sharing")
	}
}

func TestBaofuPaymentConstantsAreExplicit(t *testing.T) {
	if PaymentChannelBaofuAggregate != "baofu_aggregate" {
		t.Fatalf("baofu aggregate channel = %q", PaymentChannelBaofuAggregate)
	}
	if ExternalPaymentProviderBaofu != "baofu" {
		t.Fatalf("baofu provider = %q", ExternalPaymentProviderBaofu)
	}
	if ExternalPaymentCapabilityBaofuAccount != "baofu_account" {
		t.Fatalf("baofu account capability = %q", ExternalPaymentCapabilityBaofuAccount)
	}
	if ExternalPaymentCapabilityBaofuPayment != "baofu_payment" {
		t.Fatalf("baofu payment capability = %q", ExternalPaymentCapabilityBaofuPayment)
	}
	if ExternalPaymentCapabilityBaofuProfitSharing != "baofu_profit_sharing" {
		t.Fatalf("baofu profit sharing capability = %q", ExternalPaymentCapabilityBaofuProfitSharing)
	}
	if ExternalPaymentCapabilityBaofuWithdraw != "baofu_withdraw" {
		t.Fatalf("baofu withdraw capability = %q", ExternalPaymentCapabilityBaofuWithdraw)
	}
}
