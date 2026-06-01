const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

process.env.TZ = 'Asia/Shanghai'

const sourcePath = path.join(__dirname, '..', 'miniprogram', 'pages', 'rider', '_utils', 'rider-delivery-view.ts')

function loadModule() {
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
      if (modulePath === '../_main_shared/api/delivery') {
        return {}
      }
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

const {
  buildRiderDeliveryDeadlineView,
  isRiderDeliveryOverdue
} = loadModule()

const now = Date.parse('2026-05-24T02:00:00.000Z')

const deliveringDeadline = buildRiderDeliveryDeadlineView({
  status: 'delivering',
  estimated_delivery_at: '2026-05-24T01:59:00.000Z'
}, now)
assert.strictEqual(deliveringDeadline.text, '已超时')
assert.strictEqual(deliveringDeadline.isOverdue, true)

const onTimeDeliveredInput = {
  status: 'delivered',
  estimated_delivery_at: '2026-05-24T02:10:00.000Z',
  delivered_at: '2026-05-24T02:00:00.000Z'
}
const onTimeDelivered = buildRiderDeliveryDeadlineView(onTimeDeliveredInput, now)
assert.strictEqual(onTimeDelivered.text, '10:00 送达')
assert.strictEqual(onTimeDelivered.isOverdue, false)
assert.strictEqual(isRiderDeliveryOverdue(onTimeDeliveredInput), false)

const lateDeliveredInput = {
  status: 'completed',
  estimated_delivery_at: '2026-05-24T01:50:00.000Z',
  delivered_at: '2026-05-24T02:00:00.000Z'
}
const lateDelivered = buildRiderDeliveryDeadlineView(lateDeliveredInput, now)
assert.strictEqual(lateDelivered.text, '超时送达')
assert.strictEqual(lateDelivered.isOverdue, true)
assert.strictEqual(isRiderDeliveryOverdue(lateDeliveredInput), true)

console.log('check-rider-delivery-deadline-view tests passed')
