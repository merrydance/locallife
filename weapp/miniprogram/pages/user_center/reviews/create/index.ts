import { ReviewService, CreateReviewParams } from '../../../../api/review'
import { getOrderDetail } from '../../../../api/order'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import Message from 'tdesign-miniprogram/message/index'

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

    // 表单数据
    content: '',
    fileList: [] as any[],
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
      Message.error({ context: this, offset: [navBarHeight, 0], content: '无效的任务订单' })
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
      Message.error({ context: this, content: '订单详情加载失败' })
    }
  },

  onContentChange(e: any) {
    this.setData({ content: e.detail.value })
  },

  // 图片添加回调
  async onAddImage(e: any) {
    const { files } = e.detail
    const { fileList } = this.data

    // 先展示在界面上 (status: loading)
    const newFiles = files.map((f: any) => ({
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

  onRemoveImage(e: any) {
    const { index } = e.detail
    const { fileList } = this.data
    fileList.splice(index, 1)
    this.setData({ fileList })
  },

  async onSubmit() {
    const { orderId, content, fileList, submitting } = this.data

    if (submitting) return

    if (!content || content.length < 10) {
      Message.warning({ context: this, content: '评价内容至少10个字' })
      return
    }

    // 检查上传状态
    const uploading = fileList.some(f => f.status === 'loading')
    if (uploading) {
        Message.warning({ context: this, content: '正在上传图片中，请稍候' })
        return
    }

    this.setData({ submitting: true })

    try {
      // 提取成功上传的远程路径
      const remoteImages = fileList
        .filter(f => f.status === 'done' && f.remotePath)
        .map(f => f.remotePath)

      const reviewData: CreateReviewParams = {
        order_id: orderId,
        content,
        images: remoteImages.length > 0 ? remoteImages : undefined
      }

      await ReviewService.createReview(reviewData)

      Message.success({ context: this, content: '发布成功！感谢您的评价' })
      
      setTimeout(() => {
        // 触发上级页面刷新
        const pages = getCurrentPages()
        const prevPage = pages[pages.length - 2]
        if (prevPage && prevPage.route.includes('orders/detail')) {
            // 如果是从订单详情来的，可能需要刷新详情
        }
        wx.navigateBack()
      }, 1500)
    } catch (error: any) {
      logger.error('提交评价失败', error, 'reviews/create')
      this.setData({ submitting: false })
      // 这里的友好提示可以根据错误码处理
      const msg = error.errMsg || '提交失败，请重试'
      Message.error({ context: this, content: msg })
    }
  }
})
