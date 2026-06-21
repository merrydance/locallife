const fs = require('fs')
const path = require('path')
const assert = require('assert')

const repoRoot = path.resolve(__dirname, '..')
const read = (file) => fs.readFileSync(path.join(repoRoot, file), 'utf8')

const apiSource = read('miniprogram/pages/operator/_api/operator-basic-management.ts')
const serviceSource = read('miniprogram/pages/operator/_services/operator-safety.ts')
const reportPageSource = read('miniprogram/pages/operator/safety/report/index.ts')
const reportWxmlSource = read('miniprogram/pages/operator/safety/report/index.wxml')
const detailPageSource = read('miniprogram/pages/operator/safety/detail/index.ts')
const detailWxmlSource = read('miniprogram/pages/operator/safety/detail/index.wxml')

function extractMethod(source, methodName) {
  const start = source.indexOf(`async ${methodName}(`)
  assert(start >= 0, `missing ${methodName} method`)
  const nextMethod = source.indexOf('\n    async ', start + 1)
  return source.slice(start, nextMethod > start ? nextMethod : source.length)
}

const getFoodSafetyCasesMethod = extractMethod(apiSource, 'getFoodSafetyCases')

assert(
  /async getFoodSafetyCases\(params\?: \{\s*page\?: number\s*limit\?: number\s*status\?: OperatorFoodSafetyCaseStatus\s*\}/.test(apiSource),
  'operator food safety list API must expose only backend-supported page, limit, and status query fields'
)
assert(
  !getFoodSafetyCasesMethod.includes('region_id'),
  'operator food safety list must not pass region_id by default; backend owns default all-active-region aggregation'
)
assert(
  serviceSource.includes('status: params.status || undefined'),
  'operator food safety service must forward status filter without inventing unsupported local filters'
)
assert(
  !/loadOperatorFoodSafetyCaseListPageData[\s\S]*?catch\s*\(/.test(serviceSource),
  'operator food safety service must not swallow backend errors and turn them into an empty list'
)
assert(
  reportWxmlSource.includes('wx:if="{{error && !initialLoading}}"') &&
    reportWxmlSource.includes('description="{{error}}"'),
  'operator food safety report page must render backend/list failures as a visible error state, not as empty data'
)
assert(
  reportWxmlSource.includes('wx:elif="{{cases.length > 0}}"') &&
    reportWxmlSource.includes('description="暂无食安案件"'),
  'operator food safety report page must keep success-list and success-empty states distinct'
)
assert(
  reportPageSource.includes('this.setData({ status: e.detail.value })') &&
    reportPageSource.includes('this.loadCases(true)'),
  'operator food safety status tab changes must reset paging and reload from backend truth'
)
assert(
  detailPageSource.includes('await saveOperatorFoodSafetyInvestigation') &&
    detailPageSource.includes('await this.loadDetail()'),
  'operator food safety investigation save must re-read detail after backend mutation'
)
assert(
  detailPageSource.includes('await saveOperatorFoodSafetyResolution') &&
    detailPageSource.includes('await this.loadDetail()'),
  'operator food safety resolution submit must re-read detail after backend mutation'
)
assert(
  detailWxmlSource.includes('wx:if="{{caseDetail.is_active}}"') &&
    detailWxmlSource.includes('wx:else') &&
    detailWxmlSource.includes('结案结果'),
  'operator food safety detail page must separate active action state from resolved read-only state'
)

console.log('check-operator-food-safety-contract: operator safety pages preserve backend aggregation, error, and action recovery contracts')
