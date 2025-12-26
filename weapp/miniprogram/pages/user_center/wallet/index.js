"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Page({
    data: {
        balance: 0,
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
    loadWallet() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                // Mock data - GET /api/v1/customers/wallet
                const mockWallet = {
                    balance: 5800,
                    transactions: [
                        {
                            id: 'tx_1',
                            type: 'PAYMENT',
                            amount: -3800,
                            title: '外卖订单支付',
                            time: '2024-11-19 18:30'
                        },
                        {
                            id: 'tx_2',
                            type: 'REFUND',
                            amount: 1200,
                            title: '订单退款',
                            time: '2024-11-18 10:00'
                        },
                        {
                            id: 'tx_3',
                            type: 'TOPUP',
                            amount: 10000,
                            title: '余额充值',
                            time: '2024-11-15 09:00'
                        }
                    ]
                };
                this.setData({
                    balance: mockWallet.balance,
                    transactions: mockWallet.transactions,
                    loading: false
                });
            }
            catch (error) {
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    onTopUp() {
        wx.showToast({ title: '充值功能开发中', icon: 'none' });
    },
    onWithdraw() {
        wx.showToast({ title: '提现功能开发中', icon: 'none' });
    }
});
