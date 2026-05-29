/**
 * 商户优惠中心组件
 * 展示商户所有优惠活动：满减、代取优惠、优惠券、充值活动
 * 可用于所有支付页面
 */

import { claimVoucher } from '../../../../../api/personal'
import { getMerchantPromotionCenter, MerchantPromotionCenterResponse } from '../../../../../api/merchant'
import { formatPriceNoSymbol } from '../../../../../utils/util'
import { getErrorDebugMessage, getErrorUserMessage } from '../../../../../utils/user-facing'

const MEMBERSHIP_RECHARGE_PAUSED_MESSAGE = '会员线上充值已暂停，请联系商户线下充值后入账。'

function showMembershipRechargePausedMessage() {
    wx.showModal({
        title: '线上充值已暂停',
        content: MEMBERSHIP_RECHARGE_PAUSED_MESSAGE,
        showCancel: false,
        confirmText: '我知道了'
    })
}

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
        showRechargePaused: false,
        balanceDisplay: '0.00',
        rechargePausedMessage: MEMBERSHIP_RECHARGE_PAUSED_MESSAGE
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
                const result = await getMerchantPromotionCenter(merchantId) as MerchantPromotionCenterResponse

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
                
                // 处理代取优惠（过滤已过期）
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
                
                const hasRechargeRules = (result.recharge_rules || []).filter(isNotExpired).length > 0

                // 计算总数（过滤后）
                const totalCount = discountRules.length + deliveryFeeRules.length + 
                                   vouchers.length + (hasRechargeRules ? 1 : 0)
                const hasPromos = totalCount > 0

                this.setData({
                    discountRules,
                    deliveryFeeRules,
                    vouchers,
                    showRechargePaused: hasRechargeRules,
                    totalCount,
                    hasPromos,
                    loading: false,
                    visible: true // 始终显示，无优惠时展示空状态
                })
            } catch (err) {
                console.error('加载优惠活动失败:', err)
                this.setData({ loading: false, visible: false }) // 仅加载失败时隐藏
            }
        },

        /** 点击充值按钮 */
        onRecharge() {
            showMembershipRechargePausedMessage()
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
                await claimVoucher(voucherId)

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
