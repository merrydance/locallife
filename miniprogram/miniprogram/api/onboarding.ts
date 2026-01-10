import { supabase, supabaseRequest } from '../services/supabase'
import { uploadFile } from '../utils/request'

// ==================== OCR Status Types (Preserved for compatibility) ====================

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
  name?: string
  id_number?: string
  gender?: string
  nation?: string
  address?: string
  valid_date?: string
}

// ==================== Application Response Types ====================

export type ApplicationStatus = 'draft' | 'submitted' | 'approved' | 'rejected'

export interface MerchantApplicationDraftResponse {
  id: string
  user_id: string
  merchant_name: string
  contact_phone: string
  business_address: string
  longitude: string | null
  latitude: string | null
  region_id: number | null
  business_license_image_url: string
  business_license_number: string
  business_scope: string | null
  business_license_ocr: BusinessLicenseOCRData | null
  food_permit_url: string | null
  food_permit_ocr: FoodPermitOCRData | null
  legal_person_name: string
  legal_person_id_number: string
  legal_person_id_front_url: string
  legal_person_id_back_url: string
  id_card_front_ocr: IDCardOCRData | null
  id_card_back_ocr: IDCardOCRData | null
  storefront_images?: string[] | null
  environment_images?: string[] | null
  status: ApplicationStatus
  reject_reason: string | null
  target_merchant_id?: string | null
  onboarding_role: 'owner' | 'manager'
  created_at: string
  updated_at: string
}

export interface UploadImageResponse {
  image_url: string
}

export interface UpdateMerchantImagesRequest {
  storefront_images?: string[]
  environment_images?: string[]
}

export interface UpdateMerchantBasicInfoRequest {
  merchant_name?: string
  contact_phone?: string
  business_address?: string
  longitude?: string
  latitude?: string
  region_id?: number
  business_license_number?: string
  business_license_image_url?: string
  business_scope?: string
  legal_person_name?: string
  legal_person_id_number?: string
  legal_person_id_front_url?: string
  legal_person_id_back_url?: string
  food_permit_url?: string
  storefront_images?: string[]
  environment_images?: string[]
  onboarding_role?: 'owner' | 'manager'
}

// ==================== API Methods ====================

/**
 * 获取特定申请单或查询当前状态以决定下一步
 * @param appId 如果传了 ID，则返回该特定申请单
 * 逻辑：
 * 1. 如果传了 appId，直接按 id 查；
 * 2. 如果没传 appId：
 *    - 优先返回最近的一条 'draft' 或 'rejected' 状态的申请；
 *    - 如果只有 'approved' 或 'submitted'，则返回最新的一条。
 *    - 如果没有任何记录，则返回 null (不主动创建)
 */
export async function getMerchantApplication(appId?: string): Promise<MerchantApplicationDraftResponse | null> {
  let query = supabase.from<MerchantApplicationDraftResponse>('merchant_applications').select('*')
  
  if (appId) {
    const { data, error } = await query.eq('id', appId).single()
    if (error) throw error
    if (!data) throw new Error('Application not found')
    return data
  }

  // 查找用户的所有申请
  const { data, error } = await query.order('updated_at', { ascending: false })
  
  if (error) throw error
  
  // 优先寻找草稿或被拒绝的，方便继续编辑
  const activeDraft = data?.find(item => item.status === 'draft' || item.status === 'rejected')
  if (activeDraft) return activeDraft

  // 如果没有草稿，且有记录，返回最新的一条
  if (data && data.length > 0) return data[0]

  // 如果完全没有记录，则不在此处创建，允许前端决定时机（如上传图片或点击下一步时）
  return null
}

/**
 * 列出当前用户的所有商户申请
 */
export async function listMerchantApplications(): Promise<MerchantApplicationDraftResponse[]> {
  const { data, error } = await supabase.from<MerchantApplicationDraftResponse>('merchant_applications')
    .select('*')
    .order('created_at', { ascending: false })
  
  if (error) throw error
  return data || []
}

/**
 * 强制创建一个新的申请草稿 (针对连锁店多店申请或首次入驻)
 */
export async function createNewMerchantApplication(): Promise<MerchantApplicationDraftResponse> {
  const { data: newData, error: createError } = await supabase.from<MerchantApplicationDraftResponse>('merchant_applications').insert({
    status: 'draft',
    onboarding_role: 'owner', // 默认店主
    merchant_name: '',
    business_license_number: '',
    business_license_image_url: '',
    legal_person_name: '',
    legal_person_id_number: '',
    legal_person_id_front_url: '',
    legal_person_id_back_url: '',
    contact_phone: '',
    business_address: ''
  })
  if (createError) throw createError
  if (!newData || newData.length === 0) throw new Error('Failed to create application draft')
  return newData[0]
}

/**
 * 更新基础信息
 */
export async function updateMerchantBasicInfo(data: UpdateMerchantBasicInfoRequest): Promise<MerchantApplicationDraftResponse> {
  let draft = await getMerchantApplication()
  if (!draft) {
    draft = await createNewMerchantApplication()
  }
  const { data: updatedData, error } = await supabase.from<MerchantApplicationDraftResponse>('merchant_applications')
    .update(data as any) // 临时 bypass Partial 检查
    .eq('id', draft.id)
  
  if (error) throw error
  if (!updatedData || updatedData.length === 0) throw new Error('Update failed')
  return updatedData[0]
}

/**
 * 核心 OCR 助手函数
 */
async function performOCR(filePath: string, type: string, side?: string): Promise<MerchantApplicationDraftResponse> {
  // 1. 上传图片至 bucket: identity (由 image-service 处理)
  const uploadRes = await uploadFile(filePath, '', 'file', { category: 'identity' })
  const imageUrl = (uploadRes as any).url
  if (!imageUrl) throw new Error('Image upload failed')

  // 2. 获取当前申请单 ID (延迟创建)
  let draft = await getMerchantApplication()
  if (!draft) {
    draft = await createNewMerchantApplication()
  }

  // 3. 更新图片 URL 到对应字段
  const urlFieldMap: Record<string, string> = {
    'business_license': 'business_license_image_url',
    'food_permit': 'food_permit_url',
    'id_card_Front': 'legal_person_id_front_url',
    'id_card_Back': 'legal_person_id_back_url'
  }
  const fieldKey = type === 'id_card' ? `${type}_${side}` : type
  const targetField = urlFieldMap[fieldKey]
  
  if (targetField) {
    await supabase.from('merchant_applications').update({ [targetField]: imageUrl }).eq('id', draft.id)
  }

  // 4. 调用 ocr-service 获取回填结果
  const { error: ocrError } = await supabaseRequest<any>({
    url: '/functions/v1/ocr-service',
    method: 'POST',
    data: {
      application_id: draft.id,
      image_url: imageUrl,
      type,
      side,
      target_table: 'merchant_applications'
    }
  })

  if (ocrError) throw ocrError

  // 5. 返回最新草稿数据
  const finalDraft = await getMerchantApplication()
  if (!finalDraft) throw new Error('Failed to retrieve draft after OCR')
  return finalDraft
}

/**
 * 营业执照 OCR
 */
export async function ocrBusinessLicense(filePath?: string): Promise<MerchantApplicationDraftResponse> {
  if (!filePath) throw new Error('File path required for OCR')
  return await performOCR(filePath, 'business_license')
}

/**
 * 食品经营许可证 OCR
 */
export async function ocrFoodPermit(filePath?: string): Promise<MerchantApplicationDraftResponse> {
  if (!filePath) throw new Error('File path required for OCR')
  return await performOCR(filePath, 'food_permit')
}

/**
 * 身份证 OCR
 */
export async function ocrIdCard(filePath: string | undefined, side: 'Front' | 'Back'): Promise<MerchantApplicationDraftResponse> {
  if (!filePath) throw new Error('File path required for OCR')
  return await performOCR(filePath, 'id_card', side)
}

/**
 * 提交申请（触发后端自动审核 RPC）
 */
export async function submitMerchantApplication(): Promise<MerchantApplicationDraftResponse> {
  const draft = await getMerchantApplication()
  if (!draft) throw new Error('No draft found to submit')
  const { data, error } = await supabase.rpc<MerchantApplicationDraftResponse>('submit_merchant_application', {
    app_id: draft.id
  })
  
  if (error) throw error
  return data!
}

/**
 * 重置被拒绝申请
 */
export async function resetMerchantApplication(): Promise<MerchantApplicationDraftResponse> {
  const draft = await getMerchantApplication()
  if (!draft) throw new Error('No application found to reset')
  const { data: updatedData, error } = await supabase.from<MerchantApplicationDraftResponse>('merchant_applications')
    .update({ status: 'draft', reject_reason: null })
    .eq('id', draft.id)
  
  if (error) throw error
  if (!updatedData || updatedData.length === 0) throw new Error('Reset failed')
  return updatedData![0]
}

/**
 * 获取最新申请单 (me)
 */
export async function getMyApplication(): Promise<MerchantApplicationDraftResponse | null> {
  // 直接复用 getMerchantApplication 即可，逻辑一致 (查询当前用户的申请)
  return await getMerchantApplication()
}

/**
 * 上传普通展示图片 (门头/环境)
 */
export async function uploadMerchantImage(filePath: string, category: 'storefront' | 'environment'): Promise<UploadImageResponse> {
  const res = await uploadFile(filePath, '', 'file', { category })
  return { image_url: (res as any).url }
}

/**
 * 更新图片 URL 列表
 */
export async function updateMerchantImages(data: UpdateMerchantImagesRequest): Promise<MerchantApplicationDraftResponse> {
  let draft = await getMerchantApplication()
  if (!draft) {
    draft = await createNewMerchantApplication()
  }
  const { data: updatedData, error } = await supabase.from<MerchantApplicationDraftResponse>('merchant_applications')
    .update(data)
    .eq('id', draft.id)
  
  if (error) throw error
  if (!updatedData || updatedData.length === 0) throw new Error('Update failed')
  return updatedData[0]
}

// ==================== Legacy / Compatibility Methods (Forward to new Rider API) ====================

export function submitRiderApplication(data: any) {
    console.warn('submitRiderApplication in onboarding.ts is legacy. Use api/rider-application.ts instead.')
    return supabase.rpc('submit_rider_application', { app_id: (data as any).id })
}

export default {
  getMerchantApplication,
  listMerchantApplications,
  createNewMerchantApplication,
  updateMerchantBasicInfo,
  ocrBusinessLicense,
  ocrFoodPermit,
  ocrIdCard,
  submitMerchantApplication,
  getMyApplication,
  resetMerchantApplication,
  uploadMerchantImage,
  updateMerchantImages
}
