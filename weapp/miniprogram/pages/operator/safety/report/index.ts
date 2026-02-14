import { operatorBasicManagementService } from '../../../../api/operator-basic-management'

type SafetyLevel = 'low' | 'medium' | 'high' | 'critical'

interface InputDetail {
  value: string
}

interface LevelChangeDetail {
  value: SafetyLevel
}

Page({
  data: {
    title: '',
    description: '',
    level: 'low' as SafetyLevel
  },

  onTitleChange(e: WechatMiniprogram.CustomEvent<InputDetail>) {
    this.setData({ title: e.detail.value })
  },

  onDescChange(e: WechatMiniprogram.CustomEvent<InputDetail>) {
    this.setData({ description: e.detail.value })
  },

  onLevelChange(e: WechatMiniprogram.CustomEvent<LevelChangeDetail>) {
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
        level: this.data.level
      })
      wx.showToast({ title: '提交成功', icon: 'success' })
      setTimeout(() => wx.navigateBack(), 1500)
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '提交失败'
      wx.showToast({ title: message, icon: 'error' })
    } finally {
      wx.hideLoading()
    }
  }
})
