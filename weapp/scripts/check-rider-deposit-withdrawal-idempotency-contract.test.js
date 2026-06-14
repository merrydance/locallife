const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const ROOT = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(ROOT, relativePath), 'utf8')
}

function loadTsModule(relativePath, requireStub = () => ({}), extraSandbox = {}) {
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
    Date,
    Math,
    Promise,
    ...extraSandbox
  }

  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const capturedRequests = []
const riderApi = loadTsModule('miniprogram/pages/rider/_main_shared/api/rider.ts', (id) => {
  if (id === '../../../../utils/request') {
    return {
      request(options) {
        capturedRequests.push(options)
        return Promise.resolve(options)
      }
    }
  }
  throw new Error(`unexpected rider API require: ${id}`)
})

riderApi.RiderService.withdrawDeposit(
  { amount: 1200, remark: '骑手押金提现' },
  { idempotencyKey: 'rider-deposit-withdrawal:test-key' }
)

assert.strictEqual(capturedRequests.length, 1, 'deposit withdrawal API must submit exactly one request')
assert.strictEqual(capturedRequests[0].url, '/v1/rider/withdraw')
assert.strictEqual(capturedRequests[0].method, 'POST')
assert.strictEqual(capturedRequests[0].data.amount, 1200)
assert.strictEqual(
  (capturedRequests[0].header || {})['Idempotency-Key'],
  'rider-deposit-withdrawal:test-key',
  'deposit withdrawal API must send Idempotency-Key header'
)

const storage = new Map()
const withdrawalService = loadTsModule('miniprogram/pages/rider/_services/rider-deposit-withdrawal.ts', (id) => {
  if (id === '../_main_shared/api/rider') {
    return {
      __esModule: true,
      default: {
        getWithdrawalStatus: async () => {
          throw new Error('not needed')
        }
      }
    }
  }
  throw new Error(`unexpected rider withdrawal service require: ${id}`)
}, {
  wx: {
    setStorageSync(key, value) {
      storage.set(key, value)
    },
    getStorageSync(key) {
      return storage.get(key)
    },
    removeStorageSync(key) {
      storage.delete(key)
    }
  }
})

const pendingContext = withdrawalService.buildPendingRiderDepositWithdrawalContext({
  status: 'processing',
  requested_amount: 1200,
  accepted_amount: 1200,
  refunds: [
    {
      refund_order_id: 88,
      payment_order_id: 188,
      out_refund_no: 'RIDER_DEPOSIT_REFUND_88',
      amount: 1200,
      status: 'pending'
    }
  ]
}, 'rider-deposit-withdrawal:test-key')

assert(pendingContext, 'pending rider deposit withdrawal must build a recoverable context')
assert.strictEqual(
  pendingContext.idempotencyKey,
  'rider-deposit-withdrawal:test-key',
  'pending rider deposit withdrawal context must persist the request idempotency key'
)

const failedContext = withdrawalService.buildPendingRiderDepositWithdrawalContext({
  status: 'failed',
  requested_amount: 1200,
  accepted_amount: 1200,
  refunds: [
    {
      refund_order_id: 89,
      payment_order_id: 189,
      out_refund_no: 'RIDER_DEPOSIT_REFUND_89',
      amount: 1200,
      status: 'failed'
    }
  ]
}, 'rider-deposit-withdrawal:test-key')

assert.strictEqual(
  failedContext,
  null,
  'terminal failed rider deposit withdrawal response must not be stored as pending'
)

withdrawalService.savePendingRiderDepositWithdrawal(pendingContext)
assert.strictEqual(
  withdrawalService.getPendingRiderDepositWithdrawal().idempotencyKey,
  'rider-deposit-withdrawal:test-key',
  'stored pending rider deposit withdrawal must keep the idempotency key for re-entry'
)

const legacyContext = {
  refundOrderIds: [99],
  acceptedAmount: 990,
  updatedAt: '2026-06-13T00:00:00.000Z'
}
storage.set('riderDepositPendingWithdrawal', legacyContext)
assert.strictEqual(
  JSON.stringify(withdrawalService.getPendingRiderDepositWithdrawal().refundOrderIds),
  JSON.stringify([99]),
  'legacy pending rider deposit withdrawals without idempotency key must remain recoverable'
)

const depositPageSource = read('miniprogram/pages/rider/deposit/index.ts')
assert(
  depositPageSource.includes('buildDepositWithdrawalIdempotencyKey'),
  'deposit page must create a rider deposit withdrawal idempotency draft key'
)
assert(
  depositPageSource.includes('withdrawalIdempotencyKey'),
  'deposit page must store the rider deposit withdrawal idempotency draft key'
)
assert(
  /RiderService\.withdrawDeposit\([\s\S]*\{\s*idempotencyKey\s*\}/.test(depositPageSource),
  'deposit page must submit rider deposit withdrawal with the stable draft idempotency key'
)
assert(
  /waitForSubmittedRiderDepositWithdrawalTerminalStatus\([\s\S]*\{\s*idempotencyKey\s*\}/.test(depositPageSource),
  'deposit page must persist the draft idempotency key with pending withdrawal context'
)
assert(
  depositPageSource.includes("result.status !== 'success' && result.status !== 'failed'"),
  'deposit page must not poll or persist terminal failed rider deposit withdrawal responses as pending'
)
assert(
  !depositPageSource.includes('baofu-withdrawal'),
  'rider deposit withdrawal page must stay separate from Baofoo rider income withdrawal'
)

console.log('check-rider-deposit-withdrawal-idempotency-contract: rider deposit withdrawal sends and preserves request idempotency key')
