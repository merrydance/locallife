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
        loadImage(url) {
            return __awaiter(this, void 0, void 0, function* () {
                // 如果是本地路径或base64，直接显示
                if (!url || url.startsWith('wxfile://') || url.startsWith('http://tmp/') || url.startsWith('data:image')) {
                    this.setData({ localPath: url, loading: false, error: false });
                    return;
                }
                this.setData({ loading: true, error: false });
                try {
                    // Determine if we need to sign it
                    const resolvedUrl = yield (0, image_security_1.resolveImageURL)(url);
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
            });
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
