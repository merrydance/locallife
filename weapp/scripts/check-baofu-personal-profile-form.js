const fs = require('fs')
const path = require('path')
const vm = require('vm')
const ts = require('typescript')

const repoRoot = path.resolve(__dirname, '..')
const sourcePath = path.join(repoRoot, 'miniprogram/pages/merchant/_main_shared/services/baofu-account-profile-form.ts')
const bankFormWxmlPath = path.join(repoRoot, 'miniprogram/pages/merchant/finance/settlement-account/submit/_components/applyment-bank-form/index.wxml')
const bankFormTsPath = path.join(repoRoot, 'miniprogram/pages/merchant/finance/settlement-account/submit/_components/applyment-bank-form/index.ts')
const applymentBankApiPaths = [
  path.join(repoRoot, 'miniprogram/pages/merchant/_main_shared/api/applyment-bank.ts'),
  path.join(repoRoot, 'miniprogram/pages/operator/_main_shared/api/applyment-bank.ts'),
  path.join(repoRoot, 'miniprogram/pages/rider/_main_shared/api/applyment-bank.ts')
]
const submitFormPaths = [
  path.join(repoRoot, 'miniprogram/pages/merchant/finance/settlement-account/submit/index.wxml'),
  path.join(repoRoot, 'miniprogram/pages/merchant/finance/settlement-account/submit/index.ts'),
  path.join(repoRoot, 'miniprogram/pages/operator/finance/settlement-account/submit/index.wxml'),
  path.join(repoRoot, 'miniprogram/pages/operator/finance/settlement-account/submit/index.ts'),
  path.join(repoRoot, 'miniprogram/pages/rider/settlement-account/submit/index.wxml'),
  path.join(repoRoot, 'miniprogram/pages/rider/settlement-account/submit/index.ts'),
  bankFormWxmlPath,
  bankFormTsPath
]

function loadModule() {
  const source = fs.readFileSync(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018,
      strict: true
    }
  }).outputText
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}

function main() {
  const {
    emptyBaofuPersonalProfileForm,
    buildBaofuEnterpriseFormFromDefaults,
    buildBaofuEnterpriseBankDraftFromDefaults,
    buildBaofuEnterpriseProfilePayload,
    buildBaofuPersonalFormFromDefaults,
    buildBaofuPersonalProfilePayload,
    validateBaofuPersonalProfileForm
  } = loadModule()

  const empty = emptyBaofuPersonalProfileForm()
  assert(!Object.prototype.hasOwnProperty.call(empty, 'bank_name'), 'personal profile form should not expose bank_name')

  const riderPayload = buildBaofuPersonalProfilePayload('rider', {
    name: '张三',
    certificate_no: '110101199001010011',
    bank_account_no: '6222020202020202',
    bank_mobile: '13800138000'
  })
  assert(!Object.prototype.hasOwnProperty.call(riderPayload, 'bank_name'), 'rider payload should not include bank_name')
  assert(riderPayload.bank_account_number === '6222020202020202', 'rider payload should still carry bank_account_number')

  const operatorPayload = buildBaofuPersonalProfilePayload('operator', {
    name: '李四',
    certificate_no: '110101199001010022',
    bank_account_no: '6222020202020203',
    bank_mobile: '13800138001'
  })
  assert(!Object.prototype.hasOwnProperty.call(operatorPayload, 'bank_name'), 'operator payload should not include bank_name')
  assert(operatorPayload.bank_mobile === '13800138001', 'operator payload should still carry bank_mobile')

  const validationMessage = validateBaofuPersonalProfileForm({
    name: '张三',
    certificate_no: '110101199001010011',
    bank_account_no: '6222020202020202',
    bank_mobile: '13800138000'
  })
  assert(validationMessage === '', 'valid personal profile should pass validation')

  const personalFormFromDefaults = buildBaofuPersonalFormFromDefaults(empty, {
    legal_name: ' 张三 ',
    certificate_no: ' 110101199001010011 ',
    bank_account_no: ' 6222020202020202 ',
    bank_mobile: ' 13800138000 '
  })
  assert(personalFormFromDefaults.name === '张三', 'personal defaults should trim clear-text name')
  assert(personalFormFromDefaults.certificate_no === '110101199001010011', 'personal defaults should restore clear-text certificate number')
  assert(personalFormFromDefaults.bank_account_no === '6222020202020202', 'personal defaults should restore clear-text bank account')
  assert(personalFormFromDefaults.bank_mobile === '13800138000', 'personal defaults should trim bank mobile')

  const personalMaskOnlyDefaults = buildBaofuPersonalFormFromDefaults(empty, {
    certificate_no_mask: '110************011',
    bank_account_no_mask: '6222********0202'
  })
  assert(personalMaskOnlyDefaults.certificate_no === '', 'personal defaults must not backfill certificate mask as clear text')
  assert(personalMaskOnlyDefaults.bank_account_no === '', 'personal defaults must not backfill bank account mask as clear text')

  const trimmedOperatorPayload = buildBaofuPersonalProfilePayload('operator', {
    name: ' 李四 ',
    certificate_no: ' 110101199001010022 ',
    bank_account_no: ' 6222020202020203 ',
    bank_mobile: ' 13800138001 '
  })
  assert(trimmedOperatorPayload.legal_name === '李四', 'operator payload should trim name')
  assert(trimmedOperatorPayload.certificate_no === '110101199001010022', 'operator payload should trim certificate_no')
  assert(trimmedOperatorPayload.bank_account_no === '6222020202020203', 'operator payload should trim bank_account_no')
  assert(trimmedOperatorPayload.bank_mobile === '13800138001', 'operator payload should trim bank_mobile')

  const enterpriseDraft = buildBaofuEnterpriseBankDraftFromDefaults({
    bank_name: '邢台银行',
    deposit_bank_province: '河北省',
    deposit_bank_city: '邢台市',
    deposit_bank_name: '邢台银行宁晋支行',
    self_employed: true,
    card_user_name: '周松涛'
  })
  assert(enterpriseDraft.bank_address_code === '河北省', 'enterprise draft should restore manual province from defaults')
  assert(enterpriseDraft.manual_bank_city === '邢台市', 'enterprise draft should restore manual city from defaults')
  assert(enterpriseDraft.bank_branch_id === '邢台银行宁晋支行', 'enterprise draft should keep manual branch identifier in sync with branch name')
  assert(enterpriseDraft.bank_name === '邢台银行宁晋支行', 'enterprise draft should restore manual branch from defaults')
  assert(enterpriseDraft.need_bank_branch === true, 'enterprise draft should keep manual bank location required')

  const companyDraft = buildBaofuEnterpriseBankDraftFromDefaults({
    settlement_account_allowed_types: ['ACCOUNT_TYPE_BUSINESS'],
    legal_name: '宁晋县康味餐饮有限公司',
    legal_person_name: '周松涛',
    self_employed: true,
    card_user_name: '周松涛'
  })
  assert(companyDraft.account_type === 'ACCOUNT_TYPE_BUSINESS', 'company enterprise draft must ignore stale private-card defaults')
  assert(companyDraft.account_name === '宁晋县康味餐饮有限公司', 'company enterprise draft should use legal name for public account')

  const companyPayload = buildBaofuEnterpriseProfilePayload({
    legal_name: '宁晋县康味餐饮有限公司',
    business_license_number: '91130528MA00000001',
    legal_person_name: '周松涛',
    legal_person_id_number: '130528199001010011',
    corporate_mobile: '13800138000',
    email: 'merchant@example.com'
  }, {
    account_type: 'ACCOUNT_TYPE_PRIVATE',
    account_bank: '邢台银行',
    bank_alias: '邢台银行',
    need_bank_branch: true,
    bank_address_code: '河北省',
    deposit_bank_province: '河北省',
    deposit_bank_city: '邢台市',
    bank_name: '邢台银行宁晋支行',
    account_number: '6222020202020202',
    account_name: '周松涛'
  }, {
    settlement_account_allowed_types: ['ACCOUNT_TYPE_BUSINESS']
  })
  assert(companyPayload.self_employed === false, 'company enterprise payload must submit self_employed=false')
  assert(!Object.prototype.hasOwnProperty.call(companyPayload, 'card_user_name'), 'company enterprise payload must not submit private card holder')

  const enterpriseFormFromDefaults = buildBaofuEnterpriseFormFromDefaults({
    legal_name: ' 宁晋县周鹏饭店 ',
    business_license_number: ' 92130528MA00000001 ',
    legal_person_name: ' 周松涛 ',
    legal_person_id_number: ' 130528199001010011 ',
    corporate_mobile: ' 13800138000 ',
    email: ' merchant@example.com '
  })
  assert(enterpriseFormFromDefaults.legal_name === '宁晋县周鹏饭店', 'enterprise defaults should trim clear-text legal name')
  assert(enterpriseFormFromDefaults.legal_person_id_number === '130528199001010011', 'enterprise defaults should restore clear-text legal id')
  assert(enterpriseFormFromDefaults.corporate_mobile === '13800138000', 'enterprise defaults should restore clear-text mobile')
  assert(enterpriseFormFromDefaults.email === 'merchant@example.com', 'enterprise defaults should restore clear-text email')

  const enterpriseMaskOnlyDefaults = buildBaofuEnterpriseFormFromDefaults({
    legal_person_id_number_mask: '130************011',
    corporate_mobile_mask: '138****8000',
    email_mask: 'm***@example.com',
    bank_account_no_mask: '6222********0202'
  })
  const enterpriseMaskOnlyDraft = buildBaofuEnterpriseBankDraftFromDefaults({
    legal_person_id_number_mask: '130************011',
    corporate_mobile_mask: '138****8000',
    email_mask: 'm***@example.com',
    bank_account_no_mask: '6222********0202'
  })
  assert(enterpriseMaskOnlyDefaults.legal_person_id_number === '', 'enterprise defaults must not backfill legal id mask as clear text')
  assert(enterpriseMaskOnlyDefaults.corporate_mobile === '', 'enterprise defaults must not backfill mobile mask as clear text')
  assert(enterpriseMaskOnlyDefaults.email === '', 'enterprise defaults must not backfill email mask as clear text')
  assert(enterpriseMaskOnlyDraft.account_number === '', 'enterprise bank draft must not backfill bank account mask as clear text')

  const enterpriseDraftFromClearDefaults = buildBaofuEnterpriseBankDraftFromDefaults({
    legal_name: ' 宁晋县周鹏饭店 ',
    legal_person_name: ' 周松涛 ',
    self_employed: true,
    account_bank: ' 邢台银行 ',
    bank_account_no: ' 6222020202020202 ',
    deposit_bank_province: ' 河北省 ',
    deposit_bank_city: ' 邢台市 ',
    deposit_bank_name: ' 邢台银行宁晋支行 '
  })
  assert(enterpriseDraftFromClearDefaults.account_number === '6222020202020202', 'enterprise draft should restore clear-text bank account')
  assert(enterpriseDraftFromClearDefaults.account_bank === '邢台银行', 'enterprise draft should trim clear-text bank name')

  const enterprisePayload = buildBaofuEnterpriseProfilePayload({
    legal_name: '宁晋县周鹏饭店',
    business_license_number: '92130528MA00000001',
    legal_person_name: '周松涛',
    legal_person_id_number: '130528199001010011',
    corporate_mobile: '13800138000',
    email: 'merchant@example.com'
  }, {
    account_type: 'ACCOUNT_TYPE_PRIVATE',
    account_bank: '邢台银行',
    bank_alias: '邢台银行',
    need_bank_branch: true,
    bank_address_code: '河北省',
    deposit_bank_province: '河北省',
    deposit_bank_city: '邢台市',
    bank_name: '邢台银行宁晋支行',
    account_number: '6222020202020202',
    account_name: '周松涛'
  })
  assert(enterprisePayload.deposit_bank_province === '河北省', 'enterprise manual bank payload should keep submitted province')
  assert(enterprisePayload.deposit_bank_city === '邢台市', 'enterprise manual bank payload should keep submitted city')
  assert(enterprisePayload.deposit_bank_city !== '北京市', 'enterprise manual bank payload must not hardcode Beijing city')

  const missingManualCityPayload = buildBaofuEnterpriseProfilePayload({
    legal_name: '宁晋县周鹏饭店',
    business_license_number: '92130528MA00000001',
    legal_person_name: '周松涛',
    legal_person_id_number: '130528199001010011',
    corporate_mobile: '13800138000',
    email: 'merchant@example.com'
  }, {
    account_type: 'ACCOUNT_TYPE_PRIVATE',
    account_bank: '邢台银行',
    bank_alias: '邢台银行',
    need_bank_branch: true,
    bank_address_code: '河北省',
    deposit_bank_province: '河北省',
    bank_name: '邢台银行宁晋支行',
    account_number: '6222020202020202',
    account_name: '周松涛'
  })
  assert(missingManualCityPayload.deposit_bank_city === '', 'missing manual city should stay empty for validation instead of defaulting to Beijing')

  const trimmedEnterprisePayload = buildBaofuEnterpriseProfilePayload({
    legal_name: ' 宁晋县周鹏饭店 ',
    business_license_number: ' 92130528MA00000001 ',
    legal_person_name: ' 周松涛 ',
    legal_person_id_number: ' 130528199001010011 ',
    corporate_mobile: ' 13800138000 ',
    email: ' merchant@example.com '
  }, {
    account_type: 'ACCOUNT_TYPE_PRIVATE',
    account_bank: ' 邢台银行 ',
    bank_alias: ' 邢台银行 ',
    need_bank_branch: true,
    bank_address_code: ' 河北省 ',
    deposit_bank_province: ' 河北省 ',
    deposit_bank_city: ' 邢台市 ',
    bank_name: ' 邢台银行宁晋支行 ',
    account_number: ' 6222020202020202 ',
    account_name: ' 周松涛 '
  })
  assert(trimmedEnterprisePayload.legal_name === '宁晋县周鹏饭店', 'enterprise payload should trim legal name')
  assert(trimmedEnterprisePayload.legal_person_id_number === '130528199001010011', 'enterprise payload should trim legal id')
  assert(trimmedEnterprisePayload.bank_account_no === '6222020202020202', 'enterprise payload should trim bank account')
  assert(trimmedEnterprisePayload.deposit_bank_province === '河北省', 'enterprise payload should trim bank province')
  assert(trimmedEnterprisePayload.deposit_bank_city === '邢台市', 'enterprise payload should trim bank city')
  assert(trimmedEnterprisePayload.deposit_bank_name === '邢台银行宁晋支行', 'enterprise payload should trim bank branch')

  const bankFormWxml = fs.readFileSync(bankFormWxmlPath, 'utf8')
  const bankFormTs = fs.readFileSync(bankFormTsPath, 'utf8')
  for (const selectorToken of [
    '<t-picker',
    'showProvincePicker',
    'showCityPicker',
    'showBranchPicker',
    'onOpenProvincePicker',
    'onOpenCityPicker',
    'onOpenBranchPicker',
    'onSelectBranchOption',
    'onBranchPickerVisibleChange',
    'listApplymentProvinces',
    'listApplymentCities',
    'listApplymentBankBranches'
  ]) {
    assert(
      !bankFormWxml.includes(selectorToken) && !bankFormTs.includes(selectorToken),
      `Baofoo bank form must not keep obsolete province/city/branch selector token: ${selectorToken}`
    )
  }

  for (const searchToken of [
    'showBankPicker',
    'bankKeyword',
    'filteredBanks',
    'loadingBanks',
    'recognizingBank',
    'onOpenBankPicker',
    'onSelectBankOption',
    'onRecognizeBank',
    'onBankKeywordChange',
    'onBankPickerVisibleChange',
    'listApplymentBanks',
    'searchApplymentBanksByAccount',
    'resolveRecognizedBankSelection',
    '<t-search',
    '搜索银行名称',
    '银行列表加载中'
  ]) {
    assert(
      !bankFormWxml.includes(searchToken) && !bankFormTs.includes(searchToken),
      `Baofoo bank form must not keep obsolete bank search token: ${searchToken}`
    )
  }

  const applymentBankApiSource = applymentBankApiPaths
    .map((filePath) => `${path.relative(repoRoot, filePath)}\n${fs.readFileSync(filePath, 'utf8')}`)
    .join('\n')
  for (const oldApiToken of [
    'ApplymentBankOption',
    'ApplymentProvinceOption',
    'ApplymentCityOption',
    'ApplymentBranchOption',
    'ApplymentBankListResponse',
    'ApplymentBankSearchResponse',
    'ApplymentProvinceListResponse',
    'ApplymentCityListResponse',
    'ApplymentBranchListResponse',
    'listApplymentBanks',
    'searchApplymentBanksByAccount',
    'listApplymentProvinces',
    'listApplymentCities',
    'listApplymentBankBranches'
  ]) {
    assert(!applymentBankApiSource.includes(oldApiToken), `Baofoo shared applyment bank API must not keep obsolete picker/search API: ${oldApiToken}`)
  }

  const submitFormSource = submitFormPaths
    .map((filePath) => `${path.relative(repoRoot, filePath)}\n${fs.readFileSync(filePath, 'utf8')}`)
    .join('\n')
  for (const sensitiveToggleToken of [
    'type="{{showIdNumber ? \'text\' : \'password\'}}"',
    'type="{{showAccountNumber ? \'text\' : \'password\'}}"',
    'browse-off',
    'suffixIcon="{{showIdNumber',
    'suffixIcon="{{allowSavedAccountNumber',
    'showIdNumber',
    'showAccountNumber',
    'onToggleIdVisibility',
    'onToggleAccountNumberVisibility'
  ]) {
    assert(
      !submitFormSource.includes(sensitiveToggleToken),
      `Baofoo submit forms must not keep old privacy toggle UI: ${sensitiveToggleToken}`
    )
  }

  for (const maskToken of [
    'certificate_no_mask',
    'legal_person_id_number_mask',
    'corporate_mobile_mask',
    'email_mask',
    'bank_account_no_mask',
    'contact_mobile_mask'
  ]) {
    assert(
      !submitFormSource.includes(maskToken),
      `Baofoo submit forms must not render or consume masked profile defaults: ${maskToken}`
    )
  }

  console.log('check-baofu-personal-profile-form: validated personal profile payload shape')
}

main()
