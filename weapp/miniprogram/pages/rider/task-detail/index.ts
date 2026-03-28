import DeliveryService, { Delivery } from '../../../api/delivery'
import { logger } from '../../../utils/logger'
import { locationService } from '../../../utils/location'
import { normalizeLocationError, syncRiderDeliveryLocation } from '../../../utils/rider-location'
import { getStableBarHeights } from '../../../utils/responsive'

interface RiderTaskDetailOptions {
    id?: string
}

interface UserMessageError {
    userMessage?: string
}

type DeliveryAction = (deliveryId: number) => Promise<Delivery>

type DeliveryView = Delivery & {
    deadline_desc: string
}

Page({
    data: {
        orderId: 0,
        delivery: null as DeliveryView | null,
        loading: false,
        errorMessage: '',
        navBarHeight: 88,
        
        // 状态映射
        statusSteps: [
            { title: '已接单', status: 'assigned' },
            { title: '取餐中', status: 'picking' },
            { title: '配送中', status: 'delivering' },
            { title: '已送达', status: 'completed' }
        ],
        currentStep: 0
    },

    onLoad(options: RiderTaskDetailOptions) {
        const { navBarHeight } = getStableBarHeights()
        this.setData({ 
            navBarHeight,
            orderId: Number(options.id || 0)
        })
        this.fetchTaskDetail()
    },

    async fetchTaskDetail() {
        if (!this.data.orderId) return
        
        this.setData({ loading: true })
        try {
            // 使用正确的获取详情接口，而不是抢单接口
            const delivery = await DeliveryService.getDeliveryByOrder(this.data.orderId)
            const deliveryView = this.decorateDelivery(delivery)
            
            this.setData({ 
                delivery: deliveryView,
                currentStep: this.mapStatusToStep(delivery.status),
                errorMessage: ''
            })
        } catch (err: unknown) {
            logger.error('Fetch task detail failed', err)
            const message = err instanceof Error && err.message ? err.message : '任务详情加载失败，请稍后重试'
            this.setData({ delivery: null, errorMessage: message })
        } finally {
            this.setData({ loading: false })
        }
    },

    decorateDelivery(delivery: Delivery): DeliveryView {
        const deadline = delivery.status === 'assigned' || delivery.status === 'picking'
            ? delivery.estimated_pickup_at
            : delivery.estimated_delivery_at

        return {
            ...delivery,
            deadline_desc: this.formatDeadline(deadline)
        }
    },

    mapStatusToStep(status: string): number {
        const statusMap: Record<string, number> = {
            'assigned': 0,
            'picking': 1,
            'picked': 2,
            'delivering': 2,
            'delivered': 3,
            'completed': 3
        }
        return statusMap[status] ?? 0
    },

    formatDeadline(timeStr?: string) {
        if (!timeStr) return '尽快送达'

        const date = new Date(timeStr)
        const diff = date.getTime() - Date.now()
        if (diff < 0) {
            return '已超时'
        }

        const hours = date.getHours().toString().padStart(2, '0')
        const minutes = date.getMinutes().toString().padStart(2, '0')
        if (diff < 60 * 60 * 1000) {
            return `剩 ${Math.max(1, Math.floor(diff / 60000))} 分钟 (${hours}:${minutes})`
        }

        return `${hours}:${minutes} 前`
    },

    /**
     * 更新配送状态按钮点击
     */
    async onUpdateStatus() {
        if (!this.data.delivery) return
        const { id, status } = this.data.delivery
        
        let nextAction = ''
        let actionMethod: DeliveryAction | null = null

        if (status === 'assigned') {
            nextAction = '到达商家'
            actionMethod = DeliveryService.startPickup
        } else if (status === 'picking') {
            nextAction = '确认取餐'
            actionMethod = DeliveryService.confirmPickup
        } else if (status === 'picked') {
            nextAction = '开始配送'
            actionMethod = DeliveryService.startDelivery
        } else if (status === 'delivering') {
            nextAction = '确认送达'
            actionMethod = DeliveryService.confirmDelivery
        }

        if (!actionMethod) return
        const method = actionMethod
        const locationSourceMap: Record<NonNullable<Delivery['status']>, string> = {
            pending: 'rider_task_detail_pending',
            assigned: 'rider_task_detail_start_pickup',
            picking: 'rider_task_detail_confirm_pickup',
            picked: 'rider_task_detail_start_delivery',
            delivering: 'rider_task_detail_confirm_delivery',
            delivered: 'rider_task_detail_delivered',
            completed: 'rider_task_detail_completed',
            cancelled: 'rider_task_detail_cancelled',
            exception: 'rider_task_detail_exception'
        }

        wx.showModal({
            title: '状态更新',
            content: `确定已完成 ${nextAction} 吗？`,
            success: async (res) => {
                if (res.confirm) {
                    wx.showLoading({ title: '同步中...' })
                    try {
                        await this.syncDeliveryLocation(id, locationSourceMap[status] || 'rider_task_detail_action')
                        const updated = await method(id)
                        const updatedView = this.decorateDelivery(updated)
                        this.setData({ 
                            delivery: updatedView,
                            currentStep: this.mapStatusToStep(updated.status)
                        })
                        wx.showToast({ title: '操作成功', icon: 'success' })
                        
                        if (updated.status === 'completed' || updated.status === 'delivered') {
                            setTimeout(() => wx.navigateBack(), 1500)
                        }
                    } catch (err: unknown) {
                        const userMessage = (err as UserMessageError).userMessage
                        const message = typeof userMessage === 'string' && userMessage ? userMessage : '操作失败'
                        wx.showToast({ title: message, icon: 'none' })
                    } finally {
                        wx.hideLoading()
                    }
                }
            }
        })
    },

    onCallPhone(e: WechatMiniprogram.TouchEvent) {
        const { phone } = e.currentTarget.dataset as { phone?: string }
        if (!phone) return
        wx.makePhoneCall({ phoneNumber: phone })
    },

    async onOpenLocation(e: WechatMiniprogram.TouchEvent) {
        const {
            latitude,
            longitude,
            name,
            address,
            label
        } = e.currentTarget.dataset as {
            latitude?: number
            longitude?: number
            name?: string
            address?: string
            label?: string
        }

        await locationService.openLocation({
            latitude,
            longitude,
            name,
            address,
            failMessage: `打开${label || '导航'}失败，请稍后重试`
        })
    },

    async syncDeliveryLocation(deliveryId: number, source: string) {
        try {
            await syncRiderDeliveryLocation(deliveryId, source)
        } catch (err: unknown) {
            throw normalizeLocationError(err)
        }
    },

    onReportException() {
      wx.navigateTo({
        url: `/pages/rider/exception/index?orderId=${this.data.orderId}`
      })
    },

    onCopyOrderNo() {
        wx.setClipboardData({
            data: String(this.data.delivery?.order_no || this.data.orderId),
            success: () => wx.showToast({ title: '单号已复制' })
        })
    },

    onRetry() {
        this.fetchTaskDetail()
    },

    onBack() {
        wx.navigateBack({ delta: 1 }).catch(() => {
            wx.redirectTo({ url: '/pages/rider/dashboard/index' })
        })
    }
})
