import {
  OperatorRulesAdapter,
  operatorRulesService,
  type OperatorRuleCategory,
  type OperatorRuleItem
} from '../_api/operator-rules'

export interface OperatorRuleView extends Omit<OperatorRuleItem, 'category'> {
  category: OperatorRuleCategory
  icon?: string
}

export interface OperatorRuleCategoryViewItem {
  label: string
  value: OperatorRuleView['category']
  icon: string
}

export interface OperatorCategorizedRules {
  delivery: OperatorRuleView[]
  timeslot: OperatorRuleView[]
  weather: OperatorRuleView[]
}

export type OperatorRuleFilterCategory = OperatorRuleCategory

export interface OperatorRuleValidationResult {
  valid: boolean
  message: string
}

export interface OperatorRulesPageData {
  rules: OperatorRuleView[]
  categorizedRules: OperatorCategorizedRules
}

export function getOperatorRuleCategoryItems(): OperatorRuleCategoryViewItem[] {
  return [
    { label: '运费参数', value: 'delivery', icon: 'chart' },
    { label: '时段系数', value: 'timeslot', icon: 'time' },
    { label: '天气系数', value: 'weather', icon: 'cloud' }
  ]
}

export async function loadOperatorRulesPageData(regionId?: number): Promise<OperatorRulesPageData> {
  const params = regionId ? { region_id: regionId } : undefined
  const response = await operatorRulesService.listRules(params)
  const rules = response.rules.map((rule) => {
    const category = OperatorRulesAdapter.normalizeCategory(rule.category)
    const icon = OperatorRulesAdapter.getCategoryIcon(category)
    return { ...rule, category, icon }
  })

  return {
    rules,
    categorizedRules: {
      delivery: rules.filter((item) => item.category === 'delivery'),
      timeslot: rules.filter((item) => item.category === 'timeslot'),
      weather: rules.filter((item) => item.category === 'weather')
    }
  }
}

export async function updateOperatorRuleValue(params: {
  key: string
  value: string
  regionId?: number
}): Promise<void> {
  await operatorRulesService.updateRule(params.key, { value: params.value }, params.regionId)
}

export function validateOperatorRuleValue(rule: OperatorRuleView | null, value: string): OperatorRuleValidationResult {
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