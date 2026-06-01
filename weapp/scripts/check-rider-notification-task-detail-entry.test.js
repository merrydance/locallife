const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const ROOT = path.join(__dirname, '..')

function compile(relativePath) {
  const sourcePath = path.join(ROOT, relativePath)
  const source = fs.readFileSync(sourcePath, 'utf8')
  return {
    source,
    sourcePath,
    compiled: ts.transpileModule(source, {
      compilerOptions: {
        module: ts.ModuleKind.CommonJS,
        target: ts.ScriptTarget.ES2018
      }
    }).outputText
  }
}

function loadNotificationPage() {
  const { compiled, sourcePath } = compile('miniprogram/pages/notification/index.ts')
  let pageConfig
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (modulePath === '../../api/notification') {
        return {
          notificationService: {
            getNotifications: () => Promise.resolve({ notifications: [], total: 0, page: 1, hasMore: false }),
            getUnreadCount: () => Promise.resolve({ count: 0 })
          }
        }
      }
      throw new Error(`unexpected require from notification page: ${modulePath}`)
    },
    Page(config) {
      pageConfig = config
    },
    Date,
    Math,
    Number,
    Promise,
    String,
    console,
    wx: { showToast() {}, navigateTo() {} }
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return { exports: sandbox.module.exports, pageConfig }
}

function loadTaskDetailPage() {
  const { compiled, sourcePath } = compile('miniprogram/pages/rider/task-detail/index.ts')
  let pageConfig
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (modulePath === '../_main_shared/api/delivery') {
        return { __esModule: true, default: {} }
      }
      if (modulePath === '../../../utils/logger') {
        return { logger: { error() {}, warn() {} } }
      }
      if (modulePath === '../../../utils/location') {
        return { locationService: {} }
      }
      if (modulePath === '../_main_shared/utils/rider-location') {
        return { normalizeLocationError: (err) => err, syncRiderDeliveryLocation: () => Promise.resolve() }
      }
      if (modulePath === '../_utils/rider-live-location') {
        return { riderLiveLocationSession: { setActiveDelivery: () => Promise.resolve() } }
      }
      if (modulePath === '../_utils/rider-delivery-view') {
        return {
          buildRiderDeliveryActionConfirmFeedback: () => ({ title: '' }),
          buildRiderDeliveryActionFailureFeedback: () => ({ mode: 'toast', title: '' }),
          buildRiderDeliveryDeadlineView: () => ({ text: '' }),
          getRiderDeliveryActionState: () => ({ canUpdate: false, label: '', actionKey: '' }),
          getRiderDeliveryStep: () => 0,
          isExpectedDeliveryStatusReached: () => false,
          isRiderDeliveryTrackedStatus: () => false
        }
      }
      if (modulePath === '../_utils/rider-delivery-income-view') {
        return { buildRiderDeliveryIncomeView: () => ({}) }
      }
      if (modulePath === '../../../utils/responsive') {
        return { getStableBarHeights: () => ({ navBarHeight: 88 }) }
      }
      throw new Error(`unexpected require from rider task detail page: ${modulePath}`)
    },
    Page(config) {
      pageConfig = config
    },
    Math,
    Number,
    Promise,
    String,
    wx: { showToast() {}, showModal() {}, showLoading() {}, hideLoading() {}, navigateBack() {}, makePhoneCall() {}, setClipboardData() {} }
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return { exports: sandbox.module.exports, pageConfig }
}

const notificationPage = loadNotificationPage()
assert(notificationPage.pageConfig, 'notification page should still register a Page config')
assert.strictEqual(
  typeof notificationPage.exports.resolveRiderNotificationUrl,
  'function',
  'notification page should export resolveRiderNotificationUrl for routing contract coverage'
)

const resolveRiderNotificationUrl = notificationPage.exports.resolveRiderNotificationUrl

assert.strictEqual(
  resolveRiderNotificationUrl({
    id: 1,
    type: 'delivery',
    title: '代取状态',
    content: '骑手代取状态更新',
    related_type: 'delivery',
    related_id: 301,
    extra_data: { order_id: 101 },
    is_read: false,
    created_at: '2026-05-27T10:00:00.000Z'
  }),
  '/pages/rider/task-detail/index?orderId=101',
  'delivery notifications must route with explicit orderId from extra_data instead of related delivery id'
)

assert.strictEqual(
  resolveRiderNotificationUrl({
    id: 2,
    type: 'delivery',
    title: '代取状态',
    content: '骑手代取状态更新',
    related_type: 'delivery',
    related_id: 301,
    extra_data: {},
    is_read: false,
    created_at: '2026-05-27T10:00:00.000Z'
  }),
  '/pages/rider/tasks/index',
  'delivery notifications without an order id must not pass delivery id to task detail'
)

assert.strictEqual(
  resolveRiderNotificationUrl({
    id: 3,
    type: 'order',
    title: '订单通知',
    content: '订单状态更新',
    related_type: 'order',
    related_id: 101,
    extra_data: {},
    is_read: false,
    created_at: '2026-05-27T10:00:00.000Z'
  }),
  '/pages/rider/task-detail/index?orderId=101',
  'order notifications should also route with explicit orderId'
)

const taskDetailPage = loadTaskDetailPage()
assert(taskDetailPage.pageConfig, 'rider task detail page should still register a Page config')
assert.strictEqual(
  typeof taskDetailPage.exports.resolveRiderTaskDetailOrderId,
  'function',
  'rider task detail page should export resolveRiderTaskDetailOrderId for entry contract coverage'
)

const resolveRiderTaskDetailOrderId = taskDetailPage.exports.resolveRiderTaskDetailOrderId
assert.strictEqual(resolveRiderTaskDetailOrderId({ orderId: '101' }), 101)
assert.strictEqual(resolveRiderTaskDetailOrderId({ order_id: '102' }), 102)
assert.strictEqual(resolveRiderTaskDetailOrderId({ id: '103' }), 103)
assert.strictEqual(
  resolveRiderTaskDetailOrderId({ deliveryId: '301' }),
  0,
  'deliveryId-only deep links must not be treated as order ids'
)

const missingParamPageState = {}
taskDetailPage.pageConfig.setData = function setData(patch) {
  Object.assign(missingParamPageState, patch)
}
taskDetailPage.pageConfig.fetchTaskDetail = function fetchTaskDetail() {
  throw new Error('task detail should not fetch without an order id')
}

taskDetailPage.pageConfig.onLoad({})
assert.strictEqual(
  missingParamPageState.errorMessage,
  '缺少订单号，请从我的任务重新进入',
  'task detail page must show an explicit recovery message when all order parameters are missing'
)

taskDetailPage.pageConfig.onLoad({ deliveryId: '301' })
assert.strictEqual(
  missingParamPageState.errorMessage,
  '缺少订单信息，请从我的任务重新进入',
  'deliveryId-only deep links must show a clear recovery message instead of failing silently'
)

console.log('check-rider-notification-task-detail-entry tests passed')
