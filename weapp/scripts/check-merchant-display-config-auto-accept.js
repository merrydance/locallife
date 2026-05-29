const assert = require('assert')
const fs = require('fs')
const path = require('path')

const rootDir = path.join(__dirname, '..')
const apiPath = path.join(rootDir, 'miniprogram/api/table-device-management.ts')
const pageTsPath = path.join(rootDir, 'miniprogram/pages/merchant/settings/display-config/index.ts')
const pageWxmlPath = path.join(rootDir, 'miniprogram/pages/merchant/settings/display-config/index.wxml')

const apiSource = fs.readFileSync(apiPath, 'utf8')
const pageTsSource = fs.readFileSync(pageTsPath, 'utf8')
const pageWxmlSource = fs.readFileSync(pageWxmlPath, 'utf8')

assert(
  /export interface DisplayConfigResponse[\s\S]*auto_accept_paid_orders:\s*boolean/.test(apiSource),
  'display config response should expose auto_accept_paid_orders from backend contract'
)
assert(
  /export interface UpdateDisplayConfigRequest[\s\S]*auto_accept_paid_orders\?:\s*boolean/.test(apiSource),
  'display config update request should allow auto_accept_paid_orders'
)
assert(
  /interface DisplayConfigForm[\s\S]*auto_accept_paid_orders:\s*boolean/.test(pageTsSource),
  'display config form should track auto_accept_paid_orders'
)
assert(
  /auto_accept_paid_orders:\s*Boolean\(config\?\.auto_accept_paid_orders\)/.test(pageTsSource),
  'display config form builder should read auto_accept_paid_orders from backend response'
)
assert(
  /auto_accept_paid_orders:\s*this\.data\.settingsForm\.auto_accept_paid_orders/.test(pageTsSource),
  'display config save payload should submit auto_accept_paid_orders'
)
assert(
  pageWxmlSource.includes('title="自动接单"') &&
    pageWxmlSource.includes('value="{{settingsForm.auto_accept_paid_orders}}"') &&
    pageWxmlSource.includes('data-field="auto_accept_paid_orders"'),
  'display config page should render an auto accept switch bound to auto_accept_paid_orders'
)

console.log('check-merchant-display-config-auto-accept: display config wires auto accept switch')
