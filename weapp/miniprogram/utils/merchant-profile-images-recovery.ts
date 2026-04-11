import { waitForPublicMediaDisplayUrl } from '../api/onboarding'
import { logger } from './logger'
import {
  areImageListsEqual,
  areUploadRenderImagesEqual,
  buildImageComparableUrlSet,
  buildImageRawUrlSet,
  buildUploadRenderImages,
  hasAnyImages,
  ImageItem,
  isLocallyTrackedShopImagePendingPersistence,
  isPendingImage,
  mergeServerAndLocalImages,
  normalizeComparableImageUrl,
  normalizeImageRawUrl,
  ShopImagesResponse,
  toImageItems,
  toPersistedImageUrls
} from './merchant-profile-images-view'

type MerchantProfileImagesPageContext = WechatMiniprogram.Page.Instance<Record<string, any>, Record<string, any>> & Record<string, any>

function defineMerchantProfileImagesRecoveryMethods<T extends Record<string, unknown>>(methods: T & ThisType<MerchantProfileImagesPageContext>) {
  return methods
}

export const merchantProfileImagesRecoveryMethods = defineMerchantProfileImagesRecoveryMethods({
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
        logoFiles: buildUploadRenderImages([{ ...currentLogo, url: remoteUrl, rawUrl: remoteUrl, localFileUrl: undefined }], this.data.logoFiles)
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
      await this.persistShopImagesUpdate(
        {
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

  applyShopImagesResponse(updated: ShopImagesResponse, localStorefrontImages?: ImageItem[], localEnvironmentImages?: ImageItem[]) {
    const resolvedLocalStorefrontImages = localStorefrontImages ?? this.data.storefrontImages
    const resolvedLocalEnvironmentImages = localEnvironmentImages ?? this.data.environmentImages
    const reconciledShopImages = this.reconcilePendingDeletedShopImages(updated.storefront_images, updated.environment_images)
    const persistedStorefrontImages = toImageItems(reconciledShopImages.storefrontRawUrls)
    const persistedEnvironmentImages = toImageItems(reconciledShopImages.environmentRawUrls)
    const storefrontImages = mergeServerAndLocalImages(persistedStorefrontImages, resolvedLocalStorefrontImages)
    const environmentImages = mergeServerAndLocalImages(persistedEnvironmentImages, resolvedLocalEnvironmentImages)
    const hasPendingPersistence = this._deletedStorefrontImageRawUrls.size > 0
      || this._deletedEnvironmentImageRawUrls.size > 0
      || storefrontImages.some((image) => isLocallyTrackedShopImagePendingPersistence(image))
      || environmentImages.some((image) => isLocallyTrackedShopImagePendingPersistence(image))

    this.bumpShopImagesGeneration()
    const nextPatch: Record<string, unknown> = {
      lastLoadedAt: Date.now(),
      hasAnyImages: hasAnyImages(this.data.logoImage, storefrontImages, environmentImages)
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
      const comparableUrls = [image.url, image.rawUrl].map((url) => normalizeComparableImageUrl(url)).filter((url) => url.length > 0)
      const isPersisted = persistedStorefrontRawUrls.has(rawUrl) || comparableUrls.some((url) => persistedStorefrontComparableUrls.has(url))
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
      const comparableUrls = [image.url, image.rawUrl].map((url) => normalizeComparableImageUrl(url)).filter((url) => url.length > 0)
      const isPersisted = persistedEnvironmentRawUrls.has(rawUrl) || comparableUrls.some((url) => persistedEnvironmentComparableUrls.has(url))
      if (!isPersisted) {
        shouldRetryShopPersistence = true
      }
    })

    if (shouldRetryShopPersistence) {
      void this.retryPendingShopImagePersistence()
    }
  },

  async retryPendingShopImagePersistence() {
    this.schedulePendingShopImagesPersistence()
  }
})