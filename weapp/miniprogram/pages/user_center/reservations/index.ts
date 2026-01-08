/**
 * 我的预订页面
 * 显示用户的所有预订记录
 */

import { ReservationService, ReservationResponse, ReservationStatus } from '../../../api/reservation'
import { logger } from '../../../utils/logger'

// 状态筛选选项
const STATUS_TABS = [
    { label: '全部', value: '' },
    { label: '待支付', value: 'pending' },
    { label: '已确认', value: 'confirmed' },
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

Page({
    data: {
        reservations: [] as ReservationResponse[],
        navBarHeight: 88,
        loading: false,
        page: 1,
        pageSize: 10,
        hasMore: true,
        statusTabs: STATUS_TABS,
        currentStatus: '' as ReservationStatus | ''
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

    async loadReservations(reset = false) {
        if (this.data.loading) return
        this.setData({ loading: true })

        if (reset) {
            this.setData({ page: 1, reservations: [], hasMore: true })
        }

        try {
            const { currentStatus, page, pageSize } = this.data
            const params: any = { page_id: page, page_size: pageSize }
            if (currentStatus) {
                params.status = currentStatus
            }

            const response = await ReservationService.getUserReservations(params)
            const result = response.reservations

            // 处理显示字段
            const processedReservations = result.map((r: ReservationResponse) => this.processReservation(r))
            const reservations = reset ? processedReservations : [...this.data.reservations, ...processedReservations]

            this.setData({
                reservations,
                loading: false,
                hasMore: result.length === pageSize
            })
        } catch (error) {
            logger.error('加载预订列表失败', error, 'reservations.loadReservations')
            wx.showToast({ title: '加载失败', icon: 'error' })
            this.setData({ loading: false })
        }
    },

    processReservation(r: ReservationResponse): ReservationResponse & { _statusText: string; _statusClass: string; _canCancel: boolean; _canOrder: boolean; _dateTimeDisplay: string; _depositDisplay: string } {
        return {
            ...r,
            _statusText: this.getStatusText(r.status || ''),
            _statusClass: r.status || '',
            _canCancel: ['pending', 'paid', 'confirmed'].includes(r.status || ''),
            _canOrder: ['confirmed', 'checked_in'].includes(r.status || ''),  // 已确认或已签到可点菜
            _dateTimeDisplay: r.reservation_time, // Interface uses reservation_time for full datetime
            _depositDisplay: r.deposit_amount ? `¥${(r.deposit_amount / 100).toFixed(2)}` : ''
        }
    },

    getStatusText(status: string): string {
        const statusMap: Record<string, string> = {
            'pending': '待支付',
            'paid': '已支付',
            'confirmed': '已确认',
            'completed': '已完成',
            'cancelled': '已取消',
            'no_show': '未到店'
        }
        return statusMap[status] || status
    },

    onStatusChange(e: WechatMiniprogram.CustomEvent) {
        const status = e.detail.value || ''
        if (status === this.data.currentStatus) return
        this.setData({ currentStatus: status })
        this.loadReservations(true)
    },

    onViewDetail(e: WechatMiniprogram.BaseEvent) {
        const { id } = e.currentTarget.dataset
        wx.navigateTo({
            url: `/pages/user_center/reservations/detail/index?id=${id}`
        })
    },

    onCancelReservation(e: WechatMiniprogram.BaseEvent) {
        const { id } = e.currentTarget.dataset
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
     * 跳转到点菜页面
     */
    onGoToOrder(e: WechatMiniprogram.BaseEvent) {
        const item = e.currentTarget.dataset.item as ReservationResponse
        if (!item) return

        // 跳转到堂食点餐页面，传递预订ID和商户ID
        wx.navigateTo({
            url: `/pages/dine-in/menu/menu?reservation_id=${item.id}&merchant_id=${item.merchant_id}`
        })
    }
})
