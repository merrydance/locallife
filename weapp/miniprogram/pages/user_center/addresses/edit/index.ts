import AddressService, { CreateAddressRequest, UpdateAddressRequest } from '../_main_shared/api/address'
import { getCurrentRegion } from '../../../../api/location'
import { listRegions, RegionItem } from '../_main_shared/api/operator-application'
import { logger } from '../../../../utils/logger'
import { ErrorHandler } from '../../../../utils/error-handler'
import { getErrorDebugMessage, getErrorUserMessage } from '../../../../utils/user-facing'

interface WechatAddressData {
  contact_name: string
  contact_phone: string
  detail_address: string
  region_address?: string
}

const normalizeRegionName = (name: string): string =>
  name.replace(/(省|市|区|县|旗|盟|州|地区|特别行政区|自治区)$/g, '').trim()

const sameRegionName = (left: string, right: string): boolean => {
  const a = normalizeRegionName(left)
  const b = normalizeRegionName(right)
  return !!a && !!b && (a === b || left.includes(right) || right.includes(left))
}

const extractRegionTokens = (regionAddress: string): { cityName: string, districtName: string } => {
  const matched = regionAddress.match(/[^省市区县旗盟州]+(?:省|市|区|县|旗|盟|州|地区|自治区|特别行政区)/g) || []
  const cityName = matched.find((item) => item.endsWith('市') || item.endsWith('地区') || item.endsWith('盟') || item.endsWith('州')) || ''
  const districtName = [...matched].reverse().find((item) => item.endsWith('区') || item.endsWith('县') || item.endsWith('旗') || item.endsWith('市')) || ''
  return { cityName, districtName }
}

Page({
  data: {
    addressId: 0,
    fromSelectMode: false,
    fromWechatImport: false,
    contactName: '',
    contactPhone: '',
    regionAddress: '',
    regionId: 0,
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

  onLoad(options: { id?: string, wechat_data?: string, from_select?: string }) {
    const fromSelectMode = options.from_select === 'true'
    this.setData({
      fromSelectMode,
      fromWechatImport: !!options.wechat_data
    })

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
          regionAddress: data.region_address || '',
          detailAddress: data.detail_address,
          navTitle: fromSelectMode ? '导入并完善地址' : '完善地址'
        })
      } catch (e) {
        logger.error('Parse wechat data failed', e, 'AddressEdit')
      }
    } else {
      this.setData({ navTitle: fromSelectMode ? '新增并使用地址' : '新增地址' })
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
        regionAddress: detail.region_name || '',
        regionId: detail.region_id || 0,
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
        error: getErrorUserMessage(error, '加载地址详情失败，请稍后重试')
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
      success: async (res) => {
        const regionAddress = res.address || ''
        const detailLabel = res.name || ''
        const currentDetail = (this.data.detailAddress || '').trim()
        const newDetail = currentDetail || (detailLabel ? `${detailLabel} ` : this.data.detailAddress)

        let regionId = this.data.regionId
        try {
          const region = await getCurrentRegion({ latitude: res.latitude, longitude: res.longitude })
          if (region?.region_id) {
            regionId = region.region_id
          }
        } catch (error) {
          logger.warn('Resolve region by location failed', error, 'AddressEdit.onChooseLocation')
        }

        this.setData({
          regionAddress,
          regionId,
          detailAddress: newDetail,
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
      let resolvedRegionId = this.data.regionId
      let resolvedLatitude = (this.data.latitude || '').trim()
      let resolvedLongitude = (this.data.longitude || '').trim()
      const latitudeNum = Number(this.data.latitude)
      const longitudeNum = Number(this.data.longitude)

      if (!resolvedRegionId && Number.isFinite(latitudeNum) && Number.isFinite(longitudeNum) && latitudeNum && longitudeNum) {
        try {
          const region = await getCurrentRegion({ latitude: latitudeNum, longitude: longitudeNum })
          resolvedRegionId = region?.region_id || 0
        } catch (error) {
          logger.warn('Resolve region before save by location failed', error, 'AddressEdit.onSave')
        }
      }

      if (!resolvedRegionId && this.data.regionAddress.trim()) {
        resolvedRegionId = await this.resolveRegionIdByAddressText(this.data.regionAddress)
      }

      if (!resolvedRegionId) {
        const app = getApp<IAppOption>()
        const fallbackLat = Number(app.globalData.latitude)
        const fallbackLng = Number(app.globalData.longitude)
        if (Number.isFinite(fallbackLat) && Number.isFinite(fallbackLng) && fallbackLat && fallbackLng) {
          try {
            const region = await getCurrentRegion({ latitude: fallbackLat, longitude: fallbackLng })
            resolvedRegionId = region?.region_id || 0
            if (!resolvedLatitude || !resolvedLongitude) {
              resolvedLatitude = String(fallbackLat)
              resolvedLongitude = String(fallbackLng)
            }
          } catch (error) {
            logger.warn('Resolve region before save by app location failed', error, 'AddressEdit.onSave')
          }
        }
      }

      if (!resolvedLatitude || !resolvedLongitude) {
        const app = getApp<IAppOption>()
        const fallbackLat = Number(app.globalData.latitude)
        const fallbackLng = Number(app.globalData.longitude)
        if (Number.isFinite(fallbackLat) && Number.isFinite(fallbackLng) && fallbackLat && fallbackLng) {
          resolvedLatitude = String(fallbackLat)
          resolvedLongitude = String(fallbackLng)
        }
      }

      if (!resolvedRegionId && (!resolvedLatitude || !resolvedLongitude)) {
        wx.showModal({
          title: '无法保存地址',
          content: '请先在地图上选点后再保存',
          showCancel: false
        })
        return
      }

      logger.info('Address save resolved params', {
        regionId: resolvedRegionId,
        hasLatitude: !!resolvedLatitude,
        hasLongitude: !!resolvedLongitude
      }, 'AddressEdit.onSave')

      if (this.data.addressId) {
        // 更新地址
        const updateData: UpdateAddressRequest = {
          contact_name: this.data.contactName,
          contact_phone: this.data.contactPhone,
          detail_address: this.data.detailAddress
        }
        if (resolvedRegionId > 0) {
          updateData.region_id = resolvedRegionId
        }
        if (resolvedLatitude && resolvedLongitude) {
          updateData.latitude = resolvedLatitude
          updateData.longitude = resolvedLongitude
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
        if (resolvedRegionId > 0) {
          createData.region_id = resolvedRegionId
        }
        if (resolvedLatitude && resolvedLongitude) {
          createData.latitude = resolvedLatitude
          createData.longitude = resolvedLongitude
        }
        await AddressService.createAddress(createData)
      }

      wx.showToast({
        title: this.data.fromSelectMode ? '已保存，可用于本次下单' : '保存成功',
        icon: 'success'
      })
      setTimeout(() => wx.navigateBack(), 1000)
    } catch (error) {
      logger.error('Save failed', error)
      const message = getErrorDebugMessage(error)
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

  async resolveRegionIdByAddressText(regionAddress: string): Promise<number> {
    try {
      const { cityName, districtName } = extractRegionTokens(regionAddress)
      if (!districtName) return 0

      let cityId = 0
      if (cityName) {
        for (let page = 1; page <= 8; page++) {
          const cities = await listRegions({ page_id: page, page_size: 100, level: 2 })
          const targetCity = (cities || []).find((city: RegionItem) => sameRegionName(city.name, cityName))
          if (targetCity) {
            cityId = targetCity.id
            break
          }
          if (!cities || cities.length < 100) break
        }
      }

      if (cityId > 0) {
        for (let page = 1; page <= 8; page++) {
          const districts = await listRegions({ page_id: page, page_size: 100, level: 3, parent_id: cityId })
          const targetDistrict = (districts || []).find((district: RegionItem) => sameRegionName(district.name, districtName))
          if (targetDistrict) {
            return targetDistrict.id
          }
          if (!districts || districts.length < 100) break
        }
      }

      for (let page = 1; page <= 20; page++) {
        const districts = await listRegions({ page_id: page, page_size: 100, level: 3 })
        const targetDistrict = (districts || []).find((district: RegionItem) => sameRegionName(district.name, districtName))
        if (targetDistrict) {
          return targetDistrict.id
        }
        if (!districts || districts.length < 100) break
      }
    } catch (error) {
      logger.warn('Resolve region by address text failed', error, 'AddressEdit.resolveRegionIdByAddressText')
    }
    return 0
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
            wx.navigateBack()
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
