import { searchGroups, applyToJoinGroup } from '../../../../api/group-application'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type NavHeightEvent = {
  detail: {
    navBarHeight: number
  }
}

type InputEvent = {
  detail: {
    value: string
  }
}

type ApplyEvent = {
  currentTarget: {
    dataset: {
      id?: number
      name?: string
    }
  }
}

type GroupItem = Record<string, unknown>

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    keyword: '',
    groups: [] as GroupItem[],
    searched: false,
    loading: false,
    dialogVisible: false,
    selectedGroupId: 0,
    selectedGroupName: '',
    applyReason: ''
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onSearchChange(e: InputEvent) {
    this.setData({ keyword: e.detail.value })
  },

  async onSearchSubmit() {
    if (!this.data.keyword.trim()) return
    this.setData({ loading: true, searched: false, groups: [] })
    try {
      const res = await searchGroups(this.data.keyword)
      this.setData({ 
        groups: res || [],
        searched: true,
        loading: false
      })
    } catch (e) {
      this.setData({ loading: false })
      logger.error('Search groups failed', e)
    }
  },

  onApply(e: ApplyEvent) {
    const { id, name } = e.currentTarget.dataset
    if (typeof id !== 'number') {
      wx.showToast({ title: '集团信息异常', icon: 'none' })
      return
    }
    this.setData({
      selectedGroupId: id,
      selectedGroupName: typeof name === 'string' ? name : '',
      dialogVisible: true
    })
  },

  onReasonChange(e: InputEvent) {
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
    } catch (e: unknown) {
      wx.hideLoading()
      wx.showToast({ title: getErrorMessage(e, '申请失败'), icon: 'none' })
    }
  }
})
