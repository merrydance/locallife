/**
 * 商户管理中心
 * 功能入口汇总页
 */

Page({
    data: {
        navBarHeight: 0
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent) {
        this.setData({ navBarHeight: e.detail.height })
    },

    // ========== 日常运营 ==========
    goToOrders() {
        wx.navigateTo({ url: '/pages/merchant/orders/index' })
    },

    goToDishes() {
        wx.navigateTo({ url: '/pages/merchant/dishes/index' })
    },

    goToTables() {
        wx.navigateTo({ url: '/pages/merchant/tables/manage/manage' })
    },

    goToInventory() {
        wx.navigateTo({ url: '/pages/merchant/inventory/index' })
    },

    goToDinein() {
        wx.navigateTo({ url: '/pages/merchant/dinein/index' })
    },

    // ========== 后厨与设备 ==========
    goToKitchen() {
        wx.navigateTo({ url: '/pages/merchant/kds/index' })
    },

    goToPrinters() {
        wx.navigateTo({ url: '/pages/merchant/printers/index' })
    },

    // ========== 数据分析 ==========
    goToAnalytics() {
        wx.navigateTo({ url: '/pages/merchant/analytics/index' })
    },

    goToFinance() {
        wx.navigateTo({ url: '/pages/merchant/finance/settlement' })
    },

    // ========== 营销推广 ==========
    goToMarketing() {
        wx.navigateTo({ url: '/pages/merchant/marketing/index' })
    },

    goToReview() {
        wx.navigateTo({ url: '/pages/merchant/review/index' })
    },

    // ========== 预约管理 ==========
    goToReservations() {
        wx.navigateTo({ url: '/pages/merchant/reservations/index' })
    },

    // ========== 店铺设置 ==========
    goToProfile() {
        wx.navigateTo({ url: '/pages/merchant/profile/index' })
    },

    goToHealth() {
        wx.navigateTo({ url: '/pages/merchant/health/index' })
    }
})
