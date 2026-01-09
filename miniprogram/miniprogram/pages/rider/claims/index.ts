import { riderExceptionHandlingService } from '../../../api/rider-exception-handling'
import { appealManagementService, CreateAppealRequest, ClaimResponse } from '../../../api/appeals-customer-service'

interface ClaimDisplay {
  id: number
  task_id?: string
  type: string
  description: string
  status: string
  created_at: string
}

Page({
  data: {
    taskId: '',
    claims: [] as ClaimDisplay[],
    form: {
      type: '',
      typeLabel: '',
      description: '',
      images: [] as string[]
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
    submitting: false
  },

  onLoad(options: any) {
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

      const claims: ClaimDisplay[] = response.claims.map((c: ClaimResponse) => ({
        id: c.id,
        task_id: c.order_id?.toString(),
        type: c.claim_type,
        description: c.description,
        status: c.status,
        created_at: c.created_at
      }))

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

  onAddImage() {
    wx.chooseMedia({
      count: 1,
      mediaType: ['image'],
      success: (res) => {
        const { images } = this.data.form
        this.setData({
          'form.images': [...images, res.tempFiles[0].tempFilePath]
        })
      }
    })
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
        evidence_urls: form.images,
        reason: `[${form.type}] ${form.description}`
      }

      await appealManagementService.createRiderAppeal(appealData)

      wx.showToast({ title: '提交成功', icon: 'success' })
      this.setData({
        form: { type: '', typeLabel: '', description: '', images: [] },
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
