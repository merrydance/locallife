/**
 * 商户 PC 端顶部工具栏组件
 */
Component({
    properties: {
        // 商户名称
        merchantName: {
            type: String,
            value: '我的店铺'
        },
        // 是否营业中
        isOpen: {
            type: Boolean,
            value: true
        },
        // 店主名称
        ownerName: {
            type: String,
            value: ''
        },
        // 店主头像
        avatarUrl: {
            type: String,
            value: ''
        },
        // 未读通知数
        unreadCount: {
            type: Number,
            value: 0
        },
        // 侧边栏宽度（用于定位）
        sidebarWidth: {
            type: Number,
            value: 0
        }
    },

    data: {
        todayDate: ''
    },

    lifetimes: {
        attached() {
            this.updateDate();
        }
    },

    methods: {
        /**
         * 更新今日日期
         */
        updateDate() {
            const now = new Date();
            const weekdays = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
            const month = now.getMonth() + 1;
            const day = now.getDate();
            const weekday = weekdays[now.getDay()];
            this.setData({
                todayDate: `${month}月${day}日 ${weekday}`
            });
        },

        /**
         * 切换营业状态
         */
        onToggleStatus() {
            this.triggerEvent('togglestatus');
        },

        /**
         * 通知按钮点击
         */
        onNotificationTap() {
            this.triggerEvent('notificationtap');
            wx.navigateTo({ url: '/pages/notifications/index' });
        },

        /**
         * 用户菜单点击
         */
        onUserMenuTap() {
            this.triggerEvent('usermenu');
            wx.showActionSheet({
                itemList: ['个人设置', '帮助中心', '退出登录'],
                success: (res) => {
                    if (res.tapIndex === 0) {
                        wx.navigateTo({ url: '/pages/user/settings/index' });
                    } else if (res.tapIndex === 2) {
                        this.triggerEvent('logout');
                    }
                }
            });
        }
    }
});
