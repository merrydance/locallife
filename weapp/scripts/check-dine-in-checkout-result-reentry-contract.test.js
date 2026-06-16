const assert = require('assert')
const fs = require('fs')
const path = require('path')

const weappRoot = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(weappRoot, relativePath), 'utf8')
}

function assertIncludes(content, needle, message) {
  assert(content.includes(needle), message)
}

function main() {
  const pkg = JSON.parse(read('package.json'))
  const checkoutPage = read('miniprogram/pages/dine-in/checkout/checkout.ts')
  const paymentResultPage = read('miniprogram/pages/payment/result/index.ts')
  const dineInSessionService = read('miniprogram/pages/payment/_main_shared/services/dine-in-session.ts')
  const dineInCheckoutService = read('miniprogram/pages/dine-in/_main_shared/services/dine-in-session.ts')
  const navigation = read('miniprogram/utils/navigation.ts')

  assert(
    pkg.scripts['check:dine-in-checkout-result-reentry-contract'],
    'package.json must expose the dine-in checkout result re-entry contract check'
  )
  assert(
    pkg.scripts['quality:check'].includes('check:dine-in-checkout-result-reentry-contract'),
    'quality:check must run the dine-in checkout result re-entry contract check'
  )

  assertIncludes(
    checkoutPage,
    'savePendingDineInCheckoutContext({',
    'dine-in checkout must save pending checkout context before routing to payment result'
  )
  assert(
    /savePendingDineInCheckoutContext\(\{\s*session_id:\s*this\.data\.sessionId,\s*order_id:\s*orderId,\s*payment_order_id:\s*result\.paymentOrderId\s*\|\|\s*payment\.id\s*\}/.test(checkoutPage),
    'dine-in checkout pending context must persist session_id, order_id, and payment_order_id'
  )
  assert(
    /Navigation\.toPaymentResult\(\{[\s\S]*status:\s*result\.status,[\s\S]*paymentOrderId:\s*result\.paymentOrderId\s*\|\|\s*payment\.id,[\s\S]*businessId:\s*orderId/.test(checkoutPage),
    'dine-in checkout must route to payment result with paymentOrderId and businessId for reload recovery'
  )
  assert(
    /status=\$\{encodeURIComponent\(params\.status\)\}/.test(navigation) &&
      /paymentOrderId=\$\{encodeURIComponent\(String\(params\.paymentOrderId\)\)\}/.test(navigation) &&
      /businessId=\$\{encodeURIComponent\(String\(params\.businessId\)\)\}/.test(navigation),
    'payment result navigation must preserve status, paymentOrderId, and businessId in the URL'
  )

  assert.strictEqual(
    dineInSessionService,
    dineInCheckoutService,
    'payment result and dine-in checkout must share the same pending checkout storage contract'
  )
  assertIncludes(
    dineInSessionService,
    "const CHECKOUT_STORAGE_KEY = 'dineInPendingCheckoutContext'",
    'pending dine-in checkout context must use a stable storage key'
  )
  assert(
    /if \(!context \|\| !context\.session_id \|\| !context\.order_id\)/.test(dineInSessionService),
    'pending dine-in checkout context must reject storage entries without session_id and order_id'
  )
  assert(
    /params\.paymentOrderId && context\.payment_order_id && context\.payment_order_id !== params\.paymentOrderId/.test(dineInSessionService),
    'checkout recovery must not close a session for a mismatched payment_order_id'
  )
  assert(
    /await checkoutDiningSession\(context\.session_id\)/.test(dineInSessionService),
    'checkout recovery must close the saved session_id instead of trusting route-only context'
  )
  assert(
    /clearPendingDineInCheckoutContext\(\)[\s\S]*clearDineInSessionContext\(\)/.test(dineInSessionService),
    'successful checkout recovery must clear both pending checkout and active dine-in session context'
  )

  assert(
    /if \(status === 'pending_confirmation'\) \{[\s\S]*this\.applyPendingConfirmationState\(paymentOrderId\)[\s\S]*this\.startPaymentStatusPolling\(\)/.test(paymentResultPage),
    'payment result reload must resume polling when URL status is pending_confirmation'
  )
  assert(
    /if \(isPaymentWorkflowPaid\(result\.status\)\) \{[\s\S]*this\.stopPaymentStatusPolling\(\)[\s\S]*void this\.closeDineInCheckoutSessionIfNeeded\(\)/.test(paymentResultPage),
    'payment result polling must trigger dine-in checkout close after backend paid status'
  )
  assert(
    /else if \(isPaymentWorkflowPaid\(status\)\) \{[\s\S]*void this\.closeDineInCheckoutSessionIfNeeded\(\)/.test(paymentResultPage),
    'payment result reload with paid status must trigger dine-in checkout close'
  )
  assert(
    /checkoutPaidDineInSession\(\{[\s\S]*orderId:\s*Number\(this\.data\.businessId\) \|\| undefined,[\s\S]*paymentOrderId:\s*this\.data\.paymentOrderId \|\| undefined/.test(paymentResultPage),
    'payment result close path must pass orderId and paymentOrderId guards into pending checkout recovery'
  )

  console.log('check-dine-in-checkout-result-reentry-contract: pending checkout context survives payment-result reload and paid polling contracts')
}

main()
