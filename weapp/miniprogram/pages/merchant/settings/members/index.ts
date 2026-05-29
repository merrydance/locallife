import {
  adjustMerchantMemberBalance,
  getMerchantMemberDetail,
  getMyMerchantProfile,
  listMerchantMembers,
  MerchantMemberDetail,
  MerchantMemberSummary,
  MerchantMembershipTransaction
} from '../../../../api/merchant'
import { logger } from '../../../../utils/logger'
import { getMembershipTransactionTagView } from '../../_utils/membership-transaction-view'
import { getStableBarHeights } from '../../../../utils/responsive'
import type { StatusTagTheme } from '../../_main_shared/utils/status-tag'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type AdjustDirection = 'increase' | 'decrease'

interface MemberView extends MerchantMemberSummary {
  display_name: string
  display_initial: string
  joined_at_label: string
  balance_text: string
  recharged_text: string
  consumed_text: string
  phone_text: string
}

interface TransactionView extends MerchantMembershipTransaction {
  type_label: string
  amount_text: string
  balance_after_text: string
  created_at_label: string
  theme: StatusTagTheme
}

interface AdjustFormData {
  direction: AdjustDirection
  amount_yuan: string
  notes: string
}

function formatAmount(amount: number) {
  return `¥${(amount / 100).toFixed(2)}`
}

function formatTime(value?: string) {
  return value ? value.replace('T', ' ').slice(0, 16) : '--'
}

function buildMemberView(item: MerchantMemberSummary): MemberView {
  return {
    ...item,
    display_name: item.full_name || `用户 #${item.user_id}`,
    display_initial: (item.full_name || `用户 #${item.user_id}`).slice(0, 1),
    joined_at_label: formatTime(item.created_at),
    balance_text: formatAmount(item.balance),
    recharged_text: formatAmount(item.total_recharged),
    consumed_text: formatAmount(item.total_consumed),
    phone_text: item.phone || '未留手机号'
  }
}

function buildTransactionView(item: MerchantMembershipTransaction): TransactionView {
  const meta = getMembershipTransactionTagView(item.type)
  const signedPrefix = item.amount > 0 ? '+' : ''
  return {
    ...item,
    type_label: meta.label,
    amount_text: `${signedPrefix}${formatAmount(item.amount)}`,
    balance_after_text: formatAmount(item.balance_after),
    created_at_label: formatTime(item.created_at),
    theme: meta.theme
  }
}

function defaultAdjustForm(): AdjustFormData {
  return {
    direction: 'increase',
    amount_yuan: '',
    notes: ''
  }
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: false,
    loadingMore: false,
    merchantId: 0,
    members: [] as MemberView[],
    pageId: 1,
    pageSize: 20,
    hasMore: true,
    detailVisible: false,
    detailLoading: false,
    selectedMember: null as (MemberView & { transactions: TransactionView[] }) | null,
    adjustVisible: false,
    adjustSubmitting: false,
    adjustTargetUserId: 0,
    adjustTargetName: '',
    adjustForm: defaultAdjustForm()
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.initMerchantId()
  },

  onPullDownRefresh() {
    this.loadMembers(true, false)
  },

  onReachBottom() {
    if (this.data.hasMore && !this.data.loadingMore && !this.data.loading) {
      this.loadMembers(false, true)
    }
  },

  async initMerchantId() {
    try {
      const cached = wx.getStorageSync('current_merchant') as { id?: number, merchant_id?: number } | null
      const cachedMerchantId = Number(cached?.id || cached?.merchant_id || 0)
      if (cachedMerchantId > 0) {
        this.setData({ merchantId: cachedMerchantId })
        await this.loadMembers(true, false)
        return
      }

      const profile = await getMyMerchantProfile()
      this.setData({ merchantId: profile.id })
      await this.loadMembers(true, false)
    } catch (err) {
      logger.error('Init merchant members context failed', err)
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: '获取商户信息失败，请重试'
      })
    }
  },

  async loadMembers(reset: boolean, append: boolean) {
    if (!this.data.merchantId) return
    if (this.data.loading || this.data.loadingMore) return

    const nextPageId = append ? this.data.pageId + 1 : 1
    const hasExistingData = this.data.members.length > 0
    const isSilentRefresh = !reset && !append && hasExistingData

    this.setData({
      ...(append ? { loadingMore: true } : { loading: true }),
      ...(reset
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
        : isSilentRefresh
          ? { refreshErrorMessage: '' }
          : {})
    })

    try {
      const response = await listMerchantMembers(this.data.merchantId, nextPageId, this.data.pageSize)
      const incoming = (response.members || []).map(buildMemberView)
      const members = append ? [...this.data.members, ...incoming] : incoming
      this.setData({
        members,
        pageId: nextPageId,
        hasMore: incoming.length >= this.data.pageSize,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } catch (err) {
      logger.error('Load merchant members failed', err)
      const message = getErrorMessage(err, '会员列表加载失败，请重试')
      if (this.data.initialLoading || reset) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else if (hasExistingData) {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false, loadingMore: false })
      wx.stopPullDownRefresh()
    }
  },

  onRetry() {
    this.loadMembers(true, false)
  },

  onRetryRefresh() {
    this.loadMembers(false, false)
  },

  async onOpenDetail(e: WechatMiniprogram.TouchEvent) {
    const { userId } = e.currentTarget.dataset as { userId?: number }
    if (!userId || !this.data.merchantId) return

    const member = this.data.members.find((item) => item.user_id === userId)
    this.setData({ detailVisible: true, detailLoading: true, selectedMember: member ? { ...member, transactions: [] } : null })

    try {
      const detail: MerchantMemberDetail = await getMerchantMemberDetail(this.data.merchantId, userId)
      this.setData({
        selectedMember: {
          ...buildMemberView(detail),
          transactions: (detail.transactions || []).map(buildTransactionView)
        }
      })
    } catch (err) {
      logger.error('Load merchant member detail failed', err)
      wx.showToast({ title: getErrorMessage(err, '加载会员详情失败，请稍后重试'), icon: 'none' })
      this.setData({ detailVisible: false, selectedMember: null })
    } finally {
      this.setData({ detailLoading: false })
    }
  },

  onCloseDetail() {
    this.setData({ detailVisible: false })
  },

  onCallMember() {
    const phone = this.data.selectedMember?.phone
    if (!phone) {
      wx.showToast({ title: '会员未留手机号', icon: 'none' })
      return
    }
    wx.makePhoneCall({ phoneNumber: phone })
  },

  onOpenAdjustPopup() {
    const member = this.data.selectedMember
    if (!member) return
    this.setData({
      adjustVisible: true,
      adjustTargetUserId: member.user_id,
      adjustTargetName: member.display_name,
      adjustForm: defaultAdjustForm()
    })
  },

  onCloseAdjustPopup() {
    this.setData({ adjustVisible: false })
  },

  onChangeAdjustDirection(e: WechatMiniprogram.TouchEvent) {
    const { direction } = e.currentTarget.dataset as { direction?: AdjustDirection }
    if (!direction) return
    this.setData({ 'adjustForm.direction': direction })
  },

  onAdjustInput(e: WechatMiniprogram.Input) {
    const { field } = e.currentTarget.dataset as { field?: keyof AdjustFormData }
    if (!field) return
    this.setData({ [`adjustForm.${field}`]: e.detail.value })
  },

  async onSubmitAdjust() {
    if (this.data.adjustSubmitting) return

    const amountYuan = Number(this.data.adjustForm.amount_yuan)
    if (!Number.isFinite(amountYuan) || amountYuan <= 0) {
      wx.showToast({ title: '请输入有效的调整金额', icon: 'none' })
      return
    }
    if (!this.data.adjustForm.notes.trim()) {
      wx.showToast({ title: '请填写调整备注', icon: 'none' })
      return
    }

    const signedAmount = Math.round(amountYuan * 100) * (this.data.adjustForm.direction === 'decrease' ? -1 : 1)

    this.setData({ adjustSubmitting: true })
    wx.showLoading({ title: '提交中...' })
    try {
      const updated = await adjustMerchantMemberBalance(this.data.merchantId, this.data.adjustTargetUserId, {
        amount: signedAmount,
        notes: this.data.adjustForm.notes.trim()
      })

      const updatedView = buildMemberView(updated)
      const index = this.data.members.findIndex((item) => item.user_id === updated.user_id)
      if (index >= 0) {
        this.setData({ [`members[${index}]`]: updatedView })
      }

      if (this.data.detailVisible) {
        await this.onOpenDetail({ currentTarget: { dataset: { userId: updated.user_id } } } as unknown as WechatMiniprogram.TouchEvent)
      }

      this.setData({ adjustVisible: false })
    } catch (err) {
      logger.error('Adjust merchant member balance failed', err)
      wx.showToast({ title: getErrorMessage(err, '调整余额失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ adjustSubmitting: false })
      wx.hideLoading()
    }
  }
})