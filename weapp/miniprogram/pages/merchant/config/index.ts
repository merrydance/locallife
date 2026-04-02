import { getStableBarHeights } from '../../../utils/responsive'

interface ConfigItem {
  id: string
  title: string
  desc: string
  path: string
}

interface ConfigSection {
  id: string
  title: string
  desc: string
  items: ConfigItem[]
}

const CONFIG_SECTIONS: ConfigSection[] = [
  {
    id: 'menu',
    title: '商品与菜单',
    desc: '处理菜品、菜品分类与套餐等日常菜单维护能力。',
    items: [
      { id: 'dishes', title: '菜品管理', desc: '上架、下架、编辑门店菜品与规格信息', path: '/pages/merchant/dishes/index' },
      { id: 'dish-categories', title: '菜品分类', desc: '维护菜单分类、排序和展示结构', path: '/pages/merchant/dishes/categories/index' },
      { id: 'combos', title: '套餐管理', desc: '维护套餐内容、价格和售卖状态', path: '/pages/merchant/combos/index' }
    ]
  },
  {
    id: 'shop',
    title: '店铺与资料',
    desc: '处理门店基础资料、图资、类目和营业规则。',
    items: [
      { id: 'profile', title: '店铺资料', desc: '维护店铺名称、联系电话、地址与介绍', path: '/pages/merchant/settings/profile/index' },
      { id: 'profile-images', title: '店铺图片管理', desc: '更新 Logo、门头照、环境照', path: '/pages/merchant/profile-images/index' },
      { id: 'merchant-categories', title: '经营类目设置', desc: '维护店铺经营类目与平台分类归属，不同于菜单里的菜品分类', path: '/pages/merchant/merchant-categories/index' },
      { id: 'business-hours', title: '营业时间', desc: '维护每周营业时段并保留特殊日期安排', path: '/pages/merchant/settings/business-hours/index' },
      { id: 'membership', title: '会员设置', desc: '配置余额与赠送金可用场景及叠加规则', path: '/pages/merchant/settings/membership/index' },
      { id: 'recharge-rules', title: '充值规则', desc: '维护会员充值赠送活动、有效期与启停状态', path: '/pages/merchant/settings/recharge-rules/index' },
      { id: 'packaging-policy', title: '包装费策略', desc: '配置外卖与自取订单的包装菜品候选范围', path: '/pages/merchant/settings/packaging-policy/index' }
    ]
  },
  {
    id: 'compliance',
    title: '主体与结算',
    desc: '处理主体申请、收付通进件、签约与银行结算资料。',
    items: [
      { id: 'application', title: '主体申请', desc: '维护主体申请草稿、上传证照并提交审核', path: '/pages/merchant/settings/application/index' },
      { id: 'applyment', title: '收付通进件', desc: '查看进件状态、复制签约链接并重提银行结算资料', path: '/pages/merchant/settings/applyment/index' },
      { id: 'finance', title: '资金账户', desc: '查看账户开通状态、余额、提现与结算洞察', path: '/pages/merchant/finance/index' }
    ]
  },
  {
    id: 'device',
    title: '设备与展示',
    desc: '处理门店展示、打印、后厨分发和桌台设备配置。',
    items: [
      { id: 'display-config', title: '显示与打印设置', desc: '统一维护打印、语音播报和 KDS 分发开关', path: '/pages/merchant/settings/display-config/index' },
      { id: 'printers', title: '打印机管理', desc: '添加、配置云打印机设备', path: '/pages/merchant/printers/index' },
      { id: 'print-anomalies', title: '打印异常列表', desc: '查看异常打印任务并快速重试补打', path: '/pages/merchant/orders/print-anomalies/index' },
      { id: 'tables', title: '桌台管理', desc: '桌台信息、二维码和图片维护', path: '/pages/merchant/tables/index' }
    ]
  },
  {
    id: 'marketing',
    title: '营销与拉新',
    desc: '处理适合长期维护的营销配置能力，不承载日常订单动作。',
    items: [
      { id: 'discount-rules', title: '满减规则', desc: '维护门店订单满减活动及叠加规则', path: '/pages/merchant/discount-rules/index' },
      { id: 'delivery-promotions', title: '配送优惠', desc: '维护满减配送活动与配送优惠策略', path: '/pages/merchant/delivery-promotions/index' },
      { id: 'vouchers', title: '代金券管理', desc: '维护代金券金额、门槛、有效期和适用订单类型', path: '/pages/merchant/vouchers/index' }
    ]
  },
  {
    id: 'collaboration',
    title: '组织与协作',
    desc: '处理门店成员与协作入口，不承载日常经营动作。',
    items: [
      { id: 'reviews', title: '评价管理', desc: '统一查看顾客评价并跟进反馈内容', path: '/pages/merchant/reviews/index' },
      { id: 'group-application', title: '集团 / 品牌入驻', desc: '为连锁门店或品牌总部提交集团入驻资料并查看审核状态', path: '/pages/merchant/group/application/index' },
      { id: 'group-join', title: '申请加入集团', desc: '搜索已有集团或品牌，并提交门店加入申请', path: '/pages/merchant/group/join/index' },
      { id: 'staff', title: '员工管理', desc: '查看员工名单、生成邀请码、分配角色与移除员工', path: '/pages/merchant/staff/index' }
    ]
  }
]

Page({
  data: {
    navBarHeight: 88,
    configSections: CONFIG_SECTIONS
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
