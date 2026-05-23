const fs = require('fs')
const path = require('path')

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
const detailWxml = read('miniprogram/pages/merchant/orders/detail/index.wxml')
const listWxml = read('miniprogram/pages/merchant/orders/list/index.wxml')
const merchantOrderSources = [
  orderApi,
  detailView,
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
assertContains(detailWxml, 'fee_breakdown_view.settlement_rows', 'Merchant order detail must render fee breakdown settlement rows')
for (const label of ['用户实付', '平台服务费', '支付通道费', '商户实收']) {
  assertContains(detailView, label, `Merchant order detail view model must include ${label}`)
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

console.log('Merchant order fee breakdown contract check passed')
