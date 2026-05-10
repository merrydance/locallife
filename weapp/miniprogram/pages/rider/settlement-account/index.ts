import {
  BaofuAccountProfile,
  BaofuSettlementAccountResponse,
  buildBaofuSettlementAccountView,
  getRiderBaofuSettlementAccount,
  type BaofuSettlementAccountView
} from '../../../api/baofu-account'
import {
    continueBaofuAccountPayment,
    getPendingBaofuAccountOnboardingContext,
    getBaofuOnboardingFeedbackMessage,
    getBaofuOnboardingFeedbackTheme,
    pollBaofuSettlementAccountStatus,
    startBaofuAccountOnboarding,
    clearPendingBaofuAccountOnboardingContext,
    shouldClearPendingBaofuAccountOnboardingContext,
    type BaofuOnboardingWorkflowResult
} from '../../../services/baofu-account-onboarding'
import Toast, { hideToast } from '../../../miniprogram_npm/tdesign-miniprogram/toast/index'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'

type FeedbackTheme = 'success' | 'warning' | 'error'
type FormField = 'real_name' | 'mobile' | 'id_card_number' | 'bank_account_number' | 'bank_name'

interface ProfileForm {
  real_name: string
  mobile: string
  id_card_number: string
  bank_account_number: string
  bank_name: string
}

interface FieldDataset {
  field?: FormField
}

const EMPTY_ACCOUNT_VIEW = buildBaofuSettlementAccountView(null)
const TOAST_SELECTOR = '#t-toast'

let accountRequestPending = false

function emptyForm(): ProfileForm {
  return {
    real_name: '',
    mobile: '',
    id_card_number: '',
    bank_account_number: '',
    bank_name: ''
  }
}

function buildProfilePayload(form: ProfileForm): BaofuAccountProfile {
  return {
    real_name: form.real_name.trim(),
    mobile: form.mobile.trim(),
    id_card_number: form.id_card_number.trim(),
    bank_account_number: form.bank_account_number.trim(),
    bank_name: form.bank_name.trim()
  }
}

function showResultToast(
  context: WechatMiniprogram.Page.TrivialInstance,
  message: string,
  theme: FeedbackTheme = 'warning'
) {
  Toast({
    context,
    selector: TOAST_SELECTOR,
    message,
    theme,
    direction: 'column',
    duration: 1800
  })
}

function hidePageToast(context: WechatMiniprogram.Page.TrivialInstance) {
  hideToast({ context, selector: TOAST_SELECTOR })
}

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    accountLoaded: false,
    refreshing: false,
    submitting: false,
    syncing: false,
    actionFeedbackMessage: '',
    actionFeedbackTheme: 'success' as FeedbackTheme,
    accountView: { ...EMPTY_ACCOUNT_VIEW } as BaofuSettlementAccountView,
    form: emptyForm(),
    formErrorMessage: '',
    canEditProfile: false,
    canContinuePayment: false,
    canRefreshStatus: true
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    void this.loadAccount({ force: true })
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onShow() {
    if (!this.data.accountLoaded || this.data.initialLoading || this.data.submitting || this.data.syncing) {
      return
    }
    void this.loadAccount({ silent: true })
    void this.recoverPendingOnboarding()
  },

  onPullDownRefresh() {
    void this.loadAccount({ force: true, refreshing: true })
  },

  applyAccount(response: BaofuSettlementAccountResponse) {
    const accountView = buildBaofuSettlementAccountView(response)
    this.setData({
      accountView,
      canEditProfile: accountView.canSubmitProfile,
      canContinuePayment: accountView.canStartPayment,
      canRefreshStatus: accountView.canRefresh,
      initialLoading: false,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: '',
      accountLoaded: true
    })
  },

  async loadAccount(options: { force?: boolean, silent?: boolean, refreshing?: boolean } = {}) {
    if (accountRequestPending && !options.force) {
      wx.stopPullDownRefresh()
      return
    }

    accountRequestPending = true
    const hasTrustedData = this.data.accountLoaded
    if (!options.silent) {
      this.setData(hasTrustedData
        ? { refreshing: true, refreshErrorMessage: '' }
        : { initialLoading: true, initialError: false, initialErrorMessage: '', refreshErrorMessage: '' })
    }

    try {
      const response = await getRiderBaofuSettlementAccount()
      this.applyAccount(response)
    } catch (error: unknown) {
      logger.error('Load rider baofu settlement account failed action=load_account role=rider', error, 'rider-settlement-account')
      const message = getErrorUserMessage(error, '结算账户加载失败，请稍后重试')
      if (hasTrustedData) {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          accountView: { ...EMPTY_ACCOUNT_VIEW },
          canEditProfile: false,
          canContinuePayment: false
        })
      }
    } finally {
      accountRequestPending = false
      this.setData({ refreshing: false })
      wx.stopPullDownRefresh()
    }
  },

  async recoverPendingOnboarding() {
    try {
      const pendingContext = getPendingBaofuAccountOnboardingContext('rider')
      if (!pendingContext) {
        return
      }

      if (this.data.submitting || this.data.syncing) {
        return
      }

      this.setData({ syncing: true, actionFeedbackMessage: '' })
      const result = await continueBaofuAccountPayment({
        role: 'rider',
        context: this,
        loadingMessage: '正在恢复开户进度...'
      })
      this.applyWorkflowResult(result)
      if (shouldClearPendingBaofuAccountOnboardingContext(result)) {
        clearPendingBaofuAccountOnboardingContext('rider')
      }
    } catch (error: unknown) {
      logger.error('Recover rider baofu onboarding failed action=recover_pending role=rider', error, 'rider-settlement-account')
      this.setData({ refreshErrorMessage: '开户进度恢复失败，请稍后刷新。' })
    } finally {
      hidePageToast(this)
      this.setData({ syncing: false })
    }
  },

  onRetry() {
    void this.loadAccount({ force: true })
  },

  onInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as FieldDataset
    if (!field) {
      return
    }

    this.setData({
      [`form.${field}`]: e.detail.value,
      formErrorMessage: '',
      actionFeedbackMessage: ''
    })
  },

  validateForm() {
    const form = this.data.form as ProfileForm
    if (!form.real_name.trim()) {
      return '请输入姓名'
    }
    if (!/^1\d{10}$/.test(form.mobile.trim())) {
      return '请输入 11 位手机号'
    }
    if (!/(^\d{15}$)|(^\d{17}[\dXx]$)/.test(form.id_card_number.trim())) {
      return '请输入正确身份证号'
    }
    if (!/^\d{8,30}$/.test(form.bank_account_number.trim())) {
      return '请输入正确银行卡号'
    }
    return ''
  },

  applyWorkflowResult(result: BaofuOnboardingWorkflowResult) {
    this.applyAccount(result.account)
    const message = getBaofuOnboardingFeedbackMessage(result)
    const theme: FeedbackTheme = getBaofuOnboardingFeedbackTheme(result)
    this.setData({
      actionFeedbackMessage: message,
      actionFeedbackTheme: theme
    })
    showResultToast(this, message, theme)
  },

  async onSubmitProfile() {
    if (this.data.submitting || !this.data.canEditProfile) {
      return
    }

    const formErrorMessage = this.validateForm()
    if (formErrorMessage) {
      this.setData({ formErrorMessage })
      showResultToast(this, formErrorMessage)
      return
    }

    this.setData({ submitting: true, formErrorMessage: '', actionFeedbackMessage: '' })
    try {
      const result = await startBaofuAccountOnboarding(buildProfilePayload(this.data.form as ProfileForm), {
        role: 'rider',
        context: this,
        loadingMessage: '正在提交开户资料...'
      })
      this.applyWorkflowResult(result)
    } catch (error: unknown) {
      logger.error('Submit rider baofu settlement profile failed action=submit_profile role=rider', error, 'rider-settlement-account')
      const message = getErrorUserMessage(error, '开户资料提交失败，请稍后重试')
      this.setData({ actionFeedbackMessage: message, actionFeedbackTheme: 'error' })
      showResultToast(this, message, 'error')
    } finally {
      hidePageToast(this)
      this.setData({ submitting: false })
    }
  },

  async onContinuePayment() {
    if (this.data.syncing || this.data.submitting || !this.data.canContinuePayment) {
      return
    }

    this.setData({ syncing: true, actionFeedbackMessage: '' })
    try {
      const result = await continueBaofuAccountPayment({
        role: 'rider',
        context: this,
        loadingMessage: '正在核对支付结果...'
      })
      this.applyWorkflowResult(result)
    } catch (error: unknown) {
      logger.error('Continue rider baofu settlement payment failed action=continue_payment role=rider', error, 'rider-settlement-account')
      const message = getErrorUserMessage(error, '支付进度恢复失败，请稍后重试')
      this.setData({ actionFeedbackMessage: message, actionFeedbackTheme: 'error' })
      showResultToast(this, message, 'error')
    } finally {
      hidePageToast(this)
      this.setData({ syncing: false })
    }
  },

  async onRefreshStatus() {
    if (this.data.syncing || this.data.submitting) {
      return
    }

    this.setData({ syncing: true, actionFeedbackMessage: '' })
    try {
      const result = await pollBaofuSettlementAccountStatus({
        role: 'rider',
        context: this,
        maxAttempts: 1,
        loadingMessage: '正在刷新开户状态...'
      })
      this.applyWorkflowResult(result)
    } catch (error: unknown) {
      logger.error('Refresh rider baofu settlement status failed action=refresh_status role=rider', error, 'rider-settlement-account')
      const message = getErrorUserMessage(error, '开户状态刷新失败，请稍后重试')
      this.setData({ refreshErrorMessage: message })
      showResultToast(this, message, 'error')
    } finally {
      hidePageToast(this)
      this.setData({ syncing: false })
    }
  }
})
