import CouponService, { UserCoupon, MyCouponParams } from '../../../api/coupon'
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
        activeTab: 'AVAILABLE', // 'AVAILABLE' | 'ALL'
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
            const page = reset ? 1 : this.data.page
            let hasMore = false
            let nextPage = page + 1

            if (this.data.activeTab === 'AVAILABLE') {
                const params: MyCouponParams = {
                    page_id: page,
                    page_size: this.data.pageSize
                }
                const res = await CouponService.getMyAvailableCoupons(params)
                list = res.coupons.map((c) => this.mapUserCouponToView(c))
                hasMore = res.hasMore
                nextPage = res.page + 1
            } else {
                const params: MyCouponParams = {
                    page_id: page,
                    page_size: this.data.pageSize
                }
                const res = await CouponService.getMyCoupons(params)
                list = res.coupons.map((c) => this.mapUserCouponToView(c))
                hasMore = res.hasMore
                nextPage = res.page + 1
            }

            const newCoupons = reset ? list : [...this.data.coupons, ...list]
            this.setData({
                coupons: newCoupons,
                hasMore,
                page: nextPage,
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
    
    onGoToAvailable() {
         this.setData({ activeTab: 'AVAILABLE', page: 1, coupons: [] }, () => {
             this.loadCoupons(true)
         })
    },

    mapUserCouponToView(item: UserCoupon): CouponViewModel {
        const statusMap: Record<string, string> = {
            'available': '可使用',
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
