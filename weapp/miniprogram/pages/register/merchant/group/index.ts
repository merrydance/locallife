import { 
  getOrCreateGroupApplication, 
  updateGroupApplicationBasic, 
  deleteGroupApplicationDocument,
  ocrGroupIdCard,
  ocrGroupBusinessLicense, 
  uploadGroupTrademarkCertificate,
  submitGroupApplication,
  type GroupApplicationResponse
} from '../../_main_shared/api/group-application'
import { getPrivateMediaUrl } from '../../_main_shared/utils/image-security'
import { logger } from '../../../../utils/logger'
import Navigation from '../../../../utils/navigation'
import { buildAgreementConsentPayload } from '../../_main_shared/api/agreement-consent'
import { AdminApprovalDisplay, getAdminApprovalStatusDisplay } from '../../_main_shared/adapters/admin-review'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type NavHeightEvent = {
  detail: {
    navBarHeight: number
  }
}

type UploadEvent = {
  detail: {
    path: string
  }
}

type UploadFieldValue = {
  url: string
  rawUrl?: string
  assetId?: number
}

type OCRDisplayStateValue = 'idle' | 'processing' | 'done' | 'failed'

type GroupOCRDisplayState = {
  license: OCRDisplayStateValue
  identity: OCRDisplayStateValue
}

type UploadFeedbackState = 'idle' | 'processing' | 'success' | 'error'

type UploadFeedback = {
  state: UploadFeedbackState
  title: string
  description: string
}

type GroupUploadFeedback = {
  license: UploadFeedback
  idFront: UploadFeedback
  idBack: UploadFeedback
  trademark: UploadFeedback
}

type GroupUploadField = 'license' | 'idFront' | 'idBack' | 'trademark'

type GroupResultStatusView = AdminApprovalDisplay & {
  iconName: 'check-circle-filled' | 'time-filled'
  iconColor: string
  resultText: string
  resultHint: string
}

const DEFAULT_GROUP_OCR_DISPLAY_STATE: GroupOCRDisplayState = {
  license: 'idle',
  identity: 'idle'
}

const EMPTY_UPLOAD_FEEDBACK: UploadFeedback = {
  state: 'idle',
  title: '',
  description: ''
}

const DEFAULT_GROUP_UPLOAD_FEEDBACK: GroupUploadFeedback = {
  license: { ...EMPTY_UPLOAD_FEEDBACK },
  idFront: { ...EMPTY_UPLOAD_FEEDBACK },
  idBack: { ...EMPTY_UPLOAD_FEEDBACK },
  trademark: { ...EMPTY_UPLOAD_FEEDBACK }
}

type InputEvent = {
  currentTarget: {
    dataset: {
      field?: string
    }
  }
  detail: {
    value: string
  }
}

const getErrorMessage = getErrorUserMessage

const buildGroupResultStatusView = (status?: string): GroupResultStatusView => {
  const display = getAdminApprovalStatusDisplay(status)
  const isApproved = display.isApproved
  return {
    ...display,
    iconName: isApproved ? 'check-circle-filled' : 'time-filled',
    iconColor: isApproved ? 'var(--td-success-color)' : '#FF9800',
    resultText: isApproved ? '申请已通过' : '申请已提交，审核中',
    resultHint: isApproved
      ? '平台已完成审核，请返回个人中心继续后续操作。'
      : '平台将在 24 小时内完成审核，请留意站内消息与电话联系。'
  }
}

const createUploadFeedback = (state: UploadFeedbackState, title = '', description = ''): UploadFeedback => ({
  state,
  title,
  description
})

const hasGroupText = (value?: string): boolean => typeof value === 'string' && value.trim().length > 0

const hasGroupLicenseResult = (res?: GroupApplicationResponse): boolean => Boolean(
  hasGroupText(res?.license_number)
  || hasGroupText(res?.business_license_ocr?.credit_code)
  || hasGroupText(res?.business_license_ocr?.reg_num)
  || hasGroupText(res?.business_license_ocr?.enterprise_name)
)

const hasGroupIdentityFrontResult = (res?: GroupApplicationResponse): boolean => Boolean(
  hasGroupText(res?.legal_person_name)
  || hasGroupText(res?.legal_person_id_number)
  || hasGroupText(res?.id_card_front_ocr?.name)
  || hasGroupText(res?.id_card_front_ocr?.id_number)
)

const hasGroupIdentityBackResult = (res?: GroupApplicationResponse): boolean => hasGroupText(res?.id_card_back_ocr?.valid_date)

const hasPersistedDocument = (value?: UploadFieldValue): boolean => Boolean(value?.assetId)

Page({
  data: {
    navBarHeight: 88,
    currentStep: 0,
    submitting: false,
    applicationStatus: 'draft' as GroupApplicationResponse['status'],
    resultStatusView: buildGroupResultStatusView('draft') as GroupResultStatusView,
    rejectReason: '',
    idFront: { url: '', rawUrl: '', assetId: undefined } as UploadFieldValue,
    idBack: { url: '', rawUrl: '', assetId: undefined } as UploadFieldValue,
    license: { url: '', rawUrl: '', assetId: undefined } as UploadFieldValue,
    trademark: { url: '', rawUrl: '', assetId: undefined } as UploadFieldValue,
    ocrDisplayState: DEFAULT_GROUP_OCR_DISPLAY_STATE,
    uploadFeedback: DEFAULT_GROUP_UPLOAD_FEEDBACK,
    formData: {
      groupName: '',
      contactPhone: '',
      address: '',
      licenseNumber: '',
      legalPerson: ''
    },
    phoneError: '',
    consentChecked: false,
    consentPopupVisible: false
  },

  previewRefreshVersion: 0,

  async onLoad() {
    await this.fetchDraft()
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  buildGroupOCRDisplayState(res?: GroupApplicationResponse): GroupOCRDisplayState {
    const licenseStatus = res?.business_license_ocr?.status || ''
    const idFrontStatus = res?.id_card_front_ocr?.status || ''
    const idBackStatus = res?.id_card_back_ocr?.status || ''

    const licenseUploaded = Boolean(res?.license_image_asset_id || this.data.license.assetId || this.data.license.url)
    const identityUploaded = Boolean(
      (res?.id_card_front_asset_id || this.data.idFront.assetId || this.data.idFront.url) &&
      (res?.id_card_back_asset_id || this.data.idBack.assetId || this.data.idBack.url)
    )

    const licenseDone = licenseStatus === 'done' || hasGroupLicenseResult(res)
    const identityDone = (idFrontStatus === 'done' || hasGroupIdentityFrontResult(res))
      && (idBackStatus === 'done' || hasGroupIdentityBackResult(res))

    return {
      license: licenseDone
        ? 'done'
        : licenseStatus === 'failed'
          ? 'failed'
          : licenseUploaded
            ? 'processing'
            : 'idle',
      identity: identityDone
        ? 'done'
        : idFrontStatus === 'failed' || idBackStatus === 'failed'
          ? 'failed'
          : identityUploaded
            ? 'processing'
            : 'idle'
    }
  },

  async resolveUploadPreviewURL(assetId?: number) {
    if (!assetId) return ''
    try {
      return await getPrivateMediaUrl(assetId)
    } catch (_error) {
      return ''
    }
  },

  async refreshUploadPreviewURLs() {
    const refreshVersion = ++this.previewRefreshVersion
    const uploads: Array<{ key: GroupUploadField, value: UploadFieldValue }> = [
      { key: 'license', value: this.data.license },
      { key: 'idFront', value: this.data.idFront },
      { key: 'idBack', value: this.data.idBack },
      { key: 'trademark', value: this.data.trademark }
    ]

    for (const item of uploads) {
      const assetId = item.value?.assetId
      if (!assetId) continue
      const resolved = await this.resolveUploadPreviewURL(assetId)
      const latestValue = this.data[item.key] as UploadFieldValue | undefined
      if (
        refreshVersion === this.previewRefreshVersion
        && latestValue?.assetId === assetId
        && resolved
        && resolved !== latestValue?.url
      ) {
        this.setData({ [`${item.key}.url`]: resolved })
      }
    }
  },

  mapResponseToData(res: GroupApplicationResponse) {
    this.setData({
      applicationStatus: res.status || 'draft',
      resultStatusView: buildGroupResultStatusView(res.status),
      rejectReason: res.reject_reason || '',
      'formData.groupName': res.group_name || '',
      'formData.contactPhone': res.contact_phone || '',
      'formData.address': res.address || '',
      'formData.licenseNumber': res.license_number || '',
      'formData.legalPerson': res.legal_person_name || res.id_card_front_ocr?.name || '',
      phoneError: (res.contact_phone || '').trim() ? '' : this.data.phoneError,
      license: { url: '', rawUrl: '', assetId: res.license_image_asset_id },
      idFront: { url: '', rawUrl: '', assetId: res.id_card_front_asset_id },
      idBack: { url: '', rawUrl: '', assetId: res.id_card_back_asset_id },
      trademark: { url: '', rawUrl: '', assetId: res.trademark_certificate_asset_id },
      ocrDisplayState: this.buildGroupOCRDisplayState(res),
      uploadFeedback: this.buildGroupUploadFeedback(res)
    }, () => {
      void this.refreshUploadPreviewURLs()
    })
  },

  syncFlowWithStatus(res: GroupApplicationResponse) {
    const resultStatusView = buildGroupResultStatusView(res.status)

    if (resultStatusView.isPending || resultStatusView.isApproved) {
      this.setData({ currentStep: 4 })
      return
    }

    if (resultStatusView.isRejected) {
      wx.showModal({
        title: '审核未通过',
        content: `原因：${res.reject_reason || '资料核验失败'}`,
        confirmText: '重新填写资料',
        showCancel: false,
        success: () => {
          this.setData({ currentStep: 1 })
        }
      })
    }
  },

  buildGroupUploadFeedback(res?: GroupApplicationResponse): GroupUploadFeedback {
    const licenseStatus = res?.business_license_ocr?.status || ''
    const licenseError = res?.business_license_ocr?.error || ''
    const idFrontStatus = res?.id_card_front_ocr?.status || ''
    const idFrontError = res?.id_card_front_ocr?.error || ''
    const idBackStatus = res?.id_card_back_ocr?.status || ''
    const idBackError = res?.id_card_back_ocr?.error || ''

    const licenseUploaded = Boolean(res?.license_image_asset_id || this.data.license.assetId || this.data.license.url)
    const idFrontUploaded = Boolean(res?.id_card_front_asset_id || this.data.idFront.assetId || this.data.idFront.url)
    const idBackUploaded = Boolean(res?.id_card_back_asset_id || this.data.idBack.assetId || this.data.idBack.url)
    const trademarkUploaded = Boolean(res?.trademark_certificate_asset_id || this.data.trademark.assetId || this.data.trademark.url)
    const licenseReady = licenseStatus === 'done' || hasGroupLicenseResult(res)
    const idFrontReady = idFrontStatus === 'done' || hasGroupIdentityFrontResult(res)
    const idBackReady = idBackStatus === 'done' || hasGroupIdentityBackResult(res)

    return {
      license: licenseUploaded
        ? licenseStatus === 'failed'
          ? createUploadFeedback('error', '识别失败', licenseError || '请重新上传清晰、完整的营业执照')
          : licenseReady
            ? createUploadFeedback('success', '识别成功', '已识别营业执照主体信息')
            : createUploadFeedback('processing', '证照识别中', '正在识别营业执照信息')
        : { ...EMPTY_UPLOAD_FEEDBACK },
      idFront: idFrontUploaded
        ? idFrontStatus === 'failed'
          ? createUploadFeedback('error', '识别失败', idFrontError || '请重新上传清晰、完整的身份证人像面')
          : idFrontReady
            ? createUploadFeedback('success', '识别成功', '已识别负责人姓名和身份证号')
            : createUploadFeedback('processing', '证照识别中', '正在识别身份证人像面信息')
        : { ...EMPTY_UPLOAD_FEEDBACK },
      idBack: idBackUploaded
        ? idBackStatus === 'failed'
          ? createUploadFeedback('error', '识别失败', idBackError || '请重新上传清晰、完整的身份证国徽面')
          : idBackReady
            ? createUploadFeedback('success', '识别成功', '已识别证件有效期')
            : createUploadFeedback('processing', '证照识别中', '正在识别身份证国徽面信息')
        : { ...EMPTY_UPLOAD_FEEDBACK },
      trademark: trademarkUploaded
        ? createUploadFeedback('success', '上传成功', '已上传注册商标证书')
        : { ...EMPTY_UPLOAD_FEEDBACK }
    }
  },

  setOCRState(type: keyof GroupOCRDisplayState, status: OCRDisplayStateValue) {
    this.setData({ [`ocrDisplayState.${type}`]: status })
  },

  setUploadFeedback(field: keyof GroupUploadFeedback, feedback: UploadFeedback) {
    this.setData({ [`uploadFeedback.${field}`]: feedback })
  },

  async fetchDraft() {
    try {
      const res = await getOrCreateGroupApplication()
      if (res) {
        this.mapResponseToData(res)
        this.syncFlowWithStatus(res)
      }
    } catch (e) {
      logger.error('Fetch group draft failed', e)
    }
  },

  async onIdFrontUpload(e: UploadEvent) {
    const { path } = e.detail
    if (!path) return
    this.setData({
      'idFront.url': path,
      'idFront.rawUrl': path
    })
    this.setOCRState('identity', 'processing')
    this.setUploadFeedback('idFront', createUploadFeedback('processing', '证照识别中', '请稍候，识别结果会显示在当前卡片中'))
    try {
      const res = await ocrGroupIdCard(path, 'Front')
      this.mapResponseToData(res)
    } catch (error) {
      const message = getErrorMessage(error, '识别失败，请提供更清晰更规整的图片重试')
      this.setOCRState('identity', 'failed')
      this.setUploadFeedback('idFront', createUploadFeedback('error', '识别失败', message))
    }
  },

  async onIdBackUpload(e: UploadEvent) {
    const { path } = e.detail
    if (!path) return
    this.setData({
      'idBack.url': path,
      'idBack.rawUrl': path
    })
    this.setOCRState('identity', 'processing')
    this.setUploadFeedback('idBack', createUploadFeedback('processing', '证照识别中', '请稍候，识别结果会显示在当前卡片中'))
    try {
      const res = await ocrGroupIdCard(path, 'Back')
      this.mapResponseToData(res)
    } catch (error) {
      const message = getErrorMessage(error, '识别失败，请提供更清晰更规整的图片重试')
      this.setOCRState('identity', 'failed')
      this.setUploadFeedback('idBack', createUploadFeedback('error', '识别失败', message))
    }
  },

  async onLicenseUpload(e: UploadEvent) {
    const { path } = e.detail
    if (!path) return
    this.setData({ 'license.url': path, 'license.rawUrl': path })
    this.setOCRState('license', 'processing')
    this.setUploadFeedback('license', createUploadFeedback('processing', '证照识别中', '请稍候，识别结果会显示在当前卡片中'))
    try {
      const res = await ocrGroupBusinessLicense(path)
      this.mapResponseToData(res)
    } catch (e) {
      const message = getErrorMessage(e, '识别失败，请提供更清晰更规整的图片重试')
      this.setOCRState('license', 'failed')
      this.setUploadFeedback('license', createUploadFeedback('error', '识别失败', message))
    }
  },

  async onTrademarkUpload(e: UploadEvent) {
    const { path } = e.detail
    if (!path) return
    this.setData({ 'trademark.url': path, 'trademark.rawUrl': path })
    this.setUploadFeedback('trademark', createUploadFeedback('processing', '上传中', '正在上传注册商标证书'))
    try {
      const res = await uploadGroupTrademarkCertificate(path)
      this.mapResponseToData(res)
    } catch (e) {
      const message = getErrorMessage(e, '上传失败，请重新选择清晰、完整的证书图片')
      this.setUploadFeedback('trademark', createUploadFeedback('error', '上传失败', message))
    }
  },

  async removeUploadedDocument(field: GroupUploadField) {
    const documentMap: Record<GroupUploadField, {
      documentType: 'business_license' | 'id_card_front' | 'id_card_back' | 'trademark_certificate'
      data: Record<string, unknown>
    }> = {
      license: {
        documentType: 'business_license',
        data: {
          license: { url: '', rawUrl: '', assetId: undefined },
          'formData.licenseNumber': '',
          'uploadFeedback.license': { ...EMPTY_UPLOAD_FEEDBACK }
        }
      },
      idFront: {
        documentType: 'id_card_front',
        data: {
          idFront: { url: '', rawUrl: '', assetId: undefined },
          'formData.legalPerson': '',
          'uploadFeedback.idFront': { ...EMPTY_UPLOAD_FEEDBACK }
        }
      },
      idBack: {
        documentType: 'id_card_back',
        data: {
          idBack: { url: '', rawUrl: '', assetId: undefined },
          'uploadFeedback.idBack': { ...EMPTY_UPLOAD_FEEDBACK }
        }
      },
      trademark: {
        documentType: 'trademark_certificate',
        data: {
          trademark: { url: '', rawUrl: '', assetId: undefined },
          'uploadFeedback.trademark': { ...EMPTY_UPLOAD_FEEDBACK }
        }
      }
    }

    const target = documentMap[field]

    wx.showLoading({ title: '删除中...' })
    try {
      const res = await deleteGroupApplicationDocument(target.documentType)
      this.setData(target.data, () => {
        this.mapResponseToData(res)
      })
    } catch (e) {
      logger.error('Delete group application document failed', { field, error: e })
      wx.showToast({ title: getErrorMessage(e, '删除失败，请重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  onLicenseRemove() {
    this.removeUploadedDocument('license')
  },

  onIdFrontRemove() {
    this.removeUploadedDocument('idFront')
  },

  onIdBackRemove() {
    this.removeUploadedDocument('idBack')
  },

  onTrademarkRemove() {
    this.removeUploadedDocument('trademark')
  },

  onInput(e: InputEvent) {
    const field = e.currentTarget.dataset.field
    if (!field) {
      return
    }

    const value = e.detail.value || ''
    const nextData: Record<string, string> = {
      [`formData.${field}`]: value
    }
    if (field === 'contactPhone' && value.trim()) {
      nextData.phoneError = ''
    }
    this.setData(nextData)
  },

  onChooseAddress() {
    wx.chooseLocation({
      success: (res) => {
        this.setData({ 'formData.address': res.address || res.name })
      }
    })
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

  onViewAgreement(e: WechatMiniprogram.CustomEvent<{ type?: string, title?: string }>) {
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
    const { currentStep, idFront, idBack, license, trademark, formData } = this.data

    if (currentStep === 0 && !this.ensureConsent()) {
      return
    }
    
    if (currentStep === 1) {
      if (!hasPersistedDocument(idFront) || !hasPersistedDocument(idBack) || !hasPersistedDocument(license) || !hasPersistedDocument(trademark)) {
        wx.showToast({ title: '请上传必填资质材料', icon: 'none' })
        return
      }
    }

    if (currentStep === 2) {
      const groupName = formData.groupName.trim()
      const contactPhone = formData.contactPhone.trim()

      if (!groupName) {
        wx.showToast({ title: '请填写集团/品牌名', icon: 'none' })
        return
      }
      if (!contactPhone || contactPhone.length !== 11) {
        this.setData({ phoneError: '请填写 11 位联系电话，方便平台联系你' })
        wx.showToast({ title: '请输入11位手机号', icon: 'none' })
        return
      }
      this.setData({ phoneError: '' })
      
      // Update basic info to backend
      wx.showLoading({ title: '同步信息...' })
      try {
        await updateGroupApplicationBasic({
          group_name: groupName,
          contact_phone: contactPhone,
          address: formData.address,
          license_number: formData.licenseNumber,
          license_image_asset_id: this.data.license.assetId,
          trademark_certificate_asset_id: this.data.trademark.assetId
        })
        wx.hideLoading()
      } catch (e) {
        wx.hideLoading()
        wx.showToast({ title: '同步失败', icon: 'none' })
        return
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
      wx.showToast({ title: getErrorMessage(e, '协议信息加载失败，请稍后重试'), icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    try {
      const res = await submitGroupApplication(consentPayload)
      this.mapResponseToData(res)
      this.setData({ currentStep: 4 })
    } catch (e: unknown) {
      wx.showToast({ title: getErrorMessage(e, '提交失败'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  onBackHome() {
    wx.switchTab({ url: '/pages/user_center/index' })
  }
})
