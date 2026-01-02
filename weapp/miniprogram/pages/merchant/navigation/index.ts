/**
 * 商户应用导航页
 * 桌面级 Grid 布局，展示所有可用模块
 */
Page({
    data: {
        groups: [
            {
                title: '日常运营',
                items: [
                    { name: '订单处理', icon: '📋', url: '/pages/merchant/orders/index', color: '#1890ff' },
                    { name: '桌台管理', icon: '🪑', url: '/pages/merchant/tables/index', color: '#13c2c2' },
                    { name: '预订管理', icon: '📅', url: '/pages/merchant/reservations/index', color: '#722ed1' },
                    { name: '堂食设置', icon: '🍽️', url: '/pages/merchant/dinein/index', color: '#eb2f96' },
                    { name: '后厨显示', icon: '🍳', url: '/pages/merchant/kds/index', color: '#fa8c16' }
                ]
            },
            {
                title: '商品与库存',
                items: [
                    { name: '菜品管理', icon: '🍜', url: '/pages/merchant/dishes/index', color: '#52c41a' },
                    { name: '套餐管理', icon: '🎁', url: '/pages/merchant/combos/index', color: '#a0d911' },
                    { name: '库存管理', icon: '📦', url: '/pages/merchant/inventory/index', color: '#fadb14' }
                ]
            },
            {
                title: '营销推广',
                items: [
                    { name: '代金券', icon: '🎫', url: '/pages/merchant/vouchers/index', color: '#ff4d4f' },
                    { name: '限时折扣', icon: '🏷️', url: '/pages/merchant/discounts/index', color: '#ff7a45' }
                ]
            },
            {
                title: '客户与评价',
                items: [
                    { name: '会员管理', icon: '👥', url: '/pages/merchant/members/index', color: '#2f54eb' },
                    { name: '会员设置', icon: '💳', url: '/pages/merchant/membership-settings/index', color: '#1d39c4' },
                    { name: '评价管理', icon: '💬', url: '/pages/merchant/review/manage/index', color: '#faad14' }
                ]
            },
            {
                title: '经营管理',
                items: [
                    { name: '经营分析', icon: '📊', url: '/pages/merchant/analytics/index', color: '#722ed1' },
                    { name: '财务管理', icon: '💰', url: '/pages/merchant/finance/index', color: '#52c41a' },
                    { name: '运费减免', icon: '🚚', url: '/pages/merchant/delivery-settings/index', color: '#13c2c2' },
                    { name: '经营健康', icon: '💊', url: '/pages/merchant/health/index', color: '#ff4d4f' },
                    { name: '商户设置', icon: '⚙️', url: '/pages/merchant/settings/index', color: '#8c8c8c' }
                ]
            }
        ]
    },

    onLoad() {
        // 可以在这里加载权限控制逻辑，动态过滤显示的模块
    },

    navigateTo(e: any) {
        const url = e.currentTarget.dataset.url
        if (url) {
            wx.navigateTo({ url })
        } else {
            wx.showToast({ title: '功能开发中', icon: 'none' })
        }
    }
})
