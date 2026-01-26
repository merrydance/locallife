import DeliveryService, { Delivery } from '../../../api/delivery'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'

Page({
    data: {
        orderId: 0,
        delivery: null as Delivery | null,
        loading: false,
        navBarHeight: 88,
        
        // 状态映射
        statusSteps: [
            { title: '已接单', status: 'assigned' },
            { title: '取餐中', status: 'start_pickup' },
            { title: '配送中', status: 'delivering' },
            { title: '已送达', status: 'completed' }
        ],
        currentStep: 0
    },

    onLoad(options: any) {
        const { navBarHeight } = getStableBarHeights()
        this.setData({ 
            navBarHeight,
            orderId: Number(options.id)
        })
        this.fetchTaskDetail()
    },

    async fetchTaskDetail() {
        this.setData({ loading: true })
        try {
            const data = await DeliveryService.grabOrder(this.data.orderId) // Technically this gets the detail if already grabbed
            // Actually in our Go API, POST /v1/delivery/grab/:order_id is for grabbing.
            // GET /v1/delivery/order/:order_id is for fetching.
            const delivery = await (require('../../../utils/request').request({
                url: `/v1/delivery/order/${this.data.orderId}`,
                method: 'GET'
            }))
            
            this.setData({ 
                delivery,
                currentStep: this.mapStatusToStep(delivery.status)
            })
        } catch (err) {
            logger.error('Fetch task detail failed', err)
            wx.showToast({ title: '加载失败', icon: 'none' })
        } finally {
            this.setData({ loading: false })
        }
    },

    mapStatusToStep(status: string): number {
        const statusMap: Record<string, number> = {
            'assigned': 0,
            'start_pickup': 1,
            'picked_up': 2,
            'delivering': 2,
            'completed': 3
        }
        return statusMap[status] ?? 0
    },

    /**
     * 更新配送状态按钮点击
     */
    async onUpdateStatus() {
        if (!this.data.delivery) return
        const { id, status } = this.data.delivery
        
        let nextAction = ''
        let actionMethod: any = null

        if (status === 'assigned') {
            nextAction = '开始取餐'
            actionMethod = DeliveryService.startPickup
        } else if (status === 'start_pickup') {
            nextAction = '确认已取餐'
            actionMethod = DeliveryService.confirmPickup
        } else if (status === 'picked_up') {
            nextAction = '开始配送'
            actionMethod = DeliveryService.startDelivery
        } else if (status === 'delivering') {
            nextAction = '确认已送达'
            actionMethod = DeliveryService.confirmDelivery
        }

        if (!actionMethod) return

        wx.showModal({
            title: '状态变更',
            content: `确定要 ${nextAction} 吗？`,
            success: async (res) => {
                if (res.confirm) {
                    wx.showLoading({ title: '处理中...' })
                    try {
                        const updated = await actionMethod(id)
                        this.setData({ 
                            delivery: updated,
                            currentStep: this.mapStatusToStep(updated.status)
                        })
                        wx.showToast({ title: '操作成功', icon: 'success' })
                        
                        // 如果已完成，延迟返回或刷新
                        if (updated.status === 'completed') {
                            setTimeout(() => wx.navigateBack(), 1500)
                        }
                    } catch (err: any) {
                        wx.showToast({ title: err.userMessage || '操作失败', icon: 'none' })
                    } finally {
                        wx.hideLoading()
                    }
                }
            }
        })
    },

    onCallPhone(e: any) {
        const { phone } = e.currentTarget.dataset
        if (!phone) return
        wx.makePhoneCall({ phoneNumber: phone })
    },

    onReportException() {
      wx.navigateTo({
        url: `/pages/rider/exception/index?orderId=${this.data.orderId}`
      })
    },

    onCopyOrderNo() {
        wx.setClipboardData({
            data: String(this.data.orderId),
            success: () => wx.showToast({ title: '已复制' })
        })
    }
})
