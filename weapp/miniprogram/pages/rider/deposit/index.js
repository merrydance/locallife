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
Object.defineProperty(exports, "__esModule", { value: true });
const rider_1 = require("../../../api/rider");
const logger_1 = require("../../../utils/logger");
const util_1 = require("../../../utils/util");
Page({
    data: {
        deposit: 0,
        depositDisplay: '0.00',
        minDeposit: 50000, // 500元
        minDepositDisplay: '500.00',
        status: 'UNPAID', // UNPAID, PAID, REFUNDING
        transactions: [],
        loading: false,
        navBarHeight: 88
    },
    onLoad() {
        this.loadDepositInfo();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadDepositInfo() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const dashboard = yield (0, rider_1.getRiderDashboard)();
                const deposit = dashboard.deposit || { amount: 0, status: 'UNPAID' };
                this.setData({
                    deposit: deposit.amount,
                    depositDisplay: (0, util_1.formatPriceNoSymbol)(deposit.amount || 0),
                    status: deposit.status,
                    transactions: [], // Transaction history API missing
                    loading: false
                });
            }
            catch (error) {
                logger_1.logger.error('Load deposit failed', error, 'Deposit');
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    onPayDeposit() {
        wx.showModal({
            title: '缴纳押金',
            content: '确认支付500元押金?',
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                if (res.confirm) {
                    // TODO: Implement Pay API
                    wx.showToast({ title: '支付接口缺失', icon: 'none' });
                }
            })
        });
    },
    onRefundDeposit() {
        wx.showModal({
            title: '退还押金',
            content: '申请退还押金后将无法接单，确认申请?',
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                if (res.confirm) {
                    // TODO: Implement Refund API
                    wx.showToast({ title: '退款接口缺失', icon: 'none' });
                }
            })
        });
    }
});
