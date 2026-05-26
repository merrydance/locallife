import {
  type ApplymentAccountType,
  type ApplymentContactType,
  type ApplymentContactDocType,
  type ApplymentBankOption,
  type ApplymentBranchOption,
  type ApplymentCityOption,
  type ApplymentProvinceOption,
  listApplymentBankBranches,
  listApplymentBanks,
  listApplymentCities,
  listApplymentProvinces,
  searchApplymentBanksByAccount,
  type ApplymentBindBankPayload
} from '../../api/applyment-bank'
import { uploadMedia } from '../../utils/media'
import { getErrorUserMessage } from '../../utils/user-facing'
import { resolveRecognizedBankSelection } from './selection'

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
  apiBasePath: string
  initialDraft?: PartialApplymentBindBankDraft
  defaultAccountType: ApplymentAccountType
  preloadCatalogs?: boolean
  manualCatalog?: boolean
  showContactFields?: boolean
  requireContactEmail?: boolean
  requireAccountName?: boolean
  showAccountTypeSelector?: boolean
  allowSavedAccountNumber?: boolean
  requireBankBranch?: boolean
  embedded?: boolean
  submitBlock?: boolean
  showSubmitActions?: boolean
  uploadBusinessType?: string
}

type ApplymentBankViewOption = ApplymentBankOption & {
  display_label: string
}

type ApplymentPickerOption = {
  label: string
  value: number
}

type ApplymentPickerVisibleChangeEvent = WechatMiniprogram.CustomEvent<{ visible?: boolean }>
type ApplymentPickerChangeEvent = WechatMiniprogram.CustomEvent<{ value?: Array<string | number> }>
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

function isLongTermContactDocument(form: ApplymentBindBankDraft): boolean {
  return form.contact_id_doc_period_end.trim() === '长期'
}

function requiresSuperContactFields(form: ApplymentBindBankDraft, showContactFields?: boolean): boolean {
  return Boolean(showContactFields && form.contact_type === 'SUPER')
}

function normalizeDraft(
  accountType: ApplymentAccountType,
  draft?: PartialApplymentBindBankDraft | null
): ApplymentBindBankDraft {
  const base = createEmptyDraft(accountType)
  if (!draft) {
    return base
  }

  const nextAccountType = draft.account_type === 'ACCOUNT_TYPE_PRIVATE' || draft.account_type === 'ACCOUNT_TYPE_BUSINESS'
    ? draft.account_type
    : accountType

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

function inferProvinceCode(bankAddressCode: string): number {
  const cityCode = Number(bankAddressCode)
  if (!Number.isFinite(cityCode) || cityCode <= 0) {
    return 0
  }
  return Math.floor(cityCode / 10000) * 10000
}

function normalizeKeyword(value: string): string {
  return value.trim().toLowerCase()
}

function bankMatchesKeyword(bank: ApplymentBankOption, keyword: string): boolean {
  if (!keyword) {
    return true
  }
  const normalized = normalizeKeyword(keyword)
  return [bank.bank_alias, bank.account_bank, String(bank.account_bank_code)]
    .some((item) => item.toLowerCase().includes(normalized))
}

function branchMatchesKeyword(branch: ApplymentBranchOption, keyword: string): boolean {
  if (!keyword) {
    return true
  }
  const normalized = normalizeKeyword(keyword)
  return [branch.bank_branch_name, branch.bank_branch_id]
    .some((item) => item.toLowerCase().includes(normalized))
}

function buildBankDisplayLabel(bank: ApplymentBankOption): string {
  return bank.bank_alias
}

function decorateBankOption(bank: ApplymentBankOption): ApplymentBankViewOption {
  return {
    ...bank,
    display_label: buildBankDisplayLabel(bank)
  }
}

function decorateBankOptions(banks: ApplymentBankOption[]): ApplymentBankViewOption[] {
  return banks.map((bank) => decorateBankOption(bank))
}

function buildProvincePickerOptions(provinces: ApplymentProvinceOption[]): ApplymentPickerOption[] {
  return provinces.map((province) => ({
    label: province.province_name,
    value: province.province_code
  }))
}

function buildCityPickerOptions(cities: ApplymentCityOption[]): ApplymentPickerOption[] {
  return cities.map((city) => ({
    label: city.city_name,
    value: city.city_code
  }))
}

function buildPickerValue(value?: number): number[] {
  return value && value > 0 ? [value] : []
}

function buildSelectedBankLabel(form: ApplymentBindBankDraft): string {
  return form.bank_alias || form.account_bank
}

function hasSelectedBank(form: ApplymentBindBankDraft): boolean {
  return Boolean(form.bank_alias || form.account_bank)
}

function isManualCatalog(properties: ApplymentBankFormProperties): boolean {
  return Boolean(properties.manualCatalog || !String(properties.apiBasePath || '').trim())
}

function canLoadBankList(form: ApplymentBindBankDraft): boolean {
  return Boolean(form.account_number.trim())
}

function findSelectedBankIndex(banks: ApplymentBankOption[], form: ApplymentBindBankDraft): number {
  if (!banks.length || !form.bank_alias_code) {
    return 0
  }

  const index = banks.findIndex(
    (bank) => bank.bank_alias_code === form.bank_alias_code && bank.account_bank_code === form.account_bank_code
  )
  return index >= 0 ? index : 0
}

function findSelectedBranchIndex(branches: ApplymentBranchOption[], form: ApplymentBindBankDraft): number {
  if (!branches.length || !form.bank_branch_id) {
    return 0
  }

  const index = branches.findIndex((branch) => branch.bank_branch_id === form.bank_branch_id)
  return index >= 0 ? index : 0
}

function getSubmitBlockMessage(
  form: ApplymentBindBankDraft,
  showContactFields?: boolean,
  requireContactEmail?: boolean,
  requireAccountName: boolean = true,
  allowSavedAccountNumber: boolean = false,
  requireBankBranch: boolean = false,
  useManualCatalog: boolean = false
): string {
  if (!form.account_number.trim() && !allowSavedAccountNumber) {
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

  if (!form.need_bank_branch && !requireBankBranch && !useManualCatalog) {
    return ''
  }

  if (!form.bank_address_code.trim()) {
    return useManualCatalog ? '请先填写开户省份' : '请先选择开户城市'
  }

  if (useManualCatalog && !form.manual_bank_city.trim()) {
    return '请先填写开户城市'
  }

  if (!form.bank_alias_code.trim() && !form.bank_name.trim()) {
    return '请重新选择开户银行'
  }

  if (!form.bank_branch_id.trim() || !form.bank_name.trim()) {
    return '请先选择开户支行'
  }

  return ''
}

function canSubmitForm(
  form: ApplymentBindBankDraft,
  showContactFields?: boolean,
  requireContactEmail?: boolean,
  requireAccountName: boolean = true,
  allowSavedAccountNumber: boolean = false,
  requireBankBranch: boolean = false,
  useManualCatalog: boolean = false
): boolean {
  return !getSubmitBlockMessage(form, showContactFields, requireContactEmail, requireAccountName, allowSavedAccountNumber, requireBankBranch, useManualCatalog)
}

Component({
  properties: {
    apiBasePath: {
      type: String,
      value: ''
    },
    initialDraft: {
      type: Object,
      value: {}
    },
    defaultAccountType: {
      type: String,
      value: 'ACCOUNT_TYPE_BUSINESS'
    },
    preloadCatalogs: {
      type: Boolean,
      value: false,
      observer(preloadCatalogs: boolean) {
        if (preloadCatalogs) {
          void this.preloadSelectableCatalogs()
        }
      }
    },
    manualCatalog: {
      type: Boolean,
      value: false
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
    allowSavedAccountNumber: {
      type: Boolean,
      value: false
    },
    requireBankBranch: {
      type: Boolean,
      value: false
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
    savedAccountNumberMask: {
      type: String,
      value: ''
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
    privateBanks: [] as ApplymentBankViewOption[],
    businessBanks: [] as ApplymentBankViewOption[],
    filteredBanks: [] as ApplymentBankViewOption[],
    loadingBanks: false,
    recognizingBank: false,
    bankKeyword: '',
    provinces: [] as ApplymentProvinceOption[],
    cities: [] as ApplymentCityOption[],
    branches: [] as ApplymentBranchOption[],
    provincePickerOptions: [] as ApplymentPickerOption[],
    cityPickerOptions: [] as ApplymentPickerOption[],
    provincePickerValue: [] as number[],
    cityPickerValue: [] as number[],
    filteredBranches: [] as ApplymentBranchOption[],
    loadingProvinces: false,
    loadingCities: false,
    loadingBranches: false,
    showBankPicker: false,
    showProvincePicker: false,
    showCityPicker: false,
    showBranchPicker: false,
    selectedBankIndex: 0,
    selectedProvinceIndex: 0,
    selectedCityIndex: 0,
    selectedBranchIndex: 0,
    selectedProvinceCode: 0,
    selectedCityCode: 0,
    selectedProvinceLabel: '',
    selectedCityLabel: '',
    branchKeyword: '',
    contactDocCopyFeedbackState: 'idle',
    contactDocCopyFeedbackTitle: '',
    contactDocCopyFeedbackDescription: '',
    contactDocCopyBackFeedbackState: 'idle',
    contactDocCopyBackFeedbackTitle: '',
    contactDocCopyBackFeedbackDescription: '',
    canSubmit: false,
    submitBlockMessage: '',
    selectedBankLabel: '',
    hasSelectedBank: false,
    showAccountNumber: false,
    useManualCatalog: false
  },

  lifetimes: {
    attached() {
      void this.initializeForm()
    }
  },

  methods: {
    onToggleAccountNumberVisibility() {
      if (this.properties.allowSavedAccountNumber && this.properties.savedAccountNumberMask && !this.readForm().account_number.trim()) {
        return
      }
      this.setData({ showAccountNumber: !this.data.showAccountNumber })
    },

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
      this.setData(Object.assign({
        form: nextForm,
        selectedBankLabel: buildSelectedBankLabel(nextForm),
        hasSelectedBank: hasSelectedBank(nextForm)
      }, extraData))

      if (options?.syncSubmit !== false) {
        this.syncCanSubmit(nextForm)
      }
      if (options?.emitDraft !== false) {
        this.emitDraftChange(nextForm)
      }
    },

    async initializeForm() {
      const properties = this.properties as unknown as ApplymentBankFormProperties
      const accountType = properties.defaultAccountType
      const initialDraft = normalizeDraft(accountType, properties.initialDraft)
      const useManualCatalog = isManualCatalog(properties)

      this.setFormState(initialDraft, {
        useManualCatalog,
        canSubmit: canSubmitForm(
          initialDraft,
          properties.showContactFields,
          properties.requireContactEmail,
          properties.requireAccountName,
          properties.allowSavedAccountNumber,
          properties.requireBankBranch,
          useManualCatalog
        ),
        submitBlockMessage: getSubmitBlockMessage(
          initialDraft,
          properties.showContactFields,
          properties.requireContactEmail,
          properties.requireAccountName,
          properties.allowSavedAccountNumber,
          properties.requireBankBranch,
          useManualCatalog
        )
      }, { emitDraft: false, syncSubmit: false })

      if (!useManualCatalog) {
        await this.restoreDraftSelection(initialDraft)
      }
      if (!useManualCatalog && properties.preloadCatalogs) {
        void this.preloadSelectableCatalogs()
      }
      this.emitDraftChange(initialDraft)
    },

    async preloadSelectableCatalogs() {
      await this.ensureProvincesLoaded()
    },

    getBanksForType(accountType: ApplymentAccountType): ApplymentBankViewOption[] {
      return accountType === 'ACCOUNT_TYPE_PRIVATE'
        ? (this.data.privateBanks as ApplymentBankViewOption[])
        : (this.data.businessBanks as ApplymentBankViewOption[])
    },

    emitDraftChange(nextForm?: ApplymentBindBankDraft) {
      const form = nextForm || (this.data.form as ApplymentBindBankDraft)
      const manualCity = this.usesManualCatalog() ? form.manual_bank_city.trim() : ''
      this.triggerEvent('draftchange', {
        ...form,
        deposit_bank_province: this.data.selectedProvinceLabel || form.bank_address_code.trim() || undefined,
        deposit_bank_city: this.data.selectedCityLabel || manualCity || undefined
      })
    },

    getApiBasePath() {
      const properties = this.properties as unknown as ApplymentBankFormProperties
      return properties.apiBasePath
    },

    usesManualCatalog() {
      const properties = this.properties as unknown as ApplymentBankFormProperties
      return isManualCatalog(properties)
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

    async ensureBanksLoaded(accountType: ApplymentAccountType) {
      if (this.getBanksForType(accountType).length > 0) {
        this.updateBankFilter('')
        return
      }

      await this.loadBanks(accountType)
    },

    syncCanSubmit(nextForm?: ApplymentBindBankDraft) {
      const form = nextForm || (this.data.form as ApplymentBindBankDraft)
      const canSubmit = canSubmitForm(
        form,
        this.properties.showContactFields,
        this.properties.requireContactEmail,
        this.properties.requireAccountName,
        this.properties.allowSavedAccountNumber,
        this.properties.requireBankBranch,
        this.usesManualCatalog()
      )
      const submitBlockMessage = getSubmitBlockMessage(
        form,
        this.properties.showContactFields,
        this.properties.requireContactEmail,
        this.properties.requireAccountName,
        this.properties.allowSavedAccountNumber,
        this.properties.requireBankBranch,
        this.usesManualCatalog()
      )
      this.setData({ canSubmit, submitBlockMessage })
    },

    async restoreDraftSelection(draft: ApplymentBindBankDraft) {
      if (!draft.need_bank_branch || !draft.bank_alias_code || !draft.bank_address_code) {
        return
      }

      await this.ensureProvincesLoaded()

      const provinceCode = inferProvinceCode(draft.bank_address_code)
      const cityCode = Number(draft.bank_address_code)
      const provinces = this.data.provinces as ApplymentProvinceOption[]
      const selectedProvinceIndex = provinces.findIndex((province) => province.province_code === provinceCode)

      if (provinceCode > 0 && selectedProvinceIndex >= 0) {
        this.setData({
          selectedProvinceCode: provinceCode,
          selectedProvinceIndex,
          selectedProvinceLabel: provinces[selectedProvinceIndex]?.province_name || '',
          provincePickerValue: buildPickerValue(provinceCode)
        })
        await this.loadCities(provinceCode)
      }

      const cities = this.data.cities as ApplymentCityOption[]
      const selectedCityIndex = cities.findIndex((city) => city.city_code === cityCode)

      if (cityCode > 0 && selectedCityIndex >= 0) {
        this.setData({
          selectedCityCode: cityCode,
          selectedCityIndex,
          selectedCityLabel: cities[selectedCityIndex]?.city_name || '',
          cityPickerValue: buildPickerValue(cityCode)
        })
        await this.loadBranches(draft.bank_alias_code, cityCode)
      }

      if (!draft.bank_branch_id) {
        return
      }

      const branches = this.data.branches as ApplymentBranchOption[]
      const selectedBranchIndex = branches.findIndex((branch) => branch.bank_branch_id === draft.bank_branch_id)

      if (selectedBranchIndex < 0) {
        return
      }

      const nextForm: ApplymentBindBankDraft = {
        ...this.readForm(),
        bank_address_code: draft.bank_address_code,
        bank_branch_id: draft.bank_branch_id,
        bank_name: draft.bank_name
      }

      this.setFormState(nextForm, { selectedBranchIndex }, { emitDraft: false })
    },

    updateBankFilter(keyword?: string) {
      const nextKeyword = keyword ?? this.data.bankKeyword
      const filteredBanks = this.getBanksForType(this.data.form.account_type)
        .filter((bank: ApplymentBankViewOption) => bankMatchesKeyword(bank, nextKeyword))
        .slice(0, 100)
      const selectedBankIndex = findSelectedBankIndex(filteredBanks, this.data.form as ApplymentBindBankDraft)

      this.setData({
        bankKeyword: nextKeyword,
        filteredBanks,
        selectedBankIndex
      })
    },

    updateBranchFilter(keyword?: string) {
      const nextKeyword = keyword ?? this.data.branchKeyword
      const filteredBranches = (this.data.branches as ApplymentBranchOption[])
        .filter((branch) => branchMatchesKeyword(branch, nextKeyword))
        .slice(0, 100)
      const selectedBranchIndex = findSelectedBranchIndex(filteredBranches, this.data.form as ApplymentBindBankDraft)

      this.setData({
        branchKeyword: nextKeyword,
        filteredBranches,
        selectedBranchIndex
      })
    },

    clearResolvedBankSelection() {
      const currentForm = this.readForm()
      const nextForm: ApplymentBindBankDraft = {
        ...currentForm,
        account_bank: '',
        account_bank_code: 0,
        bank_alias: '',
        bank_alias_code: '',
        need_bank_branch: false,
        bank_address_code: '',
        bank_branch_id: '',
        bank_name: ''
      }

      this.setFormState(nextForm, {
        showBankPicker: false,
        showProvincePicker: false,
        showCityPicker: false,
        showBranchPicker: false,
        selectedBankIndex: 0,
        selectedProvinceIndex: 0,
        selectedCityIndex: 0,
        selectedBranchIndex: 0,
        selectedProvinceCode: 0,
        selectedCityCode: 0,
        selectedProvinceLabel: '',
        selectedCityLabel: '',
        provincePickerValue: [],
        cityPickerValue: [],
        cityPickerOptions: [],
        cities: [],
        branches: [],
        filteredBranches: [],
        branchKeyword: ''
      })
    },

    async loadBanks(accountType: ApplymentAccountType) {
      const existing = this.getBanksForType(accountType)
      if (existing.length > 0) {
        this.updateBankFilter('')
        return
      }
      if (this.usesManualCatalog()) {
        return
      }

      this.setData({ loadingBanks: true })
      try {
        const response = await listApplymentBanks(this.getApiBasePath(), accountType)
        const banks = decorateBankOptions(response.banks)
        if (accountType === 'ACCOUNT_TYPE_PRIVATE') {
          this.setData({ privateBanks: banks })
        } else {
          this.setData({ businessBanks: banks })
        }
        this.updateBankFilter('')
      } catch (error: unknown) {
        wx.showToast({
          title: getErrorUserMessage(error, '加载银行列表失败，请稍后重试'),
          icon: 'none'
        })
      } finally {
        this.setData({ loadingBanks: false })
      }
    },

    async ensureProvincesLoaded() {
      if ((this.data.provinces as ApplymentProvinceOption[]).length > 0) {
        return
      }
      if (this.usesManualCatalog()) {
        return
      }

      this.setData({ loadingProvinces: true })
      try {
        const response = await listApplymentProvinces(this.getApiBasePath())
        this.setData({
          provinces: response.provinces,
          provincePickerOptions: buildProvincePickerOptions(response.provinces)
        })
      } catch (error: unknown) {
        wx.showToast({
          title: getErrorUserMessage(error, '加载省份失败，请稍后重试'),
          icon: 'none'
        })
      } finally {
        this.setData({ loadingProvinces: false })
      }
    },

    async loadCities(provinceCode: number) {
      if (this.usesManualCatalog()) {
        return
      }
      this.setData({ loadingCities: true })
      try {
        const response = await listApplymentCities(this.getApiBasePath(), provinceCode)
        this.setData({
          cities: response.cities,
          cityPickerOptions: buildCityPickerOptions(response.cities),
          cityPickerValue: [],
          selectedCityIndex: 0,
          selectedCityCode: 0,
          selectedCityLabel: '',
          branches: [],
          filteredBranches: [],
          selectedBranchIndex: 0,
          branchKeyword: ''
        })
      } catch (error: unknown) {
        wx.showToast({
          title: getErrorUserMessage(error, '加载城市失败，请稍后重试'),
          icon: 'none'
        })
      } finally {
        this.setData({ loadingCities: false })
      }
    },

    async loadBranches(bankAliasCode: string, cityCode: number) {
      if (this.usesManualCatalog()) {
        return
      }
      this.setData({ loadingBranches: true })
      try {
        const response = await listApplymentBankBranches(this.getApiBasePath(), bankAliasCode, cityCode)
        const currentForm = this.readForm()
        const nextForm: ApplymentBindBankDraft = {
          ...currentForm,
          account_bank: response.account_bank || currentForm.account_bank,
          account_bank_code: response.account_bank_code || currentForm.account_bank_code,
          bank_alias: response.bank_alias || currentForm.bank_alias,
          bank_alias_code: response.bank_alias_code || currentForm.bank_alias_code,
          need_bank_branch: true,
          bank_address_code: String(cityCode),
          bank_branch_id: '',
          bank_name: ''
        }

        this.setFormState(nextForm, {
          branches: response.branches,
          filteredBranches: response.branches,
          selectedBranchIndex: 0,
          branchKeyword: ''
        })
      } catch (error: unknown) {
        wx.showToast({
          title: getErrorUserMessage(error, '加载支行失败，请稍后重试'),
          icon: 'none'
        })
      } finally {
        this.setData({ loadingBranches: false })
      }
    },

    resetBankSelection(accountType: ApplymentAccountType) {
      const currentForm = this.readForm()
      const nextForm: ApplymentBindBankDraft = {
        ...currentForm,
        account_type: accountType,
        account_bank: '',
        account_bank_code: 0,
        bank_alias: '',
        bank_alias_code: '',
        need_bank_branch: false,
        bank_address_code: '',
        bank_branch_id: '',
        bank_name: ''
      }

      this.setFormState(nextForm, {
        bankKeyword: '',
        filteredBanks: this.getBanksForType(accountType).slice(0, 100),
        showBankPicker: false,
        showProvincePicker: false,
        showCityPicker: false,
        showBranchPicker: false,
        selectedBankIndex: 0,
        provinces: [],
        cities: [],
        branches: [],
        filteredBranches: [],
        selectedBranchIndex: 0,
        selectedProvinceIndex: 0,
        selectedCityIndex: 0,
        selectedProvinceCode: 0,
        selectedCityCode: 0,
        selectedProvinceLabel: '',
        selectedCityLabel: '',
        provincePickerOptions: [],
        cityPickerOptions: [],
        provincePickerValue: [],
        cityPickerValue: [],
        branchKeyword: ''
      })
    },

    async applySelectedBank(bank: ApplymentBankViewOption) {
      const currentForm = this.readForm()
      const selectedCityCode = Number(currentForm.bank_address_code || this.data.selectedCityCode || 0)
      const nextForm: ApplymentBindBankDraft = {
        ...currentForm,
        account_bank: bank.account_bank,
        account_bank_code: bank.account_bank_code,
        bank_alias: bank.bank_alias,
        bank_alias_code: bank.bank_alias_code,
        need_bank_branch: bank.need_bank_branch,
        bank_address_code: bank.need_bank_branch ? currentForm.bank_address_code : '',
        bank_branch_id: '',
        bank_name: ''
      }
      const nextBankKeyword = bankMatchesKeyword(bank, this.data.bankKeyword) ? this.data.bankKeyword : ''
      let filteredBanks = this.getBanksForType(nextForm.account_type)
        .filter((item: ApplymentBankViewOption) => bankMatchesKeyword(item, nextBankKeyword))
        .slice(0, 100)
      if (!filteredBanks.length) {
        filteredBanks = [bank]
      }
      const selectedBankIndex = findSelectedBankIndex(filteredBanks, nextForm)

      this.setFormState(nextForm, {
        bankKeyword: nextBankKeyword,
        filteredBanks,
        selectedBankIndex
      })

      if (bank.need_bank_branch) {
        await this.ensureProvincesLoaded()
        if (selectedCityCode > 0) {
          await this.loadBranches(bank.bank_alias_code, selectedCityCode)
        }
      } else {
        this.setData({
          selectedBankIndex: 0,
          selectedProvinceIndex: 0,
          selectedCityIndex: 0,
          selectedBranchIndex: 0,
          selectedProvinceCode: 0,
          selectedCityCode: 0,
          selectedProvinceLabel: '',
          selectedCityLabel: '',
          provincePickerValue: [],
          cityPickerValue: [],
          cityPickerOptions: [],
          cities: [],
          branches: [],
          filteredBranches: [],
          branchKeyword: ''
        })
      }
    },

    onTextFieldChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
      const field = String(e.currentTarget.dataset.field || '')
      if (!field) {
        return
      }

      const value = e.detail.value || ''
      let nextForm: ApplymentBindBankDraft = {
        ...this.readForm(),
        [field]: field === 'contact_id_doc_period_end' && value.trim() === '长期'
          ? '长期'
          : value
      }

      if (field === 'account_number' && nextForm.account_type === 'ACCOUNT_TYPE_PRIVATE') {
        nextForm = {
          ...nextForm,
          account_bank: '',
          account_bank_code: 0,
          bank_alias: '',
          bank_alias_code: '',
          need_bank_branch: false,
          bank_address_code: '',
          bank_branch_id: '',
          bank_name: ''
        }

        this.setFormState(nextForm, {
          showBankPicker: false,
          showProvincePicker: false,
          showCityPicker: false,
          showBranchPicker: false,
          selectedBankIndex: 0,
          selectedProvinceIndex: 0,
          selectedCityIndex: 0,
          selectedBranchIndex: 0,
          selectedProvinceCode: 0,
          selectedCityCode: 0,
          selectedProvinceLabel: '',
          selectedCityLabel: '',
          provincePickerValue: [],
          cityPickerValue: [],
          cityPickerOptions: [],
          cities: [],
          branches: [],
          filteredBranches: [],
          branchKeyword: ''
        })
        this.updateBankFilter('')
        return
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

    async onAccountTypeSelect(e: WechatMiniprogram.TouchEvent) {
      const { value } = e.currentTarget.dataset as { value?: ApplymentAccountType }
      const accountType = value
      if (!accountType || accountType === this.readForm().account_type) {
        return
      }

      this.resetBankSelection(accountType)
      const properties = this.properties as unknown as ApplymentBankFormProperties
      if (!this.usesManualCatalog() && properties.preloadCatalogs) {
        await this.preloadSelectableCatalogs()
      }
    },

    onAccountNumberBlur() {
      if (this.usesManualCatalog()) {
        return
      }
      const form = this.readForm()
      if (form.account_type !== 'ACCOUNT_TYPE_PRIVATE') {
        return
      }
      if (this.data.recognizingBank || form.account_number.trim().length < 8 || form.bank_alias) {
        return
      }
      void this.onRecognizeBank()
    },

    onBankKeywordChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
      this.updateBankFilter(e.detail.value || '')
    },

    onBranchKeywordChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
      this.updateBranchFilter(e.detail.value || '')
    },

    async onRecognizeBank() {
      const form = this.readForm()
      const accountNumber = form.account_number.trim()
      if (this.usesManualCatalog() || form.account_type !== 'ACCOUNT_TYPE_PRIVATE' || accountNumber.length < 8) {
        return
      }

      this.clearResolvedBankSelection()
      this.setData({
        recognizingBank: true,
        selectedBankIndex: 0
      })

      try {
        const response = await searchApplymentBanksByAccount(this.getApiBasePath(), form.account_type, accountNumber)
        const matches = decorateBankOptions(response.matches)
        const selection = resolveRecognizedBankSelection(matches)

        if (selection.bank) {
          await this.applySelectedBank(selection.bank)
        }

        if (selection.shouldOpenPicker) {
          const selectedBankIndex = findSelectedBankIndex(selection.filteredBanks, this.readForm())
          this.setData({
            filteredBanks: selection.filteredBanks,
            showBankPicker: true,
            selectedBankIndex
          })
          return
        }

        if (selection.bank) {
          return
        }

        await this.ensureBanksLoaded(form.account_type)
        this.updateBankFilter('')
      } catch (error: unknown) {
        wx.showToast({
          title: getErrorUserMessage(error, '识别开户银行失败，请稍后重试'),
          icon: 'none'
        })
      } finally {
        this.setData({ recognizingBank: false })
      }
    },

    async onOpenBankPicker() {
      if (this.usesManualCatalog()) {
        return
      }
      const form = this.readForm()
      if (!canLoadBankList(form)) {
        return
      }

      await this.ensureBanksLoaded(form.account_type)
      if (this.data.loadingBanks || !(this.data.filteredBanks as ApplymentBankViewOption[]).length) {
        return
      }
      this.setData({ showBankPicker: true })
    },

    onCloseBankPicker() {
      this.setData({ showBankPicker: false })
    },

    onBankPickerVisibleChange(e: ApplymentPickerVisibleChangeEvent) {
      if (e.detail?.visible === false) {
        this.onCloseBankPicker()
      }
    },

    async onSelectBankOption(e: WechatMiniprogram.BaseEvent) {
      const index = Number(e.currentTarget.dataset.index)
      const bank = (this.data.filteredBanks as ApplymentBankViewOption[])[index]
      if (!bank) {
        this.onCloseBankPicker()
        return
      }
      await this.applySelectedBank(bank)
      this.onCloseBankPicker()
    },

    async onOpenProvincePicker() {
      if (this.usesManualCatalog()) {
        return
      }
      await this.ensureProvincesLoaded()
      const provinces = this.data.provinces as ApplymentProvinceOption[]
      if (!provinces.length) {
        return
      }
      this.setData({
        showProvincePicker: true,
        provincePickerValue: buildPickerValue(this.data.selectedProvinceCode || provinces[0]?.province_code)
      })
    },

    onCloseProvincePicker() {
      this.setData({ showProvincePicker: false })
    },

    async applySelectedProvince(provinceCode: number) {
      if (provinceCode === this.data.selectedProvinceCode && this.data.selectedCityCode) {
        return
      }

      const provinces = this.data.provinces as ApplymentProvinceOption[]
      const selectedIndex = provinces.findIndex((province) => province.province_code === provinceCode)
      const province = provinces[selectedIndex]
      if (!province) {
        return
      }

      const nextForm: ApplymentBindBankDraft = {
        ...this.readForm(),
        bank_address_code: '',
        bank_branch_id: '',
        bank_name: ''
      }

      this.setFormState(nextForm, {
        selectedProvinceIndex: selectedIndex,
        selectedProvinceCode: province.province_code,
        selectedProvinceLabel: province.province_name,
        provincePickerValue: buildPickerValue(province.province_code),
        selectedCityIndex: 0,
        selectedCityCode: 0,
        selectedCityLabel: '',
        cityPickerValue: [],
        cityPickerOptions: [],
        selectedBranchIndex: 0,
        cities: [],
        branches: [],
        filteredBranches: [],
        branchKeyword: ''
      })
      await this.loadCities(province.province_code)
    },

    onProvincePickerPick(e: ApplymentPickerChangeEvent) {
      this.setData({
        provincePickerValue: Array.isArray(e.detail?.value) ? e.detail.value.map((value) => Number(value)) : []
      })
    },

    async onProvincePickerConfirm(e: ApplymentPickerChangeEvent) {
      const provinceCode = Number(e.detail?.value?.[0] || 0)
      if (provinceCode > 0) {
        await this.applySelectedProvince(provinceCode)
      }
      this.onCloseProvincePicker()
    },

    onOpenCityPicker() {
      if (this.usesManualCatalog()) {
        return
      }
      if (!this.data.selectedProvinceCode || !(this.data.cities as ApplymentCityOption[]).length) {
        return
      }
      const cities = this.data.cities as ApplymentCityOption[]
      this.setData({
        showCityPicker: true,
        cityPickerValue: buildPickerValue(this.data.selectedCityCode || cities[0]?.city_code)
      })
    },

    onCloseCityPicker() {
      this.setData({ showCityPicker: false })
    },

    async applySelectedCity(cityCode: number) {
      if (cityCode === this.data.selectedCityCode && this.readForm().bank_name) {
        return
      }

      const cities = this.data.cities as ApplymentCityOption[]
      const selectedIndex = cities.findIndex((city) => city.city_code === cityCode)
      const city = cities[selectedIndex]
      if (!city) {
        return
      }

      const nextForm: ApplymentBindBankDraft = {
        ...this.readForm(),
        bank_address_code: String(city.city_code),
        bank_branch_id: '',
        bank_name: ''
      }

      this.setFormState(nextForm, {
        selectedCityIndex: selectedIndex,
        selectedCityCode: city.city_code,
        selectedCityLabel: city.city_name,
        cityPickerValue: buildPickerValue(city.city_code),
        branches: [],
        filteredBranches: [],
        selectedBranchIndex: 0,
        branchKeyword: ''
      })

      if (nextForm.bank_alias_code) {
        await this.loadBranches(nextForm.bank_alias_code, city.city_code)
      }
    },

    onCityPickerPick(e: ApplymentPickerChangeEvent) {
      this.setData({
        cityPickerValue: Array.isArray(e.detail?.value) ? e.detail.value.map((value) => Number(value)) : []
      })
    },

    async onCityPickerConfirm(e: ApplymentPickerChangeEvent) {
      const cityCode = Number(e.detail?.value?.[0] || 0)
      if (cityCode > 0) {
        await this.applySelectedCity(cityCode)
      }
      this.onCloseCityPicker()
    },

    onOpenBranchPicker() {
      if (this.usesManualCatalog()) {
        return
      }
      if (!this.data.selectedCityCode || !(this.data.filteredBranches as ApplymentBranchOption[]).length) {
        return
      }
      this.setData({ showBranchPicker: true })
    },

    onCloseBranchPicker() {
      this.setData({ showBranchPicker: false })
    },

    onSelectBranchOption(e: WechatMiniprogram.BaseEvent) {
      const index = Number(e.currentTarget.dataset.index)
      const branch = (this.data.filteredBranches as ApplymentBranchOption[])[index]
      if (!branch) {
        this.onCloseBranchPicker()
        return
      }

      const nextForm: ApplymentBindBankDraft = {
        ...this.readForm(),
        bank_address_code: String(this.data.selectedCityCode || ''),
        bank_branch_id: branch.bank_branch_id,
        bank_name: branch.bank_branch_name
      }

      this.setFormState(nextForm, {
        selectedBranchIndex: index
      })
      this.onCloseBranchPicker()
    },

    onBranchPickerVisibleChange(e: ApplymentPickerVisibleChangeEvent) {
      if (e.detail?.visible === false) {
        this.onCloseBranchPicker()
      }
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
        properties.requireAccountName,
        properties.allowSavedAccountNumber,
        properties.requireBankBranch,
        this.usesManualCatalog()
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
        need_bank_branch: (form.need_bank_branch || properties.requireBankBranch) || undefined,
        bank_address_code: form.bank_address_code.trim() || undefined,
        deposit_bank_province: this.data.selectedProvinceLabel || form.bank_address_code.trim() || undefined,
        deposit_bank_city: this.data.selectedCityLabel || (this.usesManualCatalog() ? form.manual_bank_city.trim() : '') || undefined,
        manual_bank_city: this.usesManualCatalog() ? form.manual_bank_city.trim() || undefined : undefined,
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
    },

    buildBankDisplayLabel,

    buildSelectedBankLabel
  }
})
