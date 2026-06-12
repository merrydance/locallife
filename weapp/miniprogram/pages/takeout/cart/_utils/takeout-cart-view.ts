import { CartResponse, CartSummaryResponse, MerchantCartResponse } from '@/api/cart'
import { getPublicImageUrl } from '@/utils/image'

export interface CartSummaryView {
  cartCount: number
  totalItems: number
  totalAmount: number
  totalAmountDisplay: string
}

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
  isAvailable: boolean
  isPackaging: boolean
  specDisplay?: string
  customizations?: Record<string, unknown>
  dishImages?: string[]
}

export interface MerchantCartGroup {
  cartId: number
  merchantId: number
  orderType?: string
  tableId?: number
  reservationId?: number
  merchantName: string
  merchantLogo: string
  items: CartItemView[]
  subtotal: number
  subtotalDisplay: string
  deliveryFee: number
  deliveryFeeDiscount: number
  deliveryFeeLabel: string
  deliveryFeeDisplay: string
  totalAmount: number
  totalAmountDisplay: string
  itemCount: number
  allAvailable: boolean
  packagingRequired: boolean
  selected: boolean
  errorStatus?: string
}

export function isTakeawayCartGroup(group: Pick<MerchantCartGroup, 'orderType'>): boolean {
  return group.orderType === 'takeaway'
}

export function getDeliveryFeeLabel(group: Pick<MerchantCartGroup, 'orderType'>): string {
  return isTakeawayCartGroup(group) ? '自取费用' : '代取费'
}

export function isAbortLikeError(error: unknown): boolean {
  if (!error) return false

  if (typeof error === 'object' && error !== null) {
    const maybeErrMsg = (error as { errMsg?: unknown }).errMsg
    if (typeof maybeErrMsg === 'string' && maybeErrMsg.toLowerCase().includes('abort')) {
      return true
    }

    const maybeMessage = (error as { message?: unknown }).message
    if (typeof maybeMessage === 'string') {
      const lower = maybeMessage.toLowerCase()
      if (lower.includes('abort') || lower.includes('请求已取消')) {
        return true
      }
    }
  }

  if (typeof error === 'string') {
    return error.toLowerCase().includes('abort')
  }

  return false
}

export function formatCurrency(fen: number): string {
  return `¥${(fen / 100).toFixed(2)}`
}

export function buildEmptyCartSummary(): CartSummaryView {
  return {
    cartCount: 0,
    totalItems: 0,
    totalAmount: 0,
    totalAmountDisplay: formatCurrency(0)
  }
}

export function buildCartSummary(summary: CartSummaryResponse | undefined, groupCount: number): CartSummaryView {
  const totalAmount = summary?.total_amount || 0
  return {
    cartCount: summary?.cart_count || groupCount,
    totalItems: summary?.total_items || 0,
    totalAmount,
    totalAmountDisplay: formatCurrency(totalAmount)
  }
}

export function buildMerchantGroup(merchantCart: MerchantCartResponse, cartDetail: CartResponse): MerchantCartGroup {
  const items: CartItemView[] = (cartDetail.items || []).map((item) => ({
    id: item.id,
    dishId: item.dish_id,
    comboId: item.combo_id,
    name: item.name,
    imageUrl: getPublicImageUrl(item.image_url || ''),
    quantity: item.quantity,
    unitPrice: item.unit_price,
    priceDisplay: formatCurrency(item.unit_price),
    subtotal: item.subtotal,
    subtotalDisplay: formatCurrency(item.subtotal),
    isAvailable: item.is_available,
    isPackaging: item.is_packaging,
    specDisplay: item.spec_text || '',
    customizations: item.customizations,
    dishImages: item.combo_member_images?.map((url) => getPublicImageUrl(url)) || []
  }))

  const subtotal = cartDetail.subtotal || 0
  const orderType = cartDetail.order_type || merchantCart.order_type || 'takeout'

  return {
    cartId: cartDetail.id,
    merchantId: merchantCart.merchant_id || 0,
    orderType,
    tableId: cartDetail.table_id ?? merchantCart.table_id,
    reservationId: cartDetail.reservation_id ?? merchantCart.reservation_id,
    merchantName: merchantCart.merchant_name || '未知商户',
    merchantLogo: getPublicImageUrl(merchantCart.merchant_logo || ''),
    items,
    subtotal,
    subtotalDisplay: formatCurrency(subtotal),
    deliveryFee: 0,
    deliveryFeeDiscount: 0,
    deliveryFeeLabel: getDeliveryFeeLabel({ orderType }),
    deliveryFeeDisplay: '待计算',
    totalAmount: subtotal,
    totalAmountDisplay: formatCurrency(subtotal),
    itemCount: items.reduce((sum, item) => sum + item.quantity, 0),
    allAvailable: items.every((item) => item.isAvailable),
    packagingRequired: cartDetail.packaging_required,
    selected: true
  }
}

export function buildUpdatedGroupWithDeliveryFee(
  group: MerchantCartGroup,
  deliveryFee: number,
  deliveryFeeDiscount: number = 0
): MerchantCartGroup {
  const payableDeliveryFee = Math.max(0, deliveryFee - deliveryFeeDiscount)
  const totalAmount = group.subtotal + payableDeliveryFee
  return {
    ...group,
    deliveryFee,
    deliveryFeeDiscount,
    deliveryFeeLabel: getDeliveryFeeLabel(group),
    deliveryFeeDisplay: isTakeawayCartGroup(group)
      ? '无需代取费'
      : (payableDeliveryFee > 0 ? formatCurrency(payableDeliveryFee) : '免代取费'),
    totalAmount,
    totalAmountDisplay: formatCurrency(totalAmount),
    errorStatus: ''
  }
}

export function buildRecalculatedGroup(group: MerchantCartGroup): MerchantCartGroup {
  const subtotal = group.items.reduce((sum, item) => sum + item.unitPrice * item.quantity, 0)
  const itemCount = group.items.reduce((sum, item) => sum + item.quantity, 0)
  const totalAmount = subtotal + Math.max(0, (group.deliveryFee || 0) - (group.deliveryFeeDiscount || 0))
  return {
    ...group,
    subtotal,
    subtotalDisplay: formatCurrency(subtotal),
    itemCount,
    totalAmount,
    totalAmountDisplay: formatCurrency(totalAmount)
  }
}

export function getCheckoutTotal(groups: MerchantCartGroup[], selectedCartIds: number[]): number {
  return groups.reduce((sum, group) => {
    if (!selectedCartIds.includes(group.cartId)) {
      return sum
    }
    return sum + group.totalAmount
  }, 0)
}

export function getTotalCount(groups: MerchantCartGroup[]): number {
  return groups.reduce((sum, group) => sum + group.itemCount, 0)
}

export function getPackagingCheckoutBlocker(groups: MerchantCartGroup[], selectedCartIds: number[]): string {
  for (const group of groups) {
    if (!selectedCartIds.includes(group.cartId) || !group.packagingRequired) {
      continue
    }

    const selectedPackagingCount = group.items.reduce((sum, item) => {
      return item.isPackaging ? sum + item.quantity : sum
    }, 0)

    if (selectedPackagingCount === 0) {
      return '请先选择包装方式'
    }
    if (selectedPackagingCount > 1) {
      return '只能选择一种包装方式'
    }
  }

  return ''
}
