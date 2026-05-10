package api

import (
	"github.com/merrydance/locallife/baofu/aggregatepay"
	"github.com/merrydance/locallife/cloudprint"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/wechat"
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	"github.com/merrydance/locallife/worker"
)

// SetDirectPaymentClientForTest injects a payment client in tests.
// It rebuilds the cached order services immediately so they pick up the new
// client; this prevents nil-pointer panics in handlers that access
// orderCommandSvc / orderQuerySvc directly. Transfer client injection must be
// handled separately via SetTransferClientForTest.
func (server *Server) SetDirectPaymentClientForTest(client wechat.DirectPaymentClientInterface) {
	server.directPaymentClient = client
	newSvc := server.buildOrderCommandService()
	server.orderCommandSvc = newSvc
	if qs, ok := newSvc.(logic.OrderQueryService); ok {
		server.orderQuerySvc = qs
	}
}

// SetTransferClientForTest injects a transfer client in tests.
func (server *Server) SetTransferClientForTest(client wechat.TransferClientInterface) {
	server.transferClient = client
}

// SetPaymentClientsForTest injects direct payment and transfer clients together
// for tests that need to manage both capabilities as one runtime fixture.
func (server *Server) SetPaymentClientsForTest(directClient wechat.DirectPaymentClientInterface, transferClient wechat.TransferClientInterface) {
	server.directPaymentClient = directClient
	server.transferClient = transferClient
	newSvc := server.buildOrderCommandService()
	server.orderCommandSvc = newSvc
	if qs, ok := newSvc.(logic.OrderQueryService); ok {
		server.orderQuerySvc = qs
	}
}

// ResetPaymentClientsForTest clears direct payment and transfer clients
// together so shared test servers do not leak runtime state across cases.
func (server *Server) ResetPaymentClientsForTest() {
	server.SetPaymentClientsForTest(nil, nil)
}

// SetTaskDistributorForTest injects a task distributor in tests.
func (server *Server) SetTaskDistributorForTest(distributor worker.TaskDistributor) {
	server.taskDistributor = distributor
	newSvc := server.buildOrderCommandService()
	server.orderCommandSvc = newSvc
	if qs, ok := newSvc.(logic.OrderQueryService); ok {
		server.orderQuerySvc = qs
	}
}

// SetEcommerceClientForTest injects an ecommerce client in tests.
// It also clears the cached paymentFacade and refundOrchestrator so they are
// rebuilt with the new client on the next request.
func (server *Server) SetEcommerceClientForTest(client wechat.EcommerceClientInterface) {
	server.ecommerceClient = client
	newSvc := server.buildOrderCommandService()
	server.orderCommandSvc = newSvc
	if qs, ok := newSvc.(logic.OrderQueryService); ok {
		server.orderQuerySvc = qs
	}
	server.paymentFacade = nil
	server.refundOrchestrator = nil
}

func (server *Server) SetOrdinaryServiceProviderClientForTest(client ordinaryserviceprovider.OrdinaryServiceProviderClientInterface) {
	server.ordinarySPClient = client
	newSvc := server.buildOrderCommandService()
	server.orderCommandSvc = newSvc
	if qs, ok := newSvc.(logic.OrderQueryService); ok {
		server.orderQuerySvc = qs
	}
	server.paymentFacade = nil
	server.refundOrchestrator = nil
}

func (server *Server) SetBaofuAggregateClientForTest(client aggregatepay.Client, config logic.BaofuAggregateFacadeConfig) {
	server.baofuAggregateClient = client
	server.config.BaofuMainBusinessEnabled = true
	server.config.BaofuCollectMerchantID = config.CollectMerchantID
	server.config.BaofuCollectTerminalID = config.CollectTerminalID
	server.config.WechatMiniAppID = config.MiniProgramAppID
	server.config.BaofuPaymentNotifyURL = config.PaymentNotifyURL
	server.config.BaofuRefundNotifyURL = config.RefundNotifyURL
	server.paymentFacade = nil
	server.refundOrchestrator = nil
}

func (server *Server) SetPrinterClientForTest(client cloudprint.Client) {
	server.printerClient = client
}
