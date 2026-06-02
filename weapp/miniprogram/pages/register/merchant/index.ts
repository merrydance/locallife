import { checkRegionAvailability } from '../../../api/location'
import { globalStore } from '../../../utils/global-store'

type MerchantRegisterIndexData = {
  navBarHeight: number
  localOperatorPhone: string
}

type RegionSubscription = (() => void) | undefined

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

  onSelectGroup() {
    wx.navigateTo({ url: './group/index' })
  },

  onJoinGroup() {
    wx.navigateTo({ url: './join-group/index' })
  },

  async loadLocalOperatorContact(regionIdParam?: number) {
    const app = getApp<IAppOption>()
    const regionId = Number(regionIdParam || app.globalData.currentRegion?.id || 0)
    if (!regionId) return
    this.requestedOperatorRegionId = regionId
    if (regionId !== this.loadedOperatorRegionId) {
      this.setData({ localOperatorPhone: '' })
    }

    try {
      const result = await checkRegionAvailability(regionId)
      if (regionId !== this.requestedOperatorRegionId) return

      const phone = (result.operator_contact_phone || '').trim()
      this.setData({ localOperatorPhone: phone })
      this.loadedOperatorRegionId = regionId
    } catch (_error) {
      if (regionId !== this.requestedOperatorRegionId) return

      this.setData({ localOperatorPhone: '' })
    }
  },

  onCallOperator() {
    const phoneNumber = this.data.localOperatorPhone.replace(/\s+/g, '')
    if (!phoneNumber) {
      wx.showToast({ title: '暂无运营商电话', icon: 'none' })
      return
    }

    wx.makePhoneCall({ phoneNumber })
  }
})
