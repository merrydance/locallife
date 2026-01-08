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
const table_1 = require("../../../api/table");
const reservation_1 = require("../../../api/reservation");
const coupon_1 = require("../../../api/coupon");
const personal_1 = require("../../../api/personal");
const util_1 = require("../../../utils/util");
const image_1 = require("../../../utils/image");
const responsive_1 = require("../../../utils/responsive");
const merchant_1 = require("../../../api/merchant");
Page({
    data: {
        tableId: 0,
        merchantId: 0,
        reservationId: 0, // 预订点菜场景
        orderType: 'dine_in',
        navBarHeight: 64,
        // 订单数据
        cart: null,
        calculation: null,
        tableInfo: null,
        merchantInfo: null,
        reservationInfo: null,
        // 支付方式
        paymentMethods: [
            { id: 'wechat_pay', name: '微信支付', icon: 'logo-wechat' },
            { id: 'balance', name: '储值支付', icon: 'wallet', disabled: true }
        ],
        selectedPaymentMethod: 'wechat_pay',
        memberBalance: 0, // 用户在该商户的储值余额（分）
        memberBalanceDisplay: '0.00', // 格式化后的余额
        // 优惠券
        vouchers: [],
        selectedVoucher: null,
        voucherVisible: false,
        voucherLoading: false, // 优惠券加载状态（不影响主页面）
        // 界面状态
        loading: true,
        submitting: false,
        // 备注信息
        remark: '',
        // 用餐信息
        diningInfo: {
            guest_count: 2 // 默认2人
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
        const { navBarHeight } = (0, responsive_1.getStableBarHeights)();
        this.setData({
            tableId: table_id ? parseInt(table_id) : 0,
            merchantId: parseInt(merchant_id),
            reservationId: reservation_id ? parseInt(reservation_id) : 0,
            orderType: order_type,
            navBarHeight
        });
        this.initPage();
    },
    /**
     * 初始化页面数据
     */
    initPage() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            const { merchantId, orderType, tableId, reservationId } = this.data;
            try {
                this.setData({ loading: true });
                const params = {
                    merchant_id: merchantId,
                    order_type: orderType,
                    table_id: tableId || 0,
                    reservation_id: reservationId || 0
                };
                // 并行加载基本信息
                const [cart, calculation] = yield Promise.all([
                    (0, cart_1.getCart)(params),
                    (0, cart_1.calculateCart)(params)
                ]);
                if (!cart.items || cart.items.length === 0) {
                    wx.showModal({
                        title: '提示',
                        content: '购物车为空，请先选择菜品',
                        showCancel: false,
                        success: () => wx.navigateBack()
                    });
                    return;
                }
                // 获取商户信息
                const merchantInfo = yield (0, merchant_1.getPublicMerchantDetail)(merchantId);
                // 获取桌台/预约信息
                let tableNo = '';
                let reservationInfo = null;
                if (this.data.orderType === 'dine_in' && this.data.tableId) {
                    const tableResult = yield (0, table_1.getTableDetail)(this.data.tableId);
                    tableNo = tableResult.table_no;
                }
                else if (this.data.orderType === 'reservation' && this.data.reservationId) {
                    reservationInfo = yield (0, reservation_1.getReservationDetail)(this.data.reservationId);
                    tableNo = reservationInfo.table_no || '';
                }
                // 预处理购物车数据
                const processedItems = (cart.items || []).map(item => (Object.assign(Object.assign({}, item), { dish_image: (0, image_1.getPublicImageUrl)(item.image_url), priceDisplay: (0, util_1.formatPriceNoSymbol)(item.unit_price || 0), subtotalDisplay: (0, util_1.formatPriceNoSymbol)(item.subtotal || 0) })));
                const processedCalculation = Object.assign(Object.assign({}, calculation), { subtotalDisplay: (0, util_1.formatPriceNoSymbol)(calculation.subtotal || 0), discountDisplay: (0, util_1.formatPriceNoSymbol)(calculation.discount_amount || 0), totalDisplay: (0, util_1.formatPriceNoSymbol)(calculation.total_amount || 0) });
                // 预处理商户信息
                const processedMerchant = Object.assign(Object.assign({}, merchantInfo), { logo_url: (0, image_1.getPublicImageUrl)(merchantInfo.logo_url) });
                // 获取用户在该商户的储值余额
                let memberBalance = 0;
                let memberBalanceDisplay = '0.00';
                try {
                    const membershipsResult = yield (0, personal_1.getMyMemberships)();
                    const membership = (_a = membershipsResult.memberships) === null || _a === void 0 ? void 0 : _a.find((m) => m.merchant_id === merchantId);
                    if (membership) {
                        memberBalance = membership.balance || 0;
                        memberBalanceDisplay = (0, util_1.formatPriceNoSymbol)(memberBalance);
                    }
                }
                catch (err) {
                    console.warn('获取会员余额失败:', err);
                }
                // 更新支付方式，添加余额显示
                const paymentMethods = [
                    { id: 'wechat_pay', name: '微信支付', icon: 'logo-wechat', disabled: false },
                    {
                        id: 'balance',
                        name: `储值支付 (¥${memberBalanceDisplay})`,
                        icon: 'wallet',
                        disabled: memberBalance <= 0
                    }
                ];
                // 设置就餐人数：优先使用预订信息中的人数
                const guestCount = (reservationInfo === null || reservationInfo === void 0 ? void 0 : reservationInfo.guest_count) || 2;
                this.setData({
                    cart: Object.assign(Object.assign({}, cart), { items: processedItems }),
                    calculation: processedCalculation,
                    tableInfo: { table_number: tableNo },
                    merchantInfo: processedMerchant,
                    reservationInfo,
                    memberBalance,
                    memberBalanceDisplay,
                    paymentMethods,
                    'diningInfo.guest_count': guestCount
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
     * 支付方式变化 (t-radio-group)
     */
    onPaymentMethodChange(e) {
        this.setData({
            selectedPaymentMethod: e.detail.value
        });
    },
    /**
     * 显示优惠券选择
     */
    onShowVouchers() {
        return __awaiter(this, void 0, void 0, function* () {
            const { merchantId, calculation } = this.data;
            // 检查订单金额是否足够使用优惠券
            if (!calculation || calculation.subtotal <= 0) {
                wx.showToast({ title: '购物车为空', icon: 'none' });
                return;
            }
            // 先打开弹窗，再加载数据（避免页面刷新）
            this.setData({
                voucherVisible: true,
                voucherLoading: true
            });
            try {
                const result = yield coupon_1.CouponService.getMyCoupons({
                    status: 'available',
                    page_id: 1,
                    page_size: 50
                });
                const coupons = (result === null || result === void 0 ? void 0 : result.coupons) || [];
                // 过滤该商户可用的优惠券
                const availableVouchers = coupons.filter((c) => c.merchant_id === merchantId || c.merchant_id === 0 // 0 表示通用券
                );
                this.setData({
                    vouchers: availableVouchers,
                    voucherLoading: false
                });
            }
            catch (error) {
                console.error('加载优惠券失败:', error);
                this.setData({
                    vouchers: [],
                    voucherLoading: false
                });
            }
        });
    },
    /**
     * 优惠券弹窗状态变化
     */
    onVoucherPopupChange(e) {
        if (!e.detail.visible) {
            this.setData({ voucherVisible: false });
        }
    },
    closeVoucherPopup() {
        this.setData({ voucherVisible: false });
    },
    /**
     * 取消使用优惠券
     */
    onClearVoucher() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({
                selectedVoucher: null,
                voucherVisible: false,
                loading: true
            });
            try {
                const params = {
                    merchant_id: this.data.merchantId,
                    order_type: this.data.orderType,
                    table_id: this.data.tableId || 0,
                    reservation_id: this.data.reservationId || 0
                };
                const calculation = yield (0, cart_1.calculateCart)(params);
                this.setData({
                    calculation: Object.assign(Object.assign({}, calculation), { subtotalDisplay: (0, util_1.formatPriceNoSymbol)(calculation.subtotal || 0), discountDisplay: (0, util_1.formatPriceNoSymbol)(calculation.discount_amount || 0), totalDisplay: (0, util_1.formatPriceNoSymbol)(calculation.total_amount || 0) })
                });
            }
            catch (error) {
                console.error('重新计算金额失败:', error);
            }
            finally {
                this.setData({ loading: false });
            }
        });
    },
    onSelectVoucher(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const voucher = e.currentTarget.dataset.voucher;
            if (!voucher)
                return;
            this.setData({
                selectedVoucher: voucher,
                voucherVisible: false,
                loading: true
            });
            try {
                // 重新计算金额
                const params = {
                    merchant_id: this.data.merchantId,
                    order_type: this.data.orderType,
                    table_id: this.data.tableId || 0,
                    reservation_id: this.data.reservationId || 0,
                    voucher_id: voucher.id
                };
                const calculation = yield (0, cart_1.calculateCart)(params);
                this.setData({
                    calculation: Object.assign(Object.assign({}, calculation), { subtotalDisplay: (0, util_1.formatPriceNoSymbol)(calculation.subtotal || 0), discountDisplay: (0, util_1.formatPriceNoSymbol)(calculation.discount_amount || 0), totalDisplay: (0, util_1.formatPriceNoSymbol)(calculation.total_amount || 0) })
                });
            }
            catch (error) {
                console.error('计算优惠失败:', error);
            }
            finally {
                this.setData({ loading: false });
            }
        });
    },
    /**
     * 备注输入 (t-textarea)
     */
    onRemarkChange(e) {
        this.setData({
            remark: e.detail.value
        });
    },
    /**
     * 用餐人数变化 (t-stepper)
     */
    onGuestCountChange(e) {
        this.setData({
            'diningInfo.guest_count': e.detail.value
        });
    },
    /**
     * 提交订单
     */
    onSubmitOrder() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
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
                // 创建订单
                const orderData = {
                    merchant_id: merchantId,
                    order_type: orderType,
                    items: cart.items.map((item) => ({
                        dish_id: item.dish_id,
                        combo_id: item.combo_id,
                        quantity: item.quantity,
                        customizations: item.customizations
                    })),
                    remark,
                    notes: remark, // 兼容后端备注字段
                    guest_count: diningInfo.guest_count,
                    user_voucher_id: (_a = this.data.selectedVoucher) === null || _a === void 0 ? void 0 : _a.id,
                    use_balance: selectedPaymentMethod === 'balance'
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
