interface CreditHistoryItem {
  id: number
  type: 'REWARD' | 'PENALTY'
  amount: number
  reason: string
  related_id?: string
  created_at: string
}

Page({
  data: {
    score: 0,
    level: '普通会员',
    levelDesc: '信用良好',
    history: [] as CreditHistoryItem[],
    privileges: [] as string[],
    loading: false,
    navBarHeight: 88
  },

  onLoad() {
    this.loadCreditInfo()
  },

  onShow() {
    this.loadCreditInfo()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadCreditInfo() {
    this.setData({
      score: 0,
      level: '已下线',
      levelDesc: '信用分功能已下线',
      privileges: [],
      history: [],
      loading: false
    })
    wx.showToast({ title: '信用分功能已下线', icon: 'none' })
  },

  calculateLevelInfo(score: number): { level: string; levelDesc: string; privileges: string[] } {
    if (score >= 800) {
      return {
        level: '钻石会员',
        levelDesc: '信用极佳',
        privileges: ['优先派单', '免押金', '极速退款', '专属客服', '生日礼包']
      }
    } else if (score >= 700) {
      return {
        level: '黄金会员',
        levelDesc: '信用极好',
        privileges: ['优先派单', '免押金', '极速退款']
      }
    } else if (score >= 600) {
      return {
        level: '白银会员',
        levelDesc: '信用良好',
        privileges: ['优先派单', '快速退款']
      }
    } else if (score >= 500) {
      return {
        level: '普通会员',
        levelDesc: '信用一般',
        privileges: ['正常服务']
      }
    } else {
      return {
        level: '受限会员',
        levelDesc: '信用较低',
        privileges: ['部分功能受限']
      }
    }
  }
})
