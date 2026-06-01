const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const ROOT = path.join(__dirname, '..')
const sourcePath = path.join(ROOT, 'miniprogram', 'pages', 'orders', '_main_shared', 'utils', 'order-fee-breakdown-view.ts')

function read(relativePath) {
  return fs.readFileSync(path.join(ROOT, relativePath), 'utf8')
}

function loadModule() {
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
      throw new Error(`unexpected require: ${modulePath}`)
    },
    Math,
    Number,
    String
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const {
  buildOrderFeeSettlementGroups,
  buildCustomerOrderFeeBreakdownView
} = loadModule()

const breakdown = {
  food_amount: 10000,
  merchant_discount_amount: 300,
  voucher_discount_amount: 200,
  food_payable_amount: 9500,
  delivery_fee_amount: 800,
  delivery_fee_discount_amount: 0,
  delivery_payable_amount: 800,
  customer_payable_amount: 10300,
  platform_service_fee_amount: 475,
  payment_channel_fee_amount: 57,
  merchant_receivable_amount: 8968,
  rider_gross_amount: 800,
  rider_payment_fee_amount: 5,
  rider_net_earnings_amount: 795
}

const groups = buildOrderFeeSettlementGroups(breakdown)
assert.strictEqual(groups.length, 2)
assert.strictEqual(groups.map((group) => group.title).join('|'), '商户账单|骑手账单')
assert.strictEqual(groups[0].total_text, '¥89.68')
assert.strictEqual(groups[1].total_text, '¥7.95')
assert.ok(groups[0].rows.some((row) => row.label === '菜品合计' && row.value_text === '¥95.00'))
assert.ok(groups[0].rows.some((row) => row.label === '- 平台服务费' && row.value_text === '-¥4.75'))
assert.ok(groups[0].rows.some((row) => row.label === '- 支付通道费' && row.value_text === '-¥0.57'))
assert.ok(groups[0].rows.some((row) => row.label === '商户实收' && row.value_text === '¥89.68'))
assert.ok(groups[1].rows.some((row) => row.label === '代取费' && row.value_text === '¥8.00'))
assert.ok(groups[1].rows.some((row) => row.label === '- 支付通道费' && row.value_text === '-¥0.05'))
assert.ok(groups[1].rows.some((row) => row.label === '骑手实收' && row.value_text === '¥7.95'))
assert.ok(!groups.flatMap((group) => group.rows).some((row) => row.label === '骑手通道费'))

const customerView = buildCustomerOrderFeeBreakdownView(breakdown)
assert.strictEqual(customerView.available, true)
assert.strictEqual(customerView.settlement_groups.length, 2)

const emptyView = buildCustomerOrderFeeBreakdownView(undefined)
assert.strictEqual(emptyView.available, false)
assert.ok(Array.isArray(emptyView.settlement_groups))
assert.strictEqual(emptyView.settlement_groups.length, 0)

const customerOrderApi = read('miniprogram/api/order.ts')
const customerOrderAdapter = read('miniprogram/pages/orders/_adapters/order.ts')
const customerOrderDetailWxml = read('miniprogram/pages/orders/detail/index.wxml')
const takeoutConfirmSupport = read('miniprogram/pages/takeout/order-confirm/_utils/takeout-order-confirm-support.ts')
const takeoutConfirmWxml = read('miniprogram/pages/takeout/order-confirm/index.wxml')

assert.match(customerOrderApi, /export interface OrderFeeBreakdown\s*\{/, 'customer order API must expose fee breakdown type')
assert.match(customerOrderApi, /fee_breakdown\?\s*:\s*OrderFeeBreakdown/, 'customer order response must expose optional fee_breakdown')
assert.ok(customerOrderAdapter.includes('buildCustomerOrderFeeBreakdownView(dto.fee_breakdown)'), 'customer order detail adapter must build fee breakdown view state')
assert.ok(customerOrderDetailWxml.includes('order.feeBreakdownView') && customerOrderDetailWxml.includes('settlement_groups'), 'customer order detail must render grouped fee breakdown')
assert.ok(takeoutConfirmSupport.includes('fee_breakdown?: OrderFeeBreakdown'), 'takeout pricing result must accept optional fee_breakdown')
assert.ok(takeoutConfirmSupport.includes('buildCustomerOrderFeeBreakdownView(result.fee_breakdown)'), 'takeout checkout must render fee breakdown only when pricing returns it')
assert.ok(takeoutConfirmWxml.includes('item.feeBreakdownView') && takeoutConfirmWxml.includes('settlement_groups'), 'takeout checkout must render grouped fee breakdown when available')

console.log('check-order-fee-breakdown-view tests passed')
