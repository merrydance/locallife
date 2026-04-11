import DeliveryService, { Delivery, DeliveryLocationPoint } from '../../../api/delivery'
import { mapService, MapMarker, MapPoint, MapPolyline } from '../../../services/map'
import { getStableBarHeights } from '../../../utils/responsive'
import { riderLiveLocationSession, RiderLiveLocationState } from '../../../utils/rider-live-location'
import { locationService } from '../../../utils/location'
import { logger } from '../../../utils/logger'

interface RiderNavigationOptions {
  id?: string
}

type TagTheme = 'primary' | 'success' | 'warning' | 'danger' | 'default'

let riderNavigationUnsubscribe: null | (() => void) = null

function formatRelativeTime(timeStr: string): string {
  if (!timeStr) return '刚刚'

  const diff = Date.now() - new Date(timeStr).getTime()
  if (!Number.isFinite(diff) || diff < 0) return '刚刚'

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

function toMapPoint(point: DeliveryLocationPoint | null): MapPoint | null {
  if (!point) return null
  return {
    latitude: point.latitude,
    longitude: point.longitude
  }
}

Page({
  data: {
    orderId: 0,
    navBarHeight: 88,
    loading: false,
    errorMessage: '',
    delivery: null as Delivery | null,
    mapCenter: { latitude: 39.9, longitude: 116.4 },
    mapScale: 13,
    markers: [] as MapMarker[],
    polyline: [] as MapPolyline[],
    includePoints: [] as MapPoint[],
    serverTrackPoints: [] as MapPoint[],
    routeSummary: '',
    latestUpdateText: '暂无定位记录',
    pendingText: '',
    locationStatusText: '等待定位',
    locationStatusTheme: 'default' as TagTheme,
    nextStopTitle: '下一站',
    nextStopAddress: '',
    nextStopLatitude: 0,
    nextStopLongitude: 0,
    needsLocationPermission: false
  },

  onLoad(options: RiderNavigationOptions) {
    const { navBarHeight } = getStableBarHeights()
    this.setData({
      navBarHeight,
      orderId: Number(options.id || 0)
    })
    this.bindLocationSession()
    void this.loadNavigationData()
  },

  onShow() {
    const { delivery } = this.data
    if (delivery && isTrackableDelivery(delivery.status)) {
      void riderLiveLocationSession.setActiveDelivery(delivery.id, 'rider_navigation_show')
    }
  },

  onUnload() {
    if (riderNavigationUnsubscribe) {
      riderNavigationUnsubscribe()
      riderNavigationUnsubscribe = null
    }
  },

  bindLocationSession() {
    if (riderNavigationUnsubscribe) {
      riderNavigationUnsubscribe()
    }

    riderNavigationUnsubscribe = riderLiveLocationSession.subscribe((state) => {
      this.applyLocationState(state)
    })
  },

  async loadNavigationData() {
    if (!this.data.orderId) {
      this.setData({ errorMessage: '缺少订单信息' })
      return
    }

    this.setData({ loading: true, errorMessage: '' })

    try {
      const delivery = await DeliveryService.getDeliveryByOrder(this.data.orderId)
      const [latestResult, trackResult, routeResult] = await Promise.all([
        DeliveryService.getRiderLocation(delivery.id).catch(() => null),
        DeliveryService.getDeliveryTrack(delivery.id).catch(() => [] as DeliveryLocationPoint[]),
        mapService.planRoute(
          { latitude: delivery.pickup_latitude, longitude: delivery.pickup_longitude },
          { latitude: delivery.delivery_latitude, longitude: delivery.delivery_longitude }
        ).catch(() => null)
      ])

      this.setData({
        delivery,
        routeSummary: routeResult
          ? `预计总路程 ${mapService.formatDistance(routeResult.distance)} · 约 ${mapService.formatDuration(routeResult.duration)}`
          : '已展示配送主线路，可直接打开导航',
        latestUpdateText: latestResult?.recorded_at ? `最近上传 ${formatRelativeTime(latestResult.recorded_at)}` : '暂无定位记录'
      })

      if (isTrackableDelivery(delivery.status)) {
        await riderLiveLocationSession.setActiveDelivery(delivery.id, 'rider_navigation_fetch')
      }

      this.setData({
        serverTrackPoints: trackResult
          .map((point) => toMapPoint(point))
          .filter((point): point is MapPoint => !!point)
      })

      this.renderMap(delivery, latestResult)
      this.updateNextStop(delivery)
      this.applyLocationState(riderLiveLocationSession.getState())
    } catch (error) {
      logger.error('Load rider navigation failed', error, 'RiderNavigation')
      this.setData({ errorMessage: '导航页加载失败，请稍后重试' })
    } finally {
      this.setData({ loading: false })
    }
  },

  updateNextStop(delivery: Delivery) {
    const nextStop = delivery.status === 'assigned' || delivery.status === 'picking'
      ? {
          title: '下一站 · 商家',
          address: delivery.pickup_address,
          latitude: delivery.pickup_latitude,
          longitude: delivery.pickup_longitude
        }
      : {
          title: '下一站 · 顾客',
          address: delivery.delivery_address,
          latitude: delivery.delivery_latitude,
          longitude: delivery.delivery_longitude
        }

    this.setData({
      nextStopTitle: nextStop.title,
      nextStopAddress: nextStop.address,
      nextStopLatitude: nextStop.latitude,
      nextStopLongitude: nextStop.longitude
    })
  },

  renderMap(delivery: Delivery, latestResult: DeliveryLocationPoint | null) {
    const pickupPoint: MapPoint = {
      latitude: delivery.pickup_latitude,
      longitude: delivery.pickup_longitude
    }
    const deliveryPoint: MapPoint = {
      latitude: delivery.delivery_latitude,
      longitude: delivery.delivery_longitude
    }
    const riderPoint = toMapPoint(latestResult)

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
      polyline.push(mapService.createPolyline(
        this.data.serverTrackPoints,
        {
          color: '#00897B',
          width: 8,
          arrowLine: true
        }
      ))
    }

    this.setData({
      markers,
      polyline,
      includePoints,
      mapCenter: riderPoint || {
        latitude: (pickupPoint.latitude + deliveryPoint.latitude) / 2,
        longitude: (pickupPoint.longitude + deliveryPoint.longitude) / 2
      }
    })
  },

  applyLocationState(state: RiderLiveLocationState) {
    const { delivery } = this.data
    if (!delivery || state.activeDeliveryId !== delivery.id) {
      return
    }

    let locationStatusText = '等待连续定位启动'
    let locationStatusTheme: TagTheme = 'default'
    let needsLocationPermission = false

    switch (state.uploadState) {
      case 'tracking':
        locationStatusText = '定位正常'
        locationStatusTheme = 'success'
        break
      case 'uploading':
        locationStatusText = '正在上传位置'
        locationStatusTheme = 'primary'
        break
      case 'retrying':
        locationStatusText = '网络恢复后会自动补发'
        locationStatusTheme = 'warning'
        break
      case 'permission_required':
        locationStatusText = '需要开启定位权限'
        locationStatusTheme = 'danger'
        needsLocationPermission = true
        break
      case 'starting':
        locationStatusText = '正在开启连续定位'
        locationStatusTheme = 'warning'
        break
    }

    this.setData({
      latestUpdateText: state.lastUploadedAt ? `最近上传 ${formatRelativeTime(state.lastUploadedAt)}` : this.data.latestUpdateText,
      pendingText: state.pendingCount > 0 ? `待补发 ${state.pendingCount} 个定位点` : '',
      locationStatusText,
      locationStatusTheme,
      needsLocationPermission
    })

    if (state.latestPoint) {
      this.renderMap(delivery, {
        latitude: state.latestPoint.latitude,
        longitude: state.latestPoint.longitude,
        recorded_at: state.latestPoint.recordedAt
      })
    }
  },

  async onOpenNextStop() {
    if (!this.data.nextStopLatitude || !this.data.nextStopLongitude) return

    await locationService.openLocation({
      latitude: this.data.nextStopLatitude,
      longitude: this.data.nextStopLongitude,
      name: this.data.nextStopTitle,
      address: this.data.nextStopAddress,
      failMessage: '打开下一站导航失败，请稍后重试'
    })
  },

  async onOpenMerchant() {
    const { delivery } = this.data
    if (!delivery) return

    await locationService.openLocation({
      latitude: delivery.pickup_latitude,
      longitude: delivery.pickup_longitude,
      name: delivery.merchant_name || '商家',
      address: delivery.pickup_address,
      failMessage: '打开商家导航失败，请稍后重试'
    })
  },

  async onOpenCustomer() {
    const { delivery } = this.data
    if (!delivery) return

    await locationService.openLocation({
      latitude: delivery.delivery_latitude,
      longitude: delivery.delivery_longitude,
      name: delivery.delivery_contact || '顾客',
      address: delivery.delivery_address,
      failMessage: '打开顾客导航失败，请稍后重试'
    })
  },

  async onRetryLocation() {
    wx.showLoading({ title: '刷新定位中...' })
    try {
      if (this.data.needsLocationPermission) {
        const granted = await riderLiveLocationSession.requestPermissionAndRestart()
        if (!granted) {
          wx.showToast({ title: '请先开启定位权限', icon: 'none' })
          return
        }
      } else {
        await riderLiveLocationSession.refreshNow('rider_navigation_manual_retry')
        await riderLiveLocationSession.flushNow()
      }
    } finally {
      wx.hideLoading()
    }
  },

  onRetry() {
    void this.loadNavigationData()
  }
})