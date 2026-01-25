import { 
  getOrCreateRiderApplication, 
  updateRiderApplicationBasic, 
  ocrRiderIdCard, 
  ocrRiderHealthCert, 
  submitRiderApplication,
  resetRiderApplication
} from '../../../api/rider-application'
import { logger } from '../../../utils/logger'

Page({
  data: {
    navBarHeight: 88,
    currentStep: 0,
    isSubmitting: false,
    idFront: { url: '', rawUrl: '' },
    idBack: { url: '', rawUrl: '' },
    healthCert: { url: '', rawUrl: '' },
    formData: {
      realName: '',
      phone: '',
      address: '',
      idNumber: '',
      idValidity: '',
      healthCertNo: '',
      healthCertDate: ''
    }
  },

  async onLoad() {
    await this.initApplication()
  },

  onNavHeight(e: any) {
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

  mapResponseToData(res: any) {
    this.setData({
      'formData.realName': res.real_name || res.id_card_ocr?.name || '',
      'formData.phone': res.phone || '',
      'formData.idNumber': res.id_card_ocr?.id_number || '',
      'formData.idValidity': res.id_card_ocr?.valid_end || '',
      'formData.healthCertNo': res.health_cert_ocr?.cert_number || '',
      'formData.healthCertDate': res.health_cert_ocr?.valid_end || '',
      idFront: { url: res.id_card_front_url || '', rawUrl: res.id_card_front_url || '' },
      idBack: { url: res.id_card_back_url || '', rawUrl: res.id_card_back_url || '' },
      healthCert: { url: res.health_cert_url || '', rawUrl: res.health_cert_url || '' }
    })
  },

  async onIdFrontUpload(e: any) {
    const { path } = e.detail
    this.setData({ 'idFront.url': path })
    this.processOCR(ocrRiderIdCard(path, 'Front'), 'identity')
  },

  async onIdBackUpload(e: any) {
    const { path } = e.detail
    this.setData({ 'idBack.url': path })
    this.processOCR(ocrRiderIdCard(path, 'Back'), 'identity')
  },

  async onHealthCertUpload(e: any) {
    const { path } = e.detail
    this.setData({ 'healthCert.url': path })
    this.processOCR(ocrRiderHealthCert(path), 'health')
  },

  async processOCR(ocrPromise: Promise<any>, type: 'identity' | 'health') {
    wx.showLoading({ title: '智能识别中...' })
    try {
      const res = await ocrPromise
      this.mapResponseToData(res)
      wx.hideLoading()
      wx.showToast({ title: '识别成功', icon: 'none' })
    } catch (e) {
      wx.hideLoading()
      logger.error('OCR failed', e)
    }
  },

  onInput(e: any) {
    const field = e.currentTarget.dataset.field
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
    const { currentStep, idFront, idBack, healthCert, formData } = this.data

    if (currentStep === 1) {
      if (!idFront.url || !idBack.url || !healthCert.url) {
        return wx.showToast({ title: '请上传所有必需证照', icon: 'none' })
      }
    }

    if (currentStep === 2) {
      if (!formData.realName || !formData.phone) {
        return wx.showToast({ title: '请确认真实姓名和手机号', icon: 'none' })
      }
      // 同步基础信息
      wx.showLoading({ title: '保存信息...' })
      try {
        await updateRiderApplicationBasic({
          real_name: formData.realName,
          phone: formData.phone
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
    this.setData({ isSubmitting: true, currentStep: 4 })
    try {
      const res = await submitRiderApplication()
      
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
    } catch (e: any) {
      this.setData({ isSubmitting: false, currentStep: 3 })
      wx.showToast({ title: e.message || '提交失败', icon: 'none' })
    }
  }
})
