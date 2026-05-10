import {
  type BaofuAccountProfile,
  type BaofuSettlementAccountResponse,
  getMerchantBaofuSettlementAccount
} from '../../../../api/baofu-account'
import {
  getBaofuOnboardingFeedbackMessage,
  getBaofuOnboardingFeedbackTheme,
  pollBaofuSettlementAccountStatus,
  startBaofuAccountOnboarding,
  type BaofuOnboardingWorkflowResult
} from '../../../../services/baofu-account-onboarding'
import {
  buildBaofuRolePageView,
  type BaofuRolePageView
} from '../../../../services/baofu-account-role-page'
import Toast, { hideToast } from '../../../../miniprogram_npm/tdesign-miniprogram/toast/index'
import {
  ensureMerchantApplymentAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../../utils/console-access'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type FeedbackTheme = 'success' | 'warning' | 'error'
type MerchantFormField =
  | 'legal_name'
  | 'business_license_number'
  | 'legal_person_name'
  | 'legal_person_id_number'
  | 'email'
  | 'bank_account_no'
  | 'bank_name'
  | 'deposit_bank_province'
  | 'deposit_bank_city'
  | 'deposit_bank_name'
  | 'contact_name'
  | 'contact_mobile'

interface MerchantProfileForm {
  legal_name: string
  business_license_number: string
  legal_person_name: string
  legal_person_id_number: string
  email: string
  bank_account_no: string
  bank_name: string
  deposit_bank_province: string
  deposit_bank_city: string
  deposit_bank_name: string
  contact_name: string
  contact_mobile: string
}

interface FieldDataset {
  field?: MerchantFormField
}

const TOAST_SELECTOR = '#t-toast'
const EMPTY_PAGE_VIEW = buildBaofuRolePageView('merchant', null)

let accountRequestPending = false

function emptyForm(): MerchantProfileForm {
  return {
    legal_name: '',
    business_license_number: '',
    legal_person_name: '',
    legal_person_id_number: '',
    email: '',
    bank_account_no: '',
    bank_name: '',
    deposit_bank_province: '',
    deposit_bank_city: '',
    deposit_bank_name: '',
    contact_name: '',
    contact_mobile: ''
  }
}

function buildProfilePayload(form: MerchantProfileForm): BaofuAccountProfile {
  return {
    legal_name: form.legal_name.trim(),
    business_license_number: form.business_license_number.trim(),
    legal_person_name: form.legal_person_name.trim(),
    legal_person_id_number: form.legal_person_id_number.trim(),
    email: form.email.trim(),
    bank_account_no: form.bank_account_no.trim(),
    bank_name: form.bank_name.trim(),
    deposit_bank_province: form.deposit_bank_province.trim(),
    deposit_bank_city: form.deposit_bank_city.trim(),
    deposit_bank_name: form.deposit_bank_name.trim(),
    contact_name: form.contact_name.trim(),
    contact_mobile: form.contact_mobile.trim()
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

function isRequiredBlank(value: string): boolean {
  return !String(value || '').trim()
}

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessDeniedMessage: '',
    accessErrorMessage: '',
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
    pageView: { ...EMPTY_PAGE_VIEW } as BaofuRolePageView,
    form: emptyForm(),
    formErrorMessage: '',
    canEditProfile: false,
    canRefreshStatus: true
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.bootstrapPage()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onShow() {
    if (!this.hasAccess() || !this.data.accountLoaded || this.data.initialLoading || this.data.submitting || this.data.syncing) {
      return
    }
    void this.loadAccount({ silent: true })
  },

  onPullDownRefresh() {
    if (!this.hasAccess()) {
      wx.stopPullDownRefresh()
      return
    }
    void this.loadAccount({ force: true, refreshing: true })
  },

  hasAccess() {
    return this.data.accessReady && !this.data.accessDenied && !this.data.accessErrorMessage
  },

  async bootstrapPage() {
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
      canEditProfile: false
    })

    try {
      const accessResult = await ensureMerchantApplymentAccess()
      if (!isMerchantConsoleAccessGranted(accessResult)) {
        if (accessResult.status === 'error') {
          logger.error('Merchant baofu settlement access check failed action=check_access role=merchant', accessResult.message, 'merchant-baofu-settlement-account')
        }
        this.setData({
          accessReady: true,
          accessDenied: isMerchantConsoleAccessDenied(accessResult),
          accessDeniedMessage: accessResult.status === 'denied' ? accessResult.message : '',
          accessErrorMessage: getMerchantConsoleAccessErrorMessage(accessResult),
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
      await this.loadAccount({ force: true })
    } catch (error: unknown) {
      logger.error('Bootstrap merchant baofu settlement account failed action=bootstrap role=merchant', error, 'merchant-baofu-settlement-account')
      this.setData({
        accessReady: true,
        accessDenied: false,
        accessDeniedMessage: '',
        accessErrorMessage: '',
        initialLoading: false,
        initialError: true,
        initialErrorMessage: '商户宝付开户状态加载失败，请稍后重试'
      })
    }
  },

  applyAccount(response: BaofuSettlementAccountResponse) {
    const pageView = buildBaofuRolePageView('merchant', response)
    this.setData({
      pageView,
      canEditProfile: pageView.shouldShowProfileAction,
      canRefreshStatus: pageView.shouldShowRefreshAction,
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
      const response = await getMerchantBaofuSettlementAccount()
      this.applyAccount(response)
    } catch (error: unknown) {
      logger.error('Load merchant baofu settlement account failed action=load_account role=merchant', error, 'merchant-baofu-settlement-account')
      const message = getErrorUserMessage(error, '商户宝付开户状态加载失败，请稍后重试')
      if (hasTrustedData) {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          pageView: { ...EMPTY_PAGE_VIEW },
          canEditProfile: false
        })
      }
    } finally {
      accountRequestPending = false
      this.setData({ refreshing: false })
      wx.stopPullDownRefresh()
    }
  },

  onRetry() {
    if (!this.hasAccess()) {
      void this.bootstrapPage()
      return
    }
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
    const form = this.data.form as MerchantProfileForm
    if (isRequiredBlank(form.legal_name)) return '请输入商户主体名称'
    if (isRequiredBlank(form.business_license_number)) return '请输入营业执照号'
    if (isRequiredBlank(form.legal_person_name)) return '请输入法人姓名'
    if (!/(^\d{15}$)|(^\d{17}[\dXx]$)/.test(form.legal_person_id_number.trim())) return '请输入正确法人身份证号'
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email.trim())) return '请输入正确联系邮箱'
    if (!/^\d{8,30}$/.test(form.bank_account_no.trim())) return '请输入正确对公账号'
    if (isRequiredBlank(form.bank_name)) return '请输入开户银行'
    if (isRequiredBlank(form.deposit_bank_province)) return '请输入开户地址省份'
    if (isRequiredBlank(form.deposit_bank_city)) return '请输入开户地址城市'
    if (isRequiredBlank(form.deposit_bank_name)) return '请输入开户支行'
    if (form.contact_mobile.trim() && !/^1\d{10}$/.test(form.contact_mobile.trim())) return '请输入正确联系人手机号'
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
      const result = await startBaofuAccountOnboarding(buildProfilePayload(this.data.form as MerchantProfileForm), {
        role: 'merchant',
        context: this,
        loadingMessage: '正在提交开户资料...'
      })
      this.applyWorkflowResult(result)
    } catch (error: unknown) {
      logger.error('Submit merchant baofu settlement profile failed action=submit_profile role=merchant', error, 'merchant-baofu-settlement-account')
      const message = getErrorUserMessage(error, '商户宝付开户资料提交失败，请稍后重试')
      this.setData({ actionFeedbackMessage: message, actionFeedbackTheme: 'error' })
      showResultToast(this, message, 'error')
    } finally {
      hidePageToast(this)
      this.setData({ submitting: false })
    }
  },

  async onRefreshStatus() {
    if (this.data.syncing || this.data.submitting) {
      return
    }

    this.setData({ syncing: true, actionFeedbackMessage: '' })
    try {
      const result = await pollBaofuSettlementAccountStatus({
        role: 'merchant',
        context: this,
        maxAttempts: 1,
        loadingMessage: '正在刷新开户状态...'
      })
      this.applyWorkflowResult(result)
    } catch (error: unknown) {
      logger.error('Refresh merchant baofu settlement status failed action=refresh_status role=merchant', error, 'merchant-baofu-settlement-account')
      const message = getErrorUserMessage(error, '商户宝付开户状态刷新失败，请稍后重试')
      this.setData({ refreshErrorMessage: message })
      showResultToast(this, message, 'error')
    } finally {
      hidePageToast(this)
      this.setData({ syncing: false })
    }
  }
})
