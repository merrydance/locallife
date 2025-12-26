"use strict";
/**
 * 订单管理页面 - 桌面级 SaaS 实现
 * 完全对齐后端 API：/v1/merchant/orders
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
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const order_management_1 = require("../../../api/order-management");
const logger_1 = require("../../../utils/logger");
const dayjs_1 = __importDefault(require("dayjs"));
const app = getApp();
// 订单状态映射
const STATUS_LABELS = {
    pending: '待支付',
    paid: '待接单',
    preparing: '制作中',
    ready: '待出餐',
    delivering: '配送中',
    completed: '已完成',
    cancelled: '已取消'
};
// 订单类型映射
const TYPE_LABELS = {
    takeout: '外卖',
    dine_in: '堂食',
    takeaway: '自取',
    reservation: '预订'
};
Page({
    data: {
        // 页面状态
        loading: false,
        // SaaS 布局
        sidebarCollapsed: false,
        merchantName: '',
        isOpen: true,
        // 订单数据
        orders: [],
        // 统计数据
        statusCounts: {
            paid: 0,
            preparing: 0,
            ready: 0,
            completed: 0
        },
        todayRevenue: '0.00',
        // 筛选条件
        currentStatus: '',
        currentType: '',
        orderTypeLabel: '全部类型',
        searchKeyword: '',
        // 分页
        currentPage: 1,
        pageSize: 20,
        totalCount: 0,
        totalPages: 1,
        pageNumbers: [1],
        pageSizeOptions: [20, 50, 100],
        pageSizeIndex: 0,
        // 选择
        allSelected: false,
        selectedCount: 0,
        canBatchAccept: false,
        // 详情抽屉
        showDetail: false,
        detailOrder: null
    },
    onLoad() {
        this.initPage();
    },
    onShow() {
        // 每次显示时刷新数据
        if (this.data.orders.length > 0) {
            this.loadOrders();
        }
    },
    initPage() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            const merchantId = app.globalData.merchantId;
            const merchantName = ((_a = app.globalData.merchantInfo) === null || _a === void 0 ? void 0 : _a.name) || '商户';
            this.setData({ merchantName });
            if (merchantId) {
                yield this.loadStatusCounts();
                yield this.loadOrders();
            }
            else {
                app.userInfoReadyCallback = () => {
                    var _a;
                    this.setData({ merchantName: ((_a = app.globalData.merchantInfo) === null || _a === void 0 ? void 0 : _a.name) || '商户' });
                    this.loadStatusCounts();
                    this.loadOrders();
                };
            }
        });
    },
    // 加载各状态订单数量
    loadStatusCounts() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                // 并行请求各状态的订单数
                const [paidRes, preparingRes, readyRes, completedRes] = yield Promise.all([
                    order_management_1.MerchantOrderManagementService.getOrderList({ page_id: 1, page_size: 1, status: 'paid' }),
                    order_management_1.MerchantOrderManagementService.getOrderList({ page_id: 1, page_size: 1, status: 'preparing' }),
                    order_management_1.MerchantOrderManagementService.getOrderList({ page_id: 1, page_size: 1, status: 'ready' }),
                    order_management_1.MerchantOrderManagementService.getOrderList({ page_id: 1, page_size: 1, status: 'completed' })
                ]);
                // 计算今日营业额（已完成订单）
                let revenue = 0;
                completedRes.forEach(order => {
                    revenue += order.total_amount;
                });
                this.setData({
                    statusCounts: {
                        paid: paidRes.length,
                        preparing: preparingRes.length,
                        ready: readyRes.length,
                        completed: completedRes.length
                    },
                    todayRevenue: (revenue / 100).toFixed(2)
                });
            }
            catch (error) {
                logger_1.logger.error('加载订单统计失败', error, 'Orders');
            }
        });
    },
    // 加载订单列表
    loadOrders() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const params = {
                    page_id: this.data.currentPage,
                    page_size: this.data.pageSize
                };
                if (this.data.currentStatus) {
                    params.status = this.data.currentStatus;
                }
                const orders = yield order_management_1.MerchantOrderManagementService.getOrderList(params);
                // 转换数据格式
                const formattedOrders = orders.map(order => this.formatOrder(order));
                // 计算分页（API 可能不返回总数，这里用返回条数判断）
                const totalCount = formattedOrders.length;
                const totalPages = Math.max(1, Math.ceil(totalCount / this.data.pageSize));
                this.setData({
                    orders: formattedOrders,
                    totalCount,
                    totalPages,
                    pageNumbers: this.generatePageNumbers(this.data.currentPage, totalPages),
                    loading: false,
                    allSelected: false,
                    selectedCount: 0
                });
            }
            catch (error) {
                logger_1.logger.error('加载订单列表失败', error, 'Orders');
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    // 格式化订单数据
    formatOrder(order) {
        const createdAt = (0, dayjs_1.default)(order.created_at);
        // 生成商品摘要
        let itemsSummary = '';
        let itemsCount = 0;
        if (order.items && order.items.length > 0) {
            itemsSummary = order.items.slice(0, 2).map(i => i.name).join('、');
            if (order.items.length > 2) {
                itemsSummary += '...';
            }
            itemsCount = order.items.reduce((sum, i) => sum + i.quantity, 0);
        }
        return Object.assign(Object.assign({}, order), { selected: false, status_label: STATUS_LABELS[order.status] || order.status, order_type_label: TYPE_LABELS[order.order_type] || order.order_type, total_display: (order.total_amount / 100).toFixed(2), discount_display: (order.discount_amount / 100).toFixed(2), items_summary: itemsSummary || '无商品信息', items_count: itemsCount, created_date: createdAt.format('MM-DD'), created_time: createdAt.format('HH:mm:ss') });
    },
    // 生成页码数组
    generatePageNumbers(current, total) {
        const pages = [];
        const range = 2;
        for (let i = Math.max(1, current - range); i <= Math.min(total, current + range); i++) {
            pages.push(i);
        }
        return pages;
    },
    // 刷新订单
    refreshOrders() {
        return __awaiter(this, void 0, void 0, function* () {
            yield this.loadStatusCounts();
            yield this.loadOrders();
            wx.showToast({ title: '已刷新', icon: 'success' });
        });
    },
    // 导出订单
    exportOrders() {
        wx.showToast({ title: '导出功能开发中', icon: 'none' });
    },
    // 按状态筛选
    filterByStatus(e) {
        const status = e.currentTarget.dataset.status || '';
        this.setData({
            currentStatus: status,
            currentPage: 1
        });
        this.loadOrders();
    },
    // 显示类型筛选
    showTypeFilter() {
        wx.showActionSheet({
            itemList: ['全部类型', '外卖', '堂食', '自取', '预订'],
            success: (res) => {
                const types = ['', 'takeout', 'dine_in', 'takeaway', 'reservation'];
                const labels = ['全部类型', '外卖', '堂食', '自取', '预订'];
                this.setData({
                    currentType: types[res.tapIndex],
                    orderTypeLabel: labels[res.tapIndex]
                });
                this.loadOrders();
            }
        });
    },
    // 搜索输入
    onSearchInput(e) {
        this.setData({ searchKeyword: e.detail.value });
    },
    // 执行搜索
    doSearch() {
        this.setData({ currentPage: 1 });
        this.loadOrders();
    },
    // 全选/取消全选
    toggleSelectAll() {
        const allSelected = !this.data.allSelected;
        const orders = this.data.orders.map(o => (Object.assign(Object.assign({}, o), { selected: allSelected })));
        const selectedCount = allSelected ? orders.length : 0;
        const canBatchAccept = allSelected && orders.every(o => o.status === 'paid');
        this.setData({ orders, allSelected, selectedCount, canBatchAccept });
    },
    // 单选
    toggleSelect(e) {
        const id = e.currentTarget.dataset.id;
        const orders = this.data.orders.map(o => {
            if (o.id === id) {
                return Object.assign(Object.assign({}, o), { selected: !o.selected });
            }
            return o;
        });
        const selectedCount = orders.filter(o => o.selected).length;
        const allSelected = selectedCount === orders.length;
        const canBatchAccept = orders.filter(o => o.selected).every(o => o.status === 'paid');
        this.setData({ orders, allSelected, selectedCount, canBatchAccept });
    },
    // 清除选择
    clearSelection() {
        const orders = this.data.orders.map(o => (Object.assign(Object.assign({}, o), { selected: false })));
        this.setData({ orders, allSelected: false, selectedCount: 0, canBatchAccept: false });
    },
    // 分页
    prevPage() {
        if (this.data.currentPage > 1) {
            this.setData({ currentPage: this.data.currentPage - 1 });
            this.loadOrders();
        }
    },
    nextPage() {
        if (this.data.currentPage < this.data.totalPages) {
            this.setData({ currentPage: this.data.currentPage + 1 });
            this.loadOrders();
        }
    },
    goToPage(e) {
        const page = e.currentTarget.dataset.page;
        this.setData({ currentPage: page });
        this.loadOrders();
    },
    changePageSize(e) {
        const index = Number(e.detail.value);
        this.setData({
            pageSizeIndex: index,
            pageSize: this.data.pageSizeOptions[index],
            currentPage: 1
        });
        this.loadOrders();
    },
    // 接单
    acceptOrder(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const id = e.currentTarget.dataset.id;
            wx.showModal({
                title: '确认接单',
                content: '接单后将开始制作，确定接单吗？',
                success: (res) => __awaiter(this, void 0, void 0, function* () {
                    if (res.confirm) {
                        try {
                            yield order_management_1.MerchantOrderManagementService.acceptOrder(id);
                            wx.showToast({ title: '已接单', icon: 'success' });
                            this.loadStatusCounts();
                            this.loadOrders();
                        }
                        catch (error) {
                            logger_1.logger.error('接单失败', error, 'Orders');
                            wx.showToast({ title: '接单失败', icon: 'error' });
                        }
                    }
                })
            });
        });
    },
    // 拒单
    rejectOrder(e) {
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
                            this.loadStatusCounts();
                            this.loadOrders();
                        }
                        catch (error) {
                            logger_1.logger.error('拒单失败', error, 'Orders');
                            wx.showToast({ title: '拒单失败', icon: 'error' });
                        }
                    }
                })
            });
        });
    },
    // 出餐
    markReady(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const id = e.currentTarget.dataset.id;
            try {
                yield order_management_1.MerchantOrderManagementService.markOrderReady(id);
                wx.showToast({ title: '已出餐', icon: 'success' });
                this.loadStatusCounts();
                this.loadOrders();
            }
            catch (error) {
                logger_1.logger.error('出餐失败', error, 'Orders');
                wx.showToast({ title: '操作失败', icon: 'error' });
            }
        });
    },
    // 完成订单
    completeOrder(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const id = e.currentTarget.dataset.id;
            try {
                yield order_management_1.MerchantOrderManagementService.completeOrder(id);
                wx.showToast({ title: '已完成', icon: 'success' });
                this.loadStatusCounts();
                this.loadOrders();
            }
            catch (error) {
                logger_1.logger.error('完成订单失败', error, 'Orders');
                wx.showToast({ title: '操作失败', icon: 'error' });
            }
        });
    },
    // 查看详情
    viewDetail(e) {
        const id = e.currentTarget.dataset.id;
        const order = this.data.orders.find(o => o.id === id);
        if (order) {
            this.setData({
                showDetail: true,
                detailOrder: order
            });
        }
    },
    // 关闭详情
    closeDetail() {
        this.setData({ showDetail: false, detailOrder: null });
    },
    // 打印订单
    printOrder(e) {
        wx.showToast({ title: '打印指令已发送', icon: 'none' });
    },
    // 批量接单
    batchAccept() {
        return __awaiter(this, void 0, void 0, function* () {
            const selectedOrders = this.data.orders.filter(o => o.selected && o.status === 'paid');
            if (selectedOrders.length === 0)
                return;
            wx.showModal({
                title: '批量接单',
                content: `确定接收 ${selectedOrders.length} 个订单吗？`,
                success: (res) => __awaiter(this, void 0, void 0, function* () {
                    if (res.confirm) {
                        try {
                            yield Promise.all(selectedOrders.map(o => order_management_1.MerchantOrderManagementService.acceptOrder(o.id)));
                            wx.showToast({ title: `已接收 ${selectedOrders.length} 单`, icon: 'success' });
                            this.clearSelection();
                            this.loadStatusCounts();
                            this.loadOrders();
                        }
                        catch (error) {
                            logger_1.logger.error('批量接单失败', error, 'Orders');
                            wx.showToast({ title: '部分订单接收失败', icon: 'none' });
                        }
                    }
                })
            });
        });
    },
    // 批量打印
    batchPrint() {
        const selectedOrders = this.data.orders.filter(o => o.selected);
        wx.showToast({ title: `${selectedOrders.length} 个打印任务已发送`, icon: 'none' });
    },
    // 详情页操作
    acceptCurrentOrder() {
        if (this.data.detailOrder) {
            this.acceptOrder({ currentTarget: { dataset: { id: this.data.detailOrder.id } } });
        }
    },
    rejectCurrentOrder() {
        if (this.data.detailOrder) {
            this.rejectOrder({ currentTarget: { dataset: { id: this.data.detailOrder.id } } });
        }
    },
    markCurrentReady() {
        if (this.data.detailOrder) {
            this.markReady({ currentTarget: { dataset: { id: this.data.detailOrder.id } } });
        }
    },
    printCurrentOrder() {
        wx.showToast({ title: '打印指令已发送', icon: 'none' });
    },
    // SaaS 布局方法
    onSidebarCollapse(e) {
        this.setData({ sidebarCollapsed: e.detail.collapsed });
    },
    goBack() {
        wx.navigateBack({
            fail: () => {
                wx.redirectTo({ url: '/pages/merchant/dashboard/index' });
            }
        });
    }
});
