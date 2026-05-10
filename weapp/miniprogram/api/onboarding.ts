import { request } from '../utils/request'
import { uploadMedia, type MediaUploadResult } from '../utils/media'
import type { AgreementConsentPayload } from './agreement-consent'
import { waitForOCRJobTerminal, waitForOCRWriteback } from './ocr-jobs'
import { AppError, ErrorType } from '../utils/error-handler'
import type { StatusTagTheme } from '../utils/status-tag'

const OCR_ENQUEUE_RETRY_DELAY_MS = 1500
const OCR_ENQUEUE_MAX_ATTEMPTS = 3
const IMAGE_MODERATION_PENDING_MARKERS = [
  'image moderation is pending',
  '内容审核中',
  '多媒体内容安全审查',
  '安全审查系统拦截'
]
const OCR_BLOCKED_MESSAGE = '图片被微信多媒体内容安全审查系统拦截，请更换图片再试'

// ==================== OCR Status Types ====================

export type OCRStatus = 'pending' | 'processing' | 'done' | 'failed'

export interface BaseOCRData {
  status: OCRStatus
  error?: string
  queued_at?: string
  started_at?: string
  ocr_at?: string
}

export interface BusinessLicenseOCRData extends BaseOCRData {
  reg_num?: string
  enterprise_name?: string
  legal_representative?: string
  type_of_enterprise?: string
  address?: string
  business_scope?: string
  registered_capital?: string
  valid_period?: string
  credit_code?: string
}

export interface FoodPermitOCRData extends BaseOCRData {
  raw_text?: string
  permit_no?: string
  company_name?: string
  valid_from?: string
  valid_to?: string
}

export interface IDCardOCRData extends BaseOCRData {
  // Front
  name?: string
  id_number?: string
  gender?: string
  nation?: string
  address?: string
  // Back
  valid_date?: string
}

// ==================== Application Response Types ====================

export type ApplicationStatus = 'draft' | 'submitted' | 'approved' | 'rejected'

export interface OnboardingReviewSummary {
  run_id: number
  stage: string
  outcome: string
  reason_code: string
  reason_message?: string
  rule_hits?: string[]
  ocr_job_refs?: number[]
  created_at: string
}

export interface ActiveCredentialSummary {
  document_type: string
  expires_at?: string
  days_until_expiry?: number
  last_reminded_at?: string
  suspended: boolean
  suspended_at?: string
  resumed_at?: string
}

export interface OnboardingReviewDisplay {
  visible: boolean
  title: string
  description: string
  statusText: string
  statusTheme: StatusTagTheme
  reasonText: string
  metaText: string
  ruleText: string
}

export interface ActiveCredentialDisplay {
  key: string
  label: string
  statusText: string
  statusTheme: StatusTagTheme
  expiryText: string
  detailText: string
}

export function buildOnboardingReviewDisplay(
  summary?: OnboardingReviewSummary | null,
  applicationStatus?: ApplicationStatus | string
): OnboardingReviewDisplay {
  const outcome = String(summary?.outcome || '').trim().toLowerCase()
  const stage = String(summary?.stage || '').trim().toLowerCase()
  const status = String(applicationStatus || '').trim().toLowerCase()
  const hasSummary = Boolean(summary?.run_id || outcome || stage || summary?.reason_code)

  if (!hasSummary && status !== 'submitted') {
    return emptyOnboardingReviewDisplay()
  }

  if (outcome === 'approved') {
    return buildReviewDisplay(summary, {
      title: '审核通过',
      description: '资质已核验通过，证照有效期将持续跟踪。',
      statusText: '已通过',
      statusTheme: 'success'
    })
  }

  if (outcome === 'needs_resubmit') {
    return buildReviewDisplay(summary, {
      title: '资料需修改',
      description: summary?.reason_message || '资料核验未通过，请根据提示修改后重新提交。',
      statusText: '待修改',
      statusTheme: 'danger'
    })
  }

  if (outcome === 'rejected') {
    return buildReviewDisplay(summary, {
      title: '审核未通过',
      description: summary?.reason_message || '资料未通过平台核验，请修改后重新提交。',
      statusText: '未通过',
      statusTheme: 'danger'
    })
  }

  if (outcome === 'needs_manual' || stage === 'manual') {
    return buildReviewDisplay(summary, {
      title: '人工复核中',
      description: summary?.reason_message || '系统已转入人工复核，结果更新后会同步到当前页面。',
      statusText: '复核中',
      statusTheme: 'warning'
    })
  }

  return buildReviewDisplay(summary, {
    title: '审核已提交',
    description: '系统已收到申请，审核完成后会更新状态。',
    statusText: '审核中',
    statusTheme: 'warning'
  })
}

export function buildActiveCredentialDisplays(credentials?: ActiveCredentialSummary[] | null): ActiveCredentialDisplay[] {
  if (!credentials?.length) {
    return []
  }

  return credentials.map((credential) => {
    const label = credentialDocumentLabel(credential.document_type)
    const days = credential.days_until_expiry
    const hasDays = typeof days === 'number' && Number.isFinite(days)
    const expiresDate = formatDate(credential.expires_at)

    if (credential.suspended) {
      return {
        key: credential.document_type,
        label,
        statusText: '已暂停',
        statusTheme: 'danger',
        expiryText: expiresDate ? `有效期至 ${expiresDate}` : '有效期需复核',
        detailText: '该资质已暂停，请完成复审后恢复。'
      }
    }

    if (hasDays && days < 0) {
      return {
        key: credential.document_type,
        label,
        statusText: '已过期',
        statusTheme: 'danger',
        expiryText: expiresDate ? `已于 ${expiresDate} 到期` : '证照已过期',
        detailText: '请尽快更新证照并提交复审。'
      }
    }

    if (hasDays && days <= 30) {
      return {
        key: credential.document_type,
        label,
        statusText: '临近到期',
        statusTheme: 'warning',
        expiryText: days === 0 ? '今天到期' : `还有 ${days} 天到期`,
        detailText: '请在到期前更新证照，避免影响服务。'
      }
    }

    return {
      key: credential.document_type,
      label,
      statusText: '有效',
      statusTheme: 'success',
      expiryText: expiresDate ? `有效期至 ${expiresDate}` : '长期有效',
      detailText: '资质状态正常。'
    }
  })
}

function emptyOnboardingReviewDisplay(): OnboardingReviewDisplay {
  return {
    visible: false,
    title: '',
    description: '',
    statusText: '',
    statusTheme: 'default',
    reasonText: '',
    metaText: '',
    ruleText: ''
  }
}

function buildReviewDisplay(
  summary: OnboardingReviewSummary | null | undefined,
  display: Pick<OnboardingReviewDisplay, 'title' | 'description' | 'statusText' | 'statusTheme'>
): OnboardingReviewDisplay {
  const ruleCount = summary?.rule_hits?.length || 0
  return {
    visible: true,
    ...display,
    reasonText: summary?.reason_message && summary.reason_message !== display.description ? summary.reason_message : '',
    metaText: summary?.created_at ? `提交于 ${formatDateTime(summary.created_at)}` : '',
    ruleText: ruleCount > 0 ? `已完成 ${ruleCount} 项规则校验` : ''
  }
}

function credentialDocumentLabel(documentType: string) {
  switch (documentType) {
    case 'business_license':
      return '营业执照'
    case 'food_permit':
      return '食品经营许可证'
    case 'health_cert':
      return '健康证'
    default:
      return '资质证照'
  }
}

function formatDate(value?: string) {
  if (!value) return ''
  return value.replace('T', ' ').slice(0, 10)
}

function formatDateTime(value?: string) {
  if (!value) return ''
  return value.replace('T', ' ').slice(0, 16)
}

export interface MerchantApplicationStatusView {
  statusCode: string
  isDraft: boolean
  isSubmitted: boolean
  isApproved: boolean
  isRejected: boolean
  tagText: string
  tagTheme: StatusTagTheme
  badgeText: string
  guideText: string
  editTip: string
  canEdit: boolean
  canSubmit: boolean
  canReset: boolean
}

export interface MerchantApplicationOCRStatusView {
  statusCode: string
  text: string
  isPending: boolean
  isReady: boolean
  isFailed: boolean
}

export function buildMerchantApplicationStatusView(status?: ApplicationStatus | string): MerchantApplicationStatusView {
  const normalizedStatus = String(status || 'draft').trim().toLowerCase() || 'draft'

  switch (normalizedStatus) {
    case 'submitted':
      return {
        statusCode: normalizedStatus,
        isDraft: false,
        isSubmitted: true,
        isApproved: false,
        isRejected: false,
        tagText: '审核中',
        tagTheme: 'warning',
        badgeText: '审核',
        guideText: '申请已提交，审核通过后继续完成宝付结算账户开户。',
        editTip: '当前状态不可编辑',
        canEdit: false,
        canSubmit: false,
        canReset: false
      }
    case 'approved':
      return {
        statusCode: normalizedStatus,
        isDraft: false,
        isSubmitted: false,
        isApproved: true,
        isRejected: false,
        tagText: '已通过',
        tagTheme: 'success',
        badgeText: '通过',
        guideText: '主体已通过，可继续完成宝付结算账户开户和签约。',
        editTip: '已通过申请如需修改，保存或上传后会自动回到草稿状态',
        canEdit: true,
        canSubmit: true,
        canReset: false
      }
    case 'rejected':
      return {
        statusCode: normalizedStatus,
        isDraft: false,
        isSubmitted: false,
        isApproved: false,
        isRejected: true,
        tagText: '已驳回',
        tagTheme: 'danger',
        badgeText: '驳回',
        guideText: '申请已驳回，请按驳回原因修改后重新提交。',
        editTip: '可保存草稿，提交前请确认无误',
        canEdit: true,
        canSubmit: true,
        canReset: true
      }
    default:
      return {
        statusCode: 'draft',
        isDraft: true,
        isSubmitted: false,
        isApproved: false,
        isRejected: false,
        tagText: '草稿中',
        tagTheme: 'default',
        badgeText: '草稿',
        guideText: '填写主体资料并上传证照，确认定位后提交审核。',
        editTip: '可保存草稿，提交前请确认无误',
        canEdit: true,
        canSubmit: true,
        canReset: false
      }
  }
}

export function buildMerchantApplicationOCRStatusView(status?: OCRStatus | string): MerchantApplicationOCRStatusView {
  const normalizedStatus = String(status || '').trim().toLowerCase()

  switch (normalizedStatus) {
    case 'done':
      return {
        statusCode: normalizedStatus,
        text: '识别完成',
        isPending: false,
        isReady: true,
        isFailed: false
      }
    case 'processing':
      return {
        statusCode: normalizedStatus,
        text: '识别中',
        isPending: true,
        isReady: false,
        isFailed: false
      }
    case 'failed':
      return {
        statusCode: normalizedStatus,
        text: '识别失败',
        isPending: false,
        isReady: false,
        isFailed: true
      }
    case 'pending':
      return {
        statusCode: normalizedStatus,
        text: '待识别',
        isPending: true,
        isReady: false,
        isFailed: false
      }
    default:
      return {
        statusCode: '',
        text: '未上传',
        isPending: false,
        isReady: false,
        isFailed: false
      }
  }
}

export function buildMerchantApplicationOCRNoticeMessage(statuses: Array<OCRStatus | string | undefined>) {
  const hasPendingStatus = statuses.some((status) => buildMerchantApplicationOCRStatusView(status).isPending)
  if (!hasPendingStatus) {
    return ''
  }

  return '部分证照仍在识别中，请等待本次识别完成后再提交审核。'
}

export function buildMerchantApplicationOCRSubmitBlockMessage(checks: Array<{ label: string, status?: OCRStatus | string }>) {
  for (const item of checks) {
    const statusView = buildMerchantApplicationOCRStatusView(item.status)
    if (statusView.isReady) {
      continue
    }
    if (statusView.isPending) {
      return `${item.label}仍在识别中，请等待识别完成后再提交`
    }
    if (statusView.isFailed) {
      return `${item.label}识别失败，请提供更清晰更规整的图片重试`
    }
    return `${item.label}识别结果未就绪，请重新上传后再试`
  }

  return ''
}

export interface MerchantApplicationDraftResponse {
  id: number
  user_id: number
  merchant_name: string
  contact_phone: string
  business_address: string
  longitude: string | null
  latitude: string | null
  region_id: number | null
  business_license_media_asset_id?: number | null
  business_license_url?: string | null
  business_license_number: string
  business_scope: string | null
  business_license_ocr: BusinessLicenseOCRData | null
  food_permit_media_asset_id?: number | null
  food_permit_url?: string | null
  food_permit_ocr: FoodPermitOCRData | null
  legal_person_name: string
  legal_person_id_number: string
  id_card_front_media_asset_id?: number | null
  id_card_back_media_asset_id?: number | null
  id_card_front_ocr: IDCardOCRData | null
  id_card_back_ocr: IDCardOCRData | null
  storefront_images?: string[] | null  // 门头照 URL 数组，最多3张
  environment_images?: string[] | null // 环境照 URL 数组，最多5张
  review_summary?: OnboardingReviewSummary | null
  active_credentials?: ActiveCredentialSummary[] | null
  status: ApplicationStatus
  reject_reason: string | null
  created_at: string
  updated_at: string
}

interface OCRJobResponse {
  ocr_job_id: number
  status: string
}

export type MerchantApplicationOCRDocumentType = 'business_license' | 'food_permit' | 'id_card'

export type MerchantApplicationOCRSubmissionState = 'queued'

export interface MerchantApplicationOCRSubmissionResult {
  draft: MerchantApplicationDraftResponse
  mediaId: number
  state: MerchantApplicationOCRSubmissionState
}

interface MerchantApplicationOCRRequestOptions {
  maxAttempts?: number
  retryDelayMs?: number
}

function buildMerchantDraftOCRCheck(
  documentType: MerchantApplicationOCRDocumentType,
  side?: 'Front' | 'Back'
) {
  return (draft: MerchantApplicationDraftResponse) => {
    if (documentType === 'business_license') {
      const status = draft.business_license_ocr?.status || ''
      const error = draft.business_license_ocr?.error || ''
      return {
        ready: status === 'done'
          || !!String(draft.business_license_number || '').trim()
          || !!String(draft.business_license_ocr?.enterprise_name || '').trim()
          || !!String(draft.business_license_ocr?.credit_code || '').trim()
          || !!String(draft.business_license_ocr?.reg_num || '').trim(),
        failed: status === 'failed',
        errorMessage: error
      }
    }

    if (documentType === 'food_permit') {
      const status = draft.food_permit_ocr?.status || ''
      const error = draft.food_permit_ocr?.error || ''
      return {
        ready: status === 'done'
          || !!String(draft.food_permit_ocr?.valid_to || '').trim()
          || !!String(draft.food_permit_ocr?.permit_no || '').trim()
          || !!String(draft.food_permit_ocr?.company_name || '').trim()
          || !!String(draft.food_permit_ocr?.raw_text || '').trim(),
        failed: status === 'failed',
        errorMessage: error
      }
    }

    if (side === 'Back') {
      const status = draft.id_card_back_ocr?.status || ''
      const error = draft.id_card_back_ocr?.error || ''
      return {
        ready: status === 'done' || !!String(draft.id_card_back_ocr?.valid_date || '').trim(),
        failed: status === 'failed',
        errorMessage: error
      }
    }

    const status = draft.id_card_front_ocr?.status || ''
    const error = draft.id_card_front_ocr?.error || ''
    return {
      ready: status === 'done'
        || !!String(draft.legal_person_name || '').trim()
        || !!String(draft.legal_person_id_number || '').trim()
        || !!String(draft.id_card_front_ocr?.name || '').trim()
        || !!String(draft.id_card_front_ocr?.id_number || '').trim(),
      failed: status === 'failed',
      errorMessage: error
    }
  }
}

// 图片上传响应
export interface UploadImageResponse {
  image_url: string
}

// 更新图片请求
export interface UpdateMerchantImagesRequest {
  storefront_images?: string[]
  environment_images?: string[]
}

// ==================== Request Types ====================

// 兼容当前商户入驻基础信息更新请求定义。
export interface UpdateMerchantBasicInfoRequest {
  merchant_name?: string
  contact_phone?: string
  business_address?: string
  longitude?: string
  latitude?: string
  region_id?: number
  business_license_number?: string
  business_scope?: string
  legal_person_name?: string
  legal_person_id_number?: string
  storefront_images?: string[] // 虽然有单独接口，但API也可能支持
  environment_images?: string[]
}

// ==================== API Methods ====================

/**
 * 获取或创建商户入驻申请草稿
 * GET /v1/merchant/application
 * - 200: 返回现有草稿
 * - 201: 创建新草稿并返回
 * - 409: 已存在 submitted/approved
 */
export function getMerchantApplication() {
  return request<MerchantApplicationDraftResponse>({
    url: '/v1/merchant/application',
    method: 'GET'
  })
}

/**
 * 更新基础信息（草稿可编辑）
 * PUT /v1/merchant/application/basic
 */
export function updateMerchantBasicInfo(data: UpdateMerchantBasicInfoRequest) {
  return request<MerchantApplicationDraftResponse>({
    url: '/v1/merchant/application/basic',
    method: 'PUT',
    data
  })
}

async function enqueueMerchantApplicationOCR(
  mediaId: number,
  documentType: MerchantApplicationOCRDocumentType,
  side?: 'Front' | 'Back',
  options?: MerchantApplicationOCRRequestOptions
): Promise<MerchantApplicationOCRSubmissionResult> {
  const draft = await getMerchantApplication()

  const maxAttempts = Math.max(1, options?.maxAttempts ?? OCR_ENQUEUE_MAX_ATTEMPTS)
  const retryDelayMs = Math.max(0, options?.retryDelayMs ?? OCR_ENQUEUE_RETRY_DELAY_MS)

  for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
    try {
      const job = await request<OCRJobResponse>({
        url: '/v1/ocr/jobs',
        method: 'POST',
        data: {
          document_type: documentType,
          media_asset_id: mediaId,
          owner_type: 'merchant_application',
          owner_id: draft.id,
          side: side ? side.toLowerCase() : undefined
        }
      })

      await waitForOCRJobTerminal(job.ocr_job_id, {
        maxAttempts: 20,
        intervalMs: 1000
      })
      const latestDraft = await waitForOCRWriteback(
        getMerchantApplication,
        buildMerchantDraftOCRCheck(documentType, side),
        {
          maxAttempts: 20,
          intervalMs: 1000
        }
      )

      return {
        draft: latestDraft,
        mediaId,
        state: 'queued' as const
      }
    } catch (error) {
      if (!isImageModerationPendingError(error)) {
        throw error
      }

      if (attempt >= maxAttempts - 1) {
        throw new AppError({
          type: ErrorType.BUSINESS,
          message: 'OCR enqueue blocked by media moderation',
          userMessage: OCR_BLOCKED_MESSAGE
        }, error)
      }

      await sleep(retryDelayMs)
    }
  }

  throw new AppError({
    type: ErrorType.BUSINESS,
    message: 'OCR enqueue blocked by media moderation',
    userMessage: OCR_BLOCKED_MESSAGE
  })
}

function sleep(ms: number) {
  return new Promise<void>((resolve) => {
    setTimeout(resolve, ms)
  })
}

function extractErrorMessage(error: unknown) {
  if (!error || typeof error !== 'object') return ''

  const maybeError = error as {
    userMessage?: string
    message?: string
    originalError?: { message?: string }
  }

  return String(maybeError.userMessage || maybeError.message || maybeError.originalError?.message || '')
}

function isImageModerationPendingError(error: unknown) {
  const message = extractErrorMessage(error).toLowerCase()
  return IMAGE_MODERATION_PENDING_MARKERS.some((marker) => message.includes(marker))
}

export function enqueueMerchantApplicationOCRForMedia(
  mediaId: number,
  documentType: MerchantApplicationOCRDocumentType,
  side?: 'Front' | 'Back',
  options?: MerchantApplicationOCRRequestOptions
) {
  return enqueueMerchantApplicationOCR(mediaId, documentType, side, options)
}

/**
 * 营业执照 OCR（异步）
 * 统一走 POST /v1/ocr/jobs，owner_type=merchant_application，document_type=business_license
 * 若传 filePath：先上传到媒体服务，再以 media_asset_id 创建 OCR 作业
 */
export async function ocrBusinessLicense(filePath?: string): Promise<MerchantApplicationOCRSubmissionResult> {
  if (!filePath) {
    throw new Error('missing filePath for merchant business license OCR')
  }
  const { mediaId } = await uploadMedia(filePath, {
    businessType: 'merchant',
    mediaCategory: 'business_license'
  })
  return enqueueMerchantApplicationOCR(mediaId, 'business_license')
}

/**
 * 食品经营许可证 OCR（异步）
 * 统一走 POST /v1/ocr/jobs，owner_type=merchant_application，document_type=food_permit
 */
export async function ocrFoodPermit(filePath?: string): Promise<MerchantApplicationOCRSubmissionResult> {
  if (!filePath) {
    throw new Error('missing filePath for merchant food permit OCR')
  }
  const { mediaId } = await uploadMedia(filePath, {
    businessType: 'merchant',
    mediaCategory: 'food_permit'
  })
  return enqueueMerchantApplicationOCR(mediaId, 'food_permit')
}

/**
 * 身份证 OCR（异步）
 * 统一走 POST /v1/ocr/jobs，owner_type=merchant_application，document_type=id_card
 * @param side 'Front' 或 'Back'
 */
export async function ocrIdCard(filePath: string | undefined, side: 'Front' | 'Back'): Promise<MerchantApplicationOCRSubmissionResult> {
  if (!filePath) {
    throw new Error('missing filePath for merchant id card OCR')
  }
  const mediaCategory = side === 'Front' ? 'id_card_front' : 'id_card_back'
  const { mediaId } = await uploadMedia(filePath, {
    businessType: 'merchant',
    mediaCategory
  })
  return enqueueMerchantApplicationOCR(mediaId, 'id_card', side)
}

/**
 * 提交申请（自动审核）
 * POST /v1/merchant/application/submit
 * 无请求体，返回 approved 或 rejected
 */
export function submitMerchantApplication(data?: AgreementConsentPayload) {
  return request<MerchantApplicationDraftResponse>({
    url: '/v1/merchant/application/submit',
    method: 'POST',
    data
  })
}

/**
 * 获取当前用户最新申请（用于 submitted 后轮询）
 * GET /v1/merchants/applications/me
 */
export function getMyApplication() {
  return request<MerchantApplicationDraftResponse>({
    url: '/v1/merchants/applications/me',
    method: 'GET'
  })
}

/**
 * 重置被拒绝申请为草稿
 * POST /v1/merchant/application/reset
 */
export function resetMerchantApplication() {
  return request<MerchantApplicationDraftResponse>({
    url: '/v1/merchant/application/reset',
    method: 'POST'
  })
}

/**
 * 上传商户图片文件（Logo、门头照、环境照）
 * 媒体服务三步流程
 * @param filePath 本地文件路径
 * @param category 'logo' | 'storefront' | 'environment'
 * @returns { mediaId, displayUrl, urls }
 */
export function uploadMerchantImage(
  filePath: string,
  category: 'logo' | 'storefront' | 'environment'
): Promise<MediaUploadResult> {
  const mediaCategory =
    category === 'logo' ? 'logo'
    : category === 'storefront' ? 'storefront'
    : 'environment'
  return uploadMedia(filePath, {
    businessType: 'merchant',
    mediaCategory
  })
}

/**
 * 保存门头照/环境照 URL 到草稿
 * PUT /v1/merchant/application/images
 */
export function updateMerchantImages(data: UpdateMerchantImagesRequest) {
  return request<MerchantApplicationDraftResponse>({
    url: '/v1/merchant/application/images',
    method: 'PUT',
    data
  })
}

export function deleteMediaAsset(mediaId: number) {
  return request<void>({
    url: `/v1/media/${mediaId}`,
    method: 'DELETE'
  })
}

export function deleteMerchantApplicationDocument(
  documentType: 'business_license' | 'food_permit' | 'id_card_front' | 'id_card_back'
) {
  return request<MerchantApplicationDraftResponse>({
    url: `/v1/merchant/application/documents/${documentType}`,
    method: 'DELETE'
  })
}

// 更新商户店铺图片请求（已入驻商户使用）
export interface UpdateShopImagesRequest {
  storefront_images?: string[]
  environment_images?: string[]
}

// 更新商户店铺图片响应
export interface UpdateShopImagesResponse {
  storefront_images: string[] | null
  environment_images: string[] | null
}
interface MediaAssetDetailResponse {
  id: number
  upload_status: string
  moderation_status: string
  urls?: {
    thumb?: string
    card?: string
    detail?: string
    original?: string
  } | null
}

export async function waitForPublicMediaDisplayUrl(
  mediaId: number,
  options?: { maxAttempts?: number, intervalMs?: number }
): Promise<string> {
  const maxAttempts = Math.max(1, options?.maxAttempts ?? 8)
  const intervalMs = Math.max(300, options?.intervalMs ?? 1500)

  for (let attempt = 0; attempt < maxAttempts; attempt++) {
    const asset = await request<MediaAssetDetailResponse>({
      url: `/v1/media/${mediaId}`,
      method: 'GET'
    })

    const displayUrl = asset.urls?.card || asset.urls?.detail || asset.urls?.original || ''
    if (displayUrl) {
      return displayUrl
    }

    if (asset.moderation_status && asset.moderation_status !== 'pending') {
      return ''
    }

    if (attempt < maxAttempts - 1) {
      await sleep(intervalMs)
    }
  }

  return ''
}

/**
 * 更新已入驻商户的门头照/环境照
 * PATCH /v1/merchants/me/shop-images
 */
export function updateShopImages(data: UpdateShopImagesRequest) {
  return request<UpdateShopImagesResponse>({
    url: '/v1/merchants/me/shop-images',
    method: 'PATCH',
    data
  })
}

// ==================== Rider & Other Types (Preserved) ====================

export interface ApplyRiderRequest {
  id_card_no: string
  phone: string
  real_name: string
  vehicle_type?: string
  address?: string
  gender?: string
  id_card_front_images?: string[]
  id_card_back_images?: string[]
  health_certificate_images?: string[]
}

export function submitRiderApplication(data: ApplyRiderRequest) {
  return request<void>({
    url: '/onboarding/rider',
    method: 'POST',
    data
  })
}

export interface DepositRequest extends Record<string, unknown> {
  amount: number
  remark?: string
}

export interface DepositResponse {
  amount?: number
  balance_after?: number
  created_at?: string
  id?: number
  remark?: string
  rider_id?: number
  type?: string
}
