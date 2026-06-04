/**
 * Shared Behavior for Baofu settlement account status pages.
 *
 * Extracts the common lifecycle, data loading, status polling, workflow result
 * handling, and payment recovery logic shared across all 4 role-specific
 * settlement account status pages (merchant, operator, platform, rider).
 *
 * Role-specific differences are injected via BaofuSettlementStatusConfig:
 *   - role:                    which role this page serves
 *   - submitPagePath:          path to the submit/profile page
 *   - getAccount:              API call to fetch account status
 *   - supportPaymentRecovery:  whether the page supports continue_payment action
 *                              and pending onboarding recovery (operator/rider)
 *   - accessGuard:             optional pre-flight access check (merchant only)
 *   - logTag:                  structured log tag
 *   - loadErrorFallback:       role-specific load failure message
 *   - refreshErrorFallback:    role-specific refresh failure message
 */
import type {
  BaofuSettlementAccountPageAction,
  BaofuSettlementAccountResponse
} from '../api/baofu-account'
import {
  buildBaofuOnboardingWaitView,
  buildBaofuOnboardingWaitDataPatch,
  buildBaofuOnboardingWaitViewFromAccount,
  buildBaofuOnboardingWaitViewFromText,
  clearPendingBaofuAccountOnboardingContext,
  continueBaofuAccountPayment,
  formatBaofuOnboardingPollProgress,
  getPendingBaofuAccountOnboardingContext,
  pollBaofuSettlementAccountStatus,
  shouldClearPendingBaofuAccountOnboardingContext,
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
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

export interface AccessCheckResult {
  granted: boolean
  denied: boolean
  deniedMessage: string
  errorMessage: string
}

export interface BaofuSettlementStatusConfig {
  role: BaofuAccountOwnerRole
  submitPagePath: string
  disableSubmitProfile?: boolean
  getAccount: () => Promise<BaofuSettlementAccountResponse>
  supportPaymentRecovery?: boolean
  accessGuard?: () => Promise<AccessCheckResult>
  logTag: string
  loadErrorFallback: string
  refreshErrorFallback: string
}

export function baofuSettlementStatusBehavior(config: BaofuSettlementStatusConfig) {
  const EMPTY_PAGE_VIEW = buildBaofuRolePageView(config.role, null)

  return Behavior({
    data: {
      navBarHeight: 88,
      _pageActive: true,
      _accountRequestPending: false,
      _submitRedirectPending: false,
      _waitSessionId: 0,
      accessReady: !config.accessGuard,
      accessDenied: false,
      accessDeniedMessage: '',
      accessErrorMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: '',
      accountLoaded: false,
      refreshing: false,
      syncing: false,
      pageView: { ...EMPTY_PAGE_VIEW } as BaofuRolePageView,
      waitVisible: false,
      waitState: 'opening_processing' as BaofuOnboardingWaitState,
      waitTheme: 'warning' as 'success' | 'warning' | 'error',
      waitTitle: '',
      waitDescription: '',
      waitProgressText: '',
      waitElapsedSeconds: 0,
      waitRemainingSeconds: 0,
      waitUntilTerminal: true,
      waitTimerVisible: false,
      waitPrimaryAction: 'refresh_status' as BaofuOnboardingWaitAction,
      waitPrimaryActionText: ''
    },

    methods: {
      _hasAccess(): boolean {
        if (!config.accessGuard) {
          return true
        }
        return this.data.accessReady && !this.data.accessDenied && !this.data.accessErrorMessage
      },

      async _bootstrapPage() {
        if (!config.accessGuard) {
          void this._loadAccount({ force: true })
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
          refreshErrorMessage: '',
          accountLoaded: false,
          pageView: { ...EMPTY_PAGE_VIEW },
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
          await this._loadAccount({ force: true })
        } catch (error: unknown) {
          logger.error(`Bootstrap baofu settlement account failed action=bootstrap role=${config.role}`, error, config.logTag)
          this.setData({
            accessReady: true,
            accessDenied: false,
            accessDeniedMessage: '',
            accessErrorMessage: '',
            initialLoading: false,
            initialError: true,
            initialErrorMessage: config.loadErrorFallback
          })
        }
      },

      _enterSubmitPageDirectly(): Promise<boolean> {
        if (this.data._submitRedirectPending) {
          return Promise.resolve(true)
        }

        this.data._submitRedirectPending = true
        return new Promise<boolean>((resolve) => {
          let redirected = false
          wx.redirectTo({
            url: config.submitPagePath,
            success: () => {
              redirected = true
            },
            fail: (error) => {
              logger.error(`Redirect baofu settlement submit failed action=enter_submit role=${config.role}`, error, config.logTag)
            },
            complete: () => {
              this.data._submitRedirectPending = false
              resolve(redirected)
            }
          })
        })
      },

      async _presentAccount(response: BaofuSettlementAccountResponse): Promise<boolean> {
        const pageView = buildBaofuRolePageView(config.role, response)
        if (pageView.shouldEnterSubmitDirectly && !config.disableSubmitProfile) {
          const redirected = await this._enterSubmitPageDirectly()
          if (redirected) {
            return false
          }
        }

        this._applyAccount(response, pageView)
        return true
      },

      _applyAccount(response: BaofuSettlementAccountResponse, pageView = buildBaofuRolePageView(config.role, response)) {
        const shouldLongWait = pageView.statusView.isWaiting
        if (!shouldLongWait) {
          this._cancelBaofuLongWaitSession()
        }
        this.setData({
          pageView,
          initialLoading: false,
          initialError: false,
          initialErrorMessage: '',
          refreshErrorMessage: '',
          accountLoaded: true,
          waitVisible: shouldLongWait ? this.data.waitVisible : false,
          waitProgressText: shouldLongWait ? this.data.waitProgressText : '',
          waitElapsedSeconds: shouldLongWait ? this.data.waitElapsedSeconds : 0,
          waitRemainingSeconds: shouldLongWait ? this.data.waitRemainingSeconds : 0,
          waitUntilTerminal: true,
          waitTimerVisible: shouldLongWait ? this.data.waitTimerVisible : false,
          syncing: shouldLongWait ? this.data.syncing : false
        })
        if (shouldLongWait && !this.data.syncing) {
          void this._startLongWaitForProcessing()
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
        const waitView = progress.account ? buildBaofuOnboardingWaitViewFromAccount(progress.account) : null
        this.setData({
          ...(waitView ? {
            waitState: waitView.state,
            waitTheme: waitView.theme,
            waitTitle: waitView.title,
            waitDescription: waitView.description,
            waitPrimaryAction: waitView.primaryAction,
            waitPrimaryActionText: waitView.primaryActionText
          } : {}),
          waitProgressText: formatBaofuOnboardingPollProgress(progress),
          waitElapsedSeconds: Math.max(0, Math.round(progress.elapsedSeconds)),
          waitRemainingSeconds: Math.max(0, Math.ceil(progress.remainingSeconds)),
          waitUntilTerminal: progress.maxAttempts === 0,
          waitTimerVisible: true
        })
      },

      async _startLongWaitForProcessing() {
        if (this.data.syncing) {
          return
        }

        const sessionId = this._beginBaofuLongWaitSession()
        this.setData({
          syncing: true,
          refreshErrorMessage: '',
          waitVisible: true,
          ...buildBaofuOnboardingWaitDataPatch(buildBaofuOnboardingWaitViewFromText({
            state: 'opening_processing',
            title: '开户状态同步中',
            description: '正在向后端确认开户、支付报备和授权目录状态。',
            theme: 'warning',
            primaryAction: 'refresh_status',
            primaryActionText: ''
          })),
          waitProgressText: '',
          waitElapsedSeconds: 0,
          waitRemainingSeconds: 0,
          waitUntilTerminal: true,
          waitTimerVisible: true
        })
        try {
          const result = await pollBaofuSettlementAccountStatus({
            role: config.role,
            context: this as unknown as WechatMiniprogram.Page.TrivialInstance,
            loadingMessage: '开户状态同步中...',
            silentToast: true,
            shouldStop: () => this._shouldStopBaofuLongWait(sessionId),
            onProgress: (progress) => {
              this._handleBaofuOnboardingProgress(progress, sessionId)
            }
          })
          if (this._shouldStopBaofuLongWait(sessionId)) {
            return
          }
          await this._applyWorkflowResult(result)
        } catch (error: unknown) {
          if (this._shouldStopBaofuLongWait(sessionId)) {
            return
          }
          logger.error(`Long wait baofu settlement status failed action=long_wait role=${config.role}`, error, config.logTag)
          const message = getErrorUserMessage(error, config.refreshErrorFallback)
          this.setData({
            refreshErrorMessage: message,
            waitVisible: true,
            ...buildBaofuOnboardingWaitDataPatch(buildBaofuOnboardingWaitViewFromText({
              state: 'error',
              title: '状态同步失败',
              description: message,
              theme: 'error',
              primaryAction: 'retry',
              primaryActionText: '重试'
            })),
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

      async _loadAccount(options: { force?: boolean, silent?: boolean, refreshing?: boolean } = {}) {
        if (this.data._accountRequestPending && !options.force) {
          wx.stopPullDownRefresh()
          return
        }

        this.data._accountRequestPending = true
        const hasTrustedData = this.data.accountLoaded
        if (!options.silent) {
          this.setData(hasTrustedData
            ? { refreshing: true, refreshErrorMessage: '' }
            : { initialLoading: true, initialError: false, initialErrorMessage: '', refreshErrorMessage: '' })
        }

        try {
          const response = await config.getAccount()
          await this._presentAccount(response)
        } catch (error: unknown) {
          logger.error(`Load baofu settlement account failed action=load_account role=${config.role}`, error, config.logTag)
          const message = getErrorUserMessage(error, config.loadErrorFallback)
          if (hasTrustedData) {
            this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
          } else {
            this.setData({
              initialLoading: false,
              initialError: true,
              initialErrorMessage: message,
              pageView: { ...EMPTY_PAGE_VIEW }
            })
          }
        } finally {
          this.data._accountRequestPending = false
          this.setData({ refreshing: false })
          wx.stopPullDownRefresh()
        }
      },

      async _recoverPendingOnboarding() {
        if (!config.supportPaymentRecovery) {
          return
        }

        let didSetSyncing = false
        let sessionId = 0
        try {
          const pendingContext = getPendingBaofuAccountOnboardingContext(config.role)
          if (!pendingContext || this.data.syncing) {
            return
          }

          this.setData({
            syncing: true,
            waitVisible: true,
            waitElapsedSeconds: 0,
            waitRemainingSeconds: 0,
            waitUntilTerminal: true,
            waitTimerVisible: true,
            ...buildBaofuOnboardingWaitDataPatch(buildBaofuOnboardingWaitViewFromText({
              state: 'payment_confirming',
              title: '开户进度恢复中',
              description: '正在向后端确认支付和开户状态。',
              theme: 'warning',
              primaryAction: 'refresh_status',
              primaryActionText: ''
            })),
            waitProgressText: ''
          })
          didSetSyncing = true
          sessionId = this._beginBaofuLongWaitSession()
          const result = await continueBaofuAccountPayment({
            role: config.role,
            context: this as unknown as WechatMiniprogram.Page.TrivialInstance,
            loadingMessage: '正在恢复开户进度...',
            silentToast: true,
            shouldStop: () => this._shouldStopBaofuLongWait(sessionId),
            onProgress: (progress) => {
              this._handleBaofuOnboardingProgress(progress, sessionId)
            }
          })
          if (this._shouldStopBaofuLongWait(sessionId)) {
            return
          }
          await this._applyWorkflowResult(result)
          if (shouldClearPendingBaofuAccountOnboardingContext(result)) {
            clearPendingBaofuAccountOnboardingContext(config.role)
          }
        } catch (error: unknown) {
          if (!this.data._pageActive || (sessionId > 0 && this._shouldStopBaofuLongWait(sessionId))) {
            return
          }
          logger.error(`Recover baofu onboarding failed action=recover_pending role=${config.role}`, error, config.logTag)
          this.setData({
            refreshErrorMessage: '开户进度恢复失败，请稍后重试。',
            waitVisible: true,
            ...buildBaofuOnboardingWaitDataPatch(buildBaofuOnboardingWaitViewFromText({
              state: 'error',
              title: '恢复失败',
              description: '开户进度恢复失败，请稍后重试。',
              theme: 'error',
              primaryAction: 'retry',
              primaryActionText: '重试'
            })),
            waitProgressText: '',
            waitElapsedSeconds: 0,
            waitRemainingSeconds: 0,
            waitUntilTerminal: true,
            waitTimerVisible: false
          })
        } finally {
          if (didSetSyncing && sessionId > 0) {
            if (!this._shouldStopBaofuLongWait(sessionId)) {
              this.setData({ syncing: false })
            }
          } else if (didSetSyncing && this.data._pageActive) {
            this.setData({ syncing: false })
          }
        }
      },

      async _applyWorkflowResult(result: BaofuOnboardingWorkflowResult) {
        const presented = await this._presentAccount(result.account)
        if (!presented) {
          return
        }
        const waitView = buildBaofuOnboardingWaitView(result)
        const shouldKeepWaitVisible = result.status === 'processing' ||
          result.status === 'pending_confirmation' ||
          result.status === 'pay_params_missing'
        this.setData({
          waitVisible: shouldKeepWaitVisible,
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
      },

      async _onContinuePayment() {
        if (!config.supportPaymentRecovery || this.data.syncing) {
          return
        }

        this.setData({
          syncing: true,
          refreshErrorMessage: '',
          waitVisible: true,
          waitElapsedSeconds: 0,
          waitRemainingSeconds: 0,
          waitUntilTerminal: true,
          waitTimerVisible: true,
          ...buildBaofuOnboardingWaitDataPatch(buildBaofuOnboardingWaitViewFromText({
            state: 'payment_confirming',
            title: '支付结果确认中',
            description: '正在确认核验费支付和开户状态。',
            theme: 'warning',
            primaryAction: 'refresh_status',
            primaryActionText: ''
          })),
          waitProgressText: ''
        })
        const sessionId = this._beginBaofuLongWaitSession()
        try {
          const result = await continueBaofuAccountPayment({
            role: config.role,
            context: this as unknown as WechatMiniprogram.Page.TrivialInstance,
            loadingMessage: '正在核对支付结果...',
            silentToast: true,
            shouldStop: () => this._shouldStopBaofuLongWait(sessionId),
            onProgress: (progress) => {
              this._handleBaofuOnboardingProgress(progress, sessionId)
            }
          })
          if (this._shouldStopBaofuLongWait(sessionId)) {
            return
          }
          await this._applyWorkflowResult(result)
        } catch (error: unknown) {
          if (this._shouldStopBaofuLongWait(sessionId)) {
            return
          }
          logger.error(`Continue baofu settlement payment failed action=continue_payment role=${config.role}`, error, config.logTag)
          const message = getErrorUserMessage(error, '支付进度恢复失败，请稍后重试')
          this.setData({
            waitVisible: true,
            ...buildBaofuOnboardingWaitDataPatch(buildBaofuOnboardingWaitViewFromText({
              state: 'error',
              title: '支付进度恢复失败',
              description: message,
              theme: 'error',
              primaryAction: 'retry',
              primaryActionText: '重试'
            })),
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

      // --- Public event handlers bound from WXML ---

      onLoad() {
        const { navBarHeight } = getStableBarHeights()
        this.data._pageActive = true
        this.setData({ navBarHeight })
        if (config.accessGuard) {
          void this._bootstrapPage()
        } else {
          void this._loadAccount({ force: true })
        }
      },

      onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
        this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
      },

      onShow() {
        this.data._pageActive = true
        if (!this._hasAccess() || !this.data.accountLoaded || this.data.initialLoading || this.data.syncing) {
          return
        }
        void this._loadAccount({ silent: true })
        if (config.supportPaymentRecovery) {
          void this._recoverPendingOnboarding()
        }
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

      onPullDownRefresh() {
        if (!this._hasAccess()) {
          wx.stopPullDownRefresh()
          return
        }
        void this._loadAccount({ force: true, refreshing: true })
      },

      onRetry() {
        if (config.accessGuard && !this._hasAccess()) {
          void this._bootstrapPage()
          return
        }
        void this._loadAccount({ force: true })
      },

      onPrimaryAction() {
        this.handleAction(this.data.pageView.primaryAction as BaofuSettlementAccountPageAction)
      },

      handleAction(action: BaofuSettlementAccountPageAction) {
        switch (action.type) {
          case 'submit_profile':
            if (!config.disableSubmitProfile && config.submitPagePath) {
              wx.navigateTo({ url: config.submitPagePath })
            }
            break
          case 'continue_payment':
            if (config.supportPaymentRecovery) {
              void this._onContinuePayment()
            }
            break
          case 'refresh_status':
            void this.onRefreshStatus()
            break
          default:
            break
        }
      },

      async onRefreshStatus() {
        if (this.data.syncing) {
          return
        }

        await this._startLongWaitForProcessing()
      },

      onWaitPrimary() {
        if (this.data.syncing) {
          return
        }
        switch (this.data.waitPrimaryAction) {
          case 'refresh_status':
          case 'retry':
            void this.onRefreshStatus()
            break
          case 'dismiss':
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
