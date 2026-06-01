const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const root = path.resolve(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(root, relativePath), 'utf8')
}

function requireFile(relativePath) {
  const fullPath = path.join(root, relativePath)
  if (!fs.existsSync(fullPath)) {
    throw new Error(`Missing required file: ${relativePath}`)
  }
  return fs.readFileSync(fullPath, 'utf8')
}

function requireContains(relativePath, expected) {
  const content = read(relativePath)
  if (!content.includes(expected)) {
    throw new Error(`${relativePath} must include ${expected}`)
  }
}

function requireNotContains(relativePath, unexpected) {
  const content = read(relativePath)
  if (content.includes(unexpected)) {
    throw new Error(`${relativePath} must not include ${unexpected}`)
  }
}

function requireAnyContains(relativePaths, expected, label) {
  const combined = relativePaths.map((relativePath) => read(relativePath)).join('\n')
  if (!combined.includes(expected)) {
    throw new Error(`${label} must include ${expected}`)
  }
}

function requireAllFiles(prefix) {
  for (const ext of ['ts', 'wxml', 'json', 'wxss']) {
    requireFile(`${prefix}/index.${ext}`)
  }
}

function loadTsModule(relativePath, requireStub = () => ({})) {
  const sourcePath = path.join(root, relativePath)
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
    require: requireStub,
    console
  }

  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

requireContains('miniprogram/app.json', '"finance/bills/index"')
requireContains('miniprogram/app.json', '"finance/reconciliation/index"')

requireAllFiles('miniprogram/pages/operator/finance/bills')
requireAllFiles('miniprogram/pages/platform/finance/reconciliation')
for (const prefix of [
  'miniprogram/pages/merchant/finance/bills',
  'miniprogram/pages/merchant/finance/settlements'
]) {
  for (const ext of ['ts', 'wxml', 'json']) {
    requireFile(`${prefix}/index.${ext}`)
  }
}

requireContains('miniprogram/pages/operator/finance/withdraw/index.wxml', '佣金账单')
requireContains('miniprogram/pages/operator/finance/withdraw/index.ts', 'onOpenBills')
requireContains('miniprogram/pages/operator/finance/bills/index.json', '"t-calendar"')
requireContains('miniprogram/pages/operator/finance/bills/index.wxml', 'type="range"')
requireContains('miniprogram/pages/operator/finance/bills/index.wxml', '区间佣金')
requireContains('miniprogram/pages/operator/finance/bills/index.ts', 'onOpenRangePicker')
requireContains('miniprogram/pages/operator/finance/bills/index.ts', 'onConfirmRangePicker')
requireContains('miniprogram/pages/operator/finance/bills/index.ts', 'onUseQuickRange')
requireContains('miniprogram/pages/operator/finance/bills/index.ts', 'validateFinanceDateRange')
requireContains('miniprogram/pages/operator/finance/bills/index.ts', '佣金账单最多选择365天')
requireContains('miniprogram/pages/operator/_services/operator-finance.ts', 'buildOperatorCommissionBillMonthRange')

requireContains('miniprogram/pages/merchant/finance/bills/index.json', '"t-calendar"')
requireContains('miniprogram/pages/merchant/finance/bills/index.wxml', 'type="range"')
requireContains('miniprogram/pages/merchant/finance/bills/index.wxml', '区间收入')
requireContains('miniprogram/pages/merchant/finance/bills/index.wxml', '账单区间')
requireContains('miniprogram/pages/merchant/finance/bills/index.ts', 'onOpenRangePicker')
requireContains('miniprogram/pages/merchant/finance/bills/index.ts', 'onConfirmRangePicker')
requireContains('miniprogram/pages/merchant/finance/bills/index.ts', 'onUseQuickRange')
requireContains('miniprogram/pages/merchant/finance/bills/index.ts', 'MERCHANT_FINANCE_BILL_MAX_RANGE_DAYS')
requireContains('miniprogram/pages/merchant/finance/bills/index.ts', '订单流水最多选择90天')
requireContains('miniprogram/pages/merchant/finance/bills/index.ts', 'loadMerchantFinanceBillPage')
requireContains('miniprogram/pages/merchant/_services/merchant-finance-workflow.ts', 'getMerchantFinanceOverview')
requireContains('miniprogram/pages/merchant/_services/merchant-finance-workflow.ts', 'loadMerchantFinanceBillPage')
requireContains('miniprogram/pages/merchant/_services/merchant-finance-workflow.ts', 'settleMerchantFinanceRequest')
requireContains('miniprogram/pages/merchant/_services/merchant-finance-workflow.ts', 'summaryErrorMessage')
requireContains('miniprogram/pages/merchant/_services/merchant-finance-workflow.ts', 'MERCHANT_FINANCE_BILL_MAX_RANGE_DAYS = 90')
requireContains('miniprogram/pages/merchant/_services/merchant-finance-workflow.ts', 'MERCHANT_FINANCE_SETTLEMENT_MAX_RANGE_DAYS = 365')
requireNotContains(
  'miniprogram/pages/merchant/_services/merchant-finance-workflow.ts',
  'Promise.all([\n    getMerchantFinanceOverview(range),\n    listMerchantFinanceOrders'
)
requireContains('miniprogram/pages/merchant/finance/settlements/index.json', '"t-calendar"')
requireContains('miniprogram/pages/merchant/finance/settlements/index.wxml', 'type="range"')
requireContains('miniprogram/pages/merchant/finance/settlements/index.wxml', '区间分账')
requireContains('miniprogram/pages/merchant/finance/settlements/index.wxml', '账单区间')
requireContains('miniprogram/pages/merchant/finance/settlements/index.ts', 'onOpenRangePicker')
requireContains('miniprogram/pages/merchant/finance/settlements/index.ts', 'onConfirmRangePicker')
requireContains('miniprogram/pages/merchant/finance/settlements/index.ts', 'onUseQuickRange')
requireContains('miniprogram/pages/merchant/finance/settlements/index.ts', 'MERCHANT_FINANCE_SETTLEMENT_MAX_RANGE_DAYS')
requireContains('miniprogram/pages/merchant/finance/settlements/index.ts', '结算记录最多选择365天')
requireContains('miniprogram/pages/merchant/finance/settlements/index.ts', 'loadMerchantFinanceSettlementPage')

requireAnyContains(
  ['miniprogram/pages/platform/dashboard/dashboard.ts', 'miniprogram/pages/platform/_services/platform-dashboard-view.ts'],
  'reconciliation',
  'platform dashboard entry surface'
)
requireAnyContains(
  ['miniprogram/pages/platform/dashboard/dashboard.ts', 'miniprogram/pages/platform/_services/platform-dashboard-view.ts'],
  '/pages/platform/finance/reconciliation/index',
  'platform dashboard entry surface'
)

requireContains('miniprogram/pages/platform/_api/platform-dashboard.ts', 'getProfitSharingReconciliation')
requireContains('miniprogram/pages/platform/_api/platform-dashboard.ts', 'getProfitSharingDetails')
requireContains('miniprogram/pages/platform/_api/platform-dashboard.ts', 'getProfitSharingSla')
requireContains('miniprogram/pages/platform/_api/platform-dashboard.ts', 'getBaofuDailyReconciliation')

requireContains('miniprogram/pages/operator/_services/operator-finance.ts', 'loadOperatorCommissionBillPage')
requireContains('miniprogram/pages/platform/_services/platform-finance-reconciliation.ts', 'loadPlatformFinanceReconciliationPage')

requireContains('miniprogram/pages/platform/finance/reconciliation/index.json', '"t-calendar"')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', 'type="range"')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.ts', 'onOpenRangePicker')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.ts', 'onConfirmRangePicker')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.ts', 'onLoadMoreDetails')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.ts', 'validateFinanceDateRange')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.ts', '对账账单最多选择365天')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.ts', 'loadPlatformFinanceReconciliationDetailsPage')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', 'view.summaryCards')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', 'summary-card')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', 'view.detailRows')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', '分账明细')
requireContains('miniprogram/pages/platform/_services/platform-finance-reconciliation.ts', 'summary')
requireContains('miniprogram/pages/platform/_services/platform-finance-reconciliation.ts', 'merchantFlowText')
requireContains('miniprogram/pages/platform/_services/platform-finance-reconciliation.ts', 'riderFlowText')
requireContains('miniprogram/pages/platform/_services/platform-finance-reconciliation.ts', 'merchantShareText')
requireContains('miniprogram/pages/platform/_services/platform-finance-reconciliation.ts', 'riderShareText')
requireContains('miniprogram/pages/platform/_services/platform-finance-reconciliation.ts', 'platformCommissionText')
requireContains('miniprogram/pages/platform/_services/platform-finance-reconciliation.ts', 'buildProfitSharingDetailRows')
requireNotContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', '可选择区间日期的日历')
requireNotContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', '分账状态汇总')
requireNotContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', '日对账明细')
requireNotContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', '区间提现成功')
requireNotContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', '区间提现申请处理中')
requireNotContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', '当前账户在途提现')
requireNotContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', '分账订单金额')
requireNotContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', '当前可提现余额')
requireNotContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', 'view.metrics')
requireNotContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', 'bind:tap="onSummaryCardTap"')
requireNotContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', 'providerText')
requireNotContains('miniprogram/pages/platform/finance/reconciliation/index.ts', 'onSummaryCardTap')
requireNotContains('miniprogram/pages/platform/_services/platform-finance-reconciliation.ts', 'withdrawSucceededText')
requireNotContains('miniprogram/pages/platform/_services/platform-finance-reconciliation.ts', 'currentAvailableAmountText')
requireNotContains('miniprogram/pages/platform/_services/platform-finance-reconciliation.ts', 'providerText')
requireNotContains('miniprogram/pages/platform/_services/platform-finance-reconciliation.ts', 'detailTarget')
requireNotContains('miniprogram/pages/platform/_services/platform-finance-reconciliation.ts', 'getProfitSharingSla')
requireNotContains('miniprogram/pages/platform/_services/platform-finance-reconciliation.ts', 'getBaofuDailyReconciliation')
requireNotContains('miniprogram/pages/platform/_services/platform-finance-reconciliation.ts', 'getBaofuWithdrawalBalance')

requireContains('miniprogram/pages/platform/finance/withdrawals/index.ts', 'buildBaofuWithdrawalLoadedSummaryView')
requireContains('miniprogram/pages/platform/finance/withdrawals/index.wxml', '当前列表小计')
requireContains('miniprogram/pages/platform/finance/withdrawals/index.wxml', '账户在途提现')
requireContains('miniprogram/pages/platform/finance/withdrawals/index.wxml', '列表申请处理中')
requireContains('miniprogram/pages/platform/finance/withdrawals/index.wxml', '账面余额')
requireContains('miniprogram/pages/platform/finance/withdrawals/index.wxml', '冻结金额')
requireContains('miniprogram/pages/operator/finance/withdrawals/index.wxml', '账户在途提现')
requireContains('miniprogram/pages/merchant/finance/withdrawals/index.wxml', '账户在途提现')
requireContains('miniprogram/pages/merchant/finance/withdrawals/index.wxml', '当前列表小计')
requireContains('miniprogram/pages/merchant/finance/withdrawals/index.wxml', '列表申请处理中')
requireContains('miniprogram/pages/merchant/finance/withdrawals/index.wxml', '账面余额')
requireContains('miniprogram/pages/merchant/finance/withdrawals/index.wxml', '冻结金额')
requireContains('miniprogram/pages/merchant/finance/withdrawals/index.ts', 'buildBaofuWithdrawalLoadedSummaryView')
requireContains('miniprogram/pages/rider/income/withdrawals/index.wxml', '账户在途提现')
requireContains('miniprogram/pages/platform/finance/withdrawals/create/index.ts', 'buildBaofuWithdrawalSubmitCheck')
requireContains('miniprogram/pages/platform/finance/withdrawals/create/index.ts', 'onAmountChange')
requireContains('miniprogram/pages/platform/finance/withdrawals/create/index.wxml', 'label="提现金额"')
requireContains('miniprogram/pages/platform/finance/withdrawals/create/index.wxml', '单笔最高提现')
requireContains('miniprogram/pages/operator/finance/withdrawals/create/index.wxml', 'label="提现金额"')
requireContains('miniprogram/pages/operator/finance/withdrawals/create/index.wxml', '单笔最高提现')
requireContains('miniprogram/pages/merchant/_main_shared/services/baofu-withdrawal-workflow.ts', 'buildBaofuWithdrawalLoadedSummaryView')

async function runMerchantFinanceWorkflowChecks() {
  let overviewShouldReject = false
  let ordersShouldReject = false
  const workflow = loadTsModule('miniprogram/pages/merchant/_services/merchant-finance-workflow.ts', (id) => {
    if (id === '../_main_shared/api/merchant-finance') {
      return {
        getMerchantFinanceOrderStatusView() {
          return { text: '已完成', theme: 'success' }
        },
        getMerchantFinanceOverview() {
          if (overviewShouldReject) {
            return Promise.reject(new Error('overview failed'))
          }
          return Promise.resolve({
            completed_orders: 1,
            total_gmv: 2000,
            total_merchant_receivable_amount: 1800,
            total_deduction_fee_amount: 200,
            pending_merchant_receivable_amount: 0,
            net_income: 1800
          })
        },
        listMerchantFinanceOrders() {
          if (ordersShouldReject) {
            return Promise.reject(new Error('orders failed'))
          }
          return Promise.resolve({
            orders: [{
              id: 8,
              order_source: 'takeout',
              merchant_receivable_amount: 1800,
              status: 'finished',
              created_at: '2026-05-20T10:00:00Z'
            }],
            total: 1,
            page: 1,
            limit: 20,
            total_pages: 1
          })
        },
        listMerchantSettlements() {
          return Promise.resolve({
            settlements: [],
            total: 0,
            page: 1,
            limit: 20,
            total_pages: 0,
            total_amount: 0,
            total_merchant_receivable_amount: 0,
            total_platform_service_fee_amount: 0,
            total_payment_channel_fee_amount: 0
          })
        }
      }
    }
    if (id === '../../../utils/user-facing') {
      return {
        getErrorUserMessage(_error, fallback) {
          return fallback
        }
      }
    }
    if (id === '../_main_shared/utils/finance-date-range') {
      return loadTsModule('miniprogram/pages/merchant/_main_shared/utils/finance-date-range.ts')
    }
    return {}
  })

  assert.strictEqual(workflow.MERCHANT_FINANCE_BILL_MAX_RANGE_DAYS, 90)
  assert.strictEqual(workflow.MERCHANT_FINANCE_SETTLEMENT_MAX_RANGE_DAYS, 365)
  const invalidBillRange = workflow.validateMerchantFinanceRange(
    { start_date: '2026-01-01', end_date: '2026-04-02' },
    workflow.MERCHANT_FINANCE_BILL_MAX_RANGE_DAYS,
    '订单流水'
  )
  assert.strictEqual(invalidBillRange.valid, false)
  assert.strictEqual(invalidBillRange.message, '订单流水最多选择90天')

  const validBillRange = workflow.validateMerchantFinanceRange(
    { start_date: '2026-01-01', end_date: '2026-04-01' },
    workflow.MERCHANT_FINANCE_BILL_MAX_RANGE_DAYS,
    '订单流水'
  )
  assert.strictEqual(validBillRange.valid, true)
  assert.strictEqual(validBillRange.message, '')

  overviewShouldReject = true
  const partialView = await workflow.loadMerchantFinanceBillPage({
    range: { start_date: '2026-05-01', end_date: '2026-05-20' },
    page: 1,
    limit: 20
  })
  assert.strictEqual(partialView.rows.length, 1)
  assert.strictEqual(partialView.summary.totalIncomeText, '--')
  assert.strictEqual(partialView.summary.totalGmvText, '--')
  assert.strictEqual(partialView.summaryErrorMessage, '汇总同步失败，订单流水可继续查看')

  overviewShouldReject = false
  ordersShouldReject = true
  await assert.rejects(
    () => workflow.loadMerchantFinanceBillPage({
      range: { start_date: '2026-05-01', end_date: '2026-05-20' },
      page: 1,
      limit: 20
    }),
    /orders failed/
  )
}

runMerchantFinanceWorkflowChecks()
  .then(() => {
    console.log('Finance bill pages contract check passed')
  })
  .catch((error) => {
    console.error(error.message)
    process.exit(1)
  })
