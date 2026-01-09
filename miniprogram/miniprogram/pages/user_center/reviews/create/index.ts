/**
 * 创建评价页面
 */

import { createReview, CreateReviewRequest } from '../../../../api/personal'
import { getOrderDetail } from '../../../../api/order'
import { logger } from '../../../../utils/logger'

Page({
    data: {
        orderId: 0,
        merchantId: 0,
        merchantName: '',
        navBarHeight: 88,
        loading: false,
        submitting: false,
        // 表单数据
        rating: 5,
        content: '',
        images: [] as string[],
        // 评分选项
        ratingOptions: [
            { value: 5, label: '非常满意', icon: '😍' },
            { value: 4, label: '满意', icon: '😊' },
            { value: 3, label: '一般', icon: '😐' },
            { value: 2, label: '不满意', icon: '😕' },
            { value: 1, label: '非常不满意', icon: '😞' }
        ],
        maxImages: 9,
        maxContentLength: 500
    },

    onLoad(options: { orderId?: string; merchantId?: string }) {
        if (options.orderId) {
            this.setData({ orderId: parseInt(options.orderId) })
            this.loadOrderInfo()
        }
        if (options.merchantId) {
            this.setData({ merchantId: parseInt(options.merchantId) })
        }
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    },

    async loadOrderInfo() {
        try {
            const order = await getOrderDetail(this.data.orderId)
            this.setData({
                merchantId: order.merchant_id,
                merchantName: order.merchant_name
            })
        } catch (error) {
            logger.error('加载订单信息失败', error, 'reviews/create.loadOrderInfo')
        }
    },

    onRatingChange(e: WechatMiniprogram.CustomEvent) {
        const rating = e.currentTarget.dataset.rating
        this.setData({ rating })
    },

    onContentInput(e: WechatMiniprogram.CustomEvent) {
        this.setData({ content: e.detail.value })
    },

    async onChooseImage() {
        const { images, maxImages } = this.data
        const remaining = maxImages - images.length

        if (remaining <= 0) {
            wx.showToast({ title: `最多上传${maxImages}张图片`, icon: 'none' })
            return
        }

        try {
            const res = await wx.chooseMedia({
                count: remaining,
                mediaType: ['image'],
                sourceType: ['album', 'camera']
            })

            // 上传图片到服务器
            const uploadedUrls: string[] = []
            for (const file of res.tempFiles) {
                const url = await this.uploadImage(file.tempFilePath)
                if (url) {
                    uploadedUrls.push(url)
                }
            }

            this.setData({
                images: [...images, ...uploadedUrls]
            })
        } catch (error) {
            logger.error('选择图片失败', error, 'reviews/create.onChooseImage')
        }
    },

    async uploadImage(filePath: string): Promise<string | null> {
        try {
            // TODO: 实际上传到服务器，这里暂时返回本地路径
            // const res = await wx.uploadFile({
            //   url: 'YOUR_UPLOAD_URL',
            //   filePath,
            //   name: 'file'
            // })
            // return JSON.parse(res.data).url
            return filePath
        } catch (error) {
            logger.error('上传图片失败', error, 'reviews/create.uploadImage')
            return null
        }
    },

    onRemoveImage(e: WechatMiniprogram.CustomEvent) {
        const index = e.currentTarget.dataset.index
        const images = [...this.data.images]
        images.splice(index, 1)
        this.setData({ images })
    },

    onPreviewImage(e: WechatMiniprogram.CustomEvent) {
        const url = e.currentTarget.dataset.url
        wx.previewImage({
            current: url,
            urls: this.data.images
        })
    },

    async onSubmit() {
        const { orderId, content, rating, images, submitting } = this.data

        if (submitting) return

        if (!content || content.length < 10) {
            wx.showToast({ title: '评价内容至少10个字', icon: 'none' })
            return
        }

        this.setData({ submitting: true })

        try {
            const reviewData: CreateReviewRequest = {
                order_id: orderId,
                content,
                images: images.length > 0 ? images : undefined
            }

            await createReview(reviewData)

            wx.showToast({ title: '评价成功', icon: 'success' })

            setTimeout(() => {
                wx.navigateBack()
            }, 1500)
        } catch (error) {
            logger.error('提交评价失败', error, 'reviews/create.onSubmit')
            wx.showToast({ title: '提交失败', icon: 'error' })
            this.setData({ submitting: false })
        }
    }
})
