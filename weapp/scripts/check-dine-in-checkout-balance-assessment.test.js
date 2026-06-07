const assert = require('assert')
const fs = require('fs')
const path = require('path')
const vm = require('vm')
const ts = require('typescript')

const weappRoot = path.join(__dirname, '..')
const helperPath = path.join(weappRoot, 'miniprogram/pages/dine-in/_utils/dine-in-checkout-view.ts')

function loadCheckoutView() {
  const source = fs.readFileSync(helperPath, 'utf8')
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
      if (request === '../../../utils/image') {
        return { getPublicImageUrl: (value) => value || '' }
      }
      if (request === '../../../utils/util') {
        return { formatPriceNoSymbol: (fen) => (Number(fen || 0) / 100).toFixed(2) }
      }
      if (request === '../_services/dine-in-checkout') {
        return { loadDineInCheckoutSession: () => ({}) }
      }
      return require(request)
    }
  }
  sandbox.exports = sandbox.module.exports

  vm.runInNewContext(compiled, sandbox, { filename: helperPath })
  return sandbox.module.exports
}

function createBaseState(overrides = {}) {
  const { buildCheckoutRenderState } = loadCheckoutView()
  return buildCheckoutRenderState({
    merchantInfo: { id: 1, name: '测试商户', logo_url: '' },
    cart: { items: [], total_count: 0 },
    calculation: {
      subtotal: 1000,
      discount_amount: 0,
      total_amount: 1000,
      applied_promotions: [],
      ladder_promotions: [],
      voucher_trials: [],
      payment_assessment: {
        is_balance_payable: true,
        usable_balance: 1000,
        principal_part: 1000,
        bonus_part: 0,
        payment_hint: ''
      },
      ...overrides.calculation
    },
    memberBalance: 2000,
    memberBalanceDisplay: '20.00',
    selectedPaymentMethod: 'balance',
    ...overrides.params
  })
}

function getBalanceMethod(state) {
  return state.paymentMethods.find((item) => item.id === 'balance')
}

const disabledByBackend = createBaseState({
  calculation: {
    payment_assessment: {
      is_balance_payable: false,
      usable_balance: 1000,
      principal_part: 1000,
      bonus_part: 0,
      payment_hint: '会员余额不可与优惠券叠加'
    }
  }
})
assert.strictEqual(
  getBalanceMethod(disabledByBackend).disabled,
  true,
  'dine-in checkout must disable balance payment when backend payment_assessment says balance is not payable'
)
assert.strictEqual(
  disabledByBackend.selectedPaymentMethod,
  'wechat_pay',
  'dine-in checkout must fall back to WeChat payment when selected balance becomes backend-disabled'
)

const allowedByBackend = createBaseState()
assert.strictEqual(
  getBalanceMethod(allowedByBackend).disabled,
  false,
  'dine-in checkout must keep balance payment selectable when backend assessment and local balance both allow it'
)
assert.strictEqual(
  allowedByBackend.selectedPaymentMethod,
  'balance',
  'dine-in checkout must preserve selected balance when it remains payable'
)

const unavailableLocalBalance = createBaseState({
  params: {
    memberBalance: 0,
    memberBalanceDisplay: '0.00'
  },
  calculation: {
    payment_assessment: undefined
  }
})
assert.strictEqual(
  getBalanceMethod(unavailableLocalBalance).disabled,
  true,
  'dine-in checkout must disable balance payment when local member balance is unavailable'
)
assert.strictEqual(
  unavailableLocalBalance.selectedPaymentMethod,
  'wechat_pay',
  'dine-in checkout must preserve the existing fallback from insufficient local balance to WeChat payment'
)

console.log('check-dine-in-checkout-balance-assessment: checkout balance payment follows backend assessment')
