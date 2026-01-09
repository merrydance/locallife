"use strict";
/**
 * 订单卡片适配器
 * 将OrderResponse转换为订单列表展示所需的OrderCard格式
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.OrderCardAdapter = void 0;
const order_1 = require("../api/order");
exports.OrderCardAdapter = {
    /**
     * 将OrderResponse转换为OrderCardViewModel
     */
    toCardViewModel(order) {
        const status = mapStatus(order.status);
        // 可取消状态：待支付、已支付、制作中
        const canCancel = ['pending', 'paid', 'preparing'].includes(order.status);
        // 可支付状态：待支付
        const canPay = order.status === 'pending';
        return {
            id: order.id,
            orderNo: order.order_no,
            merchantName: order.merchant_name,
            status,
            statusClass: status,
            statusLabel: getStatusLabel(order.status),
            highlight: generateHighlight(order),
            createTimeDisplay: formatCreatedAt(order.created_at),
            totalDisplay: formatPrice((0, order_1.getPayableAmount)(order)),
            badges: generateBadges(order),
            previewItems: extractPreviewItems(order),
            canReorder: order.status === 'completed' || order.status === 'cancelled',
            canCancel,
            canPay
        };
    },
    /**
     * 按状态优先级排序订单
     */
    sortByPriority(orders) {
        const priority = {
            preparing: 3,
            delivering: 2,
            delivered: 1,
            completed: 1,
            pending: 0
        };
        return [...orders].sort((a, b) => {
            const diff = priority[b.status] - priority[a.status];
            if (diff !== 0) {
                return diff;
            }
            // 相同状态按ID倒序（新订单在前）
            return b.id - a.id;
        });
    }
};
/**
 * 映射订单状态到展示状态
 */
function mapStatus(status) {
    switch (status) {
        case 'completed':
            return 'completed';
        case 'cancelled':
            return 'cancelled';
        case 'delivering':
            return 'delivering';
        case 'ready':
        case 'preparing':
            return 'preparing';
        case 'paid':
            return 'preparing';
        case 'pending':
            return 'pending';
        default:
            return 'pending';
    }
}
/**
 * 获取状态标签
 */
function getStatusLabel(status) {
    const labels = {
        'pending': '待支付',
        'paid': '已支付',
        'preparing': '制作中',
        'ready': '待配送',
        'delivering': '配送中',
        'completed': '已完成',
        'cancelled': '已取消'
    };
    return labels[status] || status;
}
/**
 * 生成高亮信息
 */
function generateHighlight(order) {
    switch (order.status) {
        case 'delivering':
            return '骑手正在配送中，请耐心等待';
        case 'ready':
            return '商家已备餐完成，等待骑手取餐';
        case 'preparing':
            return '商家正在制作您的餐品';
        case 'completed':
            return '订单已完成，感谢您的惠顾';
        default:
            return '';
    }
}
/**
 * 生成订单徽章
 */
function generateBadges(order) {
    const badges = [];
    // 订单类型徽章
    if (order.order_type === 'takeout') {
        badges.push('外卖');
    }
    else if (order.order_type === 'dine_in') {
        badges.push('堂食');
    }
    else if (order.order_type === 'takeaway') {
        badges.push('自取');
    }
    // 支付方式徽章（OrderResponse中没有payment_method字段）
    // 暂时注释
    // 优惠徽章
    if (order.discount_amount > 0) {
        badges.push(`已减¥${(order.discount_amount / 100).toFixed(2)}`);
    }
    return badges;
}
/**
 * 提取前3个菜品作为预览
 */
function extractPreviewItems(order) {
    if (!order.items || order.items.length === 0) {
        return [];
    }
    return order.items.slice(0, 3).map(item => ({
        dishId: item.dish_id || 0,
        dishName: item.name, // 对齐swagger: 使用name而非dish_name
        quantity: item.quantity,
        imageUrl: item.image_url || 'https://tdesign.gtimg.com/mobile/demos/example1.png' // 对齐swagger: 使用image_url
    }));
}
/**
 * 格式化价格
 */
function formatPrice(amount) {
    return `¥${(amount / 100).toFixed(2)}`;
}
/**
 * 格式化创建时间
 */
function formatCreatedAt(timeStr) {
    try {
        const date = new Date(timeStr);
        const now = new Date();
        const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
        const targetDate = new Date(date.getFullYear(), date.getMonth(), date.getDate());
        const hours = ('0' + date.getHours()).slice(-2);
        const minutes = ('0' + date.getMinutes()).slice(-2);
        const timeOnly = `${hours}:${minutes}`;
        if (targetDate.getTime() === today.getTime()) {
            return `今天 · ${timeOnly}`;
        }
        else if (targetDate.getTime() === today.getTime() - 86400000) {
            return `昨天 · ${timeOnly}`;
        }
        else {
            const month = ('0' + (date.getMonth() + 1)).slice(-2);
            const day = ('0' + date.getDate()).slice(-2);
            return `${month}-${day} · ${timeOnly}`;
        }
    }
    catch (e) {
        return timeStr;
    }
}
