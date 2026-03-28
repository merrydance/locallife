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
    id: 'shop',
    title: '店铺与资料',
    desc: '处理门店基础资料、图资、类目和营业规则。',
    items: [
      { id: 'profile', title: '店铺资料', desc: '维护店铺名称、联系电话、地址与介绍', path: '/pages/merchant/settings/profile/index' },
      { id: 'profile-images', title: '店铺图片管理', desc: '更新 Logo、门头照、环境照', path: '/pages/merchant/profile-images/index' },
      { id: 'merchant-categories', title: '经营类目设置', desc: '维护店铺经营类目与平台分类归属', path: '/pages/merchant/merchant-categories/index' },
      { id: 'business-hours', title: '营业时间', desc: '维护每周营业时段并保留特殊日期安排', path: '/pages/merchant/settings/business-hours/index' },
      { id: 'membership', title: '会员设置', desc: '配置余额与赠送金可用场景及叠加规则', path: '/pages/merchant/settings/membership/index' }
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
      { id: 'tables', title: '桌台管理', desc: '桌台信息、二维码和图片维护', path: '/pages/merchant/tables/index' }
    ]
  },
  {
    id: 'collaboration',
    title: '组织与协作',
    desc: '处理门店成员与协作入口，不承载日常经营动作。',
    items: [
      { id: 'staff', title: '员工管理', desc: '查看员工名单、生成邀请码、分配角色与移除员工', path: '/pages/merchant/staff/index' },
      { id: 'dashboard', title: '返回工作台', desc: '订单、预订、投诉等日常经营动作统一回到工作台处理', path: '/pages/merchant/dashboard/index' }
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

    if (path === '/pages/merchant/dashboard/index') {
      wx.redirectTo({ url: path })
      return
    }

    wx.navigateTo({ url: path })
  }
})
