const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const weappRoot = path.join(__dirname, '..')
const supportPath = path.join(weappRoot, 'miniprogram/pages/takeout/order-confirm/_utils/takeout-order-confirm-support.ts')

function read(relativePath) {
  return fs.readFileSync(path.join(weappRoot, relativePath), 'utf8')
}

function loadOrderConfirmSupport() {
  const source = fs.readFileSync(supportPath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018,
      strict: true
    }
  }).outputText

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require: (request) => {
      if (request.endsWith('/api/cart')) return {}
      if (request.includes('/api/order')) return {}
      if (request.includes('/utils/image')) return { getPublicImageUrl: (value) => value || '' }
      if (request.includes('/utils/order-fee-breakdown-view')) {
        return { buildCustomerOrderFeeBreakdownView: () => ({ available: false }) }
      }
      if (request.includes('/utils/util')) {
        return { formatPriceNoSymbol: (fen) => (Number(fen || 0) / 100).toFixed(2) }
      }
      if (request.includes('/utils/global-store')) {
        return { globalStore: { set: () => {} } }
      }
      return require(request)
    }
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: supportPath })
  return sandbox.module.exports
}

function assertPackageWiring() {
  const pkg = JSON.parse(read('package.json'))
  assert(
    pkg.scripts['check:takeout-checkout-rehydration-payment-contract'],
    'package.json must expose the takeout checkout rehydration/payment contract check'
  )
  assert(
    pkg.scripts['quality:check'].includes('check:takeout-checkout-rehydration-payment-contract'),
    'quality:check must run the takeout checkout rehydration/payment contract check'
  )
}

function assertSourceIncludes(source, needles, message) {
  needles.forEach((needle) => {
    assert(source.includes(needle), `${message}: missing ${needle}`)
  })
}

function assertSnapshotIsDraftUntilBackendPricing() {
  const support = loadOrderConfirmSupport()
  const staleSnapshot = {
    cartIds: [101],
    carts: [
      {
        cartId: 101,
        merchantId: 11,
        merchantName: '旧价格商户',
        orderType: 'takeout',
        items: [
          {
            id: 301,
            dishId: 401,
            name: '旧价套餐',
            imageUrl: '',
            quantity: 1,
            unitPrice: 1000,
            subtotal: 1000
          }
        ],
        subtotal: 1000
      }
    ]
  }
  const patch = support.buildCheckoutSnapshotPatch(staleSnapshot, [])
  const [draftCart] = patch.carts

  assert.strictEqual(draftCart.orderTotal, 1000, 'snapshot may seed the draft total')
  assert.strictEqual(draftCart.deliveryFeeDisplay, '待计算', 'snapshot must remain visibly unpriced')
  assert.strictEqual(draftCart.paymentAssessment, null, 'snapshot must not carry backend payment assessment truth')
  assert.notStrictEqual(
    support.buildPricingKey({ id: 88 }, patch.carts),
    '',
    'takeout snapshot with an address must trigger backend pricing rehydration'
  )

  const pricedPatch = support.buildPricingSuccessPatch([
    {
      cart: draftCart,
      result: {
        subtotal: 1200,
        total_amount: 1600,
        delivery_fee: 500,
        delivery_fee_discount: 100,
        delivery_distance: 1200,
        delivery_eta_minutes: 32,
        payment_assessment: {
          is_balance_payable: false,
          usable_balance: 0,
          principal_part: 0,
          bonus_part: 0,
          payment_hint: '以后台试算为准'
        }
      }
    }
  ])
  assert.strictEqual(pricedPatch.carts[0].orderTotal, 1600, 'backend pricing must replace stale snapshot total')
  assert.strictEqual(pricedPatch.carts[0].deliveryFee, 500, 'backend pricing must replace delivery fee truth')
  assert.strictEqual(pricedPatch.carts[0].deliveryFeeDiscount, 100, 'backend pricing must preserve delivery discount truth')
  assert.strictEqual(pricedPatch.orderTotalDisplay, '16.00', 'visible total must come from backend pricing')
  assert.strictEqual(pricedPatch.pricingError, '', 'successful backend pricing must clear pricing error')

  const request = support.buildTakeoutCreateOrderRequest({
    cart: pricedPatch.carts[0],
    addressId: 88,
    note: '门口等',
    useBalance: false
  })
  assert.strictEqual(request.merchant_id, 11)
  assert.strictEqual(request.address_id, 88)
  assert.strictEqual(request.delivery_fee, 500)
  assert.strictEqual(request.delivery_fee_discount, 100)
  assert.strictEqual(request.delivery_distance, 1200)
}

function assertTakeoutOrderConfirmRecoveryContract() {
  const source = read('miniprogram/pages/takeout/order-confirm/index.ts')
  const paymentErrorCopy = read('miniprogram/pages/takeout/order-confirm/_utils/takeout-payment-error-copy.ts')

  assertSourceIncludes(source, [
    "openerEventChannel.on('checkoutContext'",
    'this.applyCheckoutSnapshot(payload)',
    'this._snapshotFallbackTimer = setTimeout',
    'this.loadCart()',
    'CartAPI.getUserCarts(this.data.orderType)',
    'CartAPI.getCart({',
    'this.requestPricingRefresh(true)',
    'CartAPI.calculateCart({',
    'if (pricingError)',
    "wx.showToast({ title: '请先重试代取费计算'",
    'const order = await createOrder(buildTakeoutCreateOrderRequest({',
    'await this.handlePayment(createdOrder)',
    'this.handlePartialOrderCreationFailure(carts, ordersCreated)',
    "title: '部分订单已创建'",
    "confirmText: '查看订单'",
    'Navigation.redirectToOrderList',
    'await completePaymentWorkflow(await createOrderPayment(orderId), { context: this })',
    'Navigation.toPaymentResult({',
    "status: paymentResult.status",
    'paymentOrderId: paymentResult.paymentOrderId',
    'businessId: orderId',
    "businessType: paymentResult.businessType || 'order'",
    'this.showPaymentCreateFailed(orderId, paymentError)',
    "title: '订单已创建'",
    'getTakeoutPaymentCreateFailedContent(error)',
    "wx.redirectTo({ url: `/pages/orders/detail/index?id=${orderId}` })"
  ], 'takeout order confirm must keep stale-draft rehydration, pricing guard, and payment recovery wiring')

  assert(
    !/createOrder\(buildTakeoutCreateOrderRequest\([\s\S]*pricingError[\s\S]*\)\)/.test(source),
    'createOrder must stay behind the explicit pricingError guard, not before it'
  )
  assert(
    paymentErrorCopy.includes('支付创建失败，请在订单详情页重新发起支付。') &&
      paymentErrorCopy.includes('该商户资质不完整，暂不支持下单'),
    'payment creation failure copy must send customers to durable order detail without leaking provider internals'
  )
}

function assertPaymentResultReentryContract() {
  const resultSource = read('miniprogram/pages/payment/result/index.ts')
  const workflowSource = read('miniprogram/pages/payment/_main_shared/services/payment-workflow.ts')

  assertSourceIncludes(resultSource, [
    "if (status === 'pending_confirmation')",
    'this.applyPendingConfirmationState(paymentOrderId)',
    'this.startPaymentStatusPolling()',
    "this.data.status === 'pending_confirmation'",
    'await waitForPaymentWorkflowTerminalResult(this.data.paymentOrderId',
    'businessId: result.businessId ? String(result.businessId) : this.data.businessId',
    'businessType: result.businessType ? String(result.businessType) : this.data.businessType',
    'amount: this.data.amount || formatAmount(result.amountFen)',
    'if (isPaymentWorkflowPaid(result.status))',
    'this.stopPaymentStatusPolling()',
    "statusNote: getErrorUserMessage(error, '支付结果还在同步中，系统会自动确认，也可返回订单详情查看。')",
    "wx.redirectTo({ url: `/pages/orders/detail/index?id=${this.data.businessId}` })",
    "wx.redirectTo({ url: `/pages/user_center/payment-detail/index?id=${this.data.paymentOrderId}` })",
    "wx.redirectTo({ url: '/pages/orders/list/index' })"
  ], 'payment result page must rehydrate pending order payment truth after reload/re-entry')

  assertSourceIncludes(workflowSource, [
    'await invokeWechatPay(payment.pay_params)',
    'const finalStatus = await pollPaymentStatus(',
    'return await queryPaymentWorkflowTerminalResult(payment.id, finalStatus)',
    "return buildPaymentWorkflowResultFromPayment(payment, 'pending_confirmation')",
    'const payment = await getPaymentDetail(paymentOrderId)',
    'return buildPaymentWorkflowResultFromPayment(payment, mapPaymentStatusToWorkflowStatus(finalStatus))'
  ], 'payment workflow must treat requestPayment as non-terminal and re-read backend payment truth')
}

function assertCustomerWrapperDriftBoundary() {
  const takeoutOrder = read('miniprogram/pages/takeout/order-confirm/_main_shared/api/order.ts')
  const ordersOrder = read('miniprogram/pages/orders/_main_shared/api/order.ts')
  const takeoutPayment = read('miniprogram/pages/takeout/order-confirm/_main_shared/api/payment.ts')
  const paymentResultPayment = read('miniprogram/pages/payment/_main_shared/api/payment.ts')

  assertSourceIncludes(takeoutOrder, [
    "url: '/v1/orders'",
    "method: 'POST'",
    'export async function createOrder',
    "url: `/v1/orders/${orderId}`",
    'export async function getOrderDetail'
  ], 'takeout order wrapper must keep create/detail contracts')

  assertSourceIncludes(ordersOrder, [
    "url: `/v1/orders/${orderId}`",
    'export async function getOrderDetail',
    'export interface OrderResponse'
  ], 'orders detail wrapper must keep durable order detail readback')

  assertSourceIncludes(takeoutPayment, [
    "url: '/v1/payments'",
    "business_type: 'order'",
    'export async function createOrderPayment',
    'export async function getPaymentDetail',
    'export async function pollPaymentStatus'
  ], 'takeout payment wrapper must keep order payment creation and polling contracts')

  assertSourceIncludes(paymentResultPayment, [
    'export async function getPaymentDetail',
    'export async function pollPaymentStatus',
    "url: `/v1/payments/${paymentId}`",
    "url: `/v1/payments/${paymentId}/query`"
  ], 'payment result wrapper must keep payment detail/query readback contracts')
}

function main() {
  assertPackageWiring()
  assertSnapshotIsDraftUntilBackendPricing()
  assertTakeoutOrderConfirmRecoveryContract()
  assertPaymentResultReentryContract()
  assertCustomerWrapperDriftBoundary()
  console.log('check-takeout-checkout-rehydration-payment-contract: validated stale snapshot rehydration and payment recovery contract')
}

main()
