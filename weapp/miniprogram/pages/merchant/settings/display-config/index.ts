import { displayConfigService, DisplayConfigResponse } from '../../../../api/table-device-management'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface DisplayConfigForm {
  enable_print: boolean
  enable_voice: boolean
  enable_kds: boolean
  print_takeout: boolean
  print_dine_in: boolean
  print_reservation: boolean
  voice_takeout: boolean
  voice_dine_in: boolean
  kds_url: string
}

function normalizeForm(form: DisplayConfigForm): DisplayConfigForm {
  return {
    ...form,
    print_takeout: form.enable_print ? form.print_takeout : false,
    print_dine_in: form.enable_print ? form.print_dine_in : false,
    print_reservation: form.enable_print ? form.print_reservation : false,
    voice_takeout: form.enable_voice ? form.voice_takeout : false,
    voice_dine_in: form.enable_voice ? form.voice_dine_in : false
  }
}

function buildForm(config?: DisplayConfigResponse): DisplayConfigForm {
  return normalizeForm({
    enable_print: Boolean(config?.enable_print),
    enable_voice: Boolean(config?.enable_voice),
    enable_kds: Boolean(config?.enable_kds),
    print_takeout: Boolean(config?.print_takeout),
    print_dine_in: Boolean(config?.print_dine_in),
    print_reservation: Boolean(config?.print_reservation),
    voice_takeout: Boolean(config?.voice_takeout),
    voice_dine_in: Boolean(config?.voice_dine_in),
    kds_url: config?.kds_url || ''
  })
}

function hasFormChanged(current: DisplayConfigForm, initial: DisplayConfigForm) {
  return JSON.stringify(current) !== JSON.stringify(initial)
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: false,
    saving: false,
    form: buildForm() as DisplayConfigForm,
    initialForm: buildForm() as DisplayConfigForm,
    hasChanges: false
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadConfig()
  },

  onShow() {
    if (!this.data.initialLoading && !this.data.saving && !this.data.hasChanges) {
      this.loadConfig(false)
    }
  },

  onPullDownRefresh() {
    if (this.data.hasChanges) {
      wx.stopPullDownRefresh()
      wx.showToast({ title: '当前有未保存修改，请先保存后再刷新', icon: 'none' })
      return
    }
    this.loadConfig(false)
  },

  onRetryRefresh() {
    this.loadConfig(false)
  },

  async loadConfig(showLoading = true) {
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
      const config = await displayConfigService.getDisplayConfig()
      const form = buildForm(config)
      this.setData({
        form,
        initialForm: JSON.parse(JSON.stringify(form)),
        hasChanges: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } catch (err) {
      logger.error('Load merchant display config failed', err)
      const message = getErrorMessage(err, '显示配置加载失败，请重试')

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

  onToggleSwitch(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { field } = e.currentTarget.dataset as { field: keyof DisplayConfigForm }
    const nextForm = {
      ...this.data.form,
      [field]: e.detail.value
    }
    const form = normalizeForm(nextForm)
    this.setData({
      refreshErrorMessage: '',
      form,
      hasChanges: hasFormChanged(form, this.data.initialForm)
    })
  },

  onKdsUrlChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const form = normalizeForm({
      ...this.data.form,
      kds_url: e.detail.value || ''
    })
    this.setData({
      refreshErrorMessage: '',
      form,
      hasChanges: hasFormChanged(form, this.data.initialForm)
    })
  },

  validateForm() {
    const { enable_kds, kds_url } = this.data.form
    const trimmedUrl = kds_url.trim()

    if (enable_kds && trimmedUrl && !/^https?:\/\//.test(trimmedUrl)) {
      wx.showToast({ title: 'KDS 地址需以 http:// 或 https:// 开头', icon: 'none' })
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
      const normalizedForm = normalizeForm(this.data.form)
      const updated = await displayConfigService.updateDisplayConfig({
        ...normalizedForm,
        kds_url: normalizedForm.kds_url.trim() || undefined
      })
      const form = buildForm(updated)
      this.setData({
        form,
        initialForm: JSON.parse(JSON.stringify(form)),
        hasChanges: false,
        refreshErrorMessage: ''
      })
    } catch (err) {
      logger.error('Save merchant display config failed', err)
      wx.showToast({ title: getErrorMessage(err, '保存失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ saving: false })
    }
  },

  onRetry() {
    this.loadConfig()
  }
})