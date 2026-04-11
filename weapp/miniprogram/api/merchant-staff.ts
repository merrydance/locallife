import { API_BASE, request } from '../utils/request'

export type MerchantStaffRole = 'owner' | 'manager' | 'chef' | 'cashier' | 'pending'
export type MerchantStaffStatus = 'active' | 'disabled'

export interface MerchantStaffRoleMeta {
  label: string
  theme: 'primary' | 'success' | 'warning' | 'danger' | 'default'
}

export interface MerchantStaffStatusMeta {
  label: string
  theme: 'success' | 'default'
}

export interface MerchantStaffItem {
  id: number
  merchant_id: number
  user_id: number
  role: MerchantStaffRole
  status: MerchantStaffStatus
  full_name?: string
  avatar_url?: string
  created_at: string
}

export interface MerchantStaffListResponse {
  staff: MerchantStaffItem[]
  count: number
}

export interface MerchantStaffInviteCodeResponse {
  invite_code: string
  expires_at: string
}

export interface UpdateMerchantStaffRoleRequest {
  role: Exclude<MerchantStaffRole, 'owner' | 'pending'>
}

function normalizeAvatarUrl(avatarUrl?: string) {
  if (!avatarUrl) return ''
  if (avatarUrl.startsWith('http://') || avatarUrl.startsWith('https://') || avatarUrl.startsWith('wxfile://') || avatarUrl.startsWith('data:')) {
    return avatarUrl
  }
  if (avatarUrl.startsWith('//')) {
    return `https:${avatarUrl}`
  }
  if (avatarUrl.startsWith('/')) {
    return `${API_BASE}${avatarUrl}`
  }
  return `${API_BASE}/${avatarUrl}`
}

function normalizeStaff(item: MerchantStaffItem): MerchantStaffItem {
  return {
    ...item,
    avatar_url: normalizeAvatarUrl(item.avatar_url)
  }
}

export function listMerchantStaff() {
  return request<MerchantStaffListResponse>({
    url: '/v1/merchant/staff',
    method: 'GET'
  }).then((response) => ({
    ...response,
    staff: Array.isArray(response.staff) ? response.staff.map(normalizeStaff) : []
  }))
}

export function generateMerchantStaffInviteCode() {
  return request<MerchantStaffInviteCodeResponse>({
    url: '/v1/merchant/staff/invite-code',
    method: 'POST'
  })
}

export function updateMerchantStaffRole(staffId: number, data: UpdateMerchantStaffRoleRequest) {
  return request<MerchantStaffItem>({
    url: `/v1/merchant/staff/${staffId}/role`,
    method: 'PATCH',
    data
  }).then(normalizeStaff)
}

export function removeMerchantStaff(staffId: number) {
  return request<{ message?: string }>({
    url: `/v1/merchant/staff/${staffId}`,
    method: 'DELETE'
  })
}

export function isMerchantStaffOwnerRole(role: MerchantStaffRole): boolean {
  return role === 'owner'
}

export function isMerchantStaffManagerRole(role: MerchantStaffRole): boolean {
  return role === 'manager'
}

export function isMerchantStaffPendingRole(role: MerchantStaffRole): boolean {
  return role === 'pending'
}

export function isMerchantStaffActiveStatus(status: MerchantStaffStatus): boolean {
  return status === 'active'
}

export function getMerchantStaffRoleMeta(role: MerchantStaffRole): MerchantStaffRoleMeta {
  switch (role) {
    case 'owner':
      return { label: '老板', theme: 'primary' }
    case 'manager':
      return { label: '店长', theme: 'success' }
    case 'chef':
      return { label: '后厨', theme: 'warning' }
    case 'cashier':
      return { label: '收银', theme: 'primary' }
    case 'pending':
      return { label: '待分配', theme: 'danger' }
    default:
      return { label: role, theme: 'default' }
  }
}

export function getMerchantStaffStatusMeta(status: MerchantStaffStatus): MerchantStaffStatusMeta {
  switch (status) {
    case 'active':
      return { label: '在职', theme: 'success' }
    case 'disabled':
      return { label: '已移除', theme: 'default' }
    default:
      return { label: status || '未知', theme: 'default' }
  }
}