import { ReviewService, CreateReviewParams } from '../../../../api/review'
import { getOrderDetail } from '../../../../api/order'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'

interface ReviewUploadFile {
  url: string
  status?: 'loading' | 'done' | 'failed'
  remotePath?: string
}

Page({
  data: {
    orderId: 0,
    merchantId: 0,
    merchantName: '',
    orderNo: '',
    navBarHeight: 88,
    loading: false,
    initialLoading: true,
    submitting: false,

    // 评分
    rating: 5,
    ratingLabels: ['很差', '较差', '一般', '较好', '非常好'],

    // 快捷标签
    quickTags: [
      '山珍多汁', '卖相工整', '分量足', '右葛油产品', '干净卫生',
      '服务好', '送餐快', '包装好', '性价比高', '值得复垂'
    ] as string[],
    selectedTags: [] as string[],

    // 表单数据
    content: '',
    fileList: [] as ReviewUploadFile[],
    maxImages: 9,
    maxContentLength: 500
  },

  onLoad(options: { orderId?: string }) {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })

    if (options.orderId) {
      this.setData({ orderId: parseInt(options.orderId) })
      this.loadOrderInfo()
    } else {
      wx.showToast({ title: '无效的任务订单', icon: 'none' })
      setTimeout(() => wx.navigateBack(), 2000)
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

  onContentChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ content: e.detail.value })
  },

  onRatingChange(e: WechatMiniprogram.CustomEvent<{ value: number }>) {
    this.setData({ rating: e.detail.value })
  },

  onTagTap(e: WechatMiniprogram.TouchEvent) {
    const { tag } = e.currentTarget.dataset as { tag: string }
    const selected = [...this.data.selectedTags]
    const idx = selected.indexOf(tag)
    if (idx === -1) {
      selected.push(tag)
    } else {
      selected.splice(idx, 1)
    }
    this.setData({ selectedTags: selected })
  },

  // 图片添加回调
  async onAddImage(e: WechatMiniprogram.CustomEvent<{ files: Array<{ url: string }> }>) {
    const { files } = e.detail
    const { fileList } = this.data

    // 先展示在界面上 (status: loading)
    const newFiles: ReviewUploadFile[] = files.map((f) => ({
      ...f,
      status: 'loading'
    }))

    this.setData({
      fileList: [...fileList, ...newFiles]
    })

    // 逐个开始上传
    for (let i = 0; i < newFiles.length; i++) {
        const file = newFiles[i]
        const currentIndex = fileList.length + i
        
        try {
            const url = await ReviewService.uploadReviewImage(file.url)
            this.updateFileStatus(currentIndex, 'done', url)
        } catch (err) {
            this.updateFileStatus(currentIndex, 'failed')
        }
    }
  },

  updateFileStatus(index: number, status: 'loading' | 'done' | 'failed', url?: string) {
    const { fileList } = this.data
    if (!fileList[index]) return
    
    fileList[index].status = status
    if (url) {
        // 后端返回的是相对路径，需要处理
        fileList[index].remotePath = url 
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
    const { orderId, content, fileList, submitting } = this.data

    if (submitting) return

    if (!content || content.length < 10) {
      wx.showToast({ title: '评价内容至少10个字', icon: 'none' })
      return
    }

    // 检查上传状态
    const uploading = fileList.some((f) => f.status === 'loading')
    if (uploading) {
      wx.showToast({ title: '正在上传图片中，请稍候', icon: 'none' })
        return
    }

    this.setData({ submitting: true })

    try {
      // 提取成功上传的远程路径
      const remoteImages = fileList
        .filter((f) => f.status === 'done' && f.remotePath)
        .map((f) => f.remotePath)
        .filter((path): path is string => Boolean(path))

      const reviewData: CreateReviewParams = {
        order_id: orderId,
        content,
        rating: this.data.rating,
        tags: this.data.selectedTags.length > 0 ? this.data.selectedTags : undefined,
        images: remoteImages.length > 0 ? remoteImages : undefined
      }

      await ReviewService.createReview(reviewData)

      wx.showToast({ title: '发布成功！感谢您的评价', icon: 'none' })
      
      setTimeout(() => {
        // 触发上级页面刷新
        const pages = getCurrentPages()
        const prevPage = pages[pages.length - 2]
        if (prevPage && prevPage.route.includes('orders/detail')) {
            // 如果是从订单详情来的，可能需要刷新详情
        }
        wx.navigateBack()
      }, 1500)
    } catch (error: unknown) {
      logger.error('提交评价失败', error, 'reviews/create')
      this.setData({ submitting: false })
      // 这里的友好提示可以根据错误码处理
      const msg = error instanceof Error ? error.message : '提交失败，请重试'
      wx.showToast({ title: msg, icon: 'none' })
    }
  }
})
