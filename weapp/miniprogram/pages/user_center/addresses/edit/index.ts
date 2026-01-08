import AddressService, { Address, CreateAddressRequest, UpdateAddressRequest } from '../../../../api/address'
import { logger } from '../../../../utils/logger'
import { ErrorHandler } from '../../../../utils/error-handler'

interface WechatAddressData {
  contact_name: string
  contact_phone: string
  detail_address: string
}

Page({
  data: {
    addressId: 0,
    contactName: '',
    contactPhone: '',
    detailAddress: '',
    latitude: '',
    longitude: '',
    isDefault: false,
    saving: false,
    navBarHeight: 88
  },

  onLoad(options: any) {
    if (options.id) {
      this.setData({ addressId: Number(options.id) })
      this.loadAddress(Number(options.id))
      wx.setNavigationBarTitle({ title: '编辑地址' })
    } else if (options.wechat_data) {
      // 从微信导入的数据
      try {
        const data: WechatAddressData = JSON.parse(decodeURIComponent(options.wechat_data))
        this.setData({
          contactName: data.contact_name,
          contactPhone: data.contact_phone,
          detailAddress: data.detail_address
        })
        wx.setNavigationBarTitle({ title: '完善地址' })
        // 微信地址没有经纬度，需要用户选择位置
        wx.showToast({ title: '请选择地图位置', icon: 'none', duration: 2000 })
      } catch (e) {
        logger.error('Parse wechat data failed', e, 'AddressEdit')
      }
    } else {
      wx.setNavigationBarTitle({ title: '新增地址' })
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadAddress(id: number) {
    try {
      const detail = await AddressService.getAddressDetail(id)
      this.setData({
        contactName: detail.contact_name,
        contactPhone: detail.contact_phone,
        detailAddress: detail.detail_address,
        latitude: detail.latitude,
        longitude: detail.longitude,
        isDefault: detail.is_default
      })
    } catch (error) {
      logger.error('Load address failed:', error, 'AddressEdit')
      wx.showToast({ title: '加载失败', icon: 'error' })
    }
  },

  onNameChange(e: any) {
    this.setData({ contactName: e.detail.value })
  },

  onPhoneChange(e: any) {
    this.setData({ contactPhone: e.detail.value })
  },

  onDetailChange(e: any) {
    this.setData({ detailAddress: e.detail.value })
  },

  onDefaultChange(e: any) {
    this.setData({ isDefault: e.detail.value })
  },

  onChooseLocation() {
    wx.chooseLocation({
      success: (res) => {
        // 使用选择的位置更新地址和经纬度
        const newAddress = res.name || res.address
        const currentDetail = this.data.detailAddress

        // 如果当前地址为空或是微信导入的，使用新地址
        // 如果用户已有详细地址，保留
        this.setData({
          detailAddress: currentDetail || newAddress,
          latitude: String(res.latitude),
          longitude: String(res.longitude)
        })
      },
      fail: (err) => {
        if (err.errMsg.includes('cancel')) return
        logger.error('Choose location failed:', err, 'AddressEdit')
        wx.showToast({ title: '请在设置中开启位置权限', icon: 'none' })
      }
    })
  },

  async onSave() {
    if (!this.validate()) return

    this.setData({ saving: true })

    try {
      if (this.data.addressId) {
        // 更新地址
        const updateData: UpdateAddressRequest = {
          contact_name: this.data.contactName,
          contact_phone: this.data.contactPhone,
          detail_address: this.data.detailAddress,
          latitude: this.data.latitude,
          longitude: this.data.longitude
        }
        await AddressService.updateAddress(this.data.addressId, updateData)

        // 如果需要设为默认
        if (this.data.isDefault) {
          await AddressService.setDefaultAddress(this.data.addressId)
        }
      } else {
        // 创建地址
        const createData: CreateAddressRequest = {
          contact_name: this.data.contactName,
          contact_phone: this.data.contactPhone,
          detail_address: this.data.detailAddress,
          latitude: this.data.latitude,
          longitude: this.data.longitude,
          is_default: this.data.isDefault
        }
        await AddressService.createAddress(createData)
      }

      wx.showToast({ title: '保存成功', icon: 'success' })
      setTimeout(() => wx.navigateBack(), 1500)
    } catch (error) {
      logger.error('Save address failed:', error, 'AddressEdit')
      ErrorHandler.handle(error, 'AddressEdit.save')
    } finally {
      this.setData({ saving: false })
    }
  },

  async onDelete() {
    if (!this.data.addressId) return

    wx.showModal({
      title: '删除地址',
      content: '确认删除此地址?',
      success: async (res) => {
        if (res.confirm) {
          try {
            await AddressService.deleteAddress(this.data.addressId)
            wx.showToast({ title: '已删除', icon: 'success' })
            setTimeout(() => wx.navigateBack(), 1500)
          } catch (error) {
            logger.error('Delete address failed:', error, 'AddressEdit')
            ErrorHandler.handle(error, 'AddressEdit.delete')
          }
        }
      }
    })
  },

  validate(): boolean {
    const { contactName, contactPhone, detailAddress, latitude, longitude } = this.data

    if (!contactName.trim()) {
      wx.showToast({ title: '请填写联系人', icon: 'none' })
      return false
    }
    if (!contactPhone.trim() || contactPhone.length !== 11) {
      wx.showToast({ title: '请填写正确手机号', icon: 'none' })
      return false
    }
    if (!detailAddress.trim()) {
      wx.showToast({ title: '请填写详细地址', icon: 'none' })
      return false
    }
    if (!latitude || !longitude) {
      wx.showToast({ title: '请选择地图位置', icon: 'none' })
      return false
    }
    return true
  }
})
