import { DishCategory, DishManagementService } from '../../../../api/dish'
import { getStableBarHeights } from '../../../../utils/responsive'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import { settleAll, isSettledFulfilled } from '../../../../utils/promise'

interface CategoryOption {
  label: string
  value: string
}

function buildCategoryOptions(categories: DishCategory[]): CategoryOption[] {
  return categories.map((category) => ({
    label: category.name,
    value: String(category.id)
  }))
}

function buildLinkableGlobalCategories(categories: DishCategory[], globalCategories: DishCategory[]): DishCategory[] {
  const localNames = new Set(categories.map((category) => category.name.trim()))
  return globalCategories.filter((category) => !localNames.has(category.name.trim()))
}

Page({
  data: {
    navBarHeight: 88,
    bootstrapped: false,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    submitting: false,
    pendingDeleteId: 0,
    categories: [] as DishCategory[],
    globalCategories: [] as DishCategory[],
    linkableGlobalCategories: [] as DishCategory[],
    pickerOptions: [] as CategoryOption[],
    selectedCategoryId: '',
    selectedCategoryName: '',
    linkPopupVisible: false,
    pickerVisible: false
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    void this.loadData(true)
  },

  onPullDownRefresh() {
    void this.loadData(false)
  },

  async loadData(initialLoad = false) {
    if (this.data.initialLoading && !initialLoad) {
      return
    }

    const showInitialState = initialLoad || !this.data.bootstrapped
    if (showInitialState) {
      this.setData({
        initialLoading: true,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } else {
      this.setData({ refreshErrorMessage: '' })
    }

    try {
      const [categoriesResult, globalCategoriesResult] = await settleAll([
        DishManagementService.getDishCategories(),
        DishManagementService.getGlobalDishCategories()
      ] as const)

      if (!isSettledFulfilled(categoriesResult)) {
        throw categoriesResult.reason
      }

      const categories = categoriesResult.value || []
      const globalCategories = isSettledFulfilled(globalCategoriesResult)
        ? globalCategoriesResult.value || []
        : this.data.globalCategories
      const linkableGlobalCategories = buildLinkableGlobalCategories(categories, globalCategories)
      const pickerOptions = buildCategoryOptions(linkableGlobalCategories)

      this.setData({
        bootstrapped: true,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: isSettledFulfilled(globalCategoriesResult)
          ? ''
          : '全局分类库暂时不可用，当前仍可维护已有分类',
        categories,
        globalCategories,
        linkableGlobalCategories,
        pickerOptions,
        selectedCategoryId: pickerOptions[0]?.value || '',
        selectedCategoryName: pickerOptions[0]?.label || ''
      })
    } catch (err) {
      logger.error('Load dish categories page failed', err)
      const message = getErrorUserMessage(err, '分类列表加载失败，请重试')
      if (showInitialState || !this.data.categories.length) {
        this.setData({
          bootstrapped: false,
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else {
        this.setData({
          initialLoading: false,
          refreshErrorMessage: message
        })
      }
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  onRetry() {
    void this.loadData(true)
  },

  onRetryRefresh() {
    void this.loadData(false)
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

  hasCategoryName(name: string, excludeId = 0): boolean {
    return this.data.categories.some((category) => category.id !== excludeId && category.name.trim() === name.trim())
  },

  onOpenLinkPopup() {
    if (!this.data.linkableGlobalCategories.length) {
      const message = this.data.globalCategories.length
        ? '没有可关联的全局分类'
        : '全局分类暂不可用，请稍后重试'
      wx.showToast({ title: message, icon: 'none' })
      return
    }

    this.setData({
      linkPopupVisible: true,
      selectedCategoryId: this.data.pickerOptions[0]?.value || '',
      selectedCategoryName: this.data.pickerOptions[0]?.label || ''
    })
  },

  onCloseLinkPopup() {
    this.setData({
      linkPopupVisible: false,
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

    this.setData({
      selectedCategoryId: String(values[0] || ''),
      selectedCategoryName: String(labels[0] || ''),
      pickerVisible: false
    })
  },

  async onConfirmLinkCategory() {
    if (this.data.submitting) {
      return
    }

    const targetName = this.data.selectedCategoryName.trim()
    if (!targetName) {
      wx.showToast({ title: '请选择分类', icon: 'none' })
      return
    }

    if (this.hasCategoryName(targetName)) {
      wx.showToast({ title: '该分类已在当前商户中', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    try {
      await DishManagementService.createDishCategory({
        name: targetName,
        sort_order: this.data.categories.length + 1
      })
      this.setData({ linkPopupVisible: false })
      await this.loadData(false)
    } catch (err) {
      logger.error('Link global category failed', err)
      wx.showToast({ title: getErrorUserMessage(err, '关联分类失败，请重试'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  async onCreateCategory() {
    if (this.data.submitting) {
      return
    }

    const name = await this.promptCategoryName('新增分类')
    if (!name) {
      return
    }

    if (this.hasCategoryName(name)) {
      wx.showToast({ title: '该分类已在当前商户中', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    try {
      await DishManagementService.createDishCategory({
        name,
        sort_order: this.data.categories.length + 1
      })
      await this.loadData(false)
    } catch (err) {
      logger.error('Create dish category failed', err)
      wx.showToast({ title: getErrorUserMessage(err, '创建分类失败，请重试'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  async onEditCategory(e: WechatMiniprogram.TouchEvent) {
    if (this.data.submitting) {
      return
    }

    const { id, name } = e.currentTarget.dataset as { id?: number, name?: string }
    if (!id) {
      return
    }

    const nextName = await this.promptCategoryName('编辑分类', String(name || ''))
    if (!nextName) {
      return
    }

    if (this.hasCategoryName(nextName, id)) {
      wx.showToast({ title: '分类名称已存在', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    try {
      await DishManagementService.updateDishCategory(id, { name: nextName })
      await this.loadData(false)
    } catch (err) {
      logger.error('Update dish category failed', err)
      wx.showToast({ title: getErrorUserMessage(err, '更新分类失败，请重试'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  onDeleteCategory(e: WechatMiniprogram.TouchEvent) {
    if (this.data.submitting) {
      return
    }

    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) {
      return
    }

    wx.showModal({
      title: '删除分类',
      content: '删除后该分类将不可继续使用。确认删除？',
      success: async (res) => {
        if (!res.confirm) {
          return
        }

        this.setData({ pendingDeleteId: id })
        try {
          await DishManagementService.deleteDishCategory(id)
          await this.loadData(false)
        } catch (err) {
          logger.error('Delete dish category failed', err)
          wx.showToast({ title: getErrorUserMessage(err, '删除分类失败，请重试'), icon: 'none' })
        } finally {
          this.setData({ pendingDeleteId: 0 })
        }
      }
    })
  }
})