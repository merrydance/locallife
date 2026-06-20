const fs = require('fs')
const path = require('path')

const repoRoot = path.resolve(__dirname, '..')
const apiFile = path.join(repoRoot, 'miniprogram/pages/operator/_api/operator-basic-management.ts')
const source = fs.readFileSync(apiFile, 'utf8')

function assert(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}

assert(
  /export type RegionStatus = 'active' \| 'suspended'/.test(source),
  'RegionStatus must match backend operator_regions.status values: active | suspended'
)

assert(
  /export type RegionDisplayStatus = RegionStatus \| 'unknown'/.test(source),
  'Region display state must include a frontend-only unknown state for missing or unrecognized backend status'
)

assert(
  !/data\.status\s*\?\?\s*'pending'/.test(source),
  'Missing region status must not be defaulted to pending'
)

assert(
  !/status\s*\|\|\s*'pending'/.test(source),
  'Region status display must not default missing status to pending'
)

assert(
  !/RegionStatus[^=]*=.*'inactive'/.test(source),
  'RegionStatus must not include inactive; operator region suspension is represented by suspended'
)

assert(
  /suspended:\s*'已暂停'/.test(source),
  'Suspended operator regions must be labeled 已暂停'
)

assert(
  /unknown:\s*'状态未知'/.test(source),
  'Missing or unknown operator region status must be labeled 状态未知'
)
