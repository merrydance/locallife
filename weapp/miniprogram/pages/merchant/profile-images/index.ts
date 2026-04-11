import { logger } from '../../../utils/logger'
import { uploadMerchantImage, getMerchantApplication, waitForPublicMediaDisplayUrl } from '../../../api/onboarding'
import { getMyMerchantProfile, updateMyMerchantLogo } from '../../../api/merchant'
import {
  areImageListsEqual,
  areUploadRenderImagesEqual,
  buildControlledUploadImages,
  buildUploadRenderImages,
  CONTINUE_SYNC_TOAST,
  getErrorMessage,
  getPublicImageUrl,
  hasAnyImages,
  ImageItem,
  isAmbiguousSyncFailure,
  isLocallyTrackedShopImagePendingPersistence,
  mergeServerAndLocalImages,
  mergeServerAndLocalLogo,
  PROFILE_IMAGES_AUTO_REFRESH_WINDOW_MS,
  removeImageAt,
  replaceImageAt,
  ShopImageKind,
  shouldAutoRefresh,
  toImageItems,
  toPersistedImageUrls,
  UploadFileItem,
  findPendingUploadImageIndex
} from '../../../utils/merchant-profile-images-view'
import { merchantProfileImagesLifecycleMethods } from '../../../utils/merchant-profile-images-lifecycle'
import { merchantProfileImagesRecoveryMethods } from '../../../utils/merchant-profile-images-recovery'

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
  ...merchantProfileImagesLifecycleMethods,
  ...merchantProfileImagesRecoveryMethods,

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
  }

})
