"use strict";
/**
 * 运费减免设置页面
 * 商户管理配送费优惠规则
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
const delivery_fee_1 = require("../../../api/delivery-fee");
const app = getApp();
Page({
    data: {
        loading: false,
        submitting: false,
        promotions: [],
        showModal: false,
        formData: {
            name: '',
            min_order_amount: '',
            discount_amount: '',
            valid_from: '',
            valid_until: ''
        },
        // 日历选择器状态
        showCalendar: false,
        calendarField: '',
        calendarYear: 2024,
        calendarMonth: 1,
        calendarDays: []
    },
    onLoad() {
        this.loadPromotions();
    },
    onShow() {
        if (this.data.promotions.length > 0) {
            this.loadPromotions();
        }
    },
    loadPromotions() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const merchantId = Number(app.globalData.merchantId);
                if (!merchantId) {
                    wx.showToast({ title: '请先登录商户', icon: 'none' });
                    this.setData({ loading: false });
                    return;
                }
                const result = yield delivery_fee_1.deliveryFeeService.getMerchantPromotions(merchantId);
                // 格式化显示数据
                const promotions = (result || []).map((item) => {
                    var _a, _b, _c, _d;
                    return ({
                        id: item.id,
                        name: item.name,
                        min_order_amount: item.min_order_amount,
                        min_order_amount_display: delivery_fee_1.DeliveryFeeAdapter.formatFee(item.min_order_amount),
                        discount_amount: item.discount_amount,
                        discount_display: delivery_fee_1.DeliveryFeeAdapter.formatFee(item.discount_amount),
                        valid_from: ((_a = item.valid_from) === null || _a === void 0 ? void 0 : _a.split('T')[0]) || '',
                        valid_until: ((_b = item.valid_until) === null || _b === void 0 ? void 0 : _b.split('T')[0]) || '',
                        valid_period: `${((_c = item.valid_from) === null || _c === void 0 ? void 0 : _c.split('T')[0]) || ''} ~ ${((_d = item.valid_until) === null || _d === void 0 ? void 0 : _d.split('T')[0]) || ''}`,
                        is_active: item.is_active,
                        typeText: item.discount_amount === 0 ? '免运费' : '运费减免'
                    });
                });
                this.setData({ promotions, loading: false });
            }
            catch (error) {
                console.error('加载运费减免规则失败:', error);
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    showAddModal() {
        // 默认日期：今天到一个月后
        const today = new Date();
        const nextMonth = new Date(today.getTime() + 30 * 24 * 60 * 60 * 1000);
        this.setData({
            showModal: true,
            formData: {
                name: '',
                min_order_amount: '',
                discount_amount: '0',
                valid_from: this.formatDate(today),
                valid_until: this.formatDate(nextMonth)
            }
        });
    },
    hideModal() {
        this.setData({ showModal: false });
    },
    // 阻止弹窗内部点击关闭弹窗
    preventClose() {
        // 空函数，仅用于阻止事件冒泡
    },
    formatDate(date) {
        const y = date.getFullYear();
        const m = date.getMonth() + 1;
        const d = date.getDate();
        return `${y}-${m < 10 ? '0' + m : m}-${d < 10 ? '0' + d : d}`;
    },
    onInputChange(e) {
        const field = e.currentTarget.dataset.field;
        const value = e.detail.value;
        this.setData({
            [`formData.${field}`]: value
        });
    },
    onDateChange(e) {
        const field = e.currentTarget.dataset.field;
        const value = e.detail.value;
        this.setData({
            [`formData.${field}`]: value
        });
    },
    createPromotion() {
        return __awaiter(this, void 0, void 0, function* () {
            const { formData } = this.data;
            // 验证
            if (!formData.name.trim()) {
                wx.showToast({ title: '请输入规则名称', icon: 'none' });
                return;
            }
            if (!formData.min_order_amount || Number(formData.min_order_amount) <= 0) {
                wx.showToast({ title: '请输入起订金额', icon: 'none' });
                return;
            }
            if (!formData.valid_from || !formData.valid_until) {
                wx.showToast({ title: '请选择有效期', icon: 'none' });
                return;
            }
            this.setData({ submitting: true });
            try {
                const merchantId = Number(app.globalData.merchantId);
                yield delivery_fee_1.deliveryFeeService.createMerchantPromotion(merchantId, {
                    name: formData.name.trim(),
                    promotion_type: Number(formData.discount_amount) === 0 ? 'free_shipping' : 'fixed_amount',
                    min_order_amount: Math.round(Number(formData.min_order_amount) * 100), // 转分
                    discount_value: Math.round(Number(formData.discount_amount) * 100), // 转分
                    start_time: formData.valid_from + 'T00:00:00Z',
                    end_time: formData.valid_until + 'T23:59:59Z',
                    is_active: true
                });
                wx.showToast({ title: '创建成功', icon: 'success' });
                this.hideModal();
                this.loadPromotions();
            }
            catch (error) {
                console.error('创建失败:', error);
                wx.showToast({ title: error.message || '创建失败', icon: 'none' });
            }
            finally {
                this.setData({ submitting: false });
            }
        });
    },
    deletePromotion(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const promoId = e.currentTarget.dataset.id;
            wx.showModal({
                title: '确认删除',
                content: '确定要删除这条运费减免规则吗？',
                success: (res) => __awaiter(this, void 0, void 0, function* () {
                    if (res.confirm) {
                        try {
                            const merchantId = Number(app.globalData.merchantId);
                            yield delivery_fee_1.deliveryFeeService.deleteMerchantPromotion(merchantId, promoId);
                            wx.showToast({ title: '已删除', icon: 'success' });
                            this.loadPromotions();
                        }
                        catch (error) {
                            console.error('删除失败:', error);
                            wx.showToast({ title: '删除失败', icon: 'error' });
                        }
                    }
                })
            });
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
            [`formData.${field}`]: date,
            showCalendar: false
        });
    },
    onSelectToday() {
        const today = this.formatDate(new Date());
        const field = this.data.calendarField;
        this.setData({
            [`formData.${field}`]: today,
            showCalendar: false
        });
    }
});
