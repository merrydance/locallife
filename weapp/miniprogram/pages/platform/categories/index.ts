import { TagService, type TagInfo } from '../_main_shared/api/dish'
import { getStableBarHeights } from '../../../utils/responsive'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { buildCategoryIconEmoji } from '../../../adapters/takeout-categories'

interface CategoryView extends TagInfo {
  iconText: string
}

type FormInputDetail = { value?: string }
type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>
type IconTapEvent = WechatMiniprogram.TouchEvent & {
  currentTarget: { dataset: { icon?: string } }
}
type CategoryEvent = WechatMiniprogram.TouchEvent & {
  currentTarget: { dataset: { id?: number | string } }
  target?: { dataset?: { id?: number | string } }
  detail?: {
    currentTarget?: { dataset?: { id?: number | string } }
    target?: { dataset?: { id?: number | string } }
  }
}

const CATEGORY_ICON_OPTIONS = [
  '🥘',
  '🍜',
  '🍲',
  '🥞',
  '🥣',
  '🍚',
  '🥡',
  '🍱',
  '🍣',
  '🍕',
  '🍔',
  '🍗',
  '🥗',
  '🦐',
  '🐟',
  '🧋',
  '☕',
  '🍰',
  '🔥',
  '🍴'
]

function normalizeCategories(tags: TagInfo[]): CategoryView[] {
  return Array.isArray(tags)
    ? tags
      .filter((tag): tag is TagInfo => !!tag && Number.isFinite(tag.id) && tag.id > 0)
      .map((tag) => ({
        ...tag,
        iconText: tag.icon || buildCategoryIconEmoji(tag.name)
      }))
    : []
}

function readCategoryEventId(e: CategoryEvent): number {
  const candidates = [
    e.currentTarget?.dataset?.id,
    e.target?.dataset?.id,
    e.detail?.currentTarget?.dataset?.id,
    e.detail?.target?.dataset?.id
  ]

  for (const candidate of candidates) {
    const id = Number(candidate)
    if (Number.isFinite(id) && id > 0) {
      return id
    }
  }

  return 0
}

Page({
  data: {
    navBarHeight: 88,
    bootstrapped: false,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    refreshing: false,
    submitting: false,
    pendingDeleteId: 0,
    createDialogVisible: false,
    editingCategoryId: 0,
    createInputValue: '',
    selectedIcon: CATEGORY_ICON_OPTIONS[0],
    iconOptions: CATEGORY_ICON_OPTIONS,
    categories: [] as CategoryView[]
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    void this.loadData(true)
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  async onRefresh() {
    this.setData({ refreshing: true })
    try {
      await this.loadData(false)
    } finally {
      this.setData({ refreshing: false })
    }
  },

  async loadData(initialLoad = false) {
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
      const categories = normalizeCategories(await TagService.listTags('merchant'))
      this.setData({
        bootstrapped: true,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        categories
      })
    } catch (err) {
      logger.error('Load platform merchant categories page failed', err)
      const message = getErrorUserMessage(err, '经营品类加载失败，请重试')

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
    }
  },

  onRetry() {
    void this.loadData(true)
  },

  onRetryRefresh() {
    void this.loadData(false)
  },

  hasCategoryName(name: string): boolean {
    return this.data.categories.some((tag) => tag.name.trim() === name.trim())
  },

  onCreateCategory() {
    if (this.data.submitting) {
      return
    }

    this.setData({
      createDialogVisible: true,
      editingCategoryId: 0,
      createInputValue: '',
      selectedIcon: CATEGORY_ICON_OPTIONS[0]
    })
  },

  onCloseCreateCategoryDialog() {
    if (this.data.submitting) {
      return
    }

    this.setData({
      createDialogVisible: false,
      editingCategoryId: 0,
      createInputValue: '',
      selectedIcon: CATEGORY_ICON_OPTIONS[0]
    })
  },

  onCreateInputChange(e: WechatMiniprogram.CustomEvent<FormInputDetail>) {
    const value = (e.detail?.value || '').replace(/^\s+/, '')
    this.setData({ createInputValue: value })
  },

  onSelectIcon(e: IconTapEvent) {
    const icon = e.currentTarget.dataset.icon || ''
    if (!icon) return
    this.setData({ selectedIcon: icon })
  },

  onEditCategory(e: CategoryEvent) {
    if (this.data.submitting) {
      return
    }

    const id = readCategoryEventId(e)
    const category = this.data.categories.find((item) => item.id === id)
    if (!category) {
      return
    }

    this.setData({
      createDialogVisible: true,
      editingCategoryId: category.id,
      createInputValue: category.name,
      selectedIcon: category.iconText || CATEGORY_ICON_OPTIONS[0]
    })
  },

  async onConfirmCreateCategoryDialog() {
    if (this.data.submitting) {
      return
    }

    const name = this.data.createInputValue.trim()
    if (!name) {
      wx.showToast({ title: '品类名称不能为空', icon: 'none' })
      return
    }

    const editingId = this.data.editingCategoryId
    if (this.data.categories.some((tag) => tag.id !== editingId && tag.name.trim() === name.trim())) {
      wx.showToast({ title: '品类名称已存在', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    try {
      if (editingId) {
        await TagService.updateTag(editingId, { name, icon: this.data.selectedIcon })
      } else {
        await TagService.createTag({ name, type: 'merchant', icon: this.data.selectedIcon })
      }
      this.setData({
        createDialogVisible: false,
        editingCategoryId: 0,
        createInputValue: '',
        selectedIcon: CATEGORY_ICON_OPTIONS[0]
      })
      await this.loadData(false)
    } catch (err) {
      logger.error('Save platform merchant category failed', err)
      wx.showToast({ title: getErrorUserMessage(err, editingId ? '保存品类失败，请重试' : '创建品类失败，请重试'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  onDeleteCategory(e: CategoryEvent) {
    if (this.data.submitting) {
      return
    }

    const id = readCategoryEventId(e)
    if (!id) {
      return
    }

    wx.showModal({
      title: '删除品类',
      content: '删除后该品类将不再出现在商户资料选择和首页品类筛选中。确认删除？',
      success: async (res) => {
        if (!res.confirm) {
          return
        }

        this.setData({ pendingDeleteId: id })
        try {
          await TagService.deleteTag(id)
          await this.loadData(false)
        } catch (err) {
          logger.error('Delete platform merchant category failed', err)
          wx.showToast({ title: getErrorUserMessage(err, '删除品类失败，请重试'), icon: 'none' })
        } finally {
          this.setData({ pendingDeleteId: 0 })
        }
      }
    })
  }
})
