import { Order, OrderDetail, OrderItem } from '../_models/order'
import { OrderResponse, getPayableAmount } from '../_main_shared/api/order'
import { getPublicImageUrl } from '../../../utils/image'
import { buildCustomerOrderFeeBreakdownView } from '../_main_shared/utils/order-fee-breakdown-view'
import { buildCustomerOrderStatusView } from '../_utils/customer-order-status-view'
import dayjs from '../_main_shared/miniprogram_npm/dayjs/index'

/**
 * 订单类型映射 - 对齐swagger枚举值
 */
const ORDER_TYPE_MAP: Record<string, string> = {
  'takeout': '外卖',
  'dine_in': '堂食',
  'takeaway': '自取',
  'reservation': '预定'
}

export class OrderAdapter {
  /**
   * 将API响应转换为列表视图模型
   */
  static toViewModel(dto: OrderResponse): Order {
    const statusView = buildCustomerOrderStatusView(dto)
    const statusHint = dto.status_hint?.trim()
    const badgeTexts = normalizeBadges(dto.badges)

    return {
      id: dto.id,
      orderNo: dto.order_no,
      merchantId: dto.merchant_id,
      merchantName: dto.merchant_name || '未知商户',
      type: dto.order_type,
      typeText: ORDER_TYPE_MAP[dto.order_type] || '订单',
      status: dto.status,
      statusText: statusView.label,
      statusColor: statusView.color,
      statusHint: statusHint || undefined,
      badges: badgeTexts,
      actions: dto.actions,
      paymentContext: dto.payment_context,
      paidAt: dto.paid_at,
      pickupCodeMasked: dto.pickup_code_masked,
      overtime: dto.overtime,
      fulfillmentStatus: dto.fulfillment_status,
      totalAmount: dto.total_amount,
      totalAmountDisplay: `¥${(dto.total_amount / 100).toFixed(2)}`,
      itemCount: dto.items ? dto.items.reduce((sum, item) => sum + item.quantity, 0) : 0,
      createTime: dayjs(dto.created_at).format('YYYY-MM-DD HH:mm')
    }
  }

  /**
   * 将API响应转换为详情视图模型
   */
  static toDetailViewModel(dto: OrderResponse): OrderDetail {
    const base = OrderAdapter.toViewModel(dto)
    const payableAmount = getPayableAmount(dto)

    const items: OrderItem[] = (dto.items ?? []).map((item) => ({
      id: item.id,
      dishId: item.dish_id,
      comboId: item.combo_id,
      name: item.name,
      imageUrl: item.image_url ? getPublicImageUrl(item.image_url) : '',
      quantity: item.quantity,
      unitPrice: item.unit_price,
      subtotal: item.subtotal,
      unitPriceDisplay: `¥${(item.unit_price / 100).toFixed(2)}`,
      subtotalDisplay: `¥${(item.subtotal / 100).toFixed(2)}`,
      customizations: item.customizations?.map((c) => `${c.name}: ${c.value}`) || []
    }))

    const expectDeliverTime = formatDeliveryWindow(dto.estimated_delivery_at, dto.delivery_eta_minutes)

    return {
      ...base,
      items,
      subtotal: dto.subtotal,
      subtotalDisplay: `¥${(dto.subtotal / 100).toFixed(2)}`,
      deliveryFee: dto.delivery_fee,
      deliveryFeeDisplay: dto.delivery_fee > 0 ? `¥${(dto.delivery_fee / 100).toFixed(2)}` : '免代取费',
      deliveryFeeDiscount: dto.delivery_fee_discount,
      deliveryFeeDiscountDisplay: dto.delivery_fee_discount > 0 ? `-¥${(dto.delivery_fee_discount / 100).toFixed(2)}` : '',
      discountAmount: dto.discount_amount,
      discountAmountDisplay: dto.discount_amount > 0 ? `-¥${(dto.discount_amount / 100).toFixed(2)}` : '',
      payableAmount,
      payableAmountDisplay: `¥${(payableAmount / 100).toFixed(2)}`,
      notes: dto.notes,
      paidAt: dto.paid_at,
      // 代取地址信息
      address: dto.delivery_address,
      contactName: dto.delivery_contact_name,
      contactPhone: dto.delivery_contact_phone,
      // 商户电话
      merchantPhone: dto.merchant_phone,
      estimatedDeliveryAt: dto.estimated_delivery_at,
      deliveryEtaMinutes: dto.delivery_eta_minutes,
      expectDeliverTime,
      tableId: dto.table_id,
      reservationId: dto.reservation_id,
      replacedByOrderId: dto.replaced_by_order_id,
      fulfillmentStatus: dto.fulfillment_status,
      paymentContext: dto.payment_context,
      feeBreakdownView: buildCustomerOrderFeeBreakdownView(dto.fee_breakdown),
      timeline: dto.fulfillment_status ? [{
        time: dto.updated_at || dto.created_at,
        title: buildCustomerOrderStatusView(dto).label
      }] : undefined
    }
  }
}

function normalizeBadges(badges?: Array<{ text: string }> | string[]): string[] {
  if (!badges || badges.length === 0) return []
  if (typeof badges[0] === 'string') {
    return badges as string[]
  }
  return (badges as Array<{ text: string }>).map((badge) => badge.text).filter(Boolean)
}

// 生成送达时间段文案（例如 12:40-12:50）
function formatDeliveryWindow(estimatedAt?: string, etaMinutes?: number): string | undefined {
  const paddingMinutes = 5

  if (estimatedAt) {
    const target = dayjs(estimatedAt)
    if (!target.isValid()) return undefined
    const start = target.subtract(paddingMinutes, 'minute')
    const end = target.add(paddingMinutes, 'minute')
    return `${start.format('HH:mm')}-${end.format('HH:mm')}`
  }

  if (etaMinutes && etaMinutes > 0) {
    const now = dayjs()
    const start = now.add(Math.max(etaMinutes - paddingMinutes, 0), 'minute')
    const end = now.add(etaMinutes + paddingMinutes, 'minute')
    return `${start.format('HH:mm')}-${end.format('HH:mm')}`
  }

  return undefined
}
