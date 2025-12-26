import { isLargeScreen } from '../../../utils/responsive'
import { trustScoreSystemService } from '../../../api/trust-score-system'

const app = getApp<IAppOption>()

interface MetricItem {
  label: string
  value: string
  status: 'GOOD' | 'WARNING' | 'BAD'
}

interface ViolationItem {
  id: number
  type: 'WARNING' | 'VIOLATION'
  title: string
  desc: string
  created_at: string
}

Page({
  data: {
    score: 0,
    level: '健康',
    levelDesc: '经营状况良好',
    metrics: [] as MetricItem[],
    violations: [] as ViolationItem[],
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    this.loadHealthInfo()
  },

  onShow() {
    this.loadHealthInfo()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadHealthInfo() {
    this.setData({ loading: true })
    try {
      const merchantId = app.globalData.merchantId
      if (!merchantId) {
        wx.showToast({ title: '请先登录商户', icon: 'none' })
        this.setData({ loading: false })
        return
      }

      // 获取商户信任分档案和历史
      const merchantIdNum = Number(merchantId)
      const [profile, historyResponse] = await Promise.all([
        trustScoreSystemService.getTrustScoreProfile('merchant', merchantIdNum),
        trustScoreSystemService.getTrustScoreHistory('merchant', merchantIdNum, 1, 20)
      ])

      // 计算等级信息
      const { level, levelDesc } = this.calculateLevelInfo(profile.current_score)

      // 计算违规和警告次数
      const negativeChanges = historyResponse.history.filter(h => h.change_amount < 0)
      const violationCount = negativeChanges.filter(h => Math.abs(h.change_amount) >= 10).length
      const warningCount = negativeChanges.filter(h => Math.abs(h.change_amount) < 10).length

      // 构建指标数据
      const metrics: MetricItem[] = [
        {
          label: '信任分',
          value: profile.current_score.toString(),
          status: profile.current_score >= 80 ? 'GOOD' : profile.current_score >= 60 ? 'WARNING' : 'BAD'
        },
        {
          label: '行为分',
          value: profile.score_breakdown.behavior_score.toString(),
          status: profile.score_breakdown.behavior_score >= 80 ? 'GOOD' : profile.score_breakdown.behavior_score >= 60 ? 'WARNING' : 'BAD'
        },
        {
          label: '违规次数',
          value: violationCount.toString(),
          status: violationCount === 0 ? 'GOOD' : violationCount <= 2 ? 'WARNING' : 'BAD'
        },
        {
          label: '警告次数',
          value: warningCount.toString(),
          status: warningCount === 0 ? 'GOOD' : warningCount <= 3 ? 'WARNING' : 'BAD'
        }
      ]

      // 转换违规记录
      const violations: ViolationItem[] = negativeChanges
        .slice(0, 5)
        .map(h => ({
          id: h.id,
          type: Math.abs(h.change_amount) >= 10 ? 'VIOLATION' as const : 'WARNING' as const,
          title: h.change_reason,
          desc: `扣除${Math.abs(h.change_amount)}分`,
          created_at: h.created_at
        }))

      this.setData({
        score: profile.current_score,
        level,
        levelDesc,
        metrics,
        violations,
        loading: false
      })
    } catch (error) {
      console.error('加载健康信息失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  calculateLevelInfo(score: number): { level: string; levelDesc: string } {
    if (score >= 90) {
      return { level: '优秀', levelDesc: '经营状况优秀' }
    } else if (score >= 80) {
      return { level: '健康', levelDesc: '经营状况良好' }
    } else if (score >= 60) {
      return { level: '一般', levelDesc: '需要改进' }
    } else {
      return { level: '警告', levelDesc: '存在风险' }
    }
  },

  onAppeal(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.navigateTo({ url: `/pages/merchant/appeals/index?id=${id}` })
  }
})
