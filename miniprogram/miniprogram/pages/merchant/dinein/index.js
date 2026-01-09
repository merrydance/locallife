"use strict";
/**
 * 商户堂食管理页面
 * 使用真实后端API
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
const responsive_1 = require("../../../utils/responsive");
const table_device_management_1 = require("../../../api/table-device-management");
Page({
    behaviors: [responsive_1.responsiveBehavior],
    data: {
        tables: [],
        sessions: [],
        tableStats: { total: 0, available: 0, occupied: 0 },
        loading: false
    },
    onLoad() {
        // 移除 manual isLargeScreen 设置，由 responsiveBehavior 注入
        this.loadData();
    },
    onShow() {
        // 返回时刷新
        if (this.data.tables.length > 0) {
            this.loadData();
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadData() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                // 获取桌台列表
                const result = yield table_device_management_1.tableManagementService.listTables('table');
                const tables = (result.tables || []).map((table) => {
                    var _a;
                    return ({
                        id: table.id,
                        name: table.table_no,
                        status: ((_a = table.status) === null || _a === void 0 ? void 0 : _a.toUpperCase()) || 'AVAILABLE',
                        capacity: table.capacity,
                        description: table.description,
                        minimum_spend: table.minimum_spend,
                        current_reservation_id: table.current_reservation_id
                    });
                });
                // 筛选出占用中的桌台作为活跃会话
                const sessions = tables
                    .filter((t) => t.status === 'OCCUPIED')
                    .map((t) => ({
                    id: `session_${t.id}`,
                    table_id: t.id,
                    table_name: t.name,
                    status: 'ACTIVE'
                }));
                const tableStats = {
                    total: tables.length,
                    available: tables.filter((t) => t.status === 'AVAILABLE').length,
                    occupied: tables.filter((t) => t.status === 'OCCUPIED').length
                };
                this.setData({
                    tables,
                    sessions,
                    tableStats,
                    loading: false
                });
            }
            catch (error) {
                console.error('加载桌台数据失败:', error);
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    onOpenTable(e) {
        const { id } = e.currentTarget.dataset;
        wx.showModal({
            title: '开台确认',
            content: '确认开台?',
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                if (res.confirm) {
                    try {
                        yield table_device_management_1.tableManagementService.updateTableStatus(id, { status: 'occupied' });
                        wx.showToast({ title: '开台成功', icon: 'success' });
                        this.loadData();
                    }
                    catch (error) {
                        console.error('开台失败:', error);
                        wx.showToast({ title: '开台失败', icon: 'error' });
                    }
                }
            })
        });
    },
    onCloseTable(e) {
        const { id } = e.currentTarget.dataset;
        wx.showModal({
            title: '结台确认',
            content: '确认结台?',
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                if (res.confirm) {
                    try {
                        yield table_device_management_1.tableManagementService.updateTableStatus(id, { status: 'available' });
                        wx.showToast({ title: '结台成功', icon: 'success' });
                        this.loadData();
                    }
                    catch (error) {
                        console.error('结台失败:', error);
                        wx.showToast({ title: '结台失败', icon: 'error' });
                    }
                }
            })
        });
    },
    onCheckout(e) {
        const { id } = e.currentTarget.dataset;
        wx.navigateTo({ url: `/pages/merchant/dinein/checkout/index?session_id=${id}` });
    }
});
