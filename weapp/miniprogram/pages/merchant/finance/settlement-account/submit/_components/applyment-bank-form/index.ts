import {
  type ApplymentAccountType,
  type ApplymentContactType,
  type ApplymentContactDocType,
  type ApplymentBindBankPayload
} from '../../../../../_main_shared/api/applyment-bank'
import { uploadMedia } from '../../../../../../../utils/media'
import { getErrorUserMessage } from '../../../../../../../utils/user-facing'

const DEFAULT_CONTACT_DOC_TYPE: ApplymentContactDocType = 'IDENTIFICATION_TYPE_MAINLAND_IDCARD'
const CONTACT_EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

type ContactDocumentKind = 'front' | 'back'

interface ApplymentBindBankDraft {
  account_type: ApplymentAccountType
  account_bank: string
  account_bank_code: number
  bank_alias: string
  bank_alias_code: string
  need_bank_branch: boolean
  bank_address_code: string
  manual_bank_city: string
  bank_branch_id: string
  bank_name: string
  account_number: string
  account_name: string
  contact_email: string
  contact_type: ApplymentContactType
  contact_name: string
  contact_id_doc_type: ApplymentContactDocType
  contact_id_card_number: string
  contact_id_doc_copy_asset_id: number
  contact_id_doc_copy_url: string
  contact_id_doc_copy_raw_url: string
  contact_id_doc_copy_back_asset_id: number
  contact_id_doc_copy_back_url: string
  contact_id_doc_copy_back_raw_url: string
  contact_id_doc_period_begin: string
  contact_id_doc_period_end: string
}

type PartialApplymentBindBankDraft = Partial<ApplymentBindBankDraft>
export type ApplymentBankFormDraftPayload = PartialApplymentBindBankDraft
export type ApplymentBankFormPayload = ApplymentBindBankPayload

interface ApplymentBankFormProperties {
  initialDraft?: PartialApplymentBindBankDraft
  defaultAccountType: ApplymentAccountType
  allowedAccountTypes?: ApplymentAccountType[]
  businessAccountName?: string
  privateAccountName?: string
  showContactFields?: boolean
  requireContactEmail?: boolean
  requireAccountName?: boolean
  showAccountTypeSelector?: boolean
  embedded?: boolean
  submitBlock?: boolean
  showSubmitActions?: boolean
  uploadBusinessType?: string
}

type ApplymentBankFormInstance = WechatMiniprogram.Component.TrivialInstance & {
  draftForm?: ApplymentBindBankDraft
}
type ApplymentFormStateOptions = {
  emitDraft?: boolean
  syncSubmit?: boolean
}

type ContactDocumentFeedback = {
  state: 'idle' | 'processing' | 'success' | 'error'
  title: string
  description: string
}

function createEmptyDraft(accountType: ApplymentAccountType): ApplymentBindBankDraft {
  return {
    account_type: accountType,
    account_bank: '',
    account_bank_code: 0,
    bank_alias: '',
    bank_alias_code: '',
    need_bank_branch: false,
    bank_address_code: '',
    manual_bank_city: '',
    bank_branch_id: '',
    bank_name: '',
    account_number: '',
    account_name: '',
    contact_email: '',
    contact_type: 'LEGAL',
    contact_name: '',
    contact_id_doc_type: DEFAULT_CONTACT_DOC_TYPE,
    contact_id_card_number: '',
    contact_id_doc_copy_asset_id: 0,
    contact_id_doc_copy_url: '',
    contact_id_doc_copy_raw_url: '',
    contact_id_doc_copy_back_asset_id: 0,
    contact_id_doc_copy_back_url: '',
    contact_id_doc_copy_back_raw_url: '',
    contact_id_doc_period_begin: '',
    contact_id_doc_period_end: ''
  }
}

function createIdleFeedback(): ContactDocumentFeedback {
  return {
    state: 'idle',
    title: '',
    description: ''
  }
}

function normalizeContactType(value?: string | null): ApplymentContactType {
  return value === 'SUPER' ? 'SUPER' : 'LEGAL'
}

function normalizeContactDocType(value?: string | null): ApplymentContactDocType {
  return value === DEFAULT_CONTACT_DOC_TYPE ? value : DEFAULT_CONTACT_DOC_TYPE
}

function normalizeOptionalText(value?: string | null): string {
  return typeof value === 'string' ? value.trim() : ''
}

function normalizeAllowedAccountTypes(values?: ApplymentAccountType[] | null): ApplymentAccountType[] {
  if (!Array.isArray(values) || values.length === 0) {
    return ['ACCOUNT_TYPE_BUSINESS', 'ACCOUNT_TYPE_PRIVATE']
  }
  const normalized = values.filter((value) => value === 'ACCOUNT_TYPE_BUSINESS' || value === 'ACCOUNT_TYPE_PRIVATE')
  return normalized.length > 0 ? normalized : ['ACCOUNT_TYPE_BUSINESS']
}

function isAccountTypeAllowed(accountType: ApplymentAccountType, values?: ApplymentAccountType[] | null): boolean {
  return normalizeAllowedAccountTypes(values).includes(accountType)
}

function defaultAccountNameForType(accountType: ApplymentAccountType, properties: ApplymentBankFormProperties): string {
  if (accountType === 'ACCOUNT_TYPE_PRIVATE') {
    return normalizeOptionalText(properties.privateAccountName)
  }
  return normalizeOptionalText(properties.businessAccountName)
}

function isLongTermContactDocument(form: ApplymentBindBankDraft): boolean {
  return form.contact_id_doc_period_end.trim() === '长期'
}

function requiresSuperContactFields(form: ApplymentBindBankDraft, showContactFields?: boolean): boolean {
  return Boolean(showContactFields && form.contact_type === 'SUPER')
}

function normalizeDraft(
  accountType: ApplymentAccountType,
  draft?: PartialApplymentBindBankDraft | null,
  allowedAccountTypes?: ApplymentAccountType[] | null
): ApplymentBindBankDraft {
  const normalizedAllowedTypes = normalizeAllowedAccountTypes(allowedAccountTypes)
  const normalizedDefaultAccountType = normalizedAllowedTypes.includes(accountType) ? accountType : normalizedAllowedTypes[0]
  const base = createEmptyDraft(normalizedDefaultAccountType)
  if (!draft) {
    return base
  }

  const draftAccountType = draft.account_type === 'ACCOUNT_TYPE_PRIVATE' || draft.account_type === 'ACCOUNT_TYPE_BUSINESS'
    ? draft.account_type
    : normalizedDefaultAccountType
  const nextAccountType = normalizedAllowedTypes.includes(draftAccountType) ? draftAccountType : normalizedDefaultAccountType

  return {
    account_type: nextAccountType,
    account_bank: typeof draft.account_bank === 'string' ? draft.account_bank : '',
    account_bank_code: typeof draft.account_bank_code === 'number' ? draft.account_bank_code : 0,
    bank_alias: typeof draft.bank_alias === 'string' ? draft.bank_alias : '',
    bank_alias_code: typeof draft.bank_alias_code === 'string' ? draft.bank_alias_code : '',
    need_bank_branch: Boolean(draft.need_bank_branch),
    bank_address_code: typeof draft.bank_address_code === 'string' ? draft.bank_address_code : '',
    manual_bank_city: typeof draft.manual_bank_city === 'string' ? draft.manual_bank_city : '',
    bank_branch_id: typeof draft.bank_branch_id === 'string' ? draft.bank_branch_id : '',
    bank_name: typeof draft.bank_name === 'string' ? draft.bank_name : '',
    account_number: typeof draft.account_number === 'string' ? draft.account_number : '',
    account_name: typeof draft.account_name === 'string' ? draft.account_name : '',
    contact_email: typeof draft.contact_email === 'string' ? draft.contact_email : '',
    contact_type: normalizeContactType(draft.contact_type),
    contact_name: typeof draft.contact_name === 'string' ? draft.contact_name : '',
    contact_id_doc_type: normalizeContactDocType(draft.contact_id_doc_type),
    contact_id_card_number: typeof draft.contact_id_card_number === 'string' ? draft.contact_id_card_number : '',
    contact_id_doc_copy_asset_id: typeof draft.contact_id_doc_copy_asset_id === 'number' ? draft.contact_id_doc_copy_asset_id : 0,
    contact_id_doc_copy_url: typeof draft.contact_id_doc_copy_url === 'string' ? draft.contact_id_doc_copy_url : '',
    contact_id_doc_copy_raw_url: typeof draft.contact_id_doc_copy_raw_url === 'string' ? draft.contact_id_doc_copy_raw_url : '',
    contact_id_doc_copy_back_asset_id: typeof draft.contact_id_doc_copy_back_asset_id === 'number' ? draft.contact_id_doc_copy_back_asset_id : 0,
    contact_id_doc_copy_back_url: typeof draft.contact_id_doc_copy_back_url === 'string' ? draft.contact_id_doc_copy_back_url : '',
    contact_id_doc_copy_back_raw_url: typeof draft.contact_id_doc_copy_back_raw_url === 'string' ? draft.contact_id_doc_copy_back_raw_url : '',
    contact_id_doc_period_begin: normalizeOptionalText(draft.contact_id_doc_period_begin),
    contact_id_doc_period_end: normalizeOptionalText(draft.contact_id_doc_period_end)
  }
}

function getSubmitBlockMessage(
  form: ApplymentBindBankDraft,
  showContactFields?: boolean,
  requireContactEmail?: boolean,
  requireAccountName: boolean = true
): string {
  if (!form.account_number.trim()) {
    return '请先填写银行账号'
  }

  if (requireAccountName && !form.account_name.trim()) {
    return '请先填写开户名称'
  }

  if (requireContactEmail) {
    if (!form.contact_email.trim()) {
      return '请填写超级管理员邮箱'
    }
    if (!CONTACT_EMAIL_PATTERN.test(form.contact_email.trim())) {
      return '请填写正确的邮箱地址'
    }
  }

  if (!form.account_bank.trim()) {
    return '请先填写开户银行'
  }

  if (requiresSuperContactFields(form, showContactFields)) {
    if (!form.contact_name.trim()) {
      return '请先填写超级管理员姓名'
    }
    if (!form.contact_id_card_number.trim()) {
      return '请先填写超级管理员身份证号'
    }
    if (!form.contact_id_doc_copy_asset_id) {
      return '请先上传超级管理员身份证人像面'
    }
    if (!form.contact_id_doc_copy_back_asset_id) {
      return '请先上传超级管理员身份证国徽面'
    }
    if (!form.contact_id_doc_period_begin.trim()) {
      return '请先填写超级管理员证件有效期开始时间'
    }
    if (!form.contact_id_doc_period_end.trim()) {
      return '请先填写超级管理员证件有效期结束时间'
    }
  }

  if (!form.bank_address_code.trim()) {
    return '请先填写开户省份'
  }

  if (!form.manual_bank_city.trim()) {
    return '请先填写开户城市'
  }

  if (!form.bank_name.trim()) {
    return '请先填写开户支行'
  }

  return ''
}

function canSubmitForm(
  form: ApplymentBindBankDraft,
  showContactFields?: boolean,
  requireContactEmail?: boolean,
  requireAccountName: boolean = true
): boolean {
  return !getSubmitBlockMessage(form, showContactFields, requireContactEmail, requireAccountName)
}

Component({
  properties: {
    initialDraft: {
      type: Object,
      value: {}
    },
    defaultAccountType: {
      type: String,
      value: 'ACCOUNT_TYPE_BUSINESS'
    },
    allowedAccountTypes: {
      type: Array,
      value: []
    },
    businessAccountName: {
      type: String,
      value: ''
    },
    privateAccountName: {
      type: String,
      value: ''
    },
    showContactFields: {
      type: Boolean,
      value: false
    },
    requireContactEmail: {
      type: Boolean,
      value: false
    },
    requireAccountName: {
      type: Boolean,
      value: true
    },
    showAccountTypeSelector: {
      type: Boolean,
      value: true
    },
    embedded: {
      type: Boolean,
      value: false
    },
    submitBlock: {
      type: Boolean,
      value: true
    },
    showSubmitActions: {
      type: Boolean,
      value: true
    },
    showCancelButton: {
      type: Boolean,
      value: true
    },
    uploadBusinessType: {
      type: String,
      value: ''
    },
    submitLabel: {
      type: String,
      value: '提交银行账户信息'
    },
    submitting: {
      type: Boolean,
      value: false
    }
  },

  data: {
    form: createEmptyDraft('ACCOUNT_TYPE_BUSINESS' as ApplymentAccountType),
    contactDocCopyFeedbackState: 'idle',
    contactDocCopyFeedbackTitle: '',
    contactDocCopyFeedbackDescription: '',
    contactDocCopyBackFeedbackState: 'idle',
    contactDocCopyBackFeedbackTitle: '',
    contactDocCopyBackFeedbackDescription: '',
    canSubmit: false,
    submitBlockMessage: '',
    showPrivateAccountTypeOption: true
  },

  lifetimes: {
    attached() {
      void this.initializeForm()
    }
  },

  methods: {
    readForm() {
      const instance = this as unknown as ApplymentBankFormInstance
      return instance.draftForm || (this.data.form as ApplymentBindBankDraft)
    },

    setFormState(
      nextForm: ApplymentBindBankDraft,
      extraData: Record<string, unknown> = {},
      options?: ApplymentFormStateOptions
    ) {
      const instance = this as unknown as ApplymentBankFormInstance
      instance.draftForm = nextForm
      this.setData(Object.assign({ form: nextForm }, extraData))

      if (options?.syncSubmit !== false) {
        this.syncCanSubmit(nextForm)
      }
      if (options?.emitDraft !== false) {
        this.emitDraftChange(nextForm)
      }
    },

    initializeForm() {
      const properties = this.properties as unknown as ApplymentBankFormProperties
      const accountType = properties.defaultAccountType
      const initialDraft = normalizeDraft(accountType, properties.initialDraft, properties.allowedAccountTypes)

      this.setFormState(initialDraft, {
        canSubmit: canSubmitForm(
          initialDraft,
          properties.showContactFields,
          properties.requireContactEmail,
          properties.requireAccountName
        ),
        submitBlockMessage: getSubmitBlockMessage(
          initialDraft,
          properties.showContactFields,
          properties.requireContactEmail,
          properties.requireAccountName
        ),
        showPrivateAccountTypeOption: isAccountTypeAllowed('ACCOUNT_TYPE_PRIVATE' as ApplymentAccountType, properties.allowedAccountTypes)
      }, { emitDraft: false, syncSubmit: false })

    },

    emitDraftChange(nextForm?: ApplymentBindBankDraft) {
      const form = nextForm || (this.data.form as ApplymentBindBankDraft)
      this.triggerEvent('draftchange', {
        ...form,
        deposit_bank_province: form.bank_address_code.trim() || undefined,
        deposit_bank_city: form.manual_bank_city.trim() || undefined
      })
    },

    getUploadBusinessType() {
      const properties = this.properties as unknown as ApplymentBankFormProperties
      return String(properties.uploadBusinessType || '').trim()
    },

    setContactDocumentFeedback(kind: ContactDocumentKind, feedback: ContactDocumentFeedback) {
      const prefix = kind === 'front' ? 'contactDocCopy' : 'contactDocCopyBack'
      this.setData({
        [`${prefix}FeedbackState`]: feedback.state,
        [`${prefix}FeedbackTitle`]: feedback.title,
        [`${prefix}FeedbackDescription`]: feedback.description
      })
    },

    buildContactDocumentFields(kind: ContactDocumentKind, result?: { mediaId: number, displayUrl: string, rawUrl: string }) {
      if (kind === 'front') {
        return result
          ? {
              contact_id_doc_copy_asset_id: result.mediaId,
              contact_id_doc_copy_url: result.displayUrl,
              contact_id_doc_copy_raw_url: result.rawUrl
            }
          : {
              contact_id_doc_copy_asset_id: 0,
              contact_id_doc_copy_url: '',
              contact_id_doc_copy_raw_url: ''
            }
      }

      return result
        ? {
            contact_id_doc_copy_back_asset_id: result.mediaId,
            contact_id_doc_copy_back_url: result.displayUrl,
            contact_id_doc_copy_back_raw_url: result.rawUrl
          }
        : {
            contact_id_doc_copy_back_asset_id: 0,
            contact_id_doc_copy_back_url: '',
            contact_id_doc_copy_back_raw_url: ''
          }
    },

    async uploadContactDocument(kind: ContactDocumentKind, path: string) {
      const uploadBusinessType = this.getUploadBusinessType()
      if (!uploadBusinessType) {
        wx.showToast({ title: '当前页面未配置证件上传能力', icon: 'none' })
        return
      }

      this.setContactDocumentFeedback(kind, {
        state: 'processing',
        title: '证件上传中',
        description: '请稍候，上传完成后会自动回填到当前表单。'
      })

      try {
        const uploadResult = await uploadMedia(path, {
          businessType: uploadBusinessType,
          mediaCategory: kind === 'front' ? 'id_card_front' : 'id_card_back'
        })

        const nextForm: ApplymentBindBankDraft = {
          ...this.readForm(),
          ...this.buildContactDocumentFields(kind, {
            mediaId: uploadResult.mediaId,
            displayUrl: uploadResult.displayUrl,
            rawUrl: uploadResult.urls.original || uploadResult.displayUrl
          })
        }

        this.setFormState(nextForm)
        this.setContactDocumentFeedback(kind, {
          state: 'success',
          title: '证件已上传',
          description: '如需更换，请先删除后重新上传。'
        })
      } catch (error: unknown) {
        this.setContactDocumentFeedback(kind, {
          state: 'error',
          title: '上传失败',
          description: getErrorUserMessage(error, '证件上传失败，请稍后重试')
        })
        wx.showToast({
          title: getErrorUserMessage(error, '证件上传失败，请稍后重试'),
          icon: 'none'
        })
      }
    },

    syncCanSubmit(nextForm?: ApplymentBindBankDraft) {
      const form = nextForm || (this.data.form as ApplymentBindBankDraft)
      const canSubmit = canSubmitForm(
        form,
        this.properties.showContactFields,
        this.properties.requireContactEmail,
        this.properties.requireAccountName
      )
      const submitBlockMessage = getSubmitBlockMessage(
        form,
        this.properties.showContactFields,
        this.properties.requireContactEmail,
        this.properties.requireAccountName
      )
      this.setData({ canSubmit, submitBlockMessage })
    },

    resetBankSelection(accountType: ApplymentAccountType) {
      const currentForm = this.readForm()
      const properties = this.properties as unknown as ApplymentBankFormProperties
      const accountName = defaultAccountNameForType(accountType, properties)
      const nextForm: ApplymentBindBankDraft = {
        ...currentForm,
        account_type: accountType,
        account_name: accountName,
        account_bank: '',
        account_bank_code: 0,
        bank_alias: '',
        bank_alias_code: '',
        need_bank_branch: false,
        bank_address_code: '',
        manual_bank_city: '',
        bank_branch_id: '',
        bank_name: ''
      }

      this.setFormState(nextForm)
    },

    onTextFieldChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
      const field = String(e.currentTarget.dataset.field || '')
      if (!field) {
        return
      }

      const value = e.detail.value || ''
      const nextForm: ApplymentBindBankDraft = {
        ...this.readForm(),
        [field]: field === 'contact_id_doc_period_end' && value.trim() === '长期'
          ? '长期'
          : value
      }

      this.setFormState(nextForm)
    },

    onSuperContactSwitchChange(e: WechatMiniprogram.CustomEvent<{ value?: boolean }>) {
      const useSuperContact = Boolean(e.detail?.value)
      const currentForm = this.readForm()
      const nextContactType: ApplymentContactType = useSuperContact ? 'SUPER' : 'LEGAL'

      if (currentForm.contact_type === nextContactType) {
        return
      }

      const nextForm: ApplymentBindBankDraft = useSuperContact
        ? {
          ...currentForm,
          contact_type: 'SUPER',
          contact_id_doc_type: DEFAULT_CONTACT_DOC_TYPE
        }
        : {
          ...currentForm,
          contact_type: 'LEGAL',
          contact_name: '',
          contact_id_doc_type: DEFAULT_CONTACT_DOC_TYPE,
          contact_id_card_number: '',
          contact_id_doc_copy_asset_id: 0,
          contact_id_doc_copy_url: '',
          contact_id_doc_copy_raw_url: '',
          contact_id_doc_copy_back_asset_id: 0,
          contact_id_doc_copy_back_url: '',
          contact_id_doc_copy_back_raw_url: '',
          contact_id_doc_period_begin: '',
          contact_id_doc_period_end: ''
        }

      this.setFormState(nextForm)
      if (!useSuperContact) {
        this.setContactDocumentFeedback('front', createIdleFeedback())
        this.setContactDocumentFeedback('back', createIdleFeedback())
      }
    },

    onContactDocLongTermToggle() {
      const currentForm = this.readForm()
      const nextForm: ApplymentBindBankDraft = {
        ...currentForm,
        contact_id_doc_period_end: isLongTermContactDocument(currentForm) ? '' : '长期'
      }
      this.setFormState(nextForm)
    },

    async onContactIDDocCopyChange(e: WechatMiniprogram.CustomEvent<{ path?: string }>) {
      const path = String(e.detail?.path || '').trim()
      if (!path) {
        return
      }
      await this.uploadContactDocument('front', path)
    },

    async onContactIDDocCopyBackChange(e: WechatMiniprogram.CustomEvent<{ path?: string }>) {
      const path = String(e.detail?.path || '').trim()
      if (!path) {
        return
      }
      await this.uploadContactDocument('back', path)
    },

    onContactIDDocCopyRemove() {
      const nextForm: ApplymentBindBankDraft = {
        ...this.readForm(),
        ...this.buildContactDocumentFields('front')
      }
      this.setFormState(nextForm)
      this.setContactDocumentFeedback('front', createIdleFeedback())
    },

    onContactIDDocCopyBackRemove() {
      const nextForm: ApplymentBindBankDraft = {
        ...this.readForm(),
        ...this.buildContactDocumentFields('back')
      }
      this.setFormState(nextForm)
      this.setContactDocumentFeedback('back', createIdleFeedback())
    },

    onAccountTypeSelect(e: WechatMiniprogram.TouchEvent) {
      const { value } = e.currentTarget.dataset as { value?: ApplymentAccountType }
      const accountType = value
      if (!accountType || accountType === this.readForm().account_type) {
        return
      }
      if (!isAccountTypeAllowed(accountType, (this.properties as unknown as ApplymentBankFormProperties).allowedAccountTypes)) {
        return
      }

      this.resetBankSelection(accountType)
    },

    onCancel() {
      this.triggerEvent('cancel')
    },

    onManualBankFieldChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
      const field = String(e.currentTarget.dataset.field || '')
      if (!field) {
        return
      }
      const value = String(e.detail?.value || '')
      const currentForm = this.readForm()
      const patch: Partial<ApplymentBindBankDraft> = {
        [field]: value
      }

      if (field === 'account_bank') {
        patch.bank_alias = value
        patch.bank_alias_code = ''
        patch.account_bank_code = 0
      }
      if (field === 'bank_name') {
        patch.bank_branch_id = value.trim()
      }

      this.setFormState({
        ...currentForm,
        ...patch
      })
    },

    onSubmit() {
      const form = this.readForm()
      const properties = this.properties as unknown as ApplymentBankFormProperties
      const submitBlockMessage = getSubmitBlockMessage(
        form,
        properties.showContactFields,
        properties.requireContactEmail,
        properties.requireAccountName
      )
      if (submitBlockMessage) {
        wx.showToast({ title: submitBlockMessage, icon: 'none' })
        return
      }

      const payload: ApplymentBindBankPayload = {
        account_type: form.account_type,
        account_bank: form.account_bank.trim(),
        account_bank_code: form.account_bank_code > 0 ? form.account_bank_code : undefined,
        bank_alias: form.bank_alias.trim() || undefined,
        bank_alias_code: form.bank_alias_code.trim() || undefined,
        need_bank_branch: form.need_bank_branch || undefined,
        bank_address_code: form.bank_address_code.trim() || undefined,
        deposit_bank_province: form.bank_address_code.trim() || undefined,
        deposit_bank_city: form.manual_bank_city.trim() || undefined,
        manual_bank_city: form.manual_bank_city.trim() || undefined,
        bank_branch_id: form.bank_branch_id.trim() || undefined,
        bank_name: form.bank_name.trim() || undefined,
        account_number: form.account_number.trim(),
        account_name: form.account_name.trim() || undefined,
        contact_email: form.contact_email.trim() || undefined
      }

      if (properties.showContactFields && form.contact_type === 'SUPER') {
        payload.contact_type = 'SUPER'
        payload.contact_name = form.contact_name.trim()
        payload.contact_id_doc_type = form.contact_id_doc_type
        payload.contact_id_card_number = form.contact_id_card_number.trim()
        payload.contact_id_doc_copy_asset_id = form.contact_id_doc_copy_asset_id > 0 ? form.contact_id_doc_copy_asset_id : undefined
        payload.contact_id_doc_copy_back_asset_id = form.contact_id_doc_copy_back_asset_id > 0 ? form.contact_id_doc_copy_back_asset_id : undefined
        payload.contact_id_doc_period_begin = form.contact_id_doc_period_begin.trim()
        payload.contact_id_doc_period_end = form.contact_id_doc_period_end.trim()
      }

      this.triggerEvent('submit', payload)
    }
  }
})
