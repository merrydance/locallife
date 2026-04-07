import { getUserInfo, type UserResponse } from '../api/auth'

export interface ConsoleWorkbench {
  id: string
  name: string
  path: string
  icon: string
}

const MERCHANT_CONSOLE_ROLES = ['merchant', 'merchant_owner', 'merchant_staff']
const USER_INFO_CACHE_FRESHNESS_MS = 60 * 1000

// 临时测试开关，角色联调完成后改回 false。
const TEMP_DISABLE_USER_CENTER_ROLE_VALIDATION = false

let cachedUserInfo: UserResponse | null = null
let cachedUserInfoFetchedAt = 0
let userInfoPromise: Promise<UserResponse> | null = null
let userInfoCacheGeneration = 0

export function invalidateConsoleAccessUserInfoCache() {
  userInfoCacheGeneration += 1
  cachedUserInfo = null
  cachedUserInfoFetchedAt = 0
  userInfoPromise = null
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

export function hasMerchantConsoleAccess(roles: string[]) {
  const normalizedRoles = Array.from(
    new Set(
      roles
        .map((role) => String(role || '').trim().toLowerCase())
        .filter(Boolean)
    )
  )

  return normalizedRoles.some((role) => MERCHANT_CONSOLE_ROLES.includes(role))
}

export type MerchantConsoleAccessResult =
  | { status: 'granted', user?: UserResponse }
  | { status: 'denied', message: string }
  | { status: 'error', message: string }

export function isMerchantConsoleAccessGranted(result: MerchantConsoleAccessResult) {
  return result.status === 'granted'
}

export function isMerchantConsoleAccessDenied(result: MerchantConsoleAccessResult) {
  return result.status === 'denied'
}

export function getMerchantConsoleAccessErrorMessage(result: MerchantConsoleAccessResult) {
  return result.status === 'error' ? result.message : ''
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

export async function ensureMerchantConsoleAccess() {
  if (shouldBypassConsoleRoleValidation()) {
    return { status: 'granted' } as MerchantConsoleAccessResult
  }

  try {
    const user = await getRecentUserInfo()
    const hasAccess = hasMerchantConsoleAccess(user.roles || [])

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

export function resolveConsoleWorkbenches(roles: string[]): ConsoleWorkbench[] {
  if (shouldBypassConsoleRoleValidation()) {
    return [...ALL_CONSOLE_WORKBENCHES]
  }

  const normalizedRoles = Array.from(
    new Set(
      roles
        .map((role) => String(role || '').trim().toLowerCase())
        .filter(Boolean)
    )
  )

  const workbenches: ConsoleWorkbench[] = []

  if (normalizedRoles.some((role) => MERCHANT_CONSOLE_ROLES.includes(role))) {
    workbenches.push(ALL_CONSOLE_WORKBENCHES[0])
  }

  if (normalizedRoles.includes('rider')) {
    workbenches.push(ALL_CONSOLE_WORKBENCHES[1])
  }

  if (normalizedRoles.includes('operator')) {
    workbenches.push(ALL_CONSOLE_WORKBENCHES[2])
  }

  if (normalizedRoles.includes('admin')) {
    workbenches.push(ALL_CONSOLE_WORKBENCHES[3])
  }

  return workbenches
}