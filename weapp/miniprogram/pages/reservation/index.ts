import { logger } from '../../utils/logger'
import { searchRooms, getRecommendedRooms, getRecommendedMerchants, RoomSearchResult } from '../../api/search'
import { MerchantSummary } from '../../api/merchant'
import { globalStore } from '../../utils/global-store'
import { getPublicImageUrl } from '../../utils/image'
import { DishAdapter } from '../../adapters/dish'

const app = getApp<IAppOption>()

Page({
  data: {
    keyword: '',
    itemList: [] as (RoomSearchResult | MerchantSummary)[],
    activeTab: 'room' as 'room' | 'restaurant',

    // UI State
    navBarHeight: 88,
    scrollViewHeight: 600,
    address: '定位中...',
    loading: false,
    hasMore: true,
    page: 1,
    pageSize: 10,
    refresherTriggered: false,

    // Applied Filters (The actual filters used for API calls)
    appliedFilters: {
      guestCount: undefined as number | undefined,
      priceRange: '' as string,
      selectedTime: '' as string,
      date: '' as string,
      startTime: '' as string,
      endTime: '' as string
    },

    // Filter Popup UI State (Temporary)
    filterVisible: false,
    uiSelectedDate: '',
    uiSelectedTimeSlot: '',
    uiGuestCount: 2,  // 默认2人
    uiPriceRange: '',

    // Helpers
    guestOptionsShort: [
      { label: '2人', value: 2 },
      { label: '4人', value: 4 },
      { label: '6人', value: 6 },
      { label: '8人+', value: 8 }
    ],
    priceOptions: [
      { label: '不限', value: '' },
      { label: '¥100以下', value: '0-100' },
      { label: '¥100-300', value: '100-300' },
      { label: '¥300以上', value: '300-9999' }
    ],

    // Inline Options
    dateOptions: [] as Array<{ label: string, value: string }>,
    timeOptions: [
      { label: '11:00', value: '11:00' }, { label: '11:30', value: '11:30' },
      { label: '12:00', value: '12:00' }, { label: '12:30', value: '12:30' },
      { label: '13:00', value: '13:00' }, { label: '17:00', value: '17:00' },
      { label: '18:00', value: '18:00' }, { label: '19:00', value: '19:00' }
    ],
  },

  onLoad() {
    // 计算导航栏高度和滚动区域高度
    const navBarHeight = globalStore.get('navBarHeight') || 88
    const windowInfo = wx.getWindowInfo()
    // windowHeight 已扣除原生 tabBar，只需扣除自定义导航栏
    const scrollViewHeight = windowInfo.windowHeight - navBarHeight

    this.setData({ navBarHeight, scrollViewHeight })
    this.generateDateOptions()

    const loc = app.globalData.location
    if (loc && loc.name) {
      this.setData({ address: loc.name })
    } else {
      app.getLocation() // Async
    }

    // Default load (No keyword, no applied filters initially)
    this.loadItems(true)
  },

  onShow() {
    const loc = app.globalData.location
    if (loc && loc.name && loc.name !== this.data.address) {
      this.setData({ address: loc.name })
    }
  },

  // ==================== Data Loading ====================

  async loadItems(reset = false) {
    if (this.data.loading) return
    this.setData({ loading: true })

    if (reset) {
      this.setData({ page: 1, itemList: [], hasMore: true })
    }

    try {
      const { activeTab, page, pageSize, keyword, appliedFilters } = this.data
      const latitude = app.globalData.latitude || undefined
      const longitude = app.globalData.longitude || undefined

      let newList: any[] = []

      if (activeTab === 'room') {
        const hasTimeFilter = appliedFilters.date && appliedFilters.startTime

        // 将 priceRange 转换为 max_minimum_spend（分）
        let max_minimum_spend: number | undefined
        if (appliedFilters.priceRange) {
          const parts = appliedFilters.priceRange.split('-')
          // 使用上限作为 max_minimum_spend，后端期望分值
          if (parts[1]) {
            max_minimum_spend = Number(parts[1]) * 100
          }
        }

        if (hasTimeFilter) {
          // Search Mode - 需要日期时间过滤
          const results = await searchRooms({
            reservation_date: appliedFilters.date || new Date().toISOString().split('T')[0],
            reservation_time: appliedFilters.startTime || '18:00',
            min_capacity: appliedFilters.guestCount,
            max_minimum_spend,
            user_latitude: latitude,
            user_longitude: longitude,
            page_id: page,
            page_size: pageSize
          })
          // 在 TypeScript 中预处理距离和图片
          newList = results.map((r: RoomSearchResult) => ({
            ...r,
            type: 'room',
            primary_image: getPublicImageUrl(r.primary_image) || '',
            distance_display: r.distance !== undefined ? DishAdapter.formatDistance(r.distance) : ''
          }))
        } else {
          // Feed Mode - 使用推荐接口（支持人数和低消过滤）
          const results = await getRecommendedRooms({
            page_id: page,
            page_size: pageSize,
            user_latitude: latitude,
            user_longitude: longitude,
            min_capacity: appliedFilters.guestCount,
            max_minimum_spend
          })

          newList = results.map((r: RoomSearchResult) => ({
            ...r,
            type: 'room',
            primary_image: getPublicImageUrl(r.primary_image) || '',
            distance_display: r.distance !== undefined ? DishAdapter.formatDistance(r.distance) : ''
          }))
        }

      } else {
        // Restaurant Stream - 与外卖页 loadRestaurants 保持一致的数据格式
        let merchantResults: MerchantSummary[] = []

        if (keyword) {
          const { searchMerchants } = require('../../api/search')
          merchantResults = await searchMerchants({
            keyword,
            page_id: page,
            page_size: pageSize,
            user_latitude: latitude,
            user_longitude: longitude
          })
        } else {
          const result = await getRecommendedMerchants({
            user_latitude: latitude,
            user_longitude: longitude,
            limit: pageSize
          })
          // API 返回 { merchants: [...] } 或直接返回数组
          merchantResults = (result as any).merchants || result as MerchantSummary[]
        }

        // 转换为与外卖页一致的 ViewModel 格式
        newList = merchantResults.map((m: MerchantSummary) => ({
          id: m.id,
          name: m.name,
          imageUrl: getPublicImageUrl(m.logo_url) || '',
          cuisineType: m.tags ? m.tags.slice(0, 2) : [],
          distance: m.distance !== undefined ? DishAdapter.formatDistance(m.distance) : '',
          address: m.address,
          tags: m.tags ? m.tags.slice(0, 3) : [],
          type: 'restaurant'
        }))
      }

      this.setData({
        itemList: reset ? newList : [...this.data.itemList, ...newList],
        loading: false,
        hasMore: newList.length === pageSize
      })

    } catch (error) {
      logger.error('Load items failed', error, 'Reservation')
      wx.showToast({ title: '加载失败', icon: 'none' })
      this.setData({ loading: false })
    }
  },

  // ==================== Interactions ====================

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onLocationChange(e: WechatMiniprogram.CustomEvent) {
    this.loadItems(true)
  },

  onTabChange(e: WechatMiniprogram.CustomEvent) {
    const { value } = e.detail
    if (value === this.data.activeTab) return
    this.setData({ activeTab: value })
    this.loadItems(true)
  },

  onSearch(e: WechatMiniprogram.CustomEvent) {
    const keyword = e.detail.value?.trim() || ''

    // 如果有搜索词且在包间 tab，切换到餐厅 tab 搜索
    // 因为后端 searchRooms API 不支持关键词搜索
    if (keyword && this.data.activeTab === 'room') {
      this.setData({
        keyword,
        activeTab: 'restaurant'
      })
    } else {
      this.setData({ keyword })
    }

    this.loadItems(true)
  },

  onItemTap(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.navigateTo({ url: `/pages/merchant/detail/index?id=${id}` })
  },

  onReachBottom() {
    if (this.data.hasMore) {
      this.setData({ page: this.data.page + 1 })
      this.loadItems(false)
    }
  },

  /**
   * scroll-view 下拉刷新事件处理
   * 在 Skyline 模式下实现下拉刷新
   */
  async onRefresh() {
    this.setData({ refresherTriggered: true, page: 1 })

    try {
      await this.loadItems(true)
    } finally {
      setTimeout(() => {
        this.setData({ refresherTriggered: false })
      }, 300)
    }
  },

  // ==================== Filter Popup ====================

  showFilterPopup() {
    const { appliedFilters } = this.data
    this.setData({
      filterVisible: true,
      uiGuestCount: appliedFilters.guestCount || 2,
      uiPriceRange: appliedFilters.priceRange,
      uiSelectedDate: appliedFilters.date || this.data.dateOptions[0].value,
      uiSelectedTimeSlot: appliedFilters.startTime || ''
    })
  },

  hideFilterPopup() {
    this.setData({ filterVisible: false })
  },

  onFilterPopupChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ filterVisible: e.detail.visible })
  },

  resetFilter() {
    // 重置为默认值
    this.setData({
      uiGuestCount: 2,
      uiPriceRange: '',
      uiSelectedDate: this.data.dateOptions[0]?.value || '',
      uiSelectedTimeSlot: ''
    })
  },

  applyFilter() {
    const { uiGuestCount, uiPriceRange, uiSelectedDate, uiSelectedTimeSlot } = this.data

    let date = '', startTime = '', endTime = ''
    if (uiSelectedDate && uiSelectedTimeSlot) {
      date = uiSelectedDate
      startTime = uiSelectedTimeSlot
      const [h, m] = startTime.split(':').map(Number)
      endTime = `${h + 2}:${m}`
    }

    this.setData({
      appliedFilters: {
        guestCount: uiGuestCount,
        priceRange: uiPriceRange,
        selectedTime: (date && startTime) ? `${date} ${startTime}` : '',
        date,
        startTime,
        endTime
      }
    }, () => {
      this.hideFilterPopup()
      this.loadItems(true)
    })
  },

  // ==================== Inline Tags Selection ====================

  onDateTagChange(e: WechatMiniprogram.CustomEvent) {
    const { value } = e.currentTarget.dataset
    this.setData({ uiSelectedDate: value })
  },

  onTimeTagChange(e: WechatMiniprogram.CustomEvent) {
    const { value } = e.currentTarget.dataset
    this.setData({ uiSelectedTimeSlot: value === this.data.uiSelectedTimeSlot ? '' : value })
  },

  onGuestTagChange(e: WechatMiniprogram.CustomEvent) {
    const { value } = e.currentTarget.dataset
    this.setData({ uiGuestCount: value })
  },

  onPriceTagChange(e: WechatMiniprogram.CustomEvent) {
    const { value } = e.currentTarget.dataset
    this.setData({ uiPriceRange: value === this.data.uiPriceRange ? '' : value })
  },

  // ==================== Utils ====================

  generateDateOptions() {
    const options = []
    const today = new Date()
    for (let i = 0; i < 7; i++) {
      const date = new Date(today)
      date.setDate(today.getDate() + i)
      const label = i === 0 ? '今天' : i === 1 ? '明天' : `${date.getMonth() + 1}月${date.getDate()}日`
      options.push({ label, value: date.toISOString().split('T')[0] })
    }
    this.setData({ dateOptions: options })
  }
})
