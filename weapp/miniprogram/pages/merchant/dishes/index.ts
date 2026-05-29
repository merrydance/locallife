import { getStableBarHeights } from '../../../utils/responsive'
import { DishManagementService, DishResponse, DishCategory } from '../_main_shared/api/dish'
import { getPublicImageUrl } from '../../../utils/image'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'

const getErrorMessage = getErrorUserMessage

type DishOnlineFilterKey = 'all' | 'online' | 'offline'

interface DishFilterOption<T> {
  label: string
  value: T
}

interface DishListItem extends DishResponse {
  statusPending?: boolean
  deletePending?: boolean
}

const ONLINE_FILTER_OPTIONS: DishFilterOption<DishOnlineFilterKey>[] = [
  { label: '全部状态', value: 'all' },
  { label: '已上架', value: 'online' },
  { label: '已下架', value: 'offline' }
]

function normalizeDish(dish: DishResponse): DishListItem {
  return {
    ...dish,
    image_url: getPublicImageUrl(dish.image_url),
    statusPending: false,
    deletePending: false
  }
}

function buildResultSummaryText(params: {
  visibleCount: number
  currentCategoryId: number
  currentOnlineFilter: DishOnlineFilterKey
}) {
  const activeFilters: string[] = []
  if (params.currentCategoryId > 0) {
    activeFilters.push('分类')
  }
  if (params.currentOnlineFilter !== 'all') {
    activeFilters.push(params.currentOnlineFilter === 'online' ? '已上架' : '已下架')
  }

  if (activeFilters.length > 0) {
    return `${activeFilters.join(' / ')}下共 ${params.visibleCount} 项`
  }

  return `当前共 ${params.visibleCount} 项菜品`
}

function buildEmptyDescription(params: {
  currentCategoryId: number
  currentOnlineFilter: DishOnlineFilterKey
}) {
  if (params.currentCategoryId > 0 || params.currentOnlineFilter !== 'all') {
    return '暂无符合当前筛选条件的菜品'
  }

  return '还没有菜品，先新增一个'
}

function buildDishPresentationState(params: {
  loadedDishes: DishListItem[]
  currentCategoryId: number
  currentOnlineFilter: DishOnlineFilterKey
}) {
  const dishes = params.loadedDishes

  return {
    dishes,
    resultSummaryText: buildResultSummaryText({
      visibleCount: dishes.length,
      currentCategoryId: params.currentCategoryId,
      currentOnlineFilter: params.currentOnlineFilter
    }),
    emptyDescription: buildEmptyDescription({
      currentCategoryId: params.currentCategoryId,
      currentOnlineFilter: params.currentOnlineFilter
    })
  }
}

function resolveHasMore(total: number | undefined, loadedCount: number, pageSize: number, lastPageLength: number) {
  if (typeof total === 'number' && total >= 0) {
    return loadedCount < total
  }

  return lastPageLength >= pageSize
}

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: false,
    loadedDishes: [] as DishListItem[],
    dishes: [] as DishListItem[],
    categories: [] as DishCategory[],
    onlineFilterOptions: ONLINE_FILTER_OPTIONS,
    currentCategoryId: 0,
    currentOnlineFilter: 'all' as DishOnlineFilterKey,
    resultSummaryText: '当前共 0 项菜品',
    emptyDescription: '还没有菜品，先新增一个',
    pageId: 1,
    pageSize: 20,
    hasMore: true,
    deleteDialogVisible: false,
    deleteDialogSubmitting: false,
    deleteDialogDishId: 0,
    deleteDialogDishName: ''
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.refreshAll()
  },

  buildPresentationUpdate(loadedDishes: DishListItem[]) {
    return buildDishPresentationState({
      loadedDishes,
      currentCategoryId: this.data.currentCategoryId,
      currentOnlineFilter: this.data.currentOnlineFilter
    })
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
    if (this.data.loading) {
      return
    }
    if (!reset && !this.data.hasMore) {
      return
    }

    const hasExistingDishes = this.data.loadedDishes.length > 0
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
      const pageId = reset ? 1 : this.data.pageId
      const params = this.buildDishListParams(pageId, this.data.pageSize)
      const response = await DishManagementService.listDishes(params)
      const sourceDishes = Array.isArray(response?.dishes)
        ? response.dishes.filter((dish): dish is DishResponse => !!dish)
        : []
      const newDishes = sourceDishes.map(normalizeDish)
      const loadedDishes = reset ? newDishes : [...this.data.loadedDishes, ...newDishes]
      const total = typeof response?.total === 'number' ? response.total : undefined

      this.setData({
        loadedDishes,
        ...this.buildPresentationUpdate(loadedDishes),
        pageId: pageId + 1,
        hasMore: resolveHasMore(total, loadedDishes.length, this.data.pageSize, newDishes.length),
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

  buildDishListParams(pageId: number, pageSize: number) {
    const params: {
      category_id?: number
      is_online?: boolean
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

    return params
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

  onOnlineFilterChange(e: WechatMiniprogram.TouchEvent) {
    const { value } = e.currentTarget.dataset as { value?: DishOnlineFilterKey }
    if (!value || value === this.data.currentOnlineFilter) {
      return
    }

    this.setData({
      currentOnlineFilter: value,
      pageId: 1,
      hasMore: true
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

  onRetry() {
    this.refreshAll()
  },

  onRetryRefresh() {
    this.loadDishesInternal(true, false)
  },

  onActionsCatch() {},

  onDishCardTap(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) {
      return
    }

    wx.navigateTo({ url: `./edit/index?id=${id}` })
  },

  async onSwitchStatusChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) {
      return
    }

    const targetDish = this.data.loadedDishes.find((dish) => dish.id === id)
    if (!targetDish || targetDish.statusPending || targetDish.deletePending) {
      return
    }
    if (targetDish.is_packaging) {
      wx.showToast({ title: '包装菜品必须保持上架', icon: 'none' })
      return
    }

    const targetStatus = !!e.detail?.value
    if (targetStatus === targetDish.is_online) {
      return
    }

    const pendingDishes = this.data.loadedDishes.map((dish) => (
      dish.id === id ? { ...dish, statusPending: true } : dish
    ))
    this.setData({
      loadedDishes: pendingDishes,
      ...this.buildPresentationUpdate(pendingDishes)
    })

    try {
      await DishManagementService.updateDishStatus(id, { is_online: targetStatus })

      const nextLoadedDishes = pendingDishes.map((dish) => (
        dish.id === id
          ? { ...dish, is_online: targetStatus, statusPending: false }
          : dish
      ))

      this.setData({
        loadedDishes: nextLoadedDishes,
        ...this.buildPresentationUpdate(nextLoadedDishes)
      })

      if (this.data.currentOnlineFilter !== 'all') {
        void this.loadDishesInternal(true, false)
      }

    } catch (err) {
      logger.error('Toggle dish status failed', err)
      const restoredDishes = pendingDishes.map((dish) => (
        dish.id === id ? { ...dish, statusPending: false } : dish
      ))

      this.setData({
        loadedDishes: restoredDishes,
        ...this.buildPresentationUpdate(restoredDishes)
      })
      wx.showToast({ title: getErrorMessage(err, '操作失败，请稍后重试'), icon: 'none' })
    }
  },

  onAddDish() {
    wx.navigateTo({ url: './edit/index' })
  },

  onDishImageError(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) {
      return
    }

    const nextLoadedDishes = this.data.loadedDishes.map((dish) => {
      if (dish.id !== id || dish.image_url === '/assets/icons/empty.svg') {
        return dish
      }
      return {
        ...dish,
        image_url: '/assets/icons/empty.svg'
      }
    })

    this.setData({
      loadedDishes: nextLoadedDishes,
      ...this.buildPresentationUpdate(nextLoadedDishes)
    })
  },

  onRequestDeleteDish(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) {
      return
    }

    const targetDish = this.data.loadedDishes.find((dish) => dish.id === id)
    if (!targetDish || targetDish.deletePending) {
      return
    }

    this.setData({
      deleteDialogVisible: true,
      deleteDialogSubmitting: false,
      deleteDialogDishId: id,
      deleteDialogDishName: targetDish.name || '该菜品'
    })
  },

  onCancelDeleteDialog() {
    if (this.data.deleteDialogSubmitting) {
      return
    }

    this.setData({
      deleteDialogVisible: false,
      deleteDialogDishId: 0,
      deleteDialogDishName: '',
      deleteDialogSubmitting: false
    })
  },

  async onConfirmDeleteDish() {
    const id = Number(this.data.deleteDialogDishId || 0)
    if (!id) {
      this.onCancelDeleteDialog()
      return
    }

    const targetDish = this.data.loadedDishes.find((dish) => dish.id === id)
    if (!targetDish || targetDish.deletePending) {
      this.onCancelDeleteDialog()
      return
    }

    this.setData({ deleteDialogSubmitting: true })

    const pendingDishes = this.data.loadedDishes.map((dish) => (
      dish.id === id ? { ...dish, deletePending: true } : dish
    ))
    this.setData({
      loadedDishes: pendingDishes,
      ...this.buildPresentationUpdate(pendingDishes)
    })

    try {
      await DishManagementService.deleteDish(id)
      const nextLoadedDishes = pendingDishes.filter((dish) => dish.id !== id)

      this.setData({
        deleteDialogVisible: false,
        deleteDialogSubmitting: false,
        deleteDialogDishId: 0,
        deleteDialogDishName: '',
        loadedDishes: nextLoadedDishes,
        ...this.buildPresentationUpdate(nextLoadedDishes)
      })

      if (!nextLoadedDishes.length && this.data.pageId > 1) {
        void this.loadDishesInternal(true, false)
      }
      wx.showToast({ title: '菜品已删除', icon: 'none' })
    } catch (err) {
      logger.error('Delete dish failed', err)
      const restoredDishes = pendingDishes.map((dish) => (
        dish.id === id ? { ...dish, deletePending: false } : dish
      ))

      this.setData({
        deleteDialogSubmitting: false,
        loadedDishes: restoredDishes,
        ...this.buildPresentationUpdate(restoredDishes)
      })
      wx.showToast({ title: getErrorMessage(err, '删除失败，请稍后重试'), icon: 'none' })
    }
  }
})