import { getStableBarHeights } from '../../../utils/responsive'
import { DishManagementService, DishResponse, DishCategory } from '../../../api/dish'
import { getPublicImageUrl } from '../../../utils/image'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'

const getErrorMessage = getErrorUserMessage

function normalizeDish(dish: DishResponse): DishResponse {
  return {
    ...dish,
    image_url: getPublicImageUrl(dish.image_url)
  }
}

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: false,
    dishes: [] as DishResponse[],
    categories: [] as DishCategory[],
    currentCategoryId: 0,
    searchKeyword: '',
    pageId: 1,
    pageSize: 20,
    hasMore: true
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.refreshAll()
  },

  async refreshAll(showLoading = true) {
    this.setData({ pageId: 1, hasMore: true })
    await Promise.all([
      this.loadCategories(),
      this.loadDishesInternal(true, showLoading)
    ])
  },

  async loadCategories() {
    try {
      const categories = await DishManagementService.getDishCategories()
      this.setData({ categories: Array.isArray(categories) ? categories : [] })
    } catch (err) {
      logger.error('Failed to load dish categories', err)
      this.setData({ categories: [] })
    }
  },

  async loadDishes() {
    return this.loadDishesInternal(false)
  },

  async loadDishesInternal(reset = false, showLoading = true) {
    if (this.data.loading) return

    const keyword = this.data.searchKeyword.trim()
    const isSearchMode = keyword.length > 0
    if (!reset && !isSearchMode && !this.data.hasMore) return
    if (!reset && isSearchMode) return

    const hasExistingDishes = this.data.dishes.length > 0
    const isSilentRefresh = reset && !showLoading && hasExistingDishes

    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
        : isSilentRefresh
          ? { refreshErrorMessage: '' }
          : {})
    })

    try {
      if (isSearchMode) {
        const matchedDishes = await this.searchAllDishes(keyword)
        this.setData({
          dishes: matchedDishes,
          pageId: 1,
          hasMore: false,
          initialLoading: false,
          initialError: false,
          initialErrorMessage: '',
          refreshErrorMessage: ''
        })
        return
      }

      const pageId = reset ? 1 : this.data.pageId
      const params: {
        category_id?: number
        page_id: number
        page_size: number
      } = {
        page_id: pageId,
        page_size: this.data.pageSize
      }
      if (this.data.currentCategoryId > 0) {
        params.category_id = this.data.currentCategoryId
      }

      const res = await DishManagementService.listDishes(params)
      const sourceDishes = Array.isArray(res?.dishes) ? res.dishes.filter((dish): dish is DishResponse => !!dish) : []
      const newDishes = sourceDishes.map(normalizeDish)
      const total = typeof res?.total === 'number' ? res.total : 0

      this.setData({
        dishes: reset ? newDishes : [...this.data.dishes, ...newDishes],
        pageId: pageId + 1,
        hasMore: total > 0 ? pageId * this.data.pageSize < total : newDishes.length === this.data.pageSize,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } catch (err) {
      logger.error('Failed to load dishes', err)
      const message = getErrorMessage(err, '菜品加载失败，请稍后重试')

      if (this.data.initialLoading || (!hasExistingDishes && reset)) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else if (hasExistingDishes) {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  async searchAllDishes(keyword: string) {
    const dishes: DishResponse[] = []
    let pageId = 1
    let hasMorePages = true

    while (hasMorePages) {
      const params: {
        category_id?: number
        page_id: number
        page_size: number
      } = {
        page_id: pageId,
        page_size: 50
      }

      if (this.data.currentCategoryId > 0) {
        params.category_id = this.data.currentCategoryId
      }

      const response = await DishManagementService.listDishes(params)
      const pageDishes = Array.isArray(response?.dishes)
        ? response.dishes.filter((dish): dish is DishResponse => !!dish).map(normalizeDish)
        : []

      dishes.push(...pageDishes)

      const total = typeof response?.total === 'number' ? response.total : 0
      hasMorePages = !(pageDishes.length < params.page_size || (total > 0 && dishes.length >= total))
      if (hasMorePages) {
        pageId += 1
      }
    }

    return dishes.filter((dish) => dish.name.includes(keyword))
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: string | number }>) {
    const nextCategoryId = Number(e.detail?.value || 0)
    this.setData({ 
      currentCategoryId: Number.isFinite(nextCategoryId) ? nextCategoryId : 0,
      pageId: 1,
      hasMore: true
    }, () => {
      this.loadDishesInternal(true)
    })
  },

  onSearchChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ searchKeyword: e.detail.value })
  },

  onSearchSubmit() {
    this.setData({ pageId: 1, hasMore: true }, () => {
      this.loadDishesInternal(true)
    })
  },

  onPullDownRefresh() {
    this.refreshAll(false)
  },

  onReachBottom() {
    this.loadDishesInternal(false)
  },

  onManageCategories() {
    wx.navigateTo({ url: '/pages/merchant/dishes/categories/index' })
  },

  onLoadMore() {
    this.loadDishesInternal(false)
  },

  onRetry() {
    this.refreshAll()
  },

  onRetryRefresh() {
    this.loadDishesInternal(true, false)
  },

  // ==================== 状态切换 ====================

  async onToggleStatus(e: WechatMiniprogram.TouchEvent) {
    const { id, online } = e.currentTarget.dataset as { id?: number, online?: boolean }
    if (!id) return
    const targetStatus = !online
    
    wx.showLoading({ title: '处理中...' })
    try {
      await DishManagementService.updateDishStatus(id, { is_online: targetStatus })
      
      // 更新本地状态
      const index = this.data.dishes.findIndex((d) => d.id === id)
      if (index > -1) {
        const key = `dishes[${index}].is_online`
        this.setData({ [key]: targetStatus })
      }

    } catch (err) {
      logger.error('Toggle dish status failed', err)
      wx.showToast({ title: '操作失败', icon: 'error' })
    } finally {
      wx.hideLoading()
    }
  },

  // ==================== 编辑/删除 ====================

  onAddDish() {
    wx.navigateTo({ url: './edit/index' })
  },

  onEditDish(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    wx.navigateTo({ url: `./edit/index?id=${id}` })
  },

  onDishImageError(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return

    const index = this.data.dishes.findIndex((d) => d.id === id)
    if (index < 0) return
    if (this.data.dishes[index].image_url === '/assets/icons/empty.svg') return

    this.setData({
      [`dishes[${index}].image_url`]: '/assets/icons/empty.svg'
    })
  },

  onDeleteDish(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return

    wx.showModal({
      title: '确认删除',
      content: '删除后无法恢复，确定要删除该菜品吗？',
      confirmText: '确认删除',
      cancelText: '取消',
      success: async (res) => {
        if (!res.confirm) return
        try {
          await DishManagementService.deleteDish(id)
          this.setData({
            dishes: this.data.dishes.filter((d) => d.id !== id)
          })
        } catch (err) {
          logger.error('Delete dish failed', err)
          wx.showToast({ title: '删除失败', icon: 'none' })
        }
      }
    })
  }
})
