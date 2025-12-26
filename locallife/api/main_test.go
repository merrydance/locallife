package api

import (
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
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
p, admin, /v1/admin/*, GET
p, admin, /v1/admin/*, POST

# Operator policies
p, operator, /v1/operator/*, GET
p, operator, /v1/operator/*, POST
p, operator, /v1/operator/*, PATCH
p, operator, /v1/operator/*, DELETE
p, operator, /v1/operators/me/*, GET
p, operator, /v1/delivery-fee/regions/:region_id/config, POST
p, operator, /v1/delivery-fee/regions/:region_id/config, PATCH
p, operator, /v1/regions/:id/recommendation-config, GET
p, operator, /v1/regions/:id/recommendation-config, PATCH
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
p, customer, /v1/cart/*, POST
p, customer, /v1/cart/*, PATCH
p, customer, /v1/cart/*, DELETE
p, customer, /v1/favorites/*, GET
p, customer, /v1/favorites/*, POST
p, customer, /v1/favorites/*, DELETE
p, customer, /v1/memberships/*, GET
p, customer, /v1/memberships/*, POST
p, customer, /v1/reviews/*, GET
p, customer, /v1/reviews/*, POST
p, customer, /v1/recommendations/*, GET
p, customer, /v1/behaviors/*, POST
p, customer, /v1/vouchers/*, GET
p, customer, /v1/vouchers/*, POST
p, customer, /v1/reservations/*, GET
p, customer, /v1/reservations/*, POST
p, customer, /v1/history/*, GET
p, customer, /v1/rooms/*, GET
p, customer, /v1/trust-score/*, GET
p, customer, /v1/trust-score/*, POST
p, customer, /v1/claims/*, GET
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
		TokenSymmetricKey:   util.RandomString(32),
		AccessTokenDuration: time.Minute,
	}

	server, err := NewServer(config, store, nil, nil)
	require.NoError(t, err)

	return server
}

// newTestServerWithWechat creates a test server with a mock wechat client
func newTestServerWithWechat(t *testing.T, store db.Store, wechatClient interface{}) *Server {
	config := util.Config{
		TokenSymmetricKey:   util.RandomString(32),
		AccessTokenDuration: time.Minute,
	}

	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	require.NoError(t, err)

	server := &Server{
		config:          config,
		store:           store,
		tokenMaker:      tokenMaker,
		wechatClient:    wechatClient.(wechat.WechatClient),
		weatherCache:    nil,
		taskDistributor: nil,
	}

	server.setupRouter()
	return server
}

// newTestServerWithPayment creates a test server with a mock payment client
func newTestServerWithPayment(t *testing.T, store db.Store, paymentClient wechat.PaymentClientInterface) *Server {
	config := util.Config{
		TokenSymmetricKey:   util.RandomString(32),
		AccessTokenDuration: time.Minute,
	}

	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	require.NoError(t, err)

	server := &Server{
		config:          config,
		store:           store,
		tokenMaker:      tokenMaker,
		wechatClient:    nil,
		paymentClient:   paymentClient,
		weatherCache:    nil,
		taskDistributor: nil,
	}

	server.setupRouter()
	return server
}

// newTestServerWithTaskDistributor creates a test server with a mock task distributor
func newTestServerWithTaskDistributor(t *testing.T, store db.Store, taskDistributor worker.TaskDistributor) *Server {
	config := util.Config{
		TokenSymmetricKey:   util.RandomString(32),
		AccessTokenDuration: time.Minute,
	}

	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	require.NoError(t, err)

	server := &Server{
		config:          config,
		store:           store,
		tokenMaker:      tokenMaker,
		wechatClient:    nil,
		paymentClient:   nil,
		weatherCache:    nil,
		taskDistributor: taskDistributor,
	}

	server.setupRouter()
	return server
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	// 初始化测试用 Casbin enforcer
	if err := initTestCasbin(); err != nil {
		panic("failed to initialize test Casbin: " + err.Error())
	}

	os.Exit(m.Run())
}
