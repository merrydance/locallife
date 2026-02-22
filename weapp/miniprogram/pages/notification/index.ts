import { notificationService, Notification, NotificationType } from '../../api/notification'

const TYPE_ICON_MAP: Record<string, string> = {
  order: 'order-adjustment',
  payment: 'wallet',
  delivery: 'cart',
  system: 'notification',
  food_safety: 'check-circle'
}

const TYPE_CLASS_MAP: Record<string, string> = {
  order: 'icon-orange',
  payment: 'icon-green',
  delivery: 'icon-blue',
  system: 'icon-gray',
  food_safety: 'icon-red'
}

const TYPE_TABS = [
  { label: '全部', value: '' },
  { label: '订单', value: 'order' },
  { label: '配送', value: 'delivery' },
  { label: '支付', value: 'payment' },
  { label: '系统', value: 'system' }
]

const RELATED_TYPE_URL_MAP: Record<string, string> = {
  order: '/pages/orders/detail/index?id=',
  delivery: '/pages/orders/tracking/index?orderId='
}

interface NotifView extends Notification {
  typeIcon: string
  typeClass: string
  timeDisplay: string
}

function formatTime(isoStr: string): string {
  const date = new Date(isoStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMins = Math.floor(diffMs / 60000)
  if (diffMins < 1) return '刚刚'
  if (diffMins < 60) return `${diffMins}分钟前`
  const diffHours = Math.floor(diffMins / 60)
  if (diffHours < 24) return `${diffHours}小时前`
  const mm = String(date.getMonth() + 1).padStart(2, '0')
  const dd = String(date.getDate()).padStart(2, '0')
  return `${mm}-${dd}`
}

Page({
  data: {
    navBarHeight: 88,
    activeType: '' as NotificationType | '',
    typeTabs: TYPE_TABS,
    notifications: [] as NotifView[],
    unreadCount: 0,
    initialLoading: true,
    loading: false,
    refreshing: false,
    hasMore: true,
    page: 1
  },

  onLoad() {
    this.loadNotifications(true)
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onTypeChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ activeType: e.detail.value, page: 1 })
    this.loadNotifications(true)
  },

  async loadNotifications(reset = false) {
    if (this.data.loading && !reset) return
    this.setData({ loading: true })

    try {
      const page = reset ? 1 : this.data.page
      const type = this.data.activeType as NotificationType | undefined
      const res = await notificationService.getNotifications({
        page_id: page,
        page_size: 20,
        type: type || undefined
      })

      const notifs: NotifView[] = (res.notifications || []).map((n: Notification) => ({
        ...n,
        typeIcon: TYPE_ICON_MAP[n.type] || 'notification',
        typeClass: TYPE_CLASS_MAP[n.type] || 'icon-gray',
        timeDisplay: formatTime(n.created_at)
      }))

      const unreadCount = notifs.filter((n) => !n.is_read).length

      this.setData({
        notifications: reset ? notifs : [...this.data.notifications, ...notifs],
        hasMore: notifs.length === 20,
        page: page + 1,
        unreadCount: reset ? unreadCount : this.data.unreadCount,
        initialLoading: false,
        loading: false,
        refreshing: false
      })
    } catch (err) {
      console.error('加载通知失败', err)
      this.setData({ initialLoading: false, loading: false, refreshing: false })
      wx.showToast({ title: '加载失败，请重试', icon: 'none' })
    }
  },

  async onNotifTap(e: WechatMiniprogram.CustomEvent) {
    const item: NotifView = e.currentTarget.dataset.item

    // 标记为已读
    if (!item.is_read) {
      try {
        await notificationService.markAsRead(item.id)
        const notifs = this.data.notifications.map((n) =>
          n.id === item.id ? { ...n, is_read: true } : n
        )
        this.setData({
          notifications: notifs,
          unreadCount: Math.max(0, this.data.unreadCount - 1)
        })
      } catch (err) {
        console.warn('标记已读失败', err)
      }
    }

    // 跳转关联页
    if (item.related_type && item.related_id) {
      const base = RELATED_TYPE_URL_MAP[item.related_type]
      if (base) {
        wx.navigateTo({ url: `${base}${item.related_id}` })
      }
    }
  },

  async onMarkAllRead() {
    try {
      await notificationService.markAllAsRead()
      const notifs = this.data.notifications.map((n) => ({ ...n, is_read: true }))
      this.setData({ notifications: notifs, unreadCount: 0 })
      wx.showToast({ title: '已全部标记为已读', icon: 'success' })
    } catch (err) {
      wx.showToast({ title: '操作失败', icon: 'error' })
    }
  },

  onReachBottom() {
    if (this.data.hasMore && !this.data.loading) {
      this.loadNotifications(false)
    }
  },

  onPullRefresh() {
    this.setData({ refreshing: true, page: 1 })
    this.loadNotifications(true)
  }
})
