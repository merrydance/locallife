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

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    saving: false,
    tags: [] as TagItem[],
    selectedCount: 0
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.loadData()
  },

  async loadData() {
    this.setData({ loading: true })
    try {
      const [currentRes, allRes] = await Promise.all([
        getMyMerchantTags(),
        getAvailableMerchantTags()
      ])

      const selectedIds = new Set((currentRes.tags || []).map((t) => t.id))
      const tags: TagItem[] = (allRes.tags || []).map((t) => ({
        ...t,
        selected: selectedIds.has(t.id)
      }))

      this.setData({
        tags,
        selectedCount: selectedIds.size
      })
    } catch (err) {
      logger.error('[MerchantCategories] 加载失败', err)
      wx.showToast({ title: '加载失败', icon: 'error' })
    } finally {
      this.setData({ loading: false })
    }
  },

  onTagTap(e: WechatMiniprogram.TouchEvent) {
    const index = e.currentTarget.dataset.index as number
    const tags = [...this.data.tags]
    const tag = tags[index]

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
    this.setData({ tags, selectedCount })
  },

  async onSave() {
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
      await setMyMerchantTags(ids)
      wx.showToast({ title: '经营类目已保存', icon: 'success' })
    } catch (err) {
      logger.error('[MerchantCategories] 保存失败', err)
      wx.showToast({ title: '保存失败', icon: 'error' })
    } finally {
      this.setData({ saving: false })
    }
  }
})
