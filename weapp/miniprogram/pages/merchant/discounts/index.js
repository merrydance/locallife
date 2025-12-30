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
/**
 * 满减活动管理页面
 */
const marketing_management_1 = require("../../../api/marketing-management");
Page({
    data: {
        loading: true,
        rules: [],
        sidebarCollapsed: false,
        // 弹窗状态
        showModal: false,
        editingRule: null,
        submitting: false,
        // 表单数据
        formData: {
            name: '',
            description: '',
            minOrderAmount: '',
            discountAmount: '',
            validFrom: '',
            validUntil: '',
            canStackWithVoucher: false,
            canStackWithMembership: false
        },
        // 日历状态
        showCalendar: false,
        calendarField: '',
        calendarYear: 2024,
        calendarMonth: 1,
        calendarDays: []
    },
    merchantId: 0,
    onLoad() {
        const app = getApp();
        const merchantId = app.globalData.merchantId;
        if (merchantId) {
            this.merchantId = Number(merchantId);
            this.loadRules();
        }
        else {
            app.userInfoReadyCallback = () => __awaiter(this, void 0, void 0, function* () {
                if (app.globalData.merchantId) {
                    this.merchantId = Number(app.globalData.merchantId);
                    this.loadRules();
                }
            });
        }
    },
    onShow() {
        if (this.merchantId) {
            this.loadRules();
        }
    },
    onSidebarCollapse(e) {
        this.setData({ sidebarCollapsed: e.detail.collapsed });
    },
    // ==================== 数据加载 ====================
    loadRules() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const rules = yield marketing_management_1.DiscountRuleManagementService.getDiscountRuleList(this.merchantId, { page_id: 1, page_size: 50 });
                this.setData({ rules, loading: false });
            }
            catch (err) {
                console.error('[Discount] Load failed:', err);
                this.setData({ loading: false });
                wx.showToast({ title: '加载失败', icon: 'none' });
            }
        });
    },
    // ==================== 表单操作 ====================
    showCreateModal() {
        const today = this.formatDateStr(new Date());
        const nextMonth = new Date();
        nextMonth.setMonth(nextMonth.getMonth() + 1);
        this.setData({
            showModal: true,
            editingRule: null,
            formData: {
                name: '',
                description: '',
                minOrderAmount: '',
                discountAmount: '',
                validFrom: today,
                validUntil: this.formatDateStr(nextMonth),
                canStackWithVoucher: false,
                canStackWithMembership: false
            }
        });
    },
    handleEdit(e) {
        const rule = e.currentTarget.dataset.rule;
        this.setData({
            showModal: true,
            editingRule: rule,
            formData: {
                name: rule.name,
                description: rule.description || '',
                minOrderAmount: (rule.min_order_amount / 100).toString(),
                discountAmount: (rule.discount_amount / 100).toString(),
                validFrom: this.formatDateStr(new Date(rule.valid_from)),
                validUntil: this.formatDateStr(new Date(rule.valid_until)),
                canStackWithVoucher: rule.can_stack_with_voucher,
                canStackWithMembership: rule.can_stack_with_membership
            }
        });
    },
    hideModal() {
        this.setData({ showModal: false, editingRule: null });
    },
    onModalContentTap() {
        // 阻止冒泡
    },
    onInputChange(e) {
        const field = e.currentTarget.dataset.field;
        const key = 'formData.' + field;
        this.setData({ [key]: e.detail.value });
    },
    toggleSwitch(e) {
        const field = e.currentTarget.dataset.field;
        const current = this.data.formData[field];
        const key = 'formData.' + field;
        this.setData({ [key]: !current });
    },
    handleSubmit() {
        return __awaiter(this, void 0, void 0, function* () {
            const { formData, editingRule } = this.data;
            if (!formData.name.trim()) {
                wx.showToast({ title: '请输入活动名称', icon: 'none' });
                return;
            }
            const minAmount = parseFloat(formData.minOrderAmount);
            const discountAmount = parseFloat(formData.discountAmount);
            if (isNaN(minAmount) || minAmount <= 0) {
                wx.showToast({ title: '请输入有效的满减门槛', icon: 'none' });
                return;
            }
            if (isNaN(discountAmount) || discountAmount <= 0) {
                wx.showToast({ title: '请输入有效的减免金额', icon: 'none' });
                return;
            }
            if (discountAmount >= minAmount) {
                wx.showToast({ title: '减免金额必须小于满减门槛', icon: 'none' });
                return;
            }
            if (!formData.validFrom || !formData.validUntil) {
                wx.showToast({ title: '请选择有效期', icon: 'none' });
                return;
            }
            if (formData.validUntil < formData.validFrom) {
                wx.showToast({ title: '结束时间必须晚于开始时间', icon: 'none' });
                return;
            }
            this.setData({ submitting: true });
            try {
                if (editingRule) {
                    const updateData = {
                        id: editingRule.id,
                        name: formData.name.trim(),
                        description: formData.description.trim() || undefined,
                        min_order_amount: Math.round(minAmount * 100),
                        discount_amount: Math.round(discountAmount * 100),
                        can_stack_with_voucher: formData.canStackWithVoucher,
                        can_stack_with_membership: formData.canStackWithMembership,
                        valid_from: new Date(formData.validFrom + 'T00:00:00').toISOString(),
                        valid_until: new Date(formData.validUntil + 'T23:59:59').toISOString()
                    };
                    yield marketing_management_1.DiscountRuleManagementService.updateDiscountRule(this.merchantId, editingRule.id, updateData);
                    wx.showToast({ title: '更新成功', icon: 'success' });
                }
                else {
                    const createData = {
                        name: formData.name.trim(),
                        description: formData.description.trim() || undefined,
                        min_order_amount: Math.round(minAmount * 100),
                        discount_amount: Math.round(discountAmount * 100),
                        can_stack_with_voucher: formData.canStackWithVoucher,
                        can_stack_with_membership: formData.canStackWithMembership,
                        valid_from: new Date(formData.validFrom + 'T00:00:00').toISOString(),
                        valid_until: new Date(formData.validUntil + 'T23:59:59').toISOString()
                    };
                    yield marketing_management_1.DiscountRuleManagementService.createDiscountRule(this.merchantId, createData);
                    wx.showToast({ title: '创建成功', icon: 'success' });
                }
                this.setData({ showModal: false, submitting: false });
                this.loadRules();
            }
            catch (err) {
                console.error('[Discount] Submit failed:', err);
                this.setData({ submitting: false });
                wx.showToast({ title: '操作失败', icon: 'none' });
            }
        });
    },
    // ==================== 活动操作 ====================
    handleToggleActive(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id, active } = e.currentTarget.dataset;
            const newActive = !active;
            wx.showModal({
                title: newActive ? '启用活动' : '停用活动',
                content: newActive ? '确定启用此满减活动吗？' : '确定停用此满减活动吗？',
                success: (res) => __awaiter(this, void 0, void 0, function* () {
                    if (res.confirm) {
                        try {
                            yield marketing_management_1.DiscountRuleManagementService.updateDiscountRule(this.merchantId, id, { is_active: newActive });
                            wx.showToast({ title: newActive ? '已启用' : '已停用', icon: 'success' });
                            this.loadRules();
                        }
                        catch (err) {
                            console.error('[Discount] Toggle failed:', err);
                            wx.showToast({ title: '操作失败', icon: 'none' });
                        }
                    }
                })
            });
        });
    },
    handleDelete(e) {
        const id = e.currentTarget.dataset.id;
        wx.showModal({
            title: '删除活动',
            content: '确定删除此满减活动吗？删除后无法恢复。',
            confirmColor: '#ff4d4f',
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                if (res.confirm) {
                    try {
                        yield marketing_management_1.DiscountRuleManagementService.deleteDiscountRule(this.merchantId, id);
                        wx.showToast({ title: '已删除', icon: 'success' });
                        this.loadRules();
                    }
                    catch (err) {
                        console.error('[Discount] Delete failed:', err);
                        wx.showToast({ title: '删除失败', icon: 'none' });
                    }
                }
            })
        });
    },
    // ==================== 日历选择器 ====================
    onOpenCalendar(e) {
        const field = e.currentTarget.dataset.field;
        const currentValue = this.data.formData[field];
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
        const { calendarYear, calendarMonth, calendarField, formData } = this.data;
        const selectedValue = formData[calendarField];
        const today = this.formatDateStr(new Date());
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
            const date = y + '-' + pad(m) + '-' + pad(day);
            days.push({ day, date, disabled: false, selected: date === selectedValue, today: date === today, currentMonth: false });
        }
        // 当月
        for (let day = 1; day <= daysInMonth; day++) {
            const date = calendarYear + '-' + pad(calendarMonth) + '-' + pad(day);
            days.push({ day, date, disabled: false, selected: date === selectedValue, today: date === today, currentMonth: true });
        }
        // 下月填充
        const remaining = 42 - days.length;
        for (let day = 1; day <= remaining; day++) {
            const m = calendarMonth === 12 ? 1 : calendarMonth + 1;
            const y = calendarMonth === 12 ? calendarYear + 1 : calendarYear;
            const date = y + '-' + pad(m) + '-' + pad(day);
            days.push({ day, date, disabled: false, selected: date === selectedValue, today: date === today, currentMonth: false });
        }
        this.setData({ calendarDays: days });
    },
    onSelectCalendarDay(e) {
        const date = e.currentTarget.dataset.date;
        const field = this.data.calendarField;
        const key = 'formData.' + field;
        this.setData({
            [key]: date,
            showCalendar: false
        });
    },
    onSelectToday() {
        const today = this.formatDateStr(new Date());
        const field = this.data.calendarField;
        const key = 'formData.' + field;
        this.setData({
            [key]: today,
            showCalendar: false
        });
    },
    // ==================== 工具方法 ====================
    formatDateStr(date) {
        const y = date.getFullYear();
        const m = ('0' + (date.getMonth() + 1)).slice(-2);
        const d = ('0' + date.getDate()).slice(-2);
        return y + '-' + m + '-' + d;
    }
});
