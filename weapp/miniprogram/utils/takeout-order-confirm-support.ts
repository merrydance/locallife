import { CartItemResponse } from '../api/cart'
import { CreateOrderRequest, OrderItemRequest, OrderType } from '../api/order'
import { getPublicImageUrl } from './image'
import { buildCustomerOrderFeeBreakdownView, type CustomerOrderFeeBreakdownView } from './order-fee-breakdown-view'
import { formatPriceNoSymbol } from './util'
import { globalStore } from './global-store'
import type { CheckoutAddress } from '../services/takeout-checkout'
import type { OrderFeeBreakdown } from '../api/order'

export interface CartItemView {
  id: number
  dishId?: number
  comboId?: number
  name: string
  imageUrl: string
  quantity: number
  unitPrice: number
  priceDisplay: string
  subtotal: number
  subtotalDisplay: string
  specText?: string
  customizations?: Record<string, unknown>
  dishImages?: string[]
}

export interface MerchantCartView {
  merchantId: number
  merchantName: string
  orderType: OrderType
  tableId?: number
  reservationId?: number
  items: CartItemView[]
  totalCount: number
  subtotal: number
  subtotalDisplay: string
  deliveryFee: number
  deliveryFeeDisplay: string
  deliveryFeeDiscount: number
  deliveryDistance: number
  deliveryEtaMinutes: number
  deliveryEtaDisplay: string
  orderTotal: number
  orderTotalDisplay: string
  originalTotalDisplay: string
  hasDiscount: boolean
  appliedPromotions: Array<{ title: string, amount: number, amountDisplay: string, type: string }>
  ladderPromotions: Array<{ name: string, thresholdDisplay: string, discountDisplay: string, currentHit: boolean, missingNeedDisplay: string }>
  voucherTrials: Array<{ voucherName: string, amountDisplay: string, trialPayableDisplay: string }>
  paymentHint: string
  feeBreakdownView: CustomerOrderFeeBreakdownView
}

interface CheckoutSnapshotItem {
  id: number
  dishId?: number
  comboId?: number
  name: string
  imageUrl: string
  quantity: number
  unitPrice: number
  subtotal: number
  specText?: string
  customizations?: Record<string, unknown>
  dishImages?: string[]
}

interface CheckoutSnapshotCart {
  cartId?: number
  merchantId: number
  merchantName: string
  orderType?: OrderType
  tableId?: number
  reservationId?: number
  items: CheckoutSnapshotItem[]
  subtotal: number
  totalCount?: number
}

export interface CheckoutSnapshotPayload {
  cartIds?: number[]
  carts?: CheckoutSnapshotCart[]
}

type PricingResult = {
  delivery_fee?: number
  delivery_fee_discount?: number
  delivery_distance?: number
  total_amount?: number
  delivery_eta_minutes?: number
  applied_promotions?: Array<{ title?: string, amount?: number, type?: string }>
  ladder_promotions?: Array<{ name?: string, threshold?: number, discount?: number, current_hit?: boolean, missing_need?: number }>
  voucher_trials?: Array<{ voucher_name?: string, amount?: number, trial_payable?: number }>
  payment_assessment?: { payment_hint?: string }
  fee_breakdown?: OrderFeeBreakdown
}

export const ORDER_CONFIRM_CONCURRENCY = 3

export const isWechatPayCancelled = (error: unknown): boolean => {
  const wxError = error as { errMsg?: string }
  return !!wxError?.errMsg?.includes('cancel')
}

export async function mapWithConcurrency<T, R>(
  items: T[],
  limit: number,
  worker: (item: T, index: number) => Promise<R>
): Promise<R[]> {
  if (items.length === 0) {
    return []
  }

  const results = new Array<R>(items.length)
  const workerCount = Math.max(1, Math.min(limit, items.length))
  let nextIndex = 0

  async function consumeQueue() {
    while (nextIndex < items.length) {
      const currentIndex = nextIndex
      nextIndex += 1
      results[currentIndex] = await worker(items[currentIndex], currentIndex)
    }
  }

  await Promise.all(Array.from({ length: workerCount }, () => consumeQueue()))

  return results
}

export function buildAddressSyncKey(address: CheckoutAddress | null | undefined) {
  if (!address) {
    return ''
  }

  return [
    String(address.id || ''),
    String(address.region_id || ''),
    address.contact_name || '',
    address.contact_phone || '',
    address.detail_address || '',
    address.latitude || '',
    address.longitude || '',
    address.is_default ? '1' : '0'
  ].join('|')
}

export function buildCheckoutSnapshotPatch(payload: CheckoutSnapshotPayload, currentCartIds: number[]) {
  const carts: MerchantCartView[] = (payload.carts || []).map((cart) => {
    const items: CartItemView[] = (cart.items || []).map((item) => ({
      id: item.id,
      dishId: item.dishId,
      comboId: item.comboId,
      name: item.name,
      imageUrl: item.imageUrl,
      quantity: item.quantity,
      unitPrice: item.unitPrice,
      priceDisplay: formatPriceNoSymbol(item.unitPrice),
      subtotal: item.subtotal,
      subtotalDisplay: formatPriceNoSymbol(item.subtotal),
      specText: item.specText || '',
      customizations: item.customizations || undefined,
      dishImages: item.dishImages || []
    }))
    const totalCount = cart.totalCount || items.reduce((sum, item) => sum + item.quantity, 0)

    return {
      merchantId: cart.merchantId,
      merchantName: cart.merchantName || '商家',
      orderType: cart.orderType || 'takeout',
      tableId: cart.tableId || undefined,
      reservationId: cart.reservationId || undefined,
      items,
      totalCount,
      subtotal: cart.subtotal,
      subtotalDisplay: formatPriceNoSymbol(cart.subtotal),
      deliveryFee: 0,
      deliveryFeeDisplay: '待计算',
      deliveryFeeDiscount: 0,
      deliveryDistance: 0,
      deliveryEtaMinutes: 0,
      deliveryEtaDisplay: '',
      orderTotal: cart.subtotal,
      orderTotalDisplay: formatPriceNoSymbol(cart.subtotal),
      originalTotalDisplay: formatPriceNoSymbol(cart.subtotal),
      hasDiscount: false,
      appliedPromotions: [],
      ladderPromotions: [],
      voucherTrials: [],
      paymentHint: '',
      feeBreakdownView: buildCustomerOrderFeeBreakdownView()
    }
  })

  return {
    cartIds: payload.cartIds || currentCartIds,
    carts,
    initLoading: false,
    loadError: '',
    pricingError: ''
  }
}

export function syncTakeoutCartSummary(carts: MerchantCartView[]) {
  const totalCount = carts.reduce((sum, cart) => sum + (cart.totalCount || 0), 0)
  const totalPrice = carts.reduce((sum, cart) => sum + (cart.subtotal || 0), 0)

  globalStore.set('cart', {
    items: [],
    totalCount,
    totalPrice,
    totalPriceDisplay: `¥${formatPriceNoSymbol(totalPrice)}`
  })
}

export function buildOrderConfirmCartViews(
  rawResults: Array<{
    merchantCart: { merchant_name?: string, order_type?: string, table_id?: number | null, reservation_id?: number | null }
    merchantId: number
    cartDetail: { subtotal: number, items: CartItemResponse[] }
  }>
): MerchantCartView[] {
  return rawResults
    .filter(({ cartDetail }) => cartDetail.items && cartDetail.items.length > 0)
    .map(({ merchantCart, merchantId, cartDetail }) => {
      const items: CartItemView[] = cartDetail.items.map((item: CartItemResponse) => ({
        id: item.id,
        dishId: item.dish_id,
        comboId: item.combo_id,
        name: item.name,
        imageUrl: getPublicImageUrl(item.image_url || ''),
        quantity: item.quantity,
        unitPrice: item.unit_price,
        priceDisplay: formatPriceNoSymbol(item.unit_price),
        subtotal: item.subtotal,
        subtotalDisplay: formatPriceNoSymbol(item.subtotal),
        specText: item.spec_text || '',
        customizations: item.customizations || undefined,
        dishImages: (item.combo_member_images || []).map((url: string) => getPublicImageUrl(url))
      }))

      return {
        merchantId,
        merchantName: merchantCart.merchant_name || '商家',
        orderType: (merchantCart.order_type || 'takeout') as OrderType,
        tableId: merchantCart.table_id || undefined,
        reservationId: merchantCart.reservation_id || undefined,
        items,
        totalCount: items.reduce((sum, item) => sum + item.quantity, 0),
        subtotal: cartDetail.subtotal,
        subtotalDisplay: formatPriceNoSymbol(cartDetail.subtotal),
        deliveryFee: 0,
        deliveryFeeDisplay: '待计算',
        deliveryFeeDiscount: 0,
        deliveryDistance: 0,
        deliveryEtaMinutes: 0,
        deliveryEtaDisplay: '',
        orderTotal: cartDetail.subtotal,
        orderTotalDisplay: formatPriceNoSymbol(cartDetail.subtotal),
        originalTotalDisplay: formatPriceNoSymbol(cartDetail.subtotal),
        hasDiscount: false,
        appliedPromotions: [],
        ladderPromotions: [],
        voucherTrials: [],
        paymentHint: '',
        feeBreakdownView: buildCustomerOrderFeeBreakdownView()
      }
    })
}

export function buildTodaySlots(startHour: number, endHour: number, stepMinutes: number): string[] {
  const now = new Date()
  const slots: string[] = []
  for (let hour = startHour; hour < endHour; hour += 1) {
    for (let minute = 0; minute < 60; minute += stepMinutes) {
      const slot = new Date(now)
      slot.setHours(hour, minute, 0, 0)
      if (slot.getTime() > now.getTime()) {
        const hh = String(slot.getHours()).padStart(2, '0')
        const mm = String(slot.getMinutes()).padStart(2, '0')
        slots.push(`${hh}:${mm}`)
      }
    }
  }
  return slots
}

export function buildPricingKey(address: CheckoutAddress | null, carts: MerchantCartView[]) {
  if (!address?.id || !carts || carts.length === 0) {
    return ''
  }

  const cartKey = carts
    .map((cart) => `${cart.merchantId}:${cart.items.map((item) => item.id).join('-')}`)
    .join('|')

  return `${address.id}:${cartKey}`
}

function formatTime(date: Date): string {
  const hh = String(date.getHours()).padStart(2, '0')
  const mm = String(date.getMinutes()).padStart(2, '0')
  return `${hh}:${mm}`
}

export function formatEtaWindow(etaMinutes: number): string {
  if (!etaMinutes || etaMinutes <= 0) {
    return ''
  }

  const padding = 5
  const now = new Date()
  const start = new Date(now.getTime() + Math.max(etaMinutes - padding, 0) * 60 * 1000)
  const end = new Date(now.getTime() + (etaMinutes + padding) * 60 * 1000)
  return `${formatTime(start)}-${formatTime(end)}`
}

export function normalizeCustomizations(customizations: Record<string, unknown>): Record<string, number | string> {
  const normalized: Record<string, number | string> = {}
  Object.entries(customizations).forEach(([key, value]) => {
    if (typeof value === 'number' || typeof value === 'string') {
      normalized[key] = value
    } else if (value !== null && value !== undefined) {
      normalized[key] = String(value)
    }
  })
  return normalized
}

export function buildPricingSuccessPatch(
  calcResults: Array<{ cart: MerchantCartView, result: PricingResult }>
) {
  const updated = calcResults
    .filter(({ result }) => !!result)
    .map(({ cart, result }) => {
      const deliveryFee = result.delivery_fee || 0
      const deliveryFeeDiscount = result.delivery_fee_discount || 0
      const finalDeliveryFee = Math.max(0, deliveryFee - deliveryFeeDiscount)
      const orderTotal = result.total_amount || 0
      const originalTotal = (cart.subtotal || 0) + deliveryFee
      const appliedPromotions = (result.applied_promotions || []).map((promotion) => ({
        title: promotion.title || '优惠',
        amount: promotion.amount || 0,
        amountDisplay: formatPriceNoSymbol(promotion.amount || 0),
        type: promotion.type || 'merchant'
      }))

      return {
        ...cart,
        deliveryFee,
        deliveryFeeDisplay: finalDeliveryFee > 0 ? `¥${formatPriceNoSymbol(finalDeliveryFee)}` : '免代取费',
        deliveryFeeDiscount,
        deliveryDistance: result.delivery_distance || 0,
        orderTotal,
        orderTotalDisplay: formatPriceNoSymbol(orderTotal),
        originalTotalDisplay: formatPriceNoSymbol(originalTotal),
        hasDiscount: orderTotal < originalTotal,
        deliveryEtaMinutes: result.delivery_eta_minutes || 0,
        deliveryEtaDisplay: formatEtaWindow(result.delivery_eta_minutes || 0),
        appliedPromotions,
        ladderPromotions: (result.ladder_promotions || []).map((rule) => ({
          name: rule.name || '满减活动',
          thresholdDisplay: formatPriceNoSymbol(rule.threshold || 0),
          discountDisplay: formatPriceNoSymbol(rule.discount || 0),
          currentHit: !!rule.current_hit,
          missingNeedDisplay: formatPriceNoSymbol(rule.missing_need || 0)
        })),
        voucherTrials: (result.voucher_trials || []).map((trial) => ({
          voucherName: trial.voucher_name || '优惠券',
          amountDisplay: formatPriceNoSymbol(trial.amount || 0),
          trialPayableDisplay: formatPriceNoSymbol(trial.trial_payable || 0)
        })),
        paymentHint: result.payment_assessment?.payment_hint || '',
        feeBreakdownView: buildCustomerOrderFeeBreakdownView(result.fee_breakdown)
      }
    })

  const summarySubtotal = updated.reduce((sum, cart) => {
    const merchDiscount = (cart.appliedPromotions || [])
      .filter((promotion) => promotion.type === 'merchant' || promotion.type === 'voucher')
      .reduce((acc, promotion) => acc + (promotion.amount || 0), 0)
    return sum + (cart.subtotal || 0) - merchDiscount
  }, 0)
  const summaryDelivery = updated.reduce(
    (sum, cart) => sum + Math.max(0, (cart.deliveryFee || 0) - (cart.deliveryFeeDiscount || 0)),
    0
  )
  const totalOrderAmount = updated.reduce((sum, cart) => sum + (cart.orderTotal || 0), 0)

  return {
    carts: updated,
    pricingError: '',
    summarySubtotalDisplay: formatPriceNoSymbol(summarySubtotal),
    summaryDeliveryDisplay: summaryDelivery > 0 ? `¥${formatPriceNoSymbol(summaryDelivery)}` : '免代取费',
    orderTotalDisplay: formatPriceNoSymbol(totalOrderAmount)
  }
}

export function buildTakeoutCreateOrderRequest(params: {
  cart: MerchantCartView
  addressId: number
  note: string
}): CreateOrderRequest {
  return {
    merchant_id: params.cart.merchantId,
    items: params.cart.items.map((item) => {
      const orderItem: OrderItemRequest = { quantity: item.quantity }
      if (item.dishId) {
        orderItem.dish_id = item.dishId
      }
      if (item.comboId) {
        orderItem.combo_id = item.comboId
      }
      if (item.customizations) {
        orderItem.customizations = normalizeCustomizations(item.customizations as Record<string, unknown>)
      }
      return orderItem
    }),
    order_type: params.cart.orderType,
    address_id: params.addressId,
    notes: params.note,
    delivery_fee: params.cart.deliveryFee,
    delivery_fee_discount: params.cart.deliveryFeeDiscount,
    delivery_distance: params.cart.deliveryDistance
  }
}
