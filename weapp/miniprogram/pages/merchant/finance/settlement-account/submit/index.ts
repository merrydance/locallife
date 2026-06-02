import type { BaofuAccountOpeningMode, BaofuSettlementAccountProfileDefaults, BaofuSettlementAccountResponse } from '../../../_main_shared/api/baofu-account'
import { getMerchantBaofuSettlementAccount } from '../../../_main_shared/api/baofu-account'
import type { ApplymentBankFormDraftPayload, ApplymentBankFormPayload } from './_components/applyment-bank-form/index'
import { baofuSettlementSubmitBehavior } from '../../../_main_shared/behaviors/baofu-settlement-submit'
import type { AccessCheckResult } from '../../../_main_shared/behaviors/baofu-settlement-status'
import { buildBaofuOnboardingWaitViewFromText, startBaofuAccountOnboarding } from '../../../_main_shared/services/baofu-account-onboarding'
import { buildBaofuRolePageView } from '../../../_main_shared/services/baofu-account-role-page'
import {
  buildBaofuEnterpriseBankDraftFromDefaults,
  buildBaofuEnterpriseFormFromDefaults,
  buildBaofuEnterpriseProfilePayload,
  buildBaofuPersonalFormFromDefaults,
  buildBaofuPersonalProfilePayload,
  emptyBaofuEnterpriseProfileForm,
  emptyBaofuPersonalProfileForm,
  validateBaofuEnterpriseProfileForm,
  validateBaofuPersonalProfileForm,
  type BaofuEnterpriseProfileField,
  type BaofuEnterpriseProfileForm,
  type BaofuPersonalProfileField,
  type BaofuPersonalProfileForm
} from '../../../_main_shared/services/baofu-account-profile-form'
import { ensureMerchantApplymentAccess, getMerchantConsoleAccessErrorMessage, isMerchantConsoleAccessDenied, isMerchantConsoleAccessGranted } from '../../../../../utils/console-access'
import { logger } from '../../../../../utils/logger'
import { getErrorUserMessage } from '../../../../../utils/user-facing'

interface FieldDataset {
  field?: BaofuEnterpriseProfileField | BaofuPersonalProfileField
}

interface OpeningModeDataset {
  value?: BaofuAccountOpeningMode
}

function showProfileValidationMessage(context: WechatMiniprogram.Page.TrivialInstance, offsetTop: number, content: string) {
  void context
  void offsetTop
  wx.showToast({ title: content, icon: 'none', duration: 2200 })
}

async function merchantAccessGuard(): Promise<AccessCheckResult> {
  const accessResult = await ensureMerchantApplymentAccess()
  if (isMerchantConsoleAccessGranted(accessResult)) {
    return {
      granted: true,
      denied: false,
      deniedMessage: '',
      errorMessage: ''
    }
  }

  if (accessResult.status === 'error') {
    logger.error('Merchant baofu submit access check failed action=check_access role=merchant', accessResult.message, 'merchant-baofu-settlement-submit')
  }

  return {
    granted: false,
    denied: isMerchantConsoleAccessDenied(accessResult),
    deniedMessage: accessResult.status === 'denied' ? accessResult.message : '',
    errorMessage: getMerchantConsoleAccessErrorMessage(accessResult)
  }
}

Page({
  behaviors: [
    baofuSettlementSubmitBehavior({
      role: 'merchant',
      statusPagePath: '/pages/merchant/finance/settlement-account/index',
      getAccount: getMerchantBaofuSettlementAccount,
      accessGuard: merchantAccessGuard,
      logTag: 'merchant-baofu-settlement-submit',
      loadErrorFallback: '商户宝付开户资料加载失败，请稍后重试',
      refreshErrorFallback: '商户宝付开户状态同步失败，请稍后重试'
    })
  ],

  data: {
    accountOpeningMode: 'business' as BaofuAccountOpeningMode,
    accountOpeningModeOptions: [
      {
        value: 'business',
        label: '营业执照开户',
        desc: '使用商户主体资料开户，可按当前规则选择对公或对私结算卡。'
      },
      {
        value: 'personal',
        label: '个人开户',
        desc: '使用个人身份证和本人银行卡开户，仍会继续完成商户报备和小程序授权。'
      }
    ],
    form: emptyBaofuEnterpriseProfileForm(),
    personalForm: emptyBaofuPersonalProfileForm(),
    bankDraft: {
      account_type: 'ACCOUNT_TYPE_BUSINESS'
    } as ApplymentBankFormDraftPayload,
    profileDefaults: null as BaofuSettlementAccountProfileDefaults | null
  },

  applyAccount(response: BaofuSettlementAccountResponse) {
    const pageView = buildBaofuRolePageView('merchant', response)
    const profileDefaults = response.profile_defaults || null

    this.setData({
      pageView,
      profileDefaults,
      form: buildBaofuEnterpriseFormFromDefaults(profileDefaults),
      personalForm: buildBaofuPersonalFormFromDefaults(this.data.personalForm as BaofuPersonalProfileForm, profileDefaults),
      bankDraft: buildBaofuEnterpriseBankDraftFromDefaults(profileDefaults),
      canSubmitProfile: pageView.statusView.canSubmitProfile,
      initialLoading: false,
      initialError: false,
      initialErrorMessage: ''
    })
  },

  onOpeningModeChange(e: WechatMiniprogram.CustomEvent<{ value: BaofuAccountOpeningMode }>) {
    const value = e.detail.value || (e.currentTarget.dataset as OpeningModeDataset).value
    if (value !== 'business' && value !== 'personal') {
      return
    }
    if (value === this.data.accountOpeningMode) {
      return
    }

    this.setData({
      accountOpeningMode: value
    })
  },

  onInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as FieldDataset
    if (!field) {
      return
    }

    this.setData({
      [`form.${field}`]: e.detail.value
    })
  },

  onPersonalInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as FieldDataset
    if (!field) {
      return
    }

    this.setData({
      [`personalForm.${field}`]: e.detail.value
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
      [`form.${field}`]: value
    })
  },

  onPersonalInputId(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as FieldDataset
    if (!field) {
      return
    }

    // T11: Auto uppercase X for IDs
    const value = String(e.detail.value || '').toUpperCase()

    this.setData({
      [`personalForm.${field}`]: value
    })
  },

  onBankDraftChange(e: WechatMiniprogram.CustomEvent<ApplymentBankFormDraftPayload>) {
    this.setData({ bankDraft: e.detail })
  },

  async onSubmitBank(e: WechatMiniprogram.CustomEvent<ApplymentBankFormPayload>) {
    await this.submitCurrentProfile(e.detail)
  },

  async onSubmitPersonal() {
    await this.submitCurrentProfile({} as ApplymentBankFormPayload)
  },

  async submitCurrentProfile(bankPayload: ApplymentBankFormPayload) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const d = this.data as any
    if (d.submitting || d.syncing || !d.canSubmitProfile) {
      return
    }

    const accountOpeningMode = this.data.accountOpeningMode as BaofuAccountOpeningMode
    const formErrorMessage =
      accountOpeningMode === 'personal'
        ? validateBaofuPersonalProfileForm(this.data.personalForm as BaofuPersonalProfileForm, this.data.profileDefaults as BaofuSettlementAccountProfileDefaults | null)
        : validateBaofuEnterpriseProfileForm(
            'merchant',
            this.data.form as BaofuEnterpriseProfileForm,
            this.data.bankDraft as ApplymentBankFormDraftPayload,
            this.data.profileDefaults as BaofuSettlementAccountProfileDefaults | null
          )
    if (formErrorMessage) {
      const navBarHeight = Number((this.data as { navBarHeight?: number }).navBarHeight || 88)
      showProfileValidationMessage(this, navBarHeight, formErrorMessage)
      return
    }

    this.setData({
      submitting: true,
      waitVisible: true,
      waitElapsedSeconds: 0,
      waitRemainingSeconds: 0,
      waitTimerVisible: true,
      ...buildBaofuOnboardingWaitViewFromText({
        state: 'submitting',
        title: '开户资料提交中',
        description: '正在提交资料，结果以后端开户状态为准。',
        theme: 'warning',
        primaryAction: 'dismiss',
        primaryActionText: ''
      })
    })

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const waitSessionId = (this as any)._beginBaofuLongWaitSession()
    try {
      const result = await startBaofuAccountOnboarding(
        accountOpeningMode === 'personal'
          ? buildBaofuPersonalProfilePayload('merchant', this.data.personalForm as BaofuPersonalProfileForm)
          : buildBaofuEnterpriseProfilePayload(this.data.form as BaofuEnterpriseProfileForm, bankPayload, this.data.profileDefaults as BaofuSettlementAccountProfileDefaults | null),
        {
          role: 'merchant',
          accountOpeningMode,
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
      logger.error('Submit merchant baofu settlement profile failed action=submit_profile role=merchant', error, 'merchant-baofu-settlement-submit')
      const message = getErrorUserMessage(error, '商户宝付开户资料提交失败，请稍后重试')
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
