const assert = require('assert')
const fs = require('fs')
const path = require('path')

const rootDir = path.join(__dirname, '..')
const apiPath = path.join(rootDir, 'miniprogram/api/table-device-management.ts')
const configPageTsPath = path.join(rootDir, 'miniprogram/pages/merchant/config/index.ts')
const pageTsPath = path.join(rootDir, 'miniprogram/pages/merchant/settings/display-config/index.ts')
const pageWxmlPath = path.join(rootDir, 'miniprogram/pages/merchant/settings/display-config/index.wxml')

const apiSource = fs.readFileSync(apiPath, 'utf8')
const configPageTsSource = fs.readFileSync(configPageTsPath, 'utf8')
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
assert(
  !pageWxmlSource.includes('语音提醒') &&
    !pageWxmlSource.includes('语音播报') &&
    !pageWxmlSource.includes('settingsForm.enable_voice') &&
    !pageWxmlSource.includes('settingsForm.voice_takeout') &&
    !pageWxmlSource.includes('settingsForm.voice_dine_in') &&
    !pageWxmlSource.includes('data-field="enable_voice"') &&
    !pageWxmlSource.includes('data-field="voice_takeout"') &&
    !pageWxmlSource.includes('data-field="voice_dine_in"'),
  'display config page should not expose deprecated Mini Program voice controls'
)
assert(
  !/interface DisplayConfigForm[\s\S]*enable_voice:\s*boolean/.test(pageTsSource) &&
    !/interface DisplayConfigForm[\s\S]*voice_takeout:\s*boolean/.test(pageTsSource) &&
    !/interface DisplayConfigForm[\s\S]*voice_dine_in:\s*boolean/.test(pageTsSource),
  'display config form should not track deprecated voice fields'
)
assert(
  !/enable_voice:\s*this\.data\.settingsForm\.enable_voice/.test(pageTsSource) &&
    !/voice_takeout:\s*this\.data\.settingsForm\.voice_takeout/.test(pageTsSource) &&
    !/voice_dine_in:\s*this\.data\.settingsForm\.voice_dine_in/.test(pageTsSource),
  'display config save payload should not submit deprecated voice fields'
)
assert(
  !configPageTsSource.includes('语音播报配置'),
  'merchant config entry should not advertise deprecated voice broadcast configuration'
)

console.log('check-merchant-display-config-auto-accept: display config wires auto accept and hides deprecated voice controls')
