import { isLargeScreen, getStableBarHeights } from '../../../utils/responsive'
import {
  getOperatorRuleCategoryItems,
  loadOperatorRulesPageData,
  updateOperatorRuleValue,
  validateOperatorRuleValue,
  type OperatorRuleCategoryViewItem,
  type OperatorRuleFilterCategory,
  type OperatorRuleValidationResult,
  type OperatorRuleView
} from '../../../services/operator-rules-management'
import { logger } from '../../../utils/logger'

interface RuleCategoryDataset {
  val?: OperatorRuleView['category']
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
    
    activeCategory: 'delivery' as OperatorRuleFilterCategory,
    categories: getOperatorRuleCategoryItems() as OperatorRuleCategoryViewItem[],
    
    rules: [] as OperatorRuleView[],
    categorizedRules: {
      delivery: [] as OperatorRuleView[],
      timeslot: [] as OperatorRuleView[],
      weather: [] as OperatorRuleView[]
    },

    // 编辑弹窗
    showEdit: false,
    editingRule: null as OperatorRuleView | null,
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
      const nextView = await loadOperatorRulesPageData(this.data.selectedRegionId)

      this.setData({
        ...nextView,
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
    const validation = validateOperatorRuleValue(this.data.editingRule, value)
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
      await updateOperatorRuleValue({
        key: editingRule.key,
        value: trimmedValue,
        regionId: this.data.selectedRegionId > 0 ? this.data.selectedRegionId : undefined
      })
      
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

  validateRuleValue(_rule: OperatorRuleView | null, _value: string): OperatorRuleValidationResult {
    return validateOperatorRuleValue(_rule, _value)
  }
})
