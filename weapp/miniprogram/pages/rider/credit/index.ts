import { riderBasicManagementService } from '../../../api/rider-basic-management'

interface MetricItem {
  label: string
  value: string
  status: 'GOOD' | 'WARNING' | 'BAD'
}

interface HistoryItem {
  id: number
  type: 'REWARD' | 'PENALTY'
  amount: number
  reason: string
  created_at: string
}

Page({
  data: {
    score: 0,
    level: '普通骑手',
    levelDesc: '服务正常',
    metrics: [] as MetricItem[],
    history: [] as HistoryItem[],
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
      // 获取骑手积分信息和历史
      const [scoreInfo, historyResponse] = await Promise.all([
        riderBasicManagementService.getRiderScore(),
        riderBasicManagementService.getScoreHistory({ page_id: 1, page_size: 20 })
      ])

      // 计算等级信息
      const { level, levelDesc } = this.calculateLevelInfo(scoreInfo.current_score)

      // 构建指标数据
      const metrics: MetricItem[] = [
        {
          label: '当前积分',
          value: scoreInfo.current_score.toString(),
          status: scoreInfo.current_score >= 80 ? 'GOOD' : scoreInfo.current_score >= 60 ? 'WARNING' : 'BAD'
        },
        {
          label: '积分等级',
          value: scoreInfo.score_level,
          status: 'GOOD'
        },
        {
          label: '可接高值单',
          value: scoreInfo.can_take_high_value_orders ? '是' : '否',
          status: scoreInfo.can_take_high_value_orders ? 'GOOD' : 'WARNING'
        }
      ]

      // 转换历史记录
      const history: HistoryItem[] = historyResponse.history.map(h => ({
        id: h.id,
        type: h.score_change >= 0 ? 'REWARD' : 'PENALTY',
        amount: h.score_change,
        reason: h.reason,
        created_at: h.created_at
      }))

      this.setData({
        score: scoreInfo.current_score,
        level,
        levelDesc,
        metrics,
        history,
        loading: false
      })
    } catch (error) {
      console.error('加载信用信息失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  calculateLevelInfo(score: number): { level: string; levelDesc: string } {
    if (score >= 95) {
      return { level: '钻石骑手', levelDesc: '服务卓越' }
    } else if (score >= 85) {
      return { level: '金牌骑手', levelDesc: '服务优异' }
    } else if (score >= 75) {
      return { level: '银牌骑手', levelDesc: '服务良好' }
    } else if (score >= 60) {
      return { level: '普通骑手', levelDesc: '服务正常' }
    } else {
      return { level: '受限骑手', levelDesc: '需要改进' }
    }
  }
})
