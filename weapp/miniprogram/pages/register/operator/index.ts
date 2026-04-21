import { 
  getOrCreateOperatorApplication, 
  getOperatorApplication,
  resetOperatorApplication,
  updateOperatorBasic, 
  deleteOperatorApplicationDocument,
  ocrOperatorBusinessLicense, 
  ocrOperatorIdCard, 
  submitOperatorApplication,
  type OperatorApplicationResponse
} from '../../../api/operator-application'
import { logger } from '../../../utils/logger'
import Navigation from '../../../utils/navigation'
import { buildAgreementConsentPayload } from '../../../api/agreement-consent'
import {
  buildCityChangePatch,
  buildProvinceChangePatch,
  buildOperatorApplicationPatch,
  buildOperatorBasicPayload,
  buildSelectedRegionPatch,
  CityOption,
  ProvinceOption,
  createUploadFeedback,
  DEFAULT_OPERATOR_OCR_DISPLAY_STATE,
  DEFAULT_OPERATOR_UPLOAD_FEEDBACK,
  extractRegionSearchKeyword,
  FormDataValue,
  getErrorText,
  getOperatorDocumentRemovalData,
  getOperatorStepOneValidationMessage,
  isNoOperatorApplicationError,
  OCRDisplayStateValue,
  OperatorOCRDisplayState,
  OperatorUploadFeedback,
  OperatorUploadField,
  RegionOption,
  UploadEvent,
  UploadFeedback,
  UploadFieldValue
} from '../../../utils/operator-registration-view'
import {
  buildOperatorUploadStartPatch,
  buildRegionNamePatch,
  collectOperatorUploadPreviewPatch,
  fetchOperatorAvailableRegionsByCity,
  fetchOperatorCityOptions,
  fetchOperatorProvinceOptions,
  handleExistingOperatorApplication,
  resolveOperatorDefaultRegionPatch
} from '../../../utils/operator-registration-support'

Page({
  data: {
    navBarHeight: 88,
    currentStep: 0,
    isSubmitting: false,
    idFront: { url: '', rawUrl: '', assetId: undefined } as UploadFieldValue,
    idBack: { url: '', rawUrl: '', assetId: undefined } as UploadFieldValue,
    license: { url: '', rawUrl: '', assetId: undefined } as UploadFieldValue,
    formData: {
      regionId: 0,
      regionName: '',
      name: '',
      contactName: '',
      contactPhone: '',
      years: 3
    },
    regionPopupVisible: false,
    provinceOptions: [] as ProvinceOption[],
    provincePickerVisible: false,
    selectedProvinceIndex: 0,
    selectedProvinceId: 0,
    selectedProvinceName: '',
    cityOptions: [] as CityOption[],
    cityPickerVisible: false,
    selectedCityIndex: 0,
    selectedCityId: 0,
    selectedCityName: '',
    regionKeyword: '',
    regionSearchTimer: null as number | null,
    lastRegionSearchKeyword: '',
    lastRegionSearchCityId: 0,
    regionOptions: [] as RegionOption[],     // 原始列表
    filteredRegions: [] as RegionOption[],   // 搜索过滤后的列表
    ocrDisplayState: DEFAULT_OPERATOR_OCR_DISPLAY_STATE,
    uploadFeedback: DEFAULT_OPERATOR_UPLOAD_FEEDBACK,
    phoneError: '',
    consentChecked: false,
    consentPopupVisible: false
  },

  previewRefreshVersion: 0,

  async onLoad() {
    await this.initApplication()
    if (!this.data.formData.regionId) {
      await this.initDefaultRegionFromLocation()
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },
  async initApplication() {
    try {
      const res = await getOperatorApplication()
      if (res?.id) {
        this.mapResponseToData(res)
        const currentStep = handleExistingOperatorApplication(
          res,
          getApp<IAppOption>().globalData.userRole,
          () => { void this.restartRejectedApplication() }
        )
        if (currentStep > 0) {
          this.setData({ currentStep })
        }
      }
    } catch (e: unknown) {
      if (isNoOperatorApplicationError(e)) {
        const globalApp = getApp<IAppOption>()
        if (globalApp.globalData.userRole === 'operator') {
          wx.redirectTo({ url: '/pages/operator/region-expansion/index' })
          return
        }
      } else {
        logger.error('Init operator application failed', e)
      }
    }
  },

  async restartRejectedApplication() {
    wx.showLoading({ title: '恢复可编辑状态...', mask: true })
    try {
      const draft = await resetOperatorApplication()
      this.mapResponseToData(draft)
      this.setData({ currentStep: 1 })
    } catch (e: unknown) {
      logger.error('Reset rejected operator application failed', e)
      wx.showToast({ title: getErrorText(e, '恢复失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  async fetchCityOptions(provinceId?: number, withRegions: boolean = true) {
    const targetProvinceId = Number(provinceId || this.data.selectedProvinceId || 0)
    try {
      const result = await fetchOperatorCityOptions(targetProvinceId, this.data.selectedCityId)
      this.setData(result.patch)
      if (withRegions && result.selectedCityId > 0) {
        await this.fetchAvailableRegionsByCity(result.selectedCityId)
      }
    } catch (e: unknown) {
      logger.error('Fetch city regions failed', e)
    }
  },

  async fetchProvinceOptions(withCities: boolean = true) {
    try {
      const result = await fetchOperatorProvinceOptions(this.data.selectedProvinceId)
      this.setData(result.patch)
      if (withCities && result.selectedProvinceId > 0) {
        await this.fetchCityOptions(result.selectedProvinceId, true)
      }
    } catch (e: unknown) {
      logger.error('Fetch province regions failed', e)
    }
  },

  async initDefaultRegionFromLocation() {
    try {
      const patch = await resolveOperatorDefaultRegionPatch({
        getProvinceOptions: () => this.data.provinceOptions,
        getCityOptions: () => this.data.cityOptions,
        getRegionOptions: () => this.data.regionOptions,
        fetchProvinceOptions: (withCities?: boolean) => this.fetchProvinceOptions(withCities),
        fetchCityOptions: (provinceId: number, withRegions?: boolean) => this.fetchCityOptions(provinceId, withRegions),
        fetchAvailableRegionsByCity: (cityId: number, keyword?: string) => this.fetchAvailableRegionsByCity(cityId, keyword)
      })
      if (patch) {
        this.setData(patch)
      }
    } catch (e: unknown) {
      logger.warn('Init default region from location failed', e)
    }
  },

  async fetchAvailableRegionsByCity(cityID: number, keyword: string = '') {
    try {
      this.setData(await fetchOperatorAvailableRegionsByCity(cityID, keyword), () => {
        const currentRegionName = (this.data.formData.regionName || '').trim()
        if (this.data.formData.regionId && (!currentRegionName || !currentRegionName.includes(' - '))) {
          this.syncRegionName(this.data.formData.regionId)
        }
      })
    } catch (e: unknown) {
      logger.error('Fetch available districts failed', e)
    }
  },

  async refreshUploadPreviewURLs() {
    const refreshVersion = ++this.previewRefreshVersion
    const uploads: Array<{ key: 'license' | 'idFront' | 'idBack', value: UploadFieldValue }> = [
      { key: 'license', value: this.data.license },
      { key: 'idFront', value: this.data.idFront },
      { key: 'idBack', value: this.data.idBack }
    ]

    const patch = await collectOperatorUploadPreviewPatch({
      refreshVersion,
      uploads,
      getLatestUploadValue: (key) => this.data[key] as UploadFieldValue | undefined
    })
    if (refreshVersion === this.previewRefreshVersion && Object.keys(patch).length > 0) {
      this.setData(patch)
    }
  },
  mapResponseToData(res: OperatorApplicationResponse) {
    if (!res) return

    this.setData(buildOperatorApplicationPatch({
      res,
      regionOptions: this.data.regionOptions,
      phoneError: this.data.phoneError,
      uploads: {
        license: this.data.license,
        idFront: this.data.idFront,
        idBack: this.data.idBack
      }
    }), () => {
      void this.refreshUploadPreviewURLs()
    })
  },

  setOCRState(type: keyof OperatorOCRDisplayState, status: OCRDisplayStateValue) {
    this.setData({ [`ocrDisplayState.${type}`]: status })
  },

  setUploadFeedback(field: keyof OperatorUploadFeedback, feedback: UploadFeedback) {
    this.setData({ [`uploadFeedback.${field}`]: feedback })
  },
  syncRegionName(regionId: number | string) {
    const patch = buildRegionNamePatch(this.data.regionOptions, regionId)
    if (patch) {
      this.setData(patch)
    }
  },

  async onOpenRegionPopup() {
    this.setData({ regionPopupVisible: true })

    if (!this.data.provinceOptions.length) {
      await this.fetchProvinceOptions()
      return
    }

    if (!this.data.cityOptions.length) {
      await this.fetchCityOptions(this.data.selectedProvinceId)
      return
    }

    if (this.data.selectedCityId > 0) {
      await this.fetchAvailableRegionsByCity(this.data.selectedCityId, this.data.regionKeyword)
    }
  },

  onCloseRegionPopup() {
    if (this.data.regionSearchTimer) {
      clearTimeout(this.data.regionSearchTimer)
    }
    this.setData({ regionPopupVisible: false })
  },

  onRegionSearch(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    const keyword = extractRegionSearchKeyword(e.detail as unknown)
    if (this.data.regionSearchTimer) {
      clearTimeout(this.data.regionSearchTimer)
    }

    this.setData({ regionKeyword: keyword })

    if (!this.data.selectedCityId) return

    if (
      keyword === this.data.lastRegionSearchKeyword &&
      this.data.selectedCityId === this.data.lastRegionSearchCityId
    ) {
      return
    }

    const timer = setTimeout(() => {
      this.setData({
        lastRegionSearchKeyword: keyword,
        lastRegionSearchCityId: this.data.selectedCityId
      })
      this.fetchAvailableRegionsByCity(this.data.selectedCityId, keyword)
    }, 300)

    this.setData({ regionSearchTimer: timer })
  },

  onRegionSearchClear() {
    if (this.data.regionSearchTimer) {
      clearTimeout(this.data.regionSearchTimer)
    }
    this.setData({
      regionKeyword: '',
      filteredRegions: this.data.regionOptions,
      regionSearchTimer: null,
      lastRegionSearchKeyword: '',
      lastRegionSearchCityId: this.data.selectedCityId
    })

    if (this.data.selectedCityId) {
      this.fetchAvailableRegionsByCity(this.data.selectedCityId, '')
    }
  },

  async onProvinceChange(e: WechatMiniprogram.CustomEvent<{ value?: string | number }>) {
    const rawIndex = e.detail.value
    const index = Number(rawIndex)
    if (!Number.isFinite(index)) return

    const province = this.data.provinceOptions[index]
    if (!province) return

    this.setData({
      selectedProvinceIndex: index,
      ...buildProvinceChangePatch(province)
    })

    await this.fetchCityOptions(province.value, true)
  },

  onOpenProvincePicker() {
    if (!this.data.provinceOptions.length) return
    this.setData({ provincePickerVisible: true })
  },

  onCloseProvincePicker() {
    this.setData({ provincePickerVisible: false })
  },

  async onProvincePickerConfirm(e: WechatMiniprogram.CustomEvent<{ value: Array<string | number> | null }>) {
    const values = Array.isArray(e.detail?.value) ? e.detail.value : []
    const selectedValue = Number(values[0] || 0)
    const index = this.data.provinceOptions.findIndex((item) => item.value === selectedValue)
    const province = index >= 0 ? this.data.provinceOptions[index] : null
    if (!province) {
      this.setData({ provincePickerVisible: false })
      return
    }

    this.setData({ provincePickerVisible: false })
    await this.onProvinceChange({ detail: { value: index } } as WechatMiniprogram.CustomEvent<{ value?: string | number }>)
  },

  async onCityChange(e: WechatMiniprogram.CustomEvent<{ value?: string | number }>) {
    const rawIndex = e.detail.value
    const index = Number(rawIndex)
    if (!Number.isFinite(index)) return

    const city = this.data.cityOptions[index]
    if (!city) return

    this.setData({
      selectedCityIndex: index,
      ...buildCityChangePatch(city)
    })

    await this.fetchAvailableRegionsByCity(city.value)
  },

  onOpenCityPicker() {
    if (!this.data.cityOptions.length) return
    this.setData({ cityPickerVisible: true })
  },

  onCloseCityPicker() {
    this.setData({ cityPickerVisible: false })
  },

  async onCityPickerConfirm(e: WechatMiniprogram.CustomEvent<{ value: Array<string | number> | null }>) {
    const values = Array.isArray(e.detail?.value) ? e.detail.value : []
    const selectedValue = Number(values[0] || 0)
    const index = this.data.cityOptions.findIndex((item) => item.value === selectedValue)
    const city = index >= 0 ? this.data.cityOptions[index] : null
    if (!city) {
      this.setData({ cityPickerVisible: false })
      return
    }

    this.setData({ cityPickerVisible: false })
    await this.onCityChange({ detail: { value: index } } as WechatMiniprogram.CustomEvent<{ value?: string | number }>)
  },

  onSelectRegion(e: WechatMiniprogram.CustomEvent<{ value?: number | string }>) {
    const regionId = Number(e.detail.value)
    const region = this.data.regionOptions.find((r) => Number(r.value) === regionId)
    
    if (region) {
      this.setData(buildSelectedRegionPatch({
        region,
        cityOptions: this.data.cityOptions,
        selectedCityName: this.data.selectedCityName
      }))
    }
  },

  onInput(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    const field = (e.currentTarget.dataset as { field?: keyof FormDataValue }).field
    if (!field) return

    const value = e.detail.value || ''
    const nextData: Record<string, string> = { [`formData.${field}`]: value }
    if (field === 'contactPhone' && value.trim()) {
      nextData.phoneError = ''
    }
    this.setData(nextData)
  },

  onYearsChange(e: WechatMiniprogram.CustomEvent<{ value?: number }>) {
    this.setData({ 'formData.years': e.detail.value || 3 })
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

  async onIdFrontUpload(e: UploadEvent) {
    const { path } = e.detail
    if (!path) return
    this.setData(buildOperatorUploadStartPatch('idFront', path))
    this.processOCR(
      ocrOperatorIdCard(path, 'Front'),
      'idCard',
      'idFront'
    )
  },

  async onIdBackUpload(e: UploadEvent) {
    const { path } = e.detail
    if (!path) return
    this.setData(buildOperatorUploadStartPatch('idBack', path))
    this.processOCR(
      ocrOperatorIdCard(path, 'Back'),
      'idCard',
      'idBack'
    )
  },

  async onLicenseUpload(e: UploadEvent) {
    const { path } = e.detail
    if (!path) return
    this.setData(buildOperatorUploadStartPatch('license', path))
    this.processOCR(
      ocrOperatorBusinessLicense(path),
      'businessLicense',
      'license'
    )
  },

  async removeUploadedDocument(field: OperatorUploadField) {
    const target = getOperatorDocumentRemovalData(field)

    wx.showLoading({ title: '删除中...' })
    try {
      const res = await deleteOperatorApplicationDocument(target.documentType)
      this.setData(target.data, () => {
        this.mapResponseToData(res)
      })
    } catch (e) {
      logger.error('Delete operator application document failed', { field, error: e })
      wx.showToast({ title: getErrorText(e, '删除失败，请重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  onLicenseRemove() {
    this.removeUploadedDocument('license')
  },

  onIdFrontRemove() {
    this.removeUploadedDocument('idFront')
  },

  onIdBackRemove() {
    this.removeUploadedDocument('idBack')
  },

  async processOCR(
    ocrPromise: Promise<OperatorApplicationResponse>,
    type: keyof OperatorOCRDisplayState,
    feedbackField: keyof OperatorUploadFeedback
  ) {
    this.setOCRState(type, 'processing')
    this.setUploadFeedback(feedbackField, createUploadFeedback('processing', '证照识别中', '请稍候，识别结果会显示在当前卡片中'))
    try {
      const res = await ocrPromise
      this.mapResponseToData(res)
    } catch (e) {
      logger.error('OCR failed', e)
      const message = getErrorText(e, '识别失败，请提供更清晰更规整的图片重试')
      this.setOCRState(type, 'failed')
      this.setUploadFeedback(feedbackField, createUploadFeedback('error', '识别失败', message))
    }
  },

  onPrev() {
    this.setData({ currentStep: this.data.currentStep - 1 })
  },

  async onNext() {
    const { currentStep, formData, idFront, idBack } = this.data
    if (currentStep === 0) {
      if (!this.ensureConsent()) return
      this.setData({ currentStep: 1 })
      return
    }
    if (currentStep === 1) {
      const validationMessage = getOperatorStepOneValidationMessage(formData)
      if (validationMessage === '请输入11位手机号') {
        this.setData({ phoneError: '请填写 11 位联系电话，方便总部联系你' })
        return wx.showToast({ title: validationMessage, icon: 'none' })
      }
      if (validationMessage) {
        return wx.showToast({ title: validationMessage, icon: 'none' })
      }
      this.setData({ phoneError: '' })
      
      wx.showLoading({ title: '锁定区域中...', mask: true })
      try {
        const res = await getOrCreateOperatorApplication({ region_id: formData.regionId })
        this.mapResponseToData(res)
        const updated = await updateOperatorBasic(buildOperatorBasicPayload(formData))
        this.mapResponseToData(updated)
        this.setData({ currentStep: 2 })
      } catch (e: unknown) {
        logger.error('Step 1 sync failed', e)
        const msg = getErrorText(e, '锁定区域失败，可能已被占用')
        wx.showToast({ title: msg, icon: 'none' })
      } finally {
        wx.hideLoading()
      }
      return
    }
    if (currentStep === 2) {
      if (!idFront.url || !idBack.url) {
        return wx.showToast({ title: '请上传法人身份证正反面', icon: 'none' })
      }
      this.setData({ currentStep: 3 })
      return
    }
  },
  async onSubmit() {
    if (!this.ensureConsent()) {
      return
    }

    let consentPayload
    try {
      consentPayload = await buildAgreementConsentPayload()
    } catch (e: unknown) {
      wx.showToast({ title: getErrorText(e, '协议信息加载失败，请稍后重试'), icon: 'none' })
      return
    }

    this.setData({ isSubmitting: true })
    wx.showLoading({ title: '正式提交申请...', mask: true })
    try {
      await submitOperatorApplication(consentPayload)
      this.setData({ currentStep: 4 })
    } catch (e: unknown) {
      wx.showToast({ title: getErrorText(e, '提交审核失败'), icon: 'none' })
    } finally {
      this.setData({ isSubmitting: false })
      wx.hideLoading()
    }
  },
  async onNextOrSubmit() {
    if (this.data.currentStep === 3) {
      return this.onSubmit()
    }
    return this.onNext()
  },

  onBackHome() {
    wx.switchTab({ url: '/pages/user_center/index' })
  }
})
