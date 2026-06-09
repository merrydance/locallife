const assert = require('assert')
const fs = require('fs')
const path = require('path')

const ROOT = path.resolve(__dirname, '..')
const REPO_ROOT = path.resolve(ROOT, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8')
}

function assertContains(source, expected, message) {
  assert(
    source.includes(expected),
    message || `Expected source to contain ${expected}`
  )
}

function extractSection(source, startMarker, endMarker) {
  const start = source.indexOf(startMarker)
  assert(start >= 0, `Expected source to contain ${startMarker}`)
  const end = source.indexOf(endMarker, start)
  assert(end > start, `Expected source to contain ${endMarker} after ${startMarker}`)
  return source.slice(start, end)
}

const dashboardTs = read('weapp/miniprogram/pages/merchant/dashboard/index.ts')
const dashboardWxml = read('weapp/miniprogram/pages/merchant/dashboard/index.wxml')
const dashboardWxss = read('weapp/miniprogram/pages/merchant/dashboard/index.wxss')
const dashboardJson = read('weapp/miniprogram/pages/merchant/dashboard/index.json')
const statusSwitchHandler = extractSection(dashboardTs, 'async onOpenStatusSwitchChange', '\n\n  onTapEntry')

assertContains(
  dashboardTs,
  "refreshErrorMessage: '页面同步失败，当前保留上次结果'",
  'dashboard refresh failure must preserve trusted data and set a visible stale-data message'
)
assertContains(
  dashboardTs,
  "refreshErrorMessage: partialFailure\n          ? (trustedDataAvailable ? '部分数据同步失败，当前保留上次结果' : '部分数据同步失败，未获取到的数据暂未显示')",
  'dashboard partial request failures must set a stale-data or missing-data message'
)
assertContains(
  statusSwitchHandler,
  "wx.showToast({\n        title: getErrorMessage(err, nextIsOpen ? '恢复营业失败，请稍后重试' : '打烊失败，请稍后重试')",
  'open-status PATCH failure must use a Chinese action-level error toast'
)
assertContains(
  statusSwitchHandler,
  'const response = await updateMerchantStorefrontOpenStatus(nextIsOpen)',
  'open-status switch must wait for the backend PATCH response before changing local truth'
)
assertContains(
  statusSwitchHandler,
  'isOpen: response.is_open',
  'open-status switch must only change durable local truth from the backend PATCH response'
)

const patchCallIndex = statusSwitchHandler.indexOf('const response = await updateMerchantStorefrontOpenStatus(nextIsOpen)')
const successStateIndex = statusSwitchHandler.indexOf('isOpen: response.is_open')
assert(patchCallIndex >= 0 && successStateIndex > patchCallIndex, 'isOpen must be updated only after the backend PATCH response is available')
assert(
  /}\s*finally\s*{\s*this\.setData\(\{ openStatusSubmitting: false \}\)\s*}\s*},/.test(statusSwitchHandler),
  'openStatusSubmitting must be cleared inside the finally block'
)
assert(
  !/setData\s*\(\s*\{[\s\S]*?isOpen\s*:/.test(statusSwitchHandler.slice(0, patchCallIndex)),
  'open-status switch must not optimistically set isOpen before backend PATCH resolves'
)
assert(
  !/isOpen\s*:\s*nextIsOpen/.test(statusSwitchHandler),
  'open-status switch must not persist the requested state as durable truth'
)

assertContains(
  dashboardWxml,
  'wx:if="{{refreshErrorMessage}}"',
  'dashboard must render refreshErrorMessage as an in-page visible failure state'
)
assertContains(
  dashboardWxml,
  '<t-notice-bar theme="warning" visible="{{true}}" content="{{refreshErrorMessage}}">',
  'dashboard must render refreshErrorMessage through a TDesign warning notice bar'
)
assertContains(
  dashboardWxml,
  'slot="operation" class="notice-banner-operation" bind:tap="onRetry"',
  'dashboard refresh failure notice must expose an in-page retry action'
)
assertContains(
  dashboardWxml,
  'loading="{{openStatusSubmitting}}"',
  'dashboard switch must expose duplicate-submit loading state while PATCH is in flight'
)
assertContains(
  dashboardWxml,
  'disabled="{{isPageSyncing}}"',
  'dashboard switch must stay disabled during page sync to avoid state collisions'
)

assertContains(
  dashboardWxss,
  '.notice-banner-operation',
  'dashboard retry operation must have a local style hook like other merchant notice banners'
)

assertContains(
  dashboardJson,
  '"t-notice-bar": "tdesign-miniprogram/notice-bar/notice-bar"',
  'dashboard must declare the TDesign notice bar component used by the failure-state WXML'
)

console.log('check-merchant-open-status-dashboard-failure-state: dashboard status failures are visible and status writes preserve backend truth')
