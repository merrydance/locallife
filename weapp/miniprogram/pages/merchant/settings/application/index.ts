import {
  buildMerchantApplicationStatusView,
  type MerchantApplicationDraftResponse,
  getMerchantApplication,
  getMyApplication,
  ocrBusinessLicense,
  ocrFoodPermit,
  ocrIdCard,
  resetMerchantApplication,
  submitMerchantApplication,
  type MerchantApplicationOCRSubmissionResult,
  updateMerchantBasicInfo,
  deleteMerchantApplicationDocument
} from '../../../../api/onboarding'
import { buildAgreementConsentPayload } from '../../../../api/agreement-consent'
import { getPrivateMediaUrl } from '../../../../utils/image-security'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'
import { reverseGeocode } from '../../../../api/location'
import {
  APPLICATION_AUTO_REFRESH_WINDOW_MS,
  buildChosenLocationAddress,
  buildLocationLabel,
  buildMerchantApplicationBasicPayload,
  buildMerchantApplicationDraftPatch,
  buildMerchantApplicationOcrMergePatch,
  buildMerchantApplicationSubmitBlockText,
  EMPTY_FORM,
  extractApplicationErrorMessage,
  extractUploadPath,
  getBadgeColor,
  getMerchantApplicationValidationMessage,
  getMerchantApplicationUploadingKey,
  hasApplicationFormChanged,
  resolveDraftPublicAssetUrl,
  shouldAutoRefresh,
  shouldFallbackToLatestApplication,
  type ApplicationForm,
  type ApplicationStatusView,
  type OcrStatus,
  type UploadField,
  type UploadFileItem
} from '../../../../utils/merchant-application-view'

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: false,
    saving: false,
    submitting: false,
    resetting: false,
    applicationId: 0,
    status: 'draft',
    statusView: buildMerchantApplicationStatusView('draft') as ApplicationStatusView,
    statusBadgeText: buildMerchantApplicationStatusView('draft').badgeText,
    statusBadgeColor: getBadgeColor('primary'),
    rejectReason: '',
    actionNoticeMessage: '',
    regionId: 0,
    latitude: '',
    longitude: '',
    locationLabel: '--',
    form: { ...EMPTY_FORM } as ApplicationForm,
    initialForm: { ...EMPTY_FORM } as ApplicationForm,
    hasChanges: false,
    licenseAssetId: 0,
    foodPermitAssetId: 0,
    idCardFrontAssetId: 0,
    idCardBackAssetId: 0,
    licenseImageUrl: '',
    foodPermitImageUrl: '',
    idCardFrontImageUrl: '',
    idCardBackImageUrl: '',
    licenseImage: [] as UploadFileItem[],
    foodPermitImage: [] as UploadFileItem[],
    idCardFrontImage: [] as UploadFileItem[],
    idCardBackImage: [] as UploadFileItem[],
    licenseOcrText: '未上传',
    foodPermitOcrText: '未上传',
    idCardFrontOcrText: '未上传',
    idCardBackOcrText: '未上传',
    licenseOcrStatus: '' as OcrStatus,
    foodPermitOcrStatus: '' as OcrStatus,
    idCardFrontOcrStatus: '' as OcrStatus,
    idCardBackOcrStatus: '' as OcrStatus,
    licenseOcrTheme: 'default',
    foodPermitOcrTheme: 'default',
    idCardFrontOcrTheme: 'default',
    idCardBackOcrTheme: 'default',
    ocrNoticeMessage: '',
    licenseUploading: false,
    foodPermitUploading: false,
    idCardFrontUploading: false,
    idCardBackUploading: false,
    lastLoadedAt: 0
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })

    const accessResult = await ensureMerchantConsoleAccess()
    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : ''
    })
    if (accessResult.status !== 'granted') {
      this.setData({ initialLoading: false })
      return
    }

    this.loadApplication(true, true)
  },

  onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    if (!this.data.initialLoading && !this.data.saving && !this.data.submitting && !this.data.resetting && !this.data.hasChanges) {
      if (shouldAutoRefresh(this.data.lastLoadedAt, APPLICATION_AUTO_REFRESH_WINDOW_MS)) {
        this.loadApplication(false)
      }
    }
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      wx.stopPullDownRefresh()
      return
    }

    if (this.data.hasChanges) {
      wx.stopPullDownRefresh()
      wx.showToast({ title: '当前有未保存修改，请先保存草稿', icon: 'none' })
      return
    }

    this.loadApplication(false, true)
  },

  async loadApplication(showLoading = true, force = false) {
    if (this.data.loading) return

    const hasExistingData = !this.data.initialLoading
    if (!force && hasExistingData && !shouldAutoRefresh(this.data.lastLoadedAt, APPLICATION_AUTO_REFRESH_WINDOW_MS)) {
      wx.stopPullDownRefresh()
      return
    }

    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', actionNoticeMessage: '', refreshErrorMessage: '' }
        : hasExistingData
          ? { refreshErrorMessage: '' }
          : {})
    })

    try {
      const draft = await this.fetchCurrentApplication()
      await this.applyDraftToPage(draft, this.data.hasChanges)
      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
    } catch (error) {
      logger.error('Load merchant application settings failed', error, 'merchant-application-page')
      const message = extractApplicationErrorMessage(error, '商户申请资料加载失败，请稍后重试')
      if (this.data.initialLoading) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  async fetchCurrentApplication() {
    try {
      return await getMerchantApplication()
    } catch (error) {
      if (shouldFallbackToLatestApplication(error)) {
        return await getMyApplication()
      }
      throw error
    }
  },

  async applyDraftToPage(draft: MerchantApplicationDraftResponse, keepDirty: boolean) {
    const [idCardFrontUrl, idCardBackUrl] = await Promise.all([
      this.resolvePrivateAssetUrl(draft.id_card_front_media_asset_id),
      this.resolvePrivateAssetUrl(draft.id_card_back_media_asset_id)
    ])
    const licenseUrl = resolveDraftPublicAssetUrl(draft.business_license_url)
    const foodPermitUrl = resolveDraftPublicAssetUrl(draft.food_permit_url)
    const locationLabel = await this.resolveLocationLabel(draft.latitude, draft.longitude, draft.business_address)

    this.setData(buildMerchantApplicationDraftPatch(draft, {
      form: this.data.form,
      initialForm: this.data.initialForm,
      status: this.data.status,
      rejectReason: this.data.rejectReason,
      regionId: this.data.regionId,
      latitude: this.data.latitude,
      longitude: this.data.longitude,
      licenseAssetId: this.data.licenseAssetId,
      foodPermitAssetId: this.data.foodPermitAssetId,
      idCardFrontAssetId: this.data.idCardFrontAssetId,
      idCardBackAssetId: this.data.idCardBackAssetId,
      licenseImageUrl: this.data.licenseImageUrl,
      foodPermitImageUrl: this.data.foodPermitImageUrl,
      idCardFrontImageUrl: this.data.idCardFrontImageUrl,
      idCardBackImageUrl: this.data.idCardBackImageUrl,
      licenseImage: this.data.licenseImage,
      foodPermitImage: this.data.foodPermitImage,
      idCardFrontImage: this.data.idCardFrontImage,
      idCardBackImage: this.data.idCardBackImage,
      licenseOcrStatus: this.data.licenseOcrStatus,
      foodPermitOcrStatus: this.data.foodPermitOcrStatus,
      idCardFrontOcrStatus: this.data.idCardFrontOcrStatus,
      idCardBackOcrStatus: this.data.idCardBackOcrStatus
    }, {
      locationLabel,
      licenseUrl,
      foodPermitUrl,
      idCardFrontUrl,
      idCardBackUrl
    }, keepDirty))
  },

  async resolveLocationLabel(latitude?: string | null, longitude?: string | null, fallbackAddress?: string | null) {
    const lat = Number(latitude || 0)
    const lng = Number(longitude || 0)
    if (Number.isFinite(lat) && Number.isFinite(lng) && lat && lng) {
      try {
        const geocoded = await reverseGeocode({ latitude: lat, longitude: lng })
        return buildLocationLabel(geocoded.formatted_address || geocoded.address || '')
      } catch (error) {
        logger.warn('Resolve merchant application location label failed', error, 'merchant-application-page')
      }
    }
    return buildLocationLabel(String(fallbackAddress || ''))
  },

  async resolvePrivateAssetUrl(assetId?: number | null) {
    if (!assetId) return ''
    try {
      return await getPrivateMediaUrl(assetId)
    } catch (error) {
      logger.warn('Resolve merchant application private asset failed', error, 'merchant-application-page')
      return ''
    }
  },

  onInputChange(
    e: WechatMiniprogram.CustomEvent<{ value: string }> & {
      currentTarget: { dataset: { field: keyof ApplicationForm } }
    }
  ) {
    const field = e.currentTarget.dataset.field
    if (field === 'businessAddress') {
      return
    }

    const nextForm = {
      ...this.data.form,
      [field]: e.detail.value
    }
    this.setData({
      form: nextForm,
      hasChanges: hasApplicationFormChanged(nextForm, this.data.initialForm),
      actionNoticeMessage: ''
    })
  },

  validateForm(forSubmit = false) {
    const message = getMerchantApplicationValidationMessage({
      form: this.data.form,
      forSubmit,
      licenseAssetId: this.data.licenseAssetId,
      foodPermitAssetId: this.data.foodPermitAssetId,
      idCardFrontAssetId: this.data.idCardFrontAssetId,
      idCardBackAssetId: this.data.idCardBackAssetId,
      licenseImage: this.data.licenseImage,
      foodPermitImage: this.data.foodPermitImage,
      idCardFrontImage: this.data.idCardFrontImage,
      idCardBackImage: this.data.idCardBackImage,
      latitude: this.data.latitude,
      longitude: this.data.longitude,
      regionId: this.data.regionId,
      ocrBlockMessage: this.getOcrSubmitBlockMessage()
    })
    if (message) {
      wx.showToast({ title: message, icon: 'none' })
      return false
    }
    return true
  },

  async persistDraft(showSuccessToast: boolean) {
    if (this.data.saving) return false
    if (!this.validateForm(false)) return false

    this.setData({ saving: true })
    wx.showLoading({ title: '保存中...' })

    try {
      const updated = await updateMerchantBasicInfo(buildMerchantApplicationBasicPayload(this.data.form))
      await this.applyDraftToPage(updated, false)
      if (showSuccessToast) {
        this.setData({ actionNoticeMessage: '草稿已保存。' })
      }
      return true
    } catch (error) {
      logger.error('Save merchant application draft failed', error, 'merchant-application-page')
      wx.showToast({ title: extractApplicationErrorMessage(error, '草稿保存失败，请稍后重试'), icon: 'none' })
      return false
    } finally {
      wx.hideLoading()
      this.setData({ saving: false })
    }
  },

  async onSaveDraft() {
    if (!this.data.statusView.canEdit || !this.data.hasChanges) return
    await this.persistDraft(true)
  },

  async onSubmitApplication() {
    if (this.data.submitting || !this.data.statusView.canSubmit) return
    if (!this.validateForm(true)) return

    let consentPayload
    try {
      consentPayload = await buildAgreementConsentPayload()
    } catch (error) {
      wx.showToast({ title: extractApplicationErrorMessage(error, '协议信息加载失败，请稍后重试'), icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    wx.showLoading({ title: '提交中...' })

    try {
      if (this.data.hasChanges) {
        const saved = await this.persistDraft(false)
        if (!saved) return
        wx.showLoading({ title: '提交中...' })
      }

      const result = await submitMerchantApplication(consentPayload)
      await this.applyDraftToPage(result, false)
      this.setData({ actionNoticeMessage: '' })
    } catch (error) {
      logger.error('Submit merchant application failed', error, 'merchant-application-page')
      wx.showToast({ title: extractApplicationErrorMessage(error, '申请提交失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ submitting: false })
    }
  },

  async onResetApplication() {
    if (this.data.resetting || !this.data.statusView.canReset) return

    const confirmed = await new Promise<boolean>((resolve) => {
      wx.showModal({
        title: '重置申请',
        content: '重置后会回到草稿状态，您可以修改资料后重新提交。是否继续？',
        success: (res) => resolve(!!res.confirm),
        fail: () => resolve(false)
      })
    })
    if (!confirmed) return

    this.setData({ resetting: true })
    wx.showLoading({ title: '重置中...' })
    try {
      const result = await resetMerchantApplication()
      await this.applyDraftToPage(result, false)
      this.setData({ actionNoticeMessage: '已重置为草稿。' })
    } catch (error) {
      logger.error('Reset merchant application failed', error, 'merchant-application-page')
      wx.showToast({ title: extractApplicationErrorMessage(error, '申请重置失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ resetting: false })
    }
  },

  async onLicenseUpload(e: WechatMiniprogram.CustomEvent<{ path?: string, files?: Array<{ url?: string }> }>) { await this.handleDocumentUpload('license', extractUploadPath(e.detail)) },
  onLicenseRemove() { this.handleDocumentRemove('license') },
  async onFoodPermitUpload(e: WechatMiniprogram.CustomEvent<{ path?: string, files?: Array<{ url?: string }> }>) { await this.handleDocumentUpload('foodPermit', extractUploadPath(e.detail)) },
  onFoodPermitRemove() { this.handleDocumentRemove('foodPermit') },
  async onIdCardFrontUpload(e: WechatMiniprogram.CustomEvent<{ path?: string, files?: Array<{ url?: string }> }>) { await this.handleDocumentUpload('idCardFront', extractUploadPath(e.detail)) },
  onIdCardFrontRemove() { this.handleDocumentRemove('idCardFront') },
  async onIdCardBackUpload(e: WechatMiniprogram.CustomEvent<{ path?: string, files?: Array<{ url?: string }> }>) { await this.handleDocumentUpload('idCardBack', extractUploadPath(e.detail)) },
  onIdCardBackRemove() { this.handleDocumentRemove('idCardBack') },

  async handleDocumentRemove(field: UploadField) {
    if (!this.data.statusView.canEdit) {
      wx.showToast({ title: '当前状态不可编辑申请资料', icon: 'none' })
      return
    }

    const documentMap: Record<UploadField, 'business_license' | 'food_permit' | 'id_card_front' | 'id_card_back'> = {
      license: 'business_license',
      foodPermit: 'food_permit',
      idCardFront: 'id_card_front',
      idCardBack: 'id_card_back'
    }

  this.setData({ actionNoticeMessage: '' })
    wx.showLoading({ title: '删除中...' })
    try {
      const updated = await deleteMerchantApplicationDocument(documentMap[field])
      await this.applyDraftToPage(updated, false)
    } catch (error) {
      logger.error('Delete merchant application document failed', { field, error }, 'merchant-application-page')
      wx.showToast({ title: extractApplicationErrorMessage(error, '删除失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  async handleDocumentUpload(field: UploadField, path: string) {
    if (!path) return
    if (!this.data.statusView.canEdit) {
      wx.showToast({ title: '当前状态不可编辑申请资料', icon: 'none' })
      return
    }

    const uploadingKey = getMerchantApplicationUploadingKey(field)
    this.setData({
      [uploadingKey]: true,
      actionNoticeMessage: ''
    })
    wx.showLoading({ title: '证照识别中' })

    try {
      let submissionResult: MerchantApplicationOCRSubmissionResult
      if (field === 'license') {
        submissionResult = await ocrBusinessLicense(path)
      } else if (field === 'foodPermit') {
        submissionResult = await ocrFoodPermit(path)
      } else if (field === 'idCardFront') {
        submissionResult = await ocrIdCard(path, 'Front')
      } else {
        submissionResult = await ocrIdCard(path, 'Back')
      }

      if (!submissionResult) {
        throw new Error('证照上传失败，请稍后重试')
      }

      await this.mergeOcrDraft(field, submissionResult.draft, path)
    } catch (error) {
      logger.error('Upload merchant application document failed', error, 'merchant-application-page')
      wx.showToast({ title: extractApplicationErrorMessage(error, '证照上传或识别失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ [uploadingKey]: false })
    }
  },

  async mergeOcrDraft(
    field: UploadField,
    draft: MerchantApplicationDraftResponse,
    fallbackPath: string
  ) {
    const [idCardFrontUrl, idCardBackUrl] = await Promise.all([
      draft.id_card_front_media_asset_id ? this.resolvePrivateAssetUrl(draft.id_card_front_media_asset_id) : Promise.resolve(''),
      draft.id_card_back_media_asset_id ? this.resolvePrivateAssetUrl(draft.id_card_back_media_asset_id) : Promise.resolve('')
    ])
    const licenseUrl = resolveDraftPublicAssetUrl(draft.business_license_url)
    const foodPermitUrl = resolveDraftPublicAssetUrl(draft.food_permit_url)
    this.setData(buildMerchantApplicationOcrMergePatch(field, draft, {
      form: this.data.form,
      initialForm: this.data.initialForm,
      status: this.data.status,
      rejectReason: this.data.rejectReason,
      regionId: this.data.regionId,
      latitude: this.data.latitude,
      longitude: this.data.longitude,
      licenseAssetId: this.data.licenseAssetId,
      foodPermitAssetId: this.data.foodPermitAssetId,
      idCardFrontAssetId: this.data.idCardFrontAssetId,
      idCardBackAssetId: this.data.idCardBackAssetId,
      licenseImageUrl: this.data.licenseImageUrl,
      foodPermitImageUrl: this.data.foodPermitImageUrl,
      idCardFrontImageUrl: this.data.idCardFrontImageUrl,
      idCardBackImageUrl: this.data.idCardBackImageUrl,
      licenseImage: this.data.licenseImage,
      foodPermitImage: this.data.foodPermitImage,
      idCardFrontImage: this.data.idCardFrontImage,
      idCardBackImage: this.data.idCardBackImage,
      licenseOcrStatus: this.data.licenseOcrStatus,
      foodPermitOcrStatus: this.data.foodPermitOcrStatus,
      idCardFrontOcrStatus: this.data.idCardFrontOcrStatus,
      idCardBackOcrStatus: this.data.idCardBackOcrStatus
    }, {
      locationLabel: buildLocationLabel(draft.business_address || this.data.form.businessAddress),
      licenseUrl,
      foodPermitUrl,
      idCardFrontUrl,
      idCardBackUrl
    }, fallbackPath))

  },

  onPreviewImage(e: WechatMiniprogram.BaseEvent<{ url?: string, urls?: string[] }>) {
    const url = e.currentTarget?.dataset?.url || ''
    const urls = (e.currentTarget?.dataset?.urls || []).filter(Boolean)
    if (!url) return

    wx.previewImage({
      current: url,
      urls: urls.length ? urls : [url]
    })
  },

  getOcrSubmitBlockMessage() {
    const checks = [
      { label: '营业执照', status: this.data.licenseOcrStatus },
      { label: '食品经营许可证', status: this.data.foodPermitOcrStatus },
      { label: '身份证正面', status: this.data.idCardFrontOcrStatus },
      { label: '身份证背面', status: this.data.idCardBackOcrStatus }
    ]

    return buildMerchantApplicationSubmitBlockText(checks)
  },

  onChooseLocation() {
    if (!this.data.statusView.canEdit) {
      wx.showToast({ title: '当前状态不可编辑申请资料', icon: 'none' })
      return
    }

    wx.chooseLocation({
      success: async (result) => {
        const previousForm = this.data.form
        const previousHasChanges = this.data.hasChanges
        const geocoded = await reverseGeocode({ latitude: result.latitude, longitude: result.longitude }).catch(() => null)
        const fullAddress = buildChosenLocationAddress(result, geocoded?.formatted_address || geocoded?.address || '')
        const nextForm = { ...previousForm }

        this.setData({
          form: nextForm,
          latitude: String(result.latitude),
          longitude: String(result.longitude),
          locationLabel: buildLocationLabel(fullAddress),
          hasChanges: hasApplicationFormChanged(nextForm, this.data.initialForm),
          actionNoticeMessage: ''
        })

        wx.showLoading({ title: '保存位置中...' })
        try {
          const updated = await updateMerchantBasicInfo({
            latitude: String(result.latitude),
            longitude: String(result.longitude)
          })
          await this.applyDraftToPage(updated, true)
        } catch (error) {
          this.setData({
            form: previousForm,
            hasChanges: previousHasChanges
          })
          logger.error('Update merchant application location failed', error, 'merchant-application-page')
          wx.showToast({ title: extractApplicationErrorMessage(error, '位置保存失败，请稍后重试'), icon: 'none' })
        } finally {
          wx.hideLoading()
        }
      },
      fail: (error) => {
        if (typeof error?.errMsg === 'string' && error.errMsg.includes('auth deny')) {
          wx.showModal({
            title: '需要位置权限',
            content: '请在设置中开启位置权限后再选择经营位置。',
            confirmText: '去设置',
            success: (result) => {
              if (result.confirm) {
                wx.openSetting()
              }
            }
          })
          return
        }
        if (typeof error?.errMsg === 'string' && error.errMsg.includes('cancel')) {
          return
        }
        wx.showToast({ title: '位置选择失败，请稍后重试', icon: 'none' })
      }
    })
  },

  onRetry() {
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    if (!this.data.accessReady || this.data.accessDenied) return
    this.loadApplication(true, true)
  },

  onRetryRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return

    if (this.data.hasChanges) {
      wx.showToast({ title: '当前有未保存修改，请先保存草稿', icon: 'none' })
      return
    }

    this.loadApplication(false, true)
  },

  onRetryAccess() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: ''
    })
    this.onLoad()
  }
})