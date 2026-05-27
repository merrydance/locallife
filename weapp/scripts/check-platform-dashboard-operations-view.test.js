const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const ROOT = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(ROOT, relativePath), 'utf8')
}

function loadTsModule(relativePath) {
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
    require(id) {
      if (id === '../api/platform-dashboard') return {}
      throw new Error(`unexpected require: ${id}`)
    },
    Date,
    Math,
    Number,
    String,
    Array
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const service = loadTsModule('miniprogram/services/platform-dashboard-view.ts')

const view = service.buildPlatformDashboardView({
  overviewData: {
    active_merchants: 12,
    active_users: 88,
    total_commission: 5300,
    total_gmv: 1250000,
    total_orders: 321,
    summary: {
      total_orders: 321,
      total_gmv: 1250000,
      completion_rate: 96.3,
      active_merchants: 12,
      active_riders: 9,
      avg_order_value: 3894
    },
    growth_metrics: {
      user_growth_rate: 1,
      merchant_growth_rate: 2,
      order_growth_rate: 3,
      gmv_growth_rate: 4
    }
  },
  realtimeData: {
    active_merchants_24h: 7,
    active_users_24h: 41,
    delivering_orders: 4,
    gmv_24h: 456700,
    orders_24h: 37,
    pending_orders: 5,
    preparing_orders: 6,
    ready_orders: 3,
    order_distribution: {
      pending: 5,
      confirmed: 2,
      preparing: 6,
      ready: 3,
      delivering: 4,
      completed: 20,
      cancelled: 1
    },
    today_stats: {
      new_users: 8,
      new_merchants: 2,
      gmv: 456700,
      order_count: 37,
      total_orders: 37,
      completed_orders: 20,
      cancelled_orders: 1,
      total_gmv: 456700,
      avg_order_value: 12343,
      completion_rate: 96.3,
      new_riders: 1
    },
    realtime_stats: {
      online_riders: 9,
      online_merchants: 7,
      orders_per_minute: 2,
      online_users: 13,
      active_orders: 20,
      gmv_per_minute: 0
    },
    top_regions: [],
    hourly_trends: [],
    timestamp: Date.parse('2026-05-27T09:30:00+08:00')
  },
  abnormalRefundCount: 2,
  alertFeedReady: true
})

assert.deepStrictEqual(
  JSON.parse(JSON.stringify(view.todayCards.map((card) => [card.key, card.label, card.value]))),
  [
    ['orders24h', '今日订单', '37'],
    ['gmv24h', '今日GMV', '¥4,567.00'],
    ['activeOrders', '履约中', '20'],
    ['onlineRiders', '在线骑手', '9']
  ]
)

assert.deepStrictEqual(
  JSON.parse(JSON.stringify(view.riskItems.map((item) => [item.key, item.title, item.value, item.theme, item.url || '']))),
  [
    ['abnormalRefunds', '异常退款待处理', '2', 'danger', '/pages/platform/finance/reconciliation/index'],
    ['pendingOrders', '待接单订单', '5', 'warning', ''],
    ['reviewQueue', '经营实体待巡检', '点入查看', 'warning', '/pages/platform/riders/index']
  ]
)

assert.strictEqual(view.operationsStatus.syncText, '09:30 更新')
assert.strictEqual(view.operationsStatus.summary, '今日 37 单，履约中 20 单')
assert.strictEqual(view.entryGroups[0].title, '经营实体')
assert(view.entryGroups[0].items.some((item) => item.title === '骑手管理' && item.url === '/pages/platform/riders/index'), 'rider management entry must link to rider cards')
assert(view.entryGroups[0].items.some((item) => item.title === '商户管理' && item.url === '/pages/platform/merchants/index'), 'merchant management entry must link to merchant cards')
assert(view.entryGroups[0].items.some((item) => item.title === '运营商管理' && item.url === '/pages/platform/operators/index'), 'operator management entry must link to operator cards')
assert.strictEqual(view.entryGroups[1].title, '资金结算')
assert(view.entryGroups[1].items.some((item) => item.url === '/pages/platform/finance/withdrawals/index'), 'finance entries must include withdrawals')
assert(!JSON.stringify(view).includes('接口'), 'dashboard view must not expose technical interface wording')
assert(!JSON.stringify(view).includes('商户保证金'), 'dashboard view must not expose unsupported merchant deposit wording')

const pageSource = read('miniprogram/pages/platform/dashboard/dashboard.wxml')
const pageLogic = read('miniprogram/pages/platform/dashboard/dashboard.ts')
const appJson = JSON.parse(read('miniprogram/app.json'))
const platformPackage = appJson.subPackages.find((item) => item.root === 'pages/platform')
const operationalConfigSource = read('miniprogram/pages/platform/operational-configs/index.ts')
const operationalConfigView = read('miniprogram/pages/platform/operational-configs/index.wxml')

assert(pageSource.includes('todayCards'), 'dashboard page must render the operational first-screen cards')
assert(pageSource.includes('riskItems'), 'dashboard page must render the risk and todo strip')
assert(pageSource.includes('entryGroups'), 'dashboard page must render grouped management entries')
assert(!pageSource.includes('基础统计'), 'legacy cumulative-first section must be removed from dashboard first screen')
assert(!pageSource.includes('管理入口'), 'legacy flat entry grid heading must be removed')
assert(pageLogic.includes('buildPlatformDashboardView'), 'page must build view state through the dashboard view model')
assert(pageLogic.includes('dashboardView'), 'page data must expose a single dashboard view model')
assert(platformPackage.pages.includes('riders/detail'), 'platform package must register rider detail route')
assert(platformPackage.pages.includes('merchants/index'), 'platform package must register merchant management route')
assert(platformPackage.pages.includes('merchants/detail'), 'platform package must register merchant detail route')
assert(fs.existsSync(path.join(ROOT, 'miniprogram/pages/platform/merchants/index.wxml')), 'merchant list WXML must exist')
assert(fs.existsSync(path.join(ROOT, 'miniprogram/pages/platform/merchants/detail.wxml')), 'merchant detail WXML must exist')
assert(!operationalConfigSource.includes('MERCHANT_DEPOSIT'), 'operational config must not submit unsupported merchant deposit key')
assert(!operationalConfigSource.includes('merchantDeposit'), 'operational config state must not keep unsupported merchant deposit fields')
assert(!operationalConfigView.includes('商户保证金'), 'operational config view must not render unsupported merchant deposit copy')
assert(!operationalConfigView.includes('ruleValues.MERCHANT_DEPOSIT'), 'operational config view must not read unsupported merchant deposit value')

console.log('platform dashboard operations view tests passed')
