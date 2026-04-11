export interface MessageError {
  userMessage?: string
  message?: string
}

interface StatusCodeError {
  statusCode?: number | string
  code?: number | string
}

export interface ScanCodeRawPayload {
  path?: string
  result?: string
  rawData?: string
  scene?: string
  query?: Record<string, unknown>
}

export interface WebLoginSessionLookupResult {
  code?: string
  status?: string
}

export interface WebLoginMeta {
  code?: string
  sig?: string
  ts?: number
}

export const ROLE_LABEL_MAP: Record<string, string> = {
  merchant: '商家',
  merchant_owner: '商户老板',
  merchant_staff: '店员',
  rider: '骑手',
  customer: '顾客',
  operator: '运营',
  admin: '管理员'
}

export function normalizeRoles(roles: string[] | string) {
  const rawRoles = Array.isArray(roles) ? roles : [roles]
  return Array.from(new Set(rawRoles.map((role) => String(role || '').trim().toLowerCase()).filter(Boolean)))
}

export function pickPrimaryRole(roles: string[]) {
  if (roles.some((role) => ['merchant', 'merchant_owner', 'merchant_staff'].includes(role))) return 'merchant'
  if (roles.includes('rider')) return 'rider'
  if (roles.includes('operator') || roles.includes('admin')) return 'operator'
  if (roles.includes('customer')) return 'customer'
  return 'guest'
}

export function getErrorStatusCode(error: unknown) {
  if (!error || typeof error !== 'object') return 0
  const knownError = error as StatusCodeError
  const candidates = [knownError.statusCode, knownError.code]
  for (const candidate of candidates) {
    const numericStatusCode = typeof candidate === 'number' ? candidate : Number(candidate)
    if (Number.isFinite(numericStatusCode)) {
      return numericStatusCode
    }
  }
  return 0
}

export function isUsableWebLoginSession(session?: WebLoginSessionLookupResult | null) {
  if (!session?.code) return false
  return session.status !== 'expired' && session.status !== 'consumed'
}

export function toFriendlyMessage(error: unknown, fallback: string) {
  const err = error as MessageError
  const raw = err?.userMessage || err?.message || ''
  const text = String(raw || '').trim()
  if (!text) return fallback
  if (/[\u4e00-\u9fa5]/.test(text)) return text
  const lower = text.toLowerCase()
  if (lower.includes('sig') && lower.includes('required')) return '二维码签名缺失，请刷新二维码后重试'
  if ((lower.includes('ts') || lower.includes('timestamp')) && lower.includes('required')) return '二维码时间戳缺失，请刷新二维码后重试'
  if (lower.includes('signature') || lower.includes('sig') || lower.includes('mismatch')) return '二维码校验失败，请刷新二维码后重试'
  if (lower.includes('expired')) return '二维码已过期，请刷新二维码'
  if (lower.includes('session not found') || lower.includes('not found')) return '登录码不存在或已失效，请刷新二维码'
  if (lower.includes('already consumed')) return '该登录码已被使用，请刷新二维码'
  if (lower.includes('not confirmed')) return '请先在小程序确认登录'
  if (lower.includes('merchant account') || lower.includes('merchant')) return '当前账号暂无商户权限'
  if (lower.includes('too many') || lower.includes('429')) return '操作太频繁，请稍后再试'
  if (lower.includes('network') || lower.includes('timeout')) return '网络异常，请稍后重试'
  return fallback
}

export function consumeForceRefreshFlag(storageKey: string) {
  const shouldForceRefresh = wx.getStorageSync(storageKey) === '1'
  if (shouldForceRefresh) {
    wx.removeStorageSync(storageKey)
  }
  return shouldForceRefresh
}

export function extractCode(raw: string) {
  if (!raw) return ''
  const decoded = decodeURIComponent(raw)
  const match = decoded.match(/code=([^&]+)/)
  if (match) return match[1]
  const webLoginMatch = decoded.match(/web-login:([0-9a-fA-F]{32})/)
  if (webLoginMatch) return webLoginMatch[1]
  const inviteMatch = decoded.match(/invite-merchant:([A-Za-z0-9_-]+)/)
  if (inviteMatch) return inviteMatch[1]
  const hexMatch = decoded.match(/[0-9a-fA-F]{32}/)
  if (hexMatch) return hexMatch[0]
  return decoded
}

export function extractWebLoginMeta(raw: string): WebLoginMeta {
  if (!raw) return { code: '', sig: '', ts: undefined }
  const decoded = decodeURIComponent(raw)
  const queryCodeMatch = decoded.match(/code=([^&]+)/)
  const webLoginMatch = decoded.match(/web-login:([0-9a-fA-F]{32})/)
  const code = queryCodeMatch ? queryCodeMatch[1] : webLoginMatch ? webLoginMatch[1] : ''
  if (!code) return { code: '', sig: '', ts: undefined }
  const sigMatch = decoded.match(/sig=([0-9a-fA-F]+)/)
  const tsMatch = decoded.match(/ts=(\d+)/)
  return { code, sig: sigMatch ? sigMatch[1] : '', ts: tsMatch ? Number(tsMatch[1]) : undefined }
}

export function extractRawPayload(res: WechatMiniprogram.ScanCodeSuccessCallbackResult) {
  const rawRes = res as unknown as ScanCodeRawPayload
  const path = rawRes.path || ''
  const result = rawRes.result || ''
  const rawData = rawRes.rawData || ''
  const scene = rawRes.scene || ''
  const query = rawRes.query || {}
  const codeFromQuery = typeof query.code === 'string' ? query.code : ''
  const candidate = [path, result, rawData, scene, codeFromQuery].find((value) => !!value) || ''
  return {
    raw: String(candidate),
    codeCandidate: String(codeFromQuery || candidate || '')
  }
}