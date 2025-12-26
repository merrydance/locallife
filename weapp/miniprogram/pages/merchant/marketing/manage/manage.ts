/**
 * 营销活动管理页面
 * 包含优惠券管理、充值规则管理、会员设置管理
 * 使用TDesign组件库实现统一的UI风格
 */

import {
    VoucherManagementService,
    RechargeRuleManagementService,
    MembershipSettingsService,
    MarketingAdapter,
    type VoucherResponse,
    type RechargeRuleResponse,
    type MembershipSettingsResponse,
    type CreateVoucherRequest,
    type CreateRechargeRuleRequest,
    type UpdateMembershipSettingsRequest
} from '@/api/marketing-management';
import { isLargeScreen } from '@/utils/responsive';

Page({
    data: {
        isLargeScreen: false,
        // 当前Tab
        currentTab: 'voucher', // voucher, recharge, membership

        // 商户ID（从全局状态或用户信息获取）
        merchantId: 1,

        // 优惠券数据
        vouchers: [] as VoucherResponse[],
        voucherPage: 1,
        voucherPageSize: 20,
        voucherHasMore: true,

        // 充值规则数据
        rechargeRules: [] as RechargeRuleResponse[],

        // 会员设置数据
        membershipSettings: null as MembershipSettingsResponse | null,

        // 界面状态
        loading: true,
        refreshing: false,

        // 优惠券弹窗
        showVoucherModal: false,
        voucherForm: {
            name: '',
            description: '',
            discount_type: 'fixed',
            discount_value: 0,
            amount: 0,
            code: '',
            min_order_amount: 0,
            max_discount_amount: 0,
            total_quantity: 100,
            valid_from: '',
            valid_until: '',
            applicable_order_types: ['takeout', 'dine_in']
        } as any,

        // 充值规则弹窗
        showRechargeModal: false,
        rechargeForm: {
            recharge_amount: 0,
            bonus_amount: 0,
            description: ''
        } as CreateRechargeRuleRequest,

        // 会员设置弹窗
        showMembershipModal: false,
        membershipForm: {
            balance_usage_scenarios: ['takeout', 'dine_in'],
            bonus_usage_scenarios: ['takeout'],
            allow_stacking_discounts: true,
            min_recharge_amount: 1000
        } as UpdateMembershipSettingsRequest,

        // 选项
        discountTypeOptions: [
            { label: '立减', value: 'fixed' },
            { label: '折扣', value: 'percentage' }
        ],
        orderTypeOptions: [
            { label: '外卖', value: 'takeout', checked: true },
            { label: '堂食', value: 'dine_in', checked: true },
            { label: '打包自取', value: 'takeaway', checked: false },
            { label: '预定', value: 'reservation', checked: false }
        ]
    },

    onLoad() {
        this.setData({ isLargeScreen: isLargeScreen() });
        this.initPage();
    },

    onShow() {
        this.loadData();
    },

    /**
     * 初始化页面
     */
    async initPage() {
        try {
            this.setData({ loading: true });
            await this.loadData();
        } catch (error: any) {
            console.error('初始化页面失败:', error);
            wx.showToast({
                title: error.message || '加载失败',
                icon: 'error'
            });
        } finally {
            this.setData({ loading: false });
        }
    },

    /**
     * 加载数据
     */
    async loadData() {
        const { currentTab } = this.data;

        switch (currentTab) {
            case 'voucher':
                await this.loadVouchers();
                break;
            case 'recharge':
                await this.loadRechargeRules();
                break;
            case 'membership':
                await this.loadMembershipSettings();
                break;
        }
    },

    /**
     * 切换Tab
     */
    onTabChange(e: any) {
        const tab = e.detail.value;
        this.setData({ currentTab: tab });
        this.loadData();
    },

    // ==================== 优惠券管理 ====================

    /**
     * 加载优惠券列表
     */
    async loadVouchers(reset: boolean = true) {
        try {
            const { merchantId, voucherPage, voucherPageSize } = this.data;

            if (reset) {
                this.setData({ voucherPage: 1, vouchers: [], voucherHasMore: true });
            }

            const result = await VoucherManagementService.getVoucherList(merchantId, {
                page_id: reset ? 1 : voucherPage,
                page_size: voucherPageSize
            });

            const newVouchers = reset ? result : [...this.data.vouchers, ...result];

            this.setData({
                vouchers: newVouchers,
                voucherHasMore: result.length === voucherPageSize,
                voucherPage: reset ? 2 : voucherPage + 1
            });

        } catch (error: any) {
            console.error('加载优惠券失败:', error);
            wx.showToast({
                title: '加载优惠券失败',
                icon: 'error'
            });
        }
    },

    /**
     * 显示创建优惠券弹窗
     */
    showCreateVoucherModal() {
        // 重置表单
        this.setData({
            showVoucherModal: true,
            voucherForm: {
                name: '',
                description: '',
                discount_type: 'fixed',
                discount_value: 0,
                amount: 0,
                code: '',
                min_order_amount: 0,
                max_discount_amount: 0,
                total_quantity: 100,
                valid_from: '',
                valid_until: '',
                applicable_order_types: ['takeout', 'dine_in']
            }
        });
    },

    /**
     * 关闭优惠券弹窗
     */
    closeVoucherModal() {
        this.setData({ showVoucherModal: false });
    },

    /**
     * 优惠券表单输入
     */
    onVoucherFormInput(e: any) {
        const { field } = e.currentTarget.dataset;
        const { value } = e.detail;
        this.setData({
            [`voucherForm.${field}`]: value
        });
    },

    /**
     * 创建优惠券
     */
    async createVoucher() {
        const { merchantId, voucherForm } = this.data;

        // 表单验证
        if (!voucherForm.name) {
            wx.showToast({ title: '请输入优惠券名称', icon: 'error' });
            return;
        }

        try {
            wx.showLoading({ title: '创建中...' });

            await VoucherManagementService.createVoucher(merchantId, voucherForm);

            wx.showToast({
                title: '创建成功',
                icon: 'success'
            });

            this.closeVoucherModal();
            this.loadVouchers(true);

        } catch (error: any) {
            wx.showToast({
                title: error.message || '创建失败',
                icon: 'error'
            });
        } finally {
            wx.hideLoading();
        }
    },

    /**
     * 删除优惠券
     */
    async deleteVoucher(e: any) {
        const voucherId = e.currentTarget.dataset.id;
        const { merchantId } = this.data;

        wx.showModal({
            title: '确认删除',
            content: '确定要删除此优惠券吗？',
            success: async (res) => {
                if (res.confirm) {
                    try {
                        wx.showLoading({ title: '删除中...' });

                        await VoucherManagementService.deleteVoucher(merchantId, voucherId);

                        wx.showToast({
                            title: '删除成功',
                            icon: 'success'
                        });

                        this.loadVouchers(true);

                    } catch (error: any) {
                        wx.showToast({
                            title: error.message || '删除失败',
                            icon: 'error'
                        });
                    } finally {
                        wx.hideLoading();
                    }
                }
            }
        });
    },

    // ==================== 充值规则管理 ====================

    /**
     * 加载充值规则列表
     */
    async loadRechargeRules() {
        try {
            const { merchantId } = this.data;

            const result = await RechargeRuleManagementService.getRechargeRules(merchantId);

            this.setData({ rechargeRules: result });

        } catch (error: any) {
            console.error('加载充值规则失败:', error);
            wx.showToast({
                title: '加载充值规则失败',
                icon: 'error'
            });
        }
    },

    /**
     * 显示创建充值规则弹窗
     */
    showCreateRechargeModal() {
        this.setData({
            showRechargeModal: true,
            rechargeForm: {
                recharge_amount: 0,
                bonus_amount: 0,
                description: ''
            }
        });
    },

    /**
     * 关闭充值规则弹窗
     */
    closeRechargeModal() {
        this.setData({ showRechargeModal: false });
    },

    /**
     * 充值规则表单输入
     */
    onRechargeFormInput(e: any) {
        const { field } = e.currentTarget.dataset;
        const { value } = e.detail;
        this.setData({
            [`rechargeForm.${field}`]: value
        });
    },

    /**
     * 创建充值规则
     */
    async createRechargeRule() {
        const { merchantId, rechargeForm } = this.data;

        // 表单验证
        if (rechargeForm.recharge_amount <= 0) {
            wx.showToast({ title: '请输入充值金额', icon: 'error' });
            return;
        }

        try {
            wx.showLoading({ title: '创建中...' });

            await RechargeRuleManagementService.createRechargeRule(merchantId, rechargeForm);

            wx.showToast({
                title: '创建成功',
                icon: 'success'
            });

            this.closeRechargeModal();
            this.loadRechargeRules();

        } catch (error: any) {
            wx.showToast({
                title: error.message || '创建失败',
                icon: 'error'
            });
        } finally {
            wx.hideLoading();
        }
    },

    /**
     * 删除充值规则
     */
    async deleteRechargeRule(e: any) {
        const ruleId = e.currentTarget.dataset.id;
        const { merchantId } = this.data;

        wx.showModal({
            title: '确认删除',
            content: '确定要删除此充值规则吗？',
            success: async (res) => {
                if (res.confirm) {
                    try {
                        wx.showLoading({ title: '删除中...' });

                        await RechargeRuleManagementService.deleteRechargeRule(merchantId, ruleId);

                        wx.showToast({
                            title: '删除成功',
                            icon: 'success'
                        });

                        this.loadRechargeRules();

                    } catch (error: any) {
                        wx.showToast({
                            title: error.message || '删除失败',
                            icon: 'error'
                        });
                    } finally {
                        wx.hideLoading();
                    }
                }
            }
        });
    },

    // ==================== 会员设置管理 ====================

    /**
     * 加载会员设置
     */
    async loadMembershipSettings() {
        try {
            const result = await MembershipSettingsService.getMembershipSettings();

            this.setData({ membershipSettings: result });

        } catch (error: any) {
            console.error('加载会员设置失败:', error);
            wx.showToast({
                title: '加载会员设置失败',
                icon: 'error'
            });
        }
    },

    /**
     * 显示编辑会员设置弹窗
     */
    showEditMembershipModal() {
        const { membershipSettings } = this.data;

        if (membershipSettings) {
            this.setData({
                showMembershipModal: true,
                membershipForm: {
                    balance_usage_scenarios: (membershipSettings as any).balance_usable_scenes || [],
                    bonus_usage_scenarios: (membershipSettings as any).bonus_usable_scenes || [],
                    allow_stacking_discounts: (membershipSettings as any).allow_stacking_discounts || true,
                    min_recharge_amount: (membershipSettings as any).min_recharge_amount || 1000
                }
            });
        }
    },

    /**
     * 关闭会员设置弹窗
     */
    closeMembershipModal() {
        this.setData({ showMembershipModal: false });
    },

    /**
     * 更新会员设置
     */
    async updateMembershipSettings() {
        const { membershipForm } = this.data;

        try {
            wx.showLoading({ title: '保存中...' });

            await MembershipSettingsService.updateMembershipSettings(membershipForm);

            wx.showToast({
                title: '保存成功',
                icon: 'success'
            });

            this.closeMembershipModal();
            this.loadMembershipSettings();

        } catch (error: any) {
            wx.showToast({
                title: error.message || '保存失败',
                icon: 'error'
            });
        } finally {
            wx.hideLoading();
        }
    },

    // ==================== 工具方法 ====================

    /**
     * 格式化金额
     */
    formatAmount(amount: number): string {
        return MarketingAdapter.formatAmount(amount);
    },

    /**
     * 格式化折扣类型
     */
    formatDiscountType(type: string): string {
        return MarketingAdapter.formatDiscountType(type);
    },

    /**
     * 格式化折扣值
     */
    formatDiscountValue(type: string, value: number): string {
        return MarketingAdapter.formatDiscountValue(type, value);
    },

    /**
     * 计算充值优惠比例
     */
    calculateBonusRate(rechargeAmount: number, bonusAmount: number): string {
        return MarketingAdapter.calculateBonusRate(rechargeAmount, bonusAmount);
    },

    /**
     * 获取优惠券状态文本
     */
    getVoucherStatusText(voucher: VoucherResponse): string {
        return MarketingAdapter.getVoucherStatusText(voucher);
    },

    /**
     * 获取优惠券状态颜色
     */
    getVoucherStatusColor(voucher: VoucherResponse): string {
        return MarketingAdapter.getVoucherStatusColor(voucher);
    },

    /**
     * 获取充值规则状态文本
     */
    getRechargeRuleStatusText(rule: RechargeRuleResponse): string {
        return MarketingAdapter.getRechargeRuleStatusText(rule);
    },

    /**
     * 格式化适用订单类型
     */
    formatOrderTypes(types: string[]): string {
        return MarketingAdapter.formatOrderTypes(types);
    },

    /**
     * 返回工作台
     */
    onBack() {
        wx.navigateBack({
            fail: () => {
                wx.redirectTo({ url: '/pages/merchant/dashboard/index' });
            }
        });
    }
});
