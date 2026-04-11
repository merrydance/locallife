import { isLargeScreen, getStableBarHeights } from '../../../utils/responsive'
import {
  operatorRulesService,
  OperatorRulesAdapter,
  type OperatorRuleCategory,
  type OperatorRuleItem
} from '../../../api/operator-rules'
import { logger } from '../../../utils/logger'

interface RuleItem extends Omit<OperatorRuleItem, 'category'> {
  category: OperatorRuleCategory
  icon?: string
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

type RuleCategory = OperatorRuleCategory

interface RuleValidationResult {
  valid: boolean
  message: string
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
    
    activeCategory: 'delivery' as RuleCategory,
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
    newValue: '',
    valueError: '',
    saving: false
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
      const params = this.data.selectedRegionId > 0 ? { region_id: this.data.selectedRegionId } : undefined
      const res = await operatorRulesService.listRules(params)
      
      // 按后端返回的 category/editable 渲染，避免前端猜测 key 造成漂移
      const enhancedRules = res.rules.map((rule) => {
        const category = OperatorRulesAdapter.normalizeCategory(rule.category)
        const icon = OperatorRulesAdapter.getCategoryIcon(category)

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
      newValue: rule.value,
      valueError: ''
    })
  },

  onValueChange(e: WechatMiniprogram.CustomEvent<ValueChangeDetail>) {
    const value = typeof e.detail?.value === 'string' ? e.detail.value : ''
    const validation = this.validateRuleValue(this.data.editingRule, value)
    this.setData({ newValue: value, valueError: validation.valid ? '' : validation.message })
  },

  onCloseEdit() {
    this.setData({ showEdit: false, editingRule: null, newValue: '', valueError: '', saving: false })
  },

  async confirmEdit() {
    const { editingRule, newValue } = this.data
    if (!editingRule || newValue === '') return

    if (this.data.saving) return

    const trimmedValue = newValue.trim()
    if (trimmedValue === editingRule.value) {
      wx.showToast({ title: '规则值未发生变化', icon: 'none' })
      return
    }

    const validation = this.validateRuleValue(editingRule, trimmedValue)
    if (!validation.valid) {
      this.setData({ valueError: validation.message })
      wx.showToast({ title: validation.message, icon: 'none' })
      return
    }

    try {
      this.setData({ saving: true })
      wx.showLoading({ title: '保存中...', mask: true })
      await operatorRulesService.updateRule(
        editingRule.key,
        { value: trimmedValue },
        this.data.selectedRegionId > 0 ? this.data.selectedRegionId : undefined
      )
      
      wx.hideLoading()
      this.setData({ showEdit: false, editingRule: null, newValue: '', valueError: '', saving: false })
      this.loadRules() // 重新加载以确认为最新数据
    } catch (err: unknown) {
      wx.hideLoading()
      this.setData({ saving: false })
      logger.error('修改规则失败:', err)
      // request.ts 通常会自动显示错误 toast
    }
  },

  validateRuleValue(rule: RuleItem | null, value: string): RuleValidationResult {
    if (!rule) {
      return { valid: false, message: '缺少规则信息' }
    }

    const trimmedValue = value.trim()
    if (!trimmedValue) {
      return { valid: false, message: '规则值不能为空' }
    }

    const numericValue = Number(trimmedValue)
    if (!Number.isFinite(numericValue)) {
      return { valid: false, message: '规则值必须是数字' }
    }

    if (numericValue < 0) {
      return { valid: false, message: '规则值不能为负数' }
    }

    if (rule.category === 'weather' || rule.category === 'timeslot') {
      if (numericValue < 0.1 || numericValue > 10) {
        return { valid: false, message: '系数范围需在 0.1 到 10 之间' }
      }
    }

    if (rule.unit.includes('%') && numericValue > 100) {
      return { valid: false, message: '百分比规则不能超过 100' }
    }

    return { valid: true, message: '' }
  }
})
