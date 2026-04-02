import { getStableBarHeights } from '../../../utils/responsive'
import { DishManagementService, DishResponse, DishCategory } from '../../../api/dish'
import { getPublicImageUrl } from '../../../utils/image'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'

const getErrorMessage = getErrorUserMessage

type DishOnlineFilterKey = 'all' | 'online' | 'offline'
type DishAvailabilityFilterKey = 'all' | 'available' | 'unavailable'

interface DishFilterOption<T> {
  label: string
  value: T
}

const ONLINE_FILTER_OPTIONS: DishFilterOption<DishOnlineFilterKey>[] = [
  { label: '全部状态', value: 'all' },
  { label: '已上架', value: 'online' },
  { label: '已下架', value: 'offline' }
]

const AVAILABILITY_FILTER_OPTIONS: DishFilterOption<DishAvailabilityFilterKey>[] = [
  { label: '全部库存', value: 'all' },
  { label: '可售', value: 'available' },
  { label: '不可售', value: 'unavailable' }
]

interface DishListItem extends DishResponse {
  selected?: boolean
}

function normalizeDish(dish: DishResponse): DishListItem {
  return {
    ...dish,
    image_url: getPublicImageUrl(dish.image_url),
    selected: false
  }
}

function buildDishSelectionState(dishes: DishListItem[], selectedDishIds: number[]) {
  const dishIdSet = new Set(dishes.map((dish) => dish.id))
  const effectiveSelectedIds = Array.from(new Set(selectedDishIds)).filter((id) => dishIdSet.has(id))
  const selectedSet = new Set(effectiveSelectedIds)

  return {
    dishes: dishes.map((dish) => ({
      ...dish,
      selected: selectedSet.has(dish.id)
    })),
    selectedDishIds: effectiveSelectedIds,
    selectedCount: effectiveSelectedIds.length,
    allLoadedSelected: dishes.length > 0 && dishes.every((dish) => selectedSet.has(dish.id))
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
    dishes: [] as DishListItem[],
    categories: [] as DishCategory[],
    onlineFilterOptions: ONLINE_FILTER_OPTIONS,
    availabilityFilterOptions: AVAILABILITY_FILTER_OPTIONS,
    currentCategoryId: 0,
    currentOnlineFilter: 'all' as DishOnlineFilterKey,
    currentAvailabilityFilter: 'all' as DishAvailabilityFilterKey,
    searchKeyword: '',
    pageId: 1,
    pageSize: 20,
    hasMore: true,
    batchMode: false,
    batchSubmitting: false,
    batchTargetStatus: '' as '' | 'online' | 'offline',
    selectedDishIds: [] as number[],
    selectedCount: 0,
    allLoadedSelected: false
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
        const selectionState = buildDishSelectionState(
          matchedDishes,
          this.data.batchMode ? this.data.selectedDishIds : []
        )
        this.setData({
          ...selectionState,
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
      const params = this.buildDishListParams(pageId, this.data.pageSize)

      const res = await DishManagementService.listDishes(params)
      const sourceDishes = Array.isArray(res?.dishes) ? res.dishes.filter((dish): dish is DishResponse => !!dish) : []
      const newDishes = sourceDishes.map(normalizeDish)
      const total = typeof res?.total === 'number' ? res.total : 0
      const mergedDishes = reset ? newDishes : [...this.data.dishes, ...newDishes]
      const selectionState = buildDishSelectionState(
        mergedDishes,
        this.data.batchMode ? this.data.selectedDishIds : []
      )

      this.setData({
        ...selectionState,
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
    const dishes: DishListItem[] = []
    let pageId = 1
    let hasMorePages = true

    while (hasMorePages) {
      const params = this.buildDishListParams(pageId, 50)

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

  buildDishListParams(pageId: number, pageSize: number) {
    const params: {
      category_id?: number
      is_online?: boolean
      is_available?: boolean
      page_id: number
      page_size: number
    } = {
      page_id: pageId,
      page_size: pageSize
    }

    if (this.data.currentCategoryId > 0) {
      params.category_id = this.data.currentCategoryId
    }

    if (this.data.currentOnlineFilter === 'online') {
      params.is_online = true
    } else if (this.data.currentOnlineFilter === 'offline') {
      params.is_online = false
    }

    if (this.data.currentAvailabilityFilter === 'available') {
      params.is_available = true
    } else if (this.data.currentAvailabilityFilter === 'unavailable') {
      params.is_available = false
    }

    return params
  },

  hasActiveStatusFilter() {
    return this.data.currentOnlineFilter !== 'all' || this.data.currentAvailabilityFilter !== 'all'
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: string | number }>) {
    const nextCategoryId = Number(e.detail?.value || 0)
    this.setData({ 
      currentCategoryId: Number.isFinite(nextCategoryId) ? nextCategoryId : 0,
      pageId: 1,
      hasMore: true,
      batchMode: false,
      batchSubmitting: false,
      batchTargetStatus: '',
      ...buildDishSelectionState(this.data.dishes, [])
    }, () => {
      this.loadDishesInternal(true)
    })
  },

  onSearchChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ searchKeyword: e.detail.value })
  },

  onSearchSubmit() {
    this.setData({
      pageId: 1,
      hasMore: true,
      batchMode: false,
      batchSubmitting: false,
      batchTargetStatus: '',
      ...buildDishSelectionState(this.data.dishes, [])
    }, () => {
      this.loadDishesInternal(true)
    })
  },

  onOnlineFilterChange(e: WechatMiniprogram.TouchEvent) {
    const { value } = e.currentTarget.dataset as { value?: DishOnlineFilterKey }
    if (!value || value === this.data.currentOnlineFilter) {
      return
    }

    this.setData({
      currentOnlineFilter: value,
      pageId: 1,
      hasMore: true,
      batchMode: false,
      batchSubmitting: false,
      batchTargetStatus: '',
      ...buildDishSelectionState(this.data.dishes, [])
    }, () => {
      this.loadDishesInternal(true)
    })
  },

  onAvailabilityFilterChange(e: WechatMiniprogram.TouchEvent) {
    const { value } = e.currentTarget.dataset as { value?: DishAvailabilityFilterKey }
    if (!value || value === this.data.currentAvailabilityFilter) {
      return
    }

    this.setData({
      currentAvailabilityFilter: value,
      pageId: 1,
      hasMore: true,
      batchMode: false,
      batchSubmitting: false,
      batchTargetStatus: '',
      ...buildDishSelectionState(this.data.dishes, [])
    }, () => {
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

  onToggleBatchMode() {
    if (this.data.batchSubmitting) {
      return
    }

    const batchMode = !this.data.batchMode
    this.setData({
      batchMode,
      batchSubmitting: false,
      batchTargetStatus: '',
      ...buildDishSelectionState(this.data.dishes, [])
    })
  },

  onCancelBatchMode() {
    if (this.data.batchSubmitting) {
      return
    }

    this.setData({
      batchMode: false,
      batchSubmitting: false,
      batchTargetStatus: '',
      ...buildDishSelectionState(this.data.dishes, [])
    })
  },

  onSelectDish(e: WechatMiniprogram.TouchEvent) {
    if (!this.data.batchMode || this.data.batchSubmitting) {
      return
    }

    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) {
      return
    }

    const selectedSet = new Set(this.data.selectedDishIds)
    if (selectedSet.has(id)) {
      selectedSet.delete(id)
    } else {
      selectedSet.add(id)
    }

    this.setData(buildDishSelectionState(this.data.dishes, [...selectedSet]))
  },

  onSelectAllLoaded() {
    if (!this.data.batchMode || this.data.batchSubmitting || !this.data.dishes.length) {
      return
    }

    const nextSelectedIds = this.data.allLoadedSelected ? [] : this.data.dishes.map((dish) => dish.id)
    this.setData(buildDishSelectionState(this.data.dishes, nextSelectedIds))
  },

  async onBatchUpdateStatus(e: WechatMiniprogram.TouchEvent) {
    if (!this.data.batchMode || this.data.batchSubmitting) {
      return
    }

    const { online } = e.currentTarget.dataset as { online?: boolean | string }
    const targetStatus = online === true || online === 'true'
    const selectedDishIds = this.data.selectedDishIds

    if (!selectedDishIds.length) {
      wx.showToast({ title: '请先选择菜品', icon: 'none' })
      return
    }

    const actionText = targetStatus ? '上架' : '下架'
    wx.showModal({
      title: `确认批量${actionText}`,
      content: `将对已选 ${selectedDishIds.length} 个菜品执行${actionText}，是否继续？`,
      confirmText: `确认${actionText}`,
      cancelText: '取消',
      success: async (res) => {
        if (!res.confirm) {
          return
        }

        await this.applyBatchStatusUpdate(targetStatus)
      }
    })
  },

  async applyBatchStatusUpdate(targetStatus: boolean) {
    const selectedDishIds = this.data.selectedDishIds
    if (!selectedDishIds.length) {
      return
    }

    this.setData({
      batchSubmitting: true,
      batchTargetStatus: targetStatus ? 'online' : 'offline'
    })

    try {
      const response = await DishManagementService.batchUpdateDishStatus({
        dish_ids: selectedDishIds,
        is_online: targetStatus
      })

      const updatedIds = Array.isArray(response?.updated) ? response.updated : []
      const failedIds = Array.isArray(response?.failed) ? response.failed : []
      const shouldReloadFilteredList = this.data.currentOnlineFilter !== 'all' && updatedIds.length > 0
      const updatedSet = new Set(updatedIds)

      const dishes = shouldReloadFilteredList
        ? this.data.dishes
        : this.data.dishes.map((dish) => ({
          ...dish,
          is_online: updatedSet.has(dish.id) ? targetStatus : dish.is_online
        }))
      const selectionState = buildDishSelectionState(dishes, failedIds)
      const nextBatchMode = failedIds.length > 0

      this.setData({
        batchMode: nextBatchMode,
        batchSubmitting: false,
        batchTargetStatus: '',
        ...selectionState
      })

      const actionText = targetStatus ? '上架' : '下架'
      if (failedIds.length > 0) {
        wx.showModal({
          title: `批量${actionText}未全部成功`,
          content: `成功 ${updatedIds.length} 个，失败 ${failedIds.length} 个。已保留失败项勾选状态，方便继续处理。`,
          showCancel: false,
          confirmText: '我知道了'
        })
        return
      }

      if (updatedIds.length > 0) {
        wx.showToast({ title: `已批量${actionText}${updatedIds.length}个菜品`, icon: 'none' })
      }

      if (shouldReloadFilteredList) {
        this.loadDishesInternal(true, false)
      }
    } catch (err) {
      logger.error('Batch update dish status failed', err)
      this.setData({
        batchSubmitting: false,
        batchTargetStatus: ''
      })
      wx.showToast({ title: getErrorMessage(err, '批量操作失败，请稍后重试'), icon: 'none' })
    }
  },

  // ==================== 状态切换 ====================

  async onToggleStatus(e: WechatMiniprogram.TouchEvent) {
    const { id, online } = e.currentTarget.dataset as { id?: number, online?: boolean }
    if (!id) return
    const targetStatus = !online
    
    wx.showLoading({ title: '处理中...' })
    try {
      await DishManagementService.updateDishStatus(id, { is_online: targetStatus })

      if (this.hasActiveStatusFilter()) {
        this.loadDishesInternal(true, false)
      } else {
        const index = this.data.dishes.findIndex((d) => d.id === id)
        if (index > -1) {
          const key = `dishes[${index}].is_online`
          this.setData({ [key]: targetStatus })
        }
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
    if (this.data.batchMode) {
      return
    }

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
          await this.loadDishesInternal(true, false)
        } catch (err) {
          logger.error('Delete dish failed', err)
          wx.showToast({ title: '删除失败', icon: 'none' })
        }
      }
    })
  }
})
