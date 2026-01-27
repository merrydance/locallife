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
    initialLoading: false,
    error: null as string | null,
    navBarHeight: 88,
    navTitle: '编辑地址'
  },

  onLoad(options: { id?: string; wechat_data?: string }) {
    if (options.id) {
      this.setData({ 
        addressId: Number(options.id),
        initialLoading: true,
        navTitle: '编辑地址'
      })
      this.loadAddress(Number(options.id))
    } else if (options.wechat_data) {
      // 从微信导入的数据
      try {
        const data: WechatAddressData = JSON.parse(decodeURIComponent(options.wechat_data))
        this.setData({
          contactName: data.contact_name,
          contactPhone: data.contact_phone,
          detailAddress: data.detail_address,
          navTitle: '完善地址'
        })
      } catch (e) {
        logger.error('Parse wechat data failed', e, 'AddressEdit')
      }
    } else {
      this.setData({ navTitle: '新增地址' })
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadAddress(id: number) {
    this.setData({ initialLoading: true, error: null })
    try {
      const detail = await AddressService.getAddressDetail(id)
      this.setData({
        contactName: detail.contact_name,
        contactPhone: detail.contact_phone,
        detailAddress: detail.detail_address,
        latitude: detail.latitude,
        longitude: detail.longitude,
        isDefault: detail.is_default,
        initialLoading: false
      })
    } catch (error) {
      logger.error('Load address failed:', error, 'AddressEdit')
      this.setData({ 
        initialLoading: false,
        error: '加载地址详情失败'
      })
    }
  },

  onRetry() {
    if (this.data.addressId) {
      this.loadAddress(this.data.addressId)
    }
  },

  onNameChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ contactName: e.detail.value })
  },

  onPhoneChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ contactPhone: e.detail.value })
  },

  onDetailChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ detailAddress: e.detail.value })
  },

  onDefaultChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ isDefault: e.detail.value })
  },

  onChooseLocation() {
    wx.chooseLocation({
      success: (res) => {
        // 使用选择的位置更新地址和经纬度
        const newAddress = res.name || res.address || ''

        // 直接用地图选择的地址作为基础，用户可在此基础上修改门牌号
        this.setData({
          detailAddress: newAddress ? `${newAddress} ` : this.data.detailAddress,
          latitude: String(res.latitude),
          longitude: String(res.longitude)
        })
      },
      fail: (err) => {
        if (err.errMsg.includes('cancel')) return
        logger.error('Choose location failed:', err, 'AddressEdit')
        
        if (err.errMsg.includes('auth') || err.errMsg.includes('authorize')) {
            wx.showModal({
                title: '需要权限',
                content: '请在设置中开启位置权限以选择地址',
                confirmText: '去设置',
                success: (m) => {
                    if (m.confirm) wx.openSetting()
                }
            })
        } else {
            wx.showToast({ title: '无法打开地图', icon: 'none' })
        }
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
          detail_address: this.data.detailAddress
        }
        if (this.data.latitude && this.data.longitude) {
          updateData.latitude = this.data.latitude
          updateData.longitude = this.data.longitude
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
          is_default: this.data.isDefault
        }
        if (this.data.latitude && this.data.longitude) {
          createData.latitude = this.data.latitude
          createData.longitude = this.data.longitude
        }
        await AddressService.createAddress(createData)
      }

      wx.showToast({ title: '保存成功', icon: 'success' })
      setTimeout(() => wx.navigateBack(), 1000)
    } catch (error) {
      logger.error('Save failed', error)
      const err = error as any
      const message = err?.message || err?.response?.data?.error || err?.data?.error
       if (message && (
        message.includes('未能定位') ||
        message.includes('geocode')
      )) {
        wx.showModal({
          title: '区域识别失败',
          content: '请尝试在地图上重新选点，或者仅修改门牌号',
          showCancel: false
        })
        return
      }
      
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
      confirmColor: '#E34D59',
      success: async (res) => {
        if (res.confirm) {
            wx.showLoading({ title: '删除中' })
          try {
            await AddressService.deleteAddress(this.data.addressId)
            wx.showToast({ title: '已删除', icon: 'success' })
            setTimeout(() => wx.navigateBack(), 1000)
          } catch (error) {
            ErrorHandler.handle(error, 'AddressEdit.delete')
          } finally {
              wx.hideLoading()
          }
        }
      }
    })
  },

  validate(): boolean {
    const { contactName, contactPhone, detailAddress } = this.data

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
    return true
  }
})
