import { searchGroups, applyToJoinGroup } from '../../../../api/group-application'
import { logger } from '../../../../utils/logger'

Page({
  data: {
    navBarHeight: 88,
    keyword: '',
    groups: [] as any[],
    searched: false,
    dialogVisible: false,
    selectedGroupId: 0,
    selectedGroupName: '',
    applyReason: ''
  },

  onNavHeight(e: any) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onSearchChange(e: any) {
    this.setData({ keyword: e.detail.value })
  },

  async onSearchSubmit() {
    if (!this.data.keyword.trim()) return
    wx.showLoading({ title: '搜索中...' })
    try {
      const res = await searchGroups(this.data.keyword)
      this.setData({ 
        groups: res || [],
        searched: true
      })
      wx.hideLoading()
    } catch (e) {
      wx.hideLoading()
      logger.error('Search groups failed', e)
    }
  },

  onApply(e: any) {
    const { id, name } = e.currentTarget.dataset
    this.setData({
      selectedGroupId: id,
      selectedGroupName: name,
      dialogVisible: true
    })
  },

  onReasonChange(e: any) {
    this.setData({ applyReason: e.detail.value })
  },

  closeDialog() {
    this.setData({ dialogVisible: false })
  },

  async confirmApply() {
    wx.showLoading({ title: '提交申请...' })
    try {
      await applyToJoinGroup(this.data.selectedGroupId, {
        reason: this.data.applyReason
      })
      wx.hideLoading()
      this.closeDialog()
      wx.showModal({
        title: '申请已提交',
        content: `加入 ${this.data.selectedGroupName} 的申请已发送，请联系集团管理员审核。`,
        showCancel: false,
        success: () => {
          wx.navigateBack()
        }
      })
    } catch (e: any) {
      wx.hideLoading()
      wx.showToast({ title: e.message || '申请失败', icon: 'none' })
    }
  }
})
