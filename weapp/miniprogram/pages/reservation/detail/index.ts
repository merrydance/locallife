import { ReservationService } from '../../../api/reservation'
import { processPayment } from '../../../api/payment'
import Navigation from '../../../utils/navigation'
import { ReservationCardAdapter, ReservationDetailViewModel } from '../../../adapters/reservation-card'
import { logger } from '../../../utils/logger'

const getErrorMessage = (error: unknown, fallback: string): string => {
    if (error && typeof error === 'object' && 'message' in error) {
        const { message } = error as { message?: unknown }
        if (typeof message === 'string' && message.trim()) {
            return message
        }
    }
    return fallback
}

// 取消原因
const CANCEL_REASONS = [
    '行程改变',
    '订错了',
    '不想去了',
    '其他原因'
]

Page({
    data: {
        id: 0,
        reservation: null as ReservationDetailViewModel | null,
        loading: true,
        isError: false,
        errorMessage: '',
        navBarHeight: 88,
        
        // Dialog State
        showCancelDialog: false,
        cancelReason: '',
        cancelReasons: CANCEL_REASONS
    },

    onLoad(options: { id?: string }) {
        if (options.id) {
            this.setData({ id: parseInt(options.id) })
            this.loadDetail()
        }
    },

    onShow() {
        if (this.data.id && this.data.reservation) {
            this.loadDetail()
        }
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent) {
        this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
    },

    onRetry() {
        this.loadDetail()
    },

    async loadDetail() {
        // Only show full page loading if no data yet (first load or retry)
        const isFirstLoad = !this.data.reservation
        if (isFirstLoad) {
           this.setData({ loading: true, isError: false })
        }
        
        try {
            const res = await ReservationService.getReservationDetail(this.data.id)
            const viewModel = ReservationCardAdapter.toDetailViewModel(res)

            this.setData({
                reservation: viewModel,
                loading: false
            })
        } catch (error: unknown) {
            logger.error('Load reservation detail failed', error)
            
            if (isFirstLoad) {
                this.setData({ 
                    loading: false,
                    isError: true,
                    errorMessage: getErrorMessage(error, '加载失败')
                })
            } else {
                wx.showToast({ title: '刷新失败', icon: 'none' })
            }
        }
    },

    // Navigation
    onEnterMerchant() {
        const mid = this.data.reservation?.merchantId
        if (mid) {
             // 假设商户详情页
            wx.navigateTo({ url: `/pages/takeout/restaurant-detail/index?id=${mid}` })
        }
    },

    onNavMerchant() {
        const address = this.data.reservation?.merchantAddress
        if (!address) {
             wx.showToast({ title: '暂无地址信息', icon: 'none' })
             return
        }
        // 简单处理，实际应解析经纬度
        wx.openLocation({
            latitude: 39.9, // Demo default
            longitude: 116.4,
            name: this.data.reservation?.merchantName,
            address
        })
    },

    onCallMerchant() {
        const phone = this.data.reservation?.merchantPhone || this.data.reservation?.contactPhone
        if (phone) {
            wx.makePhoneCall({ phoneNumber: phone })
        } else {
            wx.showToast({ title: '暂无电话', icon: 'none' })
        }
    },

    // Actions
    onCancel() {
        this.setData({ showCancelDialog: true })
    },
    
    closeCancelDialog() {
        this.setData({ showCancelDialog: false })
    },

    onReasonChange(e: WechatMiniprogram.CustomEvent) {
        this.setData({ cancelReason: e.detail.value })
    },

    async confirmCancel() {
        if (!this.data.cancelReason) {
             wx.showToast({ title: '请选择原因', icon: 'none' })
             return
        }
        
        wx.showLoading({ title: '提交中' })
        try {
            await ReservationService.cancelReservation(this.data.id, this.data.cancelReason)
            wx.showToast({ title: '已取消', icon: 'success' })
            this.closeCancelDialog()
            this.loadDetail()
        } catch (e) {
            logger.error('Cancel failed', e)
            wx.showToast({ title: '取消失败', icon: 'none' })
        } finally {
            wx.hideLoading()
        }
    },

    async onPay() {
        if (!this.data.reservation) return
        wx.showLoading({ title: '拉起支付' })
        try {
            await processPayment(this.data.reservation.id, 'reservation')
            
            Navigation.toPaymentSuccess({
                orderId: String(this.data.id),
                orderNo: String(this.data.id),
                amount: this.data.reservation.depositDisplay?.replace('¥', '') || '0.00'
            })
        } catch (e) {
            logger.error('Pay failed', e)
            wx.showToast({ title: '支付未完成', icon: 'none' })
        } finally {
            wx.hideLoading()
        }
    },
    
    // 跳转到点菜页面进行加菜/修改 (Reservation Context)
    onModifyDishes() {
         const res = this.data.reservation
         if (!res) return

         // 传递 reservation_id 让 menu 页面识别是预订点餐
         wx.navigateTo({
             url: `/pages/dine-in/menu/menu?reservation_id=${res.id}&merchant_id=${res.merchantId}`
         })
    },

    // 再次预订 (跳转预订表单)
    onRebook() {
        const res = this.data.reservation
        if (!res) return
        wx.navigateTo({
            url: `/pages/reservation/create/index?merchantId=${res.merchantId}`
        })
    },

    // Copy ID
    onCopy(e: WechatMiniprogram.BaseEvent) {
        const text = e.currentTarget.dataset.text
        if (text) {
            wx.setClipboardData({ data: String(text) })
        }
    }
})
