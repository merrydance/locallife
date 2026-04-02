/**
 * 商户优惠中心组件
 * 展示商户所有优惠活动：满减、配送优惠、优惠券、充值活动
 * 可用于所有支付页面
 */

import { rechargeMembership, getMyMemberships, MembershipResponse } from '../../api/personal'
import { formatPriceNoSymbol } from '../../utils/util'
import { request } from '../../utils/request'
import { getErrorDebugMessage, getErrorUserMessage } from '../../utils/user-facing'

/** 优惠项目类型 - 对齐后端 api.promotionItem */
interface PromotionItem {
    type: string           // delivery_fee_return, discount, voucher, recharge
    title: string          // 优惠标题
    description: string    // 优惠描述
    min_amount: number     // 起点金额（分）
    value: number          // 优惠金额或比例
    bonus_amount: number   // 赠送金额(充值活动用)
    valid_until: string    // 有效期
    rule_id: number        // 规则ID（充值活动用）
    displayTitle?: string  // 展现用的格式化标题
}

/** 商户优惠响应 */
interface MerchantPromotionsResponse {
    merchant_id: number
    delivery_fee_rules: PromotionItem[]
    discount_rules: PromotionItem[]
    vouchers: PromotionItem[]
    recharge_rules: PromotionItem[]
}

/** 展示用的充值项 */
interface RechargeView extends PromotionItem {
    rechargeDisplay: string
    bonusDisplay: string
    totalDisplay: string
    minAmountDisplay: string
    valueDisplay: string
}

/** 展示用的优惠券项 */
interface VoucherView extends PromotionItem {
    minAmountDisplay: string
    valueDisplay: string
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
        },
        /** 订单应付总额（分），用于显示余额充足提示 */
        orderTotal: {
            type: Number,
            value: 0
        }
    },

    data: {
        visible: false,
        loading: false,
        recharging: false,
        hasPromos: false,
        totalCount: 0,
        
        // 各类优惠
        discountRules: [] as PromotionItem[],
        deliveryFeeRules: [] as PromotionItem[],
        vouchers: [] as VoucherView[],
        rechargeRules: [] as RechargeView[],
        
        // 充值选择
        selectedRechargeId: 0,
        selectedRechargeDisplay: '',
        balanceDisplay: '0.00'
    },

    observers: {
        'merchantId'(merchantId: number) {
            if (merchantId > 0) {
                this.loadPromotions()
            }
        },
        'currentBalance'(balance: number) {
            this.setData({
                balanceDisplay: formatPriceNoSymbol(balance)
            })
        }
    },

    lifetimes: {
        attached() {
            if (this.data.merchantId > 0) {
                this.loadPromotions()
            }
        }
    },

    methods: {
        /** 加载商户优惠活动 */
        async loadPromotions() {
            const { merchantId } = this.data

            if (!merchantId) return

            this.setData({ loading: true, visible: true })

            try {
                // 调用聚合API获取所有优惠
                const result = await request<MerchantPromotionsResponse>({
                    url: `/v1/merchants/${merchantId}/promotions`,
                    method: 'GET'
                })

                if (!result) {
                    this.setData({ loading: false })
                    return
                }

                // 客户端过期过滤（防止用户长时间停留页面，期间活动过期）
                const isNotExpired = (item: PromotionItem): boolean => {
                    if (!item.valid_until) return true // 无有效期则永久有效
                    const expireDate = new Date(item.valid_until + 'T23:59:59')
                    return expireDate >= new Date()
                }

                // 处理满减规则（过滤已过期）
                const discountRules = (result.discount_rules || []).filter(isNotExpired)
                
                // 处理配送优惠（过滤已过期）
                const deliveryFeeRules = (result.delivery_fee_rules || []).filter(isNotExpired).map((rule) => {
                    const valueStr = formatPriceNoSymbol(rule.value)
                    return {
                        ...rule,
                        displayTitle: rule.value > 0 ? `${rule.title} ¥${valueStr}` : rule.title
                    }
                })
                
                // 处理优惠券（过滤已过期，改为规则样式展示）
                const vouchers: VoucherView[] = (result.vouchers || [])
                    .filter(isNotExpired)
                    .map((v) => {
                        const minStr = formatPriceNoSymbol(v.min_amount)
                        const valStr = formatPriceNoSymbol(v.value)
                        return {
                            ...v,
                            minAmountDisplay: minStr,
                            valueDisplay: valStr,
                            displayTitle: `代金券 满${minStr}减${valStr}`
                        }
                    })
                
                // 处理充值规则（过滤已过期）
                const rechargeRules: RechargeView[] = (result.recharge_rules || [])
                    .filter(isNotExpired)
                    .map((r) => ({
                        ...r,
                        rechargeDisplay: formatPriceNoSymbol(r.min_amount),
                        bonusDisplay: formatPriceNoSymbol(r.bonus_amount),
                        totalDisplay: formatPriceNoSymbol(r.min_amount + r.bonus_amount),
                        minAmountDisplay: formatPriceNoSymbol(r.min_amount),
                        valueDisplay: formatPriceNoSymbol(r.value)
                    }))

                // 计算总数（过滤后）
                const totalCount = discountRules.length + deliveryFeeRules.length + 
                                   vouchers.length + rechargeRules.length
                const hasPromos = totalCount > 0

                // 默认选择第一个充值项
                const selectedRechargeId = rechargeRules.length > 0 ? rechargeRules[0].rule_id : 0
                const selectedRechargeDisplay = rechargeRules.length > 0 ? rechargeRules[0].rechargeDisplay : ''

                this.setData({
                    discountRules,
                    deliveryFeeRules,
                    vouchers,
                    rechargeRules,
                    totalCount,
                    hasPromos,
                    selectedRechargeId,
                    selectedRechargeDisplay,
                    loading: false,
                    visible: true // 始终显示，无优惠时展示空状态
                })
            } catch (err) {
                console.error('加载优惠活动失败:', err)
                this.setData({ loading: false, visible: false }) // 仅加载失败时隐藏
            }
        },

        /** 选择充值规则 */
        onSelectRecharge(e: WechatMiniprogram.TouchEvent) {
            const { rule } = e.currentTarget.dataset as { rule?: RechargeView }
            if (!rule) return

            this.setData({
                selectedRechargeId: rule.rule_id,
                selectedRechargeDisplay: rule.rechargeDisplay
            })
        },

        /** 点击充值按钮 */
        async onRecharge() {
            const { selectedRechargeId, rechargeRules, recharging } = this.data
            let { membershipId } = this.data

            if (recharging || !selectedRechargeId) return

            const selectedRule = rechargeRules.find((r) => r.rule_id === selectedRechargeId)
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
                    recharge_amount: selectedRule.min_amount  // min_amount 就是充值金额
                })

                if (result.pay_params) {
                    // 调起微信支付
                    wx.requestPayment({
                        ...result.pay_params,
                        success: () => {
                            wx.showToast({ title: '充值成功', icon: 'success' })
                            // 通知父组件刷新余额
                            this.triggerEvent('recharged', {
                                amount: selectedRule.min_amount,
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
                    title: getErrorUserMessage(err, '充值失败，请稍后重试'),
                    icon: 'none'
                })
            } finally {
                this.setData({ recharging: false })
            }
        },

        /** 领取优惠券 */
        async onClaim(e: WechatMiniprogram.TouchEvent) {
            const { voucher } = e.currentTarget.dataset as { voucher?: VoucherView & { voucher_id?: number } }
            if (!voucher) return

            // 注意：vouchers 列表中的优惠券是商户发行的优惠券模板，需要领取后才能使用
            // rule_id 在这里对应的是 voucher_id（优惠券模板ID）
            const voucherId = voucher.rule_id

            if (!voucherId) {
                // 如果没有ID，只触发事件让父组件处理
                this.triggerEvent('claimVoucher', { voucher })
                return
            }

            try {
                // 调用领券API
                await request({
                    url: `/v1/vouchers/${voucherId}/claim`,
                    method: 'POST'
                })

                wx.showToast({
                    title: '领取成功',
                    icon: 'success'
                })

                // 通知父组件刷新
                this.triggerEvent('voucherClaimed', {
                    voucher,
                    voucherId
                })

            } catch (err) {
                const debugMessage = getErrorDebugMessage(err).toLowerCase()
                const errorMsg = getErrorUserMessage(err, '领取失败，请稍后重试')
                
                // 已领取的情况
                if (debugMessage.includes('already') || errorMsg.includes('已领取')) {
                    wx.showToast({ title: '您已领取过该优惠券', icon: 'none' })
                } else if (debugMessage.includes('expired') || errorMsg.includes('过期')) {
                    wx.showToast({ title: '优惠券已过期', icon: 'none' })
                } else if (debugMessage.includes('out') || errorMsg.includes('领完')) {
                    wx.showToast({ title: '优惠券已领完', icon: 'none' })
                } else {
                    wx.showToast({ title: errorMsg, icon: 'none' })
                }
            }
        }
    }
})
