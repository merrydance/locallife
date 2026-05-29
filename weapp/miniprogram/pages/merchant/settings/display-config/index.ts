import { displayConfigService, DisplayConfigResponse } from '../../../../api/table-device-management'
import {
  ensureMerchantDeviceManagementAccess,
  getMerchantDeviceManagementErrorMessage,
  isMerchantDeviceManagementDenied,
  isMerchantDeviceManagementGranted
} from '../../../../utils/console-access'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface DisplayConfigForm {
  enable_print: boolean
  print_takeout: boolean
  print_dine_in: boolean
  print_reservation: boolean
  print_dispatch_mode: 'single_full' | 'split'
  print_trigger_mode: 'accepted' | 'ready' | 'manual'
  enable_voice: boolean
  voice_takeout: boolean
  voice_dine_in: boolean
}

interface DisplayConfigOption<T extends string> {
  label: string
  value: T
  desc: string
}

const PRINT_DISPATCH_MODE_OPTIONS: Array<DisplayConfigOption<DisplayConfigForm['print_dispatch_mode']>> = [
  { label: '整单分发', value: 'single_full', desc: '每台打印机收到完整订单，适合前台与综合出单场景' },
  { label: '按职责拆单', value: 'split', desc: '结合打印机角色拆分前台与后厨单据，减少重复打印' }
]

const PRINT_TRIGGER_MODE_OPTIONS: Array<DisplayConfigOption<DisplayConfigForm['print_trigger_mode']>> = [
  { label: '接单即打印', value: 'accepted', desc: '商户确认接单后立即触发打印' },
  { label: '备餐完成后打印', value: 'ready', desc: '适合在出餐节点再分发小票' },
  { label: '仅手动打印', value: 'manual', desc: '保留订单内手动打印，不自动推送小票' }
]

const DISPLAY_CONFIG_AUTO_REFRESH_WINDOW_MS = 60 * 1000
const getErrorMessage = getErrorUserMessage

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

function buildDisplayConfigForm(config?: DisplayConfigResponse): DisplayConfigForm {
  return {
    enable_print: Boolean(config?.enable_print),
    print_takeout: Boolean(config?.print_takeout),
    print_dine_in: Boolean(config?.print_dine_in),
    print_reservation: Boolean(config?.print_reservation),
    print_dispatch_mode: config?.print_dispatch_mode === 'split' ? 'split' : 'single_full',
    print_trigger_mode: config?.print_trigger_mode === 'ready' || config?.print_trigger_mode === 'manual'
      ? config.print_trigger_mode
      : 'accepted',
    enable_voice: Boolean(config?.enable_voice),
    voice_takeout: Boolean(config?.voice_takeout),
    voice_dine_in: Boolean(config?.voice_dine_in)
  }
}

function hasDisplayConfigChanged(current: DisplayConfigForm, initial: DisplayConfigForm) {
  return JSON.stringify(current) !== JSON.stringify(initial)
}

function showPageMessage(
  context: WechatMiniprogram.Page.TrivialInstance,
  offsetTop: number,
  content: string,
  duration = 2200
) {
  void context
  void offsetTop
  wx.showToast({ title: content, icon: 'none', duration })
}

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    settingsAccessDenied: false,
    settingsAccessDeniedMessage: '',
    settingsInitialLoading: false,
    settingsLoadedOnce: false,
    settingsInitialError: false,
    settingsInitialErrorMessage: '',
    settingsRefreshErrorMessage: '',
    settingsLoading: false,
    settingsSaving: false,
    settingsLastLoadedAt: 0,
    settingsHasChanges: false,
    settingsForm: buildDisplayConfigForm() as DisplayConfigForm,
    settingsInitialForm: buildDisplayConfigForm() as DisplayConfigForm,
    printDispatchModeOptions: PRINT_DISPATCH_MODE_OPTIONS,
    printTriggerModeOptions: PRINT_TRIGGER_MODE_OPTIONS
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({
      navBarHeight,
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      settingsAccessDenied: false,
      settingsAccessDeniedMessage: ''
    })

    const accessResult = await ensureMerchantDeviceManagementAccess({ force: true })
    if (!isMerchantDeviceManagementGranted(accessResult)) {
      this.setData({
        accessReady: true,
        accessDenied: !isMerchantDeviceManagementDenied(accessResult) && !getMerchantDeviceManagementErrorMessage(accessResult),
        accessErrorMessage: getMerchantDeviceManagementErrorMessage(accessResult),
        settingsAccessDenied: isMerchantDeviceManagementDenied(accessResult),
        settingsAccessDeniedMessage: isMerchantDeviceManagementDenied(accessResult) ? accessResult.message : ''
      })
      return
    }

    this.setData({
      accessReady: true,
      accessDenied: false,
      accessErrorMessage: '',
      settingsAccessDenied: false,
      settingsAccessDeniedMessage: ''
    })

    await this.loadSettingsConfig(true, true)
  },

  onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage || this.data.settingsAccessDenied) {
      return
    }

    if (this.data.settingsLoadedOnce && !this.data.settingsSaving && !this.data.settingsHasChanges) {
      if (shouldAutoRefresh(this.data.settingsLastLoadedAt, DISPLAY_CONFIG_AUTO_REFRESH_WINDOW_MS)) {
        this.loadSettingsConfig(false)
      }
    }
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage || this.data.settingsAccessDenied) {
      wx.stopPullDownRefresh()
      return
    }

    if (this.data.settingsHasChanges) {
      wx.stopPullDownRefresh()
      return
    }

    this.loadSettingsConfig(false, true).catch(() => undefined)
  },

  async loadSettingsConfig(showLoading = true, force = false) {
    if (this.data.settingsLoading) {
      return
    }

    if (!force && this.data.settingsLoadedOnce && !shouldAutoRefresh(this.data.settingsLastLoadedAt, DISPLAY_CONFIG_AUTO_REFRESH_WINDOW_MS)) {
      return
    }

    const isSilentRefresh = !showLoading && this.data.settingsLoadedOnce

    this.setData({
      settingsLoading: true,
      ...(showLoading
        ? { settingsInitialError: false, settingsInitialErrorMessage: '', settingsRefreshErrorMessage: '' }
        : isSilentRefresh
          ? { settingsRefreshErrorMessage: '' }
          : {})
    })

    if (showLoading && !this.data.settingsLoadedOnce) {
      this.setData({ settingsInitialLoading: true })
    }

    try {
      const config = await displayConfigService.getDisplayConfig()
      const settingsForm = buildDisplayConfigForm(config)
      this.setData({
        settingsForm,
        settingsInitialForm: JSON.parse(JSON.stringify(settingsForm)),
        settingsHasChanges: false,
        settingsInitialLoading: false,
        settingsLoadedOnce: true,
        settingsInitialError: false,
        settingsInitialErrorMessage: '',
        settingsRefreshErrorMessage: '',
        settingsLastLoadedAt: Date.now()
      })
    } catch (err) {
      logger.error('Load display config failed', err)
      const message = getErrorMessage(err, '后厨协同配置加载失败，请重试')

      if (!this.data.settingsLoadedOnce) {
        this.setData({
          settingsInitialLoading: false,
          settingsInitialError: true,
          settingsInitialErrorMessage: message
        })
      } else if (isSilentRefresh) {
        this.setData({ settingsRefreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        showPageMessage(this, this.data.navBarHeight, message)
      }
    } finally {
      this.setData({ settingsLoading: false })
      wx.stopPullDownRefresh()
    }
  },

  onRetryAccess() {
    this.onLoad()
  },

  onRetrySettings() {
    this.loadSettingsConfig(true, true)
  },

  onSettingsSwitchChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { field } = e.currentTarget.dataset as { field: keyof DisplayConfigForm }
    const settingsForm = {
      ...this.data.settingsForm,
      [field]: !!e.detail?.value
    }
    this.setData({
      settingsForm,
      settingsRefreshErrorMessage: '',
      settingsHasChanges: hasDisplayConfigChanged(settingsForm, this.data.settingsInitialForm)
    })
  },

  onDispatchModeSelect(e: WechatMiniprogram.CustomEvent) {
    const { value } = e.currentTarget.dataset as { value?: DisplayConfigForm['print_dispatch_mode'] }
    if (!value || value === this.data.settingsForm.print_dispatch_mode) {
      return
    }

    const detail = e.detail as boolean | { checked?: boolean } | undefined
    const nextChecked = typeof detail === 'boolean' ? detail : !!detail?.checked
    if (!nextChecked) {
      return
    }

    const settingsForm = {
      ...this.data.settingsForm,
      print_dispatch_mode: value
    }
    this.setData({
      settingsForm,
      settingsRefreshErrorMessage: '',
      settingsHasChanges: hasDisplayConfigChanged(settingsForm, this.data.settingsInitialForm)
    })
  },

  onTriggerModeSelect(e: WechatMiniprogram.CustomEvent) {
    const { value } = e.currentTarget.dataset as { value?: DisplayConfigForm['print_trigger_mode'] }
    if (!value || value === this.data.settingsForm.print_trigger_mode) {
      return
    }

    const detail = e.detail as boolean | { checked?: boolean } | undefined
    const nextChecked = typeof detail === 'boolean' ? detail : !!detail?.checked
    if (!nextChecked) {
      return
    }

    const settingsForm = {
      ...this.data.settingsForm,
      print_trigger_mode: value
    }
    this.setData({
      settingsForm,
      settingsRefreshErrorMessage: '',
      settingsHasChanges: hasDisplayConfigChanged(settingsForm, this.data.settingsInitialForm)
    })
  },

  async onSaveSettings() {
    if (this.data.settingsSaving || this.data.settingsInitialLoading || this.data.settingsInitialError) {
      return
    }

    if (!this.data.settingsHasChanges) {
      return
    }

    this.setData({ settingsSaving: true })
    wx.showLoading({ title: '保存中...' })

    try {
      const savedConfig = await displayConfigService.updateDisplayConfig({
        enable_print: this.data.settingsForm.enable_print,
        print_takeout: this.data.settingsForm.print_takeout,
        print_dine_in: this.data.settingsForm.print_dine_in,
        print_reservation: this.data.settingsForm.print_reservation,
        print_dispatch_mode: this.data.settingsForm.print_dispatch_mode,
        print_trigger_mode: this.data.settingsForm.print_trigger_mode,
        enable_voice: this.data.settingsForm.enable_voice,
        voice_takeout: this.data.settingsForm.voice_takeout,
        voice_dine_in: this.data.settingsForm.voice_dine_in
      })
      const settingsForm = buildDisplayConfigForm(savedConfig)
      this.setData({
        settingsForm,
        settingsInitialForm: JSON.parse(JSON.stringify(settingsForm)),
        settingsHasChanges: false,
        settingsRefreshErrorMessage: '',
        settingsLastLoadedAt: Date.now()
      })
      wx.navigateBack()
    } catch (err) {
      logger.error('Save display config failed', err)
      showPageMessage(this, this.data.navBarHeight, getErrorMessage(err, '保存失败，请稍后重试'), 2500)
    } finally {
      wx.hideLoading()
      this.setData({ settingsSaving: false })
    }
  }
})
