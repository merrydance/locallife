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

function main() {
  const paymentWorkflow = read('miniprogram/services/payment-workflow.ts')
  const orderConfirm = read('miniprogram/pages/takeout/order-confirm/index.ts')
  const cart = read('miniprogram/pages/takeout/cart/index.ts')
  const orderList = read('miniprogram/pages/orders/list/index.ts')
  const orderListWxml = read('miniprogram/pages/orders/list/index.wxml')
  const orderDetail = read('miniprogram/pages/orders/detail/index.ts')
  const paymentResult = read('miniprogram/pages/payment/result/index.ts')
  const refundDetail = read('miniprogram/pages/user_center/refund-detail/index.ts')
  const merchantOrderDetail = read('miniprogram/pages/merchant/orders/detail/index.ts')
  const merchantOrderDetailWxml = read('miniprogram/pages/merchant/orders/detail/index.wxml')
  const paymentDetail = read('miniprogram/pages/user_center/payment-detail/index.ts')

  const combinedRuntime = [
    paymentWorkflow,
    orderConfirm,
    orderList,
    orderListWxml,
    orderDetail
  ].join('\n')

  assert(
    !/\bcompleteCombinedPaymentWorkflow\b/.test(combinedRuntime),
    'weapp payment runtime must not call completeCombinedPaymentWorkflow'
  )
  assert(
    !/\bcreateCombinedPaymentOrder\b|\brecoverCombinedPaymentOrder\b/.test(combinedRuntime),
    'weapp payment runtime must not create or recover combined payment orders'
  )
  assert(
    !/\bonBatchPay\b|\bresumeCombinedPayment\b|合并支付|合单支付/.test(combinedRuntime),
    'weapp payment runtime must not expose combined-payment entrypoints or copy'
  )
  assert(
    /selectedMerchantIds|selectedMerchantCount|hasMultiMerchantSelection/.test(cart),
    'takeout cart must compute selected merchant count before checkout'
  )
  assert(
    /selectedMerchantIds|selectedMerchantCount|hasMultiMerchantSelection/.test(orderConfirm),
    'takeout order confirm must keep a multi-merchant guard as a second line of defense'
  )
  assert(
    /return buildPaymentWorkflowResultFromPayment\(payment,\s*mapPaymentStatusToWorkflowStatus\(finalStatus\)\)/.test(paymentWorkflow),
    'payment workflow must build terminal result from the refreshed payment detail after polling'
  )
  assert(
    !/while\s*\([^)]*terminalWaitToken/.test(paymentResult),
    'payment result page must not own an unbounded terminal wait loop'
  )
  assert(
    /maxAttempts|maxElapsedMs|backoff/.test(refundDetail),
    'refund detail must use a bounded backoff wait instead of fixed infinite polling'
  )
  assert(
    /waitForRefundTerminalResult/.test(merchantOrderDetail),
    'merchant refund submission must wait for backend refund terminal result'
  )
  assert(
    !/isRefundStatusTerminal\(refund\.status\)/.test(paymentDetail),
    'payment detail must not hide non-terminal refunds'
  )
  assert(
    !`${merchantOrderDetail}\n${merchantOrderDetailWxml}`.includes('微信侧'),
    'merchant refund copy must not mention provider-side wording'
  )

  console.log('check-payment-refund-terminal-flow: validated payment/refund terminal-flow boundaries')
}

main()
