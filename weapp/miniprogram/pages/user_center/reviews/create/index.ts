import { Review, ReviewService, CreateReviewParams, UpdateReviewParams } from '../../../../api/review'
import { getOrderDetail } from '../../../../api/order'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface ReviewUploadFile {
  url: string
  status?: 'loading' | 'done' | 'failed'
  mediaId?: number
}

const REVIEW_UPLOAD_PENDING = 'loading'
const REVIEW_UPLOAD_DONE = 'done'
const REVIEW_UPLOAD_FAILED = 'failed'

interface ReviewListRefreshPage {
  onReviewUpdated?: () => void
}

function normalizeReviewImages(review: Review): string[] {
  const imageUrls = Array.isArray(review.image_urls)
    ? review.image_urls
    : Array.isArray(review.imageUrls)
      ? review.imageUrls
      : Array.isArray(review.images)
        ? review.images
        : []

  return imageUrls.filter((url): url is string => typeof url === 'string' && url.length > 0)
}

function normalizeReviewImageAssetIds(review: Review): number[] {
  const imageAssetIds = Array.isArray(review.image_asset_ids)
    ? review.image_asset_ids
    : Array.isArray(review.imageAssetIds)
      ? review.imageAssetIds
      : []

  return imageAssetIds.filter((id): id is number => typeof id === 'number' && Number.isFinite(id) && id > 0)
}

function buildReviewUploadFiles(review: Review): ReviewUploadFile[] {
  const urls = normalizeReviewImages(review)
  const assetIds = normalizeReviewImageAssetIds(review)

  return urls.map((url, index) => ({
    url,
    status: REVIEW_UPLOAD_DONE,
    mediaId: assetIds[index]
  }))
}

Page({
  data: {
    mode: 'create' as 'create' | 'edit',
    reviewId: 0,
    pageTitle: '发表评价',
    submitText: '发布评价',
    orderId: 0,
    merchantId: 0,
    merchantName: '',
    orderNo: '',
    navBarHeight: 88,
    loading: false,
    initialLoading: true,
    submitting: false,

    content: '',
    fileList: [] as ReviewUploadFile[],
    maxImages: 9,
    maxContentLength: 500
  },

  onLoad(options: { orderId?: string, reviewId?: string }) {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })

    const reviewId = Number(options.reviewId || 0)
    const orderId = Number(options.orderId || 0)

    if (reviewId > 0) {
      this.setData({
        mode: 'edit',
        reviewId,
        pageTitle: '编辑评价',
        submitText: '保存评价'
      })
      this.loadReviewInfo()
    } else if (orderId > 0) {
      this.setData({ orderId })
      this.loadOrderInfo()
    } else {
      wx.showToast({ title: '评价信息缺失', icon: 'none' })
      setTimeout(() => wx.navigateBack(), 2000)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    if (e.detail.navBarHeight) {
      this.setData({ navBarHeight: e.detail.navBarHeight })
    }
  },

  async loadOrderInfo() {
    this.setData({ loading: true })
    try {
      const order = await getOrderDetail(this.data.orderId)
      this.setData({
        merchantId: order.merchant_id,
        merchantName: order.merchant_name,
        orderNo: order.order_no,
        initialLoading: false,
        loading: false
      })
    } catch (error) {
      logger.error('加载订单信息失败', error, 'reviews/create')
      this.setData({ initialLoading: false, loading: false })
      wx.showToast({ title: '订单详情加载失败', icon: 'none' })
    }
  },

  async loadReviewInfo() {
    this.setData({ loading: true })
    try {
      const review = await ReviewService.getReview(this.data.reviewId)
      this.setData({
        orderId: review.order_id,
        merchantId: review.merchant_id,
        merchantName: review.merchant_name || '',
        content: review.content || '',
        fileList: buildReviewUploadFiles(review)
      })

      try {
        const order = await getOrderDetail(review.order_id)
        this.setData({
          merchantId: order.merchant_id,
          merchantName: order.merchant_name || review.merchant_name || '餐饮商家',
          orderNo: order.order_no,
          initialLoading: false,
          loading: false
        })
      } catch (orderError) {
        logger.warn('编辑评价加载订单信息失败，保留评价详情', orderError, 'reviews/create')
        this.setData({
          merchantName: review.merchant_name || '餐饮商家',
          orderNo: String(review.order_id),
          initialLoading: false,
          loading: false
        })
      }
    } catch (error) {
      logger.error('加载评价详情失败', error, 'reviews/create')
      this.setData({ initialLoading: false, loading: false })
      wx.showToast({ title: getErrorUserMessage(error, '评价详情加载失败'), icon: 'none' })
    }
  },

  onContentChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ content: e.detail.value })
  },

  // 图片添加回调
  async onAddImage(e: WechatMiniprogram.CustomEvent<{ files: Array<{ url: string }> }>) {
    const { files } = e.detail
    const { fileList } = this.data

    // 先展示在界面上 (status: loading)
    const newFiles: ReviewUploadFile[] = files.map((f) => ({
      ...f,
      status: REVIEW_UPLOAD_PENDING
    }))

    this.setData({
      fileList: [...fileList, ...newFiles]
    })

    // 逐个开始上传
    for (let i = 0; i < newFiles.length; i++) {
      const file = newFiles[i]
      const currentIndex = fileList.length + i

      try {
        const { mediaId, displayUrl } = await ReviewService.uploadReviewImage(file.url)
        this.updateFileStatus(currentIndex, REVIEW_UPLOAD_DONE, mediaId, displayUrl)
      } catch (err) {
        this.updateFileStatus(currentIndex, REVIEW_UPLOAD_FAILED)
      }
    }
  },

  updateFileStatus(index: number, status: 'loading' | 'done' | 'failed', mediaId?: number, url?: string) {
    const { fileList } = this.data
    if (!fileList[index]) return

    fileList[index].status = status
    if (mediaId) {
      fileList[index].mediaId = mediaId
    }
    if (url) {
      fileList[index].url = url
    }

    this.setData({ fileList })
  },

  onRemoveImage(e: WechatMiniprogram.CustomEvent<{ index: number }>) {
    const { index } = e.detail
    const { fileList } = this.data
    fileList.splice(index, 1)
    this.setData({ fileList })
  },

  async onSubmit() {
    const { orderId, reviewId, mode, content, fileList, submitting } = this.data

    if (submitting) return

    if (!content || content.trim().length < 10) {
      wx.showToast({ title: '评价内容至少10个字', icon: 'none' })
      return
    }

    // 检查上传状态
    const uploading = fileList.some((f) => f.status === REVIEW_UPLOAD_PENDING)
    if (uploading) {
      wx.showToast({ title: '正在上传图片中，请稍候', icon: 'none' })
      return
    }

    const hasFailedImage = fileList.some((f) => f.status === REVIEW_UPLOAD_FAILED)
    if (hasFailedImage) {
      wx.showToast({ title: '请移除上传失败的图片', icon: 'none' })
      return
    }

    this.setData({ submitting: true })

    try {
      // 提取成功上传的媒体资产 ID
      const mediaAssetIds = fileList
        .filter((f) => f.status === REVIEW_UPLOAD_DONE && f.mediaId)
        .map((f) => f.mediaId as number)

      if (mode === 'edit') {
        const reviewData: UpdateReviewParams = {
          content: content.trim(),
          media_asset_ids: mediaAssetIds.length > 0 ? mediaAssetIds : undefined
        }
        await ReviewService.updateReview(reviewId, reviewData)
      } else {
        const reviewData: CreateReviewParams = {
          order_id: orderId,
          content: content.trim(),
          media_asset_ids: mediaAssetIds.length > 0 ? mediaAssetIds : undefined
        }

        await ReviewService.createReview(reviewData)
      }

      wx.showToast({ title: mode === 'edit' ? '评价已更新' : '发布成功！感谢您的评价', icon: 'none' })

      setTimeout(() => {
        const pages = getCurrentPages()
        const prevPage = pages[pages.length - 2] as WechatMiniprogram.Page.Instance<WechatMiniprogram.IAnyObject, WechatMiniprogram.IAnyObject> & ReviewListRefreshPage | undefined
        if (typeof prevPage?.onReviewUpdated === 'function') {
          prevPage.onReviewUpdated()
        }
        wx.navigateBack()
      }, 1000)
    } catch (error: unknown) {
      logger.error(this.data.mode === 'edit' ? '更新评价失败' : '提交评价失败', error, 'reviews/create')
      this.setData({ submitting: false })
      const msg = getErrorUserMessage(error, this.data.mode === 'edit' ? '保存失败，请重试' : '提交失败，请重试')
      wx.showToast({ title: msg, icon: 'none' })
    }
  }
})
