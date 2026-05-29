const fs = require('fs')
const path = require('path')

const ROOT = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(ROOT, relativePath), 'utf8')
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}

function assertIncludes(source, pattern, message) {
  assert(source.includes(pattern), message)
}

function main() {
  const paymentWorkflow = read('miniprogram/services/payment-workflow.ts')
  const paymentResult = read('miniprogram/pages/payment/result/index.ts')
  const paymentResultView = read('miniprogram/utils/payment-result-view.ts')
  const navigation = read('miniprogram/utils/navigation.ts')

  assertIncludes(paymentWorkflow, 'PaymentCancelledError', 'Payment workflow must distinguish user cancellation')
  assertIncludes(paymentWorkflow, "return buildPaymentWorkflowResultFromPayment(payment, 'cancelled')", 'Payment workflow must return cancelled instead of final success on user cancel')
  assertIncludes(paymentWorkflow, "return buildPaymentWorkflowResultFromPayment(payment, 'pending_confirmation')", 'Payment workflow must use pending_confirmation for unknown or delayed terminal truth')
  assertIncludes(paymentWorkflow, 'pollPaymentStatus(', 'Payment workflow must poll backend status after requestPayment')
  assertIncludes(paymentWorkflow, 'queryPaymentWorkflowTerminalResult', 'Payment workflow must refresh backend payment detail after polling terminal status')
  assertIncludes(paymentWorkflow, "status: 'create_failed'", 'Payment workflow must distinguish create_failed from paid business success')
  assertIncludes(paymentWorkflow, "'pay_params_missing'", 'Payment workflow must distinguish missing pay params from paid business success')

  assertIncludes(paymentResult, 'waitForPaymentWorkflowTerminalResult(this.data.paymentOrderId, { maxAttempts: 1, interval: 0 })', 'Payment result page must support re-entry status refresh through backend truth')
  assertIncludes(paymentResult, 'applyPendingConfirmationState(paymentOrderId)', 'Payment result page must render unknown/pending confirmation states')
  assertIncludes(paymentResult, 'closeDineInCheckoutSessionIfNeeded', 'Payment result page must recover dine-in session close after paid result or re-entry')
  assertIncludes(paymentResult, '支付结果还在同步中，请稍后刷新或返回订单详情查看。', 'Payment result page must give unknown-result recovery guidance')
  assertIncludes(paymentResultView, "case 'pending_confirmation'", 'Payment result view must model pending confirmation')
  assertIncludes(paymentResultView, "case 'cancelled'", 'Payment result view must model cancellation')
  assertIncludes(paymentResultView, "case 'create_failed'", 'Payment result view must model payment creation failure')
  assertIncludes(paymentResultView, "case 'pay_params_missing'", 'Payment result view must model missing payment params')
  assertIncludes(navigation, 'toPaymentResult', 'Payment outcomes must route through a shared payment result surface')

  const paymentEntrypoints = [
    {
      label: 'takeout order confirm',
      source: read('miniprogram/pages/takeout/order-confirm/index.ts'),
      required: ['completePaymentWorkflow(await createOrderPayment(orderId)', 'Navigation.toPaymentResult({', "businessType: paymentResult.businessType || 'order'"]
    },
    {
      label: 'order list pay',
      source: read('miniprogram/pages/orders/list/index.ts'),
      required: ['completePaymentWorkflow(await createOrderPayment(orderId)', 'Navigation.toPaymentResult({', "businessType: paymentResult.businessType || 'order'"]
    },
    {
      label: 'order detail pay',
      source: read('miniprogram/pages/orders/detail/index.ts'),
      required: ['startPaymentOrderWorkflow({', "businessType: 'order'", 'Navigation.toPaymentResult({']
    },
    {
      label: 'reservation confirm',
      source: read('miniprogram/pages/reservation/confirm/index.ts'),
      required: ['startPaymentOrderWorkflow({', "businessType: 'reservation'", 'Navigation.toPaymentResult({']
    },
    {
      label: 'reservation list pay',
      source: read('miniprogram/pages/user_center/reservations/index.ts'),
      required: ['startPaymentOrderWorkflow({', "businessType: 'reservation'", 'returnStatus: this.data.currentStatus ||']
    },
    {
      label: 'dine-in checkout',
      source: read('miniprogram/pages/dine-in/checkout/checkout.ts'),
      required: ['completeCheckoutPayment(payment', 'savePendingDineInCheckoutContext({', 'Navigation.toPaymentResult({']
    },
    {
      label: 'payment detail continue pay',
      source: read('miniprogram/pages/user_center/payment-detail/index.ts'),
      required: ['startPaymentOrderWorkflow({', 'Navigation.toPaymentResult({', "businessType: 'rider_deposit'"]
    }
  ]

  for (const entry of paymentEntrypoints) {
    for (const required of entry.required) {
      assertIncludes(entry.source, required, `${entry.label} must include ${required}`)
    }
    assert(!entry.source.includes('invokeWechatPay'), `${entry.label} must not call invokeWechatPay directly`)
  }

  const riderDepositPayment = read('miniprogram/services/rider-deposit-payment.ts')
  assertIncludes(riderDepositPayment, 'savePendingRiderDepositRecharge', 'Rider deposit flow must persist pending recharge for re-entry')
  assertIncludes(riderDepositPayment, 'continueStoredRiderDepositRecharge', 'Rider deposit flow must expose stored pending recharge recovery')
  assertIncludes(riderDepositPayment, 'getRiderDepositRechargePaymentTruth', 'Rider deposit recovery must query backend truth')
  assertIncludes(riderDepositPayment, "return buildRechargeResultFromPayment('pending_confirmation'", 'Rider deposit flow must distinguish pending confirmation')
  assertIncludes(riderDepositPayment, 'completePaymentWorkflow(payment', 'Rider deposit flow must use shared payment workflow')

  const claimRecoveryPayment = read('miniprogram/services/claim-recovery-payment.ts')
  assertIncludes(claimRecoveryPayment, 'completePaymentWorkflow(toClaimRecoveryPaymentOrder', 'Claim recovery payment must use shared payment workflow')
  assertIncludes(claimRecoveryPayment, "shouldSync: workflowResult.status !== 'cancelled'", 'Claim recovery payment must not sync as paid on cancellation')
  assertIncludes(claimRecoveryPayment, "pendingConfirmation: workflowResult.status === 'pending_confirmation'", 'Claim recovery payment must expose pending confirmation')

  const baofuOnboarding = read('miniprogram/services/baofu-account-onboarding.ts')
  assertIncludes(baofuOnboarding, "business_type: 'baofu_account_verify_fee'", 'Baofoo account verification fee must use its own payment business type')
  assertIncludes(baofuOnboarding, 'savePendingWorkflowContext(pendingWorkflowContext)', 'Baofoo onboarding must persist pending payment workflow context')
  assertIncludes(baofuOnboarding, "paymentResult.status === 'pending_confirmation'", 'Baofoo onboarding must keep pending confirmation state after delayed callback')
  assertIncludes(baofuOnboarding, "paymentResult.status === 'cancelled'", 'Baofoo onboarding must distinguish user cancellation')
  assertIncludes(baofuOnboarding, 'pollBaofuSettlementAccountStatus', 'Baofoo onboarding must poll account status after paid verification fee')

  const cancelRefundWorkflow = read('miniprogram/services/order-cancel-refund-workflow.ts')
  assertIncludes(cancelRefundWorkflow, 'cancelRefundPending: true', 'Order cancel refund flow must expose refund processing state')
  assertIncludes(cancelRefundWorkflow, 'findLatestOrderRefund', 'Order cancel refund flow must query backend refund truth after cancel')
  assertIncludes(cancelRefundWorkflow, 'getRefundStatusView', 'Order cancel refund flow must render backend refund status semantics through the shared status view')
  assertIncludes(cancelRefundWorkflow, 'REFUND_TRACK_MAX_ATTEMPTS', 'Order cancel refund flow must bound refund progress polling attempts')
  assertIncludes(cancelRefundWorkflow, 'REFUND_TRACK_POLL_INTERVAL_MS', 'Order cancel refund flow must use an explicit refund polling interval')
  assertIncludes(cancelRefundWorkflow, 'pollRefundTracking(page)', 'Order cancel refund flow must continue tracking delayed refund creation')
  assertIncludes(cancelRefundWorkflow, 'attempts >= REFUND_TRACK_MAX_ATTEMPTS', 'Order cancel refund flow must stop polling after the bounded attempt count')
  assertIncludes(cancelRefundWorkflow, '退款结果还在同步中，请稍后在退款详情页刷新查看。', 'Order cancel refund flow must guide delayed refund callback recovery')

  const refundWorkflow = read('miniprogram/services/refund-workflow.ts')
  assertIncludes(refundWorkflow, 'waitForRefundTerminalResult', 'Refund detail workflow must expose a shared terminal wait helper')
  assertIncludes(refundWorkflow, 'isRefundStatusTerminal', 'Refund terminal wait must stop only on backend terminal refund statuses')

  const refundDetail = read('miniprogram/pages/user_center/refund-detail/index.ts')
  assertIncludes(refundDetail, 'maxAttempts', 'Refund detail must bound terminal wait attempts')
  assertIncludes(refundDetail, 'backoff', 'Refund detail must use backoff for delayed terminal result')
  assertIncludes(refundDetail, 'onRetry()', 'Refund detail must expose a user-triggered retry for delayed or failed refund status')
  assertIncludes(refundDetail, 'this.loadRefundDetail()', 'Refund detail retry and re-entry must refresh from backend truth')

  const baofuWithdrawal = read('scripts/check-baofu-withdrawal-workflow.js')
  assertIncludes(baofuWithdrawal, 'Promise.allSettled', 'Withdrawal scenario evidence must require isolated balance/records failures')
  assertIncludes(baofuWithdrawal, 'balanceErrorMessage', 'Withdrawal scenario evidence must require balance error state')
  assertIncludes(baofuWithdrawal, 'recordsErrorMessage', 'Withdrawal scenario evidence must require records error state')
  assertIncludes(baofuWithdrawal, 'result.message', 'Withdrawal scenario evidence must require accepted/unknown create message')

  const wallet = read('miniprogram/pages/user_center/wallet/index.ts')
  for (const businessType of ['reservation', 'reservation_addon', 'rider_deposit', 'claim_recovery', 'baofu_account_verify_fee']) {
    assertIncludes(wallet, businessType, `Wallet ledger must label ${businessType} payment/refund records`)
  }

  console.log('check-money-async-scenario-evidence: validated payment, refund, withdrawal, delayed-callback, and re-entry evidence')
}

main()
