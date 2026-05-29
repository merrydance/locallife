import AddressService, { Address } from './_main_shared/api/address'
import { logger } from '../../../utils/logger'
import { ErrorHandler } from '../../../utils/error-handler'

type PreviousAddressPage = {
  setData?: (data: Record<string, unknown>) => void
  onAddressSelected?: (address?: Address) => void
}

Page({
  _skipNextOnShowLoad: false,

  data: {
    addresses: [] as Address[],
    navBarHeight: 88,
    loading: false,
    initialLoading: true,
    error: null as string | null,
    isSelectMode: false
  },

  onLoad(options: { select?: string }) {
    if (options.select === 'true') {
      this.setData({ isSelectMode: true })
    }
  },

  onShow() {
    if (this._skipNextOnShowLoad) {
      this._skipNextOnShowLoad = false
      return
    }
    this.loadAddresses()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  preventBubble() {},

  async loadAddresses() {
    if (this.data.loading && !this.data.initialLoading) return
    this.setData({ loading: true, error: null })

    try {
      const addresses = await AddressService.getAddresses()
      // Sort: Default first
      addresses.sort((a, b) => (b.is_default ? 1 : 0) - (a.is_default ? 1 : 0))
      
      this.setData({
        addresses,
        loading: false,
        initialLoading: false
      })
    } catch (error) {
      ErrorHandler.handle(error, 'Addresses.loadAddresses')
      this.setData({ 
        loading: false,
        initialLoading: false,
        error: '加载收货地址失败'
      })
    }
  },

  onRetry() {
    this.loadAddresses()
  },

  onAddAddress() {
    const fromSelect = this.data.isSelectMode ? '?from_select=true' : ''
    wx.navigateTo({
      url: `/pages/user_center/addresses/edit/index${fromSelect}`
    })
  },

  onImportWechatAddress() {
    this._skipNextOnShowLoad = true
    wx.chooseAddress({
      success: (res) => {
        const regionAddress = `${res.provinceName}${res.cityName}${res.countyName}`
        const params = encodeURIComponent(JSON.stringify({
          contact_name: res.userName,
          contact_phone: res.telNumber,
          region_address: regionAddress,
          detail_address: res.detailInfo || ''
        }))
        const targetUrl = `/pages/user_center/addresses/edit/index?wechat_data=${params}${this.data.isSelectMode ? '&from_select=true' : ''}`
        if (this.data.isSelectMode) {
          wx.redirectTo({ url: targetUrl })
        } else {
          wx.navigateTo({ url: targetUrl })
        }
      },
      fail: (err) => {
        this._skipNextOnShowLoad = false
        if (err.errMsg.includes('cancel')) return
        logger.error('Choose address failed:', err, 'Addresses')
        if (err.errMsg.includes('auth')) {
          wx.showModal({
            title: '需要授权',
            content: '请在设置中打开通讯录权限以导入地址',
            confirmText: '去设置',
            success: (modalRes) => {
              if (modalRes.confirm) wx.openSetting()
            }
          })
        }
      }
    })
  },

  onEditAddress(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    if (!id) return
    wx.navigateTo({
      url: `/pages/user_center/addresses/edit/index?id=${id}`
    })
  },

  onDeleteAddress(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    if (!id) return

    wx.showModal({
      title: '删除地址',
      content: '确认删除此地址?',
      confirmColor: '#E34D59',
      success: async (res) => {
        if (res.confirm) {
          try {
            await AddressService.deleteAddress(id)
            await this.loadAddresses()
          } catch (error) {
            ErrorHandler.handle(error, 'Addresses.deleteAddress')
          }
        }
      }
    })
  },

  onSelectAddress(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    if (!id) return

    if (this.data.isSelectMode) {
      const pages = getCurrentPages()
      const prevPage = pages[pages.length - 2] as PreviousAddressPage | undefined
      if (prevPage && prevPage.setData) {
        // 尝试设置上一页的数据，或者是调用一个回调如果页面支持
        // 假设上一页监听 selectedAddressId
        prevPage.setData({ selectedAddressId: id })
        // 如果有 onAddressSelected 方法也可以调用
        if (typeof prevPage.onAddressSelected === 'function') {
            prevPage.onAddressSelected(this.data.addresses.find((a) => a.id === id))
        }
      }
      wx.navigateBack()
    }
  },

  async onSetDefault(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    // 如果已经是默认，就不做操作
    const current = this.data.addresses.find((a) => a.id === id)
    if (current?.is_default) return

    try {
      wx.showLoading({ title: '设置中' })
      await AddressService.setDefaultAddress(id)
      wx.hideLoading()
      await this.loadAddresses()
    } catch (error) {
      wx.hideLoading()
      ErrorHandler.handle(error, 'Addresses.setDefault')
    }
  }
})
