import { 
  getOrCreateOperatorApplication, 
  getOperatorApplication,
  resetOperatorApplication,
  updateOperatorRegion,
  updateOperatorBasic, 
  deleteOperatorApplicationDocument,
  ocrOperatorBusinessLicense, 
  ocrOperatorIdCard, 
  submitOperatorApplication,
  type OperatorApplicationResponse
} from '../_main_shared/api/operator-application'
import { logger } from '../../../utils/logger'
import Navigation from '../../../utils/navigation'
import { buildAgreementConsentPayload } from '../_main_shared/api/agreement-consent'
import {
  buildOperatorApplicationPatch,
  buildOperatorBasicPayload,
  buildSelectedRegionPatch,
  CityOption,
  ProvinceOption,
  createUploadFeedback,
  DEFAULT_OPERATOR_OCR_DISPLAY_STATE,
  DEFAULT_OPERATOR_UPLOAD_FEEDBACK,
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
} from './_utils/operator-registration-view'
import {
  buildOperatorUploadStartPatch,
  buildRegionNamePatch,
  collectOperatorUploadPreviewPatch,
  fetchOperatorAvailableRegionsByCity,
  fetchOperatorCityOptions,
  fetchOperatorProvinceOptions,
  handleExistingOperatorApplication,
  resolveOperatorDefaultRegionPatch
} from './_utils/operator-registration-support'

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
    selectedProvinceIndex: 0,
    selectedProvinceId: 0,
    selectedProvinceName: '',
    cityOptions: [] as CityOption[],
    selectedCityIndex: 0,
    selectedCityId: 0,
    selectedCityName: '',
    selectedDistrictName: '',
    regionPickerValue: [] as number[],
    regionOptions: [] as RegionOption[],
    regionPickerPopupProps: {
      preventScrollThrough: true,
      overlayProps: {
        preventScrollThrough: true
      }
    },
    ocrDisplayState: DEFAULT_OPERATOR_OCR_DISPLAY_STATE,
    uploadFeedback: DEFAULT_OPERATOR_UPLOAD_FEEDBACK,
    phoneError: '',
    consentChecked: false,
    consentPopupVisible: false
  },

  previewRefreshVersion: 0,
  regionPickerSnapshot: null as null | {
    selectedProvinceIndex: number
    selectedProvinceId: number
    selectedProvinceName: string
    selectedCityIndex: number
    selectedCityId: number
    selectedCityName: string
    selectedDistrictName: string
    regionPickerValue: number[]
    cityOptions: CityOption[]
    regionOptions: RegionOption[]
  },

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
        fetchAvailableRegionsByCity: (cityId: number) => this.fetchAvailableRegionsByCity(cityId)
      })
      if (patch) {
        this.setData(patch)
      }
    } catch (e: unknown) {
      logger.warn('Init default region from location failed', e)
    }
  },

  async fetchAvailableRegionsByCity(cityID: number) {
    try {
      this.setData(await fetchOperatorAvailableRegionsByCity(cityID), () => {
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
      currentFormData: this.data.formData,
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
    if (!this.data.provinceOptions.length) {
      await this.fetchProvinceOptions()
    }

    if (!this.data.cityOptions.length) {
      await this.fetchCityOptions(this.data.selectedProvinceId)
    }

    if (this.data.selectedCityId > 0) {
      await this.fetchAvailableRegionsByCity(this.data.selectedCityId)
    }

    this.regionPickerSnapshot = {
      selectedProvinceIndex: this.data.selectedProvinceIndex,
      selectedProvinceId: this.data.selectedProvinceId,
      selectedProvinceName: this.data.selectedProvinceName,
      selectedCityIndex: this.data.selectedCityIndex,
      selectedCityId: this.data.selectedCityId,
      selectedCityName: this.data.selectedCityName,
      selectedDistrictName: this.data.selectedDistrictName,
      regionPickerValue: [this.data.selectedProvinceId, this.data.selectedCityId, this.data.formData.regionId],
      cityOptions: [...this.data.cityOptions],
      regionOptions: [...this.data.regionOptions]
    }

    this.setData({
      regionPopupVisible: true,
      regionPickerValue: [this.data.selectedProvinceId, this.data.selectedCityId, this.data.formData.regionId]
    })
  },

  onCloseRegionPopup() {
    if (this.regionPickerSnapshot) {
      this.setData({
        regionPopupVisible: false,
        selectedProvinceIndex: this.regionPickerSnapshot.selectedProvinceIndex,
        selectedProvinceId: this.regionPickerSnapshot.selectedProvinceId,
        selectedProvinceName: this.regionPickerSnapshot.selectedProvinceName,
        selectedCityIndex: this.regionPickerSnapshot.selectedCityIndex,
        selectedCityId: this.regionPickerSnapshot.selectedCityId,
        selectedCityName: this.regionPickerSnapshot.selectedCityName,
        selectedDistrictName: this.regionPickerSnapshot.selectedDistrictName,
        regionPickerValue: this.regionPickerSnapshot.regionPickerValue,
        cityOptions: this.regionPickerSnapshot.cityOptions,
        regionOptions: this.regionPickerSnapshot.regionOptions
      })
      this.regionPickerSnapshot = null
      return
    }

    this.setData({ regionPopupVisible: false })
  },

  async onRegionPickerPick(e: WechatMiniprogram.CustomEvent<{
    value?: Array<string | number>
    column?: number
  }>) {
    const values = Array.isArray(e.detail?.value) ? e.detail.value.map((value) => Number(value)) : []
    const column = Number(e.detail?.column ?? -1)

    this.setData({ regionPickerValue: values })

    if (column === 0) {
      const provinceId = Number(values[0] || 0)
      const index = this.data.provinceOptions.findIndex((item) => item.value === provinceId)
      const province = index >= 0 ? this.data.provinceOptions[index] : null
      if (!province || province.value === this.data.selectedProvinceId) {
        return
      }

      this.setData({
        selectedProvinceIndex: index,
        selectedProvinceId: province.value,
        selectedProvinceName: province.label,
        cityOptions: [],
        selectedCityIndex: 0,
        selectedCityId: 0,
        selectedCityName: '',
        selectedDistrictName: '',
        regionOptions: []
      })

      await this.fetchCityOptions(province.value, true)
      this.setData({
        regionPickerValue: [province.value, this.data.selectedCityId, this.data.regionOptions[0]?.value || 0],
        selectedDistrictName: this.data.regionOptions[0]?.label || ''
      })
      return
    }

    if (column === 1) {
      const cityId = Number(values[1] || 0)
      const index = this.data.cityOptions.findIndex((item) => item.value === cityId)
      const city = index >= 0 ? this.data.cityOptions[index] : null
      if (!city || city.value === this.data.selectedCityId) {
        return
      }

      this.setData({
        selectedCityIndex: index,
        selectedCityId: city.value,
        selectedCityName: city.label,
        selectedDistrictName: '',
        regionOptions: []
      })

      await this.fetchAvailableRegionsByCity(city.value)
      this.setData({
        regionPickerValue: [this.data.selectedProvinceId, city.value, this.data.regionOptions[0]?.value || 0],
        selectedDistrictName: this.data.regionOptions[0]?.label || ''
      })
      return
    }

    if (column === 2) {
      const regionId = Number(values[2] || 0)
      const region = this.data.regionOptions.find((item) => Number(item.value) === regionId)
      if (region) {
        this.setData({
          selectedDistrictName: region.label,
          regionPickerValue: [this.data.selectedProvinceId, this.data.selectedCityId, region.value]
        })
      }
    }
  },

  onRegionPickerConfirm(e: WechatMiniprogram.CustomEvent<{ value: Array<string | number> | null }>) {
    const values = Array.isArray(e.detail?.value) ? e.detail.value : []
    const regionId = Number(values[2] || 0)
    const region = this.data.regionOptions.find((r) => Number(r.value) === regionId)

    if (region) {
      this.setData(buildSelectedRegionPatch({
        region,
        cityOptions: this.data.cityOptions,
        selectedCityName: this.data.selectedCityName
      }))
      this.regionPickerSnapshot = null
      return
    }

    this.onCloseRegionPopup()
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

  async syncDraftBeforeStepTwo() {
    const existing = await getOperatorApplication()
    if (existing?.id) {
      if (existing.status && existing.status !== 'draft') {
        throw new Error('当前申请状态不支持继续编辑，请刷新页面后重试')
      }

      if (Number(existing.region_id || 0) !== Number(this.data.formData.regionId || 0)) {
        return updateOperatorRegion({ region_id: this.data.formData.regionId })
      }

      return existing
    }

    return getOrCreateOperatorApplication({ region_id: this.data.formData.regionId })
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
        const res = await this.syncDraftBeforeStepTwo()
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
