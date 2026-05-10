import {
  calculateCart,
  getCart,
  type CalculateCartResponse,
  type CartResponse
} from '../api/cart'
import { getDiningSessionMenu } from '../api/dining-session'
import { getPublicMerchantDetail, type PublicMerchantDetail } from '../api/merchant'
import { createOrderFromCart } from '../api/order'
import { createOrderPayment } from '../api/payment'
import { getMyMemberships } from '../api/personal'
import type { ScanTableTableInfo } from '../api/table'
import { completePaymentWorkflow } from './payment-workflow'

export type CheckoutCartResponse = CartResponse
export type CheckoutCalculationResponse = CalculateCartResponse
export type CheckoutMerchantDetail = PublicMerchantDetail
export type CheckoutTableDetail = ScanTableTableInfo

export function loadDineInCheckoutSession(sessionId: number) {
  return getDiningSessionMenu(sessionId)
}

export function loadCheckoutMerchantDetail(merchantId: number) {
  return getPublicMerchantDetail(merchantId)
}

export function loadCheckoutCart(params: Parameters<typeof getCart>[0]) {
  return getCart(params)
}

export function calculateCheckoutCart(params: Parameters<typeof calculateCart>[0]) {
  return calculateCart(params)
}

export function loadCheckoutMemberships() {
  return getMyMemberships()
}

export function createCheckoutOrderFromCart(
  merchantId: number,
  orderType: Parameters<typeof createOrderFromCart>[1],
  payload: Parameters<typeof createOrderFromCart>[2]
) {
  return createOrderFromCart(merchantId, orderType, payload)
}

export function createCheckoutOrderPayment(orderId: number) {
  return createOrderPayment(orderId)
}

export function completeCheckoutPayment(
  payment: Parameters<typeof completePaymentWorkflow>[0],
  options: Parameters<typeof completePaymentWorkflow>[1] = {}
) {
  return completePaymentWorkflow(payment, options)
}
