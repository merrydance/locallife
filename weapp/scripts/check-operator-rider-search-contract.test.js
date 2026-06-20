const fs = require('fs')
const path = require('path')
const assert = require('assert')

const repoRoot = path.resolve(__dirname, '..')
const read = (file) => fs.readFileSync(path.join(repoRoot, file), 'utf8')

const apiSource = read('miniprogram/pages/operator/_api/operator-rider-management.ts')
const serviceSource = read('miniprogram/pages/operator/_services/operator-rider-management.ts')
const pageSource = read('miniprogram/pages/operator/riders/index.ts')

assert(
  apiSource.includes("export type RiderOnlineStatus = 'online' | 'offline'"),
  'operator rider API type must expose only backend-supported online_status values'
)
assert(
  serviceSource.includes('const keyword = params.searchKeyword?.trim() || undefined'),
  'operator rider list service must trim keyword before sending it to the backend'
)
assert(
  serviceSource.includes('keyword,'),
  'operator rider list service must send the trimmed keyword in RiderQueryParams'
)
assert(
  !serviceSource.includes("sort_by: 'created_at'") && !serviceSource.includes("sort_order: 'desc'"),
  'operator rider list service must not send sort fields that the backend rider list contract does not support'
)
assert(
  pageSource.includes('let riderListRequestSeq = 0'),
  'operator rider list page must own a request sequence guard'
)
assert(
  pageSource.includes('const requestSeq = ++riderListRequestSeq'),
  'operator rider list page must increment request sequence for each load'
)
assert(
  pageSource.includes('requestSeq !== riderListRequestSeq'),
  'operator rider list page must ignore stale search/list responses'
)

console.log('check-operator-rider-search-contract: rider search request contract is guarded')
