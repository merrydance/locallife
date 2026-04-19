import type { OperatorApplicationResponse } from '../api/operator-application'

export type CityOption = {
  label: string
  value: number
}

export type RegionOption = {
  label: string
  secondary: string
  value: number
  parentId?: number
}

export type FormDataValue = {
  regionId: number
  regionName: string
  name: string
  contactName: string
  contactPhone: string
  years: number
}

export type UploadEvent = WechatMiniprogram.CustomEvent<{ path?: string }>

export type UploadFieldValue = {
  url: string
  rawUrl?: string
  assetId?: number
}

export type OCRDisplayStateValue = 'idle' | 'processing' | 'done' | 'failed'

export type OperatorOCRDisplayState = {
  businessLicense: OCRDisplayStateValue
  idCard: OCRDisplayStateValue
}

export type UploadFeedbackState = 'idle' | 'processing' | 'success' | 'error'

export type UploadFeedback = {
  state: UploadFeedbackState
  title: string
  description: string
}

export type OperatorUploadFeedback = {
  license: UploadFeedback
  idFront: UploadFeedback
  idBack: UploadFeedback
}

export type OperatorUploadField = 'license' | 'idFront' | 'idBack'

export const DEFAULT_OPERATOR_OCR_DISPLAY_STATE: OperatorOCRDisplayState = {
  businessLicense: 'idle',
  idCard: 'idle'
}

export const EMPTY_UPLOAD_FEEDBACK: UploadFeedback = {
  state: 'idle',
  title: '',
  description: ''
}

export const DEFAULT_OPERATOR_UPLOAD_FEEDBACK: OperatorUploadFeedback = {
  license: { ...EMPTY_UPLOAD_FEEDBACK },
  idFront: { ...EMPTY_UPLOAD_FEEDBACK },
  idBack: { ...EMPTY_UPLOAD_FEEDBACK }
}

export function getOCRString(payload: Record<string, unknown> | undefined, key: string): string {
  const value = payload?.[key]
  return typeof value === 'string' ? value.trim() : ''
}

export function getErrorText(error: unknown, fallback: string): string {
  if (error && typeof error === 'object' && 'userMessage' in error) {
    const userMessage = (error as { userMessage?: string }).userMessage
    if (userMessage) {
      return userMessage
    }
  }
  if (error && typeof error === 'object' && 'data' in error) {
    const data = (error as { data?: { message?: string } }).data
    if (data?.message) {
      return data.message
    }
  }
  return fallback
}

export function createUploadFeedback(state: UploadFeedbackState, title = '', description = ''): UploadFeedback {
  return { state, title, description }
}

export function isNoOperatorApplicationError(error: unknown): boolean {
  if (!error || typeof error !== 'object') {
    return false
  }

  const maybeError = error as {
    statusCode?: number
    userMessage?: string
    message?: string
    data?: { code?: number, message?: string }
  }

  if (maybeError.statusCode === 404) {
    return true
  }

  const userMessage = maybeError.userMessage || ''
  const message = maybeError.message || ''
  const dataMessage = maybeError.data?.message || ''
  const dataCode = maybeError.data?.code
  if (dataCode === 40400) {
    return true
  }

  const fullMessage = `${userMessage} ${message} ${dataMessage}`
  return fullMessage.includes('您还没有申请记录') || fullMessage.includes('40400')
}

export function normalizeRegionText(value: string): string {
  return value
    .trim()
    .replace(/\s+/g, '')
    .replace(/特别行政区|自治州|自治县|地区|盟/g, '')
    .replace(/[市区县]$/g, '')
    .replace(/區/g, '区')
    .replace(/灣/g, '湾')
    .replace(/東/g, '东')
    .replace(/龍/g, '龙')
    .replace(/環/g, '环')
    .replace(/臺/g, '台')
    .toLowerCase()
}

export function buildRegionFullName(region: RegionOption): string {
  return region.secondary ? `${region.secondary} - ${region.label}` : region.label
}

export function hasOperatorBusinessLicenseResult(res?: OperatorApplicationResponse): boolean {
  return Boolean(
    String(res?.business_license_number || '').trim()
    || getOCRString(res?.business_license_ocr as Record<string, unknown> | undefined, 'enterprise_name')
    || getOCRString(res?.business_license_ocr as Record<string, unknown> | undefined, 'credit_code')
    || getOCRString(res?.business_license_ocr as Record<string, unknown> | undefined, 'reg_num')
  )
}

export function hasOperatorIDCardFrontResult(res?: OperatorApplicationResponse): boolean {
  return Boolean(
    String(res?.legal_person_name || '').trim()
    || String(res?.legal_person_id_number || '').trim()
    || getOCRString(res?.id_card_front_ocr as Record<string, unknown> | undefined, 'name')
    || getOCRString(res?.id_card_front_ocr as Record<string, unknown> | undefined, 'id_number')
  )
}

export function hasOperatorIDCardBackResult(res?: OperatorApplicationResponse): boolean {
  return Boolean(
    getOCRString(res?.id_card_back_ocr as Record<string, unknown> | undefined, 'valid_end')
    || getOCRString(res?.id_card_back_ocr as Record<string, unknown> | undefined, 'valid_date')
  )
}

export function buildCityOptionsPatch(cities: CityOption[], currentSelectedCityId: number) {
  const selectedCityId = currentSelectedCityId || (cities[0]?.value || 0)
  const selectedCityIndex = Math.max(0, cities.findIndex((item) => item.value === selectedCityId))
  return {
    cityOptions: cities,
    cityPickerVisible: false,
    selectedCityIndex,
    selectedCityId,
    selectedCityName: cities[selectedCityIndex]?.label || ''
  }
}

export function findMatchedCityOption(cityOptions: CityOption[], cityName: string): CityOption | null {
  if (!cityName || !cityOptions.length) {
    return null
  }

  const target = normalizeRegionText(cityName)
  const exact = cityOptions.find((city) => {
    const current = normalizeRegionText(city.label)
    return current === target || current.includes(target) || target.includes(current)
  })
  if (exact) {
    return exact
  }

  if (target.includes('香港')) {
    return cityOptions.find((city) => city.label.includes('香港')) || null
  }

  return null
}

export function findMatchedDistrictOption(regionOptions: RegionOption[], districtName: string): RegionOption | null {
  if (!districtName || !regionOptions.length) {
    return null
  }

  const target = normalizeRegionText(districtName)
  return regionOptions.find((district) => {
    const current = normalizeRegionText(district.label)
    return current === target || current.includes(target) || target.includes(current)
  }) || null
}

export function buildAvailableRegionsPatch(districts: RegionOption[], keyword: string) {
  const normalizedKeyword = keyword.trim()
  return {
    regionOptions: districts,
    filteredRegions: normalizedKeyword
      ? districts.filter((item) =>
          item.label.toLowerCase().includes(normalizedKeyword.toLowerCase())
          || item.secondary.toLowerCase().includes(normalizedKeyword.toLowerCase())
        )
      : districts,
    regionKeyword: normalizedKeyword
  }
}

export function buildOperatorOcrDisplayState(
  res: OperatorApplicationResponse | undefined,
  uploads: { license: UploadFieldValue, idFront: UploadFieldValue, idBack: UploadFieldValue }
): OperatorOCRDisplayState {
  const businessLicenseUploaded = Boolean(res?.business_license_asset_id || uploads.license.assetId || uploads.license.url)
  const idCardUploaded = Boolean(
    (res?.id_card_front_asset_id || uploads.idFront.assetId || uploads.idFront.url)
    && (res?.id_card_back_asset_id || uploads.idBack.assetId || uploads.idBack.url)
  )

  const businessLicenseStatus = getOCRString(res?.business_license_ocr as Record<string, unknown> | undefined, 'status')
  const idCardFrontStatus = getOCRString(res?.id_card_front_ocr as Record<string, unknown> | undefined, 'status')
  const idCardBackStatus = getOCRString(res?.id_card_back_ocr as Record<string, unknown> | undefined, 'status')
  const businessLicenseDone = businessLicenseStatus === 'done' || hasOperatorBusinessLicenseResult(res)
  const idCardDone = (idCardFrontStatus === 'done' || hasOperatorIDCardFrontResult(res))
    && (idCardBackStatus === 'done' || hasOperatorIDCardBackResult(res))

  return {
    businessLicense: businessLicenseDone
      ? 'done'
      : businessLicenseStatus === 'failed'
        ? 'failed'
        : businessLicenseUploaded
          ? 'processing'
          : 'idle',
    idCard: idCardDone
      ? 'done'
      : idCardFrontStatus === 'failed' || idCardBackStatus === 'failed'
        ? 'failed'
        : idCardUploaded
          ? 'processing'
          : 'idle'
  }
}

export function buildOperatorUploadFeedback(
  res: OperatorApplicationResponse | undefined,
  uploads: { license: UploadFieldValue, idFront: UploadFieldValue, idBack: UploadFieldValue }
): OperatorUploadFeedback {
  const licenseStatus = getOCRString(res?.business_license_ocr as Record<string, unknown> | undefined, 'status')
  const licenseError = getOCRString(res?.business_license_ocr as Record<string, unknown> | undefined, 'error')
  const idFrontStatus = getOCRString(res?.id_card_front_ocr as Record<string, unknown> | undefined, 'status')
  const idFrontError = getOCRString(res?.id_card_front_ocr as Record<string, unknown> | undefined, 'error')
  const idBackStatus = getOCRString(res?.id_card_back_ocr as Record<string, unknown> | undefined, 'status')
  const idBackError = getOCRString(res?.id_card_back_ocr as Record<string, unknown> | undefined, 'error')

  const licenseUploaded = Boolean(res?.business_license_asset_id || uploads.license.assetId || uploads.license.url)
  const idFrontUploaded = Boolean(res?.id_card_front_asset_id || uploads.idFront.assetId || uploads.idFront.url)
  const idBackUploaded = Boolean(res?.id_card_back_asset_id || uploads.idBack.assetId || uploads.idBack.url)
  const licenseReady = licenseStatus === 'done' || hasOperatorBusinessLicenseResult(res)
  const idFrontReady = idFrontStatus === 'done' || hasOperatorIDCardFrontResult(res)
  const idBackReady = idBackStatus === 'done' || hasOperatorIDCardBackResult(res)

  return {
    license: licenseUploaded
      ? licenseStatus === 'failed'
        ? createUploadFeedback('error', '识别失败', licenseError || '请重新上传清晰、完整的营业执照')
        : licenseReady
          ? createUploadFeedback('success', '识别成功', '已识别主体名称和营业执照信息')
          : createUploadFeedback('processing', '证照识别中', '正在识别营业执照信息')
      : { ...EMPTY_UPLOAD_FEEDBACK },
    idFront: idFrontUploaded
      ? idFrontStatus === 'failed'
        ? createUploadFeedback('error', '识别失败', idFrontError || '请重新上传清晰、完整的身份证人像面')
        : idFrontReady
          ? createUploadFeedback('success', '识别成功', '已识别负责人姓名和身份证号')
          : createUploadFeedback('processing', '证照识别中', '正在识别身份证人像面信息')
      : { ...EMPTY_UPLOAD_FEEDBACK },
    idBack: idBackUploaded
      ? idBackStatus === 'failed'
        ? createUploadFeedback('error', '识别失败', idBackError || '请重新上传清晰、完整的身份证国徽面')
        : idBackReady
          ? createUploadFeedback('success', '识别成功', '已识别证件有效期')
          : createUploadFeedback('processing', '证照识别中', '正在识别身份证国徽面信息')
      : { ...EMPTY_UPLOAD_FEEDBACK }
  }
}

export function buildOperatorApplicationPatch(params: {
  res: OperatorApplicationResponse
  regionOptions: RegionOption[]
  phoneError: string
  uploads: { license: UploadFieldValue, idFront: UploadFieldValue, idBack: UploadFieldValue }
}) {
  let regionName = String(params.res.region_name || '')
  const regionId = Number(params.res.region_id || 0)
  if (!regionName && regionId && params.regionOptions.length > 0) {
    const matched = params.regionOptions.find((region) => Number(region.value) === regionId)
    if (matched) {
      regionName = buildRegionFullName(matched)
    }
  }
  if (regionName && regionId && params.regionOptions.length > 0 && !regionName.includes(' - ')) {
    const matched = params.regionOptions.find((region) => Number(region.value) === regionId)
    if (matched) {
      regionName = buildRegionFullName(matched)
    }
  }

  const patch: Record<string, unknown> = {
    'formData.regionId': regionId,
    'formData.name': String(params.res.name || ''),
    'formData.contactName': String(params.res.contact_name || ''),
    'formData.contactPhone': String(params.res.contact_phone || ''),
    'formData.years': Number(params.res.requested_contract_years || 3),
    phoneError: String(params.res.contact_phone || '').trim() ? '' : params.phoneError,
    idFront: { url: '', assetId: params.res.id_card_front_asset_id },
    idBack: { url: '', assetId: params.res.id_card_back_asset_id },
    license: { url: '', assetId: params.res.business_license_asset_id },
    ocrDisplayState: buildOperatorOcrDisplayState(params.res, params.uploads),
    uploadFeedback: buildOperatorUploadFeedback(params.res, params.uploads)
  }

  if (regionName) {
    patch['formData.regionName'] = regionName
  }

  return patch
}

export function getOperatorDocumentRemovalData(field: OperatorUploadField) {
  const documentMap: Record<OperatorUploadField, {
    documentType: 'business_license' | 'id_card_front' | 'id_card_back'
    data: Record<string, unknown>
  }> = {
    license: {
      documentType: 'business_license',
      data: {
        license: { url: '', rawUrl: '', assetId: undefined },
        'uploadFeedback.license': { ...EMPTY_UPLOAD_FEEDBACK }
      }
    },
    idFront: {
      documentType: 'id_card_front',
      data: {
        idFront: { url: '', rawUrl: '', assetId: undefined },
        'uploadFeedback.idFront': { ...EMPTY_UPLOAD_FEEDBACK }
      }
    },
    idBack: {
      documentType: 'id_card_back',
      data: {
        idBack: { url: '', rawUrl: '', assetId: undefined },
        'uploadFeedback.idBack': { ...EMPTY_UPLOAD_FEEDBACK }
      }
    }
  }

  return documentMap[field]
}

export function extractRegionSearchKeyword(detail: unknown): string {
  const rawValue = typeof detail === 'string'
    ? detail
    : detail && typeof detail === 'object' && 'value' in detail
      ? String((detail as { value?: string }).value || '')
      : ''
  const normalizedValue = rawValue === 'undefined' ? '' : rawValue
  return normalizedValue.trim()
}

export function buildCityChangePatch(city: CityOption) {
  return {
    selectedCityId: city.value,
    selectedCityName: city.label,
    regionKeyword: '',
    regionSearchTimer: null,
    lastRegionSearchKeyword: '',
    lastRegionSearchCityId: city.value,
    regionOptions: [],
    filteredRegions: [],
    'formData.regionId': 0,
    'formData.regionName': ''
  }
}

export function buildSelectedRegionPatch(params: {
  region: RegionOption
  cityOptions: CityOption[]
  selectedCityName: string
}) {
  const parentName = params.region.secondary || params.selectedCityName
  const fullName = parentName ? `${parentName} - ${params.region.label}` : buildRegionFullName(params.region)
  const matchedIndex = params.cityOptions.findIndex((item) =>
    (params.region.parentId ? item.value === params.region.parentId : false)
    || (parentName ? item.label === parentName : false)
  )

  const cityState = matchedIndex >= 0
    ? {
        selectedCityIndex: matchedIndex,
        selectedCityId: params.cityOptions[matchedIndex].value,
        selectedCityName: params.cityOptions[matchedIndex].label
      }
    : {
        selectedCityName: parentName || params.selectedCityName
      }

  return {
    ...cityState,
    'formData.regionId': params.region.value,
    'formData.regionName': fullName,
    regionPopupVisible: false
  }
}

export function getOperatorStepOneValidationMessage(formData: FormDataValue): string {
  const normalizedContactName = (formData.contactName || '').trim()
  if (!formData.regionId) {
    return '请选择运营区域'
  }
  if (!normalizedContactName || normalizedContactName.length < 2) {
    return '负责人姓名至少2位'
  }
  if (!formData.contactPhone || formData.contactPhone.length !== 11) {
    return '请输入11位手机号'
  }
  return ''
}

export function buildOperatorBasicPayload(formData: FormDataValue) {
  return {
    name: (formData.name || '').trim(),
    contact_name: (formData.contactName || '').trim(),
    contact_phone: formData.contactPhone,
    requested_contract_years: formData.years
  }
}