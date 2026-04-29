import type { CustomerMerchantCouponView } from '../../services/customer-discovery-workflow'

Component({
  properties: {
    coupons: {
      type: Array,
      value: [] as CustomerMerchantCouponView[]
    },
    claimingCouponId: {
      type: Number,
      value: 0
    }
  },

  methods: {
    onClaimCoupon(e: WechatMiniprogram.CustomEvent) {
      const couponId = Number(e.currentTarget.dataset.id)
      if (!Number.isFinite(couponId)) return
      this.triggerEvent('claim', { id: couponId })
    }
  }
})
