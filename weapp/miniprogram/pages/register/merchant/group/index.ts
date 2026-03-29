import { 
  getOrCreateGroupApplication, 
  updateGroupApplicationBasic, 
  ocrGroupBusinessLicense, 
  submitGroupApplication 
} from '../../../../api/group-application'
import { ocrIdCard } from '../../../../api/onboarding'
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
    idFront: { url: '', rawUrl: '' },
    idBack: { url: '', rawUrl: '' },
    license: { url: '', rawUrl: '' },
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

  async fetchDraft() {
    try {
      const res = await getOrCreateGroupApplication()
      if (res) {
        this.setData({
          'formData.groupName': res.group_name || '',
          'formData.contactPhone': res.contact_phone || '',
          'formData.address': res.address || '',
          'formData.licenseNumber': res.license_number || '',
          license: { url: res.license_image_url || '', rawUrl: res.license_image_url || '' }
        })
      }
    } catch (e) {
      logger.error('Fetch group draft failed', e)
    }
  },

  async onIdFrontUpload(e: UploadEvent) {
    const { path } = e.detail
    this.setData({ 'idFront.url': path })
    wx.showLoading({ title: '识别身份证...' })
    try {
      const res = await ocrIdCard(path, 'Front')
      if (res.draft.id_card_front_ocr?.name) {
        this.setData({ 'formData.legalPerson': res.draft.id_card_front_ocr.name || '' })
      } else if (res.state === 'moderation_pending') {
        wx.showToast({ title: '身份证已上传，系统处理中', icon: 'none' })
      }
      wx.hideLoading()
    } catch (e) {
      wx.hideLoading()
      wx.showToast({ title: getErrorMessage(e, '识别失败，请手动确认'), icon: 'none' })
    }
  },

  onIdBackUpload(e: UploadEvent) {
    this.setData({ 'idBack.url': e.detail.path })
  },

  async onLicenseUpload(e: UploadEvent) {
    const { path } = e.detail
    this.setData({ 'license.url': path })
    wx.showLoading({ title: '识别执照...' })
    try {
      const res = await ocrGroupBusinessLicense(path)
      if (res.license_number) {
        this.setData({ 'formData.licenseNumber': res.license_number })
      }
      wx.hideLoading()
    } catch (e) {
      wx.hideLoading()
      wx.showToast({ title: getErrorMessage(e, '图片已上传，系统处理中'), icon: 'none' })
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
          license_image_url: this.data.license.url
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
