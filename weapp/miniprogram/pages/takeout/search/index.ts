import {
  getSearchHistory,
  getPopularKeywords,
  getSearchSuggestions,
  unifiedSearch,
  deleteSearchHistory,
  clearSearchHistory,
  SearchHistory,
  PopularKeyword,
  SearchSuggestion
} from '../../../api/search'
import { getPublicImageUrl } from '../../../utils/image'
import { formatPriceNoSymbol } from '../../../utils/util'

const DEBOUNCE_MS = 300

interface DishResult {
  id: number
  name: string
  merchant_id: number
  merchant_name: string
  imageUrl: string
  priceDisplay: string
  merchant_is_open: boolean
}

interface MerchantResult {
  id: number
  name: string
  logo_url: string
  address: string
  is_open: boolean
  tags: string[]
}

let debounceTimer: ReturnType<typeof setTimeout> | null = null

Page({
  data: {
    navBarHeight: 88,
    keyword: '',
    // 视图状态
    showInitial: true,
    showSuggestions: false,
    showResults: false,
    // 初始态数据
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    history: [] as SearchHistory[],
    hotWords: [] as PopularKeyword[],
    // 建议态数据
    suggestions: [] as SearchSuggestion[],
    // 结果态数据
    searching: false,
    resultsError: false,
    resultsErrorMessage: '',
    lastSearchKeyword: '',
    activeResultTab: 'dishes',
    resultDishes: [] as DishResult[],
    resultMerchants: [] as MerchantResult[],
    resultDishCount: 0,
    resultMerchantCount: 0
  },

  onLoad() {
    this.loadInitialData()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadInitialData() {
    this.setData({
      initialLoading: true,
      initialError: false,
      initialErrorMessage: ''
    })

    try {
      const [historyResult, hotWordsResult] = await Promise.allSettled([
        getSearchHistory(10),
        getPopularKeywords('dish')
      ])

      const history = historyResult.status === 'fulfilled' ? historyResult.value : []
      const hotWords = hotWordsResult.status === 'fulfilled' ? hotWordsResult.value : []
      const initialError = historyResult.status === 'rejected' && hotWordsResult.status === 'rejected'

      this.setData({
        history,
        hotWords,
        initialLoading: false,
        initialError,
        initialErrorMessage: initialError ? '搜索记录加载失败，请重试' : ''
      })

      if (initialError) {
        console.warn('加载搜索初始数据失败', {
          historyError: historyResult.status === 'rejected' ? historyResult.reason : undefined,
          hotWordsError: hotWordsResult.status === 'rejected' ? hotWordsResult.reason : undefined
        })
      }
    } catch (err) {
      console.warn('加载搜索初始数据失败', err)
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: '搜索记录加载失败，请重试'
      })
    }
  },

  // 关键词变化：防抖请求实时建议
  onKeywordChange(e: WechatMiniprogram.CustomEvent) {
    const keyword: string = e.detail.value ?? ''
    this.setData({ keyword })

    if (!keyword.trim()) {
      // 清空时回到初始态
      this.setData({
        showInitial: true,
        showSuggestions: false,
        showResults: false,
        resultsError: false,
        resultsErrorMessage: '',
        suggestions: []
      })
      if (debounceTimer) clearTimeout(debounceTimer)
      return
    }

    // 有输入 → 切到建议态
    this.setData({ showInitial: false, showSuggestions: true, showResults: false })

    if (debounceTimer) clearTimeout(debounceTimer)
    debounceTimer = setTimeout(() => this.fetchSuggestions(keyword), DEBOUNCE_MS)
  },

  async fetchSuggestions(keyword: string) {
    try {
      const suggestions = await getSearchSuggestions(keyword, 'dish')
      if (this.data.keyword === keyword) {
        this.setData({ suggestions })
      }
    } catch (err) {
      console.warn('获取搜索建议失败', err)
      if (this.data.keyword === keyword) {
        this.setData({ suggestions: [] })
      }
    }
  },

  // 执行搜索
  onSearch() {
    const keyword = this.data.keyword.trim()
    if (!keyword) return
    this.doSearch(keyword)
  },

  async doSearch(keyword: string) {
    this.setData({
      keyword,
      showInitial: false,
      showSuggestions: false,
      showResults: true,
      searching: true,
      resultsError: false,
      resultsErrorMessage: '',
      lastSearchKeyword: keyword,
      resultDishes: [],
      resultMerchants: [],
      resultDishCount: 0,
      resultMerchantCount: 0
    })

    try {
      const app = getApp<IAppOption>()
      const result = await unifiedSearch(keyword, {
        user_latitude: app.globalData.latitude ?? undefined,
        user_longitude: app.globalData.longitude ?? undefined,
        dish_limit: 20,
        merchant_limit: 20
      })

      const dishes: DishResult[] = (result.dishes || []).map((d) => ({
        id: d.id,
        name: d.name,
        merchant_id: d.merchant_id,
        merchant_name: d.merchant_name || '',
        imageUrl: getPublicImageUrl(d.image_url || ''),
        priceDisplay: formatPriceNoSymbol(d.price || 0),
        merchant_is_open: d.merchant_is_open ?? true
      }))

      const merchants: MerchantResult[] = (result.merchants || []).map((m) => ({
        id: m.id,
        name: m.name,
        logo_url: m.logo_url || '',
        address: m.address || '',
        is_open: m.is_open ?? true,
        tags: m.tags ? m.tags.slice(0, 3) : []
      }))

      this.setData({
        resultDishes: dishes,
        resultMerchants: merchants,
        resultDishCount: result.total_dishes,
        resultMerchantCount: result.total_merchants,
        searching: false
      })
    } catch (err) {
      console.error('搜索失败', err)
      const userMessage = typeof err === 'object' && err !== null && 'userMessage' in err
        ? (err as { userMessage?: string }).userMessage
        : ''

      this.setData({
        searching: false,
        resultsError: true,
        resultsErrorMessage: userMessage || '搜索失败，请稍后重试'
      })
    }
  },

  onClear() {
    this.setData({
      keyword: '',
      showInitial: true,
      showSuggestions: false,
      showResults: false,
      resultsError: false,
      resultsErrorMessage: '',
      suggestions: []
    })
  },

  onCancel() {
    wx.navigateBack()
  },

  // 历史/建议/热搜点击
  onHistoryTap(e: WechatMiniprogram.CustomEvent) {
    const { keyword } = e.currentTarget.dataset as { keyword: string }
    this.setData({ keyword })
    this.doSearch(keyword)
  },

  onHotTap(e: WechatMiniprogram.CustomEvent) {
    const { keyword } = e.currentTarget.dataset as { keyword: string }
    this.setData({ keyword })
    this.doSearch(keyword)
  },

  onSuggestionTap(e: WechatMiniprogram.CustomEvent) {
    const { keyword } = e.currentTarget.dataset as { keyword: string }
    this.setData({ keyword })
    this.doSearch(keyword)
  },

  // 历史管理
  async onDeleteHistory(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset as { id: number }
    try {
      await deleteSearchHistory(id)
      this.setData({ history: this.data.history.filter((h) => h.id !== id) })
    } catch (err) {
      console.warn('删除历史失败', err)
    }
  },

  async onClearHistory() {
    wx.showModal({
      title: '清除历史',
      content: '确认清除全部搜索历史？',
      success: async (res) => {
        if (res.confirm) {
          await clearSearchHistory().catch(() => null)
          this.setData({ history: [] })
        }
      }
    })
  },

  // 结果 Tab 切换
  onResultTabChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ activeResultTab: e.detail.value })
  },

  onRetryInitial() {
    this.loadInitialData()
  },

  onRetrySearch() {
    const keyword = this.data.lastSearchKeyword || this.data.keyword.trim()
    if (!keyword) return
    this.doSearch(keyword)
  },

  // 点击结果跳转
  onDishTap(e: WechatMiniprogram.CustomEvent) {
    const { id, merchantId } = e.currentTarget.dataset as { id: number, merchantId: number }
    wx.navigateTo({ url: `/pages/takeout/dish-detail/index?id=${id}&merchant_id=${merchantId}` })
  },

  onMerchantTap(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset as { id: number }
    wx.navigateTo({ url: `/pages/takeout/restaurant-detail/index?id=${id}` })
  },

  stopPropagation() {
    // 阻止冒泡
  },

  onUnload() {
    if (debounceTimer) {
      clearTimeout(debounceTimer)
      debounceTimer = null
    }
  }
})
