import RiderService from '../../../api/rider'
import { Delivery } from '../../../api/delivery'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'

type RiderExceptionType = 'customer_unreachable' | 'merchant_not_ready' | 'weather_issue' | 'road_blocked' | 'other'

interface ExceptionOptions {
  orderId?: string
}

interface UserMessageError {
  userMessage?: string
}

interface LastSubmittedReport {
  orderId: number
  typeLabel: string
  description: string
}

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    submitting: false,
    errorMessage: '',
    
    activeOrders: [] as Delivery[],
    lastSubmittedReport: null as LastSubmittedReport | null,
    
    // 表单数据
    formData: {
      orderId: 0,
      type: 'customer_unreachable' as RiderExceptionType,
      description: ''
    },
    
    typeOptions: [
      { label: '无法联系顾客', value: 'customer_unreachable', icon: 'mobile-off' },
      { label: '商户未出餐', value: 'merchant_not_ready', icon: 'hourglass-low' },
      { label: '天气原因', value: 'weather_issue', icon: 'cloud-lightning' },
      { label: '道路阻塞', value: 'road_blocked', icon: 'map-information-2' },
      { label: '其他原因', value: 'other', icon: 'more' }
    ]
  },

  onLoad(options: ExceptionOptions) {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    
    if (options.orderId) {
      this.setData({ 'formData.orderId': Number(options.orderId) })
    }
    
    this.fetchData()
  },

  onRefresh() {
    this.fetchData()
  },

  async fetchData() {
    this.setData({ loading: true })
    try {
      const { request } = require('../../../utils/request')
      const activeOrders = await (request({ url: '/v1/delivery/active', method: 'GET' }) as Promise<Delivery[]>)

      this.setData({ 
        activeOrders: activeOrders || [],
        errorMessage: ''
      })
    } catch (err: unknown) {
      logger.error('Fetch exception data failed', err)
      const userMessage = (err as UserMessageError).userMessage
      const message = typeof userMessage === 'string' && userMessage ? userMessage : '异常页加载失败，请稍后重试'
      this.setData({
        activeOrders: [],
        errorMessage: message
      })
    } finally {
      this.setData({ loading: false })
    }
  },

  onSelectOrder(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    this.setData({ 'formData.orderId': id })
  },

  onTypeSelect(e: WechatMiniprogram.TouchEvent) {
    const { value } = e.currentTarget.dataset as { value?: RiderExceptionType }
    if (!value) return
    this.setData({ 'formData.type': value })
  },

  onDescriptionChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ 'formData.description': e.detail.value })
  },

  /**
   * 提交上报
   */
  async onSubmit() {
    const { formData, submitting } = this.data
    if (submitting) return
    
    if (!formData.orderId) {
      wx.showToast({ title: '请选择相关订单', icon: 'none' })
      return
    }
    if (!formData.description || formData.description.length < 5) {
      wx.showToast({ title: '请详细填写异常描述(至少5字)', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    wx.showLoading({ title: '提交中...', mask: true })
    
    try {
      await RiderService.reportException(formData.orderId, {
        exception_type: formData.type,
        description: formData.description
      })

      const selectedType = this.data.typeOptions.find((item) => item.value === formData.type)
      
      wx.showToast({ title: '上报成功', icon: 'success' })
      
      this.setData({ 
        'formData.description': '',
        submitting: false,
        lastSubmittedReport: {
          orderId: formData.orderId,
          typeLabel: selectedType?.label || formData.type,
          description: formData.description
        }
      })
      
      setTimeout(() => this.fetchData(), 500)
    } catch (err: unknown) {
      logger.error('Report exception failed', err)
      const userMessage = (err as UserMessageError).userMessage
      const message = typeof userMessage === 'string' && userMessage ? userMessage : '上报失败'
      wx.showToast({ 
        title: message,
        icon: 'none' 
      })
      this.setData({ submitting: false })
    } finally {
      wx.hideLoading()
    }
  },

  onGoToClaims() {
    wx.navigateTo({ url: '/pages/rider/claims/index' })
  }
})
