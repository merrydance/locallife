const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const ROOT = path.resolve(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(ROOT, relativePath), 'utf8')
}

function assertContains(content, pattern, message) {
  const found = pattern instanceof RegExp ? pattern.test(content) : content.includes(pattern)
  if (!found) {
    throw new Error(message)
  }
}

function assertNotContains(content, pattern, message) {
  const found = pattern instanceof RegExp ? pattern.test(content) : content.includes(pattern)
  if (found) {
    throw new Error(message)
  }
}

const orderApi = read('miniprogram/api/order-management.ts')
const detailView = read('miniprogram/utils/merchant-order-detail-view.ts')
const sharedFeeBreakdownView = read('miniprogram/utils/order-fee-breakdown-view.ts')
const detailWxml = read('miniprogram/pages/merchant/orders/detail/index.wxml')
const listWxml = read('miniprogram/pages/merchant/orders/list/index.wxml')
const merchantOrderSources = [
  orderApi,
  detailView,
  sharedFeeBreakdownView,
  detailWxml,
  listWxml,
  read('miniprogram/pages/merchant/orders/detail/index.ts'),
  read('miniprogram/pages/merchant/orders/list/index.ts')
].join('\n')

assertContains(orderApi, /export interface MerchantOrderFeeBreakdown\s*\{/, 'Order API must define MerchantOrderFeeBreakdown')
assertContains(orderApi, /fee_breakdown\?\s*:\s*MerchantOrderFeeBreakdown/, 'OrderResponse must expose optional fee_breakdown')

for (const field of [
  'food_amount',
  'merchant_discount_amount',
  'voucher_discount_amount',
  'food_payable_amount',
  'delivery_fee_amount',
  'delivery_fee_discount_amount',
  'delivery_payable_amount',
  'customer_payable_amount',
  'platform_service_fee_amount',
  'payment_channel_fee_amount',
  'merchant_receivable_amount'
]) {
  assertContains(orderApi, field, `MerchantOrderFeeBreakdown must include ${field}`)
}

assertContains(detailView, /buildMerchantOrderFeeBreakdownView/, 'Detail view helper must build fee breakdown view state')
assertContains(detailView, /MerchantOrderFeeBreakdownView/, 'Detail view helper must expose MerchantOrderFeeBreakdownView')

assertContains(detailWxml, 'fee_breakdown_view.summary_rows', 'Merchant order detail must render fee breakdown summary rows')
assertContains(detailWxml, 'fee_breakdown_view.settlement_groups', 'Merchant order detail must render fee breakdown settlement groups')
for (const label of ['用户实付', '商户部分', '骑手部分', '商户实收', '骑手收入']) {
  assertContains(`${detailView}\n${sharedFeeBreakdownView}`, label, `Merchant order detail view model must include ${label}`)
}
assertContains(listWxml, '实收', 'Merchant order list must render merchant receivable summary')

assertNotContains(
  orderApi,
  /subtotal\s*\+\s*order\.delivery_fee\s*-\s*order\.delivery_fee_discount\s*-\s*order\.discount_amount/,
  'Order API must not calculate customer payable from legacy fields'
)

for (const forbidden of [
  'provider_payment_fee',
  'provider_payment_fee_rate_bps',
  'platform_commission',
  'operator_commission'
]) {
  assertNotContains(merchantOrderSources, forbidden, `Merchant order weapp code must not expose internal field ${forbidden}`)
}

function loadDetailViewModule() {
  const moduleCache = {}

  function loadTsModule(relativePath) {
    const sourcePath = path.join(ROOT, relativePath)
    if (moduleCache[sourcePath]) return moduleCache[sourcePath].exports

    const module = { exports: {} }
    moduleCache[sourcePath] = module
    const source = fs.readFileSync(sourcePath, 'utf8')
    const compiled = ts.transpileModule(source, {
      compilerOptions: {
        module: ts.ModuleKind.CommonJS,
        target: ts.ScriptTarget.ES2018,
        esModuleInterop: true
      }
    }).outputText

    const sandbox = {
      exports: module.exports,
      module,
      require(modulePath) {
        if (modulePath === 'dayjs') return require('dayjs')
        if (modulePath === './order-fee-breakdown-view') {
          return loadTsModule(path.join('miniprogram', 'utils', 'order-fee-breakdown-view.ts'))
        }
        throw new Error(`unexpected require: ${modulePath}`)
      },
      Date,
      Math,
      Number,
      String
    }
    vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
    return module.exports
  }

  return loadTsModule(path.join('miniprogram', 'utils', 'merchant-order-detail-view.ts'))
}

const { buildMerchantOrderFeeBreakdownView } = loadDetailViewModule()
const feeBreakdownView = buildMerchantOrderFeeBreakdownView({
  total_amount: 10300,
  fee_breakdown: {
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
})

if (!Array.isArray(feeBreakdownView.settlement_groups)) {
  throw new Error('Merchant order detail fee breakdown must expose grouped settlement view state')
}

const settlementGroupLabels = feeBreakdownView.settlement_groups.map((group) => group.title)
if (settlementGroupLabels.join('|') !== '商户部分|骑手部分') {
  throw new Error(`Merchant order detail fee breakdown must group settlements by participant, got ${settlementGroupLabels.join('|')}`)
}

const merchantGroup = feeBreakdownView.settlement_groups[0]
const riderGroup = feeBreakdownView.settlement_groups[1]
if (merchantGroup.total_text !== '¥89.68' || riderGroup.total_text !== '¥7.95') {
  throw new Error('Merchant order detail fee breakdown must summarize merchant and rider totals separately')
}
if (!merchantGroup.rows.some((row) => row.label === '平台服务费') || !riderGroup.rows.some((row) => row.label === '骑手通道费')) {
  throw new Error('Merchant order detail fee breakdown must keep fee rows inside their participant groups')
}

console.log('Merchant order fee breakdown contract check passed')
