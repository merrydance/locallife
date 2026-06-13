import {
  buildRiderApplicationStatusView,
  deleteRiderApplicationDocument,
  ocrRiderHealthCert,
  ocrRiderIdCard,
  type RiderApplicationResponse
} from '../_api/rider-application'
import {
  buildActiveCredentialDisplays,
  buildOnboardingReviewDisplay
} from '../../_main_shared/api/onboarding'
import {
  buildRiderOCRPanelState,
  buildRiderOcrDisplayState,
  buildRiderUploadFeedback,
  createUploadFeedback,
  EMPTY_UPLOAD_FEEDBACK,
  pickOCRText,
  type OCRDisplayStateValue,
  type RiderUploadFeedback,
  type UploadField,
  type UploadFieldValue
} from './rider-register-view'

export type RiderApplicationFormData = {
  realName: string
  phone: string
  idNumber: string
  idValidity: string
  healthCertNo: string
  healthCertDate: string
}

export type RiderApplicationResponseSnapshot = {
  formData: RiderApplicationFormData
  phoneError: string
  currentStep: number
  idFront: UploadFieldValue
  idBack: UploadFieldValue
  healthCert: UploadFieldValue
}

export type RiderDocumentResponseSnapshot = {
  formData: RiderApplicationFormData
  idFront: UploadFieldValue
  idBack: UploadFieldValue
  healthCert: UploadFieldValue
}

export type RiderDocumentOCRWorkflow = {
  field: UploadField
  displayType: 'identity' | 'health'
  feedbackField: keyof RiderUploadFeedback
  startPatch: Record<string, unknown>
  run: () => Promise<RiderApplicationResponse>
}

const RIDER_DOCUMENT_TYPE_MAP: Record<UploadField, 'id_card_front' | 'id_card_back' | 'health_cert'> = {
  idFront: 'id_card_front',
  idBack: 'id_card_back',
  healthCert: 'health_cert'
}

export function buildRiderApplicationResponsePatch(
  res: RiderApplicationResponse,
  snapshot: RiderApplicationResponseSnapshot
): Record<string, unknown> {
  const currentForm = snapshot.formData
  const nextPhone = res.phone || currentForm.phone || ''
  const idCardOCR = res.id_card_ocr as Record<string, unknown> | undefined
  const healthCertOCR = res.health_cert_ocr as Record<string, unknown> | undefined
  const statusView = buildRiderApplicationStatus(res.status)
  const uploadSnapshot = {
    idFront: snapshot.idFront,
    idBack: snapshot.idBack,
    healthCert: snapshot.healthCert
  }
  const ocrDisplayState = buildRiderOcrDisplayState(res, uploadSnapshot)

  return {
    'formData.realName': res.real_name || pickOCRText(idCardOCR, 'name') || currentForm.realName || '',
    'formData.phone': nextPhone,
    'formData.idNumber': pickOCRText(idCardOCR, 'id_number', 'id_num') || currentForm.idNumber || '',
    'formData.idValidity': pickOCRText(idCardOCR, 'valid_end', 'valid_date', 'valid_period') || currentForm.idValidity || '',
    'formData.healthCertNo': pickOCRText(healthCertOCR, 'cert_number', 'certificate_number', 'certificate') || currentForm.healthCertNo || '',
    'formData.healthCertDate': pickOCRText(healthCertOCR, 'valid_end', 'valid_date', 'valid_period') || currentForm.healthCertDate || '',
    phoneError: nextPhone.trim() ? '' : snapshot.phoneError,
    currentStep: statusView.isSubmitted ? 4 : snapshot.currentStep,
    applicationStatus: res.status,
    riderStatusView: statusView,
    isSubmitting: false,
    idFront: { url: '', assetId: res.id_card_front_asset_id },
    idBack: { url: '', assetId: res.id_card_back_asset_id },
    healthCert: { url: '', assetId: res.health_cert_asset_id },
    ocrDisplayState,
    ocrPanelState: buildRiderOCRPanelState(ocrDisplayState),
    uploadFeedback: buildRiderUploadFeedback(res, uploadSnapshot),
    reviewDisplay: buildOnboardingReviewDisplay(res.review_summary, res.status),
    activeCredentialDisplays: buildActiveCredentialDisplays(res.active_credentials)
  }
}

export function buildRiderDocumentResponsePatch(
  field: UploadField,
  res: RiderApplicationResponse,
  snapshot: RiderDocumentResponseSnapshot
): Record<string, unknown> {
  const currentForm = snapshot.formData
  const idCardOCR = res.id_card_ocr as Record<string, unknown> | undefined
  const healthCertOCR = res.health_cert_ocr as Record<string, unknown> | undefined
  const mergedRes: RiderApplicationResponse = {
    ...res,
    id_card_front_asset_id: field === 'idFront' ? res.id_card_front_asset_id : snapshot.idFront.assetId,
    id_card_back_asset_id: field === 'idBack' ? res.id_card_back_asset_id : snapshot.idBack.assetId,
    health_cert_asset_id: field === 'healthCert' ? res.health_cert_asset_id : snapshot.healthCert.assetId
  }
  const uploadSnapshot = {
    idFront: snapshot.idFront,
    idBack: snapshot.idBack,
    healthCert: snapshot.healthCert
  }
  const nextOCRDisplayState = buildRiderOcrDisplayState(mergedRes, uploadSnapshot)
  const nextData: Record<string, unknown> = {
    ocrDisplayState: nextOCRDisplayState,
    ocrPanelState: buildRiderOCRPanelState(nextOCRDisplayState),
    uploadFeedback: buildRiderUploadFeedback(mergedRes, uploadSnapshot)
  }

  if (field === 'idFront') {
    nextData['formData.realName'] = res.real_name || pickOCRText(idCardOCR, 'name') || currentForm.realName || ''
    nextData['formData.idNumber'] = pickOCRText(idCardOCR, 'id_number', 'id_num') || currentForm.idNumber || ''
    nextData.idFront = { url: '', assetId: res.id_card_front_asset_id }
  }

  if (field === 'idBack') {
    nextData['formData.idValidity'] = pickOCRText(idCardOCR, 'valid_end', 'valid_date', 'valid_period') || currentForm.idValidity || ''
    nextData.idBack = { url: '', assetId: res.id_card_back_asset_id }
  }

  if (field === 'healthCert') {
    nextData['formData.healthCertNo'] = pickOCRText(healthCertOCR, 'cert_number', 'certificate_number', 'certificate') || currentForm.healthCertNo || ''
    nextData['formData.healthCertDate'] = pickOCRText(healthCertOCR, 'valid_end', 'valid_date', 'valid_period') || currentForm.healthCertDate || ''
    nextData.healthCert = { url: '', assetId: res.health_cert_asset_id }
  }

  return nextData
}

export function createRiderDocumentOCRWorkflow(field: UploadField, path: string): RiderDocumentOCRWorkflow {
  if (field === 'idFront') {
    return {
      field,
      displayType: 'identity',
      feedbackField: 'idFront',
      startPatch: {
        'idFront.url': path,
        'idFront.rawUrl': path,
        'idFront.assetId': undefined,
        'ocrDisplayState.identity': 'processing',
        'uploadFeedback.idFront': createUploadFeedback('processing', '证照识别中', '请稍候，识别结果会显示在当前卡片中')
      },
      run: () => ocrRiderIdCard(path, 'Front')
    }
  }

  if (field === 'idBack') {
    return {
      field,
      displayType: 'identity',
      feedbackField: 'idBack',
      startPatch: {
        'idBack.url': path,
        'idBack.rawUrl': path,
        'idBack.assetId': undefined,
        'ocrDisplayState.identity': 'processing',
        'uploadFeedback.idBack': createUploadFeedback('processing', '证照识别中', '请稍候，识别结果会显示在当前卡片中')
      },
      run: () => ocrRiderIdCard(path, 'Back')
    }
  }

  return {
    field,
    displayType: 'health',
    feedbackField: 'healthCert',
    startPatch: {
      'healthCert.url': path,
      'healthCert.rawUrl': path,
      'healthCert.assetId': undefined,
      'ocrDisplayState.health': 'processing',
      'uploadFeedback.healthCert': createUploadFeedback('processing', '证照识别中', '请稍候，识别结果会显示在当前卡片中')
    },
    run: () => ocrRiderHealthCert(path)
  }
}

export async function deleteRiderDocumentByField(field: UploadField) {
  return deleteRiderApplicationDocument(RIDER_DOCUMENT_TYPE_MAP[field])
}

export function buildRiderDocumentDeleteLocalPatch(field: UploadField): Record<string, unknown> {
  switch (field) {
    case 'idFront':
      return {
        idFront: { url: '', rawUrl: '', assetId: undefined },
        'formData.realName': '',
        'formData.idNumber': '',
        'uploadFeedback.idFront': { ...EMPTY_UPLOAD_FEEDBACK }
      }
    case 'idBack':
      return {
        idBack: { url: '', rawUrl: '', assetId: undefined },
        'formData.idValidity': '',
        'uploadFeedback.idBack': { ...EMPTY_UPLOAD_FEEDBACK }
      }
    default:
      return {
        healthCert: { url: '', rawUrl: '', assetId: undefined },
        'formData.healthCertNo': '',
        'formData.healthCertDate': '',
        'uploadFeedback.healthCert': { ...EMPTY_UPLOAD_FEEDBACK }
      }
  }
}

export function buildRiderDocumentOCRFailurePatch(
  displayType: 'identity' | 'health',
  feedbackField: keyof RiderUploadFeedback,
  message: string
): Record<string, unknown> {
  return {
    [`ocrDisplayState.${displayType}`]: 'failed' as OCRDisplayStateValue,
    [`uploadFeedback.${String(feedbackField)}`]: createUploadFeedback('error', '识别失败', message)
  }
}

function buildRiderApplicationStatus(status: RiderApplicationResponse['status']) {
  return buildRiderApplicationStatusView(status)
}
