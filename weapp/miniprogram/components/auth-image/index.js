"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const image_security_1 = require("../../utils/image-security");
Component({
    externalClasses: ['class', 'custom-class'],
    properties: {
        src: {
            type: String,
            observer(newVal) {
                if (newVal) {
                    this.loadImage(newVal);
                }
            }
        },
        mode: {
            type: String,
            value: 'scaleToFill'
        },
        lazyLoad: {
            type: Boolean,
            value: false
        },
        showMenuByLongpress: {
            type: Boolean,
            value: false
        },
        className: {
            type: String,
            value: ''
        },
        customStyle: {
            type: String,
            value: ''
        }
    },
    data: {
        localPath: '',
        loading: false,
        error: false
    },
    methods: {
        async loadImage(url) {
            // 如果是本地路径或base64，直接显示
            if (!url || url.startsWith('wxfile://') || url.startsWith('http://tmp/') || url.startsWith('data:image')) {
                this.setData({ localPath: url, loading: false, error: false });
                return;
            }
            this.setData({ loading: true, error: false });
            try {
                // Determine if we need to sign it
                const resolvedUrl = await (0, image_security_1.resolveImageURL)(url);
                this.setData({
                    localPath: resolvedUrl, // Use localPath to store the displayable URL
                    loading: false
                });
                this.triggerEvent('load', { path: resolvedUrl });
            }
            catch (e) {
                console.error('Failed to resolve image URL', e);
                this.setData({ loading: false, error: true });
                this.triggerEvent('error', e);
            }
        },
        retry() {
            if (this.properties.src) {
                this.loadImage(this.properties.src);
            }
        },
        onError(e) {
            this.triggerEvent('error', e);
        },
        onLoad(e) {
            this.triggerEvent('load', e);
        }
    }
});
