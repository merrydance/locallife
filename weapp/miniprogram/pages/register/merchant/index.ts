import { globalStore } from '../../../utils/global-store'
import { logger } from '../../../utils/logger'
import { getCurrentRegionId, getLocalOperatorContactPhone, normalizeOperatorPhoneNumber } from '../../../utils/operator-contact'
import {
  getMerchantWorkbenchFromProfile,
  hasMerchantConsoleAccess,
  isMerchantWorkbenchGranted
} from '../../../utils/console-access'
import { fetchUserProfile } from '../../../services/user-profile'

type MerchantRegisterIndexData = {
  navBarHeight: number
  localOperatorPhone: string
}

type RegionSubscription = (() => void) | undefined
const app = getApp<IAppOption>()

Page({
  data: {
    navBarHeight: 88,
    localOperatorPhone: ''
  } as MerchantRegisterIndexData,

  unsubscribeRegion: undefined as RegionSubscription,
  loadedOperatorRegionId: 0,
  requestedOperatorRegionId: 0,

  onLoad() {
    this.loadLocalOperatorContact()
    void this.redirectExistingMerchantWorkbench()
    this.unsubscribeRegion = globalStore.subscribe('currentRegion', (region) => {
      const regionId = Number(region?.id || 0)
      if (regionId && regionId !== this.loadedOperatorRegionId) {
        this.loadLocalOperatorContact(regionId)
      }
    })
  },

  onUnload() {
    if (this.unsubscribeRegion) {
      this.unsubscribeRegion()
      this.unsubscribeRegion = undefined
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onSelectStore() {
    wx.navigateTo({ url: './store/index' })
  },

  tryRedirectExistingMerchantWorkbench(workbenches = app.globalData.userWorkbenches, roles = app.globalData.userRoles || [], allowRoleFallback = true) {
    const merchantWorkbench = getMerchantWorkbenchFromProfile(workbenches)
    const normalizedRoles = roles.map((role) => String(role || '').trim().toLowerCase()).filter(Boolean)
    if (!isMerchantWorkbenchGranted(merchantWorkbench) && (merchantWorkbench || !allowRoleFallback || !hasMerchantConsoleAccess(normalizedRoles))) {
      return false
    }

    wx.redirectTo({ url: '/pages/merchant/dashboard/index' })
    return true
  },

  async redirectExistingMerchantWorkbench() {
    const cachedRoles = app.globalData.userRoles?.length
      ? app.globalData.userRoles
      : [app.globalData.userRole].filter(Boolean)
    if (this.tryRedirectExistingMerchantWorkbench(app.globalData.userWorkbenches, cachedRoles, false)) {
      return
    }

    try {
      const user = await fetchUserProfile()
      const roles = (user.roles || []).map((role) => String(role || '').trim().toLowerCase()).filter(Boolean)
      app.globalData.userRoles = roles
      app.globalData.userWorkbenches = user.workbenches || []
      this.tryRedirectExistingMerchantWorkbench(user.workbenches || [], roles)
    } catch (error) {
      logger.warn('商户入驻入口身份刷新失败，继续展示入驻页', error, 'MerchantRegisterIndex.redirectExistingMerchantWorkbench')
    }
  },

  onSelectGroup() {
    wx.navigateTo({ url: './group/index' })
  },

  onJoinGroup() {
    wx.navigateTo({ url: '/pages/merchant/group/join/index' })
  },

  async loadLocalOperatorContact(regionIdParam?: number) {
    const regionId = Number(regionIdParam || getCurrentRegionId())
    if (!regionId) return
    this.requestedOperatorRegionId = regionId
    if (regionId !== this.loadedOperatorRegionId) {
      this.setData({ localOperatorPhone: '' })
    }

    try {
      const phone = await getLocalOperatorContactPhone(regionId)
      if (regionId !== this.requestedOperatorRegionId) return

      this.setData({ localOperatorPhone: phone })
      this.loadedOperatorRegionId = regionId
    } catch (_error) {
      if (regionId !== this.requestedOperatorRegionId) return

      this.setData({ localOperatorPhone: '' })
    }
  },

  onCallOperator() {
    const phoneNumber = normalizeOperatorPhoneNumber(this.data.localOperatorPhone)
    if (!phoneNumber) {
      wx.showToast({ title: '暂无运营商电话', icon: 'none' })
      return
    }

    wx.makePhoneCall({ phoneNumber })
  }
})
