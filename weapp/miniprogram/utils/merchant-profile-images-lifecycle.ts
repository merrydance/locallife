import { updateShopImages } from '../api/onboarding'
import { ensureMerchantConsoleAccess } from './console-access'
import { logger } from './logger'
import { getStableBarHeights } from './responsive'
import {
  buildUploadRenderImages,
  clearPendingSyncFromImages,
  getCurrentMerchantIdFromStorage,
  getPendingDeletedShopImagesStorageKey,
  hasAnyImages,
  ImageItem,
  isAmbiguousSyncFailure,
  isExplicitSyncFailure,
  isLocallyTrackedShopImagePendingPersistence,
  mergeImagesWithRawUrls,
  normalizeImageRawUrl,
  PENDING_DELETED_SHOP_IMAGES_ANONYMOUS_SCOPE,
  PendingDeletedShopImagesStorageState,
  PROFILE_IMAGES_AUTO_REFRESH_WINDOW_MS,
  SHOP_IMAGES_PERSIST_RETRY_BASE_DELAY_MS,
  SHOP_IMAGES_PERSIST_RETRY_MAX_DELAY_MS,
  ShopImageKind,
  toMerchantPendingDeletedScope,
  toNormalizedRawUrls,
  toPersistedImageUrls,
  shouldAutoRefresh
} from './merchant-profile-images-view'

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type MerchantProfileImagesPageContext = WechatMiniprogram.Page.Instance<Record<string, any>, Record<string, any>> & Record<string, any>

function defineMerchantProfileImagesLifecycleMethods<T extends Record<string, unknown>>(methods: T & ThisType<MerchantProfileImagesPageContext>) {
  return methods
}

export const merchantProfileImagesLifecycleMethods = defineMerchantProfileImagesLifecycleMethods({
  resolvePendingDeletedShopImagesStorageScope(merchantId?: number) {
    const resolvedMerchantId = Number(merchantId || 0)
    if (resolvedMerchantId > 0) {
      return toMerchantPendingDeletedScope(resolvedMerchantId)
    }
    return toMerchantPendingDeletedScope(getCurrentMerchantIdFromStorage())
  },

  readPendingDeletedShopImagesStorageState(scope: string) {
    try {
      const stored = wx.getStorageSync(getPendingDeletedShopImagesStorageKey(scope)) as PendingDeletedShopImagesStorageState | null
      return {
        storefrontRawUrls: toNormalizedRawUrls(stored?.storefrontRawUrls),
        environmentRawUrls: toNormalizedRawUrls(stored?.environmentRawUrls)
      }
    } catch (err) {
      logger.warn('[ProfileImages] 读取待确认删除图片缓存失败', { scope, err })
      return { storefrontRawUrls: [] as string[], environmentRawUrls: [] as string[] }
    }
  },

  applyPendingDeletedShopImagesStorageState(state: { storefrontRawUrls?: string[], environmentRawUrls?: string[] }) {
    this._deletedStorefrontImageRawUrls = new Set(toNormalizedRawUrls(state.storefrontRawUrls))
    this._deletedEnvironmentImageRawUrls = new Set(toNormalizedRawUrls(state.environmentRawUrls))
  },

  persistPendingDeletedShopImagesStorageState() {
    if (!this._pendingDeletedShopImagesStorageScope) {
      this._pendingDeletedShopImagesStorageScope = this.resolvePendingDeletedShopImagesStorageScope()
      this._pendingDeletedShopImagesStorageKey = getPendingDeletedShopImagesStorageKey(this._pendingDeletedShopImagesStorageScope)
    }
    const storefrontRawUrls = Array.from(this._deletedStorefrontImageRawUrls)
    const environmentRawUrls = Array.from(this._deletedEnvironmentImageRawUrls)
    try {
      if (storefrontRawUrls.length === 0 && environmentRawUrls.length === 0) {
        wx.removeStorageSync(this._pendingDeletedShopImagesStorageKey)
        return
      }
      wx.setStorageSync(this._pendingDeletedShopImagesStorageKey, {
        storefrontRawUrls,
        environmentRawUrls,
        updatedAt: Date.now()
      } as PendingDeletedShopImagesStorageState)
    } catch (err) {
      logger.warn('[ProfileImages] 写入待确认删除图片缓存失败', { scope: this._pendingDeletedShopImagesStorageScope, err })
    }
  },

  restorePendingDeletedShopImagesStorageState() {
    const scope = this.resolvePendingDeletedShopImagesStorageScope()
    const state = this.readPendingDeletedShopImagesStorageState(scope)
    this._pendingDeletedShopImagesStorageScope = scope
    this._pendingDeletedShopImagesStorageKey = getPendingDeletedShopImagesStorageKey(scope)
    this.applyPendingDeletedShopImagesStorageState(state)
  },

  ensurePendingDeletedShopImagesStorageScope(merchantId?: number) {
    const nextScope = this.resolvePendingDeletedShopImagesStorageScope(merchantId)
    if (nextScope === this._pendingDeletedShopImagesStorageScope) {
      return
    }
    const previousScope = this._pendingDeletedShopImagesStorageScope
    const previousStorageKey = this._pendingDeletedShopImagesStorageKey
    const previousStorefrontRawUrls = Array.from(this._deletedStorefrontImageRawUrls)
    const previousEnvironmentRawUrls = Array.from(this._deletedEnvironmentImageRawUrls)
    const nextState = this.readPendingDeletedShopImagesStorageState(nextScope)
    const shouldMergePrevious = previousScope === PENDING_DELETED_SHOP_IMAGES_ANONYMOUS_SCOPE

    this._pendingDeletedShopImagesStorageScope = nextScope
    this._pendingDeletedShopImagesStorageKey = getPendingDeletedShopImagesStorageKey(nextScope)
    this.applyPendingDeletedShopImagesStorageState({
      storefrontRawUrls: shouldMergePrevious ? [...previousStorefrontRawUrls, ...nextState.storefrontRawUrls] : nextState.storefrontRawUrls,
      environmentRawUrls: shouldMergePrevious ? [...previousEnvironmentRawUrls, ...nextState.environmentRawUrls] : nextState.environmentRawUrls
    })
    this.persistPendingDeletedShopImagesStorageState()

    if (shouldMergePrevious && previousStorageKey && previousStorageKey !== this._pendingDeletedShopImagesStorageKey) {
      try {
        wx.removeStorageSync(previousStorageKey)
      } catch (err) {
        logger.warn('[ProfileImages] 清理匿名待确认删除图片缓存失败', { err })
      }
    }
  },

  getDeletedShopImageRawUrlSet(kind: ShopImageKind): Set<string> {
    return (kind === 'storefront'
      ? this._deletedStorefrontImageRawUrls
      : this._deletedEnvironmentImageRawUrls) as Set<string>
  },

  markDeletedShopImage(kind: ShopImageKind, image: ImageItem | null | undefined) {
    const rawUrl = normalizeImageRawUrl(image?.rawUrl)
    if (!rawUrl) {
      return
    }
    this.getDeletedShopImageRawUrlSet(kind).add(rawUrl)
    this.persistPendingDeletedShopImagesStorageState()
  },

  unmarkDeletedShopImage(kind: ShopImageKind, image: ImageItem | null | undefined) {
    const rawUrl = normalizeImageRawUrl(image?.rawUrl)
    if (!rawUrl) {
      return
    }
    this.getDeletedShopImageRawUrlSet(kind).delete(rawUrl)
    this.persistPendingDeletedShopImagesStorageState()
  },

  hasPendingDeletedShopImages() {
    return this._deletedStorefrontImageRawUrls.size > 0 || this._deletedEnvironmentImageRawUrls.size > 0
  },

  hasPendingLocalShopImages() {
    return this.data.storefrontImages.some((image: ImageItem) => isLocallyTrackedShopImagePendingPersistence(image))
      || this.data.environmentImages.some((image: ImageItem) => isLocallyTrackedShopImagePendingPersistence(image))
  },

  hasPendingShopImagesPersistence() {
    return this.hasPendingDeletedShopImages() || this.hasPendingLocalShopImages()
  },

  clearPendingDeletedShopImages() {
    if (!this.hasPendingDeletedShopImages()) {
      return
    }
    this._deletedStorefrontImageRawUrls.clear()
    this._deletedEnvironmentImageRawUrls.clear()
    this.persistPendingDeletedShopImagesStorageState()
  },

  restorePendingDeletedShopImagesToCurrentLists() {
    if (!this.hasPendingDeletedShopImages()) {
      return { storefrontImages: this.data.storefrontImages, environmentImages: this.data.environmentImages }
    }
    const storefrontImages = mergeImagesWithRawUrls(this.data.storefrontImages, Array.from(this._deletedStorefrontImageRawUrls))
    const environmentImages = mergeImagesWithRawUrls(this.data.environmentImages, Array.from(this._deletedEnvironmentImageRawUrls))
    if (storefrontImages.length === this.data.storefrontImages.length && environmentImages.length === this.data.environmentImages.length) {
      return { storefrontImages, environmentImages }
    }
    this.bumpShopImagesGeneration()
    this.setData({
      storefrontImages,
      storefrontFiles: buildUploadRenderImages(storefrontImages, this.data.storefrontFiles),
      environmentImages,
      environmentFiles: buildUploadRenderImages(environmentImages, this.data.environmentFiles),
      hasAnyImages: hasAnyImages(this.data.logoImage, storefrontImages, environmentImages)
    })
    return { storefrontImages, environmentImages }
  },

  reconcilePendingDeletedShopImageRawUrls(kind: ShopImageKind, rawUrls?: string[] | null) {
    const deletedRawUrls = this.getDeletedShopImageRawUrlSet(kind)
    if (deletedRawUrls.size === 0) {
      return false
    }
    const persistedRawUrlSet = new Set(toNormalizedRawUrls(rawUrls))
    let changed = false
    Array.from(deletedRawUrls as Set<string>).forEach((pendingRawUrl) => {
      if (!persistedRawUrlSet.has(pendingRawUrl)) {
        deletedRawUrls.delete(pendingRawUrl)
        changed = true
      }
    })
    return changed
  },

  reconcilePendingDeletedShopImages(storefrontRawUrls?: string[] | null, environmentRawUrls?: string[] | null) {
    const normalizedStorefrontRawUrls = toNormalizedRawUrls(storefrontRawUrls)
    const normalizedEnvironmentRawUrls = toNormalizedRawUrls(environmentRawUrls)
    const hasChanged = this.reconcilePendingDeletedShopImageRawUrls('storefront', normalizedStorefrontRawUrls)
      || this.reconcilePendingDeletedShopImageRawUrls('environment', normalizedEnvironmentRawUrls)

    if (hasChanged) {
      this.persistPendingDeletedShopImagesStorageState()
    }
    if (this.hasPendingDeletedShopImages()) {
      this.schedulePendingShopImagesPersistenceWithBackoff()
    } else {
      this.resetPendingShopImagesPersistenceRetryState()
    }
    return {
      storefrontRawUrls: this.filterDeletedShopImageRawUrls('storefront', normalizedStorefrontRawUrls),
      environmentRawUrls: this.filterDeletedShopImageRawUrls('environment', normalizedEnvironmentRawUrls)
    }
  },

  filterDeletedShopImageRawUrls(kind: ShopImageKind, rawUrls?: string[] | null): string[] {
    if (!Array.isArray(rawUrls) || rawUrls.length === 0) {
      return []
    }
    const deletedRawUrls = this.getDeletedShopImageRawUrlSet(kind)
    if (deletedRawUrls.size === 0) {
      return rawUrls
    }
    return rawUrls.filter((rawUrl) => {
      const normalizedRawUrl = normalizeImageRawUrl(rawUrl)
      return !!normalizedRawUrl && !deletedRawUrls.has(normalizedRawUrl)
    })
  },

  bumpShopImagesGeneration() {
    this._shopImagesGeneration += 1
    return this._shopImagesGeneration
  },

  clearPendingShopImagesPersistenceRetryTimer() {
    if (this._shopImagesPersistRetryTimer === null) {
      return
    }
    clearTimeout(this._shopImagesPersistRetryTimer)
    this._shopImagesPersistRetryTimer = null
  },

  resetPendingShopImagesPersistenceRetryState() {
    this._shopImagesPersistRetryDelayMs = 0
    this.clearPendingShopImagesPersistenceRetryTimer()
  },

  schedulePendingShopImagesPersistence(retryDelayMs = 0) {
    this._shopImagesPersistRetryPending = true
    if (retryDelayMs > 0) {
      if (this._shopImagesPersistRetryTimer !== null) {
        return
      }
      this._shopImagesPersistRetryTimer = setTimeout(() => {
        this._shopImagesPersistRetryTimer = null
        this.maybeRunPendingShopImagesPersistence()
      }, retryDelayMs) as unknown as number
      return
    }
    this.maybeRunPendingShopImagesPersistence()
  },

  schedulePendingShopImagesPersistenceWithBackoff() {
    if (!this.hasPendingShopImagesPersistence()) {
      return
    }
    this._shopImagesPersistRetryPending = true
    if (this._shopImagesPersistRetryTimer !== null) {
      return
    }
    const retryDelayMs = this._shopImagesPersistRetryDelayMs > 0 ? this._shopImagesPersistRetryDelayMs : SHOP_IMAGES_PERSIST_RETRY_BASE_DELAY_MS
    this._shopImagesPersistRetryDelayMs = Math.min(retryDelayMs * 2, SHOP_IMAGES_PERSIST_RETRY_MAX_DELAY_MS)
    this.schedulePendingShopImagesPersistence(retryDelayMs)
  },

  maybeRunPendingShopImagesPersistence() {
    if (!this._shopImagesPersistRetryPending || this._shopImagesPersistRetryRunning || this._shopImagesPersistForegroundCount > 0 || this._shopImagesPersistRetryTimer !== null) {
      return
    }
    this._shopImagesPersistRetryRunning = true
    void this.flushPendingShopImagesPersistence()
  },

  deferPendingShopImagesPersistenceCheck() {
    void Promise.resolve().then(() => {
      this.maybeRunPendingShopImagesPersistence()
    })
  },

  async convergePendingDeletedShopImagesFromServerTruth() {
    if (!this.hasPendingDeletedShopImages()) {
      return
    }
    this.restorePendingDeletedShopImagesToCurrentLists()
    this.clearPendingDeletedShopImages()
    this._shopImagesPersistRetryPending = false
    this.resetPendingShopImagesPersistenceRetryState()
    await this.loadData(false, true, true)
  },

  async flushPendingShopImagesPersistence() {
    try {
      while (this._shopImagesPersistRetryPending) {
        if (this._shopImagesPersistForegroundCount > 0) {
          return
        }
        this._shopImagesPersistRetryPending = false
        const generation = this._shopImagesGeneration
        const storefrontImages = [...this.data.storefrontImages]
        const environmentImages = [...this.data.environmentImages]
        const updated = await updateShopImages({
          storefront_images: toPersistedImageUrls(storefrontImages),
          environment_images: toPersistedImageUrls(environmentImages)
        })
        if (generation !== this._shopImagesGeneration) {
          if (this.hasPendingShopImagesPersistence()) {
            this._shopImagesPersistRetryPending = true
            this.schedulePendingShopImagesPersistenceWithBackoff()
          }
          return
        }
        this.applyShopImagesResponse(updated, clearPendingSyncFromImages(storefrontImages), clearPendingSyncFromImages(environmentImages))
      }
    } catch (err) {
      logger.warn('[ProfileImages] 重试持久化门头照/环境照失败', { err })
      if (isExplicitSyncFailure(err) && this.hasPendingDeletedShopImages()) {
        logger.warn('[ProfileImages] 后台同步收到明确失败，回退到服务端真相', { err })
        try {
          await this.convergePendingDeletedShopImagesFromServerTruth()
        } catch (convergeErr) {
          logger.warn('[ProfileImages] 回退到服务端真相失败', { err: convergeErr })
        }
        return
      }
      if (isAmbiguousSyncFailure(err) && this.hasPendingShopImagesPersistence()) {
        this.schedulePendingShopImagesPersistenceWithBackoff()
      }
    } finally {
      this._shopImagesPersistRetryRunning = false
      if (this._shopImagesPersistRetryPending && this._shopImagesPersistRetryTimer === null) {
        this.maybeRunPendingShopImagesPersistence()
      }
    }
  },

  async persistShopImagesUpdate(payload: { storefront_images?: string[], environment_images?: string[] }, localStorefrontImages: ImageItem[], localEnvironmentImages: ImageItem[]) {
    const generation = this._shopImagesGeneration
    this._shopImagesPersistForegroundCount += 1
    try {
      const updated = await updateShopImages(payload)
      if (generation !== this._shopImagesGeneration) {
        return
      }
      this.applyShopImagesResponse(updated, clearPendingSyncFromImages(localStorefrontImages), clearPendingSyncFromImages(localEnvironmentImages))
    } finally {
      this._shopImagesPersistForegroundCount = Math.max(0, this._shopImagesPersistForegroundCount - 1)
      this.deferPendingShopImagesPersistenceCheck()
    }
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.restorePendingDeletedShopImagesStorageState()

    const accessResult = await ensureMerchantConsoleAccess()
    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : ''
    })
    if (accessResult.status !== 'granted') {
      return
    }
    await this.loadData(true, true)
  },

  onUnload() {
    this.clearPendingShopImagesPersistenceRetryTimer()
  },

  onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      return
    }
    const hasPendingMutation = this.data.logoUploading || this.data.storefrontSaving || this.data.environmentSaving
    if (this.data.hasLoaded && !this.data.loading && !hasPendingMutation && shouldAutoRefresh(this.data.lastLoadedAt, PROFILE_IMAGES_AUTO_REFRESH_WINDOW_MS)) {
      this.loadData(false)
    }
  }
})