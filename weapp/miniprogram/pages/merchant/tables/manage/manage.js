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
const merchant_table_device_management_1 = require("../../../../api/merchant-table-device-management");
const logger_1 = require("../../../../utils/logger");
const error_handler_1 = require("../../../../utils/error-handler");
const app = getApp();
Page({
    data: {
        allTables: [],
        tables: [],
        tableTypes: [{ type: 'all', label: '全部', available: 0, occupied: 0 }],
        activeTypeTab: 'all',
        loading: true,
        merchantId: 0,
        selectedTable: null,
        isAdding: false,
        statusOptions: [
            { value: 'available', label: '空闲', theme: 'success' },
            { value: 'occupied', label: '就餐中', theme: 'warning' },
            { value: 'disabled', label: '已封锁', theme: 'default' }
        ]
    },
    onLoad() {
        this.initData();
    },
    onBack() {
        wx.navigateBack();
    },
    initData() {
        return __awaiter(this, void 0, void 0, function* () {
            const merchantId = app.globalData.merchantId;
            if (merchantId) {
                this.setData({ merchantId: Number(merchantId) });
                this.loadTables();
            }
            else {
                app.userInfoReadyCallback = () => {
                    if (app.globalData.merchantId) {
                        this.setData({ merchantId: Number(app.globalData.merchantId) });
                        this.loadTables();
                    }
                };
            }
        });
    },
    loadTables() {
        return __awaiter(this, void 0, void 0, function* () {
            if (!this.data.merchantId)
                return;
            this.setData({ loading: true });
            try {
                const result = yield (0, merchant_table_device_management_1.getTables)({ page: 1, page_size: 100 });
                const processedTables = result.tables.map((t) => (Object.assign(Object.assign({}, t), { status_label: this.getStatusLabel(t.status), status_theme: this.getStatusTheme(t.status) })));
                this.setData({ allTables: processedTables });
                this.updateStats(processedTables);
                this.applyFilter();
                if (!this.data.selectedTable && processedTables.length > 0) {
                    this.setData({ selectedTable: Object.assign({}, processedTables[0]), isAdding: false });
                }
            }
            catch (error) {
                logger_1.logger.error('Table.loadTables', error);
                this.setData({ loading: false });
            }
        });
    },
    updateStats(allTables) {
        const available = allTables.filter(t => t.status === 'available').length;
        const occupied = allTables.filter(t => t.status === 'occupied').length;
        this.setData({
            tableTypes: [{ type: 'all', label: '全部', available, occupied }],
            loading: false
        });
    },
    applyFilter() {
        const { allTables, activeTypeTab } = this.data;
        let filtered = allTables;
        if (activeTypeTab !== 'all') {
            filtered = allTables.filter(t => t.table_type === activeTypeTab);
        }
        this.setData({ tables: filtered });
    },
    getStatusLabel(status) {
        var _a;
        return ((_a = this.data.statusOptions.find(o => o.value === status)) === null || _a === void 0 ? void 0 : _a.label) || status;
    },
    getStatusTheme(status) {
        var _a;
        return ((_a = this.data.statusOptions.find(o => o.value === status)) === null || _a === void 0 ? void 0 : _a.theme) || 'default';
    },
    onSelectTable(e) {
        const table = e.currentTarget.dataset.item;
        this.setData({ selectedTable: Object.assign({}, table), isAdding: false });
    },
    onTypeTabChange(e) {
        this.setData({ activeTypeTab: e.detail.value });
        this.applyFilter();
    },
    onFieldChange(e) {
        const { field } = e.currentTarget.dataset;
        const { value } = e.detail;
        this.setData({ [`selectedTable.${field}`]: value });
    },
    onPriceFieldChange(e) {
        const { field } = e.currentTarget.dataset;
        const value = Math.round(parseFloat(e.detail.value) * 100);
        this.setData({ [`selectedTable.${field}`]: value });
    },
    onSaveTable() {
        return __awaiter(this, void 0, void 0, function* () {
            const { selectedTable, isAdding } = this.data;
            if (!selectedTable.table_no || !selectedTable.capacity) {
                wx.showToast({ title: '请填写桌号和人数', icon: 'none' });
                return;
            }
            wx.showLoading({ title: '正在保存...' });
            try {
                if (isAdding) {
                    yield (0, merchant_table_device_management_1.createTable)(selectedTable);
                    wx.showToast({ title: '新增成功' });
                }
                else {
                    yield (0, merchant_table_device_management_1.updateTable)(selectedTable.id, selectedTable);
                    wx.showToast({ title: '配置已更新' });
                }
                this.setData({ isAdding: false });
                yield this.loadTables();
            }
            catch (error) {
                error_handler_1.ErrorHandler.handle(error, 'SaveTable');
            }
            finally {
                wx.hideLoading();
            }
        });
    },
    addTable() {
        this.setData({
            isAdding: true,
            selectedTable: {
                table_no: '',
                capacity: 4,
                table_type: 'table',
                minimum_spend: 0,
                status: 'available',
                description: ''
            }
        });
    },
    onTypePicker() {
        const types = ['普通桌位 (table)', '私密包厢 (room)'];
        wx.showActionSheet({
            itemList: types,
            success: (res) => {
                const type = res.tapIndex === 0 ? 'table' : 'room';
                this.setData({ 'selectedTable.table_type': type });
            }
        });
    },
    onRefreshQRCode() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            if (!((_a = this.data.selectedTable) === null || _a === void 0 ? void 0 : _a.id))
                return;
            wx.showLoading({ title: '生成中...' });
            try {
                const res = yield (0, merchant_table_device_management_1.generateTableQRCode)(this.data.selectedTable.id);
                this.setData({ 'selectedTable.qr_code_url': res.qr_code_url });
                wx.showToast({ title: '已更新' });
            }
            catch (error) {
                error_handler_1.ErrorHandler.handle(error, 'GenerateQR');
            }
            finally {
                wx.hideLoading();
            }
        });
    },
    onDeleteTable() {
        return __awaiter(this, void 0, void 0, function* () {
            const { selectedTable } = this.data;
            if (!selectedTable || !selectedTable.id)
                return;
            const res = yield wx.showModal({
                title: '确认删除',
                content: `确定要删除点位 "${selectedTable.table_no}" 吗？`,
                confirmColor: '#e34d59'
            });
            if (res.confirm) {
                wx.showLoading({ title: '删除中...' });
                try {
                    yield (0, merchant_table_device_management_1.deleteTable)(selectedTable.id);
                    wx.showToast({ title: '已删除' });
                    this.setData({ selectedTable: null });
                    yield this.loadTables();
                }
                catch (error) {
                    error_handler_1.ErrorHandler.handle(error, 'DeleteTable');
                }
                finally {
                    wx.hideLoading();
                }
            }
        });
    }
});
