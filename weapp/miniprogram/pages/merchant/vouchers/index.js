"use strict";
/**
 * 代金券管理页面
 * 功能：代金券的创建、编辑、启用/停用、删除
 * 遵循 PC-SaaS 布局规范
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
const logger_1 = require("@/utils/logger");
const marketing_management_1 = require("@/api/marketing-management");
Page({
    data: {
        // 布局状态
        sidebarCollapsed: false,
        loading: true,
        // 代金券列表
        vouchers: [],
        // 弹窗状态
        showModal: false,
        editingVoucher: null,
        form: {
            name: '',
            code: '',
            amount: '',
            min_order_amount: '',
            total_quantity: '',
            valid_from: '',
            valid_until: '',
            description: '',
            allowed_order_types: []
        },
        // 订单类型选项
        orderTypeOptions: [
            { value: 'dine_in', label: '堂食' },
            { value: 'takeout', label: '外卖' },
            { value: 'reservation', label: '预订' }
        ],
        // 商户ID
        merchantId: 0,
        // 日历选择器状态
        showCalendar: false,
        calendarField: '',
        calendarYear: 2024,
        calendarMonth: 1,
        calendarDays: []
    },
    onLoad() {
        return __awaiter(this, void 0, void 0, function* () {
            yield this.initData();
        });
    },
    initData() {
        return __awaiter(this, void 0, void 0, function* () {
            const app = getApp();
            const merchantId = app.globalData.merchantId;
            if (merchantId) {
                this.setData({ merchantId: Number(merchantId) });
                yield this.loadVouchers();
            }
            else {
                app.userInfoReadyCallback = () => __awaiter(this, void 0, void 0, function* () {
                    if (app.globalData.merchantId) {
                        this.setData({ merchantId: Number(app.globalData.merchantId) });
                        yield this.loadVouchers();
                    }
                });
            }
        });
    },
    onSidebarCollapse(e) {
        this.setData({ sidebarCollapsed: e.detail.collapsed });
    },
    // ==================== 数据加载 ====================
    loadVouchers() {
        return __awaiter(this, void 0, void 0, function* () {
            const { merchantId } = this.data;
            if (!merchantId)
                return;
            this.setData({ loading: true });
            try {
                const vouchers = yield marketing_management_1.VoucherManagementService.getVoucherList(merchantId, {
                    page_id: 1,
                    page_size: 50
                });
                // 添加状态信息
                const vouchersWithStatus = vouchers.map(v => (Object.assign(Object.assign({}, v), { statusText: marketing_management_1.MarketingAdapter.getVoucherStatusText(v), statusClass: this.getStatusClass(v) })));
                this.setData({ vouchers: vouchersWithStatus });
            }
            catch (error) {
                logger_1.logger.error('加载代金券失败', error, 'vouchers');
                wx.showToast({ title: '加载失败', icon: 'error' });
            }
            finally {
                this.setData({ loading: false });
            }
        });
    },
    getStatusClass(v) {
        if (!v.is_active)
            return 'inactive';
        if (marketing_management_1.MarketingAdapter.isVoucherExpired(v))
            return 'expired';
        if (marketing_management_1.MarketingAdapter.isVoucherSoldOut(v))
            return 'soldout';
        return 'active';
    },
    // ==================== 代金券操作 ====================
    onAddVoucher() {
        const today = new Date();
        const nextMonth = new Date();
        nextMonth.setMonth(nextMonth.getMonth() + 1);
        this.setData({
            showModal: true,
            editingVoucher: null,
            form: {
                name: '',
                code: '',
                amount: '',
                min_order_amount: '0',
                total_quantity: '100',
                valid_from: this.formatDate(today),
                valid_until: this.formatDate(nextMonth),
                description: '',
                allowed_order_types: ['dine_in', 'takeout', 'reservation']
            }
        });
    },
    onEditVoucher(e) {
        const voucher = e.currentTarget.dataset.voucher;
        this.setData({
            showModal: true,
            editingVoucher: voucher,
            form: {
                name: voucher.name,
                code: voucher.code,
                amount: String(voucher.amount / 100),
                min_order_amount: String(voucher.min_order_amount / 100),
                total_quantity: String(voucher.total_quantity),
                valid_from: voucher.valid_from.slice(0, 10),
                valid_until: voucher.valid_until.slice(0, 10),
                description: voucher.description || '',
                allowed_order_types: [...voucher.allowed_order_types]
            }
        });
    },
    onCloseModal() {
        this.setData({ showModal: false, editingVoucher: null });
    },
    onModalContentTap() {
        // 阻止冒泡
    },
    onFormInput(e) {
        const field = e.currentTarget.dataset.field;
        this.setData({ [`form.${field}`]: e.detail.value });
    },
    onOrderTypeToggle(e) {
        const type = e.currentTarget.dataset.type;
        const types = [...this.data.form.allowed_order_types];
        const index = types.indexOf(type);
        if (index > -1) {
            types.splice(index, 1);
        }
        else {
            types.push(type);
        }
        this.setData({ 'form.allowed_order_types': types });
    },
    onSaveVoucher() {
        return __awaiter(this, void 0, void 0, function* () {
            const { merchantId, editingVoucher, form } = this.data;
            // 验证
            if (!form.name.trim()) {
                wx.showToast({ title: '请输入代金券名称', icon: 'none' });
                return;
            }
            const amount = parseFloat(form.amount);
            if (isNaN(amount) || amount <= 0) {
                wx.showToast({ title: '请输入有效的优惠金额', icon: 'none' });
                return;
            }
            const quantity = parseInt(form.total_quantity, 10);
            if (isNaN(quantity) || quantity <= 0) {
                wx.showToast({ title: '请输入有效的发行数量', icon: 'none' });
                return;
            }
            if (!form.valid_from || !form.valid_until) {
                wx.showToast({ title: '请选择有效期', icon: 'none' });
                return;
            }
            if (form.valid_until < form.valid_from) {
                wx.showToast({ title: '结束日期应晚于开始日期', icon: 'none' });
                return;
            }
            if (form.allowed_order_types.length === 0) {
                wx.showToast({ title: '请至少选择一个适用场景', icon: 'none' });
                return;
            }
            wx.showLoading({ title: '保存中...' });
            try {
                const minAmount = parseFloat(form.min_order_amount) || 0;
                if (editingVoucher) {
                    const request = {
                        name: form.name,
                        description: form.description || undefined,
                        total_quantity: quantity,
                        valid_from: form.valid_from + 'T00:00:00Z',
                        valid_until: form.valid_until + 'T23:59:59Z',
                        allowed_order_types: form.allowed_order_types
                    };
                    yield marketing_management_1.VoucherManagementService.updateVoucher(merchantId, editingVoucher.id, request);
                }
                else {
                    const request = {
                        name: form.name,
                        amount: Math.round(amount * 100),
                        min_order_amount: Math.round(minAmount * 100),
                        total_quantity: quantity,
                        valid_from: form.valid_from + 'T00:00:00Z',
                        valid_until: form.valid_until + 'T23:59:59Z',
                        description: form.description || undefined,
                        allowed_order_types: form.allowed_order_types
                    };
                    yield marketing_management_1.VoucherManagementService.createVoucher(merchantId, request);
                }
                wx.hideLoading();
                wx.showToast({ title: '保存成功', icon: 'success' });
                this.setData({ showModal: false });
                yield this.loadVouchers();
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('保存代金券失败', error, 'vouchers');
                wx.showToast({ title: '保存失败', icon: 'error' });
            }
        });
    },
    onDeleteVoucher(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const voucher = e.currentTarget.dataset.voucher;
            wx.showModal({
                title: '确认删除',
                content: `确定删除代金券"${voucher.name}"？已领取但未使用的券将失效。`,
                success: (res) => __awaiter(this, void 0, void 0, function* () {
                    if (res.confirm) {
                        wx.showLoading({ title: '删除中...' });
                        try {
                            yield marketing_management_1.VoucherManagementService.deleteVoucher(this.data.merchantId, voucher.id);
                            wx.hideLoading();
                            wx.showToast({ title: '已删除', icon: 'success' });
                            yield this.loadVouchers();
                        }
                        catch (error) {
                            wx.hideLoading();
                            logger_1.logger.error('删除代金券失败', error, 'vouchers');
                            wx.showToast({ title: '删除失败，可能有未使用的券', icon: 'none' });
                        }
                    }
                })
            });
        });
    },
    onToggleVoucherStatus(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const voucher = e.currentTarget.dataset.voucher;
            const newStatus = !voucher.is_active;
            try {
                yield marketing_management_1.VoucherManagementService.updateVoucher(this.data.merchantId, voucher.id, { is_active: newStatus });
                wx.showToast({ title: newStatus ? '已启用' : '已停用', icon: 'success' });
                yield this.loadVouchers();
            }
            catch (error) {
                logger_1.logger.error('更新代金券状态失败', error, 'vouchers');
                wx.showToast({ title: '操作失败', icon: 'error' });
            }
        });
    },
    // ==================== 日历选择器 ====================
    onOpenCalendar(e) {
        const field = e.currentTarget.dataset.field;
        const currentValue = this.data.form[field];
        let year, month;
        if (currentValue) {
            const parts = currentValue.split('-');
            year = parseInt(parts[0], 10);
            month = parseInt(parts[1], 10);
        }
        else {
            const now = new Date();
            year = now.getFullYear();
            month = now.getMonth() + 1;
        }
        this.setData({
            showCalendar: true,
            calendarField: field,
            calendarYear: year,
            calendarMonth: month
        });
        this.generateCalendarDays();
    },
    onCloseCalendar() {
        this.setData({ showCalendar: false });
    },
    onCalendarContentTap() {
        // 阻止冒泡
    },
    onPrevMonth() {
        let { calendarYear, calendarMonth } = this.data;
        calendarMonth--;
        if (calendarMonth < 1) {
            calendarMonth = 12;
            calendarYear--;
        }
        this.setData({ calendarYear, calendarMonth });
        this.generateCalendarDays();
    },
    onNextMonth() {
        let { calendarYear, calendarMonth } = this.data;
        calendarMonth++;
        if (calendarMonth > 12) {
            calendarMonth = 1;
            calendarYear++;
        }
        this.setData({ calendarYear, calendarMonth });
        this.generateCalendarDays();
    },
    generateCalendarDays() {
        const { calendarYear, calendarMonth, calendarField, form } = this.data;
        const selectedValue = form[calendarField];
        const today = this.formatDate(new Date());
        const firstDay = new Date(calendarYear, calendarMonth - 1, 1);
        const lastDay = new Date(calendarYear, calendarMonth, 0);
        const startWeekday = firstDay.getDay();
        const daysInMonth = lastDay.getDate();
        const days = [];
        const pad = (n) => ('0' + n).slice(-2);
        // 上月填充
        const prevMonth = new Date(calendarYear, calendarMonth - 1, 0);
        const prevDays = prevMonth.getDate();
        for (let i = startWeekday - 1; i >= 0; i--) {
            const day = prevDays - i;
            const m = calendarMonth === 1 ? 12 : calendarMonth - 1;
            const y = calendarMonth === 1 ? calendarYear - 1 : calendarYear;
            const date = `${y}-${pad(m)}-${pad(day)}`;
            days.push({ day, date, disabled: false, selected: date === selectedValue, today: date === today, currentMonth: false });
        }
        // 当月
        for (let day = 1; day <= daysInMonth; day++) {
            const date = `${calendarYear}-${pad(calendarMonth)}-${pad(day)}`;
            days.push({ day, date, disabled: false, selected: date === selectedValue, today: date === today, currentMonth: true });
        }
        // 下月填充
        const remaining = 42 - days.length;
        for (let day = 1; day <= remaining; day++) {
            const m = calendarMonth === 12 ? 1 : calendarMonth + 1;
            const y = calendarMonth === 12 ? calendarYear + 1 : calendarYear;
            const date = `${y}-${pad(m)}-${pad(day)}`;
            days.push({ day, date, disabled: false, selected: date === selectedValue, today: date === today, currentMonth: false });
        }
        this.setData({ calendarDays: days });
    },
    onSelectCalendarDay(e) {
        const date = e.currentTarget.dataset.date;
        const field = this.data.calendarField;
        this.setData({
            [`form.${field}`]: date,
            showCalendar: false
        });
    },
    onSelectToday() {
        const today = this.formatDate(new Date());
        const field = this.data.calendarField;
        this.setData({
            [`form.${field}`]: today,
            showCalendar: false
        });
    },
    // ==================== 工具方法 ====================
    formatDate(date) {
        const y = date.getFullYear();
        const m = ('0' + (date.getMonth() + 1)).slice(-2);
        const d = ('0' + date.getDate()).slice(-2);
        return `${y}-${m}-${d}`;
    }
});
