const assert = require('assert')
const fs = require('fs')
const path = require('path')

const repoRoot = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), 'utf8')
}

const apiSource = read('miniprogram/api/table-device-management.ts')
const pageTsSource = read('miniprogram/pages/merchant/printers/index.ts')
const pageWxmlSource = read('miniprogram/pages/merchant/printers/index.wxml')
const pageJsonSource = read('miniprogram/pages/merchant/printers/index.json')
const reconciliationViewSource = read('miniprogram/utils/printer-reconciliation-view.ts')
const componentTsSource = read('miniprogram/components/merchant-printer-reconciliation-section/index.ts')
const componentWxmlSource = read('miniprogram/components/merchant-printer-reconciliation-section/index.wxml')

assert(
  apiSource.includes('listPrinterReconciliationJobs(') &&
    apiSource.includes("url: '/v1/merchant/devices/reconciliation-jobs'"),
  'device management service should keep the backend reconciliation-job list wrapper'
)

assert(
  apiSource.includes('retryPrinterReconciliationJob(') &&
    apiSource.includes('`/v1/merchant/devices/reconciliation-jobs/${jobId}/retry`'),
  'device management service should keep the backend reconciliation retry wrapper'
)

assert(
  pageTsSource.includes('PrinterReconciliationJobView') &&
    pageTsSource.includes('buildPrinterReconciliationJobView') &&
    reconciliationViewSource.includes('buildPrinterReconciliationJobStatusView'),
  'merchant printer page should build a typed reconciliation-job view model from backend status'
)

assert(
  pageTsSource.includes('reconciliationJobs: [] as PrinterReconciliationJobView[]') &&
    pageTsSource.includes('reconciliationErrorMessage') &&
    pageTsSource.includes('retryingReconciliationJobId'),
  'merchant printer page should keep reconciliation jobs, local load error, and retrying state'
)

assert(
  pageTsSource.includes('buildReconciliationLoadErrorMessage') &&
    pageTsSource.includes('当前已保留上次同步结果'),
  'reconciliation refresh failures should preserve and label the last trusted recovery state'
)

assert(
  pageTsSource.includes("from '../../../utils/promise'") &&
    /settleAll\(\[[\s\S]*deviceManagementService\.listPrinters\(\)[\s\S]*deviceManagementService\.listPrinterReconciliationJobs\('pending'\)/.test(pageTsSource),
  'merchant printer page should load printers and pending reconciliation jobs together without making recovery status unreachable'
)

assert(
  pageTsSource.includes('onRetryReconciliationJob') &&
    pageTsSource.includes('deviceManagementService.retryPrinterReconciliationJob') &&
    pageTsSource.includes('retryingReconciliationJobId'),
  'merchant printer page should expose a guarded retry action for reconciliation jobs'
)

assert(
  pageTsSource.includes('设备同步恢复已完成') &&
    pageTsSource.includes('await this.loadPageData(false, true)'),
  'successful reconciliation retry should give durable feedback and re-read backend truth'
)

const retryHandlerMatch = pageTsSource.match(/async onRetryReconciliationJob[\s\S]*?\n  },/)
assert(retryHandlerMatch, 'merchant printer page should keep an explicit reconciliation retry handler')
assert(
  !retryHandlerMatch[0].includes('wx.showToast'),
  'reconciliation retry failures should use the inline recovery notice instead of duplicating feedback through Toast'
)

assert(
  !pageWxmlSource.includes('last_error') &&
    !pageWxmlSource.includes('failure_reason') &&
    !pageWxmlSource.includes('lastError') &&
    !pageWxmlSource.includes('failureReason') &&
    !componentWxmlSource.includes('last_error') &&
    !componentWxmlSource.includes('failure_reason') &&
    !componentWxmlSource.includes('lastError') &&
    !componentWxmlSource.includes('failureReason'),
  'merchant printer page must not render raw provider reconciliation diagnostics'
)

assert(
  pageJsonSource.includes('merchant-printer-reconciliation-section') &&
    pageWxmlSource.includes('<merchant-printer-reconciliation-section') &&
    componentWxmlSource.includes('jobs.length') &&
    componentWxmlSource.includes('onRetryTap') &&
    componentWxmlSource.includes('retryingJobId === item.id') &&
    componentWxmlSource.includes('设备同步异常'),
  'merchant printer page should render pending reconciliation jobs with retry affordance'
)

assert(
  pageWxmlSource.includes('reconciliationErrorMessage') &&
    pageWxmlSource.includes('onRefreshReconciliationJobs') &&
    componentTsSource.includes("this.triggerEvent('refresh')") &&
    componentTsSource.includes("this.triggerEvent('retry', { id })"),
  'merchant printer page should render reconciliation load failure with an inline retry path'
)

console.log('check-merchant-printer-reconciliation-recovery: printer page exposes merchant-visible recovery for cloud-printer reconciliation jobs')
