import { 
  getOrCreateGroupApplication, 
  updateGroupApplicationBasic, 
  ocrGroupBusinessLicense, 
  submitGroupApplication 
} from '../../../../api/group-application'
import { ocrIdCard } from '../../../../api/onboarding'
import { logger } from '../../../../utils/logger'

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
  if (error && typeof error === 'object' && 'message' in error) {
    const { message } = error as { message?: unknown }
    if (typeof message === 'string' && message.trim()) {
      return message
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
    }
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
      if (res.id_card_front_ocr) {
        this.setData({ 'formData.legalPerson': res.id_card_front_ocr.name || '' })
      }
      wx.hideLoading()
    } catch (e) {
      wx.hideLoading()
      wx.showToast({ title: '识别失败，请手动确认', icon: 'none' })
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

  async onNext() {
    const { currentStep, idFront, idBack, formData } = this.data
    
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
    this.setData({ submitting: true })
    try {
      await submitGroupApplication()
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
