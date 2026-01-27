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
        countdown: 5,
        navBarHeight: 88
    },

    onNavHeight(e: any) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },

    onLoad(options: { order_id?: string; amount?: string; merchant_name?: string; table_number?: string }) {
        const { order_id, amount, merchant_name, table_number } = options;

        this.setData({
            orderId: parseInt(order_id || '0', 10) || 0,
            paymentAmount: parseFloat(amount || '0') || 0,
            merchantInfo: { name: merchant_name || '' },
            tableInfo: { table_number: table_number || '' }
        });

        // 开始倒计时
        this.startCountdown();
    },

    /**
     * 开始倒计时
     */
    startCountdown() {
        const timer = setInterval(() => {
            const { countdown } = this.data;
            if (countdown <= 1) {
                clearInterval(timer);
                this.goToOrderDetail();
            } else {
                this.setData({
                    countdown: countdown - 1
                });
            }
        }, 1000);
    },

    /**
     * 查看订单详情
     */
    goToOrderDetail() {
        wx.redirectTo({
            url: `/pages/orders/detail/index?id=${this.data.orderId}&type=dine_in`
        });
    },

    /**
     * 跳转到外卖页面
     */
    goToTakeout() {
        wx.switchTab({
            url: '/pages/takeout/index'
        });
    },

    /**
     * 跳转到包间预定页面
     */
    goToReservation() {
        wx.switchTab({
            url: '/pages/reservation/index'
        });
    },

    /**
     * 返回首页
     */
    goToHome() {
        wx.switchTab({
            url: '/pages/takeout/index'
        });
    }
});