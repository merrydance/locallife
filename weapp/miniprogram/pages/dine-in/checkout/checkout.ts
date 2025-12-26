/**
 * 堂食结算页面
 * 处理堂食订单的结算和支付流程
 */

import {
    getCart,
    calculateCart,
    createOrder
} from '../../../api/customer-cart-order';
import {
    createWechatPayment,
    createAlipayPayment,
    createBalancePayment
} from '../../../api/payment-refund';
import { getTableInfo } from '../../../api/customer-reservation';

interface OrderCalculation {
    subtotal: number;
    delivery_fee: number;
    service_fee: number;
    discount_amount: number;
    total_amount: number;
    items: any[];
}

Page({
    data: {
        tableId: 0,
        merchantId: 0,
        orderType: 'dine_in',

        // 订单数据
        cart: null as any,
        calculation: null as OrderCalculation | null,
        tableInfo: null as any,

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

    onLoad(options: any) {
        const { table_id, merchant_id, order_type = 'dine_in' } = options;

        if (!table_id || !merchant_id) {
            wx.showToast({
                title: '参数错误',
                icon: 'error'
            });
            wx.navigateBack();
            return;
        }

        this.setData({
            tableId: parseInt(table_id),
            merchantId: parseInt(merchant_id),
            orderType: order_type
        });

        this.initPage();
    },

    /**
     * 初始化页面数据
     */
    async initPage() {
        try {
            this.setData({ loading: true });

            // 并行加载数据
            const [cart, calculation, tableInfo] = await Promise.all([
                getCart(),
                calculateCart(),
                getTableInfo(this.data.tableId)
            ]);

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
                calculation,
                tableInfo
            });

        } catch (error: any) {
            console.error('初始化页面失败:', error);
            wx.showToast({
                title: error.message || '加载失败',
                icon: 'error'
            });
        } finally {
            this.setData({ loading: false });
        }
    },

    /**
     * 选择支付方式
     */
    selectPaymentMethod(e: any) {
        const methodId = e.currentTarget.dataset.id;
        this.setData({
            selectedPaymentMethod: methodId
        });
    },

    /**
     * 输入备注
     */
    onRemarkInput(e: any) {
        this.setData({
            remark: e.detail.value
        });
    },

    /**
     * 输入用餐人数
     */
    onGuestCountInput(e: any) {
        const guestCount = parseInt(e.detail.value) || 1;
        this.setData({
            'diningInfo.guest_count': Math.max(1, guestCount)
        });
    },

    /**
     * 输入特殊要求
     */
    onSpecialRequestsInput(e: any) {
        this.setData({
            'diningInfo.special_requests': e.detail.value
        });
    },

    /**
     * 提交订单
     */
    async submitOrder() {
        const {
            cart,
            calculation,
            tableId,
            merchantId,
            orderType,
            selectedPaymentMethod,
            remark,
            diningInfo,
            submitting
        } = this.data;

        if (submitting) return;

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
                table_id: tableId,
                items: cart.items,
                remark,
                guest_count: diningInfo.guest_count,
                special_requests: diningInfo.special_requests
            };

            const order = await createOrder(orderData);

            // 创建支付
            await this.createPayment(order.id, calculation!.total_amount, selectedPaymentMethod);

        } catch (error: any) {
            console.error('提交订单失败:', error);
            wx.showToast({
                title: error.message || '提交失败',
                icon: 'error'
            });
        } finally {
            this.setData({ submitting: false });
        }
    },

    /**
     * 创建支付
     */
    async createPayment(orderId: number, amount: number, paymentMethod: string) {
        try {
            let paymentResult;
            const description = `堂食订单 ${orderId}`;

            switch (paymentMethod) {
                case 'wechat_pay':
                    paymentResult = await createWechatPayment(orderId, amount, description);
                    await this.handleWechatPay(paymentResult);
                    break;

                case 'alipay':
                    paymentResult = await createAlipayPayment(orderId, amount, description);
                    await this.handleAlipay(paymentResult);
                    break;

                case 'balance':
                    paymentResult = await createBalancePayment(orderId, amount, description);
                    await this.handleBalancePay(paymentResult);
                    break;

                default:
                    throw new Error('不支持的支付方式');
            }

        } catch (error: any) {
            console.error('创建支付失败:', error);
            throw error;
        }
    },

    /**
     * 处理微信支付
     */
    async handleWechatPay(paymentResult: any) {
        const { payment_info } = paymentResult;

        if (payment_info?.jsapi_params) {
            // 调用微信支付
            wx.requestPayment({
                ...payment_info.jsapi_params,
                success: () => {
                    this.onPaymentSuccess(paymentResult.payment);
                },
                fail: (error) => {
                    console.error('微信支付失败:', error);
                    wx.showToast({
                        title: '支付失败',
                        icon: 'error'
                    });
                }
            });
        } else {
            throw new Error('微信支付参数错误');
        }
    },

    /**
     * 处理支付宝支付
     */
    async handleAlipay(paymentResult: any) {
        // 支付宝支付逻辑
        // 这里需要根据实际的支付宝SDK实现
        wx.showToast({
            title: '支付宝支付暂未开放',
            icon: 'none'
        });
    },

    /**
     * 处理余额支付
     */
    async handleBalancePay(paymentResult: any) {
        // 余额支付通常是同步的
        if (paymentResult.payment.status === 'paid') {
            this.onPaymentSuccess(paymentResult.payment);
        } else {
            throw new Error('余额不足');
        }
    },

    /**
     * 支付成功处理
     */
    onPaymentSuccess(payment: any) {
        const { calculation, tableInfo } = this.data;

        wx.showToast({
            title: '支付成功',
            icon: 'success'
        });

        // 跳转到支付成功页面
        setTimeout(() => {
            wx.redirectTo({
                url: `/pages/dine-in/payment-success/payment-success?order_id=${payment.order_id}&amount=${calculation?.total_amount}&merchant_name=${encodeURIComponent(tableInfo?.merchant_name || '')}&table_number=${tableInfo?.table_number}`
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