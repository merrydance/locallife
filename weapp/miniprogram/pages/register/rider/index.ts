import { 
  getOrCreateRiderApplication, 
  updateRiderApplicationBasic, 
  ocrRiderIdCard, 
  ocrRiderHealthCert, 
  submitRiderApplication,
  resetRiderApplication,
  deleteRiderApplicationHealthCert,
  type RiderApplicationResponse
} from '../../../api/rider-application'
import { getPrivateMediaUrl } from '../../../utils/image-security'
import { logger } from '../../../utils/logger'
import Navigation from '../../../utils/navigation'
import { buildAgreementConsentPayload } from '../../../api/agreement-consent'

type UploadEvent = WechatMiniprogram.CustomEvent<{ path?: string }>

type UploadFieldValue = {
  url: string
  rawUrl?: string
  assetId?: number
}

type OCRDisplayStateValue = 'idle' | 'processing' | 'done' | 'failed'

type RiderOCRDisplayState = {
  identity: OCRDisplayStateValue
  health: OCRDisplayStateValue
}

type UploadFeedbackState = 'idle' | 'processing' | 'success' | 'error'

type UploadFeedback = {
  state: UploadFeedbackState
  title: string
  description: string
}

type RiderUploadFeedback = {
  idFront: UploadFeedback
  idBack: UploadFeedback
  healthCert: UploadFeedback
}

const DEFAULT_RIDER_OCR_DISPLAY_STATE: RiderOCRDisplayState = {
  identity: 'idle',
  health: 'idle'
}

const EMPTY_UPLOAD_FEEDBACK: UploadFeedback = {
  state: 'idle',
  title: '',
  description: ''
}

const DEFAULT_RIDER_UPLOAD_FEEDBACK: RiderUploadFeedback = {
  idFront: { ...EMPTY_UPLOAD_FEEDBACK },
  idBack: { ...EMPTY_UPLOAD_FEEDBACK },
  healthCert: { ...EMPTY_UPLOAD_FEEDBACK }
}

function getErrorMessage(error: unknown, fallback: string): string {
  if (error && typeof error === 'object') {
    const maybeError = error as { userMessage?: string, message?: string }
    if (maybeError.userMessage) return maybeError.userMessage
    if (maybeError.message) return maybeError.message
  }
  return fallback
}

function createUploadFeedback(state: UploadFeedbackState, title = '', description = ''): UploadFeedback {
  return { state, title, description }
}

function pickOCRText(payload: Record<string, unknown> | undefined, ...keys: string[]): string {
  for (const key of keys) {
    const value = payload?.[key]
    if (typeof value === 'string' && value.trim()) {
      return value.trim()
    }
  }
  return ''
}

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
    consentPopupVisible: false
  },

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
        
        // 如果已被拒绝，自动重置或提示
        if (res.status === 'rejected') {
          wx.showModal({
            title: '申请未通过',
            content: `驳回原因：${res.reject_reason || '资料不全'}. 是否修改重试？`,
            success: async (modalRes) => {
              if (modalRes.confirm) {
                const refreshed = await resetRiderApplication()
                this.mapResponseToData(refreshed)
              } else {
                wx.navigateBack()
              }
            }
          })
        } else if (res.status === 'approved') {
          wx.showToast({ title: '您已入驻成功' })
          setTimeout(() => wx.reLaunch({ url: '/pages/rider/dashboard/index' }), 1000)
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
    this.setData({
      'formData.realName': res.real_name || pickOCRText(idCardOCR, 'name') || currentForm.realName || '',
      'formData.phone': nextPhone,
      'formData.idNumber': pickOCRText(idCardOCR, 'id_number', 'id_num') || currentForm.idNumber || '',
      'formData.idValidity': pickOCRText(idCardOCR, 'valid_end', 'valid_date', 'valid_period') || currentForm.idValidity || '',
      'formData.healthCertNo': pickOCRText(healthCertOCR, 'cert_number', 'certificate_number', 'certificate') || currentForm.healthCertNo || '',
      'formData.healthCertDate': pickOCRText(healthCertOCR, 'valid_end', 'valid_date', 'valid_period') || currentForm.healthCertDate || '',
      phoneError: nextPhone.trim() ? '' : this.data.phoneError,
      idFront: { url: '', assetId: res.id_card_front_asset_id },
      idBack: { url: '', assetId: res.id_card_back_asset_id },
      healthCert: { url: '', assetId: res.health_cert_asset_id },
      ocrDisplayState: this.buildRiderOcrDisplayState(res),
      uploadFeedback: this.buildRiderUploadFeedback(res)
    }, () => {
      void this.refreshUploadPreviewURLs()
    })
  },

  buildRiderUploadFeedback(res?: RiderApplicationResponse): RiderUploadFeedback {
    const idCardOCR = res?.id_card_ocr as Record<string, unknown> | undefined
    const healthCertOCR = res?.health_cert_ocr as Record<string, unknown> | undefined
    const idStatus = pickOCRText(idCardOCR, 'status')
    const idError = pickOCRText(idCardOCR, 'error')
    const healthStatus = pickOCRText(healthCertOCR, 'status')
    const healthError = pickOCRText(healthCertOCR, 'error')

    const idFrontUploaded = Boolean(res?.id_card_front_asset_id || this.data.idFront.assetId || this.data.idFront.url)
    const idBackUploaded = Boolean(res?.id_card_back_asset_id || this.data.idBack.assetId || this.data.idBack.url)
    const healthUploaded = Boolean(res?.health_cert_asset_id || this.data.healthCert.assetId || this.data.healthCert.url)

    const idFrontReady = Boolean(pickOCRText(idCardOCR, 'name', 'id_number', 'id_num') || idStatus === 'done')
    const idBackReady = Boolean(pickOCRText(idCardOCR, 'valid_end', 'valid_date', 'valid_period') || idStatus === 'done')
    const healthReady = Boolean(pickOCRText(healthCertOCR, 'cert_number', 'certificate_number', 'certificate', 'valid_end', 'valid_date', 'valid_period', 'name') || healthStatus === 'done')

    return {
      idFront: idFrontUploaded
        ? idStatus === 'failed'
          ? createUploadFeedback('error', '识别失败', idError || '请重新上传清晰、完整的身份证人像面')
          : idFrontReady
            ? createUploadFeedback('success', '识别成功', '已识别姓名和身份证号')
            : createUploadFeedback('processing', '证照识别中', '正在识别身份证人像面信息')
        : { ...EMPTY_UPLOAD_FEEDBACK },
      idBack: idBackUploaded
        ? idStatus === 'failed'
          ? createUploadFeedback('error', '识别失败', idError || '请重新上传清晰、完整的身份证国徽面')
          : idBackReady
            ? createUploadFeedback('success', '识别成功', '已识别证件有效期')
            : createUploadFeedback('processing', '证照识别中', '正在识别身份证国徽面信息')
        : { ...EMPTY_UPLOAD_FEEDBACK },
      healthCert: healthUploaded
        ? healthStatus === 'failed'
          ? createUploadFeedback('error', '识别失败', healthError || '请重新上传清晰、无遮挡的健康证照片')
          : healthReady
            ? createUploadFeedback('success', '识别成功', '已识别健康证信息')
            : createUploadFeedback('processing', '证照识别中', '正在识别健康证信息')
        : { ...EMPTY_UPLOAD_FEEDBACK }
    }
  },

  buildRiderOcrDisplayState(res?: RiderApplicationResponse): RiderOCRDisplayState {
    const identityUploaded = Boolean(
      (res?.id_card_front_asset_id || this.data.idFront.assetId || this.data.idFront.url)
      && (res?.id_card_back_asset_id || this.data.idBack.assetId || this.data.idBack.url)
    )
    const healthUploaded = Boolean(res?.health_cert_asset_id || this.data.healthCert.assetId || this.data.healthCert.url)
    const idCardOCR = res?.id_card_ocr as Record<string, unknown> | undefined
    const healthCertOCR = res?.health_cert_ocr as Record<string, unknown> | undefined

    const identityDone = Boolean(
      pickOCRText(idCardOCR, 'status') === 'done'
      || (pickOCRText(idCardOCR, 'name') && pickOCRText(idCardOCR, 'id_number', 'id_num') && pickOCRText(idCardOCR, 'valid_end', 'valid_date', 'valid_period'))
    )
    const healthDone = Boolean(
      pickOCRText(healthCertOCR, 'status') === 'done'
      || pickOCRText(healthCertOCR, 'cert_number', 'certificate_number', 'certificate', 'valid_end', 'valid_date', 'valid_period', 'name')
    )

    return {
      identity: identityDone ? 'done' : identityUploaded ? 'processing' : 'idle',
      health: healthDone ? 'done' : healthUploaded ? 'processing' : 'idle'
    }
  },

  setOCRState(type: 'identity' | 'health', status: OCRDisplayStateValue) {
    this.setData({ [`ocrDisplayState.${type}`]: status })
  },

  setUploadFeedback(field: keyof RiderUploadFeedback, feedback: UploadFeedback) {
    this.setData({ [`uploadFeedback.${field}`]: feedback })
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
    const uploads: Array<{ key: 'idFront' | 'idBack' | 'healthCert', value: UploadFieldValue }> = [
      { key: 'idFront', value: this.data.idFront },
      { key: 'idBack', value: this.data.idBack },
      { key: 'healthCert', value: this.data.healthCert }
    ]

    for (const item of uploads) {
      const assetId = item.value?.assetId
      if (!assetId) continue

      const resolved = await this.resolveUploadPreviewURL(assetId)
      if (resolved && resolved !== item.value.url) {
        this.setData({ [`${item.key}.url`]: resolved })
      }
    }
  },

  async onIdFrontUpload(e: UploadEvent) {
    const { path } = e.detail
    if (!path) return
    this.setData({ 'idFront.url': path, 'idFront.rawUrl': path })
    this.processOCR(
      ocrRiderIdCard(path, 'Front'),
      'identity',
      'idFront'
    )
  },

  async onIdBackUpload(e: UploadEvent) {
    const { path } = e.detail
    if (!path) return
    this.setData({ 'idBack.url': path, 'idBack.rawUrl': path })
    this.processOCR(
      ocrRiderIdCard(path, 'Back'),
      'identity',
      'idBack'
    )
  },

  async onHealthCertUpload(e: UploadEvent) {
    const { path } = e.detail
    if (!path) return
    this.setData({ 'healthCert.url': path, 'healthCert.rawUrl': path })
    this.processOCR(
      ocrRiderHealthCert(path),
      'health',
      'healthCert'
    )
  },

  async onHealthCertRemove() {
    wx.showLoading({ title: '删除中...' })
    try {
      const res = await deleteRiderApplicationHealthCert()
      this.mapResponseToData(res)
    } catch (e) {
      logger.error('Delete rider health cert failed', e)
      wx.showToast({ title: getErrorMessage(e, '删除失败，请重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  async processOCR(
    ocrPromise: Promise<RiderApplicationResponse>,
    _type: 'identity' | 'health',
    feedbackField: keyof RiderUploadFeedback
  ) {
    this.setOCRState(_type, 'processing')
    this.setUploadFeedback(feedbackField, createUploadFeedback('processing', '证照识别中', '请稍候，识别结果会显示在当前卡片中'))
    try {
      const res = await ocrPromise
      this.mapResponseToData(res)
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
        if (res.status === 'approved') {
          wx.showModal({
            title: '审核通过',
            content: '恭喜！您已正式成为 LocalLife 骑手。',
            showCancel: false,
            success: () => wx.reLaunch({ url: '/pages/rider/dashboard/index' })
          })
        } else if (res.status === 'rejected') {
          wx.showModal({
            title: '审核未通过',
            content: res.reject_reason || '资料核验不匹配',
            confirmText: '修改资料',
            success: async (m) => {
              if (m.confirm) {
                const draft = await resetRiderApplication()
                this.mapResponseToData(draft)
                this.setData({ currentStep: 1, isSubmitting: false })
              } else {
                wx.navigateBack()
              }
            }
          })
        }
      }, 1500)
    } catch (e: unknown) {
      this.setData({ isSubmitting: false, currentStep: 3 })
      wx.showToast({ title: getErrorMessage(e, '提交失败'), icon: 'none' })
    }
  }
})
