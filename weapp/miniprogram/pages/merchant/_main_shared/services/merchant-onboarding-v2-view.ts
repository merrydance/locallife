import type { MerchantApplicationDraftResponse } from '../api/onboarding'
import type { BaofuSettlementAccountResponse } from '../api/baofu-account'
import {
  buildBaofuSettlementAccountView,
  getBaofuAccountStatusText,
  getBaofuAccountNextActionText,
  type StatusTagTheme
} from '../api/baofu-account'
import {
  MERCHANT_ONBOARDING_V2_INTENT_QR_UPDATED_AT,
  MERCHANT_ONBOARDING_V2_INTENT_QR_URL,
  isMerchantOnboardingV2QrUrlConfigured
} from '../config/merchant-onboarding-v2'

// weapp-gate allow-page-api-composition: onboarding v2 ViewState adapter intentionally composes platform application and Baofoo account status into one merchant onboarding task model.
export type MerchantOnboardingV2PlatformState =
  | 'platform_not_started'
  | 'platform_draft'
  | 'platform_submitted'
  | 'platform_rejected'
  | 'platform_approved'

export type MerchantOnboardingV2BaofuState =
  | 'locked_until_platform_approved'
  | 'owner_not_ready_after_approval'
  | 'baofu_profile_pending'
  | 'baofu_verify_fee_pending'
  | 'baofu_verify_fee_processing'
  | 'baofu_opening_processing'
  | 'baofu_merchant_report_processing'
  | 'baofu_applet_auth_pending'
  | 'baofu_failed'
  | 'baofu_voided'
  | 'baofu_ready'

export type MerchantOnboardingV2IntentState =
  | 'locked_until_baofu_ready'
  | 'intent_qr_pending'
  | 'intent_qr_unavailable'

export type MerchantOnboardingV2PrimaryAction =
  | 'start_platform'
  | 'continue_platform'
  | 'refresh'
  | 'submit_baofu'
  | 'open_intent'
  | 'contact_support'
  | 'none'

export type MerchantOnboardingV2StageStatus = 'todo' | 'current' | 'done' | 'error' | 'locked'

export interface MerchantOnboardingV2StageView {
  key: 'platform' | 'baofu' | 'intent'
  title: string
  status: MerchantOnboardingV2StageStatus
  statusText: string
  description: string
}

export interface MerchantOnboardingV2ViewState {
  platformState: MerchantOnboardingV2PlatformState
  baofuState: MerchantOnboardingV2BaofuState
  intentState: MerchantOnboardingV2IntentState
  currentStep: number
  currentTitle: string
  currentDescription: string
  currentStatusText: string
  currentStatusTheme: StatusTagTheme
  primaryAction: MerchantOnboardingV2PrimaryAction
  primaryActionText: string
  secondaryAction: MerchantOnboardingV2PrimaryAction
  secondaryActionText: string
  canRefresh: boolean
  canCopySupportId: boolean
  supportCopyText: string
  ownerIdText: string
  stages: MerchantOnboardingV2StageView[]
  qrUrl: string
  qrUpdatedAt: string
  qrConfigured: boolean
}

export interface MerchantOnboardingV2BuildInput {
  application?: MerchantApplicationDraftResponse | null
  platformNotStarted?: boolean
  baofuAccount?: BaofuSettlementAccountResponse | null
  ownerNotReadyAfterApproval?: boolean
}

export interface MerchantOnboardingV2IntentViewState {
  intentState: MerchantOnboardingV2IntentState
  ready: boolean
  title: string
  statusText: string
  statusTheme: StatusTagTheme
  description: string
  qrUrl: string
  qrUpdatedAt: string
  qrConfigured: boolean
  primaryActionText: string
}

function normalizeStatus(value?: string): string {
  return String(value || '').trim().toLowerCase()
}

function resolvePlatformState(application?: MerchantApplicationDraftResponse | null, platformNotStarted = false): MerchantOnboardingV2PlatformState {
  if (platformNotStarted || !application) {
    return 'platform_not_started'
  }

  switch (normalizeStatus(application.status)) {
    case 'approved':
      return 'platform_approved'
    case 'submitted':
      return 'platform_submitted'
    case 'rejected':
      return 'platform_rejected'
    default:
      return 'platform_draft'
  }
}

function resolveBaofuState(
  platformState: MerchantOnboardingV2PlatformState,
  baofuAccount?: BaofuSettlementAccountResponse | null,
  ownerNotReadyAfterApproval = false
): MerchantOnboardingV2BaofuState {
  if (platformState !== 'platform_approved') {
    return 'locked_until_platform_approved'
  }

  if (ownerNotReadyAfterApproval) {
    return 'owner_not_ready_after_approval'
  }

  if (!baofuAccount) {
    return 'baofu_profile_pending'
  }

  const baofuView = buildBaofuSettlementAccountView(baofuAccount)
  switch (baofuView.normalizedStatus) {
    case 'ready':
      return 'baofu_ready'
    case 'verify_fee_pending':
      return 'baofu_verify_fee_pending'
    case 'verify_fee_processing':
      return 'baofu_verify_fee_processing'
    case 'opening_processing':
      return 'baofu_opening_processing'
    case 'merchant_report_processing':
      return 'baofu_merchant_report_processing'
    case 'applet_auth_pending':
      return 'baofu_applet_auth_pending'
    case 'failed':
      return 'baofu_failed'
    case 'voided':
      return 'baofu_voided'
    default:
      return 'baofu_profile_pending'
  }
}

function resolveIntentState(baofuState: MerchantOnboardingV2BaofuState): MerchantOnboardingV2IntentState {
  if (baofuState !== 'baofu_ready') {
    return 'locked_until_baofu_ready'
  }
  return isMerchantOnboardingV2QrUrlConfigured() ? 'intent_qr_pending' : 'intent_qr_unavailable'
}

function buildPlatformCopy(state: MerchantOnboardingV2PlatformState) {
  switch (state) {
    case 'platform_draft':
      return {
        statusText: '资料草稿',
        description: '继续补全平台入驻资料，提交后等待审核。',
        action: 'continue_platform' as MerchantOnboardingV2PrimaryAction,
        actionText: '继续填写入驻资料'
      }
    case 'platform_submitted':
      return {
        statusText: '平台审核中',
        description: '资料已提交，审核通过后继续宝付开户。',
        action: 'refresh' as MerchantOnboardingV2PrimaryAction,
        actionText: '刷新审核状态'
      }
    case 'platform_rejected':
      return {
        statusText: '资料需修改',
        description: '平台审核未通过，请修改资料后重新提交。',
        action: 'continue_platform' as MerchantOnboardingV2PrimaryAction,
        actionText: '修改后重新提交'
      }
    case 'platform_approved':
      return {
        statusText: '平台已通过',
        description: '平台入驻已完成，可以继续宝付开户。',
        action: 'none' as MerchantOnboardingV2PrimaryAction,
        actionText: ''
      }
    default:
      return {
        statusText: '未开始',
        description: '先完成平台入驻资料，审核通过后才能提交宝付开户资料。',
        action: 'start_platform' as MerchantOnboardingV2PrimaryAction,
        actionText: '开始平台入驻'
      }
  }
}

function buildBaofuCopy(state: MerchantOnboardingV2BaofuState, account?: BaofuSettlementAccountResponse | null) {
  const status = account?.status || account?.state
  switch (state) {
    case 'locked_until_platform_approved':
      return {
        statusText: '待平台审核',
        description: '平台入驻通过后再提交宝付开户资料。',
        action: 'none' as MerchantOnboardingV2PrimaryAction,
        actionText: ''
      }
    case 'owner_not_ready_after_approval':
      return {
        statusText: '账号待生效',
        description: '平台审核已通过，商户老板账号仍在生效中，请刷新或联系平台处理。',
        action: 'refresh' as MerchantOnboardingV2PrimaryAction,
        actionText: '刷新开户状态'
      }
    case 'baofu_profile_pending':
      return {
        statusText: '待提交资料',
        description: '提交宝付开户资料后，页面会持续同步后端开户状态。',
        action: 'submit_baofu' as MerchantOnboardingV2PrimaryAction,
        actionText: '提交宝付开户资料'
      }
    case 'baofu_ready':
      return {
        statusText: getBaofuAccountStatusText(status),
        description: '宝付开户已完成，继续查看法人微信确认开户意愿的二维码流程。',
        action: 'open_intent' as MerchantOnboardingV2PrimaryAction,
        actionText: '查看确认流程'
      }
    case 'baofu_failed': {
      const baofuView = buildBaofuSettlementAccountView(account)
      return {
        statusText: getBaofuAccountStatusText(status),
        description: account?.status_desc || getBaofuAccountNextActionText(status),
        action: baofuView.canSubmitProfile ? 'submit_baofu' as MerchantOnboardingV2PrimaryAction : 'contact_support' as MerchantOnboardingV2PrimaryAction,
        actionText: baofuView.canSubmitProfile ? '修改开户资料' : '联系平台处理'
      }
    }
    case 'baofu_voided':
      return {
        statusText: getBaofuAccountStatusText(status),
        description: account?.status_desc || '当前开户流程已作废，请联系平台处理。',
        action: 'contact_support' as MerchantOnboardingV2PrimaryAction,
        actionText: '联系平台处理'
      }
    default:
      return {
        statusText: getBaofuAccountStatusText(status),
        description: account?.status_desc || getBaofuAccountNextActionText(status),
        action: 'refresh' as MerchantOnboardingV2PrimaryAction,
        actionText: '刷新开户状态'
      }
  }
}

function buildIntentCopy(state: MerchantOnboardingV2IntentState) {
  switch (state) {
    case 'intent_qr_pending':
      return {
        statusText: '需法人微信确认',
        description: '保存拓展二维码后，请法人使用本人微信识别或扫描二维码。',
        action: 'open_intent' as MerchantOnboardingV2PrimaryAction,
        actionText: '保存确认二维码'
      }
    case 'intent_qr_unavailable':
      return {
        statusText: '二维码待配置',
        description: '拓展二维码暂未配置，请联系平台处理。',
        action: 'contact_support' as MerchantOnboardingV2PrimaryAction,
        actionText: '联系平台处理'
      }
    default:
      return {
        statusText: '待宝付开户',
        description: '宝付开户完成后再进行法人微信确认。',
        action: 'none' as MerchantOnboardingV2PrimaryAction,
        actionText: ''
      }
  }
}

function stageStatus(done: boolean, current: boolean, error = false, locked = false): MerchantOnboardingV2StageStatus {
  if (done) return 'done'
  if (error) return 'error'
  if (locked) return 'locked'
  return current ? 'current' : 'todo'
}

function buildOwnerIdText(account?: BaofuSettlementAccountResponse | null): string {
  if (account?.owner_id) {
    return `商户编号 ${account.owner_id}`
  }
  return ''
}

export function buildMerchantOnboardingV2ViewState(input: MerchantOnboardingV2BuildInput): MerchantOnboardingV2ViewState {
  const platformState = resolvePlatformState(input.application, input.platformNotStarted)
  const baofuState = resolveBaofuState(platformState, input.baofuAccount, input.ownerNotReadyAfterApproval)
  const intentState = resolveIntentState(baofuState)
  const platformCopy = buildPlatformCopy(platformState)
  const baofuCopy = buildBaofuCopy(baofuState, input.baofuAccount)
  const intentCopy = buildIntentCopy(intentState)
  const ownerIdText = buildOwnerIdText(input.baofuAccount)
  const baofuDone = baofuState === 'baofu_ready'
  const platformDone = platformState === 'platform_approved'
  const platformCurrent = !platformDone
  const baofuCurrent = platformDone && !baofuDone
  const intentCurrent = baofuDone
  const currentStep = platformCurrent ? 0 : baofuCurrent ? 1 : 2
  const currentCopy = platformCurrent ? platformCopy : baofuCurrent ? baofuCopy : intentCopy
  const baofuError = baofuState === 'baofu_failed' || baofuState === 'baofu_voided'

  return {
    platformState,
    baofuState,
    intentState,
    currentStep,
    currentTitle: platformCurrent ? '平台入驻' : baofuCurrent ? '宝付开户' : '开户意愿确认',
    currentDescription: currentCopy.description,
    currentStatusText: currentCopy.statusText,
    currentStatusTheme: baofuError || platformState === 'platform_rejected' || intentState === 'intent_qr_unavailable' ? 'danger' : currentStep === 2 || platformDone ? 'success' : 'warning',
    primaryAction: currentCopy.action,
    primaryActionText: currentCopy.actionText,
    secondaryAction: baofuState === 'owner_not_ready_after_approval' || baofuState === 'baofu_voided' ? 'contact_support' : 'refresh',
    secondaryActionText: baofuState === 'owner_not_ready_after_approval' || baofuState === 'baofu_voided' ? '联系平台处理' : '刷新进度',
    canRefresh: true,
    canCopySupportId: !!ownerIdText,
    supportCopyText: ownerIdText,
    ownerIdText,
    stages: [
      {
        key: 'platform',
        title: '平台入驻',
        status: stageStatus(platformDone, platformCurrent, platformState === 'platform_rejected'),
        statusText: platformCopy.statusText,
        description: platformCopy.description
      },
      {
        key: 'baofu',
        title: '宝付开户',
        status: stageStatus(baofuDone, baofuCurrent, baofuError, !platformDone),
        statusText: baofuCopy.statusText,
        description: baofuCopy.description
      },
      {
        key: 'intent',
        title: '开户意愿确认',
        status: stageStatus(false, intentCurrent, intentState === 'intent_qr_unavailable', !baofuDone),
        statusText: intentCopy.statusText,
        description: intentCopy.description
      }
    ],
    qrUrl: MERCHANT_ONBOARDING_V2_INTENT_QR_URL,
    qrUpdatedAt: MERCHANT_ONBOARDING_V2_INTENT_QR_UPDATED_AT,
    qrConfigured: isMerchantOnboardingV2QrUrlConfigured()
  }
}

export function buildMerchantOnboardingV2IntentViewState(viewState: MerchantOnboardingV2ViewState): MerchantOnboardingV2IntentViewState {
  const ready = viewState.intentState === 'intent_qr_pending'
  if (viewState.intentState === 'intent_qr_unavailable') {
    return {
      intentState: viewState.intentState,
      ready: false,
      title: '确认二维码待配置',
      statusText: '二维码待配置',
      statusTheme: 'danger',
      description: '拓展二维码暂未配置，请联系平台处理。',
      qrUrl: viewState.qrUrl,
      qrUpdatedAt: viewState.qrUpdatedAt,
      qrConfigured: false,
      primaryActionText: '返回入驻进度'
    }
  }

  if (!ready) {
    return {
      intentState: 'locked_until_baofu_ready',
      ready: false,
      title: '待完成宝付开户',
      statusText: '待宝付开户',
      statusTheme: 'warning',
      description: '宝付开户完成后再展示法人微信确认二维码。',
      qrUrl: viewState.qrUrl,
      qrUpdatedAt: viewState.qrUpdatedAt,
      qrConfigured: viewState.qrConfigured,
      primaryActionText: '返回入驻进度'
    }
  }

  return {
    intentState: 'intent_qr_pending',
    ready: true,
    title: '法人微信确认开户意愿',
    statusText: '需法人微信确认',
    statusTheme: 'warning',
    description: '保存二维码后，请法人使用本人微信从相册识别二维码，或扫描另一台设备上展示的二维码。',
    qrUrl: viewState.qrUrl,
    qrUpdatedAt: viewState.qrUpdatedAt,
    qrConfigured: viewState.qrConfigured,
    primaryActionText: '保存二维码'
  }
}
