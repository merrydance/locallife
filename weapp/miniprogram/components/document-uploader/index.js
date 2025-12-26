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
Component({
    properties: {
        value: {
            type: String,
            value: ''
        },
        // 原始 URL（未签名），用于重新签名
        rawUrl: {
            type: String,
            optionalTypes: [String, null],
            value: ''
        },
        type: {
            type: String,
            value: 'generic' // id-front, id-back, license, permit, shop
        },
        title: {
            type: String,
            value: ''
        },
        disabled: {
            type: Boolean,
            value: false
        }
    },
    data: {
        uploading: false,
        retryCount: 0,
        maxRetries: 2
    },
    methods: {
        onUpload() {
            return __awaiter(this, void 0, void 0, function* () {
                if (this.data.disabled || this.data.value)
                    return;
                try {
                    const res = yield wx.chooseMedia({
                        count: 1,
                        mediaType: ['image'],
                        sourceType: ['album', 'camera'],
                        sizeType: ['compressed']
                    });
                    const tempFilePath = res.tempFiles[0].tempFilePath;
                    // Emit event for parent to handle upload (or just display)
                    this.triggerEvent('change', { file: res.tempFiles[0], path: tempFilePath });
                }
                catch (err) {
                    // User cancelled or error
                    console.debug('Choose media cancelled/failed', err);
                }
            });
        },
        onRemove() {
            if (this.data.disabled)
                return;
            this.setData({ value: '' });
            this.triggerEvent('remove');
            this.triggerEvent('change', { file: null, path: '' });
        },
        onPreview() {
            if (this.data.value) {
                wx.previewImage({
                    urls: [this.data.value]
                });
            }
        },
        // 图片加载错误时触发，尝试重新签名
        onImageError() {
            const { retryCount, maxRetries } = this.data;
            const rawUrl = this.properties.rawUrl;
            console.warn('[document-uploader] 图片加载失败, retryCount:', retryCount);
            if (retryCount < maxRetries && rawUrl) {
                this.setData({ retryCount: retryCount + 1 });
                // 触发事件让父组件重新签名
                this.triggerEvent('imageerror', { rawUrl, retryCount: retryCount + 1 });
            }
        },
        // 重置重试计数
        resetRetry() {
            this.setData({ retryCount: 0 });
        }
    }
});
