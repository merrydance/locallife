import { logger } from './logger'
import { ErrorHandler } from './error-handler'
import { DraftStorage } from './draft-storage'
import {
  buildMerchantApplicationOCRStatusView,
  buildMerchantApplicationStatusView,
  getMerchantApplication,
  updateMerchantBasicInfo,
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
  waitForPublicMediaDisplayUrl,
  type MerchantApplicationOCRSubmissionResult,
  type MerchantApplicationDraftResponse
} from '../api/onboarding'
import { getPrivateMediaUrl } from './image-security'
import { getMediaDisplayUrl } from './media'
import { getErrorDebugMessage, getErrorUserMessage } from './user-facing'
import Navigation from './navigation'
import { buildAgreementConsentPayload } from '../api/agreement-consent'
import { getCurrentRegion, reverseGeocode, searchRegions } from '../api/location'
import {
  DEFAULT_MERCHANT_OCR_DISPLAY_STATE,
  DEFAULT_MERCHANT_UPLOAD_FEEDBACK,
  buildMapLocationLabel,
  buildMerchantBusinessLicenseOcrRecognizedPatch,
  buildMerchantFoodPermitOcrRecognizedPatch,
  buildMerchantIdCardBackOcrRecognizedPatch,
  buildMerchantIdCardFrontOcrRecognizedPatch,
  buildMerchantInitialDraftFormPatch,
  buildMerchantInitialDraftOcrResults,
  buildMerchantInitialShopImagesPatch,
  buildMerchantLatestOcrFormPatch,
  buildMerchantOcrDisplayState,
  buildMerchantOcrProgressMessage,
  buildMerchantShopImagesPatch,
  buildMerchantShopImagesPayload,
  buildMerchantUploadErrorFeedback,
  buildMerchantUploadFeedback,
  buildMerchantUploadProcessingFeedback,
  buildMerchantUploadedImagePatch,
  buildRegionSearchKeywords,
  buildUploadRenderImages,
  getMerchantShopImageFilesFieldName,
  getMerchantShopImageImagesFieldName,
  getMerchantStoreRegistrationDocumentRemovalTarget,
  markImagesPersisted,
  pickBestRegionSearchResult,
  toSafeNumber,
  type ImageFieldItem,
  type MerchantDraftExt,
  type MerchantOCRDisplayState,
  type MerchantRegistrationUploadField,
  type MerchantShopImageKind,
  type MerchantUploadFeedback,
  type UploadFeedback
} from './merchant-store-registration-view'

export {
  DEFAULT_MERCHANT_OCR_DISPLAY_STATE,
  DEFAULT_MERCHANT_UPLOAD_FEEDBACK,
  type ImageFieldItem,
  type MerchantOCRDisplayState,
  type MerchantUploadFeedback,
  type UploadFeedback
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type MerchantStoreRegistrationPageContext = WechatMiniprogram.Page.Instance<Record<string, any>, Record<string, any>> & Record<string, any>

const DRAFT_KEY = 'merchant_register_draft'

export type OCRResult = {
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

export type UploadField = MerchantRegistrationUploadField

type OCRFieldKey = 'business_license_ocr' | 'food_permit_ocr' | 'id_card_front_ocr' | 'id_card_back_ocr'

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

const getErrorMessage = getErrorUserMessage

function isMerchantCorrectionError(message: string): boolean {
  return [
    '营业执照',
    '食品经营许可证',
    '身份证',
    '过期',
    '不一致',
    '未识别',
    '位置',
    '坐标距离过近',
    '经营范围'
  ].some((keyword) => message.includes(keyword))
}

function toSafeString(value: unknown): string {
  if (value === null || value === undefined || value === true || value === 'true') {
    return ''
  }
  return String(value)
}

export const merchantStoreRegistrationRuntimeMethods: Record<string, unknown> & ThisType<MerchantStoreRegistrationPageContext> = {
  async onLoad() {
    await this.initApplication()
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

      this.setData({ applicationInitialized: true })

      const statusView = buildMerchantApplicationStatusView(data.status)
      if (statusView.isApproved) {
        wx.reLaunch({ url: '/pages/merchant/dashboard/index' })
        return
      }
      if (statusView.isSubmitted) {
        wx.showToast({ title: '申请审核中', icon: 'none' })
        this.setData({ currentStep: 5 })
        this.startPollingStatus()
        return
      }

      const formData = {
        ...this.data.formData,
        ...buildMerchantInitialDraftFormPatch(data)
      }

      const ocrResults = buildMerchantInitialDraftOcrResults(data)

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

      const licenseImages = licenseUrl ? [{ url: licenseUrl, assetId: data.business_license_media_asset_id ?? undefined }] : []
      const foodLicenseImages = foodLicenseUrl ? [{ url: foodLicenseUrl, assetId: data.food_permit_media_asset_id ?? undefined }] : []
      const idCardFrontImages = idCardFrontUrl ? [{ url: idCardFrontUrl, rawUrl: buildPrivateAssetKey(data.id_card_front_media_asset_id), assetId: data.id_card_front_media_asset_id ?? undefined }] : []
      const idCardBackImages = idCardBackUrl ? [{ url: idCardBackUrl, rawUrl: buildPrivateAssetKey(data.id_card_back_media_asset_id), assetId: data.id_card_back_media_asset_id ?? undefined }] : []
      const accountPermitImages: Array<{ url: string }> = []
      const shopImagesPatch = buildMerchantInitialShopImagesPatch({
        data,
        resolveDisplayUrl: getMediaDisplayUrl
      })

      console.log('[DEBUG] setData payload:', {
        formData,
        licenseImages: licenseImages.length,
        storefrontImages: shopImagesPatch.storefrontImages.length,
        environmentImages: shopImagesPatch.environmentImages.length
      })

      this.setData({
        formData,
        ocrDisplayState: this.buildMerchantOcrDisplayState(data),
        uploadFeedback: this.buildMerchantUploadFeedback(data),
        ocrResults,
        licenseImages,
        foodLicenseImages,
        idCardFrontImages,
        idCardBackImages,
        accountPermitImages,
        ...shopImagesPatch
      })

      logger.debug('[MerchantRegister] initApplication 完成', formData, 'initApplication')
      wx.hideLoading()
    } catch (e: unknown) {
      wx.hideLoading()
      console.error('[MerchantRegister] initApplication Error:', e)
      wx.showModal({
        title: '加载失败',
        content: getErrorMessage(e, '无法加载申请数据，请检查网络后重试'),
        confirmText: '重试',
        cancelText: '返回',
        success: (res) => {
          if (res.confirm) {
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

  setShopImages(kind: MerchantShopImageKind, images: ImageFieldItem[]) {
    const currentFilesFieldName = getMerchantShopImageFilesFieldName(kind)
    const currentFiles = [...this.data[currentFilesFieldName]] as ImageFieldItem[]

    this.setData(buildMerchantShopImagesPatch({ kind, images, currentFiles }) as Record<string, unknown>)
  },

  buildShopImagesPayload(kind: MerchantShopImageKind, images: ImageFieldItem[]) {
    return buildMerchantShopImagesPayload({
      kind,
      images,
      storefrontImages: this.data.storefrontImages,
      environmentImages: this.data.environmentImages
    })
  },

  applyLatestOcrDraft(data: MerchantDraftExt) {
    this.setData({
      formData: {
        ...this.data.formData,
        ...buildMerchantLatestOcrFormPatch(data, this.data.formData.address)
      },
      ocrDisplayState: this.buildMerchantOcrDisplayState(data),
      uploadFeedback: this.buildMerchantUploadFeedback(data),
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
    const target = getMerchantStoreRegistrationDocumentRemovalTarget(field)

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
    return buildMerchantOcrProgressMessage({
      data,
      hasBusinessLicenseImage: this.data.licenseImages.length > 0,
      hasFoodPermitImage: this.data.foodLicenseImages.length > 0,
      hasIdCardFrontImage: this.data.idCardFrontImages.length > 0,
      hasIdCardBackImage: this.data.idCardBackImages.length > 0
    })
  },

  buildMerchantOcrDisplayState(data?: MerchantDraftExt): MerchantOCRDisplayState {
    return buildMerchantOcrDisplayState({
      data,
      hasBusinessLicenseImage: this.data.licenseImages.length > 0,
      hasFoodPermitImage: this.data.foodLicenseImages.length > 0,
      hasIdCardFrontImage: this.data.idCardFrontImages.length > 0,
      hasIdCardBackImage: this.data.idCardBackImages.length > 0
    })
  },

  buildMerchantUploadFeedback(data?: MerchantDraftExt): MerchantUploadFeedback {
    return buildMerchantUploadFeedback({
      data,
      hasBusinessLicenseImage: this.data.licenseImages.length > 0,
      hasFoodPermitImage: this.data.foodLicenseImages.length > 0,
      hasIdCardFrontImage: this.data.idCardFrontImages.length > 0,
      hasIdCardBackImage: this.data.idCardBackImages.length > 0
    })
  },

  updateOcrProgressMessage(data?: MerchantDraftExt) {
    this.setData({
      ocrProgressMessage: this.buildOcrProgressMessage(data),
      ocrDisplayState: this.buildMerchantOcrDisplayState(data),
      uploadFeedback: this.buildMerchantUploadFeedback(data)
    })
  },

  setUploadFeedback(field: keyof MerchantUploadFeedback, feedback: UploadFeedback) {
    this.setData({ [`uploadFeedback.${field}`]: feedback })
  },

  setUploadedImage(field: UploadField, path: string, assetId?: number) {
    this.setData(buildMerchantUploadedImagePatch(field, path, assetId))
  },

  handleDocumentOCRSubmission(
    fieldKey: OCRFieldKey,
    result: MerchantApplicationOCRSubmissionResult,
    onRecognized: (ocr: OCRResult) => void
  ) {
    const latestDraft = result.draft as MerchantDraftExt
    const latestOCR = latestDraft[fieldKey]
    this.updateOcrProgressMessage(latestDraft)
    if (buildMerchantApplicationOCRStatusView(latestOCR?.status).isReady) {
      onRecognized(latestOCR as OCRResult)
    }
  },

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

  async syncToBackend(): Promise<MerchantApplicationDraftResponse | null> {
    if (!this.data.applicationInitialized) return null
    const { formData } = this.data
    const merchantName = formData.name?.trim() || formData.licenseName?.trim()
    const contactPhone = formData.phone?.trim()
    const businessAddress = formData.address?.trim() || formData.registerAddress?.trim()
    const regionId = toSafeNumber(formData.regionId)

    const hasSyncableFields = Boolean(
      merchantName
      || contactPhone
      || businessAddress
      || formData.longitude
      || formData.latitude
      || regionId
    )

    if (!hasSyncableFields) {
      logger.debug('[MerchantRegister] syncToBackend skipped: no syncable fields')
      return null
    }

    try {
      const payload = {
        merchant_name: merchantName || undefined,
        contact_phone: contactPhone || undefined,
        business_address: businessAddress || undefined,
        longitude: formData.longitude ? String(formData.longitude) : undefined,
        latitude: formData.latitude ? String(formData.latitude) : undefined,
        region_id: regionId || undefined
      }

      console.log('[MerchantRegister] syncToBackend payload:', JSON.stringify(payload, null, 2))
      const updated = await updateMerchantBasicInfo(payload)

      if (updated.region_id && Number(updated.region_id) > 0 && Number(updated.region_id) !== regionId) {
        this.setData({ 'formData.regionId': Number(updated.region_id) })
      }

      const storefrontImages = this.data.storefrontImages || []
      const environmentImages = this.data.environmentImages || []
      if (storefrontImages.length > 0 || environmentImages.length > 0) {
        await updateMerchantImages(buildMerchantShopImagesPayload({
          kind: 'storefront',
          images: storefrontImages,
          storefrontImages,
          environmentImages
        }))
      }

      console.log('[MerchantRegister] Sync to backend success')
      return updated
    } catch (err: unknown) {
      console.error('[MerchantRegister] Sync to backend failed', err)
      const errData = err as { originalError?: unknown, data?: unknown, message?: string }
      console.error('[MerchantRegister] Error details:', errData.originalError || errData.data || errData.message)
      return null
    }
  },

  async resolveRegionIdByLocation(latitude?: number, longitude?: number): Promise<number> {
    const lat = Number(latitude)
    const lng = Number(longitude)

    if (!Number.isFinite(lat) || !Number.isFinite(lng) || !lat || !lng) {
      return 0
    }

    try {
      const region = await getCurrentRegion({ latitude: lat, longitude: lng })
      const resolvedRegionId = Number(region?.region_id || 0)

      if (resolvedRegionId && resolvedRegionId !== toSafeNumber(this.data.formData.regionId)) {
        this.setData({ 'formData.regionId': resolvedRegionId })
      }

      return resolvedRegionId
    } catch (error) {
      logger.warn('[MerchantRegister] 坐标解析所属区域失败', error)
      return 0
    }
  },

  async resolveRegionIdByAddressText(address?: string): Promise<number> {
    const rawAddress = (address || '').trim()
    if (!rawAddress) {
      return 0
    }

    const keywords = buildRegionSearchKeywords(rawAddress)
    for (const keyword of keywords) {
      try {
        const regions = await searchRegions(keyword)
        const matchedRegion = pickBestRegionSearchResult(regions || [], rawAddress)
        if (!matchedRegion?.id) {
          continue
        }

        const resolvedRegionId = Number(matchedRegion.id)
        if (resolvedRegionId && resolvedRegionId !== toSafeNumber(this.data.formData.regionId)) {
          this.setData({ 'formData.regionId': resolvedRegionId })
        }
        return resolvedRegionId
      } catch (error) {
        logger.warn('[MerchantRegister] 地址文本解析所属区域失败', { keyword, error })
      }
    }

    return 0
  },

  async resolveAndSyncRegionId(latitude?: number, longitude?: number, address?: string): Promise<number> {
    const resolvedRegionId = await this.resolveRegionIdByLocation(latitude, longitude)
    const fallbackRegionId = resolvedRegionId || await this.resolveRegionIdByAddressText(address)
    if (!fallbackRegionId) {
      return 0
    }

    const syncedDraft = await this.syncToBackend()
    return Number(syncedDraft?.region_id || fallbackRegionId)
  },

  loadDraft() {
    const draft = DraftStorage.load(DRAFT_KEY) as DraftData | null
    if (draft) {
      const safeFormData = { ...this.data.formData }

      const draftFormData = draft.formData || {}
      if (draftFormData) {
        Object.keys(safeFormData).forEach((key) => {
          let val = draftFormData[key]

          if (val === null || val === undefined) {
            val = ''
          }
          if (val === true || val === 'true') {
            val = ''
          }

          if (key === 'latitude' || key === 'longitude') {
            (safeFormData as Record<string, unknown>)[key] = Number(val) || 0
          } else if (key === 'regionId') {
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
        storefrontImages: draft.storefrontImages || [],
        storefrontFiles: buildUploadRenderImages(draft.storefrontImages || []),
        environmentImages: draft.environmentImages || [],
        environmentFiles: buildUploadRenderImages(draft.environmentImages || []),
        shopImages: draft.shopImages || [],
        ocrResults: draft.ocrResults || { license: null, idCard: null }
      })
    }
  },

  updateFormData(key: string, value: unknown) {
    this.setData({ [`formData.${key}`]: value })
    this.saveDraft()
  },

  onNameInput(e: WechatMiniprogram.Input) { this.updateFormData('name', e.detail.value) },
  onPhoneInput(e: WechatMiniprogram.Input) {
    const value = e.detail.value || ''
    const nextData: Record<string, unknown> = { 'formData.phone': value }
    if (value.trim()) {
      nextData.phoneError = ''
    }
    this.setData(nextData)
    this.saveDraft()
  },
  onAddressDetailInput(e: WechatMiniprogram.Input) { this.updateFormData('addressDetail', e.detail.value) },
  onLicenseNameInput(e: WechatMiniprogram.Input) { this.updateFormData('licenseName', e.detail.value) },
  onCreditCodeInput(e: WechatMiniprogram.Input) { this.updateFormData('creditCode', e.detail.value) },
  onRegisterAddressInput(e: WechatMiniprogram.Input) { this.updateFormData('registerAddress', e.detail.value) },
  onLicenseValidityInput(e: WechatMiniprogram.Input) { this.updateFormData('licenseValidity', e.detail.value) },
  onBusinessScopeInput(e: WechatMiniprogram.Input) { this.updateFormData('businessScope', e.detail.value) },
  onFoodLicenseValidityInput(e: WechatMiniprogram.Input) { this.updateFormData('foodLicenseValidity', e.detail.value) },
  onLegalPersonInput(e: WechatMiniprogram.Input) { this.updateFormData('legalPerson', e.detail.value) },
  onIdCardInput(e: WechatMiniprogram.Input) { this.updateFormData('idCard', e.detail.value) },
  onGenderInput(e: WechatMiniprogram.Input) { this.updateFormData('gender', e.detail.value) },
  onHometownInput(e: WechatMiniprogram.Input) { this.updateFormData('hometown', e.detail.value) },
  onCurrentAddressInput(e: WechatMiniprogram.Input) { this.updateFormData('currentAddress', e.detail.value) },
  onIdCardValidityInput(e: WechatMiniprogram.Input) { this.updateFormData('idCardValidity', e.detail.value) },
  onBankNameInput(e: WechatMiniprogram.Input) { this.updateFormData('bankName', e.detail.value) },
  onBankAccountInput(e: WechatMiniprogram.Input) { this.updateFormData('bankAccount', e.detail.value) },
  onAccountNameInput(e: WechatMiniprogram.Input) { this.updateFormData('accountName', e.detail.value) },
  onAddressInput(e: WechatMiniprogram.Input) { this.updateFormData('address', e.detail.value) },

  onChooseAddress() {
    wx.chooseLocation({
      success: async (res) => {
        const addr = res.address || ''
        const name = res.name || ''
        const geocoded = await reverseGeocode({ latitude: res.latitude, longitude: res.longitude }).catch(() => null)
        const locationLabel = buildMapLocationLabel({
          geocodedAddress: geocoded?.formatted_address || geocoded?.address,
          chosenAddress: addr,
          chosenName: name,
          latitude: res.latitude,
          longitude: res.longitude
        })

        console.log('[ChooseLocation] Final Location Label:', locationLabel, { res, geocoded })

        this.setData({
          'formData.addressDetail': locationLabel,
          'formData.regionId': 0,
          'formData.latitude': res.latitude,
          'formData.longitude': res.longitude
        }, () => {
          this.saveDraft()
          void (async () => {
            const resolvedRegionId = await this.resolveAndSyncRegionId(res.latitude, res.longitude, geocoded?.formatted_address || geocoded?.address || addr)
            if (!resolvedRegionId) {
              await this.syncToBackend()
            }
          })()
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

    if (currentStep === 0) {
      if (!this.ensureConsent()) return
      this.syncToBackend()
      this.setData({ currentStep: 1 })
      return
    }

    if (currentStep === 1) {
      if (licenseImages.length === 0 || foodLicenseImages.length === 0 || idCardFrontImages.length === 0 || idCardBackImages.length === 0) {
        wx.showToast({ title: '请上传所有必需的证照', icon: 'none' })
        return
      }
      try {
        const latestDraft = await getMerchantApplication() as MerchantDraftExt
        this.applyLatestOcrDraft(latestDraft)
        this.setData({ currentStep: 2 })
        return
      } catch (error) {
        wx.showToast({ title: getErrorMessage(error, '申请数据加载失败，请重试'), icon: 'none', duration: 3000 })
        return
      }
    }

    if (currentStep === 2) {
      let latestDraft: MerchantDraftExt | null = null
      try {
        latestDraft = await getMerchantApplication() as MerchantDraftExt
        this.applyLatestOcrDraft(latestDraft)
      } catch (error) {
        logger.warn('[MerchantRegister] 刷新 OCR 草稿失败', error, 'nextStep')
      }

      const currentFormData = this.data.formData
      const mergedCreditCode = currentFormData.creditCode || toSafeString(latestDraft?.business_license_number || latestDraft?.business_license_ocr?.reg_num || latestDraft?.business_license_ocr?.credit_code)
      const mergedLegalPerson = currentFormData.legalPerson || toSafeString(latestDraft?.id_card_front_ocr?.name || latestDraft?.legal_person_name || latestDraft?.business_license_ocr?.legal_representative)
      const mergedIdCard = currentFormData.idCard || toSafeString(latestDraft?.id_card_front_ocr?.id_number || latestDraft?.legal_person_id_number)

      const missingOCRFields: string[] = []

      if (!mergedCreditCode?.trim()) missingOCRFields.push('统一信用代码')
      if (!mergedLegalPerson?.trim()) missingOCRFields.push('法人姓名')
      if (!mergedIdCard?.trim()) missingOCRFields.push('身份证号')

      if (missingOCRFields.length > 0) {
        wx.showToast({ title: `请补全: ${missingOCRFields.join('、')}`, icon: 'none', duration: 3000 })
        return
      }

      this.refreshShopImageUrls()
      this.setData({ currentStep: 3 })
      return
    }

    if (currentStep === 3) {
      const { address, latitude, longitude, phone } = this.data.formData
      const normalizedPhone = (phone || '').trim()
      if (!normalizedPhone || normalizedPhone.length !== 11) {
        this.setData({ phoneError: '请填写 11 位联系电话，方便平台联系门店' })
        wx.showToast({ title: '请输入11位联系电话', icon: 'none' })
        return
      }
      this.setData({ phoneError: '' })
      if (!address) {
        wx.showToast({ title: '请选择店铺地址', icon: 'none' })
        return
      }
      if (!latitude || !longitude) {
        wx.showToast({ title: '请通过地图选择精确位置', icon: 'none' })
        return
      }

      let resolvedRegionId = await this.resolveAndSyncRegionId(latitude, longitude, this.data.formData.addressDetail || address)

      if (!resolvedRegionId) {
        const syncedDraft = await this.syncToBackend()
        resolvedRegionId = Number(syncedDraft?.region_id || this.data.formData.regionId || 0)
      }

      if (!resolvedRegionId) {
        try {
          const latestDraft = await getMerchantApplication() as MerchantDraftExt
          resolvedRegionId = Number(latestDraft.region_id || 0)
        } catch (error) {
          logger.warn('[MerchantRegister] 第三步回读草稿失败', error)
        }
      }

      if (!resolvedRegionId) {
        wx.showToast({ title: '未识别到所属区域，请重新选择店铺位置', icon: 'none', duration: 3000 })
        return
      }

      if (resolvedRegionId !== toSafeNumber(this.data.formData.regionId)) {
        this.setData({ 'formData.regionId': resolvedRegionId })
      }

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

  async onLicenseUpload(e: WechatMiniprogram.CustomEvent) {
    const { path } = e.detail
    if (!path) return

    if (!this.data.applicationInitialized) {
      wx.showToast({ title: '请等待页面加载完成', icon: 'none' })
      return
    }

    this.setUploadedImage('license', path)
    this.saveDraft()

    this.setUploadFeedback('license', buildMerchantUploadProcessingFeedback())
    try {
      const result = await ocrBusinessLicense(path)
      this.setUploadedImage('license', path, result.mediaId)
      this.saveDraft()
      logger.info('[MerchantRegister] 营业执照上传成功', result, 'onLicenseUpload')
      this.handleDocumentOCRSubmission('business_license_ocr', result, (ocr: OCRResult) => {
        if (ocr) {
          this.setData(buildMerchantBusinessLicenseOcrRecognizedPatch(ocr, this.data.formData.address))
          this.saveDraft()
        }
      })
    } catch (error: unknown) {
      logger.error('[MerchantRegister] 营业执照上传失败', error, 'onLicenseUpload')
      const errMsg = getErrorMessage(error, '上传失败，请重试')
      this.setUploadFeedback('license', buildMerchantUploadErrorFeedback(errMsg))
    }
  },

  onLicenseRemove() {
    this.removeUploadedDocument('license')
  },

  async onFoodLicenseUpload(e: WechatMiniprogram.CustomEvent) {
    const { path } = e.detail
    if (!path) return

    if (!this.data.applicationInitialized) {
      wx.showToast({ title: '请等待页面加载完成', icon: 'none' })
      return
    }

    this.setUploadedImage('foodPermit', path)
    this.saveDraft()

    this.setUploadFeedback('foodPermit', buildMerchantUploadProcessingFeedback())
    try {
      const result = await ocrFoodPermit(path)
      this.setUploadedImage('foodPermit', path, result.mediaId)
      this.saveDraft()
      logger.info('[MerchantRegister] 食品许可证上传成功', result, 'onFoodLicenseUpload')
      this.handleDocumentOCRSubmission('food_permit_ocr', result, (ocr: OCRResult) => {
        if (ocr) {
          this.setData(buildMerchantFoodPermitOcrRecognizedPatch(ocr))
          this.saveDraft()
        }
      })
    } catch (error: unknown) {
      logger.error('[MerchantRegister] 食品许可证上传失败', error, 'onFoodLicenseUpload')
      const errMsg = getErrorMessage(error, '上传失败，请重试')
      this.setUploadFeedback('foodPermit', buildMerchantUploadErrorFeedback(errMsg))
    }
  },

  onFoodLicenseRemove() {
    this.removeUploadedDocument('foodPermit')
  },

  async onIdCardFrontUpload(e: WechatMiniprogram.CustomEvent) {
    const { path } = e.detail
    if (!path) return

    if (!this.data.applicationInitialized) {
      wx.showToast({ title: '请等待页面加载完成', icon: 'none' })
      return
    }

    this.setUploadedImage('idCardFront', path)
    this.saveDraft()

    this.setUploadFeedback('idCardFront', buildMerchantUploadProcessingFeedback())
    try {
      const result = await ocrIdCard(path, 'Front')
      this.setUploadedImage('idCardFront', path, result.mediaId)
      this.saveDraft()
      logger.info('[MerchantRegister] 身份证正面上传成功', result, 'onIdCardFrontUpload')
      this.handleDocumentOCRSubmission('id_card_front_ocr', result, (ocr: OCRResult) => {
        if (ocr) {
          this.setData(buildMerchantIdCardFrontOcrRecognizedPatch(ocr))
          this.saveDraft()
        }
      })
    } catch (error: unknown) {
      logger.error('[MerchantRegister] 身份证正面上传失败', error, 'onIdCardFrontUpload')
      const errMsg = getErrorMessage(error, '上传失败，请重试')
      this.setUploadFeedback('idCardFront', buildMerchantUploadErrorFeedback(errMsg))
    }
  },

  onIdCardFrontRemove() {
    this.removeUploadedDocument('idCardFront')
  },

  async onIdCardBackUpload(e: WechatMiniprogram.CustomEvent) {
    const { path } = e.detail
    if (!path) return

    if (!this.data.applicationInitialized) {
      wx.showToast({ title: '请等待页面加载完成', icon: 'none' })
      return
    }

    this.setUploadedImage('idCardBack', path)
    this.saveDraft()

    this.setUploadFeedback('idCardBack', buildMerchantUploadProcessingFeedback())
    try {
      const result = await ocrIdCard(path, 'Back')
      this.setUploadedImage('idCardBack', path, result.mediaId)
      this.saveDraft()
      logger.info('[MerchantRegister] 身份证反面上传成功', result, 'onIdCardBackUpload')
      this.handleDocumentOCRSubmission('id_card_back_ocr', result, (ocr: OCRResult) => {
        if (ocr) {
          this.setData(buildMerchantIdCardBackOcrRecognizedPatch(ocr))
          this.saveDraft()
        }
      })
    } catch (error: unknown) {
      logger.error('[MerchantRegister] 身份证反面上传失败', error, 'onIdCardBackUpload')
      const errMsg = getErrorMessage(error, '上传失败，请重试')
      this.setUploadFeedback('idCardBack', buildMerchantUploadErrorFeedback(errMsg))
    }
  },

  onIdCardBackRemove() {
    this.removeUploadedDocument('idCardBack')
  },

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

  async onImageError(e: WechatMiniprogram.CustomEvent) {
    const { rawUrl } = e.detail
    if (!rawUrl) return

    console.log('[MerchantRegister] 图片加载失败，重新签名:', rawUrl, 'retryCount:', e.detail.retryCount)

    try {
      const imageArrays = ['licenseImages', 'foodLicenseImages', 'idCardFrontImages', 'idCardBackImages'] as const
      for (const arrayName of imageArrays) {
        const arr = this.data[arrayName] as ImageFieldItem[]
        for (let i = 0; i < arr.length; i++) {
          if (arr[i].rawUrl === rawUrl) {
            const assetId = arr[i].assetId
            const newSignedUrl = typeof assetId === 'number' ? await getPrivateMediaUrl(assetId) : ''
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

  async refreshShopImageUrls() {
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

    this.setData({
      storefrontImages,
      storefrontFiles: buildUploadRenderImages(storefrontImages, this.data.storefrontFiles),
      environmentImages,
      environmentFiles: buildUploadRenderImages(environmentImages, this.data.environmentFiles)
    })
    console.log('[MerchantRegister] 已刷新门头照/环境照签名')
  },

  onFieldBlur(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    const field = e.currentTarget.dataset.field as string
    const value = e.detail.value

    if (field && typeof field === 'string') {
      this.setData({ [`formData.${field}`]: value }, () => {
        this.saveDraft()
        this.syncToBackend()
      })
    }
  },

  async onStorefrontImageUpload(e: WechatMiniprogram.CustomEvent) {
    if (this.data.storefrontSaving) {
      return
    }

    const files = e.detail.files as Array<{ url: string }>
    if (!files || files.length === 0) {
      console.warn('[MerchantRegister] 门头照上传: 无文件', e.detail)
      return
    }

    const newFile = files[files.length - 1]
    if (!newFile?.url) {
      console.warn('[MerchantRegister] 门头照上传: 文件无 URL', newFile)
      return
    }

    const previousImages = [...this.data.storefrontImages]
    if (previousImages.length >= 3) {
      wx.showToast({ title: '最多上传3张门头照', icon: 'none' })
      return
    }

    const currentImages: ImageFieldItem[] = [...previousImages, {
      url: newFile.url,
      localFileUrl: newFile.url,
      pendingSync: true,
      status: 'loading' as const
    }]

    this.setData({ storefrontSaving: true })
    this.setShopImages('storefront', currentImages)

    console.log('[MerchantRegister] 门头照上传开始:', newFile.url)
    wx.showLoading({ title: '上传中...' })
    try {
      const result = await uploadMerchantImage(newFile.url, 'storefront')
      console.log('[MerchantRegister] 门头照上传响应:', JSON.stringify(result))

      const displayUrl = result.displayUrl || newFile.url
      console.log('[MerchantRegister] 门头照显示 URL:', displayUrl)

      currentImages[currentImages.length - 1] = {
        url: displayUrl,
        rawUrl: result.displayUrl || undefined,
        assetId: result.mediaId,
        localFileUrl: newFile.url,
        pendingSync: true,
        status: 'loading'
      }
      this.setShopImages('storefront', currentImages)

      if (result.displayUrl) {
        const persistedImages = markImagesPersisted(currentImages)
        await updateMerchantImages(this.buildShopImagesPayload('storefront', persistedImages))
        this.setShopImages('storefront', persistedImages)
        this.saveDraft()
      } else {
        void this.finalizePendingShopImage('storefront', currentImages.length - 1, result.mediaId)
      }
    } catch (error) {
      this.setShopImages('storefront', previousImages)
      wx.hideLoading()
      logger.error('[MerchantRegister] 门头照上传失败', error)
      wx.showToast({ title: '上传失败', icon: 'none' })
      this.setData({ storefrontSaving: false })
      return
    }

    wx.hideLoading()
    this.setData({ storefrontSaving: false })
  },

  async onStorefrontImageRemove(e: WechatMiniprogram.CustomEvent) {
    if (this.data.storefrontSaving) {
      return
    }

    const { index } = e.detail
    const removedImage = this.data.storefrontImages[index]
    const nextImages = [...this.data.storefrontImages]
    nextImages.splice(index, 1)

    this.setData({ storefrontSaving: true })
    wx.showLoading({ title: '删除中...' })

    try {
      await updateMerchantImages(this.buildShopImagesPayload('storefront', nextImages))
      this.setShopImages('storefront', nextImages)

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
      wx.showToast({ title: getErrorMessage(error, '删除失败，请重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ storefrontSaving: false })
    }
  },

  async onEnvironmentImageUpload(e: WechatMiniprogram.CustomEvent) {
    if (this.data.environmentSaving) {
      return
    }

    const files = e.detail.files as Array<{ url: string }>
    if (!files || files.length === 0) {
      console.warn('[MerchantRegister] 环境照上传: 无文件', e.detail)
      return
    }

    const newFile = files[files.length - 1]
    if (!newFile?.url) {
      console.warn('[MerchantRegister] 环境照上传: 文件无 URL', newFile)
      return
    }

    const previousImages = [...this.data.environmentImages]
    if (previousImages.length >= 5) {
      wx.showToast({ title: '最多上传5张环境照', icon: 'none' })
      return
    }

    const currentImages: ImageFieldItem[] = [...previousImages, {
      url: newFile.url,
      localFileUrl: newFile.url,
      pendingSync: true,
      status: 'loading' as const
    }]

    this.setData({ environmentSaving: true })
    this.setShopImages('environment', currentImages)

    console.log('[MerchantRegister] 环境照上传开始:', newFile.url)
    wx.showLoading({ title: '上传中...' })
    try {
      const result = await uploadMerchantImage(newFile.url, 'environment')
      console.log('[MerchantRegister] 环境照上传响应:', JSON.stringify(result))

      const displayUrl = result.displayUrl || newFile.url
      console.log('[MerchantRegister] 环境照显示 URL:', displayUrl)

      currentImages[currentImages.length - 1] = {
        url: displayUrl,
        rawUrl: result.displayUrl || undefined,
        assetId: result.mediaId,
        localFileUrl: newFile.url,
        pendingSync: true,
        status: 'loading'
      }
      this.setShopImages('environment', currentImages)

      if (result.displayUrl) {
        const persistedImages = markImagesPersisted(currentImages)
        await updateMerchantImages(this.buildShopImagesPayload('environment', persistedImages))
        this.setShopImages('environment', persistedImages)
        this.saveDraft()
      } else {
        void this.finalizePendingShopImage('environment', currentImages.length - 1, result.mediaId)
      }
    } catch (error) {
      this.setShopImages('environment', previousImages)
      wx.hideLoading()
      logger.error('[MerchantRegister] 环境照上传失败', error)
      wx.showToast({ title: '上传失败', icon: 'none' })
      this.setData({ environmentSaving: false })
      return
    }

    wx.hideLoading()
    this.setData({ environmentSaving: false })
  },

  async onEnvironmentImageRemove(e: WechatMiniprogram.CustomEvent) {
    if (this.data.environmentSaving) {
      return
    }

    const { index } = e.detail
    const removedImage = this.data.environmentImages[index]
    const nextImages = [...this.data.environmentImages]
    nextImages.splice(index, 1)

    this.setData({ environmentSaving: true })
    wx.showLoading({ title: '删除中...' })

    try {
      await updateMerchantImages(this.buildShopImagesPayload('environment', nextImages))
      this.setShopImages('environment', nextImages)

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
      wx.showToast({ title: getErrorMessage(error, '删除失败，请重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ environmentSaving: false })
    }
  },

  async finalizePendingShopImage(kind: 'storefront' | 'environment', index: number, mediaId: number) {
    try {
      const remoteUrl = await waitForPublicMediaDisplayUrl(mediaId)
      if (!remoteUrl) {
        return
      }

      const fieldName = getMerchantShopImageImagesFieldName(kind)
      const currentImages = [...this.data[fieldName]] as ImageFieldItem[]
      const target = currentImages[index]
      if (!target || target.assetId !== mediaId) {
        return
      }

      currentImages[index] = {
        ...target,
        url: remoteUrl,
        rawUrl: remoteUrl,
        assetId: mediaId,
        localFileUrl: target.localFileUrl,
        pendingSync: true,
        status: 'loading'
      }

      this.setShopImages(kind, currentImages)

      const persistedImages = markImagesPersisted(currentImages)

      await updateMerchantImages(this.buildShopImagesPayload(kind, persistedImages))
      this.setShopImages(kind, persistedImages)

      this.saveDraft()
    } catch (error) {
      logger.warn('[MerchantRegister] 等待图片审核通过后持久化失败', { kind, mediaId, error })
    }
  },

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

    if (isSubmitting) {
      logger.warn('[MerchantRegister] 重复提交，已忽略')
      return
    }

    const missingFields: string[] = []

    if (!formData.phone?.trim()) missingFields.push('联系电话')
    if (formData.phone?.trim() && formData.phone.trim().length !== 11) missingFields.push('11位联系电话')
    if (!formData.address?.trim()) missingFields.push('店铺地址')
    if (!toSafeNumber(formData.regionId)) missingFields.push('所属区域')
    if (!licenseImages || licenseImages.length === 0) missingFields.push('营业执照')
    if (!idCardFrontImages || idCardFrontImages.length === 0) missingFields.push('身份证正面')
    if (!idCardBackImages || idCardBackImages.length === 0) missingFields.push('身份证背面')
    if (!formData.creditCode?.trim()) missingFields.push('统一信用代码(OCR)')
    if (!formData.legalPerson?.trim()) missingFields.push('法人姓名(OCR)')
    if (!formData.idCard?.trim()) missingFields.push('身份证号(OCR)')

    if (missingFields.length > 0) {
      const message = missingFields.length <= 3 ? `请填写: ${missingFields.join('、')}` : `还有 ${missingFields.length} 项必填信息未完善`
      wx.showToast({ title: message, icon: 'none', duration: 3000 })
      logger.warn('[MerchantRegister] 校验失败', { missingFields })
      return
    }

    this.setData({ isSubmitting: true })

    try {
      if (!toSafeNumber(this.data.formData.regionId)) {
        const resolvedRegionId = await this.resolveAndSyncRegionId(formData.latitude, formData.longitude, formData.addressDetail || formData.address)
        if (resolvedRegionId && resolvedRegionId !== toSafeNumber(this.data.formData.regionId)) {
          this.setData({ 'formData.regionId': resolvedRegionId })
        }
      }

      await this.syncToBackend()

      const latestDraft = await getMerchantApplication() as MerchantDraftExt
      const latestRegionId = Number(latestDraft.region_id || 0)
      if (!latestRegionId) {
        this.setData({ isSubmitting: false, currentStep: 4 })
        wx.showToast({ title: '所属区域缺失，请重新选择店铺位置', icon: 'none', duration: 3000 })
        return
      }

      if (latestRegionId !== toSafeNumber(this.data.formData.regionId)) {
        this.setData({ 'formData.regionId': latestRegionId })
      }

      this.setData({ currentStep: 5 })

      const result = await submitMerchantApplication(consentPayload)
      logger.info('[MerchantRegister] 提交结果', result)

      const resultStatusView = buildMerchantApplicationStatusView(result.status)
      if (resultStatusView.isApproved) {
        DraftStorage.clear(DRAFT_KEY)

        const app = getApp<IAppOption>()
        app.globalData.userRole = 'merchant'

        wx.reLaunch({ url: '/pages/merchant/dashboard/index' })
        return
      } else if (resultStatusView.isRejected) {
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
        this.startPollingStatus()
      }
    } catch (err: unknown) {
      const errMsg = getErrorMessage(err, '提交失败，请重试')
      const debugMessage = getErrorDebugMessage(err)
      logger.error('[MerchantRegister] Submit failed', {
        error: err,
        userMessage: errMsg,
        debugMessage
      }, 'merchant-register-submit')
      this.setData({ isSubmitting: false, currentStep: 4 })
      wx.showModal({
        title: isMerchantCorrectionError(errMsg) ? '请修改资料后重试' : '提交失败',
        content: errMsg,
        showCancel: false,
        success: () => {
          if (isMerchantCorrectionError(errMsg)) {
            this.setData({ currentStep: 1 })
          }
        }
      })
    }
  },

  startPollingStatus() {
    let attempts = 0
    const maxAttempts = 20
    const intervalId = setInterval(async () => {
      attempts++
      try {
        const res = await getMyApplication()
        const pollingStatusView = buildMerchantApplicationStatusView(res.status)
        if (pollingStatusView.isApproved) {
          clearInterval(intervalId)
          DraftStorage.clear(DRAFT_KEY)

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
        } else if (pollingStatusView.isRejected) {
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

  async onResetApplication() {
    try {
      wx.showLoading({ title: '重置中...' })
      await resetMerchantApplication()
      wx.hideLoading()
      this.setData({ currentStep: 1 })
      void this.initApplication()
    } catch (e) {
      wx.hideLoading()
      logger.error('[MerchantRegister] Reset failed', e)
      ErrorHandler.handle(e)
    }
  }
}
