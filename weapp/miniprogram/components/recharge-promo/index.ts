/**
 * 充值活动促销组件
 * 用于在支付页面展示商户的充值活动，并提供充值入口
 */

import { MEMBERSHIP_RECHARGE_PAUSED_MESSAGE, showMembershipRechargePausedMessage } from '../../utils/membership-recharge-pause'

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
        pausedMessage: MEMBERSHIP_RECHARGE_PAUSED_MESSAGE
    },

    observers: {
        'merchantId'(merchantId: number) {
            if (merchantId > 0) {
                this.showPausedEntry()
            }
        }
    },

    lifetimes: {
        attached() {
            if (this.data.merchantId > 0) {
                this.showPausedEntry()
            }
        }
    },

    methods: {
        showPausedEntry() {
            const { merchantId } = this.data
            if (!merchantId) return

            this.setData({
                visible: true,
                loading: false
            })
        },

        /** 点击充值按钮 */
        onRecharge() {
            showMembershipRechargePausedMessage()
        },

        /** 自定义充值（无活动时） */
        onCustomRecharge() {
            showMembershipRechargePausedMessage()
        }
    }
})
