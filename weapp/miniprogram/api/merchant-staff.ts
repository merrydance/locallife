import { API_BASE, request } from '../utils/request'

export type MerchantStaffRole = 'owner' | 'manager' | 'chef' | 'cashier' | 'pending'
export type MerchantStaffStatus = 'active' | 'disabled'

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