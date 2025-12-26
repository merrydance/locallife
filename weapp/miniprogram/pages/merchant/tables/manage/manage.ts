import {
    getTables,
    updateTableStatus,
    updateTable,
    createTable,
    deleteTable,
    generateTableQRCode
} from '../../../../api/merchant-table-device-management';
import { logger } from '../../../../utils/logger';
import { ErrorHandler } from '../../../../utils/error-handler';

const app = getApp<IAppOption>();

Page({
    data: {
        allTables: [] as any[],
        tables: [] as any[],
        tableTypes: [{ type: 'all', label: '全部', available: 0, occupied: 0 }] as any[],
        activeTypeTab: 'all',
        loading: true,
        merchantId: 0,
        selectedTable: null as any,
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

    async initData() {
        const merchantId = app.globalData.merchantId;
        if (merchantId) {
            this.setData({ merchantId: Number(merchantId) });
            this.loadTables();
        } else {
            app.userInfoReadyCallback = () => {
                if (app.globalData.merchantId) {
                    this.setData({ merchantId: Number(app.globalData.merchantId) });
                    this.loadTables();
                }
            };
        }
    },

    async loadTables() {
        if (!this.data.merchantId) return;
        this.setData({ loading: true });

        try {
            const result = await getTables({ page: 1, page_size: 100 });
            const processedTables = result.tables.map((t: any) => ({
                ...t,
                status_label: this.getStatusLabel(t.status),
                status_theme: this.getStatusTheme(t.status)
            }));

            this.setData({ allTables: processedTables });
            this.updateStats(processedTables);
            this.applyFilter();

            if (!this.data.selectedTable && processedTables.length > 0) {
                this.setData({ selectedTable: { ...processedTables[0] }, isAdding: false });
            }
        } catch (error) {
            logger.error('Table.loadTables', error);
            this.setData({ loading: false });
        }
    },

    updateStats(allTables: any[]) {
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

    getStatusLabel(status: string) {
        return this.data.statusOptions.find(o => o.value === status)?.label || status;
    },

    getStatusTheme(status: string) {
        return this.data.statusOptions.find(o => o.value === status)?.theme || 'default';
    },

    onSelectTable(e: any) {
        const table = e.currentTarget.dataset.item;
        this.setData({ selectedTable: { ...table }, isAdding: false });
    },

    onTypeTabChange(e: any) {
        this.setData({ activeTypeTab: e.detail.value });
        this.applyFilter();
    },

    onFieldChange(e: any) {
        const { field } = e.currentTarget.dataset;
        const { value } = e.detail;
        this.setData({ [`selectedTable.${field}`]: value });
    },

    onPriceFieldChange(e: any) {
        const { field } = e.currentTarget.dataset;
        const value = Math.round(parseFloat(e.detail.value) * 100);
        this.setData({ [`selectedTable.${field}`]: value });
    },

    async onSaveTable() {
        const { selectedTable, isAdding } = this.data;
        if (!selectedTable.table_no || !selectedTable.capacity) {
            wx.showToast({ title: '请填写桌号和人数', icon: 'none' });
            return;
        }

        wx.showLoading({ title: '正在保存...' });
        try {
            if (isAdding) {
                await createTable(selectedTable);
                wx.showToast({ title: '新增成功' });
            } else {
                await updateTable(selectedTable.id, selectedTable);
                wx.showToast({ title: '配置已更新' });
            }
            this.setData({ isAdding: false });
            await this.loadTables();
        } catch (error) {
            ErrorHandler.handle(error, 'SaveTable');
        } finally {
            wx.hideLoading();
        }
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

    async onRefreshQRCode() {
        if (!this.data.selectedTable?.id) return;
        wx.showLoading({ title: '生成中...' });
        try {
            const res = await generateTableQRCode(this.data.selectedTable.id);
            this.setData({ 'selectedTable.qr_code_url': res.qr_code_url });
            wx.showToast({ title: '已更新' });
        } catch (error) {
            ErrorHandler.handle(error, 'GenerateQR');
        } finally {
            wx.hideLoading();
        }
    },

    async onDeleteTable() {
        const { selectedTable } = this.data;
        if (!selectedTable || !selectedTable.id) return;

        const res = await wx.showModal({
            title: '确认删除',
            content: `确定要删除点位 "${selectedTable.table_no}" 吗？`,
            confirmColor: '#e34d59'
        });

        if (res.confirm) {
            wx.showLoading({ title: '删除中...' });
            try {
                await deleteTable(selectedTable.id);
                wx.showToast({ title: '已删除' });
                this.setData({ selectedTable: null });
                await this.loadTables();
            } catch (error) {
                ErrorHandler.handle(error, 'DeleteTable');
            } finally {
                wx.hideLoading();
            }
        }
    }
});
