"use strict";
/**
 * 堂食/预订结算页面
 * 处理堂食和预订订单的结算和支付流程
 */
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
const cart_1 = require("../../../api/cart");
const order_1 = require("../../../api/order");
const payment_1 = require("../../../api/payment");
Page({
    data: {
        tableId: 0,
        merchantId: 0,
        reservationId: 0, // 预订点菜场景
        orderType: 'dine_in',
        // 订单数据
        cart: null,
        calculation: null,
        tableInfo: null,
        // 支付方式
        paymentMethods: [
            { id: 'wechat_pay', name: '微信支付', icon: '💳', enabled: true },
            { id: 'alipay', name: '支付宝', icon: '💰', enabled: true },
            { id: 'balance', name: '余额支付', icon: '💎', enabled: true }
        ],
        selectedPaymentMethod: 'wechat_pay',
        // 界面状态
        loading: true,
        submitting: false,
        // 备注信息
        remark: '',
        // 用餐信息
        diningInfo: {
            guest_count: 1,
            special_requests: ''
        }
    },
    onLoad(options) {
        const { table_id, merchant_id, order_type = 'dine_in', reservation_id } = options;
        if (!merchant_id) {
            wx.showToast({
                title: '参数错误',
                icon: 'error'
            });
            wx.navigateBack();
            return;
        }
        // 预订场景需要 reservation_id，堂食场景需要 table_id
        if (order_type === 'reservation' && !reservation_id) {
            wx.showToast({ title: '缺少预订ID', icon: 'error' });
            wx.navigateBack();
            return;
        }
        if (order_type === 'dine_in' && !table_id) {
            wx.showToast({ title: '缺少桌台ID', icon: 'error' });
            wx.navigateBack();
            return;
        }
        this.setData({
            tableId: table_id ? parseInt(table_id) : 0,
            merchantId: parseInt(merchant_id),
            reservationId: reservation_id ? parseInt(reservation_id) : 0,
            orderType: order_type
        });
        this.initPage();
    },
    /**
     * 初始化页面数据
     */
    initPage() {
        return __awaiter(this, void 0, void 0, function* () {
            const { merchantId } = this.data;
            try {
                this.setData({ loading: true });
                // 加载购物车和计算结果
                const cart = yield (0, cart_1.getCart)(merchantId);
                const calculation = yield (0, cart_1.calculateCart)({ merchant_id: merchantId });
                if (!cart.items || cart.items.length === 0) {
                    wx.showModal({
                        title: '提示',
                        content: '购物车为空，请先选择菜品',
                        success: () => {
                            wx.navigateBack();
                        }
                    });
                    return;
                }
                this.setData({
                    cart,
                    calculation
                });
            }
            catch (error) {
                console.error('初始化页面失败:', error);
                wx.showToast({
                    title: error.message || '加载失败',
                    icon: 'error'
                });
            }
            finally {
                this.setData({ loading: false });
            }
        });
    },
    /**
     * 选择支付方式
     */
    selectPaymentMethod(e) {
        const methodId = e.currentTarget.dataset.id;
        this.setData({
            selectedPaymentMethod: methodId
        });
    },
    /**
     * 输入备注
     */
    onRemarkInput(e) {
        this.setData({
            remark: e.detail.value
        });
    },
    /**
     * 输入用餐人数
     */
    onGuestCountInput(e) {
        const guestCount = parseInt(e.detail.value) || 1;
        this.setData({
            'diningInfo.guest_count': Math.max(1, guestCount)
        });
    },
    /**
     * 输入特殊要求
     */
    onSpecialRequestsInput(e) {
        this.setData({
            'diningInfo.special_requests': e.detail.value
        });
    },
    /**
     * 提交订单
     */
    submitOrder() {
        return __awaiter(this, void 0, void 0, function* () {
            const { cart, calculation, tableId, merchantId, orderType, selectedPaymentMethod, remark, diningInfo, submitting } = this.data;
            if (submitting)
                return;
            if (!cart.items || cart.items.length === 0) {
                wx.showToast({
                    title: '购物车为空',
                    icon: 'error'
                });
                return;
            }
            try {
                this.setData({ submitting: true });
                // 创建订单 - 根据订单类型传递不同字段
                const orderData = {
                    merchant_id: merchantId,
                    order_type: orderType,
                    items: cart.items,
                    remark,
                    guest_count: diningInfo.guest_count,
                    special_requests: diningInfo.special_requests
                };
                // 堂食场景传 table_id，预订场景传 reservation_id
                if (orderType === 'dine_in') {
                    orderData.table_id = tableId;
                }
                else if (orderType === 'reservation') {
                    orderData.reservation_id = this.data.reservationId;
                }
                const order = yield (0, order_1.createOrder)(orderData);
                // 创建支付
                yield this.doCreatePayment(order.id, calculation.total_amount, selectedPaymentMethod);
            }
            catch (error) {
                console.error('提交订单失败:', error);
                wx.showToast({
                    title: error.message || '提交失败',
                    icon: 'error'
                });
            }
            finally {
                this.setData({ submitting: false });
            }
        });
    },
    /**
     * 创建支付
     */
    doCreatePayment(orderId, _amount, paymentMethod) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                if (paymentMethod === 'wechat_pay') {
                    // 创建支付订单
                    const paymentResult = yield (0, payment_1.createPayment)({
                        order_id: orderId,
                        payment_type: 'miniprogram',
                        business_type: 'order'
                    });
                    // 调起微信支付
                    if (paymentResult.pay_params) {
                        yield (0, payment_1.invokeWechatPay)(paymentResult.pay_params);
                        this.handlePaymentSuccess();
                    }
                    else {
                        throw new Error('支付参数缺失');
                    }
                }
                else if (paymentMethod === 'balance') {
                    // 余额支付通过创建订单时的 use_balance 参数处理
                    this.handlePaymentSuccess();
                }
                else {
                    throw new Error('不支持的支付方式');
                }
            }
            catch (error) {
                console.error('创建支付失败:', error);
                throw error;
            }
        });
    },
    /**
     * 支付成功处理
     */
    handlePaymentSuccess() {
        wx.showToast({ title: '支付成功', icon: 'success' });
        setTimeout(() => {
            wx.redirectTo({
                url: '/pages/orders/list/index?tab=dine_in'
            });
        }, 1500);
    },
    /**
     * 处理微信支付
     */
    handleWechatPay(paymentResult) {
        return __awaiter(this, void 0, void 0, function* () {
            const { payment_info } = paymentResult;
            if (payment_info === null || payment_info === void 0 ? void 0 : payment_info.jsapi_params) {
                // 调用微信支付
                wx.requestPayment(Object.assign(Object.assign({}, payment_info.jsapi_params), { success: () => {
                        this.onPaymentSuccess(paymentResult.payment);
                    }, fail: (error) => {
                        console.error('微信支付失败:', error);
                        wx.showToast({
                            title: '支付失败',
                            icon: 'error'
                        });
                    } }));
            }
            else {
                throw new Error('微信支付参数错误');
            }
        });
    },
    /**
     * 处理支付宝支付
     */
    handleAlipay(paymentResult) {
        return __awaiter(this, void 0, void 0, function* () {
            // 支付宝支付逻辑
            // 这里需要根据实际的支付宝SDK实现
            wx.showToast({
                title: '支付宝支付暂未开放',
                icon: 'none'
            });
        });
    },
    /**
     * 处理余额支付
     */
    handleBalancePay(paymentResult) {
        return __awaiter(this, void 0, void 0, function* () {
            // 余额支付通常是同步的
            if (paymentResult.payment.status === 'paid') {
                this.onPaymentSuccess(paymentResult.payment);
            }
            else {
                throw new Error('余额不足');
            }
        });
    },
    /**
     * 支付成功处理
     */
    onPaymentSuccess(payment) {
        const { calculation, tableInfo } = this.data;
        wx.showToast({
            title: '支付成功',
            icon: 'success'
        });
        // 跳转到支付成功页面
        setTimeout(() => {
            wx.redirectTo({
                url: `/pages/dine-in/payment-success/payment-success?order_id=${payment.order_id}&amount=${calculation === null || calculation === void 0 ? void 0 : calculation.total_amount}&merchant_name=${encodeURIComponent((tableInfo === null || tableInfo === void 0 ? void 0 : tableInfo.merchant_name) || '')}&table_number=${tableInfo === null || tableInfo === void 0 ? void 0 : tableInfo.table_number}`
            });
        }, 1500);
    },
    /**
     * 返回菜单
     */
    backToMenu() {
        wx.navigateBack();
    },
    /**
     * 查看订单详情
     */
    viewOrderDetail() {
        // 如果有正在处理的订单，跳转到订单详情
        wx.navigateTo({
            url: '/pages/order/list/list?type=dine_in'
        });
    }
});
