import { 
  buildRiderApplicationStatusView,
  getOrCreateRiderApplication, 
  updateRiderApplicationBasic, 
  ocrRiderIdCard, 
  ocrRiderHealthCert, 
  submitRiderApplication,
  deleteRiderApplicationDocument,
  type RiderApplicationResponse
} from '../../../api/rider-application'
import { getPrivateMediaUrl } from '../../../utils/image-security'
import { logger } from '../../../utils/logger'
import Navigation from '../../../utils/navigation'
import { buildAgreementConsentPayload } from '../../../api/agreement-consent'
import { buildMerchantApplicationOCRStatusView, type ApplicationStatus } from '../../../api/onboarding'
import { getErrorDebugMessage, getErrorUserMessage } from '../../../utils/user-facing'
import {
  buildRiderOCRPanelState,
  buildRiderOcrDisplayState,
  buildRiderUploadFeedback,
  createUploadFeedback,
  DEFAULT_RIDER_OCR_DISPLAY_STATE,
  DEFAULT_RIDER_OCR_PANEL_STATE,
  DEFAULT_RIDER_UPLOAD_FEEDBACK,
  EMPTY_UPLOAD_FEEDBACK,
  hasHealthCertKeyFields,
  isDocumentCorrectionError,
  isRejectedRiderApplication,
  pickOCRText,
  type OCRDisplayStateValue,
  type RiderOCRDisplayState,
  type RiderUploadFeedback,
  type UploadFeedback,
  type UploadField,
  type UploadFieldValue
} from '../../../utils/rider-register-view'

type UploadEvent = WechatMiniprogram.CustomEvent<{ path?: string }>

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    currentStep: 0,
    isSubmitting: false,
    idFront: { url: '', assetId: undefined } as UploadFieldValue,
    idBack: { url: '', assetId: undefined } as UploadFieldValue,
    healthCert: { url: '', assetId: undefined } as UploadFieldValue,
    ocrDisplayState: DEFAULT_RIDER_OCR_DISPLAY_STATE,
    uploadFeedback: DEFAULT_RIDER_UPLOAD_FEEDBACK,
    formData: {
      realName: '',
      phone: '',
      idNumber: '',
      idValidity: '',
      healthCertNo: '',
      healthCertDate: ''
    },
    phoneError: '',
    consentChecked: false,
    consentPopupVisible: false,
    applicationStatus: 'draft' as ApplicationStatus,
    riderStatusView: buildRiderApplicationStatusView('draft'),
    ocrPanelState: DEFAULT_RIDER_OCR_PANEL_STATE
  },

  previewRefreshVersion: 0,
  documentRequestVersion: {
    idFront: 0,
    idBack: 0,
    healthCert: 0
  } as Record<UploadField, number>,

  async onLoad() {
    await this.initApplication()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async initApplication() {
    wx.showLoading({ title: '加载中...' })
    try {
      const res = await getOrCreateRiderApplication()
      if (res) {
        this.mapResponseToData(res)
        const statusView = buildRiderApplicationStatusView(res.status)
        
        if (statusView.isSubmitted) {
          this.setData({ currentStep: 4, isSubmitting: true })
        } else if (statusView.isApproved) {
          wx.showModal({
            title: '审核通过',
            content: '恭喜！您已正式成为 LocalLife 骑手。',
            showCancel: false,
            success: () => wx.reLaunch({ url: '/pages/rider/dashboard/index' })
          })
        } else if (isRejectedRiderApplication(res)) {
          this.showRejectedApplicationModal(res)
        }
      }
    } catch (e) {
      logger.error('Init rider application failed', e)
    } finally {
      wx.hideLoading()
    }
  },

  mapResponseToData(res: RiderApplicationResponse) {
    const currentForm = this.data.formData
    const nextPhone = res.phone || currentForm.phone || ''
    const idCardOCR = res.id_card_ocr as Record<string, unknown> | undefined
    const healthCertOCR = res.health_cert_ocr as Record<string, unknown> | undefined
    const statusView = buildRiderApplicationStatusView(res.status)
    const ocrDisplayState = buildRiderOcrDisplayState(res, this.data)
    this.setData({
      'formData.realName': res.real_name || pickOCRText(idCardOCR, 'name') || currentForm.realName || '',
      'formData.phone': nextPhone,
      'formData.idNumber': pickOCRText(idCardOCR, 'id_number', 'id_num') || currentForm.idNumber || '',
      'formData.idValidity': pickOCRText(idCardOCR, 'valid_end', 'valid_date', 'valid_period') || currentForm.idValidity || '',
      'formData.healthCertNo': pickOCRText(healthCertOCR, 'cert_number', 'certificate_number', 'certificate') || currentForm.healthCertNo || '',
      'formData.healthCertDate': pickOCRText(healthCertOCR, 'valid_end', 'valid_date', 'valid_period') || currentForm.healthCertDate || '',
      phoneError: nextPhone.trim() ? '' : this.data.phoneError,
      currentStep: statusView.isSubmitted ? 4 : this.data.currentStep,
      applicationStatus: res.status,
      riderStatusView: statusView,
      isSubmitting: statusView.isSubmitted,
      idFront: { url: '', assetId: res.id_card_front_asset_id },
      idBack: { url: '', assetId: res.id_card_back_asset_id },
      healthCert: { url: '', assetId: res.health_cert_asset_id },
      ocrDisplayState,
      ocrPanelState: buildRiderOCRPanelState(ocrDisplayState),
      uploadFeedback: buildRiderUploadFeedback(res, this.data)
    }, () => {
      void this.refreshUploadPreviewURLs()
    })
  },

  showRejectedApplicationModal(res: RiderApplicationResponse) {
    this.setData({
      currentStep: 1,
      isSubmitting: false,
      applicationStatus: 'draft',
      riderStatusView: buildRiderApplicationStatusView('draft')
    })
    wx.showModal({
      title: '申请未通过',
      content: res.reject_reason || '资料核验未通过，请修改后重新提交。',
      confirmText: '修改资料',
      cancelText: '返回',
      success: (modalRes) => {
        if (!modalRes.confirm) {
          wx.navigateBack()
        }
      }
    })
  },

  beginDocumentRequest(field: UploadField): number {
    const nextVersion = (this.documentRequestVersion[field] || 0) + 1
    this.documentRequestVersion[field] = nextVersion
    return nextVersion
  },

  isLatestDocumentRequest(field: UploadField, version: number): boolean {
    return this.documentRequestVersion[field] === version
  },

  applyDocumentResponse(field: UploadField, version: number, res: RiderApplicationResponse) {
    if (!this.isLatestDocumentRequest(field, version)) {
      return
    }

    const currentForm = this.data.formData
    const idCardOCR = res.id_card_ocr as Record<string, unknown> | undefined
    const healthCertOCR = res.health_cert_ocr as Record<string, unknown> | undefined
    const mergedRes: RiderApplicationResponse = {
      ...res,
      id_card_front_asset_id: field === 'idFront' ? res.id_card_front_asset_id : this.data.idFront.assetId,
      id_card_back_asset_id: field === 'idBack' ? res.id_card_back_asset_id : this.data.idBack.assetId,
      health_cert_asset_id: field === 'healthCert' ? res.health_cert_asset_id : this.data.healthCert.assetId
    }

    const nextOCRDisplayState = buildRiderOcrDisplayState(mergedRes, this.data)
    const nextData: Record<string, unknown> = {
      ocrDisplayState: nextOCRDisplayState,
      ocrPanelState: buildRiderOCRPanelState(nextOCRDisplayState),
      uploadFeedback: buildRiderUploadFeedback(mergedRes, this.data)
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

    this.setData(nextData, () => {
      void this.refreshUploadPreviewURLs()
    })
  },

  setOCRState(type: 'identity' | 'health', status: OCRDisplayStateValue) {
    this.setData({ [`ocrDisplayState.${type}`]: status })
  },

  setUploadFeedback(field: keyof RiderUploadFeedback, feedback: UploadFeedback) {
    this.setData({ [`uploadFeedback.${field}`]: feedback })
  },

  isApplicationEditable() {
    return this.data.riderStatusView.isDraft
  },

  ensureApplicationEditable() {
    if (this.isApplicationEditable()) {
      return true
    }

    let message = '当前申请状态暂不支持修改资料'
    if (this.data.riderStatusView.isSubmitted) {
      message = '申请已提交，暂时不能修改资料'
      this.setData({ currentStep: 4, isSubmitting: true })
    } else if (this.data.riderStatusView.isApproved) {
      message = '入驻已通过，无需重复上传资料'
    } else if (this.data.riderStatusView.isRejected) {
      message = '申请已驳回，请先重置后再修改资料'
    }

    wx.showToast({ title: message, icon: 'none' })
    return false
  },

  async resolveUploadPreviewURL(assetId?: number): Promise<string> {
    if (assetId && assetId > 0) {
      try {
        return await getPrivateMediaUrl(assetId)
      } catch (_e) {
        return ''
      }
    }

    return ''
  },

  async refreshUploadPreviewURLs() {
    const refreshVersion = ++this.previewRefreshVersion
    const uploads: Array<{ key: 'idFront' | 'idBack' | 'healthCert', value: UploadFieldValue }> = [
      { key: 'idFront', value: this.data.idFront },
      { key: 'idBack', value: this.data.idBack },
      { key: 'healthCert', value: this.data.healthCert }
    ]

    for (const item of uploads) {
      const assetId = item.value?.assetId
      if (!assetId) continue

      const resolved = await this.resolveUploadPreviewURL(assetId)
      const latestValue = this.data[item.key] as UploadFieldValue | undefined
      if (
        refreshVersion === this.previewRefreshVersion
        && latestValue?.assetId === assetId
        && resolved
        && resolved !== latestValue?.url
      ) {
        this.setData({ [`${item.key}.url`]: resolved })
      }
    }
  },

  async onIdFrontUpload(e: UploadEvent) {
    if (!this.ensureApplicationEditable()) return
    const { path } = e.detail
    if (!path) return
    this.setData({ 'idFront.url': path, 'idFront.rawUrl': path })
    this.processOCR(
      ocrRiderIdCard(path, 'Front'),
      'idFront',
      'identity',
      'idFront'
    )
  },

  async onIdBackUpload(e: UploadEvent) {
    if (!this.ensureApplicationEditable()) return
    const { path } = e.detail
    if (!path) return
    this.setData({ 'idBack.url': path, 'idBack.rawUrl': path })
    this.processOCR(
      ocrRiderIdCard(path, 'Back'),
      'idBack',
      'identity',
      'idBack'
    )
  },

  async onHealthCertUpload(e: UploadEvent) {
    if (!this.ensureApplicationEditable()) return
    const { path } = e.detail
    if (!path) return
    this.setData({ 'healthCert.url': path, 'healthCert.rawUrl': path })
    this.processOCR(
      ocrRiderHealthCert(path),
      'healthCert',
      'health',
      'healthCert'
    )
  },

  async removeUploadedDocument(field: UploadField) {
    if (!this.ensureApplicationEditable()) return

    const documentMap: Record<UploadField, {
      documentType: 'id_card_front' | 'id_card_back' | 'health_cert'
      data: Record<string, unknown>
    }> = {
      idFront: {
        documentType: 'id_card_front',
        data: {
          idFront: { url: '', rawUrl: '', assetId: undefined },
          'formData.realName': '',
          'formData.idNumber': '',
          'uploadFeedback.idFront': { ...EMPTY_UPLOAD_FEEDBACK }
        }
      },
      idBack: {
        documentType: 'id_card_back',
        data: {
          idBack: { url: '', rawUrl: '', assetId: undefined },
          'formData.idValidity': '',
          'uploadFeedback.idBack': { ...EMPTY_UPLOAD_FEEDBACK }
        }
      },
      healthCert: {
        documentType: 'health_cert',
        data: {
          healthCert: { url: '', rawUrl: '', assetId: undefined },
          'formData.healthCertNo': '',
          'formData.healthCertDate': '',
          'uploadFeedback.healthCert': { ...EMPTY_UPLOAD_FEEDBACK }
        }
      }
    }

    const target = documentMap[field]
    const requestVersion = this.beginDocumentRequest(field)

    wx.showLoading({ title: '删除中...' })
    try {
      const res = await deleteRiderApplicationDocument(target.documentType)
      if (!this.isLatestDocumentRequest(field, requestVersion)) {
        return
      }
      this.setData(target.data, () => {
        this.applyDocumentResponse(field, requestVersion, res)
      })
    } catch (e) {
      logger.error('Delete rider application document failed', { field, error: e })
      wx.showToast({ title: getErrorMessage(e, '删除失败，请重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  onIdFrontRemove() {
    this.removeUploadedDocument('idFront')
  },

  onIdBackRemove() {
    this.removeUploadedDocument('idBack')
  },

  onHealthCertRemove() {
    this.removeUploadedDocument('healthCert')
  },

  async processOCR(
    ocrPromise: Promise<RiderApplicationResponse>,
    field: UploadField,
    _type: 'identity' | 'health',
    feedbackField: keyof RiderUploadFeedback
  ) {
    const requestVersion = this.beginDocumentRequest(field)
    this.setOCRState(_type, 'processing')
    this.setUploadFeedback(feedbackField, createUploadFeedback('processing', '证照识别中', '请稍候，识别结果会显示在当前卡片中'))
    try {
      const res = await ocrPromise
      this.applyDocumentResponse(field, requestVersion, res)
    } catch (e) {
      logger.error('OCR failed', e)
      const message = getErrorMessage(e, '识别失败，请提供更清晰更规整的图片重试')
      this.setOCRState(_type, 'failed')
      this.setUploadFeedback(feedbackField, createUploadFeedback('error', '识别失败', message))
    }
  },

  onInput(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    const field = (e.currentTarget.dataset as { field?: string }).field
    if (!field) return

    const value = e.detail.value || ''
    const nextData: Record<string, string> = {
      [`formData.${field}`]: value
    }
    if (field === 'phone' && value.trim()) {
      nextData.phoneError = ''
    }
    this.setData(nextData)
  },

  onPrev() {
    this.setData({ currentStep: this.data.currentStep - 1 })
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

  async onNext() {
    const { currentStep, idFront, idBack, healthCert, formData } = this.data

    if (currentStep === 0 && !this.ensureConsent()) {
      return
    }

    if (currentStep === 1) {
      if (!idFront.url || !idBack.url || !healthCert.url) {
        return wx.showToast({ title: '请上传所有必需证照', icon: 'none' })
      }
    }

    if (currentStep === 2) {
      const realName = formData.realName.trim()
      const phone = formData.phone.trim()

      if (!realName) {
        return wx.showToast({ title: '请确认真实姓名', icon: 'none' })
      }
      if (!phone) {
        this.setData({ phoneError: '请填写联系电话，方便平台与你联系' })
        return wx.showToast({ title: '请填写联系电话', icon: 'none' })
      }

      this.setData({ phoneError: '' })
      // 同步基础信息
      wx.showLoading({ title: '保存信息...' })
      try {
        await updateRiderApplicationBasic({
          real_name: realName,
          phone
        })
      } catch (e) {
        logger.error('Update basic failed', e)
      } finally {
        wx.hideLoading()
      }
    }

    this.setData({ currentStep: currentStep + 1 })
  },

  async onSubmit() {
    if (!this.ensureConsent()) {
      return
    }

    let consentPayload
    try {
      consentPayload = await buildAgreementConsentPayload()
    } catch (e: unknown) {
      wx.showToast({ title: getErrorMessage(e, '协议信息加载失败，请稍后重试'), icon: 'none' })
      return
    }

    this.setData({ isSubmitting: true, currentStep: 4 })
    try {
      const res = await submitRiderApplication(consentPayload)
      
      // 模拟审核轮询或等待
      setTimeout(() => {
        const statusView = buildRiderApplicationStatusView(res.status)
        if (statusView.isApproved) {
          wx.showModal({
            title: '审核通过',
            content: '恭喜！您已正式成为 LocalLife 骑手。',
            showCancel: false,
            success: () => wx.reLaunch({ url: '/pages/rider/dashboard/index' })
          })
        } else if (isRejectedRiderApplication(res)) {
          this.mapResponseToData(res)
          this.showRejectedApplicationModal(res)
        } else if (statusView.isSubmitted) {
          this.setData({ isSubmitting: true, currentStep: 4 })
        }
      }, 1500)
    } catch (e: unknown) {
      const message = getErrorMessage(e, '提交失败')
      const debugMessage = getErrorDebugMessage(e)
      logger.error('[RiderRegister] Submit failed', {
        error: e,
        userMessage: message,
        debugMessage
      }, 'rider-register-submit')
      const shouldReturnToEdit = isDocumentCorrectionError(message)
      this.setData({ isSubmitting: false, currentStep: shouldReturnToEdit ? 2 : 3 })
      wx.showModal({
        title: shouldReturnToEdit ? '请修改资料后重试' : '提交失败',
        content: message,
        showCancel: false,
        success: () => {
          if (shouldReturnToEdit) {
            this.setData({ currentStep: 1 })
          }
        }
      })
    }
  }
})
