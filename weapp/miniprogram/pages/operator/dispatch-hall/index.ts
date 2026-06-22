import {
  loadOperatorRegions,
  type ConsoleRegionOption
} from '../_services/operator-regions'
import {
  loadOperatorDispatchMonitorPageData,
  type OperatorDispatchMonitorSummaryView,
  type OperatorPendingDispatchView
} from '../_services/operator-dispatch-monitor'
import { getErrorUserMessage } from '../../../utils/user-facing'

type DispatchHallOptions = {
  region_id?: string
}

type DispatchSummaryState = OperatorDispatchMonitorSummaryView & {
  latestRefreshText: string
}

function createEmptySummary(regionName = '当前区域'): DispatchSummaryState {
  return {
    regionId: 0,
    regionName,
    pendingTotal: 0,
    timeoutOver3mTotal: 0,
    oldestWaitSeconds: 0,
    oldestWaitText: '0 秒',
    latestRefreshText: '刚刚'
  }
}

let dispatchHallRequestSeq = 0

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    loadingMore: false,
    refreshing: false,
    initialLoading: true,
    error: '',
    emptyRegion: false,
    page: 1,
    limit: 20,
    total: 0,
    hasMore: false,
    preferredRegionId: 0,
    regions: [] as ConsoleRegionOption[],
    regionPickerOptions: [] as Array<{ label: string, value: string }>,
    regionPickerVisible: false,
    selectedRegionIdx: 0,
    selectedRegionId: 0,
    selectedRegionValue: '',
    summary: createEmptySummary(),
    dispatches: [] as OperatorPendingDispatchView[]
  },

  onLoad(options: DispatchHallOptions) {
    const preferredRegionId = Number(options.region_id || 0)
    this.setData({ preferredRegionId })
    this.initPage(preferredRegionId)
  },

  onShow() {
    if (!this.data.initialLoading && this.data.selectedRegionId) {
      this.loadDispatches(true, true)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  async initPage(preferredRegionId = 0) {
    this.setData({ initialLoading: true, error: '', emptyRegion: false })
    try {
      const regionState = await loadOperatorRegions()
      const nextState = this.resolveRegionState(regionState, preferredRegionId)
      this.setData(nextState)

      if (!nextState.selectedRegionId) {
        this.setData({
          initialLoading: false,
          emptyRegion: true,
          summary: createEmptySummary()
        })
        return
      }

      await this.loadDispatches(true)
    } catch (error: unknown) {
      this.setData({
        initialLoading: false,
        loading: false,
        loadingMore: false,
        emptyRegion: false,
        error: getErrorUserMessage(error, '加载可管理区域失败，请稍后重试'),
        summary: createEmptySummary(),
        dispatches: []
      })
    }
  },

  resolveRegionState(
    regionState: {
      regions: ConsoleRegionOption[]
      regionPickerOptions: Array<{ label: string, value: string }>
      regionPickerVisible: boolean
      selectedRegionIdx: number
      selectedRegionId: number
      selectedRegionValue: string
    },
    preferredRegionId: number
  ) {
    if (!preferredRegionId) {
      return regionState
    }

    const index = regionState.regions.findIndex((item) => item.id === preferredRegionId)
    if (index < 0) {
      return regionState
    }

    return {
      ...regionState,
      selectedRegionIdx: index,
      selectedRegionId: preferredRegionId,
      selectedRegionValue: String(preferredRegionId)
    }
  },

  async loadDispatches(refresh: boolean, silent = false) {
    if (!this.data.selectedRegionId) {
      return
    }
    if (!refresh && (this.data.loading || this.data.loadingMore)) {
      return
    }
    const requestSeq = ++dispatchHallRequestSeq

    try {
      if (refresh) {
        this.setData({
          loading: true,
          loadingMore: false,
          error: '',
          page: 1,
          ...(silent ? {} : { initialLoading: this.data.initialLoading })
        })
      } else {
        this.setData({ loadingMore: true })
      }

      const currentPage = refresh ? 1 : this.data.page
      const result = await loadOperatorDispatchMonitorPageData({
        regionId: this.data.selectedRegionId,
        pageId: currentPage,
        pageSize: this.data.limit
      })
      if (requestSeq !== dispatchHallRequestSeq) {
        return
      }

      const dispatches = refresh ? result.items : [...this.data.dispatches, ...result.items]
      this.setData({
        summary: result.summary,
        dispatches,
        total: result.total,
        hasMore: result.hasMore,
        page: result.page + 1,
        loading: false,
        loadingMore: false,
        initialLoading: false,
        emptyRegion: false
      })
    } catch (error: unknown) {
      if (requestSeq !== dispatchHallRequestSeq) {
        return
      }
      this.setData({
        loading: false,
        loadingMore: false,
        initialLoading: false,
        error: getErrorUserMessage(error, '加载待接单大厅失败，请稍后重试')
      })
    }
  },

  onRetry() {
    if (!this.data.selectedRegionId) {
      this.initPage(this.data.preferredRegionId)
      return
    }

    this.loadDispatches(true)
  },

  onPullDownRefresh() {
    this.setData({ refreshing: true })
    this.loadDispatches(true).finally(() => {
      this.setData({ refreshing: false })
      wx.stopPullDownRefresh()
    })
  },

  onLoadMore() {
    if (!this.data.hasMore || this.data.loading || this.data.loadingMore) {
      return
    }

    this.loadDispatches(false)
  },

  onOpenRegionPicker() {
    if (this.data.regions.length <= 1) {
      return
    }

    this.setData({ regionPickerVisible: true })
  },

  onCloseRegionPicker() {
    this.setData({ regionPickerVisible: false })
  },

  onRegionConfirm(e: WechatMiniprogram.CustomEvent<{ value: Array<string | number> | null }>) {
    const values = Array.isArray(e.detail?.value) ? e.detail.value : []
    const selectedValue = String(values[0] || '')
    const idx = this.data.regionPickerOptions.findIndex((item) => item.value === selectedValue)
    const region = idx >= 0 ? this.data.regions[idx] : null

    this.setData({
      regionPickerVisible: false,
      selectedRegionIdx: idx >= 0 ? idx : this.data.selectedRegionIdx,
      selectedRegionId: region?.id || this.data.selectedRegionId,
      selectedRegionValue: selectedValue || this.data.selectedRegionValue,
      summary: createEmptySummary(region?.name || this.data.summary.regionName),
      dispatches: [],
      total: 0,
      hasMore: false
    }, () => {
      if (region?.id) {
        this.loadDispatches(true)
      }
    })
  },

  onOpenNotifications() {
    wx.navigateTo({ url: '/pages/operator/notifications/index' })
  }
})
