import type {
  BaofuSettlementAccountProfileDefaults,
  BaofuSettlementAccountResponse
} from '../../_main_shared/api/baofu-account'
import { getRiderBaofuSettlementAccount } from '../../_main_shared/api/baofu-account'
import { baofuSettlementSubmitBehavior } from '../../_main_shared/behaviors/baofu-settlement-submit'
import {
  buildBaofuOnboardingWaitDataPatch,
  buildBaofuOnboardingWaitViewFromText,
  startBaofuAccountOnboarding
} from '../../_main_shared/services/baofu-account-onboarding'
import {
  buildBaofuRolePageView
} from '../../_main_shared/services/baofu-account-role-page'
import {
  buildBaofuPersonalFormFromDefaults,
  buildBaofuPersonalProfilePayload,
  emptyBaofuPersonalProfileForm,
  validateBaofuPersonalProfileForm,
  type BaofuPersonalProfileField,
  type BaofuPersonalProfileForm
} from '../../_main_shared/services/baofu-account-profile-form'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface PersonalFormChangeDetail {
  field?: BaofuPersonalProfileField
  value?: string
}

Page({
  behaviors: [
    baofuSettlementSubmitBehavior({
      role: 'rider',
      statusPagePath: '/pages/rider/settlement-account/index',
      getAccount: getRiderBaofuSettlementAccount,
      logTag: 'rider-settlement-account-submit',
      loadErrorFallback: '开户资料加载失败，请稍后重试',
      refreshErrorFallback: '开户状态同步失败，请稍后重试'
    })
  ],

  data: {
    form: emptyBaofuPersonalProfileForm(),
    profileDefaults: null as BaofuSettlementAccountProfileDefaults | null
  },

  applyAccount(response: BaofuSettlementAccountResponse) {
    const pageView = buildBaofuRolePageView('rider', response)
    const profileDefaults = response.profile_defaults || null
    const currentForm = this.data.form as BaofuPersonalProfileForm

    this.setData({
      pageView,
      profileDefaults,
      form: buildBaofuPersonalFormFromDefaults(currentForm, profileDefaults),
      canSubmitProfile: pageView.statusView.canSubmitProfile,
      initialLoading: false,
      initialError: false,
      initialErrorMessage: ''
    })
  },

  onPersonalFormChange(e: WechatMiniprogram.CustomEvent<PersonalFormChangeDetail>) {
    const { field, value } = e.detail
    if (!field) {
      return
    }

    this.setData({
      [`form.${field}`]: value || '',
      formErrorMessage: ''
    })
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
      waitElapsedSeconds: 0,
      waitRemainingSeconds: 0,
      waitTimerVisible: true,
      ...buildBaofuOnboardingWaitDataPatch(buildBaofuOnboardingWaitViewFromText({
        state: 'submitting',
        title: '开户资料提交中',
        description: '正在提交资料并发起核验，结果以后端开户状态为准。',
        theme: 'warning',
        primaryAction: 'dismiss',
        primaryActionText: ''
      }))
    })

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const waitSessionId = (this as any)._beginBaofuLongWaitSession()
    try {
      const result = await startBaofuAccountOnboarding(
        buildBaofuPersonalProfilePayload('rider', this.data.form as BaofuPersonalProfileForm),
        {
          role: 'rider',
          context: this,
          loadingMessage: '正在提交开户资料...',
          silentToast: true,
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          shouldStop: () => (this as any)._shouldStopBaofuLongWait(waitSessionId),
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          onProgress: (progress) => (this as any)._handleBaofuOnboardingProgress(progress, waitSessionId)
        }
      )
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      if ((this as any)._shouldStopBaofuLongWait(waitSessionId)) {
        return
      }
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      void (this as any)._applyWorkflowResult(result)
    } catch (error: unknown) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      if ((this as any)._shouldStopBaofuLongWait(waitSessionId)) {
        return
      }
      logger.error('Submit rider baofu settlement profile failed action=submit_profile role=rider', error, 'rider-settlement-account-submit')
      const message = getErrorUserMessage(error, '开户资料提交失败，请稍后重试')
      this.setData({
        submitting: false,
        syncing: false,
        waitVisible: true,
        ...buildBaofuOnboardingWaitDataPatch(buildBaofuOnboardingWaitViewFromText({
          state: 'error',
          title: '提交失败',
          description: message,
          theme: 'error',
          primaryAction: 'back_to_status',
          primaryActionText: '返回状态页'
        })),
        waitProgressText: '',
        waitElapsedSeconds: 0,
        waitRemainingSeconds: 0,
        waitUntilTerminal: true,
        waitTimerVisible: false
      })
    } finally {
      this.setData({ submitting: false })
    }
  }
})
