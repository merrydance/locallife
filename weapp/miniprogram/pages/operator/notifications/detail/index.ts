import {
  loadOperatorNotificationDetail,
  type OperatorNotificationView
} from '../../_services/operator-notification-center'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type NotificationDetailOptions = {
  id?: string
}

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    error: '',
    notification: null as OperatorNotificationView | null
  },

  onLoad(options: NotificationDetailOptions) {
    const id = Number(options.id || 0)
    if (!id) {
      this.setData({ loading: false, error: '提醒参数缺失，请返回重试。' })
      return
    }

    this.loadDetail(id)
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  async loadDetail(id: number) {
    this.setData({ loading: true, error: '' })
    try {
      this.setData({
        notification: await loadOperatorNotificationDetail(id),
        loading: false
      })
    } catch (error: unknown) {
      this.setData({
        loading: false,
        error: getErrorUserMessage(error, '加载提醒详情失败，请稍后重试')
      })
    }
  },

  onRetry() {
    const id = this.data.notification?.id
    if (id) {
      this.loadDetail(id)
      return
    }

    const pages = getCurrentPages()
    const current = pages[pages.length - 1]
    const currentId = Number(current?.options?.id || 0)
    if (currentId) {
      this.loadDetail(currentId)
    }
  },

  onOpenDispatchHall() {
    const regionId = Number(this.data.notification?.region_id || 0)
    if (!regionId) {
      return
    }

    wx.navigateTo({ url: `/pages/operator/dispatch-hall/index?region_id=${regionId}` })
  }
})