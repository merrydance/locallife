"use strict";
/**
 * 创建评价页面
 */
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
const personal_1 = require("../../../../api/personal");
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
        rating: 5,
        content: '',
        images: [],
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
    loadOrderInfo() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const order = yield (0, order_1.getOrderDetail)(this.data.orderId);
                this.setData({
                    merchantId: order.merchant_id,
                    merchantName: order.merchant_name
                });
            }
            catch (error) {
                logger_1.logger.error('加载订单信息失败', error, 'reviews/create.loadOrderInfo');
            }
        });
    },
    onRatingChange(e) {
        const rating = e.currentTarget.dataset.rating;
        this.setData({ rating });
    },
    onContentInput(e) {
        this.setData({ content: e.detail.value });
    },
    onChooseImage() {
        return __awaiter(this, void 0, void 0, function* () {
            const { images, maxImages } = this.data;
            const remaining = maxImages - images.length;
            if (remaining <= 0) {
                wx.showToast({ title: `最多上传${maxImages}张图片`, icon: 'none' });
                return;
            }
            try {
                const res = yield wx.chooseMedia({
                    count: remaining,
                    mediaType: ['image'],
                    sourceType: ['album', 'camera']
                });
                // 上传图片到服务器
                const uploadedUrls = [];
                for (const file of res.tempFiles) {
                    const url = yield this.uploadImage(file.tempFilePath);
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
        });
    },
    uploadImage(filePath) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                // TODO: 实际上传到服务器，这里暂时返回本地路径
                // const res = await wx.uploadFile({
                //   url: 'YOUR_UPLOAD_URL',
                //   filePath,
                //   name: 'file'
                // })
                // return JSON.parse(res.data).url
                return filePath;
            }
            catch (error) {
                logger_1.logger.error('上传图片失败', error, 'reviews/create.uploadImage');
                return null;
            }
        });
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
    onSubmit() {
        return __awaiter(this, void 0, void 0, function* () {
            const { orderId, content, rating, images, submitting } = this.data;
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
                yield (0, personal_1.createReview)(reviewData);
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
        });
    }
});
