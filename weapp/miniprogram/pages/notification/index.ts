import { notificationService, Notification, NotificationType } from '../../api/notification'

type NotificationCategory = '' | 'orders' | 'income' | 'deposit' | 'claims'

interface NotificationPageOptions {
  mode?: string
  category?: NotificationCategory
}

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

const RIDER_TYPE_TABS: Array<{ label: string, value: NotificationCategory }> = [
  { label: '全部', value: '' },
  { label: '订单', value: 'orders' },
  { label: '收入', value: 'income' },
  { label: '押金', value: 'deposit' },
  { label: '追偿', value: 'claims' }
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

type NotificationPageResult = Awaited<ReturnType<typeof notificationService.getNotifications>>

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

function getNotificationSearchText(item: Notification): string {
  return `${item.title || ''} ${item.content || ''} ${item.related_type || ''}`.toLowerCase()
}

function getExtraNumber(item: Notification, key: string): number {
  const value = item.extra_data?.[key]
  const numberValue = typeof value === 'number' ? value : Number(value || 0)
  return Number.isFinite(numberValue) ? numberValue : 0
}

function getRiderNotificationCategory(item: Notification): NotificationCategory {
  const relatedType = String(item.related_type || '').toLowerCase()
  const text = getNotificationSearchText(item)
  const extra = item.extra_data || {}

  if (relatedType.includes('claim') || relatedType.includes('appeal') || extra.claim_id || extra.recovery_id || extra.appeal_id || text.includes('追偿') || text.includes('申诉') || text.includes('索赔')) {
    return 'claims'
  }

  if (relatedType.includes('deposit') || extra.credit_id || extra.deposit_id || text.includes('押金')) {
    return 'deposit'
  }

  if (relatedType.includes('income') || relatedType.includes('profit') || extra.profit_sharing_order_id || text.includes('分账') || text.includes('结算') || text.includes('配送费')) {
    return 'income'
  }

  if (item.type === 'order' || item.type === 'delivery' || relatedType.includes('order') || relatedType.includes('delivery')) {
    return 'orders'
  }

  return ''
}

function matchesRiderCategory(item: Notification, category: NotificationCategory): boolean {
  return !category || getRiderNotificationCategory(item) === category
}

function resolveRiderNotificationUrl(item: Notification): string {
  const relatedType = String(item.related_type || '').toLowerCase()
  const category = getRiderNotificationCategory(item)

  if (category === 'claims') {
    const claimID = relatedType === 'claim' || relatedType === 'rider_claim' ? item.related_id : getExtraNumber(item, 'claim_id')
    return claimID ? `/pages/rider/claims/detail/index?id=${claimID}` : '/pages/rider/claims/index'
  }

  if (category === 'deposit') {
    return '/pages/rider/deposit/index'
  }

  if (category === 'income') {
    return '/pages/rider/income/index'
  }

  if (category === 'orders') {
    const orderID = relatedType.includes('order') ? item.related_id : getExtraNumber(item, 'order_id')
    return orderID ? `/pages/rider/task-detail/index?id=${orderID}` : '/pages/rider/tasks/index'
  }

  if (relatedType === 'rider') {
    return '/pages/rider/dashboard/index'
  }

  return ''
}

Page({
  data: {
    navBarHeight: 88,
    activeType: '' as NotificationType | '',
    activeCategory: '' as NotificationCategory,
    typeTabs: TYPE_TABS,
    isRiderMode: false,
    notifications: [] as NotifView[],
    unreadCount: 0,
    initialLoading: true,
    loading: false,
    refreshing: false,
    hasMore: true,
    page: 1,
    total: 0
  },

  onLoad(options: NotificationPageOptions = {}) {
    const isRiderMode = options.mode === 'rider'
    this.setData({
      isRiderMode,
      typeTabs: isRiderMode ? RIDER_TYPE_TABS : TYPE_TABS,
      activeCategory: isRiderMode ? (options.category || '') : '',
      activeType: isRiderMode ? '' : (options.category as NotificationType || '')
    })
    this.loadNotifications(true)
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onTypeChange(e: WechatMiniprogram.CustomEvent) {
    if (this.data.isRiderMode) {
      this.setData({ activeCategory: e.detail.value as NotificationCategory, page: 1 })
    } else {
      this.setData({ activeType: e.detail.value, page: 1 })
    }
    this.loadNotifications(true)
  },

  async loadNotifications(reset = false) {
    if (this.data.loading && !reset) return
    this.setData({ loading: true })

    try {
      const page = reset ? 1 : this.data.page
      const type = this.data.activeType as NotificationType | undefined
      const unreadCountPromise = reset
        ? notificationService.getUnreadCount().catch(() => ({ count: this.data.unreadCount }))
        : Promise.resolve({ count: this.data.unreadCount })

      let res: NotificationPageResult
      let sourceNotifications: Notification[] = []

      if (this.data.isRiderMode) {
        let nextPage = page
        let lastResult: NotificationPageResult | null = null
        const activeCategory = this.data.activeCategory as NotificationCategory
        let shouldContinue = true

        while (shouldContinue) {
          lastResult = await notificationService.getNotifications({
            page_id: nextPage,
            page_size: 20
          })
          sourceNotifications = sourceNotifications.concat((lastResult.notifications || []).filter((n: Notification) => matchesRiderCategory(n, activeCategory)))
          nextPage = lastResult.page + 1

          shouldContinue = Boolean(activeCategory && sourceNotifications.length === 0 && lastResult.hasMore)
        }

        res = lastResult || {
          items: [],
          notifications: [],
          total: 0,
          page,
          pageSize: 20,
          hasMore: false
        }
      } else {
        res = await notificationService.getNotifications({
          page_id: page,
          page_size: 20,
          type: type || undefined
        })
        sourceNotifications = res.notifications || []
      }

      const unreadResult = await unreadCountPromise

      const notifs: NotifView[] = sourceNotifications.map((n: Notification) => ({
        ...n,
        typeIcon: TYPE_ICON_MAP[n.type] || 'notification',
        typeClass: TYPE_CLASS_MAP[n.type] || 'icon-gray',
        timeDisplay: formatTime(n.created_at)
      }))
      const notifications = reset ? notifs : [...this.data.notifications, ...notifs]
      const total = typeof res.total === 'number' ? res.total : notifications.length

      this.setData({
        notifications,
        hasMore: res.hasMore,
        page: res.page + 1,
        total,
        unreadCount: unreadResult.count,
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
      const riderUrl = this.data.isRiderMode ? resolveRiderNotificationUrl(item) : ''
      if (riderUrl) {
        wx.navigateTo({ url: riderUrl })
        return
      }

      const base = RELATED_TYPE_URL_MAP[item.related_type]
      if (base) {
        wx.navigateTo({ url: `${base}${item.related_id}` })
      }
    } else if (this.data.isRiderMode) {
      const riderUrl = resolveRiderNotificationUrl(item)
      if (riderUrl) {
        wx.navigateTo({ url: riderUrl })
      }
    }
  },

  async onMarkAllRead() {
    try {
      await notificationService.markAllAsRead()
      const notifs = this.data.notifications.map((n) => ({ ...n, is_read: true }))
      this.setData({ notifications: notifs, unreadCount: 0 })
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
