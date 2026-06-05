const assert = require('assert')
const fs = require('fs')
const path = require('path')

const rootDir = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(rootDir, relativePath), 'utf8')
}

const apiSource = read('miniprogram/pages/operator/_api/operator-merchant-management.ts')
const serviceSource = read('miniprogram/pages/operator/_services/operator-merchant-management.ts')
const pageSource = read('miniprogram/pages/operator/merchants/detail/index.ts')
const wxmlSource = read('miniprogram/pages/operator/merchants/detail/index.wxml')

assert(
  /getMerchantCapabilities\s*\(/.test(apiSource),
  'operator merchant API should expose getMerchantCapabilities()'
)
assert(
  /updateMerchantCapabilities\s*\(/.test(apiSource),
  'operator merchant API should expose updateMerchantCapabilities()'
)
assert(
  /\/v1\/operator\/merchants\/\$\{merchantId\}\/capabilities/.test(apiSource),
  'operator merchant API should call /v1/operator/merchants/:id/capabilities'
)
assert(
  /open_kitchen_status/.test(apiSource) && /dine_in_status/.test(apiSource),
  'operator merchant API should type backend capability fields'
)

assert(
  /loadOperatorMerchantCapabilitiesView/.test(serviceSource),
  'operator merchant service should map capability response into a page view model'
)
assert(
  /submitOperatorMerchantCapabilities/.test(serviceSource),
  'operator merchant service should own capability submission mapping'
)
assert(
  /system_labels/.test(serviceSource) && /system_label_text/.test(serviceSource),
  'operator merchant service should preserve backend system labels for display'
)

assert(
  /onOpenCapabilityEditor/.test(pageSource) &&
    /onSubmitCapabilityEditor/.test(pageSource) &&
    /loadCapabilities/.test(pageSource),
  'operator merchant detail page should wire capability load and edit handlers'
)
assert(
  /!this\.data\.capabilities/.test(pageSource) &&
    /capabilityError/.test(pageSource),
  'operator merchant detail page should not open capability editor before capabilities load successfully'
)
assert(
  /note:\s*form\.note\.trim\(\)/.test(serviceSource) &&
    !/note:\s*form\.note\.trim\(\)\s*\|\|\s*undefined/.test(serviceSource),
  'operator merchant capability submission should send an empty note when the user clears it'
)
assert(
  /经营能力/.test(wxmlSource) &&
    /堂食服务/.test(wxmlSource) &&
    /系统展示标签/.test(wxmlSource) &&
    /调整能力/.test(wxmlSource),
  'operator merchant detail page should render capability status and edit entry'
)
assert(
  /disabled="\{\{capabilityLoading \|\| capabilityError \|\| !capabilities\}\}"/.test(wxmlSource),
  'operator merchant detail page should disable capability editing when capabilities are unavailable'
)

console.log('check-operator-merchant-capabilities: operator merchant capability entry is wired')
