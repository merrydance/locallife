import {
  type ApplymentAccountType,
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
import { getErrorUserMessage } from '../../utils/user-facing'

interface ApplymentBindBankDraft {
  account_type: ApplymentAccountType
  account_bank: string
  account_bank_code: number
  bank_alias: string
  bank_alias_code: string
  need_bank_branch: boolean
  bank_address_code: string
  bank_branch_id: string
  bank_name: string
  account_number: string
  account_name: string
}

type PartialApplymentBindBankDraft = Partial<ApplymentBindBankDraft>

interface ApplymentBankFormProperties {
  apiBasePath: string
  initialDraft?: PartialApplymentBindBankDraft
  defaultAccountType: ApplymentAccountType
  preloadCatalogs?: boolean
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

function createEmptyDraft(accountType: ApplymentAccountType): ApplymentBindBankDraft {
  return {
    account_type: accountType,
    account_bank: '',
    account_bank_code: 0,
    bank_alias: '',
    bank_alias_code: '',
    need_bank_branch: false,
    bank_address_code: '',
    bank_branch_id: '',
    bank_name: '',
    account_number: '',
    account_name: ''
  }
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
    bank_branch_id: typeof draft.bank_branch_id === 'string' ? draft.bank_branch_id : '',
    bank_name: typeof draft.bank_name === 'string' ? draft.bank_name : '',
    account_number: typeof draft.account_number === 'string' ? draft.account_number : '',
    account_name: typeof draft.account_name === 'string' ? draft.account_name : ''
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

function canSubmitForm(form: ApplymentBindBankDraft): boolean {
  const baseValid = Boolean(
    form.account_bank.trim() &&
    form.account_number.trim() &&
    form.account_name.trim()
  )

  if (!baseValid) {
    return false
  }

  if (!form.need_bank_branch) {
    return true
  }

  return Boolean(
    form.bank_address_code.trim() &&
    form.bank_branch_id.trim() &&
    form.bank_name.trim() &&
    form.bank_alias_code.trim()
  )
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
    canSubmit: false,
    selectedBankLabel: '',
    hasSelectedBank: false
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

      this.setFormState(initialDraft, { canSubmit: canSubmitForm(initialDraft) }, { emitDraft: false, syncSubmit: false })

      await this.restoreDraftSelection(initialDraft)
      if (properties.preloadCatalogs) {
        void this.preloadSelectableCatalogs()
      }
      this.emitDraftChange(initialDraft)
    },

    async preloadSelectableCatalogs() {
      const form = this.readForm()
      await Promise.all([
        this.ensureBanksLoaded(form.account_type),
        this.ensureProvincesLoaded()
      ])
    },

    getBanksForType(accountType: ApplymentAccountType): ApplymentBankViewOption[] {
      return accountType === 'ACCOUNT_TYPE_PRIVATE'
        ? (this.data.privateBanks as ApplymentBankViewOption[])
        : (this.data.businessBanks as ApplymentBankViewOption[])
    },

    emitDraftChange(nextForm?: ApplymentBindBankDraft) {
      const form = nextForm || (this.data.form as ApplymentBindBankDraft)
      this.triggerEvent('draftchange', { ...form })
    },

    getApiBasePath() {
      const properties = this.properties as unknown as ApplymentBankFormProperties
      return properties.apiBasePath
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
      this.setData({ canSubmit: canSubmitForm(form) })
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
        filteredBanks: this.getBanksForType(accountType),
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
        [field]: value
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

    async onAccountTypeSelect(e: WechatMiniprogram.TouchEvent) {
      const { value } = e.currentTarget.dataset as { value?: ApplymentAccountType }
      const accountType = value
      if (!accountType || accountType === this.readForm().account_type) {
        return
      }

      this.resetBankSelection(accountType)
      const properties = this.properties as unknown as ApplymentBankFormProperties
      if (properties.preloadCatalogs) {
        await this.preloadSelectableCatalogs()
      }
    },

    onAccountNumberBlur() {
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
      if (form.account_type !== 'ACCOUNT_TYPE_PRIVATE' || accountNumber.length < 8) {
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

        if (matches.length === 1) {
          await this.applySelectedBank(matches[0])
          return
        }

        if (matches.length > 1) {
          this.setData({
            filteredBanks: matches,
            showBankPicker: true,
            selectedBankIndex: 0
          })
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
      const form = this.readForm()
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

    onSubmit() {
      const form = this.data.form as ApplymentBindBankDraft
      if (!canSubmitForm(form)) {
        wx.showToast({ title: '请先补全必填信息', icon: 'none' })
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
        bank_branch_id: form.bank_branch_id.trim() || undefined,
        bank_name: form.bank_name.trim() || undefined,
        account_number: form.account_number.trim(),
        account_name: form.account_name.trim()
      }

      this.triggerEvent('submit', payload)
    },

    buildBankDisplayLabel,

    buildSelectedBankLabel
  }
})