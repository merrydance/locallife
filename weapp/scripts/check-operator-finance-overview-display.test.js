const fs = require('fs')
const path = require('path')
const assert = require('assert')

const repoRoot = path.resolve(__dirname, '..')
const read = (file) => fs.readFileSync(path.join(repoRoot, file), 'utf8')

const pageSource = read('miniprogram/pages/operator/finance/withdraw/index.ts')
const wxmlSource = read('miniprogram/pages/operator/finance/withdraw/index.wxml')
const serviceSource = read('miniprogram/pages/operator/_services/operator-finance.ts')
const normalizedServiceSource = serviceSource.replace(/\s+/g, ' ')

assert(
  !wxmlSource.includes('formatFen(') && !wxmlSource.includes('formatShareRatio('),
  'operator finance overview WXML must render preformatted view-model strings instead of calling Page methods'
)
assert(
  wxmlSource.includes('{{totalIncomeDisplay}}') &&
    wxmlSource.includes('{{currentMonthIncomeDisplay}}') &&
    wxmlSource.includes('{{operatorShareRatioDisplay}}') &&
    wxmlSource.includes('{{currentMonthGmvDisplay}}') &&
    wxmlSource.includes('{{currentMonthCommissionDisplay}}'),
  'operator finance overview WXML must bind display strings for all summary money and ratio fields'
)
assert(
  wxmlSource.includes('{{item.totalCommissionDisplay}}') &&
    wxmlSource.includes('{{item.totalGmvDisplay}}'),
  'operator finance commission rows must bind display strings for money fields'
)
assert(
  serviceSource.includes('totalIncomeDisplay') &&
    serviceSource.includes('currentMonthIncomeDisplay') &&
    serviceSource.includes('operatorShareRatioDisplay') &&
    serviceSource.includes('totalCommissionDisplay') &&
    serviceSource.includes('totalGmvDisplay'),
  'operator finance service must own money and ratio display fields before data reaches WXML'
)
assert(
  !pageSource.includes('formatFen(') && !pageSource.includes('formatShareRatio('),
  'operator finance page must not expose template-only formatter methods that WXML cannot call reliably'
)
assert(
  normalizedServiceSource.includes('operatorBasicManagementService.getFinanceOverview().catch(() => null)') &&
    normalizedServiceSource.includes('operatorBasicManagementService.getCommissionList({ page: 1, limit: 10 }).catch(() => null)'),
  'operator finance overview and recent commission must both use backend default all-active-region selection when no UI region filter exists'
)
assert(
  normalizedServiceSource.includes('operatorBasicManagementService.getCommissionList({ ...range, page, limit })'),
  'operator commission bills must not inject a default region_id that drifts from finance overview aggregation'
)
assert(
  serviceSource.includes("commissionError: commissionList ? '' : '佣金明细加载失败，请稍后重试'"),
  'operator finance commission failures must render an explicit error state instead of a successful empty list'
)

console.log('check-operator-finance-overview-display: operator finance overview renders preformatted values and default commission aggregation stays backend-owned')
