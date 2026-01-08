"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
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
const CartAPI = __importStar(require("../../../api/cart"));
const logger_1 = require("../../../utils/logger");
const address_1 = __importDefault(require("../../../api/address"));
const order_1 = require("../../../api/order");
const util_1 = require("../../../utils/util");
const image_1 = require("../../../utils/image");
const request_1 = require("../../../utils/request");
Page({
    data: {
        cart: null,
        cartIds: [],
        address: null,
        remark: '',
        deliveryTime: 'ASAP',
        navBarHeight: 88,
        loading: false,
        orderTotalDisplay: '0.00',
        deliveryFee: 500, // 配送费（分）
        deliveryFeeDisplay: '5.00'
    },
    onLoad(options) {
        // 解析URL中的cart_ids参数
        if (options.cart_ids) {
            const cartIds = options.cart_ids.split(',').map(Number).filter(id => !isNaN(id));
            this.setData({ cartIds });
        }
        this.loadCart();
        this.loadDefaultAddress();
    },
    onShow() {
        // If returning from address selection, we might have a selectedAddressId
        const pages = getCurrentPages();
        const currPage = pages[pages.length - 1];
        if (currPage.data.selectedAddressId) {
            this.loadAddressById(currPage.data.selectedAddressId);
            currPage.setData({ selectedAddressId: null });
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadCart() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                this.setData({ loading: true });
                console.log('[Order-confirm] Loading takeout cart...');
                // Step 1: 获取外卖类型的购物车列表
                const userCarts = yield CartAPI.getUserCarts('takeout');
                console.log('[Order-confirm] userCarts:', JSON.stringify(userCarts));
                if (!userCarts.carts || userCarts.carts.length === 0) {
                    console.log('[Order-confirm] No carts found, navigating back');
                    wx.showToast({ title: '购物车为空', icon: 'none' });
                    setTimeout(() => wx.navigateBack(), 1500);
                    return;
                }
                // 如果有指定的cart_ids，只使用这些购物车
                const { cartIds } = this.data;
                let selectedCarts = userCarts.carts;
                if (cartIds.length > 0) {
                    selectedCarts = userCarts.carts.filter(c => cartIds.includes(c.cart_id || 0));
                }
                if (selectedCarts.length === 0) {
                    // 如果没有匹配的cart_ids，使用第一个购物车
                    selectedCarts = [userCarts.carts[0]];
                }
                // 目前只支持单商户结算
                const merchantCart = selectedCarts[0];
                const merchantId = merchantCart.merchant_id;
                if (!merchantId) {
                    wx.showToast({ title: '商户信息缺失', icon: 'none' });
                    setTimeout(() => wx.navigateBack(), 1500);
                    return;
                }
                // Step 2: 获取购物车商品详情
                const cartDetail = yield CartAPI.getCart({
                    merchant_id: merchantId,
                    order_type: 'takeout'
                });
                if (!cartDetail.items || cartDetail.items.length === 0) {
                    wx.showToast({ title: '购物车为空', icon: 'none' });
                    setTimeout(() => wx.navigateBack(), 1500);
                    return;
                }
                // 转换为页面数据格式
                const items = cartDetail.items.map((item) => ({
                    id: item.id,
                    dishId: item.dish_id,
                    comboId: item.combo_id,
                    name: item.name,
                    imageUrl: (0, image_1.getPublicImageUrl)(item.image_url || ''),
                    quantity: item.quantity,
                    unitPrice: item.unit_price,
                    priceDisplay: (0, util_1.formatPriceNoSymbol)(item.unit_price),
                    subtotal: item.subtotal,
                    subtotalDisplay: (0, util_1.formatPriceNoSymbol)(item.subtotal)
                }));
                const totalCount = items.reduce((sum, item) => sum + item.quantity, 0);
                const totalPrice = cartDetail.subtotal;
                const cart = {
                    items,
                    merchantId,
                    merchantName: merchantCart.merchant_name || '商家',
                    totalCount,
                    totalPrice,
                    totalPriceDisplay: (0, util_1.formatPriceNoSymbol)(totalPrice)
                };
                // 计算订单总价（商品金额 + 配送费）
                const { deliveryFee } = this.data;
                const orderTotal = totalPrice + deliveryFee;
                this.setData({
                    cart,
                    orderTotalDisplay: (0, util_1.formatPriceNoSymbol)(orderTotal),
                    loading: false
                });
            }
            catch (error) {
                logger_1.logger.error('Load cart failed', error, 'Order-confirm');
                wx.showToast({ title: '加载购物车失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    loadDefaultAddress() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const addresses = yield address_1.default.getAddresses();
                if (addresses && addresses.length > 0) {
                    const defaultAddr = addresses.find((a) => a.is_default) || addresses[0];
                    this.setData({ address: defaultAddr });
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
                const addresses = yield address_1.default.getAddresses();
                const addr = addresses.find((a) => String(a.id) === String(id));
                if (addr) {
                    this.setData({ address: addr });
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
    onSubmitOrder() {
        return __awaiter(this, void 0, void 0, function* () {
            const { cart, address, remark } = this.data;
            if (!address || !address.id) {
                wx.showToast({ title: '请选择收货地址', icon: 'none' });
                return;
            }
            if (!cart || cart.totalCount === 0) {
                wx.showToast({ title: '购物车为空', icon: 'none' });
                return;
            }
            if (!cart.merchantId) {
                wx.showToast({ title: '商户信息丢失', icon: 'none' });
                return;
            }
            this.setData({ loading: true });
            try {
                // Step 1: 创建订单
                const requestData = {
                    merchant_id: cart.merchantId,
                    items: cart.items.map((item) => {
                        const orderItem = {
                            quantity: item.quantity
                        };
                        if (item.dishId) {
                            orderItem.dish_id = item.dishId;
                        }
                        if (item.comboId) {
                            orderItem.combo_id = item.comboId;
                        }
                        return orderItem;
                    }),
                    order_type: 'takeout',
                    address_id: address.id,
                    notes: remark
                };
                const order = yield (0, order_1.createOrder)(requestData);
                console.log('[Order-confirm] Order created:', order.id);
                // Step 2: 创建支付订单
                try {
                    // 调用后端 /v1/payments API (对应 createPaymentOrder)
                    const paymentResult = yield (0, request_1.request)({
                        url: '/v1/payments',
                        method: 'POST',
                        data: {
                            order_id: order.id,
                            payment_type: 'miniprogram', // 小程序支付
                            business_type: 'order' // 订单支付
                        }
                    });
                    console.log('[Order-confirm] Payment created:', paymentResult);
                    // Step 3: 检查是否返回了支付参数 (后端返回 pay_params)
                    if (paymentResult.pay_params) {
                        // 调用微信支付
                        const params = paymentResult.pay_params;
                        wx.requestPayment({
                            timeStamp: params.timeStamp,
                            nonceStr: params.nonceStr,
                            package: params.package,
                            signType: (params.signType || 'RSA'),
                            paySign: params.paySign,
                            success: () => {
                                wx.showToast({ title: '支付成功', icon: 'success' });
                                setTimeout(() => {
                                    wx.redirectTo({ url: `/pages/orders/detail/index?id=${order.id}` });
                                }, 1500);
                            },
                            fail: (err) => {
                                console.log('[Order-confirm] Payment cancelled or failed:', err);
                                wx.showToast({ title: '支付取消', icon: 'none' });
                                // 支付取消/失败，跳转到订单详情（状态为待支付）
                                setTimeout(() => {
                                    wx.redirectTo({ url: `/pages/orders/detail/index?id=${order.id}` });
                                }, 1500);
                            }
                        });
                    }
                    else {
                        // 支付参数未返回（可能是后端未配置微信支付）
                        this.showPaymentDevModal(order.id);
                    }
                }
                catch (paymentError) {
                    console.error('[Order-confirm] Payment creation failed:', paymentError);
                    // 支付订单创建失败，提示开发中
                    this.showPaymentDevModal(order.id);
                }
            }
            catch (error) {
                logger_1.logger.error('Create order failed:', error, 'Order-confirm');
                wx.showToast({ title: '下单失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    showPaymentDevModal(orderId) {
        this.setData({ loading: false });
        wx.showModal({
            title: '支付功能开发中',
            content: '微信支付功能正在开发中，订单已创建成功。',
            showCancel: false,
            confirmText: '查看订单',
            success: () => {
                wx.redirectTo({ url: `/pages/orders/detail/index?id=${orderId}` });
            }
        });
    }
});
