const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const sourcePath = path.join(__dirname, '..', 'miniprogram', 'pages', 'rider', '_utils', 'rider-delivery-view.ts')

function plain(value) {
  return JSON.parse(JSON.stringify(value))
}

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
    String,
    RegExp
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const {
  buildRiderDeliveryActionConfirmFeedback,
  buildRiderDeliveryActionFailureFeedback
} = loadModule()

const confirmFeedback = buildRiderDeliveryActionConfirmFeedback('confirm_delivery', '确认已送达')
assert.strictEqual(confirmFeedback.title, '确认送达')
assert.strictEqual(confirmFeedback.confirmText, '确认送达')
assert(confirmFeedback.content.includes('用户位置点'), 'confirm delivery copy should mention the user location point')
assert(confirmFeedback.content.includes('未到达时无法送达'), 'confirm delivery copy should explain the geofence consequence')

const distanceFeedback = buildRiderDeliveryActionFailureFeedback(
  { userMessage: '您距离代取地址142米，请靠近后确认送达（需在100米内）' },
  'confirm_delivery',
  '操作失败'
)
assert.strictEqual(distanceFeedback.mode, 'modal')
assert.strictEqual(distanceFeedback.title, '暂未到达送达点')
assert(distanceFeedback.content.includes('用户位置点142米'), 'distance copy should name the user location point')
assert(distanceFeedback.content.includes('需在100米内'), 'distance copy should preserve the backend radius')
assert(!distanceFeedback.content.includes('代取地址'), 'distance copy should not use the vague legacy dropoff wording')

const staleLocationFeedback = buildRiderDeliveryActionFailureFeedback(
  { message: '骑手定位已过期，无法确认送达，请刷新定位后重试' },
  'confirmDelivery',
  '操作失败'
)
assert.strictEqual(staleLocationFeedback.mode, 'modal')
assert.strictEqual(staleLocationFeedback.title, '定位未同步')
assert(staleLocationFeedback.content.includes('刷新定位后重试'), 'stale location copy should preserve the recovery step')

const pickupFeedback = buildRiderDeliveryActionFailureFeedback(
  { userMessage: '商户未出餐，暂不可确认取餐' },
  'confirm_pickup',
  '操作失败'
)
assert.deepStrictEqual(plain(pickupFeedback), {
  mode: 'toast',
  title: '商户未出餐，暂不可确认取餐'
})

console.log('check-rider-delivery-confirm-feedback tests passed')
