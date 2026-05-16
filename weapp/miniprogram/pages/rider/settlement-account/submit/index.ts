import type {
  BaofuSettlementAccountProfileDefaults,
  BaofuSettlementAccountResponse
} from '../../../../api/baofu-account'
import { getRiderBaofuSettlementAccount } from '../../../../api/baofu-account'
import { baofuSettlementSubmitBehavior } from '../../../../behaviors/baofu-settlement-submit'
import {
  buildBaofuOnboardingWaitViewFromText,
  startBaofuAccountOnboarding
} from '../../../../services/baofu-account-onboarding'
import {
  buildBaofuRolePageView
} from '../../../../services/baofu-account-role-page'
import {
  buildBaofuPersonalFormFromDefaults,
  buildBaofuPersonalProfilePayload,
  emptyBaofuPersonalProfileForm,
  validateBaofuPersonalProfileForm,
  type BaofuPersonalProfileField,
  type BaofuPersonalProfileForm
} from '../../../../services/baofu-account-profile-form'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface FieldDataset {
  field?: BaofuPersonalProfileField
}

Page({
  behaviors: [
    baofuSettlementSubmitBehavior({
      role: 'rider',
      statusPagePath: '/pages/rider/settlement-account/index',
      getAccount: getRiderBaofuSettlementAccount,
      logTag: 'rider-settlement-account-submit',
      loadErrorFallback: '开户资料加载失败，请稍后重试',
      refreshErrorFallback: '开户状态刷新失败，请稍后重试'
    })
  ],

  data: {
    form: emptyBaofuPersonalProfileForm(),
    profileDefaults: null as BaofuSettlementAccountProfileDefaults | null,
    hasStoredCertificateNo: false,
    showIdNumber: false,
    showBankAccount: false
  },

  applyAccount(response: BaofuSettlementAccountResponse) {
    const pageView = buildBaofuRolePageView('rider', response)
    const profileDefaults = response.profile_defaults || null
    const currentForm = this.data.form as BaofuPersonalProfileForm

    this.setData({
      pageView,
      profileDefaults,
      form: buildBaofuPersonalFormFromDefaults(currentForm, profileDefaults),
      hasStoredCertificateNo: Boolean(profileDefaults?.has_certificate_no),
      canSubmitProfile: pageView.statusView.canSubmitProfile,
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

  onToggleBankAccountVisibility() {
    this.setData({ showBankAccount: !this.data.showBankAccount })
  },

  async onSubmitProfile() {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const d = this.data as any
    if (d.submitting || d.syncing || !d.canSubmitProfile) {
      return
    }

    const formErrorMessage = validateBaofuPersonalProfileForm(
      this.data.form as BaofuPersonalProfileForm,
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
        description: '正在提交资料并发起核验，结果以后端开户状态为准。',
        theme: 'warning',
        primaryAction: 'dismiss',
        primaryActionText: ''
      })
    })

    try {
      const result = await startBaofuAccountOnboarding(
        buildBaofuPersonalProfilePayload('rider', this.data.form as BaofuPersonalProfileForm),
        {
          role: 'rider',
          context: this,
          loadingMessage: '正在提交开户资料...',
          silentToast: true
        }
      )
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      ;(this as any)._applyWorkflowResult(result)
    } catch (error: unknown) {
      logger.error('Submit rider baofu settlement profile failed action=submit_profile role=rider', error, 'rider-settlement-account-submit')
      const message = getErrorUserMessage(error, '开户资料提交失败，请稍后重试')
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
