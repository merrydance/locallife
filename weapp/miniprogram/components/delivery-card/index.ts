import { buildDeliveryCardView } from '../../utils/delivery-card-view'

type DeliveryTask = {
  id?: number
  status?: string | number
  fee?: number
  merchant_phone?: string
  customer_phone?: string
}

Component({
  properties: {
    task: {
      type: Object,
      value: {}
    }
  },

  data: {
    taskView: buildDeliveryCardView()
  },

  observers: {
    task(task: DeliveryTask) {
      this.setData({ taskView: buildDeliveryCardView(task) })
    }
  },

  methods: {
    onAction() {
      const task = this.properties.task as DeliveryTask
      if (!task || !task.id) return
      const nextAction = this.data.taskView.nextAction

      if (nextAction) {
        this.triggerEvent('action', { id: task.id, action: nextAction })
      }
    },

    onCall(e: WechatMiniprogram.TouchEvent) {
      const { type } = e.currentTarget.dataset
      const task = this.properties.task as DeliveryTask
      // Use task phone if available, else fallback (per ISSUES.md)
      const phoneNumber = type === 'shop' ? (task.merchant_phone || '13800138000') : (task.customer_phone || '13900139000')
      wx.makePhoneCall({ phoneNumber })
    }
  }
})
