import { responsiveBehavior } from '@/utils/responsive'
import { getAdminApprovalStatusDisplay, type AdminApprovalTheme } from '@/adapters/admin-review'
import {
  platformManagementService,
  type AdminOperatorApplicationDetail
} from '@/api/platform-management'
import { getPrivateMediaUrl } from '@/utils/image-security'
import { resolveStatusTagTheme } from '@/utils/status-tag'
import { getErrorUserMessage } from '@/utils/user-facing'

type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>
type ImageTapEvent = WechatMiniprogram.CustomEvent & {
  currentTarget: {
    dataset: {
      url?: string
    }
  }
}
type TapEvent = WechatMiniprogram.CustomEvent & {
  currentTarget: {
    dataset: {
      id?: number | string
      name?: string
    }
  }
}

Page({
  behaviors: [responsiveBehavior],
  data: {
    navBarHeight: 0,
    loading: true,
    requesting: false,
    submittingAction: '' as '' | 'approve' | 'reject',
    error: null as string | null,
    applicationID: 0,
    application: null as AdminOperatorApplicationDetail | null,
    businessLicenseUrl: '',
    idCardFrontUrl: '',
    idCardBackUrl: '',
    idCardPreviewUnavailable: false,
    statusLabel: '',
    statusTheme: resolveStatusTagTheme('info') as AdminApprovalTheme,
    showRejectReason: false,
    canReview: false,
    rejectReason: ''
  },

  onLoad(options: Record<string, string>) {
    const id = Number(options.id || 0)
    if (!id) {
      this.setData({ loading: false, error: '申请ID无效' })
      return
    }

    this.setData({ applicationID: id })
    this.loadDetail()
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 0 })
  },

  async resolvePrivateAssetUrl(assetId?: number): Promise<string> {
    if (!assetId) {
      return ''
    }

    try {
      return await getPrivateMediaUrl(assetId)
    } catch {
      return ''
    }
  },

  async loadDetail() {
    if (this.data.requesting || !this.data.applicationID) {
      return
    }

    this.setData({ loading: true, requesting: true, error: null })
    try {
      const detail = await platformManagementService.getAdminOperatorApplicationDetail(this.data.applicationID)
      const [businessLicenseUrl, idCardFrontUrl, idCardBackUrl] = await Promise.all([
        this.resolvePrivateAssetUrl(detail.business_license_asset_id),
        this.resolvePrivateAssetUrl(detail.id_card_front_asset_id),
        this.resolvePrivateAssetUrl(detail.id_card_back_asset_id)
      ])

      const status = getAdminApprovalStatusDisplay(detail.status, { unknownTheme: 'primary' })
      const hasIDCardAsset = Boolean(detail.id_card_front_asset_id || detail.id_card_back_asset_id)
      const idCardPreviewUnavailable = hasIDCardAsset && !idCardFrontUrl && !idCardBackUrl

      this.setData({
        application: detail,
        businessLicenseUrl,
        idCardFrontUrl,
        idCardBackUrl,
        idCardPreviewUnavailable,
        statusLabel: status.label,
        statusTheme: status.theme,
        showRejectReason: status.isRejected && !!detail.reject_reason,
        canReview: status.isPending
      })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '加载详情失败，请稍后重试')
      this.setData({ error: message })
    } finally {
      this.setData({ loading: false, requesting: false })
    }
  },

  onRetry() {
    this.loadDetail()
  },

  async onApproveTap(e: TapEvent) {
    const id = Number(e.currentTarget.dataset.id || this.data.applicationID || 0)
    if (!id || this.data.submittingAction) return

    const confirm = await new Promise<boolean>((resolve) => {
      wx.showModal({
        title: '通过申请',
        content: '通过后将创建运营商账号并开通对应区域权限。',
        success: (res) => resolve(res.confirm),
        fail: () => resolve(false)
      })
    })
    if (!confirm) return

    try {
      this.setData({ submittingAction: 'approve' })
      await platformManagementService.approveOperatorApplication(id)
      await this.loadDetail()
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '审核失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ submittingAction: '' })
    }
  },

  onRejectReasonChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ rejectReason: e.detail.value || '' })
  },

  async onRejectTap() {
    const id = this.data.applicationID
    const reason = this.data.rejectReason.trim()
    if (!id || this.data.submittingAction) return
    if (!reason) {
      wx.showToast({ title: '请输入驳回原因', icon: 'none' })
      return
    }

    const confirm = await new Promise<boolean>((resolve) => {
      wx.showModal({
        title: '驳回申请',
        content: '确认驳回该申请？',
        success: (res) => resolve(res.confirm),
        fail: () => resolve(false)
      })
    })
    if (!confirm) return

    try {
      this.setData({ submittingAction: 'reject' })
      await platformManagementService.rejectOperatorApplication(id, { reject_reason: reason })
      this.setData({ rejectReason: '' })
      await this.loadDetail()
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '驳回失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ submittingAction: '' })
    }
  },

  onPreviewImage(e: ImageTapEvent) {
    const url = String(e.currentTarget.dataset.url || '').trim()
    if (!url) {
      return
    }
    wx.previewImage({ urls: [url], current: url })
  }
})
