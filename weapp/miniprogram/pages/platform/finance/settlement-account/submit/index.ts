import type {
  BaofuSettlementAccountProfileDefaults,
  BaofuSettlementAccountResponse
} from '../../../../../api/baofu-account'
import { getPlatformBaofuSettlementAccount } from '../../../../../api/baofu-account'
import type {
  ApplymentBankFormDraftPayload,
  ApplymentBankFormPayload
} from '../../../../../components/applyment-bank-form/index'
import { baofuSettlementSubmitBehavior } from '../../../../../behaviors/baofu-settlement-submit'
import {
  buildBaofuOnboardingWaitViewFromText,
  startBaofuAccountOnboarding
} from '../../../../../services/baofu-account-onboarding'
import {
  buildBaofuRolePageView
} from '../../../../../services/baofu-account-role-page'
import {
  buildBaofuEnterpriseBankDraftFromDefaults,
  buildBaofuEnterpriseFormFromDefaults,
  buildBaofuEnterpriseProfilePayload,
  emptyBaofuEnterpriseProfileForm,
  getBaofuEnterpriseStoredFlags,
  validateBaofuEnterpriseProfileForm,
  type BaofuEnterpriseProfileField,
  type BaofuEnterpriseProfileForm
} from '../../../../../services/baofu-account-profile-form'
import { logger } from '../../../../../utils/logger'
import { getErrorUserMessage } from '../../../../../utils/user-facing'

interface FieldDataset {
  field?: BaofuEnterpriseProfileField
}

Page({
  behaviors: [
    baofuSettlementSubmitBehavior({
      role: 'platform',
      statusPagePath: '/pages/platform/finance/settlement-account/index',
      getAccount: getPlatformBaofuSettlementAccount,
      logTag: 'platform-baofu-settlement-submit',
      loadErrorFallback: '平台宝付开户资料加载失败，请稍后重试',
      refreshErrorFallback: '平台宝付开户状态刷新失败，请稍后重试'
    })
  ],

  data: {
    form: emptyBaofuEnterpriseProfileForm(),
    bankDraft: { account_type: 'ACCOUNT_TYPE_BUSINESS' } as ApplymentBankFormDraftPayload,
    profileDefaults: null as BaofuSettlementAccountProfileDefaults | null,
    hasStoredLegalPersonID: false,
    hasStoredCorporateMobile: false,
    hasStoredEmail: false,
    hasStoredBankAccount: false,
    showIdNumber: false
  },

  applyAccount(response: BaofuSettlementAccountResponse) {
    const pageView = buildBaofuRolePageView('platform', response)
    const profileDefaults = response.profile_defaults || null
    const flags = getBaofuEnterpriseStoredFlags(profileDefaults)

    this.setData({
      pageView,
      profileDefaults,
      form: buildBaofuEnterpriseFormFromDefaults(profileDefaults),
      bankDraft: buildBaofuEnterpriseBankDraftFromDefaults(profileDefaults),
      canSubmitProfile: pageView.statusView.canSubmitProfile,
      hasStoredLegalPersonID: flags.hasStoredLegalPersonID,
      hasStoredCorporateMobile: flags.hasStoredCorporateMobile,
      hasStoredEmail: flags.hasStoredEmail,
      hasStoredBankAccount: flags.hasStoredBankAccount,
      initialLoading: false,
      initialError: false,
      initialErrorMessage: ''
    })
  },

  onInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as FieldDataset
    if (!field) {
      return
    }

    this.setData({
      [`form.${field}`]: e.detail.value,
      formErrorMessage: ''
    })
  },

  onInputId(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as FieldDataset
    if (!field) {
      return
    }

    // T11: Auto uppercase X for IDs
    const value = String(e.detail.value || '').toUpperCase()

    this.setData({
      [`form.${field}`]: value,
      formErrorMessage: ''
    })
  },

  onToggleIdVisibility() {
    this.setData({ showIdNumber: !this.data.showIdNumber })
  },

  onBankDraftChange(e: WechatMiniprogram.CustomEvent<ApplymentBankFormDraftPayload>) {
    this.setData({ bankDraft: e.detail })
  },

  async onSubmitBank(e: WechatMiniprogram.CustomEvent<ApplymentBankFormPayload>) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const d = this.data as any
    if (d.submitting || d.syncing || !d.canSubmitProfile) {
      return
    }

    const formErrorMessage = validateBaofuEnterpriseProfileForm(
      'platform',
      this.data.form as BaofuEnterpriseProfileForm,
      this.data.bankDraft as ApplymentBankFormDraftPayload,
      this.data.profileDefaults as BaofuSettlementAccountProfileDefaults | null
    )
    if (formErrorMessage) {
      this.setData({ formErrorMessage })
      return
    }

    this.setData({
      submitting: true,
      formErrorMessage: '',
      waitVisible: true,
      ...buildBaofuOnboardingWaitViewFromText({
        state: 'submitting',
        title: '开户资料提交中',
        description: '正在提交资料，结果以后端开户状态为准。',
        theme: 'warning',
        primaryAction: 'dismiss',
        primaryActionText: ''
      })
    })

    try {
      const result = await startBaofuAccountOnboarding(
        buildBaofuEnterpriseProfilePayload(
          this.data.form as BaofuEnterpriseProfileForm,
          e.detail,
          this.data.profileDefaults as BaofuSettlementAccountProfileDefaults | null
        ),
        {
          role: 'platform',
          context: this,
          loadingMessage: '正在提交开户资料...',
          silentToast: true
        }
      )
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      ;(this as any)._applyWorkflowResult(result)
    } catch (error: unknown) {
      logger.error('Submit platform baofu settlement profile failed action=submit_profile role=platform', error, 'platform-baofu-settlement-submit')
      const message = getErrorUserMessage(error, '平台宝付开户资料提交失败，请稍后重试')
      this.setData({
        waitVisible: true,
        ...buildBaofuOnboardingWaitViewFromText({
          state: 'error',
          title: '提交失败',
          description: message,
          theme: 'error',
          primaryAction: 'back_to_status',
          primaryActionText: '返回状态页'
        })
      })
    } finally {
      this.setData({ submitting: false })
    }
  }
})
