import { DishCategory, DishManagementService } from '../../../../api/dish'
import { getStableBarHeights } from '../../../../utils/responsive'
import { logger } from '../../../../utils/logger'

interface CategoryOption {
  label: string
  value: string
}

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    submitting: false,
    categories: [] as DishCategory[],
    globalCategories: [] as DishCategory[],
    addPopupVisible: false,
    pickerVisible: false,
    pickerOptions: [] as CategoryOption[],
    selectedCategoryId: '',
    selectedCategoryName: ''
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadCategories()
  },

  onShow() {
    this.loadCategories()
  },

  async loadCategories() {
    if (this.data.loading) return
    this.setData({ loading: true })
    try {
      const categories = await DishManagementService.getDishCategories()
      this.setData({ categories })
    } catch (err) {
      logger.error('Load dish categories failed', err)
      wx.showToast({ title: '加载分类失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  async loadGlobalCategories() {
    try {
      const categories = await DishManagementService.getGlobalDishCategories()
      this.setData({
        globalCategories: categories,
        pickerOptions: categories.map((item) => ({
          label: item.name,
          value: String(item.id)
        }))
      })
    } catch (err) {
      logger.error('Load global dish categories failed', err)
      wx.showToast({ title: '加载全局分类失败', icon: 'none' })
    }
  },

  onPullDownRefresh() {
    this.loadCategories()
  },

  async promptCategoryName(title: string, initialValue = ''): Promise<string | null> {
    return new Promise((resolve) => {
      wx.showModal({
        title,
        editable: true,
        placeholderText: '请输入分类名称',
        content: initialValue,
        success: (res) => {
          if (!res.confirm) {
            resolve(null)
            return
          }
          const name = (res.content || '').trim()
          if (!name) {
            wx.showToast({ title: '分类名称不能为空', icon: 'none' })
            resolve(null)
            return
          }
          resolve(name)
        },
        fail: () => resolve(null)
      })
    })
  },

  async onAddCategory() {
    await this.loadGlobalCategories()
    const defaultOption = this.data.pickerOptions[0]
    this.setData({
      addPopupVisible: true,
      selectedCategoryId: defaultOption?.value || '',
      selectedCategoryName: defaultOption?.label || ''
    })
  },

  onCloseAddPopup() {
    this.setData({
      addPopupVisible: false,
      pickerVisible: false
    })
  },

  onOpenPicker() {
    if (!this.data.pickerOptions.length) {
      wx.showToast({ title: '暂无可选分类', icon: 'none' })
      return
    }
    this.setData({ pickerVisible: true })
  },

  onPickerCancel() {
    this.setData({ pickerVisible: false })
  },

  onPickerConfirm(e: WechatMiniprogram.CustomEvent<{ value: Array<string | number> | null, label: string[] | null }>) {
    const values = Array.isArray(e.detail?.value) ? e.detail.value : []
    const labels = Array.isArray(e.detail?.label) ? e.detail.label : []
    const selectedCategoryId = String(values[0] || '')
    const selectedCategoryName = String(labels[0] || '')

    this.setData({
      selectedCategoryId,
      selectedCategoryName,
      pickerVisible: false
    })
  },

  async onConfirmLinkCategory() {
    if (this.data.submitting) return

    const targetName = this.data.selectedCategoryName.trim()
    if (!targetName) {
      wx.showToast({ title: '请选择分类', icon: 'none' })
      return
    }

    const exists = this.data.categories.some((item) => item.name === targetName)
    if (exists) {
      wx.showToast({ title: '该分类已在当前商户中', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    try {
      await DishManagementService.createDishCategory({
        name: targetName,
        sort_order: this.data.categories.length + 1
      })
      wx.showToast({ title: '已关联分类', icon: 'success' })
      this.setData({ addPopupVisible: false })
      await this.loadCategories()
    } catch (err) {
      logger.error('Link global category failed', err)
      wx.showToast({ title: '关联失败', icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  async onCreateCategory() {
    if (this.data.submitting) return
    const name = await this.promptCategoryName('新增分类')
    if (!name) return

    const exists = this.data.categories.some((item) => item.name === name)
    if (exists) {
      wx.showToast({ title: '该分类已在当前商户中', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    try {
      await DishManagementService.createDishCategory({
        name,
        sort_order: this.data.categories.length + 1
      })
      wx.showToast({ title: '创建成功', icon: 'success' })
      this.setData({ addPopupVisible: false })
      await this.loadCategories()
    } catch (err) {
      logger.error('Create dish category failed', err)
      wx.showToast({ title: '创建失败', icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  async onEditCategory(e: WechatMiniprogram.TouchEvent) {
    if (this.data.submitting) return
    const { id, name } = e.currentTarget.dataset as { id?: number, name?: string }
    if (!id) return

    const nextName = await this.promptCategoryName('编辑分类', String(name || ''))
    if (!nextName) return

    this.setData({ submitting: true })
    try {
      await DishManagementService.updateDishCategory(id, { name: nextName })
      wx.showToast({ title: '更新成功', icon: 'success' })
      await this.loadCategories()
    } catch (err) {
      logger.error('Update dish category failed', err)
      wx.showToast({ title: '更新失败', icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  onDeleteCategory(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return

    wx.showModal({
      title: '删除分类',
      content: '删除后不会删除菜品，但该分类将不可用。确认删除？',
      success: async (res) => {
        if (!res.confirm) return
        try {
          await DishManagementService.deleteDishCategory(id)
          wx.showToast({ title: '删除成功', icon: 'success' })
          this.loadCategories()
        } catch (err) {
          logger.error('Delete dish category failed', err)
          wx.showToast({ title: '删除失败', icon: 'none' })
        }
      }
    })
  }
})
