/**
 * 充值活动促销组件
 * 用于在支付页面展示商户的充值活动，并提供充值入口
 */

import { getPublicRechargeRules, rechargeMembership, RechargeRuleResponse, getMyMemberships, MembershipResponse } from '../../api/personal'
import { formatPriceNoSymbol } from '../../utils/util'

interface RuleView extends RechargeRuleResponse {
    rechargeDisplay: string      // 充值金额展示
    bonusDisplay: string         // 赠送金额展示
    totalDisplay: string         // 到账金额展示
}

Component({
    properties: {
        /** 商户ID */
        merchantId: {
            type: Number,
            value: 0
        },
        /** 是否显示余额不足提示 */
        showInsufficientTip: {
            type: Boolean,
            value: false
        },
        /** 当前余额（分） */
        currentBalance: {
            type: Number,
            value: 0
        },
        /** 会员ID（如果有） */
        membershipId: {
            type: Number,
            value: 0
        }
    },

    data: {
        visible: false,
        loading: false,
        recharging: false,
        rules: [] as RuleView[],
        selectedRuleId: 0,
        selectedRechargeDisplay: ''
    },

    observers: {
        'merchantId': function(merchantId: number) {
            if (merchantId > 0) {
                this.loadRechargeRules()
            }
        }
    },

    lifetimes: {
        attached() {
            if (this.data.merchantId > 0) {
                this.loadRechargeRules()
            }
        }
    },

    methods: {
        /** 加载充值规则 */
        async loadRechargeRules() {
            const { merchantId } = this.data

            if (!merchantId) return

            this.setData({ loading: true, visible: true })

            try {
                const rules = await getPublicRechargeRules(merchantId)

                // 转换为展示数据
                const rulesView: RuleView[] = rules.map(rule => ({
                    ...rule,
                    rechargeDisplay: formatPriceNoSymbol(rule.recharge_amount),
                    bonusDisplay: formatPriceNoSymbol(rule.bonus_amount),
                    totalDisplay: formatPriceNoSymbol(rule.recharge_amount + rule.bonus_amount)
                }))

                // 默认选择第一个
                const selectedRuleId = rulesView.length > 0 ? rulesView[0].id : 0
                const selectedRechargeDisplay = rulesView.length > 0 ? rulesView[0].rechargeDisplay : ''

                this.setData({
                    rules: rulesView,
                    selectedRuleId,
                    selectedRechargeDisplay,
                    loading: false
                })
            } catch (err) {
                console.error('加载充值规则失败:', err)
                this.setData({ loading: false, visible: false })
            }
        },

        /** 选择充值规则 */
        onSelectRule(e: WechatMiniprogram.TouchEvent) {
            const { rule } = e.currentTarget.dataset as { rule?: RuleView }
            if (!rule) return

            this.setData({
                selectedRuleId: rule.id,
                selectedRechargeDisplay: rule.rechargeDisplay
            })
        },

        /** 点击充值按钮 */
        async onRecharge() {
            const { selectedRuleId, rules, recharging } = this.data
            let { membershipId } = this.data

            if (recharging || !selectedRuleId) return

            const selectedRule = rules.find(r => r.id === selectedRuleId)
            if (!selectedRule) return

            this.setData({ recharging: true })

            try {
                // 如果没有 membershipId，需要先获取或创建
                if (!membershipId) {
                    const membershipsResult = await getMyMemberships()
                    const membership = membershipsResult.memberships?.find(
                        (m: MembershipResponse) => m.merchant_id === this.data.merchantId
                    )
                    if (membership) {
                        membershipId = membership.id
                    } else {
                        // 用户还不是会员，需要先加入
                        wx.showToast({ title: '请先加入会员', icon: 'none' })
                        this.setData({ recharging: false })
                        return
                    }
                }

                // 调用充值接口
                const result = await rechargeMembership({
                    membership_id: membershipId,
                    payment_method: 'wechat',
                    recharge_amount: selectedRule.recharge_amount
                })

                if (result.pay_params) {
                    // 调起微信支付
                    wx.requestPayment({
                        ...result.pay_params,
                        success: () => {
                            wx.showToast({ title: '充值成功', icon: 'success' })
                            // 通知父组件刷新余额
                            this.triggerEvent('recharged', {
                                amount: selectedRule.recharge_amount,
                                bonus: selectedRule.bonus_amount
                            })
                        },
                        fail: (err: WechatMiniprogram.GeneralCallbackResult) => {
                            if (err.errMsg.includes('cancel')) {
                                wx.showToast({ title: '已取消支付', icon: 'none' })
                            } else {
                                wx.showToast({ title: '支付失败', icon: 'error' })
                            }
                        }
                    })
                }
            } catch (err) {
                console.error('充值失败:', err)
                wx.showToast({
                    title: err instanceof Error ? err.message : '充值失败',
                    icon: 'error'
                })
            } finally {
                this.setData({ recharging: false })
            }
        },

        /** 自定义充值（无活动时） */
        onCustomRecharge() {
            // 跳转到充值页面或打开充值弹窗
            this.triggerEvent('customRecharge', {
                merchantId: this.data.merchantId,
                membershipId: this.data.membershipId
            })
        }
    }
})
