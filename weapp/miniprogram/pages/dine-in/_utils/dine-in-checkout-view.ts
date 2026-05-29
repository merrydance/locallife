import { getPublicImageUrl } from '../../../utils/image'
import { formatPriceNoSymbol } from '../../../utils/util'
import {
  loadDineInCheckoutSession,
  type CheckoutCalculationResponse,
  type CheckoutCartResponse,
  type CheckoutMerchantDetail,
  type CheckoutTableDetail
} from '../_services/dine-in-checkout'

export type PromotionItem = NonNullable<CheckoutCalculationResponse['applied_promotions']>[number] & {
  amountDisplay: string
}

export type LadderItem = NonNullable<CheckoutCalculationResponse['ladder_promotions']>[number] & {
  thresholdDisplay: string
  discountDisplay: string
  missingNeedDisplay: string
}

export type VoucherTrialItem = NonNullable<CheckoutCalculationResponse['voucher_trials']>[number] & {
  amountDisplay: string
  trialPayableDisplay: string
}

export type PaymentAssessmentItem = NonNullable<CheckoutCalculationResponse['payment_assessment']>

export interface CalculationView {
  subtotal: number
  discount_amount: number
  total_amount: number
  subtotalDisplay: string
  totalDisplay: string
  applied_promotions: PromotionItem[]
  ladder_promotions: LadderItem[]
  voucher_trials: VoucherTrialItem[]
  payment_assessment: PaymentAssessmentItem | null
}

export interface PaymentMethodView {
  id: string
  name: string
  icon: string
  disabled: boolean
}

export type CartItemView = CheckoutCartResponse['items'][number] & {
  image_url?: string
  dish_image?: string
  priceDisplay: string
  subtotalDisplay: string
}

export type CartView = CheckoutCartResponse & {
  items: CartItemView[]
}

export type CheckoutMerchantInfo = CheckoutMerchantDetail | {
  id: number
  name: string
  logo_url?: string
  cover_image?: string
  address?: string
}

export type CheckoutTableInfo = CheckoutTableDetail | {
  table_no: string
}

type CheckoutSessionResponse = Awaited<ReturnType<typeof loadDineInCheckoutSession>>

export function buildCheckoutSessionState(menuResponse: CheckoutSessionResponse, billingGroupId: number) {
  return {
    sessionId: menuResponse.session.id,
    billingGroupId: billingGroupId || menuResponse.billing_group.id,
    merchantId: menuResponse.session.merchant_id,
    tableId: menuResponse.session.table_id,
    reservationId: menuResponse.session.reservation_id || 0,
    orderType: 'dine_in' as const,
    merchantInfo: menuResponse.merchant,
    tableInfo: menuResponse.table
  }
}

export function buildCalculationView(calculation: CheckoutCalculationResponse): CalculationView {
  return {
    ...calculation,
    subtotalDisplay: formatPriceNoSymbol(calculation.subtotal || 0),
    totalDisplay: formatPriceNoSymbol(calculation.total_amount || 0),
    applied_promotions: (calculation.applied_promotions || []).map((item) => ({
      ...item,
      amountDisplay: formatPriceNoSymbol(item.amount || 0)
    })),
    ladder_promotions: (calculation.ladder_promotions || []).map((item) => ({
      ...item,
      thresholdDisplay: formatPriceNoSymbol(item.threshold || 0),
      discountDisplay: formatPriceNoSymbol(item.discount || 0),
      missingNeedDisplay: formatPriceNoSymbol(item.missing_need || 0)
    })),
    voucher_trials: (calculation.voucher_trials || []).map((item) => ({
      ...item,
      amountDisplay: formatPriceNoSymbol(item.amount || 0),
      trialPayableDisplay: formatPriceNoSymbol(item.trial_payable || 0)
    })),
    payment_assessment: calculation.payment_assessment || null
  }
}

export function buildCheckoutCartView(cart: CheckoutCartResponse): CartView {
  return {
    ...cart,
    items: (cart.items || []).map((item) => {
      const rawDishImage = (item as { dish_image?: string }).dish_image
      const normalizedImage = getPublicImageUrl(item.image_url || rawDishImage || '')
      return {
        ...item,
        image_url: normalizedImage,
        dish_image: normalizedImage,
        priceDisplay: formatPriceNoSymbol(item.unit_price || 0),
        subtotalDisplay: formatPriceNoSymbol(item.subtotal || 0)
      }
    })
  }
}

export function buildPaymentMethods(memberBalance: number, memberBalanceDisplay: string): PaymentMethodView[] {
  return [
    { id: 'wechat_pay', name: '微信支付', icon: 'logo-wechat', disabled: false },
    {
      id: 'balance',
      name: `储值支付 (¥${memberBalanceDisplay})`,
      icon: 'wallet',
      disabled: memberBalance <= 0
    }
  ]
}

export function buildCheckoutRenderState(params: {
  merchantInfo: CheckoutMerchantInfo
  cart: CheckoutCartResponse
  calculation: CheckoutCalculationResponse
  memberBalance: number
  memberBalanceDisplay: string
  selectedPaymentMethod: string
}) {
  const processedCalculation = buildCalculationView(params.calculation)
  const processedCart = buildCheckoutCartView(params.cart)
  const balanceInsufficient = params.memberBalance < params.calculation.total_amount

  return {
    merchantInfo: {
      ...params.merchantInfo,
      logo_url: getPublicImageUrl(params.merchantInfo.logo_url || params.merchantInfo.cover_image || '')
    },
    cart: processedCart,
    calculation: processedCalculation,
    balanceInsufficient,
    paymentMethods: buildPaymentMethods(params.memberBalance, params.memberBalanceDisplay),
    selectedPaymentMethod: balanceInsufficient ? 'wechat_pay' : params.selectedPaymentMethod
  }
}