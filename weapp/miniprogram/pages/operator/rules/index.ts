import { isLargeScreen, getStableBarHeights } from '../../../utils/responsive'
import { request } from '../../../utils/request'
import { logger } from '../../../utils/logger'

interface RuleItem {
  id: string
  name: string
  key: string
  value: string
  unit: string
  desc: string
  category: 'rider' | 'merchant' | 'platform'
  icon?: string
}

interface RulesResponse {
  rules: RuleItem[]
}

interface RuleCategoryItem {
  label: string
  value: RuleItem['category']
  icon: string
}

interface RuleCategoryDataset {
  val?: RuleItem['category']
}

interface RuleIdDataset {
  id?: string
}

interface ValueChangeDetail {
  value: string
}

Page({
  data: {
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false,
    initialLoading: true,
    error: null as string | null,
    
    activeCategory: 'rider',
    categories: [
      { label: '骑手规则', value: 'rider', icon: 'user' },
      { label: '商户规则', value: 'merchant', icon: 'shop' },
      { label: '平台策略', value: 'platform', icon: 'setting' }
    ] as RuleCategoryItem[],
    
    rules: [] as RuleItem[],
    categorizedRules: {
      rider: [] as RuleItem[],
      merchant: [] as RuleItem[],
      platform: [] as RuleItem[]
    },

    // 编辑弹窗
    showEdit: false,
    editingRule: null as RuleItem | null,
    newValue: ''
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ 
      isLargeScreen: isLargeScreen(),
      navBarHeight 
    })
    this.loadRules()
  },

  async loadRules() {
    this.setData({ loading: true, error: null })
    try {
      const res = await request<RulesResponse>({ url: '/v1/operator/rules', method: 'GET' })
      
      // 模拟一些分类数据，以便 UI 展示更丰富（如果是真实后端，这些字段应由后端返回）
      const enhancedRules = res.rules.map((rule) => {
        let category: RuleItem['category'] = 'platform'
        let icon = 'info-circle'
        
        if (rule.key.includes('RIDER') || rule.key.includes('DELIVERY')) {
          category = 'rider'
          icon = 'user-assignment'
        } else if (rule.key.includes('MERCHANT') || rule.key.includes('COMMISSION')) {
          category = 'merchant'
          icon = 'shop'
        }
        
        return { ...rule, category, icon }
      })

      const categorized = {
        rider: enhancedRules.filter((r) => r.category === 'rider'),
        merchant: enhancedRules.filter((r) => r.category === 'merchant'),
        platform: enhancedRules.filter((r) => r.category === 'platform')
      }

      this.setData({
        rules: enhancedRules,
        categorizedRules: categorized,
        loading: false,
        initialLoading: false
      })
    } catch (error) {
      logger.error('加载规则配置失败:', error)
      this.setData({ 
        loading: false,
        initialLoading: false,
        error: '加载规则配置失败，请检查网络或后端权限' 
      })
    }
  },

  switchCategory(e: WechatMiniprogram.TouchEvent) {
    const { val } = e.currentTarget.dataset as RuleCategoryDataset
    if (!val) return
    this.setData({ activeCategory: val })
  },

  onEditRule(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as RuleIdDataset
    if (!id) return
    const rule = this.data.rules.find((r) => r.id === id)
    if (!rule) return

    // 天气系数由系统同步，通常不允许手动修改（除非有特殊业务逻辑）
    if (rule.key === 'WEATHER_COEFFICIENT') {
      wx.showToast({ title: '该项由气象接口同步，无法手动修改', icon: 'none' })
      return
    }

    this.setData({
      showEdit: true,
      editingRule: rule,
      newValue: rule.value
    })
  },

  onValueChange(e: WechatMiniprogram.CustomEvent<ValueChangeDetail>) {
    this.setData({ newValue: e.detail.value })
  },

  onCloseEdit() {
    this.setData({ showEdit: false, editingRule: null })
  },

  async confirmEdit() {
    const { editingRule, newValue } = this.data
    if (!editingRule || newValue === '') return

    try {
      wx.showLoading({ title: '保存中...', mask: true })
      await request({ 
        url: `/v1/operator/rules/${editingRule.key}`, 
        method: 'PATCH', 
        data: { value: newValue } 
      })
      
      wx.hideLoading()
      wx.showToast({ title: '更新成功', icon: 'success' })
      this.setData({ showEdit: false })
      this.loadRules() // 重新加载以确认为最新数据
    } catch (err: unknown) {
      wx.hideLoading()
      logger.error('修改规则失败:', err)
      // request.ts 通常会自动显示错误 toast
    }
  }
})
