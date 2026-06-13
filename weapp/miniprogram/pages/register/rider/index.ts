import {
  buildRiderApplicationStatusView,
  getOrCreateRiderApplication,
  patchRiderHealthCertOCRFields,
  updateRiderApplicationBasic,
  submitRiderApplication,
  type RiderApplicationResponse
} from './_api/rider-application'
import { getPrivateMediaUrl } from '../_main_shared/utils/image-security'
import { logger } from '../../../utils/logger'
import Navigation from '../../../utils/navigation'
import { buildAgreementConsentPayload } from '../_main_shared/api/agreement-consent'
import {
  buildActiveCredentialDisplays,
  buildOnboardingReviewDisplay,
  type ApplicationStatus
} from '../_main_shared/api/onboarding'
import {
  getErrorDebugMessage,
  getErrorUserMessage
} from '../../../utils/user-facing'
import {
  DEFAULT_RIDER_OCR_DISPLAY_STATE,
  DEFAULT_RIDER_OCR_PANEL_STATE,
  DEFAULT_RIDER_UPLOAD_FEEDBACK,
  hasRiderUploadAssetId,
  isDocumentCorrectionError,
  isRejectedRiderApplication,
  type UploadField,
  type UploadFieldValue
} from './_utils/rider-register-view'
import {
  buildRiderApplicationResponsePatch,
  buildRiderDocumentDeleteLocalPatch,
  buildRiderDocumentOCRFailurePatch,
  buildRiderDocumentResponsePatch,
  createRiderDocumentOCRWorkflow,
  deleteRiderDocumentByField,
  type RiderDocumentOCRWorkflow
} from './_utils/rider-application-document-workflow'

type UploadEvent = WechatMiniprogram.CustomEvent<{ path?: string }>

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    currentStep: 0,
    isSubmitting: false,
    isSavingHealthCertCorrection: false,
    idFront: { url: '', assetId: undefined } as UploadFieldValue,
    idBack: { url: '', assetId: undefined } as UploadFieldValue,
    healthCert: { url: '', assetId: undefined } as UploadFieldValue,
    ocrDisplayState: DEFAULT_RIDER_OCR_DISPLAY_STATE,
    uploadFeedback: DEFAULT_RIDER_UPLOAD_FEEDBACK,
    formData: {
      realName: '',
      phone: '',
      idNumber: '',
      idValidity: '',
      healthCertNo: '',
      healthCertDate: ''
    },
    phoneError: '',
    consentChecked: false,
    consentPopupVisible: false,
    applicationStatus: 'draft' as ApplicationStatus,
    riderStatusView: buildRiderApplicationStatusView('draft'),
    ocrPanelState: DEFAULT_RIDER_OCR_PANEL_STATE,
    reviewDisplay: buildOnboardingReviewDisplay(null, 'draft'),
    activeCredentialDisplays: buildActiveCredentialDisplays([])
  },

  previewRefreshVersion: 0,
  documentRequestVersion: {
    idFront: 0,
    idBack: 0,
    healthCert: 0
  } as Record<UploadField, number>,

  async onLoad() {
    await this.initApplication()
  },

  onShow() {
    if (this.data.riderStatusView.isSubmitted) {
      void this.initApplication()
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async initApplication() {
    wx.showLoading({ title: '加载中...' })
    try {
      const res = await getOrCreateRiderApplication()
      if (res) {
        this.mapResponseToData(res)
        const statusView = buildRiderApplicationStatusView(res.status)

        if (statusView.isSubmitted) {
          this.setData({ currentStep: 4, isSubmitting: false })
        } else if (statusView.isApproved) {
          wx.showModal({
            title: '审核通过',
            content: '恭喜！您已正式成为 LocalLife 骑手。',
            showCancel: false,
            success: () => wx.reLaunch({ url: '/pages/rider/dashboard/index' })
          })
        } else if (isRejectedRiderApplication(res)) {
          this.showRejectedApplicationModal(res)
        }
      }
    } catch (e) {
      logger.error('Init rider application failed', e)
    } finally {
      wx.hideLoading()
    }
  },

  mapResponseToData(res: RiderApplicationResponse) {
    this.setData(
      buildRiderApplicationResponsePatch(res, {
        formData: this.data.formData,
        phoneError: this.data.phoneError,
        currentStep: this.data.currentStep,
        idFront: this.data.idFront,
        idBack: this.data.idBack,
        healthCert: this.data.healthCert
      }),
      () => {
        void this.refreshUploadPreviewURLs()
      }
    )
  },

  showRejectedApplicationModal(res: RiderApplicationResponse) {
    this.setData({
      currentStep: 1,
      isSubmitting: false,
      applicationStatus: 'draft',
      riderStatusView: buildRiderApplicationStatusView('draft')
    })
    wx.showModal({
      title: '申请未通过',
      content: res.reject_reason || '资料核验未通过，请修改后重新提交。',
      confirmText: '修改资料',
      cancelText: '返回',
      success: (modalRes) => {
        if (!modalRes.confirm) {
          wx.navigateBack()
        }
      }
    })
  },

  beginDocumentRequest(field: UploadField): number {
    const nextVersion = (this.documentRequestVersion[field] || 0) + 1
    this.documentRequestVersion[field] = nextVersion
    return nextVersion
  },

  isLatestDocumentRequest(field: UploadField, version: number): boolean {
    return this.documentRequestVersion[field] === version
  },

  applyDocumentResponse(
    field: UploadField,
    version: number,
    res: RiderApplicationResponse
  ) {
    if (!this.isLatestDocumentRequest(field, version)) {
      return
    }

    this.setData(
      buildRiderDocumentResponsePatch(field, res, {
        formData: this.data.formData,
        idFront: this.data.idFront,
        idBack: this.data.idBack,
        healthCert: this.data.healthCert
      }),
      () => {
        void this.refreshUploadPreviewURLs()
      }
    )
  },

  isApplicationEditable() {
    return this.data.riderStatusView.isDraft
  },

  ensureApplicationEditable() {
    if (this.isApplicationEditable()) {
      return true
    }

    let message = '当前申请状态暂不支持修改资料'
    if (this.data.riderStatusView.isSubmitted) {
      message = '申请已提交，暂时不能修改资料'
      this.setData({ currentStep: 4, isSubmitting: false })
    } else if (this.data.riderStatusView.isApproved) {
      message = '入驻已通过，无需重复上传资料'
    } else if (this.data.riderStatusView.isRejected) {
      message = '申请已驳回，请先重置后再修改资料'
    }

    wx.showToast({ title: message, icon: 'none' })
    return false
  },

  async resolveUploadPreviewURL(assetId?: number): Promise<string> {
    if (assetId && assetId > 0) {
      try {
        return await getPrivateMediaUrl(assetId)
      } catch (_e) {
        return ''
      }
    }

    return ''
  },

  async refreshUploadPreviewURLs() {
    const refreshVersion = ++this.previewRefreshVersion
    const uploads: Array<{
      key: 'idFront' | 'idBack' | 'healthCert'
      value: UploadFieldValue
    }> = [
      { key: 'idFront', value: this.data.idFront },
      { key: 'idBack', value: this.data.idBack },
      { key: 'healthCert', value: this.data.healthCert }
    ]

    for (const item of uploads) {
      const assetId = item.value?.assetId
      if (!assetId) continue

      const resolved = await this.resolveUploadPreviewURL(assetId)
      const latestValue = this.data[item.key] as UploadFieldValue | undefined
      if (
        refreshVersion === this.previewRefreshVersion &&
        latestValue?.assetId === assetId &&
        resolved &&
        resolved !== latestValue?.url
      ) {
        this.setData({ [`${item.key}.url`]: resolved })
      }
    }
  },

  async onIdFrontUpload(e: UploadEvent) {
    await this.handleDocumentUpload('idFront', e.detail.path)
  },

  async onIdBackUpload(e: UploadEvent) {
    await this.handleDocumentUpload('idBack', e.detail.path)
  },

  async onHealthCertUpload(e: UploadEvent) {
    await this.handleDocumentUpload('healthCert', e.detail.path)
  },

  async handleDocumentUpload(field: UploadField, path?: string) {
    if (!this.ensureApplicationEditable()) return
    if (!path) return

    await this.processOCR(createRiderDocumentOCRWorkflow(field, path))
  },

  async removeUploadedDocument(field: UploadField) {
    if (!this.ensureApplicationEditable()) return
    const requestVersion = this.beginDocumentRequest(field)

    wx.showLoading({ title: '删除中...' })
    try {
      const res = await deleteRiderDocumentByField(field)
      if (!this.isLatestDocumentRequest(field, requestVersion)) {
        return
      }
      this.setData(buildRiderDocumentDeleteLocalPatch(field), () => {
        this.applyDocumentResponse(field, requestVersion, res)
      })
    } catch (e) {
      logger.error('Delete rider application document failed', {
        field,
        error: e
      })
      wx.showToast({
        title: getErrorMessage(e, '删除失败，请重试'),
        icon: 'none'
      })
    } finally {
      wx.hideLoading()
    }
  },

  onIdFrontRemove() {
    this.removeUploadedDocument('idFront')
  },

  onIdBackRemove() {
    this.removeUploadedDocument('idBack')
  },

  onHealthCertRemove() {
    this.removeUploadedDocument('healthCert')
  },

  async processOCR(workflow: RiderDocumentOCRWorkflow) {
    const requestVersion = this.beginDocumentRequest(workflow.field)
    this.setData(workflow.startPatch)
    try {
      const res = await workflow.run()
      this.applyDocumentResponse(workflow.field, requestVersion, res)
    } catch (e) {
      logger.error('OCR failed', e)
      const message = getErrorMessage(
        e,
        '识别失败，请提供更清晰更规整的图片重试'
      )
      this.setData(
        buildRiderDocumentOCRFailurePatch(
          workflow.displayType,
          workflow.feedbackField,
          message
        )
      )
    }
  },

  async saveHealthCertOCRCorrection() {
    const healthCertDate = this.data.formData.healthCertDate.trim()
    if (!this.data.healthCert.assetId || !healthCertDate) {
      return
    }

    this.setData({ isSavingHealthCertCorrection: true })
    try {
      const res = await patchRiderHealthCertOCRFields({
        cert_number: this.data.formData.healthCertNo.trim(),
        valid_end: healthCertDate
      })
      this.mapResponseToData(res)
    } finally {
      this.setData({ isSavingHealthCertCorrection: false })
    }
  },

  onInput(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    const field = (e.currentTarget.dataset as { field?: string }).field
    if (!field) return

    const value = e.detail.value || ''
    const nextData: Record<string, string> = {
      [`formData.${field}`]: value
    }
    if (field === 'phone' && value.trim()) {
      nextData.phoneError = ''
    }
    this.setData(nextData)
  },

  onPrev() {
    this.setData({ currentStep: this.data.currentStep - 1 })
  },

  onConsentChange(e: WechatMiniprogram.CustomEvent<{ value?: string[] }>) {
    const values = e.detail.value || []
    this.setData({ consentChecked: values.includes('agree') })
  },

  openConsentPopup() {
    this.setData({ consentPopupVisible: true })
  },

  closeConsentPopup() {
    this.setData({ consentPopupVisible: false })
  },

  onConfirmConsent() {
    this.setData({ consentChecked: true, consentPopupVisible: false })
  },

  onViewAgreement(
    e: WechatMiniprogram.CustomEvent<{ type?: string, title?: string }>
  ) {
    const type = (e.currentTarget.dataset as { type?: string }).type
    const title = (e.currentTarget.dataset as { title?: string }).title
    if (!type) return
    Navigation.toAgreementDetail(type, title)
  },

  ensureConsent(): boolean {
    if (this.data.consentChecked) return true
    this.openConsentPopup()
    wx.showToast({ title: '请先阅读并同意协议', icon: 'none' })
    return false
  },

  async onNext() {
    const { currentStep, idFront, idBack, healthCert, formData } = this.data

    if (currentStep === 0 && !this.ensureConsent()) {
      return
    }

    if (currentStep === 1) {
      if (
        !hasRiderUploadAssetId(idFront.assetId) ||
        !hasRiderUploadAssetId(idBack.assetId) ||
        !hasRiderUploadAssetId(healthCert.assetId)
      ) {
        return wx.showToast({ title: '请上传所有必需证照', icon: 'none' })
      }
    }

    if (currentStep === 2) {
      const realName = formData.realName.trim()
      const phone = formData.phone.trim()

      if (!realName) {
        return wx.showToast({ title: '请确认真实姓名', icon: 'none' })
      }
      if (!phone) {
        this.setData({ phoneError: '请填写联系电话，方便平台与你联系' })
        return wx.showToast({ title: '请填写联系电话', icon: 'none' })
      }

      this.setData({ phoneError: '' })
      // 同步基础信息
      wx.showLoading({ title: '保存信息...' })
      try {
        await updateRiderApplicationBasic({
          real_name: realName,
          phone
        })
        await this.saveHealthCertOCRCorrection()
      } catch (e) {
        logger.error('Update rider application confirmation failed', e)
        wx.showToast({
          title: getErrorMessage(e, '保存信息失败，请重试'),
          icon: 'none'
        })
        return
      } finally {
        wx.hideLoading()
      }
    }

    this.setData({ currentStep: currentStep + 1 })
  },

  async onSubmit() {
    if (!this.ensureConsent()) {
      return
    }

    let consentPayload
    try {
      consentPayload = await buildAgreementConsentPayload()
    } catch (e: unknown) {
      wx.showToast({
        title: getErrorMessage(e, '协议信息加载失败，请稍后重试'),
        icon: 'none'
      })
      return
    }

    this.setData({ isSubmitting: true, currentStep: 4 })
    try {
      await this.saveHealthCertOCRCorrection()
      const res = await submitRiderApplication(consentPayload)

      this.mapResponseToData(res)

      const statusView = buildRiderApplicationStatusView(res.status)
      if (statusView.isApproved) {
        wx.showModal({
          title: '审核通过',
          content: '恭喜！您已正式成为 LocalLife 骑手。',
          showCancel: false,
          success: () => wx.reLaunch({ url: '/pages/rider/dashboard/index' })
        })
        return
      }

      if (isRejectedRiderApplication(res)) {
        this.showRejectedApplicationModal(res)
        return
      }

      if (statusView.isSubmitted) {
        wx.showToast({
          title: '已提交审核，可稍后回来查看',
          icon: 'none'
        })
      }
    } catch (e: unknown) {
      const message = getErrorMessage(e, '提交失败')
      const debugMessage = getErrorDebugMessage(e)
      logger.error(
        '[RiderRegister] Submit failed',
        {
          error: e,
          userMessage: message,
          debugMessage
        },
        'rider-register-submit'
      )
      const shouldReturnToEdit = isDocumentCorrectionError(message)
      this.setData({
        isSubmitting: false,
        currentStep: shouldReturnToEdit ? 2 : 3
      })
      wx.showModal({
        title: shouldReturnToEdit ? '请修改资料后重试' : '提交失败',
        content: message,
        showCancel: false,
        success: () => {
          if (shouldReturnToEdit) {
            this.setData({ currentStep: 1 })
          }
        }
      })
    }
  }
})
