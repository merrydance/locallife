import { checkRegionAvailability } from '../api/location'

export function getCurrentRegionId(): number {
  try {
    const app = getApp<IAppOption>()
    return Number(app?.globalData?.currentRegion?.id || 0)
  } catch (_error) {
    return 0
  }
}

export async function getLocalOperatorContactPhone(regionIdParam?: number): Promise<string> {
  const regionId = Number(regionIdParam || getCurrentRegionId())
  if (!Number.isFinite(regionId) || regionId <= 0) {
    return ''
  }

  const result = await checkRegionAvailability(regionId)
  return (result.operator_contact_phone || '').trim()
}

export function normalizeOperatorPhoneNumber(phone?: string): string {
  return String(phone || '').replace(/\s+/g, '')
}
