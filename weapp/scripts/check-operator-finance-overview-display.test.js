const fs = require('fs')
const path = require('path')
const assert = require('assert')

const repoRoot = path.resolve(__dirname, '..')
const read = (file) => fs.readFileSync(path.join(repoRoot, file), 'utf8')

const pageSource = read('miniprogram/pages/operator/finance/withdraw/index.ts')
const wxmlSource = read('miniprogram/pages/operator/finance/withdraw/index.wxml')
const serviceSource = read('miniprogram/pages/operator/_services/operator-finance.ts')

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

console.log('check-operator-finance-overview-display: operator finance overview renders preformatted values')
