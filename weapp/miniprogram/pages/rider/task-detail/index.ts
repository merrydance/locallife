import { pickupOrder, deliverOrder, getRiderDashboard, RiderOrderDTO } from '../../../api/rider'
import { logger } from '../../../utils/logger'
import { ErrorHandler } from '../../../utils/error-handler'
import { formatPriceNoSymbol } from '../../../utils/util'

Page({
  data: {
    taskId: '',
    task: null as any,
    navBarHeight: 88,
    loading: false
  },

  onLoad(options: any) {
    if (options.id) {
      this.setData({ taskId: options.id })
      this.loadTaskDetail()
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadTaskDetail() {
    this.setData({ loading: true })
    try {
      // Note: Missing GET /rider/orders/{id} API.
      // Trying to find in dashboard active tasks.
      const dashboard = await getRiderDashboard()
      const taskDTO = dashboard.active_tasks.find((t) => t.id === this.data.taskId)

      if (taskDTO) {
        this.setData({
          task: this.mapTask(taskDTO),
          loading: false
        })
      } else {
        wx.showToast({ title: '任务详情获取失败(API缺失)', icon: 'none' })
        this.setData({ loading: false })
      }
    } catch (error) {
      logger.error('Load failed', error, 'Task-detail')
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  mapTask(dto: RiderOrderDTO) {
    return {
      id: dto.id,
      order_no: dto.id.slice(-8).toUpperCase(),
      status: dto.status,
      income: dto.fee, // Cents
      incomeDisplay: formatPriceNoSymbol(dto.fee || 0),
      time_limit: dto.expect_deliver_time ? dto.expect_deliver_time.slice(11, 16) : '',
      merchant: {
        name: dto.merchant_name,
        address: dto.merchant_address,
        phone: '13800000000', // Missing in DTO
        lat: 0, // Missing in DTO
        lng: 0  // Missing in DTO
      },
      customer: {
        name: '顾客', // Missing in DTO
        address: dto.customer_address,
        phone: '13900000000', // Missing in DTO
        lat: 0, // Missing in DTO
        lng: 0  // Missing in DTO
      },
      items: [] // Missing in DTO
    }
  },

  onCall(e: WechatMiniprogram.CustomEvent) {
    const { phone } = e.currentTarget.dataset
    if (!phone || phone === '13800000000' || phone === '13900000000') {
      wx.showToast({ title: '暂无电话信息', icon: 'none' })
      return
    }
    wx.makePhoneCall({ phoneNumber: phone })
  },

  onUpdateStatus() {
    const { task } = this.data
    if (!task) return

    let actionPromise: Promise<void> | null = null
    let actionText = ''

    if (task.status === 'ACCEPTED' || task.status === 'CONFIRMED') { // Assuming CONFIRMED is 'To Pickup'
      actionText = '确认取货'
      actionPromise = pickupOrder(task.id)
    } else if (task.status === 'DELIVERING') {
      actionText = '确认送达'
      actionPromise = deliverOrder(task.id)
    }

    if (!actionPromise) return

    wx.showModal({
      title: '状态更新',
      content: `确认${actionText}?`,
      success: async (res) => {
        if (res.confirm) {
          try {
            await actionPromise
            wx.showToast({ title: '操作成功', icon: 'success' })
            this.loadTaskDetail()
          } catch (error) {
            wx.showToast({ title: '操作失败', icon: 'none' })
          }
        }
      }
    })
  },

  onReportIssue() {
    wx.navigateTo({ url: `/pages/rider/claims/index?taskId=${this.data.taskId}` })
  }
})
