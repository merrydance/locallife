import RiderService from '../../../api/rider'
import { Delivery } from '../../../api/delivery'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    
    // 列表数据
    historyApps: [] as any[],
    activeOrders: [] as Delivery[],
    
    // 表单数据
    showForm: false,
    formData: {
      orderId: 0,
      type: 'customer_unreachable',
      description: ''
    },
    
    typeOptions: [
      { label: '联系不上顾客', value: 'customer_unreachable' },
      { label: '商户未出餐', value: 'merchant_not_ready' },
      { label: '天气恶劣', value: 'weather_issue' },
      { label: '道路封堵', value: 'road_blocked' },
      { label: '其他', value: 'other' }
    ]
  },

  onLoad(options: any) {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    
    if (options.orderId) {
      this.setData({ 
        'formData.orderId': Number(options.orderId),
        showForm: true
      })
    }
    
    this.fetchData()
  },

  async fetchData() {
    this.setData({ loading: true })
    try {
      // 1. 获取活跃订单供选择
      const activeOrders = await (require('../../../utils/request').request({
        url: '/v1/delivery/active',
        method: 'GET'
      }))
      
      // 2. 获取历史申诉/异常记录
      const history = await (require('../../../utils/request').request({
          url: '/v1/rider/appeals',
          method: 'GET'
      }))

      this.setData({ 
        activeOrders,
        historyApps: history.appeals || []
      })
    } catch (err) {
      logger.error('Fetch exception data failed', err)
    } finally {
      this.setData({ loading: false })
    }
  },

  onOpenForm() {
    this.setData({ showForm: true })
  },

  onCloseForm() {
    this.setData({ showForm: false })
  },

  onSelectOrder(e: any) {
    const { id } = e.currentTarget.dataset
    this.setData({ 'formData.orderId': id })
  },

  onTypeChange(e: any) {
    this.setData({ 'formData.type': e.detail.value })
  },

  onDescriptionChange(e: any) {
    this.setData({ 'formData.description': e.detail.value })
  },

  /**
   * 提交上报
   */
  async onSubmit() {
    const { formData } = this.data
    if (!formData.orderId) {
      wx.showToast({ title: '请选择相关订单', icon: 'none' })
      return
    }
    if (!formData.description) {
      wx.showToast({ title: '请填写异常描述', icon: 'none' })
      return
    }

    wx.showLoading({ title: '提交中...' })
    try {
      await RiderService.reportException(formData.orderId, {
        exception_type: formData.type,
        description: formData.description
      })
      
      wx.showToast({ title: '上报成功', icon: 'success' })
      this.setData({ showForm: false })
      this.fetchData() // 刷新列表
    } catch (err: any) {
      wx.showToast({ title: err.userMessage || '上报失败', icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  }
})
