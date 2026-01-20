"use strict";
/**
 * 创建评价页面
 */
Object.defineProperty(exports, "__esModule", { value: true });
const personal_1 = require("../../../../api/personal");
const review_1 = require("../../../../api/review");
const order_1 = require("../../../../api/order");
const logger_1 = require("../../../../utils/logger");
Page({
    data: {
        orderId: 0,
        merchantId: 0,
        merchantName: '',
        navBarHeight: 88,
        loading: false,
        submitting: false,
        // 表单数据
        content: '',
        images: [],
        maxImages: 9,
        maxContentLength: 500
    },
    onLoad(options) {
        if (options.orderId) {
            this.setData({ orderId: parseInt(options.orderId) });
            this.loadOrderInfo();
        }
        if (options.merchantId) {
            this.setData({ merchantId: parseInt(options.merchantId) });
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    async loadOrderInfo() {
        try {
            const order = await (0, order_1.getOrderDetail)(this.data.orderId);
            this.setData({
                merchantId: order.merchant_id,
                merchantName: order.merchant_name
            });
        }
        catch (error) {
            logger_1.logger.error('加载订单信息失败', error, 'reviews/create.loadOrderInfo');
        }
    },
    onContentInput(e) {
        this.setData({ content: e.detail.value });
    },
    async onChooseImage() {
        const { images, maxImages } = this.data;
        const remaining = maxImages - images.length;
        if (remaining <= 0) {
            wx.showToast({ title: `最多上传${maxImages}张图片`, icon: 'none' });
            return;
        }
        try {
            const res = await wx.chooseMedia({
                count: remaining,
                mediaType: ['image'],
                sourceType: ['album', 'camera']
            });
            // 上传图片到服务器
            const uploadedUrls = [];
            for (const file of res.tempFiles) {
                const url = await this.uploadImage(file.tempFilePath);
                if (url) {
                    uploadedUrls.push(url);
                }
            }
            this.setData({
                images: [...images, ...uploadedUrls]
            });
        }
        catch (error) {
            logger_1.logger.error('选择图片失败', error, 'reviews/create.onChooseImage');
        }
    },
    async uploadImage(filePath) {
        try {
            return await review_1.ReviewService.uploadReviewImage(filePath);
        }
        catch (error) {
            logger_1.logger.error('上传图片失败', error, 'reviews/create.uploadImage');
            return null;
        }
    },
    onRemoveImage(e) {
        const index = e.currentTarget.dataset.index;
        const images = [...this.data.images];
        images.splice(index, 1);
        this.setData({ images });
    },
    onPreviewImage(e) {
        const url = e.currentTarget.dataset.url;
        wx.previewImage({
            current: url,
            urls: this.data.images
        });
    },
    async onSubmit() {
        const { orderId, content, images, submitting } = this.data;
        if (submitting)
            return;
        if (!content || content.length < 10) {
            wx.showToast({ title: '评价内容至少10个字', icon: 'none' });
            return;
        }
        this.setData({ submitting: true });
        try {
            const reviewData = {
                order_id: orderId,
                content,
                images: images.length > 0 ? images : undefined
            };
            await (0, personal_1.createReview)(reviewData);
            wx.showToast({ title: '评价成功', icon: 'success' });
            setTimeout(() => {
                wx.navigateBack();
            }, 1500);
        }
        catch (error) {
            logger_1.logger.error('提交评价失败', error, 'reviews/create.onSubmit');
            wx.showToast({ title: '提交失败', icon: 'error' });
            this.setData({ submitting: false });
        }
    }
});
