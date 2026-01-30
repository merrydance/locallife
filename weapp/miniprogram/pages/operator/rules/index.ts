import { isLargeScreen } from '@/utils/responsive'
import { request } from '@/utils/request'

interface RuleItem {
  id: string
  name: string
  key: string
  value: string
  unit: string
  desc: string
}

interface RulesResponse {
  rules: RuleItem[]
}

Page({
  data: {
    rules: [] as RuleItem[],
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false,
    initialLoading: true,
    error: null as string | null,
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    this.loadRules()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadRules() {
    this.setData({ loading: true, error: null })
    try {
      const res = await request<RulesResponse>({ url: '/v1/operator/rules', method: 'GET' })
      this.setData({
        rules: res.rules,
        loading: false,
        initialLoading: false
      })
    } catch (error) {
      console.error('加载规则配置失败:', error)
      this.setData({ 
        loading: false,
        initialLoading: false,
        error: '加载规则配置失败' 
      })
    }
  },

  onRetry() {
    this.loadRules()
  },

  onEditRule(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    const rule = this.data.rules.find((r) => r.id === id)
    if (!rule) return

    // 天气系数由系统更新，不允许手动修改
    if (rule.key === 'WEATHER_COEFFICIENT') {
      wx.showToast({ title: '该规则由系统自动更新，无法手动修改', icon: 'none' })
      return
    }

    wx.showModal({
      title: `修改${rule.name}`,
      content: rule.value,
      editable: true,
      placeholderText: '请输入新值',
      success: async (res) => {
        if (res.confirm && res.content) {
          try {
            wx.showLoading({ title: '保存中...' })
            await request({ 
              url: `/v1/operator/rules/${rule.key}`, 
              method: 'PATCH', 
              data: { value: res.content } 
            })
            wx.hideLoading()
            wx.showToast({ title: '修改成功', icon: 'success' })
            this.loadRules()
          } catch (err) {
            wx.hideLoading()
            console.error('修改规则失败:', err)
            // request.ts 通常会自动显示错误 toast
          }
        }
      }
    })
  }
})
