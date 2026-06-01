import DeliveryService, {
  buildDeliveryProgress,
  DeliveryProgressView,
  DeliveryResponse,
  getDeliveryStatusDisplay,
  shouldPollDeliveryTrackingState
} from '../_main_shared/api/delivery'
import { BicyclingDirectionResponse, getBicyclingDirection } from '../../../api/location'
import { mapService } from '../_main_shared/services/map'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { confirmReceiptWithRecovery } from '../_services/order-receipt-confirmation'

interface MapPoint {
  latitude: number
  longitude: number
}

interface RoutePointCandidate {
  lat?: number
  lng?: number
  latitude?: number
  longitude?: number
}

interface MiniMarker {
  id: number
  latitude: number
  longitude: number
  width: number
  height: number
  iconPath: string
  callout: {
    content: string
    color: string
    fontSize: number
    padding: number
    borderRadius: number
    display: 'ALWAYS'
    bgColor: string
  }
}

interface Polyline {
  points: MapPoint[]
  color: string
  width: number
  dottedLine?: boolean
  arrowLine?: boolean
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    orderId: 0,
    deliveryId: 0,
    navBarHeight: 88,
    loading: true,
    isError: false,
    errorMsg: '',
    // 代取信息
    delivery: null as DeliveryResponse | null,
    riderName: '',
    riderPhone: '',
    riderPhoneDisplay: '',
    estimatedDeliveryTime: '',
    deliveryStatus: '',
    deliveryStatusText: '',
    showConfirmReceipt: false,
    // 地图相关
    mapCenter: { latitude: 39.9, longitude: 116.4 },
    scale: 14,
    markers: [] as MiniMarker[],
    polyline: [] as Polyline[],
    includePoints: [] as MapPoint[],
    merchantPoint: null as MapPoint | null,
    customerPoint: null as MapPoint | null,
    riderPoint: null as MapPoint | null,
    routePoints: [] as MapPoint[],
    remainingRoutePoints: [] as MapPoint[],
    remainingStageText: '',
    remainingDistanceText: '',
    remainingDurationText: '',
    lastRiderLocationText: '',
    showRouteSummary: false,
    // 进度
    progress: [] as DeliveryProgressView[],
    // 位置同步定时器
    locationTimer: null as number | null
  },

  onLoad(options: { orderId?: string }) {
    const orderId = options.orderId ? parseInt(options.orderId) : NaN
    if (Number.isFinite(orderId) && orderId > 0) {
      this.setData({ orderId })
      this.loadDeliveryData()
      return
    }

    // 未传或无效订单ID，立即停止加载，避免空白页
    this.setData({ loading: false })
    wx.showToast({ title: '缺少订单ID', icon: 'none' })
  },

  onUnload() {
    this.stopLocationTracking()
  },

  onHide() {
    // 页面隐藏时暂停轮询（用户切到其他页面）
    this.stopLocationTracking()
  },

  onShow() {
    // 页面重新显示时恢复轮询（副作用：也立即同步状态和位置）
    if (this.data.deliveryId && this.data.delivery && this.shouldPollTrackingState(this.data.delivery.status)) {
      this.refreshTrackingState()
      this.startLocationTracking()
    }
  },

  stopLocationTracking() {
    if (this.data.locationTimer !== null) {
      clearInterval(this.data.locationTimer)
      this.setData({ locationTimer: null })
    }
  },

  async onConfirmReceipt() {
    try {
      const result = await confirmReceiptWithRecovery({
        orderId: this.data.orderId,
        modalContent: '确认已收到包裹？',
        source: 'Tracking.onConfirmReceipt'
      })
      if (result.status === 'confirmed') {
        wx.navigateBack({ delta: 1 })
      }
    } catch (error) {
      wx.hideLoading()
      logger.error('确认收货失败', error, 'Tracking.onConfirmReceipt')
      wx.showToast({ title: getErrorUserMessage(error, '订单状态同步失败，请稍后重试'), icon: 'none' })
    }
  },

  async loadDeliveryData() {
    this.setData({ loading: true, isError: false })
    try {
      // 1. 获取代取信息
      const delivery = await DeliveryService.getDeliveryByOrder(this.data.orderId)

      // 2. 处理骑手信息
      const riderPhone = delivery.pickup_phone || ''
      const riderPhoneDisplay = riderPhone
        ? riderPhone.replace(/(\d{3})\d{4}(\d{4})/, '$1****$2')
        : ''
      const statusDisplay = getDeliveryStatusDisplay(delivery.status)

      // 3. 生成代取进度
      const progress = buildDeliveryProgress(delivery, this.formatTime)

      // 4. 设置地图标记点
      const merchantPoint: MapPoint = {
        latitude: delivery.pickup_latitude,
        longitude: delivery.pickup_longitude
      }
      const customerPoint: MapPoint = {
        latitude: delivery.delivery_latitude,
        longitude: delivery.delivery_longitude
      }

      this.setData({
        delivery,
        deliveryId: delivery.id,
        riderName: '骑手', // 后端暂无骑手姓名字段
        riderPhone,
        riderPhoneDisplay,
        estimatedDeliveryTime: delivery.estimated_delivery_at
          ? this.formatTime(delivery.estimated_delivery_at)
          : '计算中',
        deliveryStatus: delivery.status,
        deliveryStatusText: statusDisplay.text,
        showConfirmReceipt: statusDisplay.canConfirmReceipt,
        progress,
        loading: false
      })

      // 5. 设置地图
      this.setupMap(merchantPoint, customerPoint)

      // 6. 开始追踪状态，等待接单后继续刷新骑手位置
      if (this.shouldPollTrackingState(delivery.status)) {
        this.startLocationTracking()
      }
    } catch (error: unknown) {
      logger.error('加载代取信息失败', error, 'tracking.loadDeliveryData')
      this.setData({ 
        loading: false, 
        isError: true, 
        errorMsg: getErrorMessage(error, '获取代取信息失败') 
      })
    }
  },

  formatTime(timeStr: string): string {
    try {
      const date = new Date(timeStr)
      const hours = ('0' + date.getHours()).slice(-2)
      const minutes = ('0' + date.getMinutes()).slice(-2)
      return `${hours}:${minutes}`
    } catch {
      return ''
    }
  },

  async setupMap(merchantPoint: MapPoint, customerPoint: MapPoint) {
    const markers: MiniMarker[] = [
      this.buildMarker(1, merchantPoint, '商家', '/assets/merchant.png'),
      this.buildMarker(3, customerPoint, '我', '/assets/customer.png')
    ]

    const includePoints = [merchantPoint, customerPoint]

    // 计算地图中心
    const mapCenter = {
      latitude: (merchantPoint.latitude + customerPoint.latitude) / 2,
      longitude: (merchantPoint.longitude + customerPoint.longitude) / 2
    }

    this.setData({
      markers,
      includePoints,
      mapCenter,
      merchantPoint,
      customerPoint,
      riderPoint: null,
      routePoints: [],
      remainingRoutePoints: [],
      remainingStageText: '',
      remainingDistanceText: '',
      remainingDurationText: '',
      lastRiderLocationText: '',
      showRouteSummary: false,
      polyline: []
    })

    // 规划路线
    await this.planRoute(merchantPoint, customerPoint)

    // 获取骑手位置并按最新位置同步剩余路线
    await this.updateRiderLocation()
  },

  async updateRiderLocation() {
    const { deliveryId, delivery } = this.data
    if (!deliveryId || !delivery) return

    try {
      const location = await DeliveryService.getRiderLocation(deliveryId)
      if (
        location
        && Number.isFinite(location.latitude)
        && Number.isFinite(location.longitude)
      ) {
        const riderPoint: MapPoint = {
          latitude: location.latitude,
          longitude: location.longitude
        }

        // 更新骑手标记
        const markers = [...this.data.markers]
        const riderMarkerIndex = markers.findIndex((m) => m.id === 2)
        const riderMarker = this.buildMarker(
          2,
          riderPoint,
          '骑手',
          '/assets/rider.png'
        )

        if (riderMarkerIndex >= 0) {
          markers[riderMarkerIndex] = riderMarker
        } else {
          markers.push(riderMarker)
        }

        // 更新includePoints
        const includePoints = [
          {
            latitude: delivery.pickup_latitude,
            longitude: delivery.pickup_longitude
          },
          riderPoint,
          {
            latitude: delivery.delivery_latitude,
            longitude: delivery.delivery_longitude
          }
        ]

        const shouldShowRemainingRoute = this.shouldAdvanceRouteByRider()

        this.setData({
          markers,
          includePoints,
          riderPoint,
          showRouteSummary: shouldShowRemainingRoute,
          lastRiderLocationText: location.recorded_at ? `最近同步 ${this.formatRelativeTime(location.recorded_at)}` : ''
        })

        if (shouldShowRemainingRoute) {
          await this.planRemainingRoute(riderPoint)
        } else {
          this.renderRoutePolyline(this.data.routePoints, this.data.merchantPoint, this.data.customerPoint)
        }
      }
    } catch (error) {
      logger.warn('获取骑手位置失败', error, 'tracking.updateRiderLocation')
    }
  },

  startLocationTracking() {
    if (this.data.locationTimer !== null) return

    // 每10秒同步一次代取状态和骑手位置
    const timer = setInterval(() => {
      this.refreshTrackingState()
    }, 10000) as unknown as number

    this.setData({ locationTimer: timer })
  },

  shouldPollTrackingState(status?: DeliveryResponse['status']): boolean {
    return shouldPollDeliveryTrackingState(status)
  },

  async refreshTrackingState() {
    if (!this.data.orderId) return

    try {
      const delivery = await DeliveryService.getDeliveryByOrder(this.data.orderId)
      const statusDisplay = getDeliveryStatusDisplay(delivery.status)
      const progress = buildDeliveryProgress(delivery, this.formatTime)

      this.setData({
        delivery,
        deliveryId: delivery.id,
        deliveryStatus: delivery.status,
        deliveryStatusText: statusDisplay.text,
        showConfirmReceipt: statusDisplay.canConfirmReceipt,
        progress,
        estimatedDeliveryTime: delivery.estimated_delivery_at
          ? this.formatTime(delivery.estimated_delivery_at)
          : '计算中'
      })

      const hasMapAnchors = !!this.data.merchantPoint && !!this.data.customerPoint
      if (!hasMapAnchors) {
        await this.setupMap(
          { latitude: delivery.pickup_latitude, longitude: delivery.pickup_longitude },
          { latitude: delivery.delivery_latitude, longitude: delivery.delivery_longitude }
        )
      } else if (statusDisplay.isLocationTracked) {
        await this.updateRiderLocation()
      }

      if (!this.shouldPollTrackingState(delivery.status)) {
        this.stopLocationTracking()
      }
    } catch (error) {
      logger.warn('同步代取追踪状态失败', error, 'tracking.refreshTrackingState')
    }
  },

  async planRoute(merchantPoint: MapPoint, customerPoint: MapPoint) {
    try {
      const fromStr = `${merchantPoint.latitude},${merchantPoint.longitude}`
      const toStr = `${customerPoint.latitude},${customerPoint.longitude}`

      const data = await getBicyclingDirection({ from: fromStr, to: toStr })
      const routeData = this.unwrapDirectionData(data)
      const routePoints = this.normalizeRoutePoints(routeData?.points)

      if (routePoints.length > 1) {
        this.setData({ routePoints })
        this.renderRoutePolyline(
          routePoints,
          merchantPoint,
          customerPoint
        )
      } else {
        this.setData({ routePoints: [] })
        this.useFallbackRoute(merchantPoint, customerPoint)
      }
    } catch (error) {
      logger.warn('路线规划失败', error, 'tracking.planRoute')
      this.setData({ routePoints: [] })
      this.useFallbackRoute(merchantPoint, customerPoint)
    }
  },

  async planRemainingRoute(riderPoint: MapPoint) {
    const targetPoint = this.getRemainingRouteTarget()
    if (!targetPoint) {
      return
    }

    try {
      const fromStr = `${riderPoint.latitude},${riderPoint.longitude}`
      const toStr = `${targetPoint.latitude},${targetPoint.longitude}`
      const data = await getBicyclingDirection({ from: fromStr, to: toStr })
      const routeData = this.unwrapDirectionData(data)
      const remainingRoutePoints = this.normalizeRoutePoints(routeData?.points)
      const points = remainingRoutePoints.length > 1
        ? remainingRoutePoints
        : [riderPoint, targetPoint]

      this.setData({
        remainingRoutePoints: points,
        remainingStageText: this.getRemainingStageText(),
        remainingDistanceText: typeof routeData?.distance === 'number'
          ? mapService.formatDistance(routeData.distance)
          : '',
        remainingDurationText: typeof routeData?.duration === 'number'
          ? mapService.formatDuration(routeData.duration)
          : ''
      })

      this.renderRoutePolyline(points, riderPoint, targetPoint)
    } catch (error) {
      logger.warn('剩余路线规划失败', error, 'tracking.planRemainingRoute')
      const fallbackPoints = [riderPoint, targetPoint]
      this.setData({
        remainingRoutePoints: fallbackPoints,
        remainingStageText: this.getRemainingStageText(),
        remainingDistanceText: '',
        remainingDurationText: ''
      })
      this.renderRoutePolyline(fallbackPoints, riderPoint, targetPoint)
    }
  },

  unwrapDirectionData(response: BicyclingDirectionResponse) {
    if ('code' in response) {
      return response.code === 0 ? response.data : undefined
    }
    return response
  },

  normalizeRoutePoints(points?: RoutePointCandidate[]): MapPoint[] {
    if (!Array.isArray(points)) {
      return []
    }

    return points
      .map((point) => {
        const latitude = typeof point.latitude === 'number' ? point.latitude : point.lat
        const longitude = typeof point.longitude === 'number' ? point.longitude : point.lng
        if (typeof latitude !== 'number' || typeof longitude !== 'number') {
          return null
        }
        if (!Number.isFinite(latitude) || !Number.isFinite(longitude)) {
          return null
        }
        return { latitude, longitude }
      })
      .filter((point): point is MapPoint => !!point)
  },

  shouldAdvanceRouteByRider(): boolean {
    return getDeliveryStatusDisplay(this.data.delivery?.status).isLocationTracked
  },

  getRemainingRouteTarget(): MapPoint | null {
    const statusDisplay = getDeliveryStatusDisplay(this.data.delivery?.status)
    if (statusDisplay.isAssignedStage) {
      return this.data.merchantPoint
    }
    if (statusDisplay.isPickedStage || statusDisplay.isDeliveringStage) {
      return this.data.customerPoint
    }
    return null
  },

  getRemainingStageText(): string {
    const statusDisplay = getDeliveryStatusDisplay(this.data.delivery?.status)
    return statusDisplay.isAssignedStage ? '距取餐点' : '距送达点'
  },

  renderRoutePolyline(
    routePoints: MapPoint[],
    merchantPoint: MapPoint | null,
    customerPoint: MapPoint | null,
    riderPoint?: MapPoint | null
  ) {
    if (!merchantPoint || !customerPoint) return

    const remainingPoints = this.getRemainingRoutePoints(routePoints, customerPoint, riderPoint)
    const points = remainingPoints.length > 1 ? remainingPoints : routePoints

    if (points.length > 1) {
      this.setData({
        polyline: [
          {
            points,
            color: '#1d63ff',
            width: 8,
            dottedLine: false,
            arrowLine: true
          }
        ]
      })
      return
    }

    this.useFallbackRoute(merchantPoint, customerPoint)
  },

  getRemainingRoutePoints(
    routePoints: MapPoint[],
    customerPoint: MapPoint,
    riderPoint?: MapPoint | null
  ): MapPoint[] {
    if (!riderPoint || routePoints.length < 2) {
      return routePoints
    }

    let nearestIndex = 0
    let nearestDistance = Number.POSITIVE_INFINITY
    routePoints.forEach((point, index) => {
      const latDelta = point.latitude - riderPoint.latitude
      const lngDelta = point.longitude - riderPoint.longitude
      const distance = latDelta * latDelta + lngDelta * lngDelta
      if (distance < nearestDistance) {
        nearestDistance = distance
        nearestIndex = index
      }
    })

    const remaining = routePoints.slice(Math.min(nearestIndex + 1, routePoints.length - 1))
    const points = [riderPoint, ...remaining]
    return points.length > 1 ? points : [riderPoint, customerPoint]
  },

  useFallbackRoute(merchantPoint: MapPoint, customerPoint: MapPoint) {
    const startPoint = this.shouldAdvanceRouteByRider() ? this.data.riderPoint || merchantPoint : merchantPoint
    this.setData({
      polyline: [
        {
          points: [startPoint, customerPoint],
          color: '#1d63ff',
          width: 6,
          dottedLine: true
        }
      ]
    })
  },

  buildMarker(
    id: number,
    point: MapPoint,
    label: string,
    iconPath: string
  ): MiniMarker {
    return {
      id,
      latitude: point.latitude,
      longitude: point.longitude,
      width: 40,
      height: 40,
      iconPath,
      callout: {
        content: label,
        color: '#333',
        fontSize: 14,
        padding: 6,
        borderRadius: 12,
        display: 'ALWAYS',
        bgColor: '#fff'
      }
    }
  },

  formatRelativeTime(timeStr: string): string {
    const timestamp = new Date(timeStr).getTime()
    const diff = Date.now() - timestamp
    if (!Number.isFinite(diff) || diff < 0) return '刚刚'

    const minutes = Math.floor(diff / 60000)
    if (minutes < 1) return '刚刚'
    if (minutes < 60) return `${minutes}分钟前`

    const hours = Math.floor(minutes / 60)
    if (hours < 24) return `${hours}小时前`

    return `${Math.floor(hours / 24)}天前`
  },

  onCallRider() {
    const { riderPhone } = this.data
    if (riderPhone) {
      wx.makePhoneCall({ phoneNumber: riderPhone })
    } else {
      wx.showToast({ title: '暂无骑手电话', icon: 'none' })
    }
  },

  onNavHeight(event: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: event.detail.navBarHeight })
  }
})
