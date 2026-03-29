
import { logger } from '../../../../utils/logger'
import { ErrorHandler } from '../../../../utils/error-handler'
import { DraftStorage } from '../../../../utils/draft-storage'
import {
  getMerchantApplication,
  updateMerchantBasicInfo,
  enqueueMerchantApplicationOCRForMedia,
  ocrBusinessLicense,
  ocrFoodPermit,
  ocrIdCard,
  submitMerchantApplication,
  getMyApplication,
  resetMerchantApplication,
  uploadMerchantImage,
  updateMerchantImages,
  deleteMerchantApplicationDocument,
  deleteMediaAsset,
  type MerchantApplicationOCRDocumentType,
  type MerchantApplicationOCRSubmissionResult,
  type MerchantApplicationDraftResponse
} from '../../../../api/onboarding'
import { getPrivateMediaUrl } from '../../../../utils/image-security'
import { getMediaDisplayUrl } from '../../../../utils/media'
import Navigation from '../../../../utils/navigation'
import { buildAgreementConsentPayload } from '../../../../api/agreement-consent'

const DRAFT_KEY = 'merchant_register_draft'

type OCRResult = {
  status?: string
  error?: string
  legal_representative?: string
  person?: string
  name?: string
  enterprise_name?: string
  reg_num?: string
  credit_code?: string
  address?: string
  valid_period?: string
  business_scope?: string
  valid_to?: string
  id_number?: string
  gender?: string
  valid_date?: string
}

type OCRCheckResult = {
  label: string
  status: string
  error?: string
}

type MerchantDraftExt = MerchantApplicationDraftResponse & {
  business_address_detail?: string
  legal_person_contact_address?: string
  bank_name?: string
  bank_account?: string
  bank_account_name?: string
}

type ImageFieldItem = {
  url: string
  rawUrl?: string
  assetId?: number
}

type UploadField = 'license' | 'foodPermit' | 'idCardFront' | 'idCardBack'

type OCRFieldKey = 'business_license_ocr' | 'food_permit_ocr' | 'id_card_front_ocr' | 'id_card_back_ocr'

type OCRPollOptions = {
  retryQueue?: {
    mediaId: number
    documentType: MerchantApplicationOCRDocumentType
    side?: 'Front' | 'Back'
  }
  timeoutMessage?: string
}

const activeOcrPollers: Partial<Record<OCRFieldKey, number>> = {}

function buildPrivateAssetKey(assetId?: number | null): string | undefined {
  return assetId && assetId > 0 ? `asset:${assetId}` : undefined
}

type DraftData = {
  formData?: Record<string, unknown>
  licenseImages?: ImageFieldItem[]
  foodLicenseImages?: ImageFieldItem[]
  idCardFrontImages?: ImageFieldItem[]
  idCardBackImages?: ImageFieldItem[]
  accountPermitImages?: ImageFieldItem[]
  storefrontImages?: ImageFieldItem[]
  environmentImages?: ImageFieldItem[]
  shopImages?: ImageFieldItem[]
  ocrResults?: {
    license: OCRResult | null
    idCard: OCRResult | null
  }
}

function getErrorMessage(error: unknown, fallback: string): string {
  if (error && typeof error === 'object') {
    const maybeError = error as {
      userMessage?: string
      message?: string
      data?: { message?: string }
    }
    return maybeError.userMessage || maybeError.message || maybeError.data?.message || fallback
  }
  return fallback
}

function toSafeString(value: unknown): string {
  if (value === null || value === undefined || value === true || value === 'true') {
    return ''
  }
  return String(value)
}

Page({
  data: {
    navBarHeight: 88,
    currentStep: 0, // 0: Intro, 1: Upload, 2: Info, 3: Location, 4: Review, 5: Polling
    isSubmitting: false, // 防止重复提交
    applicationInitialized: false, // 标记申请草稿是否已成功创建
    ocrProgressMessage: '',
    ocrFieldPlaceholder: '系统识别中，完成后自动回填',
    formData: {
      // 基本信息
      name: '',
      phone: '',
      address: '',
      addressDetail: '',
      regionId: 0,
      latitude: 0,
      longitude: 0,
      // 证照信息
      licenseName: '',
      creditCode: '',
      registerAddress: '',
      licenseValidity: '',
      businessScope: '',
      foodLicenseValidity: '',

      // 法人信息
      legalPerson: '',
      idCard: '',
      gender: '',
      hometown: '',
      currentAddress: '',
      idCardValidity: '',

      // 结算信息
      bankName: '',
      bankAccount: '',
      accountName: ''
    },
    // 图片 (包含 url 和 rawUrl)
    licenseImages: [] as ImageFieldItem[],
    foodLicenseImages: [] as ImageFieldItem[],
    idCardFrontImages: [] as ImageFieldItem[],
    idCardBackImages: [] as ImageFieldItem[],
    accountPermitImages: [] as ImageFieldItem[],
    storefrontImages: [] as ImageFieldItem[],  // 门头照，最多3张
    environmentImages: [] as ImageFieldItem[], // 环境照，最多5张

    // OCR原始结果 (用于一致性校验)
    ocrResults: {
      license: null as OCRResult | null,
      idCard: null as OCRResult | null
    },

    // 选择器状态
    typePickerVisible: false,
    typePickerValue: [],
    typeOptions: [
      { label: '中餐', value: 'chinese' },
      { label: '西餐', value: 'western' },
      { label: '日韩料理', value: 'japanese_korean' },
      { label: '快餐', value: 'fast_food' },
      { label: '小吃', value: 'snack' },
      { label: '甜品饮品', value: 'dessert' },
      { label: '其他', value: 'other' }
    ],
    timePickerVisible: false,
    timePickerValue: [],
    timeOptions: [
      { label: '全天营业 (00:00-24:00)', value: 'all_day' },
      { label: '早餐时段 (06:00-10:00)', value: 'breakfast' },
      { label: '午餐时段 (11:00-14:00)', value: 'lunch' },
      { label: '晚餐时段 (17:00-21:00)', value: 'dinner' },
      { label: '自定义时间', value: 'custom' }
    ],
    consentChecked: false,
    consentPopupVisible: false
  },

  async onLoad() {
    // 先从后端加载草稿数据（后端优先）
    await this.initApplication()
    // 后端数据加载后，再尝试从本地草稿恢复（仅补充后端没有返回的数据）
    // 注意：如果后端已返回数据，loadDraft 可能会覆盖，需要谨慎处理
    // 暂时禁用 loadDraft，完全依赖后端数据
    // this.loadDraft()
  },

  async initApplication() {
    wx.showLoading({ title: '加载中...' })
    console.log('[DEBUG] initApplication 开始')
    try {
      const res = await getMerchantApplication()
      console.log('[DEBUG] getMerchantApplication 返回:', res)
      if (!res) {
        logger.warn('[MerchantRegister] getMerchantApplication returned null', undefined, 'initApplication')
        wx.hideLoading()
        wx.showToast({ title: '无法创建申请，请重试', icon: 'none' })
        return
      }

      const data = res as MerchantDraftExt
      console.log('[DEBUG] 后端返回的原始数据:', JSON.stringify(data, null, 2))
      logger.info('[MerchantRegister] 加载申请数据', data, 'initApplication')

      // 标记申请已初始化成功
      this.setData({ applicationInitialized: true })

      // 检查申请状态 - 如果已提交或已通过，直接跳转
      if (data.status === 'approved') {
        wx.showToast({ title: '您已是商户', icon: 'success' })
        setTimeout(() => {
          wx.reLaunch({ url: '/pages/merchant/dashboard/index' })
        }, 1000)
        return
      }
      if (data.status === 'submitted') {
        wx.showToast({ title: '申请审核中', icon: 'none' })
        this.setData({ currentStep: 5 })
        this.startPollingStatus()
        return
      }

      const safeStr = (val: unknown): string => {
        if (val === null || val === undefined || val === true || val === 'true') return ''
        return String(val)
      }

      // 映射 formData
      const formData = {
        ...this.data.formData,
        name: safeStr(data.merchant_name),
        phone: safeStr(data.contact_phone),
        address: safeStr(data.business_address),
        addressDetail: safeStr(data.business_address_detail),
        latitude: data.latitude ? parseFloat(data.latitude) : 0,
        longitude: data.longitude ? parseFloat(data.longitude) : 0,

        // OCR 回填
        licenseName: safeStr(data.business_license_ocr?.enterprise_name),
        creditCode: safeStr(data.business_license_number || data.business_license_ocr?.reg_num || data.business_license_ocr?.credit_code),
        registerAddress: safeStr(data.business_license_ocr?.address),
        licenseValidity: safeStr(data.business_license_ocr?.valid_period),
        businessScope: safeStr(data.business_scope || data.business_license_ocr?.business_scope),
        foodLicenseValidity: safeStr(data.food_permit_ocr?.valid_to),

        legalPerson: safeStr(data.id_card_front_ocr?.name || data.legal_person_name || data.business_license_ocr?.legal_representative),
        idCard: safeStr(data.id_card_front_ocr?.id_number || data.legal_person_id_number),
        gender: safeStr(data.id_card_front_ocr?.gender),
        hometown: safeStr(data.id_card_front_ocr?.address),
        idCardValidity: safeStr(data.id_card_back_ocr?.valid_date),

        currentAddress: safeStr(data.legal_person_contact_address),
        bankName: safeStr(data.bank_name),
        bankAccount: safeStr(data.bank_account),
        accountName: safeStr(data.bank_account_name)
      }

      // OCR 原始数据
      const ocrResults = {
        license: data.business_license_ocr || null,
        idCard: data.id_card_front_ocr || null
      }

      // 解析图片 URL
      const safeResolve = async (assetId?: number | null): Promise<string> => {
        if (assetId && assetId > 0) {
          try { return await getPrivateMediaUrl(assetId) } catch { return '' }
        }
        return ''
      }

      const licenseUrl = getMediaDisplayUrl(data.business_license_url || '')
      const foodLicenseUrl = getMediaDisplayUrl(data.food_permit_url || '')
      const idCardFrontUrl = await safeResolve(data.id_card_front_media_asset_id)
      const idCardBackUrl = await safeResolve(data.id_card_back_media_asset_id)

      // 公开证照直接使用后端返回的本人可见 URL；仅身份证保留私有重签名标识。
      const licenseImages = licenseUrl ? [{ url: licenseUrl, assetId: data.business_license_media_asset_id ?? undefined }] : []
      const foodLicenseImages = foodLicenseUrl ? [{ url: foodLicenseUrl, assetId: data.food_permit_media_asset_id ?? undefined }] : []
      const idCardFrontImages = idCardFrontUrl ? [{ url: idCardFrontUrl, rawUrl: buildPrivateAssetKey(data.id_card_front_media_asset_id), assetId: data.id_card_front_media_asset_id ?? undefined }] : []
      const idCardBackImages = idCardBackUrl ? [{ url: idCardBackUrl, rawUrl: buildPrivateAssetKey(data.id_card_back_media_asset_id), assetId: data.id_card_back_media_asset_id ?? undefined }] : []
      const accountPermitImages: Array<{ url: string }> = []

      // 门头照
      const storefrontRaw: string[] = Array.isArray(data.storefront_images) ? data.storefront_images : []
      const storefrontImages: Array<{ url: string, rawUrl?: string }> = []
      for (const url of storefrontRaw) {
        const resolved = getMediaDisplayUrl(url)
        if (resolved) storefrontImages.push({ url: resolved, rawUrl: url })
      }

      // 环境照
      const environmentRaw: string[] = Array.isArray(data.environment_images) ? data.environment_images : []
      const environmentImages: Array<{ url: string, rawUrl?: string }> = []
      for (const url of environmentRaw) {
        const resolved = getMediaDisplayUrl(url)
        if (resolved) environmentImages.push({ url: resolved, rawUrl: url })
      }

      console.log('[DEBUG] setData payload:', { formData, licenseImages: licenseImages.length, storefrontImages: storefrontImages.length, environmentImages: environmentImages.length })

      // 关键：一次性设置所有数据
      this.setData({
        formData,
        ocrResults,
        licenseImages,
        foodLicenseImages,
        idCardFrontImages,
        idCardBackImages,
        accountPermitImages,
        storefrontImages,
        environmentImages
      })

      logger.debug('[MerchantRegister] initApplication 完成', formData, 'initApplication')
      wx.hideLoading()
    } catch (e: unknown) {
      wx.hideLoading()
      console.error('[MerchantRegister] initApplication Error:', e)
      // 如果初始化失败，提示用户刷新页面
      wx.showModal({
        title: '加载失败',
        content: getErrorMessage(e, '无法加载申请数据，请检查网络后重试'),
        confirmText: '重试',
        cancelText: '返回',
        success: (res) => {
          if (res.confirm) {
            // 重试初始化
            this.initApplication()
          } else {
            wx.navigateBack()
          }
        }
      })
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onUnload() {
    this.clearAllOcrPollers()
  },

  applyLatestOcrDraft(data: MerchantDraftExt) {
    this.setData({
      formData: {
        ...this.data.formData,
        licenseName: toSafeString(data.business_license_ocr?.enterprise_name),
        creditCode: toSafeString(data.business_license_number || data.business_license_ocr?.reg_num || data.business_license_ocr?.credit_code),
        registerAddress: toSafeString(data.business_license_ocr?.address),
        licenseValidity: toSafeString(data.business_license_ocr?.valid_period),
        businessScope: toSafeString(data.business_scope || data.business_license_ocr?.business_scope),
        foodLicenseValidity: toSafeString(data.food_permit_ocr?.valid_to),
        legalPerson: toSafeString(data.id_card_front_ocr?.name || data.legal_person_name || data.business_license_ocr?.legal_representative),
        idCard: toSafeString(data.id_card_front_ocr?.id_number || data.legal_person_id_number),
        gender: toSafeString(data.id_card_front_ocr?.gender),
        hometown: toSafeString(data.id_card_front_ocr?.address),
        idCardValidity: toSafeString(data.id_card_back_ocr?.valid_date)
      },
      ocrResults: {
        license: data.business_license_ocr || null,
        idCard: data.id_card_front_ocr || null
      }
    }, () => {
      this.updateOcrProgressMessage(data)
      this.saveDraft()
    })
  },

  async removeUploadedDocument(field: UploadField) {
    const documentMap: Record<UploadField, {
      documentType: 'business_license' | 'food_permit' | 'id_card_front' | 'id_card_back'
      fieldKey: OCRFieldKey
      data: Record<string, unknown>
    }> = {
      license: {
        documentType: 'business_license',
        fieldKey: 'business_license_ocr',
        data: {
          licenseImages: [],
          'formData.licenseName': '',
          'formData.creditCode': '',
          'formData.registerAddress': '',
          'formData.licenseValidity': '',
          'formData.businessScope': '',
          'ocrResults.license': null
        }
      },
      foodPermit: {
        documentType: 'food_permit',
        fieldKey: 'food_permit_ocr',
        data: {
          foodLicenseImages: [],
          'formData.foodLicenseValidity': ''
        }
      },
      idCardFront: {
        documentType: 'id_card_front',
        fieldKey: 'id_card_front_ocr',
        data: {
          idCardFrontImages: [],
          'formData.legalPerson': '',
          'formData.idCard': '',
          'formData.gender': '',
          'formData.hometown': '',
          'ocrResults.idCard': null
        }
      },
      idCardBack: {
        documentType: 'id_card_back',
        fieldKey: 'id_card_back_ocr',
        data: {
          idCardBackImages: [],
          'formData.idCardValidity': ''
        }
      }
    }

    const target = documentMap[field]
    this.stopOcrPolling(target.fieldKey)

    wx.showLoading({ title: '删除中...' })
    try {
      const latestDraft = await deleteMerchantApplicationDocument(target.documentType) as MerchantDraftExt
      this.setData(target.data, () => {
        this.applyLatestOcrDraft(latestDraft)
      })
    } catch (error) {
      logger.error('[MerchantRegister] 删除证照失败', { field, error }, 'removeUploadedDocument')
      wx.showToast({ title: getErrorMessage(error, '删除失败，请重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  buildOcrProgressMessage(data?: MerchantDraftExt) {
    const checks = [
      {
        uploaded: Boolean((data?.business_license_media_asset_id && data.business_license_media_asset_id > 0) || this.data.licenseImages.length > 0),
        status: data?.business_license_ocr?.status || ''
      },
      {
        uploaded: Boolean((data?.food_permit_media_asset_id && data.food_permit_media_asset_id > 0) || this.data.foodLicenseImages.length > 0),
        status: data?.food_permit_ocr?.status || ''
      },
      {
        uploaded: Boolean((data?.id_card_front_media_asset_id && data.id_card_front_media_asset_id > 0) || this.data.idCardFrontImages.length > 0),
        status: data?.id_card_front_ocr?.status || ''
      },
      {
        uploaded: Boolean((data?.id_card_back_media_asset_id && data.id_card_back_media_asset_id > 0) || this.data.idCardBackImages.length > 0),
        status: data?.id_card_back_ocr?.status || ''
      }
    ]

    const hasInProgress = checks.some((item) => item.uploaded && item.status !== 'done' && item.status !== 'failed')
    if (!hasInProgress) {
      return ''
    }

    return '证照已上传，系统正在自动识别，完成后会自动回填。你可以先继续填写后续信息。'
  },

  updateOcrProgressMessage(data?: MerchantDraftExt) {
    this.setData({
      ocrProgressMessage: this.buildOcrProgressMessage(data)
    })
  },

  getUploadStepOcrChecks(data: MerchantDraftExt): OCRCheckResult[] {
    return [
      {
        label: '营业执照',
        status: data.business_license_ocr?.status || '',
        error: data.business_license_ocr?.error
      },
      {
        label: '食品经营许可证',
        status: data.food_permit_ocr?.status || '',
        error: data.food_permit_ocr?.error
      },
      {
        label: '身份证正面',
        status: data.id_card_front_ocr?.status || '',
        error: data.id_card_front_ocr?.error
      },
      {
        label: '身份证反面',
        status: data.id_card_back_ocr?.status || '',
        error: data.id_card_back_ocr?.error
      }
    ]
  },

  clearAllOcrPollers() {
    (Object.keys(activeOcrPollers) as OCRFieldKey[]).forEach((fieldKey) => {
      const intervalId = activeOcrPollers[fieldKey]
      if (intervalId) {
        clearInterval(intervalId)
        delete activeOcrPollers[fieldKey]
      }
    })
  },

  stopOcrPolling(fieldKey: OCRFieldKey) {
    const intervalId = activeOcrPollers[fieldKey]
    if (!intervalId) return
    clearInterval(intervalId)
    delete activeOcrPollers[fieldKey]
  },

  setUploadedImage(field: UploadField, path: string, assetId?: number) {
    const image = [{ url: path, assetId }]
    switch (field) {
      case 'license':
        this.setData({ licenseImages: image })
        break
      case 'foodPermit':
        this.setData({ foodLicenseImages: image })
        break
      case 'idCardFront':
        this.setData({ idCardFrontImages: image })
        break
      default:
        this.setData({ idCardBackImages: image })
        break
    }
  },

  handleDocumentOCRSubmission(
    label: string,
    fieldKey: OCRFieldKey,
    result: MerchantApplicationOCRSubmissionResult,
    onRecognized: (ocr: OCRResult) => void,
    options?: {
      documentType: MerchantApplicationOCRDocumentType
      side?: 'Front' | 'Back'
    }
  ) {
    if (result.state === 'queued') {
      wx.hideLoading()
      wx.showToast({ title: '上传成功，识别中...', icon: 'none' })
      this.updateOcrProgressMessage(result.draft as MerchantDraftExt)
      this.pollOcrStatus(fieldKey, onRecognized)
      return
    }

    wx.hideLoading()
    wx.showToast({
      title: `${label}已上传，系统处理中`,
      icon: 'none',
      duration: 2500
    })
    this.updateOcrProgressMessage(result.draft as MerchantDraftExt)
    this.pollOcrStatus(fieldKey, onRecognized, {
      retryQueue: {
        mediaId: result.mediaId,
        documentType: options?.documentType || 'business_license',
        side: options?.side
      }
    })
  },

  async ensureUploadStepOCRQueued(data: MerchantDraftExt) {
    const retryTargets: Array<{
      fieldKey: OCRFieldKey
      assetId?: number
      documentType: MerchantApplicationOCRDocumentType
      side?: 'Front' | 'Back'
    }> = [
      {
        fieldKey: 'business_license_ocr',
        assetId: data.business_license_media_asset_id ?? this.data.licenseImages[0]?.assetId,
        documentType: 'business_license'
      },
      {
        fieldKey: 'food_permit_ocr',
        assetId: data.food_permit_media_asset_id ?? this.data.foodLicenseImages[0]?.assetId,
        documentType: 'food_permit'
      },
      {
        fieldKey: 'id_card_front_ocr',
        assetId: data.id_card_front_media_asset_id ?? this.data.idCardFrontImages[0]?.assetId,
        documentType: 'id_card',
        side: 'Front'
      },
      {
        fieldKey: 'id_card_back_ocr',
        assetId: data.id_card_back_media_asset_id ?? this.data.idCardBackImages[0]?.assetId,
        documentType: 'id_card',
        side: 'Back'
      }
    ]

    let latestDraft = data
    for (const target of retryTargets) {
      if (!target.assetId || latestDraft[target.fieldKey]?.status) {
        continue
      }

      try {
        const retryResult = await enqueueMerchantApplicationOCRForMedia(
          target.assetId,
          target.documentType,
          target.side,
          { maxAttempts: 1, retryDelayMs: 0 }
        )
        latestDraft = retryResult.draft as MerchantDraftExt
      } catch (error) {
        logger.warn('[MerchantRegister] 补建 OCR 任务失败', {
          fieldKey: target.fieldKey,
          assetId: target.assetId,
          error
        }, 'ensureUploadStepOCRQueued')
      }
    }

    return latestDraft
  },

  // ==================== 草稿管理 ====================

  saveDraft() {
    const data = {
      formData: this.data.formData,
      licenseImages: this.data.licenseImages,
      foodLicenseImages: this.data.foodLicenseImages,
      idCardFrontImages: this.data.idCardFrontImages,
      idCardBackImages: this.data.idCardBackImages,
      accountPermitImages: this.data.accountPermitImages,
      storefrontImages: this.data.storefrontImages,
      environmentImages: this.data.environmentImages,
      ocrResults: this.data.ocrResults
    }
    DraftStorage.save(DRAFT_KEY, data)
  },

  async syncToBackend() {
    if (!this.data.applicationInitialized) return
    const { formData } = this.data
    // Only sync if minimum required fields are present
    if (!formData.name) {
      logger.debug('[MerchantRegister] syncToBackend skipped: no name', { name: formData.name })
      return
    }

    try {
      // 构造基础信息 payload（仅包含 PUT /basic 支持的字段）
      const payload = {
        merchant_name: formData.name,
        contact_phone: formData.phone,
        business_address: formData.addressDetail || formData.address,
        longitude: formData.longitude ? String(formData.longitude) : undefined,
        latitude: formData.latitude ? String(formData.latitude) : undefined,
        region_id: formData.regionId
      }

      console.log('[MerchantRegister] syncToBackend payload:', JSON.stringify(payload, null, 2))

      await updateMerchantBasicInfo(payload)

      // 门头照/环境照通过单独接口保存（PUT /basic 不处理这两个字段）
      const storefrontImages = this.data.storefrontImages || []
      const environmentImages = this.data.environmentImages || []
      if (storefrontImages.length > 0 || environmentImages.length > 0) {
        await updateMerchantImages({
          storefront_images: storefrontImages.map((i) => i.rawUrl || i.url),
          environment_images: environmentImages.map((i) => i.rawUrl || i.url)
        })
      }

      console.log('[MerchantRegister] Sync to backend success')
    } catch (err: unknown) {
      console.error('[MerchantRegister] Sync to backend failed', err)
      const errData = err as { originalError?: unknown, data?: unknown, message?: string }
      console.error('[MerchantRegister] Error details:', errData.originalError || errData.data || errData.message)
      // Silent fail to not interrupt user flow, unless final submit
    }
  },

  loadDraft() {
    const draft = DraftStorage.load(DRAFT_KEY) as DraftData | null
    if (draft) {
      // 深度清洗 formData (防止 null/true/undefined 导致崩溃)
      const safeFormData = { ...this.data.formData }

      const draftFormData = draft.formData || {}
      if (draftFormData) {
        // No special sanitization needed for removed fields
        Object.keys(safeFormData).forEach((key) => {
          let val = draftFormData[key]

          // 1. 强制转为空字符串的情况
          if (val === null || val === undefined) {
            val = ''
          }
          // 2. 修复错误的 "true" 值 (字符串或布尔)
          if (val === true || val === 'true') {
            val = ''
          }

          // 3. 类型赋值
          if (key === 'latitude' || key === 'longitude') {
            (safeFormData as Record<string, unknown>)[key] = Number(val) || 0
          } else {
            (safeFormData as Record<string, unknown>)[key] = String(val)
          }
        })
      }

      this.setData({
        formData: safeFormData,
        licenseImages: draft.licenseImages || [],
        foodLicenseImages: draft.foodLicenseImages || [],
        idCardFrontImages: draft.idCardFrontImages || [],
        idCardBackImages: draft.idCardBackImages || [],
        accountPermitImages: draft.accountPermitImages || [],
        shopImages: draft.shopImages || [],
        ocrResults: draft.ocrResults || { license: null, idCard: null }
      })
    }
  },

  // ==================== 表单输入 ====================

  updateFormData(key: string, value: unknown) {
    this.setData({ [`formData.${key}`]: value })
    this.saveDraft()
  },

  onNameInput(e: WechatMiniprogram.Input) { this.updateFormData('name', e.detail.value) },
  onPhoneInput(e: WechatMiniprogram.Input) { this.updateFormData('phone', e.detail.value) },
  onAddressDetailInput(e: WechatMiniprogram.Input) { this.updateFormData('addressDetail', e.detail.value) },

  // 证照信息输入
  onLicenseNameInput(e: WechatMiniprogram.Input) { this.updateFormData('licenseName', e.detail.value) },
  onCreditCodeInput(e: WechatMiniprogram.Input) { this.updateFormData('creditCode', e.detail.value) },
  onRegisterAddressInput(e: WechatMiniprogram.Input) { this.updateFormData('registerAddress', e.detail.value) },
  onLicenseValidityInput(e: WechatMiniprogram.Input) { this.updateFormData('licenseValidity', e.detail.value) },
  onBusinessScopeInput(e: WechatMiniprogram.Input) { this.updateFormData('businessScope', e.detail.value) },
  onFoodLicenseValidityInput(e: WechatMiniprogram.Input) { this.updateFormData('foodLicenseValidity', e.detail.value) },

  // 法人信息输入
  onLegalPersonInput(e: WechatMiniprogram.Input) { this.updateFormData('legalPerson', e.detail.value) },
  onIdCardInput(e: WechatMiniprogram.Input) { this.updateFormData('idCard', e.detail.value) },
  onGenderInput(e: WechatMiniprogram.Input) { this.updateFormData('gender', e.detail.value) },
  onHometownInput(e: WechatMiniprogram.Input) { this.updateFormData('hometown', e.detail.value) },
  onCurrentAddressInput(e: WechatMiniprogram.Input) { this.updateFormData('currentAddress', e.detail.value) },
  onIdCardValidityInput(e: WechatMiniprogram.Input) { this.updateFormData('idCardValidity', e.detail.value) },

  // 结算信息输入
  onBankNameInput(e: WechatMiniprogram.Input) { this.updateFormData('bankName', e.detail.value) },
  onBankAccountInput(e: WechatMiniprogram.Input) { this.updateFormData('bankAccount', e.detail.value) },
  onAccountNameInput(e: WechatMiniprogram.Input) { this.updateFormData('accountName', e.detail.value) },

  // ==================== 地址选择 ====================

  onAddressInput(e: WechatMiniprogram.Input) { this.updateFormData('address', e.detail.value) },

  // ==================== 地址选择 ====================

  onChooseAddress() {
    wx.chooseLocation({
      success: (res) => {
        // 用户强调需要显示详细地址 (省市区+街道+门牌+名称)
        // 无论返回结构如何，都要尽可能组合出完整地址
        const addr = res.address || ''
        const name = res.name || ''

        // 组合地址: 优先全部信息，确保不遗漏
        let fullAddress = ''
        if (addr && name) {
          // 如果地址已包含名称，不重复
          fullAddress = addr.includes(name) ? addr : `${addr} ${name}`
        } else if (addr) {
          fullAddress = addr
        } else if (name) {
          fullAddress = name
        }

        // 如果 fullAddress 还是空的，尝试用经纬度提示
        if (!fullAddress && (res.latitude || res.longitude)) {
          fullAddress = `位置: ${res.latitude.toFixed(6)}, ${res.longitude.toFixed(6)}`
        }

        console.log('[ChooseLocation] Final Address:', fullAddress, { res })

        // 使用 setData 的回调确保视图更新
        this.setData({
          'formData.address': fullAddress,
          'formData.addressDetail': fullAddress, // 同时设置 addressDetail 保持一致性
          'formData.latitude': res.latitude,
          'formData.longitude': res.longitude
        }, () => {
          this.saveDraft()
        })
      },
      fail: (err) => {
        if (err.errMsg.includes('auth deny')) {
          wx.showModal({
            title: '需要位置权限',
            content: '请在设置中开启位置权限',
            confirmText: '去设置',
            success: (modalRes) => {
              if (modalRes.confirm) {
                wx.openSetting()
              }
            }
          })
        }
      }
    })
  },

  // ==================== 选择器 ====================

  // ==================== 步骤导航 ====================
  onConsentChange(e: WechatMiniprogram.CustomEvent<{ value?: string[] }>) {
    const values = e.detail.value || []
    this.setData({ consentChecked: values.includes('agree') })
  },

  openConsentPopup() {
    this.setData({ consentPopupVisible: true })
  },

  closeConsentPopup() {
    this.setData({ consentPopupVisible: false })
  },

  onConfirmConsent() {
    this.setData({ consentChecked: true, consentPopupVisible: false })
  },

  onViewAgreement(e: WechatMiniprogram.CustomEvent<{ type?: string, title?: string }>) {
    const type = (e.currentTarget.dataset as { type?: string }).type
    const title = (e.currentTarget.dataset as { title?: string }).title
    if (!type) return
    Navigation.toAgreementDetail(type, title)
  },

  ensureConsent(): boolean {
    if (this.data.consentChecked) return true
    this.openConsentPopup()
    wx.showToast({ title: '请先阅读并同意协议', icon: 'none' })
    return false
  },

  async nextStep() {
    const { currentStep, licenseImages, foodLicenseImages, idCardFrontImages, idCardBackImages } = this.data

    // Step 1 check (Intro) - No validation
    if (currentStep === 0) {
      if (!this.ensureConsent()) return
      this.syncToBackend()
      this.setData({ currentStep: 1 })
      return
    }

    // Step 2 check (Upload) - Require all images
    if (currentStep === 1) {
      if (licenseImages.length === 0 || foodLicenseImages.length === 0 || idCardFrontImages.length === 0 || idCardBackImages.length === 0) {
        wx.showToast({ title: '请上传所有必需的证照', icon: 'none' })
        return
      }

      wx.showLoading({ title: '校验识别结果...' })
      try {
        let latestDraft = await getMerchantApplication() as MerchantDraftExt
        latestDraft = await this.ensureUploadStepOCRQueued(latestDraft)
        this.applyLatestOcrDraft(latestDraft)

        const failedCheck = this.getUploadStepOcrChecks(latestDraft).find((item) => item.status === 'failed')
        if (failedCheck) {
          wx.showToast({
            title: failedCheck.error || `${failedCheck.label}识别失败，请重新上传`,
            icon: 'none',
            duration: 3000
          })
          return
        }

        this.setData({ currentStep: 2 })
        return
      } catch (error) {
        wx.showToast({
          title: getErrorMessage(error, '识别结果校验失败，请重试'),
          icon: 'none',
          duration: 3000
        })
        return
      } finally {
        wx.hideLoading()
      }
    }

    // Step 3 check (Info) - 必填字段校验
    if (currentStep === 2) {
      let latestDraft: MerchantDraftExt | null = null
      try {
        latestDraft = await getMerchantApplication() as MerchantDraftExt
        this.applyLatestOcrDraft(latestDraft)
      } catch (error) {
        logger.warn('[MerchantRegister] 刷新 OCR 草稿失败', error, 'nextStep')
      }

      const currentFormData = this.data.formData
      const mergedCreditCode = currentFormData.creditCode
        || toSafeString(latestDraft?.business_license_number || latestDraft?.business_license_ocr?.reg_num || latestDraft?.business_license_ocr?.credit_code)
      const mergedLegalPerson = currentFormData.legalPerson
        || toSafeString(latestDraft?.id_card_front_ocr?.name || latestDraft?.legal_person_name || latestDraft?.business_license_ocr?.legal_representative)
      const mergedIdCard = currentFormData.idCard
        || toSafeString(latestDraft?.id_card_front_ocr?.id_number || latestDraft?.legal_person_id_number)

      const { name, phone } = currentFormData
      const missingBasicFields: string[] = []
      const missingOCRFields: string[] = []

      if (!name?.trim()) missingBasicFields.push('店铺名称')
      if (!phone?.trim()) missingBasicFields.push('联系电话')
      if (!mergedCreditCode?.trim()) missingOCRFields.push('统一信用代码')
      if (!mergedLegalPerson?.trim()) missingOCRFields.push('法人姓名')
      if (!mergedIdCard?.trim()) missingOCRFields.push('身份证号')

      if (missingBasicFields.length > 0) {
        const message = missingBasicFields.length <= 3
          ? `请填写: ${missingBasicFields.join('、')}`
          : `还有 ${missingBasicFields.length} 项必填信息未完善`
        wx.showToast({ title: message, icon: 'none', duration: 3000 })
        return
      }

      if (missingOCRFields.length > 0) {
        wx.showToast({
          title: '证照信息处理中',
          icon: 'none',
          duration: 3000
        })
        return
      }

      // 进入 Step 3 前刷新门头照/环境照签名
      this.refreshShopImageUrls()
      this.setData({ currentStep: 3 })
      return
    }

    // Step 4 check (Location)
    if (currentStep === 3) {
      // 使用 address 字段（地图选择时设置的）而不是 addressDetail
      const { address, latitude, longitude } = this.data.formData
      if (!address) {
        wx.showToast({ title: '请选择店铺地址', icon: 'none' })
        return
      }
      // regionId 由后端根据经纬度自动匹配，不强制前端校验
      if (!latitude || !longitude) {
        wx.showToast({ title: '请通过地图选择精确位置', icon: 'none' })
        return
      }
      this.syncToBackend()
      this.setData({ currentStep: 4 })
      return
    }
  },

  prevStep() {
    const { currentStep } = this.data
    if (currentStep > 0) {
      this.setData({ currentStep: currentStep - 1 })
    }
  },
  // ==================== 图片上传与OCR ====================

  // 营业执照
  async onLicenseUpload(e: WechatMiniprogram.CustomEvent) {
    const { path } = e.detail
    if (!path) return

    // 检查申请是否已初始化
    if (!this.data.applicationInitialized) {
      wx.showToast({ title: '请等待页面加载完成', icon: 'none' })
      return
    }

    // 先显示本地图片
    this.setUploadedImage('license', path)
    this.saveDraft()

    wx.showLoading({ title: '上传中...' })
    try {
      const result = await ocrBusinessLicense(path)
      this.setUploadedImage('license', path, result.mediaId)
      this.saveDraft()
      logger.info('[MerchantRegister] 营业执照上传成功', result, 'onLicenseUpload')
      this.handleDocumentOCRSubmission('营业执照', 'business_license_ocr', result, (ocr) => {
        if (ocr) {
          this.setData({
            'formData.licenseName': ocr.enterprise_name || '',
            'formData.creditCode': ocr.reg_num || ocr.credit_code || '',
            'formData.registerAddress': ocr.address || '',
            'formData.legalPerson': ocr.legal_representative || '',
            'formData.licenseValidity': ocr.valid_period || '',
            'formData.businessScope': ocr.business_scope || '',
            'ocrResults.license': ocr
          })
          this.saveDraft()
          wx.showToast({ title: '识别成功', icon: 'success' })
        }
      }, {
        documentType: 'business_license'
      })
    } catch (error: unknown) {
      wx.hideLoading()
      logger.error('[MerchantRegister] 营业执照上传失败', error, 'onLicenseUpload')
      // 显示更具体的错误信息
      const errMsg = getErrorMessage(error, '上传失败，请重试')
      wx.showToast({ title: errMsg, icon: 'none', duration: 3000 })
    }
  },
  onLicenseRemove() {
    this.removeUploadedDocument('license')
  },

  // 食品经营许可证
  async onFoodLicenseUpload(e: WechatMiniprogram.CustomEvent) {
    const { path } = e.detail
    if (!path) return

    if (!this.data.applicationInitialized) {
      wx.showToast({ title: '请等待页面加载完成', icon: 'none' })
      return
    }

    this.setUploadedImage('foodPermit', path)
    this.saveDraft()

    wx.showLoading({ title: '上传中...' })
    try {
      const result = await ocrFoodPermit(path)
      this.setUploadedImage('foodPermit', path, result.mediaId)
      this.saveDraft()
      logger.info('[MerchantRegister] 食品许可证上传成功', result, 'onFoodLicenseUpload')
      this.handleDocumentOCRSubmission('食品经营许可证', 'food_permit_ocr', result, (ocr) => {
        if (ocr) {
          this.setData({
            'formData.foodLicenseValidity': ocr.valid_to || ''
          })
          this.saveDraft()
          wx.showToast({ title: '识别成功', icon: 'success' })
        }
      }, {
        documentType: 'food_permit'
      })
    } catch (error: unknown) {
      wx.hideLoading()
      logger.error('[MerchantRegister] 食品许可证上传失败', error, 'onFoodLicenseUpload')
      const errMsg = getErrorMessage(error, '上传失败，请重试')
      wx.showToast({ title: errMsg, icon: 'none', duration: 3000 })
    }
  },
  onFoodLicenseRemove() {
    this.removeUploadedDocument('foodPermit')
  },

  // 身份证正面
  async onIdCardFrontUpload(e: WechatMiniprogram.CustomEvent) {
    const { path } = e.detail
    if (!path) return

    if (!this.data.applicationInitialized) {
      wx.showToast({ title: '请等待页面加载完成', icon: 'none' })
      return
    }

    this.setUploadedImage('idCardFront', path)
    this.saveDraft()

    wx.showLoading({ title: '上传中...' })
    try {
      const result = await ocrIdCard(path, 'Front')
      this.setUploadedImage('idCardFront', path, result.mediaId)
      this.saveDraft()
      logger.info('[MerchantRegister] 身份证正面上传成功', result, 'onIdCardFrontUpload')
      this.handleDocumentOCRSubmission('身份证正面', 'id_card_front_ocr', result, (ocr) => {
        if (ocr) {
          this.setData({
            'formData.legalPerson': ocr.name || '',
            'formData.idCard': ocr.id_number || '',
            'formData.gender': ocr.gender || '',
            'formData.hometown': ocr.address || '',
            'ocrResults.idCard': ocr
          })
          this.saveDraft()
          wx.showToast({ title: '识别成功', icon: 'success' })
        }
      }, {
        documentType: 'id_card',
        side: 'Front'
      })
    } catch (error: unknown) {
      wx.hideLoading()
      logger.error('[MerchantRegister] 身份证正面上传失败', error, 'onIdCardFrontUpload')
      const errMsg = getErrorMessage(error, '上传失败，请重试')
      wx.showToast({ title: errMsg, icon: 'none', duration: 3000 })
    }
  },
  onIdCardFrontRemove() {
    this.removeUploadedDocument('idCardFront')
  },

  // 身份证反面
  async onIdCardBackUpload(e: WechatMiniprogram.CustomEvent) {
    const { path } = e.detail
    if (!path) return

    if (!this.data.applicationInitialized) {
      wx.showToast({ title: '请等待页面加载完成', icon: 'none' })
      return
    }

    this.setUploadedImage('idCardBack', path)
    this.saveDraft()

    wx.showLoading({ title: '上传中...' })
    try {
      const result = await ocrIdCard(path, 'Back')
      this.setUploadedImage('idCardBack', path, result.mediaId)
      this.saveDraft()
      logger.info('[MerchantRegister] 身份证反面上传成功', result, 'onIdCardBackUpload')
      this.handleDocumentOCRSubmission('身份证反面', 'id_card_back_ocr', result, (ocr) => {
        if (ocr) {
          this.setData({
            'formData.idCardValidity': ocr.valid_date || ''
          })
          this.saveDraft()
          wx.showToast({ title: '识别成功', icon: 'success' })
        }
      }, {
        documentType: 'id_card',
        side: 'Back'
      })
    } catch (error: unknown) {
      wx.hideLoading()
      logger.error('[MerchantRegister] 身份证反面上传失败', error, 'onIdCardBackUpload')
      const errMsg = getErrorMessage(error, '上传失败，请重试')
      wx.showToast({ title: errMsg, icon: 'none', duration: 3000 })
    }
  },
  onIdCardBackRemove() {
    this.removeUploadedDocument('idCardBack')
  },

  // 开户许可证
  onAccountPermitUpload(e: WechatMiniprogram.CustomEvent) {
    const { path } = e.detail
    const files = path ? [{ url: path }] : []
    this.setData({ accountPermitImages: files })
    this.saveDraft()
  },
  onAccountPermitRemove() {
    this.setData({ accountPermitImages: [] })
    this.saveDraft()
  },

  // ==================== 图片加载错误重试 ====================

  async onImageError(e: WechatMiniprogram.CustomEvent) {
    const { rawUrl, retryCount } = e.detail
    if (!rawUrl) return

    console.log('[MerchantRegister] 图片加载失败，重新签名:', rawUrl, 'retryCount:', retryCount)

    try {
      // 根据 rawUrl 找到对应的图片数组并更新
      const imageArrays = ['licenseImages', 'foodLicenseImages', 'idCardFrontImages', 'idCardBackImages'] as const
      for (const arrayName of imageArrays) {
        const arr = this.data[arrayName] as ImageFieldItem[]
        for (let i = 0; i < arr.length; i++) {
          if (arr[i].rawUrl === rawUrl) {
            const assetId = arr[i].assetId
            const newSignedUrl = typeof assetId === 'number'
              ? await getPrivateMediaUrl(assetId)
              : ''
            const newArr = [...arr]
            newArr[i] = { ...newArr[i], url: newSignedUrl }
            this.setData({ [arrayName]: newArr } as Record<string, unknown>)
            console.log('[MerchantRegister] 已更新签名 URL:', arrayName, i)
            return
          }
        }
      }
    } catch (error) {
      logger.error('[MerchantRegister] 重新签名失败', error)
    }
  },

  // 刷新门头照/环境照签名 URL
  async refreshShopImageUrls() {
    // 768 require REMOVED

    // 刷新门头照
    const storefrontImages = [...this.data.storefrontImages]
    for (let i = 0; i < storefrontImages.length; i++) {
      const img = storefrontImages[i]
      if (img.rawUrl) {
        try {
          const newUrl = getMediaDisplayUrl(img.rawUrl)
          if (!newUrl) continue
          storefrontImages[i] = { ...img, url: newUrl }
        } catch (e) {
          console.warn('[MerchantRegister] 刷新门头照签名失败:', img.rawUrl)
        }
      }
    }

    // 刷新环境照
    const environmentImages = [...this.data.environmentImages]
    for (let i = 0; i < environmentImages.length; i++) {
      const img = environmentImages[i]
      if (img.rawUrl) {
        try {
          const newUrl = getMediaDisplayUrl(img.rawUrl)
          if (!newUrl) continue
          environmentImages[i] = { ...img, url: newUrl }
        } catch (e) {
          console.warn('[MerchantRegister] 刷新环境照签名失败:', img.rawUrl)
        }
      }
    }

    this.setData({ storefrontImages, environmentImages })
    console.log('[MerchantRegister] 已刷新门头照/环境照签名')
  },

  // 字段失去焦点时保存
  onFieldBlur(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    const field = e.currentTarget.dataset.field as string
    const value = e.detail.value

    if (field && typeof field === 'string') {
      // 使用 setData 回调确保数据更新后再同步到后端
      this.setData({ [`formData.${field}`]: value }, () => {
        this.saveDraft()
        this.syncToBackend()
      })
    }
  },

  // ==================== 门头照/环境照上传 ====================

  async onStorefrontImageUpload(e: WechatMiniprogram.CustomEvent) {
    // t-upload bind:add 传递的是 files 数组
    const files = e.detail.files as Array<{ url: string }>
    if (!files || files.length === 0) {
      console.warn('[MerchantRegister] 门头照上传: 无文件', e.detail)
      return
    }

    // 取最后一个新添加的文件
    const newFile = files[files.length - 1]
    if (!newFile?.url) {
      console.warn('[MerchantRegister] 门头照上传: 文件无 URL', newFile)
      return
    }

    const currentImages = [...this.data.storefrontImages]
    if (currentImages.length >= 3) {
      wx.showToast({ title: '最多上传3张门头照', icon: 'none' })
      return
    }

    console.log('[MerchantRegister] 门头照上传开始:', newFile.url)
    wx.showLoading({ title: '上传中...' })
    try {
      // 842-843 require REMOVED

      const result = await uploadMerchantImage(newFile.url, 'storefront')
      console.log('[MerchantRegister] 门头照上传响应:', JSON.stringify(result))

      const displayUrl = result.displayUrl
      console.log('[MerchantRegister] 门头照显示 URL:', displayUrl)

      currentImages.push({ url: displayUrl, rawUrl: displayUrl, assetId: result.mediaId })
      this.setData({ storefrontImages: currentImages })

      // 保存到后端（使用相对路径）
      await updateMerchantImages({
        storefront_images: currentImages.map((img) => img.rawUrl || img.url)
      })

      wx.hideLoading()
      wx.showToast({ title: '上传成功', icon: 'success' })
      this.saveDraft()
    } catch (error) {
      wx.hideLoading()
      logger.error('[MerchantRegister] 门头照上传失败', error)
      wx.showToast({ title: '上传失败', icon: 'none' })
    }
  },

  async onStorefrontImageRemove(e: WechatMiniprogram.CustomEvent) {
    const { index } = e.detail
    const removedImage = this.data.storefrontImages[index]
    const images = [...this.data.storefrontImages]
    images.splice(index, 1)
    this.setData({ storefrontImages: images })

    try {
      await updateMerchantImages({
        storefront_images: images.map((img) => img.rawUrl || img.url)
      })

      if (removedImage?.assetId) {
        try {
          await deleteMediaAsset(removedImage.assetId)
        } catch (deleteError) {
          logger.warn('[MerchantRegister] 门头照 media asset 软删除失败', deleteError)
        }
      }

      this.saveDraft()
    } catch (error) {
      logger.error('[MerchantRegister] 删除门头照失败', error)
    }
  },

  async onEnvironmentImageUpload(e: WechatMiniprogram.CustomEvent) {
    // t-upload bind:add 传递的是 files 数组
    const files = e.detail.files as Array<{ url: string }>
    if (!files || files.length === 0) {
      console.warn('[MerchantRegister] 环境照上传: 无文件', e.detail)
      return
    }

    // 取最后一个新添加的文件
    const newFile = files[files.length - 1]
    if (!newFile?.url) {
      console.warn('[MerchantRegister] 环境照上传: 文件无 URL', newFile)
      return
    }

    const currentImages = [...this.data.environmentImages]
    if (currentImages.length >= 5) {
      wx.showToast({ title: '最多上传5张环境照', icon: 'none' })
      return
    }

    console.log('[MerchantRegister] 环境照上传开始:', newFile.url)
    wx.showLoading({ title: '上传中...' })
    try {
      // 918-919 require REMOVED

      const result = await uploadMerchantImage(newFile.url, 'environment')
      console.log('[MerchantRegister] 环境照上传响应:', JSON.stringify(result))

      const displayUrl = result.displayUrl
      console.log('[MerchantRegister] 环境照显示 URL:', displayUrl)

      currentImages.push({ url: displayUrl, rawUrl: displayUrl, assetId: result.mediaId })
      this.setData({ environmentImages: currentImages })

      // 保存到后端（使用相对路径）
      await updateMerchantImages({
        environment_images: currentImages.map((img) => img.rawUrl || img.url)
      })

      wx.hideLoading()
      wx.showToast({ title: '上传成功', icon: 'success' })
      this.saveDraft()
    } catch (error) {
      wx.hideLoading()
      logger.error('[MerchantRegister] 环境照上传失败', error)
      wx.showToast({ title: '上传失败', icon: 'none' })
    }
  },

  async onEnvironmentImageRemove(e: WechatMiniprogram.CustomEvent) {
    const { index } = e.detail
    const removedImage = this.data.environmentImages[index]
    const images = [...this.data.environmentImages]
    images.splice(index, 1)
    this.setData({ environmentImages: images })

    try {
      await updateMerchantImages({
        environment_images: images.map((img) => img.rawUrl || img.url)
      })

      if (removedImage?.assetId) {
        try {
          await deleteMediaAsset(removedImage.assetId)
        } catch (deleteError) {
          logger.warn('[MerchantRegister] 环境照 media asset 软删除失败', deleteError)
        }
      }

      this.saveDraft()
    } catch (error) {
      logger.error('[MerchantRegister] 删除环境照失败', error)
    }
  },

  // ==================== 提交申请 ====================

  // ==================== 校验逻辑 ====================

  // ==================== OCR 轮询 ====================

  pollOcrStatus(fieldKey: OCRFieldKey, callback: (ocr: OCRResult) => void, options?: OCRPollOptions) {
    this.stopOcrPolling(fieldKey)

    let attempts = 0
    const maxAttempts = 150 // 最长约 5 分钟，避免后台处理中时过早停止自动回填
    let retryingQueue = false

    const intervalId = setInterval(async () => {
      attempts++
      try {
        const res = await getMerchantApplication() as MerchantDraftExt
        this.updateOcrProgressMessage(res)
        const ocrData = res[fieldKey]
        if (ocrData?.status === 'done') {
          this.stopOcrPolling(fieldKey)
          this.updateOcrProgressMessage(res)
          callback(ocrData as OCRResult)
          return
        } else if (ocrData?.status === 'failed') {
          this.stopOcrPolling(fieldKey)
          this.updateOcrProgressMessage(res)
          wx.showToast({ title: `识别失败: ${ocrData.error || '未知错误'}`, icon: 'none' })
          return
        }

        if (options?.retryQueue && !ocrData?.status && !retryingQueue) {
          retryingQueue = true
          try {
            await enqueueMerchantApplicationOCRForMedia(
              options.retryQueue.mediaId,
              options.retryQueue.documentType,
              options.retryQueue.side,
              { maxAttempts: 1, retryDelayMs: 0 }
            )
          } catch (error) {
            logger.warn('[MerchantRegister] OCR 建单重试失败', {
              fieldKey,
              mediaId: options.retryQueue.mediaId,
              error
            }, 'pollOcrStatus')
          } finally {
            retryingQueue = false
          }
        }
      } catch (e) {
        console.error('[pollOcrStatus] Error:', e)
      }

      if (attempts >= maxAttempts) {
        this.stopOcrPolling(fieldKey)
      }
    }, 2000)

    activeOcrPollers[fieldKey] = intervalId as unknown as number
  },

  // ==================== 提交申请 ====================

  async onSubmit() {
    if (!this.ensureConsent()) {
      return
    }

    let consentPayload
    try {
      consentPayload = await buildAgreementConsentPayload()
    } catch (e: unknown) {
      wx.showToast({ title: getErrorMessage(e, '协议信息加载失败，请稍后重试'), icon: 'none', duration: 3000 })
      return
    }

    const { formData, isSubmitting, licenseImages, idCardFrontImages, idCardBackImages } = this.data

    // 防止重复提交
    if (isSubmitting) {
      logger.warn('[MerchantRegister] 重复提交，已忽略')
      return
    }

    // ========== 前端校验 ==========
    const missingFields: string[] = []

    // 基本信息
    if (!formData.name?.trim()) missingFields.push('店铺名称')
    if (!formData.phone?.trim()) missingFields.push('联系电话')
    if (!formData.address?.trim()) missingFields.push('店铺地址')

    // 证照图片
    if (!licenseImages || licenseImages.length === 0) missingFields.push('营业执照')
    if (!idCardFrontImages || idCardFrontImages.length === 0) missingFields.push('身份证正面')
    if (!idCardBackImages || idCardBackImages.length === 0) missingFields.push('身份证背面')

    // OCR 识别的信息（可选但建议有）
    if (!formData.creditCode?.trim()) missingFields.push('统一信用代码(OCR)')
    if (!formData.legalPerson?.trim()) missingFields.push('法人姓名(OCR)')
    if (!formData.idCard?.trim()) missingFields.push('身份证号(OCR)')

    if (missingFields.length > 0) {
      const message = missingFields.length <= 3
        ? `请填写: ${missingFields.join('、')}`
        : `还有 ${missingFields.length} 项必填信息未完善`
      wx.showToast({ title: message, icon: 'none', duration: 3000 })
      logger.warn('[MerchantRegister] 校验失败', { missingFields })
      return
    }

    this.setData({ isSubmitting: true })

    try {
      // 1. Sync latest data to backend (prevent "empty merchant name" error)
      await this.syncToBackend()

      // 2. Enter Polling State UI
      this.setData({ currentStep: 5 })

      // 3. Submit Application (自动审核)
      const result = await submitMerchantApplication(consentPayload)
      logger.info('[MerchantRegister] 提交结果', result)

      // 4. 检查审核结果
      if (result.status === 'approved') {
        DraftStorage.clear(DRAFT_KEY)

        // 更新用户角色为商户（后端已授予 MERCHANT 角色）
        const app = getApp<IAppOption>()
        app.globalData.userRole = 'merchant'
        // 商户ID将在商户后台页面加载时从API获取

        wx.showToast({ title: '审核通过', icon: 'success' })
        setTimeout(() => {
          wx.reLaunch({ url: '/pages/merchant/dashboard/index' })
        }, 1000)
        return // 立即返回，不重置 isSubmitting
      } else if (result.status === 'rejected') {
        this.setData({
          currentStep: 4,
          isSubmitting: false,
          'formData.rejectReason': result.reject_reason || ''
        })
        wx.showModal({
          title: '审核未通过',
          content: result.reject_reason || '请检查提交信息',
          showCancel: false
        })
      } else {
        // submitted - 开始轮询
        this.startPollingStatus()
      }
    } catch (err: unknown) {
      logger.error('[MerchantRegister] Submit failed', err)
      const errMsg = getErrorMessage(err, '提交失败，请重试')
      wx.showToast({ title: errMsg, icon: 'none', duration: 3000 })
      this.setData({ isSubmitting: false })
    }
  },

  startPollingStatus() {
    let attempts = 0
    const maxAttempts = 20
    const intervalId = setInterval(async () => {
      attempts++
      try {
        const res = await getMyApplication()
        if (res.status === 'approved') {
          clearInterval(intervalId)
          DraftStorage.clear(DRAFT_KEY)

          // 更新用户角色为商户
          const app = getApp<IAppOption>()
          app.globalData.userRole = 'merchant'

          wx.showModal({
            title: '入驻成功 🎉',
            content: '恭喜您入驻成功！您的店铺现已开通。默认状态为「打烊」，请在商户工作台手动开店。',
            showCancel: false,
            confirmText: '进入工作台',
            success: () => {
              wx.reLaunch({ url: '/pages/merchant/dashboard/index' })
            }
          })
        } else if (res.status === 'rejected') {
          clearInterval(intervalId)
          this.setData({
            currentStep: 4,
            'formData.rejectReason': res.reject_reason || ''
          })
          wx.showModal({
            title: '审核未通过',
            content: res.reject_reason || '请检查提交信息',
            showCancel: false
          })
        }
      } catch (e) {
        console.error('Polling error', e)
      }

      if (attempts >= maxAttempts) {
        clearInterval(intervalId)
        wx.showToast({ title: '提交成功，请稍后查看审核结果', icon: 'none' })
        setTimeout(() => {
          wx.reLaunch({ url: '/pages/merchant/dashboard/index' })
        }, 1500)
      }
    }, 2000)
  },

  // 重置被拒绝的申请
  async onResetApplication() {
    try {
      wx.showLoading({ title: '重置中...' })
      await resetMerchantApplication()
      wx.hideLoading()
      wx.showToast({ title: '已重置，可重新编辑', icon: 'success' })
      this.setData({ currentStep: 1 })
      this.initApplication()
    } catch (e) {
      wx.hideLoading()
      logger.error('[MerchantRegister] Reset failed', e)
      ErrorHandler.handle(e)
    }
  }
})
