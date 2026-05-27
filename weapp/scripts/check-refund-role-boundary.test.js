const fs = require('fs')
const path = require('path')

const repoRoot = path.resolve(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), 'utf8')
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}

const customerRefundDetail = read('miniprogram/pages/user_center/refund-detail/index.ts')
const customerRefundDetailWxml = read('miniprogram/pages/user_center/refund-detail/index.wxml')
const merchantOrderDetail = read('miniprogram/pages/merchant/orders/detail/index.ts')
const paymentApi = read('miniprogram/api/payment.ts')
const paymentRefundCompatApi = read('miniprogram/api/payment-refund.ts')

assert(
  !/\bgetRefundReturns\b|\bProfitSharingReturn\b|profitSharingReturns|profit-sharing-return-view/.test(customerRefundDetail),
  'customer refund detail must not call or model merchant profit-sharing returns'
)

assert(
  !/分账回退|profitSharingReturns|refund-return/.test(customerRefundDetailWxml),
  'customer refund detail must not render merchant profit-sharing return sections'
)

assert(
  /\bgetMerchantRefundReturns\b/.test(paymentApi),
  'payment API must expose an explicitly merchant-scoped refund return reader'
)

assert(
  /\bgetMerchantRefundReturns\b/.test(paymentRefundCompatApi),
  'payment-refund compatibility API must re-export the merchant-scoped refund return reader'
)

assert(
  /\bgetMerchantRefundReturns\b/.test(merchantOrderDetail) && !/\bgetRefundReturns\b/.test(merchantOrderDetail),
  'merchant order detail must use the merchant-scoped refund return reader'
)

assert(
  !/user_center\/refund-detail/.test(merchantOrderDetail),
  'merchant refund flow must not navigate into the customer refund detail page'
)

assert(
  /分账回退同步失败|return_load_failed|returns_error/.test(merchantOrderDetail),
  'merchant order detail must surface profit-sharing return sync failures instead of silently treating them as empty'
)

console.log('check-refund-role-boundary: validated refund role boundaries')
