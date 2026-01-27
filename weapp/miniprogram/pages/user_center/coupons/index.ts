import CouponService, { Coupon, UserCoupon, CouponListParams, MyCouponParams } from '../../../api/coupon'
import { logger } from '../../../utils/logger'
import { ErrorHandler } from '../../../utils/error-handler'

// ViewModel
interface CouponViewModel {
    id: number
    title: string
    merchantName: string
    valueDisplay: string
    minSpendDisplay: string
    timeRange: string
    status: string // 'available' | 'used' | 'expired' | 'claimed'
    statusClass: string
    statusText: string
    isClaimed: boolean
}

Page({
    data: {
        activeTab: 'AVAILABLE', // 'AVAILABLE' | 'MY'
        coupons: [] as CouponViewModel[],
        navBarHeight: 88,
        loading: false,
        initialLoading: true,
        error: null as string | null,
        
        // Paging
        page: 1,
        pageSize: 10,
        hasMore: true
    },

    onLoad() {
        this.loadCoupons(true)
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    },

    onTabChange(e: WechatMiniprogram.CustomEvent) {
        this.setData({ 
            activeTab: e.detail.value,
            coupons: [],
            page: 1,
            hasMore: true
        }, () => {
            this.loadCoupons(true)
        })
    },

    async loadCoupons(reset = false) {
        if (!this.data.hasMore && !reset) return
        if (this.data.loading && !this.data.initialLoading) return

        this.setData({ loading: true, error: null })

        try {
            let list: CouponViewModel[] = []
            let total = 0

            if (this.data.activeTab === 'AVAILABLE') {
                // Fetch Available Coupons
                const params: CouponListParams = {
                    page_id: this.data.page,
                    page_size: this.data.pageSize
                }
                const res = await CouponService.getAvailableCoupons(params)
                list = res.coupons.map(c => this.mapCouponToView(c))
                total = res.total
            } else {
                // Fetch My Coupons
                const params: MyCouponParams = {
                    page_id: this.data.page,
                    page_size: this.data.pageSize
                }
                const res = await CouponService.getMyCoupons(params)
                list = res.coupons.map(c => this.mapUserCouponToView(c))
                total = res.total
            }

            const newCoupons = reset ? list : [...this.data.coupons, ...list]
            this.setData({
                coupons: newCoupons,
                hasMore: newCoupons.length < total,
                page: this.data.page + 1,
                loading: false,
                initialLoading: false
            })
        } catch (error) {
            this.setData({ 
                loading: false,
                initialLoading: false,
                error: '加载优惠券失败'
            })
            ErrorHandler.handle(error, 'Coupons.load')
        }
    },

    onRetry() {
        this.loadCoupons(true)
    },

    onReachBottom() {
        this.loadCoupons()
    },

    async onClaim(e: WechatMiniprogram.BaseEvent) {
        const id = e.currentTarget.dataset.id
        if (!id) return

        wx.showLoading({ title: '领取中' })
        try {
            await CouponService.claimCoupon(id)
            wx.showToast({ title: '领取成功', icon: 'success' })
            
            // Optimistic update: mark as claimed
            const coupons = this.data.coupons.map(c => {
                if (c.id === id) {
                    return { ...c, isClaimed: true }
                }
                return c
            })
            this.setData({ coupons })
        } catch (error) {
           ErrorHandler.handle(error, 'Coupons.claim')
        } finally {
            wx.hideLoading()
        }
    },
    
    // Switch to available tab
    onGoToCenter() {
         this.setData({ activeTab: 'AVAILABLE', page: 1, coupons: [] }, () => {
             this.loadCoupons(true)
         })
    },

    // Mappers
    mapCouponToView(item: Coupon): CouponViewModel {
        return {
            id: item.id,
            title: item.title,
            merchantName: item.merchant_name,
            valueDisplay: item.type === 'discount' ? `${item.value}` : `${item.value / 100}`,
            minSpendDisplay: `${item.min_spend / 100}`,
            timeRange: `${this.formatDate(item.start_time)} - ${this.formatDate(item.end_time)}`,
            status: item.is_claimed ? 'claimed' : 'available',
            statusClass: item.is_claimed ? 'disabled' : 'normal',
            statusText: item.is_claimed ? '已领取' : '立即领取',
            isClaimed: !!item.is_claimed
        }
    },

    mapUserCouponToView(item: UserCoupon): CouponViewModel {
        const statusMap: Record<string, string> = {
            'available': '未使用',
            'used': '已使用',
            'expired': '已过期'
        }
        return {
            id: item.id,
            title: item.title,
            merchantName: item.merchant_name,
            valueDisplay: item.type === 'discount' ? `${item.value}` : `${item.value / 100}`,
            minSpendDisplay: `${item.min_spend / 100}`,
            timeRange: `${this.formatDate(item.start_time)} - ${this.formatDate(item.end_time)}`,
            status: item.status, // available, used, expired
            statusClass: item.status,
            statusText: statusMap[item.status] || item.status,
            isClaimed: true
        }
    },

    formatDate(isoStr?: string): string {
        if (!isoStr) return ''
        return isoStr.split('T')[0]
    }
})
