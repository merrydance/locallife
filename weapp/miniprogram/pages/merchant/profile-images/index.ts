import { getStableBarHeights } from '../../../utils/responsive'
import { logger } from '../../../utils/logger'
import { uploadMerchantImage, getMerchantApplication, updateShopImages } from '../../../api/onboarding'
import { getMyMerchantProfile, updateMyMerchantLogo } from '../../../api/merchant'
import { getPublicImageUrl } from '../../../utils/image'

type ImageItem = { url: string, rawUrl?: string }

function hasAnyImages(logoImage: ImageItem | null, storefrontImages: ImageItem[], environmentImages: ImageItem[]) {
  return !!logoImage || storefrontImages.length > 0 || environmentImages.length > 0
}

function getErrorMessage(error: unknown, fallback: string): string {
  if (typeof error === 'object' && error !== null && 'userMessage' in error) {
    const userMessage = (error as { userMessage?: unknown }).userMessage
    if (typeof userMessage === 'string' && userMessage.trim()) {
      return userMessage
    }
  }

  if (typeof error === 'object' && error !== null && 'message' in error) {
    const message = (error as { message?: unknown }).message
    if (typeof message === 'string' && message.trim()) {
      return message
    }
  }

  return fallback
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

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    initialError: false,
    initialErrorMessage: '',
    hasLoaded: false,
    hasAnyImages: false,

    // Logo（单张）
    logoImage: null as ImageItem | null,
    logoUploading: false,

    // 门头照（最多3张）
    storefrontImages: [] as ImageItem[],
    storefrontSaving: false,

    // 环境照（最多5张）
    environmentImages: [] as ImageItem[],
    environmentSaving: false,

    // 商户版本号（乐观锁，logo更新时使用）
    _merchantVersion: 0
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.loadData()
  },

  onShow() {
    if (!this.data.loading) {
      this.loadData()
    }
  },

  // ==================== 数据加载 ====================

  async loadData() {
    if (this.data.loading) {
      return
    }

    this.setData({ loading: true })
    try {
      const [merchant, application] = await Promise.all([
        getMyMerchantProfile(),
        getMerchantApplication().catch(() => null)
      ])

      // Logo
      let logoImage: ImageItem | null = null
      if (merchant.logo_url) {
        const displayUrl = getPublicImageUrl(merchant.logo_url)
        if (displayUrl) {
          logoImage = { url: displayUrl, rawUrl: merchant.logo_url }
        }
      }

      // 门头照
      const storefrontImages = toImageItems(application?.storefront_images)

      // 环境照
      const environmentImages = toImageItems(application?.environment_images)

      this.setData({
        initialError: false,
        initialErrorMessage: '',
        hasLoaded: true,
        hasAnyImages: hasAnyImages(logoImage, storefrontImages, environmentImages),
        logoImage,
        storefrontImages,
        environmentImages,
        _merchantVersion: merchant.version,
        loading: false
      })
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
    this.loadData()
  },

  // ==================== Logo 上传 ====================

  async onLogoUpload(e: WechatMiniprogram.CustomEvent) {
    const files = e.detail.files as Array<{ url: string }>
    if (!files?.length) return

    const newFile = files[files.length - 1]
    if (!newFile?.url) return

    this.setData({ logoUploading: true })
    wx.showLoading({ title: '上传中...' })
    try {
      const { mediaId, displayUrl } = await uploadMerchantImage(newFile.url, 'logo')

      // 保存到后端（需要当前 version）
      const updated = await updateMyMerchantLogo(mediaId, this.data._merchantVersion)

      this.setData({
        logoImage: { url: displayUrl, rawUrl: displayUrl },
        hasAnyImages: true,
        _merchantVersion: updated.version,
        logoUploading: false
      })
      wx.hideLoading()
      wx.showToast({ title: 'Logo 已更新', icon: 'success' })
    } catch (err) {
      wx.hideLoading()
      this.setData({ logoUploading: false })
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
        wx.showLoading({ title: '保存中...' })
        try {
          const updated = await updateMyMerchantLogo(null, this.data._merchantVersion)
          this.setData({
            logoImage: null,
            hasAnyImages: hasAnyImages(null, this.data.storefrontImages, this.data.environmentImages),
            _merchantVersion: updated.version
          })
          wx.hideLoading()
          wx.showToast({ title: '已删除', icon: 'success' })
        } catch (err) {
          wx.hideLoading()
          logger.error('[ProfileImages] 删除 Logo 失败', err)
          wx.showToast({ title: getErrorMessage(err, '操作失败，请稍后重试'), icon: 'none' })
        }
      }
    })
  },

  // ==================== 门头照 ====================

  async onStorefrontUpload(e: WechatMiniprogram.CustomEvent) {
    if (this.data.storefrontSaving) return

    const files = e.detail.files as Array<{ url: string }>
    if (!files?.length) return

    const newFile = files[files.length - 1]
    if (!newFile?.url) return

    const currentImages = [...this.data.storefrontImages]
    if (currentImages.length >= 3) {
      wx.showToast({ title: '最多上传3张门头照', icon: 'none' })
      return
    }

    this.setData({ storefrontSaving: true })
    wx.showLoading({ title: '上传中...' })
    try {
      const { displayUrl } = await uploadMerchantImage(newFile.url, 'storefront')
      currentImages.push({ url: displayUrl, rawUrl: displayUrl })

      const updated = await updateShopImages({
        storefront_images: currentImages.map((img) => img.rawUrl || img.url)
      })

      const storefrontImages = toImageItems(updated.storefront_images)
      this.setData({
        storefrontImages,
        hasAnyImages: hasAnyImages(this.data.logoImage, storefrontImages, this.data.environmentImages),
        storefrontSaving: false
      })
      wx.hideLoading()
      wx.showToast({ title: '上传成功', icon: 'success' })
    } catch (err) {
      wx.hideLoading()
      this.setData({ storefrontSaving: false })
      logger.error('[ProfileImages] 门头照上传失败', err)
      wx.showToast({ title: getErrorMessage(err, '上传失败，请重试'), icon: 'none' })
    }
  },

  async onStorefrontRemove(e: WechatMiniprogram.CustomEvent) {
    if (this.data.storefrontSaving) return

    const { index } = e.detail as { index: number }
    const images = [...this.data.storefrontImages]
    images.splice(index, 1)
    this.setData({ storefrontSaving: true })
    wx.showLoading({ title: '保存中...' })
    try {
      const updated = await updateShopImages({
        storefront_images: images.map((img) => img.rawUrl || img.url)
      })
      const storefrontImages = toImageItems(updated.storefront_images)
      this.setData({
        storefrontImages,
        hasAnyImages: hasAnyImages(this.data.logoImage, storefrontImages, this.data.environmentImages),
        storefrontSaving: false
      })
      wx.hideLoading()
      wx.showToast({ title: '门头照已删除', icon: 'success' })
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

    const files = e.detail.files as Array<{ url: string }>
    if (!files?.length) return

    const newFile = files[files.length - 1]
    if (!newFile?.url) return

    const currentImages = [...this.data.environmentImages]
    if (currentImages.length >= 5) {
      wx.showToast({ title: '最多上传5张环境照', icon: 'none' })
      return
    }

    this.setData({ environmentSaving: true })
    wx.showLoading({ title: '上传中...' })
    try {
      const { displayUrl } = await uploadMerchantImage(newFile.url, 'environment')
      currentImages.push({ url: displayUrl, rawUrl: displayUrl })

      const updated = await updateShopImages({
        environment_images: currentImages.map((img) => img.rawUrl || img.url)
      })

      const environmentImages = toImageItems(updated.environment_images)
      this.setData({
        environmentImages,
        hasAnyImages: hasAnyImages(this.data.logoImage, this.data.storefrontImages, environmentImages),
        environmentSaving: false
      })
      wx.hideLoading()
      wx.showToast({ title: '上传成功', icon: 'success' })
    } catch (err) {
      wx.hideLoading()
      this.setData({ environmentSaving: false })
      logger.error('[ProfileImages] 环境照上传失败', err)
      wx.showToast({ title: getErrorMessage(err, '上传失败，请重试'), icon: 'none' })
    }
  },

  async onEnvironmentRemove(e: WechatMiniprogram.CustomEvent) {
    if (this.data.environmentSaving) return

    const { index } = e.detail as { index: number }
    const images = [...this.data.environmentImages]
    images.splice(index, 1)
    this.setData({ environmentSaving: true })
    wx.showLoading({ title: '保存中...' })
    try {
      const updated = await updateShopImages({
        environment_images: images.map((img) => img.rawUrl || img.url)
      })
      const environmentImages = toImageItems(updated.environment_images)
      this.setData({
        environmentImages,
        hasAnyImages: hasAnyImages(this.data.logoImage, this.data.storefrontImages, environmentImages),
        environmentSaving: false
      })
      wx.hideLoading()
      wx.showToast({ title: '环境照已删除', icon: 'success' })
    } catch (err) {
      logger.error('[ProfileImages] 删除环境照失败', err)
      this.setData({ environmentSaving: false })
      wx.hideLoading()
      wx.showToast({ title: getErrorMessage(err, '删除环境照失败，请稍后重试'), icon: 'none' })
    }
  }
})
