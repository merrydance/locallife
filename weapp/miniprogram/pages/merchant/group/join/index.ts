import { applyToJoinGroup, searchGroups } from '../../_main_shared/api/group-application'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'

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
const CONFIG_PAGE_ROUTE = 'pages/merchant/config/index'

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    searchErrorMessage: '',
    keyword: '',
    groups: [] as GroupItem[],
    searched: false,
    loading: false,
    dialogVisible: false,
    selectedGroupId: 0,
    selectedGroupName: '',
    applyReason: ''
  },

  async onLoad() {
    const accessResult = await ensureMerchantConsoleAccess()
    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : ''
    })
  },

  onRetryAccess() {
    this.setData({ accessReady: false, accessDenied: false, accessErrorMessage: '' })
    this.onLoad()
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onSearchChange(e: InputEvent) {
    this.setData({ keyword: e.detail.value })
  },

  async onSearchSubmit() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    if (!this.data.keyword.trim()) return
    this.setData({ loading: true, searched: false, groups: [], searchErrorMessage: '' })
    try {
      const res = await searchGroups(this.data.keyword)
      this.setData({
        groups: res || [],
        searched: true,
        searchErrorMessage: '',
        loading: false
      })
    } catch (e) {
      this.setData({
        loading: false,
        searched: true,
        searchErrorMessage: getErrorMessage(e, '搜索失败，请稍后重试')
      })
      logger.error('Search groups failed', e)
    }
  },

  onApply(e: ApplyEvent) {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return

    const { id, name } = e.currentTarget.dataset
    if (typeof id !== 'number') {
      wx.showToast({ title: '集团信息异常', icon: 'none' })
      return
    }
    this.setData({
      selectedGroupId: id,
      selectedGroupName: typeof name === 'string' ? name : '',
      dialogVisible: true,
      applyReason: ''
    })
  },

  onReasonChange(e: InputEvent) {
    this.setData({ applyReason: e.detail.value })
  },

  closeDialog() {
    this.setData({ dialogVisible: false })
  },

  async confirmApply() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return

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
          const pages = getCurrentPages()
          const previousPage = pages[pages.length - 2]

          if (previousPage?.route === CONFIG_PAGE_ROUTE) {
            wx.navigateBack()
            return
          }

          wx.redirectTo({ url: '/pages/merchant/config/index' })
        }
      })
    } catch (e: unknown) {
      wx.hideLoading()
      wx.showToast({ title: getErrorMessage(e, '申请失败'), icon: 'none' })
    }
  }
})