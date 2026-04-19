import { claimManagementService } from '../../../../api/appeals-customer-service'
import type { UserClaimType, SubmitClaimResponse } from '../../../../api/appeals-customer-service'
import { getOrderDetail } from '../../../../api/order'
import { logger } from '../../../../utils/logger'
import { getSubmitResultPresentation } from '../../../../utils/user-claim-submit-view'
import { getErrorUserMessage } from '../../../../utils/user-facing'

const SUPPORTED_USER_CLAIM_TYPES: UserClaimType[] = ['foreign-object', 'damage', 'timeout']

function normalizeUserClaimType(claimType?: string): UserClaimType | null {
  if (claimType && SUPPORTED_USER_CLAIM_TYPES.includes(claimType as UserClaimType)) {
    return claimType as UserClaimType
  }
  return null
}

/** 用户索赔类型 → 中文显示 */
const USER_CLAIM_TYPE_MAP: Record<UserClaimType, string> = {
  'foreign-object': '异物问题',
  'damage': '餐品损坏',
	'timeout': '配送超时'
}

function formatUserClaimType(type: UserClaimType): string {
  return USER_CLAIM_TYPE_MAP[type] || type
}

/** 索赔类型 → 图标映射 */
const TYPE_ICON_MAP: Record<UserClaimType, string> = {
  'foreign-object': 'search',
  'damage': 'heart-filled',
  'timeout': 'time'
}

/** 索赔类型 → 页面标题映射 */
const TYPE_TITLE_MAP: Record<UserClaimType, string> = {
  'foreign-object': '异物问题反馈',
  'damage': '餐品损坏反馈',
  'timeout': '配送超时反馈'
}

/** 最小索赔原因字数 */
const MIN_REASON_LENGTH = 5

/** 格式化金额（分→元） */
function formatAmount(fen: number): string {
  return (fen / 100).toFixed(2)
}

Page({
  data: {
    navBarHeight: 88,
    claimType: '' as UserClaimType,
    claimTypeText: '',
    pageTitle: '提交反馈',
    typeIcon: '',
    typeClass: '',

    // 订单
    selectedOrder: null as { id: number, orderNo: string, amount: number, amountDisplay: string } | null,

    // 表单
    amountInput: '',
    reasonInput: '',
    canSubmit: false,
    submitting: false,

    // 结果
    submitResult: null as SubmitClaimResponse | null,
    resultIcon: '',
    resultColor: '',
    resultTitle: '',
    resultSummary: '',
    approvedAmountDisplay: ''
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    if (e.detail.navBarHeight !== null && e.detail.navBarHeight !== undefined) {
      this.setData({ navBarHeight: e.detail.navBarHeight })
    }
  },

  onLoad(options: { claimType?: string, orderId?: string }) {
	const claimType = normalizeUserClaimType(options.claimType)
  if (!claimType) {
    logger.warn('[SubmitClaim] unsupported claim type', options.claimType)
    wx.showToast({ title: '暂不支持该反馈类型', icon: 'none', duration: 2000 })
    setTimeout(() => {
    wx.navigateBack({
      fail: () => {
      wx.redirectTo({ url: '/pages/user_center/service_center/index' })
      }
    })
    }, 2000)
    return
  }
    this.setData({
      claimType,
      claimTypeText: formatUserClaimType(claimType),
      pageTitle: TYPE_TITLE_MAP[claimType] || '提交反馈',
      typeIcon: TYPE_ICON_MAP[claimType] || 'info-circle',
      typeClass: `type-${claimType}`
    })

    // 如果从订单详情跳转，自动关联订单
    if (options.orderId) {
      this.loadOrder(parseInt(options.orderId))
    }
  },

  async loadOrder(orderId: number) {
    try {
      const order = await getOrderDetail(orderId)
      this.setData({
        selectedOrder: {
          id: orderId,
          orderNo: order.order_no,
          amount: order.total_amount,
          amountDisplay: formatAmount(order.total_amount)
        }
      })
      this.validateForm()
    } catch (err) {
      logger.error('[SubmitClaim] loadOrder failed', err)
    }
  },

  onSelectOrder() {
    wx.navigateTo({
      url: '/pages/orders/list/index?tab=completed&selectMode=1',
      events: {
        onOrderSelected: (order: { id: number, orderNo: string, totalAmount: number }) => {
          this.setData({
            selectedOrder: {
              id: order.id,
              orderNo: order.orderNo,
              amount: order.totalAmount,
              amountDisplay: formatAmount(order.totalAmount)
            }
          })
          this.validateForm()
        }
      }
    })
  },

  onAmountInput(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    this.setData({ amountInput: e.detail.value })
    this.validateForm()
  },

  onReasonInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ reasonInput: e.detail.value })
    this.validateForm()
  },

  validateForm() {
    const { selectedOrder, amountInput, reasonInput } = this.data
    const amount = parseFloat(amountInput)
    const hasOrder = selectedOrder !== null
    const hasValidAmount = !isNaN(amount) && amount > 0
    const hasValidReason = reasonInput.trim().length >= MIN_REASON_LENGTH
    const notExceedMax = !hasOrder || (amount * 100) <= selectedOrder!.amount

    this.setData({
      canSubmit: hasOrder && hasValidAmount && hasValidReason && notExceedMax
    })
  },

  async onSubmit() {
    if (this.data.submitting || !this.data.canSubmit || !this.data.selectedOrder) return

    const amountFen = Math.round(parseFloat(this.data.amountInput) * 100)

    if (amountFen > this.data.selectedOrder.amount) {
      wx.showToast({ title: '金额不能超过订单总额', icon: 'none' })
      return
    }

    this.setData({ submitting: true })

    try {
      const result = await claimManagementService.submitClaim({
        order_id: this.data.selectedOrder.id,
        claim_type: this.data.claimType,
        claim_amount: amountFen,
        claim_reason: this.data.reasonInput.trim()
      })

      const presentation = getSubmitResultPresentation(result)

      this.setData({
        submitResult: result,
        resultIcon: presentation.icon,
        resultColor: presentation.color,
        resultTitle: presentation.title,
        resultSummary: presentation.summary,
        approvedAmountDisplay:
          result.approved_amount !== null && result.approved_amount !== undefined
            ? formatAmount(result.approved_amount)
            : '',
        submitting: false
      })
    } catch (err: unknown) {
      this.setData({ submitting: false })
      const message = getErrorUserMessage(err, '提交失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none', duration: 3000 })
      logger.error('[SubmitClaim] submit failed', err)
    }
  },

  onBackToCenter() {
    wx.navigateBack()
  }
})
