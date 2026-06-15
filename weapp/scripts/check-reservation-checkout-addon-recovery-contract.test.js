const assert = require('assert')
const fs = require('fs')
const path = require('path')

const weappRoot = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(weappRoot, relativePath), 'utf8')
}

function assertIncludes(source, needles, message) {
  needles.forEach((needle) => {
    assert(source.includes(needle), `${message}: missing ${needle}`)
  })
}

function assertPackageWiring() {
  const pkg = JSON.parse(read('package.json'))
  assert(
    pkg.scripts['check:reservation-checkout-addon-recovery-contract'],
    'package.json must expose the customer reservation checkout/add-on recovery contract check'
  )
  assert(
    pkg.scripts['quality:check'].includes('check:reservation-checkout-addon-recovery-contract'),
    'quality:check must run the customer reservation checkout/add-on recovery contract check'
  )
}

function assertReservationCreatePaymentRecovery() {
  const confirmPage = read('miniprogram/pages/reservation/confirm/index.ts')
  const detailPage = read('miniprogram/pages/reservation/detail/index.ts')
  const userReservations = read('miniprogram/pages/user_center/reservations/index.ts')

  assertIncludes(confirmPage, [
    'const reservation = await createReservation(reservationData)',
    'await startPaymentOrderWorkflow({',
    "businessType: 'reservation'",
    'paymentOrderId: paymentResult.paymentOrderId',
    'businessId: reservation.id',
    "businessType: 'reservation'",
    "status: 'create_failed'",
    'businessId: reservation.id',
    "businessType: 'reservation'"
  ], 'reservation confirm must route post-create deposit payment outcomes through durable reservation truth')

  assertIncludes(detailPage, [
    'if (!this.data.reservation || this.data.paying) return',
    'await startPaymentOrderWorkflow({',
    "businessType: 'reservation'",
    'status: paymentResult.status',
    'paymentOrderId: paymentResult.paymentOrderId',
    'businessId: this.data.id',
    "businessType: 'reservation'",
    "status: 'pending_confirmation'",
    'businessId: this.data.id'
  ], 'reservation detail must safely restart reservation payment from persisted reservation state')

  assertIncludes(userReservations, [
    'if (!id || this.data.payingReservationId) return',
    'await startPaymentOrderWorkflow({',
    "businessType: 'reservation'",
    'status: paymentResult.status',
    'paymentOrderId: paymentResult.paymentOrderId',
    'businessId: id',
    "businessType: 'reservation'",
    "status: 'pending_confirmation'",
    'businessId: id'
  ], 'user-center reservations must safely restart reservation payment from persisted reservation state')
}

function assertReservationAddonPaymentRecovery() {
  const modifyPage = read('miniprogram/pages/reservation/modify/index.ts')
  const resultPage = read('miniprogram/pages/payment/result/index.ts')

  assertIncludes(modifyPage, [
    "result.outcome === 'payment_required'",
    'buildAdjustmentPaymentOrder(this.data.reservationId, result.payment)',
    "business_type: 'reservation_addon'",
    'await completePaymentWorkflow(payment,',
    'paymentOrderId: paymentResult.paymentOrderId',
    'businessId: this.data.reservationId',
    "businessType: 'reservation'",
    'amount: formatPriceNoSymbol(result.payment.amount)'
  ], 'reservation modify must preserve add-on payment order context for unknown-result recovery')

  assertIncludes(resultPage, [
    'function isReservationPaymentBusinessType',
    "businessType === 'reservation'",
    "businessType === 'reservation_addon'",
    "wx.redirectTo({ url: `/pages/reservation/detail/index?id=${this.data.businessId}` })",
    'await waitForPaymentWorkflowTerminalResult(this.data.paymentOrderId'
  ], 'payment result page must send reservation and reservation_addon outcomes back to reservation detail')
}

function assertReservationRefundRecoveryCopyAndWrappers() {
  const modifyPage = read('miniprogram/pages/reservation/modify/index.ts')
  const reservationApi = read('miniprogram/pages/reservation/modify/_main_shared/api/reservation.ts')
  const paymentApi = read('miniprogram/pages/payment/_main_shared/api/payment.ts')

  assertIncludes(modifyPage, [
    "result.outcome === 'refund_initiated'",
    "wx.showModal({",
    "title: '退款处理中'",
    "confirmText: '查看详情'",
    'formatPriceNoSymbol(result.refund_amount)',
    "wx.navigateBack()",
    'getErrorUserMessage(error'
  ], 'reservation modify must treat refund_initiated as async backend truth and avoid local instant-success terminal state')

  assertIncludes(reservationApi, [
    'export type ReservationDishAdjustmentOutcome',
    "'applied' | 'payment_required' | 'refund_initiated'",
    'payment_order_id: number',
    'pay_params?: MiniProgramPayParams',
    'refund_amount?: number',
    'static async modifyDishes',
    "url: `/v1/reservations/${id}/modify-dishes`",
    'static async addDishes',
    "url: `/v1/reservations/${id}/add-dishes`",
    'static async getReservationDetail',
    "url: `/v1/reservations/${id}`"
  ], 'reservation wrapper must keep modify/add-dishes outcomes and reservation readback contracts aligned')

  assertIncludes(paymentApi, [
    'export async function getPaymentDetail',
    'export async function queryPaymentOrder',
    'export async function getPaymentRefunds',
    "url: `/v1/payments/${paymentId}`",
    "url: `/v1/payments/${paymentId}/query`",
    "url: `/v1/payments/${paymentId}/refunds`"
  ], 'payment wrapper must keep payment and refund readback contracts available')
}

function assertContractCardReferencesThisGate() {
  const card = read('../artifacts/production-risk-audit/flows/state-sequencing-customer-reservation-checkout-addon-noshow-2026-06-15.md')
  assert(
    card.includes('npm run check:reservation-checkout-addon-recovery-contract'),
    'reservation checkout audit card must name the executable customer contract gate'
  )
}

function main() {
  assertPackageWiring()
  assertReservationCreatePaymentRecovery()
  assertReservationAddonPaymentRecovery()
  assertReservationRefundRecoveryCopyAndWrappers()
  assertContractCardReferencesThisGate()
  console.log('check-reservation-checkout-addon-recovery-contract: customer reservation payment/add-on recovery contract passed')
}

main()
