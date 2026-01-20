"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const logger_1 = require("../../../utils/logger");
const cart_1 = __importDefault(require("../../../services/cart"));
const order_1 = require("../../../api/order");
const payment_1 = require("../../../api/payment");
const order_2 = require("../../../adapters/order");
const timeline_1 = require("../../../utils/timeline");
const reservation_1 = require("../../../api/reservation");
// 取消原因选项
const CANCEL_REASONS = [
    '不想要了',
    '信息填写错误',
    '商品价格较贵',
    '配送时间太长',
    '其他原因'
];
Page({
    data: {
        orderId: '',
        order: null,
        orderDTO: null,
        reservationInfo: null,
        navBarHeight: 88,
        loading: false,
        showTrackingButton: false,
        showConfirmButton: false,
        showCancelButton: false,
        showPayButton: false,
        showUrgeButton: false,
        lastUrgeTime: 0, // 上次催单时间
        urgeCountdown: 0 // 催单倒计时（秒）
    },
    onLoad(options) {
        if (options.id) {
            this.setData({ orderId: options.id });
            this.loadOrderDetail();
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    async loadOrderDetail() {
        this.setData({ loading: true });
        try {
            const orderDTO = await (0, order_1.getOrderDetail)(parseInt(this.data.orderId));
            const order = order_2.OrderAdapter.toDetailViewModel(orderDTO);
            let reservationInfo = null;
            if (orderDTO.order_type === 'reservation' && orderDTO.reservation_id) {
                try {
                    reservationInfo = await reservation_1.ReservationService.getReservationDetail(orderDTO.reservation_id);
                }
                catch (e) {
                    logger_1.logger.warn('Fetch reservation detail failed', e);
                }
            }
            const actions = orderDTO.actions || [];
            // 判断是否显示物流追踪按钮（外卖订单且状态为配送中/待确认）
            const showTrackingButton = orderDTO.order_type === 'takeout' &&
                ['delivering', 'rider_delivered'].includes(orderDTO.status);
            // 判断是否显示确认收货按钮（服务端 actions 控制）
            const showConfirmButton = actions.includes('confirm');
            // 待支付展示去支付按钮
            const showPayButton = actions.includes('pay');
            // 判断是否显示取消按钮（服务端 actions 控制）
            const showCancelButton = actions.includes('cancel');
            // 判断是否显示催单按钮（服务端 actions 控制）
            const showUrgeButton = actions.includes('urge');
            // 生成订单时间线
            const timeline = (0, timeline_1.generateOrderTimeline)(orderDTO);
            this.setData({
                order: { ...order, timeline },
                orderDTO,
                reservationInfo,
                loading: false,
                showTrackingButton,
                showConfirmButton,
                showCancelButton,
                showPayButton,
                showUrgeButton
            });
            // 检查催单冷却时间
            this.checkUrgeCooldown();
        }
        catch (error) {
            logger_1.logger.error('Load order detail failed:', error, 'Detail');
            wx.showToast({ title: '加载失败', icon: 'error' });
            this.setData({ loading: false });
        }
    },
    // 检查催单冷却时间
    checkUrgeCooldown() {
        const { lastUrgeTime } = this.data;
        if (!lastUrgeTime)
            return;
        const elapsed = Date.now() - lastUrgeTime;
        const cooldownMs = 5 * 60 * 1000; // 5分钟冷却
        if (elapsed < cooldownMs) {
            const remaining = Math.ceil((cooldownMs - elapsed) / 1000);
            this.setData({ urgeCountdown: remaining });
            this.startUrgeCountdown();
        }
    },
    // 开始催单倒计时
    startUrgeCountdown() {
        const timer = setInterval(() => {
            const { urgeCountdown } = this.data;
            if (urgeCountdown <= 1) {
                clearInterval(timer);
                this.setData({ urgeCountdown: 0 });
            }
            else {
                this.setData({ urgeCountdown: urgeCountdown - 1 });
            }
        }, 1000);
    },
    onCallMerchant() {
        wx.showToast({ title: '暂无商家电话', icon: 'none' });
    },
    onCancelOrder() {
        // 显示取消原因选择
        wx.showActionSheet({
            itemList: CANCEL_REASONS,
            success: async (res) => {
                const reason = CANCEL_REASONS[res.tapIndex];
                await this.doCancelOrder(reason);
            }
        });
    },
    async doCancelOrder(reason) {
        wx.showLoading({ title: '取消中...' });
        try {
            await (0, order_1.cancelOrder)(parseInt(this.data.orderId), { reason });
            wx.hideLoading();
            wx.showToast({ title: '已取消', icon: 'success' });
            setTimeout(() => this.loadOrderDetail(), 1500);
        }
        catch (error) {
            wx.hideLoading();
            logger_1.logger.error('取消订单失败', error, 'Detail.doCancelOrder');
            wx.showToast({ title: '取消失败', icon: 'error' });
        }
    },
    // 催单功能
    async onUrgeOrder() {
        const { urgeCountdown } = this.data;
        if (urgeCountdown > 0) {
            wx.showToast({ title: `${urgeCountdown}秒后可再次催单`, icon: 'none' });
            return;
        }
        wx.showLoading({ title: '催单中...' });
        try {
            await (0, order_1.urgeOrder)(parseInt(this.data.orderId), { message: '请尽快处理' });
            wx.hideLoading();
            wx.showToast({ title: '催单成功', icon: 'success' });
            // 设置5分钟冷却
            this.setData({
                lastUrgeTime: Date.now(),
                urgeCountdown: 300
            });
            this.startUrgeCountdown();
        }
        catch (error) {
            wx.hideLoading();
            logger_1.logger.error('催单失败', error, 'Detail.onUrgeOrder');
            wx.showToast({ title: '催单失败', icon: 'error' });
        }
    },
    onReview() {
        const { orderDTO } = this.data;
        if (orderDTO) {
            wx.navigateTo({
                url: `/pages/user_center/reviews/create/index?orderId=${orderDTO.id}&merchantId=${orderDTO.merchant_id}`
            });
        }
    },
    async onReorder() {
        const { order } = this.data;
        if (!order)
            return;
        wx.showLoading({ title: '再次购买中...' });
        try {
            await cart_1.default.loadCart(order.merchantId, {
                orderType: order.type,
                tableId: order.tableId,
                reservationId: order.reservationId
            });
            const addResults = await Promise.all(order.items.map((item) => cart_1.default.addItem({
                merchantId: order.merchantId,
                dishId: item.dishId,
                comboId: item.comboId,
                quantity: item.quantity
            })));
            if (addResults.some((ok) => !ok)) {
                wx.showToast({ title: '部分商品添加失败', icon: 'none' });
                return;
            }
            wx.showToast({ title: '已加入购物车', icon: 'success' });
            setTimeout(() => {
                wx.navigateTo({ url: '/pages/takeout/cart/index' });
            }, 300);
        }
        catch (error) {
            logger_1.logger.error('再次购买失败', error, 'Detail.onReorder');
            wx.showToast({ title: '操作失败', icon: 'error' });
        }
        finally {
            wx.hideLoading();
        }
    },
    onViewTracking() {
        wx.navigateTo({
            url: `/pages/orders/tracking/index?orderId=${this.data.orderId}`
        });
    },
    // 去支付（直接拉起微信支付）
    async onPayOrder() {
        const { orderId } = this.data;
        if (!orderId) {
            wx.showToast({ title: '订单信息缺失', icon: 'none' });
            return;
        }
        wx.showLoading({ title: '拉起支付...' });
        try {
            await (0, payment_1.processPayment)(parseInt(orderId), 'order');
            wx.showToast({ title: '支付成功', icon: 'success' });
            setTimeout(() => this.loadOrderDetail(), 800);
        }
        catch (error) {
            logger_1.logger.error('支付失败', error, 'Detail.onPayOrder');
            wx.showToast({ title: '支付未完成', icon: 'none' });
        }
        finally {
            wx.hideLoading();
        }
    },
    async onConfirmReceipt() {
        wx.showModal({
            title: '确认收货',
            content: '确认已收到订单？',
            success: async (res) => {
                if (res.confirm) {
                    try {
                        await (0, order_1.confirmOrder)(parseInt(this.data.orderId));
                        wx.showToast({ title: '确认成功', icon: 'success' });
                        setTimeout(() => this.loadOrderDetail(), 1500);
                    }
                    catch (error) {
                        logger_1.logger.error('确认收货失败', error, 'Detail.onConfirmReceipt');
                        wx.showToast({ title: '确认失败', icon: 'error' });
                    }
                }
            }
        });
    },
    onContactRider() {
        wx.showToast({ title: '联系骑手功能开发中', icon: 'none' });
    },
    onViewPayment() {
        wx.navigateTo({
            url: `/pages/user_center/payment-detail/index?orderId=${this.data.orderId}`
        });
    }
});
