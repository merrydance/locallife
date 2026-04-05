import { getStableBarHeights } from '../../../utils/responsive'
import { logger } from '../../../utils/logger'
import { uploadMerchantImage, getMerchantApplication, updateShopImages, waitForPublicMediaDisplayUrl } from '../../../api/onboarding'
import { getMyMerchantProfile, updateMyMerchantLogo } from '../../../api/merchant'
import { ensureMerchantConsoleAccess } from '../../../utils/console-access'
import { getPublicImageUrl } from '../../../utils/image'
import { getErrorUserMessage } from '../../../utils/user-facing'

type ImageItem = { url: string, rawUrl?: string, assetId?: number, localFileUrl?: string, pendingSync?: boolean, status?: 'loading' | 'done' | 'failed' | 'reload' }
type UploadFileItem = { url?: string | null }
type ShopImagesResponse = { storefront_images?: string[] | null, environment_images?: string[] | null }
type ShopImageKind = 'storefront' | 'environment'
type CurrentMerchantCache = { id?: number, merchant_id?: number }
type PendingDeletedShopImagesStorageState = {
  storefrontRawUrls?: string[]
  environmentRawUrls?: string[]
  updatedAt?: number
}
type ErrorWithSyncStatus = {
  statusCode?: unknown
  code?: unknown
  message?: unknown
  errMsg?: unknown
  userMessage?: unknown
  detailMessage?: unknown
  data?: {
    message?: unknown
  }
  body?: {
    message?: unknown
  }
  originalError?: {
    message?: unknown
    errMsg?: unknown
  }
}

const PROFILE_IMAGES_AUTO_REFRESH_WINDOW_MS = 60 * 1000
const CONTINUE_SYNC_TOAST = '正在继续同步，请稍后刷新查看'
const PENDING_DELETED_SHOP_IMAGES_STORAGE_PREFIX = 'merchant_profile_images_pending_deleted_v1'
const PENDING_DELETED_SHOP_IMAGES_ANONYMOUS_SCOPE = 'anonymous'
const SHOP_IMAGES_PERSIST_RETRY_BASE_DELAY_MS = 1500
const SHOP_IMAGES_PERSIST_RETRY_MAX_DELAY_MS = 30000

function normalizeImageRawUrl(rawUrl?: string | null): string {
  return typeof rawUrl === 'string' ? rawUrl.trim() : ''
}

function normalizeComparableImageUrl(url?: string | null): string {
  if (typeof url !== 'string') {
    return ''
  }

  const trimmedUrl = url.trim()
  if (!trimmedUrl) {
    return ''
  }

  return getPublicImageUrl(trimmedUrl) || trimmedUrl
}

function buildImageRawUrlSet(images: ImageItem[]): Set<string> {
  return new Set(
    images
      .map((image) => normalizeImageRawUrl(image.rawUrl))
      .filter((rawUrl) => rawUrl.length > 0)
  )
}

function buildImageComparableUrlSet(images: ImageItem[]): Set<string> {
  const comparableUrls = new Set<string>()

  images.forEach((image) => {
    const imageUrl = normalizeComparableImageUrl(image.url)
    if (imageUrl) {
      comparableUrls.add(imageUrl)
    }

    const rawUrl = normalizeComparableImageUrl(image.rawUrl)
    if (rawUrl) {
      comparableUrls.add(rawUrl)
    }
  })

  return comparableUrls
}

function toNormalizedRawUrls(rawUrls?: Array<string | null | undefined> | null): string[] {
  return Array.from(new Set(
    (Array.isArray(rawUrls) ? rawUrls : [])
      .map((rawUrl) => normalizeImageRawUrl(rawUrl))
      .filter((rawUrl) => rawUrl.length > 0)
  ))
}

function toMerchantPendingDeletedScope(merchantId: number): string {
  return merchantId > 0 ? `merchant_${merchantId}` : PENDING_DELETED_SHOP_IMAGES_ANONYMOUS_SCOPE
}

function getPendingDeletedShopImagesStorageKey(scope: string): string {
  return `${PENDING_DELETED_SHOP_IMAGES_STORAGE_PREFIX}:${scope}`
}

function getCurrentMerchantIdFromStorage(): number {
  try {
    const cached = wx.getStorageSync('current_merchant') as CurrentMerchantCache | null
    const merchantId = Number(cached?.merchant_id || cached?.id || 0)
    return merchantId > 0 ? merchantId : 0
  } catch (err) {
    logger.warn('[ProfileImages] 读取 current_merchant 失败', { err })
    return 0
  }
}

function shouldKeepLocalImage(
  image: ImageItem | null | undefined,
  persistedRawUrls: Set<string>,
  persistedComparableUrls: Set<string>
): image is ImageItem {
  if (!isLocallyTrackedShopImagePendingPersistence(image)) {
    return false
  }

  if (!image?.assetId) return false
  const rawUrl = normalizeImageRawUrl(image.rawUrl)
  if (rawUrl && persistedRawUrls.has(rawUrl)) {
    return false
  }

  const comparableUrls = [image.url, image.rawUrl]
    .map((url) => normalizeComparableImageUrl(url))
    .filter((url) => url.length > 0)

  return comparableUrls.every((url) => !persistedComparableUrls.has(url))
}

function mergeServerAndLocalImages(persistedImages: ImageItem[], localImages: ImageItem[]): ImageItem[] {
  const persistedRawUrls = buildImageRawUrlSet(persistedImages)
  const persistedComparableUrls = buildImageComparableUrlSet(persistedImages)
  const mergedImages = [...persistedImages]

  localImages.forEach((image) => {
    if (!shouldKeepLocalImage(image, persistedRawUrls, persistedComparableUrls)) {
      return
    }
    if (image.assetId && mergedImages.some((item) => item.assetId === image.assetId)) {
      return
    }
    mergedImages.push({ ...image })
  })

  return mergedImages
}

function mergeServerAndLocalLogo(persistedLogo: ImageItem | null, localLogo: ImageItem | null): ImageItem | null {
  if (persistedLogo) {
    return persistedLogo
  }
  if (!localLogo?.assetId) {
    return null
  }
  return { ...localLogo }
}

function isPendingImage(image: ImageItem | null | undefined): image is ImageItem & { assetId: number } {
  return !!image?.assetId && !normalizeImageRawUrl(image.rawUrl)
}

function isLocallyTrackedShopImagePendingPersistence(image: ImageItem | null | undefined): boolean {
  if (!image?.assetId) {
    return false
  }

  return !!image.pendingSync || !!image.localFileUrl || !normalizeImageRawUrl(image.rawUrl)
}

function toPersistedImageUrls(images: ImageItem[]): string[] {
  return images
    .map((image) => image.rawUrl)
    .filter((url): url is string => typeof url === 'string' && url.trim().length > 0)
}

function hasAnyImages(logoImage: ImageItem | null, storefrontImages: ImageItem[], environmentImages: ImageItem[]) {
  return !!logoImage || storefrontImages.length > 0 || environmentImages.length > 0
}

const getErrorMessage = getErrorUserMessage

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

function normalizeSyncErrorText(value: unknown): string {
  return typeof value === 'string' ? value.replace(/\s+/g, ' ').trim().toLowerCase() : ''
}

function getSyncErrorStatusCode(error: unknown): number | undefined {
  if (!error || typeof error !== 'object') {
    return undefined
  }

  const knownError = error as ErrorWithSyncStatus
  const candidates = [knownError.statusCode, knownError.code]

  for (const candidate of candidates) {
    const numericCode = typeof candidate === 'number' ? candidate : Number(candidate)
    if (Number.isFinite(numericCode)) {
      return numericCode
    }
  }

  return undefined
}

function isAmbiguousSyncFailure(error: unknown): boolean {
  const statusCode = getSyncErrorStatusCode(error)
  if (statusCode === 408 || statusCode === 429) {
    return true
  }

  if (typeof statusCode === 'number' && statusCode >= 500 && statusCode < 600) {
    return true
  }

  if (!error || typeof error !== 'object') {
    return false
  }

  const knownError = error as ErrorWithSyncStatus
  const normalizedText = [
    knownError.message,
    knownError.errMsg,
    knownError.userMessage,
    knownError.detailMessage,
    knownError.data?.message,
    knownError.body?.message,
    knownError.originalError?.message,
    knownError.originalError?.errMsg
  ]
    .map(normalizeSyncErrorText)
    .filter((text) => text.length > 0)
    .join(' ')

  if (!normalizedText) {
    return false
  }

  return [
    'request:fail',
    'too many requests',
    'rate limit exceeded',
    '请求太频繁',
    'network error',
    'network request failed',
    'offline',
    'timeout',
    'timed out',
    'bad gateway',
    'gateway timeout',
    'service unavailable',
    '网络请求失败',
    '服务暂时不可用',
    '服务器内部错误'
  ].some((marker) => normalizedText.includes(marker))
}

function isExplicitSyncFailure(error: unknown): boolean {
  const statusCode = getSyncErrorStatusCode(error)
  return typeof statusCode === 'number' && statusCode >= 400 && statusCode < 500 && statusCode !== 408 && statusCode !== 429
}

function toImageItems(rawUrls?: string[] | null): ImageItem[] {
  if (!Array.isArray(rawUrls)) return []
  return rawUrls
    .map((rawUrl) => ({
      url: getPublicImageUrl(rawUrl),
      rawUrl
    }))
    .filter((item) => !!item.url)
}

function mergeImagesWithRawUrls(images: ImageItem[], rawUrls?: Array<string | null | undefined> | null): ImageItem[] {
  const mergedImages = [...images]
  const existingRawUrls = buildImageRawUrlSet(images)

  toNormalizedRawUrls(rawUrls).forEach((rawUrl) => {
    if (existingRawUrls.has(rawUrl)) {
      return
    }

    const url = getPublicImageUrl(rawUrl)
    if (!url) {
      return
    }

    mergedImages.push({ url, rawUrl })
    existingRawUrls.add(rawUrl)
  })

  return mergedImages
}

function buildControlledUploadImages(existingImages: ImageItem[], files: UploadFileItem[]) {
  const newImages = files.reduce<ImageItem[]>((result, file) => {
    const fileUrl = typeof file?.url === 'string' ? file.url.trim() : ''
    if (!fileUrl) {
      return result
    }

    result.push({ url: fileUrl, localFileUrl: fileUrl, pendingSync: true })
    return result
  }, [])

  return {
    controlledImages: [...existingImages, ...newImages],
    newLocalFileUrls: newImages
      .map((image) => image.localFileUrl || image.url)
      .filter((fileUrl): fileUrl is string => typeof fileUrl === 'string' && fileUrl.length > 0)
  }
}

function findPendingUploadImageIndex(images: ImageItem[], localFileUrl: string): number {
  return images.findIndex((image) => !image.assetId && image.localFileUrl === localFileUrl)
}

function replaceImageAt(images: ImageItem[], index: number, image: ImageItem): ImageItem[] {
  if (index < 0 || index >= images.length) {
    return images
  }

  const nextImages = [...images]
  nextImages[index] = image
  return nextImages
}

function removeImageAt(images: ImageItem[], index: number): ImageItem[] {
  if (index < 0 || index >= images.length) {
    return images
  }

  const nextImages = [...images]
  nextImages.splice(index, 1)
  return nextImages
}

function clearPendingSyncFromImages(images: ImageItem[]): ImageItem[] {
  return images.map((image) => {
    if (!image.pendingSync && !image.localFileUrl) {
      return image
    }

    if (!normalizeImageRawUrl(image.rawUrl)) {
      return image
    }

    return {
      ...image,
      pendingSync: false,
      localFileUrl: undefined
    }
  })
}

function buildLogoUploadFiles(logoImage: ImageItem | null): ImageItem[] {
  return buildUploadRenderImages(logoImage ? [logoImage] : [])
}

function isSameImageIdentity(left: ImageItem | null | undefined, right: ImageItem | null | undefined): boolean {
  if (!left || !right) {
    return false
  }

  if (left.assetId && right.assetId && left.assetId === right.assetId) {
    return true
  }

  const leftRawUrl = normalizeImageRawUrl(left.rawUrl)
  const rightRawUrl = normalizeImageRawUrl(right.rawUrl)
  if (leftRawUrl && rightRawUrl && leftRawUrl === rightRawUrl) {
    return true
  }

  const leftComparableUrls = [left.url, left.rawUrl]
    .map((url) => normalizeComparableImageUrl(url))
    .filter((url) => url.length > 0)
  const rightComparableUrls = [right.url, right.rawUrl]
    .map((url) => normalizeComparableImageUrl(url))
    .filter((url) => url.length > 0)

  return leftComparableUrls.some((url) => rightComparableUrls.includes(url))
}

function buildUploadRenderImages(images: ImageItem[], previousFiles: ImageItem[] = []): ImageItem[] {
  const nextFiles: ImageItem[] = []

  images.forEach((image) => {
    const matchedPreviousFile = previousFiles.find((previousFile) => isSameImageIdentity(previousFile, image))
    const visibleUrl = matchedPreviousFile?.url || image.localFileUrl || image.url
    const status: ImageItem['status'] = isLocallyTrackedShopImagePendingPersistence(image) ? 'loading' : 'done'

    if (!visibleUrl) {
      return
    }

    if (matchedPreviousFile && matchedPreviousFile.url === visibleUrl && matchedPreviousFile.status === status) {
      nextFiles.push(matchedPreviousFile)
      return
    }

    nextFiles.push({ url: visibleUrl, status })
  })

  return nextFiles
}

function areImageItemsEqual(left: ImageItem | null | undefined, right: ImageItem | null | undefined): boolean {
  if (!left && !right) {
    return true
  }

  if (!left || !right) {
    return false
  }

  return left.url === right.url
    && left.rawUrl === right.rawUrl
    && left.assetId === right.assetId
    && left.localFileUrl === right.localFileUrl
    && left.pendingSync === right.pendingSync
}

function areImageListsEqual(left: ImageItem[], right: ImageItem[]): boolean {
  if (left === right) {
    return true
  }

  if (left.length !== right.length) {
    return false
  }

  return left.every((image, index) => areImageItemsEqual(image, right[index]))
}

function areUploadRenderImagesEqual(left: ImageItem[], right: ImageItem[]): boolean {
  if (left === right) {
    return true
  }

  if (left.length !== right.length) {
    return false
  }

  return left.every((image, index) => image.url === right[index]?.url)
}

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    loading: false,
    initialError: false,
    initialErrorMessage: '',
    hasLoaded: false,
    hasAnyImages: false,
    lastLoadedAt: 0,

    // Logo（单张）
    logoImage: null as ImageItem | null,
    logoFiles: [] as ImageItem[],
    logoUploading: false,

    // 门头照（最多3张）
    storefrontImages: [] as ImageItem[],
    storefrontFiles: [] as ImageItem[],
    storefrontSaving: false,

    // 环境照（最多5张）
    environmentImages: [] as ImageItem[],
    environmentFiles: [] as ImageItem[],
    environmentSaving: false,

    // 商户版本号（乐观锁，logo更新时使用）
    _merchantVersion: 0
  },

  _shopImagesGeneration: 0,
  _shopImagesPersistForegroundCount: 0,
  _shopImagesPersistRetryPending: false,
  _shopImagesPersistRetryRunning: false,
  _shopImagesPersistRetryDelayMs: 0,
  _shopImagesPersistRetryTimer: null as number | null,
  _deletedStorefrontImageRawUrls: new Set<string>(),
  _deletedEnvironmentImageRawUrls: new Set<string>(),
  _pendingDeletedShopImagesStorageScope: '',
  _pendingDeletedShopImagesStorageKey: '',

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
      return {
        storefrontRawUrls: [] as string[],
        environmentRawUrls: [] as string[]
      }
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
      logger.warn('[ProfileImages] 写入待确认删除图片缓存失败', {
        scope: this._pendingDeletedShopImagesStorageScope,
        err
      })
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
      storefrontRawUrls: shouldMergePrevious
        ? [...previousStorefrontRawUrls, ...nextState.storefrontRawUrls]
        : nextState.storefrontRawUrls,
      environmentRawUrls: shouldMergePrevious
        ? [...previousEnvironmentRawUrls, ...nextState.environmentRawUrls]
        : nextState.environmentRawUrls
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

  getDeletedShopImageRawUrlSet(kind: ShopImageKind) {
    return kind === 'storefront'
      ? this._deletedStorefrontImageRawUrls
      : this._deletedEnvironmentImageRawUrls
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
    return this.data.storefrontImages.some((image) => isLocallyTrackedShopImagePendingPersistence(image))
      || this.data.environmentImages.some((image) => isLocallyTrackedShopImagePendingPersistence(image))
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
      return {
        storefrontImages: this.data.storefrontImages,
        environmentImages: this.data.environmentImages
      }
    }

    const storefrontImages = mergeImagesWithRawUrls(
      this.data.storefrontImages,
      Array.from(this._deletedStorefrontImageRawUrls)
    )
    const environmentImages = mergeImagesWithRawUrls(
      this.data.environmentImages,
      Array.from(this._deletedEnvironmentImageRawUrls)
    )

    if (
      storefrontImages.length === this.data.storefrontImages.length
      && environmentImages.length === this.data.environmentImages.length
    ) {
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

    Array.from(deletedRawUrls).forEach((pendingRawUrl) => {
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

    const retryDelayMs = this._shopImagesPersistRetryDelayMs > 0
      ? this._shopImagesPersistRetryDelayMs
      : SHOP_IMAGES_PERSIST_RETRY_BASE_DELAY_MS

    this._shopImagesPersistRetryDelayMs = Math.min(
      retryDelayMs * 2,
      SHOP_IMAGES_PERSIST_RETRY_MAX_DELAY_MS
    )

    this.schedulePendingShopImagesPersistence(retryDelayMs)
  },

  maybeRunPendingShopImagesPersistence() {
    if (
      !this._shopImagesPersistRetryPending
      || this._shopImagesPersistRetryRunning
      || this._shopImagesPersistForegroundCount > 0
      || this._shopImagesPersistRetryTimer !== null
    ) {
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

        this.applyShopImagesResponse(
          updated,
          clearPendingSyncFromImages(storefrontImages),
          clearPendingSyncFromImages(environmentImages)
        )
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

  async persistShopImagesUpdate(
    payload: { storefront_images?: string[], environment_images?: string[] },
    localStorefrontImages: ImageItem[],
    localEnvironmentImages: ImageItem[]
  ) {
    const generation = this._shopImagesGeneration
    this._shopImagesPersistForegroundCount += 1

    try {
      const updated = await updateShopImages(payload)

      if (generation !== this._shopImagesGeneration) {
        // A newer local mutation already superseded this request snapshot.
        // Let the newer foreground path or the next explicit retry decision own persistence
        // instead of immediately firing a duplicate PATCH.
        return
      }

      this.applyShopImagesResponse(
        updated,
        clearPendingSyncFromImages(localStorefrontImages),
        clearPendingSyncFromImages(localEnvironmentImages)
      )
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
    if (this.data.hasLoaded && !this.data.loading && !hasPendingMutation) {
      if (shouldAutoRefresh(this.data.lastLoadedAt, PROFILE_IMAGES_AUTO_REFRESH_WINDOW_MS)) {
        this.loadData(false)
      }
    }
  },

  // ==================== 数据加载 ====================

  async loadData(_showLoading = true, force = false, strictApplication = false) {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      return
    }

    if (this.data.loading) {
      return
    }

    const hasExistingData = this.data.hasLoaded
    if (!force && hasExistingData && !shouldAutoRefresh(this.data.lastLoadedAt, PROFILE_IMAGES_AUTO_REFRESH_WINDOW_MS)) {
      return
    }

    this.setData({ loading: true })
    try {
      const currentLogoImage = this.data.logoImage
      const requestShopImagesGeneration = this._shopImagesGeneration
      const applicationRequest = strictApplication
        ? getMerchantApplication()
        : getMerchantApplication().catch(() => null)
      const [merchant, application] = await Promise.all([
        getMyMerchantProfile(),
        applicationRequest
      ])

      this.ensurePendingDeletedShopImagesStorageScope(Number(merchant.id || 0))

      // Logo
      let logoImage: ImageItem | null = null
      if (merchant.logo_url) {
        const displayUrl = getPublicImageUrl(merchant.logo_url)
        if (displayUrl) {
          logoImage = { url: displayUrl, rawUrl: merchant.logo_url }
        }
      }

      const latestStorefrontImages = [...this.data.storefrontImages]
      const latestEnvironmentImages = [...this.data.environmentImages]
      const shopImagesGenerationChanged = requestShopImagesGeneration !== this._shopImagesGeneration
      const applicationUnavailable = !application

      if (applicationUnavailable && this.hasPendingDeletedShopImages()) {
        this.schedulePendingShopImagesPersistenceWithBackoff()
      }

      const reconciledShopImages = applicationUnavailable
        ? null
        : this.reconcilePendingDeletedShopImages(
          application.storefront_images,
          application.environment_images
        )

      // 门头照
      const persistedStorefrontImages = applicationUnavailable
        ? toImageItems(toPersistedImageUrls(latestStorefrontImages))
        : toImageItems(reconciledShopImages?.storefrontRawUrls)

      // 环境照
      const persistedEnvironmentImages = applicationUnavailable
        ? toImageItems(toPersistedImageUrls(latestEnvironmentImages))
        : toImageItems(reconciledShopImages?.environmentRawUrls)

      const mergedLogoImage = mergeServerAndLocalLogo(logoImage, currentLogoImage)
      const mergedStorefrontImages = shopImagesGenerationChanged
        ? latestStorefrontImages
        : applicationUnavailable
          ? latestStorefrontImages
        : mergeServerAndLocalImages(persistedStorefrontImages, latestStorefrontImages)
      const mergedEnvironmentImages = shopImagesGenerationChanged
        ? latestEnvironmentImages
        : applicationUnavailable
          ? latestEnvironmentImages
        : mergeServerAndLocalImages(persistedEnvironmentImages, latestEnvironmentImages)

      this.bumpShopImagesGeneration()

      this.setData({
        initialError: false,
        initialErrorMessage: '',
        hasLoaded: true,
        hasAnyImages: hasAnyImages(mergedLogoImage, mergedStorefrontImages, mergedEnvironmentImages),
        lastLoadedAt: Date.now(),
        logoImage: mergedLogoImage,
        logoFiles: buildUploadRenderImages(mergedLogoImage ? [mergedLogoImage] : [], this.data.logoFiles),
        storefrontImages: mergedStorefrontImages,
        storefrontFiles: buildUploadRenderImages(mergedStorefrontImages, this.data.storefrontFiles),
        environmentImages: mergedEnvironmentImages,
        environmentFiles: buildUploadRenderImages(mergedEnvironmentImages, this.data.environmentFiles),
        _merchantVersion: merchant.version,
        loading: false
      })

      this.resumePendingImageRecovery(
        mergedLogoImage,
        mergedStorefrontImages,
        mergedEnvironmentImages,
        persistedStorefrontImages,
        persistedEnvironmentImages
      )
    } catch (err) {
      logger.error('[ProfileImages] 加载数据失败', err)
      const message = getErrorMessage(err, '加载失败，请重试')
      if (!this.data.hasLoaded) {
        this.setData({
          initialError: true,
          initialErrorMessage: message,
          loading: false
        })
        return
      }

      wx.showToast({ title: message, icon: 'none' })
      this.setData({ loading: false })
    }
  },

  onRetry() {
    this.loadData(true, true)
  },

  onRetryAccess() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      initialError: false,
      initialErrorMessage: ''
    })
    this.onLoad()
  },

  // ==================== Logo 上传 ====================

  async onLogoUpload(e: WechatMiniprogram.CustomEvent) {
    const files = e.detail.files as Array<{ url: string }>
    if (!files?.length) return

    const newFile = files[files.length - 1]
    if (!newFile?.url) return

    const previousLogoImage = this.data.logoImage
    const uploadingLogoImage: ImageItem = {
      url: newFile.url,
      localFileUrl: newFile.url
    }

    this.setData({
      logoUploading: true,
      logoImage: uploadingLogoImage,
      logoFiles: buildUploadRenderImages([uploadingLogoImage], this.data.logoFiles),
      hasAnyImages: true
    })
    wx.showLoading({ title: '上传中...' })
    try {
      const { mediaId, displayUrl } = await uploadMerchantImage(newFile.url, 'logo')

      const uploadedLogoImage: ImageItem = {
        url: displayUrl || newFile.url,
        rawUrl: displayUrl || undefined,
        assetId: mediaId,
        localFileUrl: newFile.url
      }

      this.setData({
        logoImage: uploadedLogoImage,
        logoFiles: buildUploadRenderImages([uploadedLogoImage], this.data.logoFiles),
        hasAnyImages: true
      })

      // 保存到后端（需要当前 version）
      const updated = await updateMyMerchantLogo(mediaId, this.data._merchantVersion)
      const persistedLogoUrl = getPublicImageUrl(updated.logo_url || '') || displayUrl || newFile.url

      this.setData({
        logoImage: {
          url: persistedLogoUrl,
          rawUrl: updated.logo_url || displayUrl || undefined,
          assetId: mediaId,
          localFileUrl: undefined
        },
        logoFiles: buildUploadRenderImages([{
          url: persistedLogoUrl,
          rawUrl: updated.logo_url || displayUrl || undefined,
          assetId: mediaId,
          localFileUrl: undefined
        }], this.data.logoFiles),
        hasAnyImages: true,
        _merchantVersion: updated.version,
        logoUploading: false,
        lastLoadedAt: Date.now()
      })

      if (!getPublicImageUrl(updated.logo_url || '') && !displayUrl) {
        void this.finalizePendingLogo(mediaId)
      }

      wx.hideLoading()
    } catch (err) {
      wx.hideLoading()
      this.setData({
        logoUploading: false,
        logoImage: previousLogoImage,
        logoFiles: buildUploadRenderImages(previousLogoImage ? [previousLogoImage] : [], this.data.logoFiles),
        hasAnyImages: hasAnyImages(previousLogoImage, this.data.storefrontImages, this.data.environmentImages)
      })
      logger.error('[ProfileImages] Logo 上传失败', err)
      wx.showToast({ title: getErrorMessage(err, '上传失败，请重试'), icon: 'none' })
    }
  },

  async onLogoRemove() {
    wx.showModal({
      title: '确认删除',
      content: '确定要删除 Logo 吗？',
      success: async (res) => {
        if (!res.confirm) return

        const previousLogoImage = this.data.logoImage
        const previousVersion = this.data._merchantVersion

        this.setData({
          logoUploading: true,
          logoImage: null,
          logoFiles: [],
          hasAnyImages: hasAnyImages(null, this.data.storefrontImages, this.data.environmentImages)
        })

        wx.showLoading({ title: '保存中...' })
        try {
          const updated = await updateMyMerchantLogo(null, previousVersion)
          this.setData({
            hasAnyImages: hasAnyImages(null, this.data.storefrontImages, this.data.environmentImages),
            _merchantVersion: updated.version,
            logoUploading: false,
            logoFiles: [],
            lastLoadedAt: Date.now()
          })
          wx.hideLoading()
        } catch (err) {
          wx.hideLoading()
          this.setData({
            logoUploading: false,
            logoImage: previousLogoImage,
            logoFiles: buildUploadRenderImages(previousLogoImage ? [previousLogoImage] : [], this.data.logoFiles),
            hasAnyImages: hasAnyImages(previousLogoImage, this.data.storefrontImages, this.data.environmentImages),
            _merchantVersion: previousVersion
          })
          logger.error('[ProfileImages] 删除 Logo 失败', err)
          wx.showToast({ title: getErrorMessage(err, '操作失败，请稍后重试'), icon: 'none' })
        }
      }
    })
  },

  // ==================== 门头照 ====================

  async processShopImagesUpload(kind: ShopImageKind, files: UploadFileItem[], maxCount: number) {
    const imagesFieldName = kind === 'storefront' ? 'storefrontImages' : 'environmentImages'
    const savingFieldName = kind === 'storefront' ? 'storefrontSaving' : 'environmentSaving'
    const title = kind === 'storefront' ? '门头照' : '环境照'
    const limitMessage = kind === 'storefront' ? '最多上传3张门头照' : '最多上传5张环境照'
    const previousImages = [...this.data[imagesFieldName]] as ImageItem[]
    const { controlledImages, newLocalFileUrls } = buildControlledUploadImages(previousImages, files)

    if (!newLocalFileUrls.length) {
      return
    }

    if (controlledImages.length > maxCount) {
      wx.showToast({ title: limitMessage, icon: 'none' })
      return
    }

    let currentImages = controlledImages
    let uploadError: unknown = null
    let hasAmbiguousPersistence = false

    this.bumpShopImagesGeneration()
    this.setData({
      [savingFieldName]: true,
      [imagesFieldName]: currentImages,
      [kind === 'storefront' ? 'storefrontFiles' : 'environmentFiles']: buildUploadRenderImages(
        currentImages,
        kind === 'storefront' ? this.data.storefrontFiles : this.data.environmentFiles
      ),
      hasAnyImages: hasAnyImages(
        this.data.logoImage,
        kind === 'storefront' ? currentImages : this.data.storefrontImages,
        kind === 'environment' ? currentImages : this.data.environmentImages
      )
    } as Record<string, unknown>)
    wx.showLoading({ title: '上传中...' })

    for (const localFileUrl of newLocalFileUrls) {
      const pendingIndex = findPendingUploadImageIndex(currentImages, localFileUrl)
      if (pendingIndex < 0) {
        continue
      }

      try {
        const result = await uploadMerchantImage(localFileUrl, kind)
        currentImages = replaceImageAt(currentImages, pendingIndex, {
          ...currentImages[pendingIndex],
          url: result.displayUrl || localFileUrl,
          rawUrl: result.displayUrl || undefined,
          assetId: result.mediaId,
          localFileUrl,
          pendingSync: true
        })

        this.bumpShopImagesGeneration()
        this.setData({
          [imagesFieldName]: currentImages,
          [kind === 'storefront' ? 'storefrontFiles' : 'environmentFiles']: buildUploadRenderImages(
            currentImages,
            kind === 'storefront' ? this.data.storefrontFiles : this.data.environmentFiles
          ),
          hasAnyImages: hasAnyImages(
            this.data.logoImage,
            kind === 'storefront' ? currentImages : this.data.storefrontImages,
            kind === 'environment' ? currentImages : this.data.environmentImages
          )
        } as Record<string, unknown>)

        if (!result.displayUrl) {
          void this.finalizePendingShopImage(kind, result.mediaId)
          continue
        }

        const nextStorefrontImages = kind === 'storefront'
          ? currentImages
          : [...this.data.storefrontImages]
        const nextEnvironmentImages = kind === 'environment'
          ? currentImages
          : [...this.data.environmentImages]

        try {
          await this.persistShopImagesUpdate(
            kind === 'storefront'
              ? { storefront_images: toPersistedImageUrls(currentImages) }
              : { environment_images: toPersistedImageUrls(currentImages) },
            nextStorefrontImages,
            nextEnvironmentImages
          )
          currentImages = [...this.data[imagesFieldName]] as ImageItem[]
        } catch (persistErr) {
          if (isAmbiguousSyncFailure(persistErr)) {
            hasAmbiguousPersistence = true
            logger.warn(`[ProfileImages] ${title}上传结果未确认，继续后台同步`, { err: persistErr })
            this.schedulePendingShopImagesPersistence()
            currentImages = [...this.data[imagesFieldName]] as ImageItem[]
            continue
          }

          currentImages = removeImageAt(currentImages, pendingIndex)
          this.bumpShopImagesGeneration()
          this.setData({
            [imagesFieldName]: currentImages,
            [kind === 'storefront' ? 'storefrontFiles' : 'environmentFiles']: buildUploadRenderImages(
              currentImages,
              kind === 'storefront' ? this.data.storefrontFiles : this.data.environmentFiles
            ),
            hasAnyImages: hasAnyImages(
              this.data.logoImage,
              kind === 'storefront' ? currentImages : this.data.storefrontImages,
              kind === 'environment' ? currentImages : this.data.environmentImages
            )
          } as Record<string, unknown>)
          uploadError = uploadError || persistErr
          logger.error(`[ProfileImages] ${title}上传后保存失败`, persistErr)
        }
      } catch (err) {
        currentImages = removeImageAt(currentImages, pendingIndex)
        this.bumpShopImagesGeneration()
        this.setData({
          [imagesFieldName]: currentImages,
          [kind === 'storefront' ? 'storefrontFiles' : 'environmentFiles']: buildUploadRenderImages(
            currentImages,
            kind === 'storefront' ? this.data.storefrontFiles : this.data.environmentFiles
          ),
          hasAnyImages: hasAnyImages(
            this.data.logoImage,
            kind === 'storefront' ? currentImages : this.data.storefrontImages,
            kind === 'environment' ? currentImages : this.data.environmentImages
          )
        } as Record<string, unknown>)
        uploadError = uploadError || err
        logger.error(`[ProfileImages] ${title}上传失败`, err)
      }
    }

    wx.hideLoading()
    this.setData({
      [savingFieldName]: false
    } as Record<string, unknown>)

    if (uploadError) {
      wx.showToast({ title: getErrorMessage(uploadError, '上传失败，请重试'), icon: 'none' })
      return
    }

    if (hasAmbiguousPersistence) {
      wx.showToast({ title: CONTINUE_SYNC_TOAST, icon: 'none' })
    }
  },

  async onStorefrontUpload(e: WechatMiniprogram.CustomEvent) {
    if (this.data.storefrontSaving) return

    const files = Array.isArray(e.detail?.files) ? e.detail.files as UploadFileItem[] : []
    if (!files?.length) return

    await this.processShopImagesUpload('storefront', files, 3)
  },

  async onStorefrontRemove(e: WechatMiniprogram.CustomEvent) {
    if (this.data.storefrontSaving) return

    const { index } = e.detail as { index: number }
    const previousImages = [...this.data.storefrontImages]
    const images = [...previousImages]
    images.splice(index, 1)
    this.setData({ storefrontSaving: true })
    wx.showLoading({ title: '保存中...' })
    try {
      await this.persistShopImagesUpdate({
        storefront_images: toPersistedImageUrls(images)
      }, images, this.data.environmentImages)
      this.setData({ storefrontSaving: false })
      wx.hideLoading()
    } catch (err) {
      logger.error('[ProfileImages] 删除门头照失败', err)
      this.setData({ storefrontSaving: false })
      wx.hideLoading()
      wx.showToast({ title: getErrorMessage(err, '删除门头照失败，请稍后重试'), icon: 'none' })
    }
  },

  // ==================== 环境照 ====================

  async onEnvironmentUpload(e: WechatMiniprogram.CustomEvent) {
    if (this.data.environmentSaving) return

    const files = Array.isArray(e.detail?.files) ? e.detail.files as UploadFileItem[] : []
    if (!files?.length) return

    await this.processShopImagesUpload('environment', files, 5)
  },

  async onEnvironmentRemove(e: WechatMiniprogram.CustomEvent) {
    if (this.data.environmentSaving) return

    const { index } = e.detail as { index: number }
    const previousImages = [...this.data.environmentImages]
    const images = [...previousImages]
    images.splice(index, 1)
    this.setData({ environmentSaving: true })
    wx.showLoading({ title: '保存中...' })
    try {
      await this.persistShopImagesUpdate({
        environment_images: toPersistedImageUrls(images)
      }, this.data.storefrontImages, images)
      this.setData({ environmentSaving: false })
      wx.hideLoading()
    } catch (err) {
      logger.error('[ProfileImages] 删除环境照失败', err)
      this.setData({ environmentSaving: false })
      wx.hideLoading()
      wx.showToast({ title: getErrorMessage(err, '删除环境照失败，请稍后重试'), icon: 'none' })
    }
  },

  async finalizePendingLogo(mediaId: number) {
    try {
      const remoteUrl = await waitForPublicMediaDisplayUrl(mediaId)
      if (!remoteUrl) {
        return
      }

      const currentLogo = this.data.logoImage
      if (!currentLogo || currentLogo.assetId !== mediaId) {
        return
      }

      this.setData({
        logoImage: {
          ...currentLogo,
          url: remoteUrl,
          rawUrl: remoteUrl,
          localFileUrl: undefined
        },
        logoFiles: buildUploadRenderImages([{
          ...currentLogo,
          url: remoteUrl,
          rawUrl: remoteUrl,
          localFileUrl: undefined
        }], this.data.logoFiles)
      })
    } catch (err) {
      logger.warn('[ProfileImages] 等待 Logo 公网地址失败', { mediaId, err })
    }
  },

  async finalizePendingShopImage(kind: 'storefront' | 'environment', mediaId: number) {
    try {
      const remoteUrl = await waitForPublicMediaDisplayUrl(mediaId)
      if (!remoteUrl) {
        return
      }

      const fieldName = kind === 'storefront' ? 'storefrontImages' : 'environmentImages'
      const filesFieldName = kind === 'storefront' ? 'storefrontFiles' : 'environmentFiles'
      const currentImages = [...this.data[fieldName]] as ImageItem[]
      const targetIndex = currentImages.findIndex((image) => image.assetId === mediaId)
      if (targetIndex < 0) {
        return
      }
      const target = currentImages[targetIndex]

      currentImages[targetIndex] = {
        ...target,
        url: remoteUrl,
        rawUrl: remoteUrl,
        assetId: mediaId,
        localFileUrl: undefined,
        pendingSync: true
      }

      this.bumpShopImagesGeneration()
      const nextPatch: Record<string, unknown> = {
        [fieldName]: currentImages,
        hasAnyImages: hasAnyImages(
          this.data.logoImage,
          kind === 'storefront' ? currentImages : this.data.storefrontImages,
          kind === 'environment' ? currentImages : this.data.environmentImages
        )
      }
      const currentFiles = [...this.data[filesFieldName]] as ImageItem[]
      const nextFiles = buildUploadRenderImages(currentImages, currentFiles)
      if (!areUploadRenderImagesEqual(currentFiles, nextFiles)) {
        nextPatch[filesFieldName] = nextFiles
      }
      this.setData(nextPatch)

      await this.persistShopImagesUpdate({
        storefront_images: kind === 'storefront' ? toPersistedImageUrls(currentImages) : toPersistedImageUrls(this.data.storefrontImages),
        environment_images: kind === 'environment' ? toPersistedImageUrls(currentImages) : toPersistedImageUrls(this.data.environmentImages)
      },
        kind === 'storefront' ? currentImages : this.data.storefrontImages,
        kind === 'environment' ? currentImages : this.data.environmentImages
      )
    } catch (err) {
      logger.warn('[ProfileImages] 等待图片审核通过后持久化失败', { kind, mediaId, err })
    }
  },

  applyShopImagesResponse(
    updated: ShopImagesResponse,
    localStorefrontImages?: ImageItem[],
    localEnvironmentImages?: ImageItem[]
  ) {
    const resolvedLocalStorefrontImages = localStorefrontImages ?? this.data.storefrontImages
    const resolvedLocalEnvironmentImages = localEnvironmentImages ?? this.data.environmentImages
    const reconciledShopImages = this.reconcilePendingDeletedShopImages(
      updated.storefront_images,
      updated.environment_images
    )
    const persistedStorefrontImages = toImageItems(reconciledShopImages.storefrontRawUrls)
    const persistedEnvironmentImages = toImageItems(reconciledShopImages.environmentRawUrls)
    const storefrontImages = mergeServerAndLocalImages(persistedStorefrontImages, resolvedLocalStorefrontImages)
    const environmentImages = mergeServerAndLocalImages(persistedEnvironmentImages, resolvedLocalEnvironmentImages)
    const hasPendingPersistence = this._deletedStorefrontImageRawUrls.size > 0
      || this._deletedEnvironmentImageRawUrls.size > 0
      || storefrontImages.some((image) => isLocallyTrackedShopImagePendingPersistence(image))
      || environmentImages.some((image) => isLocallyTrackedShopImagePendingPersistence(image))

    this.bumpShopImagesGeneration()

    const nextHasAnyImages = hasAnyImages(this.data.logoImage, storefrontImages, environmentImages)
    const nextPatch: Record<string, unknown> = {
      lastLoadedAt: Date.now(),
      hasAnyImages: nextHasAnyImages
    }

    if (!areImageListsEqual(this.data.storefrontImages, storefrontImages)) {
      nextPatch.storefrontImages = storefrontImages
    }

    const nextStorefrontFiles = buildUploadRenderImages(storefrontImages, this.data.storefrontFiles)
    if (!areUploadRenderImagesEqual(this.data.storefrontFiles, nextStorefrontFiles)) {
      nextPatch.storefrontFiles = nextStorefrontFiles
    }

    if (!areImageListsEqual(this.data.environmentImages, environmentImages)) {
      nextPatch.environmentImages = environmentImages
    }

    const nextEnvironmentFiles = buildUploadRenderImages(environmentImages, this.data.environmentFiles)
    if (!areUploadRenderImagesEqual(this.data.environmentFiles, nextEnvironmentFiles)) {
      nextPatch.environmentFiles = nextEnvironmentFiles
    }

    this.setData(nextPatch)

    if (hasPendingPersistence) {
      this._shopImagesPersistRetryPending = true
      return
    }

    this._shopImagesPersistRetryPending = false
    this.resetPendingShopImagesPersistenceRetryState()
  },

  resumePendingImageRecovery(
    logoImage: ImageItem | null,
    storefrontImages: ImageItem[],
    environmentImages: ImageItem[],
    persistedStorefrontImages: ImageItem[],
    persistedEnvironmentImages: ImageItem[]
  ) {
    if (isPendingImage(logoImage)) {
      void this.finalizePendingLogo(logoImage.assetId)
    }

    const persistedStorefrontRawUrls = buildImageRawUrlSet(persistedStorefrontImages)
    const persistedEnvironmentRawUrls = buildImageRawUrlSet(persistedEnvironmentImages)
    const persistedStorefrontComparableUrls = buildImageComparableUrlSet(persistedStorefrontImages)
    const persistedEnvironmentComparableUrls = buildImageComparableUrlSet(persistedEnvironmentImages)
    let shouldRetryShopPersistence = false

    storefrontImages.forEach((image) => {
      if (!image.assetId) {
        return
      }
      const rawUrl = normalizeImageRawUrl(image.rawUrl)
      if (!rawUrl) {
        void this.finalizePendingShopImage('storefront', image.assetId)
        return
      }
      const comparableUrls = [image.url, image.rawUrl]
        .map((url) => normalizeComparableImageUrl(url))
        .filter((url) => url.length > 0)
      const isPersisted = persistedStorefrontRawUrls.has(rawUrl)
        || comparableUrls.some((url) => persistedStorefrontComparableUrls.has(url))

      if (!isPersisted) {
        shouldRetryShopPersistence = true
      }
    })

    environmentImages.forEach((image) => {
      if (!image.assetId) {
        return
      }
      const rawUrl = normalizeImageRawUrl(image.rawUrl)
      if (!rawUrl) {
        void this.finalizePendingShopImage('environment', image.assetId)
        return
      }
      const comparableUrls = [image.url, image.rawUrl]
        .map((url) => normalizeComparableImageUrl(url))
        .filter((url) => url.length > 0)
      const isPersisted = persistedEnvironmentRawUrls.has(rawUrl)
        || comparableUrls.some((url) => persistedEnvironmentComparableUrls.has(url))

      if (!isPersisted) {
        shouldRetryShopPersistence = true
      }
    })

    if (!shouldRetryShopPersistence) {
      return
    }

    void this.retryPendingShopImagePersistence()
  },

  async retryPendingShopImagePersistence() {
    this.schedulePendingShopImagesPersistence()
  }
})
