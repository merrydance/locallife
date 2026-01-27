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
            { title: '取餐中', status: 'picking' },
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
        if (!this.data.orderId) return
        
        this.setData({ loading: true })
        try {
            // 使用正确的获取详情接口，而不是抢单接口
            const delivery = await DeliveryService.getDeliveryByOrder(this.data.orderId)
            
            this.setData({ 
                delivery,
                currentStep: this.mapStatusToStep(delivery.status)
            })
        } catch (err: any) {
            logger.error('Fetch task detail failed', err)
            // 404 错误在 request.ts 中已经有全局提示，这里主要处理数据缺失逻辑
            this.setData({ delivery: null })
        } finally {
            this.setData({ loading: false })
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

    /**
     * 更新配送状态按钮点击
     */
    async onUpdateStatus() {
        if (!this.data.delivery) return
        const { id, status } = this.data.delivery
        
        let nextAction = ''
        let actionMethod: any = null

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

        wx.showModal({
            title: '状态更新',
            content: `确定已完成 ${nextAction} 吗？`,
            success: async (res) => {
                if (res.confirm) {
                    wx.showLoading({ title: '同步中...' })
                    try {
                        const updated = await actionMethod(id)
                        this.setData({ 
                            delivery: updated,
                            currentStep: this.mapStatusToStep(updated.status)
                        })
                        wx.showToast({ title: '操作成功', icon: 'success' })
                        
                        if (updated.status === 'completed' || updated.status === 'delivered') {
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
            success: () => wx.showToast({ title: '单号已复制' })
        })
    },

    onBack() {
        wx.navigateBack({ delta: 1 }).catch(() => {
            wx.redirectTo({ url: '/pages/rider/dashboard/index' })
        })
    }
})
