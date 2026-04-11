import {
  applyRegionExpansion,
  getRegionExpansionStatusDisplay,
  listAvailableRegions,
  listRegionExpansionApplications,
  listRegions,
  type RegionExpansionApplication
} from '../../../api/operator-application'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'

type CityOption = { label: string, value: number }
type RegionOption = { label: string, secondary: string, value: number }
type RegionExpansionApplicationView = RegionExpansionApplication & {
  status_label: string
  status_theme: 'warning' | 'primary' | 'danger'
  is_rejected: boolean
}

function formatDate(iso: string): string {
  try {
    const d = new Date(iso)
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
  } catch {
    return iso
  }
}

Page({
  data: {
    navBarHeight: 88,

    // 申请列表
    applications: [] as RegionExpansionApplicationView[],
    listLoading: true,
    listError: '',

    // 选区状态
    cityOptions:         [] as CityOption[],
    regionOptions:       [] as RegionOption[],
    filteredRegions:     [] as RegionOption[],
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
      const res = await listRegionExpansionApplications()
      const apps = (res.applications || []).map((a) => {
        const statusDisplay = getRegionExpansionStatusDisplay(a.status)
        return {
          ...a,
          created_at: formatDate(a.created_at),
          status_label: statusDisplay.label,
          status_theme: statusDisplay.theme,
          is_rejected: statusDisplay.isRejected
        }
      })
      this.setData({ applications: apps })
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
      const cities: CityOption[] = []
      let pageID = 1
      for (;;) {
        const items = await listRegions({ page_id: pageID, page_size: 100, level: 2 })
        if (!items || items.length === 0) break
        items.forEach((item) => cities.push({ label: item.name, value: item.id }))
        if (items.length < 100) break
        pageID++
      }
      const selectedCityId = cities[0]?.value || 0
      this.setData({ cityOptions: cities, selectedCityId, selectedCityName: cities[0]?.label || '' })
      if (selectedCityId) await this.fetchRegionsByCity(selectedCityId)
    } catch (e: unknown) {
      logger.error('Fetch cities failed', e)
    }
  },

  async fetchRegionsByCity(cityID: number, keyword = '') {
    try {
      const districts: RegionOption[] = []
      let pageID = 1
      for (;;) {
        const response = await listAvailableRegions({
          page_id: pageID,
          page_size: 100,
          level: 3,
          parent_id: cityID,
          keyword: keyword || undefined
        })
        const items = response.regions || []
        if (!items || items.length === 0) break
        items.forEach((r) => districts.push({
          label: r.name,
          secondary: r.parent_name || this.data.selectedCityName,
          value: r.id
        }))
        if (items.length < 100) break
        pageID++
      }
      this.setData({
        regionOptions:   districts,
        filteredRegions: districts,
        selectedRegionId:   0,
        selectedRegionName: '',
        regionKeyword:   keyword
      })
    } catch (e: unknown) {
      logger.error('Fetch districts failed', e)
    }
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
      await applyRegionExpansion(selectedRegionId)
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
