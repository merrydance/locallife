import { trustScoreSystemService, TrustScoreProfileResponse, TrustScoreHistoryResponse } from '../../../api/trust-score-system'

const app = getApp<IAppOption>()

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
    this.setData({ loading: true })
    try {
      const userId = app.globalData.userInfo?.id
      if (!userId) {
        wx.showToast({ title: '请先登录', icon: 'none' })
        this.setData({ loading: false })
        return
      }

      // 获取信任分档案和历史
      const [profile, historyResponse] = await Promise.all([
        trustScoreSystemService.getTrustScoreProfile('customer', userId),
        trustScoreSystemService.getTrustScoreHistory('customer', userId, 1, 20)
      ])

      // 转换历史记录格式
      const history: CreditHistoryItem[] = historyResponse.history.map(h => ({
        id: h.id,
        type: h.change_amount >= 0 ? 'REWARD' : 'PENALTY',
        amount: h.change_amount,
        reason: h.reason,
        related_id: h.related_order_id?.toString(),
        created_at: h.created_at
      }))

      // 根据分数计算等级和描述
      const { level, levelDesc, privileges } = this.calculateLevelInfo(profile.current_score)

      this.setData({
        score: profile.current_score,
        level,
        levelDesc,
        privileges,
        history,
        loading: false
      })
    } catch (error) {
      console.error('加载信用信息失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
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
