const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const sourcePath = path.join(__dirname, '..', 'miniprogram', 'services', 'order-receipt-confirmation.ts')

function loadModule({ openBusinessView, modalConfirm = true } = {}) {
  const source = fs.readFileSync(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018
    }
  }).outputText

  const calls = {
    confirmOrder: [],
    modals: [],
    toasts: [],
    loading: [],
    hiddenLoading: 0,
    openBusinessView: [],
    logger: []
  }
  const app = { globalData: {} }

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (modulePath === '../api/order') {
        return {
          confirmOrder(orderId) {
            calls.confirmOrder.push(orderId)
            return Promise.resolve({ id: orderId, status: 'completed' })
          }
        }
      }
      if (modulePath === '../utils/logger') {
        return {
          logger: {
            info(message, data, context) {
              calls.logger.push({ level: 'info', message, data, context })
            },
            warn(message, data, context) {
              calls.logger.push({ level: 'warn', message, data, context })
            },
            error(message, data, context) {
              calls.logger.push({ level: 'error', message, data, context })
            }
          }
        }
      }
      if (modulePath === '../utils/user-facing') {
        return { getErrorUserMessage: (_error, fallback) => fallback }
      }
      throw new Error(`unexpected require: ${modulePath}`)
    },
    getApp() {
      return app
    },
    wx: {
      showModal(options) {
        calls.modals.push(options)
        setTimeout(() => options.success && options.success({ confirm: modalConfirm, cancel: !modalConfirm }), 0)
      },
      showLoading(options) {
        calls.loading.push(options)
      },
      hideLoading() {
        calls.hiddenLoading += 1
      },
      showToast(options) {
        calls.toasts.push(options)
      },
      openBusinessView(options) {
        calls.openBusinessView.push(options)
        openBusinessView(options)
      }
    },
    Promise,
    setTimeout,
    Error
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return {
    module: sandbox.module.exports,
    calls,
    app
  }
}

async function testFallsBackToLocalConfirmWhenWechatComponentFails() {
  const loaded = loadModule({
    openBusinessView(options) {
      setTimeout(() => options.fail && options.fail({ errMsg: 'openBusinessView:fail 获取订单失败' }), 0)
    }
  })

  const result = await loaded.module.confirmReceiptWithRecovery({
    orderId: 42,
    transactionId: '420000000020260523000001'
  })

  assert.strictEqual(result.status, 'confirmed')
  assert.strictEqual(
    JSON.stringify(loaded.calls.openBusinessView.map((call) => call.extraData)),
    JSON.stringify([{ transaction_id: '420000000020260523000001' }])
  )
  assert.deepStrictEqual(loaded.calls.confirmOrder, [42])
  assert.strictEqual(loaded.app.globalData.pendingConfirmOrderId, undefined)
}

async function testKeepsWechatPendingWhenComponentOpens() {
  const loaded = loadModule({
    openBusinessView(options) {
      setTimeout(() => options.success && options.success({ errMsg: 'openBusinessView:ok' }), 0)
    }
  })

  const result = await loaded.module.confirmReceiptWithRecovery({
    orderId: 43,
    transactionId: '420000000020260523000002'
  })

  assert.strictEqual(result.status, 'wechat_opened')
  assert.deepStrictEqual(loaded.calls.confirmOrder, [])
  assert.strictEqual(loaded.app.globalData.pendingConfirmOrderId, 43)
}

async function testConfirmsLocallyWhenTransactionIsMissing() {
  const loaded = loadModule()

  const result = await loaded.module.confirmReceiptWithRecovery({ orderId: 44 })

  assert.strictEqual(result.status, 'confirmed')
  assert.deepStrictEqual(loaded.calls.openBusinessView, [])
  assert.deepStrictEqual(loaded.calls.confirmOrder, [44])
}

(async () => {
  await testFallsBackToLocalConfirmWhenWechatComponentFails()
  await testKeepsWechatPendingWhenComponentOpens()
  await testConfirmsLocallyWhenTransactionIsMissing()
})().then(() => {
  console.log('check-order-receipt-confirmation tests passed')
}, (error) => {
  console.error(error)
  process.exit(1)
})
