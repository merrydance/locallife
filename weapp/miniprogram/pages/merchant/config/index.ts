import { getStableBarHeights } from '../../../utils/responsive'

interface ConfigItem {
  id: string
  title: string
  desc: string
  path: string
}

Page({
  data: {
    navBarHeight: 88,
    configItems: [
      { id: 'claims', title: '索赔与追偿', desc: '查看责任判定、提交异议、支付平台垫付款', path: '/pages/merchant/claims/index' },
      { id: 'dishes', title: '菜品管理', desc: '新增、编辑、上下架菜品', path: '/pages/merchant/dishes/index' },
      { id: 'dish-categories', title: '菜品分类管理', desc: '维护分类与排序', path: '/pages/merchant/dishes/categories/index' },
      { id: 'combos', title: '套餐管理', desc: '维护套餐组合与上架状态', path: '/pages/merchant/combos/index' },
      { id: 'inventory', title: '当日库存设置', desc: '按日调整菜品库存', path: '/pages/merchant/inventory/index' },
      { id: 'tables', title: '桌台管理', desc: '桌台信息与二维码维护', path: '/pages/merchant/tables/index' },
      { id: 'merchant-categories', title: '经营类目设置', desc: '选择店铺经营类目，影响首页分类筛选排名 ⚡', path: '/pages/merchant/merchant-categories/index' },
      { id: 'profile-images', title: '店铺图片管理', desc: '更新 Logo、门头照、环境照', path: '/pages/merchant/profile-images/index' },
      { id: 'delivery-promotions', title: '配送优惠管理', desc: '配置满减免运费等配送优惠活动', path: '/pages/merchant/delivery-promotions/index' },
      { id: 'printers', title: '打印机管理', desc: '添加、配置云打印机设备', path: '/pages/merchant/printers/index' },
      { id: 'finance', title: '银行账户 / 收款设置', desc: '绑定银行账户，用于分账收款与提现', path: '/pages/merchant/finance/index' }
    ] as ConfigItem[]
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
  },

  onTapItem(e: WechatMiniprogram.TouchEvent) {
    const { path } = e.currentTarget.dataset as { path?: string }
    if (!path) return
    wx.navigateTo({ url: path })
  }
})
