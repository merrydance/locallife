import {
  buildRiderApplicationStatusView,
  type RiderApplicationResponse
} from '../_api/rider-application'
import { buildMerchantApplicationOCRStatusView } from '../../_main_shared/api/onboarding'

export type UploadFieldValue = {
  url: string
  rawUrl?: string
  assetId?: number
}

export type OCRDisplayStateValue = 'idle' | 'processing' | 'done' | 'failed'

export type RiderOCRDisplayState = {
  identity: OCRDisplayStateValue
  health: OCRDisplayStateValue
}

export type RiderOCRPanelState = {
  identityProcessing: boolean
  identityFailed: boolean
  healthProcessing: boolean
  healthFailed: boolean
}

export type UploadFeedbackState = 'idle' | 'processing' | 'success' | 'error'

export type UploadFeedback = {
  state: UploadFeedbackState
  title: string
  description: string
}

export type RiderUploadFeedback = {
  idFront: UploadFeedback
  idBack: UploadFeedback
  healthCert: UploadFeedback
}

export type UploadField = 'idFront' | 'idBack' | 'healthCert'

export type RiderUploadSnapshot = {
  idFront?: UploadFieldValue
  idBack?: UploadFieldValue
  healthCert?: UploadFieldValue
}

export const DEFAULT_RIDER_OCR_DISPLAY_STATE: RiderOCRDisplayState = {
  identity: 'idle',
  health: 'idle'
}

export const DEFAULT_RIDER_OCR_PANEL_STATE: RiderOCRPanelState = {
  identityProcessing: false,
  identityFailed: false,
  healthProcessing: false,
  healthFailed: false
}

export const EMPTY_UPLOAD_FEEDBACK: UploadFeedback = {
  state: 'idle',
  title: '',
  description: ''
}

export const DEFAULT_RIDER_UPLOAD_FEEDBACK: RiderUploadFeedback = {
  idFront: { ...EMPTY_UPLOAD_FEEDBACK },
  idBack: { ...EMPTY_UPLOAD_FEEDBACK },
  healthCert: { ...EMPTY_UPLOAD_FEEDBACK }
}

export function createUploadFeedback(state: UploadFeedbackState, title = '', description = ''): UploadFeedback {
  return { state, title, description }
}

export function isDocumentCorrectionError(message: string): boolean {
  return [
    '身份证',
    '健康证',
    '过期',
    '不一致',
    '未识别',
    '资料核验'
  ].some((keyword) => message.includes(keyword))
}

export function pickOCRText(payload: Record<string, unknown> | undefined, ...keys: string[]): string {
  for (const key of keys) {
    const value = payload?.[key]
    if (typeof value === 'string' && value.trim()) {
      return value.trim()
    }
  }
  return ''
}

export function hasRiderUploadAssetId(assetId?: number): boolean {
  return typeof assetId === 'number' && assetId > 0
}

function hasOCRText(payload: Record<string, unknown> | undefined, ...keys: string[]): boolean {
  return keys.some((key) => {
    const value = payload?.[key]
    return typeof value === 'string' && value.trim().length > 0
  })
}

export function hasHealthCertKeyFields(payload: Record<string, unknown> | undefined): boolean {
  return hasOCRText(payload, 'name')
    && hasOCRText(payload, 'valid_end', 'valid_date', 'valid_period')
}

export function isRejectedRiderApplication(res?: RiderApplicationResponse): boolean {
  if (!res) return false
  const statusView = buildRiderApplicationStatusView(res.status)
  return (statusView.isDraft || statusView.isRejected) && Boolean(res.reject_reason)
}

export function buildRiderOCRPanelState(displayState: RiderOCRDisplayState): RiderOCRPanelState {
  const identityStatusView = buildMerchantApplicationOCRStatusView(displayState.identity)
  const healthStatusView = buildMerchantApplicationOCRStatusView(displayState.health)

  return {
    identityProcessing: identityStatusView.isPending,
    identityFailed: identityStatusView.isFailed,
    healthProcessing: healthStatusView.isPending,
    healthFailed: healthStatusView.isFailed
  }
}

export function buildRiderOcrDisplayState(
  res: RiderApplicationResponse | undefined,
  uploads: RiderUploadSnapshot
): RiderOCRDisplayState {
  const identityUploaded = Boolean(
    (hasRiderUploadAssetId(res?.id_card_front_asset_id) || hasRiderUploadAssetId(uploads.idFront?.assetId))
    && (hasRiderUploadAssetId(res?.id_card_back_asset_id) || hasRiderUploadAssetId(uploads.idBack?.assetId))
  )
  const healthUploaded = hasRiderUploadAssetId(res?.health_cert_asset_id) || hasRiderUploadAssetId(uploads.healthCert?.assetId)
  const idCardOCR = res?.id_card_ocr as Record<string, unknown> | undefined
  const healthCertOCR = res?.health_cert_ocr as Record<string, unknown> | undefined
  const idCardStatusView = buildMerchantApplicationOCRStatusView(pickOCRText(idCardOCR, 'status'))
  const healthStatusView = buildMerchantApplicationOCRStatusView(pickOCRText(healthCertOCR, 'status'))

  const identityDone = Boolean(
    pickOCRText(idCardOCR, 'name')
    && pickOCRText(idCardOCR, 'id_number', 'id_num')
    && pickOCRText(idCardOCR, 'valid_end', 'valid_date', 'valid_period')
  )
  const healthDone = hasHealthCertKeyFields(healthCertOCR)
  const identityFailed = idCardStatusView.isFailed
  const healthFailed = healthStatusView.isFailed || (healthUploaded && healthStatusView.isReady && !healthDone)

  return {
    identity: identityFailed ? 'failed' : identityDone ? 'done' : identityUploaded ? 'processing' : 'idle',
    health: healthFailed ? 'failed' : healthDone ? 'done' : healthUploaded ? 'processing' : 'idle'
  }
}

export function buildRiderUploadFeedback(
  res: RiderApplicationResponse | undefined,
  uploads: RiderUploadSnapshot
): RiderUploadFeedback {
  const idCardOCR = res?.id_card_ocr as Record<string, unknown> | undefined
  const healthCertOCR = res?.health_cert_ocr as Record<string, unknown> | undefined
  const idStatus = pickOCRText(idCardOCR, 'status')
  const idError = pickOCRText(idCardOCR, 'error')
  const healthStatus = pickOCRText(healthCertOCR, 'status')
  const healthError = pickOCRText(healthCertOCR, 'error')
  const idStatusView = buildMerchantApplicationOCRStatusView(idStatus)
  const healthStatusView = buildMerchantApplicationOCRStatusView(healthStatus)

  const idFrontUploaded = hasRiderUploadAssetId(res?.id_card_front_asset_id) || hasRiderUploadAssetId(uploads.idFront?.assetId)
  const idBackUploaded = hasRiderUploadAssetId(res?.id_card_back_asset_id) || hasRiderUploadAssetId(uploads.idBack?.assetId)
  const healthUploaded = hasRiderUploadAssetId(res?.health_cert_asset_id) || hasRiderUploadAssetId(uploads.healthCert?.assetId)

  const idFrontReady = Boolean(
    pickOCRText(idCardOCR, 'name')
    && pickOCRText(idCardOCR, 'id_number', 'id_num')
  )
  const idBackReady = Boolean(pickOCRText(idCardOCR, 'valid_end', 'valid_date', 'valid_period'))
  const healthReady = hasHealthCertKeyFields(healthCertOCR)
  const healthWritebackFailed = healthUploaded && healthStatusView.isReady && !healthReady

  return {
    idFront: idFrontUploaded
      ? idStatusView.isFailed
        ? createUploadFeedback('error', '识别失败', idError || '请重新上传清晰、完整的身份证人像面')
        : idFrontReady
          ? createUploadFeedback('success', '识别成功', '已识别姓名和身份证号')
          : createUploadFeedback('processing', '证照识别中', '正在识别身份证人像面信息')
      : { ...EMPTY_UPLOAD_FEEDBACK },
    idBack: idBackUploaded
      ? idStatusView.isFailed
        ? createUploadFeedback('error', '识别失败', idError || '请重新上传清晰、完整的身份证国徽面')
        : idBackReady
          ? createUploadFeedback('success', '识别成功', '已识别证件有效期')
          : createUploadFeedback('processing', '证照识别中', '正在识别身份证国徽面信息')
      : { ...EMPTY_UPLOAD_FEEDBACK },
    healthCert: healthUploaded
      ? healthStatusView.isFailed || healthWritebackFailed
        ? createUploadFeedback('error', '识别失败', healthError || '健康证关键字段未识别，请重新上传清晰、无遮挡的健康证照片')
        : healthReady
          ? createUploadFeedback('success', '识别成功', '已识别健康证信息')
          : createUploadFeedback('processing', '证照识别中', '正在识别健康证信息')
      : { ...EMPTY_UPLOAD_FEEDBACK }
  }
}
