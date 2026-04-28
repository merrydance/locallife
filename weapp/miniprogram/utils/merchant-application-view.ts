import {
  buildMerchantApplicationOCRNoticeMessage,
  buildMerchantApplicationOCRStatusView,
  buildMerchantApplicationOCRSubmitBlockMessage,
  buildMerchantApplicationStatusView,
  buildActiveCredentialDisplays,
  buildOnboardingReviewDisplay,
  type MerchantApplicationDraftResponse,
  type OCRStatus as MerchantApplicationOCRStatus
} from '../api/onboarding'
import { getMediaDisplayUrl } from './media'
import { getErrorDebugMessage, getErrorUserMessage } from './user-facing'

export type ApplicationForm = {
  merchantName: string
  contactPhone: string
  businessAddress: string
  businessLicenseNumber: string
  businessScope: string
  legalPersonName: string
  legalPersonIdNumber: string
}

export type UploadFileItem = {
  url: string
  name: string
}

export type UploadField = 'license' | 'foodPermit' | 'idCardFront' | 'idCardBack'

export type OcrStatus = MerchantApplicationOCRStatus | ''

export type ApplicationStatusView = ReturnType<typeof buildMerchantApplicationStatusView>

export interface MerchantApplicationResolvedAssets {
  locationLabel: string
  licenseUrl: string
  foodPermitUrl: string
  idCardFrontUrl: string
  idCardBackUrl: string
}

export interface MerchantApplicationViewSnapshot {
  form: ApplicationForm
  initialForm: ApplicationForm
  status: string
  rejectReason: string
  regionId: number
  latitude: string
  longitude: string
  licenseAssetId: number
  foodPermitAssetId: number
  idCardFrontAssetId: number
  idCardBackAssetId: number
  licenseImageUrl: string
  foodPermitImageUrl: string
  idCardFrontImageUrl: string
  idCardBackImageUrl: string
  licenseImage: UploadFileItem[]
  foodPermitImage: UploadFileItem[]
  idCardFrontImage: UploadFileItem[]
  idCardBackImage: UploadFileItem[]
  licenseOcrStatus: OcrStatus
  foodPermitOcrStatus: OcrStatus
  idCardFrontOcrStatus: OcrStatus
  idCardBackOcrStatus: OcrStatus
}

export const EMPTY_FORM: ApplicationForm = {
  merchantName: '',
  contactPhone: '',
  businessAddress: '',
  businessLicenseNumber: '',
  businessScope: '',
  legalPersonName: '',
  legalPersonIdNumber: ''
}

export const APPLICATION_AUTO_REFRESH_WINDOW_MS = 60 * 1000

export function extractApplicationErrorMessage(error: unknown, fallback: string) {
  return getErrorUserMessage(error, fallback)
}

export function shouldFallbackToLatestApplication(error: unknown) {
  const message = getErrorDebugMessage(error).toLowerCase()
  return message.includes('409')
    || message.includes('冲突')
    || message.includes('submitted')
    || message.includes('approved')
    || message.includes('已提交')
    || message.includes('已通过')
}

export function buildApplicationForm(draft: MerchantApplicationDraftResponse): ApplicationForm {
  return {
    merchantName: draft.merchant_name || draft.business_license_ocr?.enterprise_name || '',
    contactPhone: draft.contact_phone || '',
    businessAddress: draft.business_address || draft.business_license_ocr?.address || '',
    businessLicenseNumber: draft.business_license_number || draft.business_license_ocr?.reg_num || draft.business_license_ocr?.credit_code || '',
    businessScope: draft.business_scope || draft.business_license_ocr?.business_scope || '',
    legalPersonName: draft.legal_person_name || draft.id_card_front_ocr?.name || draft.business_license_ocr?.legal_representative || '',
    legalPersonIdNumber: draft.legal_person_id_number || draft.id_card_front_ocr?.id_number || ''
  }
}

export function hasApplicationFormChanged(current: ApplicationForm, initial: ApplicationForm) {
  return current.merchantName !== initial.merchantName
    || current.contactPhone !== initial.contactPhone
    || current.businessAddress !== initial.businessAddress
    || current.businessLicenseNumber !== initial.businessLicenseNumber
    || current.businessScope !== initial.businessScope
    || current.legalPersonName !== initial.legalPersonName
    || current.legalPersonIdNumber !== initial.legalPersonIdNumber
}

export function extractUploadPath(detail: { path?: string, files?: Array<{ url?: string }> }) {
  if (detail?.path) return detail.path
  const latestFile = detail?.files?.[detail.files.length - 1]
  return latestFile?.url || ''
}

export function buildLocationLabel(address: string) {
  return address.trim() || '--'
}

export function buildChosenLocationAddress(result: WechatMiniprogram.ChooseLocationSuccessCallbackResult, geocodedAddress = '') {
  const address = geocodedAddress.trim() || result.address || ''
  const name = result.name || ''
  if (address && name) {
    return address.includes(name) ? address : `${address} ${name}`
  }
  return address || name || ''
}

export function resolveDraftPublicAssetUrl(url?: string | null) {
  return getMediaDisplayUrl(url || '')
}

export function buildUploadFileItem(url: string, name: string) {
  return [{ url, name }]
}

export function buildUploadFileState(params: { url: string, name: string, uploaded: boolean, currentFiles: UploadFileItem[] }) {
  if (params.url) return buildUploadFileItem(params.url, params.name)
  if (params.uploaded) return params.currentFiles
  return []
}

export function hasUploadedDocument(params: { assetId?: number | null, files?: UploadFileItem[] }) {
  return (typeof params.assetId === 'number' && params.assetId > 0) || Boolean(params.files?.length)
}

export function getOcrTagTheme(status?: string) {
  const statusView = buildMerchantApplicationOCRStatusView(status)
  if (statusView.isReady) return 'success'
  if (statusView.isFailed) return 'danger'
  if (statusView.isPending) return 'warning'
  return 'default'
}

export function getBadgeColor(theme?: string) {
  switch (theme) {
    case 'success':
      return '#0A7A46'
    case 'danger':
      return '#B6171E'
    case 'warning':
      return '#FC820C'
    case 'primary':
      return '#006B31'
    default:
      return '#667365'
  }
}

export function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

export function buildMerchantApplicationBasicPayload(form: ApplicationForm) {
  return {
    merchant_name: form.merchantName.trim(),
    contact_phone: form.contactPhone.trim(),
    business_address: form.businessAddress.trim(),
    business_license_number: form.businessLicenseNumber.trim() || undefined,
    business_scope: form.businessScope.trim() || undefined,
    legal_person_name: form.legalPersonName.trim() || undefined,
    legal_person_id_number: form.legalPersonIdNumber.trim() || undefined
  }
}

export function buildMerchantApplicationOcrStatusText(status?: string) {
  return buildMerchantApplicationOCRStatusView(status).text
}

export function buildMerchantApplicationSubmitBlockText(statuses: Array<{ label: string, status: OcrStatus }>) {
  return buildMerchantApplicationOCRSubmitBlockMessage(statuses)
}

export function getMerchantApplicationValidationMessage(params: {
  form: ApplicationForm
  forSubmit: boolean
  licenseAssetId: number
  foodPermitAssetId: number
  idCardFrontAssetId: number
  idCardBackAssetId: number
  licenseImage: UploadFileItem[]
  foodPermitImage: UploadFileItem[]
  idCardFrontImage: UploadFileItem[]
  idCardBackImage: UploadFileItem[]
  latitude: string
  longitude: string
  regionId: number
  ocrBlockMessage: string
}) {
  const { form } = params
  if (!form.merchantName.trim()) return '请填写店铺名称'
  if (!form.contactPhone.trim() || form.contactPhone.trim().length !== 11) return '请填写 11 位联系电话'
  if (!form.businessAddress.trim() || form.businessAddress.trim().length < 5) return '请填写完整经营地址'
  if (!params.forSubmit) return ''
  if (!form.businessLicenseNumber.trim()) return '请上传营业执照并补齐统一信用代码'
  if (!form.legalPersonName.trim() || !form.legalPersonIdNumber.trim()) return '请上传身份证并补齐法人信息'
  if (
    !hasUploadedDocument({ assetId: params.licenseAssetId, files: params.licenseImage })
    || !hasUploadedDocument({ assetId: params.foodPermitAssetId, files: params.foodPermitImage })
    || !hasUploadedDocument({ assetId: params.idCardFrontAssetId, files: params.idCardFrontImage })
    || !hasUploadedDocument({ assetId: params.idCardBackAssetId, files: params.idCardBackImage })
  ) {
    return '请先上传营业执照、食品经营许可证和身份证正反面'
  }
  if (!params.latitude || !params.longitude) return '请先选择经营位置'
  if (!params.regionId) return '当前位置还未匹配到经营区域，请重新选择更准确的位置'
  return params.ocrBlockMessage
}

export function getMerchantApplicationUploadingKey(field: UploadField) {
  switch (field) {
    case 'license':
      return 'licenseUploading'
    case 'foodPermit':
      return 'foodPermitUploading'
    case 'idCardFront':
      return 'idCardFrontUploading'
    default:
      return 'idCardBackUploading'
  }
}

export function buildMerchantApplicationDraftPatch(
  draft: MerchantApplicationDraftResponse,
  snapshot: MerchantApplicationViewSnapshot,
  resolved: MerchantApplicationResolvedAssets,
  keepDirty: boolean
) {
  const form = keepDirty ? snapshot.form : buildApplicationForm(draft)
  const initialForm = buildApplicationForm(draft)
  const statusView = buildMerchantApplicationStatusView(draft.status)
  const licenseAssetId = Number(draft.business_license_media_asset_id || 0)
  const foodPermitAssetId = Number(draft.food_permit_media_asset_id || 0)
  const idCardFrontAssetId = Number(draft.id_card_front_media_asset_id || 0)
  const idCardBackAssetId = Number(draft.id_card_back_media_asset_id || 0)
  const ocrStatuses = [
    (draft.business_license_ocr?.status || '') as OcrStatus,
    (draft.food_permit_ocr?.status || '') as OcrStatus,
    (draft.id_card_front_ocr?.status || '') as OcrStatus,
    (draft.id_card_back_ocr?.status || '') as OcrStatus
  ]

  return {
    applicationId: draft.id,
    status: draft.status || 'draft',
    statusView,
    statusBadgeText: statusView.badgeText,
    statusBadgeColor: getBadgeColor(statusView.tagTheme),
    rejectReason: draft.reject_reason || '',
    regionId: draft.region_id || 0,
    latitude: draft.latitude || '',
    longitude: draft.longitude || '',
    locationLabel: resolved.locationLabel,
    form,
    initialForm,
    hasChanges: keepDirty ? hasApplicationFormChanged(snapshot.form, initialForm) : false,
    licenseAssetId,
    foodPermitAssetId,
    idCardFrontAssetId,
    idCardBackAssetId,
    licenseImageUrl: resolved.licenseUrl,
    foodPermitImageUrl: resolved.foodPermitUrl,
    idCardFrontImageUrl: resolved.idCardFrontUrl,
    idCardBackImageUrl: resolved.idCardBackUrl,
    licenseImage: buildUploadFileState({ url: resolved.licenseUrl, name: '营业执照', uploaded: licenseAssetId > 0, currentFiles: snapshot.licenseImage }),
    foodPermitImage: buildUploadFileState({ url: resolved.foodPermitUrl, name: '食品经营许可证', uploaded: foodPermitAssetId > 0, currentFiles: snapshot.foodPermitImage }),
    idCardFrontImage: buildUploadFileState({ url: resolved.idCardFrontUrl, name: '身份证正面', uploaded: idCardFrontAssetId > 0, currentFiles: snapshot.idCardFrontImage }),
    idCardBackImage: buildUploadFileState({ url: resolved.idCardBackUrl, name: '身份证背面', uploaded: idCardBackAssetId > 0, currentFiles: snapshot.idCardBackImage }),
    licenseOcrText: buildMerchantApplicationOcrStatusText(draft.business_license_ocr?.status),
    foodPermitOcrText: buildMerchantApplicationOcrStatusText(draft.food_permit_ocr?.status),
    idCardFrontOcrText: buildMerchantApplicationOcrStatusText(draft.id_card_front_ocr?.status),
    idCardBackOcrText: buildMerchantApplicationOcrStatusText(draft.id_card_back_ocr?.status),
    licenseOcrStatus: (draft.business_license_ocr?.status || '') as OcrStatus,
    foodPermitOcrStatus: (draft.food_permit_ocr?.status || '') as OcrStatus,
    idCardFrontOcrStatus: (draft.id_card_front_ocr?.status || '') as OcrStatus,
    idCardBackOcrStatus: (draft.id_card_back_ocr?.status || '') as OcrStatus,
    licenseOcrTheme: getOcrTagTheme(draft.business_license_ocr?.status),
    foodPermitOcrTheme: getOcrTagTheme(draft.food_permit_ocr?.status),
    idCardFrontOcrTheme: getOcrTagTheme(draft.id_card_front_ocr?.status),
    idCardBackOcrTheme: getOcrTagTheme(draft.id_card_back_ocr?.status),
    ocrNoticeMessage: buildMerchantApplicationOCRNoticeMessage(ocrStatuses),
    reviewDisplay: buildOnboardingReviewDisplay(draft.review_summary, draft.status),
    activeCredentialDisplays: buildActiveCredentialDisplays(draft.active_credentials)
  }
}

export function buildMerchantApplicationOcrMergePatch(
  field: UploadField,
  draft: MerchantApplicationDraftResponse,
  snapshot: MerchantApplicationViewSnapshot,
  resolved: MerchantApplicationResolvedAssets,
  fallbackPath: string
) {
  const nextForm = { ...snapshot.form }

  if (field === 'license') {
    nextForm.merchantName = nextForm.merchantName || draft.business_license_ocr?.enterprise_name || draft.merchant_name || ''
    nextForm.businessLicenseNumber = draft.business_license_number || draft.business_license_ocr?.reg_num || draft.business_license_ocr?.credit_code || nextForm.businessLicenseNumber
    nextForm.businessScope = draft.business_scope || draft.business_license_ocr?.business_scope || nextForm.businessScope
    nextForm.legalPersonName = draft.legal_person_name || draft.business_license_ocr?.legal_representative || nextForm.legalPersonName
  }

  if (field === 'idCardFront') {
    nextForm.legalPersonName = draft.id_card_front_ocr?.name || draft.legal_person_name || nextForm.legalPersonName
    nextForm.legalPersonIdNumber = draft.id_card_front_ocr?.id_number || draft.legal_person_id_number || nextForm.legalPersonIdNumber
  }

  const licenseAssetId = Number(draft.business_license_media_asset_id || snapshot.licenseAssetId || 0)
  const foodPermitAssetId = Number(draft.food_permit_media_asset_id || snapshot.foodPermitAssetId || 0)
  const idCardFrontAssetId = Number(draft.id_card_front_media_asset_id || snapshot.idCardFrontAssetId || 0)
  const idCardBackAssetId = Number(draft.id_card_back_media_asset_id || snapshot.idCardBackAssetId || 0)
  const nextStatus = draft.status || snapshot.status
  const nextStatusView = buildMerchantApplicationStatusView(nextStatus)
  const ocrStatuses = [
    (draft.business_license_ocr?.status || snapshot.licenseOcrStatus || '') as OcrStatus,
    (draft.food_permit_ocr?.status || snapshot.foodPermitOcrStatus || '') as OcrStatus,
    (draft.id_card_front_ocr?.status || snapshot.idCardFrontOcrStatus || '') as OcrStatus,
    (draft.id_card_back_ocr?.status || snapshot.idCardBackOcrStatus || '') as OcrStatus
  ]

  const licenseImageUrl = resolved.licenseUrl || (field === 'license' ? fallbackPath : snapshot.licenseImageUrl)
  const foodPermitImageUrl = resolved.foodPermitUrl || (field === 'foodPermit' ? fallbackPath : snapshot.foodPermitImageUrl)
  const idCardFrontImageUrl = resolved.idCardFrontUrl || (field === 'idCardFront' ? fallbackPath : snapshot.idCardFrontImageUrl)
  const idCardBackImageUrl = resolved.idCardBackUrl || (field === 'idCardBack' ? fallbackPath : snapshot.idCardBackImageUrl)

  return {
    status: nextStatus,
    statusView: nextStatusView,
    statusBadgeText: nextStatusView.badgeText,
    statusBadgeColor: getBadgeColor(nextStatusView.tagTheme),
    rejectReason: draft.reject_reason || snapshot.rejectReason,
    regionId: draft.region_id || snapshot.regionId,
    latitude: draft.latitude || snapshot.latitude,
    longitude: draft.longitude || snapshot.longitude,
    locationLabel: buildLocationLabel(draft.business_address || nextForm.businessAddress),
    form: nextForm,
    hasChanges: hasApplicationFormChanged(nextForm, snapshot.initialForm),
    licenseAssetId,
    foodPermitAssetId,
    idCardFrontAssetId,
    idCardBackAssetId,
    licenseImageUrl,
    foodPermitImageUrl,
    idCardFrontImageUrl,
    idCardBackImageUrl,
    licenseImage: (resolved.licenseUrl || field === 'license') ? buildUploadFileItem(licenseImageUrl, '营业执照') : buildUploadFileState({ url: '', name: '营业执照', uploaded: licenseAssetId > 0, currentFiles: snapshot.licenseImage }),
    foodPermitImage: (resolved.foodPermitUrl || field === 'foodPermit') ? buildUploadFileItem(foodPermitImageUrl, '食品经营许可证') : buildUploadFileState({ url: '', name: '食品经营许可证', uploaded: foodPermitAssetId > 0, currentFiles: snapshot.foodPermitImage }),
    idCardFrontImage: (resolved.idCardFrontUrl || field === 'idCardFront') ? buildUploadFileItem(idCardFrontImageUrl, '身份证正面') : buildUploadFileState({ url: '', name: '身份证正面', uploaded: idCardFrontAssetId > 0, currentFiles: snapshot.idCardFrontImage }),
    idCardBackImage: (resolved.idCardBackUrl || field === 'idCardBack') ? buildUploadFileItem(idCardBackImageUrl, '身份证背面') : buildUploadFileState({ url: '', name: '身份证背面', uploaded: idCardBackAssetId > 0, currentFiles: snapshot.idCardBackImage }),
    licenseOcrText: buildMerchantApplicationOcrStatusText(draft.business_license_ocr?.status),
    foodPermitOcrText: buildMerchantApplicationOcrStatusText(draft.food_permit_ocr?.status),
    idCardFrontOcrText: buildMerchantApplicationOcrStatusText(draft.id_card_front_ocr?.status),
    idCardBackOcrText: buildMerchantApplicationOcrStatusText(draft.id_card_back_ocr?.status),
    licenseOcrStatus: (draft.business_license_ocr?.status || snapshot.licenseOcrStatus || '') as OcrStatus,
    foodPermitOcrStatus: (draft.food_permit_ocr?.status || snapshot.foodPermitOcrStatus || '') as OcrStatus,
    idCardFrontOcrStatus: (draft.id_card_front_ocr?.status || snapshot.idCardFrontOcrStatus || '') as OcrStatus,
    idCardBackOcrStatus: (draft.id_card_back_ocr?.status || snapshot.idCardBackOcrStatus || '') as OcrStatus,
    licenseOcrTheme: getOcrTagTheme(draft.business_license_ocr?.status || snapshot.licenseOcrStatus || ''),
    foodPermitOcrTheme: getOcrTagTheme(draft.food_permit_ocr?.status || snapshot.foodPermitOcrStatus || ''),
    idCardFrontOcrTheme: getOcrTagTheme(draft.id_card_front_ocr?.status || snapshot.idCardFrontOcrStatus || ''),
    idCardBackOcrTheme: getOcrTagTheme(draft.id_card_back_ocr?.status || snapshot.idCardBackOcrStatus || ''),
    ocrNoticeMessage: buildMerchantApplicationOCRNoticeMessage(ocrStatuses),
    reviewDisplay: buildOnboardingReviewDisplay(draft.review_summary, nextStatus),
    activeCredentialDisplays: buildActiveCredentialDisplays(draft.active_credentials)
  }
}