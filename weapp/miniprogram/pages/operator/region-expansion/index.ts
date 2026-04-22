import {
  loadOperatorRegionExpansionApplications,
  loadOperatorRegionExpansionCityState,
  loadOperatorRegionExpansionRegionsByCity,
  submitOperatorRegionExpansion,
  type OperatorRegionExpansionApplicationView,
  type OperatorRegionExpansionCityOption,
  type OperatorRegionExpansionRegionOption
} from '../../../services/operator-region-expansion'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'

Page({
  data: {
    navBarHeight: 88,

    // 申请列表
    applications: [] as OperatorRegionExpansionApplicationView[],
    listLoading: true,
    listError: '',

    // 选区状态
  cityOptions:         [] as OperatorRegionExpansionCityOption[],
    cityPickerVisible:   false,
  regionOptions:       [] as OperatorRegionExpansionRegionOption[],
  filteredRegions:     [] as OperatorRegionExpansionRegionOption[],
    selectedCityIndex:   0,
    selectedCityId:      0,
    selectedCityName:    '',
    selectedRegionId:    0,
    selectedRegionName:  '',
    regionKeyword:       '',

    showForm:     false,
    submitting:   false
  },

  onLoad() {
    this.loadApplications()
    this.fetchCityOptions()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  // ─── 申请列表 ────────────────────────────────────────────

  async loadApplications() {
    this.setData({ listLoading: true, listError: '' })
    try {
      this.setData({ applications: await loadOperatorRegionExpansionApplications() })
    } catch (e: unknown) {
      const msg = getErrorUserMessage(e, '加载申请记录失败，请稍后重试')
      this.setData({ listError: msg })
      logger.error('Load region expansion applications failed', e)
    } finally {
      this.setData({ listLoading: false })
    }
  },

  onRetry() {
    this.loadApplications()
  },

  // ─── 城市/区域选择 ────────────────────────────────────────

  async fetchCityOptions() {
    try {
      const cityState = await loadOperatorRegionExpansionCityState()
      const selectedCityId = cityState.selectedCityId
      this.setData({
        cityOptions: cityState.cityOptions,
        cityPickerVisible: false,
        selectedCityId,
        selectedCityName: cityState.selectedCityName
      })
      if (selectedCityId) await this.fetchRegionsByCity(selectedCityId)
    } catch (e: unknown) {
      logger.error('Fetch cities failed', e)
    }
  },

  async fetchRegionsByCity(cityID: number, keyword = '') {
    try {
      const regionState = await loadOperatorRegionExpansionRegionsByCity({
        cityId: cityID,
        cityName: this.data.selectedCityName,
        keyword
      })
      this.setData({
        regionOptions:   regionState.regionOptions,
        filteredRegions: regionState.filteredRegions,
        selectedRegionId:   0,
        selectedRegionName: '',
        regionKeyword:   regionState.regionKeyword
      })
    } catch (e: unknown) {
      logger.error('Fetch districts failed', e)
    }
  },

  onOpenCityPicker() {
    if (!this.data.cityOptions.length) return
    this.setData({ cityPickerVisible: true })
  },

  onCloseCityPicker() {
    this.setData({ cityPickerVisible: false })
  },

  onCityConfirm(e: WechatMiniprogram.CustomEvent<{ value: Array<string | number> | null }>) {
    const values = Array.isArray(e.detail?.value) ? e.detail.value : []
    const selectedValue = Number(values[0] || 0)
    const idx = this.data.cityOptions.findIndex((item) => item.value === selectedValue)
    const city = idx >= 0 ? this.data.cityOptions[idx] : null

    if (!city) {
      this.setData({ cityPickerVisible: false })
      return
    }

    this.setData({
      cityPickerVisible: false,
      selectedCityIndex: idx,
      selectedCityId: city.value,
      selectedCityName: city.label,
      selectedRegionId: 0,
      selectedRegionName: ''
    })
    this.fetchRegionsByCity(city.value)
  },

  onCityChange(e: WechatMiniprogram.PickerChange) {
    const idx = Number(e.detail.value)
    const city = this.data.cityOptions[idx]
    if (!city) return
    this.setData({
      selectedCityIndex: idx,
      selectedCityId:    city.value,
      selectedCityName:  city.label,
      selectedRegionId:    0,
      selectedRegionName:  ''
    })
    this.fetchRegionsByCity(city.value)
  },

  onRegionSearch(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const keyword = e.detail.value || ''
    this.setData({ regionKeyword: keyword })
    this.fetchRegionsByCity(this.data.selectedCityId, keyword)
  },

  onSelectRegion(e: WechatMiniprogram.TouchEvent) {
    const { id, name } = e.currentTarget.dataset as { id: number, name: string }
    this.setData({ selectedRegionId: id, selectedRegionName: `${this.data.selectedCityName} - ${name}` })
  },

  // ─── 申请提交 ────────────────────────────────────────────

  onShowForm() {
    this.setData({ showForm: true })
  },

  onHideForm() {
    this.setData({ showForm: false })
  },

  async onSubmit() {
    const { selectedRegionId } = this.data
    if (!selectedRegionId) {
      wx.showToast({ title: '请先选择目标区域', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    try {
      await submitOperatorRegionExpansion(selectedRegionId)
      this.setData({ showForm: false, selectedRegionId: 0, selectedRegionName: '' })
      await this.loadApplications()
    } catch (e: unknown) {
      const msg = getErrorUserMessage(e, '提交失败，请稍后重试')
      wx.showToast({ title: msg, icon: 'none' })
      logger.error('Submit region expansion failed', e)
    } finally {
      this.setData({ submitting: false })
    }
  }
})
