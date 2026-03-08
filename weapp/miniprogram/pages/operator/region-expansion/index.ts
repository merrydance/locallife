import {
  applyRegionExpansion,
  listRegionExpansionApplications,
  listRegions,
  type RegionExpansionApplication
} from '../../../api/operator-application'
import { logger } from '../../../utils/logger'

type CityOption = { label: string, value: number }
type RegionOption = { label: string, secondary: string, value: number }

const STATUS_MAP: Record<string, { label: string, theme: 'primary' | 'warning' | 'danger' | 'default' }> = {
  pending:  { label: '审核中', theme: 'warning' },
  approved: { label: '已通过', theme: 'primary' },
  rejected: { label: '已拒绝', theme: 'danger' }
}

Page({
  data: {
    navBarHeight: 88,

    // 申请列表
    applications: [] as RegionExpansionApplication[],
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
      this.setData({ applications: res.applications || [] })
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : '加载失败'
      this.setData({ listError: msg })
      logger.error('Load region expansion applications failed', e)
    } finally {
      this.setData({ listLoading: false })
    }
  },

  onRetry() {
    this.loadApplications()
  },

  getStatusMeta(status: string) {
    return STATUS_MAP[status] ?? { label: status, theme: 'default' }
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
        const items = await listRegions({
          page_id: pageID,
          page_size: 100,
          level: 3,
          parent_id: cityID
        })
        if (!items || items.length === 0) break
        items.forEach((r) => districts.push({ label: r.name, secondary: this.data.selectedCityName, value: r.id }))
        if (items.length < 100) break
        pageID++
      }
      const filtered = keyword
        ? districts.filter((r) => r.label.includes(keyword))
        : districts
      this.setData({
        regionOptions:   districts,
        filteredRegions: filtered,
        selectedRegionId:   0,
        selectedRegionName: '',
        regionKeyword:   ''
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
    const filtered = this.data.regionOptions.filter((r) =>
      r.label.includes(keyword) || r.secondary.includes(keyword)
    )
    this.setData({ filteredRegions: filtered, regionKeyword: keyword })
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
      wx.showToast({ title: '申请已提交，等待审核', icon: 'success' })
      this.setData({ showForm: false, selectedRegionId: 0, selectedRegionName: '' })
      await this.loadApplications()
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : '提交失败'
      wx.showToast({ title: msg, icon: 'none' })
      logger.error('Submit region expansion failed', e)
    } finally {
      this.setData({ submitting: false })
    }
  }
})
