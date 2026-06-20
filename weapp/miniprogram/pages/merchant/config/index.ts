import { getStableBarHeights } from '../../../utils/responsive'
import {
  canManageMerchantApplyment,
  canUseMerchantDeviceManagementFallback,
  ensureMerchantConsoleAccess,
  getRecentMerchantDeviceAccess
} from '../../../utils/console-access'
import { logger } from '../../../utils/logger'

interface ConfigItem {
  id: string
  title: string
  desc: string
  path: string
}

interface ConfigGroup {
  id: string
  title: string
  desc: string
  items: ConfigItem[]
}

interface ConfigSection {
  id: string
  title: string
  desc: string
  items?: ConfigItem[]
  groups?: ConfigGroup[]
}

const CONFIG_SECTIONS: ConfigSection[] = [
  {
    id: 'operations',
    title: '经营与菜单',
    desc: '处理菜品、库存、桌台与菜单结构等日常经营配置。',
    items: [
      { id: 'dishes', title: '菜品管理', desc: '维护菜品、规格和上下架状态', path: '/pages/merchant/dishes/index' },
      { id: 'dish-categories', title: '菜品分类', desc: '维护菜单分类、排序和展示结构', path: '/pages/merchant/dishes/categories/index' },
      { id: 'packaging', title: '包装设置', desc: '维护包装规则、适用订单和包装费用', path: '/pages/merchant/packaging/index' },
      { id: 'combos', title: '套餐管理', desc: '维护套餐内容、价格和售卖状态', path: '/pages/merchant/combos/index' },
      { id: 'inventory', title: '库存管理', desc: '维护库存状态、余量和相关经营开关', path: '/pages/merchant/inventory/index' },
      { id: 'tables', title: '桌台管理', desc: '维护桌台信息、图片和二维码能力', path: '/pages/merchant/tables/index' }
    ]
  },
  {
    id: 'store',
    title: '店铺资料',
    desc: '维护门店基础资料、图资与营业规则。',
    items: [
      { id: 'profile', title: '店铺资料', desc: '维护店铺名称、联系电话、地址、经营类目与介绍', path: '/pages/merchant/settings/profile/index' },
      { id: 'profile-images', title: '店铺图片管理', desc: '维护 Logo、门头照和环境照', path: '/pages/merchant/profile-images/index' },
      { id: 'business-hours', title: '营业时间', desc: '维护营业时段与特殊日期安排', path: '/pages/merchant/settings/business-hours/index' }
    ]
  },
  {
    id: 'growth',
    title: '会员与营销',
    desc: '维护会员权益、充值活动和营销规则。',
    items: [
      { id: 'membership', title: '叠加规则', desc: '配置余额与赠送金可用场景及叠加规则', path: '/pages/merchant/settings/membership/index' },
      { id: 'recharge-rules', title: '充值规则', desc: '维护会员充值赠送活动与有效期', path: '/pages/merchant/settings/recharge-rules/index' },
      { id: 'discount-rules', title: '满减规则', desc: '维护订单满减活动与叠加规则', path: '/pages/merchant/discount-rules/index' },
      { id: 'delivery-promotions', title: '代取优惠', desc: '维护代取活动与优惠策略', path: '/pages/merchant/delivery-promotions/index' },
      { id: 'vouchers', title: '代金券管理', desc: '维护代金券金额、门槛与适用订单类型', path: '/pages/merchant/vouchers/index' }
    ]
  },
  {
    id: 'compliance',
    title: '主体与结算',
    desc: '处理主体申请、宝付开户与资金账户相关设置。',
    items: [
      { id: 'application', title: '主体申请', desc: '维护主体申请草稿、上传证照并提交审核', path: '/pages/merchant/settings/application/index' },
      { id: 'baofu-settlement-account', title: '结算账户', desc: '查看和维护收款、分账所需的账户状态', path: '/pages/merchant/finance/settlement-account/index' }
    ]
  },
  {
    id: 'device',
    title: '设备与展示',
    desc: '维护显示、打印和门店设备设置。',
    items: [
      { id: 'display-config', title: '后厨协同设置', desc: '统一维护打印分发与自动接单配置', path: '/pages/merchant/settings/display-config/index' },
      { id: 'printers', title: '乐客来福打印机', desc: '输入 SN 和绑定码绑定打印机，维护云打印设备和测试状态', path: '/pages/merchant/printers/index' }
    ]
  },
  {
    id: 'collaboration',
    title: '人员与协作',
    desc: '将门店成员协作与品牌 / 集团合作拆开处理，避免和店铺配置或资金设置混在一起。',
    groups: [
      {
        id: 'staff-collaboration',
        title: '门店成员协作',
        desc: '邀请店员加入门店、查看成员名单，并持续维护岗位分配。',
        items: [
          { id: 'staff', title: '员工管理', desc: '查看门店成员、展示扫码邀请并分配角色', path: '/pages/merchant/staff/index' }
        ]
      },
      {
        id: 'group-collaboration',
        title: '品牌 / 集团合作',
        desc: '处理集团主体入驻，以及门店加入品牌或集团后的协作归属。',
        items: [
          { id: 'group-application', title: '集团入驻', desc: '提交品牌或集团主体入驻申请，跟进入驻审核进度', path: '/pages/merchant/group/application/index' },
          { id: 'group-join', title: '申请加入集团', desc: '搜索集团或品牌并发起合作申请，处理协作归属', path: '/pages/merchant/group/join/index' }
        ]
      }
    ]
  }
]

const DEVICE_MANAGE_ITEM_IDS = new Set(['display-config', 'printers'])

function filterConfigSections(
  sections: ConfigSection[],
  canManageDeviceSettings: boolean,
  canManageApplyment: boolean
) {
  return sections
    .map((section) => ({
      ...section,
      items: section.items?.filter((item) => {
        const passesDeviceGate = canManageDeviceSettings || !DEVICE_MANAGE_ITEM_IDS.has(item.id)
        const passesApplymentGate = canManageApplyment || item.id !== 'baofu-settlement-account'
        return passesDeviceGate && passesApplymentGate
      }),
      groups: section.groups
        ?.map((group) => ({
          ...group,
          items: group.items.filter((item) => {
            const passesDeviceGate = canManageDeviceSettings || !DEVICE_MANAGE_ITEM_IDS.has(item.id)
            const passesApplymentGate = canManageApplyment || item.id !== 'baofu-settlement-account'
            return passesDeviceGate && passesApplymentGate
          })
        }))
        .filter((group) => group.items.length > 0)
    }))
    .filter((section) => (section.items && section.items.length > 0) || (section.groups && section.groups.length > 0))
}

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    configSections: CONFIG_SECTIONS
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })

    const accessResult = await ensureMerchantConsoleAccess()
    let canManageDeviceSettings = false
    let canManageApplyment = false
    if (accessResult.status === 'granted') {
      canManageApplyment = canManageMerchantApplyment(accessResult.user?.roles || [])
      try {
        const deviceAccess = await getRecentMerchantDeviceAccess()
        canManageDeviceSettings = canUseMerchantDeviceManagementFallback(accessResult.user?.roles || [], deviceAccess)
      } catch (err) {
        logger.warn('Load merchant device access for config page failed', err)
        canManageDeviceSettings = canUseMerchantDeviceManagementFallback(accessResult.user?.roles || [])
      }
    }

    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : '',
      configSections: accessResult.status === 'granted'
        ? filterConfigSections(CONFIG_SECTIONS, canManageDeviceSettings, canManageApplyment)
        : []
    })
  },

  onRetryAccess() {
    this.setData({ accessReady: false, accessDenied: false, accessErrorMessage: '' })
    this.onLoad()
  },

  onTapItem(e: WechatMiniprogram.TouchEvent) {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return

    const { path } = e.currentTarget.dataset as { path?: string }
    if (!path) return

    wx.navigateTo({ url: path })
  }
})
