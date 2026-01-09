import type { OrderStatus, OrderType } from '../api/order'

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
    address?: string              // 配送地址
    contactName?: string          // 配送联系人
    contactPhone?: string         // 配送联系电话
    merchantPhone?: string        // 商户电话
    timeline?: OrderTimelineItem[]
}

/**
 * 订单时间线项
 */
export interface OrderTimelineItem {
    time: string
    title: string
    description?: string
}
