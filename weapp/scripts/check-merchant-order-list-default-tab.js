const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const ROOT = path.resolve(__dirname, '..')
const orderApiPath = path.join(ROOT, 'miniprogram/api/order-management.ts')
const orderListPagePath = path.join(ROOT, 'miniprogram/pages/merchant/orders/list/index.ts')

function compile(filePath) {
  return ts.transpileModule(fs.readFileSync(filePath, 'utf8'), {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018
    }
  }).outputText
}

function loadOrderManagementApi() {
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (modulePath === '../utils/request') {
        return { request: () => Promise.resolve({}) }
      }
      throw new Error(`unexpected require from order-management: ${modulePath}`)
    }
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compile(orderApiPath), sandbox, { filename: orderApiPath })
  return sandbox.module.exports
}

function loadMerchantOrderListPage(orderManagementApi) {
  let pageConfig
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (modulePath === '../../../../utils/responsive') {
        return { getStableBarHeights: () => ({ navBarHeight: 88 }) }
      }
      if (modulePath === '../../../../api/order-management') {
        return {
          ...orderManagementApi,
          MerchantOrderManagementService: { getOrderList: () => Promise.resolve({ orders: [], total: 0 }) },
          OrderManagementAdapter: {},
          MERCHANT_REJECT_REASON_OPTIONS: []
        }
      }
      if (modulePath === '../../../../utils/logger') {
        return { logger: { error() {}, warn() {}, info() {} } }
      }
      if (modulePath === 'dayjs') {
        return () => ({ format: () => '' })
      }
      if (modulePath === '../../../../utils/user-facing') {
        return { getErrorUserMessage: (_err, fallback) => fallback }
      }
      if (modulePath === '../../../../utils/console-access') {
        return {
          ensureMerchantConsoleAccess: () => Promise.resolve('granted'),
          getMerchantConsoleAccessErrorMessage: () => '',
          isMerchantConsoleAccessDenied: () => false,
          isMerchantConsoleAccessGranted: () => true
        }
      }
      if (modulePath === '../../../../utils/merchant-order-detail-view') {
        return { buildMerchantOrderFeeBreakdownView: () => ({ customer_payable_text: '', merchant_receivable_text: '', available: false }) }
      }
      if (modulePath === '../../../../utils/websocket') {
        return {
          wsManager: { connect() {}, disconnect() {}, on: () => () => {} },
          WSMessageType: { NOTIFICATION: 'notification', CONNECTION_BLOCKED: 'connection_blocked' }
        }
      }
      throw new Error(`unexpected require from merchant order list page: ${modulePath}`)
    },
    Page(config) {
      pageConfig = config
    },
    wx: { stopPullDownRefresh() {}, showToast() {}, navigateTo() {}, showActionSheet() {} },
    Promise,
    setTimeout,
    clearTimeout
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compile(orderListPagePath), sandbox, { filename: orderListPagePath })
  return pageConfig
}

const orderManagementApi = loadOrderManagementApi()
assert.strictEqual(
  orderManagementApi.normalizeMerchantVisibleOrderStatusFilter(undefined),
  '',
  'merchant order status filter should default to the all-orders tab when no status is provided'
)
assert.strictEqual(
  orderManagementApi.normalizeMerchantVisibleOrderStatusFilter('pending'),
  'paid',
  'pending remains hidden from merchant tabs and should fall back to paid'
)

const pageConfig = loadMerchantOrderListPage(orderManagementApi)
assert(pageConfig, 'merchant order list page config should be registered')
assert.strictEqual(pageConfig.data.currentStatus, '', 'merchant order list initial tab should be all orders')

console.log('check-merchant-order-list-default-tab: merchant order list defaults to all tab')
