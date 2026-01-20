"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
Component({
    options: {
        addGlobalClass: true
    },
    properties: {
        review: {
            type: Object,
            value: undefined
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
            const review = this.properties.review;
            const images = (review === null || review === void 0 ? void 0 : review.images) || [];
            wx.previewImage({
                current: images[index],
                urls: images
            });
        },
        onReply(e) {
            // Trigger event for merchant reply
            const review = this.properties.review;
            if (!(review === null || review === void 0 ? void 0 : review.id))
                return;
            this.triggerEvent('reply', { id: review.id });
        }
    }
});
