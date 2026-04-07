import { TagService, type TagInfo } from '../../../../api/dish'
import { getStableBarHeights } from '../../../../utils/responsive'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'

function normalizeTags(tags: TagInfo[]): TagInfo[] {
  return Array.isArray(tags)
    ? tags.filter((tag): tag is TagInfo => !!tag && Number.isFinite(tag.id) && tag.id > 0)
    : []
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
    tags: [] as TagInfo[]
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
      const tags = normalizeTags(await TagService.listTags('table'))
      this.setData({
        bootstrapped: true,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        tags
      })
    } catch (err) {
      logger.error('Load table tags page failed', err)
      const message = getErrorUserMessage(err, '桌台标签加载失败，请重试')

      if (showInitialState || !this.data.tags.length) {
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

  hasTagName(name: string): boolean {
    return this.data.tags.some((tag) => tag.name.trim() === name.trim())
  },

  onCreateTag() {
    if (this.data.submitting) {
      return
    }

    wx.showModal({
      title: '新增桌台标签',
      editable: true,
      placeholderText: '请输入标签名称',
      success: async (res) => {
        if (!res.confirm) {
          return
        }

        const name = (res.content || '').trim()
        if (!name) {
          wx.showToast({ title: '标签名称不能为空', icon: 'none' })
          return
        }

        if (this.hasTagName(name)) {
          wx.showToast({ title: '标签名称已存在', icon: 'none' })
          return
        }

        this.setData({ submitting: true })
        try {
          await TagService.createTag({ name, type: 'table' })
          await this.loadData(false)
        } catch (err) {
          logger.error('Create table tag failed', err)
          wx.showToast({ title: getErrorUserMessage(err, '创建标签失败，请稍后重试'), icon: 'none' })
        } finally {
          this.setData({ submitting: false })
        }
      }
    })
  },

  onDeleteTag(e: WechatMiniprogram.TouchEvent) {
    if (this.data.submitting) {
      return
    }

    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) {
      return
    }

    wx.showModal({
      title: '删除桌台标签',
      content: '删除后该标签将无法继续分配给桌台。确认删除？',
      success: async (res) => {
        if (!res.confirm) {
          return
        }

        this.setData({ pendingDeleteId: id })
        try {
          await TagService.deleteTag(id)
          await this.loadData(false)
        } catch (err) {
          logger.error('Delete table tag failed', err)
          wx.showToast({ title: getErrorUserMessage(err, '删除标签失败，请稍后重试'), icon: 'none' })
        } finally {
          this.setData({ pendingDeleteId: 0 })
        }
      }
    })
  }
})
