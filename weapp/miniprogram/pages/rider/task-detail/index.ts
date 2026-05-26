import DeliveryService, { Delivery } from '../../../api/delivery'
import { logger } from '../../../utils/logger'
import { locationService } from '../../../utils/location'
import { normalizeLocationError, syncRiderDeliveryLocation } from '../../../utils/rider-location'
import { riderLiveLocationSession } from '../../../utils/rider-live-location'
import {
    buildRiderDeliveryDeadlineView,
    getRiderDeliveryActionState,
    getRiderDeliveryStep,
    isExpectedDeliveryStatusReached,
    isRiderDeliveryTrackedStatus,
    RiderDeliveryActionKey
} from '../../../utils/rider-delivery-view'
import { buildRiderDeliveryIncomeView, RiderDeliveryIncomeView } from '../../../utils/rider-delivery-income-view'
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
    can_update_status: boolean
    action_label: string
    income_view: RiderDeliveryIncomeView
}

const DELIVERY_ACTION_METHODS: Record<Exclude<RiderDeliveryActionKey, ''>, DeliveryAction> = {
    start_pickup: DeliveryService.startPickup,
    confirm_pickup: DeliveryService.confirmPickup,
    start_delivery: DeliveryService.startDelivery,
    confirm_delivery: DeliveryService.confirmDelivery
}

function getUserMessage(err: unknown, fallback: string) {
    const userMessage = (err as UserMessageError).userMessage
    return typeof userMessage === 'string' && userMessage ? userMessage : fallback
}

Page({
    data: {
        orderId: 0,
        delivery: null as DeliveryView | null,
        loading: false,
        actionLoading: false,
        errorMessage: '',
        syncWarningMessage: '',
        navBarHeight: 88,

        // 状态映射
        statusSteps: [
            { title: '已接单', status: 'assigned' },
            { title: '取餐中', status: 'picking' },
            { title: '代取中', status: 'delivering' },
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

    onShow() {
        if (this.data.delivery && isRiderDeliveryTrackedStatus(this.data.delivery.status)) {
            void riderLiveLocationSession.setActiveDelivery(this.data.delivery.id, 'rider_task_detail_show')
        }
        if (this.data.delivery && !this.data.loading && !this.data.actionLoading) {
            void this.fetchTaskDetail(true)
        }
    },

    async fetchTaskDetail(silent = false) {
        if (!this.data.orderId) return
        
        this.setData(silent ? { syncWarningMessage: '' } : { loading: true, syncWarningMessage: '' })
        try {
            const delivery = await DeliveryService.getDeliveryByOrder(this.data.orderId)
            const deliveryView = this.decorateDelivery(delivery)
            
            this.setData({ 
                delivery: deliveryView,
                currentStep: this.mapStatusToStep(delivery.status),
                errorMessage: '',
                syncWarningMessage: ''
            })

            if (isRiderDeliveryTrackedStatus(deliveryView.status)) {
                await riderLiveLocationSession.setActiveDelivery(deliveryView.id, 'rider_task_detail_fetch')
            }
        } catch (err: unknown) {
            logger.error('Fetch task detail failed', err)
            const message = getUserMessage(err, '任务详情加载失败，请稍后重试')
            if (silent && this.data.delivery) {
                this.setData({ syncWarningMessage: `${message}，当前已保留上次任务状态` })
            } else {
                this.setData({ delivery: null, errorMessage: message })
            }
        } finally {
            if (!silent) {
                this.setData({ loading: false })
            }
        }
    },

    async reconcileDeliveryState(expectedStatus: Delivery['status']) {
        try {
            const latest = await DeliveryService.getDeliveryByOrder(this.data.orderId)
            if (!isExpectedDeliveryStatusReached(latest.status, expectedStatus)) {
                return false
            }

            const deliveryView = this.decorateDelivery(latest)
            this.setData({
                delivery: deliveryView,
                currentStep: this.mapStatusToStep(latest.status),
                errorMessage: ''
            })
            return true
        } catch (err: unknown) {
            logger.warn('Reconcile task detail state failed', { expectedStatus, err }, 'RiderTaskDetail')
            return false
        }
    },

    decorateDelivery(delivery: Delivery): DeliveryView {
        const actionState = getRiderDeliveryActionState(delivery.status)
        const deadlineView = buildRiderDeliveryDeadlineView(delivery)

        return {
            ...delivery,
            deadline_desc: deadlineView.text,
            can_update_status: actionState.canUpdate,
            action_label: actionState.label,
            income_view: buildRiderDeliveryIncomeView(delivery)
        }
    },

    mapStatusToStep(status: string): number {
        return getRiderDeliveryStep(status)
    },

    /**
     * 更新代取状态按钮点击
     */
    async onUpdateStatus() {
        if (this.data.actionLoading) return
        if (!this.data.delivery) return
        const { id, status } = this.data.delivery
        const actionState = getRiderDeliveryActionState(status)

        if (!actionState.canUpdate || !actionState.expectedStatus || !actionState.actionKey) return
        const method = DELIVERY_ACTION_METHODS[actionState.actionKey]
        const nextExpectedStatus = actionState.expectedStatus

        this.setData({ actionLoading: true })
        wx.showModal({
            title: '状态更新',
            content: `确定已完成 ${actionState.label.replace('我已', '')} 吗？`,
            success: async (res) => {
                if (!res.confirm) {
                    this.setData({ actionLoading: false })
                    return
                }

                try {
                    wx.showLoading({ title: '同步中...' })
                    await this.syncDeliveryLocation(id, actionState.locationSource)
                    const updated = await method(id)
                    const updatedView = this.decorateDelivery(updated)
                    this.setData({
                        delivery: updatedView,
                        currentStep: this.mapStatusToStep(updated.status),
                        syncWarningMessage: ''
                    })

                    if (isExpectedDeliveryStatusReached(updated.status, 'delivered')) {
                        wx.navigateBack()
                        return
                    }
                } catch (err: unknown) {
                    const reconciled = await this.reconcileDeliveryState(nextExpectedStatus)
                    if (reconciled) {
                        const latestStatus = this.data.delivery?.status
                        if (latestStatus && isExpectedDeliveryStatusReached(latestStatus, 'delivered')) {
                            wx.navigateBack()
                            return
                        }
                        return
                    }

                    const message = getUserMessage(err, '操作失败')
                    wx.showToast({ title: message, icon: 'none' })
                } finally {
                    wx.hideLoading()
                    this.setData({ actionLoading: false })
                }
            },
            fail: () => {
                this.setData({ actionLoading: false })
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

    onCopyOrderNo() {
        wx.setClipboardData({
            data: String(this.data.delivery?.order_no || this.data.orderId),
            success: () => wx.showToast({ title: '单号已复制' })
        })
    },

    onRetry() {
        this.fetchTaskDetail(!!this.data.delivery)
    },

    onBack() {
        wx.navigateBack({ delta: 1 }).catch(() => {
            wx.redirectTo({ url: '/pages/rider/dashboard/index' })
        })
    }
})
