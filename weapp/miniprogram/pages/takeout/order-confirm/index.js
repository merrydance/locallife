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
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const cart_1 = __importDefault(require("../../../services/cart"));
const logger_1 = require("../../../utils/logger");
const address_1 = require("../../../api/address");
const order_1 = require("../../../api/order");
Page({
    data: {
        cart: null,
        address: null,
        remark: '',
        deliveryTime: 'ASAP',
        navBarHeight: 88,
        loading: false,
        previewData: null
    },
    onLoad() {
        this.loadCart();
        this.loadDefaultAddress();
    },
    onShow() {
        // If returning from address selection, we might have a selectedAddressId
        const pages = getCurrentPages();
        const currPage = pages[pages.length - 1];
        if (currPage.data.selectedAddressId) {
            this.loadAddressById(currPage.data.selectedAddressId);
            // clear it
            currPage.setData({ selectedAddressId: null });
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadCart() {
        const cart = cart_1.default.getCart();
        this.setData({ cart });
        this.updateOrderPreview();
    },
    loadDefaultAddress() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const addresses = yield (0, address_1.getAddressList)();
                if (addresses && addresses.length > 0) {
                    const defaultAddr = addresses.find((a) => a.is_default) || addresses[0];
                    this.setData({ address: defaultAddr });
                    this.updateOrderPreview();
                }
            }
            catch (error) {
                logger_1.logger.error('Load address failed', error, 'Order-confirm');
            }
        });
    },
    loadAddressById(id) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const addresses = yield (0, address_1.getAddressList)(); // Ideally use getAddressDetail or find from cached list
                const addr = addresses.find((a) => a.id === id);
                if (addr) {
                    this.setData({ address: addr });
                    this.updateOrderPreview();
                }
            }
            catch (error) {
                logger_1.logger.error('Load address failed', error, 'Order-confirm');
            }
        });
    },
    onSelectAddress() {
        wx.navigateTo({ url: '/pages/user_center/addresses/index?select=true' });
    },
    onRemarkInput(e) {
        this.setData({ remark: e.detail.value });
    },
    onDeliveryTimeChange(e) {
        this.setData({ deliveryTime: e.detail.value });
    },
    updateOrderPreview() {
        return __awaiter(this, void 0, void 0, function* () {
            const { cart, address } = this.data;
            const CartService = require('../../../services/cart').default;
            const merchantId = CartService.getMerchantId();
            if (!cart || cart.totalCount === 0 || !merchantId)
                return;
            const requestData = {
                merchant_id: merchantId,
                items: cart.items.map((item) => ({
                    dish_id: item.dishId,
                    quantity: item.quantity,
                    extra_options: []
                })),
                order_type: 'DELIVERY',
                address_id: address === null || address === void 0 ? void 0 : address.id
            };
            try {
                const preview = yield (0, order_1.previewOrder)(requestData);
                this.setData({ previewData: preview });
            }
            catch (e) {
                logger_1.logger.error('Preview failed', e, 'Order-confirm');
            }
        });
    },
    onSubmitOrder() {
        return __awaiter(this, void 0, void 0, function* () {
            const { cart, address, remark } = this.data;
            const CartService = require('../../../services/cart').default;
            const merchantId = CartService.getMerchantId();
            if (!address) {
                wx.showToast({ title: '请选择收货地址', icon: 'none' });
                return;
            }
            if (cart.totalCount === 0) {
                wx.showToast({ title: '购物车为空', icon: 'none' });
                return;
            }
            if (!merchantId) {
                wx.showToast({ title: '商户信息丢失', icon: 'none' });
                return;
            }
            this.setData({ loading: true });
            try {
                // Construct Request
                const requestData = {
                    merchant_id: merchantId,
                    items: cart.items.map((item) => ({
                        dish_id: item.dishId,
                        quantity: item.quantity,
                        extra_options: []
                    })),
                    order_type: 'DELIVERY',
                    address_id: address.id,
                    comment: remark
                };
                const order = yield (0, order_1.createOrder)(requestData);
                wx.showToast({ title: '下单成功', icon: 'success' });
                // 清空购物车
                CartService.clear();
                // 跳转到订单详情
                setTimeout(() => {
                    wx.redirectTo({ url: `/pages/orders/detail/index?id=${order.id}` });
                }, 1500);
            }
            catch (error) {
                logger_1.logger.error('Create order failed:', error, 'Order-confirm');
                wx.showToast({ title: '下单失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    }
});
