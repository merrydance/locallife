import { logger } from '../../utils/logger'
import { searchRooms, getRecommendedRooms, getRecommendedMerchants, RoomSearchResult } from '../../api/search'
import { MerchantSummary } from '../../api/merchant'
import { globalStore } from '../../utils/global-store'

const app = getApp<IAppOption>()

Page({
  data: {
    keyword: '',
    itemList: [] as (RoomSearchResult | MerchantSummary)[],
    activeTab: 'room' as 'room' | 'restaurant',

    // UI State
    navBarHeight: 88,
    address: '定位中...',
    loading: false,
    hasMore: true,
    page: 1,
    pageSize: 10,

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
    uiGuestCount: 2,
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
    this.setData({ navBarHeight: globalStore.get('navBarHeight') })
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

      // Parse price range from APPLIED filters
      let min_price, max_price
      if (appliedFilters.priceRange) {
        const parts = appliedFilters.priceRange.split('-')
        min_price = Number(parts[0])
        max_price = Number(parts[1])
      }

      if (activeTab === 'room') {
        const hasTimeFilter = appliedFilters.date && appliedFilters.startTime

        if (keyword || hasTimeFilter) {
          // Search Mode
          const results = await searchRooms({
            page_id: page,
            page_size: pageSize,
            keyword,
            user_latitude: latitude,
            user_longitude: longitude,
            date: appliedFilters.date || new Date().toISOString().split('T')[0],
            start_time: appliedFilters.startTime || '18:00',
            end_time: appliedFilters.endTime || '20:00',
            guest_count: appliedFilters.guestCount || 2,
            min_price,
            max_price
          })
          newList = results.map((r: RoomSearchResult) => ({ ...r, type: 'room' }))
        } else {
          // Feed Mode
          const results = await getRecommendedRooms({
            page_id: page,
            limit: pageSize,
            user_latitude: latitude,
            user_longitude: longitude,
            guest_count: appliedFilters.guestCount,
            min_price,
            max_price
          })
          newList = results.map((r: RoomSearchResult) => ({ ...r, type: 'room' }))
        }

      } else {
        // Restaurant Stream
        if (keyword) {
          const { searchMerchants } = require('../../api/search')
          const results = await searchMerchants({
            keyword,
            page_id: page,
            page_size: pageSize,
            user_latitude: latitude,
            user_longitude: longitude
          })
          newList = results.map((m: MerchantSummary) => ({ ...m, type: 'restaurant' }))
        } else {
          const results = await getRecommendedMerchants({
            user_latitude: latitude,
            user_longitude: longitude,
            limit: pageSize
          })
          newList = results.map((m: MerchantSummary) => ({ ...m, type: 'restaurant' }))
        }
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
    this.setData({ keyword: e.detail.value })
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
    this.setData({
      uiGuestCount: 2,
      uiPriceRange: '',
      uiSelectedDate: this.data.dateOptions[0].value,
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
