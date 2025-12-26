"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.OrderAdapter = void 0;
const order_1 = require("../api/order");
const dayjs_1 = __importDefault(require("dayjs"));
/**
 * 订单类型映射 - 对齐swagger枚举值
 */
const ORDER_TYPE_MAP = {
    'takeout': '外卖',
    'dine_in': '堂食',
    'takeaway': '自取',
    'reservation': '预定'
};
/**
 * 订单状态映射 - 对齐swagger枚举值
 */
const ORDER_STATUS_MAP = {
    'pending': { text: '待支付', color: '#E34D59' },
    'paid': { text: '已支付', color: '#ED7B2F' },
    'preparing': { text: '制作中', color: '#0052D9' },
    'ready': { text: '待配送', color: '#0052D9' },
    'delivering': { text: '配送中', color: '#0052D9' },
    'completed': { text: '已完成', color: '#00A870' },
    'cancelled': { text: '已取消', color: '#999999' }
};
class OrderAdapter {
    /**
     * 将API响应转换为列表视图模型
     */
    static toViewModel(dto) {
        const statusInfo = ORDER_STATUS_MAP[dto.status] || { text: dto.status, color: '#999999' };
        return {
            id: dto.id,
            orderNo: dto.order_no,
            merchantId: dto.merchant_id,
            merchantName: dto.merchant_name || '未知商户',
            type: dto.order_type,
            typeText: ORDER_TYPE_MAP[dto.order_type] || '订单',
            status: dto.status,
            statusText: statusInfo.text,
            statusColor: statusInfo.color,
            totalAmount: dto.total_amount,
            totalAmountDisplay: `¥${(dto.total_amount / 100).toFixed(2)}`,
            itemCount: dto.items ? dto.items.reduce((sum, item) => sum + item.quantity, 0) : 0,
            createTime: (0, dayjs_1.default)(dto.created_at).format('YYYY-MM-DD HH:mm')
        };
    }
    /**
     * 将API响应转换为详情视图模型
     */
    static toDetailViewModel(dto) {
        const base = OrderAdapter.toViewModel(dto);
        const payableAmount = (0, order_1.getPayableAmount)(dto);
        const items = dto.items.map((item) => {
            var _a;
            return ({
                id: item.id,
                dishId: item.dish_id,
                comboId: item.combo_id,
                name: item.name,
                imageUrl: item.image_url || '',
                quantity: item.quantity,
                unitPrice: item.unit_price,
                subtotal: item.subtotal,
                unitPriceDisplay: `¥${(item.unit_price / 100).toFixed(2)}`,
                subtotalDisplay: `¥${(item.subtotal / 100).toFixed(2)}`,
                customizations: ((_a = item.customizations) === null || _a === void 0 ? void 0 : _a.map(c => `${c.group_name}: ${c.option_name}`)) || []
            });
        });
        return Object.assign(Object.assign({}, base), { items, subtotal: dto.subtotal, subtotalDisplay: `¥${(dto.subtotal / 100).toFixed(2)}`, deliveryFee: dto.delivery_fee, deliveryFeeDisplay: dto.delivery_fee > 0 ? `¥${(dto.delivery_fee / 100).toFixed(2)}` : '免配送费', deliveryFeeDiscount: dto.delivery_fee_discount, deliveryFeeDiscountDisplay: dto.delivery_fee_discount > 0 ? `-¥${(dto.delivery_fee_discount / 100).toFixed(2)}` : '', discountAmount: dto.discount_amount, discountAmountDisplay: dto.discount_amount > 0 ? `-¥${(dto.discount_amount / 100).toFixed(2)}` : '', payableAmount, payableAmountDisplay: `¥${(payableAmount / 100).toFixed(2)}`, notes: dto.notes });
    }
}
exports.OrderAdapter = OrderAdapter;
