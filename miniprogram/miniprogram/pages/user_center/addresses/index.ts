import AddressService, { Address } from '../../../api/address'
import { logger } from '../../../utils/logger'
import { ErrorHandler } from '../../../utils/error-handler'

Page({
  data: {
    addresses: [] as Address[],
    navBarHeight: 88,
    loading: false,
    isSelectMode: false
  },

  onLoad(options: any) {
    if (options.select === 'true') {
      this.setData({ isSelectMode: true })
    }
  },

  onShow() {
    this.loadAddresses()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadAddresses() {
    this.setData({ loading: true })

    try {
      const addresses = await AddressService.getAddresses()
      this.setData({
        addresses,
        loading: false
      })
    } catch (error) {
      ErrorHandler.handle(error, 'Addresses.loadAddresses')
      this.setData({ loading: false })
    }
  },

  onAddAddress() {
    wx.navigateTo({
      url: '/pages/user_center/addresses/edit/index'
    })
  },

  /**
   * 从微信导入地址
   */
  onImportWechatAddress() {
    wx.chooseAddress({
      success: (res) => {
        // 跳转到编辑页，预填微信地址数据
        const params = encodeURIComponent(JSON.stringify({
          contact_name: res.userName,
          contact_phone: res.telNumber,
          detail_address: `${res.provinceName}${res.cityName}${res.countyName}${res.detailInfo}`
        }))
        wx.navigateTo({
          url: `/pages/user_center/addresses/edit/index?wechat_data=${params}`
        })
      },
      fail: (err) => {
        if (err.errMsg.includes('cancel')) return
        logger.error('Choose address failed:', err, 'Addresses')
        wx.showToast({ title: '获取微信地址失败', icon: 'none' })
      }
    })
  },

  onEditAddress(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.navigateTo({
      url: `/pages/user_center/addresses/edit/index?id=${id}`
    })
  },

  onDeleteAddress(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.showModal({
      title: '删除地址',
      content: '确认删除此地址?',
      success: async (res) => {
        if (res.confirm) {
          try {
            await AddressService.deleteAddress(id)
            wx.showToast({ title: '已删除', icon: 'success' })
            this.loadAddresses()
          } catch (error) {
            ErrorHandler.handle(error, 'Addresses.deleteAddress')
          }
        }
      }
    })
  },

  onSelectAddress(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    if (this.data.isSelectMode) {
      const pages = getCurrentPages()
      const prevPage = pages[pages.length - 2]
      if (prevPage) {
        (prevPage as any).setData({ selectedAddressId: id })
      }
      wx.navigateBack()
    }
  },

  async onSetDefault(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset

    try {
      await AddressService.setDefaultAddress(id)
      wx.showToast({ title: '已设为默认', icon: 'success' })
      this.loadAddresses()
    } catch (error) {
      ErrorHandler.handle(error, 'Addresses.setDefault')
    }
  }
})
