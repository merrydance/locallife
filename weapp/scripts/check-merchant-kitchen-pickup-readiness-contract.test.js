const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

function loadModule(relativePath, stubs = {}) {
  const sourcePath = path.join(__dirname, '..', 'miniprogram', ...relativePath.split('/'))
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
    require(modulePath) {
      if (stubs[modulePath]) {
        return stubs[modulePath]
      }
      throw new Error(`unexpected require: ${modulePath}`)
    },
    Date,
    Math,
    Number,
    String,
    RegExp,
    Array
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const { OrderManagementAdapter } = loadModule('api/order-management.ts', {
  '../utils/request': { request: async () => ({}) },
  '../utils/merchant-order-action-view': loadModule('utils/merchant-order-action-view.ts', {
    '../api/order-management': {}
  })
})

const { getKitchenStatusView } = loadModule('utils/merchant-kitchen-detail-view.ts', {
  '../api/order-management': {}
})

assert.strictEqual(OrderManagementAdapter.canMarkReady({
  status: 'courier_accepted',
  order_type: 'takeout',
  fulfillment_status: 'preparing'
}), true)

assert.strictEqual(OrderManagementAdapter.canMarkReady({
  status: 'courier_accepted',
  order_type: 'takeout',
  fulfillment_status: 'ready'
}), false)

const statusView = getKitchenStatusView({
  status: 'preparing',
  order_status: 'courier_accepted',
  kitchen_status: 'preparing',
  fulfillment_status: 'preparing',
  can_mark_ready: true,
  status_hint: '骑手已接单，餐品仍在制作，请出餐后标记完成'
})

assert.strictEqual(statusView.label, '制作中')
assert.strictEqual(statusView.canMarkReady, true)
assert.strictEqual(statusView.statusHint, '骑手已接单，餐品仍在制作，请出餐后标记完成')

console.log('check-merchant-kitchen-pickup-readiness-contract tests passed')
