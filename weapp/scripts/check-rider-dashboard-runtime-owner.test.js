const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const repoRoot = path.join(__dirname, '..')
const ownerPath = path.join(repoRoot, 'miniprogram', 'pages', 'rider', '_utils', 'rider-dashboard-delivery-view.ts')
const runtimePath = path.join(repoRoot, 'miniprogram', 'pages', 'rider', '_utils', 'rider-dashboard-runtime.ts')
const workbenchPath = path.join(repoRoot, 'miniprogram', 'pages', 'rider', '_services', 'rider-workbench.ts')

function plain(value) {
  return JSON.parse(JSON.stringify(value))
}

function loadOwnerModule() {
  const source = fs.readFileSync(ownerPath, 'utf8')
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
        return {
          getDeliveryStatusDisplay(status) {
            const map = {
              assigned: { text: '骑手已接单', theme: 'primary' },
              picking: { text: '骑手正在取餐', theme: 'primary' },
              picked: { text: '骑手已取餐', theme: 'primary' },
              delivering: { text: '骑手正在代取', theme: 'primary' },
              completed: { text: '已送达', theme: 'success' }
            }
            return map[status] || { text: status || '', theme: 'default' }
          }
        }
      }
      if (modulePath === '../_api/rider-workbench') {
        return {}
      }
      if (modulePath === '../_utils/rider-delivery-view') {
        return {
          buildRiderDeliveryDeadlineView(delivery) {
            return {
              text: delivery.status === 'completed' ? '已送达' : '尽快送达',
              isOverdue: false,
              isVeryUrgent: delivery.status === 'delivering'
            }
          },
          getRiderDeliveryActionState(delivery) {
            if (delivery.status === 'picking' && delivery.can_confirm_pickup !== true) {
              return {
                canUpdate: false,
                label: delivery.pickup_action_label || '等待商户出餐',
                disabledReason: delivery.pickup_block_reason || '当前任务状态暂不可用，请稍后重试'
              }
            }
            return { canUpdate: true, label: '确认取餐', disabledReason: '' }
          }
        }
      }
      if (modulePath === '../_utils/rider-delivery-income-view') {
        return {
          buildRiderDeliveryIncomeView() {
            return { summaryText: '¥7.20', hasBill: true }
          }
        }
      }
      if (modulePath === '../_main_shared/utils/status-tag') {
        return {
          resolveStatusTagTheme(tone) {
            return tone === 'warning' ? 'warning' : 'success'
          }
        }
      }
      throw new Error(`unexpected require: ${modulePath}`)
    },
    Date,
    Math,
    Number,
    String,
    Set
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: ownerPath })
  return sandbox.module.exports
}

const owner = loadOwnerModule()

assert.deepStrictEqual(plain(owner.buildRiderDashboardBannerState([])), {
  message: '',
  canRetry: true
})
assert.deepStrictEqual(plain(owner.buildRiderDashboardBannerState([
  { message: '网络已断开', canRetry: true },
  { message: '网络已断开', canRetry: true },
  { message: '我的任务加载失败', canRetry: false }
])), {
  message: '网络已断开；我的任务加载失败',
  canRetry: false
})

assert.strictEqual(owner.isRiderDashboardTrackableDelivery('assigned'), true)
assert.strictEqual(owner.isRiderDashboardTrackableDelivery('delivering'), true)
assert.strictEqual(owner.isRiderDashboardTrackableDelivery('completed'), false)

const blockedPicking = owner.buildDashboardDeliveryView({
  id: 1001,
  order_id: 2001,
  status: 'picking',
  can_confirm_pickup: false,
  pickup_block_reason: '商户未出餐，暂不可确认取餐',
  pickup_action_label: '等待商户出餐',
  delivery_fee: 800,
  rider_earnings: 720,
  pickup_address: '门店',
  pickup_longitude: 0,
  pickup_latitude: 0,
  delivery_address: '顾客',
  delivery_longitude: 0,
  delivery_latitude: 0
}, [1001])

assert.strictEqual(blockedPicking.status_desc, '骑手正在取餐')
assert.strictEqual(blockedPicking.status_tag_theme, 'warning')
assert.strictEqual(blockedPicking.can_confirm_pickup, false)
assert.strictEqual(blockedPicking.pickup_block_reason, '商户未出餐，暂不可确认取餐')
assert.strictEqual(blockedPicking.pickup_action_label, '等待商户出餐')
assert.strictEqual(blockedPicking.is_action_loading, true)
assert.strictEqual(blockedPicking.income_view.summaryText, '¥7.20')

const workbenchDelivery = owner.buildWorkbenchDashboardDeliveryView({
  id: 1002,
  order_id: 2002,
  status: 'picked',
  delivery_fee: 900,
  rider_earnings: 810,
  pickup_address: '门店',
  delivery_address: '顾客'
})
assert.strictEqual(workbenchDelivery.pickup_longitude, 0)
assert.strictEqual(workbenchDelivery.delivery_longitude, 0)
assert.strictEqual(workbenchDelivery.can_start_delivery, true)

const nearCheck = owner.buildRiderDashboardGrabDistanceCheck({
  currentLatitude: 31.2304,
  currentLongitude: 121.4737,
  pickupLatitude: 31.2305,
  pickupLongitude: 121.4738
})
assert.strictEqual(nearCheck.canGrab, true)
assert.strictEqual(nearCheck.message, '')

const farCheck = owner.buildRiderDashboardGrabDistanceCheck({
  currentLatitude: 31.2304,
  currentLongitude: 121.4737,
  pickupLatitude: 31.3304,
  pickupLongitude: 121.5737
})
assert.strictEqual(farCheck.canGrab, false)
assert(farCheck.message.includes('仅限5km内抢单'), 'far-distance message should keep the 5km grab rule visible')

const runtimeSource = fs.readFileSync(runtimePath, 'utf8')
const forbiddenRuntimePatterns = [
  'function buildBannerState',
  'function isTrackableDelivery',
  'function buildDashboardDeliveryActionState',
  'function getDistance',
  'MAX_GRAB_DISTANCE'
]

for (const pattern of forbiddenRuntimePatterns) {
  assert(!runtimeSource.includes(pattern), `rider-dashboard-runtime.ts must not own ${pattern}`)
}

assert(
  runtimeSource.includes("from './rider-dashboard-delivery-view'"),
  'rider-dashboard-runtime.ts must consume rider-dashboard-delivery-view owner'
)

const workbenchSource = fs.readFileSync(workbenchPath, 'utf8')
assert(
  workbenchSource.includes('buildWorkbenchDashboardDeliveryView'),
  'rider-workbench service must share the dashboard delivery view builder'
)

console.log('check-rider-dashboard-runtime-owner tests passed')
