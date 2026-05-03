import {
  merchantBindBank,
  type ApplymentStatusResponse,
  type MerchantBindBankResponse,
  type MerchantBindBankRequest
} from '../../../../../api/merchant-applyment'
import {
  getMerchantApplication,
  getMyApplication,
  type MerchantApplicationDraftResponse
} from '../../../../../api/onboarding'
import type { ApplymentBindBankDraftPayload, ApplymentBindBankPayload } from '../../../../../api/applyment-bank'
import {
  buildMerchantApplymentWorkflowView,
  fetchMerchantApplymentWorkflowView,
  type MerchantApplymentWorkflowView
} from '../../../../../services/merchant-applyment-workflow'
import {
  ensureMerchantApplymentAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../../../utils/console-access'
import Toast, { hideToast } from '../../../../../miniprogram_npm/tdesign-miniprogram/toast/index'
import { logger } from '../../../../../utils/logger'
import { getStableBarHeights } from '../../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../../utils/user-facing'
import { shouldFallbackToLatestApplication } from '../../../../../utils/merchant-application-view'

const APPLYMENT_FORCE_REFRESH_STORAGE_KEY = 'merchantApplymentShouldRefresh'
const APPLYMENT_PERMISSION_RESTRICTED_CODE = 40363
const APPLYMENT_SUBMIT_STATUS_POLL_INTERVAL_MS = 3000
const APPLYMENT_SUBMIT_STATUS_POLL_TIMEOUT_MS = 27000
const EMPTY_WORKFLOW_VIEW = buildMerchantApplymentWorkflowView(null)
const TOAST_SELECTOR = '#t-toast'

interface SubjectSummary {
  merchantName: string
  businessLicenseNumber: string
  legalPersonName: string
  contactPhone: string
}

const EMPTY_SUBJECT_SUMMARY: SubjectSummary = {
  merchantName: '-',
  businessLicenseNumber: '-',
  legalPersonName: '-',
  contactPhone: '-'
}

function goBackToApplymentStatus() {
  const pages = getCurrentPages()
  if (pages.length > 1) {
    wx.navigateBack()
    return
  }

  wx.redirectTo({ url: '/pages/merchant/settings/applyment/index' })
}

function redirectToApplymentEntry() {
  wx.redirectTo({ url: '/pages/merchant/settings/applyment/index' })
}

function buildStatusResponseFromBindBank(response: MerchantBindBankResponse): ApplymentStatusResponse {
  return {
    status: response.status,
    status_desc: response.status_desc || response.message || '',
    sign_url: response.sign_url,
    sign_state: response.sign_state,
    legal_validation_url: response.legal_validation_url,
    account_validation: response.account_validation,
    sub_mch_id: response.sub_mch_id,
    reject_reason: response.reject_reason
  }
}

function resolveSubmittedWorkflowPath(workflowView: MerchantApplymentWorkflowView) {
  if (workflowView.currentStage === 'action_required' && workflowView.primaryActionIntent === 'navigate' && workflowView.primaryActionPath) {
    return workflowView.primaryActionPath
  }

  return '/pages/merchant/settings/applyment/index'
}

function showSubmitLoadingToast(context: WechatMiniprogram.Page.TrivialInstance) {
  Toast({
    context,
    selector: TOAST_SELECTOR,
    message: '提交中...',
    theme: 'loading',
    direction: 'column',
    duration: 0,
    preventScrollThrough: true
  })
}

function showSubmitSyncingToast(context: WechatMiniprogram.Page.TrivialInstance) {
  Toast({
    context,
    selector: TOAST_SELECTOR,
    message: '已提交，正在同步微信待办...',
    theme: 'loading',
    direction: 'column',
    duration: 0,
    preventScrollThrough: true
  })
}

function hideSubmitToast(context: WechatMiniprogram.Page.TrivialInstance) {
  hideToast({ context, selector: TOAST_SELECTOR })
}

function showSubmitResultToast(
  context: WechatMiniprogram.Page.TrivialInstance,
  message: string,
  theme: 'success' | 'warning' | 'error',
  close?: () => void
) {
  Toast({
    context,
    selector: TOAST_SELECTOR,
    message,
    theme,
    direction: 'column',
    duration: 1800,
    close
  })
}

function isApplymentPermissionRestrictedError(error: unknown): boolean {
  if (!error || typeof error !== 'object') {
    return false
  }

  const knownError = error as {
    code?: string | number
    message?: string
    detailMessage?: string
    userMessage?: string
  }

  if (knownError.code === APPLYMENT_PERMISSION_RESTRICTED_CODE || knownError.code === String(APPLYMENT_PERMISSION_RESTRICTED_CODE)) {
    return true
  }

  const candidates = [knownError.message, knownError.detailMessage, knownError.userMessage]
  const applymentName = '进' + '件'
  const specialMerchantName = '特约' + '商户'
  return candidates.some((candidate) => {
    if (typeof candidate !== 'string') {
      return false
    }

    return candidate.includes('permission is not enabled for the current platform merchant') ||
      candidate.includes(`${applymentName}${specialMerchantName}的权限已被受限`) ||
      candidate.includes('NO_AUTH')
  })
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function shouldPollSubmittedApplymentStatus(workflowView: MerchantApplymentWorkflowView): boolean {
  return workflowView.currentStage === 'reviewing' || workflowView.resultState === 'processing'
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
    workflowView: { ...EMPTY_WORKFLOW_VIEW },
    subjectSummary: { ...EMPTY_SUBJECT_SUMMARY } as SubjectSummary,
    bindBankDraft: null as ApplymentBindBankDraftPayload | null,
    submitErrorMessage: '',
    submitting: false
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.bootstrapPage()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
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
      workflowView: { ...EMPTY_WORKFLOW_VIEW },
      subjectSummary: { ...EMPTY_SUBJECT_SUMMARY },
      bindBankDraft: null,
      submitErrorMessage: '',
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
      const [workflowView, application] = await Promise.all([
        fetchMerchantApplymentWorkflowView(),
        this.fetchCurrentApplication()
      ])
      const canSubmit = workflowView.currentTask.type === 'submit_material' || workflowView.currentTask.type === 'resubmit_after_reject'

      if (!canSubmit) {
        redirectToApplymentEntry()
        return
      }

      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        workflowView,
        subjectSummary: this.buildSubjectSummary(application),
        canSubmit: true,
        blockMessage: ''
      })
    } catch (error: unknown) {
      logger.error('Load merchant applyment submit page failed', error, 'merchant-applyment-submit-page')
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorUserMessage(error, '开户状态加载失败，请稍后重试')
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

  async fetchCurrentApplication() {
    try {
      return await getMerchantApplication()
    } catch (error: unknown) {
      if (shouldFallbackToLatestApplication(error)) {
        return await getMyApplication()
      }
      throw error
    }
  },

  buildSubjectSummary(application: MerchantApplicationDraftResponse): SubjectSummary {
    return {
      merchantName: application.merchant_name || '-',
      businessLicenseNumber: application.business_license_number || '-',
      legalPersonName: application.legal_person_name || '-',
      contactPhone: application.contact_phone || '-'
    }
  },

  onBindDraftChange(e: WechatMiniprogram.CustomEvent<ApplymentBindBankDraftPayload>) {
    this.setData({ bindBankDraft: e.detail })
  },

  async pollSubmittedApplymentStatus(initialWorkflowView: MerchantApplymentWorkflowView): Promise<MerchantApplymentWorkflowView> {
    let latestWorkflowView = initialWorkflowView
    if (!shouldPollSubmittedApplymentStatus(latestWorkflowView)) {
      return latestWorkflowView
    }

    const deadlineAt = Date.now() + APPLYMENT_SUBMIT_STATUS_POLL_TIMEOUT_MS
    while (Date.now() < deadlineAt) {
      await sleep(APPLYMENT_SUBMIT_STATUS_POLL_INTERVAL_MS)
      try {
        latestWorkflowView = await fetchMerchantApplymentWorkflowView()
        if (!shouldPollSubmittedApplymentStatus(latestWorkflowView)) {
          return latestWorkflowView
        }
      } catch (error: unknown) {
        logger.warn('Poll merchant applyment status after submit failed', error, 'merchant-applyment-submit-page')
      }
    }

    return latestWorkflowView
  },

  redirectAfterSubmittedWorkflow(workflowView: MerchantApplymentWorkflowView) {
    wx.setStorageSync(APPLYMENT_FORCE_REFRESH_STORAGE_KEY, '1')
    const redirectPath = resolveSubmittedWorkflowPath(workflowView)
    if (workflowView.currentStage === 'action_required') {
      wx.redirectTo({ url: redirectPath })
      return
    }

    showSubmitResultToast(
      this,
      '开户资料已提交，平台正在同步微信待办。',
      'success',
      () => wx.redirectTo({ url: redirectPath })
    )
  },

  async onSubmitBindBank(e: WechatMiniprogram.CustomEvent<ApplymentBindBankPayload>) {
    if (this.data.submitting) {
      return
    }

    this.setData({ submitting: true, submitErrorMessage: '' })
    showSubmitLoadingToast(this)

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
        contact_email: String(e.detail.contact_email || '').trim(),
        contact_type: e.detail.contact_type,
        contact_name: e.detail.contact_name,
        contact_id_doc_type: e.detail.contact_id_doc_type,
        contact_id_card_number: e.detail.contact_id_card_number,
        contact_id_doc_copy_asset_id: e.detail.contact_id_doc_copy_asset_id,
        contact_id_doc_copy_back_asset_id: e.detail.contact_id_doc_copy_back_asset_id,
        contact_id_doc_period_begin: e.detail.contact_id_doc_period_begin,
        contact_id_doc_period_end: e.detail.contact_id_doc_period_end
      }

      const response = await merchantBindBank(requestPayload)
      const initialWorkflowView = buildMerchantApplymentWorkflowView(buildStatusResponseFromBindBank(response))
      showSubmitSyncingToast(this)
      const workflowView = await this.pollSubmittedApplymentStatus(initialWorkflowView)
      hideSubmitToast(this)
      this.redirectAfterSubmittedWorkflow(workflowView)
    } catch (error: unknown) {
      logger.error('Submit merchant applyment bind bank failed', error, 'merchant-applyment-submit-page')
      hideSubmitToast(this)
      if (isApplymentPermissionRestrictedError(error)) {
        this.setData({ submitErrorMessage: '开户申请提交失败，请联系平台处理。' })
        return
      }
      showSubmitResultToast(this, getErrorUserMessage(error, '提交开户资料失败，请稍后重试'), 'warning')
    } finally {
      this.setData({ submitting: false })
    }
  }
})
