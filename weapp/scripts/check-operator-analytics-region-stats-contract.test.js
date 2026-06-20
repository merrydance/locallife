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
      if (id === '../_api/operator-analytics') {
        return {
          operatorAnalyticsService: {
            getRealtimeStats: async () => ({
              active_merchant_count: 8,
              active_rider_count: 5,
              pending_merchant_count: 1,
              pending_rider_count: 0
            }),
            getDailyTrend: async () => [],
            getRegionStats: async () => ({
              region_id: 596,
              region_name: '宁晋县',
              merchant_count: 8,
              total_orders: 12,
              total_gmv: 340000,
              total_commission: 2200
            })
          }
        }
      }
      if (id === '../_api/operator-merchant-management') {
        return { operatorMerchantManagementService: { getMerchantRanking: async () => [] } }
      }
      if (id === '../_api/operator-rider-management') {
        return { operatorRiderManagementService: { getRiderRanking: async () => [] } }
      }
      if (id === '../../../utils/util') {
        return {
          formatPrice: (amount) => `¥${(Number(amount || 0) / 100).toFixed(2)}`,
          formatPriceNoSymbol: (amount) => (Number(amount || 0) / 100).toFixed(2)
        }
      }
      throw new Error(`unexpected require: ${id}`)
    },
    Date,
    Math,
    Number,
    String,
    Array,
    Promise
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

async function main() {
  const apiSource = read('miniprogram/pages/operator/_api/operator-analytics.ts')
  const serviceSource = read('miniprogram/pages/operator/_services/operator-analytics-dashboard.ts')
  const pageSource = read('miniprogram/pages/operator/analytics/index.ts')
  const wxmlSource = read('miniprogram/pages/operator/analytics/index.wxml')

  for (const unsupported of [
    'merchant_stats',
    'rider_stats',
    'order_stats',
    'financial_stats',
    'growth_stats',
    'DataAnalysisService',
    'generateRegionAnalysisReport'
  ]) {
    assert(!apiSource.includes(unsupported), `operator analytics API must not expose unsupported nested stats: ${unsupported}`)
  }

  for (const required of ['merchant_count', 'total_orders', 'total_gmv', 'total_commission']) {
    assert(apiSource.includes(required), `operator region stats contract must include backend field ${required}`)
  }

  assert(!serviceSource.includes('.merchant_stats'), 'analytics service must not dereference nested merchant_stats')
  assert(!serviceSource.includes('.rider_stats'), 'analytics service must not dereference nested rider_stats')
  assert(!serviceSource.includes('.order_stats'), 'analytics service must not dereference nested order_stats')
  assert(!serviceSource.includes('.financial_stats'), 'analytics service must not dereference nested financial_stats')
  assert(serviceSource.includes('buildOperatorAnalyticsRegionSummary'), 'analytics service should adapt region stats through a view-model helper')

  const service = loadTsModule('miniprogram/pages/operator/_services/operator-analytics-dashboard.ts')
  const view = await service.loadOperatorAnalyticsPageData({
    timeDimension: 'week',
    selectedRegionId: 596,
    selectedRegionName: '宁晋县'
  })

  assert.deepStrictEqual(
    JSON.parse(JSON.stringify(view.regionSummary)),
    {
      regionName: '宁晋县',
      merchantText: '8',
      riderText: '5',
      orderText: '12',
      commission: '¥22.00'
    }
  )

  assert(pageSource.includes('analyticsRequestSeq'), 'analytics page should guard against stale region/time responses')
  assert(pageSource.includes('requestSeq !== analyticsRequestSeq'), 'analytics page should ignore stale loadData responses')
  assert(wxmlSource.includes('regionSummary.orderText'), 'analytics page should render backed order count text')
  assert(!wxmlSource.includes('履约完成率'), 'analytics page must not label unsupported completion-rate data')
  assert(!wxmlSource.includes('在线骑手 / 活跃骑手'), 'analytics page must not label unsupported online rider data')
}

main()
  .then(() => {
    console.log('check-operator-analytics-region-stats-contract: analytics region stats use backend truth')
  })
  .catch((error) => {
    console.error(error)
    process.exit(1)
  })
