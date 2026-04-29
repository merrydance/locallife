import { claimManagementService } from '../../../../api/appeals-customer-service'
import type { UserClaimResponse, UserClaimType, SubmitClaimResponse } from '../../../../api/appeals-customer-service'
import { getOrderDetail, getOrderList } from '../../../../api/order'
import { logger } from '../../../../utils/logger'
import {
  buildClaimOrderOptions,
  formatClaimAmount,
  getClaimCandidateOrderListParams,
  getSubmitResultPresentation,
  isClaimCandidateOrder,
  toSelectedClaimOrder,
  type ClaimOrderOption,
  type SelectedClaimOrder
} from '../../../../utils/user-claim-submit-view'
import { getErrorUserMessage } from '../../../../utils/user-facing'

const SUPPORTED_USER_CLAIM_TYPES: UserClaimType[] = ['foreign-object', 'damage', 'timeout']

function normalizeUserClaimType(claimType?: string): UserClaimType | null {
  if (claimType && SUPPORTED_USER_CLAIM_TYPES.includes(claimType as UserClaimType)) {
    return claimType as UserClaimType
  }
  return null
}

/** 索赔类型 → 页面标题映射 */
const TYPE_TITLE_MAP: Record<UserClaimType, string> = {
  'foreign-object': '异物问题反馈',
  'damage': '餐品损坏反馈',
  'timeout': '配送超时反馈'
}

/** 最小索赔原因字数 */
const MIN_REASON_LENGTH = 5

function getEventOrderId(event: WechatMiniprogram.BaseEvent): number {
  const dataset = event.currentTarget.dataset as { id?: string | number }
  const id = typeof dataset.id === 'number' ? dataset.id : Number(dataset.id)
  return Number.isFinite(id) ? id : 0
}

Page({
  data: {
    navBarHeight: 88,
    claimType: '' as UserClaimType,
    pageTitle: '提交反馈',

    // 订单
    selectedOrder: null as SelectedClaimOrder | null,
    orderPickerVisible: false,
    orderCandidates: [] as ClaimOrderOption[],
    orderCandidatesLoading: false,
    orderCandidatesLoaded: false,
    orderCandidatesError: '',

    // 表单
    reasonInput: '',
    canSubmit: false,
    submitting: false,

    // 结果
    submitResult: null as SubmitClaimResponse | null,
    resultTheme: 'default' as 'default' | 'success' | 'warning' | 'error',
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
      pageTitle: TYPE_TITLE_MAP[claimType] || '提交反馈'
    })

    if (options.orderId) {
      this.loadOrder(parseInt(options.orderId, 10))
    }
  },

  async loadUserClaimsForEligibility(): Promise<UserClaimResponse[]> {
    const claims: UserClaimResponse[] = []
    const pageSize = 100
    const maxPages = 5

    for (let page = 1; page <= maxPages; page += 1) {
      const result = await claimManagementService.getUserClaims({ page, page_size: pageSize })
      const pageClaims = result.claims || []
      claims.push(...pageClaims)
      if (claims.length >= result.total || pageClaims.length < pageSize) break
    }

    return claims
  },

  applySelectedOrder(selectedOrder: SelectedClaimOrder) {
    const nextCandidates = this.data.orderCandidates.map((order) => ({
      ...order,
      selected: order.id === selectedOrder.id
    }))

    this.setData({
      selectedOrder,
      orderCandidates: nextCandidates,
      orderPickerVisible: false
    })
    this.validateForm()
  },

  async loadOrder(orderId: number) {
    try {
      const [order, claims] = await Promise.all([
        getOrderDetail(orderId),
        this.loadUserClaimsForEligibility()
      ])

      if (!isClaimCandidateOrder(order)) {
        wx.showToast({ title: '该订单暂不支持索赔', icon: 'none' })
        return
      }

      if (claims.some((claim) => claim.order_id === orderId)) {
        wx.showToast({ title: '该订单已提交过索赔', icon: 'none' })
        return
      }

      this.applySelectedOrder(toSelectedClaimOrder(order))
    } catch (err) {
      logger.error('[SubmitClaim] loadOrder failed', err)
      wx.showToast({ title: getErrorUserMessage(err, '订单信息加载失败'), icon: 'none' })
    }
  },

  onSelectOrder() {
    this.setData({ orderPickerVisible: true })
    if (!this.data.orderCandidatesLoaded && !this.data.orderCandidatesLoading) {
      void this.loadClaimOrderCandidates()
    }
  },

  onOrderPickerVisibleChange(e: WechatMiniprogram.CustomEvent<{ visible: boolean }>) {
    this.setData({ orderPickerVisible: e.detail.visible })
  },

  closeOrderPicker() {
    this.setData({ orderPickerVisible: false })
  },

  async loadClaimOrderCandidates() {
    this.setData({ orderCandidatesLoading: true, orderCandidatesError: '' })
    try {
      const [orderResult, claims] = await Promise.all([
        getOrderList(getClaimCandidateOrderListParams()),
        this.loadUserClaimsForEligibility()
      ])
      const candidates = buildClaimOrderOptions(
        orderResult.orders || [],
        claims,
        this.data.selectedOrder?.id
      )

      this.setData({
        orderCandidates: candidates,
        orderCandidatesLoading: false,
        orderCandidatesLoaded: true
      })
    } catch (err) {
      logger.error('[SubmitClaim] load claim order candidates failed', err)
      this.setData({
        orderCandidatesLoading: false,
        orderCandidatesLoaded: true,
        orderCandidatesError: getErrorUserMessage(err, '订单加载失败，请稍后重试')
      })
    }
  },

  onRetryClaimOrders() {
    void this.loadClaimOrderCandidates()
  },

  onChooseOrder(e: WechatMiniprogram.BaseEvent) {
    const id = getEventOrderId(e)
    if (!id) return
    const selectedOrder = this.data.orderCandidates.find((order) => order.id === id)
    if (!selectedOrder) return
    this.applySelectedOrder(selectedOrder)
  },

  onReasonInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ reasonInput: e.detail.value })
    this.validateForm()
  },

  validateForm() {
    const { selectedOrder, reasonInput } = this.data
    const hasValidReason = reasonInput.trim().length >= MIN_REASON_LENGTH

    this.setData({
      canSubmit: !!selectedOrder && selectedOrder.amount > 0 && hasValidReason
    })
  },

  async onSubmit() {
    if (this.data.submitting || !this.data.canSubmit || !this.data.selectedOrder) return

    this.setData({ submitting: true })

    try {
      const result = await claimManagementService.submitClaim({
        order_id: this.data.selectedOrder.id,
        claim_type: this.data.claimType,
        claim_amount: this.data.selectedOrder.amount,
        claim_reason: this.data.reasonInput.trim()
      })

      const presentation = getSubmitResultPresentation(result)

      this.setData({
        submitResult: result,
        resultTheme: presentation.theme,
        resultTitle: presentation.title,
        resultSummary: presentation.summary,
        approvedAmountDisplay:
          result.approved_amount !== null && result.approved_amount !== undefined
            ? formatClaimAmount(result.approved_amount)
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
