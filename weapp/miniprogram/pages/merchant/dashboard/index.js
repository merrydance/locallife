"use strict";
/**
 * 商户工作台 v4.0 - 全屏沉浸式设计
 * 简化版：专注当日经营，三栏布局，WebSocket实时更新
 */
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
const merchant_1 = require("../../../api/merchant");
const merchant_table_device_management_1 = require("../../../api/merchant-table-device-management");
const merchant_analytics_1 = require("../../../api/merchant-analytics");
const order_management_1 = require("../../../api/order-management");
const websocket_realtime_1 = require("../../../api/websocket-realtime");
const logger_1 = require("../../../utils/logger");
const app = getApp();
Page({
    data: {
        // 商户信息
        merchantName: '',
        isOpen: false,
        currentDate: '',
        // WebSocket 状态
        wsConnected: false,
        // 统计数据
        stats: {
            todayRevenue: 0,
            todayOrders: 0
        },
        revenueDisplay: '0.00',
        // 订单标签
        orderTab: 'all',
        // 状态计数
        statusCounts: {
            paid: 0,
            preparing: 0,
            ready: 0
        },
        // 订单数据
        pendingOrders: [],
        filteredOrders: [],
        // 桌台数据
        tableGroups: [],
        tableStats: {
            total: 0,
            available: 0,
            occupied: 0
        },
        // 桌台弹窗
        showTablePopup: false,
        activeTable: null
    },
    onLoad() {
        this.updateDate();
        this.loadData();
    },
    onShow() {
        if (this.data.merchantName) {
            this.loadOrders();
            this.loadTables();
        }
    },
    onHide() {
        // 页面隐藏时保持连接
    },
    onUnload() {
        // 页面卸载时断开 WebSocket
        websocket_realtime_1.WebSocketUtils.closeAll();
    },
    updateDate() {
        const now = new Date();
        const weekDays = ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六'];
        const dateStr = `${now.getFullYear()}年${now.getMonth() + 1}月${now.getDate()}日 ${weekDays[now.getDay()]}`;
        this.setData({ currentDate: dateStr });
    },
    loadData() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                yield this.loadMerchantInfo();
                yield Promise.all([
                    this.loadStats(),
                    this.loadOrders(),
                    this.loadTables()
                ]);
                // 营业中时连接 WebSocket
                if (this.data.isOpen) {
                    this.connectWebSocket();
                }
            }
            catch (error) {
                logger_1.logger.error('加载数据失败', error, 'Dashboard');
                wx.showToast({ title: '加载失败', icon: 'none' });
            }
        });
    },
    loadMerchantInfo() {
        return __awaiter(this, void 0, void 0, function* () {
            const info = yield merchant_1.MerchantManagementService.getMerchantInfo();
            if (info) {
                this.setData({
                    merchantName: info.name,
                    isOpen: info.is_open
                });
                app.globalData.merchantId = String(info.id);
                app.globalData.userRole = 'merchant';
            }
        });
    },
    loadStats() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const today = new Date().toISOString().split('T')[0];
                const stats = yield merchant_analytics_1.MerchantStatsService.getStatsOverview({
                    start_date: today,
                    end_date: today
                });
                if (stats) {
                    const revenue = stats.total_revenue || 0;
                    this.setData({
                        stats: {
                            todayRevenue: revenue,
                            todayOrders: stats.total_orders || 0
                        },
                        revenueDisplay: (revenue / 100).toFixed(2)
                    });
                }
            }
            catch (error) {
                logger_1.logger.error('加载统计失败', error, 'Dashboard');
            }
        });
    },
    loadOrders() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const orders = yield order_management_1.MerchantOrderManagementService.getOrderList({
                    page_id: 1,
                    page_size: 50
                });
                // 订单类型和状态映射
                const typeMap = {
                    'takeout': '外卖',
                    'dine_in': '堂食',
                    'takeaway': '自取',
                    'reservation': '预订'
                };
                const statusMap = {
                    'paid': '待接单',
                    'preparing': '制作中',
                    'ready': '待取餐'
                };
                // 过滤和格式化
                const pendingOrders = (orders || [])
                    .filter((o) => ['paid', 'preparing', 'ready'].includes(o.status))
                    .map((o) => {
                    var _a;
                    let createdTime = '';
                    if (o.created_at) {
                        const d = new Date(o.created_at);
                        const h = d.getHours();
                        const m = d.getMinutes();
                        createdTime = (h < 10 ? '0' : '') + h + ':' + (m < 10 ? '0' : '') + m;
                    }
                    return {
                        id: o.id,
                        order_no: o.order_no,
                        status: o.status,
                        status_text: statusMap[o.status] || o.status,
                        order_type: o.order_type,
                        order_type_text: typeMap[o.order_type] || o.order_type,
                        total_amount: o.total_amount,
                        amount_display: (o.total_amount / 100).toFixed(2),
                        items_summary: ((_a = o.items) === null || _a === void 0 ? void 0 : _a.slice(0, 2).map((i) => i.name).join('、')) || '订单商品',
                        table_no: o.table_no,
                        created_at: o.created_at,
                        created_time: createdTime
                    };
                });
                const statusCounts = {
                    paid: pendingOrders.filter((o) => o.status === 'paid').length,
                    preparing: pendingOrders.filter((o) => o.status === 'preparing').length,
                    ready: pendingOrders.filter((o) => o.status === 'ready').length
                };
                this.setData({ pendingOrders, statusCounts });
                this.filterOrders();
            }
            catch (error) {
                logger_1.logger.error('加载订单失败', error, 'Dashboard');
            }
        });
    },
    // 切换订单标签
    switchOrderTab(e) {
        const tab = e.currentTarget.dataset.tab;
        this.setData({ orderTab: tab });
        this.filterOrders();
    },
    // 筛选订单
    filterOrders() {
        const { pendingOrders, orderTab } = this.data;
        let filtered = pendingOrders;
        if (orderTab !== 'all') {
            filtered = pendingOrders.filter((o) => o.status === orderTab);
        }
        this.setData({ filteredOrders: filtered });
    },
    loadTables() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const response = yield (0, merchant_table_device_management_1.getTables)({});
                const tables = response.tables || [];
                const statusClassMap = {
                    'available': 'status-available',
                    'occupied': 'status-occupied',
                    'reserved': 'status-reserved',
                    'disabled': 'status-disabled'
                };
                const statusTextMap = {
                    'available': '空闲',
                    'occupied': '就餐中',
                    'reserved': '已预订',
                    'disabled': '停用'
                };
                const formattedTables = tables.map((t) => (Object.assign(Object.assign({}, t), { status_class: statusClassMap[t.status] || '', status_text: statusTextMap[t.status] || t.status })));
                // 分组：散台和包间
                const tablesByType = new Map();
                formattedTables.forEach((table) => {
                    const type = table.table_type || 'table';
                    if (!tablesByType.has(type)) {
                        tablesByType.set(type, []);
                    }
                    tablesByType.get(type).push(table);
                });
                const tableGroups = [];
                if (tablesByType.has('table')) {
                    tableGroups.push({ name: '散台', type: 'table', tables: tablesByType.get('table') });
                }
                if (tablesByType.has('room')) {
                    tableGroups.push({ name: '包间', type: 'room', tables: tablesByType.get('room') });
                }
                const tableStats = {
                    total: tables.length,
                    available: tables.filter((t) => t.status === 'available').length,
                    occupied: tables.filter((t) => t.status === 'occupied').length
                };
                this.setData({ tableGroups, tableStats });
            }
            catch (error) {
                logger_1.logger.error('加载桌台失败', error, 'Dashboard');
            }
        });
    },
    // WebSocket 连接
    connectWebSocket() {
        return __awaiter(this, void 0, void 0, function* () {
            const merchantId = app.globalData.merchantId;
            const userId = app.globalData.userId;
            if (!merchantId) {
                logger_1.logger.warn('商户ID不存在，跳过WebSocket连接', {}, 'Dashboard');
                return;
            }
            try {
                yield websocket_realtime_1.RealtimeUtils.initializeForMerchant(Number(userId || 0), Number(merchantId), {
                    onOpen: () => {
                        logger_1.logger.info('WebSocket 连接成功', { merchantId }, 'Dashboard');
                        this.setData({ wsConnected: true });
                    },
                    onMessage: (msg) => {
                        this.handleWebSocketMessage(msg);
                    },
                    onNotification: (notif) => {
                        var _a, _b;
                        if (((_a = notif.title) === null || _a === void 0 ? void 0 : _a.includes('订单')) || ((_b = notif.content) === null || _b === void 0 ? void 0 : _b.includes('订单'))) {
                            this.loadOrders();
                            wx.vibrateShort({ type: 'medium' });
                        }
                    },
                    onOrderUpdate: (orderData) => {
                        logger_1.logger.info('收到订单更新', orderData, 'Dashboard');
                        this.loadOrders();
                        this.loadStats();
                        wx.vibrateShort({ type: 'medium' });
                    }
                });
            }
            catch (error) {
                logger_1.logger.error('WebSocket 连接失败', error, 'Dashboard');
                this.setData({ wsConnected: false });
            }
        });
    },
    handleWebSocketMessage(msg) {
        if (msg.type === 'new_order' || msg.type === 'order_update') {
            wx.vibrateShort({ type: 'medium' });
            this.loadOrders();
            this.loadStats();
        }
        else if (msg.type === 'table_status_change') {
            this.loadTables();
        }
    },
    // 切换营业状态
    onToggleStatus() {
        return __awaiter(this, void 0, void 0, function* () {
            const newStatus = !this.data.isOpen;
            try {
                yield merchant_1.MerchantManagementService.updateMerchantStatus({ is_open: newStatus });
                this.setData({ isOpen: newStatus });
                wx.showToast({ title: newStatus ? '已开始营业' : '已暂停营业', icon: 'none' });
                // 营业状态变化时管理 WebSocket
                if (newStatus) {
                    this.connectWebSocket();
                }
                else {
                    websocket_realtime_1.WebSocketUtils.closeAll();
                    this.setData({ wsConnected: false });
                    // 打烊时释放所有桌台
                    this.releaseAllTables();
                }
            }
            catch (error) {
                wx.showToast({ title: '操作失败', icon: 'none' });
            }
        });
    },
    // 释放所有桌台（打烊时调用）
    releaseAllTables() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const response = yield (0, merchant_table_device_management_1.getTables)();
                const allTables = response.tables || [];
                const occupiedTables = allTables.filter((t) => t.status === 'occupied');
                if (occupiedTables.length > 0) {
                    // 批量更新所有占用的桌台为可用
                    for (const table of occupiedTables) {
                        yield (0, merchant_table_device_management_1.updateTableStatus)(table.id, 'available');
                    }
                    logger_1.logger.info(`打烊释放了 ${occupiedTables.length} 个桌台`, null, 'Dashboard');
                    this.loadTables();
                }
            }
            catch (error) {
                logger_1.logger.error('释放桌台失败', error, 'Dashboard');
            }
        });
    },
    // 订单操作
    onOrderTap(e) {
        const id = e.currentTarget.dataset.id;
        wx.navigateTo({ url: `/pages/merchant/orders/index?highlight=${id}` });
    },
    onAcceptOrder(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const id = e.currentTarget.dataset.id;
            try {
                yield order_management_1.MerchantOrderManagementService.acceptOrder(id);
                wx.showToast({ title: '已接单', icon: 'success' });
                this.loadOrders();
            }
            catch (error) {
                wx.showToast({ title: '接单失败', icon: 'none' });
            }
        });
    },
    onRejectOrder(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const id = e.currentTarget.dataset.id;
            wx.showModal({
                title: '拒单原因',
                editable: true,
                placeholderText: '请输入拒单原因',
                success: (res) => __awaiter(this, void 0, void 0, function* () {
                    if (res.confirm && res.content) {
                        try {
                            yield order_management_1.MerchantOrderManagementService.rejectOrder(id, { reason: res.content });
                            wx.showToast({ title: '已拒单', icon: 'success' });
                            this.loadOrders();
                        }
                        catch (error) {
                            wx.showToast({ title: '拒单失败', icon: 'none' });
                        }
                    }
                })
            });
        });
    },
    onReadyOrder(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const id = e.currentTarget.dataset.id;
            try {
                yield order_management_1.MerchantOrderManagementService.markOrderReady(id);
                wx.showToast({ title: '已出餐', icon: 'success' });
                this.loadOrders();
            }
            catch (error) {
                wx.showToast({ title: '操作失败', icon: 'none' });
            }
        });
    },
    // 桌台操作 - 点击显示弹窗
    onTableCardTap(e) {
        const table = e.currentTarget.dataset.table;
        this.setData({
            showTablePopup: true,
            activeTable: table
        });
    },
    closeTablePopup() {
        this.setData({
            showTablePopup: false,
            activeTable: null
        });
    },
    setTableStatus(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const newStatus = e.currentTarget.dataset.status;
            const { activeTable } = this.data;
            if (!(activeTable === null || activeTable === void 0 ? void 0 : activeTable.id))
                return;
            try {
                yield (0, merchant_table_device_management_1.updateTableStatus)(activeTable.id, newStatus);
                wx.showToast({ title: '状态已更新', icon: 'success' });
                this.closeTablePopup();
                this.loadTables();
            }
            catch (error) {
                logger_1.logger.error('更新桌台状态失败', error, 'Dashboard');
                wx.showToast({ title: '更新失败', icon: 'none' });
            }
        });
    },
    // 跳转到预订管理页面添加预订
    goToAddReservation() {
        const { activeTable } = this.data;
        this.closeTablePopup();
        // 跳转到预订页面，带上桌台ID参数
        wx.navigateTo({
            url: `/pages/merchant/reservations/index?tableId=${activeTable === null || activeTable === void 0 ? void 0 : activeTable.id}&openAdd=true`
        });
    },
    // 旧的导航方法（保留用于快捷入口）
    onTableTap(e) {
        const id = e.currentTarget.dataset.id;
        wx.navigateTo({ url: `/pages/merchant/tables/index?tableId=${id}` });
    },
    // 快捷导航
    goToInventory() {
        wx.navigateTo({ url: '/pages/merchant/inventory/index' });
    },
    goToMembers() {
        wx.navigateTo({ url: '/pages/merchant/members/index' });
    },
    goToReservations() {
        wx.navigateTo({ url: '/pages/merchant/reservations/index' });
    },
    goToKitchen() {
        wx.navigateTo({ url: '/pages/merchant/kds/index' });
    },
    goToStats() {
        wx.navigateTo({ url: '/pages/merchant/analytics/index' });
    },
    goToFinance() {
        wx.navigateTo({ url: '/pages/merchant/finance/index' });
    },
    goToSettings() {
        wx.navigateTo({ url: '/pages/merchant/settings/index' });
    },
    goToDishes() {
        wx.navigateTo({ url: '/pages/merchant/dishes/index' });
    },
    goToCombos() {
        wx.navigateTo({ url: '/pages/merchant/combos/index' });
    },
    goToTables() {
        wx.navigateTo({ url: '/pages/merchant/tables/index' });
    },
    goToMarketing() {
        wx.navigateTo({ url: '/pages/merchant/vouchers/index' });
    }
});
