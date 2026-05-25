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

const service = loadTsModule('miniprogram/services/platform-finance-reconciliation.ts')

const view = service.buildPlatformFinanceReconciliationPageView({
  range: { start_date: '2026-05-01', end_date: '2026-05-07' },
  reconciliationRows: [
    {
      status: 'finished',
      total_orders: 2,
      total_amount: 10000,
      total_merchant_flow: 8000,
      total_rider_flow: 1000,
      total_platform_commission: 600,
      total_operator_commission: 400,
      total_merchant_amount: 7900,
      total_rider_amount: 900
    },
    {
      status: 'processing',
      total_orders: 1,
      total_amount: 3000,
      total_merchant_flow: 2400,
      total_rider_flow: 300,
      total_platform_commission: 180,
      total_operator_commission: 120,
      total_merchant_amount: 2380,
      total_rider_amount: 290
    }
  ],
  detailsResponse: {
    items: [
      {
        id: 91,
        payment_order_id: 901,
        merchant_id: 23,
        operator_id: 8,
        rider_id: 7,
        order_source: 'takeout',
        status: 'finished',
        total_amount: 5000,
        merchant_flow: 4200,
        rider_flow: 500,
        platform_commission: 200,
        operator_commission: 100,
        merchant_amount: 4100,
        rider_amount: 450,
        out_order_no: 'PS20260501001',
        sharing_order_id: 'S20260501001',
        reconciliation_date: '2026-05-01',
        created_at: '2026-05-01T10:00:00Z',
        finished_at: '2026-05-01T10:05:00Z',
        provider: 'baofu',
        channel: 'wechat_jsapi',
        calculation_version: 'v1',
        settlement_mode: 'split',
        platform_receiver_amount: 200
      }
    ],
    total: 1,
    page_id: 1,
    page_size: 20,
    has_more: false
  }
})

assert.deepStrictEqual(
  JSON.parse(JSON.stringify(view.summaryCards.map((card) => [card.key, card.label, card.value, card.detailTarget]))),
  [
    ['merchant_flow', '商户流水', '¥104.00', 'profitSharingDetails'],
    ['rider_flow', '骑手流水', '¥13.00', 'profitSharingDetails'],
    ['platform_share', '平台分账', '¥7.80', 'profitSharingDetails'],
    ['merchant_share', '商户分账', '¥102.80', 'profitSharingDetails'],
    ['rider_share', '骑手分账', '¥11.90', 'profitSharingDetails'],
    ['operator_share', '运营商分账', '¥5.20', 'profitSharingDetails']
  ]
)

assert.strictEqual(view.detailRows.length, 1, 'page view must expose backed profit-sharing detail rows')
assert.strictEqual(view.detailRows[0].statusLabel, '已完成')
assert.strictEqual(view.detailRows[0].merchantFlowText, '¥42.00')
assert.strictEqual(view.detailsTotalText, '共 1 条')
assert.strictEqual(view.detailsHasMore, false)

const pageSource = read('miniprogram/pages/platform/finance/reconciliation/index.wxml')
const pageLogic = read('miniprogram/pages/platform/finance/reconciliation/index.ts')
const serviceSource = read('miniprogram/services/platform-finance-reconciliation.ts')
const dashboardApiSource = read('miniprogram/api/platform-dashboard.ts')
const summaryLoadStart = serviceSource.indexOf('export async function loadPlatformFinanceReconciliationPage')
const detailLoadStart = serviceSource.indexOf('export async function loadPlatformFinanceReconciliationDetailsPage')
const summaryLoadSource = summaryLoadStart >= 0 && detailLoadStart > summaryLoadStart
  ? serviceSource.slice(summaryLoadStart, detailLoadStart)
  : ''

assert(pageSource.includes('summary-card'), 'platform reconciliation page must render summary cards')
assert(pageSource.includes('view.summaryCards'), 'summary cards must come from service view model')
assert(pageSource.includes('bind:tap="onSummaryCardTap"'), 'summary cards must open the backed detail section')
assert(pageSource.includes('view.detailRows'), 'profit-sharing details section must render backed detail rows')
assert(pageSource.includes('onLoadMoreDetails'), 'profit-sharing details section must expose pagination')
assert(!pageSource.includes('range-card__caption'), 'range selector must not render a redundant explanatory caption')
assert(!pageSource.includes('分账订单金额'), 'legacy cell summary must not remain on the page')
assert(!pageSource.includes('当前可提现余额'), 'withdrawal balance cells do not belong on reconciliation detail page')
assert(!pageSource.includes('view.metrics'), 'SLA metric cards do not belong on this page')
assert(!pageSource.includes('分账状态汇总'), 'status aggregate must not appear below the detail list')
assert(!pageSource.includes('日对账明细'), 'daily Baofu reconciliation list must not appear below the detail list')
assert(!pageSource.includes('title="对账区间"'), 'range selector must not show the legacy 对账区间 cell title')
assert(!pageSource.includes('quick-range-row'), 'quick range row must be removed from the page')
assert(!pageLogic.includes('onUseQuickRange'), 'quick range handler must be removed')
assert(!pageLogic.includes('buildQuickRanges'), 'quick range view state must be removed')
assert(pageLogic.includes('loadPlatformFinanceReconciliationDetailsPage'), 'page must load backed profit-sharing detail pages')
assert(pageLogic.includes('detailsRequestSeq'), 'page must ignore stale detail responses after range changes')
assert(serviceSource.includes('buildProfitSharingDetailRows'), 'service must build a detail-row view model')
assert(serviceSource.includes('getProfitSharingDetails'), 'service must call the backed detail API')
assert(!serviceSource.includes('getProfitSharingSla'), 'page service must not request SLA metrics for this view')
assert(!serviceSource.includes('getBaofuDailyReconciliation'), 'page service must not request daily Baofu reconciliation for this view')
assert(!serviceSource.includes('getBaofuWithdrawalBalance'), 'page service must not request withdrawal balance for this view')
assert(summaryLoadSource, 'test must locate the summary load function')
assert(!summaryLoadSource.includes('getProfitSharingDetails'), 'summary load must not fail just because the detail page request failed')
assert(dashboardApiSource.includes('/v1/platform/stats/profit-sharing/details'), 'API layer must expose the detail endpoint')

const apiSource = read('../locallife/api/platform_stats.go')
const querySource = read('../locallife/db/query/profit_sharing_order.sql')

for (const required of [
  'TotalMerchantFlow',
  'TotalRiderFlow',
  'TotalMerchantAmount',
  'TotalRiderAmount'
]) {
  assert(apiSource.includes(required), `platform profit-sharing API must expose ${required}`)
}

for (const required of [
  'total_merchant_flow',
  'total_rider_flow',
  'total_merchant_amount',
  'total_rider_amount'
]) {
  assert(querySource.includes(required), `profit sharing reconciliation SQL must aggregate ${required}`)
}

console.log('platform reconciliation summary card contract tests passed')
