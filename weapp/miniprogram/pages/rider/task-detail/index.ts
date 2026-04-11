import DeliveryService, { Delivery, DeliveryLocationPoint } from '../../../api/delivery'
import { mapService, MapMarker, MapPoint, MapPolyline } from '../../../services/map'
import { logger } from '../../../utils/logger'
import { locationService } from '../../../utils/location'
import { normalizeLocationError, syncRiderDeliveryLocation } from '../../../utils/rider-location'
import { riderLiveLocationSession, RiderLiveLocationState } from '../../../utils/rider-live-location'
import { getStableBarHeights } from '../../../utils/responsive'

interface RiderTaskDetailOptions {
    id?: string
}

interface UserMessageError {
    userMessage?: string
}

type DeliveryAction = (deliveryId: number) => Promise<Delivery>

type DeliveryView = Delivery & {
    deadline_desc: string
    can_update_status: boolean
    action_label: string
}

type TagTheme = 'primary' | 'success' | 'warning' | 'danger' | 'default'

let taskDetailLocationUnsubscribe: null | (() => void) = null

function getUserMessage(err: unknown, fallback: string) {
    const userMessage = (err as UserMessageError).userMessage
    return typeof userMessage === 'string' && userMessage ? userMessage : fallback
}

function isExpectedStatusReached(status: Delivery['status'], expectedStatus: Delivery['status']) {
    if (expectedStatus === 'delivered') {
        return status === 'delivered' || status === 'completed'
    }

    return status === expectedStatus
}

function getDeliveryActionLabel(status: Delivery['status']): string {
    if (status === 'assigned') return '我已到达商家'
    if (status === 'picking') return '确认取餐'
    if (status === 'picked') return '开始配送'
    if (status === 'delivering') return '确认已送达'
    return ''
}

function canUpdateDeliveryStatus(status: Delivery['status']): boolean {
    return status === 'assigned' || status === 'picking' || status === 'picked' || status === 'delivering'
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

    const days = Math.floor(hours / 24)
    return `${days} 天前`
}

function toMapPoint(point: DeliveryLocationPoint | null): MapPoint | null {
    if (!point) return null
    return {
        latitude: point.latitude,
        longitude: point.longitude
    }
}

function isLocationTrackedStatus(status: Delivery['status']): boolean {
    return status === 'assigned' || status === 'picking' || status === 'picked' || status === 'delivering'
}

Page({
    data: {
        orderId: 0,
        delivery: null as DeliveryView | null,
        loading: false,
        errorMessage: '',
        navBarHeight: 88,

        // 状态映射
        statusSteps: [
            { title: '已接单', status: 'assigned' },
            { title: '取餐中', status: 'picking' },
            { title: '配送中', status: 'delivering' },
            { title: '已送达', status: 'completed' }
        ],
        currentStep: 0,

        mapCenter: { latitude: 39.9, longitude: 116.4 },
        mapScale: 13,
        markers: [] as MapMarker[],
        polyline: [] as MapPolyline[],
        includePoints: [] as MapPoint[],
        serverTrackPoints: [] as MapPoint[],
        serverLatestPoint: null as MapPoint | null,
        serverLatestRecordedAt: '',
        routeSummary: '',

        locationStatusText: '等待进入配送',
        locationStatusTheme: 'default' as TagTheme,
        locationPendingText: '',
        locationUpdatedText: '暂无定位记录',
        locationActionText: '立即刷新',
        showLocationAction: false,
        needsLocationPermission: false
    },

    onLoad(options: RiderTaskDetailOptions) {
        const { navBarHeight } = getStableBarHeights()
        this.setData({ 
            navBarHeight,
            orderId: Number(options.id || 0)
        })
        this.bindLocationSession()
        this.fetchTaskDetail()
    },

    onShow() {
        if (this.data.delivery && isLocationTrackedStatus(this.data.delivery.status)) {
            void riderLiveLocationSession.setActiveDelivery(this.data.delivery.id, 'rider_task_detail_show')
        }
    },

    onUnload() {
        if (taskDetailLocationUnsubscribe) {
            taskDetailLocationUnsubscribe()
            taskDetailLocationUnsubscribe = null
        }
    },

    async fetchTaskDetail() {
        if (!this.data.orderId) return
        
        this.setData({ loading: true })
        try {
            // 使用正确的获取详情接口，而不是抢单接口
            const delivery = await DeliveryService.getDeliveryByOrder(this.data.orderId)
            const deliveryView = this.decorateDelivery(delivery)
            
            this.setData({ 
                delivery: deliveryView,
                currentStep: this.mapStatusToStep(delivery.status),
                errorMessage: ''
            })

            await this.loadLocationMap(deliveryView)

            if (isLocationTrackedStatus(deliveryView.status)) {
                await riderLiveLocationSession.setActiveDelivery(deliveryView.id, 'rider_task_detail_fetch')
            }

            this.applyLocationSessionState(riderLiveLocationSession.getState())
        } catch (err: unknown) {
            logger.error('Fetch task detail failed', err)
            const message = getUserMessage(err, '任务详情加载失败，请稍后重试')
            this.setData({ delivery: null, errorMessage: message })
        } finally {
            this.setData({ loading: false })
        }
    },

    async reconcileDeliveryState(expectedStatus: Delivery['status']) {
        try {
            const latest = await DeliveryService.getDeliveryByOrder(this.data.orderId)
            if (!isExpectedStatusReached(latest.status, expectedStatus)) {
                return false
            }

            const deliveryView = this.decorateDelivery(latest)
            this.setData({
                delivery: deliveryView,
                currentStep: this.mapStatusToStep(latest.status),
                errorMessage: ''
            })
            await this.loadLocationMap(deliveryView)
            return true
        } catch (err: unknown) {
            logger.warn('Reconcile task detail state failed', { expectedStatus, err }, 'RiderTaskDetail')
            return false
        }
    },

    decorateDelivery(delivery: Delivery): DeliveryView {
        const deadline = delivery.status === 'assigned' || delivery.status === 'picking'
            ? delivery.estimated_pickup_at
            : delivery.estimated_delivery_at

        return {
            ...delivery,
            deadline_desc: this.formatDeadline(deadline),
            can_update_status: canUpdateDeliveryStatus(delivery.status),
            action_label: getDeliveryActionLabel(delivery.status)
        }
    },

    mapStatusToStep(status: string): number {
        const statusMap: Record<string, number> = {
            'assigned': 0,
            'picking': 1,
            'picked': 2,
            'delivering': 2,
            'delivered': 3,
            'completed': 3
        }
        return statusMap[status] ?? 0
    },

    formatDeadline(timeStr?: string) {
        if (!timeStr) return '尽快送达'

        const date = new Date(timeStr)
        const diff = date.getTime() - Date.now()
        if (diff < 0) {
            return '已超时'
        }

        const hours = date.getHours().toString().padStart(2, '0')
        const minutes = date.getMinutes().toString().padStart(2, '0')
        if (diff < 60 * 60 * 1000) {
            return `剩 ${Math.max(1, Math.floor(diff / 60000))} 分钟 (${hours}:${minutes})`
        }

        return `${hours}:${minutes} 前`
    },

    bindLocationSession() {
        if (taskDetailLocationUnsubscribe) {
            taskDetailLocationUnsubscribe()
        }

        taskDetailLocationUnsubscribe = riderLiveLocationSession.subscribe((state) => {
            this.applyLocationSessionState(state)
        })
    },

    buildLocationView(state: RiderLiveLocationState | null, fallbackRecordedAt: string) {
        if (!state || !state.activeDeliveryId || !this.data.delivery || state.activeDeliveryId !== this.data.delivery.id) {
            return {
                locationStatusText: isLocationTrackedStatus(this.data.delivery?.status || 'pending') ? '等待连续定位启动' : '当前状态无需定位',
                locationStatusTheme: 'default' as TagTheme,
                locationPendingText: '',
                locationUpdatedText: fallbackRecordedAt ? `最近轨迹 ${formatRelativeTime(fallbackRecordedAt)}` : '暂无定位记录',
                locationActionText: '立即刷新',
                showLocationAction: !!this.data.delivery && isLocationTrackedStatus(this.data.delivery.status),
                needsLocationPermission: false
            }
        }

        const updatedText = state.lastUploadedAt
            ? `最近上传 ${formatRelativeTime(state.lastUploadedAt)}`
            : (fallbackRecordedAt ? `最近轨迹 ${formatRelativeTime(fallbackRecordedAt)}` : '暂未上传')

        const pendingText = state.pendingCount > 0
            ? `待补发 ${state.pendingCount} 个定位点`
            : ''

        const baseView = {
            locationPendingText: pendingText,
            locationUpdatedText: updatedText,
            locationActionText: state.uploadState === 'permission_required' ? '开启定位' : '立即刷新',
            showLocationAction: state.uploadState === 'permission_required' || state.uploadState === 'retrying' || state.uploadState === 'tracking',
            needsLocationPermission: state.uploadState === 'permission_required'
        }

        switch (state.uploadState) {
            case 'starting':
                return { ...baseView, locationStatusText: '正在开启连续定位', locationStatusTheme: 'warning' as TagTheme }
            case 'uploading':
                return { ...baseView, locationStatusText: '正在上传位置', locationStatusTheme: 'primary' as TagTheme }
            case 'retrying':
                return { ...baseView, locationStatusText: '网络恢复后会自动补发', locationStatusTheme: 'warning' as TagTheme }
            case 'permission_required':
                return { ...baseView, locationStatusText: '需要开启定位权限', locationStatusTheme: 'danger' as TagTheme }
            case 'tracking':
                return { ...baseView, locationStatusText: '定位正常', locationStatusTheme: 'success' as TagTheme }
            default:
                return { ...baseView, locationStatusText: '等待连续定位启动', locationStatusTheme: 'default' as TagTheme }
        }
    },

    applyLocationSessionState(state: RiderLiveLocationState) {
        const delivery = this.data.delivery
        if (!delivery) {
            return
        }

        const view = this.buildLocationView(state, this.data.serverLatestRecordedAt)
        this.setData(view)

        const riderPoint = state.activeDeliveryId === delivery.id && state.latestPoint
            ? {
                latitude: state.latestPoint.latitude,
                longitude: state.latestPoint.longitude
            }
            : this.data.serverLatestPoint

        this.renderDeliveryMap(delivery, riderPoint)
    },

    async loadLocationMap(delivery: DeliveryView) {
        const pickupPoint: MapPoint = {
            latitude: delivery.pickup_latitude,
            longitude: delivery.pickup_longitude
        }
        const deliveryPoint: MapPoint = {
            latitude: delivery.delivery_latitude,
            longitude: delivery.delivery_longitude
        }

        const [latestResult, trackResult, routeResult] = await Promise.all([
            DeliveryService.getRiderLocation(delivery.id).catch(() => null),
            DeliveryService.getDeliveryTrack(delivery.id).catch(() => [] as DeliveryLocationPoint[]),
            mapService.planRoute(pickupPoint, deliveryPoint).catch(() => null)
        ])

        const serverLatestPoint = latestResult
            ? { latitude: latestResult.latitude, longitude: latestResult.longitude }
            : null
        const serverLatestRecordedAt = latestResult?.recorded_at || ''
        const serverTrackPoints = trackResult
            .map((point) => toMapPoint(point))
            .filter((point): point is MapPoint => !!point)

        const routeSummary = routeResult
            ? `预计路程 ${(routeResult.distance / 1000).toFixed(1)}km · 约 ${Math.max(1, Math.round(routeResult.duration / 60))} 分钟`
            : '已展示配送主线路，实时位置会随骑手移动更新'

        this.setData({
            serverLatestPoint,
            serverLatestRecordedAt,
            serverTrackPoints,
            routeSummary
        })

        this.renderDeliveryMap(delivery, serverLatestPoint)
    },

    renderDeliveryMap(delivery: DeliveryView, riderPoint: MapPoint | null) {
        const pickupPoint: MapPoint = {
            latitude: delivery.pickup_latitude,
            longitude: delivery.pickup_longitude
        }
        const deliveryPoint: MapPoint = {
            latitude: delivery.delivery_latitude,
            longitude: delivery.delivery_longitude
        }

        const markers: MapMarker[] = [
            mapService.createMarker(1, pickupPoint, '商家', '/assets/merchant.png'),
            mapService.createMarker(3, deliveryPoint, '顾客', '/assets/customer.png')
        ]
        const includePoints: MapPoint[] = [pickupPoint, deliveryPoint]

        if (riderPoint) {
            markers.push(mapService.createMarker(2, riderPoint, '骑手', '/assets/rider.png'))
            includePoints.push(riderPoint)
        }

        const polyline: MapPolyline[] = [
            mapService.createPolyline([pickupPoint, deliveryPoint], {
                color: '#1d63ff',
                width: 6,
                dottedLine: this.data.serverTrackPoints.length < 2
            })
        ]

        if (this.data.serverTrackPoints.length > 1) {
            polyline.push(mapService.createPolyline(this.data.serverTrackPoints, {
                color: '#00897B',
                width: 8,
                arrowLine: true
            }))
        }

        const mapCenter = riderPoint || {
            latitude: (pickupPoint.latitude + deliveryPoint.latitude) / 2,
            longitude: (pickupPoint.longitude + deliveryPoint.longitude) / 2
        }

        this.setData({
            markers,
            polyline,
            includePoints,
            mapCenter
        })
    },

    /**
     * 更新配送状态按钮点击
     */
    async onUpdateStatus() {
        if (!this.data.delivery) return
        const { id, status } = this.data.delivery
        
        let nextAction = ''
        let actionMethod: DeliveryAction | null = null
        let expectedStatus: Delivery['status'] | null = null

        if (status === 'assigned') {
            nextAction = '到达商家'
            actionMethod = DeliveryService.startPickup
            expectedStatus = 'picking'
        } else if (status === 'picking') {
            nextAction = '确认取餐'
            actionMethod = DeliveryService.confirmPickup
            expectedStatus = 'picked'
        } else if (status === 'picked') {
            nextAction = '开始配送'
            actionMethod = DeliveryService.startDelivery
            expectedStatus = 'delivering'
        } else if (status === 'delivering') {
            nextAction = '确认送达'
            actionMethod = DeliveryService.confirmDelivery
            expectedStatus = 'delivered'
        }

        if (!actionMethod || !expectedStatus) return
        const method = actionMethod
        const nextExpectedStatus = expectedStatus
        const locationSourceMap: Record<NonNullable<Delivery['status']>, string> = {
            pending: 'rider_task_detail_pending',
            assigned: 'rider_task_detail_start_pickup',
            picking: 'rider_task_detail_confirm_pickup',
            picked: 'rider_task_detail_start_delivery',
            delivering: 'rider_task_detail_confirm_delivery',
            delivered: 'rider_task_detail_delivered',
            completed: 'rider_task_detail_completed',
            cancelled: 'rider_task_detail_cancelled',
            exception: 'rider_task_detail_exception'
        }

        wx.showModal({
            title: '状态更新',
            content: `确定已完成 ${nextAction} 吗？`,
            success: async (res) => {
                if (res.confirm) {
                    wx.showLoading({ title: '同步中...' })
                    try {
                        await this.syncDeliveryLocation(id, locationSourceMap[status] || 'rider_task_detail_action')
                        const updated = await method(id)
                        const updatedView = this.decorateDelivery(updated)
                        this.setData({ 
                            delivery: updatedView,
                            currentStep: this.mapStatusToStep(updated.status)
                        })
                        
                        if (updated.status === 'completed' || updated.status === 'delivered') {
                            wx.navigateBack()
                            return
                        }
                    } catch (err: unknown) {
                        const reconciled = await this.reconcileDeliveryState(nextExpectedStatus)
                        if (reconciled) {
                            const latestStatus = this.data.delivery?.status
                            if (latestStatus === 'completed' || latestStatus === 'delivered') {
                                wx.navigateBack()
                                return
                            }
                            return
                        }

                        const message = getUserMessage(err, '操作失败')
                        wx.showToast({ title: message, icon: 'none' })
                    } finally {
                        wx.hideLoading()
                    }
                }
            }
        })
    },

    onCallPhone(e: WechatMiniprogram.TouchEvent) {
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

    async syncDeliveryLocation(deliveryId: number, source: string) {
        try {
            await syncRiderDeliveryLocation(deliveryId, source)
        } catch (err: unknown) {
            throw normalizeLocationError(err)
        }
    },

    async onRetryLocationSync() {
        const delivery = this.data.delivery
        if (!delivery || !isLocationTrackedStatus(delivery.status)) {
            return
        }

        wx.showLoading({ title: '刷新定位中...' })
        try {
            if (this.data.needsLocationPermission) {
                const granted = await riderLiveLocationSession.requestPermissionAndRestart()
                if (!granted) {
                    wx.showToast({ title: '请先开启定位权限', icon: 'none' })
                    return
                }
            } else {
                await riderLiveLocationSession.refreshNow('rider_task_detail_manual_retry')
                await riderLiveLocationSession.flushNow()
            }

            this.applyLocationSessionState(riderLiveLocationSession.getState())
        } catch (err: unknown) {
            const message = getUserMessage(err, '定位刷新失败，请稍后重试')
            wx.showToast({ title: message, icon: 'none' })
        } finally {
            wx.hideLoading()
        }
    },

    onGoToNavigation() {
        if (!this.data.orderId) return

        wx.navigateTo({
            url: `/pages/rider/navigation/index?id=${this.data.orderId}`
        })
    },

    onCopyOrderNo() {
        wx.setClipboardData({
            data: String(this.data.delivery?.order_no || this.data.orderId),
            success: () => wx.showToast({ title: '单号已复制' })
        })
    },

    onRetry() {
        this.fetchTaskDetail()
    },

    onBack() {
        wx.navigateBack({ delta: 1 }).catch(() => {
            wx.redirectTo({ url: '/pages/rider/dashboard/index' })
        })
    }
})
