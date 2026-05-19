const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const ROOT = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(ROOT, relativePath), 'utf8')
}

function loadTsModule(relativePath, requireStub = () => ({})) {
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
    console
  }

  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const apiSource = read('miniprogram/api/baofu-withdrawal.ts')
assert(!apiSource.includes('owner_type'), 'Baofu withdrawal API must not expose owner_type')
assert(!apiSource.includes('owner_id'), 'Baofu withdrawal API must not expose owner_id')

const capturedRequests = []
const api = loadTsModule('miniprogram/api/baofu-withdrawal.ts', (id) => {
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

assert.strictEqual(api.baofuWithdrawalEndpoint('merchant'), '/v1/merchant/finance/baofu-withdrawal')
assert.strictEqual(api.baofuWithdrawalEndpoint('platform'), '/v1/platform/finance/baofu-withdrawal')
assert.strictEqual(api.baofuWithdrawalEndpoint('operator'), '/v1/operators/me/finance/baofu-withdrawal')
assert.strictEqual(api.baofuWithdrawalEndpoint('rider'), '/v1/rider/income/baofu-withdrawal')

api.getBaofuWithdrawalBalance('merchant')
api.listBaofuWithdrawals('operator', { page: 2, limit: 20 })
api.getBaofuWithdrawal('platform', 18)
api.createBaofuWithdrawal('rider', { amount: 1200, remark: '提现' })

assert.deepStrictEqual(capturedRequests.map((request) => request.url), [
  '/v1/merchant/finance/baofu-withdrawal/balance',
  '/v1/operators/me/finance/baofu-withdrawal/withdrawals',
  '/v1/platform/finance/baofu-withdrawal/withdrawals/18',
  '/v1/rider/income/baofu-withdrawal/withdraw'
])
assert.strictEqual(capturedRequests[3].data.amount, 1200)
assert(!('owner_type' in capturedRequests[3].data), 'Create request must not include owner_type')
assert(!('owner_id' in capturedRequests[3].data), 'Create request must not include owner_id')

const workflow = loadTsModule('miniprogram/services/baofu-withdrawal-workflow.ts')

assert.strictEqual(workflow.formatFenToYuanText(12345), '¥123.45')
assert.strictEqual(workflow.parseYuanInputToFen('12.30').amount, 1230)
assert.strictEqual(workflow.parseYuanInputToFen('12.345').errorMessage, '金额最多保留两位小数')
assert.strictEqual(workflow.parseYuanInputToFen('abc').errorMessage, '请输入有效金额')
assert.strictEqual(workflow.buildBaofuWithdrawalStatusView('processing').text, '提现处理中')
assert.strictEqual(workflow.buildBaofuWithdrawalStatusView('succeeded').theme, 'success')
assert.strictEqual(workflow.buildBaofuWithdrawalStatusView('returned').text, '提现退票')
assert.strictEqual(workflow.buildBaofuWithdrawalBalanceView({
  available_amount: 99,
  pending_amount: 0,
  ledger_amount: 99,
  frozen_amount: 0,
  min_withdraw_amount: 100,
  max_withdraw_amount: 500000000,
  can_withdraw: false,
  disabled_reason: ''
}).disabledReason, '可提现金额不足')

const merchantWithdrawalSources = [
  read('miniprogram/pages/merchant/finance/withdrawals/index.ts'),
  read('miniprogram/pages/merchant/finance/withdrawals/create/index.ts'),
  read('miniprogram/pages/merchant/finance/withdrawals/detail/index.ts')
].join('\n')

for (const required of [
  "getBaofuWithdrawalBalance('merchant')",
  "listBaofuWithdrawals('merchant'",
  "createBaofuWithdrawal('merchant'",
  "getBaofuWithdrawal('merchant'"
]) {
  assert(merchantWithdrawalSources.includes(required), `Merchant withdrawal pages must call ${required}`)
}

assert(!merchantWithdrawalSources.includes('owner_type'), 'Merchant withdrawal pages must not pass owner_type')
assert(!merchantWithdrawalSources.includes('owner_id'), 'Merchant withdrawal pages must not pass owner_id')
assert(!merchantWithdrawalSources.includes('/account/withdraw'), 'Merchant withdrawal pages must not call legacy WeChat withdraw routes')
assert(!merchantWithdrawalSources.includes('/v1/rider/withdraw'), 'Merchant withdrawal pages must not call rider deposit withdraw routes')

console.log('Baofu withdrawal workflow contract check passed')
