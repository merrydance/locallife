import { getAddressList, deleteAddress, updateAddress, AddressDTO } from '../../../api/address'
import { logger } from '../../../utils/logger'
import { ErrorHandler } from '../../../utils/error-handler'

Page({
  data: {
    addresses: [] as AddressDTO[],
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
      const addresses = await getAddressList()
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
    // Ensure edit page exists or create one. For now just a toast if not exist
    wx.navigateTo({
      url: '/pages/user_center/addresses/edit/index', fail: () => {
        wx.showToast({ title: '编辑页开发中', icon: 'none' })
      }
    })
  },

  onEditAddress(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.navigateTo({
      url: `/pages/user_center/addresses/edit/index?id=${id}`,
      fail: () => {
        wx.showToast({ title: '编辑页开发中', icon: 'none' })
      }
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
            await deleteAddress(id)
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
        prevPage.setData({ selectedAddressId: id })
      }
      wx.navigateBack()
    }
  },

  async onSetDefault(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    const address = this.data.addresses.find((a) => a.id === id)
    if (!address) return

    try {
      // Convert AddressDTO back to CreateAddressRequest-like object for update
      // Note: In a real app, maybe a specific set_default endpoint exists or we just update is_default
      await updateAddress(id, {
        ...address,
        is_default: true
      })

      wx.showToast({ title: '已设为默认', icon: 'success' })
      this.loadAddresses()
    } catch (error) {
      ErrorHandler.handle(error, 'Addresses.setDefault')
    }
  }
})
