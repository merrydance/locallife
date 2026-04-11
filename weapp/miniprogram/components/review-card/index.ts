import { Review } from '../../api/review'

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
        onPreviewImage(e: WechatMiniprogram.TouchEvent) {
            const { index } = e.currentTarget.dataset
            const review = this.properties.review as Review | undefined
            const images = review?.images || []

            wx.previewImage({
                current: images[index],
                urls: images
            })
        },

        onReply(_e: WechatMiniprogram.TouchEvent) {
            // Trigger event for merchant reply
            const review = this.properties.review as Review | undefined
            if (!review?.id) return
            this.triggerEvent('reply', { id: review.id })
        }
    }
})
