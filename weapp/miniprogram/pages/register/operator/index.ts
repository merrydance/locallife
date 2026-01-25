import { 
  getOrCreateOperatorApplication, 
  getOperatorApplication,
  updateOperatorBasic, 
  ocrOperatorBusinessLicense, 
  ocrOperatorIdCard, 
  submitOperatorApplication,
  listAvailableRegions
} from '../../../api/operator-application'
import { logger } from '../../../utils/logger'

Page({
  data: {
    navBarHeight: 88,
    currentStep: 0,
    isSubmitting: false,
    idFront: { url: '', rawUrl: '' },
    idBack: { url: '', rawUrl: '' },
    license: { url: '', rawUrl: '' },
    
    // 核心表单数据
    formData: {
      regionId: 0,
      regionName: '',
      name: '',
      contactName: '',
      contactPhone: '',
      years: 3
    },

    // 区域选择相关状态
    regionPopupVisible: false,
    regionOptions: [] as any[],     // 原始列表
    filteredRegions: [] as any[]    // 搜索过滤后的列表
  },

  onLoad() {
    this.initApplication()
    this.fetchAvailableRegions()
  },

  onNavHeight(e: any) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  /**
   * 初始化申请状态（静默加载）
   */
  async initApplication() {
    try {
      // 使用 GET 获取已有申请草稿
      const res = await getOperatorApplication()
      if (res) {
        this.mapResponseToData(res)
        
        // 根据状态跳转
        if (res.status === 'submitted') {
          this.setData({ currentStep: 4 })
        } else if (res.status === 'approved') {
          wx.showToast({ title: '您已是合伙人' })
          setTimeout(() => wx.reLaunch({ url: '/pages/operator/dashboard/index' }), 1500)
        } else if (res.status === 'rejected') {
          wx.showModal({
            title: '审核未通过',
            content: `原因：${res.reject_reason || '资料核验失败'}`,
            confirmText: '修改资料'
          })
        }
      }
    } catch (e: any) {
      // 404 说明没申请过，留在介绍页，非异常
      if (e.statusCode !== 404) {
        logger.error('Init operator application failed', e)
      }
    }
  },

  /**
   * 获取所有可用的 Level 3 (区县) 区域
   */
  async fetchAvailableRegions() {
    try {
      const res = await listAvailableRegions({ page_id: 1, page_size: 100, level: 3 })
      if (res && res.regions) {
        const options = res.regions.map(r => ({
          label: r.name,
          secondary: r.parent_name || '',
          value: r.id
        }))
        this.setData({ 
          regionOptions: options,
          filteredRegions: options
        }, () => {
          // 在选项加载完成后，如果已经有 regionId，再次尝试回填显示名
          if (this.data.formData.regionId && !this.data.formData.regionName) {
            this.syncRegionName(this.data.formData.regionId)
          }
        })
      }
    } catch (e) {
      logger.error('Fetch regions failed', e)
    }
  },

  /**
   * 将后端响应映射到视图数据
   */
  mapResponseToData(res: any) {
    if (!res) return

    const newData: any = {
      'formData.regionId': res.region_id || 0,
      'formData.name': res.name || '',
      'formData.contactName': res.contact_name || '',
      'formData.contactPhone': res.contact_phone || '',
      'formData.years': res.requested_contract_years || 3,
      idFront: { url: res.id_card_front_url || '', rawUrl: res.id_card_front_url || '' },
      idBack: { url: res.id_card_back_url || '', rawUrl: res.id_card_back_url || '' },
      license: { url: res.business_license_url || '', rawUrl: res.business_license_url || '' }
    }

    // 优先使用后端返回的名称，否则尝试从本地 Options 中反查
    const regionId = res.region_id
    let regionName = res.region_name || ''

    if (!regionName && regionId && this.data.regionOptions.length > 0) {
      const matched = this.data.regionOptions.find(r => Number(r.value) === Number(regionId))
      if (matched) {
        regionName = matched.secondary ? `${matched.secondary} - ${matched.label}` : matched.label
      }
    }
    
    if (regionName) {
      newData['formData.regionName'] = regionName
    }

    this.setData(newData)
  },

  /**
   * 根据 ID 同步本地显示名称
   */
  syncRegionName(regionId: number | string) {
    const id = Number(regionId)
    const matched = this.data.regionOptions.find(r => Number(r.value) === id)
    if (matched) {
      const fullName = matched.secondary ? `${matched.secondary} - ${matched.label}` : matched.label
      this.setData({ 'formData.regionName': fullName })
    }
  },

  // ==================== 区域搜索逻辑 ====================

  onOpenRegionPopup() {
    this.setData({ regionPopupVisible: true })
  },

  onCloseRegionPopup() {
    this.setData({ regionPopupVisible: false })
  },

  onRegionSearch(e: any) {
    const keyword = (e.detail.value || '').toLowerCase()
    const filtered = this.data.regionOptions.filter(item => 
      item.label.toLowerCase().includes(keyword) || 
      item.secondary.toLowerCase().includes(keyword)
    )
    this.setData({ filteredRegions: filtered })
  },

  onSelectRegion(e: any) {
    const regionId = Number(e.detail.value)
    const region = this.data.regionOptions.find(r => Number(r.value) === regionId)
    
    if (region) {
      const fullName = region.secondary ? `${region.secondary} - ${region.label}` : region.label
      this.setData({
        'formData.regionId': region.value,
        'formData.regionName': fullName,
        regionPopupVisible: false
      })
    }
  },

  // ==================== 输入处理 ====================

  onInput(e: any) {
    const field = e.currentTarget.dataset.field
    this.setData({ [`formData.${field}`]: e.detail.value })
  },

  onYearsChange(e: any) {
    this.setData({ 'formData.years': e.detail.value })
  },

  // ==================== 证照上传 (对齐集团模式) ====================

  async onIdFrontUpload(e: any) {
    const { path } = e.detail
    this.setData({ 'idFront.url': path })
    this.processOCR(ocrOperatorIdCard(path, 'Front'))
  },

  async onIdBackUpload(e: any) {
    const { path } = e.detail
    this.setData({ 'idBack.url': path })
    this.processOCR(ocrOperatorIdCard(path, 'Back'))
  },

  async onLicenseUpload(e: any) {
    const { path } = e.detail
    this.setData({ 'license.url': path })
    this.processOCR(ocrOperatorBusinessLicense(path))
  },

  async processOCR(ocrPromise: Promise<any>) {
    wx.showLoading({ title: '智能识别中...' })
    try {
      const res = await ocrPromise
      this.mapResponseToData(res)
      wx.hideLoading()
      wx.showToast({ title: '自动识别成功', icon: 'none' })
    } catch (e) {
      wx.hideLoading()
      logger.error('OCR failed', e)
    }
  },

  // ==================== 流程导航 ====================

  onPrev() {
    this.setData({ currentStep: this.data.currentStep - 1 })
  },

  async onNext() {
    const { currentStep, formData, license, idFront, idBack } = this.data

    // 从介绍页进入 Step 1
    if (currentStep === 0) {
      this.setData({ currentStep: 1 })
      return
    }

    // 从 Step 1 进入 Step 2：锁定区域并保存基础信息
    if (currentStep === 1) {
      const { name, contactName, contactPhone, years, regionId } = formData

      // 1. 本地前置校验
      if (!regionId) return wx.showToast({ title: '请选择运营区域', icon: 'none' })
      if (!name || name.length < 2) return wx.showToast({ title: '运营商名称至少2位', icon: 'none' })
      if (!contactName || contactName.length < 2) return wx.showToast({ title: '负责人姓名至少2位', icon: 'none' })
      if (!contactPhone || contactPhone.length !== 11) return wx.showToast({ title: '请输入11位手机号', icon: 'none' })
      
      wx.showLoading({ title: '锁定区域中...', mask: true })
      try {
        // 1. 创建/获取草稿
        const res = await getOrCreateOperatorApplication({ region_id: regionId })
        this.mapResponseToData(res)
        
        // 2. 更新基础信息
        const updated = await updateOperatorBasic({
          name: name,
          contact_name: contactName,
          contact_phone: contactPhone,
          requested_contract_years: years
        })
        this.mapResponseToData(updated)
        
        this.setData({ currentStep: 2 })
      } catch (e: any) {
        logger.error('Step 1 sync failed', e)
        // 优先显示后端返回的精准消息
        const msg = e.userMessage || e.data?.message || '锁定区域失败，可能已被占用'
        wx.showToast({ title: msg, icon: 'none' })
      } finally {
        wx.hideLoading()
      }
      return
    }

    // 从 Step 2 进入 Step 3：验证图片是否上传
    if (currentStep === 2) {
      if (!license.url || !idFront.url || !idBack.url) {
        return wx.showToast({ title: '请上传所有资质原件', icon: 'none' })
      }
      this.setData({ currentStep: 3 })
      return
    }
  },

  /**
   * 提交最终申请
   */
  async onSubmit() {
    this.setData({ isSubmitting: true })
    wx.showLoading({ title: '正式提交申请...', mask: true })
    try {
      await submitOperatorApplication()
      this.setData({ currentStep: 4 })
    } catch (e: any) {
      wx.showToast({ title: e.userMessage || '提交审核失败', icon: 'none' })
    } finally {
      this.setData({ isSubmitting: false })
      wx.hideLoading()
    }
  },

  onBackHome() {
    wx.switchTab({ url: '/pages/user_center/index' })
  }
})
