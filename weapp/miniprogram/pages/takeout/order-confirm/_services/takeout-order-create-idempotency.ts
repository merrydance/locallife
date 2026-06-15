import { logger } from '../../../../utils/logger'

export const TAKEOUT_ORDER_CREATE_IDEMPOTENCY_STORAGE_KEY = 'takeoutOrderCreateIdempotency'
const TAKEOUT_ORDER_CREATE_IDEMPOTENCY_TTL_MS = 2 * 60 * 60 * 1000

interface TakeoutOrderCreateIdempotencyContext {
  signature: string
  idempotencyKey: string
  updatedAt: string
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}

function stableNormalize(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => stableNormalize(item))
  }

  if (!isRecord(value)) {
    return value
  }

  return Object.keys(value).sort().reduce<Record<string, unknown>>((acc, key) => {
    const normalized = stableNormalize(value[key])
    if (normalized !== undefined) {
      acc[key] = normalized
    }
    return acc
  }, {})
}

function isValidContext(value: unknown): value is TakeoutOrderCreateIdempotencyContext {
  if (!isRecord(value)) {
    return false
  }

  return typeof value.signature === 'string' &&
    typeof value.idempotencyKey === 'string' &&
    typeof value.updatedAt === 'string' &&
    value.signature.length > 0 &&
    value.idempotencyKey.length > 0
}

function isContextFresh(context: TakeoutOrderCreateIdempotencyContext): boolean {
  const updatedAt = new Date(context.updatedAt).getTime()
  const ageMs = Date.now() - updatedAt
  return Number.isFinite(updatedAt) && ageMs >= 0 && ageMs < TAKEOUT_ORDER_CREATE_IDEMPOTENCY_TTL_MS
}

function readStoredContext(): TakeoutOrderCreateIdempotencyContext | null {
  try {
    const stored = wx.getStorageSync(TAKEOUT_ORDER_CREATE_IDEMPOTENCY_STORAGE_KEY) as unknown
    return isValidContext(stored) ? stored : null
  } catch (error: unknown) {
    logger.error('读取外卖下单幂等上下文失败', error, 'takeout-order-create-idempotency')
    return null
  }
}

function saveContext(context: TakeoutOrderCreateIdempotencyContext) {
  try {
    wx.setStorageSync(TAKEOUT_ORDER_CREATE_IDEMPOTENCY_STORAGE_KEY, context)
  } catch (error: unknown) {
    logger.error('保存外卖下单幂等上下文失败', error, 'takeout-order-create-idempotency')
    const recoverableError = new Error('下单状态暂时无法保存，请稍后重试')
    ;(recoverableError as Error & { userMessage?: string }).userMessage = '下单状态暂时无法保存，请稍后重试'
    throw recoverableError
  }
}

function buildIdempotencyKey(): string {
  return `takeout-order-create:${Date.now()}:${Math.random().toString(36).slice(2, 12)}`
}

function hashStableText(text: string): string {
  let hash1 = 0x811c9dc5
  let hash2 = 0x9e3779b9
  for (let index = 0; index < text.length; index += 1) {
    const code = text.charCodeAt(index)
    hash1 = Math.imul(hash1 ^ code, 0x01000193)
    hash2 = Math.imul(hash2 + code, 0x85ebca6b)
    hash2 ^= hash2 >>> 13
  }
  return `${(hash1 >>> 0).toString(36)}:${(hash2 >>> 0).toString(36)}:${text.length.toString(36)}`
}

export function buildTakeoutOrderCreateRequestSignature(orderRequest: Record<string, unknown>): string {
  return `v1:${hashStableText(JSON.stringify(stableNormalize(orderRequest)))}`
}

export function ensureTakeoutOrderCreateIdempotencyKey(signature: string): string {
  const normalizedSignature = signature.trim()
  if (!normalizedSignature) {
    const error = new Error('下单请求状态缺失，请刷新后重试')
    ;(error as Error & { userMessage?: string }).userMessage = '下单请求状态缺失，请刷新后重试'
    throw error
  }

  const stored = readStoredContext()
  if (stored?.signature === normalizedSignature && isContextFresh(stored)) {
    return stored.idempotencyKey
  }

  const context = {
    signature: normalizedSignature,
    idempotencyKey: buildIdempotencyKey(),
    updatedAt: new Date().toISOString()
  }
  saveContext(context)
  return context.idempotencyKey
}

export function clearTakeoutOrderCreateIdempotency(idempotencyKey?: string) {
  try {
    const stored = readStoredContext()
    if (idempotencyKey && stored?.idempotencyKey && stored.idempotencyKey !== idempotencyKey) {
      return
    }
    wx.removeStorageSync(TAKEOUT_ORDER_CREATE_IDEMPOTENCY_STORAGE_KEY)
  } catch (error: unknown) {
    logger.error('清理外卖下单幂等上下文失败', error, 'takeout-order-create-idempotency')
  }
}
