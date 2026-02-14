import { riderExceptionHandlingService } from '../../../api/rider-exception-handling'
import {
  appealManagementService,
  claimManagementService,
  ClaimRecoveryResponse,
  ClaimResponse,
  CreateAppealRequest
} from '../../../api/appeals-customer-service'

interface RiderClaimsOptions {
  taskId?: string
}

interface ClaimDisplay {
  id: number
  task_id?: string
  type: string
  description: string
  status: string
  created_at: string
  recoveryStatus?: string
  recoveryAmount?: number
  recoveryTarget?: string
}

Page({
  data: {
    taskId: '',
    claims: [] as ClaimDisplay[],
    form: {
      type: '',
      typeLabel: '',
      description: ''
    },
    types: [
      { label: '商家出餐慢', value: 'MERCHANT_DELAY' },
      { label: '顾客联系不上', value: 'CUSTOMER_UNREACHABLE' },
      { label: '餐品损坏', value: 'DAMAGED' },
      { label: '其他', value: 'OTHER' }
    ],
    showTypePicker: false,
    navBarHeight: 88,
    loading: false,
    submitting: false,
    recoveryPaying: {} as Record<number, boolean>
  },

  onLoad(options: RiderClaimsOptions) {
    if (options.taskId) {
      this.setData({ taskId: options.taskId })
    }
    this.loadClaims()
  },

  onShow() {
    this.loadClaims()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadClaims() {
    this.setData({ loading: true })
    try {
      const response = await riderExceptionHandlingService.getRiderClaims({
        page_id: 1,
        page_size: 20
      })

      const claims: ClaimDisplay[] = await Promise.all(
        response.claims.map(async (c: ClaimResponse) => {
          let recovery: ClaimRecoveryResponse | null = null
          try {
            recovery = await claimManagementService.getRiderClaimRecovery(c.id)
          } catch (error) {
            recovery = null
          }

          return {
            id: c.id,
            task_id: c.order_id?.toString(),
            type: c.claim_type,
            description: c.description,
            status: c.status,
            created_at: c.created_at,
            recoveryStatus: recovery?.status,
            recoveryAmount: recovery?.recovery_amount,
            recoveryTarget: recovery?.recovery_target
          }
        })
      )

      this.setData({
        claims,
        loading: false
      })
    } catch (error) {
      console.error('加载申诉列表失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false, claims: [] })
    }
  },

  onTypeClick() {
    this.setData({ showTypePicker: true })
  },

  onTypeChange(e: WechatMiniprogram.CustomEvent) {
    const { value } = e.detail
    const selected = this.data.types.find((t) => t.value === value[0])
    this.setData({
      'form.type': value[0],
      'form.typeLabel': selected?.label || '',
      showTypePicker: false
    })
  },

  onTypeCancel() {
    this.setData({ showTypePicker: false })
  },

  onDescChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ 'form.description': e.detail.value })
  },

  async onPayRecovery(e: WechatMiniprogram.CustomEvent) {
    const claimId = Number(e.currentTarget.dataset.claimId)
    if (!claimId) {
      return
    }

    const payingMap = { ...this.data.recoveryPaying, [claimId]: true }
    this.setData({ recoveryPaying: payingMap })
    try {
      await claimManagementService.payRiderClaimRecovery(claimId)
      wx.showToast({ title: '追偿已支付', icon: 'success' })
      this.loadClaims()
    } catch (error) {
      console.error('支付追偿失败:', error)
      wx.showToast({ title: '支付失败', icon: 'error' })
    } finally {
      const nextMap = { ...this.data.recoveryPaying, [claimId]: false }
      this.setData({ recoveryPaying: nextMap })
    }
  },

  async onSubmit() {
    const { form, taskId } = this.data
    if (!form.type || !form.description) {
      wx.showToast({ title: '请填写完整信息', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    try {
      // 创建申诉
      const appealData: CreateAppealRequest = {
        claim_id: taskId ? Number(taskId) : 0,
        reason: `[${form.type}] ${form.description}`
      }

      await appealManagementService.createRiderAppeal(appealData)

      wx.showToast({ title: '提交成功', icon: 'success' })
      this.setData({
        form: { type: '', typeLabel: '', description: '' },
        submitting: false
      })
      this.loadClaims()
    } catch (error) {
      console.error('提交申诉失败:', error)
      wx.showToast({ title: '提交失败', icon: 'error' })
      this.setData({ submitting: false })
    }
  }
})
