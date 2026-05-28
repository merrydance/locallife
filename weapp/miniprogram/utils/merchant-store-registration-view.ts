import type { MerchantApplicationDraftResponse } from '../api/onboarding'
import type { RegionSearchResult } from '../api/location'

export type MerchantDraftExt = MerchantApplicationDraftResponse & {
  business_address_detail?: string
  legal_person_contact_address?: string
  bank_name?: string
  bank_account?: string
  bank_account_name?: string
}

export type ImageFieldItem = {
  url: string
  rawUrl?: string
  assetId?: number
  localFileUrl?: string
  pendingSync?: boolean
  status?: 'loading' | 'done' | 'failed' | 'reload'
}

type ParsedRegionAddress = {
  province: string
  city: string
  district: string
}

export function buildLegalBusinessAddress(data?: MerchantDraftExt): string {
  return String(data?.business_address || data?.business_license_ocr?.address || '').trim()
}

export function buildMapLocationLabel(params: {
  geocodedAddress?: string
  chosenAddress?: string
  chosenName?: string
  latitude?: number
  longitude?: number
}): string {
  const geocodedAddress = String(params.geocodedAddress || '').trim()
  if (geocodedAddress) return geocodedAddress

  const chosenAddress = String(params.chosenAddress || '').trim()
  const chosenName = String(params.chosenName || '').trim()
  if (chosenAddress && chosenName) {
    return chosenAddress.includes(chosenName) ? chosenAddress : `${chosenAddress} ${chosenName}`
  }
  if (chosenAddress) return chosenAddress
  if (chosenName) return chosenName

  const lat = Number(params.latitude)
  const lng = Number(params.longitude)
  if (Number.isFinite(lat) && Number.isFinite(lng) && lat && lng) {
    return `已选位置：${lat.toFixed(6)}, ${lng.toFixed(6)}`
  }
  return ''
}

export function normalizeImageRawUrl(rawUrl?: string | null): string {
  return typeof rawUrl === 'string' ? rawUrl.trim() : ''
}

export function toPersistedImageUrls(images: ImageFieldItem[]): string[] {
  return Array.from(new Set(
    images
      .map((image) => normalizeImageRawUrl(image.rawUrl))
      .filter((url) => url.length > 0)
  ))
}

export function isImagePendingPersistence(image: ImageFieldItem | null | undefined): boolean {
  if (!image) {
    return false
  }

  return !!image.pendingSync || !!image.localFileUrl || !normalizeImageRawUrl(image.rawUrl)
}

export function isSameImageIdentity(left: ImageFieldItem | null | undefined, right: ImageFieldItem | null | undefined): boolean {
  if (!left || !right) {
    return false
  }

  if (left.assetId && right.assetId && left.assetId === right.assetId) {
    return true
  }

  const leftRawUrl = normalizeImageRawUrl(left.rawUrl)
  const rightRawUrl = normalizeImageRawUrl(right.rawUrl)
  if (leftRawUrl && rightRawUrl && leftRawUrl === rightRawUrl) {
    return true
  }

  return !!left.url && left.url === right.url
}

export function buildUploadRenderImages(images: ImageFieldItem[], previousFiles: ImageFieldItem[] = []): ImageFieldItem[] {
  const nextFiles: ImageFieldItem[] = []

  images.forEach((image) => {
    const matchedPreviousFile = previousFiles.find((previousFile) => isSameImageIdentity(previousFile, image))
    const visibleUrl = matchedPreviousFile?.url || image.localFileUrl || image.url
    const status: ImageFieldItem['status'] = isImagePendingPersistence(image) ? 'loading' : 'done'

    if (!visibleUrl) {
      return
    }

    if (matchedPreviousFile && matchedPreviousFile.url === visibleUrl && matchedPreviousFile.status === status) {
      nextFiles.push(matchedPreviousFile)
      return
    }

    nextFiles.push({
      url: visibleUrl,
      status,
      assetId: image.assetId,
      rawUrl: image.rawUrl,
      localFileUrl: image.localFileUrl,
      pendingSync: image.pendingSync
    })
  })

  return nextFiles
}

export function markImagesPersisted(images: ImageFieldItem[]): ImageFieldItem[] {
  return images.map((image) => {
    if (!normalizeImageRawUrl(image.rawUrl)) {
      return image
    }

    return {
      ...image,
      localFileUrl: undefined,
      pendingSync: false,
      status: 'done'
    }
  })
}

export function toSafeNumber(value: unknown): number {
  const num = Number(value)
  return Number.isFinite(num) ? num : 0
}

function normalizeRegionText(value: string): string {
  return value.replace(/\s+/g, '').trim()
}

function stripRegionSuffix(value: string): string {
  return normalizeRegionText(value).replace(/(特别行政区|自治区|自治州|地区|省|市|区|县|旗)$/u, '')
}

export function parseRegionAddress(address: string): ParsedRegionAddress {
  const normalized = normalizeRegionText(address)
  const provinceMatch = normalized.match(/^(北京|天津|上海|重庆|河北|山西|辽宁|吉林|黑龙江|江苏|浙江|安徽|福建|江西|山东|河南|湖北|湖南|广东|海南|四川|贵州|云南|陕西|甘肃|青海|台湾|内蒙古|广西|西藏|宁夏|新疆|香港|澳门)(省|市|自治区|特别行政区)?/u)
  const province = provinceMatch?.[0] || ''
  const afterProvince = province ? normalized.slice(province.length) : normalized
  const cityMatch = afterProvince.match(/^(.+?)(市|地区|自治州|盟)/u)
  const city = cityMatch?.[0] || ''
  const afterCity = city ? afterProvince.slice(city.length) : afterProvince
  const districtMatch = afterCity.match(/^(.+?)(区|县|旗)/u)
  const district = districtMatch?.[0] || ''

  return { province, city, district }
}

export function buildRegionSearchKeywords(address: string): string[] {
  const parsed = parseRegionAddress(address)
  const candidates = [
    parsed.district,
    stripRegionSuffix(parsed.district),
    parsed.city,
    stripRegionSuffix(parsed.city)
  ]

  const seen = new Set<string>()
  return candidates.filter((value) => {
    const normalized = normalizeRegionText(value)
    if (!normalized || seen.has(normalized)) {
      return false
    }
    seen.add(normalized)
    return true
  })
}

export function pickBestRegionSearchResult(regions: RegionSearchResult[], address: string): RegionSearchResult | null {
  const parsed = parseRegionAddress(address)
  const district = normalizeRegionText(parsed.district)
  const districtBase = stripRegionSuffix(parsed.district)
  const city = normalizeRegionText(parsed.city)
  const cityBase = stripRegionSuffix(parsed.city)

  const candidates = regions.filter((region) => region.level === 3 || region.level === 4)
  if (!candidates.length) {
    return null
  }

  const exactDistrict = candidates.find((region) => normalizeRegionText(region.name) === district)
  if (exactDistrict) {
    return exactDistrict
  }

  const suffixDistrict = candidates.find((region) => {
    const regionName = normalizeRegionText(region.name)
    return Boolean(districtBase) && (regionName === districtBase || stripRegionSuffix(regionName) === districtBase)
  })
  if (suffixDistrict) {
    return suffixDistrict
  }

  const cityScoped = candidates.find((region) => {
    const regionName = normalizeRegionText(region.name)
    return Boolean(cityBase) && Boolean(districtBase) && city.includes(cityBase) && (regionName.includes(districtBase) || district.includes(regionName))
  })
  if (cityScoped) {
    return cityScoped
  }

  return null
}
