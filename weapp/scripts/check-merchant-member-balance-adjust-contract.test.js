const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const ROOT = path.resolve(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(ROOT, relativePath), 'utf8')
}

function loadTsModule(relativePath, requireStub = () => ({}), globals = {}) {
  const sourcePath = path.join(ROOT, relativePath)
  const source = fs.readFileSync(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018,
      esModuleInterop: true
    }
  }).outputText

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require: requireStub,
    console,
    ...globals
  }

  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const capturedRequests = []
const api = loadTsModule('miniprogram/api/merchant.ts', (id) => {
  if (id === '../utils/request') {
    return {
      request(options) {
        capturedRequests.push(options)
        return Promise.resolve(options)
      }
    }
  }
  return {}
})

api.adjustMerchantMemberBalance(11, 22, { amount: 300, notes: '补偿' }, { idempotencyKey: 'adjust-key-1' })
assert.strictEqual(capturedRequests.length, 1)
assert.strictEqual(capturedRequests[0].method, 'POST')
assert.strictEqual(capturedRequests[0].url, '/v1/merchants/11/members/22/balance')
assert.strictEqual(capturedRequests[0].header['Idempotency-Key'], 'adjust-key-1')

const tagView = loadTsModule('miniprogram/pages/merchant/_utils/membership-transaction-view.ts', (id) => {
  if (id === '../_main_shared/utils/status-tag') {
    return {
      buildStatusTagView(label, theme) {
        return { label, theme }
      }
    }
  }
  return {}
})

const adjustmentCredit = tagView.getMembershipTransactionTagView('adjustment_credit')
assert.strictEqual(adjustmentCredit.label, '人工调整')
assert.strictEqual(adjustmentCredit.theme, 'info')
const adjustmentDebit = tagView.getMembershipTransactionTagView('adjustment_debit')
assert.strictEqual(adjustmentDebit.label, '人工扣减')
assert.strictEqual(adjustmentDebit.theme, 'warning')

const requestSource = read('miniprogram/utils/request.ts')
const apiSource = read('miniprogram/api/merchant.ts')
const pageSource = read('miniprogram/pages/merchant/settings/members/index.ts')

assert(requestSource.includes('header?: Record<string, string>'), 'request wrapper must accept per-call headers')
assert(requestSource.includes('...(header || {})'), 'request wrapper must forward per-call headers')
assert(
  requestSource.indexOf('...(header || {})') < requestSource.indexOf("'Authorization': `Bearer ${getToken()}`"),
  'request wrapper must not allow per-call headers to override Authorization'
)
assert(apiSource.includes("'Idempotency-Key': options.idempotencyKey"), 'adjustment API must send Idempotency-Key')
assert(pageSource.includes('buildAdjustIdempotencyKey'), 'member page must build an adjustment idempotency key')
assert(pageSource.includes('adjustIdempotencyKey:'), 'member page must keep adjustment idempotency key in popup state')
assert(pageSource.includes('this.data.adjustIdempotencyKey || buildAdjustIdempotencyKey'), 'member page must reuse the same key when retrying an unchanged draft')
assert(pageSource.includes('hasMore: members.length < (response.total || 0)'), 'member page must use backend total for pagination')

console.log('check-merchant-member-balance-adjust-contract: validated adjustment idempotency and labels')
