const fs = require('fs')
const path = require('path')
const vm = require('vm')
const ts = require('typescript')

const repoRoot = path.resolve(__dirname, '..')
const sourcePath = path.join(repoRoot, 'miniprogram/services/baofu-account-profile-form.ts')

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
    buildBaofuEnterpriseBankDraftFromDefaults,
    buildBaofuEnterpriseProfilePayload,
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

  console.log('check-baofu-personal-profile-form: validated personal profile payload shape')
}

main()
