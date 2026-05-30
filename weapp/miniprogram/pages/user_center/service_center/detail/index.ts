import { claimManagementService, getUserClaimPresentation } from '../_main_shared/api/appeals-customer-service'
import type { ClaimPayoutConfirmationResponse, UserClaimResponse } from '../_main_shared/api/appeals-customer-service'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import {
  isClaimPayoutRealNameRequiredError
} from '../_utils/claim-payout-real-name'
import { ensureClaimPayoutRealName } from '../_utils/claim-payout-real-name-workflow'

/** 索赔类型 → 中文显示（涵盖所有 claim_type 值） */
const CLAIM_TYPE_DISPLAY: Record<string, string> = {
  'foreign-object': '异物问题',
  'damage': '餐品损坏',
  'timeout': '代取超时',
  'food-safety': '食品安全',
  'refund': '退款',
  'compensation': '赔偿',
  'quality_issue': '质量问题',
  'delivery_issue': '代取问题'
}

function displayClaimType(type: string): string {
  return CLAIM_TYPE_DISPLAY[type] || type
}

/** 格式化金额（分 → 元） */
function formatAmount(amount: number): string {
  return (amount / 100).toFixed(2)
}

/** 格式化时间 */
function formatDateTime(dateStr: string): string {
  const d = new Date(dateStr)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const h = String(d.getHours()).padStart(2, '0')
  const min = String(d.getMinutes()).padStart(2, '0')
  return `${y}-${m}-${day} ${h}:${min}`
}

interface RequestMerchantTransferOptions {
  mchId: string
  appId: string
  package: string
  success?: (res: RequestMerchantTransferResult) => void
  fail?: (err: { errMsg?: string }) => void
}

interface WxWithMerchantTransfer {
  requestMerchantTransfer?: (options: RequestMerchantTransferOptions) => void
}

interface RequestMerchantTransferResult {
  errMsg?: string
}

class RequestMerchantTransferCancelError extends Error {
  constructor(message = '用户已取消确认收款') {
    super(message)
    this.name = 'RequestMerchantTransferCancelError'
  }
}

function isRequestMerchantTransferCancelError(err: unknown): boolean {
  return err instanceof RequestMerchantTransferCancelError
}

function normalizeRequestMerchantTransferErrMsg(result?: RequestMerchantTransferResult): string {
  return String(result?.errMsg || '').trim()
}

function rejectRequestMerchantTransferResult(result: RequestMerchantTransferResult | undefined, reject: (reason?: unknown) => void): void {
  const errMsg = normalizeRequestMerchantTransferErrMsg(result)
  if (errMsg.includes('cancel')) {
    reject(new RequestMerchantTransferCancelError())
    return
  }
  reject(new Error(errMsg || 'requestMerchantTransfer failed'))
}

function requestMerchantTransfer(params: ClaimPayoutConfirmationResponse): Promise<void> {
  return new Promise((resolve, reject) => {
    const wxWithTransfer = wx as unknown as WxWithMerchantTransfer
    if (typeof wxWithTransfer.requestMerchantTransfer !== 'function') {
      reject(new Error('wx.requestMerchantTransfer is unavailable'))
      return
    }

    wxWithTransfer.requestMerchantTransfer({
      mchId: params.mch_id,
      appId: params.app_id,
      package: params.package,
      success: (res) => {
        const errMsg = normalizeRequestMerchantTransferErrMsg(res)
        if (!errMsg || errMsg.includes('ok')) {
          resolve()
          return
        }
        rejectRequestMerchantTransferResult(res, reject)
      },
      fail: (err) => rejectRequestMerchantTransferResult(err, reject)
    })
  })
}

interface DisplayClaimDetail {
  id: number
  statusText: string
  statusSummary: string
  claimTypeText: string
  claimAmountDisplay: string
  approvedAmountDisplay: string | null
  description: string
  reason: string | null
  orderId: number
  createTimeDisplay: string
  processedAtDisplay: string | null
  payoutEta: string | null
  canConfirmContinue: boolean
  canWithdraw: boolean
  canConfirmPayoutReceipt: boolean
}

function adaptClaimDetail(c: UserClaimResponse): DisplayClaimDetail {
  const presentation = getUserClaimPresentation(c)
  return {
    id: c.id,
    statusText: presentation.statusText,
    statusSummary: presentation.summary,
    claimTypeText: displayClaimType(c.claim_type),

    claimAmountDisplay: formatAmount(c.claim_amount),
    approvedAmountDisplay:
      c.approved_amount !== null && c.approved_amount !== undefined
        ? formatAmount(c.approved_amount)
        : null,
    description: c.description,
    reason: c.reason || null,
    orderId: c.order_id,
    createTimeDisplay: formatDateTime(c.created_at),
    processedAtDisplay: c.processed_at ? formatDateTime(c.processed_at) : null,
    payoutEta: c.payout_eta || null,
    canConfirmContinue: c.customer_action_required === true && c.customer_action === 'confirm_continue',
    canWithdraw: c.customer_action_required === true && c.customer_action === 'confirm_continue',
    canConfirmPayoutReceipt: c.payout_confirmation_required === true && c.payout_confirmation_action === 'request_merchant_transfer'
  }
}

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    isError: false,
    errorMsg: '',
    claim: null as DisplayClaimDetail | null,
    statusIcon: '',
    statusColor: '',
    claimId: 0,
    actionSubmitting: false,
    actionError: ''
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    if (e.detail.navBarHeight !== null && e.detail.navBarHeight !== undefined) {
      this.setData({ navBarHeight: e.detail.navBarHeight })
    }
  },

  onLoad(options: { id?: string }) {
    if (!options.id) {
      this.setData({ isError: true, errorMsg: '缺少工单ID', loading: false })
      return
    }
    this.setData({ claimId: parseInt(options.id) })
    this.loadDetail()
  },

  async loadDetail() {
    this.setData({ loading: true, isError: false })
    try {
      const claim = await claimManagementService.getClaimDetail(this.data.claimId)
      this.applyClaimDetail(claim, { loading: false, actionError: '' })
    } catch (err) {
      logger.error('[ClaimDetail] loadDetail failed', err)
      this.setData({
        loading: false,
        isError: true,
        errorMsg: '加载失败，请稍后重试'
      })
    }
  },

  applyClaimDetail(claim: UserClaimResponse, patch: Record<string, unknown> = {}) {
    const displayClaim = adaptClaimDetail(claim)
    const presentation = getUserClaimPresentation(claim)

    this.setData({
      claim: displayClaim,
      statusIcon: presentation.statusIcon,
      statusColor: presentation.statusColor,
      ...patch
    })
  },

  async onConfirmContinue(): Promise<void> {
    if (this.data.actionSubmitting || !this.data.claim?.canConfirmContinue) return
    const realNameReady = await ensureClaimPayoutRealName('ClaimDetail')
    if (!realNameReady) return

    this.setData({ actionSubmitting: true, actionError: '' })
    for (let attempt = 0; attempt < 2; attempt += 1) {
      try {
        const claim = await claimManagementService.confirmContinueClaim(this.data.claimId)
        this.applyClaimDetail(claim, { actionSubmitting: false })
        return
      } catch (err) {
        logger.error('[ClaimDetail] confirm continue failed', err)
        if (attempt === 0 && isClaimPayoutRealNameRequiredError(err)) {
          this.setData({ actionSubmitting: false })
          const retried = await ensureClaimPayoutRealName('ClaimDetail')
          if (retried) {
            this.setData({ actionSubmitting: true, actionError: '' })
            continue
          }
          return
        }
        this.setData({
          actionSubmitting: false,
          actionError: getErrorUserMessage(err, '当前工单暂不能继续处理，请刷新后重试')
        })
        return
      }
    }
  },

  async onWithdraw() {
    if (this.data.actionSubmitting || !this.data.claim?.canWithdraw) return
    wx.showModal({
      title: '撤回反馈',
      content: '撤回后本次反馈不会继续赔付处理。',
      confirmText: '撤回',
      confirmColor: '#d32f2f',
      success: (res) => {
        if (res.confirm) {
          void this.submitWithdraw()
        }
      }
    })
  },

  async submitWithdraw() {
    this.setData({ actionSubmitting: true, actionError: '' })
    try {
      const claim = await claimManagementService.withdrawClaim(this.data.claimId)
      this.applyClaimDetail(claim, { actionSubmitting: false })
    } catch (err) {
      logger.error('[ClaimDetail] withdraw failed', err)
      this.setData({
        actionSubmitting: false,
        actionError: getErrorUserMessage(err, '当前工单暂不能撤回，请刷新后重试')
      })
    }
  },

  async onConfirmPayoutReceipt(): Promise<void> {
    if (this.data.actionSubmitting || !this.data.claim?.canConfirmPayoutReceipt) return

    this.setData({ actionSubmitting: true, actionError: '' })
    try {
      const params = await claimManagementService.getClaimPayoutConfirmation(this.data.claimId)
      await requestMerchantTransfer(params)
      wx.showToast({ title: '确认页已返回，请等待到账结果', icon: 'none', duration: 2000 })
      await this.loadDetail()
    } catch (err) {
      if (isRequestMerchantTransferCancelError(err)) {
        this.setData({ actionError: '已取消确认收款，可稍后再次确认' })
        return
      }
      logger.error('[ClaimDetail] confirm payout receipt failed', err)
      this.setData({
        actionError: getErrorUserMessage(err, '当前无法拉起微信确认收款，请稍后刷新重试')
      })
    } finally {
      this.setData({ actionSubmitting: false })
    }
  }
})
