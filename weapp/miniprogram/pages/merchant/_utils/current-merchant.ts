import { getMyMerchantProfile } from '../../../api/merchant'
import { logger } from '../../../utils/logger'

const CURRENT_MERCHANT_STORAGE_KEY = 'current_merchant'

interface CurrentMerchantCache {
  id?: number
  merchant_id?: number
  name?: string
}

export interface CurrentMerchantContext {
  merchantId: number
  merchantName: string
}

export interface CurrentMerchantContextSyncResult extends CurrentMerchantContext {
  changed: boolean
  source: 'cache' | 'profile'
}

function normalizeCurrentMerchantCache(cache?: CurrentMerchantCache | null): CurrentMerchantContext {
  const merchantId = Number(cache?.merchant_id || cache?.id || 0)

  return {
    merchantId: merchantId > 0 ? merchantId : 0,
    merchantName: String(cache?.name || '').trim()
  }
}

export function readCurrentMerchantContextFromStorage(): CurrentMerchantContext {
  try {
    const cache = wx.getStorageSync(CURRENT_MERCHANT_STORAGE_KEY) as CurrentMerchantCache | null
    return normalizeCurrentMerchantCache(cache)
  } catch (err) {
    logger.warn('Read current merchant cache failed', err)
    return {
      merchantId: 0,
      merchantName: ''
    }
  }
}

export function writeCurrentMerchantContextToStorage(context: CurrentMerchantContext) {
  if (context.merchantId <= 0) {
    return
  }

  try {
    const currentMerchant = wx.getStorageSync(CURRENT_MERCHANT_STORAGE_KEY) || {}
    wx.setStorageSync(CURRENT_MERCHANT_STORAGE_KEY, {
      ...currentMerchant,
      id: context.merchantId,
      merchant_id: context.merchantId,
      name: context.merchantName || currentMerchant.name || ''
    })
  } catch (err) {
    logger.warn('Write current merchant cache failed', err)
  }
}

export async function syncCurrentMerchantContext(options?: {
  currentMerchantId?: number
}): Promise<CurrentMerchantContextSyncResult> {
  const previousMerchantId = Number(options?.currentMerchantId || 0)
  const cachedContext = readCurrentMerchantContextFromStorage()

  if (cachedContext.merchantId > 0) {
    return {
      ...cachedContext,
      changed: previousMerchantId > 0 && previousMerchantId !== cachedContext.merchantId,
      source: 'cache'
    }
  }

  const profile = await getMyMerchantProfile()
  const profileContext: CurrentMerchantContext = {
    merchantId: Number(profile.id || 0),
    merchantName: String(profile.name || '').trim()
  }

  if (profileContext.merchantId <= 0) {
    throw new Error('invalid merchant id')
  }

  writeCurrentMerchantContextToStorage(profileContext)

  return {
    ...profileContext,
    changed: previousMerchantId > 0 && previousMerchantId !== profileContext.merchantId,
    source: 'profile'
  }
}