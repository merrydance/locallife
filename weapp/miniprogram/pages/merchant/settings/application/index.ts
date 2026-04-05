import {
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
import { getMediaDisplayUrl } from '../../../../utils/media'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorDebugMessage, getErrorUserMessage } from '../../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'

type ApplicationForm = {
  merchantName: string
  contactPhone: string
  businessAddress: string
  businessLicenseNumber: string
  businessScope: string
  legalPersonName: string
  legalPersonIdNumber: string
}

type UploadFileItem = {
  url: string
  name: string
}

type UploadField = 'license' | 'foodPermit' | 'idCardFront' | 'idCardBack'

type OcrStatus = 'pending' | 'processing' | 'done' | 'failed' | ''

const EMPTY_FORM: ApplicationForm = {
  merchantName: '',
  contactPhone: '',
  businessAddress: '',
  businessLicenseNumber: '',
  businessScope: '',
  legalPersonName: '',
  legalPersonIdNumber: ''
}

const APPLICATION_AUTO_REFRESH_WINDOW_MS = 60 * 1000

function extractErrorMessage(error: unknown, fallback: string) {
  return getErrorUserMessage(error, fallback)
}

function shouldFallbackToLatest(error: unknown) {
  const message = getErrorDebugMessage(error).toLowerCase()
  return message.includes('409')
    || message.includes('冲突')
    || message.includes('submitted')
    || message.includes('approved')
    || message.includes('已提交')
    || message.includes('已通过')
}

function buildForm(draft: MerchantApplicationDraftResponse): ApplicationForm {
  return {
    merchantName: draft.merchant_name || draft.business_license_ocr?.enterprise_name || '',
    contactPhone: draft.contact_phone || '',
    businessAddress: draft.business_address || '',
    businessLicenseNumber: draft.business_license_number || draft.business_license_ocr?.reg_num || draft.business_license_ocr?.credit_code || '',
    businessScope: draft.business_scope || draft.business_license_ocr?.business_scope || '',
    legalPersonName: draft.legal_person_name || draft.id_card_front_ocr?.name || draft.business_license_ocr?.legal_representative || '',
    legalPersonIdNumber: draft.legal_person_id_number || draft.id_card_front_ocr?.id_number || ''
  }
}

function hasFormChanged(current: ApplicationForm, initial: ApplicationForm) {
  return current.merchantName !== initial.merchantName
    || current.contactPhone !== initial.contactPhone
    || current.businessAddress !== initial.businessAddress
    || current.businessLicenseNumber !== initial.businessLicenseNumber
    || current.businessScope !== initial.businessScope
    || current.legalPersonName !== initial.legalPersonName
    || current.legalPersonIdNumber !== initial.legalPersonIdNumber
}

function getStatusText(status: string) {
  const map: Record<string, string> = {
    draft: '草稿中',
    submitted: '审核中',
    approved: '已通过',
    rejected: '已驳回'
  }
  return map[status] || status || '草稿中'
}

function getStatusTheme(status: string) {
  switch (status) {
    case 'approved':
      return 'success'
    case 'rejected':
      return 'danger'
    case 'submitted':
      return 'warning'
    default:
      return 'primary'
  }
}

function getStatusGuide(status: string) {
  switch (status) {
    case 'submitted':
      return '申请已提交，审核通过后继续完成收付通进件。'
    case 'approved':
      return '主体已通过，可继续完成收付通进件和签约。'
    case 'rejected':
      return '申请已驳回，请按驳回原因修改后重新提交。'
    default:
      return '填写主体资料并上传证照，确认定位后提交审核。'
  }
}

function getEditTip(status: string) {
  if (status === 'approved') {
    return '已通过申请如需修改，保存或上传后会自动回到草稿状态'
  }
  return canEdit(status) ? '可保存草稿，提交前请确认无误' : '当前状态不可编辑'
}

function canEdit(status: string) {
  return status === 'draft' || status === 'rejected' || status === 'approved' || !status
}

function canSubmit(status: string) {
  return status === 'draft' || status === 'rejected' || status === 'approved' || !status
}

function canReset(status: string) {
  return status === 'rejected'
}

function extractUploadPath(detail: { path?: string, files?: Array<{ url?: string }> }) {
  if (detail?.path) return detail.path
  const latestFile = detail?.files?.[detail.files.length - 1]
  return latestFile?.url || ''
}

function buildLocationLabel(address: string, latitude?: string | null, longitude?: string | null) {
  if (address.trim()) return address.trim()
  if (latitude && longitude) return `坐标 ${latitude}, ${longitude}`
  return '未选择经营位置'
}

function buildLocationHint(regionId?: number | null, latitude?: string | null, longitude?: string | null) {
  if (regionId) {
    return '位置已保存，已匹配经营区域。'
  }
  if (latitude && longitude) {
    return '位置已保存，暂未匹配经营区域，请重新选择更准确的位置。'
  }
  return '提交前请完成定位，系统会自动匹配经营区域。'
}

function buildChosenLocationAddress(result: WechatMiniprogram.ChooseLocationSuccessCallbackResult) {
  const address = result.address || ''
  const name = result.name || ''
  if (address && name) {
    return address.includes(name) ? address : `${address} ${name}`
  }
  return address || name || ''
}

function resolveDraftPublicAssetUrl(url?: string | null) {
  return getMediaDisplayUrl(url || '')
}

function isPendingOcrStatus(status?: string) {
  return status === 'pending' || status === 'processing'
}

function buildOcrNoticeMessage(statuses: OcrStatus[]) {
  const hasPendingStatus = statuses.some((status) => isPendingOcrStatus(status))
  if (!hasPendingStatus) {
    return ''
  }

  return '部分证照仍在识别中，请等待本次识别完成后再提交审核。'
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

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
    rejectReason: '',
    updatedAtLabel: '--',
    actionNoticeMessage: '',
    regionId: 0,
    latitude: '',
    longitude: '',
    locationLabel: '未选择经营位置',
    locationHint: '提交审核前必须完成定位，后端会据此匹配经营区域。',
    form: { ...EMPTY_FORM } as ApplicationForm,
    initialForm: { ...EMPTY_FORM } as ApplicationForm,
    hasChanges: false,
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
      await this.applyDraftToPage(draft, false)
      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
    } catch (error) {
      logger.error('Load merchant application settings failed', error, 'merchant-application-page')
      const message = extractErrorMessage(error, '商户申请资料加载失败，请稍后重试')
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
      if (shouldFallbackToLatest(error)) {
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

    const form = keepDirty ? this.data.form : buildForm(draft)
    const initialForm = buildForm(draft)

    const ocrStatuses = [
      (draft.business_license_ocr?.status || '') as OcrStatus,
      (draft.food_permit_ocr?.status || '') as OcrStatus,
      (draft.id_card_front_ocr?.status || '') as OcrStatus,
      (draft.id_card_back_ocr?.status || '') as OcrStatus
    ]

    this.setData({
      applicationId: draft.id,
      status: draft.status || 'draft',
      rejectReason: draft.reject_reason || '',
      updatedAtLabel: draft.updated_at ? draft.updated_at.replace('T', ' ').slice(0, 16) : '--',
      regionId: draft.region_id || 0,
      latitude: draft.latitude || '',
      longitude: draft.longitude || '',
      locationLabel: buildLocationLabel(draft.business_address || form.businessAddress, draft.latitude, draft.longitude),
      locationHint: buildLocationHint(draft.region_id, draft.latitude, draft.longitude),
      form,
      initialForm,
      hasChanges: keepDirty ? hasFormChanged(this.data.form, initialForm) : false,
      licenseImage: licenseUrl ? [{ url: licenseUrl, name: '营业执照' }] : [],
      foodPermitImage: foodPermitUrl ? [{ url: foodPermitUrl, name: '食品经营许可证' }] : [],
      idCardFrontImage: idCardFrontUrl ? [{ url: idCardFrontUrl, name: '身份证正面' }] : [],
      idCardBackImage: idCardBackUrl ? [{ url: idCardBackUrl, name: '身份证背面' }] : [],
      licenseOcrText: this.getOcrStatusText(draft.business_license_ocr?.status),
      foodPermitOcrText: this.getOcrStatusText(draft.food_permit_ocr?.status),
      idCardFrontOcrText: this.getOcrStatusText(draft.id_card_front_ocr?.status),
      idCardBackOcrText: this.getOcrStatusText(draft.id_card_back_ocr?.status),
      licenseOcrStatus: (draft.business_license_ocr?.status || '') as OcrStatus,
      foodPermitOcrStatus: (draft.food_permit_ocr?.status || '') as OcrStatus,
      idCardFrontOcrStatus: (draft.id_card_front_ocr?.status || '') as OcrStatus,
      idCardBackOcrStatus: (draft.id_card_back_ocr?.status || '') as OcrStatus,
      ocrNoticeMessage: buildOcrNoticeMessage(ocrStatuses)
    })
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
      hasChanges: hasFormChanged(nextForm, this.data.initialForm),
      actionNoticeMessage: ''
    })
  },

  validateForm(forSubmit = false) {
    const { form } = this.data
    if (!form.merchantName.trim()) {
      wx.showToast({ title: '请填写店铺名称', icon: 'none' })
      return false
    }
    if (!form.contactPhone.trim() || form.contactPhone.trim().length !== 11) {
      wx.showToast({ title: '请填写 11 位联系电话', icon: 'none' })
      return false
    }
    if (!form.businessAddress.trim() || form.businessAddress.trim().length < 5) {
      wx.showToast({ title: '请填写完整经营地址', icon: 'none' })
      return false
    }

    if (!forSubmit) return true

    if (!form.businessLicenseNumber.trim()) {
      wx.showToast({ title: '请上传营业执照并补齐统一信用代码', icon: 'none' })
      return false
    }
    if (!form.legalPersonName.trim() || !form.legalPersonIdNumber.trim()) {
      wx.showToast({ title: '请上传身份证并补齐法人信息', icon: 'none' })
      return false
    }
    if (!this.data.licenseImage.length || !this.data.foodPermitImage.length || !this.data.idCardFrontImage.length || !this.data.idCardBackImage.length) {
      wx.showToast({ title: '请先上传营业执照、食品经营许可证和身份证正反面', icon: 'none' })
      return false
    }
    if (!this.data.latitude || !this.data.longitude) {
      wx.showToast({ title: '请先选择经营位置', icon: 'none' })
      return false
    }
    if (!this.data.regionId) {
      wx.showToast({ title: '当前位置还未匹配到经营区域，请重新选择更准确的位置', icon: 'none' })
      return false
    }
    const ocrMessage = this.getOcrSubmitBlockMessage()
    if (ocrMessage) {
      wx.showToast({ title: ocrMessage, icon: 'none' })
      return false
    }
    return true
  },

  buildBasicPayload() {
    const { form } = this.data
    return {
      merchant_name: form.merchantName.trim(),
      contact_phone: form.contactPhone.trim(),
      business_address: form.businessAddress.trim(),
      business_license_number: form.businessLicenseNumber.trim() || undefined,
      business_scope: form.businessScope.trim() || undefined,
      legal_person_name: form.legalPersonName.trim() || undefined,
      legal_person_id_number: form.legalPersonIdNumber.trim() || undefined
    }
  },

  async persistDraft(showSuccessToast: boolean) {
    if (this.data.saving) return false
    if (!this.validateForm(false)) return false

    this.setData({ saving: true })
    wx.showLoading({ title: '保存中...' })

    try {
      const updated = await updateMerchantBasicInfo(this.buildBasicPayload())
      await this.applyDraftToPage(updated, false)
      if (showSuccessToast) {
        this.setData({ actionNoticeMessage: '草稿已保存。' })
      }
      return true
    } catch (error) {
      logger.error('Save merchant application draft failed', error, 'merchant-application-page')
      wx.showToast({ title: extractErrorMessage(error, '草稿保存失败，请稍后重试'), icon: 'none' })
      return false
    } finally {
      wx.hideLoading()
      this.setData({ saving: false })
    }
  },

  async onSaveDraft() {
    if (!canEdit(this.data.status) || !this.data.hasChanges) return
    await this.persistDraft(true)
  },

  async onSubmitApplication() {
    if (this.data.submitting || !canSubmit(this.data.status)) return
    if (!this.validateForm(true)) return

    let consentPayload
    try {
      consentPayload = await buildAgreementConsentPayload()
    } catch (error) {
      wx.showToast({ title: extractErrorMessage(error, '协议信息加载失败，请稍后重试'), icon: 'none' })
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
      wx.showToast({ title: extractErrorMessage(error, '申请提交失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ submitting: false })
    }
  },

  async onResetApplication() {
    if (this.data.resetting || !canReset(this.data.status)) return

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
      wx.showToast({ title: extractErrorMessage(error, '申请重置失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ resetting: false })
    }
  },

  async onLicenseUpload(e: WechatMiniprogram.CustomEvent<{ path?: string, files?: Array<{ url?: string }> }>) {
    await this.handleDocumentUpload('license', extractUploadPath(e.detail))
  },

  onLicenseRemove() {
    this.handleDocumentRemove('license')
  },

  async onFoodPermitUpload(e: WechatMiniprogram.CustomEvent<{ path?: string, files?: Array<{ url?: string }> }>) {
    await this.handleDocumentUpload('foodPermit', extractUploadPath(e.detail))
  },

  onFoodPermitRemove() {
    this.handleDocumentRemove('foodPermit')
  },

  async onIdCardFrontUpload(e: WechatMiniprogram.CustomEvent<{ path?: string, files?: Array<{ url?: string }> }>) {
    await this.handleDocumentUpload('idCardFront', extractUploadPath(e.detail))
  },

  onIdCardFrontRemove() {
    this.handleDocumentRemove('idCardFront')
  },

  async onIdCardBackUpload(e: WechatMiniprogram.CustomEvent<{ path?: string, files?: Array<{ url?: string }> }>) {
    await this.handleDocumentUpload('idCardBack', extractUploadPath(e.detail))
  },

  onIdCardBackRemove() {
    this.handleDocumentRemove('idCardBack')
  },

  async handleDocumentRemove(field: UploadField) {
    if (!canEdit(this.data.status)) {
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
      wx.showToast({ title: extractErrorMessage(error, '删除失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  async handleDocumentUpload(field: UploadField, path: string) {
    if (!path) return
    if (!canEdit(this.data.status)) {
      wx.showToast({ title: '当前状态不可编辑申请资料', icon: 'none' })
      return
    }

    const uploadingKey = this.getUploadingKey(field)
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
      wx.showToast({ title: extractErrorMessage(error, '证照上传或识别失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ [uploadingKey]: false })
    }
  },

  getUploadingKey(field: UploadField) {
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
  },

  async mergeOcrDraft(
    field: UploadField,
    draft: MerchantApplicationDraftResponse,
    fallbackPath: string
  ) {
    const nextForm = { ...this.data.form }

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

    const [idCardFrontUrl, idCardBackUrl] = await Promise.all([
      draft.id_card_front_media_asset_id ? this.resolvePrivateAssetUrl(draft.id_card_front_media_asset_id) : Promise.resolve(''),
      draft.id_card_back_media_asset_id ? this.resolvePrivateAssetUrl(draft.id_card_back_media_asset_id) : Promise.resolve('')
    ])
    const licenseUrl = resolveDraftPublicAssetUrl(draft.business_license_url)
    const foodPermitUrl = resolveDraftPublicAssetUrl(draft.food_permit_url)

    const ocrStatuses = [
      (draft.business_license_ocr?.status || this.data.licenseOcrStatus || '') as OcrStatus,
      (draft.food_permit_ocr?.status || this.data.foodPermitOcrStatus || '') as OcrStatus,
      (draft.id_card_front_ocr?.status || this.data.idCardFrontOcrStatus || '') as OcrStatus,
      (draft.id_card_back_ocr?.status || this.data.idCardBackOcrStatus || '') as OcrStatus
    ]

    this.setData({
      status: draft.status || this.data.status,
      rejectReason: draft.reject_reason || this.data.rejectReason,
      updatedAtLabel: draft.updated_at ? draft.updated_at.replace('T', ' ').slice(0, 16) : this.data.updatedAtLabel,
      regionId: draft.region_id || this.data.regionId,
      latitude: draft.latitude || this.data.latitude,
      longitude: draft.longitude || this.data.longitude,
      locationLabel: buildLocationLabel(draft.business_address || nextForm.businessAddress, draft.latitude || this.data.latitude, draft.longitude || this.data.longitude),
      locationHint: buildLocationHint(draft.region_id || this.data.regionId, draft.latitude || this.data.latitude, draft.longitude || this.data.longitude),
      form: nextForm,
      hasChanges: hasFormChanged(nextForm, this.data.initialForm),
      licenseImage: (licenseUrl || field === 'license') ? [{ url: licenseUrl || fallbackPath, name: '营业执照' }] : this.data.licenseImage,
      foodPermitImage: (foodPermitUrl || field === 'foodPermit') ? [{ url: foodPermitUrl || fallbackPath, name: '食品经营许可证' }] : this.data.foodPermitImage,
      idCardFrontImage: (idCardFrontUrl || field === 'idCardFront') ? [{ url: idCardFrontUrl || fallbackPath, name: '身份证正面' }] : this.data.idCardFrontImage,
      idCardBackImage: (idCardBackUrl || field === 'idCardBack') ? [{ url: idCardBackUrl || fallbackPath, name: '身份证背面' }] : this.data.idCardBackImage,
      licenseOcrText: this.getOcrStatusText(draft.business_license_ocr?.status),
      foodPermitOcrText: this.getOcrStatusText(draft.food_permit_ocr?.status),
      idCardFrontOcrText: this.getOcrStatusText(draft.id_card_front_ocr?.status),
      idCardBackOcrText: this.getOcrStatusText(draft.id_card_back_ocr?.status),
      licenseOcrStatus: (draft.business_license_ocr?.status || this.data.licenseOcrStatus || '') as OcrStatus,
      foodPermitOcrStatus: (draft.food_permit_ocr?.status || this.data.foodPermitOcrStatus || '') as OcrStatus,
      idCardFrontOcrStatus: (draft.id_card_front_ocr?.status || this.data.idCardFrontOcrStatus || '') as OcrStatus,
      idCardBackOcrStatus: (draft.id_card_back_ocr?.status || this.data.idCardBackOcrStatus || '') as OcrStatus,
      ocrNoticeMessage: buildOcrNoticeMessage(ocrStatuses)
    })

  },

  getOcrStatusText(status?: string) {
    switch (status) {
      case 'done':
        return '识别完成'
      case 'processing':
        return '识别中'
      case 'failed':
        return '识别失败'
      case 'pending':
        return '待识别'
      default:
        return '未上传'
    }
  },

  getOcrSubmitBlockMessage() {
    const checks = [
      { label: '营业执照', status: this.data.licenseOcrStatus },
      { label: '食品经营许可证', status: this.data.foodPermitOcrStatus },
      { label: '身份证正面', status: this.data.idCardFrontOcrStatus },
      { label: '身份证背面', status: this.data.idCardBackOcrStatus }
    ]

    for (const item of checks) {
      if (item.status === 'done') continue
      if (item.status === 'processing' || item.status === 'pending') {
        return `${item.label}仍在识别中，请等待识别完成后再提交`
      }
      if (item.status === 'failed') {
        return `${item.label}识别失败，请提供更清晰更规整的图片重试`
      }
      return `${item.label}识别结果未就绪，请重新上传后再试`
    }

    return ''
  },

  onChooseLocation() {
    if (!canEdit(this.data.status)) {
      wx.showToast({ title: '当前状态不可编辑申请资料', icon: 'none' })
      return
    }

    wx.chooseLocation({
      success: async (result) => {
        const previousForm = this.data.form
        const previousHasChanges = this.data.hasChanges
        const fullAddress = buildChosenLocationAddress(result)
        const nextForm = {
          ...previousForm,
          businessAddress: fullAddress || previousForm.businessAddress
        }

        this.setData({
          form: nextForm,
          hasChanges: hasFormChanged(nextForm, this.data.initialForm),
          actionNoticeMessage: ''
        })

        wx.showLoading({ title: '保存位置中...' })
        try {
          const updated = await updateMerchantBasicInfo({
            business_address: nextForm.businessAddress.trim() || undefined,
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
          wx.showToast({ title: extractErrorMessage(error, '位置保存失败，请稍后重试'), icon: 'none' })
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

  onGoProfileImages() {
    wx.navigateTo({ url: '/pages/merchant/profile-images/index' })
  },

  onGoApplyment() {
    wx.navigateTo({ url: '/pages/merchant/settings/applyment/index' })
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
  },

  getStatusText(status: string) {
    return getStatusText(status)
  },

  getStatusTheme(status: string) {
    return getStatusTheme(status)
  },

  getStatusGuide(status: string) {
    return getStatusGuide(status)
  },

  canEditCurrent(status: string) {
    return canEdit(status)
  },

  getEditTipCurrent(status: string) {
    return getEditTip(status)
  },

  canSubmitCurrent(status: string) {
    return canSubmit(status)
  },

  canResetCurrent(status: string) {
    return canReset(status)
  }
})