import DeliveryService, {
  buildDeliveryProgress,
  DeliveryProgressView,
  DeliveryResponse,
  getDeliveryStatusDisplay
} from '../../../api/delivery'
import { BicyclingDirectionResponse, getBicyclingDirection } from '../../../api/location'
import { getOrderDetail } from '../../../api/order'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { confirmReceiptWithRecovery } from '../../../services/order-receipt-confirmation'

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
    // 配送信息
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
    // 进度
    progress: [] as DeliveryProgressView[],
    // 位置刷新定时器
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
    // 页面重新显示时恢复轮询（副作用：也刻刷新一次位置）
    if (this.data.deliveryId && this.data.delivery && getDeliveryStatusDisplay(this.data.delivery.status).isLocationTracked) {
      this.updateRiderLocation()
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
      wx.showLoading({ title: '加载中...' })
      const order = await getOrderDetail(this.data.orderId)
      wx.hideLoading()

      const result = await confirmReceiptWithRecovery({
        orderId: this.data.orderId,
        transactionId: order.wechat_transaction_id,
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
      // 1. 获取配送信息
      const delivery = await DeliveryService.getDeliveryByOrder(this.data.orderId)

      // 2. 处理骑手信息
      const riderPhone = delivery.pickup_phone || ''
      const riderPhoneDisplay = riderPhone
        ? riderPhone.replace(/(\d{3})\d{4}(\d{4})/, '$1****$2')
        : ''
      const statusDisplay = getDeliveryStatusDisplay(delivery.status)

      // 3. 生成配送进度
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

      // 6. 开始位置追踪（配送中状态）
      if (statusDisplay.isLocationTracked) {
        this.startLocationTracking()
      }
    } catch (error: unknown) {
      logger.error('加载配送信息失败', error, 'tracking.loadDeliveryData')
      this.setData({ 
        loading: false, 
        isError: true, 
        errorMsg: getErrorMessage(error, '获取配送信息失败') 
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
      polyline: []
    })

    // 获取骑手位置
    await this.updateRiderLocation()

    // 规划路线
    this.planRoute(merchantPoint, customerPoint)
  },

  async updateRiderLocation() {
    const { deliveryId, delivery } = this.data
    if (!deliveryId || !delivery) return

    try {
      const location = await DeliveryService.getRiderLocation(deliveryId)
      if (location && location.latitude && location.longitude) {
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

        this.setData({ markers, includePoints, riderPoint })
        this.renderRoutePolyline(
          this.data.routePoints,
          this.data.merchantPoint,
          this.data.customerPoint,
          this.shouldAdvanceRouteByRider() ? riderPoint : null
        )
      }
    } catch (error) {
      logger.warn('获取骑手位置失败', error, 'tracking.updateRiderLocation')
    }
  },

  startLocationTracking() {
    // 每10秒刷新一次骑手位置
    const timer = setInterval(() => {
      this.updateRiderLocation()
    }, 10000) as unknown as number

    this.setData({ locationTimer: timer })
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
          customerPoint,
          this.shouldAdvanceRouteByRider() ? this.data.riderPoint : null
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
  },

  onRefresh() {
    this.loadDeliveryData()
  }
})
