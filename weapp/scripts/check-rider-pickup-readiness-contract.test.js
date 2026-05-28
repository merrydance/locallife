const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

function loadModule(relativePath, stubs = {}) {
  const sourcePath = path.join(__dirname, '..', 'miniprogram', ...relativePath.split('/'))
  const source = fs.readFileSync(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018
    }
  }).outputText

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (stubs[modulePath]) {
        return stubs[modulePath]
      }
      if (modulePath === '../api/delivery') {
        return {}
      }
      throw new Error(`unexpected require: ${modulePath}`)
    },
    Date,
    Math,
    Number,
    String,
    RegExp,
    Set
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const { getRiderDeliveryActionState, buildRiderDeliveryActionFailureFeedback } = loadModule('utils/rider-delivery-view.ts')

const waitingState = getRiderDeliveryActionState({
  status: 'picking',
  order_status: 'courier_accepted',
  fulfillment_status: 'preparing',
  can_confirm_pickup: false,
  pickup_block_reason: '商户未出餐，暂不可确认取餐',
  pickup_action_label: '等待商户出餐'
})

assert.strictEqual(waitingState.canUpdate, false)
assert.strictEqual(waitingState.actionKey, '')
assert.strictEqual(waitingState.expectedStatus, null)
assert.strictEqual(waitingState.label, '等待商户出餐')
assert.strictEqual(waitingState.disabledReason, '商户未出餐，暂不可确认取餐')

const readyState = getRiderDeliveryActionState({
  status: 'picking',
  order_status: 'courier_accepted',
  fulfillment_status: 'ready',
  can_confirm_pickup: true,
  pickup_action_label: '确认取餐'
})

assert.strictEqual(readyState.canUpdate, true)
assert.strictEqual(readyState.actionKey, 'confirm_pickup')
assert.strictEqual(readyState.expectedStatus, 'picked')
assert.strictEqual(readyState.label, '确认取餐')
assert.strictEqual(readyState.disabledReason, '')

const codeBlockedState = getRiderDeliveryActionState({
  status: 'picking',
  order_status: 'courier_accepted',
  fulfillment_status: 'preparing',
  pickup_block_reason: '商户未出餐，暂不可确认取餐'
})

assert.strictEqual(codeBlockedState.canUpdate, false)
assert.strictEqual(codeBlockedState.label, '等待商户出餐')
assert.strictEqual(codeBlockedState.disabledReason, '商户未出餐，暂不可确认取餐')

const missingBackendReadinessState = getRiderDeliveryActionState({
  status: 'picking',
  order_status: 'courier_accepted',
  fulfillment_status: 'ready'
})

assert.strictEqual(missingBackendReadinessState.canUpdate, false)
assert.strictEqual(missingBackendReadinessState.actionKey, '')
assert.strictEqual(missingBackendReadinessState.expectedStatus, null)
assert.strictEqual(missingBackendReadinessState.disabledReason, '当前任务状态暂不可用，请刷新后重试')

const feedback = buildRiderDeliveryActionFailureFeedback({
  userMessage: '商户未出餐，暂不可确认取餐',
  data: {
    code: 40973,
    reason: 'merchant_not_ready',
    action_label: '等待商户出餐',
    error: '商户未出餐，暂不可确认取餐'
  }
}, 'confirm_pickup', '操作失败')

assert.strictEqual(feedback.mode, 'modal')
assert.strictEqual(feedback.title, '等待商户出餐')
assert.strictEqual(feedback.content, '商户未出餐，暂不可确认取餐')

const wrappedFeedback = buildRiderDeliveryActionFailureFeedback({
  userMessage: '商户未出餐，暂不可确认取餐',
  data: {
    code: 40973,
    message: '商户未出餐，暂不可确认取餐',
    data: {
      code: 40973,
      reason: 'merchant_not_ready',
      action_label: '等待商户出餐',
      error: '商户未出餐，暂不可确认取餐'
    }
  },
  originalError: {
    data: {
      code: 40973,
      reason: 'merchant_not_ready',
      action_label: '等待商户出餐',
      error: '商户未出餐，暂不可确认取餐'
    }
  }
}, 'confirm_pickup', '操作失败')

assert.strictEqual(wrappedFeedback.mode, 'modal')
assert.strictEqual(wrappedFeedback.title, '等待商户出餐')
assert.strictEqual(wrappedFeedback.content, '商户未出餐，暂不可确认取餐')

const { buildRiderWorkbenchDashboardView } = loadModule('services/rider-workbench.ts', {
  '../api/delivery': {
    getDeliveryStatusDisplay: (status) => ({ text: status })
  },
  '../utils/rider-delivery-income-view': {
    buildRiderDeliveryIncomeView: () => ({ amountText: '0.00' })
  },
  '../utils/util': {
    formatPriceNoSymbol: (value) => (Number(value || 0) / 100).toFixed(2)
  },
  '../utils/status-tag': {
    resolveStatusTagTheme: (theme) => theme
  },
  '../utils/rider-delivery-view': {
    getRiderDeliveryActionState
  },
  './rider-deposit-finance': {
    buildRiderDepositWorkbenchSummaryView: () => ({})
  },
  '../utils/rider-claims-view': {
    getRiderClaimActionHint: () => ''
  }
})

const workbenchView = buildRiderWorkbenchDashboardView({
  rider_status: {
    status: 'active',
    is_online: true,
    online_status: 'delivering',
    active_deliveries: 1,
    current_region_id: 1,
    required_deposit: 0,
    can_go_online: true,
    can_go_offline: false
  },
  current_deliveries: {
    active_count: 1,
    items: [{
      id: 1001,
      order_id: 2001,
      status: 'picking',
      order_status: 'courier_accepted',
      fulfillment_status: 'preparing',
      can_confirm_pickup: false,
      pickup_block_reason: '商户未出餐，暂不可确认取餐',
      pickup_action_label: '等待商户出餐',
      delivery_fee: 800,
      rider_earnings: 720,
      pickup_address: '门店',
      delivery_address: '顾客',
      created_at: '2026-05-28T00:00:00Z'
    }]
  },
  order_pool: { available_count: 0 },
  today: { date: '2026-05-28', completed_deliveries: 0 },
  income: { total_rider_income: 0 },
  deposit: { available_deposit: 0 },
  claims: { pending_action_count: 0 },
  notifications: { unread_count: 0 },
  sections: []
})

assert.strictEqual(workbenchView.currentDelivery.can_confirm_pickup, false)
assert.strictEqual(workbenchView.currentDelivery.pickup_block_reason, '商户未出餐，暂不可确认取餐')
assert.strictEqual(workbenchView.currentDelivery.pickup_action_label, '等待商户出餐')

console.log('check-rider-pickup-readiness-contract tests passed')
