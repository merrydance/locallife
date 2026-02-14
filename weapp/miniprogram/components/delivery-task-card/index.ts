import { DeliveryAdapter } from '../../api/rider-delivery'

type DeliveryTask = {
    order_id?: number
    delivery_id?: number
}

Component({
    properties: {
        task: {
            type: Object,
            value: undefined
        },
        type: {
            type: String,
            value: 'active' // available, active, history
        }
    },

    methods: {
        onGrab() {
            const task = this.properties.task as unknown as DeliveryTask | undefined
            if (!task?.order_id) return
            this.triggerEvent('grab', { id: task.order_id })
        },

        onPickup() {
            const task = this.properties.task as unknown as DeliveryTask | undefined
            if (!task?.delivery_id) return
            this.triggerEvent('pickup', { id: task.delivery_id })
        },

        onConfirmPickup() {
            const task = this.properties.task as unknown as DeliveryTask | undefined
            if (!task?.delivery_id) return
            this.triggerEvent('confirmPickup', { id: task.delivery_id })
        },

        onDeliver() {
            const task = this.properties.task as unknown as DeliveryTask | undefined
            if (!task?.delivery_id) return
            this.triggerEvent('deliver', { id: task.delivery_id })
        },

        onConfirmDeliver() {
            const task = this.properties.task as unknown as DeliveryTask | undefined
            if (!task?.delivery_id) return
            this.triggerEvent('confirmDeliver', { id: task.delivery_id })
        },

        onException() {
            const task = this.properties.task as unknown as DeliveryTask | undefined
            if (!task?.delivery_id) return
            this.triggerEvent('exception', { id: task.delivery_id })
        },

        onCall(e: WechatMiniprogram.TouchEvent) {
            const phone = e.currentTarget.dataset.phone
            if (phone) {
                wx.makePhoneCall({ phoneNumber: phone })
            }
        },

        // Formatters exposed to WXML
        formatAmount(amount: number) {
            return DeliveryAdapter.formatAmount(amount)
        },

        formatDistance(distance: number) {
            return DeliveryAdapter.formatDistance(distance)
        },

        formatDeliveryStatus(status: string) {
            return DeliveryAdapter.formatDeliveryStatus(status)
        },

        formatTime(dateStr: string) {
            if (!dateStr) return ''
            const date = new Date(dateStr)
            const hours = ('0' + date.getHours()).slice(-2)
            const minutes = ('0' + date.getMinutes()).slice(-2)
            return `${hours}:${minutes}`
        },

        calculateEstimatedArrival(createdAt: string, estimatedTime: number) {
            return DeliveryAdapter.calculateEstimatedArrival(createdAt, estimatedTime)
        }
    }
})
