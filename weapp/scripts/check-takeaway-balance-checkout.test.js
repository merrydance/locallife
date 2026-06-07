const assert = require('assert')
const fs = require('fs')
const path = require('path')
const vm = require('vm')
const ts = require('typescript')

const weappRoot = path.join(__dirname, '..')
const supportPath = path.join(weappRoot, 'miniprogram/pages/takeout/order-confirm/_utils/takeout-order-confirm-support.ts')

function read(relativePath) {
  return fs.readFileSync(path.join(weappRoot, relativePath), 'utf8')
}

function assertTDesignComponentExists(componentPath) {
  const distPath = path.join(weappRoot, 'miniprogram/miniprogram_npm', `${componentPath}.json`)
  const sourcePath = path.join(
    weappRoot,
    'node_modules',
    `${componentPath.replace('tdesign-miniprogram/', 'tdesign-miniprogram/miniprogram_dist/')}.json`
  )
  assert(
    fs.existsSync(distPath) || fs.existsSync(sourcePath),
    `declared TDesign component must exist: ${componentPath}`
  )
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

function createCart(orderType) {
  return {
    merchantId: 11,
    merchantName: '自取商户',
    orderType,
    items: [
      {
        id: 101,
        dishId: 201,
        name: '招牌饭',
        imageUrl: '',
        quantity: 1,
        unitPrice: 1200,
        priceDisplay: '12.00',
        subtotal: 1200,
        subtotalDisplay: '12.00'
      }
    ],
    totalCount: 1,
    subtotal: 1200,
    subtotalDisplay: '12.00',
    deliveryFee: 0,
    deliveryFeeDisplay: '待计算',
    deliveryFeeDiscount: 0,
    deliveryDistance: 0,
    deliveryEtaMinutes: 0,
    deliveryEtaDisplay: '',
    orderTotal: 1200,
    orderTotalDisplay: '12.00',
    originalTotalDisplay: '12.00',
    hasDiscount: false,
    appliedPromotions: [],
    ladderPromotions: [],
    voucherTrials: [],
    paymentHint: '',
    feeBreakdownView: { available: false }
  }
}

const support = loadOrderConfirmSupport()
const takeawayCart = createCart('takeaway')
const takeoutCart = createCart('takeout')

assert.strictEqual(
  support.checkoutRequiresAddress([takeawayCart]),
  false,
  'takeaway checkout must not require a delivery address'
)
assert.strictEqual(
  support.checkoutRequiresAddress([takeoutCart]),
  true,
  'takeout checkout must continue requiring a delivery address'
)
assert.notStrictEqual(
  support.buildPricingKey(null, [takeawayCart]),
  '',
  'takeaway checkout must still call cart preview without an address so payment_assessment is available'
)
assert.strictEqual(
  support.buildPricingKey(null, [takeoutCart]),
  '',
  'takeout checkout must keep waiting for address before pricing'
)

const pricingPatch = support.buildPricingSuccessPatch([
  {
    cart: takeawayCart,
    result: {
      subtotal: 1200,
      total_amount: 1200,
      delivery_fee: 0,
      delivery_fee_discount: 0,
      prepare_minutes: 12,
      payment_assessment: {
        is_balance_payable: true,
        usable_balance: 1200,
        principal_part: 1200,
        bonus_part: 0,
        payment_hint: ''
      }
    }
  }
])
assert.strictEqual(
  pricingPatch.carts[0].deliveryFeeDisplay,
  '无需代取费',
  'takeaway pricing must render as self-pickup without delivery fee'
)
assert.strictEqual(
  pricingPatch.carts[0].paymentAssessment.is_balance_payable,
  true,
  'takeaway pricing must preserve backend payment_assessment for balance selector'
)
assert.strictEqual(
  pricingPatch.summaryDeliveryDisplay,
  '无需代取费',
  'takeaway summary must not show delivery-fee copy'
)

const takeawayBalanceRequest = support.buildTakeoutCreateOrderRequest({
  cart: pricingPatch.carts[0],
  note: '少辣',
  useBalance: true
})
assert.strictEqual(takeawayBalanceRequest.order_type, 'takeaway')
assert.strictEqual(takeawayBalanceRequest.use_balance, true)
assert.strictEqual(
  Object.prototype.hasOwnProperty.call(takeawayBalanceRequest, 'address_id'),
  false,
  'takeaway balance order request must not send address_id'
)
assert.strictEqual(
  Object.prototype.hasOwnProperty.call(takeawayBalanceRequest, 'delivery_fee'),
  false,
  'takeaway balance order request must not send delivery fee fields'
)

const takeoutWechatRequest = support.buildTakeoutCreateOrderRequest({
  cart: takeoutCart,
  addressId: 88,
  note: '',
  useBalance: false
})
assert.strictEqual(takeoutWechatRequest.order_type, 'takeout')
assert.strictEqual(takeoutWechatRequest.address_id, 88)
assert.strictEqual(
  Object.prototype.hasOwnProperty.call(takeoutWechatRequest, 'use_balance'),
  false,
  'takeout checkout must not submit balance payment from the takeout confirmation path'
)

const paymentMethods = support.buildCheckoutPaymentMethods(
  pricingPatch.carts[0],
  { 11: 2000 }
)
assert(
  paymentMethods.some((item) => item.id === 'balance' && item.disabled === false),
  'takeaway checkout must expose enabled balance payment when backend assessment and local balance allow it'
)
assert(
  support.buildCheckoutPaymentMethods({ ...pricingPatch.carts[0], paymentAssessment: null }, { 11: 2000 })
    .some((item) => item.id === 'balance' && item.disabled === true),
  'takeaway checkout must keep balance disabled until backend payment_assessment explicitly allows it'
)
assert(
  support.buildCheckoutPaymentMethods(pricingPatch.carts[0], { 11: 600 })
    .some((item) => item.id === 'balance' && item.disabled === true),
  'takeaway checkout must disable balance payment when local member balance is insufficient'
)
assert.strictEqual(
  support.resolveSelectedPaymentMethod(pricingPatch.carts[0], { 11: 2000 }, 'balance'),
  'balance',
  'takeaway checkout must preserve selected balance when it remains payable'
)
assert.strictEqual(
  support.resolveSelectedPaymentMethod({ ...pricingPatch.carts[0], paymentAssessment: null }, { 11: 2000 }, 'balance'),
  'wechat_pay',
  'takeaway checkout must fall back to WeChat until backend payment_assessment explicitly allows balance'
)

const restaurantSource = read('miniprogram/pages/takeout/restaurant-detail/index.ts')
const restaurantWxml = read('miniprogram/pages/takeout/restaurant-detail/index.wxml')
const cartSource = read('miniprogram/pages/takeout/cart/index.ts')
const dishDetailSource = read('miniprogram/pages/takeout/dish-detail/index.ts')
const comboDetailSource = read('miniprogram/pages/takeout/combo-detail/index.ts')
const orderConfirmSource = read('miniprogram/pages/takeout/order-confirm/index.ts')
const orderConfirmWxml = read('miniprogram/pages/takeout/order-confirm/index.wxml')
const paymentMethodsWxml = read('miniprogram/pages/takeout/order-confirm/_components/payment-methods/index.wxml')
const navigationSource = read('miniprogram/utils/navigation.ts')
const takeoutHomeSource = read('miniprogram/pages/takeout/index.ts')
const orderDetailSource = read('miniprogram/pages/orders/detail/index.ts')
const orderListSource = read('miniprogram/pages/orders/list/index.ts')
const cartServiceSource = read('miniprogram/services/cart.ts')
const restaurantJson = JSON.parse(read('miniprogram/pages/takeout/restaurant-detail/index.json'))
const orderConfirmJson = JSON.parse(read('miniprogram/pages/takeout/order-confirm/index.json'))

Object.values(restaurantJson.usingComponents || {})
  .filter((componentPath) => typeof componentPath === 'string' && componentPath.startsWith('tdesign-miniprogram/'))
  .forEach(assertTDesignComponentExists)
Object.values(orderConfirmJson.usingComponents || {})
  .filter((componentPath) => typeof componentPath === 'string' && componentPath.startsWith('tdesign-miniprogram/'))
  .forEach(assertTDesignComponentExists)

assert(
  restaurantSource.includes("orderType: 'takeout' as 'takeout' | 'takeaway'") &&
    restaurantSource.includes('onOrderTypeChange') &&
    restaurantSource.includes('orderType: this.data.orderType') &&
    restaurantWxml.includes('<t-radio value="takeaway"') &&
    !restaurantWxml.includes('t-radio-button'),
  'restaurant detail must let customers choose takeaway and pass that order_type into cart writes'
)
assert(
  navigationSource.includes("toCart(options?: { orderType?: 'takeout' | 'takeaway' })"),
  'Navigation.toCart must carry the selected order_type into the cart page'
)
assert(
  takeoutHomeSource.includes("orderType: 'takeout'") &&
    cartServiceSource.includes("if (item.orderType !== 'dine_in')") &&
    cartServiceSource.includes('this.currentTableId = null') &&
    cartServiceSource.includes("if (item.orderType !== 'reservation')") &&
    cartServiceSource.includes('this.currentReservationId = null'),
  'takeout home direct add-to-cart must not inherit a stale takeaway CartService order type'
)
assert(
  orderDetailSource.includes("Navigation.toCart({ orderType: orderType === 'takeaway' ? 'takeaway' : 'takeout' })") &&
    orderListSource.includes("Navigation.toCart({ orderType: orderType === 'takeaway' ? 'takeaway' : 'takeout' })"),
  'reorder flows must navigate to the cart matching the recreated takeout/takeaway order type'
)
assert(
  cartSource.includes("onLoad(options: { order_type?: 'takeout' | 'takeaway' })") &&
    cartSource.includes('CartAPI.getUserCarts(this.data.orderType') &&
    cartSource.includes('order_type=${this.data.orderType}'),
  'cart page must load and forward the selected takeaway order_type'
)
assert(
  dishDetailSource.includes("orderType: 'takeout' as 'takeout' | 'takeaway'") &&
    dishDetailSource.includes('orderType: this.data.orderType'),
  'dish detail must preserve takeaway order_type for add-to-cart and buy-now'
)
assert(
  comboDetailSource.includes("orderType: 'takeout' as 'takeout' | 'takeaway'") &&
    comboDetailSource.includes('orderType: this.data.orderType'),
  'combo detail must preserve takeaway order_type for add-to-cart and buy-now'
)
assert(
  orderConfirmSource.includes('selectedPaymentMethod') &&
    orderConfirmSource.includes('updatePaymentState') &&
    orderConfirmSource.includes('isPaidOrderStatus(order.status)') &&
    orderConfirmWxml.includes('requiresAddress') &&
    orderConfirmWxml.includes('payment-methods') &&
    paymentMethodsWxml.includes('t-radio-group') &&
    orderConfirmWxml.includes('(requiresAddress && !(address && address.id))'),
  'order confirm must support takeaway no-address checkout, balance selection, and paid balance-order terminal handling'
)

console.log('check-takeaway-balance-checkout: takeaway balance checkout is user-reachable and preview/direct aligned')
