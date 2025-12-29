/**
 * 商户 PC 端侧边栏导航组件
 */
Component({
  properties: {
    // 当前路径，用于高亮当前菜单项
    currentPath: {
      type: String,
      value: 'dashboard'
    },
    // 是否折叠
    collapsed: {
      type: Boolean,
      value: false
    },
    // 待处理订单数，用于显示徽章
    pendingOrderCount: {
      type: Number,
      value: 0
    }
  },

  data: {
    // 运营模块菜单
    operationMenus: [
      { name: '工作台', path: 'dashboard', icon: '/assets/icons/menu/dashboard.png' },
      { name: '订单管理', path: 'orders', icon: '/assets/icons/menu/order.png', badge: 0 },
      { name: '厨房KDS', path: 'kitchen', icon: '/assets/icons/menu/kitchen.png' },
      { name: '桌台管理', path: 'tables', icon: '/assets/icons/menu/table.png' },
      { name: '预订管理', path: 'reservations', icon: '/assets/icons/menu/calendar.png' }
    ],
    // 商品模块菜单
    productMenus: [
      { name: '菜品管理', path: 'dishes', icon: '/assets/icons/menu/dish.png' },
      { name: '套餐管理', path: 'combos', icon: '/assets/icons/menu/combo.png' },
      { name: '库存管理', path: 'inventory', icon: '/assets/icons/menu/inventory.png' }
    ],
    // 营销模块菜单
    marketingMenus: [
      { name: '优惠券', path: 'vouchers', icon: '/assets/icons/menu/voucher.png' },
      { name: '折扣活动', path: 'discounts', icon: '/assets/icons/menu/discount.png' },
      { name: '会员管理', path: 'members', icon: '/assets/icons/menu/member.png' },
      { name: '储值管理', path: 'membership-settings', icon: '/assets/icons/menu/member.png' }
    ],
    // 数据模块菜单
    dataMenus: [
      { name: '经营统计', path: 'analytics', icon: '/assets/icons/menu/chart.png' },
      { name: '财务管理', path: 'finance', icon: '/assets/icons/menu/finance.png' }
    ],
    // 设置模块菜单
    settingMenus: [
      { name: '设置中心', path: 'settings', icon: '/assets/icons/menu/settings.png' }
    ]
  },

  observers: {
    'pendingOrderCount': function (count: number) {
      // 更新订单菜单的徽章
      const operationMenus = [...this.data.operationMenus];
      const orderMenu = operationMenus.find(m => m.path === 'orders');
      if (orderMenu) {
        orderMenu.badge = count > 0 ? count : 0;
        this.setData({ operationMenus });
      }
    }
  },

  methods: {
    /**
     * 菜单点击
     */
    onMenuTap(e: WechatMiniprogram.TouchEvent) {
      const path = e.currentTarget.dataset.path as string;
      this.triggerEvent('menuchange', { path });

      // 跳转到对应页面
      const pageMap: Record<string, string> = {
        dashboard: '/pages/merchant/dashboard/index',
        orders: '/pages/merchant/orders/index',
        kitchen: '/pages/merchant/kds/index',
        tables: '/pages/merchant/tables/index',
        reservations: '/pages/merchant/reservations/index',
        dishes: '/pages/merchant/dishes/index',
        combos: '/pages/merchant/combos/index',
        inventory: '/pages/merchant/inventory/index',
        vouchers: '/pages/merchant/vouchers/index',
        discounts: '/pages/merchant/discounts/index',
        members: '/pages/merchant/members/index',
        'membership-settings': '/pages/merchant/membership-settings/index',
        analytics: '/pages/merchant/analytics/index',
        finance: '/pages/merchant/finance/index',
        settings: '/pages/merchant/settings/index',
        profile: '/pages/merchant/profile/index',
        printers: '/pages/merchant/printers/index',
        review: '/pages/merchant/review/index'
      };

      const targetPage = pageMap[path];
      if (targetPage && path !== this.data.currentPath) {
        wx.redirectTo({ url: targetPage });
      }
    },

    /**
     * Logo 点击 - 回到工作台
     */
    onLogoTap() {
      if (this.data.currentPath !== 'dashboard') {
        wx.redirectTo({ url: '/pages/merchant/dashboard/index' });
      }
    },

    /**
     * 折叠/展开切换
     */
    onCollapseTap() {
      this.triggerEvent('collapse', { collapsed: !this.properties.collapsed });
    }
  }
});
