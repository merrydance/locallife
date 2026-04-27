/**
 * 堂食支付成功页面
 * 显示支付结果并引导用户使用其他服务
 */

Page({
    data: {
        orderId: 0,
        paymentAmount: 0,
        merchantInfo: null as { name: string } | null,
        tableInfo: null as { table_number: string } | null,
        navBarHeight: 88
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    },

    onLoad(options: { order_id?: string, amount?: string, merchant_name?: string, table_number?: string, confirmed?: string }) {
        const { order_id, amount, merchant_name, table_number, confirmed } = options
        const orderId = parseInt(order_id || '0', 10) || 0

        if (confirmed !== '1') {
            wx.redirectTo({
                url: `/pages/payment/result/index?status=pending_confirmation&businessId=${orderId}&businessType=order&amount=${encodeURIComponent(amount || '')}`
            })
            return
        }

        this.setData({
            orderId,
            paymentAmount: parseFloat(amount || '0') || 0,
            merchantInfo: merchant_name ? { name: merchant_name } : null,
            tableInfo: table_number ? { table_number } : null
        })
    },

    /**
     * 查看订单详情
     */
    goToOrderDetail() {
        wx.redirectTo({
            url: `/pages/orders/detail/index?id=${this.data.orderId}&type=dine_in`
        })
    },

    /**
     * 跳转到外卖页面
     */
    goToTakeout() {
        wx.switchTab({
            url: '/pages/takeout/index'
        })
    },

    /**
     * 跳转到包间预定页面
     */
    goToReservation() {
        wx.switchTab({
            url: '/pages/reservation/index'
        })
    },

    /**
     * 返回首页
     */
    goToHome() {
        wx.switchTab({
            url: '/pages/takeout/index'
        })
    }
})