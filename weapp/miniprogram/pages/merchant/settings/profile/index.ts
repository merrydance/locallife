import { getMyMerchantOpenStatus, getMyMerchantProfile, MerchantOperatorResponse, updateMyMerchantProfile } from '../../../../api/merchant'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'

interface MerchantProfileForm {
  name: string
  phone: string
  address: string
  description: string
  latitude: string
  longitude: string
}

const EMPTY_FORM: MerchantProfileForm = {
  name: '',
  phone: '',
  address: '',
  description: '',
  latitude: '',
  longitude: ''
}

const PROFILE_AUTO_REFRESH_WINDOW_MS = 60 * 1000

function buildLocationLabel(address: string, latitude?: string | null, longitude?: string | null) {
  if (address.trim()) return address.trim()
  if (latitude && longitude) return `坐标 ${latitude}, ${longitude}`
  return '未选择经营位置'
}

function buildLocationHint(address: string, latitude?: string | null, longitude?: string | null) {
  const hasAddress = address.trim().length > 0
  const hasCoordinates = !!latitude && !!longitude

  if (hasAddress && hasCoordinates) {
    return '地址与坐标会在保存时一并提交。'
  }

  if (hasAddress || hasCoordinates) {
    return '当前位置信息不完整，请重新选择经营位置。'
  }

  return '请选择经营位置，系统会同步更新地址与坐标。'
}

function buildChosenLocationAddress(result: WechatMiniprogram.ChooseLocationSuccessCallbackResult) {
  const address = result.address || ''
  const name = result.name || ''
  if (address && name) {
    return address.includes(name) ? address : `${address} ${name}`
  }
  return address || name || ''
}

function buildForm(profile: MerchantOperatorResponse): MerchantProfileForm {
  return {
    name: profile.name || '',
    phone: profile.phone || '',
    address: profile.address || '',
    description: profile.description || '',
    latitude: profile.latitude || '',
    longitude: profile.longitude || ''
  }
}

function hasFormChanged(current: MerchantProfileForm, initial: MerchantProfileForm) {
  return current.name !== initial.name
    || current.phone !== initial.phone
    || current.address !== initial.address
    || current.description !== initial.description
    || current.latitude !== initial.latitude
    || current.longitude !== initial.longitude
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

function isCoordinateInRange(value: string, min: number, max: number): boolean {
  const parsed = Number.parseFloat(value)
  return Number.isFinite(parsed) && parsed >= min && parsed <= max
}

function hasCompleteLocation(form: MerchantProfileForm) {
  return !!form.address.trim() && !!form.latitude.trim() && !!form.longitude.trim()
}

function hasPartialLocation(form: MerchantProfileForm) {
  return !!form.address.trim() || !!form.latitude.trim() || !!form.longitude.trim()
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    actionNoticeMessage: '',
    refreshErrorMessage: '',
    loading: false,
    saving: false,
    lastLoadedAt: 0,
    isOpen: false,
    merchantId: 0,
    version: 0,
    updatedAtLabel: '--',
    locationLabel: '未选择经营位置',
    locationHint: '请选择经营位置，系统会同步更新地址与坐标。',
    form: { ...EMPTY_FORM } as MerchantProfileForm,
    initialForm: { ...EMPTY_FORM } as MerchantProfileForm,
    hasChanges: false
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })

    const accessResult = await ensureMerchantConsoleAccess()
    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : ''
    })
    if (accessResult.status !== 'granted') {
      this.setData({ initialLoading: false })
      return
    }

    this.loadProfile(true, true)
  },

  onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    if (!this.data.initialLoading && !this.data.saving && !this.data.hasChanges) {
      if (shouldAutoRefresh(this.data.lastLoadedAt, PROFILE_AUTO_REFRESH_WINDOW_MS)) {
        this.loadProfile(false)
      }
    }
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      wx.stopPullDownRefresh()
      return
    }
    if (this.data.hasChanges) {
      wx.stopPullDownRefresh()
      wx.showToast({ title: '当前有未保存修改，请先保存后再刷新', icon: 'none' })
      return
    }
    this.loadProfile(false, true)
  },

  onRetryRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.loadProfile(false, true)
  },

  onRetryAccess() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: ''
    })
    this.onLoad()
  },

  async loadProfile(showLoading = true, force = false) {
    if (this.data.loading) return

    const hasExistingData = !this.data.initialLoading
    const isSilentRefresh = !showLoading && hasExistingData

    if (!force && hasExistingData && !shouldAutoRefresh(this.data.lastLoadedAt, PROFILE_AUTO_REFRESH_WINDOW_MS)) {
      wx.stopPullDownRefresh()
      return
    }

    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
        : isSilentRefresh
          ? { refreshErrorMessage: '' }
          : {})
    })

    try {
      const [profile, status] = await Promise.all([
        getMyMerchantProfile(),
        getMyMerchantOpenStatus().catch(() => null)
      ])
      const form = buildForm(profile)
      this.setData({
        merchantId: profile.id,
        version: profile.version,
        updatedAtLabel: profile.updated_at ? profile.updated_at.replace('T', ' ').slice(0, 16) : '--',
        isOpen: status?.is_open ?? profile.is_open,
        locationLabel: buildLocationLabel(form.address, form.latitude, form.longitude),
        locationHint: buildLocationHint(form.address, form.latitude, form.longitude),
        form,
        initialForm: { ...form },
        hasChanges: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
    } catch (err: unknown) {
      logger.error('Load merchant profile settings failed', err)
      const message = getErrorMessage(err, '店铺资料加载失败，请重试')

      if (this.data.initialLoading) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else if (isSilentRefresh) {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onInputChange(
    e: WechatMiniprogram.CustomEvent<{ value: string }> & { currentTarget: { dataset: { field: keyof MerchantProfileForm } } }
  ) {
    const field = e.currentTarget.dataset.field
    const nextForm = {
      ...this.data.form,
      [field]: e.detail.value
    }
    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      form: nextForm,
      hasChanges: hasFormChanged(nextForm, this.data.initialForm)
    })
  },

  validateForm() {
    const { name, phone, address, description, latitude, longitude } = this.data.form
    if (name.trim().length < 2) {
      wx.showToast({ title: '店铺名称至少 2 个字', icon: 'none' })
      return false
    }
    if (phone.trim() && phone.trim().length !== 11) {
      wx.showToast({ title: '联系电话需为 11 位手机号', icon: 'none' })
      return false
    }
    if (description.trim().length > 500) {
      wx.showToast({ title: '店铺介绍最多 500 字', icon: 'none' })
      return false
    }

    const latitudeValue = latitude.trim()
    const longitudeValue = longitude.trim()

    if (hasPartialLocation(this.data.form) && !hasCompleteLocation(this.data.form)) {
      wx.showToast({ title: '请通过“选择经营位置”更新完整位置', icon: 'none' })
      return false
    }

    if (address.trim() && address.trim().length < 5) {
      wx.showToast({ title: '请选择更准确的经营位置', icon: 'none' })
      return false
    }

    if (latitudeValue && !isCoordinateInRange(latitudeValue, 3, 54)) {
      wx.showToast({ title: '纬度需在 3.0 到 54.0 之间', icon: 'none' })
      return false
    }

    if (longitudeValue && !isCoordinateInRange(longitudeValue, 73, 135)) {
      wx.showToast({ title: '经度需在 73.0 到 135.0 之间', icon: 'none' })
      return false
    }

    return true
  },

  onChooseLocation() {
    if (this.data.loading || this.data.saving) {
      return
    }

    wx.chooseLocation({
      success: (result) => {
        const fullAddress = buildChosenLocationAddress(result)
        const nextForm = {
          ...this.data.form,
          address: fullAddress || this.data.form.address,
          latitude: String(result.latitude),
          longitude: String(result.longitude)
        }

        this.setData({
          actionNoticeMessage: '',
          refreshErrorMessage: '',
          form: nextForm,
          locationLabel: buildLocationLabel(nextForm.address, nextForm.latitude, nextForm.longitude),
          locationHint: buildLocationHint(nextForm.address, nextForm.latitude, nextForm.longitude),
          hasChanges: hasFormChanged(nextForm, this.data.initialForm)
        })
      },
      fail: (error) => {
        if (typeof error?.errMsg === 'string' && error.errMsg.includes('auth deny')) {
          wx.showModal({
            title: '需要位置权限',
            content: '请在设置中开启位置权限后再选择经营位置。',
            confirmText: '去设置',
            success: (result) => {
              if (result.confirm) {
                wx.openSetting()
              }
            }
          })
          return
        }

        if (typeof error?.errMsg === 'string' && error.errMsg.includes('cancel')) {
          return
        }

        wx.showToast({ title: '位置选择失败，请稍后重试', icon: 'none' })
      }
    })
  },

  async onSave() {
    if (this.data.saving || !this.data.hasChanges) return
    if (!this.validateForm()) return

    this.setData({ saving: true })
    wx.showLoading({ title: '保存中...' })

    try {
      const latitude = this.data.form.latitude.trim()
      const longitude = this.data.form.longitude.trim()
      const payload = {
        version: this.data.version,
        name: this.data.form.name.trim(),
        phone: this.data.form.phone.trim() || undefined,
        address: this.data.form.address.trim() || undefined,
        description: this.data.form.description.trim() || undefined,
        latitude: latitude || undefined,
        longitude: longitude || undefined
      }
      const updated = await updateMyMerchantProfile(payload)
      const form = buildForm(updated)
      this.setData({
        version: updated.version,
        updatedAtLabel: updated.updated_at ? updated.updated_at.replace('T', ' ').slice(0, 16) : '--',
        actionNoticeMessage: '店铺资料已保存。',
        locationLabel: buildLocationLabel(form.address, form.latitude, form.longitude),
        locationHint: buildLocationHint(form.address, form.latitude, form.longitude),
        form,
        initialForm: { ...form },
        hasChanges: false,
        lastLoadedAt: Date.now()
      })

      try {
        const currentMerchant = wx.getStorageSync('current_merchant') || {}
        wx.setStorageSync('current_merchant', {
          ...currentMerchant,
          id: updated.id,
          merchant_id: updated.id,
          name: updated.name
        })
      } catch (storageErr) {
        logger.warn('Sync merchant profile cache failed', storageErr)
      }
    } catch (err: unknown) {
      logger.error('Save merchant profile settings failed', err)
      const message = getErrorMessage(err, '保存失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ saving: false })
    }
  },

  onGoProfileImages() {
    wx.navigateTo({ url: '/pages/merchant/profile-images/index' })
  },

  onGoMerchantCategories() {
    wx.navigateTo({ url: '/pages/merchant/merchant-categories/index' })
  },

  onGoBusinessHours() {
    wx.navigateTo({ url: '/pages/merchant/settings/business-hours/index' })
  },

  onGoMembership() {
    wx.navigateTo({ url: '/pages/merchant/settings/membership/index' })
  },

  onGoApplication() {
    wx.navigateTo({ url: '/pages/merchant/settings/application/index' })
  },

  onGoApplyment() {
    wx.navigateTo({ url: '/pages/merchant/settings/applyment/index' })
  },

  onRetry() {
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    if (!this.data.accessReady || this.data.accessDenied) return
    this.loadProfile(true, true)
  }
})