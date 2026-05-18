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

  console.log('check-baofu-personal-profile-form: validated personal profile payload shape')
}

main()
