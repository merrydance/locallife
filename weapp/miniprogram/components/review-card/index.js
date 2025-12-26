"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
Component({
    options: {
        addGlobalClass: true
    },
    properties: {
        review: {
            type: Object,
            value: null
        },
        // Whether to show merchant info instead of user info (e.g. in "My Reviews")
        showMerchant: {
            type: Boolean,
            value: false
        }
    },
    methods: {
        onPreviewImage(e) {
            const { index } = e.currentTarget.dataset;
            const images = this.properties.review.images || [];
            wx.previewImage({
                current: images[index],
                urls: images
            });
        },
        onReply(e) {
            // Trigger event for merchant reply
            this.triggerEvent('reply', { id: this.properties.review.id });
        }
    }
});
