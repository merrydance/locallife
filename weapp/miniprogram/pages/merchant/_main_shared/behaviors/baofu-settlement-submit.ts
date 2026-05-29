/**
 * Shared Behavior for Baofu settlement account submit (profile) pages.
 *
 * Extracts the common page shell logic shared across all 4 role-specific
 * submit pages: draft loading, status refresh polling, workflow result
 * presentation, wait-panel interaction, and back-navigation.
 *
 * Form-specific logic (applyAccount, onSubmit*, onInput, form data) remains
 * in each page because enterprise and personal forms have fundamentally
 * different data shapes and validation rules.
 *
 * Role-specific differences are injected via BaofuSettlementSubmitConfig:
 *   - role:                    which role this page serves
 *   - statusPagePath:          path to the status page (for back navigation)
 *   - getAccount:              API call to fetch account for draft loading
 *   - accessGuard:             optional pre-flight access check (merchant only)
 *   - logTag:                  structured log tag
 *   - loadErrorFallback:       role-specific load failure message
 *   - refreshErrorFallback:    role-specific refresh failure message
 */
import type { BaofuSettlementAccountResponse } from '../api/baofu-account'
import {
  buildBaofuOnboardingWaitView,
  buildBaofuOnboardingWaitViewFromText,
  formatBaofuOnboardingPollProgress,
  pollBaofuSettlementAccountStatus,
  type BaofuOnboardingPollProgress,
  type BaofuOnboardingWaitAction,
  type BaofuOnboardingWaitState,
  type BaofuOnboardingWorkflowResult
} from '../services/baofu-account-onboarding'
import {
  buildBaofuRolePageView,
  type BaofuRolePageView
} from '../services/baofu-account-role-page'
import type { BaofuAccountOwnerRole } from '../api/baofu-account'
import type { AccessCheckResult } from './baofu-settlement-status'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

export interface BaofuSettlementSubmitConfig {
  role: BaofuAccountOwnerRole
  statusPagePath: string
  getAccount: () => Promise<BaofuSettlementAccountResponse>
  accessGuard?: () => Promise<AccessCheckResult>
  logTag: string
  loadErrorFallback: string
  refreshErrorFallback: string
}

export function baofuSettlementSubmitBehavior(config: BaofuSettlementSubmitConfig) {
  const EMPTY_PAGE_VIEW = buildBaofuRolePageView(config.role, null)
  const STATUS_PAGE_ROUTE = config.statusPagePath.replace(/^\//, '')

  function backToStatusPage() {
    const pages = getCurrentPages()
    const previousPage = pages[pages.length - 2]
    if (previousPage?.route === STATUS_PAGE_ROUTE) {
      wx.navigateBack()
      return
    }
    wx.redirectTo({ url: config.statusPagePath })
  }

  function shouldReturnToStatusPage(result: BaofuOnboardingWorkflowResult): boolean {
    return result.status === 'ready' || result.status === 'failed' || result.status === 'voided'
  }

  return Behavior({
    data: {
      navBarHeight: 88,
      _pageActive: true,
      _waitSessionId: 0,
      accessReady: !config.accessGuard,
      accessDenied: false,
      accessDeniedMessage: '',
      accessErrorMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: '',
      submitting: false,
      syncing: false,
      formErrorMessage: '',
      pageView: { ...EMPTY_PAGE_VIEW } as BaofuRolePageView,
      canSubmitProfile: false,
      waitVisible: false,
      waitState: 'submitting' as BaofuOnboardingWaitState,
      waitTheme: 'warning' as 'success' | 'warning' | 'error',
      waitTitle: '',
      waitDescription: '',
      waitProgressText: '',
      waitElapsedSeconds: 0,
      waitRemainingSeconds: 0,
      waitUntilTerminal: true,
      waitTimerVisible: false,
      waitPrimaryAction: 'dismiss' as BaofuOnboardingWaitAction,
      waitPrimaryActionText: ''
    },

    methods: {
      // --- Internal methods ---

      async _bootstrapSubmitPage() {
        if (!config.accessGuard) {
          void this._loadDraft()
          return
        }

        this.setData({
          accessReady: false,
          accessDenied: false,
          accessDeniedMessage: '',
          accessErrorMessage: '',
          initialLoading: true,
          initialError: false,
          initialErrorMessage: '',
          formErrorMessage: '',
          waitVisible: false,
          waitProgressText: '',
          waitElapsedSeconds: 0,
          waitRemainingSeconds: 0,
          waitUntilTerminal: true,
          waitTimerVisible: false
        })

        try {
          const accessResult = await config.accessGuard()
          if (!accessResult.granted) {
            this.setData({
              accessReady: true,
              accessDenied: accessResult.denied,
              accessDeniedMessage: accessResult.deniedMessage,
              accessErrorMessage: accessResult.errorMessage,
              initialLoading: false
            })
            return
          }

          this.setData({
            accessReady: true,
            accessDenied: false,
            accessDeniedMessage: '',
            accessErrorMessage: ''
          })
          await this._loadDraft()
        } catch (error: unknown) {
          logger.error(`Bootstrap baofu submit page failed action=bootstrap role=${config.role}`, error, config.logTag)
          this.setData({
            accessReady: true,
            initialLoading: false,
            initialError: true,
            initialErrorMessage: config.loadErrorFallback
          })
        }
      },

      async _loadDraft() {
        this.setData({
          initialLoading: true,
          initialError: false,
          initialErrorMessage: '',
          formErrorMessage: '',
          waitVisible: false,
          waitProgressText: '',
          waitElapsedSeconds: 0,
          waitRemainingSeconds: 0,
          waitUntilTerminal: true,
          waitTimerVisible: false
        })

        try {
          const response = await config.getAccount()
          // Delegate to page-level applyAccount (form-type specific).
          // The page MUST define applyAccount(); Behavior can't see it at compile time.
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          ;(this as any).applyAccount(response)
        } catch (error: unknown) {
          logger.error(`Load baofu submit draft failed action=load_draft role=${config.role}`, error, config.logTag)
          this.setData({
            initialLoading: false,
            initialError: true,
            initialErrorMessage: getErrorUserMessage(error, config.loadErrorFallback)
          })
        }
      },

      _applyWorkflowResult(result: BaofuOnboardingWorkflowResult) {
        const waitView = buildBaofuOnboardingWaitView(result)
        this.setData({
          waitVisible: true,
          waitState: waitView.state,
          waitTheme: waitView.theme,
          waitTitle: waitView.title,
          waitDescription: waitView.description,
          waitProgressText: '',
          waitElapsedSeconds: 0,
          waitRemainingSeconds: 0,
          waitUntilTerminal: true,
          waitTimerVisible: false,
          waitPrimaryAction: waitView.primaryAction,
          waitPrimaryActionText: waitView.primaryActionText
        })
        if (shouldReturnToStatusPage(result)) {
          backToStatusPage()
        }
      },

      _beginBaofuLongWaitSession(): number {
        const nextSessionId = Number(this.data._waitSessionId || 0) + 1
        this.data._waitSessionId = nextSessionId
        return nextSessionId
      },

      _cancelBaofuLongWaitSession() {
        this.data._waitSessionId = Number(this.data._waitSessionId || 0) + 1
      },

      _shouldStopBaofuLongWait(sessionId: number): boolean {
        return !this.data._pageActive || this.data._waitSessionId !== sessionId
      },

      _handleBaofuOnboardingProgress(progress: BaofuOnboardingPollProgress, sessionId?: number) {
        if (sessionId !== undefined && this._shouldStopBaofuLongWait(sessionId)) {
          return
        }
        this.setData({
          waitProgressText: formatBaofuOnboardingPollProgress(progress),
          waitElapsedSeconds: Math.max(0, Math.round(progress.elapsedSeconds)),
          waitRemainingSeconds: Math.max(0, Math.ceil(progress.remainingSeconds)),
          waitUntilTerminal: progress.maxAttempts === 0,
          waitTimerVisible: true
        })
      },

      // --- Public event handlers bound from WXML ---

      onLoad() {
        const { navBarHeight } = getStableBarHeights()
        this.data._pageActive = true
        this.setData({ navBarHeight })
        if (config.accessGuard) {
          void this._bootstrapSubmitPage()
        } else {
          void this._loadDraft()
        }
      },

      onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
        this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
      },

      onShow() {
        this.data._pageActive = true
      },

      onHide() {
        this.data._pageActive = false
        this._cancelBaofuLongWaitSession()
        this.setData({ syncing: false })
      },

      onUnload() {
        this.data._pageActive = false
        this._cancelBaofuLongWaitSession()
        this.setData({ syncing: false })
      },

      onRetry() {
        if (config.accessGuard && !(this.data.accessReady && !this.data.accessDenied && !this.data.accessErrorMessage)) {
          void this._bootstrapSubmitPage()
          return
        }
        void this._loadDraft()
      },

      onBackToStatus() {
        backToStatusPage()
      },

      async onRefreshStatus() {
        if (this.data.syncing || this.data.submitting) {
          return
        }

        this.setData({
          syncing: true,
          waitVisible: true,
          waitElapsedSeconds: 0,
          waitRemainingSeconds: 0,
          waitUntilTerminal: true,
          waitTimerVisible: true,
          ...buildBaofuOnboardingWaitViewFromText({
            state: 'opening_processing',
            title: '开户状态同步中',
            description: '正在向后端确认最新开户状态。',
            theme: 'warning',
            primaryAction: 'refresh_status',
            primaryActionText: ''
          }),
          waitProgressText: ''
        })

        const sessionId = this._beginBaofuLongWaitSession()
        try {
          const result = await pollBaofuSettlementAccountStatus({
            role: config.role,
            context: this as unknown as WechatMiniprogram.Page.TrivialInstance,
            loadingMessage: '正在刷新开户状态...',
            silentToast: true,
            shouldStop: () => this._shouldStopBaofuLongWait(sessionId),
            onProgress: (progress) => {
              this._handleBaofuOnboardingProgress(progress, sessionId)
            }
          })
          if (this._shouldStopBaofuLongWait(sessionId)) {
            return
          }
          this._applyWorkflowResult(result)
        } catch (error: unknown) {
          logger.error(`Refresh baofu submit status failed action=refresh_status role=${config.role}`, error, config.logTag)
          const message = getErrorUserMessage(error, config.refreshErrorFallback)
          this.setData({
            waitVisible: true,
            ...buildBaofuOnboardingWaitViewFromText({
              state: 'error',
              title: '状态刷新失败',
              description: message,
              theme: 'error',
              primaryAction: 'retry',
              primaryActionText: '重试'
            }),
            waitProgressText: '',
            waitElapsedSeconds: 0,
            waitRemainingSeconds: 0,
            waitUntilTerminal: true,
            waitTimerVisible: false
          })
        } finally {
          if (!this._shouldStopBaofuLongWait(sessionId)) {
            this.setData({ syncing: false })
          }
        }
      },

      onWaitPrimary() {
        switch (this.data.waitPrimaryAction) {
          case 'refresh_status':
          case 'retry':
            void this.onRefreshStatus()
            break
          case 'back_to_status':
            backToStatusPage()
            break
          default:
            this._cancelBaofuLongWaitSession()
            this.setData({
              waitVisible: false,
              waitProgressText: '',
              waitElapsedSeconds: 0,
              waitRemainingSeconds: 0,
              waitUntilTerminal: true,
              waitTimerVisible: false
            })
            break
        }
      }
    }
  })
}
