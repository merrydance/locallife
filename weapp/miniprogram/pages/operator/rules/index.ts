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
  editable: boolean
  category: 'delivery' | 'weather' | 'timeslot'
  action?: 'edit' | 'navigate_peak'
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

interface RulesPageOptions {
  region_id?: string
  region_name?: string
}

interface RuleIdDataset {
  id?: string
}

interface ValueChangeDetail {
  value?: string
}

Page({
  data: {
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false,
    initialLoading: true,
    error: null as string | null,

    selectedRegionId: 0,
    selectedRegionName: '',
    
    activeCategory: 'delivery',
    categories: [
      { label: '运费参数', value: 'delivery', icon: 'chart' },
      { label: '时段系数', value: 'timeslot', icon: 'time' },
      { label: '天气系数', value: 'weather', icon: 'cloud' }
    ] as RuleCategoryItem[],
    
    rules: [] as RuleItem[],
    categorizedRules: {
      delivery: [] as RuleItem[],
      timeslot: [] as RuleItem[],
      weather: [] as RuleItem[]
    },

    // 编辑弹窗
    showEdit: false,
    editingRule: null as RuleItem | null,
    newValue: ''
  },

  onLoad(options: RulesPageOptions) {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ 
      isLargeScreen: isLargeScreen(),
      navBarHeight 
    })
    const selectedRegionId = Number(options?.region_id || 0)
    const selectedRegionName = options?.region_name ? decodeURIComponent(options.region_name) : ''

    if (!selectedRegionId) {
      wx.redirectTo({ url: '/pages/operator/region/index?target=rules' })
      return
    }

    this.setData({ selectedRegionId, selectedRegionName })
    this.loadRules()
  },

  async loadRules() {
    this.setData({ loading: true, error: null })
    try {
      const data = this.data.selectedRegionId > 0 ? { region_id: this.data.selectedRegionId } : undefined
      const res = await request<RulesResponse>({ url: '/v1/operator/rules', method: 'GET', data })
      
      // 按后端返回的 category/editable 渲染，避免前端猜测 key 造成漂移
      const enhancedRules = res.rules.map((rule) => {
        const category: RuleItem['category'] =
          rule.category === 'weather' || rule.category === 'timeslot' ? rule.category : 'delivery'

        let icon = 'chart'
        if (category === 'weather') {
          icon = 'cloud'
        } else if (category === 'timeslot') {
          icon = 'time'
        }

        return { ...rule, category, icon }
      })

      const categorized = {
        delivery: enhancedRules.filter((r) => r.category === 'delivery'),
        timeslot: enhancedRules.filter((r) => r.category === 'timeslot'),
        weather: enhancedRules.filter((r) => r.category === 'weather')
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

    if (!rule.editable) {
      wx.showToast({ title: '该项由平台统一维护，运营商不可修改', icon: 'none' })
      return
    }

    if (rule.action === 'navigate_peak') {
      const regionName = this.data.selectedRegionName ? `&region_name=${encodeURIComponent(this.data.selectedRegionName)}` : ''
      const query = this.data.selectedRegionId > 0 ? `?region_id=${this.data.selectedRegionId}${regionName}` : ''
      wx.navigateTo({ url: `/pages/operator/timeslot/index${query}` })
      return
    }

    this.setData({
      showEdit: true,
      editingRule: rule,
      newValue: rule.value
    })
  },

  onValueChange(e: WechatMiniprogram.CustomEvent<ValueChangeDetail>) {
    const value = typeof e.detail?.value === 'string' ? e.detail.value : ''
    this.setData({ newValue: value })
  },

  onCloseEdit() {
    this.setData({ showEdit: false, editingRule: null })
  },

  async confirmEdit() {
    const { editingRule, newValue } = this.data
    if (!editingRule || newValue === '') return

    try {
      wx.showLoading({ title: '保存中...', mask: true })
      const regionQuery = this.data.selectedRegionId > 0 ? `?region_id=${this.data.selectedRegionId}` : ''
      await request({ 
        url: `/v1/operator/rules/${editingRule.key}${regionQuery}`, 
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
