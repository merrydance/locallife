export const MERCHANT_ONBOARDING_V2_INTENT_QR_URL = 'https://llapi.merrydance.cn/static/baofu/merchant-intent-confirm-qr.png'

export const MERCHANT_ONBOARDING_V2_INTENT_QR_UPDATED_AT = '2026-06-05'

export const MERCHANT_ONBOARDING_V2_ALLOWED_QR_HOSTS = [
  'llapi.merrydance.cn'
]

export function isMerchantOnboardingV2QrUrlConfigured(url = MERCHANT_ONBOARDING_V2_INTENT_QR_URL): boolean {
  const normalized = String(url || '').trim()
  if (!normalized || normalized === 'https://...') {
    return false
  }

  try {
    const parsed = new URL(normalized)
    return parsed.protocol === 'https:' && MERCHANT_ONBOARDING_V2_ALLOWED_QR_HOSTS.includes(parsed.hostname)
  } catch (_error) {
    return false
  }
}
