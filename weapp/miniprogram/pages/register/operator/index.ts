import { 
  getOrCreateOperatorApplication, 
  getOperatorApplication,
  updateOperatorBasic, 
  ocrOperatorBusinessLicense, 
  ocrOperatorIdCard, 
  submitOperatorApplication,
  listAvailableRegions,
  listRegions,
  type OperatorApplicationResponse
} from '../../../api/operator-application'
import { getCurrentRegion, type CurrentRegionResponse } from '../../../api/location'
import { logger } from '../../../utils/logger'
import { locationService, type LocationInfo } from '../../../utils/location'
import { getPrivateMediaUrl } from '../../../utils/image-security'
import Navigation from '../../../utils/navigation'
import { buildAgreementConsentPayload } from '../../../api/agreement-consent'

type CityOption = {
  label: string
  value: number
}

type RegionOption = {
  label: string
  secondary: string
  value: number
  parentId?: number
}

type FormDataValue = {
  regionId: number
  regionName: string
  name: string
  contactName: string
  contactPhone: string
  years: number
}

type UploadEvent = WechatMiniprogram.CustomEvent<{ path?: string }>

type UploadFieldValue = {
  url: string
  rawUrl?: string
  assetId?: number
}

type OCRDisplayStateValue = 'idle' | 'processing' | 'done' | 'failed'

type OperatorOCRDisplayState = {
  businessLicense: OCRDisplayStateValue
  idCard: OCRDisplayStateValue
}

const DEFAULT_OPERATOR_OCR_DISPLAY_STATE: OperatorOCRDisplayState = {
  businessLicense: 'idle',
  idCard: 'idle'
}

function getOCRString(payload: Record<string, unknown> | undefined, key: string): string {
  const value = payload?.[key]
  return typeof value === 'string' ? value.trim() : ''
}

function getErrorText(error: unknown, fallback: string): string {
  if (error && typeof error === 'object' && 'userMessage' in error) {
    const userMessage = (error as { userMessage?: string }).userMessage
    if (userMessage) return userMessage
  }
  if (error && typeof error === 'object' && 'data' in error) {
    const data = (error as { data?: { message?: string } }).data
    if (data?.message) return data.message
  }
  return fallback
}

function isNoOperatorApplicationError(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false

  const maybeError = error as {
    statusCode?: number
    userMessage?: string
    message?: string
    data?: { code?: number, message?: string }
  }

  if (maybeError.statusCode === 404) return true

  const userMessage = maybeError.userMessage || ''
  const message = maybeError.message || ''
  const dataMessage = maybeError.data?.message || ''
  const dataCode = maybeError.data?.code

  if (dataCode === 40400) return true

  const fullMessage = `${userMessage} ${message} ${dataMessage}`
  return fullMessage.includes('您还没有申请记录') || fullMessage.includes('40400')
}

function normalizeRegionText(value: string): string {
  return value
    .trim()
    .replace(/\s+/g, '')
    .replace(/特别行政区|自治州|自治县|地区|盟/g, '')
    .replace(/[市区县]$/g, '')
    .replace(/區/g, '区')
    .replace(/灣/g, '湾')
    .replace(/東/g, '东')
    .replace(/龍/g, '龙')
    .replace(/環/g, '环')
    .replace(/臺/g, '台')
    .toLowerCase()
}

function buildRegionFullName(region: RegionOption): string {
  return region.secondary ? `${region.secondary} - ${region.label}` : region.label
}

Page({
  data: {
    navBarHeight: 88,
    currentStep: 0,
    isSubmitting: false,
    idFront: { url: '', rawUrl: '', assetId: undefined } as UploadFieldValue,
    idBack: { url: '', rawUrl: '', assetId: undefined } as UploadFieldValue,
    license: { url: '', rawUrl: '', assetId: undefined } as UploadFieldValue,
    
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
    cityOptions: [] as CityOption[],
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
    consentChecked: false,
    consentPopupVisible: false
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
        } else if (res.status === 'approved' && res.is_operator) {
          // 已是正式运营商：询问去控制台还是申请更多区域
          wx.showModal({
            title: '您已是运营商',
            content: '您的入驻已完成，请选择下一步操作',
            confirmText: '进入控制台',
            cancelText: '申请更多区域',
            success(r) {
              if (r.confirm) {
                wx.reLaunch({ url: '/pages/operator/dashboard/index' })
              } else if (r.cancel) {
                wx.reLaunch({ url: '/pages/operator/region-expansion/index' })
              }
            }
          })
        } else if (res.status === 'approved' && !res.is_operator) {
          // 审核通过但运营商账号尚未建立（极少数中间态）→ 正常进开户流程
          wx.showToast({ title: '审核通过，请先完成微信开户', icon: 'none' })
          setTimeout(() => wx.reLaunch({ url: '/pages/operator/applyment/index' }), 1500)
        } else if (res.status === 'rejected') {
          wx.showModal({
            title: '审核未通过',
            content: `原因：${res.reject_reason || '资料核验失败'}`,
            confirmText: '修改资料'
          })
        }
      }
    } catch (e: unknown) {
      if (isNoOperatorApplicationError(e)) {
        // 无申请记录 — 若当前用户已是运营商，直接跳转申请更多区域页
        const globalApp = getApp<IAppOption>()
        if (globalApp.globalData.userRole === 'operator') {
          wx.redirectTo({ url: '/pages/operator/region-expansion/index' })
          return
        }
        // 否则是新用户，留在介绍页（正常注册流程）
      } else {
        logger.error('Init operator application failed', e)
      }
    }
  },

  async fetchCityOptions(withRegions: boolean = true) {
    try {
      const cities: CityOption[] = []
      let pageID = 1

      for (;;) {
        const items = await listRegions({ page_id: pageID, page_size: 100, level: 2 })
        if (!items || items.length === 0) break

        items.forEach((item) => {
          cities.push({
            label: item.name,
            value: item.id
          })
        })

        if (items.length < 100) break
        pageID += 1
      }

      const selectedCityId = this.data.selectedCityId || (cities[0]?.value || 0)
      const selectedCityIndex = Math.max(0, cities.findIndex((item) => item.value === selectedCityId))
      const selectedCityName = cities[selectedCityIndex]?.label || ''

      this.setData({
        cityOptions: cities,
        selectedCityIndex,
        selectedCityId,
        selectedCityName
      })

      if (withRegions && selectedCityId > 0) {
        await this.fetchAvailableRegionsByCity(selectedCityId)
      }
    } catch (e: unknown) {
      logger.error('Fetch city regions failed', e)
    }
  },

  async getCurrentLocationForRegion(): Promise<LocationInfo | null> {
    const cached = locationService.getFromGlobal()
    if (cached?.city || cached?.district) {
      return cached
    }

    try {
      return await locationService.getLocationWithPermission()
    } catch (e: unknown) {
      logger.warn('Get location for default region failed', e)
      return null
    }
  },

  async resolveCurrentRegionByLocation(location: LocationInfo): Promise<CurrentRegionResponse | null> {
    const latitude = Number(location.latitude)
    const longitude = Number(location.longitude)

    if (!Number.isFinite(latitude) || !Number.isFinite(longitude)) {
      return null
    }

    try {
      return await getCurrentRegion({ latitude, longitude })
    } catch (e: unknown) {
      logger.warn('Resolve current region by location failed', e)
      return null
    }
  },

  findMatchedCityOption(cityName: string): CityOption | null {
    if (!cityName || !this.data.cityOptions.length) return null

    const target = normalizeRegionText(cityName)
    const exact = this.data.cityOptions.find((city) => {
      const current = normalizeRegionText(city.label)
      return current === target || current.includes(target) || target.includes(current)
    })
    if (exact) return exact

    if (target.includes('香港')) {
      return this.data.cityOptions.find((city) => city.label.includes('香港')) || null
    }

    return null
  },

  findMatchedDistrictOption(districtName: string): RegionOption | null {
    if (!districtName || !this.data.regionOptions.length) return null

    const target = normalizeRegionText(districtName)
    return this.data.regionOptions.find((district) => {
      const current = normalizeRegionText(district.label)
      return current === target || current.includes(target) || target.includes(current)
    }) || null
  },

  async initDefaultRegionFromLocation() {
    try {
      const location = await this.getCurrentLocationForRegion()
      if (!location) return

      const currentRegion = await this.resolveCurrentRegionByLocation(location)

      const cityName = String(location.city || '').trim()
      const districtName = String(location.district || '').trim()

      if (!currentRegion && (!cityName || !districtName)) return

      if (!this.data.cityOptions.length) {
        await this.fetchCityOptions(false)
      }

      if (currentRegion?.parent_id) {
        const matchedCity = this.data.cityOptions.find((item) => item.value === currentRegion.parent_id)
          || (currentRegion.parent_name ? this.findMatchedCityOption(currentRegion.parent_name) : null)

        if (matchedCity) {
          const cityIndex = this.data.cityOptions.findIndex((item) => item.value === matchedCity.value)
          this.setData({
            selectedCityIndex: Math.max(0, cityIndex),
            selectedCityId: matchedCity.value,
            selectedCityName: matchedCity.label
          })

          await this.fetchAvailableRegionsByCity(matchedCity.value)

          const matchedDistrict = this.data.regionOptions.find((item) => item.value === currentRegion.region_id)
            || this.findMatchedDistrictOption(currentRegion.region_name)

          if (matchedDistrict) {
            const fullName = `${matchedCity.label} - ${matchedDistrict.label}`
            this.setData({
              'formData.regionId': matchedDistrict.value,
              'formData.regionName': fullName
            })
            return
          }
        }
      }

      const city = this.findMatchedCityOption(cityName)
      if (!city) return

      const cityIndex = this.data.cityOptions.findIndex((item) => item.value === city.value)
      this.setData({
        selectedCityIndex: Math.max(0, cityIndex),
        selectedCityId: city.value,
        selectedCityName: city.label
      })

      await this.fetchAvailableRegionsByCity(city.value)

      const district = this.findMatchedDistrictOption(districtName)
      if (!district) return

      const fullName = `${city.label} - ${district.label}`
      this.setData({
        'formData.regionId': district.value,
        'formData.regionName': fullName
      })
    } catch (e: unknown) {
      logger.warn('Init default region from location failed', e)
    }
  },

  async fetchAvailableRegionsByCity(cityID: number, keyword: string = '') {
    try {
      const districts: RegionOption[] = []
      let pageID = 1
      const normalizedKeyword = keyword.trim()

      for (;;) {
        const query: {
          page_id: number
          page_size: number
          level: number
          parent_id: number
          keyword?: string
        } = {
          page_id: pageID,
          page_size: 100,
          level: 3,
          parent_id: cityID
        }

        if (normalizedKeyword) {
          query.keyword = normalizedKeyword
        }

        const res = await listAvailableRegions(query)
        const regions = res?.regions || []
        if (regions.length === 0) break

        regions.forEach((region) => {
          districts.push({
            label: region.name,
            secondary: region.parent_name || '',
            value: region.id,
            parentId: region.parent_id
          })
        })

        if (regions.length < 100) break
        pageID += 1
      }

      this.setData({
        regionOptions: districts,
        filteredRegions: normalizedKeyword
          ? districts.filter((item) =>
              item.label.toLowerCase().includes(normalizedKeyword.toLowerCase()) ||
              item.secondary.toLowerCase().includes(normalizedKeyword.toLowerCase())
            )
          : districts,
        regionKeyword: normalizedKeyword
      }, () => {
        const currentRegionName = (this.data.formData.regionName || '').trim()
        if (this.data.formData.regionId && (!currentRegionName || !currentRegionName.includes(' - '))) {
          this.syncRegionName(this.data.formData.regionId)
        }
      })
    } catch (e: unknown) {
      logger.error('Fetch available districts failed', e)
    }
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

  /**
   * 将后端响应映射到视图数据
   */
  mapResponseToData(res: OperatorApplicationResponse) {
    if (!res) return

    const newData: Record<string, unknown> = {
      'formData.regionId': Number(res.region_id || 0),
      'formData.name': String(res.name || ''),
      'formData.contactName': String(res.contact_name || ''),
      'formData.contactPhone': String(res.contact_phone || ''),
      'formData.years': Number(res.requested_contract_years || 3),
      idFront: { url: '', assetId: res.id_card_front_asset_id },
      idBack: { url: '', assetId: res.id_card_back_asset_id },
      license: { url: '', assetId: res.business_license_asset_id },
      ocrDisplayState: this.buildOperatorOcrDisplayState(res)
    }

    // 优先使用后端返回的名称，否则尝试从本地 Options 中反查
    const regionId = Number(res.region_id || 0)
    let regionName = String(res.region_name || '')

    if (!regionName && regionId && this.data.regionOptions.length > 0) {
      const matched = this.data.regionOptions.find((r) => Number(r.value) === Number(regionId))
      if (matched) {
        regionName = buildRegionFullName(matched)
      }
    }

    if (regionName && regionId && this.data.regionOptions.length > 0 && !regionName.includes(' - ')) {
      const matched = this.data.regionOptions.find((r) => Number(r.value) === Number(regionId))
      if (matched) {
        regionName = buildRegionFullName(matched)
      }
    }
    
    if (regionName) {
      newData['formData.regionName'] = regionName
    }

    this.setData(newData, () => {
      void this.refreshUploadPreviewURLs()
    })
  },

  buildOperatorOcrDisplayState(res?: OperatorApplicationResponse): OperatorOCRDisplayState {
    const businessLicenseUploaded = Boolean(res?.business_license_asset_id || this.data.license.assetId || this.data.license.url)
    const idCardUploaded = Boolean(
      (res?.id_card_front_asset_id || this.data.idFront.assetId || this.data.idFront.url)
      && (res?.id_card_back_asset_id || this.data.idBack.assetId || this.data.idBack.url)
    )

    const businessLicenseDone = Boolean(
      getOCRString(res?.business_license_ocr as Record<string, unknown> | undefined, 'enterprise_name')
      || getOCRString(res?.business_license_ocr as Record<string, unknown> | undefined, 'credit_code')
      || getOCRString(res?.business_license_ocr as Record<string, unknown> | undefined, 'reg_num')
      || String(res?.business_license_number || '').trim()
    )
    const idCardDone = Boolean(
      getOCRString(res?.id_card_front_ocr as Record<string, unknown> | undefined, 'name')
      && getOCRString(res?.id_card_front_ocr as Record<string, unknown> | undefined, 'id_number')
      && getOCRString(res?.id_card_back_ocr as Record<string, unknown> | undefined, 'valid_date')
    )

    return {
      businessLicense: businessLicenseDone ? 'done' : businessLicenseUploaded ? 'processing' : 'idle',
      idCard: idCardDone ? 'done' : idCardUploaded ? 'processing' : 'idle'
    }
  },

  setOCRState(type: keyof OperatorOCRDisplayState, status: OCRDisplayStateValue) {
    this.setData({ [`ocrDisplayState.${type}`]: status })
  },

  isPendingOCRMessage(message: string): boolean {
    return message.includes('处理中') || message.includes('审核中') || message.includes('识别中')
  },

  /**
   * 根据 ID 同步本地显示名称
   */
  syncRegionName(regionId: number | string) {
    const id = Number(regionId)
    const matched = this.data.regionOptions.find((r) => Number(r.value) === id)
    if (matched) {
      const fullName = buildRegionFullName(matched)
      this.setData({ 'formData.regionName': fullName })
    }
  },

  // ==================== 区域搜索逻辑 ====================

  async onOpenRegionPopup() {
    this.setData({ regionPopupVisible: true })

    if (!this.data.cityOptions.length) {
      await this.fetchCityOptions()
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
    const detail = e.detail as unknown
    const rawValue = typeof detail === 'string'
      ? detail
      : (detail && typeof detail === 'object' && 'value' in detail)
        ? String((detail as { value?: string }).value || '')
        : ''

    const normalizedValue = rawValue === 'undefined' ? '' : rawValue
    const keyword = normalizedValue.trim()
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

  async onCityChange(e: WechatMiniprogram.CustomEvent<{ value?: string | number }>) {
    const rawIndex = e.detail.value
    const index = Number(rawIndex)
    if (!Number.isFinite(index)) return

    const city = this.data.cityOptions[index]
    if (!city) return

    this.setData({
      selectedCityIndex: index,
      selectedCityId: city.value,
      selectedCityName: city.label,
      regionKeyword: '',
      regionSearchTimer: null,
      lastRegionSearchKeyword: '',
      lastRegionSearchCityId: city.value,
      regionOptions: [],
      filteredRegions: [],
      'formData.regionId': 0,
      'formData.regionName': ''
    })

    await this.fetchAvailableRegionsByCity(city.value)
  },

  onSelectRegion(e: WechatMiniprogram.CustomEvent<{ value?: number | string }>) {
    const regionId = Number(e.detail.value)
    const region = this.data.regionOptions.find((r) => Number(r.value) === regionId)
    
    if (region) {
      const parentName = region.secondary || this.data.selectedCityName
      const fullName = parentName ? `${parentName} - ${region.label}` : buildRegionFullName(region)

      const matchedIndex = this.data.cityOptions.findIndex((item) =>
        (region.parentId ? item.value === region.parentId : false) ||
        (parentName ? item.label === parentName : false)
      )

      const cityState = matchedIndex >= 0
        ? {
            selectedCityIndex: matchedIndex,
            selectedCityId: this.data.cityOptions[matchedIndex].value,
            selectedCityName: this.data.cityOptions[matchedIndex].label
          }
        : {
            selectedCityName: parentName || this.data.selectedCityName
          }

      this.setData({
        ...cityState,
        'formData.regionId': region.value,
        'formData.regionName': fullName,
        regionPopupVisible: false
      })
    }
  },

  // ==================== 输入处理 ====================

  onInput(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    const field = (e.currentTarget.dataset as { field?: keyof FormDataValue }).field
    if (!field) return
    this.setData({ [`formData.${field}`]: e.detail.value || '' })
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

  // ==================== 证照上传 (对齐集团模式) ====================

  async onIdFrontUpload(e: UploadEvent) {
    const { path } = e.detail
    if (!path) return
    this.setData({
      'idFront.url': path,
      'idFront.rawUrl': path
    })
    this.processOCR(ocrOperatorIdCard(path, 'Front'), 'idCard')
  },

  async onIdBackUpload(e: UploadEvent) {
    const { path } = e.detail
    if (!path) return
    this.setData({
      'idBack.url': path,
      'idBack.rawUrl': path
    })
    this.processOCR(ocrOperatorIdCard(path, 'Back'), 'idCard')
  },

  async onLicenseUpload(e: UploadEvent) {
    const { path } = e.detail
    if (!path) return
    this.setData({
      'license.url': path,
      'license.rawUrl': path
    })
    this.processOCR(ocrOperatorBusinessLicense(path), 'businessLicense')
  },

  async processOCR(ocrPromise: Promise<OperatorApplicationResponse>, type: keyof OperatorOCRDisplayState) {
    this.setOCRState(type, 'processing')
    wx.showLoading({ title: '智能识别中...' })
    try {
      const res = await ocrPromise
      const nextState = this.buildOperatorOcrDisplayState(res)
      this.mapResponseToData(res)
      wx.hideLoading()
      wx.showToast({
        title: nextState[type] === 'done' ? '自动识别成功' : '图片已上传，系统继续识别中',
        icon: 'none'
      })
    } catch (e) {
      wx.hideLoading()
      logger.error('OCR failed', e)
      const message = getErrorText(e, '图片已上传，系统处理中')
      this.setOCRState(type, this.isPendingOCRMessage(message) ? 'processing' : 'failed')
      wx.showToast({ title: message, icon: 'none', duration: 3000 })
    }
  },

  // ==================== 流程导航 ====================

  onPrev() {
    this.setData({ currentStep: this.data.currentStep - 1 })
  },

  async onNext() {
    const { currentStep, formData, idFront, idBack } = this.data

    // 从介绍页进入 Step 1
    if (currentStep === 0) {
      if (!this.ensureConsent()) return
      this.setData({ currentStep: 1 })
      return
    }

    // 从 Step 1 进入 Step 2：锁定区域并保存基础信息
    if (currentStep === 1) {
      const { name, contactName, contactPhone, years, regionId } = formData
      const normalizedName = (name || '').trim()
      const normalizedContactName = (contactName || '').trim()

      // 1. 本地前置校验
      if (!regionId) return wx.showToast({ title: '请选择运营区域', icon: 'none' })
      if (!normalizedContactName || normalizedContactName.length < 2) return wx.showToast({ title: '负责人姓名至少2位', icon: 'none' })
      if (!contactPhone || contactPhone.length !== 11) return wx.showToast({ title: '请输入11位手机号', icon: 'none' })
      
      wx.showLoading({ title: '锁定区域中...', mask: true })
      try {
        // 1. 创建/获取草稿
        const res = await getOrCreateOperatorApplication({ region_id: regionId })
        this.mapResponseToData(res)
        
        // 2. 更新基础信息
        const updated = await updateOperatorBasic({
          name: normalizedName,
          contact_name: normalizedContactName,
          contact_phone: contactPhone,
          requested_contract_years: years
        })
        this.mapResponseToData(updated)
        
        this.setData({ currentStep: 2 })
      } catch (e: unknown) {
        logger.error('Step 1 sync failed', e)
        // 优先显示后端返回的精准消息
        const msg = getErrorText(e, '锁定区域失败，可能已被占用')
        wx.showToast({ title: msg, icon: 'none' })
      } finally {
        wx.hideLoading()
      }
      return
    }

    // 从 Step 2 进入 Step 3：验证图片是否上传
    if (currentStep === 2) {
      if (!idFront.url || !idBack.url) {
        return wx.showToast({ title: '请上传法人身份证正反面', icon: 'none' })
      }
      this.setData({ currentStep: 3 })
      return
    }
  },

  /**
   * 提交最终申请
   */
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

  /**
   * 「下一步 / 确认提交」统一入口
   * 微信 WXML 的 bindtap 不支持三元表达式，必须绑定静态函数名
   */
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
