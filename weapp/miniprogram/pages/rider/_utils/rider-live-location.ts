import RiderService from '../_main_shared/api/rider'
import { networkMonitor } from '../../../utils/network-monitor'
import { locationService } from '../../../utils/location'
import { logger } from '../../../utils/logger'
import { normalizeLocationError } from '../_main_shared/utils/rider-location'
import { resolveCurrentRegionId } from '../_main_shared/utils/current-region'

type UploadState = 'idle' | 'starting' | 'tracking' | 'uploading' | 'retrying' | 'permission_required' | 'stopped'
type SessionListener = (state: RiderLiveLocationState) => void

interface PendingLocationPoint {
  delivery_id: number
  longitude: number
  latitude: number
  recorded_at: string
  source?: string
  accuracy?: number
  speed?: number
  heading?: number
}

interface LiveLocationPoint {
  latitude: number
  longitude: number
  recordedAt: string
}

interface LiveLocationChangeResult {
  latitude: number
  longitude: number
  accuracy?: number
  speed?: number
}

export interface RiderLiveLocationState {
  activeDeliveryId: number | null
  isRunning: boolean
  uploadState: UploadState
  pendingCount: number
  permissionGranted: boolean | null
  lastLocalAt: string
  lastUploadedAt: string
  lastError: string
  latestPoint: LiveLocationPoint | null
}

const STORAGE_KEY = 'rider_live_location_queue_v1'
const FLUSH_INTERVAL_MS = 15000
const MIN_SAMPLE_INTERVAL_MS = 10000
const MIN_SAMPLE_DISTANCE_METERS = 30
const MAX_PENDING_POINTS = 30
const BACKEND_LOCATION_MAX_PAST_MS = 60 * 60 * 1000
const STALE_LOCATION_QUEUE_MARGIN_MS = 5 * 60 * 1000
const MAX_STORED_LOCATION_AGE_MS = BACKEND_LOCATION_MAX_PAST_MS - STALE_LOCATION_QUEUE_MARGIN_MS

function isFreshPendingLocationPoint(point: PendingLocationPoint): boolean {
  const recordedAt = Date.parse(point.recorded_at)
  if (!Number.isFinite(recordedAt)) {
    return false
  }

  return recordedAt >= Date.now() - MAX_STORED_LOCATION_AGE_MS
}

function getDistanceMeters(lat1: number, lng1: number, lat2: number, lng2: number): number {
  const radius = 6371000
  const phi1 = lat1 * Math.PI / 180
  const phi2 = lat2 * Math.PI / 180
  const deltaPhi = (lat2 - lat1) * Math.PI / 180
  const deltaLambda = (lng2 - lng1) * Math.PI / 180

  const value = Math.sin(deltaPhi / 2) * Math.sin(deltaPhi / 2) +
    Math.cos(phi1) * Math.cos(phi2) * Math.sin(deltaLambda / 2) * Math.sin(deltaLambda / 2)
  const angle = 2 * Math.atan2(Math.sqrt(value), Math.sqrt(1 - value))

  return radius * angle
}

class RiderLiveLocationSession {
  private state: RiderLiveLocationState = {
    activeDeliveryId: null,
    isRunning: false,
    uploadState: 'idle',
    pendingCount: 0,
    permissionGranted: null,
    lastLocalAt: '',
    lastUploadedAt: '',
    lastError: '',
    latestPoint: null
  }

  private listeners = new Set<SessionListener>()
  private queue: PendingLocationPoint[] = []
  private lastQueuedAt = 0
  private latestQueuedPoint: LiveLocationPoint | null = null
  private flushTimer: number | null = null
  private locationListenerBound = false
  private isFlushing = false

  constructor() {
    this.queue = this.loadQueue()
    this.state.pendingCount = this.queue.length

    networkMonitor.subscribe((networkState) => {
      if (networkState.isConnected) {
        void this.flushNow()
      }
    })
  }

  subscribe(listener: SessionListener): () => void {
    this.listeners.add(listener)
    listener(this.getState())

    return () => {
      this.listeners.delete(listener)
    }
  }

  getState(): RiderLiveLocationState {
    return { ...this.state }
  }

  async setActiveDelivery(deliveryId: number | null, source: string = 'session'): Promise<void> {
    if (!deliveryId) {
      this.stopTracking('no_active_delivery')
      return
    }

    const switchedDelivery = this.state.activeDeliveryId !== null && this.state.activeDeliveryId !== deliveryId
    const queueBelongsToOtherDelivery = this.queue.some((item) => item.delivery_id !== deliveryId)
    if (switchedDelivery || queueBelongsToOtherDelivery) {
      this.clearQueue('delivery_changed')
    }

    this.state.activeDeliveryId = deliveryId
    this.notify()

    if (!this.state.isRunning || switchedDelivery) {
      await this.startTracking(source)
      return
    }

    if (this.queue.length > 0) {
      void this.flushNow()
    }
  }

  async refreshNow(source: string = 'manual_refresh'): Promise<void> {
    if (!this.state.activeDeliveryId) {
      return
    }

    await this.captureCurrentLocation(source)
  }

  async flushNow(): Promise<void> {
    if (this.isFlushing || this.queue.length === 0 || !this.state.activeDeliveryId) {
      return
    }

    this.pruneStaleQueue('before_flush')
    if (this.queue.length === 0) {
      this.state.uploadState = this.state.isRunning ? 'tracking' : 'idle'
      this.notify()
      return
    }

    if (!networkMonitor.isOnline()) {
      this.state.uploadState = this.state.isRunning ? 'retrying' : 'stopped'
      this.state.lastError = this.state.lastError || '网络恢复后会自动补发定位'
      this.notify()
      return
    }

    this.isFlushing = true
    const batch = [...this.queue]
    this.state.uploadState = 'uploading'
    this.notify()

    try {
      const regionId = await resolveCurrentRegionId()
      await RiderService.updateLocation(regionId, batch)
      this.queue.splice(0, batch.length)
      this.persistQueue()

      const latest = batch[batch.length - 1]
      this.state.lastUploadedAt = latest.recorded_at
      this.state.lastError = ''
      this.state.pendingCount = this.queue.length
      this.state.uploadState = this.state.isRunning ? 'tracking' : 'idle'
      this.notify()
    } catch (error) {
      const normalized = normalizeLocationError(error)
      this.state.lastError = normalized.message
      this.state.pendingCount = this.queue.length
      this.state.uploadState = this.state.isRunning ? 'retrying' : 'stopped'
      this.notify()
      logger.warn('Flush rider live location failed', { error, pendingCount: this.queue.length }, 'RiderLiveLocationSession')
    } finally {
      this.isFlushing = false
    }
  }

  async requestPermissionAndRestart(): Promise<boolean> {
    const granted = await locationService.requestLocationPermission()
    this.state.permissionGranted = granted

    if (!granted) {
      this.state.uploadState = 'permission_required'
      this.state.lastError = '请先开启定位权限'
      this.notify()
      return false
    }

    if (this.state.activeDeliveryId) {
      await this.startTracking('permission_granted')
    }

    return true
  }

  private async startTracking(source: string): Promise<void> {
    if (!this.state.activeDeliveryId) {
      return
    }

    this.bindLocationListener()
    this.ensureFlushTimer()

    this.state.isRunning = true
    this.state.uploadState = 'starting'
    this.state.lastError = ''
    this.notify()

    try {
      await this.startHardwareLocation()
      this.state.permissionGranted = true
      this.state.uploadState = 'tracking'
      this.notify()
      await this.captureCurrentLocation(`${source}_bootstrap`)
      void this.flushNow()
    } catch (error) {
      const normalized = normalizeLocationError(error)
      this.state.isRunning = false
      this.state.permissionGranted = false
      this.state.uploadState = 'permission_required'
      this.state.lastError = normalized.message
      this.notify()
      logger.warn('Start rider live location failed', { error }, 'RiderLiveLocationSession')
    }
  }

  private stopTracking(reason: string): void {
    this.state.activeDeliveryId = null
    this.state.isRunning = false
    this.state.uploadState = 'stopped'
    this.state.lastError = ''
    this.state.latestPoint = null
    this.latestQueuedPoint = null
    this.lastQueuedAt = 0
    this.clearQueue(reason)
    this.clearFlushTimer()

    if (typeof wx.stopLocationUpdate === 'function') {
      wx.stopLocationUpdate({
        fail: (error) => {
          logger.warn('Stop location update failed', error, 'RiderLiveLocationSession')
        }
      })
    }

    this.notify()
  }

  private bindLocationListener(): void {
    if (this.locationListenerBound) {
      return
    }

    wx.onLocationChange((location) => {
      void this.handleLocationChange(location as LiveLocationChangeResult)
    })
    this.locationListenerBound = true
  }

  private ensureFlushTimer(): void {
    if (this.flushTimer !== null) {
      return
    }

    this.flushTimer = setInterval(() => {
      void this.flushNow()
    }, FLUSH_INTERVAL_MS) as unknown as number
  }

  private clearFlushTimer(): void {
    if (this.flushTimer === null) {
      return
    }

    clearInterval(this.flushTimer)
    this.flushTimer = null
  }

  private async startHardwareLocation(): Promise<void> {
    await new Promise<void>((resolve, reject) => {
      if (typeof wx.startLocationUpdate !== 'function') {
        reject(new Error('当前微信版本不支持持续定位'))
        return
      }

      wx.startLocationUpdate({
        success: () => resolve(),
        fail: (error) => reject(error)
      })
    })
  }

  private async captureCurrentLocation(source: string): Promise<void> {
    if (!this.state.activeDeliveryId) {
      return
    }

    try {
      const location = await locationService.getCurrentLocation()
      this.enqueuePoint({
        delivery_id: this.state.activeDeliveryId,
        latitude: location.latitude,
        longitude: location.longitude,
        recorded_at: new Date().toISOString(),
        source
      })
    } catch (error) {
      throw normalizeLocationError(error)
    }
  }

  private async handleLocationChange(location: LiveLocationChangeResult): Promise<void> {
    if (!this.state.activeDeliveryId || !this.state.isRunning) {
      return
    }

    this.enqueuePoint({
      delivery_id: this.state.activeDeliveryId,
      latitude: location.latitude,
      longitude: location.longitude,
      recorded_at: new Date().toISOString(),
      source: 'live_tracking',
      accuracy: typeof location.accuracy === 'number' ? location.accuracy : undefined,
      speed: typeof location.speed === 'number' ? location.speed : undefined
    })

    if (this.queue.length >= 3) {
      void this.flushNow()
    }
  }

  private enqueuePoint(point: PendingLocationPoint): void {
    const now = Date.now()
    const previousPoint = this.latestQueuedPoint

    if (previousPoint) {
      const distance = getDistanceMeters(
        previousPoint.latitude,
        previousPoint.longitude,
        point.latitude,
        point.longitude
      )
      if (now - this.lastQueuedAt < MIN_SAMPLE_INTERVAL_MS && distance < MIN_SAMPLE_DISTANCE_METERS) {
        return
      }
    }

    this.queue.push(point)
    if (this.queue.length > MAX_PENDING_POINTS) {
      this.queue.splice(0, this.queue.length - MAX_PENDING_POINTS)
    }

    this.latestQueuedPoint = {
      latitude: point.latitude,
      longitude: point.longitude,
      recordedAt: point.recorded_at
    }
    this.lastQueuedAt = now

    this.state.latestPoint = this.latestQueuedPoint
    this.state.lastLocalAt = point.recorded_at
    this.state.pendingCount = this.queue.length
    this.state.lastError = ''
    if (this.state.uploadState !== 'uploading') {
      this.state.uploadState = networkMonitor.isOnline() ? 'tracking' : 'retrying'
    }

    this.persistQueue()
    this.notify()
  }

  private persistQueue(): void {
    wx.setStorageSync(STORAGE_KEY, this.queue)
  }

  private loadQueue(): PendingLocationPoint[] {
    try {
      const stored = wx.getStorageSync(STORAGE_KEY)
      if (!Array.isArray(stored)) {
        return []
      }

      const validItems = stored.filter((item) => item && typeof item === 'object') as PendingLocationPoint[]
      const freshItems = validItems.filter(isFreshPendingLocationPoint)
      if (freshItems.length !== validItems.length) {
        wx.setStorageSync(STORAGE_KEY, freshItems)
      }

      return freshItems
    } catch (error) {
      logger.warn('Load rider live location queue failed', error, 'RiderLiveLocationSession')
      return []
    }
  }

  private pruneStaleQueue(reason: string): void {
    const before = this.queue.length
    if (before === 0) {
      return
    }

    this.queue = this.queue.filter(isFreshPendingLocationPoint)
    const removed = before - this.queue.length
    if (removed === 0) {
      return
    }

    logger.info('Prune stale rider live location queue points', { reason, removed }, 'RiderLiveLocationSession')
    this.state.pendingCount = this.queue.length
    this.persistQueue()
  }

  private clearQueue(reason: string): void {
    if (this.queue.length > 0) {
      logger.info('Clear rider live location queue', { reason, count: this.queue.length }, 'RiderLiveLocationSession')
    }

    this.queue = []
    this.state.pendingCount = 0
    this.persistQueue()
  }

  private notify(): void {
    const snapshot = this.getState()
    this.listeners.forEach((listener) => {
      listener(snapshot)
    })
  }
}

export const riderLiveLocationSession = new RiderLiveLocationSession()
