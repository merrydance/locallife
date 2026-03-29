import { 
  getOrCreateGroupApplication, 
  updateGroupApplicationBasic, 
  ocrGroupIdCard,
  ocrGroupBusinessLicense, 
  submitGroupApplication,
  type GroupApplicationResponse
} from '../../../../api/group-application'
import { getPrivateMediaUrl } from '../../../../utils/image-security'
import { logger } from '../../../../utils/logger'
import Navigation from '../../../../utils/navigation'
import { buildAgreementConsentPayload } from '../../../../api/agreement-consent'

type NavHeightEvent = {
  detail: {
    navBarHeight: number
  }
}

type UploadEvent = {
  detail: {
    path: string
  }
}

type UploadFieldValue = {
  url: string
  rawUrl?: string
  assetId?: number
}

type OCRDisplayStateValue = 'idle' | 'processing' | 'done' | 'failed'

type GroupOCRDisplayState = {
  license: OCRDisplayStateValue
  identity: OCRDisplayStateValue
}

const DEFAULT_GROUP_OCR_DISPLAY_STATE: GroupOCRDisplayState = {
  license: 'idle',
  identity: 'idle'
}

type InputEvent = {
  currentTarget: {
    dataset: {
      field?: string
    }
  }
  detail: {
    value: string
  }
}

const getErrorMessage = (error: unknown, fallback: string): string => {
  if (error && typeof error === 'object') {
    const maybeError = error as { userMessage?: unknown, message?: unknown }
    if (typeof maybeError.userMessage === 'string' && maybeError.userMessage.trim()) {
      return maybeError.userMessage
    }
    if (typeof maybeError.message === 'string' && maybeError.message.trim()) {
      return maybeError.message
    }
  }
  return fallback
}

Page({
  data: {
    navBarHeight: 88,
    currentStep: 0,
    submitting: false,
    idFront: { url: '', rawUrl: '', assetId: undefined } as UploadFieldValue,
    idBack: { url: '', rawUrl: '', assetId: undefined } as UploadFieldValue,
    license: { url: '', rawUrl: '', assetId: undefined } as UploadFieldValue,
    ocrDisplayState: DEFAULT_GROUP_OCR_DISPLAY_STATE,
    formData: {
      groupName: '',
      contactPhone: '',
      address: '',
      licenseNumber: '',
      legalPerson: ''
    },
    consentChecked: false,
    consentPopupVisible: false
  },

  async onLoad() {
    await this.fetchDraft()
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  buildGroupOCRDisplayState(res?: GroupApplicationResponse): GroupOCRDisplayState {
    const licenseStatus = res?.business_license_ocr?.status || ''
    const idFrontStatus = res?.id_card_front_ocr?.status || ''
    const idBackStatus = res?.id_card_back_ocr?.status || ''

    const licenseUploaded = Boolean(res?.license_image_asset_id || this.data.license.assetId || this.data.license.url)
    const identityUploaded = Boolean(
      (res?.id_card_front_asset_id || this.data.idFront.assetId || this.data.idFront.url) &&
      (res?.id_card_back_asset_id || this.data.idBack.assetId || this.data.idBack.url)
    )

    const licenseDone = licenseStatus === 'done' || Boolean(res?.license_number)
    const identityDone = idFrontStatus === 'done' && idBackStatus === 'done'

    return {
      license: licenseStatus === 'failed' ? 'failed' : licenseDone ? 'done' : licenseUploaded ? 'processing' : 'idle',
      identity: idFrontStatus === 'failed' || idBackStatus === 'failed'
        ? 'failed'
        : identityDone
          ? 'done'
          : identityUploaded
            ? 'processing'
            : 'idle'
    }
  },

  async resolveUploadPreviewURL(assetId?: number) {
    if (!assetId) return ''
    try {
      return await getPrivateMediaUrl(assetId)
    } catch (_error) {
      return ''
    }
  },

  async refreshUploadPreviewURLs() {
    const uploads: Array<{ key: 'license' | 'idFront' | 'idBack', value: UploadFieldValue }> = [
      { key: 'license', value: this.data.license },
      { key: 'idFront', value: this.data.idFront },
      { key: 'idBack', value: this.data.idBack }
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

  mapResponseToData(res: GroupApplicationResponse) {
    this.setData({
      'formData.groupName': res.group_name || '',
      'formData.contactPhone': res.contact_phone || '',
      'formData.address': res.address || '',
      'formData.licenseNumber': res.license_number || '',
      'formData.legalPerson': res.legal_person_name || res.id_card_front_ocr?.name || '',
      license: { url: '', rawUrl: '', assetId: res.license_image_asset_id },
      idFront: { url: '', rawUrl: '', assetId: res.id_card_front_asset_id },
      idBack: { url: '', rawUrl: '', assetId: res.id_card_back_asset_id },
      ocrDisplayState: this.buildGroupOCRDisplayState(res)
    }, () => {
      void this.refreshUploadPreviewURLs()
    })
  },

  setOCRState(type: keyof GroupOCRDisplayState, status: OCRDisplayStateValue) {
    this.setData({ [`ocrDisplayState.${type}`]: status })
  },

  isPendingOCRMessage(message: string): boolean {
    return message.includes('处理中') || message.includes('审核中') || message.includes('识别中')
  },

  async fetchDraft() {
    try {
      const res = await getOrCreateGroupApplication()
      if (res) {
        this.mapResponseToData(res)
      }
    } catch (e) {
      logger.error('Fetch group draft failed', e)
    }
  },

  async onIdFrontUpload(e: UploadEvent) {
    const { path } = e.detail
    if (!path) return
    this.setData({
      'idFront.url': path,
      'idFront.rawUrl': path
    })
    this.setOCRState('identity', 'processing')
    wx.showLoading({ title: '识别身份证...' })
    try {
      const res = await ocrGroupIdCard(path, 'Front')
      const nextState = this.buildGroupOCRDisplayState(res)
      this.mapResponseToData(res)
      wx.hideLoading()
      wx.showToast({ title: nextState.identity === 'done' ? '身份证识别成功' : '身份证已上传，系统继续识别中', icon: 'none' })
    } catch (error) {
      wx.hideLoading()
      const message = getErrorMessage(error, '身份证已上传，系统处理中')
      this.setOCRState('identity', this.isPendingOCRMessage(message) ? 'processing' : 'failed')
      wx.showToast({ title: message, icon: 'none' })
    }
  },

  async onIdBackUpload(e: UploadEvent) {
    const { path } = e.detail
    if (!path) return
    this.setData({
      'idBack.url': path,
      'idBack.rawUrl': path
    })
    this.setOCRState('identity', 'processing')
    wx.showLoading({ title: '识别身份证...' })
    try {
      const res = await ocrGroupIdCard(path, 'Back')
      const nextState = this.buildGroupOCRDisplayState(res)
      this.mapResponseToData(res)
      wx.hideLoading()
      wx.showToast({ title: nextState.identity === 'done' ? '身份证识别成功' : '身份证已上传，系统继续识别中', icon: 'none' })
    } catch (error) {
      wx.hideLoading()
      const message = getErrorMessage(error, '身份证已上传，系统处理中')
      this.setOCRState('identity', this.isPendingOCRMessage(message) ? 'processing' : 'failed')
      wx.showToast({ title: message, icon: 'none' })
    }
  },

  async onLicenseUpload(e: UploadEvent) {
    const { path } = e.detail
    if (!path) return
    this.setData({ 'license.url': path, 'license.rawUrl': path })
    this.setOCRState('license', 'processing')
    wx.showLoading({ title: '识别执照...' })
    try {
      const res = await ocrGroupBusinessLicense(path)
      const nextState = this.buildGroupOCRDisplayState(res)
      this.mapResponseToData(res)
      wx.hideLoading()
      wx.showToast({ title: nextState.license === 'done' ? '营业执照识别成功' : '营业执照已上传，系统继续识别中', icon: 'none' })
    } catch (e) {
      wx.hideLoading()
      const message = getErrorMessage(e, '图片已上传，系统处理中')
      this.setOCRState('license', this.isPendingOCRMessage(message) ? 'processing' : 'failed')
      wx.showToast({ title: message, icon: 'none' })
    }
  },

  onInput(e: InputEvent) {
    const field = e.currentTarget.dataset.field
    if (!field) {
      return
    }
    this.setData({ [`formData.${field}`]: e.detail.value })
  },

  onChooseAddress() {
    wx.chooseLocation({
      success: (res) => {
        this.setData({ 'formData.address': res.address || res.name })
      }
    })
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
    const { currentStep, idFront, idBack, formData } = this.data

    if (currentStep === 0 && !this.ensureConsent()) {
      return
    }
    
    if (currentStep === 1) {
      if (!idFront.url || !idBack.url) {
        wx.showToast({ title: '请上传身份证正反面', icon: 'none' })
        return
      }
    }

    if (currentStep === 2) {
      if (!formData.groupName || !formData.contactPhone) {
        wx.showToast({ title: '请填写基本信息', icon: 'none' })
        return
      }
      
      // Update basic info to backend
      wx.showLoading({ title: '同步信息...' })
      try {
        await updateGroupApplicationBasic({
          group_name: formData.groupName,
          contact_phone: formData.contactPhone,
          address: formData.address,
          license_number: formData.licenseNumber,
          license_image_asset_id: this.data.license.assetId
        })
        wx.hideLoading()
      } catch (e) {
        wx.hideLoading()
        wx.showToast({ title: '同步失败', icon: 'none' })
        return
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

    this.setData({ submitting: true })
    try {
      await submitGroupApplication(consentPayload)
      this.setData({ currentStep: 4 })
      
      setTimeout(() => {
        wx.showModal({
          title: '提交成功',
          content: '您的集团入驻申请已进入审核，请耐心等待。',
          showCancel: false,
          success: () => {
            wx.switchTab({ url: '/pages/user_center/index' })
          }
        })
      }, 2000)
    } catch (e: unknown) {
      this.setData({ submitting: false })
      wx.showToast({ title: getErrorMessage(e, '提交失败'), icon: 'none' })
    }
  }
})
