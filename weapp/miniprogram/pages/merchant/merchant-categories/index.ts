import { getStableBarHeights } from '../../../utils/responsive'
import { logger } from '../../../utils/logger'
import {
  getMyMerchantTags,
  getAvailableMerchantTags,
  setMyMerchantTags,
  MerchantCategoryTag
} from '../../../api/merchant'

interface TagItem extends MerchantCategoryTag {
  selected: boolean
}

function getErrorMessage(error: unknown, fallback: string) {
  if (error && typeof error === 'object') {
    const knownError = error as { userMessage?: string, message?: string }
    return knownError.userMessage || knownError.message || fallback
  }

  return fallback
}

function buildTagItems(allTags: MerchantCategoryTag[], selectedTags: MerchantCategoryTag[]) {
  const selectedIds = new Set((selectedTags || []).map((tag) => tag.id))

  return {
    tags: (allTags || []).map((tag) => ({
      ...tag,
      selected: selectedIds.has(tag.id)
    })),
    selectedCount: selectedIds.size,
    persistedTagIds: [...selectedIds]
  }
}

function hasSelectionChanged(currentTags: TagItem[], persistedTagIds: number[]) {
  const currentSelectedIds = currentTags
    .filter((tag) => tag.selected)
    .map((tag) => tag.id)
    .sort((left, right) => left - right)
  const lastSavedIds = [...persistedTagIds].sort((left, right) => left - right)

  if (currentSelectedIds.length !== lastSavedIds.length) {
    return true
  }

  return currentSelectedIds.some((id, index) => id !== lastSavedIds[index])
}

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    initialError: false,
    initialErrorMessage: '',
    saving: false,
    tags: [] as TagItem[],
    selectedCount: 0,
    persistedTagIds: [] as number[],
    hasChanges: false
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.loadData()
  },

  onPullDownRefresh() {
    this.loadData()
  },

  async loadData() {
    if (this.data.loading) {
      return
    }

    this.setData({
      loading: true,
      initialError: false,
      initialErrorMessage: ''
    })
    try {
      const [currentRes, allRes] = await Promise.all([
        getMyMerchantTags(),
        getAvailableMerchantTags()
      ])

      const nextState = buildTagItems(allRes.tags || [], currentRes.tags || [])

      this.setData({
        ...nextState,
        hasChanges: false
      })
    } catch (err) {
      logger.error('[MerchantCategories] 加载失败', err)
      this.setData({
        initialError: true,
        initialErrorMessage: getErrorMessage(err, '经营类目加载失败，请重试')
      })
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onTagTap(e: WechatMiniprogram.TouchEvent) {
    if (this.data.saving || this.data.loading) {
      return
    }

    const index = e.currentTarget.dataset.index as number
    const tags = [...this.data.tags]
    const tag = tags[index]

    if (!tag) {
      return
    }

    if (tag.selected) {
      tag.selected = false
    } else {
      if (this.data.selectedCount >= 5) {
        wx.showToast({ title: '最多选 5 个类目', icon: 'none' })
        return
      }
      tag.selected = true
    }

    const selectedCount = tags.filter((t) => t.selected).length
    this.setData({
      tags,
      selectedCount,
      hasChanges: hasSelectionChanged(tags, this.data.persistedTagIds)
    })
  },

  async onSave() {
    if (this.data.saving || this.data.loading || !this.data.hasChanges) {
      return
    }

    const selectedIds = this.data.tags
      .filter((t) => t.selected)
      .map((t) => t.id)

    if (selectedIds.length === 0) {
      wx.showModal({
        title: '确认清除类目？',
        content: '未选择任何类目将导致您的店铺不出现在任何分类筛选下，继续吗？',
        confirmText: '确认清除',
        cancelText: '取消',
        success: (res) => {
          if (res.confirm) this.doSave(selectedIds)
        }
      })
      return
    }

    await this.doSave(selectedIds)
  },

  async doSave(ids: number[]) {
    this.setData({ saving: true })
    try {
      const response = await setMyMerchantTags(ids)
      const nextState = buildTagItems(this.data.tags, response.tags || [])

      this.setData({
        ...nextState,
        hasChanges: false
      })
      wx.showToast({ title: '经营类目已保存', icon: 'success' })
    } catch (err) {
      logger.error('[MerchantCategories] 保存失败', err)
      wx.showToast({
        title: getErrorMessage(err, '保存失败，请稍后重试'),
        icon: 'none'
      })
    } finally {
      this.setData({ saving: false })
    }
  },

  onRetry() {
    this.loadData()
  }
})
