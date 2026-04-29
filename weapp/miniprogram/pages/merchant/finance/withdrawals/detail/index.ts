import {
  getMerchantCancelWithdrawEligibility,
  getMerchantWithdrawal,
  listMerchantCancelWithdrawApplications,
  uploadMerchantCancelWithdrawMaterial
} from '../../../../../api/merchant-finance'
import {
  buildMerchantCancelWithdrawPayload,
  buildCancelWithdrawApplicationView,
  buildCancelWithdrawEligibilityView,
  buildWithdrawalView,
  getMerchantFinanceUserMessage,
  submitMerchantCancelWithdrawAndWait,
  type MerchantCancelWithdrawApplicationView,
  type MerchantCancelWithdrawSubmitDraft,
  type MerchantCancelWithdrawEligibilityView,
  type MerchantWithdrawalView
} from '../../../../../services/merchant-finance-workflow'
import { logger } from '../../../../../utils/logger'
import { getStableBarHeights } from '../../../../../utils/responsive'

interface UploadedMaterialView {
  assetId: number
  url: string
  name: string
  type: 'image'
  status: 'done'
}

const EMPTY_CANCEL_FORM: MerchantCancelWithdrawSubmitDraft = {
  withdraw: 'NOT_APPLY_WITHDRAW',
  businessLicenseStatusDeclaration: '',
  accountType: 'ACCOUNT_TYPE_CORPORATE',
  accountName: '',
  accountBank: '',
  bankBranchId: '',
  bankBranchName: '',
  accountNumber: '',
  idDocType: 'IDENTIFICATION_TYPE_ID_CARD',
  identificationName: '',
  identificationNo: '',
  proofMediaAssetIds: [],
  additionalMaterialAssetIds: [],
  remark: ''
}

const WITHDRAW_MODE_OPTIONS = [
  { label: '不提现注销', value: 'NOT_APPLY_WITHDRAW' },
  { label: '提现后注销', value: 'APPLY_WITHDRAW' }
]

const LICENSE_STATUS_OPTIONS = [
  { label: '营业执照正常', value: 'ACTIVE' },
  { label: '营业执照已注销', value: 'CANCELED' },
  { label: '营业执照已吊销', value: 'REVOKED' }
]

const ACCOUNT_TYPE_OPTIONS = [
  { label: '企业账户', value: 'ACCOUNT_TYPE_CORPORATE' },
  { label: '个人账户', value: 'ACCOUNT_TYPE_PERSONAL' }
]

const ID_DOC_TYPE_OPTIONS = [
  { label: '居民身份证', value: 'IDENTIFICATION_TYPE_ID_CARD' },
  { label: '外国人居留证', value: 'IDENTIFICATION_TYPE_FOREIGN_RESIDENT' },
  { label: '港澳居民来往内地通行证', value: 'IDENTIFICATION_TYPE_HONGKONG_MACAO_RESIDENT' },
  { label: '台湾居民来往大陆通行证', value: 'IDENTIFICATION_TYPE_TAIWAN_RESIDENT' },
  { label: '护照', value: 'IDENTIFICATION_TYPE_OVERSEA_PASSPORT' }
]

function getInputValue(detail: unknown): string {
  if (typeof detail === 'string' || typeof detail === 'number') {
    return String(detail)
  }
  if (detail && typeof detail === 'object' && 'value' in detail) {
    return String((detail as { value?: unknown }).value || '')
  }
  return ''
}

function getRadioValue(detail: unknown): string {
  return getInputValue(detail)
}

function buildCancelForm(hasFundsToWithdraw: boolean): MerchantCancelWithdrawSubmitDraft {
  return {
    ...EMPTY_CANCEL_FORM,
    withdraw: hasFundsToWithdraw ? 'APPLY_WITHDRAW' : 'NOT_APPLY_WITHDRAW',
    businessLicenseStatusDeclaration: hasFundsToWithdraw ? 'ACTIVE' : ''
  }
}

function getUploadFiles(detail: unknown): Array<{ url?: string, name?: string }> {
  const value = detail && typeof detail === 'object'
    ? ((detail as { files?: unknown, currentSelectedFiles?: unknown }).currentSelectedFiles || (detail as { files?: unknown }).files)
    : detail
  if (!Array.isArray(value)) {
    return []
  }
  return value.map((item) => {
    if (!item || typeof item !== 'object') {
      return {}
    }
    const file = item as { url?: string, tempFilePath?: string, path?: string, name?: string }
    return { url: file.url || file.tempFilePath || file.path || '', name: file.name || '材料图片' }
  })
}

Page({
  data: {
    navBarHeight: 88,
    id: 0,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loadingDetail: false,
    cancelSubmitting: false,
    cancelDialogVisible: false,
    withdrawal: null as MerchantWithdrawalView | null,
    cancelEligibility: null as MerchantCancelWithdrawEligibilityView | null,
    cancelApplications: [] as MerchantCancelWithdrawApplicationView[],
    canSubmitCancelWithdraw: false,
    cancelWithdrawUnavailableNote: '',
    hasFundsToWithdraw: false,
    cancelForm: { ...EMPTY_CANCEL_FORM },
    withdrawModeOptions: WITHDRAW_MODE_OPTIONS,
    licenseStatusOptions: LICENSE_STATUS_OPTIONS,
    accountTypeOptions: ACCOUNT_TYPE_OPTIONS,
    idDocTypeOptions: ID_DOC_TYPE_OPTIONS,
    uploadMediaType: ['image'] as string[],
    uploadGridConfig: { column: 4, width: 150, height: 150 },
    proofMediaFiles: [] as UploadedMaterialView[],
    additionalMaterialFiles: [] as UploadedMaterialView[],
    uploadingProofMedia: false,
    uploadingAdditionalMaterial: false,
    cancelFormErrorMessage: '',
    cancelSyncMessage: '',
    showApplyWithdrawForm: false,
    showPersonalIdentityForm: false,
    proofMediaRequired: false
  },

  async onLoad(query: { id?: string }) {
    const { navBarHeight } = getStableBarHeights()
    const id = Number(query.id)
    this.setData({ navBarHeight, id: Number.isFinite(id) ? id : 0 })
    await this.loadDetail()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onPullDownRefresh() {
    void this.loadDetail({ silent: true })
  },

  async loadDetail(options: { silent?: boolean } = {}) {
    if (this.data.loadingDetail) {
      wx.stopPullDownRefresh()
      return
    }
    if (!this.data.id) {
      this.setData({ initialLoading: false, initialError: true, initialErrorMessage: '提现记录不存在' })
      wx.stopPullDownRefresh()
      return
    }

    const { silent = false } = options
    const hasTrustedData = !!this.data.withdrawal
    this.setData({
      loadingDetail: true,
      ...(silent || hasTrustedData ? { refreshErrorMessage: '' } : { initialLoading: true, initialError: false, initialErrorMessage: '', refreshErrorMessage: '' })
    })

    try {
      const [withdrawal, eligibility, applications] = await Promise.all([
        getMerchantWithdrawal(this.data.id),
        getMerchantCancelWithdrawEligibility(),
        listMerchantCancelWithdrawApplications(1, 3)
      ])
      const eligibilityView = buildCancelWithdrawEligibilityView(eligibility)
      const hasFundsToWithdraw = (eligibility.eligibility?.account_info || []).some((item) => item.amount > 0)
      const shouldResetCancelForm = !hasTrustedData || !this.data.canSubmitCancelWithdraw || this.data.hasFundsToWithdraw !== hasFundsToWithdraw
      const cancelForm = shouldResetCancelForm ? buildCancelForm(hasFundsToWithdraw) : this.data.cancelForm
      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        loadingDetail: false,
        withdrawal: buildWithdrawalView(withdrawal),
        cancelEligibility: eligibilityView,
        cancelApplications: applications.applications.map(buildCancelWithdrawApplicationView),
        hasFundsToWithdraw,
        canSubmitCancelWithdraw: eligibilityView.eligible,
        cancelWithdrawUnavailableNote: eligibilityView.eligible ? '' : eligibilityView.blockReasonText,
        cancelForm,
        proofMediaFiles: shouldResetCancelForm ? [] : this.data.proofMediaFiles,
        additionalMaterialFiles: shouldResetCancelForm ? [] : this.data.additionalMaterialFiles,
        cancelFormErrorMessage: '',
        cancelSyncMessage: '',
        showApplyWithdrawForm: cancelForm.withdraw === 'APPLY_WITHDRAW',
        showPersonalIdentityForm: cancelForm.accountType === 'ACCOUNT_TYPE_PERSONAL',
        proofMediaRequired: cancelForm.businessLicenseStatusDeclaration === 'CANCELED' || cancelForm.businessLicenseStatusDeclaration === 'REVOKED'
      })
    } catch (error) {
      logger.warn('Merchant withdrawal detail load failed', error)
      const message = getMerchantFinanceUserMessage(error, '提现详情加载失败，请稍后重试')
      this.setData({ initialLoading: false, initialError: !hasTrustedData, initialErrorMessage: hasTrustedData ? '' : message, refreshErrorMessage: hasTrustedData ? message : '', loadingDetail: false })
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  onRetry() { void this.loadDetail() },
  onOpenCancelDialog() {
    if (!this.data.canSubmitCancelWithdraw || this.data.cancelSubmitting || this.data.uploadingProofMedia || this.data.uploadingAdditionalMaterial) {
      return
    }
    const result = buildMerchantCancelWithdrawPayload(this.data.cancelForm)
    const errorMessage = this.validateCancelWithdrawDraft(result.errorMessage)
    if (errorMessage || !result.payload) {
      this.setData({ cancelFormErrorMessage: errorMessage || '请补全注销提现资料' })
      return
    }
    this.setData({ cancelFormErrorMessage: '', cancelDialogVisible: true })
  },
  onCancelDialogClose() { if (!this.data.cancelSubmitting) this.setData({ cancelDialogVisible: false }) },

  onWithdrawModeChange(event: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    const value = getRadioValue(event.detail) === 'APPLY_WITHDRAW' ? 'APPLY_WITHDRAW' : 'NOT_APPLY_WITHDRAW'
    this.setData({
      'cancelForm.withdraw': value,
      'cancelForm.businessLicenseStatusDeclaration': value === 'APPLY_WITHDRAW' ? 'ACTIVE' : '',
      cancelFormErrorMessage: '',
      showApplyWithdrawForm: value === 'APPLY_WITHDRAW',
      proofMediaRequired: false
    })
  },

  onLicenseStatusChange(event: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    const value = getRadioValue(event.detail)
    const declaration = value === 'CANCELED' || value === 'REVOKED' ? value : 'ACTIVE'
    this.setData({
      'cancelForm.businessLicenseStatusDeclaration': declaration,
      cancelFormErrorMessage: '',
      proofMediaRequired: declaration === 'CANCELED' || declaration === 'REVOKED'
    })
  },

  onAccountTypeChange(event: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    const accountType = getRadioValue(event.detail) === 'ACCOUNT_TYPE_PERSONAL' ? 'ACCOUNT_TYPE_PERSONAL' : 'ACCOUNT_TYPE_CORPORATE'
    this.setData({
      'cancelForm.accountType': accountType,
      cancelFormErrorMessage: '',
      showPersonalIdentityForm: accountType === 'ACCOUNT_TYPE_PERSONAL'
    })
  },

  onIdDocTypeChange(event: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    this.setData({ 'cancelForm.idDocType': getRadioValue(event.detail) || 'IDENTIFICATION_TYPE_ID_CARD', cancelFormErrorMessage: '' })
  },

  onTapCancelApplication(event: WechatMiniprogram.TouchEvent) {
    const id = Number(event.currentTarget.dataset.id)
    if (!Number.isFinite(id) || id <= 0) {
      return
    }
    wx.navigateTo({ url: `/pages/merchant/finance/cancel-withdraw/detail/index?id=${id}` })
  },

  onCancelFormInput(event: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    const field = String(event.currentTarget.dataset.field || '')
    if (!field) {
      return
    }
    this.setData({ [`cancelForm.${field}`]: getInputValue(event.detail), cancelFormErrorMessage: '' })
  },

  onAddProofMedia(event: WechatMiniprogram.CustomEvent<{ files?: unknown, currentSelectedFiles?: unknown }>) {
    void this.uploadCancelWithdrawMaterials('proof', getUploadFiles(event.detail))
  },

  onAddAdditionalMaterial(event: WechatMiniprogram.CustomEvent<{ files?: unknown, currentSelectedFiles?: unknown }>) {
    void this.uploadCancelWithdrawMaterials('additional', getUploadFiles(event.detail))
  },

  onRemoveProofMedia() {
    if (this.data.cancelSubmitting || this.data.uploadingProofMedia) {
      return
    }
    this.setData({ proofMediaFiles: [], 'cancelForm.proofMediaAssetIds': [], cancelFormErrorMessage: '' })
  },

  onRemoveAdditionalMaterial(event: WechatMiniprogram.CustomEvent<{ index?: number }>) {
    if (this.data.cancelSubmitting || this.data.uploadingAdditionalMaterial) {
      return
    }
    const index = Number(event.detail?.index)
    if (!Number.isFinite(index) || index < 0) {
      return
    }
    const files = this.data.additionalMaterialFiles.slice()
    files.splice(index, 1)
    this.setData({
      additionalMaterialFiles: files,
      'cancelForm.additionalMaterialAssetIds': files.map((file) => file.assetId),
      cancelFormErrorMessage: ''
    })
  },

  async uploadCancelWithdrawMaterials(kind: 'proof' | 'additional', files: Array<{ url?: string, name?: string }>) {
    const usableFiles = files.filter((file) => !!file.url)
    if (!usableFiles.length) {
      return
    }
    if (kind === 'proof' && this.data.proofMediaFiles.length >= 1) {
      this.setData({ cancelFormErrorMessage: '注销提现证明材料最多上传 1 张' })
      return
    }
    if (kind === 'additional' && this.data.additionalMaterialFiles.length + usableFiles.length > 10) {
      this.setData({ cancelFormErrorMessage: '补充材料最多上传 10 张' })
      return
    }

    const loadingKey = kind === 'proof' ? 'uploadingProofMedia' : 'uploadingAdditionalMaterial'
    this.setData({ [loadingKey]: true, cancelFormErrorMessage: '' })
    try {
      const uploaded: UploadedMaterialView[] = []
      for (const file of usableFiles) {
        const result = await uploadMerchantCancelWithdrawMaterial(file.url || '')
        uploaded.push({ assetId: result.mediaId, url: result.displayUrl || file.url || '', name: file.name || '材料图片', type: 'image', status: 'done' })
      }
      if (kind === 'proof') {
        const nextFiles = uploaded.slice(0, 1)
        this.setData({ proofMediaFiles: nextFiles, 'cancelForm.proofMediaAssetIds': nextFiles.map((file) => file.assetId) })
      } else {
        const nextFiles = this.data.additionalMaterialFiles.concat(uploaded).slice(0, 10)
        this.setData({ additionalMaterialFiles: nextFiles, 'cancelForm.additionalMaterialAssetIds': nextFiles.map((file) => file.assetId) })
      }
    } catch (error) {
      logger.warn('Merchant cancel withdraw material upload failed', error)
      this.setData({ cancelFormErrorMessage: getMerchantFinanceUserMessage(error, '材料上传失败，请更换图片后重试') })
    } finally {
      this.setData({ [loadingKey]: false })
    }
  },

  validateCancelWithdrawDraft(errorMessage?: string): string {
    if (errorMessage) {
      return errorMessage
    }
    if (this.data.hasFundsToWithdraw && this.data.cancelForm.withdraw === 'NOT_APPLY_WITHDRAW') {
      return '账户仍有可提现余额，请选择提现后注销'
    }
    return ''
  },

  async onConfirmCancelWithdraw() {
    if (!this.data.canSubmitCancelWithdraw || this.data.cancelSubmitting) return
    const result = buildMerchantCancelWithdrawPayload(this.data.cancelForm)
    const errorMessage = this.validateCancelWithdrawDraft(result.errorMessage)
    if (errorMessage || !result.payload) {
      this.setData({ cancelDialogVisible: false, cancelFormErrorMessage: errorMessage || '请补全注销提现资料' })
      return
    }
    this.setData({ cancelSubmitting: true })
    try {
      this.setData({ cancelSyncMessage: '申请已提交，正在同步微信与后端结果...' })
      await submitMerchantCancelWithdrawAndWait(result.payload, { maxAttempts: 6 })
      this.setData({ cancelSubmitting: false, cancelDialogVisible: false, cancelSyncMessage: '' })
      await this.loadDetail({ silent: true })
    } catch (error) {
      logger.warn('Merchant cancel withdraw submit failed', error)
      this.setData({ cancelSubmitting: false, cancelDialogVisible: false, cancelSyncMessage: '', cancelFormErrorMessage: getMerchantFinanceUserMessage(error, '注销提现申请提交失败，请稍后重试') })
    }
  }
})