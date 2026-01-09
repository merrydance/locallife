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
const responsive_1 = require("@/utils/responsive");
Page({
    data: {
        automations: [],
        isLargeScreen: false,
        navBarHeight: 88,
        loading: false
    },
    onLoad() {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
        this.loadAutomations();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadAutomations() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                // Mock data - GET /api/v1/operator/automations
                const mockAutomations = [
                    {
                        id: 'auto_1',
                        name: '超时自动取消订单',
                        desc: '顾客下单后15分钟未支付，自动取消订单',
                        status: 'ACTIVE',
                        trigger: 'ORDER_CREATED',
                        condition: 'UNPAID_DURATION > 15min',
                        action: 'CANCEL_ORDER'
                    },
                    {
                        id: 'auto_2',
                        name: '骑手超时自动惩罚',
                        desc: '骑手接单后超过预计送达时间30分钟未送达，自动扣除信用分',
                        status: 'INACTIVE',
                        trigger: 'ORDER_DELIVERING',
                        condition: 'DELAY > 30min',
                        action: 'DEDUCT_CREDIT'
                    }
                ];
                this.setData({
                    automations: mockAutomations,
                    loading: false
                });
            }
            catch (error) {
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    onToggleStatus(e) {
        const { id } = e.detail;
        wx.showToast({ title: '状态已更新', icon: 'success' });
        // Mock toggle
        const newList = this.data.automations.map((item) => {
            if (item.id === id) {
                return Object.assign(Object.assign({}, item), { status: item.status === 'ACTIVE' ? 'INACTIVE' : 'ACTIVE' });
            }
            return item;
        });
        this.setData({ automations: newList });
    }
});
