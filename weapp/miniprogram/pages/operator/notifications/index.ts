import { isLargeScreen } from '@/utils/responsive'
import {
  loadOperatorNotificationListPageData,
  markAllOperatorNotificationsAsRead,
  markOperatorNotificationAsRead,
  type OperatorNotificationFilterCategory,
  type OperatorNotificationView
} from '../../../services/operator-notification-center'
import { getErrorUserMessage } from '../../../utils/user-facing'

type NotificationListOptions = {
  category?: string
}

Page({
  data: {
    navBarHeight: 88,
    isLargeScreen: false,
    loading: false,
    loadingMore: false,
    refreshing: false,
    initialLoading: true,
    error: '',
    page: 1,
    limit: 20,
    total: 0,
    hasMore: true,
    unreadCount: 0,
    activeCategory: '' as OperatorNotificationFilterCategory,
    notifications: [] as OperatorNotificationView[],
    tabs: [
      { label: '全部', value: '' },
      { label: '待接单', value: 'dispatch_timeout' }
    ]
  },

  onLoad(options: NotificationListOptions) {
    const activeCategory = options.category === 'dispatch_timeout' ? 'dispatch_timeout' : ''
    this.setData({ isLargeScreen: isLargeScreen(), activeCategory })
    this.loadNotifications(true)
  },

  onShow() {
    if (!this.data.initialLoading) {
      this.loadNotifications(true, true)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onPullDownRefresh() {
    this.setData({ refreshing: true })
    this.loadNotifications(true).finally(() => {
      this.setData({ refreshing: false })
      wx.stopPullDownRefresh()
    })
  },

  async loadNotifications(refresh: boolean, silent = false) {
    if (this.data.loading || (this.data.loadingMore && !refresh)) return

    try {
      if (refresh) {
        this.setData({
          loading: true,
          error: '',
          page: 1,
          ...(silent ? {} : { initialLoading: this.data.initialLoading })
        })
      } else {
        this.setData({ loadingMore: true })
      }

      const page = refresh ? 1 : this.data.page
      const result = await loadOperatorNotificationListPageData({
        pageId: page,
        pageSize: this.data.limit,
        category: this.data.activeCategory || undefined,
        includeSummary: refresh,
        fallbackUnreadCount: this.data.unreadCount
      })

      const notifications = refresh ? result.notifications : [...this.data.notifications, ...result.notifications]
      const total = refresh ? result.total : Number(result.total || notifications.length)

      this.setData({
        notifications,
        unreadCount: result.unreadCount,
        page: refresh ? result.nextPage : this.data.page + 1,
        total,
        hasMore: notifications.length < total,
        loading: false,
        loadingMore: false,
        initialLoading: false
      })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '加载待接单提醒失败，请稍后重试')
      this.setData({ loading: false, loadingMore: false, initialLoading: false, error: message })
    }
  },

  onRetry() {
    this.loadNotifications(true)
  },

  onLoadMore() {
    if (this.data.hasMore && !this.data.loading && !this.data.loadingMore) {
      this.loadNotifications(false)
    }
  },

  onCategoryChange(e: WechatMiniprogram.CustomEvent<{ value: OperatorNotificationFilterCategory }>) {
    this.setData({ activeCategory: e.detail.value })
    this.loadNotifications(true)
  },

  async onMarkAllRead() {
    if (this.data.unreadCount === 0) {
      return
    }

    try {
      await markAllOperatorNotificationsAsRead()
      this.setData({
        notifications: this.data.notifications.map((item) => ({ ...item, is_read: true })),
        unreadCount: 0
      })
    } catch (error) {
      wx.showToast({ title: getErrorUserMessage(error, '操作失败，请稍后重试'), icon: 'none' })
    }
  },

  async onTapNotification(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) {
      return
    }

    const current = this.data.notifications.find((item) => item.id === id)
    if (current && !current.is_read) {
      try {
        await markOperatorNotificationAsRead(id)
        this.setData({
          notifications: this.data.notifications.map((item) => item.id === id ? { ...item, is_read: true } : item),
          unreadCount: Math.max(0, this.data.unreadCount - 1)
        })
      } catch (_error) {
        // 已读失败不阻断进入详情
      }
    }

    wx.navigateTo({ url: `/pages/operator/notifications/detail/index?id=${id}` })
  }
})