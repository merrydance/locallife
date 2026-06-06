import { getStableBarHeights } from '../../../utils/responsive'
import { logger } from '../../../utils/logger'
import {
  refreshMerchantOnboardingV2Runtime,
  type MerchantOnboardingV2RuntimeState
} from '../_main_shared/services/merchant-onboarding-v2-runtime'
import type {
  MerchantOnboardingV2PrimaryAction,
  MerchantOnboardingV2ViewState
} from '../_main_shared/services/merchant-onboarding-v2-view'

const PLATFORM_APPLICATION_PATH = '/pages/register/merchant/store/index'
const BAOFU_SUBMIT_PATH = '/pages/merchant/onboarding-v2/baofu-submit/index'
const INTENT_GUIDE_PATH = '/pages/merchant/onboarding-v2/intent/index'

const EMPTY_RUNTIME_STATE: MerchantOnboardingV2RuntimeState = {
  lastTrustedViewState: null,
  requestSeq: 0
}

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshing: false,
    refreshError: '',
    navigating: false,
    fromBaofuSubmit: false,
    viewState: null as MerchantOnboardingV2ViewState | null
  },

  _runtimeState: { ...EMPTY_RUNTIME_STATE } as MerchantOnboardingV2RuntimeState,

  onLoad(query?: Record<string, string | undefined>) {
    const { navBarHeight } = getStableBarHeights()
    this.setData({
      navBarHeight,
      fromBaofuSubmit: query?.from === 'baofu_submit'
    })
    void this.loadProgress({ forceIntentAfterReady: query?.from === 'baofu_submit' })
  },

  onShow() {
    if (this.data.initialLoading) {
      return
    }
    void this.loadProgress({ silent: true, forceIntentAfterReady: this.data.fromBaofuSubmit })
  },

  onPullDownRefresh() {
    void this.loadProgress({ refreshing: true })
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  async loadProgress(options: { silent?: boolean, refreshing?: boolean, forceIntentAfterReady?: boolean } = {}) {
    if (this.data.refreshing && !options.refreshing) {
      return
    }

    const hasTrustedView = !!this.data.viewState
    if (options.refreshing || hasTrustedView || options.silent) {
      this.setData({ refreshing: true, refreshError: '' })
    } else {
      this.setData({
        initialLoading: true,
        initialError: false,
        initialErrorMessage: '',
        refreshError: ''
      })
    }

    try {
      const result = await refreshMerchantOnboardingV2Runtime(this._runtimeState)
      const viewState = result.viewState
      if (result.ok && viewState) {
        this.setData({
          viewState,
          initialLoading: false,
          initialError: false,
          initialErrorMessage: '',
          refreshError: '',
          refreshing: false
        })
        if (options.forceIntentAfterReady && viewState.baofuState === 'baofu_ready') {
          this.setData({ fromBaofuSubmit: false })
          this.navigateToIntentGuide()
        }
        return
      }

      if (viewState) {
        this.setData({
          viewState,
          initialLoading: false,
          initialError: false,
          refreshError: result.errorMessage,
          refreshing: false
        })
        return
      }

      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: result.errorMessage || '入驻进度加载失败，请稍后重试',
        refreshing: false
      })
    } catch (error: unknown) {
      logger.error('Load merchant onboarding v2 failed action=load_progress', error, 'merchant-onboarding-v2')
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: '入驻进度加载失败，请稍后重试',
        refreshing: false
      })
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  onRetry() {
    void this.loadProgress()
  },

  onRefresh() {
    void this.loadProgress({ refreshing: true })
  },

  onPrimaryAction() {
    const action = this.data.viewState?.primaryAction || 'none'
    this.handleAction(action)
  },

  onSecondaryAction() {
    const action = this.data.viewState?.secondaryAction || 'refresh'
    this.handleAction(action)
  },

  handleAction(action: MerchantOnboardingV2PrimaryAction) {
    if (this.data.navigating) {
      return
    }

    switch (action) {
      case 'start_platform':
      case 'continue_platform':
        this.navigateTo(PLATFORM_APPLICATION_PATH)
        break
      case 'submit_baofu':
        this.navigateTo(BAOFU_SUBMIT_PATH)
        break
      case 'open_intent':
        this.navigateToIntentGuide()
        break
      case 'contact_support':
        this.copySupportId()
        break
      case 'refresh':
        void this.loadProgress({ refreshing: true })
        break
      default:
        break
    }
  },

  navigateToIntentGuide() {
    this.navigateTo(INTENT_GUIDE_PATH)
  },

  navigateTo(url: string) {
    if (this.data.navigating) {
      return
    }
    this.setData({ navigating: true })
    wx.navigateTo({
      url,
      complete: () => {
        this.setData({ navigating: false })
      }
    })
  },

  copySupportId() {
    const text = this.data.viewState?.supportCopyText || ''
    if (!text) {
      wx.showToast({ title: '请联系平台客服处理', icon: 'none' })
      return
    }

    wx.setClipboardData({
      data: text,
      success: () => {
        wx.showToast({ title: '商户编号已复制', icon: 'success' })
      }
    })
  }
})
