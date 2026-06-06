import { getStableBarHeights } from '../../../../utils/responsive'
import { logger } from '../../../../utils/logger'
import {
  refreshMerchantOnboardingV2Runtime,
  type MerchantOnboardingV2RuntimeState
} from '../../_main_shared/services/merchant-onboarding-v2-runtime'
import {
  buildMerchantOnboardingV2IntentViewState,
  type MerchantOnboardingV2IntentViewState,
  type MerchantOnboardingV2ViewState
} from '../../_main_shared/services/merchant-onboarding-v2-view'

type QrSaveState =
  | 'idle'
  | 'downloading'
  | 'saving'
  | 'saved'
  | 'download_failed'
  | 'save_failed'
  | 'permission_denied'

interface MiniProgramErrorLike {
  errMsg?: string
  message?: string
}

const PROGRESS_PATH = '/pages/merchant/onboarding-v2/index'

const EMPTY_RUNTIME_STATE: MerchantOnboardingV2RuntimeState = {
  lastTrustedViewState: null,
  requestSeq: 0
}

function getMiniProgramErrorMessage(error: unknown): string {
  if (typeof error === 'string') {
    return error
  }
  if (error instanceof Error) {
    return error.message
  }
  if (error && typeof error === 'object') {
    const candidate = error as MiniProgramErrorLike
    return String(candidate.errMsg || candidate.message || '')
  }
  return ''
}

function isPermissionDeniedError(error: unknown): boolean {
  const message = getMiniProgramErrorMessage(error).toLowerCase()
  return message.includes('auth deny') ||
    message.includes('auth denied') ||
    message.includes('authorize no response') ||
    message.includes('scope.writephotosalbum')
}

function isUserCancelledError(error: unknown): boolean {
  return getMiniProgramErrorMessage(error).toLowerCase().includes('cancel')
}

function downloadQrImage(qrUrl: string): Promise<string> {
  return new Promise((resolve, reject) => {
    wx.downloadFile({
      url: qrUrl,
      success: (res) => {
        if (res.statusCode >= 200 && res.statusCode < 300 && res.tempFilePath) {
          resolve(res.tempFilePath)
          return
        }
        reject(new Error('download failed'))
      },
      fail: reject
    })
  })
}

function saveImageToAlbum(filePath: string): Promise<void> {
  return new Promise((resolve, reject) => {
    wx.saveImageToPhotosAlbum({
      filePath,
      success: () => resolve(),
      fail: reject
    })
  })
}

function buildSaveMessage(qrSaveState: QrSaveState): string {
  switch (qrSaveState) {
    case 'downloading':
      return '正在下载二维码...'
    case 'saving':
      return '正在保存到相册...'
    case 'saved':
      return '二维码已保存到相册'
    case 'download_failed':
      return '二维码下载失败，请稍后重试'
    case 'save_failed':
      return '保存失败，请稍后重试'
    case 'permission_denied':
      return '请开启保存到相册权限后重试'
    default:
      return ''
  }
}

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshing: false,
    refreshError: '',
    saving: false,
    qrSaveState: 'idle' as QrSaveState,
    qrSaveMessage: '',
    sourceViewState: null as MerchantOnboardingV2ViewState | null,
    intentViewState: null as MerchantOnboardingV2IntentViewState | null
  },

  _runtimeState: { ...EMPTY_RUNTIME_STATE } as MerchantOnboardingV2RuntimeState,

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    void this.loadIntentState()
  },

  onShow() {
    if (this.data.initialLoading) {
      return
    }
    void this.loadIntentState({ silent: true })
  },

  onPullDownRefresh() {
    void this.loadIntentState({ refreshing: true })
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  async loadIntentState(options: { silent?: boolean, refreshing?: boolean } = {}) {
    const hasTrustedView = !!this.data.intentViewState
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
      const sourceViewState = result.viewState
      if (sourceViewState) {
        this.setData({
          sourceViewState,
          intentViewState: buildMerchantOnboardingV2IntentViewState(sourceViewState),
          initialLoading: false,
          initialError: false,
          initialErrorMessage: '',
          refreshError: result.ok ? '' : result.errorMessage,
          refreshing: false
        })
        return
      }

      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: result.errorMessage || '确认流程加载失败，请稍后重试',
        refreshing: false
      })
    } catch (error: unknown) {
      logger.error('Load merchant onboarding v2 intent failed action=load_intent', error, 'merchant-onboarding-v2-intent')
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: '确认流程加载失败，请稍后重试',
        refreshing: false
      })
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  onRetry() {
    void this.loadIntentState()
  },

  onRefresh() {
    void this.loadIntentState({ refreshing: true })
  },

  onBackToProgress() {
    wx.redirectTo({ url: PROGRESS_PATH })
  },

  onOpenSetting() {
    wx.openSetting()
  },

  async onSaveQr() {
    const intentViewState = this.data.intentViewState
    if (this.data.saving || !intentViewState?.ready || !intentViewState.qrConfigured || !intentViewState.qrUrl) {
      return
    }

    this.setData({
      saving: true,
      qrSaveState: 'downloading',
      qrSaveMessage: buildSaveMessage('downloading')
    })

    try {
      const tempFilePath = await downloadQrImage(intentViewState.qrUrl)
      this.setData({
        qrSaveState: 'saving',
        qrSaveMessage: buildSaveMessage('saving')
      })
      await saveImageToAlbum(tempFilePath)
      this.setData({
        qrSaveState: 'saved',
        qrSaveMessage: buildSaveMessage('saved')
      })
      wx.showToast({ title: '二维码已保存', icon: 'success' })
    } catch (error: unknown) {
      logger.error('Save merchant onboarding v2 intent qr failed action=save_qr', error, 'merchant-onboarding-v2-intent')
      if (isPermissionDeniedError(error)) {
        this.setData({
          qrSaveState: 'permission_denied',
          qrSaveMessage: buildSaveMessage('permission_denied')
        })
        wx.showModal({
          title: '需要相册权限',
          content: '请在设置中开启“保存到相册”权限后重试。',
          confirmText: '去设置',
          success: (result) => {
            if (result.confirm) {
              wx.openSetting()
            }
          }
        })
        return
      }

      if (isUserCancelledError(error)) {
        this.setData({
          qrSaveState: 'idle',
          qrSaveMessage: ''
        })
        return
      }

      const failedState: QrSaveState = this.data.qrSaveState === 'saving' ? 'save_failed' : 'download_failed'
      this.setData({
        qrSaveState: failedState,
        qrSaveMessage: buildSaveMessage(failedState)
      })
      wx.showToast({ title: buildSaveMessage(failedState), icon: 'none' })
    } finally {
      this.setData({ saving: false })
    }
  }
})
