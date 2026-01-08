"use strict";
/**
 * 储值管理页面
 * 功能：储值使用场景设置、充值规则管理
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
const util_1 = require("@/utils/util");
const marketing_membership_1 = require("@/api/marketing-membership");
Page({
    data: {
        // 布局状态
        sidebarCollapsed: false,
        loading: true,
        saving: false,
        // 会员设置
        settings: {
            merchant_id: 0,
            balance_usable_scenes: [],
            bonus_usable_scenes: [],
            allow_with_voucher: true,
            allow_with_discount: true,
            max_deduction_percent: 100
        },
        // 充值规则
        rechargeRules: [],
        // 规则弹窗
        showRuleModal: false,
        editingRule: null,
        ruleForm: {
            recharge_amount: '',
            bonus_amount: '',
            valid_from: '',
            valid_until: ''
        },
        // 场景选项
        sceneOptions: [
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
                yield this.loadData();
            }
            else {
                // 等待商户信息就绪
                app.userInfoReadyCallback = () => __awaiter(this, void 0, void 0, function* () {
                    if (app.globalData.merchantId) {
                        this.setData({ merchantId: Number(app.globalData.merchantId) });
                        yield this.loadData();
                    }
                });
            }
        });
    },
    onSidebarCollapse(e) {
        this.setData({ sidebarCollapsed: e.detail.collapsed });
    },
    // ==================== 数据加载 ====================
    loadData() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                yield Promise.all([
                    this.loadSettings(),
                    this.loadRechargeRules()
                ]);
            }
            catch (error) {
                logger_1.logger.error('加载数据失败', error, 'membership-settings');
                wx.showToast({ title: '加载失败', icon: 'error' });
            }
            finally {
                this.setData({ loading: false });
            }
        });
    },
    loadSettings() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const settings = yield marketing_membership_1.membershipSettingsService.getMembershipSettings();
                this.setData({ settings });
            }
            catch (error) {
                logger_1.logger.error('加载会员设置失败', error, 'membership-settings');
            }
        });
    },
    loadRechargeRules() {
        return __awaiter(this, void 0, void 0, function* () {
            const { merchantId } = this.data;
            if (!merchantId)
                return;
            try {
                const rules = yield marketing_membership_1.rechargeRuleManagementService.listRechargeRules(merchantId);
                // 预处理价格
                const processedRules = rules.map(rule => (Object.assign(Object.assign({}, rule), { recharge_amount_display: (0, util_1.formatPriceNoSymbol)(rule.recharge_amount || 0), bonus_amount_display: (0, util_1.formatPriceNoSymbol)(rule.bonus_amount || 0), valid_from_display: rule.valid_from ? rule.valid_from.slice(0, 10) : '-', valid_until_display: rule.valid_until ? rule.valid_until.slice(0, 10) : '-' })));
                this.setData({ rechargeRules: processedRules });
            }
            catch (error) {
                logger_1.logger.error('加载充值规则失败', error, 'membership-settings');
            }
        });
    },
    // ==================== 会员设置操作 ====================
    onSceneToggle(e) {
        const { scene } = e.currentTarget.dataset;
        const current = [...this.data.settings.balance_usable_scenes];
        const index = current.indexOf(scene);
        if (index > -1) {
            current.splice(index, 1);
        }
        else {
            current.push(scene);
        }
        this.setData({ 'settings.balance_usable_scenes': current });
    },
    onSaveSettings() {
        return __awaiter(this, void 0, void 0, function* () {
            const { settings } = this.data;
            this.setData({ saving: true });
            try {
                const request = {
                    balance_usable_scenes: settings.balance_usable_scenes
                };
                yield marketing_membership_1.membershipSettingsService.updateMembershipSettings(request);
                wx.showToast({ title: '保存成功', icon: 'success' });
            }
            catch (error) {
                logger_1.logger.error('保存设置失败', error, 'membership-settings');
                wx.showToast({ title: '保存失败', icon: 'error' });
            }
            finally {
                this.setData({ saving: false });
            }
        });
    },
    // ==================== 充值规则操作 ====================
    onAddRule() {
        // 默认有效期：今天到一个月后
        const today = new Date();
        const nextMonth = new Date();
        nextMonth.setMonth(nextMonth.getMonth() + 1);
        this.setData({
            showRuleModal: true,
            editingRule: null,
            ruleForm: {
                recharge_amount: '',
                bonus_amount: '',
                valid_from: this.formatDate(today),
                valid_until: this.formatDate(nextMonth)
            }
        });
    },
    onEditRule(e) {
        const rule = e.currentTarget.dataset.rule;
        this.setData({
            showRuleModal: true,
            editingRule: rule,
            ruleForm: {
                recharge_amount: (0, util_1.formatPriceNoSymbol)(rule.recharge_amount || 0),
                bonus_amount: (0, util_1.formatPriceNoSymbol)(rule.bonus_amount || 0),
                valid_from: rule.valid_from.slice(0, 10),
                valid_until: rule.valid_until.slice(0, 10)
            }
        });
    },
    onCloseRuleModal() {
        this.setData({ showRuleModal: false, editingRule: null });
    },
    onModalContentTap() {
        // 阻止冒泡
    },
    onRuleFormInput(e) {
        const field = e.currentTarget.dataset.field;
        this.setData({ [`ruleForm.${field}`]: e.detail.value });
    },
    // ==================== 日历选择器 ====================
    onOpenCalendar(e) {
        const field = e.currentTarget.dataset.field;
        const currentValue = this.data.ruleForm[field];
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
        const { calendarYear, calendarMonth, calendarField, ruleForm } = this.data;
        const selectedValue = ruleForm[calendarField];
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
            [`ruleForm.${field}`]: date,
            showCalendar: false
        });
    },
    onSelectToday() {
        const today = this.formatDate(new Date());
        const field = this.data.calendarField;
        this.setData({
            [`ruleForm.${field}`]: today,
            showCalendar: false
        });
    },
    onSaveRule() {
        return __awaiter(this, void 0, void 0, function* () {
            const { merchantId, editingRule, ruleForm } = this.data;
            // 验证
            const rechargeYuan = parseFloat(ruleForm.recharge_amount);
            const bonusYuan = parseFloat(ruleForm.bonus_amount);
            if (isNaN(rechargeYuan) || rechargeYuan <= 0) {
                wx.showToast({ title: '请输入充值金额', icon: 'none' });
                return;
            }
            if (isNaN(bonusYuan) || bonusYuan < 0) {
                wx.showToast({ title: '请输入赠送金额', icon: 'none' });
                return;
            }
            if (!ruleForm.valid_from || !ruleForm.valid_until) {
                wx.showToast({ title: '请选择有效期', icon: 'none' });
                return;
            }
            // 检查日期顺序
            if (ruleForm.valid_until < ruleForm.valid_from) {
                wx.showToast({ title: '活动结束日期应晚于开始日期', icon: 'none' });
                return;
            }
            wx.showLoading({ title: '保存中...' });
            try {
                if (editingRule) {
                    // 更新
                    const request = {
                        recharge_amount: Math.round(rechargeYuan * 100),
                        bonus_amount: Math.round(bonusYuan * 100),
                        valid_from: ruleForm.valid_from + 'T00:00:00Z',
                        valid_until: ruleForm.valid_until + 'T23:59:59Z'
                    };
                    yield marketing_membership_1.rechargeRuleManagementService.updateRechargeRule(merchantId, editingRule.id, request);
                }
                else {
                    // 创建
                    const request = {
                        recharge_amount: Math.round(rechargeYuan * 100),
                        bonus_amount: Math.round(bonusYuan * 100),
                        valid_from: ruleForm.valid_from + 'T00:00:00Z',
                        valid_until: ruleForm.valid_until + 'T23:59:59Z'
                    };
                    yield marketing_membership_1.rechargeRuleManagementService.createRechargeRule(merchantId, request);
                }
                wx.hideLoading();
                wx.showToast({ title: '保存成功', icon: 'success' });
                this.setData({ showRuleModal: false });
                yield this.loadRechargeRules();
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('保存规则失败', error, 'membership-settings');
                wx.showToast({ title: '保存失败', icon: 'error' });
            }
        });
    },
    onDeleteRule(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const rule = e.currentTarget.dataset.rule;
            const rechargeDisplay = (0, util_1.formatPriceNoSymbol)(rule.recharge_amount || 0);
            const bonusDisplay = (0, util_1.formatPriceNoSymbol)(rule.bonus_amount || 0);
            wx.showModal({
                title: '确认删除',
                content: `确定删除"充${rechargeDisplay}元送${bonusDisplay}元"规则？`,
                success: (res) => __awaiter(this, void 0, void 0, function* () {
                    if (res.confirm) {
                        wx.showLoading({ title: '删除中...' });
                        try {
                            yield marketing_membership_1.rechargeRuleManagementService.deleteRechargeRule(this.data.merchantId, rule.id);
                            wx.hideLoading();
                            wx.showToast({ title: '已删除', icon: 'success' });
                            yield this.loadRechargeRules();
                        }
                        catch (error) {
                            wx.hideLoading();
                            logger_1.logger.error('删除规则失败', error, 'membership-settings');
                            wx.showToast({ title: '删除失败', icon: 'error' });
                        }
                    }
                })
            });
        });
    },
    onToggleRuleStatus(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const rule = e.currentTarget.dataset.rule;
            const newStatus = !rule.is_active;
            try {
                yield marketing_membership_1.rechargeRuleManagementService.updateRechargeRule(this.data.merchantId, rule.id, { is_active: newStatus });
                wx.showToast({ title: newStatus ? '已启用' : '已停用', icon: 'success' });
                yield this.loadRechargeRules();
            }
            catch (error) {
                logger_1.logger.error('更新规则状态失败', error, 'membership-settings');
                wx.showToast({ title: '操作失败', icon: 'error' });
            }
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
