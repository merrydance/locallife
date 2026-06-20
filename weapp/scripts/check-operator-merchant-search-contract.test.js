const fs = require('fs')
const path = require('path')
const assert = require('assert')

const repoRoot = path.resolve(__dirname, '..')
const read = (file) => fs.readFileSync(path.join(repoRoot, file), 'utf8')

const serviceSource = read('miniprogram/pages/operator/_services/operator-merchant-management.ts')
const pageSource = read('miniprogram/pages/operator/merchants/index.ts')

assert(
  serviceSource.includes('const keyword = params.searchKeyword?.trim() || undefined'),
  'operator merchant list service must trim keyword before sending it to the backend'
)
assert(
  serviceSource.includes('keyword,'),
  'operator merchant list service must send the trimmed keyword in MerchantQueryParams'
)
assert(
  pageSource.includes('let merchantListRequestSeq = 0'),
  'operator merchant list page must own a request sequence guard'
)
assert(
  pageSource.includes('const requestSeq = ++merchantListRequestSeq'),
  'operator merchant list page must increment request sequence for each load'
)
assert(
  pageSource.includes('requestSeq !== merchantListRequestSeq'),
  'operator merchant list page must ignore stale search/list responses'
)

console.log('check-operator-merchant-search-contract: merchant search request contract is guarded')
