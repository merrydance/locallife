const fs = require('fs')
const path = require('path')

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

function requireAllFiles(prefix) {
  for (const ext of ['ts', 'wxml', 'json', 'wxss']) {
    requireFile(`${prefix}/index.${ext}`)
  }
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
requireContains('miniprogram/services/operator-finance.ts', 'buildOperatorCommissionBillMonthRange')

requireContains('miniprogram/pages/merchant/finance/bills/index.json', '"t-calendar"')
requireContains('miniprogram/pages/merchant/finance/bills/index.wxml', 'type="range"')
requireContains('miniprogram/pages/merchant/finance/bills/index.wxml', '区间收入')
requireContains('miniprogram/pages/merchant/finance/bills/index.wxml', '账单区间')
requireContains('miniprogram/pages/merchant/finance/bills/index.ts', 'onOpenRangePicker')
requireContains('miniprogram/pages/merchant/finance/bills/index.ts', 'onConfirmRangePicker')
requireContains('miniprogram/pages/merchant/finance/bills/index.ts', 'onUseQuickRange')
requireContains('miniprogram/pages/merchant/finance/bills/index.ts', 'getMerchantFinanceOverview')
requireContains('miniprogram/pages/merchant/finance/settlements/index.json', '"t-calendar"')
requireContains('miniprogram/pages/merchant/finance/settlements/index.wxml', 'type="range"')
requireContains('miniprogram/pages/merchant/finance/settlements/index.wxml', '区间分账')
requireContains('miniprogram/pages/merchant/finance/settlements/index.wxml', '账单区间')
requireContains('miniprogram/pages/merchant/finance/settlements/index.ts', 'onOpenRangePicker')
requireContains('miniprogram/pages/merchant/finance/settlements/index.ts', 'onConfirmRangePicker')
requireContains('miniprogram/pages/merchant/finance/settlements/index.ts', 'onUseQuickRange')

requireContains('miniprogram/pages/platform/dashboard/dashboard.ts', 'reconciliation')
requireContains('miniprogram/pages/platform/dashboard/dashboard.ts', '/pages/platform/finance/reconciliation/index')

requireContains('miniprogram/api/platform-dashboard.ts', 'getProfitSharingReconciliation')
requireContains('miniprogram/api/platform-dashboard.ts', 'getProfitSharingDetails')
requireContains('miniprogram/api/platform-dashboard.ts', 'getProfitSharingSla')
requireContains('miniprogram/api/platform-dashboard.ts', 'getBaofuDailyReconciliation')

requireContains('miniprogram/services/operator-finance.ts', 'loadOperatorCommissionBillPage')
requireContains('miniprogram/services/platform-finance-reconciliation.ts', 'loadPlatformFinanceReconciliationPage')

requireContains('miniprogram/pages/platform/finance/reconciliation/index.json', '"t-calendar"')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', 'type="range"')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.ts', 'onOpenRangePicker')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.ts', 'onConfirmRangePicker')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.ts', 'onLoadMoreDetails')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.ts', 'loadPlatformFinanceReconciliationDetailsPage')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', 'view.summaryCards')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', 'summary-card')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', 'view.detailRows')
requireContains('miniprogram/pages/platform/finance/reconciliation/index.wxml', '分账明细')
requireContains('miniprogram/services/platform-finance-reconciliation.ts', 'summary')
requireContains('miniprogram/services/platform-finance-reconciliation.ts', 'merchantFlowText')
requireContains('miniprogram/services/platform-finance-reconciliation.ts', 'riderFlowText')
requireContains('miniprogram/services/platform-finance-reconciliation.ts', 'merchantShareText')
requireContains('miniprogram/services/platform-finance-reconciliation.ts', 'riderShareText')
requireContains('miniprogram/services/platform-finance-reconciliation.ts', 'platformCommissionText')
requireContains('miniprogram/services/platform-finance-reconciliation.ts', 'buildProfitSharingDetailRows')
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
requireNotContains('miniprogram/services/platform-finance-reconciliation.ts', 'withdrawSucceededText')
requireNotContains('miniprogram/services/platform-finance-reconciliation.ts', 'currentAvailableAmountText')
requireNotContains('miniprogram/services/platform-finance-reconciliation.ts', 'providerText')
requireNotContains('miniprogram/services/platform-finance-reconciliation.ts', 'detailTarget')
requireNotContains('miniprogram/services/platform-finance-reconciliation.ts', 'getProfitSharingSla')
requireNotContains('miniprogram/services/platform-finance-reconciliation.ts', 'getBaofuDailyReconciliation')
requireNotContains('miniprogram/services/platform-finance-reconciliation.ts', 'getBaofuWithdrawalBalance')

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
requireContains('miniprogram/services/baofu-withdrawal-workflow.ts', 'buildBaofuWithdrawalLoadedSummaryView')

console.log('Finance bill pages contract check passed')
