package api

import (
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// 测试用的 Casbin 模型定义
const testCasbinModelDef = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && r.act == p.act
`

// 测试用的 Casbin 策略定义（精简版）
const testCasbinPolicyDef = `
# Role inheritance
g, operator, customer
g, merchant_owner, customer
g, merchant_staff, customer
g, rider, customer

# Admin policies
p, admin, /v1/platform/stats/*, GET
p, admin, /v1/platform/profit-sharing/*, GET
p, admin, /v1/platform/profit-sharing/*, POST
p, admin, /v1/platform/profit-sharing/*, PATCH
p, admin, /v1/platform/finance/*, GET
p, admin, /v1/platform/finance/*, POST
p, admin, /v1/platform/finance/*, PUT
p, admin, /v1/platform/finance/*, DELETE
p, admin, /v1/platform/refunds/*, POST
p, admin, /v1/admin/*, GET
p, admin, /v1/admin/*, POST
p, admin, /v1/groups, POST
p, admin, /v1/groups/applications/:id/review, POST
p, admin, /v1/food-safety/merchants/:id/suspend, PATCH
p, admin, /v1/fraud/detect, POST

# Operator policies
p, operator, /v1/operator/*, GET
p, operator, /v1/operator/*, POST
p, operator, /v1/operator/*, PATCH
p, operator, /v1/operator/*, DELETE
p, operator, /v1/operators/me/*, GET
p, operator, /v1/operators/me/*, POST
p, operator, /v1/operators/me/*, PUT
p, operator, /v1/delivery-fee/regions/:region_id/config, POST
p, operator, /v1/delivery-fee/regions/:region_id/config, PATCH
p, operator, /v1/reviews/:id, DELETE

# Merchant owner policies
p, merchant_owner, /v1/merchant/*, GET
p, merchant_owner, /v1/merchant/*, POST
p, merchant_owner, /v1/merchant/*, PATCH
p, merchant_owner, /v1/merchant/*, PUT
p, merchant_owner, /v1/dishes/*, GET
p, merchant_owner, /v1/dishes/*, POST
p, merchant_owner, /v1/dishes/*, PUT
p, merchant_owner, /v1/dishes/*, PATCH
p, merchant_owner, /v1/dishes/*, DELETE
p, merchant_owner, /v1/combos/*, GET
p, merchant_owner, /v1/combos/*, POST
p, merchant_owner, /v1/combos/*, PUT
p, merchant_owner, /v1/combos/*, DELETE
p, merchant_owner, /v1/inventory/*, GET
p, merchant_owner, /v1/inventory/*, POST
p, merchant_owner, /v1/inventory/*, PUT
p, merchant_owner, /v1/inventory/*, PATCH
p, merchant_owner, /v1/tables/*, GET
p, merchant_owner, /v1/tables/*, POST
p, merchant_owner, /v1/tables/*, PATCH
p, merchant_owner, /v1/tables/*, DELETE
p, merchant_owner, /v1/merchants/*, GET
p, merchant_owner, /v1/merchants/*, POST
p, merchant_owner, /v1/merchants/*, PATCH
p, merchant_owner, /v1/merchants/*, PUT
p, merchant_owner, /v1/reservations/*, GET
p, merchant_owner, /v1/reservations/*, POST
p, merchant_owner, /v1/kitchen/*, GET
p, merchant_owner, /v1/kitchen/*, POST
p, merchant_owner, /v1/reviews/*, GET
p, merchant_owner, /v1/reviews/*, POST
p, merchant_owner, /v1/delivery-fee/*, GET
p, merchant_owner, /v1/delivery-fee/*, POST
p, merchant_owner, /v1/delivery-fee/*, DELETE
p, merchant_owner, /v1/refunds/*, GET
p, merchant_owner, /v1/refunds/*, POST

# Merchant staff policies
p, merchant_staff, /v1/merchant/orders/*, GET
p, merchant_staff, /v1/merchant/orders/*, POST
p, merchant_staff, /v1/kitchen/*, GET
p, merchant_staff, /v1/kitchen/*, POST
p, merchant_staff, /v1/inventory/*, GET
p, merchant_staff, /v1/inventory/*, PATCH

# Rider policies
p, rider, /v1/rider/*, GET
p, rider, /v1/rider/*, POST
p, rider, /v1/delivery/*, GET
p, rider, /v1/delivery/*, POST

# Customer policies
p, customer, /v1/users/me, GET
p, customer, /v1/users/me, PATCH
p, customer, /v1/auth/*, POST
p, customer, /v1/addresses/*, GET
p, customer, /v1/addresses/*, POST
p, customer, /v1/addresses/*, PATCH
p, customer, /v1/addresses/*, DELETE
p, customer, /v1/orders/*, GET
p, customer, /v1/orders/*, POST
p, customer, /v1/payments/*, GET
p, customer, /v1/payments/*, POST
p, customer, /v1/notifications/*, GET
p, customer, /v1/notifications/*, PUT
p, customer, /v1/notifications/*, DELETE
p, customer, /v1/cart/*, GET
p, customer, /v1/cart/*, PATCH
p, customer, /v1/cart/*, DELETE
p, customer, /v1/favorites/*, GET
p, customer, /v1/favorites/*, POST
p, customer, /v1/favorites/*, DELETE
p, customer, /v1/memberships/*, GET
p, customer, /v1/memberships/*, POST
p, customer, /v1/reviews/*, GET
p, customer, /v1/reviews/*, POST
p, customer, /v1/vouchers/*, GET
p, customer, /v1/vouchers/*, POST
p, customer, /v1/reservations/*, GET
p, customer, /v1/reservations/*, POST
p, customer, /v1/history/*, GET
p, customer, /v1/rooms/*, GET
p, customer, /v1/claims/*, GET
p, customer, /v1/claims/*, POST
p, customer, /v1/food-safety/*, POST
p, customer, /v1/ws, GET
p, customer, /v1/delivery-fee/*, GET
p, customer, /v1/delivery-fee/calculate, POST
`

// initTestCasbin 初始化测试用的 Casbin enforcer
func initTestCasbin() error {
	enforcer, err := NewCasbinEnforcerFromString(testCasbinModelDef, testCasbinPolicyDef)
	if err != nil {
		return err
	}
	SetGlobalCasbinEnforcer(enforcer)
	return nil
}

func newTestServer(t *testing.T, store db.Store) *Server {
	config := util.Config{
		Environment:         "test",
		TokenSymmetricKey:   util.RandomString(32),
		AccessTokenDuration: time.Minute,
	}

	server, err := NewServer(config, store, nil, nil, NewNoopAuditWriter())
	require.NoError(t, err)

	// Disable websocket side effects for unit tests.
	server.wsHub = nil
	server.wsPubSub = nil
	server.merchantStatusChangePublisher = nil
	server.onboardingReviewService = nil
	server.credentialGovernanceService = nil

	return server
}

func TestSetDirectPaymentClientForTest_DoesNotPopulateTransferClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	server := newTestServer(t, nil)
	paymentClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)

	server.SetDirectPaymentClientForTest(paymentClient)

	require.Same(t, paymentClient, server.directPaymentClient)
	require.Nil(t, server.transferClient)
}

func TestSetPaymentClientsForTest_SetsBothClients(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	server := newTestServer(t, nil)
	directClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	transferClient := mockwechat.NewMockTransferClientInterface(ctrl)

	server.SetPaymentClientsForTest(directClient, transferClient)

	require.Same(t, directClient, server.directPaymentClient)
	require.Same(t, transferClient, server.transferClient)
}

func TestResetPaymentClientsForTest_ClearsBothClients(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	server := newTestServer(t, nil)
	directClient := mockwechat.NewMockDirectPaymentClientInterface(ctrl)
	transferClient := mockwechat.NewMockTransferClientInterface(ctrl)
	server.SetPaymentClientsForTest(directClient, transferClient)

	server.ResetPaymentClientsForTest()

	require.Nil(t, server.directPaymentClient)
	require.Nil(t, server.transferClient)
}

// newTestServerWithWechat creates a test server with a mock wechat client
func newTestServerWithWechat(t *testing.T, store db.Store, wechatClient interface{}) *Server {
	config := util.Config{
		Environment:         "test",
		TokenSymmetricKey:   util.RandomString(32),
		AccessTokenDuration: time.Minute,
	}

	server, err := NewServer(config, store, nil, nil, NewNoopAuditWriter())
	require.NoError(t, err)
	server.wechatClient = wechatClient.(wechat.WechatClient)
	server.wsHub = nil
	server.wsPubSub = nil
	server.onboardingReviewService = nil
	server.credentialGovernanceService = nil
	return server
}

// newTestServerWithPayment creates a test server with a mock payment client
func newTestServerWithPayment(t *testing.T, store db.Store, paymentClient wechat.DirectPaymentClientInterface) *Server {
	config := util.Config{
		Environment:         "test",
		TokenSymmetricKey:   util.RandomString(32),
		AccessTokenDuration: time.Minute,
	}

	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	require.NoError(t, err)

	server := &Server{
		config:              config,
		store:               store,
		tokenMaker:          tokenMaker,
		auditWriter:         NewNoopAuditWriter(),
		wechatClient:        nil,
		directPaymentClient: paymentClient,
		weatherCache:        nil,
		taskDistributor:     nil,
	}

	server.setupRouter()
	server.wsHub = nil
	server.wsPubSub = nil
	server.onboardingReviewService = nil
	server.credentialGovernanceService = nil
	return server
}

// newTestServerWithTaskDistributor creates a test server with a mock task distributor
func newTestServerWithTaskDistributor(t *testing.T, store db.Store, taskDistributor worker.TaskDistributor) *Server {
	config := util.Config{
		Environment:         "test",
		TokenSymmetricKey:   util.RandomString(32),
		AccessTokenDuration: time.Minute,
	}

	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	require.NoError(t, err)

	server := &Server{
		config:              config,
		store:               store,
		tokenMaker:          tokenMaker,
		auditWriter:         NewNoopAuditWriter(),
		wechatClient:        nil,
		directPaymentClient: nil,
		weatherCache:        nil,
		taskDistributor:     taskDistributor,
	}

	server.setupRouter()
	server.wsHub = nil
	server.wsPubSub = nil
	server.onboardingReviewService = nil
	server.credentialGovernanceService = nil
	return server
}

// newTestServerForMedia creates a test server with a temp-dir-backed LocalStorage.
// Returns the server and the temp directory (auto-cleaned on test end).
func newTestServerForMedia(t *testing.T, store db.Store) (*Server, string) {
	t.Helper()
	config := util.Config{
		Environment:         "test",
		TokenSymmetricKey:   util.RandomString(32),
		AccessTokenDuration: time.Minute,
	}

	server, err := NewServer(config, store, nil, nil, NewNoopAuditWriter())
	require.NoError(t, err)
	server.wsHub = nil
	server.wsPubSub = nil
	server.onboardingReviewService = nil
	server.credentialGovernanceService = nil

	tempDir := t.TempDir()
	ls := media.NewLocalStorage("http://testserver", tempDir)
	server.mediaRegistry = media.NewRegistry(store, ls)
	server.mediaResolver = media.NewURLResolver(media.ResolverConfig{
		CDNPublicBaseURL: "https://cdn.test.example.com",
		ThumbWidth:       200,
		CardWidth:        400,
		DetailWidth:      960,
	}, ls)

	return server, tempDir
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	// 初始化测试用 Casbin enforcer
	if err := initTestCasbin(); err != nil {
		panic("failed to initialize test Casbin: " + err.Error())
	}

	os.Exit(m.Run())
}
