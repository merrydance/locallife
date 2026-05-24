import type { OrderStatus, OrderType, FulfillmentStatus, OrderPaymentContext } from '../api/order'
import type { CustomerOrderFeeBreakdownView } from '../utils/order-fee-breakdown-view'

/**
 * 订单视图模型 - 用于UI展示
 * 对齐swagger api.orderResponse，字段类型与后端一致
 */
export interface Order {
    id: number                    // 改为number，对齐后端
    orderNo: string
    merchantId: number            // 改为number，对齐后端
    merchantName: string
    type: OrderType               // 使用API层的枚举类型
    typeText: string              // ViewModel: 外卖/堂食/自取
    status: OrderStatus           // 使用API层的枚举类型
    statusText: string            // ViewModel: 待支付/已支付等
    statusColor: string           // ViewModel: 状态颜色
    statusHint?: string           // 后端提示文案
    badges?: string[]             // 徽章文本
    actions?: string[]            // 可执行动作
    paymentContext?: OrderPaymentContext
    paidAt?: string               // 支付时间
    pickupCodeMasked?: string     // 取餐码（脱敏）
    overtime?: boolean            // 是否超时
    fulfillmentStatus?: FulfillmentStatus // 履约状态
    totalAmount: number           // 订单总金额（分）
    totalAmountDisplay: string    // ViewModel: ¥xx.xx
    itemCount: number
    createTime: string            // ViewModel: 格式化后的时间
}

/**
 * 订单商品项视图模型
 */
export interface OrderItem {
    id: number                    // 改为number
    dishId?: number               // 改为number，可选
    comboId?: number              // 套餐ID
    name: string                  // 商品名称（对齐swagger的name字段）
    imageUrl: string              // 商品图片
    quantity: number
    unitPrice: number             // 单价（分）
    subtotal: number              // 小计（分）
    unitPriceDisplay: string      // ViewModel: ¥xx.xx
    subtotalDisplay: string       // ViewModel: ¥xx.xx
    customizations: string[]      // ViewModel: 定制化选项描述
}

/**
 * 订单详情视图模型
 */
export interface OrderDetail extends Order {
    items: OrderItem[]
    subtotal: number              // 商品小计（分）
    subtotalDisplay: string
    deliveryFee: number
    deliveryFeeDisplay: string
    deliveryFeeDiscount: number
    deliveryFeeDiscountDisplay: string
    discountAmount: number
    discountAmountDisplay: string
    payableAmount: number         // 计算值: totalAmount - discountAmount
    payableAmountDisplay: string
    notes?: string
    address?: string              // 代取地址
    contactName?: string          // 代取联系人
    contactPhone?: string         // 代取联系电话
    merchantPhone?: string        // 商户电话
    paidAt?: string               // 支付时间
    estimatedDeliveryAt?: string  // 预计送达时间戳
    deliveryEtaMinutes?: number   // 预计送达总时长（分钟）
    expectDeliverTime?: string    // 展示用的送达时间段
    tableId?: number              // 堂食/预订 桌台ID
    reservationId?: number        // 预订ID
    replacedByOrderId?: number    // 被替换的新订单ID
    fulfillmentStatus?: FulfillmentStatus
    paymentContext?: OrderPaymentContext
    reservationDate?: string
    reservationTime?: string
    guestCount?: number
    timeline?: OrderTimelineItem[]
    feeBreakdownView?: CustomerOrderFeeBreakdownView
}

/**
 * 订单时间线项
 */
export interface OrderTimelineItem {
    time: string
    title: string
    description?: string
}
