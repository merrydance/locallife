"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const cart_1 = __importDefault(require("@/services/cart"));
Page({
    data: {
        items: [],
        totalCount: 0,
        totalPrice: 0,
        totalPriceDisplay: '$0.00',
        navBarHeight: 88
    },
    onLoad() {
        this.loadCart();
    },
    onShow() {
        this.loadCart();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadCart() {
        const cart = cart_1.default.getCart();
        this.setData({
            items: cart.items,
            totalCount: cart.totalCount,
            totalPrice: cart.totalPrice,
            totalPriceDisplay: cart.totalPriceDisplay
        });
    },
    onIncrease(e) {
        const { dishId } = e.currentTarget.dataset;
        const item = this.data.items.find((i) => i.dishId === dishId);
        if (item) {
            cart_1.default.updateQuantity(dishId, item.quantity + 1);
            this.loadCart();
        }
    },
    onDecrease(e) {
        const { dishId } = e.currentTarget.dataset;
        const item = this.data.items.find((i) => i.dishId === dishId);
        if (item) {
            cart_1.default.updateQuantity(dishId, item.quantity - 1);
            this.loadCart();
        }
    },
    onClearAll() {
        wx.showModal({
            title: '清空购物车',
            content: '确定要清空购物车吗?',
            success: (res) => {
                if (res.confirm) {
                    cart_1.default.clear();
                    this.loadCart();
                }
            }
        });
    },
    onCheckout() {
        if (this.data.totalCount === 0) {
            wx.showToast({ title: '购物车为空', icon: 'none' });
            return;
        }
        wx.navigateTo({ url: '/pages/takeout/order-confirm/index' });
    },
    onBackToTakeout() {
        wx.navigateBack();
    }
});
