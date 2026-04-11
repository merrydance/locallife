import { getUserInfo, type UserResponse, type UserWorkbenchResponse } from '../api/auth'
import {
  getMerchantDeviceAccess,
  type MerchantDeviceAccessResponse
} from '../api/table-device-management'

export interface ConsoleWorkbench {
  id: string
  name: string
  path: string
  icon: string
  description?: string
  disabled?: boolean
  message?: string
}

const MERCHANT_CONSOLE_ROLES = ['merchant', 'merchant_owner', 'merchant_staff']
const MERCHANT_APPLYMENT_MANAGER_ROLES = ['merchant', 'merchant_owner']
const USER_INFO_CACHE_FRESHNESS_MS = 60 * 1000
const MERCHANT_DEVICE_ACCESS_CACHE_FRESHNESS_MS = 60 * 1000

// 临时测试开关，角色联调完成后改回 false。
const TEMP_DISABLE_USER_CENTER_ROLE_VALIDATION = false

let cachedUserInfo: UserResponse | null = null
let cachedUserInfoFetchedAt = 0
let userInfoPromise: Promise<UserResponse> | null = null
let cachedMerchantDeviceAccess: MerchantDeviceAccessResponse | null = null
let cachedMerchantDeviceAccessFetchedAt = 0
let merchantDeviceAccessPromise: Promise<MerchantDeviceAccessResponse> | null = null
let userInfoCacheGeneration = 0

export function invalidateConsoleAccessUserInfoCache() {
  userInfoCacheGeneration += 1
  cachedUserInfo = null
  cachedUserInfoFetchedAt = 0
  userInfoPromise = null
  cachedMerchantDeviceAccess = null
  cachedMerchantDeviceAccessFetchedAt = 0
  merchantDeviceAccessPromise = null
}

const ALL_CONSOLE_WORKBENCHES: ConsoleWorkbench[] = [
  {
    id: 'merchant',
    name: '商户中心',
    icon: '/assets/icons/store.svg',
    path: '/pages/merchant/dashboard/index'
  },
  {
    id: 'rider',
    name: '骑手配送',
    icon: '/assets/icons/rider.svg',
    path: '/pages/rider/dashboard/index'
  },
  {
    id: 'operator',
    name: '运营管理中心',
    icon: '/assets/icons/bill-list.svg',
    path: '/pages/operator/dashboard/index'
  },
  {
    id: 'admin',
    name: '平台管理中心',
    icon: '/assets/icons/platform.svg',
    path: '/pages/platform/dashboard/dashboard'
  }
]

export function shouldBypassConsoleRoleValidation() {
  return TEMP_DISABLE_USER_CENTER_ROLE_VALIDATION
}

function normalizeConsoleRoles(roles: string[]) {
  return Array.from(
    new Set(
      roles
        .map((role) => String(role || '').trim().toLowerCase())
        .filter(Boolean)
    )
  )
}

export function hasMerchantConsoleAccess(roles: string[]) {
  const normalizedRoles = normalizeConsoleRoles(roles)

  return normalizedRoles.some((role) => MERCHANT_CONSOLE_ROLES.includes(role))
}

function hasGrantedMerchantWorkbench(workbenches?: UserWorkbenchResponse[]) {
	return (workbenches || []).some((workbench) => workbench.id === 'merchant' && workbench.status === 'granted')
}

export function canManageMerchantApplyment(roles: string[]) {
  const normalizedRoles = normalizeConsoleRoles(roles)
  return normalizedRoles.some((role) => MERCHANT_APPLYMENT_MANAGER_ROLES.includes(role))
}

export type MerchantConsoleAccessResult =
  | { status: 'granted', user?: UserResponse }
  | { status: 'denied', message: string }
  | { status: 'error', message: string }

export type MerchantDeviceManagementAccessResult =
  | { status: 'granted', message: '', capability: MerchantDeviceAccessResponse, user?: UserResponse }
  | { status: 'denied', message: string, capability?: MerchantDeviceAccessResponse, user?: UserResponse }
  | { status: 'error', message: string, user?: UserResponse }

export function isMerchantConsoleAccessGranted(result: MerchantConsoleAccessResult): result is Extract<MerchantConsoleAccessResult, { status: 'granted' }> {
  return result.status === 'granted'
}

export function isMerchantConsoleAccessDenied(result: MerchantConsoleAccessResult): result is Extract<MerchantConsoleAccessResult, { status: 'denied' }> {
  return result.status === 'denied'
}

export function getMerchantConsoleAccessErrorMessage(result: MerchantConsoleAccessResult) {
  return result.status === 'error' ? result.message : ''
}

export function isMerchantDeviceManagementGranted(result: MerchantDeviceManagementAccessResult): result is Extract<MerchantDeviceManagementAccessResult, { status: 'granted' }> {
  return result.status === 'granted'
}

export function isMerchantDeviceManagementDenied(result: MerchantDeviceManagementAccessResult): result is Extract<MerchantDeviceManagementAccessResult, { status: 'denied' }> {
  return result.status === 'denied'
}

export function getMerchantDeviceManagementErrorMessage(result: MerchantDeviceManagementAccessResult) {
  return result.status === 'error' ? result.message : ''
}

export function getMerchantApplymentAccessDeniedMessage() {
  return '收付通进件仅支持老板账号维护，请联系老板处理。'
}

function getFreshCachedUserInfo() {
  if (!cachedUserInfo) {
    return null
  }

  if (Date.now() - cachedUserInfoFetchedAt > USER_INFO_CACHE_FRESHNESS_MS) {
    return null
  }

  return cachedUserInfo
}

function getFreshCachedMerchantDeviceAccess() {
  if (!cachedMerchantDeviceAccess) {
    return null
  }

  if (Date.now() - cachedMerchantDeviceAccessFetchedAt > MERCHANT_DEVICE_ACCESS_CACHE_FRESHNESS_MS) {
    return null
  }

  return cachedMerchantDeviceAccess
}

function getRecentUserInfo() {
  const cachedUser = getFreshCachedUserInfo()
  if (cachedUser) {
    return Promise.resolve(cachedUser)
  }

  if (userInfoPromise) {
    return userInfoPromise
  }

  const generationAtRequestStart = userInfoCacheGeneration
  const requestPromise = getUserInfo().then((user) => {
    if (generationAtRequestStart === userInfoCacheGeneration) {
      cachedUserInfo = user
      cachedUserInfoFetchedAt = Date.now()
    }
    return user
  })

  userInfoPromise = requestPromise
  requestPromise.finally(() => {
    if (userInfoPromise === requestPromise) {
      userInfoPromise = null
    }
  })

  return requestPromise
}

export async function getRecentMerchantDeviceAccess(force = false) {
  if (!force) {
    const cachedAccess = getFreshCachedMerchantDeviceAccess()
    if (cachedAccess) {
      return cachedAccess
    }
  }

  if (merchantDeviceAccessPromise) {
    return merchantDeviceAccessPromise
  }

  const requestPromise = getMerchantDeviceAccess().then((capability) => {
    cachedMerchantDeviceAccess = capability
    cachedMerchantDeviceAccessFetchedAt = Date.now()
    return capability
  })

  merchantDeviceAccessPromise = requestPromise
  requestPromise.finally(() => {
    if (merchantDeviceAccessPromise === requestPromise) {
      merchantDeviceAccessPromise = null
    }
  })

  return requestPromise
}

export function canUseMerchantDeviceManagement(capability?: MerchantDeviceAccessResponse | null) {
  return !!capability?.can_manage
}

export function canUseMerchantDeviceManagementFallback(roles: string[], capability?: MerchantDeviceAccessResponse | null) {
  if (capability) {
    return capability.can_manage
  }

  return normalizeConsoleRoles(roles).some((role) => ['merchant', 'merchant_owner'].includes(role))
}

export async function ensureMerchantConsoleAccess() {
  if (shouldBypassConsoleRoleValidation()) {
    return { status: 'granted' } as MerchantConsoleAccessResult
  }

  try {
    const user = await getRecentUserInfo()
    const hasAccess = hasGrantedMerchantWorkbench(user.workbenches) || hasMerchantConsoleAccess(user.roles || [])

    if (!hasAccess) {
      return {
        status: 'denied',
        message: '当前账号无商户权限，请返回“我的”切换身份'
      } as MerchantConsoleAccessResult
    }

    return { status: 'granted', user } as MerchantConsoleAccessResult
  } catch (_err) {
    return {
      status: 'error',
      message: '商户权限校验失败，请检查网络后重试'
    } as MerchantConsoleAccessResult
  }
}

export async function ensureMerchantApplymentAccess() {
  if (shouldBypassConsoleRoleValidation()) {
    return { status: 'granted' } as MerchantConsoleAccessResult
  }

  try {
    const user = await getRecentUserInfo()
    const roles = user.roles || []

    if (!hasMerchantConsoleAccess(roles)) {
      return {
        status: 'denied',
        message: '当前账号无商户权限，请返回“我的”切换身份'
      } as MerchantConsoleAccessResult
    }

    if (!canManageMerchantApplyment(roles)) {
      return {
        status: 'denied',
        message: getMerchantApplymentAccessDeniedMessage()
      } as MerchantConsoleAccessResult
    }

    return { status: 'granted', user } as MerchantConsoleAccessResult
  } catch (_err) {
    return {
      status: 'error',
      message: '商户权限校验失败，请检查网络后重试'
    } as MerchantConsoleAccessResult
  }
}

export async function ensureMerchantDeviceManagementAccess(options?: { force?: boolean }) {
  if (shouldBypassConsoleRoleValidation()) {
    return { status: 'granted', message: '', capability: { merchant_id: 0, merchant_name: '', staff_role: 'owner', can_manage: true, allowed_roles: ['owner', 'manager'] } } as MerchantDeviceManagementAccessResult
  }

  const consoleAccess = await ensureMerchantConsoleAccess()
  if (consoleAccess.status !== 'granted') {
    return {
      status: consoleAccess.status,
      message: consoleAccess.status === 'error' ? consoleAccess.message : '当前账号无商户权限，请返回“我的”切换身份'
    } as MerchantDeviceManagementAccessResult
  }

  try {
    const capability = await getRecentMerchantDeviceAccess(Boolean(options?.force))
    if (capability.can_manage) {
      return {
        status: 'granted',
        message: '',
        capability,
        user: consoleAccess.user
      } as MerchantDeviceManagementAccessResult
    }

    return {
      status: 'denied',
      message: capability.block_reason || '打印设备和后厨协同设置仅支持老板或店长管理',
      capability,
      user: consoleAccess.user
    } as MerchantDeviceManagementAccessResult
  } catch (_error) {
    return {
      status: 'error',
      message: '设备管理权限校验失败，请检查网络后重试',
      user: consoleAccess.user
    } as MerchantDeviceManagementAccessResult
  }
}

export function resolveConsoleWorkbenches(roles: string[]): ConsoleWorkbench[] {
  return resolveConsoleWorkbenchesFromProfile(roles)
}

function buildWorkbenchFromBackend(workbench: UserWorkbenchResponse): ConsoleWorkbench | null {
  switch (workbench.id) {
    case 'merchant':
      if (workbench.status === 'pending_assignment') {
        return {
          id: 'merchant',
          name: '商户工作台',
          path: '',
          icon: '/assets/icons/store.svg',
          description: workbench.merchant_name
            ? `已加入 ${workbench.merchant_name}，等待老板分配岗位后开通`
            : '已加入商户，等待老板分配岗位后开通',
          disabled: true,
          message: workbench.message || '已加入商户，等待老板分配岗位后即可进入工作台。'
        }
      }
      return {
        id: 'merchant',
        name: '商户中心',
        path: '/pages/merchant/dashboard/index',
        icon: '/assets/icons/store.svg',
        description: workbench.merchant_name || undefined
      }
    case 'rider':
      return { ...ALL_CONSOLE_WORKBENCHES[1] }
    case 'operator':
      return { ...ALL_CONSOLE_WORKBENCHES[2] }
    case 'admin':
      return { ...ALL_CONSOLE_WORKBENCHES[3] }
    default:
      return null
  }
}

export function resolveConsoleWorkbenchesFromProfile(roles: string[], profileWorkbenches?: UserWorkbenchResponse[]): ConsoleWorkbench[] {
  if (shouldBypassConsoleRoleValidation()) {
    return [...ALL_CONSOLE_WORKBENCHES]
  }

  if (profileWorkbenches && profileWorkbenches.length > 0) {
    return profileWorkbenches
      .map((workbench) => buildWorkbenchFromBackend(workbench))
      .filter((workbench): workbench is ConsoleWorkbench => !!workbench)
  }

  const normalizedRoles = normalizeConsoleRoles(roles)

  const resolvedWorkbenches: ConsoleWorkbench[] = []

  if (normalizedRoles.some((role) => MERCHANT_CONSOLE_ROLES.includes(role))) {
    resolvedWorkbenches.push(ALL_CONSOLE_WORKBENCHES[0])
  }

  if (normalizedRoles.includes('rider')) {
    resolvedWorkbenches.push(ALL_CONSOLE_WORKBENCHES[1])
  }

  if (normalizedRoles.includes('operator')) {
    resolvedWorkbenches.push(ALL_CONSOLE_WORKBENCHES[2])
  }

  if (normalizedRoles.includes('admin')) {
    resolvedWorkbenches.push(ALL_CONSOLE_WORKBENCHES[3])
  }

  return resolvedWorkbenches
}