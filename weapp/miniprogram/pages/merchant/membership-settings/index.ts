/**
 * 会员设置管理页面
 * 功能：储值使用场景设置、充值规则管理
 * 遵循 PC-SaaS 布局规范
 */

import { logger } from '@/utils/logger'
import {
    membershipSettingsService,
    rechargeRuleManagementService,
    type MembershipSettingsResponse,
    type RechargeRuleResponse,
    type UpdateMembershipSettingsRequest,
    type CreateRechargeRuleRequest,
    type UpdateRechargeRuleRequest,
    type UsableScene
} from '@/api/marketing-membership'

Page({
    data: {
        // 布局状态
        sidebarCollapsed: false,
        loading: true,
        saving: false,

        // 会员设置
        settings: {
            merchant_id: 0,
            balance_usable_scenes: [] as UsableScene[],
            bonus_usable_scenes: [] as UsableScene[],
            allow_with_voucher: true,
            allow_with_discount: true,
            max_deduction_percent: 100
        } as MembershipSettingsResponse,

        // 充值规则
        rechargeRules: [] as RechargeRuleResponse[],

        // 规则弹窗
        showRuleModal: false,
        editingRule: null as RechargeRuleResponse | null,
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
        merchantId: 0
    },

    async onLoad() {
        await this.initData()
    },

    async initData() {
        const app = getApp<IAppOption>()
        const merchantId = app.globalData.merchantId

        if (merchantId) {
            this.setData({ merchantId: Number(merchantId) })
            await this.loadData()
        } else {
            // 等待商户信息就绪
            app.userInfoReadyCallback = async () => {
                if (app.globalData.merchantId) {
                    this.setData({ merchantId: Number(app.globalData.merchantId) })
                    await this.loadData()
                }
            }
        }
    },

    onSidebarCollapse(e: WechatMiniprogram.CustomEvent) {
        this.setData({ sidebarCollapsed: e.detail.collapsed })
    },

    // ==================== 数据加载 ====================

    async loadData() {
        this.setData({ loading: true })
        try {
            await Promise.all([
                this.loadSettings(),
                this.loadRechargeRules()
            ])
        } catch (error) {
            logger.error('加载数据失败', error, 'membership-settings')
            wx.showToast({ title: '加载失败', icon: 'error' })
        } finally {
            this.setData({ loading: false })
        }
    },

    async loadSettings() {
        try {
            const settings = await membershipSettingsService.getMembershipSettings()
            this.setData({ settings })
        } catch (error) {
            logger.error('加载会员设置失败', error, 'membership-settings')
        }
    },

    async loadRechargeRules() {
        const { merchantId } = this.data
        if (!merchantId) return

        try {
            const rules = await rechargeRuleManagementService.listRechargeRules(merchantId)
            this.setData({ rechargeRules: rules })
        } catch (error) {
            logger.error('加载充值规则失败', error, 'membership-settings')
        }
    },

    // ==================== 会员设置操作 ====================

    onSceneToggle(e: WechatMiniprogram.TouchEvent) {
        const { scene } = e.currentTarget.dataset as { scene: UsableScene }
        const current = [...this.data.settings.balance_usable_scenes]

        const index = current.indexOf(scene)
        if (index > -1) {
            current.splice(index, 1)
        } else {
            current.push(scene)
        }

        this.setData({ 'settings.balance_usable_scenes': current })
    },

    async onSaveSettings() {
        const { settings } = this.data
        this.setData({ saving: true })

        try {
            const request: UpdateMembershipSettingsRequest = {
                balance_usable_scenes: settings.balance_usable_scenes
            }

            await membershipSettingsService.updateMembershipSettings(request)
            wx.showToast({ title: '保存成功', icon: 'success' })
        } catch (error) {
            logger.error('保存设置失败', error, 'membership-settings')
            wx.showToast({ title: '保存失败', icon: 'error' })
        } finally {
            this.setData({ saving: false })
        }
    },

    // ==================== 充值规则操作 ====================

    onAddRule() {
        // 默认有效期：今天到一年后
        const today = new Date()
        const nextYear = new Date()
        nextYear.setFullYear(nextYear.getFullYear() + 1)

        this.setData({
            showRuleModal: true,
            editingRule: null,
            ruleForm: {
                recharge_amount: '',
                bonus_amount: '',
                valid_from: this.formatDate(today),
                valid_until: this.formatDate(nextYear)
            }
        })
    },

    onEditRule(e: WechatMiniprogram.TouchEvent) {
        const rule = e.currentTarget.dataset.rule as RechargeRuleResponse
        this.setData({
            showRuleModal: true,
            editingRule: rule,
            ruleForm: {
                recharge_amount: String(rule.recharge_amount / 100),
                bonus_amount: String(rule.bonus_amount / 100),
                valid_from: rule.valid_from.slice(0, 10),
                valid_until: rule.valid_until.slice(0, 10)
            }
        })
    },

    onCloseRuleModal() {
        this.setData({ showRuleModal: false, editingRule: null })
    },

    onModalContentTap() {
        // 阻止冒泡
    },

    onRuleFormInput(e: WechatMiniprogram.Input) {
        const field = e.currentTarget.dataset.field as string
        this.setData({ [`ruleForm.${field}`]: e.detail.value })
    },

    onDateChange(e: WechatMiniprogram.PickerChange) {
        const field = e.currentTarget.dataset.field as string
        this.setData({ [`ruleForm.${field}`]: e.detail.value })
    },

    async onSaveRule() {
        const { merchantId, editingRule, ruleForm } = this.data

        // 验证
        const rechargeYuan = parseFloat(ruleForm.recharge_amount)
        const bonusYuan = parseFloat(ruleForm.bonus_amount)

        if (isNaN(rechargeYuan) || rechargeYuan <= 0) {
            wx.showToast({ title: '请输入充值金额', icon: 'none' })
            return
        }
        if (isNaN(bonusYuan) || bonusYuan < 0) {
            wx.showToast({ title: '请输入赠送金额', icon: 'none' })
            return
        }

        if (!ruleForm.valid_from || !ruleForm.valid_until) {
            wx.showToast({ title: '请选择有效期', icon: 'none' })
            return
        }

        wx.showLoading({ title: '保存中...' })

        try {
            if (editingRule) {
                // 更新
                const request: UpdateRechargeRuleRequest = {
                    recharge_amount: Math.round(rechargeYuan * 100),
                    bonus_amount: Math.round(bonusYuan * 100),
                    valid_from: ruleForm.valid_from + 'T00:00:00Z',
                    valid_until: ruleForm.valid_until + 'T23:59:59Z'
                }
                await rechargeRuleManagementService.updateRechargeRule(merchantId, editingRule.id, request)
            } else {
                // 创建
                const request: CreateRechargeRuleRequest = {
                    recharge_amount: Math.round(rechargeYuan * 100),
                    bonus_amount: Math.round(bonusYuan * 100),
                    valid_from: ruleForm.valid_from + 'T00:00:00Z',
                    valid_until: ruleForm.valid_until + 'T23:59:59Z'
                }
                await rechargeRuleManagementService.createRechargeRule(merchantId, request)
            }

            wx.hideLoading()
            wx.showToast({ title: '保存成功', icon: 'success' })
            this.setData({ showRuleModal: false })
            await this.loadRechargeRules()
        } catch (error) {
            wx.hideLoading()
            logger.error('保存规则失败', error, 'membership-settings')
            wx.showToast({ title: '保存失败', icon: 'error' })
        }
    },

    async onDeleteRule(e: WechatMiniprogram.TouchEvent) {
        const rule = e.currentTarget.dataset.rule as RechargeRuleResponse

        wx.showModal({
            title: '确认删除',
            content: `确定删除"充${rule.recharge_amount / 100}元送${rule.bonus_amount / 100}元"规则？`,
            success: async (res) => {
                if (res.confirm) {
                    wx.showLoading({ title: '删除中...' })
                    try {
                        await rechargeRuleManagementService.deleteRechargeRule(this.data.merchantId, rule.id)
                        wx.hideLoading()
                        wx.showToast({ title: '已删除', icon: 'success' })
                        await this.loadRechargeRules()
                    } catch (error) {
                        wx.hideLoading()
                        logger.error('删除规则失败', error, 'membership-settings')
                        wx.showToast({ title: '删除失败', icon: 'error' })
                    }
                }
            }
        })
    },

    async onToggleRuleStatus(e: WechatMiniprogram.TouchEvent) {
        const rule = e.currentTarget.dataset.rule as RechargeRuleResponse
        const newStatus = !rule.is_active

        try {
            await rechargeRuleManagementService.updateRechargeRule(
                this.data.merchantId,
                rule.id,
                { is_active: newStatus }
            )
            wx.showToast({ title: newStatus ? '已启用' : '已停用', icon: 'success' })
            await this.loadRechargeRules()
        } catch (error) {
            logger.error('更新规则状态失败', error, 'membership-settings')
            wx.showToast({ title: '操作失败', icon: 'error' })
        }
    },

    // ==================== 工具方法 ====================

    formatDate(date: Date): string {
        const y = date.getFullYear()
        const m = ('0' + (date.getMonth() + 1)).slice(-2)
        const d = ('0' + date.getDate()).slice(-2)
        return `${y}-${m}-${d}`
    }
})
