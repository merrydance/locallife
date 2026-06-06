import type { UserResponse } from '../../../../api/auth'
import { getUserInfo } from '../../../../api/auth'
import type { MerchantApplicationDraftResponse } from '../api/onboarding'
import {
  getMerchantBaofuSettlementAccount,
  type BaofuSettlementAccountResponse
} from '../api/baofu-account'
import { ErrorType } from '../../../../utils/error-handler'
import { getErrorDebugMessage, getErrorUserMessage } from '../../../../utils/user-facing'
import {
  buildMerchantOnboardingV2ViewState,
  type MerchantOnboardingV2ViewState
} from './merchant-onboarding-v2-view'

// weapp-gate allow-page-api-composition: onboarding v2 runtime is the task-domain workflow owner that composes platform application and Baofoo account truth before pages render ViewState.
export interface MerchantOnboardingV2RuntimeSnapshot {
  viewState: MerchantOnboardingV2ViewState
  application: MerchantApplicationDraftResponse | null
  baofuAccount: BaofuSettlementAccountResponse | null
}

export interface MerchantOnboardingV2RuntimeLoadResult {
  ok: boolean
  viewState: MerchantOnboardingV2ViewState | null
  errorMessage: string
  lastTrustedViewState: MerchantOnboardingV2ViewState | null
}

export interface MerchantOnboardingV2RuntimeState {
  lastTrustedViewState: MerchantOnboardingV2ViewState | null
  requestSeq: number
}

function getStatusCode(error: unknown): number {
  if (!error || typeof error !== 'object') {
    return 0
  }
  const candidate = error as { statusCode?: unknown, code?: unknown, type?: unknown }
  const statusCode = Number(candidate.statusCode || 0)
  if (Number.isFinite(statusCode) && statusCode > 0) {
    return statusCode
  }
  const code = Number(candidate.code || 0)
  if (Number.isFinite(code) && code >= 100 && code < 600) {
    return code
  }
  return 0
}

function getErrorType(error: unknown): unknown {
  if (!error || typeof error !== 'object') {
    return undefined
  }
  return (error as { type?: unknown }).type
}

export function isKnownNoApplicationError(error: unknown): boolean {
  if (getStatusCode(error) !== 404) {
    return false
  }
  const userMessage = getErrorUserMessage(error, '')
  const debugMessage = getErrorDebugMessage(error).toLowerCase()
  return userMessage.includes('暂无申请记录') ||
    userMessage.includes('您还没有申请记录') ||
    debugMessage.includes('no application') ||
    debugMessage.includes('application not found')
}

export function isOwnerNotReadyAfterApprovalError(error: unknown): boolean {
  const statusCode = getStatusCode(error)
  if (statusCode !== 403 && statusCode !== 404 && statusCode !== 409) {
    return false
  }

  const combined = `${getErrorUserMessage(error, '')} ${getErrorDebugMessage(error)}`.toLowerCase()
  return combined.includes('老板账号') ||
    combined.includes('账号状态未生效') ||
    combined.includes('owner role') ||
    combined.includes('merchant owner') ||
    combined.includes('account is not active') ||
    combined.includes('merchant account is not active') ||
    combined.includes('role not ready') ||
    combined.includes('尚未完成开户')
}

export function isAuthLikeError(error: unknown): boolean {
  return getStatusCode(error) === 401 || getErrorType(error) === ErrorType.AUTH
}

export function hasMerchantOnboardingV2PlatformAccess(user?: UserResponse | null): boolean {
  const roles = (user?.roles || [])
    .map((role) => String(role || '').trim().toLowerCase())
    .filter(Boolean)
  const hasMerchantRole = roles.some((role) => ['merchant', 'merchant_owner', 'merchant_staff'].includes(role))
  const hasGrantedMerchantWorkbench = (user?.workbenches || []).some((workbench) => {
    return workbench.id === 'merchant' && workbench.status === 'granted'
  })
  return hasMerchantRole || hasGrantedMerchantWorkbench
}

export async function loadMerchantOnboardingV2Snapshot(): Promise<MerchantOnboardingV2RuntimeSnapshot> {
  const user = await getUserInfo()
  if (!hasMerchantOnboardingV2PlatformAccess(user)) {
    const viewState = buildMerchantOnboardingV2ViewState({
      application: null,
      platformNotStarted: true,
      baofuAccount: null
    })
    return { viewState, application: null, baofuAccount: null }
  }

  try {
    const baofuAccount = await getMerchantBaofuSettlementAccount()
    const application = { status: 'approved' } as MerchantApplicationDraftResponse
    const viewState = buildMerchantOnboardingV2ViewState({ application, baofuAccount })
    return { viewState, application, baofuAccount }
  } catch (error: unknown) {
    if (isOwnerNotReadyAfterApprovalError(error)) {
      const application = { status: 'approved' } as MerchantApplicationDraftResponse
      const viewState = buildMerchantOnboardingV2ViewState({
        application,
        baofuAccount: null,
        ownerNotReadyAfterApproval: true
      })
      return { viewState, application, baofuAccount: null }
    }
    throw error
  }
}

export async function refreshMerchantOnboardingV2Runtime(
  runtimeState: MerchantOnboardingV2RuntimeState
): Promise<MerchantOnboardingV2RuntimeLoadResult> {
  runtimeState.requestSeq += 1
  const requestSeq = runtimeState.requestSeq

  try {
    const snapshot = await loadMerchantOnboardingV2Snapshot()
    if (requestSeq !== runtimeState.requestSeq) {
      return {
        ok: false,
        viewState: runtimeState.lastTrustedViewState,
        errorMessage: '',
        lastTrustedViewState: runtimeState.lastTrustedViewState
      }
    }

    runtimeState.lastTrustedViewState = snapshot.viewState
    return {
      ok: true,
      viewState: snapshot.viewState,
      errorMessage: '',
      lastTrustedViewState: runtimeState.lastTrustedViewState
    }
  } catch (error: unknown) {
    const message = isAuthLikeError(error)
      ? '登录状态已失效，请重新进入后再试'
      : getErrorUserMessage(error, '入驻进度加载失败，请稍后重试')
    return {
      ok: false,
      viewState: runtimeState.lastTrustedViewState,
      errorMessage: runtimeState.lastTrustedViewState ? `${message}，当前已保留上次同步结果` : message,
      lastTrustedViewState: runtimeState.lastTrustedViewState
    }
  }
}
