import { operatorBasicManagementService } from '../../../../api/operator-basic-management'

Page({
  data: {
    title: '',
    description: '',
    level: 'low'
  },

  onTitleChange(e: any) {
    this.setData({ title: e.detail.value })
  },

  onDescChange(e: any) {
    this.setData({ description: e.detail.value })
  },

  onLevelChange(e: any) {
    this.setData({ level: e.detail.value })
  },

  async onSubmit() {
    if (!this.data.title || !this.data.description) {
      wx.showToast({ title: '请填写完整信息', icon: 'none' })
      return
    }

    try {
      wx.showLoading({ title: '提交中' })
      await operatorBasicManagementService.submitSafetyReport({
        title: this.data.title,
        description: this.data.description,
        level: this.data.level as any
      })
      wx.showToast({ title: '提交成功', icon: 'success' })
      setTimeout(() => wx.navigateBack(), 1500)
    } catch (error: any) {
      wx.showToast({ title: error.message || '提交失败', icon: 'error' })
    } finally {
      wx.hideLoading()
    }
  }
})
