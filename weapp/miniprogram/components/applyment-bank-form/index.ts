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
  if (bank.account_bank === bank.bank_alias) {
    return bank.bank_alias
  }
  return `${bank.bank_alias} · 微信开户银行填写为${bank.account_bank}`
}

function buildSelectedBankLabel(form: ApplymentBindBankDraft): string {
  return form.bank_alias || form.account_bank
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
    defaultAccountType: {
      type: String,
      value: 'ACCOUNT_TYPE_BUSINESS'
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
    privateBanks: [] as ApplymentBankOption[],
    businessBanks: [] as ApplymentBankOption[],
    filteredBanks: [] as ApplymentBankOption[],
    loadingBanks: false,
    recognizingBank: false,
    recognizedBanks: [] as ApplymentBankOption[],
    recognitionHint: '',
    bankKeyword: '',
    provinces: [] as ApplymentProvinceOption[],
    cities: [] as ApplymentCityOption[],
    branches: [] as ApplymentBranchOption[],
    filteredBranches: [] as ApplymentBranchOption[],
    loadingProvinces: false,
    loadingCities: false,
    loadingBranches: false,
    selectedProvinceIndex: 0,
    selectedCityIndex: 0,
    selectedProvinceCode: 0,
    selectedCityCode: 0,
    branchKeyword: '',
    canSubmit: false
  },

  lifetimes: {
    attached() {
      const accountType = this.properties.defaultAccountType as ApplymentAccountType
      this.setData({
        form: createEmptyDraft(accountType),
        canSubmit: false
      })
      void this.loadBanks(accountType)
    }
  },

  methods: {
    getBanksForType(accountType: ApplymentAccountType): ApplymentBankOption[] {
      return accountType === 'ACCOUNT_TYPE_PRIVATE'
        ? (this.data.privateBanks as ApplymentBankOption[])
        : (this.data.businessBanks as ApplymentBankOption[])
    },

    syncCanSubmit(nextForm?: ApplymentBindBankDraft) {
      const form = nextForm || (this.data.form as ApplymentBindBankDraft)
      this.setData({ canSubmit: canSubmitForm(form) })
    },

    updateBankFilter(keyword?: string) {
      const nextKeyword = keyword ?? this.data.bankKeyword
      const filteredBanks = this.getBanksForType(this.data.form.account_type)
        .filter((bank) => bankMatchesKeyword(bank, nextKeyword))

      this.setData({
        bankKeyword: nextKeyword,
        filteredBanks
      })
    },

    updateBranchFilter(keyword?: string) {
      const nextKeyword = keyword ?? this.data.branchKeyword
      const filteredBranches = (this.data.branches as ApplymentBranchOption[])
        .filter((branch) => branchMatchesKeyword(branch, nextKeyword))

      this.setData({
        branchKeyword: nextKeyword,
        filteredBranches
      })
    },

    clearResolvedBankSelection(options?: { keepRecognitionHint?: boolean }) {
      const currentForm = this.data.form as ApplymentBindBankDraft
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

      this.setData({
        form: nextForm,
        recognizedBanks: [],
        recognitionHint: options?.keepRecognitionHint ? this.data.recognitionHint : '',
        selectedProvinceIndex: 0,
        selectedCityIndex: 0,
        selectedProvinceCode: 0,
        selectedCityCode: 0,
        cities: [],
        branches: [],
        filteredBranches: [],
        branchKeyword: ''
      })
      this.syncCanSubmit(nextForm)
    },

    async loadBanks(accountType: ApplymentAccountType) {
      const existing = this.getBanksForType(accountType)
      if (existing.length > 0) {
        this.updateBankFilter('')
        return
      }

      this.setData({ loadingBanks: true })
      try {
        const response = await listApplymentBanks(this.properties.apiBasePath, accountType)
        if (accountType === 'ACCOUNT_TYPE_PRIVATE') {
          this.setData({ privateBanks: response.banks })
        } else {
          this.setData({ businessBanks: response.banks })
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
        const response = await listApplymentProvinces(this.properties.apiBasePath)
        this.setData({ provinces: response.provinces })
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
        const response = await listApplymentCities(this.properties.apiBasePath, provinceCode)
        this.setData({
          cities: response.cities,
          selectedCityIndex: 0,
          selectedCityCode: 0,
          branches: [],
          filteredBranches: [],
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
        const response = await listApplymentBankBranches(this.properties.apiBasePath, bankAliasCode, cityCode)
        const nextForm: ApplymentBindBankDraft = {
          ...(this.data.form as ApplymentBindBankDraft),
          account_bank: response.account_bank || this.data.form.account_bank,
          account_bank_code: response.account_bank_code || this.data.form.account_bank_code,
          bank_alias: response.bank_alias || this.data.form.bank_alias,
          bank_alias_code: response.bank_alias_code || this.data.form.bank_alias_code,
          bank_address_code: String(cityCode),
          bank_branch_id: '',
          bank_name: ''
        }

        this.setData({
          form: nextForm,
          branches: response.branches,
          filteredBranches: response.branches,
          branchKeyword: ''
        })
        this.syncCanSubmit(nextForm)
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
      const currentForm = this.data.form as ApplymentBindBankDraft
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

      this.setData({
        form: nextForm,
        recognizedBanks: [],
        recognitionHint: '',
        bankKeyword: '',
        filteredBanks: this.getBanksForType(accountType),
        provinces: [],
        cities: [],
        branches: [],
        filteredBranches: [],
        selectedProvinceIndex: 0,
        selectedCityIndex: 0,
        selectedProvinceCode: 0,
        selectedCityCode: 0,
        branchKeyword: ''
      })
      this.syncCanSubmit(nextForm)
    },

    async applySelectedBank(bank: ApplymentBankOption, hint?: string) {
      const currentForm = this.data.form as ApplymentBindBankDraft
      const nextForm: ApplymentBindBankDraft = {
        ...currentForm,
        account_bank: bank.account_bank,
        account_bank_code: bank.account_bank_code,
        bank_alias: bank.bank_alias,
        bank_alias_code: bank.bank_alias_code,
        need_bank_branch: bank.need_bank_branch,
        bank_address_code: bank.need_bank_branch ? currentForm.bank_address_code : '',
        bank_branch_id: bank.need_bank_branch ? currentForm.bank_branch_id : '',
        bank_name: bank.need_bank_branch ? currentForm.bank_name : ''
      }

      this.setData({
        form: nextForm,
        recognitionHint: hint || (bank.need_bank_branch ? '这家银行还需要继续选择开户地址和支行。' : '开户银行已确定，可直接继续填写并提交。')
      })
      this.syncCanSubmit(nextForm)

      if (bank.need_bank_branch) {
        await this.ensureProvincesLoaded()
      } else {
        this.setData({
          selectedProvinceIndex: 0,
          selectedCityIndex: 0,
          selectedProvinceCode: 0,
          selectedCityCode: 0,
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
        ...(this.data.form as ApplymentBindBankDraft),
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

        this.setData({
          form: nextForm,
          recognizedBanks: [],
          recognitionHint: '',
          selectedProvinceIndex: 0,
          selectedCityIndex: 0,
          selectedProvinceCode: 0,
          selectedCityCode: 0,
          cities: [],
          branches: [],
          filteredBranches: [],
          branchKeyword: ''
        })
        this.syncCanSubmit(nextForm)
        return
      }

      this.setData({ form: nextForm })
      this.syncCanSubmit(nextForm)
    },

    async onAccountTypeChange(e: WechatMiniprogram.CustomEvent<{ value: ApplymentAccountType }>) {
      const accountType = e.detail.value as ApplymentAccountType
      this.resetBankSelection(accountType)
      if (accountType === 'ACCOUNT_TYPE_BUSINESS') {
        this.setData({ recognitionHint: '对公账户暂不支持卡号自动识别，请直接在下方选择开户银行。' })
      }
      await this.loadBanks(accountType)
    },

    onAccountNumberBlur() {
      const form = this.data.form as ApplymentBindBankDraft
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
      const form = this.data.form as ApplymentBindBankDraft
      const accountNumber = form.account_number.trim()
      if (form.account_type !== 'ACCOUNT_TYPE_PRIVATE' || accountNumber.length < 8) {
        wx.showToast({ title: '请输入至少 8 位银行卡号后再识别', icon: 'none' })
        return
      }

      this.clearResolvedBankSelection()
      this.setData({
        recognizingBank: true,
        recognizedBanks: [],
        recognitionHint: ''
      })

      try {
        const response = await searchApplymentBanksByAccount(this.properties.apiBasePath, form.account_type, accountNumber)
        this.setData({ recognizedBanks: response.matches })

        if (response.matches.length === 1) {
          await this.applySelectedBank(response.matches[0], '已根据银行卡号自动识别并回填开户银行，请核对后继续。')
          return
        }

        if (response.matches.length > 1) {
          this.setData({ recognitionHint: `系统没法仅靠卡号精确定位开户银行，已帮你缩小到 ${response.matches.length} 家候选，请手动确认具体银行。` })
          return
        }

        this.setData({ recognitionHint: '暂时无法自动识别开户银行，请在下方银行列表中手动选择。' })
      } catch (error: unknown) {
        wx.showToast({
          title: getErrorUserMessage(error, '识别开户银行失败，请稍后重试'),
          icon: 'none'
        })
      } finally {
        this.setData({ recognizingBank: false })
      }
    },

    async onSelectRecognizedBank(e: WechatMiniprogram.BaseEvent) {
      const index = Number(e.currentTarget.dataset.index)
      const bank = (this.data.recognizedBanks as ApplymentBankOption[])[index]
      if (!bank) {
        return
      }
      await this.applySelectedBank(bank)
    },

    async onSelectBank(e: WechatMiniprogram.BaseEvent) {
      const index = Number(e.currentTarget.dataset.index)
      const bank = (this.data.filteredBanks as ApplymentBankOption[])[index]
      if (!bank) {
        return
      }
      await this.applySelectedBank(bank)
    },

    async onProvinceChange(e: WechatMiniprogram.PickerChange) {
      const selectedIndex = Number(e.detail.value)
      const province = (this.data.provinces as ApplymentProvinceOption[])[selectedIndex]
      if (!province) {
        return
      }

      const nextForm: ApplymentBindBankDraft = {
        ...(this.data.form as ApplymentBindBankDraft),
        bank_address_code: '',
        bank_branch_id: '',
        bank_name: ''
      }

      this.setData({
        selectedProvinceIndex: selectedIndex,
        selectedProvinceCode: province.province_code,
        form: nextForm,
        selectedCityIndex: 0,
        selectedCityCode: 0,
        cities: [],
        branches: [],
        filteredBranches: [],
        branchKeyword: ''
      })
      this.syncCanSubmit(nextForm)
      await this.loadCities(province.province_code)
    },

    async onCityChange(e: WechatMiniprogram.PickerChange) {
      const selectedIndex = Number(e.detail.value)
      const city = (this.data.cities as ApplymentCityOption[])[selectedIndex]
      if (!city) {
        return
      }

      const nextForm: ApplymentBindBankDraft = {
        ...(this.data.form as ApplymentBindBankDraft),
        bank_address_code: String(city.city_code),
        bank_branch_id: '',
        bank_name: ''
      }

      this.setData({
        selectedCityIndex: selectedIndex,
        selectedCityCode: city.city_code,
        form: nextForm,
        branches: [],
        filteredBranches: [],
        branchKeyword: ''
      })
      this.syncCanSubmit(nextForm)

      if (nextForm.bank_alias_code) {
        await this.loadBranches(nextForm.bank_alias_code, city.city_code)
      }
    },

    onSelectBranch(e: WechatMiniprogram.BaseEvent) {
      const index = Number(e.currentTarget.dataset.index)
      const branch = (this.data.filteredBranches as ApplymentBranchOption[])[index]
      if (!branch) {
        return
      }

      const nextForm: ApplymentBindBankDraft = {
        ...(this.data.form as ApplymentBindBankDraft),
        bank_address_code: String(this.data.selectedCityCode || ''),
        bank_branch_id: branch.bank_branch_id,
        bank_name: branch.bank_branch_name
      }

      this.setData({ form: nextForm })
      this.syncCanSubmit(nextForm)
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

    buildSelectedBankLabel,

    isBankSelected(bank: ApplymentBankOption): boolean {
      const form = this.data.form as ApplymentBindBankDraft
      return form.bank_alias_code === bank.bank_alias_code && form.account_bank_code === bank.account_bank_code
    },

    isBranchSelected(branch: ApplymentBranchOption): boolean {
      return this.data.form.bank_branch_id === branch.bank_branch_id
    }
  }
})