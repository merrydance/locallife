const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const ROOT = path.resolve(__dirname, '..')

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

const capturedRequests = []
const paymentApi = loadTsModule('miniprogram/pages/merchant/_main_shared/api/payment.ts', (id) => {
  if (id === '../../../../utils/request') {
    return {
      request(options) {
        capturedRequests.push(options)
        return Promise.resolve(options)
      }
    }
  }
  if (id === '../../../../utils/logger') {
    return { logger: { warn() {}, error() {}, info() {}, debug() {} } }
  }
  return {}
})

paymentApi.createRefund({
  payment_order_id: 123,
  refund_type: 'partial',
  refund_amount: 456,
  refund_reason: '菜品售后'
}, {
  idempotencyKey: 'refund-key-1'
})

assert.strictEqual(capturedRequests.length, 1)
assert.strictEqual(capturedRequests[0].method, 'POST')
assert.strictEqual(capturedRequests[0].url, '/v1/refunds')
assert.strictEqual(capturedRequests[0].data.payment_order_id, 123)
assert.strictEqual(capturedRequests[0].header['Idempotency-Key'], 'refund-key-1')

const paymentSource = read('miniprogram/pages/merchant/_main_shared/api/payment.ts')
const detailSource = read('miniprogram/pages/merchant/orders/detail/index.ts')

assert(paymentSource.includes('CreateRefundOptions'), 'merchant payment API must require refund request options')
assert(paymentSource.includes("'Idempotency-Key':") && paymentSource.includes('.idempotencyKey'), 'merchant refund API must send Idempotency-Key')
assert(detailSource.includes('buildRefundIdempotencyKey'), 'merchant order detail must build a refund idempotency key')
assert(detailSource.includes('refundIdempotencyKey:'), 'merchant order detail must keep refund idempotency key in popup state')
assert(
  detailSource.includes('this.data.refundIdempotencyKey || buildRefundIdempotencyKey'),
  'merchant order detail must reuse the same key when retrying an unchanged refund draft'
)
assert(
  detailSource.match(/refundIdempotencyKey:\s*buildRefundIdempotencyKey/g)?.length >= 3,
  'merchant order detail must refresh the key when opening or changing refund draft fields'
)

console.log('check-merchant-manual-refund-idempotency-contract: validated refund idempotency header and draft key reuse')
