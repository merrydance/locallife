import { logger } from './logger'
import { getPublicImageUrl } from './image'
export { getPublicImageUrl } from './image'
import { getErrorUserMessage } from './user-facing'

export type ImageItem = { url: string, rawUrl?: string, assetId?: number, localFileUrl?: string, pendingSync?: boolean, status?: 'loading' | 'done' | 'failed' | 'reload' }
export type UploadFileItem = { url?: string | null }
export type ShopImagesResponse = { storefront_images?: string[] | null, environment_images?: string[] | null }
export type ShopImageKind = 'storefront' | 'environment'
type CurrentMerchantCache = { id?: number, merchant_id?: number }
export type PendingDeletedShopImagesStorageState = {
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
  data?: { message?: unknown }
  body?: { message?: unknown }
  originalError?: { message?: unknown, errMsg?: unknown }
}

export const PROFILE_IMAGES_AUTO_REFRESH_WINDOW_MS = 60 * 1000
export const CONTINUE_SYNC_TOAST = '正在继续同步，请稍后刷新查看'
export const PENDING_DELETED_SHOP_IMAGES_STORAGE_PREFIX = 'merchant_profile_images_pending_deleted_v1'
export const PENDING_DELETED_SHOP_IMAGES_ANONYMOUS_SCOPE = 'anonymous'
export const SHOP_IMAGES_PERSIST_RETRY_BASE_DELAY_MS = 1500
export const SHOP_IMAGES_PERSIST_RETRY_MAX_DELAY_MS = 30000
export const getErrorMessage = getErrorUserMessage

export function normalizeImageRawUrl(rawUrl?: string | null): string {
  return typeof rawUrl === 'string' ? rawUrl.trim() : ''
}

export function normalizeComparableImageUrl(url?: string | null): string {
  if (typeof url !== 'string') {
    return ''
  }

  const trimmedUrl = url.trim()
  if (!trimmedUrl) {
    return ''
  }

  return getPublicImageUrl(trimmedUrl) || trimmedUrl
}

export function buildImageRawUrlSet(images: ImageItem[]): Set<string> {
  return new Set(
    images
      .map((image) => normalizeImageRawUrl(image.rawUrl))
      .filter((rawUrl) => rawUrl.length > 0)
  )
}

export function buildImageComparableUrlSet(images: ImageItem[]): Set<string> {
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

export function toNormalizedRawUrls(rawUrls?: Array<string | null | undefined> | null): string[] {
  return Array.from(new Set(
    (Array.isArray(rawUrls) ? rawUrls : [])
      .map((rawUrl) => normalizeImageRawUrl(rawUrl))
      .filter((rawUrl) => rawUrl.length > 0)
  ))
}

export function toMerchantPendingDeletedScope(merchantId: number): string {
  return merchantId > 0 ? `merchant_${merchantId}` : PENDING_DELETED_SHOP_IMAGES_ANONYMOUS_SCOPE
}

export function getPendingDeletedShopImagesStorageKey(scope: string): string {
  return `${PENDING_DELETED_SHOP_IMAGES_STORAGE_PREFIX}:${scope}`
}

export function getCurrentMerchantIdFromStorage(): number {
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
  if (!image?.assetId) {
    return false
  }
  const rawUrl = normalizeImageRawUrl(image.rawUrl)
  if (rawUrl && persistedRawUrls.has(rawUrl)) {
    return false
  }
  const comparableUrls = [image.url, image.rawUrl]
    .map((url) => normalizeComparableImageUrl(url))
    .filter((url) => url.length > 0)
  return comparableUrls.every((url) => !persistedComparableUrls.has(url))
}

export function mergeServerAndLocalImages(persistedImages: ImageItem[], localImages: ImageItem[]): ImageItem[] {
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

export function mergeServerAndLocalLogo(persistedLogo: ImageItem | null, localLogo: ImageItem | null): ImageItem | null {
  if (persistedLogo) {
    return persistedLogo
  }
  if (!localLogo?.assetId) {
    return null
  }
  return { ...localLogo }
}

export function isPendingImage(image: ImageItem | null | undefined): image is ImageItem & { assetId: number } {
  return !!image?.assetId && !normalizeImageRawUrl(image.rawUrl)
}

export function isLocallyTrackedShopImagePendingPersistence(image: ImageItem | null | undefined): boolean {
  if (!image?.assetId) {
    return false
  }
  return !!image.pendingSync || !!image.localFileUrl || !normalizeImageRawUrl(image.rawUrl)
}

export function toPersistedImageUrls(images: ImageItem[]): string[] {
  return images
    .map((image) => image.rawUrl)
    .filter((url): url is string => typeof url === 'string' && url.trim().length > 0)
}

export function hasAnyImages(logoImage: ImageItem | null, storefrontImages: ImageItem[], environmentImages: ImageItem[]) {
  return !!logoImage || storefrontImages.length > 0 || environmentImages.length > 0
}

export function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
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

export function isAmbiguousSyncFailure(error: unknown): boolean {
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
  ].map(normalizeSyncErrorText).filter((text) => text.length > 0).join(' ')
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

export function isExplicitSyncFailure(error: unknown): boolean {
  const statusCode = getSyncErrorStatusCode(error)
  return typeof statusCode === 'number' && statusCode >= 400 && statusCode < 500 && statusCode !== 408 && statusCode !== 429
}

export function toImageItems(rawUrls?: string[] | null): ImageItem[] {
  if (!Array.isArray(rawUrls)) {
    return []
  }
  return rawUrls
    .map((rawUrl) => ({
      url: getPublicImageUrl(rawUrl),
      rawUrl
    }))
    .filter((item) => !!item.url)
}

export function mergeImagesWithRawUrls(images: ImageItem[], rawUrls?: Array<string | null | undefined> | null): ImageItem[] {
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

export function buildControlledUploadImages(existingImages: ImageItem[], files: UploadFileItem[]) {
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

export function findPendingUploadImageIndex(images: ImageItem[], localFileUrl: string): number {
  return images.findIndex((image) => !image.assetId && image.localFileUrl === localFileUrl)
}

export function replaceImageAt(images: ImageItem[], index: number, image: ImageItem): ImageItem[] {
  if (index < 0 || index >= images.length) {
    return images
  }
  const nextImages = [...images]
  nextImages[index] = image
  return nextImages
}

export function removeImageAt(images: ImageItem[], index: number): ImageItem[] {
  if (index < 0 || index >= images.length) {
    return images
  }
  const nextImages = [...images]
  nextImages.splice(index, 1)
  return nextImages
}

export function clearPendingSyncFromImages(images: ImageItem[]): ImageItem[] {
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
  const leftComparableUrls = [left.url, left.rawUrl].map((url) => normalizeComparableImageUrl(url)).filter((url) => url.length > 0)
  const rightComparableUrls = [right.url, right.rawUrl].map((url) => normalizeComparableImageUrl(url)).filter((url) => url.length > 0)
  return leftComparableUrls.some((url) => rightComparableUrls.includes(url))
}

export function buildUploadRenderImages(images: ImageItem[], previousFiles: ImageItem[] = []): ImageItem[] {
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

export function areImageListsEqual(left: ImageItem[], right: ImageItem[]): boolean {
  if (left === right) {
    return true
  }
  if (left.length !== right.length) {
    return false
  }
  return left.every((image, index) => areImageItemsEqual(image, right[index]))
}

export function areUploadRenderImagesEqual(left: ImageItem[], right: ImageItem[]): boolean {
  if (left === right) {
    return true
  }
  if (left.length !== right.length) {
    return false
  }
  return left.every((image, index) => image.url === right[index]?.url)
}