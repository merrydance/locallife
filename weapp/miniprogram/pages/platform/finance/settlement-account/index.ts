import type {
  BaofuAccountProfile,
  BaofuSettlementAccountProfileDefaults,
  BaofuSettlementAccountResponse
} from '../../../../api/baofu-account'
import { getPlatformBaofuSettlementAccount } from '../../../../api/baofu-account'
import type {
  ApplymentBankFormDraftPayload,
  ApplymentBankFormPayload
} from '../../../../components/applyment-bank-form/index'
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
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type FeedbackTheme = 'success' | 'warning' | 'error'
type PlatformFormField =
  | 'legal_name'
  | 'business_license_number'
  | 'legal_person_name'
  | 'legal_person_id_number'
  | 'corporate_mobile'
  | 'email'
  | 'contact_name'
  | 'contact_mobile'

interface PlatformProfileForm {
  legal_name: string
  business_license_number: string
  legal_person_name: string
  legal_person_id_number: string
  corporate_mobile: string
  email: string
  contact_name: string
  contact_mobile: string
}

interface FieldDataset {
  field?: PlatformFormField
}

const TOAST_SELECTOR = '#t-toast'
const EMPTY_PAGE_VIEW = buildBaofuRolePageView('platform', null)

let accountRequestPending = false

function emptyForm(): PlatformProfileForm {
  return {
    legal_name: '',
    business_license_number: '',
    legal_person_name: '',
    legal_person_id_number: '',
    corporate_mobile: '',
    email: '',
    contact_name: '',
    contact_mobile: ''
  }
}

function normalizeText(value?: string | null): string {
  return typeof value === 'string' ? value.trim() : ''
}

function buildBankDraftFromDefaults(defaults?: BaofuSettlementAccountProfileDefaults | null): ApplymentBankFormDraftPayload | null {
  const fallbackAccountType = 'ACCOUNT_TYPE_BUSINESS' as const
  if (!defaults) {
    return { account_type: fallbackAccountType }
  }
  const selfEmployed = Boolean(defaults.self_employed)
  return {
    account_type: selfEmployed ? 'ACCOUNT_TYPE_PRIVATE' : 'ACCOUNT_TYPE_BUSINESS',
    account_bank: normalizeText(defaults.account_bank || defaults.bank_name),
    account_bank_code: defaults.account_bank_code || 0,
    bank_alias: normalizeText(defaults.bank_alias || defaults.bank_name),
    bank_alias_code: normalizeText(defaults.bank_alias_code),
    need_bank_branch: Boolean(defaults.bank_branch_id || defaults.bank_address_code || defaults.deposit_bank_name),
    bank_address_code: normalizeText(defaults.bank_address_code),
    bank_branch_id: normalizeText(defaults.bank_branch_id),
    bank_name: normalizeText(defaults.deposit_bank_name),
    account_number: '',
    account_name: normalizeText(selfEmployed ? (defaults.card_user_name || defaults.legal_person_name) : defaults.legal_name),
    contact_email: ''
  }
}

function hasStoredLegalPersonID(defaults?: BaofuSettlementAccountProfileDefaults | null): boolean {
  return Boolean(defaults?.has_legal_person_id_number)
}

function hasStoredEmail(defaults?: BaofuSettlementAccountProfileDefaults | null): boolean {
  return Boolean(defaults?.has_email)
}

function hasStoredCorporateMobile(defaults?: BaofuSettlementAccountProfileDefaults | null): boolean {
  return Boolean(defaults?.has_corporate_mobile)
}

function hasStoredBankAccount(defaults?: BaofuSettlementAccountProfileDefaults | null): boolean {
  return Boolean(defaults?.has_bank_account_no)
}

function hasStoredContactMobile(defaults?: BaofuSettlementAccountProfileDefaults | null): boolean {
  return Boolean(defaults?.has_contact_mobile)
}

function buildFormFromDefaults(defaults?: BaofuSettlementAccountProfileDefaults | null): PlatformProfileForm {
  return {
    legal_name: normalizeText(defaults?.legal_name),
    business_license_number: normalizeText(defaults?.business_license_number),
    legal_person_name: normalizeText(defaults?.legal_person_name),
    legal_person_id_number: '',
    corporate_mobile: '',
    email: '',
    contact_name: normalizeText(defaults?.contact_name),
    contact_mobile: ''
  }
}

function buildProfilePayload(
  form: PlatformProfileForm,
  bank: ApplymentBankFormPayload,
  defaults?: BaofuSettlementAccountProfileDefaults | null
): BaofuAccountProfile {
  const payload: BaofuAccountProfile = {
    legal_name: form.legal_name.trim(),
    business_license_number: form.business_license_number.trim(),
    legal_person_name: form.legal_person_name.trim(),
    legal_person_id_number: form.legal_person_id_number.trim(),
    corporate_mobile: form.corporate_mobile.trim(),
    email: form.email.trim(),
    bank_account_no: normalizeText(bank.account_number),
    bank_name: normalizeText(bank.bank_alias || bank.account_bank || defaults?.bank_name),
    deposit_bank_province: normalizeText(bank.deposit_bank_province || defaults?.deposit_bank_province),
    deposit_bank_city: normalizeText(bank.deposit_bank_city || defaults?.deposit_bank_city),
    deposit_bank_name: normalizeText(bank.bank_name || defaults?.deposit_bank_name),
    contact_name: form.contact_name.trim(),
    contact_mobile: form.contact_mobile.trim()
  }
  if (bank.account_type === 'ACCOUNT_TYPE_PRIVATE') {
    payload.self_employed = true
    payload.card_user_name = normalizeText(bank.account_name || form.legal_person_name || defaults?.legal_person_name)
  } else {
    payload.self_employed = false
  }
  if (!payload.legal_person_id_number && hasStoredLegalPersonID(defaults)) {
    delete payload.legal_person_id_number
  }
  if (!payload.corporate_mobile && hasStoredCorporateMobile(defaults)) {
    delete payload.corporate_mobile
  }
  if (!payload.email && hasStoredEmail(defaults)) {
    delete payload.email
  }
  if (!payload.bank_account_no && hasStoredBankAccount(defaults)) {
    delete payload.bank_account_no
  }
  if (!payload.contact_mobile && hasStoredContactMobile(defaults)) {
    delete payload.contact_mobile
  }
  return payload
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
    pageView: { ...EMPTY_PAGE_VIEW } as BaofuRolePageView,
    form: emptyForm(),
    bankDraft: null as ApplymentBankFormDraftPayload | null,
    profileDefaults: null as BaofuSettlementAccountProfileDefaults | null,
    formErrorMessage: '',
    canEditProfile: false,
    canRefreshStatus: true,
    hasStoredLegalPersonID: false,
    hasStoredCorporateMobile: false,
    hasStoredEmail: false,
    hasStoredBankAccount: false,
    hasStoredContactMobile: false
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
  },

  onPullDownRefresh() {
    void this.loadAccount({ force: true, refreshing: true })
  },

  applyAccount(response: BaofuSettlementAccountResponse) {
    const pageView = buildBaofuRolePageView('platform', response)
    const profileDefaults = response.profile_defaults || null
    const canEditProfile = pageView.shouldShowProfileAction
    this.setData({
      pageView,
      profileDefaults,
      form: canEditProfile ? buildFormFromDefaults(profileDefaults) : this.data.form,
      bankDraft: canEditProfile ? buildBankDraftFromDefaults(profileDefaults) : this.data.bankDraft,
      canEditProfile,
      canRefreshStatus: pageView.shouldShowRefreshAction,
      hasStoredLegalPersonID: hasStoredLegalPersonID(profileDefaults),
      hasStoredCorporateMobile: hasStoredCorporateMobile(profileDefaults),
      hasStoredEmail: hasStoredEmail(profileDefaults),
      hasStoredBankAccount: hasStoredBankAccount(profileDefaults),
      hasStoredContactMobile: hasStoredContactMobile(profileDefaults),
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
      const response = await getPlatformBaofuSettlementAccount()
      this.applyAccount(response)
    } catch (error: unknown) {
      logger.error('Load platform baofu settlement account failed action=load_account role=platform', error, 'platform-baofu-settlement-account')
      const message = getErrorUserMessage(error, '平台宝付开户状态加载失败，请稍后重试')
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

  validateSubjectForm() {
    const form = this.data.form as PlatformProfileForm
    const defaults = this.data.profileDefaults as BaofuSettlementAccountProfileDefaults | null
    if (!form.legal_name.trim()) return '请输入平台主体名称'
    if (!form.business_license_number.trim()) return '请输入营业执照号'
    if (!form.legal_person_name.trim()) return '请输入法人姓名'
    if (!hasStoredLegalPersonID(defaults) && !/(^\d{15}$)|(^\d{17}[\dXx]$)/.test(form.legal_person_id_number.trim())) return '请输入正确法人身份证号'
    const bankDraft = this.data.bankDraft as ApplymentBankFormDraftPayload | null
    if (bankDraft?.account_type === 'ACCOUNT_TYPE_PRIVATE' && !hasStoredCorporateMobile(defaults) && !/^1\d{10}$/.test(form.corporate_mobile.trim())) return '请输入正确法人手机号'
    if (!hasStoredEmail(defaults) && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email.trim())) return '请输入正确联系邮箱'
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

  async onSubmitBank(e: WechatMiniprogram.CustomEvent<ApplymentBankFormPayload>) {
    if (this.data.submitting || !this.data.canEditProfile) {
      return
    }

    const formErrorMessage = this.validateSubjectForm()
    if (formErrorMessage) {
      this.setData({ formErrorMessage })
      showResultToast(this, formErrorMessage)
      return
    }

    this.setData({ submitting: true, formErrorMessage: '', actionFeedbackMessage: '' })
    try {
      const result = await startBaofuAccountOnboarding(buildProfilePayload(
        this.data.form as PlatformProfileForm,
        e.detail,
        this.data.profileDefaults as BaofuSettlementAccountProfileDefaults | null
      ), {
        role: 'platform',
        context: this,
        loadingMessage: '正在提交开户资料...'
      })
      this.applyWorkflowResult(result)
    } catch (error: unknown) {
      logger.error('Submit platform baofu settlement profile failed action=submit_profile role=platform', error, 'platform-baofu-settlement-account')
      const message = getErrorUserMessage(error, '平台宝付开户资料提交失败，请稍后重试')
      this.setData({ actionFeedbackMessage: message, actionFeedbackTheme: 'error' })
      showResultToast(this, message, 'error')
    } finally {
      hidePageToast(this)
      this.setData({ submitting: false })
    }
  },

  onBankDraftChange(e: WechatMiniprogram.CustomEvent<ApplymentBankFormDraftPayload>) {
    this.setData({ bankDraft: e.detail })
  },

  async onRefreshStatus() {
    if (this.data.syncing || this.data.submitting) {
      return
    }

    this.setData({ syncing: true, actionFeedbackMessage: '' })
    try {
      const result = await pollBaofuSettlementAccountStatus({
        role: 'platform',
        context: this,
        maxAttempts: 1,
        loadingMessage: '正在刷新开户状态...'
      })
      this.applyWorkflowResult(result)
    } catch (error: unknown) {
      logger.error('Refresh platform baofu settlement status failed action=refresh_status role=platform', error, 'platform-baofu-settlement-account')
      const message = getErrorUserMessage(error, '平台宝付开户状态刷新失败，请稍后重试')
      this.setData({ refreshErrorMessage: message })
      showResultToast(this, message, 'error')
    } finally {
      hidePageToast(this)
      this.setData({ syncing: false })
    }
  }
})
