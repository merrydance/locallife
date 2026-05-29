import {
  buildMerchantDiscountRuleStatusView,
  type MerchantDiscountRuleResponse
} from '../../../api/merchant'

export interface DiscountRuleView extends MerchantDiscountRuleResponse {
  min_order_amount_yuan: string
  discount_amount_yuan: string
  valid_range_text: string
  stacking_text: string
  statusPending: boolean
  deletePending: boolean
}

export interface DiscountRulePageChunk {
  rules: DiscountRuleView[]
  pageId: number
  pageSize: number
  total?: number
}

export interface DiscountRulePageProbeResult {
  hasMore: boolean
  total: number
  nextPageCache: DiscountRulePageChunk | null
}

export const DISCOUNT_RULES_PAGE_SIZE = 50
export const DISCOUNT_RULES_AUTO_REFRESH_WINDOW_MS = 60 * 1000

function formatAmount(amount: number) {
  return (amount / 100).toFixed(2)
}

function formatDate(date: string) {
  return String(date || '').slice(0, 10)
}

function buildStackingText(rule: MerchantDiscountRuleResponse) {
  const parts: string[] = []

  if (rule.can_stack_with_voucher) {
    parts.push('可叠加代金券')
  }
  if (rule.can_stack_with_membership) {
    parts.push('可叠加会员权益')
  }
  if (rule.stacking_group) {
    parts.push(`分组 ${rule.stacking_group}`)
  }

  return parts.length ? parts.join(' · ') : '默认不与其他优惠叠加'
}

export function buildRuleView(rule: MerchantDiscountRuleResponse): DiscountRuleView {
  const statusView = buildMerchantDiscountRuleStatusView(rule)

  return {
    ...rule,
    status_code: statusView.code,
    status_label: statusView.label,
    status_theme: statusView.theme,
    min_order_amount_yuan: formatAmount(rule.min_order_amount),
    discount_amount_yuan: formatAmount(rule.discount_amount),
    valid_range_text: `${formatDate(rule.valid_from)} 至 ${formatDate(rule.valid_until)}`,
    stacking_text: buildStackingText(rule),
    statusPending: false,
    deletePending: false
  }
}

function buildResultSummaryText(visibleCount: number) {
  return `当前已加载 ${visibleCount} 条满减规则`
}

export function buildDiscountRulePresentationUpdate(rules: DiscountRuleView[]) {
  return {
    rules,
    resultSummaryText: buildResultSummaryText(rules.length),
    emptyDescription: '当前还没有满减规则，先新增一个'
  }
}

export function appendRuleViews(existingRules: DiscountRuleView[], incomingRules: DiscountRuleView[]) {
  if (!incomingRules.length) {
    return existingRules
  }

  const merged = [...existingRules]
  const seen = new Set(existingRules.map((item) => item.id))

  incomingRules.forEach((rule) => {
    if (seen.has(rule.id)) {
      return
    }

    seen.add(rule.id)
    merged.push(rule)
  })

  return merged
}

export function upsertRuleView(rules: DiscountRuleView[], rule: MerchantDiscountRuleResponse) {
  const nextRule = buildRuleView(rule)
  const index = rules.findIndex((item) => item.id === nextRule.id)

  if (index === -1) {
    return [nextRule, ...rules]
  }

  const nextRules = [...rules]
  nextRules[index] = nextRule
  return nextRules
}

export function removeRuleView(rules: DiscountRuleView[], ruleId: number) {
  return rules.filter((item) => item.id !== ruleId)
}

export function hasReliableTotal(total: number | undefined, loadedCount: number) {
  return typeof total === 'number' && total > loadedCount
}

export function normalizeTotal(total: number | undefined, fallback: number) {
  return typeof total === 'number' && total >= 0 ? total : fallback
}

export function resolveTargetPageCount(pageId: number, pageSize: number, visibleCount: number) {
  const normalizedPageSize = pageSize > 0 ? pageSize : DISCOUNT_RULES_PAGE_SIZE
  const visiblePageCount = visibleCount > 0 ? Math.ceil(visibleCount / normalizedPageSize) : 1
  return Math.max(pageId || 0, visiblePageCount, 1)
}

export function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

export function buildDiscountRuleEditPageUrl(ruleId?: number) {
  if (ruleId && ruleId > 0) {
    return `/pages/merchant/discount-rules/edit/index?id=${ruleId}`
  }

  return '/pages/merchant/discount-rules/edit/index'
}