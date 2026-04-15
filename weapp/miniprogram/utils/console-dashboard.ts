import { getErrorDebugMessage, getErrorUserMessage } from './user-facing'

type ConsoleDashboardKind = 'merchant' | 'rider' | 'operator' | 'platform'

type ConsoleDashboardErrorReason = 'permission' | 'activation' | 'transient' | 'unknown'

const PERMISSION_MESSAGES: Record<ConsoleDashboardKind, string> = {
  merchant: '当前角色不支持使用此功能，请切换到商户角色后再试。',
  rider: '当前角色不支持使用此功能，请切换到骑手角色后再试。',
  operator: '当前角色不支持使用此功能，请切换到运营角色后再试。',
  platform: '当前角色不支持使用此功能，请切换到平台管理员角色后再试。'
}

const DEFAULT_MESSAGES: Record<ConsoleDashboardKind, string> = {
  merchant: '商户工作台暂时无法加载，请稍后重试。',
  rider: '骑手工作台暂时无法加载，请稍后重试。',
  operator: '运营中心暂时无法加载，请稍后重试。',
  platform: '平台管理中心暂时无法加载，请稍后重试。'
}

export interface ConsoleDashboardErrorState {
  message: string
  canRetry: boolean
  reason: ConsoleDashboardErrorReason
}

function extractErrorTexts(error: unknown): string[] {
  const debugMessage = getErrorDebugMessage(error)
  const userMessage = getErrorUserMessage(error, '')
  return [userMessage, debugMessage].filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
}

function isPermissionDenied(kind: ConsoleDashboardKind, normalized: string): boolean {
  const commonPatterns = [
    '当前无权限执行该操作',
    '无权限操作',
    'permission denied',
    'forbidden',
    'requires '
  ]

  const rolePatterns: Record<ConsoleDashboardKind, string[]> = {
    merchant: [
      'requires merchant role',
      'merchant owner role not found',
      'not a merchant',
      'you are not associated with any merchant',
      'merchant account'
    ],
    rider: [
      'requires rider role',
      'rider role not found',
      'not a rider',
      'rider account'
    ],
    operator: [
      'requires operator role',
      'operator role not found',
      'operator not found in context',
      'not an operator',
      'operator account'
    ],
    platform: [
      'requires admin role',
      'requires platform role',
      'admin role not found',
      'platform role not found',
      'not an admin',
      'not a platform admin'
    ]
  }

  return commonPatterns.some((pattern) => normalized.includes(pattern.toLowerCase())) ||
    rolePatterns[kind].some((pattern) => normalized.includes(pattern.toLowerCase()))
}

export function getConsoleDashboardErrorState(
  kind: ConsoleDashboardKind,
  error: unknown,
  fallback?: string
): ConsoleDashboardErrorState {
  const candidates = extractErrorTexts(error)

  for (const candidate of candidates) {
    const trimmed = candidate.trim()
    const normalized = trimmed.toLowerCase()

    if (isPermissionDenied(kind, normalized)) {
      return {
        message: PERMISSION_MESSAGES[kind],
        canRetry: false,
        reason: 'permission'
      }
    }

    if (
      normalized.includes('operator account is not active') ||
      normalized.includes('operator is not active') ||
      normalized.includes('开户') ||
      normalized.includes('bindbank')
    ) {
      return {
        message: '当前运营账号状态未生效，暂时不能进入该功能，请联系平台处理。',
        canRetry: false,
        reason: 'activation'
      }
    }

    if (
      normalized.includes('service unavailable') ||
      normalized.includes('bad gateway') ||
      normalized.includes('gateway timeout') ||
      normalized.includes('服务器内部错误') ||
      normalized.includes('服务暂时不可用') ||
      normalized.includes('网络请求失败') ||
      normalized.includes('network error')
    ) {
      return {
        message: fallback || DEFAULT_MESSAGES[kind],
        canRetry: true,
        reason: 'transient'
      }
    }

    if (trimmed) {
      return {
        message: getErrorUserMessage(error, fallback || DEFAULT_MESSAGES[kind]),
        canRetry: false,
        reason: 'unknown'
      }
    }
  }

  return {
    message: fallback || DEFAULT_MESSAGES[kind],
    canRetry: true,
    reason: 'transient'
  }
}

export function getConsoleDashboardErrorMessage(
  kind: ConsoleDashboardKind,
  error: unknown,
  fallback?: string
): string {
  return getConsoleDashboardErrorState(kind, error, fallback).message
}