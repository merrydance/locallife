"use strict";
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
const review_1 = require("../../../api/review");
Page({
    data: {
        orderId: 0,
        rating: 5,
        content: '',
        fileList: [], // For t-upload
        uploadAction: 'https://myserver.com/upload', // Mock upload URL
        submitting: false
    },
    onLoad(options) {
        if (options.orderId) {
            this.setData({ orderId: parseInt(options.orderId) });
        }
    },
    onRateChange(e) {
        this.setData({ rating: e.detail.value });
    },
    onContentInput(e) {
        this.setData({ content: e.detail.value });
    },
    onAddFile(e) {
        const { fileList } = this.data;
        const { files } = e.detail;
        // Mock upload: usually we upload here and get URL. 
        // For simple demo, we just append local path and pretend it's a URL in submit.
        // In real app, use wx.uploadFile
        const newFiles = files.map((file) => (Object.assign(Object.assign({}, file), { url: file.url, status: 'done' })));
        this.setData({
            fileList: [...fileList, ...newFiles]
        });
    },
    onRemoveFile(e) {
        const { index } = e.detail;
        const { fileList } = this.data;
        fileList.splice(index, 1);
        this.setData({ fileList });
    },
    onSubmit() {
        return __awaiter(this, void 0, void 0, function* () {
            const { orderId, rating, content, fileList } = this.data;
            if (!content) {
                wx.showToast({ title: '虽然不想写，但还是写点评价吧', icon: 'none' });
                return;
            }
            this.setData({ submitting: true });
            try {
                const images = fileList.map(f => f.url);
                yield review_1.ReviewService.createReview({
                    order_id: orderId,
                    rating: rating,
                    content,
                    images
                });
                wx.showToast({ title: '评价成功', icon: 'success' });
                setTimeout(() => {
                    wx.navigateBack();
                }, 1500);
            }
            catch (error) {
                wx.showToast({ title: error.message || '评价失败', icon: 'none' });
            }
            finally {
                this.setData({ submitting: false });
            }
        });
    }
});
