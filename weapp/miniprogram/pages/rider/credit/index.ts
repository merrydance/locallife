import { riderBasicManagementService } from '../../../api/rider-basic-management'

interface SummaryItem {
  label: string
  value: string
  tone: 'primary' | 'success' | 'warning' | 'danger'
}

interface HistoryItem {
  id: number
  type: 'positive' | 'negative' | 'neutral'
  amount: string
  title: string
  subtitle: string
  tagText: string
  created_at: string
}

interface QualificationInfo {
  label: string
  desc: string
  tagTheme: 'success' | 'warning' | 'danger'
}

Page({
  data: {
    score: 0,
    riderName: '',
    qualificationLabel: '资格校验中',
    qualificationDesc: '正在加载高值单资格信息',
    qualificationTheme: 'default',
    summary: [] as SummaryItem[],
    history: [] as HistoryItem[],
    total: 0,
    loading: false,
    loadingMore: false,
    errorMessage: '',
    navBarHeight: 88,
    pageID: 1,
    pageSize: 20,
    hasMore: true
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

  async loadCreditInfo(reset: boolean = true) {
    if (this.data.loading || this.data.loadingMore) return

    const nextPage = reset ? 1 : this.data.pageID + 1
    this.setData(reset ? { loading: true } : { loadingMore: true })
    try {
      const [scoreInfo, historyResponse] = await Promise.all([
        riderBasicManagementService.getRiderScore(),
        riderBasicManagementService.getScoreHistory({ page_id: nextPage, page_size: this.data.pageSize })
      ])

      const qualification = this.getQualificationInfo(scoreInfo.premium_score, scoreInfo.can_accept_premium_order)

      const summary: SummaryItem[] = [
        {
          label: '当前资格分',
          value: String(scoreInfo.premium_score),
          tone: scoreInfo.premium_score >= 0 ? 'success' : 'danger'
        },
        {
          label: '当前资格',
          value: scoreInfo.can_accept_premium_order ? '可接高值单' : '暂不可接高值单',
          tone: scoreInfo.can_accept_premium_order ? 'success' : 'warning'
        },
        {
          label: '变更记录',
          value: String(historyResponse.total || 0),
          tone: 'primary'
        }
      ]

      const history: HistoryItem[] = (historyResponse.logs || []).map((item) => {
        const type = item.change_amount > 0 ? 'positive' : item.change_amount < 0 ? 'negative' : 'neutral'
        const amount = item.change_amount > 0 ? `+${item.change_amount}` : `${item.change_amount}`
        const relatedLabel = item.related_order_id ? `关联订单 #${item.related_order_id}` : item.related_delivery_id ? `配送单 #${item.related_delivery_id}` : '无关联单据'

        return {
          id: item.id,
          type,
          amount,
          title: item.change_type_name || item.change_type,
          subtitle: item.remark || relatedLabel,
          tagText: item.change_type_name || item.change_type,
          created_at: item.created_at
        }
      })

      this.setData({
        score: scoreInfo.premium_score,
        riderName: scoreInfo.real_name,
        qualificationLabel: qualification.label,
        qualificationDesc: qualification.desc,
        qualificationTheme: qualification.tagTheme,
        summary,
        history: reset ? history : [...this.data.history, ...history],
        total: historyResponse.total || 0,
        pageID: nextPage,
        hasMore: nextPage * this.data.pageSize < (historyResponse.total || 0),
        errorMessage: ''
      })
    } catch (error) {
      console.error('加载信用信息失败:', error)
      const message = error instanceof Error && error.message ? error.message : '高值单资格加载失败，请稍后重试'
      if (reset) {
        this.setData({ errorMessage: message, history: [] })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false, loadingMore: false })
    }
  },

  getQualificationInfo(score: number, canAcceptPremiumOrder: boolean): QualificationInfo {
    if (canAcceptPremiumOrder && score >= 10) {
      return { label: '资格稳定', desc: '当前可稳定承接高值单，继续保持履约质量。', tagTheme: 'success' }
    }
    if (canAcceptPremiumOrder) {
      return { label: '已达资格线', desc: '当前可接高值单，近期波动较小时资格会更稳定。', tagTheme: 'success' }
    }
    if (score >= -10) {
      return { label: '暂时受限', desc: '当前暂不可接高值单，继续正常履约可逐步恢复资格。', tagTheme: 'warning' }
    }

    return { label: '风险较高', desc: '近期资格分偏低，建议优先减少超时和异常履约。', tagTheme: 'danger' }
  },

  onRetry() {
    this.loadCreditInfo(true)
  },

  onReachBottom() {
    if (this.data.hasMore && !this.data.loading && !this.data.loadingMore) {
      this.loadCreditInfo(false)
    }
  }
})
