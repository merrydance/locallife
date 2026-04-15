import {
  buildMerchantApplymentStatusView,
  getMerchantApplymentStatus,
  merchantBindBank,
  type MerchantBindBankRequest
} from '../../../../../api/merchant-applyment'
import type { ApplymentBindBankDraftPayload, ApplymentBindBankPayload } from '../../../../../api/applyment-bank'
import {
  ensureMerchantApplymentAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../../../utils/console-access'
import { logger } from '../../../../../utils/logger'
import { getStableBarHeights } from '../../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../../utils/user-facing'

const APPLYMENT_FORCE_REFRESH_STORAGE_KEY = 'merchantApplymentShouldRefresh'

function goBackToApplymentStatus() {
  const pages = getCurrentPages()
  if (pages.length > 1) {
    wx.navigateBack()
    return
  }

  wx.redirectTo({ url: '/pages/merchant/settings/applyment/index' })
}

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessDeniedMessage: '',
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    canSubmit: false,
    blockMessage: '',
    bindBankDraft: null as ApplymentBindBankDraftPayload | null,
    submitting: false
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.bootstrapPage()
  },

  async bootstrapPage() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessDeniedMessage: '',
      accessErrorMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: '',
      canSubmit: false,
      blockMessage: '',
      bindBankDraft: null,
      submitting: false
    })

    const accessResult = await ensureMerchantApplymentAccess()
    if (!isMerchantConsoleAccessGranted(accessResult)) {
      this.setData({
        accessReady: true,
        accessDenied: isMerchantConsoleAccessDenied(accessResult),
        accessDeniedMessage: accessResult.status === 'denied' ? accessResult.message : '',
        accessErrorMessage: getMerchantConsoleAccessErrorMessage(accessResult),
        initialLoading: false
      })
      return
    }

    this.setData({
      accessReady: true,
      accessDenied: false,
      accessDeniedMessage: '',
      accessErrorMessage: ''
    })

    try {
      const applymentStatus = await getMerchantApplymentStatus()
      const applymentView = buildMerchantApplymentStatusView(applymentStatus)

      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        canSubmit: applymentView.canSubmitOpenInfo,
        blockMessage: applymentView.canSubmitOpenInfo
          ? ''
          : (applymentView.blockReason || applymentView.guideDescription || '当前状态暂不支持重新提交进件资料。')
      })
    } catch (error: unknown) {
      logger.error('Load merchant applyment submit page failed', error, 'merchant-applyment-submit-page')
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorUserMessage(error, '进件状态加载失败，请稍后重试')
      })
    }
  },

  onRetryAccess() {
    void this.bootstrapPage()
  },

  onRetry() {
    void this.bootstrapPage()
  },

  onBackToStatus() {
    goBackToApplymentStatus()
  },

  onBindDraftChange(e: WechatMiniprogram.CustomEvent<ApplymentBindBankDraftPayload>) {
    this.setData({ bindBankDraft: e.detail })
  },

  async onSubmitBindBank(e: WechatMiniprogram.CustomEvent<ApplymentBindBankPayload>) {
    if (this.data.submitting) {
      return
    }

    this.setData({ submitting: true })
    wx.showLoading({ title: '提交中...' })

    try {
    const requestPayload: MerchantBindBankRequest = {
      account_type: e.detail.account_type,
      account_bank: e.detail.account_bank,
      account_bank_code: e.detail.account_bank_code,
      bank_alias: e.detail.bank_alias,
      bank_alias_code: e.detail.bank_alias_code,
      need_bank_branch: e.detail.need_bank_branch,
      bank_address_code: e.detail.bank_address_code,
      bank_branch_id: e.detail.bank_branch_id,
      bank_name: e.detail.bank_name,
      account_number: e.detail.account_number,
      account_name: String(e.detail.account_name || '').trim(),
      contact_type: e.detail.contact_type,
      contact_name: e.detail.contact_name,
      contact_id_doc_type: e.detail.contact_id_doc_type,
      contact_id_card_number: e.detail.contact_id_card_number,
      contact_id_doc_copy_asset_id: e.detail.contact_id_doc_copy_asset_id,
      contact_id_doc_copy_back_asset_id: e.detail.contact_id_doc_copy_back_asset_id,
      contact_id_doc_period_begin: e.detail.contact_id_doc_period_begin,
      contact_id_doc_period_end: e.detail.contact_id_doc_period_end
    }

    await merchantBindBank(requestPayload)
      wx.setStorageSync(APPLYMENT_FORCE_REFRESH_STORAGE_KEY, '1')
      wx.showToast({ title: '进件资料已提交', icon: 'success' })
      goBackToApplymentStatus()
    } catch (error: unknown) {
      logger.error('Submit merchant applyment bind bank failed', error, 'merchant-applyment-submit-page')
      wx.showToast({ title: getErrorUserMessage(error, '提交进件资料失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ submitting: false })
    }
  }
})