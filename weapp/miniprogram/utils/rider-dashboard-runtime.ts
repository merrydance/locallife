import RiderService, { RiderInfo, RiderStatus } from '../api/rider'
import RiderWorkbenchService from '../api/rider-workbench'
import DeliveryService, { RecommendedOrder, Delivery, getDeliveryStatusDisplay } from '../api/delivery'
import { deliveryTaskManagementService } from '../api/delivery-task-management'
import { buildRiderWorkbenchDashboardView } from '../services/rider-workbench'
import { logger } from './logger'
import { locationService } from './location'
import { normalizeLocationError, syncRiderDeliveryLocation } from './rider-location'
import { getRiderLocationStatusView } from './rider-location-status-view'
import { riderLiveLocationSession, RiderLiveLocationState } from './rider-live-location'
import { getStableBarHeights } from './responsive'
import { resolveStatusTagTheme, type StatusTagTheme } from './status-tag'
import { wsManager, WSMessageType } from './websocket'
import { networkMonitor } from './network-monitor'
import { getConsoleDashboardErrorMessage, getConsoleDashboardErrorState } from './console-dashboard'

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type RiderDashboardPageContext = WechatMiniprogram.Page.Instance<Record<string, any>, Record<string, any>> & Record<string, any>

const MAX_GRAB_DISTANCE = 5000
const DEPOSIT_BLOCK_REASON_PATTERN = /押金不足/

let runtimeNetworkUnsubscribe: null | (() => void) = null
let dashboardLocationUnsubscribe: null | (() => void) = null

export type WsUnsubscribe = () => void
export type DeliveryActionType = 'startPickup' | 'confirmPickup' | 'startDelivery' | 'confirmDelivery'
type DeliveryActionMethod = (deliveryId: number) => Promise<Delivery>
export type TagTheme = StatusTagTheme

export type DashboardDeliveryView = Delivery & {
  status_desc: string
  status_tag_theme: TagTheme
  deadline_desc: string
  is_overdue: boolean
  is_very_urgent: boolean
  is_pickup_finished: boolean
  can_start_pickup: boolean
  can_confirm_pickup: boolean
  can_start_delivery: boolean
  can_confirm_delivery: boolean
  is_action_loading: boolean
}

interface DeliveryActionConfig {
  method: DeliveryActionMethod
  loading: string
  source: string
}

function getUserMessage(err: unknown, fallback: string) {
  return getConsoleDashboardErrorMessage('rider', err, fallback)
}

function withDeliveryActionLoading(
  deliveries: DashboardDeliveryView[],
  deliveryId: number,
  isLoading: boolean
) {
  return deliveries.map((delivery) => delivery.id === deliveryId
    ? { ...delivery, is_action_loading: isLoading }
    : delivery)
}

function setDeliveryActionLoadingState(
  page: RiderDashboardPageContext,
  deliveryId: number,
  isLoading: boolean
) {
  const loadingIds = new Set<number>(page.data.deliveryActionLoadingIds || [])
  if (isLoading) {
    loadingIds.add(deliveryId)
  } else {
    loadingIds.delete(deliveryId)
  }

  const nextLoadingIds = Array.from(loadingIds)
  const activeDeliveries = withDeliveryActionLoading(page.data.activeDeliveries || [], deliveryId, isLoading)
  const currentDelivery = page.data.currentDelivery?.id === deliveryId
    ? { ...page.data.currentDelivery, is_action_loading: isLoading }
    : page.data.currentDelivery

  page.setData({ deliveryActionLoadingIds: nextLoadingIds, activeDeliveries, currentDelivery })
}

function buildBannerState(states: Array<{ message: string, canRetry: boolean }>) {
  if (!states.length) {
    return { message: '', canRetry: true }
  }

  const uniqueMessages = Array.from(new Set(states.map((state) => state.message).filter(Boolean)))

  return {
    message: uniqueMessages.join('；'),
    canRetry: states.every((state) => state.canRetry)
  }
}

function formatRelativeTime(timeStr: string): string {
  if (!timeStr) return '刚刚'

  const diff = Date.now() - new Date(timeStr).getTime()
  if (!Number.isFinite(diff) || diff < 0) {
    return '刚刚'
  }

  const minutes = Math.floor(diff / 60000)
  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes} 分钟前`

  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} 小时前`

  return `${Math.floor(hours / 24)} 天前`
}

function isTrackableDelivery(status: Delivery['status']): boolean {
  return status === 'assigned' || status === 'picking' || status === 'picked' || status === 'delivering'
}

function buildDashboardDeliveryActionState(status: Delivery['status']) {
  return {
    statusTagTheme: status === 'assigned' || status === 'picking'
      ? resolveStatusTagTheme('warning')
      : resolveStatusTagTheme('success'),
    isPickupFinished: status === 'picked' || status === 'delivering',
    canStartPickup: status === 'assigned',
    canConfirmPickup: status === 'picking',
    canStartDelivery: status === 'picked',
    canConfirmDelivery: status === 'delivering'
  }
}

function getDistance(lat1: number, lng1: number, lat2: number, lng2: number): number {
  const R = 6371e3
  const φ1 = lat1 * Math.PI / 180
  const φ2 = lat2 * Math.PI / 180
  const Δφ = (lat2 - lat1) * Math.PI / 180
  const Δλ = (lng2 - lng1) * Math.PI / 180

  const a = Math.sin(Δφ / 2) * Math.sin(Δφ / 2) +
    Math.cos(φ1) * Math.cos(φ2) *
    Math.sin(Δλ / 2) * Math.sin(Δλ / 2)
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a))

  return R * c
}

export const riderDashboardRuntimeMethods: Record<string, unknown> & ThisType<RiderDashboardPageContext> = {
  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.bindNetworkMonitor()
    this.bindLocationSession()
    this.initData().catch((err: unknown) => logger.error('Load init error', err))
  },

  onShow() {
    this.bindNetworkMonitor()
    if (this.data.loading) return

    if (!this.data.riderInfo || !this.data.riderStatus) {
      this.initData().catch((err: unknown) => logger.error('Show init error', err))
      return
    }

    this.refreshRiderOverview()
      .then((overview: { isOnline: boolean }) => {
        if (overview.isOnline) {
          return this.enterOnlineRuntime()
        }

        this.cleanupWebSocket()
        void this.syncLocationSession([])
        this.setData({
          recommendOrders: [],
          activeDeliveries: [],
          currentDelivery: null,
          hallTabLabel: '抢单大厅 0',
          myTabLabel: '我的任务 0',
          newOrdersCount: 0,
          hallLoadError: '',
          myLoadError: ''
        })
        return undefined
      })
      .catch((err: unknown) => logger.error('Show refresh error', err))
  },

  onHide() {
    this.cleanupWebSocket()
    this.unbindNetworkMonitor()
  },

  onUnload() {
    this.cleanupWebSocket()
    this.unbindNetworkMonitor()
    if (dashboardLocationUnsubscribe) {
      dashboardLocationUnsubscribe()
      dashboardLocationUnsubscribe = null
    }
  },

  bindNetworkMonitor() {
    if (runtimeNetworkUnsubscribe) return

    runtimeNetworkUnsubscribe = networkMonitor.subscribe((state) => {
      if (!state.isConnected) {
        if (this.data.isOnline && this.data.recommendOrders.length === 0 && this.data.activeDeliveries.length === 0) {
          const errorState = getConsoleDashboardErrorState('rider', '网络已断开，请恢复后重试', '网络已断开，请恢复后重试')
          this.setData({
            initError: errorState.message,
            initErrorCanRetry: errorState.canRetry,
            dashboardInlineError: errorState.message,
            hallLoadError: '',
            myLoadError: ''
          })
        }
        return
      }

      if (this.data.loading) return

      if (this.data.initError || !this.data.riderInfo || !this.data.riderStatus) {
        this.initData().catch((err: unknown) => logger.error('Network restore init error', err))
        return
      }

      if (this.data.isOnline) {
        this.refreshRiderOverview()
          .then(() => this.enterOnlineRuntime())
          .catch((err: unknown) => logger.error('Network restore refresh error', err))
      }
    })
  },

  unbindNetworkMonitor() {
    if (runtimeNetworkUnsubscribe) {
      runtimeNetworkUnsubscribe()
      runtimeNetworkUnsubscribe = null
    }
  },

  async initData() {
    this.setData({ loading: true })
    try {
      const overview = await this.refreshRiderOverview()

      if (overview.isOnline) {
        await this.enterOnlineRuntime()
      } else {
        await this.syncLocationSession([])
      }
    } catch (err: unknown) {
      logger.error('Failed to init rider data', err)
      const errorState = getConsoleDashboardErrorState('rider', err, '骑手工作台暂时无法加载，请稍后重试。')
      const statusViewData = this.buildStatusViewData(null, null)
      this.setData({
        initError: errorState.message,
        initErrorCanRetry: errorState.canRetry,
        dashboardInlineError: errorState.message,
        hallLoadError: '',
        myLoadError: '',
        riderInfo: null,
        recommendOrders: [],
        activeDeliveries: [],
        currentDelivery: null,
        hallTabLabel: '抢单大厅 0',
        myTabLabel: '我的任务 0',
        workbenchRefreshError: '',
        locationDeliveryId: 0,
        locationStatusText: '',
        locationPendingText: '',
        locationUpdatedText: '',
        locationNeedsPermission: false,
        ...statusViewData
      })
      await this.syncLocationSession([])
    } finally {
      this.setData({ loading: false })
    }
  },

  bindLocationSession() {
    if (dashboardLocationUnsubscribe) {
      dashboardLocationUnsubscribe()
    }

    dashboardLocationUnsubscribe = riderLiveLocationSession.subscribe((state) => {
      this.applyLocationSessionState(state)
    })
  },

  getDepositReminderText(status: RiderStatus | null, riderInfo?: RiderInfo | null) {
    const currentRiderInfo = riderInfo === undefined ? this.data.riderInfo : riderInfo

    if (!status || status.is_online || status.can_go_online) return ''

    const reason = status.online_block_reason || ''
    if (!DEPOSIT_BLOCK_REASON_PATTERN.test(reason)) return ''

    if (currentRiderInfo?.status === 'pending' || currentRiderInfo?.status === 'rejected' || currentRiderInfo?.status === 'suspended') {
      return ''
    }

    return reason || '可用押金不足，请先补足押金后再尝试上线'
  },

  buildStatusViewData(status: RiderStatus | null, riderInfo?: RiderInfo | null) {
    const currentRiderInfo = riderInfo === undefined ? this.data.riderInfo : riderInfo
    const isOnline = !!status?.is_online
    const depositReminderText = this.getDepositReminderText(status, currentRiderInfo)

    return {
      riderStatus: status,
      isOnline,
      onlineStatusLabel: isOnline ? '在线接单' : '休息中',
      onlineStatusTheme: isOnline ? 'success' : 'default',
      statusText: isOnline ? '正在接单中' : (depositReminderText ? '暂不能上线' : '休息中 - 点击上线'),
      showDepositReminder: !!depositReminderText,
      depositReminderText
    }
  },

  buildTabLabels(hallCount: number, myCount: number) {
    return {
      hallTabLabel: `抢单大厅 ${hallCount}`,
      myTabLabel: `我的任务 ${myCount}`
    }
  },

  async refreshRiderOverview() {
    const [info, workbenchSummary] = await Promise.all([
      RiderService.getMe(),
      RiderWorkbenchService.getSummary()
    ])

    const workbench = buildRiderWorkbenchDashboardView(workbenchSummary)
    const status = workbench.riderStatus
    const statusViewData = this.buildStatusViewData(status, info)
    const currentDelivery = this.data.currentDelivery || null
    const tabLabels = this.buildTabLabels(workbench.availableOrderCount, workbench.activeDeliveryCount)

    this.setData({
      riderInfo: info,
      initError: '',
      initErrorCanRetry: true,
      workbenchRefreshError: workbench.unavailableText,
      dashboardInlineError: workbench.unavailableText,
      workbench,
      currentDelivery,
      stats: {
        todayCount: workbench.todayCompletedDeliveries,
        todayEarnings: workbenchSummary.income?.total_rider_income || 0,
        creditScore: info.credit_score || 0
      },
      ...tabLabels,
      ...statusViewData
    })

    return {
      info,
      status,
      workbench,
      ...statusViewData
    }
  },

  async refreshData() {
    if (!this.data.isOnline) return

    try {
      const activeDeliveriesRequest = deliveryTaskManagementService.getActiveDeliveries() as Promise<Delivery[]>

      let hallLoadError = ''
      const bannerStates: Array<{ message: string, canRetry: boolean }> = []
      const location = await this.getLocation().catch(() => null)
      const hallOrdersRequest = location
        ? DeliveryService.getRecommendedOrders(location.longitude, location.latitude)
        : Promise.resolve([] as RecommendedOrder[])

      if (!location) {
        const errorState = getConsoleDashboardErrorState('rider', '暂时无法获取当前位置，抢单大厅稍后再试', '暂时无法获取当前位置，抢单大厅稍后再试')
        hallLoadError = errorState.message
        bannerStates.push({ message: errorState.message, canRetry: errorState.canRetry })
      }

      const [hallOrdersResult, myDeliveriesResult] = await Promise.all([
        hallOrdersRequest.then(
          (value) => ({ ok: true as const, value }),
          (reason) => ({ ok: false as const, reason })
        ),
        activeDeliveriesRequest.then(
          (value) => ({ ok: true as const, value }),
          (reason) => ({ ok: false as const, reason })
        )
      ])

      const hallOrders = hallOrdersResult.ok
        ? (hallOrdersResult.value || []).map((o: RecommendedOrder) => ({
            ...o,
            expires_at_format: this.formatExpiry(o.expires_at),
            distance_km: (o.distance / 1000).toFixed(1),
            pickup_distance_km: (o.distance_to_pickup / 1000).toFixed(1),
            route_distance_km: (((o.real_distance || o.distance_to_pickup || o.distance) || 0) / 1000).toFixed(1)
          }))
        : []

      if (!hallOrdersResult.ok && !hallLoadError) {
        const errorState = getConsoleDashboardErrorState('rider', hallOrdersResult.reason, '抢单大厅加载失败，请重试')
        hallLoadError = errorState.message
        bannerStates.push({ message: errorState.message, canRetry: errorState.canRetry })
      }

      let myLoadError = ''
      const myDeliveriesSource = myDeliveriesResult.ok
        ? (myDeliveriesResult.value || [])
        : this.data.activeDeliveries

      if (!myDeliveriesResult.ok) {
        const errorState = getConsoleDashboardErrorState('rider', myDeliveriesResult.reason, '我的任务加载失败，请重试')
        myLoadError = errorState.message
        bannerStates.push({ message: errorState.message, canRetry: errorState.canRetry })
      }

      const myDeliveries = myDeliveriesSource.map((d: Delivery) => {
        const isOverdue = d.estimated_delivery_at ? new Date(d.estimated_delivery_at).getTime() < Date.now() : false
        const deadline = d.status === 'assigned' || d.status === 'picking' ? d.estimated_pickup_at : d.estimated_delivery_at
        const statusDisplay = getDeliveryStatusDisplay(d.status)
        const actionState = buildDashboardDeliveryActionState(d.status)

        return {
          ...d,
          status_desc: statusDisplay.text || this.getStatusDesc(d.status),
          status_tag_theme: actionState.statusTagTheme,
          deadline_desc: this.formatDeadline(deadline),
          is_overdue: isOverdue,
          is_very_urgent: !isOverdue && deadline ? (new Date(deadline).getTime() - Date.now() < 15 * 60 * 1000) : false,
          is_pickup_finished: actionState.isPickupFinished,
          can_start_pickup: actionState.canStartPickup,
          can_confirm_pickup: actionState.canConfirmPickup,
          can_start_delivery: actionState.canStartDelivery,
          can_confirm_delivery: actionState.canConfirmDelivery,
          is_action_loading: (this.data.deliveryActionLoadingIds || []).includes(d.id)
        }
      }) as DashboardDeliveryView[]

      const currentDelivery = myDeliveries.find((delivery) => isTrackableDelivery(delivery.status)) || myDeliveries[0] || null
      const tabLabels = this.buildTabLabels(hallOrders.length, myDeliveries.length)
      const bannerState = buildBannerState(bannerStates)

      this.setData({
        recommendOrders: hallOrders,
        activeDeliveries: myDeliveries,
        currentDelivery,
        isRefresherTriggered: false,
        initError: bannerState.message,
        initErrorCanRetry: bannerState.canRetry,
        dashboardInlineError: bannerState.message || this.data.workbenchRefreshError,
        hallLoadError,
        myLoadError,
        ...tabLabels
      })

      await this.syncLocationSession(myDeliveries)
    } catch (err: unknown) {
      logger.error('Refresh data error', err)
      const errorState = getConsoleDashboardErrorState('rider', err, '任务数据加载失败，请重试')
      this.setData({
        isRefresherTriggered: false,
        initError: errorState.message,
        initErrorCanRetry: errorState.canRetry,
        dashboardInlineError: errorState.message,
        hallLoadError: '',
        myLoadError: ''
      })
    }
  },

  applyLocationSessionState(state: RiderLiveLocationState) {
    const trackedDelivery = this.data.activeDeliveries.find((delivery: Delivery) => delivery.id === state.activeDeliveryId)

    if (!trackedDelivery) {
      this.setData({
        locationDeliveryId: 0,
        locationStatusText: '',
        locationStatusTheme: resolveStatusTagTheme('neutral'),
        locationPendingText: '',
        locationUpdatedText: '',
        locationNeedsPermission: false
      })
      return
    }

    const fallbackUpdatedAt = trackedDelivery.location_updated_at || ''
    const locationUpdatedText = state.lastUploadedAt
      ? `最近上传 ${formatRelativeTime(state.lastUploadedAt)}`
      : (fallbackUpdatedAt ? `最近上传 ${formatRelativeTime(fallbackUpdatedAt)}` : '暂未上传定位')

    const locationPendingText = state.pendingCount > 0
      ? `待补发 ${state.pendingCount} 个定位点`
      : ''

    const locationStatusView = getRiderLocationStatusView(state.uploadState)

    this.setData({
      locationDeliveryId: trackedDelivery.id,
      locationStatusText: locationStatusView.text,
      locationStatusTheme: locationStatusView.theme,
      locationPendingText,
      locationUpdatedText,
      locationNeedsPermission: locationStatusView.needsPermission
    })
  },

  async syncLocationSession(deliveries: Delivery[]) {
    if (!this.data.isOnline) {
      await riderLiveLocationSession.setActiveDelivery(null)
      return
    }

    const trackedDelivery = deliveries.find((delivery) => isTrackableDelivery(delivery.status))
    if (!trackedDelivery) {
      await riderLiveLocationSession.setActiveDelivery(null)
      return
    }

    await riderLiveLocationSession.setActiveDelivery(trackedDelivery.id, 'rider_dashboard_runtime')
  },

  getStatusDesc(status: string) {
    const map: Record<string, string> = {
      assigned: '前往商家',
      picking: '取餐中',
      picked: '准备配送',
      delivering: '配送中',
      completed: '已送达',
      exception: '订单异常'
    }
    return map[status] || status
  },

  formatDeadline(timeStr?: string) {
    if (!timeStr) return ''
    const date = new Date(timeStr)
    const now = new Date()
    const diff = date.getTime() - now.getTime()

    if (diff < 0) return '已超时'

    const h = date.getHours().toString().padStart(2, '0')
    const m = date.getMinutes().toString().padStart(2, '0')

    if (diff < 60 * 60 * 1000) {
      return `剩 ${Math.floor(diff / 60000)} 分钟 (${h}:${m})`
    }
    return `${h}:${m} 前`
  },

  formatExpiry(expiresAt: string) {
    const diff = new Date(expiresAt).getTime() - Date.now()
    if (diff <= 0) return '即将消失'
    return `剩 ${Math.ceil(diff / 60000)} 分钟`
  },

  onCall(e: WechatMiniprogram.TouchEvent) {
    const { phone } = e.currentTarget.dataset as { phone?: string }
    if (!phone) return
    wx.makePhoneCall({ phoneNumber: phone })
  },

  async onOpenLocation(e: WechatMiniprogram.TouchEvent) {
    const {
      latitude,
      longitude,
      name,
      address,
      label
    } = e.currentTarget.dataset as {
      latitude?: number
      longitude?: number
      name?: string
      address?: string
      label?: string
    }

    await locationService.openLocation({
      latitude,
      longitude,
      name,
      address,
      failMessage: `打开${label || '导航'}失败，请稍后重试`
    })
  },

  async onUpdateStatus(e: WechatMiniprogram.TouchEvent) {
    const { id, action } = e.currentTarget.dataset as { id?: number, action?: DeliveryActionType }
    if (!id || !action) return
    if ((this.data.deliveryActionLoadingIds || []).includes(id)) return

    const delivery = this.data.activeDeliveries.find((item: DashboardDeliveryView) => item.id === id)
    if (delivery?.is_action_loading) return

    const actionMap: Record<DeliveryActionType, DeliveryActionConfig> = {
      startPickup: { method: DeliveryService.startPickup, loading: '正在操作...', source: 'rider_dashboard_start_pickup' },
      confirmPickup: { method: DeliveryService.confirmPickup, loading: '确认取餐中...', source: 'rider_dashboard_confirm_pickup' },
      startDelivery: { method: DeliveryService.startDelivery, loading: '开始配送...', source: 'rider_dashboard_start_delivery' },
      confirmDelivery: { method: DeliveryService.confirmDelivery, loading: '确认送达中...', source: 'rider_dashboard_confirm_delivery' }
    }

    const config = actionMap[action]
    if (!config) return

    setDeliveryActionLoadingState(this, id, true)
    try {
      await this.syncDeliveryLocation(id, config.source)
      await config.method(id)
      await this.refreshRiderOverview()
      await this.refreshData()
    } catch (err: unknown) {
      const reconciled = await this.reconcileDeliveryAction(id, action)
      if (reconciled) {
        return
      }

      const message = getUserMessage(err, '操作失败')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      setDeliveryActionLoadingState(this, id, false)
    }
  },

  onGoToHistory() {
    wx.navigateTo({ url: '/pages/rider/tasks/index' })
  },

  onGoToClaims() {
    wx.navigateTo({ url: '/pages/rider/claims/index' })
  },

  onGoToIncome() {
    wx.navigateTo({ url: '/pages/rider/income/index' })
  },

  async getLocation(): Promise<WechatMiniprogram.GetLocationSuccessCallbackResult> {
    return new Promise((resolve, reject) => {
      wx.getLocation({
        type: 'gcj02',
        success: resolve,
        fail: (err) => reject(err || new Error('getLocation failed'))
      })
    })
  },

  async syncDeliveryLocation(deliveryId: number, source: string) {
    try {
      await syncRiderDeliveryLocation(deliveryId, source)
    } catch (err: unknown) {
      throw normalizeLocationError(err)
    }
  },

  isActionReconciled(action: DeliveryActionType, status: Delivery['status']) {
    if (action === 'confirmDelivery') {
      return status === 'delivered' || status === 'completed'
    }

    const expectedStatusMap: Record<Exclude<DeliveryActionType, 'confirmDelivery'>, Delivery['status']> = {
      startPickup: 'picking',
      confirmPickup: 'picked',
      startDelivery: 'delivering'
    }

    return expectedStatusMap[action as Exclude<DeliveryActionType, 'confirmDelivery'>] === status
  },

  async reconcileDeliveryAction(deliveryId: number, action: DeliveryActionType) {
    const currentDelivery = this.data.activeDeliveries.find((delivery: Delivery) => delivery.id === deliveryId)
    if (!currentDelivery) return false

    try {
      const latest = await DeliveryService.getDeliveryByOrder(currentDelivery.order_id)
      if (!this.isActionReconciled(action, latest.status)) {
        return false
      }

      await this.refreshRiderOverview()
      await this.refreshData()
      return true
    } catch (err: unknown) {
      logger.warn('Reconcile delivery action failed', { deliveryId, action, err }, 'RiderDashboard')
      return false
    }
  },

  async reconcileGrabOrder(orderId: number) {
    try {
      await DeliveryService.getDeliveryByOrder(orderId)
      this.setData({ activeTab: 'my' })
      await this.refreshRiderOverview()
      await this.refreshData()
      return true
    } catch (err: unknown) {
      logger.warn('Reconcile grab order failed', { orderId, err }, 'RiderDashboard')
      return false
    }
  },

  async onToggleOnline(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    await this.toggleOnlineStatus(e.detail.value)
  },

  async toggleOnlineStatus(targetOnline: boolean) {
    if (this.data.onlineSwitchLoading) return

    this.setData({ onlineSwitchLoading: true })
    wx.showLoading({ title: targetOnline ? '正在上线...' : '正在下线...' })

    try {
      const latestStatus = await RiderService.getStatus()
      const canToggle = targetOnline ? latestStatus.can_go_online : latestStatus.can_go_offline

      if (!canToggle) {
        const message = targetOnline
          ? (latestStatus.online_block_reason || '当前无法上线')
          : (latestStatus.active_deliveries > 0 ? '有配送中的订单，无法下线' : '当前无法下线')

        const statusViewData = this.buildStatusViewData(latestStatus)

        this.setData(statusViewData)

        if (targetOnline && statusViewData.showDepositReminder) {
          this.showDepositBlockedModal(statusViewData.depositReminderText)
          return
        }

        wx.showToast({ title: message, icon: 'none' })
        return
      }

      let info: RiderInfo
      if (targetOnline) {
        info = await RiderService.goOnline()
      } else {
        info = await RiderService.goOffline()
        wx.showToast({ title: '已下线', icon: 'none' })
      }

      const fallbackStatus: RiderStatus = {
        ...latestStatus,
        is_online: targetOnline,
        online_status: targetOnline
          ? (latestStatus.active_deliveries > 0 ? 'delivering' : 'online')
          : 'offline',
        can_go_online: !targetOnline,
        can_go_offline: targetOnline,
        online_block_reason: targetOnline ? undefined : latestStatus.online_block_reason
      }

      const nextStatus = await RiderService.getStatus().catch(() => fallbackStatus)
      const statusViewData = this.buildStatusViewData(nextStatus, info)

      this.setData({
        riderInfo: info,
        initError: '',
        initErrorCanRetry: true,
        dashboardInlineError: this.data.workbenchRefreshError,
        ...statusViewData
      })

      if (targetOnline) {
        this.enterOnlineRuntime().catch((err: unknown) => logger.error('Toggle online refresh error', err))
      } else {
        await this.syncLocationSession([])
        this.setData({
          recommendOrders: [],
          activeDeliveries: [],
          currentDelivery: null,
          hallTabLabel: '抢单大厅 0',
          myTabLabel: '我的任务 0',
          newOrdersCount: 0,
          hallLoadError: '',
          myLoadError: ''
        })
        this.cleanupWebSocket()
      }
    } catch (err: unknown) {
      const fallbackStatus = this.data.riderStatus
      this.setData(this.buildStatusViewData(fallbackStatus))
      const message = getUserMessage(err, '操作失败')
      wx.showToast({
        title: message,
        icon: 'none'
      })
    } finally {
      wx.hideLoading()
      this.setData({ onlineSwitchLoading: false })
    }
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: 'hall' | 'my' }>) {
    this.setData({ activeTab: e.detail.value })
  },

  async onPullDownRefresh() {
    this.setData({ isRefresherTriggered: true })
    try {
      const overview = await this.refreshRiderOverview()
      if (overview.isOnline) {
        await this.refreshData()
      } else {
        this.cleanupWebSocket()
        await this.syncLocationSession([])
        this.setData({
          recommendOrders: [],
          activeDeliveries: [],
          currentDelivery: null,
          hallLoadError: '',
          myLoadError: '',
          newOrdersCount: 0,
          ...this.buildTabLabels(overview.workbench.availableOrderCount, overview.workbench.activeDeliveryCount)
        })
      }
    } catch (err: unknown) {
      logger.error('Pull down refresh error', err)
      const errorState = getConsoleDashboardErrorState('rider', err, '骑手工作台刷新失败，请稍后重试。')
      this.setData({
        initError: errorState.message,
        initErrorCanRetry: errorState.canRetry,
        dashboardInlineError: errorState.message
      })
    }
    this.setData({ isRefresherTriggered: false })
    wx.stopPullDownRefresh()
  },

  async onGrabOrder(e: WechatMiniprogram.TouchEvent) {
    if (this.data.grabActionLoading) return

    const { orderId } = e.currentTarget.dataset as { orderId?: number }
    if (!orderId) return

    const order = this.data.recommendOrders.find((o: RecommendedOrder) => o.order_id === orderId)
    if (!order) return

    this.setData({ grabActionLoading: true })
    wx.showLoading({ title: '校验中...' })
    try {
      const location = await this.getLocation().catch(() => null)
      if (!location) {
        wx.showToast({ title: '无法获取当前位置，抢单失败', icon: 'none' })
        return
      }

      const distance = getDistance(
        location.latitude,
        location.longitude,
        order.pickup_latitude,
        order.pickup_longitude
      )

      if (distance > MAX_GRAB_DISTANCE) {
        wx.showToast({
          title: `距离过远 (约${(distance / 1000).toFixed(1)}km)，仅限${MAX_GRAB_DISTANCE / 1000}km内抢单`,
          icon: 'none',
          duration: 3000
        })
        return
      }

      wx.showLoading({ title: '抢单中...' })
      await DeliveryService.grabOrder(orderId)
      this.setData({ activeTab: 'my' })
      await this.refreshRiderOverview()
      await this.refreshData()
    } catch (err: unknown) {
      const reconciled = await this.reconcileGrabOrder(orderId)
      if (reconciled) {
        return
      }

      const message = getUserMessage(err, '抢单失败')
      wx.showToast({
        title: message,
        icon: 'none'
      })
    } finally {
      wx.hideLoading()
      this.setData({ grabActionLoading: false })
    }
  },

  onGoToDetail(e: WechatMiniprogram.TouchEvent) {
    const { orderId } = e.currentTarget.dataset as { orderId?: number }
    if (!orderId) return
    wx.navigateTo({
      url: `/pages/rider/task-detail/index?id=${orderId}`
    })
  },

  onGoToNavigation(e: WechatMiniprogram.TouchEvent) {
    const { orderId } = e.currentTarget.dataset as { orderId?: number }
    if (!orderId) return

    wx.navigateTo({
      url: `/pages/rider/navigation/index?id=${orderId}`
    })
  },

  onSwitchToHall() {
    this.setData({ activeTab: 'hall' })
  },

  onGoToDeposit() {
    wx.navigateTo({ url: '/pages/rider/deposit/index' })
  },

  showDepositBlockedModal(message: string) {
    wx.showModal({
      title: '暂时无法上线',
      content: `${message}。请先前往押金与账单补足后，再尝试上线接单。`,
      confirmText: '去充值',
      cancelText: '知道了',
      success: ({ confirm }) => {
        if (confirm) {
          this.onGoToDeposit()
        }
      }
    })
  },

  async enterOnlineRuntime() {
    if (!this.data.isOnline) return

    await this.refreshData()
    this.initWebSocket()
  },

  initWebSocket() {
    if (!this.data.isOnline) return

    this.cleanupWebSocket()
    wsManager.connect()

    const goneSub = wsManager.on(WSMessageType.DELIVERY_POOL_GONE, (data) => {
      const payload =
        typeof data === 'object' && data !== null
          ? (data as { order_id?: number })
          : {}
      const { order_id } = payload
      if (!order_id) return
      const { recommendOrders } = this.data

      const index = recommendOrders.findIndex((o: RecommendedOrder) => o.order_id === order_id)
      if (index > -1) {
        logger.info(`订单 ${order_id} 已被他人抢走，从本地移除`, undefined, 'RiderDashboard')
        const newList = [...recommendOrders]
        newList.splice(index, 1)
        this.setData({ recommendOrders: newList })
      }
    })

    const newSub = wsManager.on(WSMessageType.DELIVERY_POOL_NEW, () => {
      this.setData({
        newOrdersCount: this.data.newOrdersCount + 1
      })

      wx.vibrateShort({ type: 'medium' })
    })

    this.data._wsListeners = [goneSub, newSub]
  },

  cleanupWebSocket() {
    if (this.data._wsListeners) {
      this.data._wsListeners.forEach((unsub: WsUnsubscribe) => {
        if (typeof unsub === 'function') unsub()
      })
      this.data._wsListeners = []
    }
  },

  onRefreshHall() {
    this.setData({ newOrdersCount: 0 })
    this.refreshData().catch((err: unknown) => logger.error('Manual refresh error', err))
  },

  async onRetryLoad() {
    if (this.data.initError || !this.data.riderInfo || !this.data.riderStatus) {
      this.initData().catch((err: unknown) => logger.error('Retry init error', err))
      return
    }
    try {
      const overview = await this.refreshRiderOverview()
      if (overview.isOnline) {
        await this.refreshData()
      } else {
        this.cleanupWebSocket()
        await this.syncLocationSession([])
        this.setData({
          recommendOrders: [],
          activeDeliveries: [],
          currentDelivery: null,
          hallLoadError: '',
          myLoadError: '',
          newOrdersCount: 0,
          ...this.buildTabLabels(overview.workbench.availableOrderCount, overview.workbench.activeDeliveryCount)
        })
      }
    } catch (err: unknown) {
      logger.error('Retry refresh error', err)
    }
  },

  async onRetryLocationSync() {
    if (this.data.locationActionLoading) return

    this.setData({ locationActionLoading: true })
    wx.showLoading({ title: '刷新定位中...' })
    try {
      if (this.data.locationNeedsPermission) {
        const granted = await riderLiveLocationSession.requestPermissionAndRestart()
        if (!granted) {
          wx.showToast({ title: '请先开启定位权限', icon: 'none' })
          return
        }
      } else {
        await riderLiveLocationSession.refreshNow('rider_dashboard_manual_retry')
        await riderLiveLocationSession.flushNow()
      }
    } catch (err: unknown) {
      const message = getUserMessage(err, '定位刷新失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ locationActionLoading: false })
    }
  },

  async onResolveLocationPermission() {
    if (this.data.locationActionLoading) return

    this.setData({ locationActionLoading: true })
    wx.showLoading({ title: '开启定位中...' })
    try {
      const granted = await riderLiveLocationSession.requestPermissionAndRestart()
      if (!granted) {
        wx.showToast({ title: '请先开启定位权限', icon: 'none' })
        return
      }
    } finally {
      wx.hideLoading()
      this.setData({ locationActionLoading: false })
    }
  }
}
