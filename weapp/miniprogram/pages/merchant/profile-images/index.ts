import { getStableBarHeights } from '../../../utils/responsive'
import { logger } from '../../../utils/logger'
import { resolveImageURL } from '../../../utils/image-security'
import { uploadMerchantImage, getMerchantApplication, updateShopImages } from '../../../api/onboarding'
import { getMyMerchantProfile, updateMyMerchantLogo } from '../../../api/merchant'

type ImageItem = { url: string, rawUrl?: string }

Page({
  data: {
    navBarHeight: 88,
    loading: true,

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
    this.setData({ loading: true })
    try {
      const [merchant, application] = await Promise.all([
        getMyMerchantProfile(),
        getMerchantApplication().catch(() => null)
      ])

      // Logo
      let logoImage: ImageItem | null = null
      if (merchant.logo_url) {
        const displayUrl = await resolveImageURL(merchant.logo_url)
        logoImage = { url: displayUrl, rawUrl: merchant.logo_url }
      }

      // 门头照
      const storefrontRaw: string[] = application?.storefront_images || []
      const storefrontImages: ImageItem[] = await Promise.all(
        storefrontRaw.map(async (rawUrl) => ({
          url: await resolveImageURL(rawUrl),
          rawUrl
        }))
      )

      // 环境照
      const environmentRaw: string[] = application?.environment_images || []
      const environmentImages: ImageItem[] = await Promise.all(
        environmentRaw.map(async (rawUrl) => ({
          url: await resolveImageURL(rawUrl),
          rawUrl
        }))
      )

      this.setData({
        logoImage,
        storefrontImages,
        environmentImages,
        _merchantVersion: merchant.version,
        loading: false
      })
    } catch (err) {
      logger.error('[ProfileImages] 加载数据失败', err)
      wx.showToast({ title: '加载失败，请重试', icon: 'none' })
      this.setData({ loading: false })
    }
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
      const result = await uploadMerchantImage(newFile.url, 'logo')
      if (!result.image_url) throw new Error('上传响应格式错误')

      const rawUrl = result.image_url
      const displayUrl = await resolveImageURL(rawUrl)

      // 保存到后端（需要当前 version）
      const updated = await updateMyMerchantLogo(rawUrl, this.data._merchantVersion)

      this.setData({
        logoImage: { url: displayUrl, rawUrl },
        _merchantVersion: updated.version,
        logoUploading: false
      })
      wx.hideLoading()
      wx.showToast({ title: 'Logo 已更新', icon: 'success' })
    } catch (err) {
      wx.hideLoading()
      this.setData({ logoUploading: false })
      logger.error('[ProfileImages] Logo 上传失败', err)
      wx.showToast({ title: '上传失败，请重试', icon: 'none' })
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
          const updated = await updateMyMerchantLogo('', this.data._merchantVersion)
          this.setData({ logoImage: null, _merchantVersion: updated.version })
          wx.hideLoading()
          wx.showToast({ title: '已删除', icon: 'success' })
        } catch (err) {
          wx.hideLoading()
          logger.error('[ProfileImages] 删除 Logo 失败', err)
          wx.showToast({ title: '操作失败', icon: 'none' })
        }
      }
    })
  },

  // ==================== 门头照 ====================

  async onStorefrontUpload(e: WechatMiniprogram.CustomEvent) {
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
      const result = await uploadMerchantImage(newFile.url, 'storefront')
      if (!result.image_url) throw new Error('上传响应格式错误')

      const rawUrl = result.image_url
      const displayUrl = await resolveImageURL(rawUrl)
      currentImages.push({ url: displayUrl, rawUrl })

      await updateShopImages({
        storefront_images: currentImages.map((img) => img.rawUrl || img.url)
      })

      this.setData({ storefrontImages: currentImages, storefrontSaving: false })
      wx.hideLoading()
      wx.showToast({ title: '上传成功', icon: 'success' })
    } catch (err) {
      wx.hideLoading()
      this.setData({ storefrontSaving: false })
      logger.error('[ProfileImages] 门头照上传失败', err)
      wx.showToast({ title: '上传失败，请重试', icon: 'none' })
    }
  },

  async onStorefrontRemove(e: WechatMiniprogram.CustomEvent) {
    const { index } = e.detail as { index: number }
    const images = [...this.data.storefrontImages]
    images.splice(index, 1)
    this.setData({ storefrontImages: images, storefrontSaving: true })
    try {
      await updateShopImages({
        storefront_images: images.map((img) => img.rawUrl || img.url)
      })
      this.setData({ storefrontSaving: false })
    } catch (err) {
      logger.error('[ProfileImages] 删除门头照失败', err)
      this.setData({ storefrontSaving: false })
      wx.showToast({ title: '操作失败', icon: 'none' })
    }
  },

  // ==================== 环境照 ====================

  async onEnvironmentUpload(e: WechatMiniprogram.CustomEvent) {
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
      const result = await uploadMerchantImage(newFile.url, 'environment')
      if (!result.image_url) throw new Error('上传响应格式错误')

      const rawUrl = result.image_url
      const displayUrl = await resolveImageURL(rawUrl)
      currentImages.push({ url: displayUrl, rawUrl })

      await updateShopImages({
        environment_images: currentImages.map((img) => img.rawUrl || img.url)
      })

      this.setData({ environmentImages: currentImages, environmentSaving: false })
      wx.hideLoading()
      wx.showToast({ title: '上传成功', icon: 'success' })
    } catch (err) {
      wx.hideLoading()
      this.setData({ environmentSaving: false })
      logger.error('[ProfileImages] 环境照上传失败', err)
      wx.showToast({ title: '上传失败，请重试', icon: 'none' })
    }
  },

  async onEnvironmentRemove(e: WechatMiniprogram.CustomEvent) {
    const { index } = e.detail as { index: number }
    const images = [...this.data.environmentImages]
    images.splice(index, 1)
    this.setData({ environmentImages: images, environmentSaving: true })
    try {
      await updateShopImages({
        environment_images: images.map((img) => img.rawUrl || img.url)
      })
      this.setData({ environmentSaving: false })
    } catch (err) {
      logger.error('[ProfileImages] 删除环境照失败', err)
      this.setData({ environmentSaving: false })
      wx.showToast({ title: '操作失败', icon: 'none' })
    }
  }
})
