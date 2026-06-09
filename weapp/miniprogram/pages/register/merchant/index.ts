import { globalStore } from '../../../utils/global-store'
import { getCurrentRegionId, getLocalOperatorContactPhone, normalizeOperatorPhoneNumber } from '../../../utils/operator-contact'

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
