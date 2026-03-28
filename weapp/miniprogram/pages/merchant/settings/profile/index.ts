import { getMyMerchantOpenStatus, getMyMerchantProfile, MerchantOperatorResponse, updateMyMerchantProfile } from '../../../../api/merchant'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'

interface MerchantProfileForm {
  name: string
  phone: string
  address: string
  description: string
}

const EMPTY_FORM: MerchantProfileForm = {
  name: '',
  phone: '',
  address: '',
  description: ''
}

function buildForm(profile: MerchantOperatorResponse): MerchantProfileForm {
  return {
    name: profile.name || '',
    phone: profile.phone || '',
    address: profile.address || '',
    description: profile.description || ''
  }
}

function hasFormChanged(current: MerchantProfileForm, initial: MerchantProfileForm) {
  return current.name !== initial.name
    || current.phone !== initial.phone
    || current.address !== initial.address
    || current.description !== initial.description
}

function getErrorMessage(err: unknown, fallback: string) {
  if (typeof err === 'object' && err !== null && 'userMessage' in err) {
    const userMessage = (err as { userMessage?: unknown }).userMessage
    if (typeof userMessage === 'string' && userMessage.trim()) {
      return userMessage
    }
  }
  return fallback
}

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: false,
    saving: false,
    isOpen: false,
    merchantId: 0,
    version: 0,
    updatedAtLabel: '--',
    form: { ...EMPTY_FORM } as MerchantProfileForm,
    initialForm: { ...EMPTY_FORM } as MerchantProfileForm,
    hasChanges: false
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadProfile()
  },

  onShow() {
    if (!this.data.initialLoading && !this.data.saving && !this.data.hasChanges) {
      this.loadProfile(false)
    }
  },

  onPullDownRefresh() {
    if (this.data.hasChanges) {
      wx.stopPullDownRefresh()
      wx.showToast({ title: '当前有未保存修改，请先保存后再刷新', icon: 'none' })
      return
    }
    this.loadProfile(false)
  },

  onRetryRefresh() {
    this.loadProfile(false)
  },

  async loadProfile(showLoading = true) {
    if (this.data.loading) return

    const hasExistingData = !this.data.initialLoading
    const isSilentRefresh = !showLoading && hasExistingData

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
        form,
        initialForm: { ...form },
        hasChanges: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
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
      refreshErrorMessage: '',
      form: nextForm,
      hasChanges: hasFormChanged(nextForm, this.data.initialForm)
    })
  },

  validateForm() {
    const { name, phone, address, description } = this.data.form
    if (name.trim().length < 2) {
      wx.showToast({ title: '店铺名称至少 2 个字', icon: 'none' })
      return false
    }
    if (phone.trim() && phone.trim().length !== 11) {
      wx.showToast({ title: '联系电话需为 11 位手机号', icon: 'none' })
      return false
    }
    if (address.trim() && address.trim().length < 5) {
      wx.showToast({ title: '店铺地址至少 5 个字', icon: 'none' })
      return false
    }
    if (description.trim().length > 500) {
      wx.showToast({ title: '店铺介绍最多 500 字', icon: 'none' })
      return false
    }
    return true
  },

  async onSave() {
    if (this.data.saving || !this.data.hasChanges) return
    if (!this.validateForm()) return

    this.setData({ saving: true })
    wx.showLoading({ title: '保存中...' })

    try {
      const payload = {
        version: this.data.version,
        name: this.data.form.name.trim(),
        phone: this.data.form.phone.trim() || undefined,
        address: this.data.form.address.trim() || undefined,
        description: this.data.form.description.trim() || undefined
      }
      const updated = await updateMyMerchantProfile(payload)
      const form = buildForm(updated)
      this.setData({
        version: updated.version,
        updatedAtLabel: updated.updated_at ? updated.updated_at.replace('T', ' ').slice(0, 16) : '--',
        form,
        initialForm: { ...form },
        hasChanges: false
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

      wx.showToast({ title: '店铺资料已保存', icon: 'success' })
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
    this.loadProfile()
  }
})