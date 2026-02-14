import { getStableBarHeights } from '../../../utils/responsive'
import { DishManagementService, DishResponse, DishCategory } from '../../../api/dish'
import { logger } from '../../../utils/logger'

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    dishes: [] as DishResponse[],
    categories: [] as DishCategory[],
    currentCategoryId: 0,
    searchKeyword: '',
    pageId: 1,
    pageSize: 20,
    hasMore: true,
    swipeActions: [
      { text: '编辑', theme: 'primary' },
      { text: '删除', theme: 'danger' }
    ]
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.refreshAll()
  },

  async refreshAll() {
    this.setData({ pageId: 1, dishes: [], hasMore: true })
    await Promise.all([
      this.loadCategories(),
      this.loadDishes()
    ])
  },

  async loadCategories() {
    try {
      const categories = await DishManagementService.getDishCategories()
      this.setData({ categories })
    } catch (err) {
      logger.error('Failed to load dish categories', err)
    }
  },

  async loadDishes() {
    if (this.data.loading || !this.data.hasMore) return
    
    this.setData({ loading: true })
    try {
      const res = await DishManagementService.listDishes({
        category_id: this.data.currentCategoryId || undefined,
        page_id: this.data.pageId,
        page_size: this.data.pageSize
      })

      const newDishes = res.dishes || []
      // 客户端搜索过滤 (若后端未完全支持 keyword 筛选)
      const filteredDishes = this.data.searchKeyword 
        ? newDishes.filter((d) => d.name.includes(this.data.searchKeyword))
        : newDishes

      this.setData({
        dishes: [...this.data.dishes, ...filteredDishes],
        pageId: this.data.pageId + 1,
        hasMore: newDishes.length === this.data.pageSize
      })
    } catch (err) {
      logger.error('Failed to load dishes', err)
      wx.showToast({ title: '加载失败', icon: 'error' })
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: number }>) {
    this.setData({ 
      currentCategoryId: e.detail.value,
      pageId: 1,
      dishes: [],
      hasMore: true
    }, () => {
      this.loadDishes()
    })
  },

  onSearchChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ searchKeyword: e.detail.value })
  },

  onSearchSubmit() {
    this.setData({ pageId: 1, dishes: [], hasMore: true }, () => {
      this.loadDishes()
    })
  },

  onPullDownRefresh() {
    this.refreshAll()
  },

  onReachBottom() {
    this.loadDishes()
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
      
      wx.showToast({ title: targetStatus ? '上架成功' : '已下架', icon: 'success' })
    } catch (err) {
      logger.error('Toggle dish status failed', err)
      wx.showToast({ title: '操作失败', icon: 'error' })
    } finally {
      wx.hideLoading()
    }
  },

  // ==================== 编辑/删除 ====================

  onAddDish() {
    wx.navigateTo({ url: '../edit/index' })
  },

  onEditDish(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    wx.navigateTo({ url: `../edit/index?id=${id}` })
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
          wx.showToast({ title: '删除成功', icon: 'success' })
        } catch (err) {
          logger.error('Delete dish failed', err)
          wx.showToast({ title: '删除失败', icon: 'none' })
        }
      }
    })
  }
})
