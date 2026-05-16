const fs = require('fs')
const path = require('path')
const vm = require('vm')
const ts = require('typescript')

const repoRoot = path.resolve(__dirname, '..')
const helperPath = path.join(repoRoot, 'miniprogram/components/applyment-bank-form/selection.ts')

function loadHelper() {
  const source = fs.readFileSync(helperPath, 'utf8')
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
  vm.runInNewContext(compiled, sandbox, { filename: helperPath })
  return sandbox.module.exports
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}

function bank(overrides) {
  return {
    bank_alias: '招商银行',
    bank_alias_code: '1001',
    account_bank: '招商银行',
    account_bank_code: 1001,
    need_bank_branch: false,
    ...overrides
  }
}

function main() {
  const { resolveRecognizedBankSelection } = loadHelper()

  const empty = resolveRecognizedBankSelection([])
  assert(empty.bank === null, 'empty recognition should not select a bank')
  assert(empty.shouldOpenPicker === false, 'empty recognition should not open picker')
  assert(empty.filteredBanks.length === 0, 'empty recognition should keep empty filtered list')

  const singleBank = bank({ bank_alias: '中国工商银行', bank_alias_code: '1002', account_bank_code: 1002 })
  const single = resolveRecognizedBankSelection([singleBank])
  assert(single.bank === singleBank, 'single recognition should select the matched bank')
  assert(single.shouldOpenPicker === false, 'single recognition should not require picker confirmation')
  assert(single.selectedBankIndex === 0, 'single recognition should point at the selected bank')

  const first = bank({ bank_alias: '招商银行', bank_alias_code: '1001', account_bank_code: 1001 })
  const second = bank({ bank_alias: '招商银行信用卡', bank_alias_code: '1003', account_bank_code: 1003 })
  const multiple = resolveRecognizedBankSelection([first, second])
  assert(multiple.bank === first, 'multiple recognition should make the first provider-ranked match current')
  assert(multiple.shouldOpenPicker === true, 'multiple recognition should still open picker for review')
  assert(multiple.selectedBankIndex === 0, 'multiple recognition should point at the current match')
  assert(multiple.filteredBanks.length === 2, 'multiple recognition should preserve all visible matches')

  const many = Array.from({ length: 120 }, (_, index) => bank({
    bank_alias: `银行${index}`,
    bank_alias_code: String(2000 + index),
    account_bank_code: 2000 + index
  }))
  const capped = resolveRecognizedBankSelection(many)
  assert(capped.filteredBanks.length === 100, 'recognition picker list should stay capped at 100')
  assert(capped.bank === many[0], 'capped recognition should still select the first provider-ranked match')

  console.log('check-applyment-bank-selection: validated recognition selection behavior')
}

main()
