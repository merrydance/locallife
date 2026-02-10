import RiderService from '../../../api/rider'
import { Delivery } from '../../../api/delivery'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import dayjs from 'dayjs'

interface ExceptionAudit {
  id: number;
  order_id: number;
  exception_type: string;
  description: string;
  status: 'pending' | 'resolved' | 'dismissed';
  created_at: string;
  resolution?: string;
  type_label?: string;
  status_label?: string;
}

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    submitting: false,
    activeTab: 'report', // report | history
    
    // 列表数据
    historyApps: [] as ExceptionAudit[],
    activeOrders: [] as Delivery[],
    
    // 表单数据
    formData: {
      orderId: 0,
      type: 'customer_unreachable',
      description: ''
    },
    fileList: [] as any[],
    
    typeOptions: [
      { label: '无法联系顾客', value: 'customer_unreachable', icon: 'mobile-off' },
      { label: '商户未出餐', value: 'merchant_not_ready', icon: 'hourglass-low' },
      { label: '商户已闭店', value: 'merchant_closed', icon: 'store' },
      { label: '车辆故障', value: 'vehicle_breakdown', icon: 'tools' },
      { label: '天气恶劣', value: 'bad_weather', icon: 'cloud-lightning' },
      { label: '其他原因', value: 'other', icon: 'more' }
    ]
  },

  onLoad(options: any) {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    
    if (options.orderId) {
      this.setData({ 
        'formData.orderId': Number(options.orderId),
        activeTab: 'report'
      })
    } else if (options.tab) {
      this.setData({ activeTab: options.tab })
    }
    
    this.fetchData()
  },

  onRefresh() {
    this.fetchData()
  },

  switchTab(e: any) {
    const { tab } = e.currentTarget.dataset
    if (tab === this.data.activeTab) return
    this.setData({ activeTab: tab })
  },

  async fetchData() {
    this.setData({ loading: true })
    try {
      const { request } = require('../../../utils/request')
      
      // 并发请求活跃订单和历史记录
      const [activeOrders, historyRes] = await Promise.all([
        request({ url: '/v1/delivery/active', method: 'GET' }) as Promise<Delivery[]>,
        request({ url: '/v1/rider/appeals', method: 'GET' }) as Promise<{ appeals: any[] }>
      ])
      
      // 格式化历史记录
      const history = (historyRes?.appeals || []).map((item: any) => {
        const typeOpt = this.data.typeOptions.find(o => o.value === item.exception_type)
        const statusMap = {
          'pending': '处理中',
          'resolved': '已通过',
          'dismissed': '已驳回'
        }
        return {
          ...item,
          type_label: typeOpt ? typeOpt.label : item.exception_type,
          status_label: statusMap[item.status as keyof typeof statusMap] || item.status,
          created_at: dayjs(item.created_at).format('YYYY-MM-DD HH:mm')
        }
      })

      this.setData({ 
        activeOrders: activeOrders || [],
        historyApps: history
      })
    } catch (err) {
      logger.error('Fetch exception data failed', err)
    } finally {
      this.setData({ loading: false })
    }
  },

  onSelectOrder(e: any) {
    const { id } = e.currentTarget.dataset
    this.setData({ 'formData.orderId': id })
  },

  onTypeSelect(e: any) {
    const { value } = e.currentTarget.dataset
    this.setData({ 'formData.type': value })
  },

  onDescriptionChange(e: any) {
    this.setData({ 'formData.description': e.detail.value })
  },

  onAddImage(e: any) {
    const { files } = e.detail
    this.setData({
      fileList: [...this.data.fileList, ...files]
    })
  },

  onRemoveImage(e: any) {
    const { index } = e.detail
    const { fileList } = this.data
    fileList.splice(index, 1)
    this.setData({ fileList })
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
      // 模拟上报接口调用
      await RiderService.reportException(formData.orderId, {
        exception_type: formData.type,
        description: formData.description
      })
      
      wx.showToast({ title: '上报成功', icon: 'success' })
      
      // 提交成功后，重置表单并切换到历史 Tab
      this.setData({ 
        'formData.description': '',
        fileList: [],
        submitting: false,
        activeTab: 'history'
      })
      
      // 延迟刷新列表，避免立刻刷新导致感知不到新数据（如果后端有延迟）
      setTimeout(() => this.fetchData(), 500)
    } catch (err: any) {
      logger.error('Report exception failed', err)
      wx.showToast({ 
        title: err.userMessage || '上报失败', 
        icon: 'none' 
      })
      this.setData({ submitting: false })
    } finally {
      wx.hideLoading()
    }
  }
})
