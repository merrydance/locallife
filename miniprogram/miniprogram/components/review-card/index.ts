import { ReviewResponse } from '../../api/review';

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
        onPreviewImage(e: WechatMiniprogram.TouchEvent) {
            const { index } = e.currentTarget.dataset;
            const images = (this.properties.review as any).images || [];

            wx.previewImage({
                current: images[index],
                urls: images
            });
        },

        onReply(e: WechatMiniprogram.TouchEvent) {
            // Trigger event for merchant reply
            this.triggerEvent('reply', { id: (this.properties.review as any).id });
        }
    }
});
