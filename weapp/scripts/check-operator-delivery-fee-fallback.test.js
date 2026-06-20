const assert = require('assert')
const fs = require('fs')
const path = require('path')

const rootDir = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(rootDir, relativePath), 'utf8')
}

const apiSource = read('miniprogram/pages/operator/_main_shared/api/delivery-fee.ts')
const serviceSource = read('miniprogram/pages/operator/_services/operator-region-config.ts')
const pageSource = read('miniprogram/pages/operator/delivery-fee/index.ts')
const wxmlSource = read('miniprogram/pages/operator/delivery-fee/index.wxml')

assert(
  /function\s+isDeliveryFeeConfigNotFoundError\s*\(\s*error:\s*unknown\s*\)/.test(apiSource),
  'delivery fee API should centralize the config-not-found fallback predicate'
)
assert(
  /statusCode\s*===\s*404/.test(apiSource),
  'delivery fee fallback predicate should key off HTTP statusCode === 404'
)
assert(
  /catch\s*\(\s*error:\s*unknown\s*\)/.test(apiSource),
  'updateRegionConfig() should inspect the caught PATCH error'
)
assert(
  /if\s*\(\s*isDeliveryFeeConfigNotFoundError\s*\(\s*error\s*\)\s*\)/.test(apiSource),
  'updateRegionConfig() should POST only after a config-not-found PATCH error'
)
assert(
  !/catch\s*\(\s*_e\s*\)\s*\{[\s\S]*?method:\s*'POST'/.test(apiSource),
  'updateRegionConfig() must not catch every PATCH error and immediately POST'
)

assert(
  /Promise<OperatorDeliveryFeeConfigView>/.test(serviceSource),
  'saveOperatorDeliveryFeeConfig() should return the backend-confirmed view model'
)
assert(
  /adaptDeliveryFeeConfigToView/.test(serviceSource),
  'operator region config service should adapt the saved backend response back into the form view'
)

assert(
  /saving:\s*false/.test(pageSource),
  'delivery fee page should track saving state'
)
assert(
  /if\s*\(\s*this\.data\.saving\s*\)/.test(pageSource),
  'delivery fee page should ignore duplicate save taps while a save is in flight'
)
assert(
  /saving:\s*true/.test(pageSource) && /saving:\s*false/.test(pageSource),
  'delivery fee page should set and clear saving state around submit'
)
assert(
  /disabled="\{\{saving\}\}"/.test(wxmlSource) && /loading="\{\{saving\}\}"/.test(wxmlSource),
  'save button should be disabled and show loading while saving'
)

console.log('check-operator-delivery-fee-fallback: delivery fee save fallback is constrained')
