"use strict";
/**
 * 餐厅工作台首页
 * 响应式设计：PC(堂食桌台+外卖订单) / 手机(经营速览+快捷入口)
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
const websocket_realtime_1 = require("../../../api/websocket-realtime");
const logger_1 = require("../../../utils/logger");
const responsive_1 = require("../../../utils/responsive");
const app = getApp();
Page({
    behaviors: [responsive_1.responsiveBehavior],
    data: {
        loading: true,
        navBarHeight: 0,
        // Tab 切换
        activeTab: 'overview',
        // 骨架屏配置
        skeletonRowCol: [
            { width: '100%', height: '200rpx' },
            [{ width: '48%', height: '160rpx' }, { width: '48%', height: '160rpx', marginLeft: '4%' }],
            { width: '100%', height: '300rpx', marginTop: '24rpx' }
        ],
        // 滑动操作按钮
        orderSwipeRight: [
            { text: '接单', className: 'swipe-btn-accept' },
            { text: '详情', className: 'swipe-btn-detail' }
        ],
        // 商户信息
        merchantId: '',
        merchantName: '',
        isOpen: false,
        // 状态
        currentDate: '',
        // 统计数据
        stats: {
            todayRevenue: 0,
            todayOrders: 0,
            revenueGrowth: 0
        },
        // 桌台数据
        tableGroups: [],
        tableStats: {
            total: 0,
            available: 0,
            occupied: 0
        },
        selectedTable: null,
        statusOptions: [
            { value: 'available', label: '开闲', theme: 'success', icon: 'check-circle' },
            { value: 'occupied', label: '开台', theme: 'warning', icon: 'play-circle' },
            { value: 'disabled', label: '关台', theme: 'default', icon: 'minus-circle' }
        ],
        // 订单数据
        pendingOrders: [],
        pendingCount: 0,
        // 订单分类
        orderCategory: 'all',
        filteredOrders: [],
        // 提醒列表
        alerts: [],
        // 响应式状态
        deviceType: 'mobile',
        gridColumn: 4,
        // 系统日期
        todayDate: '',
        // PC SaaS 布局状态
        sidebarCollapsed: false,
        ownerName: '',
        avatarUrl: '',
        unreadNotifications: 0
    },
    onLoad() {
        this.updateSystemDate();
        this.setData({
            currentDate: new Date().toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' })
        });
        // 移除 manual deviceType 设置，由 responsiveBehavior 自动注入
        this.loadData();
    },
    onShow() {
        // 每次显示时刷新数据
        if (!this.data.loading) {
            this.refreshData();
        }
    },
    onHide() {
        // 页面隐藏时可选择断开 WebSocket
    },
    onUnload() {
        // 页面卸载时断开 WebSocket
        websocket_realtime_1.WebSocketUtils.closeAll();
    },
    onNavHeight(e) {
        const height = e.detail.navBarHeight;
        // @ts-ignore
        const statusBarHeight = this.data.statusBarHeight || 0;
        if (height !== undefined && height !== this.data.navBarHeight) {
            this.setData({
                navBarHeight: height,
                navBarContentHeight: height - statusBarHeight
            });
        }
    },
    onResize() {
        // 处理屏幕旋转或窗口缩放
        const { getDeviceInfo } = require('../../../utils/responsive');
        const { type } = getDeviceInfo();
        if (type !== this.data.deviceType) {
            logger_1.logger.info('布局发生变更，重新评估 WebSocket', { old: this.data.deviceType, new: type }, 'Dashboard');
            this.setData({ deviceType: type });
            this.connectWebSocket();
        }
    },
    /**
     * 更新系统日期 (格式化)
     */
    updateSystemDate() {
        const now = new Date();
        const year = now.getFullYear();
        const month = now.getMonth() + 1;
        const day = now.getDate();
        const weekDays = ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六'];
        const weekDay = weekDays[now.getDay()];
        this.setData({
            todayDate: `${year}年${month}月${day}日 ${weekDay}`
        });
    },
    /**
     * Tab 切换
     */
    onTabChange(e) {
        this.setData({ activeTab: e.detail.value });
    },
    /**
     * 订单滑动操作点击
     */
    onOrderSwipeClick(e) {
        const { index } = e.detail;
        const orderId = e.currentTarget.dataset.id;
        if (index === 0) {
            // 接单
            this.onAcceptOrder({ currentTarget: { dataset: { id: orderId } } });
        }
        else {
            // 详情
            wx.navigateTo({ url: `/pages/merchant/orders/detail/detail?id=${orderId}` });
        }
    },
    /**
     * 加载所有数据
     */
    loadData() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({
                loading: true,
                gridColumn: (0, responsive_1.isLargeScreen)() ? 3 : 4
            });
            try {
                // 先加载商户信息
                yield this.loadMerchantInfo();
                // 并行加载其他数据
                yield Promise.all([
                    this.loadMerchantStatus(),
                    this.loadTables(),
                    this.loadOrders(),
                    this.loadStats()
                ]);
                // WebSocket 连接是可选的，不阻塞页面加载
                this.connectWebSocket().catch(() => {
                    // WebSocket 连接失败不影响页面正常使用
                });
            }
            catch (error) {
                logger_1.logger.error('加载数据失败', error, 'Dashboard.loadData');
                wx.showToast({ title: '加载失败', icon: 'none' });
            }
            finally {
                this.setData({ loading: false });
            }
        });
    },
    /**
     * 刷新数据
     */
    refreshData() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                yield Promise.all([
                    this.loadMerchantStatus(),
                    this.loadTables(),
                    this.loadOrders()
                ]);
            }
            catch (error) {
                logger_1.logger.error('刷新数据失败', error, 'Dashboard.refreshData');
            }
        });
    },
    /**
     * 加载商户信息
     */
    loadMerchantInfo() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const merchantInfo = yield merchant_1.MerchantManagementService.getMerchantInfo();
                if (merchantInfo) {
                    // 更新 UI 数据
                    this.setData({
                        merchantId: String(merchantInfo.id),
                        merchantName: merchantInfo.name,
                        isOpen: merchantInfo.is_open
                    });
                    // 更新全局状态 (原子化操作)
                    app.globalData.merchantId = String(merchantInfo.id);
                    app.globalData.userRole = 'merchant';
                    if (!app.globalData.userId) {
                        app.globalData.userId = merchantInfo.owner_user_id;
                    }
                    // 保存所有者 ID 作为备用（某些鉴权场景可能需要）
                    this._merchantOwnerId = merchantInfo.owner_user_id;
                }
            }
            catch (error) {
                logger_1.logger.error('加载商户信息失败', error, 'Dashboard.loadMerchantInfo');
                // 如果获取失败，可能不是商户，跳转到入驻页
                this.handleNotMerchant();
            }
        });
    },
    /**
     * 加载商户营业状态
     */
    loadMerchantStatus() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const status = yield merchant_1.MerchantManagementService.getMerchantStatus();
                this.setData({ isOpen: status.is_open });
            }
            catch (error) {
                logger_1.logger.error('加载营业状态失败', error, 'Dashboard.loadMerchantStatus');
            }
        });
    },
    /**
     * 加载桌台数据
     * 使用 GET /v1/tables API
     */
    loadTables() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const response = yield (0, merchant_table_device_management_1.getTables)({});
                const tables = response.tables || [];
                // 按类型分组：餐桌和包间
                const tablesByType = new Map();
                tables.forEach((table) => {
                    const type = table.table_type || 'table';
                    if (!tablesByType.has(type)) {
                        tablesByType.set(type, []);
                    }
                    tablesByType.get(type).push(table);
                });
                // 转换为分组数组
                const tableGroups = [];
                if (tablesByType.has('table')) {
                    tableGroups.push({
                        name: '餐桌区',
                        type: 'table',
                        tables: tablesByType.get('table')
                    });
                }
                if (tablesByType.has('room')) {
                    tableGroups.push({
                        name: '包间',
                        type: 'room',
                        tables: tablesByType.get('room')
                    });
                }
                // 统计桌台状态
                const tableStats = {
                    total: tables.length,
                    available: tables.filter((t) => t.status === 'available').length,
                    occupied: tables.filter((t) => t.status === 'occupied').length
                };
                this.setData({ tableGroups, tableStats });
            }
            catch (error) {
                logger_1.logger.error('加载桌台失败', error, 'Dashboard.loadTables');
            }
        });
    },
    /**
     * 加载订单数据
     * TODO: 使用 GET /v1/merchant/orders API
     */
    loadOrders() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                // TODO: 调用真实 API
                // const response = await getMerchantOrders({ page_id: 1, page_size: 20, status: 'paid' })
                // 暂时使用空数据，等待 API 集成
                const orders = [];
                // 生成提醒
                const alerts = [];
                orders.filter(o => o.status === 'paid').forEach(order => {
                    alerts.push({
                        id: `order-${order.id}`,
                        type: 'order',
                        icon: 'notification',
                        text: `新${order.order_type === 'takeout' ? '外卖' : '堂食'}订单 #${order.order_no}`,
                        time: '刚刚',
                        data: order
                    });
                });
                this.setData({
                    pendingOrders: orders,
                    pendingCount: orders.filter(o => o.status === 'paid').length,
                    alerts
                }, () => {
                    this.filterOrders(); // Initial filter
                });
            }
            catch (error) {
                logger_1.logger.error('加载订单失败', error, 'Dashboard.loadOrders');
            }
        });
    },
    /**
     * 切换订单分类
     */
    onOrderCategoryChange(e) {
        const category = e.detail.value || e.currentTarget.dataset.value;
        this.setData({ orderCategory: category }, () => {
            this.filterOrders();
        });
    },
    /**
     * 过滤订单
     */
    filterOrders() {
        const { pendingOrders, orderCategory } = this.data;
        let filtered = pendingOrders;
        if (orderCategory !== 'all') {
            filtered = pendingOrders.filter(order => order.order_type === orderCategory);
        }
        this.setData({ filteredOrders: filtered });
    },
    /**
     * 加载统计数据
     */
    loadStats() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const today = new Date().toISOString().split('T')[0];
                const stats = yield merchant_analytics_1.MerchantStatsService.getStatsOverview({
                    start_date: today,
                    end_date: today
                });
                if (stats) {
                    this.setData({
                        stats: {
                            todayRevenue: stats.total_revenue || 0,
                            todayOrders: stats.total_orders || 0,
                            revenueGrowth: Math.round((stats.growth_rate || 0) * 100)
                        }
                    });
                }
            }
            catch (error) {
                logger_1.logger.error('加载统计失败', error, 'Dashboard.loadStats');
                // 容错处理：即使 API 失败也显示 0，不报错影响用户使用
                this.setData({
                    stats: {
                        todayRevenue: 0,
                        todayOrders: 0,
                        revenueGrowth: 0
                    }
                });
            }
        });
    },
    /**
     * 连接 WebSocket 接收实时推送
     */
    connectWebSocket() {
        return __awaiter(this, void 0, void 0, function* () {
            const { deviceType } = this.data;
            const platform = (0, responsive_1.getPlatformInfo)();
            const merchantId = this.data.merchantId || app.globalData.merchantId;
            logger_1.logger.debug('WebSocket 接入评估', { deviceType, platform: platform.type, merchantId }, 'Dashboard.connectWebSocket');
            // 准入规则：
            // 1. 只有非手机布局（Tablet / Desktop / PC-Full）才在页面初始化时尝试连接
            //    由于 1024px 宽度的显示器会被判定为 'tablet'，所以它们会自动建立连接。
            // 2. 只有真正的手机布局 (deviceType === 'mobile') 才跳过，
            //    这样你在模拟器选 iPhone 时，deviceType 就是 'mobile'，逻辑就会生效。
            if (deviceType === 'mobile') {
                logger_1.logger.info('手机小屏布局，跳过 WebSocket 连接', { deviceType, platform: platform.type }, 'Dashboard');
                return;
            }
            logger_1.logger.info('准备建立 WebSocket 连接', { deviceType, platform: platform.type }, 'Dashboard');
            // 检查是否已经建立了连接，如果已连接则复用
            if (websocket_realtime_1.WebSocketUtils.isConnected()) {
                logger_1.logger.info('WebSocket 已建立或正在开启中，复用该连接', undefined, 'Dashboard');
                return;
            }
            try {
                // 这里的 merchantId 实际上是实体的 ID
                if (!merchantId) {
                    logger_1.logger.warn('商户ID不存在，跳过WebSocket连接', {}, 'Dashboard');
                    return;
                }
                // 获取当前登录用户 ID（用于握手鉴权，必须与 Token 的身份一致）
                const currentUserId = app.globalData.userId;
                if (!currentUserId) {
                    logger_1.logger.warn('当前用户信息尚未加载，WebSocket 鉴权可能失败', {}, 'Dashboard');
                }
                // 为商户初始化实时通信
                yield websocket_realtime_1.RealtimeUtils.initializeForMerchant(Number(currentUserId || 0), Number(merchantId), {
                    onOpen: () => {
                        logger_1.logger.info('WebSocket 连接成功', { userId: currentUserId, merchantId }, 'Dashboard');
                    },
                    onMessage: (msg) => {
                        this.handleWebSocketMessage(msg);
                    },
                    onNotification: (notif) => {
                        var _a, _b;
                        logger_1.logger.info('收到系统通知', notif, 'Dashboard.WebSocket');
                        // 如果通知内容包含“订单”，则刷新订单列表
                        if (((_a = notif.title) === null || _a === void 0 ? void 0 : _a.includes('订单')) || ((_b = notif.content) === null || _b === void 0 ? void 0 : _b.includes('订单'))) {
                            this.loadOrders();
                            wx.vibrateShort({ type: 'medium' });
                        }
                    },
                    onOrderUpdate: (orderData) => {
                        logger_1.logger.info('收到订单更新', orderData, 'Dashboard.WebSocket');
                        this.loadOrders();
                        wx.vibrateShort({ type: 'medium' });
                    }
                });
            }
            catch (error) {
                logger_1.logger.error('WebSocket 连接失败', error, 'Dashboard.connectWebSocket');
            }
        });
    },
    /**
     * 处理 WebSocket 消息
     */
    handleWebSocketMessage(msg) {
        var _a, _b, _c, _d, _e, _f;
        if (msg.type === 'new_order' || (msg.type === 'notification' && (((_b = (_a = msg.data) === null || _a === void 0 ? void 0 : _a.title) === null || _b === void 0 ? void 0 : _b.includes('订单')) || ((_d = (_c = msg.data) === null || _c === void 0 ? void 0 : _c.content) === null || _d === void 0 ? void 0 : _d.includes('订单'))))) {
            // 新订单通知
            wx.vibrateShort({ type: 'medium' });
            this.loadOrders();
            if ((_e = msg.data) === null || _e === void 0 ? void 0 : _e.title) {
                wx.showToast({ title: msg.data.title, icon: 'none' });
            }
        }
        else if (msg.type === 'table_status_change') {
            // 桌台状态变化
            this.loadTables();
        }
        else if (msg.type === 'notification') {
            // 其他普通通知
            if ((_f = msg.data) === null || _f === void 0 ? void 0 : _f.title) {
                wx.showToast({ title: msg.data.title, icon: 'none' });
            }
        }
    },
    /**
     * 非商户处理
     */
    handleNotMerchant() {
        wx.showModal({
            title: '无法访问',
            content: '您可能还未完成商户入驻，或商户审核尚未通过',
            confirmText: '去入驻',
            cancelText: '返回首页',
            success: (res) => {
                if (res.confirm) {
                    wx.redirectTo({ url: '/pages/register/merchant/index' });
                }
                else {
                    wx.switchTab({ url: '/pages/takeout/index' });
                }
            }
        });
    },
    /**
     * 切换营业状态
     */
    onToggleStatus() {
        return __awaiter(this, void 0, void 0, function* () {
            const newStatus = !this.data.isOpen;
            try {
                yield merchant_1.MerchantManagementService.updateMerchantStatus({
                    is_open: newStatus
                });
                this.setData({ isOpen: newStatus });
                wx.showToast({ title: newStatus ? '已开始营业' : '已暂停营业', icon: 'none' });
            }
            catch (error) {
                logger_1.logger.error('切换营业状态失败', error, 'Dashboard.onToggleStatus');
                wx.showToast({ title: '操作失败', icon: 'none' });
            }
        });
    },
    /**
     * 点击桌台 - 集成监控逻辑
     */
    onTableTap(e) {
        const id = e.currentTarget.dataset.id;
        let table = null;
        // 兼容性替代 flatMap
        for (let i = 0; i < this.data.tableGroups.length; i++) {
            const found = this.data.tableGroups[i].tables.find((t) => t.id === id);
            if (found) {
                table = found;
                break;
            }
        }
        if ((0, responsive_1.isLargeScreen)()) {
            this.setData({ selectedTable: table });
        }
        else {
            // 手机端跳转
            wx.navigateTo({
                url: `/pages/merchant/tables/manage/manage?tableId=${id}`
            });
        }
    },
    onDeselectTable() {
        this.setData({ selectedTable: null });
    },
    updateTableStatusAction(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { status } = e.currentTarget.dataset;
            const { selectedTable } = this.data;
            if (!selectedTable)
                return;
            wx.showLoading({ title: '指挥执行中...' });
            try {
                const { updateTableStatus } = require('../../../api/merchant-table-device-management');
                yield updateTableStatus(selectedTable.id, status);
                wx.showToast({ title: '操作成功', icon: 'success' });
                yield this.loadTables(); // 刷新数据
                // 更新当前选中的桌台状态 (兼容性查找)
                let updatedTable = null;
                for (let i = 0; i < this.data.tableGroups.length; i++) {
                    const found = this.data.tableGroups[i].tables.find((t) => t.id === selectedTable.id);
                    if (found) {
                        updatedTable = found;
                        break;
                    }
                }
                this.setData({ selectedTable: updatedTable });
            }
            catch (error) {
                logger_1.logger.error('Dashboard.updateTableStatus', error);
                wx.showToast({ title: '同步失败', icon: 'none' });
            }
            finally {
                wx.hideLoading();
            }
        });
    },
    /**
     * 接单
     * 使用 POST /v1/merchant/orders/{id}/accept
     */
    onAcceptOrder(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const orderId = e.currentTarget.dataset.id;
            try {
                wx.showLoading({ title: '接单中...' });
                // TODO: 调用真实 API
                // await acceptMerchantOrder(orderId)
                wx.hideLoading();
                wx.showToast({ title: '接单成功', icon: 'success' });
                this.loadOrders();
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('接单失败', error, 'Dashboard.onAcceptOrder');
                wx.showToast({ title: '接单失败', icon: 'none' });
            }
        });
    },
    /**
     * 出餐
     * 使用 POST /v1/merchant/orders/{id}/ready
     */
    onReadyOrder(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const orderId = e.currentTarget.dataset.id;
            try {
                wx.showLoading({ title: '处理中...' });
                // TODO: 调用真实 API
                // await readyMerchantOrder(orderId)
                wx.hideLoading();
                wx.showToast({ title: '已出餐', icon: 'success' });
                this.loadOrders();
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('出餐失败', error, 'Dashboard.onReadyOrder');
                wx.showToast({ title: '操作失败', icon: 'none' });
            }
        });
    },
    /**
     * 点击提醒
     */
    onAlertTap(e) {
        const item = e.currentTarget.dataset.item;
        if (item.type === 'order') {
            wx.navigateTo({ url: '/pages/merchant/orders/index' });
        }
        else if (item.type === 'table') {
            this.goToTables();
        }
    },
    // ========== 侧边栏控制方法 ==========
    /**
     * 侧边栏折叠/展开
     */
    onSidebarCollapse(e) {
        this.setData({ sidebarCollapsed: e.detail.collapsed });
    },
    /**
     * 侧边栏菜单点击（由组件内部处理导航，这里可做额外逻辑）
     */
    onSidebarMenuChange(e) {
        const { path } = e.detail;
        logger_1.logger.info('菜单切换', { path }, 'Dashboard.onSidebarMenuChange');
    },
    /**
     * 拒单
     */
    onRejectOrder(e) {
        const orderId = e.currentTarget.dataset.id;
        wx.showModal({
            title: '确认拒单',
            content: '拒单后订单将取消，确定要拒绝此订单吗？',
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                if (res.confirm) {
                    try {
                        wx.showLoading({ title: '处理中...' });
                        // TODO: 调用拒单 API
                        wx.hideLoading();
                        wx.showToast({ title: '已拒单', icon: 'none' });
                        this.loadOrders();
                    }
                    catch (error) {
                        wx.hideLoading();
                        wx.showToast({ title: '操作失败', icon: 'none' });
                    }
                }
            })
        });
    },
    // ========== 导航方法 ==========
    goToTables() {
        wx.navigateTo({ url: '/pages/merchant/tables/manage/manage' });
    },
    goToOrders() {
        wx.navigateTo({ url: '/pages/merchant/orders/index' });
    },
    goToKitchen() {
        wx.navigateTo({ url: '/pages/merchant/kitchen/display/display' });
    },
    goToAdmin() {
        wx.navigateTo({ url: '/pages/merchant/admin/index' });
    },
    goToDishes() {
        wx.navigateTo({ url: '/pages/merchant/dishes/index' });
    },
    goToCombos() {
        wx.navigateTo({ url: '/pages/merchant/combos/index' });
    },
    goToStats() {
        wx.navigateTo({ url: '/pages/merchant/analytics/index' });
    },
    goToSettings() {
        wx.navigateTo({ url: '/pages/merchant/profile/index' });
    },
    goToInventory() {
        wx.navigateTo({ url: '/pages/merchant/inventory/index' });
    },
    goToMembers() {
        wx.navigateTo({ url: '/pages/merchant/members/index' });
    },
    goToMarketing() {
        wx.navigateTo({ url: '/pages/merchant/marketing/manage/manage' });
    },
    goToFinance() {
        wx.navigateTo({ url: '/pages/merchant/finance/settlement' });
    },
    goToReviews() {
        wx.navigateTo({ url: '/pages/merchant/review/index' });
    }
});
