import { ReservationService } from '../../../api/reservation'
import Navigation from '../../../utils/navigation'
import { ReservationCardAdapter, ReservationDetailViewModel } from '../../../adapters/reservation-card'
import { logger } from '../../../utils/logger'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { startPaymentOrderWorkflow } from '../../../services/payment-workflow'

const getErrorMessage = getErrorUserMessage

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
        refreshErrorMessage: '',
        navBarHeight: 88,
        paying: false,
        
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
              this.setData({ loading: true, isError: false, refreshErrorMessage: '' })
          } else {
              this.setData({ refreshErrorMessage: '' })
        }
        
        try {
            const res = await ReservationService.getReservationDetail(this.data.id)
            const viewModel = ReservationCardAdapter.toDetailViewModel(res)

            this.setData({
                reservation: viewModel,
                loading: false,
                refreshErrorMessage: ''
            })
        } catch (error: unknown) {
            logger.error('Load reservation detail failed', error)
            
            if (isFirstLoad) {
                this.setData({ 
                    loading: false,
                    isError: true,
                    errorMessage: getErrorMessage(error, '加载失败'),
                    refreshErrorMessage: ''
                })
            } else {
                this.setData({
                    refreshErrorMessage: `${getErrorMessage(error, '刷新失败，请稍后重试')}，当前已保留上次结果`
                })
            }
        }
    },

    onRetryRefresh() {
        this.loadDetail()
    },

    // Navigation
    onEnterMerchant() {
        const mid = this.data.reservation?.merchantId
        if (mid) {
            Navigation.toRestaurantDetail(mid)
        }
    },

    onNavMerchant() {
        const address = this.data.reservation?.merchantAddress
        if (!address) {
             wx.showToast({ title: '暂无地址信息', icon: 'none' })
             return
        }
        wx.showToast({ title: '暂不支持地图导航，请联系商家确认位置', icon: 'none' })
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
        if (!this.data.reservation || this.data.paying) return
        this.setData({ paying: true })
        try {
            const paymentResult = await startPaymentOrderWorkflow({
                orderId: this.data.reservation.id,
                businessType: 'reservation'
            })

            Navigation.toPaymentResult({
                status: paymentResult.status,
                paymentOrderId: paymentResult.paymentOrderId,
                businessId: this.data.id,
                businessType: 'reservation',
                orderNo: paymentResult.outTradeNo,
                amount: this.data.reservation.depositDisplay?.replace('¥', '') || '0.00'
            })
        } catch (e) {
            logger.error('Pay failed', e)
            Navigation.toPaymentResult({
                status: 'pending_confirmation',
                businessId: this.data.id,
                businessType: 'reservation',
                amount: this.data.reservation.depositDisplay?.replace('¥', '') || '0.00'
            })
        } finally {
            this.setData({ paying: false })
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
        Navigation.toReservationCreate({
            merchantId: res.merchantId,
            merchantName: res.merchantName || ''
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
