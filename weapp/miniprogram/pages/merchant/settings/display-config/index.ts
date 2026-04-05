import { displayConfigService, DisplayConfigResponse } from '../../../../api/table-device-management'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'

interface DisplayConfigForm {
  enable_print: boolean
  enable_voice: boolean
  enable_kds: boolean
  print_takeout: boolean
  print_dine_in: boolean
  print_reservation: boolean
  print_dispatch_mode: 'single_full' | 'split'
  print_trigger_mode: 'accepted' | 'ready' | 'manual'
  voice_takeout: boolean
  voice_dine_in: boolean
  kds_url: string
}

const DISPLAY_CONFIG_AUTO_REFRESH_WINDOW_MS = 60 * 1000

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
    print_dispatch_mode: config?.print_dispatch_mode === 'split' ? 'split' : 'single_full',
    print_trigger_mode: config?.print_trigger_mode === 'ready' || config?.print_trigger_mode === 'manual'
      ? config.print_trigger_mode
      : 'accepted',
    voice_takeout: Boolean(config?.voice_takeout),
    voice_dine_in: Boolean(config?.voice_dine_in),
    kds_url: config?.kds_url || ''
  })
}

function hasFormChanged(current: DisplayConfigForm, initial: DisplayConfigForm) {
  return JSON.stringify(current) !== JSON.stringify(initial)
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
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
    form: buildForm() as DisplayConfigForm,
    initialForm: buildForm() as DisplayConfigForm,
    hasChanges: false,
    printDispatchModeOptions: [
      { label: '整单分发', value: 'single_full', desc: '每台打印机收到完整订单，适合前台与综合出单场景' },
      { label: '按职责拆单', value: 'split', desc: '结合打印机角色拆分前台与后厨单据，减少重复打印' }
    ],
    printTriggerModeOptions: [
      { label: '接单即打印', value: 'accepted', desc: '商户确认接单后立即触发打印' },
      { label: '备餐完成后打印', value: 'ready', desc: '适合需要在出餐节点再分发小票的场景' },
      { label: '仅手动打印', value: 'manual', desc: '保留订单内手动打印，不自动推送小票' }
    ]
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

    this.loadConfig(true, true)
  },

  onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    if (!this.data.initialLoading && !this.data.saving && !this.data.hasChanges) {
      if (shouldAutoRefresh(this.data.lastLoadedAt, DISPLAY_CONFIG_AUTO_REFRESH_WINDOW_MS)) {
        this.loadConfig(false)
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
    this.loadConfig(false, true)
  },

  onRetryRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.loadConfig(false, true)
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

  async loadConfig(showLoading = true, force = false) {
    if (this.data.loading) return

    const hasExistingData = !this.data.initialLoading
    const isSilentRefresh = !showLoading && hasExistingData

    if (!force && hasExistingData && !shouldAutoRefresh(this.data.lastLoadedAt, DISPLAY_CONFIG_AUTO_REFRESH_WINDOW_MS)) {
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
      const config = await displayConfigService.getDisplayConfig()
      const form = buildForm(config)
      this.setData({
        form,
        initialForm: JSON.parse(JSON.stringify(form)),
        actionNoticeMessage: '',
        hasChanges: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
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
      actionNoticeMessage: '',
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
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      form,
      hasChanges: hasFormChanged(form, this.data.initialForm)
    })
  },

  onDispatchModeChange(e: WechatMiniprogram.CustomEvent) {
    const form = {
      ...this.data.form,
      print_dispatch_mode: e.detail.value as DisplayConfigForm['print_dispatch_mode']
    }
    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      form,
      hasChanges: hasFormChanged(form, this.data.initialForm)
    })
  },

  onTriggerModeChange(e: WechatMiniprogram.CustomEvent) {
    const form = {
      ...this.data.form,
      print_trigger_mode: e.detail.value as DisplayConfigForm['print_trigger_mode']
    }
    this.setData({
      actionNoticeMessage: '',
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
        actionNoticeMessage: '显示设置已保存。',
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
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    if (!this.data.accessReady || this.data.accessDenied) return
    this.loadConfig(true, true)
  }
})