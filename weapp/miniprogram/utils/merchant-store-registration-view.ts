import type { MerchantApplicationDraftResponse } from '../api/onboarding'
import { buildMerchantApplicationOCRStatusView } from '../api/onboarding'
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

type UploadFeedbackState = 'idle' | 'processing' | 'success' | 'error'

export type UploadFeedback = {
  state: UploadFeedbackState
  title: string
  description: string
}

export type MerchantUploadFeedback = {
  license: UploadFeedback
  foodPermit: UploadFeedback
  idCardFront: UploadFeedback
  idCardBack: UploadFeedback
}

export type MerchantOCRDisplayState = {
  businessLicenseReady: boolean
  businessLicenseProcessing: boolean
  businessLicenseFailed: boolean
  foodPermitReady: boolean
  foodPermitProcessing: boolean
  foodPermitFailed: boolean
  idCardReady: boolean
  idCardProcessing: boolean
  idCardFailed: boolean
}

export const DEFAULT_MERCHANT_OCR_DISPLAY_STATE: MerchantOCRDisplayState = {
  businessLicenseReady: false,
  businessLicenseProcessing: false,
  businessLicenseFailed: false,
  foodPermitReady: false,
  foodPermitProcessing: false,
  foodPermitFailed: false,
  idCardReady: false,
  idCardProcessing: false,
  idCardFailed: false
}

const EMPTY_UPLOAD_FEEDBACK: UploadFeedback = {
  state: 'idle',
  title: '',
  description: ''
}

export const DEFAULT_MERCHANT_UPLOAD_FEEDBACK: MerchantUploadFeedback = {
  license: { ...EMPTY_UPLOAD_FEEDBACK },
  foodPermit: { ...EMPTY_UPLOAD_FEEDBACK },
  idCardFront: { ...EMPTY_UPLOAD_FEEDBACK },
  idCardBack: { ...EMPTY_UPLOAD_FEEDBACK }
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

export function createUploadFeedback(state: UploadFeedbackState, title = '', description = ''): UploadFeedback {
  return { state, title, description }
}

export function hasMerchantBusinessLicenseResult(data?: MerchantDraftExt): boolean {
  return Boolean(
    String(data?.business_license_number || '').trim()
    || String(data?.business_license_ocr?.enterprise_name || '').trim()
    || String(data?.business_license_ocr?.credit_code || '').trim()
    || String(data?.business_license_ocr?.reg_num || '').trim()
    || String(data?.business_license_ocr?.address || '').trim()
  )
}

export function hasMerchantFoodPermitResult(data?: MerchantDraftExt): boolean {
  return Boolean(
    String(data?.food_permit_ocr?.valid_to || '').trim()
    || String(data?.food_permit_ocr?.permit_no || '').trim()
    || String(data?.food_permit_ocr?.company_name || '').trim()
    || String(data?.food_permit_ocr?.raw_text || '').trim()
  )
}

export function hasMerchantIDCardFrontResult(data?: MerchantDraftExt): boolean {
  return Boolean(
    String(data?.id_card_front_ocr?.name || '').trim()
    || String(data?.legal_person_name || '').trim()
    || String(data?.business_license_ocr?.legal_representative || '').trim()
    || String(data?.id_card_front_ocr?.id_number || '').trim()
    || String(data?.legal_person_id_number || '').trim()
  )
}

export function hasMerchantIDCardBackResult(data?: MerchantDraftExt): boolean {
  return Boolean(String(data?.id_card_back_ocr?.valid_date || '').trim())
}

export function buildMerchantOcrProgressMessage(params: {
  data?: MerchantDraftExt
  hasBusinessLicenseImage: boolean
  hasFoodPermitImage: boolean
  hasIdCardFrontImage: boolean
  hasIdCardBackImage: boolean
}): string {
  const data = params.data
  const checks = [
    {
      uploaded: Boolean((data?.business_license_media_asset_id && data.business_license_media_asset_id > 0) || params.hasBusinessLicenseImage),
      status: data?.business_license_ocr?.status || '',
      ready: hasMerchantBusinessLicenseResult(data)
    },
    {
      uploaded: Boolean((data?.food_permit_media_asset_id && data.food_permit_media_asset_id > 0) || params.hasFoodPermitImage),
      status: data?.food_permit_ocr?.status || '',
      ready: hasMerchantFoodPermitResult(data)
    },
    {
      uploaded: Boolean((data?.id_card_front_media_asset_id && data.id_card_front_media_asset_id > 0) || params.hasIdCardFrontImage),
      status: data?.id_card_front_ocr?.status || '',
      ready: hasMerchantIDCardFrontResult(data)
    },
    {
      uploaded: Boolean((data?.id_card_back_media_asset_id && data.id_card_back_media_asset_id > 0) || params.hasIdCardBackImage),
      status: data?.id_card_back_ocr?.status || '',
      ready: hasMerchantIDCardBackResult(data)
    }
  ]

  const hasInProgress = checks.some((item) => item.uploaded && buildMerchantApplicationOCRStatusView(item.status).isPending && !item.ready)
  if (!hasInProgress) {
    return ''
  }

  return '证照已上传，系统正在自动识别，完成后会自动回填。你可以先继续填写后续信息。'
}

export function buildMerchantOcrDisplayState(params: {
  data?: MerchantDraftExt
  hasBusinessLicenseImage: boolean
  hasFoodPermitImage: boolean
  hasIdCardFrontImage: boolean
  hasIdCardBackImage: boolean
}): MerchantOCRDisplayState {
  const data = params.data
  const businessLicenseUploaded = Boolean((data?.business_license_media_asset_id && data.business_license_media_asset_id > 0) || params.hasBusinessLicenseImage)
  const foodPermitUploaded = Boolean((data?.food_permit_media_asset_id && data.food_permit_media_asset_id > 0) || params.hasFoodPermitImage)
  const idCardFrontUploaded = Boolean((data?.id_card_front_media_asset_id && data.id_card_front_media_asset_id > 0) || params.hasIdCardFrontImage)
  const idCardBackUploaded = Boolean((data?.id_card_back_media_asset_id && data.id_card_back_media_asset_id > 0) || params.hasIdCardBackImage)

  const businessLicenseStatusView = buildMerchantApplicationOCRStatusView(data?.business_license_ocr?.status)
  const foodPermitStatusView = buildMerchantApplicationOCRStatusView(data?.food_permit_ocr?.status)
  const idCardFrontStatusView = buildMerchantApplicationOCRStatusView(data?.id_card_front_ocr?.status)
  const idCardBackStatusView = buildMerchantApplicationOCRStatusView(data?.id_card_back_ocr?.status)

  const businessLicenseDone = businessLicenseStatusView.isReady || hasMerchantBusinessLicenseResult(data)
  const foodPermitDone = foodPermitStatusView.isReady || hasMerchantFoodPermitResult(data)
  const idCardFrontDone = idCardFrontStatusView.isReady || hasMerchantIDCardFrontResult(data)
  const idCardBackDone = idCardBackStatusView.isReady || hasMerchantIDCardBackResult(data)

  return {
    businessLicenseReady: businessLicenseDone,
    businessLicenseFailed: !businessLicenseDone && businessLicenseStatusView.isFailed,
    businessLicenseProcessing: !businessLicenseDone && !businessLicenseStatusView.isFailed && businessLicenseUploaded,
    foodPermitReady: foodPermitDone,
    foodPermitFailed: !foodPermitDone && foodPermitStatusView.isFailed,
    foodPermitProcessing: !foodPermitDone && !foodPermitStatusView.isFailed && foodPermitUploaded,
    idCardReady: idCardFrontDone && idCardBackDone,
    idCardFailed: !(idCardFrontDone && idCardBackDone) && (idCardFrontStatusView.isFailed || idCardBackStatusView.isFailed),
    idCardProcessing: !(idCardFrontDone && idCardBackDone) && !(idCardFrontStatusView.isFailed || idCardBackStatusView.isFailed) && (idCardFrontUploaded || idCardBackUploaded)
  }
}

export function buildMerchantUploadFeedback(params: {
  data?: MerchantDraftExt
  hasBusinessLicenseImage: boolean
  hasFoodPermitImage: boolean
  hasIdCardFrontImage: boolean
  hasIdCardBackImage: boolean
}): MerchantUploadFeedback {
  const data = params.data
  const businessLicenseUploaded = Boolean((data?.business_license_media_asset_id && data.business_license_media_asset_id > 0) || params.hasBusinessLicenseImage)
  const foodPermitUploaded = Boolean((data?.food_permit_media_asset_id && data.food_permit_media_asset_id > 0) || params.hasFoodPermitImage)
  const idCardFrontUploaded = Boolean((data?.id_card_front_media_asset_id && data.id_card_front_media_asset_id > 0) || params.hasIdCardFrontImage)
  const idCardBackUploaded = Boolean((data?.id_card_back_media_asset_id && data.id_card_back_media_asset_id > 0) || params.hasIdCardBackImage)

  const businessLicenseStatusView = buildMerchantApplicationOCRStatusView(data?.business_license_ocr?.status)
  const foodPermitStatusView = buildMerchantApplicationOCRStatusView(data?.food_permit_ocr?.status)
  const idCardFrontStatusView = buildMerchantApplicationOCRStatusView(data?.id_card_front_ocr?.status)
  const idCardBackStatusView = buildMerchantApplicationOCRStatusView(data?.id_card_back_ocr?.status)
  const businessLicenseReady = businessLicenseStatusView.isReady || hasMerchantBusinessLicenseResult(data)
  const foodPermitReady = foodPermitStatusView.isReady || hasMerchantFoodPermitResult(data)
  const idCardFrontReady = idCardFrontStatusView.isReady || hasMerchantIDCardFrontResult(data)
  const idCardBackReady = idCardBackStatusView.isReady || hasMerchantIDCardBackResult(data)

  return {
    license: businessLicenseUploaded
      ? businessLicenseStatusView.isFailed
        ? createUploadFeedback('error', '识别失败', data?.business_license_ocr?.error || '请重新上传清晰、完整的营业执照')
        : businessLicenseReady
          ? createUploadFeedback('success', '识别成功', '已回填主体名称、统一信用代码和经营范围')
          : createUploadFeedback('processing', '证照识别中', '正在识别营业执照信息')
      : { ...EMPTY_UPLOAD_FEEDBACK },
    foodPermit: foodPermitUploaded
      ? foodPermitStatusView.isFailed
        ? createUploadFeedback('error', '识别失败', data?.food_permit_ocr?.error || '请重新上传清晰、完整的食品经营许可证')
        : foodPermitReady
          ? createUploadFeedback('success', '识别成功', '已回填食品经营许可证有效期')
          : createUploadFeedback('processing', '证照识别中', '正在识别食品经营许可证信息')
      : { ...EMPTY_UPLOAD_FEEDBACK },
    idCardFront: idCardFrontUploaded
      ? idCardFrontStatusView.isFailed
        ? createUploadFeedback('error', '识别失败', data?.id_card_front_ocr?.error || '请重新上传清晰、完整的身份证人像面')
        : idCardFrontReady
          ? createUploadFeedback('success', '识别成功', '已回填法人姓名和身份证号')
          : createUploadFeedback('processing', '证照识别中', '正在识别身份证人像面信息')
      : { ...EMPTY_UPLOAD_FEEDBACK },
    idCardBack: idCardBackUploaded
      ? idCardBackStatusView.isFailed
        ? createUploadFeedback('error', '识别失败', data?.id_card_back_ocr?.error || '请重新上传清晰、完整的身份证国徽面')
        : idCardBackReady
          ? createUploadFeedback('success', '识别成功', '已回填身份证有效期')
          : createUploadFeedback('processing', '证照识别中', '正在识别身份证国徽面信息')
      : { ...EMPTY_UPLOAD_FEEDBACK }
  }
}
