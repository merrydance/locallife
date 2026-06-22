const fs = require('fs')
const path = require('path')
const assert = require('assert')

const repoRoot = path.resolve(__dirname, '..')
const read = (file) => fs.readFileSync(path.join(repoRoot, file), 'utf8')

const pageSource = read('miniprogram/pages/operator/safety/report/index.ts')

assert(
  pageSource.includes('let foodSafetyCaseListRequestSeq = 0'),
  'operator food safety case list must own a request sequence guard'
)
assert(
  pageSource.includes('const requestSeq = ++foodSafetyCaseListRequestSeq'),
  'operator food safety case list must increment request sequence for each list request'
)
assert(
  /if\s*\(\s*!reset\s*&&\s*\(\s*this\.data\.loading\s*\|\|\s*this\.data\.loadingMore\s*\)\s*\)\s*return/.test(pageSource),
  'operator food safety case list must allow reset requests while older refreshes are in flight'
)
assert(
  (pageSource.match(/requestSeq !== foodSafetyCaseListRequestSeq/g) || []).length >= 2,
  'operator food safety case list must ignore stale success and stale failure responses'
)
assert(
  /if\s*\(\s*requestSeq !== foodSafetyCaseListRequestSeq\s*\)\s*\{\s*return\s*\}/.test(pageSource),
  'operator food safety case list stale responses must return before setData'
)
assert(
  pageSource.includes('const status = this.data.status') &&
    pageSource.includes('status'),
  'operator food safety case list must snapshot the active status for the request'
)

console.log('check-operator-food-safety-list-sequence: operator food safety case list ignores stale list responses')
