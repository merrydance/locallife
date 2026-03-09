/**
 * 我的预订页面
 * 显示用户的所有预订记录
 */

import { ReservationService, ReservationStatus, ReservationListParams } from '../../../api/reservation'
import { logger } from '../../../utils/logger'
import { ReservationCardAdapter, ReservationCardViewModel } from '../../../adapters/reservation-card'
import { processPayment, PaymentCancelledError } from '../../../api/payment'
import Navigation from '../../../utils/navigation'

// 状态筛选选项
const STATUS_TABS = [
    { label: '全部', value: '' },
    { label: '待支付', value: 'pending' },
    { label: '已确认', value: 'confirmed' }, // 包含 confirmed, checked_in
    { label: '已完成', value: 'completed' },
    { label: '已取消', value: 'cancelled' }
]

// 取消原因选项
const CANCEL_REASONS = [
    '行程有变',
    '预订错误',
    '找到更好的选择',
    '其他原因'
]

const getEventId = (event: WechatMiniprogram.BaseEvent): number | null => {
    const dataset = event.currentTarget.dataset as { id?: string | number }
    const id = dataset.id
    const numericId = typeof id === 'number' ? id : Number(id)
    return Number.isFinite(numericId) ? numericId : null
}

Page({
    data: {
        reservations: [] as ReservationCardViewModel[],
        navBarHeight: 88,
        loading: false,
        initialLoading: true,
        error: null as string | null,
        page: 1,
        pageSize: 10,
        hasMore: true,
        statusTabs: STATUS_TABS,
        currentStatus: '' as string
    },

    onLoad() {
        this.loadReservations(true)
    },

    onShow() {
        if (this.data.reservations.length > 0) {
            this.loadReservations(true)
        }
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    },

    onReachBottom() {
        if (this.data.hasMore && !this.data.loading) {
            this.setData({ page: this.data.page + 1 })
            this.loadReservations(false)
        }
    },

    preventBubble() {},

    async loadReservations(reset = false) {
        if (this.data.loading && !this.data.initialLoading) return
        this.setData({ loading: true, error: null })

        if (reset) {
            this.setData({ page: 1, reservations: [], hasMore: true })
        }

        try {
            const { currentStatus, page, pageSize } = this.data
            const params: ReservationListParams = {
                page_id: page,
                page_size: pageSize,
                ...(currentStatus ? { status: currentStatus as ReservationStatus } : {})
            }

            const response = await ReservationService.getUserReservations(params)
            const result = response.reservations
            const totalCount = typeof response.total_count === 'number' ? response.total_count : result.length

            const viewModels = result.map((r) => ReservationCardAdapter.toCardViewModel(r))
            
            const reservations = reset ? viewModels : [...this.data.reservations, ...viewModels]

            this.setData({
                reservations,
                loading: false,
                initialLoading: false,
                hasMore: page * pageSize < totalCount
            })
        } catch (error) {
            logger.error('加载预订列表失败', error, 'reservations.loadReservations')
            this.setData({ 
                loading: false,
                initialLoading: false,
                error: '加载预订列表失败'
            })
        }
    },

    onRetry() {
        this.loadReservations(true)
    },

    onStatusChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
        const status = e.detail.value || ''
        if (status === this.data.currentStatus) return
        this.setData({ currentStatus: status })
        this.loadReservations(true)
    },

    onViewDetail(e: WechatMiniprogram.BaseEvent) {
        const id = getEventId(e)
        if (!id) return
        wx.navigateTo({
            url: `/pages/reservation/detail/index?id=${id}`
        })
    },

    onCancelReservation(e: WechatMiniprogram.BaseEvent) {
        const id = getEventId(e)
        if (!id) return

        wx.showActionSheet({
            itemList: CANCEL_REASONS,
            success: async (res) => {
                const reason = CANCEL_REASONS[res.tapIndex]
                await this.doCancelReservation(Number(id), reason)
            }
        })
    },

    async doCancelReservation(reservationId: number, reason: string) {
        wx.showLoading({ title: '取消中...' })
        try {
            await ReservationService.cancelReservation(reservationId, reason)
            wx.hideLoading()
            wx.showToast({ title: '已取消', icon: 'success' })
            setTimeout(() => this.loadReservations(true), 1500)
        } catch (error) {
            wx.hideLoading()
            logger.error('取消预订失败', error, 'reservations.doCancelReservation')
            wx.showToast({ title: '取消失败', icon: 'error' })
        }
    },

    /**
     * 去支付
     */
    async onPayReservation(e: WechatMiniprogram.BaseEvent) {
        const id = getEventId(e)
        if (!id) return

        wx.showLoading({ title: '拉起支付...' })
        try {
             // 预订支付通常是定金，这里的objectType可能是 'reservation'
            await processPayment(id, 'reservation')
            
            // 找到对应的预订数据用于展示
            const res = this.data.reservations.find((r) => r.id === id)
            Navigation.toPaymentSuccess({
                orderId: String(id),
                orderNo: String(id), // 预订号暂用ID
                amount: res?.depositDisplay?.replace('¥', '') || '0.00'
            })
        } catch (error) {
            if (error instanceof PaymentCancelledError) {
                wx.showToast({ title: '已取消支付', icon: 'none' })
            } else {
                logger.error('支付失败', error, 'Reservations.onPay')
                wx.showToast({ title: '支付失败', icon: 'none' })
            }
        } finally {
            wx.hideLoading()
        }
    },

    /**
     * 跳转到点菜页面
     */
    onGoToOrder(e: WechatMiniprogram.BaseEvent) {
        const item = (e.currentTarget.dataset as { item?: ReservationCardViewModel }).item
        if (!item) return

        // 跳转到堂食点餐页面，传递预订ID和商户ID
        // 注意：这里需要确保 menu 页面支持 reservation_id 参数
        wx.navigateTo({
            url: `/pages/dine-in/menu/menu?reservation_id=${item.id}&merchant_id=${item.merchantId}`
        })
    }
})
