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
const order_1 = require("../../../api/order");
const logger_1 = require("../../../utils/logger");
const order_card_1 = require("../../../adapters/order-card");
const cart_1 = __importDefault(require("../../../services/cart"));
const order_2 = require("../../../adapters/order");
// 不同订单类型的状态筛选选项
const STATUS_TABS_MAP = {
    takeout: [
        { label: "全部", value: "" },
        { label: "待支付", value: "pending" },
        { label: "待接单", value: "paid" },
        { label: "制作中", value: "preparing" },
        { label: "已接单", value: "courier_accepted" },
        { label: "已取餐", value: "picked" },
        { label: "配送中", value: "delivering" },
        { label: "待确认", value: "rider_delivered" },
        { label: "已送达", value: "user_delivered" },
        { label: "已完成", value: "completed" },
        { label: "已取消", value: "cancelled" },
    ],
    dine_in: [
        { label: "全部", value: "" },
        { label: "待支付", value: "pending" },
        { label: "待确认", value: "paid" },
        { label: "制作中", value: "preparing" },
        { label: "已完成", value: "completed" },
        { label: "已取消", value: "cancelled" },
    ],
    reservation: [
        { label: "全部", value: "" },
        { label: "待支付", value: "pending" },
        { label: "待确认", value: "paid" },
        { label: "制作中", value: "preparing" },
        { label: "已完成", value: "completed" },
        { label: "已取消", value: "cancelled" },
    ],
    takeaway: [
        { label: "全部", value: "" },
        { label: "待支付", value: "pending" },
        { label: "待接单", value: "paid" },
        { label: "制作中", value: "preparing" },
        { label: "已取餐", value: "picked" },
        { label: "已完成", value: "completed" },
        { label: "已送达", value: "user_delivered" },
        { label: "已取消", value: "cancelled" },
    ],
    default: [
        { label: "全部", value: "" },
        { label: "待支付", value: "pending" },
        { label: "已完成", value: "completed" },
        { label: "已取消", value: "cancelled" },
    ],
};
// 取消原因选项
const CANCEL_REASONS = [
    "不想要了",
    "信息填写错误",
    "商品价格较贵",
    "配送时间太长",
    "其他原因",
];
Page({
    data: {
        orders: [],
        navBarHeight: 88,
        loading: false,
        page: 1,
        pageSize: 10,
        hasMore: true,
        statusTabs: STATUS_TABS_MAP.default,
        currentStatus: "",
        orderType: "",
        pageTitle: "我的订单",
    },
    onLoad(options) {
        const orderType = (options === null || options === void 0 ? void 0 : options.order_type) || "";
        const titleMap = {
            takeout: "外卖订单",
            reservation: "预订订单",
            dine_in: "堂食订单",
        };
        this.setData({
            orderType: orderType,
            pageTitle: titleMap[orderType] || "我的订单",
            statusTabs: STATUS_TABS_MAP[orderType] || STATUS_TABS_MAP.default,
        });
        this.loadOrders(true);
    },
    onShow() {
        // 返回时刷新列表
        if (this.data.orders.length > 0) {
            this.loadOrders(true);
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    onReachBottom() {
        if (this.data.hasMore && !this.data.loading) {
            this.setData({ page: this.data.page + 1 });
            this.loadOrders(false);
        }
    },
    loadOrders() {
        return __awaiter(this, arguments, void 0, function* (reset = false) {
            if (this.data.loading)
                return;
            this.setData({ loading: true });
            if (reset) {
                this.setData({ page: 1, orders: [], hasMore: true });
            }
            try {
                const { currentStatus, page, pageSize, orderType } = this.data;
                // API Call with status filter
                const params = currentStatus
                    ? {
                        status: currentStatus,
                        page_id: page,
                        page_size: pageSize,
                        order_type: orderType || undefined,
                    }
                    : {
                        page_id: page,
                        page_size: pageSize,
                        order_type: orderType || undefined,
                    };
                const result = yield (0, order_1.getOrders)(params);
                // 兼容不同返回结构：数组 / {orders} / {list} / {items} / {data: {...}}
                const unwrap = (payload) => {
                    if (Array.isArray(payload))
                        return payload;
                    if (payload && typeof payload === 'object') {
                        if (Array.isArray(payload.orders))
                            return payload.orders;
                        if (Array.isArray(payload.list))
                            return payload.list;
                        if (Array.isArray(payload.items))
                            return payload.items;
                        if (payload.data)
                            return unwrap(payload.data);
                    }
                    return [];
                };
                const orderDTOsRaw = unwrap(result);
                // 过滤掉空值或非对象；并在 map 阶段做单条 try/catch，避免坏数据导致整页崩溃
                const orderDTOs = orderDTOsRaw
                    .filter(item => item && typeof item === 'object')
                    .map((item) => {
                    try {
                        return order_card_1.OrderCardAdapter.toCardViewModel(item);
                    }
                    catch (err) {
                        logger_1.logger.error('Order map failed:', err, item);
                        return null;
                    }
                })
                    .filter(Boolean);
                // Sort by priority (preparing > delivering > completed)
                const sortedOrders = order_card_1.OrderCardAdapter.sortByPriority(orderDTOs);
                const orders = reset
                    ? sortedOrders
                    : [...this.data.orders, ...sortedOrders];
                this.setData({
                    orders,
                    hasMore: orderDTOs.length >= pageSize,
                });
            }
            catch (error) {
                logger_1.logger.error("Load orders failed:", error, "List");
                wx.showToast({ title: "加载失败", icon: "error" });
            }
            finally {
                this.setData({ loading: false });
            }
        });
    },
    // 状态筛选切换
    onStatusChange(e) {
        const status = e.detail.value || "";
        if (status === this.data.currentStatus)
            return;
        this.setData({ currentStatus: status });
        this.loadOrders(true);
    },
    onViewOrder(e) {
        const { id } = e.currentTarget.dataset;
        wx.navigateTo({ url: `/pages/orders/detail/index?id=${id}` });
    },
    // 快速取消订单
    onCancelOrder(e) {
        const { id } = e.currentTarget.dataset;
        if (!id)
            return;
        wx.showActionSheet({
            itemList: CANCEL_REASONS,
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                const reason = CANCEL_REASONS[res.tapIndex];
                yield this.doCancelOrder(Number(id), reason);
            }),
        });
    },
    doCancelOrder(orderId, reason) {
        return __awaiter(this, void 0, void 0, function* () {
            wx.showLoading({ title: "取消中..." });
            try {
                yield (0, order_1.cancelOrder)(orderId, { reason });
                wx.hideLoading();
                wx.showToast({ title: "已取消", icon: "success" });
                setTimeout(() => this.loadOrders(true), 1500);
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error("取消订单失败", error, "List.doCancelOrder");
                wx.showToast({ title: "取消失败", icon: "error" });
            }
        });
    },
    // 去支付
    onPayOrder(e) {
        const { id } = e.currentTarget.dataset;
        if (!id) {
            wx.showToast({ title: "订单信息缺失", icon: "none" });
            return;
        }
        wx.navigateTo({
            url: `/pages/user_center/payment-detail/index?orderId=${id}`,
        });
    },
    onReorder(e) {
        const { id } = e.currentTarget.dataset;
        const orderId = Number(id);
        if (!orderId) {
            wx.showToast({ title: "订单信息缺失", icon: "none" });
            return;
        }
        wx.showLoading({ title: "再次购买中..." });
        (() => __awaiter(this, void 0, void 0, function* () {
            try {
                const orderDTO = yield (0, order_1.getOrderDetail)(orderId);
                const orderDetail = order_2.OrderAdapter.toDetailViewModel(orderDTO);
                const orderType = orderDetail.type || 'takeout';
                const cartContext = { orderType };
                // 根据订单类型只传递对应的上下文，避免不相关字段干扰
                if (orderType === 'dine_in' && orderDetail.tableId) {
                    cartContext.tableId = orderDetail.tableId;
                }
                if (orderType === 'reservation' && orderDetail.reservationId) {
                    cartContext.reservationId = orderDetail.reservationId;
                }
                yield cart_1.default.loadCart(orderDetail.merchantId, cartContext);
                // 直接累加到当前购物车，避免覆盖已有商品
                const addResults = yield Promise.all(orderDetail.items.map((item) => cart_1.default.addItem({
                    merchantId: orderDetail.merchantId,
                    dishId: item.dishId,
                    comboId: item.comboId,
                    quantity: item.quantity,
                })));
                if (addResults.some((ok) => !ok)) {
                    wx.hideLoading();
                    wx.showToast({ title: "部分商品添加失败", icon: "none" });
                    return;
                }
                wx.hideLoading();
                wx.showToast({ title: "已加入购物车", icon: "success" });
                setTimeout(() => {
                    wx.navigateTo({ url: "/pages/takeout/cart/index" });
                }, 300);
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error("再次购买失败", error, "List.onReorder");
                wx.showToast({ title: "操作失败", icon: "error" });
            }
        }))();
    },
});
