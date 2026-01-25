import { getCart, calculateCart } from '../../../api/cart'
import { getPublicMerchantDetail } from '../../../api/merchant'
import { getTableDetail } from '../../../api/table'
import { getReservationDetail } from '../../../api/reservation'
import { createOrder } from '../../../api/order'
import { createOrderPayment, invokeWechatPay } from '../../../api/payment'
import { getMyMemberships, MembershipResponse } from '../../../api/personal'
import { formatPriceNoSymbol } from '../../../utils/util'
import { getPublicImageUrl } from '../../../utils/image'

Page({
    data: {
        merchantId: 0,
        tableId: 0,
        reservationId: 0,
        orderType: 'dine_in' as 'dine_in' | 'reservation',
        
        loading: true,
        merchantInfo: null as any,
        tableInfo: null as any,
        reservationInfo: null as any,
        cart: null as any,
        calculation: {
            subtotal: 0,
            subtotalDisplay: '0.00',
            discount_amount: 0,
            discountDisplay: '0.00',
            total_amount: 0,
            totalDisplay: '0.00',
            applied_promotions: [] as any[]
        },
        
        remark: '',
        selectedPaymentMethod: 'wechat_pay',
        paymentMethods: [] as any[],
        memberBalance: 0,
        memberBalanceDisplay: '0.00',
        membershipId: 0,
        balanceInsufficient: false,
        
        diningInfo: {
            guest_count: 2
        }
    },

    async onLoad(options: any) {
        const merchantId = parseInt(options.merchant_id);
        const tableId = options.table_id ? parseInt(options.table_id) : 0;
        const reservationId = options.reservation_id ? parseInt(options.reservation_id) : 0;
        const orderType = options.order_type || (reservationId ? 'reservation' : 'dine_in');

        this.setData({ 
            merchantId, 
            tableId, 
            reservationId, 
            orderType 
        });

        await this.initData();
    },

    /**
     * 初始化数据（SSOT：一切以 calculateCart 为准）
     */
    async initData() {
        this.setData({ loading: true });
        const { merchantId, tableId, reservationId, orderType } = this.data;

        try {
            // 1. 获取基础信息
            const [merchantInfo, cart] = await Promise.all([
                getPublicMerchantDetail(merchantId),
                getCart({ merchant_id: merchantId, order_type: orderType, table_id: tableId || undefined, reservation_id: reservationId || undefined })
            ]);

            // 2. 获取计算结果 (后端自动应用最优优惠)
            const calculationResult = await calculateCart({
                merchant_id: merchantId,
                order_type: orderType,
                table_id: tableId || undefined,
                reservation_id: reservationId || undefined
            });

            // 3. 处理会员信息
            await this.loadMembershipInfo();

            // 4. 更新 UI 数据
            this.renderData(merchantInfo, cart, calculationResult);

        } catch (error) {
            console.error('初始化失败:', error);
            wx.showToast({ title: '加载失败', icon: 'error' });
        } finally {
            this.setData({ loading: false });
        }
    },

    async loadMembershipInfo() {
        try {
            const membershipsResult = await getMyMemberships();
            const membership = membershipsResult.memberships?.find(
                (m: MembershipResponse) => m.merchant_id === this.data.merchantId
            );
            if (membership) {
                const balance = membership.balance || 0;
                this.setData({
                    memberBalance: balance,
                    memberBalanceDisplay: formatPriceNoSymbol(balance),
                    membershipId: membership.id
                });
            }
        } catch (err) {
            console.warn('获取余额失败', err);
        }
    },

    renderData(merchantInfo: any, cart: any, calculation: any) {
        const processedCalculation = {
            ...calculation,
            subtotalDisplay: formatPriceNoSymbol(calculation.subtotal || 0),
            totalDisplay: formatPriceNoSymbol(calculation.total_amount || 0),
            applied_promotions: (calculation.applied_promotions || []).map((p: any) => ({
                ...p,
                amountDisplay: formatPriceNoSymbol(p.amount || 0)
            }))
        };

        const balanceInsufficient = this.data.memberBalance < calculation.total_amount;

        const paymentMethods = [
            { id: 'wechat_pay', name: '微信支付', icon: 'logo-wechat', disabled: false },
            { 
                id: 'balance', 
                name: `储值支付 (¥${this.data.memberBalanceDisplay})`, 
                icon: 'wallet', 
                disabled: this.data.memberBalance <= 0 
            }
        ];

        this.setData({
            merchantInfo: { ...merchantInfo, logo_url: getPublicImageUrl(merchantInfo.logo_url) },
            cart,
            calculation: processedCalculation,
            balanceInsufficient,
            paymentMethods,
            selectedPaymentMethod: balanceInsufficient ? 'wechat_pay' : this.data.selectedPaymentMethod
        });
    },

    onPaymentMethodChange(e: any) {
        this.setData({ selectedPaymentMethod: e.detail.value });
    },

    onRemarkChange(e: any) {
        this.setData({ remark: e.detail.value });
    },

    onRecharged() {
        this.initData(); // 重新初始化，自动刷新余额和计算结果
    },

    onVoucherClaimed() {
        this.initData(); // 重新初始化，自动应用新领的券
    },

    async onSubmit() {
        if (this.data.loading) return;
        this.setData({ loading: true });

        const { merchantId, orderType, tableId, reservationId, selectedPaymentMethod, remark } = this.data;

        try {
            const order = await createOrder({
                merchant_id: merchantId,
                order_type: orderType,
                table_id: tableId || undefined,
                reservation_id: reservationId || undefined,
                notes: remark,
                use_balance: selectedPaymentMethod === 'balance',
                items: [] // 购物车已经在后端，前端传空即可（符合后端逻辑）
            });

            await this.handlePayment(order.id);

        } catch (error: any) {
            wx.showToast({ title: error.message || '下单失败', icon: 'error' });
            this.setData({ loading: false });
        }
    },

    async handlePayment(orderId: number) {
        try {
            const payment = await createOrderPayment(orderId);
            if (payment.pay_params) {
                await invokeWechatPay(payment.pay_params);
            }
            wx.showToast({ title: '支付成功', icon: 'success' });
            setTimeout(() => {
                wx.redirectTo({ url: `/pages/orders/detail/index?id=${orderId}` });
            }, 1000);
        } catch (error) {
            console.error('支付失败', error);
            wx.redirectTo({ url: `/pages/orders/detail/index?id=${orderId}` });
        }
    }
});