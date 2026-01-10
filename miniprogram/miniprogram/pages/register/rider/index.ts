
import { ocrRiderIdCard } from '../../../api/ocr'
import { logger } from '../../../utils/logger'
import { ErrorHandler } from '../../../utils/error-handler'
import { DraftStorage } from '../../../utils/draft-storage'
import { getRiderApplicationDraft, submitRiderApplication, updateRiderApplicationBasic } from '../../../api/rider-application'

const DRAFT_KEY = 'rider_register_draft'

Page({
  data: {
    navBarHeight: 88,
    formData: {
      // 基本信息
      name: '',
      phone: '',
      idCard: '',
      address: '',
      addressDetail: '',
      latitude: 0,
      longitude: 0,
      vehicle: '',
      availableTime: '',

      // 身份信息
      gender: '',
      hometown: '',
      currentAddress: '',
      idCardValidity: ''
    },
    // 图片
    idCardFrontImages: [] as Array<any>,
    idCardBackImages: [] as Array<any>,
    healthCertImages: [] as Array<any>,

    // 选择器状态
    vehiclePickerVisible: false,
    vehiclePickerValue: [],
    vehicleOptions: [
      { label: '电动车', value: 'electric_bike' },
      { label: '摩托车', value: 'motorcycle' },
      { label: '自行车', value: 'bicycle' },
      { label: '汽车', value: 'car' },
      { label: '步行', value: 'walk' }
    ],
    timePickerVisible: false,
    timePickerValue: [],
    timeOptions: [
      { label: '全天', value: 'all_day' },
      { label: '仅白天', value: 'day_only' },
      { label: '仅晚上', value: 'night_only' },
      { label: '周末', value: 'weekend' },
      { label: '工作日', value: 'workday' }
    ]
  },

  async onLoad() {
    this.loadDraft()
    
    // 从服务器同步最新草稿
    try {
        const serverDraft = await getRiderApplicationDraft()
        if (serverDraft) {
            this.setData({
                'formData.name': serverDraft.real_name || this.data.formData.name,
                'formData.phone': serverDraft.phone || this.data.formData.phone,
                'formData.gender': serverDraft.gender || this.data.formData.gender,
                'formData.hometown': serverDraft.hometown || this.data.formData.hometown,
                'formData.currentAddress': serverDraft.current_address || this.data.formData.currentAddress,
                'formData.idCard': serverDraft.id_card_number || this.data.formData.idCard,
                'formData.idCardValidity': serverDraft.id_card_validity || this.data.formData.idCardValidity,
                'formData.address': serverDraft.address || this.data.formData.address,
                'formData.addressDetail': serverDraft.address_detail || this.data.formData.addressDetail,
                'formData.latitude': serverDraft.latitude || this.data.formData.latitude,
                'formData.longitude': serverDraft.longitude || this.data.formData.longitude,
                'formData.vehicle': serverDraft.vehicle_type || this.data.formData.vehicle,
                'formData.availableTime': serverDraft.available_time || this.data.formData.availableTime,
            })
            this.saveDraft()
        }
    } catch (error) {
        logger.error('Load server draft failed', error)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  // ==================== 草稿管理 ====================

  saveDraft() {
    const data = {
      formData: this.data.formData,
      idCardFrontImages: this.data.idCardFrontImages,
      idCardBackImages: this.data.idCardBackImages,
      healthCertImages: this.data.healthCertImages
    }
    DraftStorage.save(DRAFT_KEY, data)
  },

  loadDraft() {
    const draft = DraftStorage.load(DRAFT_KEY)
    if (draft) {
      this.setData(draft)
    }
  },

  // ==================== 表单输入 ====================

  updateFormData(key: string, value: any) {
    this.setData({ [`formData.${key}`]: value })
    this.saveDraft()
  },

  onNameInput(e: WechatMiniprogram.Input) { this.updateFormData('name', e.detail.value) },
  onPhoneInput(e: WechatMiniprogram.Input) { this.updateFormData('phone', e.detail.value) },
  onIdCardInput(e: WechatMiniprogram.Input) { this.updateFormData('idCard', e.detail.value) },
  onAddressDetailInput(e: WechatMiniprogram.Input) { this.updateFormData('addressDetail', e.detail.value) },

  // 身份信息输入
  onGenderInput(e: WechatMiniprogram.Input) { this.updateFormData('gender', e.detail.value) },
  onHometownInput(e: WechatMiniprogram.Input) { this.updateFormData('hometown', e.detail.value) },
  onCurrentAddressInput(e: WechatMiniprogram.Input) { this.updateFormData('currentAddress', e.detail.value) },
  onIdCardValidityInput(e: WechatMiniprogram.Input) { this.updateFormData('idCardValidity', e.detail.value) },

  // ==================== 地址选择 ====================

  onChooseAddress() {
    wx.chooseLocation({
      success: (res) => {
        this.setData({
          'formData.address': res.address || res.name,
          'formData.latitude': res.latitude,
          'formData.longitude': res.longitude
        })
        this.saveDraft()
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

  onChooseVehicle() { this.setData({ vehiclePickerVisible: true }) },
  onVehicleConfirm(e: WechatMiniprogram.CustomEvent) {
    const { value } = e.detail
    const selectedOption = this.data.vehicleOptions.find((opt) => opt.value === value[0])
    if (selectedOption) {
      this.updateFormData('vehicle', selectedOption.label)
      this.setData({ vehiclePickerVisible: false })
    }
  },
  onVehicleCancel() { this.setData({ vehiclePickerVisible: false }) },

  onChooseTime() { this.setData({ timePickerVisible: true }) },
  onTimeConfirm(e: WechatMiniprogram.CustomEvent) {
    const { value } = e.detail
    const selectedOption = this.data.timeOptions.find((opt) => opt.value === value[0])
    if (selectedOption) {
      this.updateFormData('availableTime', selectedOption.label)
      this.setData({ timePickerVisible: false })
    }
  },
  onTimeCancel() { this.setData({ timePickerVisible: false }) },

  // ==================== 图片上传与OCR ====================

  // 身份证正面
  async onIdCardFrontUpload(e: any) {
    const { path, file } = e.detail
    if (!path) return
    
    // 构造存储格式
    const fileItem = { url: path, thumb: path, type: 'image', ...file }
    this.setData({ idCardFrontImages: [fileItem] })
    this.saveDraft()

    wx.showLoading({ title: '识别中...' })
    try {
      const res = await ocrRiderIdCard(path, 'front')
      const info = res.ocrData

      this.setData({
        'formData.name': info.name || '',
        'formData.idCard': info.id || info.id_number || info.id_num || '',
        'formData.gender': info.gender || '',
        'formData.hometown': info.address || info.addr || ''
      })
      this.saveDraft()
      wx.showToast({ title: '识别成功', icon: 'success' })
    } catch (error) {
      logger.error('OCR failed', error, 'Rider')
      wx.showToast({ title: '识别失败', icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },
  onIdCardFrontRemove() {
    this.setData({ idCardFrontImages: [] })
    this.saveDraft()
  },

  // 身份证反面
  async onIdCardBackUpload(e: any) {
    const { path, file } = e.detail
    if (!path) return

    const fileItem = { url: path, thumb: path, type: 'image', ...file }
    this.setData({ idCardBackImages: [fileItem] })
    this.saveDraft()

    wx.showLoading({ title: '识别中...' })
    try {
      const res = await ocrRiderIdCard(path, 'back')
      const info = res.ocrData

      this.setData({
        'formData.idCardValidity': info.valid_date || info.valid_period || info.valid_end || ''
      })
      this.saveDraft()
      wx.showToast({ title: '识别成功', icon: 'success' })
    } catch (error) {
      logger.error('OCR failed', error, 'Rider')
      wx.showToast({ title: '识别失败', icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },
  onIdCardBackRemove() {
    this.setData({ idCardBackImages: [] })
    this.saveDraft()
  },

  // 健康证
  onHealthCertUpload(e: any) {
    const { path, file } = e.detail
    if (!path) return
    const fileItem = { url: path, thumb: path, type: 'image', ...file }
    this.setData({ healthCertImages: [fileItem] })
    this.saveDraft()
  },
  onHealthCertRemove() {
    this.setData({ healthCertImages: [] })
    this.saveDraft()
  },

  // ==================== 提交申请 ====================

  async onSubmit() {
    const { formData, idCardFrontImages, idCardBackImages, healthCertImages } = this.data

    // 验证必填字段 (仅保留用户要求的 6 项核心数据相关的校验)
    if (!idCardFrontImages.length) return wx.showToast({ title: '请上传身份证正面', icon: 'none' })
    if (!idCardBackImages.length) return wx.showToast({ title: '请上传身份证反面', icon: 'none' })
    if (!healthCertImages.length) return wx.showToast({ title: '请上传健康证', icon: 'none' })

    const requiredKeys = ['name', 'gender', 'idCard', 'idCardValidity', 'hometown', 'currentAddress']
    const requiredLabels: Record<string, string> = {
        name: '真实姓名',
        gender: '性别',
        idCard: '身份证号',
        idCardValidity: '有效期限',
        hometown: '籍贯地址',
        currentAddress: '当前住址'
    }

    for (const key of requiredKeys) {
        if (!formData[key as keyof typeof formData]) {
            return wx.showToast({ title: `请提供${requiredLabels[key]}`, icon: 'none' })
        }
    }

    // 同步核心身份信息到数据库草稿
    try {
        await updateRiderApplicationBasic({
            real_name: formData.name,
            phone: formData.phone,
            gender: formData.gender,
            hometown: formData.hometown,
            current_address: formData.currentAddress,
            id_card_number: formData.idCard,
            id_card_validity: formData.idCardValidity,
            address: formData.address,
            address_detail: formData.addressDetail,
            latitude: formData.latitude,
            longitude: formData.longitude,
            vehicle_type: formData.vehicle,
            available_time: formData.availableTime,
        })
    } catch (error) {
        logger.error('Update basic info failed', error)
    }

    wx.showLoading({ title: '提交中...' })

    try {
      await submitRiderApplication()

      wx.showToast({
        title: '申请提交成功',
        icon: 'success',
        duration: 2000,
        success: () => {
          DraftStorage.clear(DRAFT_KEY)
          setTimeout(() => {
            wx.navigateBack()
          }, 2000)
        }
      })
    } catch (error) {
      logger.error('Apply rider failed:', error, 'Rider')
      wx.showToast({ title: '提交失败', icon: 'error' })
    } finally {
      wx.hideLoading()
    }
  }
})
