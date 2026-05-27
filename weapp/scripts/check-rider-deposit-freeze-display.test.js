const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const ROOT = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(ROOT, relativePath), 'utf8')
}

function loadTsModule(relativePath) {
  const sourcePath = path.join(ROOT, relativePath)
  const source = fs.readFileSync(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018
    }
  }).outputText

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      throw new Error(`unexpected require: ${modulePath}`)
    },
    Date,
    Math,
    Number,
    String
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const depositPage = read('miniprogram/pages/rider/deposit/index.wxml')

assert(
  depositPage.includes('<text class="label">押金总额 (元)</text>'),
  'Rider deposit page must keep total deposit as the primary amount so frozen funds do not look deducted'
)
assert(
  !depositPage.includes('<text class="label">可用押金 (元)</text>'),
  'Rider deposit page must not use available deposit as the primary amount'
)
assert(
  depositPage.includes('<text class="sub-label">可用押金</text>'),
  'Rider deposit page must still show available deposit as a secondary balance'
)
assert(
  depositPage.includes('<text class="sub-label">冻结中</text>'),
  'Rider deposit page must label active delivery hold as frozen, not deducted'
)

const { decorateDepositRecord } = loadTsModule('miniprogram/utils/rider-deposit-record-view.ts')

const freezeRecord = decorateDepositRecord({
  id: 1,
  amount: 1200,
  type: 'freeze',
  created_at: '2026-05-27T10:00:00.000Z',
  remark: '接单冻结押金'
})

assert.strictEqual(freezeRecord.display_type_text, '代取冻结')
assert.strictEqual(freezeRecord.status_text, '冻结中')
assert.strictEqual(freezeRecord.display_amount_text, '冻结 12.00')
assert.strictEqual(freezeRecord.display_amount_class, 'neutral')

const unfreezeRecord = decorateDepositRecord({
  id: 2,
  amount: 1200,
  type: 'unfreeze',
  created_at: '2026-05-27T10:30:00.000Z',
  remark: '代取完成解冻押金'
})

assert.strictEqual(unfreezeRecord.display_type_text, '代取解冻')
assert.strictEqual(unfreezeRecord.display_amount_text, '释放 12.00')
assert.strictEqual(unfreezeRecord.display_amount_class, 'positive')

console.log('check-rider-deposit-freeze-display tests passed')
