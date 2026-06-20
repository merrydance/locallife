import {
  MerchantPackagingOrderType,
  MerchantPackagingService
} from '../_main_shared/api/packaging'
import { ensureMerchantConsoleAccess } from '../../../utils/console-access'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'
import {
  buildPackagingDraft,
  buildPackagingSaveFailurePatch,
  clonePackagingDraft,
  createBlankPackagingOption,
  DEFAULT_ORDER_TYPES,
  hasPackagingDraftChanged,
  normalizePackagingPriceYuan,
  ORDER_TYPE_OPTIONS,
  PACKAGING_AUTO_REFRESH_WINDOW_MS,
  PackagingSettingsDraft,
  replacePackagingOptionAt,
  savePackagingDraft,
  shouldRefreshPackagingSettings,
  validatePackagingDraft
} from '../_utils/merchant-packaging-settings-view'

if (typeof Page !== 'undefined') {
  Page({
    data: {
      navBarHeight: 88,
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: '',
      saveErrorMessage: '',
      loading: false,
      saving: false,
      lastLoadedAt: 0,
      orderTypeOptions: ORDER_TYPE_OPTIONS,
      form: buildPackagingDraft({
        merchant_id: 0,
        enabled: false,
        required: true,
        applicable_order_types: DEFAULT_ORDER_TYPES
      }, []) as PackagingSettingsDraft,
      initialForm: buildPackagingDraft({
        merchant_id: 0,
        enabled: false,
        required: true,
        applicable_order_types: DEFAULT_ORDER_TYPES
      }, []) as PackagingSettingsDraft,
      hasChanges: false
    },

    async onLoad() {
      const { navBarHeight } = getStableBarHeights()
      this.setData({
        navBarHeight,
        accessReady: false,
        accessDenied: false,
        accessErrorMessage: '',
        initialLoading: true,
        initialError: false,
        initialErrorMessage: ''
      })

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

      void this.loadSettings(true, true)
    },

    onShow() {
      if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
        return
      }
      if (!this.data.initialLoading && !this.data.saving && !this.data.hasChanges) {
        if (shouldRefreshPackagingSettings(this.data.lastLoadedAt, PACKAGING_AUTO_REFRESH_WINDOW_MS)) {
          void this.loadSettings(false)
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
        wx.showToast({ title: '当前有未保存修改，请先保存后再同步', icon: 'none' })
        return
      }
      void this.loadSettings(false, true)
    },

    onRetryAccess() {
      void this.onLoad()
    },

    onRetry() {
      if (this.data.accessErrorMessage) {
        this.onRetryAccess()
        return
      }
      if (!this.data.accessReady || this.data.accessDenied) {
        return
      }
      void this.loadSettings(true, true)
    },

    onRetryRefresh() {
      if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
        return
      }
      void this.loadSettings(false, true)
    },

    async loadSettings(showLoading = true, force = false) {
      if (this.data.loading) {
        wx.stopPullDownRefresh()
        return false
      }

      const hasExistingData = !this.data.initialLoading
      const isSilentRefresh = !showLoading && hasExistingData
      if (!force && hasExistingData && !shouldRefreshPackagingSettings(this.data.lastLoadedAt, PACKAGING_AUTO_REFRESH_WINDOW_MS)) {
        wx.stopPullDownRefresh()
        return true
      }

      this.setData({
        loading: true,
        ...(showLoading
          ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '', saveErrorMessage: '' }
          : isSilentRefresh
            ? { refreshErrorMessage: '' }
            : {})
      })

      try {
        const [settings, options] = await Promise.all([
          MerchantPackagingService.getSettings(),
          MerchantPackagingService.listOptions()
        ])
        const form = buildPackagingDraft(settings, options)
        this.setData({
          form,
          initialForm: clonePackagingDraft(form),
          hasChanges: false,
          initialLoading: false,
          initialError: false,
          initialErrorMessage: '',
          refreshErrorMessage: '',
          saveErrorMessage: '',
          lastLoadedAt: Date.now()
        })
        return true
      } catch (err) {
        logger.error('Load merchant packaging settings failed', err)
        const message = getErrorUserMessage(err, '包装设置加载失败，请重试')
        if (this.data.initialLoading) {
          this.setData({
            initialLoading: false,
            initialError: true,
            initialErrorMessage: message
          })
        } else if (hasExistingData) {
          this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
        } else {
          wx.showToast({ title: message, icon: 'none' })
        }
        return false
      } finally {
        this.setData({ loading: false })
        wx.stopPullDownRefresh()
      }
    },

    applyFormPatch(form: PackagingSettingsDraft) {
      if (this.data.saving) return
      this.setData({
        form,
        refreshErrorMessage: '',
        saveErrorMessage: '',
        hasChanges: hasPackagingDraftChanged(form, this.data.initialForm)
      })
    },

    onToggleSetting(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
      const { field } = e.currentTarget.dataset as { field?: 'enabled' | 'required' }
      if (!field) {
        return
      }
      const form = {
        ...this.data.form,
        [field]: !!e.detail?.value
      }
      this.applyFormPatch(form)
    },

    onToggleOrderType(e: WechatMiniprogram.CustomEvent) {
      const { orderType } = e.currentTarget.dataset as { orderType?: MerchantPackagingOrderType }
      if (!orderType) {
        return
      }
      const detail = e.detail as boolean | { checked?: boolean } | undefined
      const nextChecked = typeof detail === 'boolean' ? detail : !!detail?.checked
      const selected = new Set(this.data.form.applicable_order_types)
      if (nextChecked) {
        selected.add(orderType)
      } else {
        selected.delete(orderType)
      }
      const form = {
        ...this.data.form,
        applicable_order_types: DEFAULT_ORDER_TYPES.filter((item) => selected.has(item))
      }
      this.applyFormPatch(form)
    },

    onOptionInputChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
      const { localId, field } = e.currentTarget.dataset as {
        localId?: string
        field?: 'name' | 'description' | 'price_yuan'
      }
      if (!localId || !field) {
        return
      }
      const value = e.detail?.value || ''
      const form = {
        ...this.data.form,
        options: replacePackagingOptionAt(this.data.form.options, localId, (option) => ({
          ...option,
          [field]: field === 'name' || field === 'description' ? value.replace(/^\s+/, '') : value.trim()
        }))
      }
      this.applyFormPatch(form)
    },

    onOptionPriceBlur(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
      const { localId } = e.currentTarget.dataset as { localId?: string }
      if (!localId) {
        return
      }
      const value = normalizePackagingPriceYuan(e.detail?.value || '')
      const form = {
        ...this.data.form,
        options: replacePackagingOptionAt(this.data.form.options, localId, (option) => ({
          ...option,
          price_yuan: value || '0'
        }))
      }
      this.applyFormPatch(form)
    },

    onOptionSwitchChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
      const { localId, field } = e.currentTarget.dataset as {
        localId?: string
        field?: 'is_enabled'
      }
      if (!localId || field !== 'is_enabled') {
        return
      }
      const enabled = !!e.detail?.value
      const currentOption = this.data.form.options.find((option) => option.local_id === localId)
      const form = {
        ...this.data.form,
        default_option_id: currentOption?.id === this.data.form.default_option_id && !enabled ? 0 : this.data.form.default_option_id,
        options: replacePackagingOptionAt(this.data.form.options, localId, (option) => ({
          ...option,
          is_enabled: enabled
        }))
      }
      this.applyFormPatch(form)
    },

    onSelectDefaultOption(e: WechatMiniprogram.CustomEvent) {
      if (this.data.saving) return
      const { localId } = e.currentTarget.dataset as { localId?: string }
      const option = this.data.form.options.find((item) => item.local_id === localId)
      if (!option || option.deleted || !option.is_enabled || option.id <= 0) {
        wx.showToast({ title: '保存后才能设为默认包装', icon: 'none' })
        return
      }
      const form = {
        ...this.data.form,
        default_option_id: option.id === this.data.form.default_option_id ? 0 : option.id
      }
      this.applyFormPatch(form)
    },

    onAddOption() {
      if (this.data.saving) return
      const options = [...this.data.form.options, createBlankPackagingOption(this.data.form.options.length)]
      this.applyFormPatch({ ...this.data.form, options })
    },

    onRemoveOption(e: WechatMiniprogram.TouchEvent) {
      if (this.data.saving) return
      const { localId } = e.currentTarget.dataset as { localId?: string }
      if (!localId) {
        return
      }
      const target = this.data.form.options.find((option) => option.local_id === localId)
      if (!target) {
        return
      }
      const options = target.id > 0
        ? replacePackagingOptionAt(this.data.form.options, localId, (option) => ({ ...option, deleted: true }))
        : this.data.form.options.filter((option) => option.local_id !== localId)
      const form = {
        ...this.data.form,
        default_option_id: target.id === this.data.form.default_option_id ? 0 : this.data.form.default_option_id,
        options
      }
      this.applyFormPatch(form)
    },

    onRestoreOption(e: WechatMiniprogram.TouchEvent) {
      if (this.data.saving) return
      const { localId } = e.currentTarget.dataset as { localId?: string }
      if (!localId) {
        return
      }
      const form = {
        ...this.data.form,
        options: replacePackagingOptionAt(this.data.form.options, localId, (option) => ({ ...option, deleted: false }))
      }
      this.applyFormPatch(form)
    },

    async onSave() {
      if (this.data.saving || this.data.initialLoading || this.data.initialError) {
        return
      }
      if (!this.data.hasChanges) {
        return
      }

      try {
        validatePackagingDraft(this.data.form)
      } catch (err) {
        const message = err instanceof Error ? err.message : '包装设置填写不完整'
        this.setData({ saveErrorMessage: message })
        wx.showToast({ title: message, icon: 'none' })
        return
      }

      const currentForm = clonePackagingDraft(this.data.form)
      this.setData({ saving: true, saveErrorMessage: '' })

      try {
        const savedForm = await savePackagingDraft(currentForm)
        this.setData({
          form: savedForm,
          initialForm: clonePackagingDraft(savedForm),
          hasChanges: false,
          refreshErrorMessage: '',
          saveErrorMessage: '',
          lastLoadedAt: Date.now()
        })

        const reloaded = await this.loadSettings(true, true)
        if (reloaded) {
          wx.showToast({ title: '包装设置已保存', icon: 'success' })
        } else {
          this.setData({
            form: savedForm,
            initialForm: clonePackagingDraft(savedForm),
            hasChanges: false,
            refreshErrorMessage: '包装设置已保存，但最新状态同步失败，请稍后重新进入确认'
          })
          wx.showToast({ title: '已保存，稍后同步确认', icon: 'none' })
        }
      } catch (err) {
        logger.error('Save merchant packaging settings failed', err)
        this.setData(buildPackagingSaveFailurePatch(err, currentForm))
      } finally {
        this.setData({ saving: false })
      }
    }
  })
}
