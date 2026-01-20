"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const util_1 = require("../../../utils/util");
const payment_refund_1 = require("../../../api/payment-refund");
const personal_1 = require("../../../api/personal");
Page({
    data: {
        balance: 0,
        balanceDisplay: '0.00',
        transactions: [],
        loading: false,
        navBarHeight: 88
    },
    onLoad() {
        this.loadWallet();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    async loadWallet() {
        this.setData({ loading: true });
        try {
            const [memberships, payments] = await Promise.all([
                (0, personal_1.getMyMemberships)(),
                (0, payment_refund_1.getPayments)({ page: 1, page_size: 20 })
            ]);
            const balance = memberships.memberships.reduce((sum, m) => sum + (m.balance || 0), 0);
            const processedTransactions = (payments.payment_orders || []).map((payment) => {
                const isRefund = payment.status === 'refunded';
                const signedAmount = isRefund ? payment.amount : -payment.amount;
                const title = payment.business_type === 'reservation'
                    ? (isRefund ? '预订退款' : '预订支付')
                    : (isRefund ? '订单退款' : '订单支付');
                return {
                    id: String(payment.id),
                    type: isRefund ? 'REFUND' : 'PAYMENT',
                    amount: signedAmount,
                    amountDisplay: (signedAmount > 0 ? '+' : '') + (0, util_1.formatPriceNoSymbol)(Math.abs(signedAmount)),
                    title,
                    time: payment.paid_at || payment.created_at
                };
            });
            this.setData({
                balance,
                balanceDisplay: (0, util_1.formatPriceNoSymbol)(balance),
                transactions: processedTransactions,
                loading: false
            });
        }
        catch (error) {
            wx.showToast({ title: '加载失败', icon: 'error' });
            this.setData({ loading: false });
        }
    },
    onTopUp() {
        wx.showToast({ title: '充值功能开发中', icon: 'none' });
    },
    onWithdraw() {
        wx.showToast({ title: '提现功能开发中', icon: 'none' });
    }
});
