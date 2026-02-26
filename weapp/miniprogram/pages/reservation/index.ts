import { logger } from '../../utils/logger'
import { searchRooms, getRecommendedMerchants, searchMerchants, RoomSearchResult } from '../../api/search'
import { MerchantSummary } from '../../api/merchant'
import { globalStore } from '../../utils/global-store'
import { getPublicImageUrl } from '../../utils/image'
import { DishAdapter } from '../../adapters/dish'

const app = getApp<IAppOption>()

type RoomItemView = RoomSearchResult & {
  type: 'room'
  primary_image: string
  distance_display: string
}

type RestaurantItemView = {
  type: 'restaurant'
  id: number
  name: string
  imageUrl: string
  cuisineType: string[]
  distance: string
  address: string
  tags: string[]
}

type ReservationListItem = RoomItemView | RestaurantItemView

interface MessageError {
  message?: string
  userMessage?: string
}

const isNoServiceAreaMessage = (message: string): boolean => {
  return message.includes('当前区域暂无')
}

Page({
  data: {
    keyword: '',
    itemList: [] as ReservationListItem[],
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
      minSpend: undefined as number | undefined, // 分
      maxSpend: undefined as number | undefined, // 分
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

    // Helper for Error State
    isError: false,
    errorMessage: '',
    hasRestaurantsInArea: true,

    // Helpers
    guestOptionsShort: [
      { label: '2人', value: 2 },
      { label: '4人', value: 4 },
      { label: '6人', value: 6 },
      { label: '8人+', value: 8 }
    ],
    priceOptions: [
      { label: '不限', value: '', min: undefined, max: undefined },
      { label: '¥100以下', value: '0-100', min: undefined, max: 100 },
      { label: '¥100-300', value: '100-300', min: 100, max: 300 },
      { label: '¥300以上', value: '300-9999', min: 300, max: undefined }
    ],

    // Inline Options
    dateOptions: [] as Array<{ label: string, value: string }>,
    timeOptions: [
      { label: '11:00', value: '11:00' }, { label: '11:30', value: '11:30' },
      { label: '12:00', value: '12:00' }, { label: '12:30', value: '12:30' },
      { label: '13:00', value: '13:00' }, { label: '17:00', value: '17:00' },
      { label: '18:00', value: '18:00' }, { label: '19:00', value: '19:00' }
    ]
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

  onMerchantRegister() {
    wx.navigateTo({
      url: '/pages/register/merchant/index'
    })
  },

  onOperatorRegister() {
    wx.navigateTo({
      url: '/pages/register/operator/index'
    })
  },

  // ==================== Data Loading ====================

  async loadItems(reset = false) {
    if (this.data.loading) return
    this.setData({ loading: true, isError: false })

    if (reset) {
      this.setData({ page: 1, itemList: [], hasMore: true })
    }

    try {
      const { activeTab, page, pageSize, keyword, appliedFilters } = this.data
      const latitude = app.globalData.latitude || undefined
      const longitude = app.globalData.longitude || undefined

      let newList: ReservationListItem[] = []

      if (activeTab === 'room') {
        // 统一走 search 路由；缺省日期/时段使用默认值
        const effectiveDate = appliedFilters.date || this.data.dateOptions[0]?.value || this.formatDateLocal(new Date())
        const effectiveTime = appliedFilters.startTime || this.data.timeOptions[0]?.value || '18:00'

        const results = await searchRooms({
          reservation_date: effectiveDate,
          reservation_time: effectiveTime,
          min_capacity: appliedFilters.guestCount,
          min_minimum_spend: appliedFilters.minSpend,
          max_minimum_spend: appliedFilters.maxSpend,
          user_latitude: latitude,
          user_longitude: longitude,
          page_id: page,
          page_size: pageSize
        })
        // 在 TypeScript 中预处理距离和图片
        newList = results.map((r: RoomSearchResult) => {
          const room = r as RoomSearchResult & {
            merchantName?: string
            tableNo?: string
            merchantAddress?: string
          }

          return {
            ...r,
            merchant_name: r.merchant_name || room.merchantName || '',
            table_no: r.table_no || room.tableNo || r.name || '',
            merchant_address: r.merchant_address || room.merchantAddress || '',
            type: 'room',
            primary_image: getPublicImageUrl(r.primary_image) || '',
            distance_display: r.distance !== undefined ? DishAdapter.formatDistance(r.distance) : ''
          }
        })

        if (reset && newList.length === 0) {
          const hasRestaurantsInArea = await this.checkRestaurantAvailability(latitude, longitude)
          this.setData({ hasRestaurantsInArea })
        } else if (newList.length > 0) {
          this.setData({ hasRestaurantsInArea: true })
        }

      } else {
        // Restaurant Stream - 与外卖页 loadRestaurants 保持一致的数据格式
        let merchantResults: MerchantSummary[] = []

        if (keyword) {
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
          merchantResults = result
        }

        // 转换为与外卖页一致的 ViewModel 格式
        newList = merchantResults.map((m: MerchantSummary) => ({
          id: m.id,
          name: m.name,
          imageUrl: getPublicImageUrl(m.logo_url) || '',
          cuisineType: m.tags ? m.tags.slice(0, 2) : [],
          distance: m.distance !== undefined ? DishAdapter.formatDistance(m.distance) : '',
          address: m.address || '',
          tags: m.tags ? m.tags.slice(0, 3) : [],
          type: 'restaurant'
        }))

        if (reset) {
          this.setData({ hasRestaurantsInArea: newList.length > 0 })
        }
      }

      this.setData({
        itemList: reset ? newList : [...this.data.itemList, ...newList],
        loading: false,
        hasMore: newList.length === pageSize
      })

    } catch (error: unknown) {
      logger.error('Load items failed', error, 'Reservation')
      const userMessage = (error as MessageError).userMessage
      const message = (typeof userMessage === 'string' && userMessage)
        ? userMessage
        : ((error as MessageError).message || '加载失败，请重试')

      if (isNoServiceAreaMessage(message)) {
        this.setData({
          loading: false,
          isError: false,
          errorMessage: '',
          hasMore: false
        })
        return
      }

      this.setData({ 
        loading: false,
        isError: true,
        errorMessage: message
      })
    }
  },

  async checkRestaurantAvailability(latitude?: number, longitude?: number): Promise<boolean> {
    try {
      const merchants = await searchMerchants({
        keyword: '',
        page_id: 1,
        page_size: 1,
        user_latitude: latitude,
        user_longitude: longitude
      })
      return merchants.length > 0
    } catch (error) {
      logger.warn('检查区域餐厅供给失败，默认按有餐厅处理', error, 'Reservation.checkRestaurantAvailability')
      return true
    }
  },

  onRetry() {
    this.loadItems(true)
  },

  // ==================== Interactions ====================

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onLocationChange(_e: WechatMiniprogram.CustomEvent) {
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
      uiSelectedTimeSlot: appliedFilters.startTime || this.data.timeOptions[0]?.value || ''
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
      uiSelectedDate: this.data.dateOptions[0]?.value || this.formatDateLocal(new Date()),
      uiSelectedTimeSlot: this.data.timeOptions[0]?.value || '18:00'
    })
  },

  applyFilter() {
    const { uiGuestCount, uiPriceRange, uiSelectedDate, uiSelectedTimeSlot } = this.data

    const date = uiSelectedDate || this.data.dateOptions[0]?.value || this.formatDateLocal(new Date())
    const startTime = uiSelectedTimeSlot || this.data.timeOptions[0]?.value || '18:00'
    const [h, m] = startTime.split(':').map(Number)
    const endTime = `${h + 2}:${m}`

    const priceOption = this.data.priceOptions.find((p) => p.value === uiPriceRange)
    const minSpend = priceOption?.min !== undefined ? priceOption.min * 100 : undefined
    const maxSpend = priceOption?.max !== undefined ? priceOption.max * 100 : undefined

    this.setData({
      appliedFilters: {
        guestCount: uiGuestCount,
        priceRange: uiPriceRange,
        minSpend,
        maxSpend,
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

  formatDateLocal(date: Date): string {
    const y = date.getFullYear()
    const m = String(date.getMonth() + 1).padStart(2, '0')
    const d = String(date.getDate()).padStart(2, '0')
    return `${y}-${m}-${d}`
  },

  generateDateOptions() {
    const options = []
    const today = new Date()
    for (let i = 0; i < 7; i++) {
      const date = new Date(today)
      date.setDate(today.getDate() + i)
      const label = i === 0 ? '今天' : i === 1 ? '明天' : `${date.getMonth() + 1}月${date.getDate()}日`
      options.push({ label, value: this.formatDateLocal(date) })
    }
    this.setData({ dateOptions: options })
  }
})
