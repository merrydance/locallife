export interface ConsoleWorkbench {
  id: string
  name: string
  path: string
  icon: string
}

// 临时测试开关，角色联调完成后改回 false。
const TEMP_DISABLE_USER_CENTER_ROLE_VALIDATION = true

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

  if (normalizedRoles.some((role) => ['merchant', 'merchant_boss', 'merchant_staff'].includes(role))) {
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