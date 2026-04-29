import type { FoodSafetyIncidentType, ReportFoodSafetyResponse } from '../../../../api/food-safety'
import { loadCustomerFoodSafetyOrderView, submitCustomerFoodSafetyReport } from '../../../../services/customer-food-safety-report'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface IncidentTypeOption {
  label: string
  value: FoodSafetyIncidentType
}

interface SeverityOption {
  label: string
  value: number
}

const INCIDENT_TYPE_OPTIONS: IncidentTypeOption[] = [
  { label: '异物', value: 'foreign-object' },
  { label: '污染变质', value: 'contamination' },
  { label: '过期食品', value: 'expired' }
]

const SEVERITY_OPTIONS: SeverityOption[] = [
  { label: '轻微', value: 1 },
  { label: '一般', value: 2 },
  { label: '明显不适', value: 3 },
  { label: '多人不适', value: 4 },
  { label: '严重风险', value: 5 }
]

function parsePositiveID(value?: string): number {
  const id = Number(value || 0)
  return Number.isFinite(id) && id > 0 ? id : 0
}

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    loadError: '',
    orderId: 0,
    orderNo: '',
    merchantId: 0,
    merchantName: '',
    incidentTypeOptions: INCIDENT_TYPE_OPTIONS,
    severityOptions: SEVERITY_OPTIONS,
    incidentType: 'contamination' as FoodSafetyIncidentType,
    incidentTypeLabel: '污染变质',
    severityLevel: 3,
    severityLabel: '明显不适',
    description: '',
    canSubmit: false,
    submitting: false,
    submitError: '',
    submitResult: null as ReportFoodSafetyResponse | null,
    resultTitle: '',
    resultSummary: ''
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    if (e.detail.navBarHeight !== null && e.detail.navBarHeight !== undefined) {
      this.setData({ navBarHeight: e.detail.navBarHeight })
    }
  },

  onLoad(options: { orderId?: string, merchantId?: string, merchantName?: string }) {
    const orderId = parsePositiveID(options.orderId)
    const merchantId = parsePositiveID(options.merchantId)
    const merchantName = options.merchantName ? decodeURIComponent(options.merchantName) : ''

    this.setData({ orderId, merchantId, merchantName })
    if (orderId > 0) {
      void this.loadOrder(orderId)
      return
    }

    this.setData({ loading: false, loadError: '请选择已履约订单后再提交食安反馈' })
  },

  async loadOrder(orderId: number) {
    this.setData({ loading: true, loadError: '' })
    try {
      const order = await loadCustomerFoodSafetyOrderView(orderId)
      if (!order.reportable) {
        this.setData({ loading: false, loadError: '该订单暂不支持提交食安反馈' })
        return
      }

      this.setData({
        orderId: order.orderId,
        orderNo: order.orderNo,
        merchantId: order.merchantId,
        merchantName: order.merchantName,
        loading: false
      })
      this.validateForm()
    } catch (err) {
      logger.error('[FoodSafetyReport] load order failed', err)
      this.setData({
        loading: false,
        loadError: getErrorUserMessage(err, '订单信息加载失败，请稍后重试')
      })
    }
  },

  onRetryLoad() {
    if (this.data.orderId > 0) {
      void this.loadOrder(this.data.orderId)
      return
    }
    this.onSelectOrder()
  },

  onSelectOrder() {
    wx.navigateTo({
      url: '/pages/orders/list/index?status=completed&selectMode=1',
      events: {
        onOrderSelected: (order: { id: number }) => {
          this.setData({ orderId: order.id, submitResult: null })
          void this.loadOrder(order.id)
        }
      }
    })
  },

  onIncidentTypeChange(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    const incidentType = e.detail.value as FoodSafetyIncidentType
    const option = INCIDENT_TYPE_OPTIONS.find((item) => item.value === incidentType) || INCIDENT_TYPE_OPTIONS[1]
    this.setData({ incidentType: option.value, incidentTypeLabel: option.label })
    this.validateForm()
  },

  onSeverityChange(e: WechatMiniprogram.CustomEvent<{ value?: string | number }>) {
    const severityLevel = Number(e.detail.value || 0)
    const option = SEVERITY_OPTIONS.find((item) => item.value === severityLevel) || SEVERITY_OPTIONS[2]
    this.setData({ severityLevel: option.value, severityLabel: option.label })
    this.validateForm()
  },

  onDescriptionInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ description: e.detail.value, submitError: '' })
    this.validateForm()
  },

  validateForm() {
    this.setData({
      canSubmit: this.data.orderId > 0 && this.data.merchantId > 0 && this.data.description.trim().length >= 10
    })
  },

  async onSubmit() {
    if (this.data.submitting || !this.data.canSubmit) return

    this.setData({ submitting: true, submitError: '' })
    try {
      const result = await submitCustomerFoodSafetyReport({
        merchantId: this.data.merchantId,
        orderId: this.data.orderId,
        incidentType: this.data.incidentType,
        description: this.data.description.trim(),
        severityLevel: this.data.severityLevel
      })

      this.setData({
        submitResult: result,
        resultTitle: result.merchant_suspended ? '已提交并触发风险处置' : '已提交食安反馈',
        resultSummary: result.message || '平台已记录反馈并进入食安处理流程',
        submitting: false
      })
    } catch (err) {
      const message = getErrorUserMessage(err, '提交失败，请稍后重试')
      logger.error('[FoodSafetyReport] submit failed', err)
      this.setData({ submitting: false, submitError: message })
    }
  },

  onBackToCenter() {
    wx.redirectTo({ url: '/pages/user_center/service_center/index' })
  }
})