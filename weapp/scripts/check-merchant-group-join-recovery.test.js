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

const apiTs = read('weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts')
const adapterTs = read('weapp/miniprogram/pages/merchant/_main_shared/adapters/group-join-request.ts')
const pageTs = read('weapp/miniprogram/pages/merchant/group/join/index.ts')
const pageWxml = read('weapp/miniprogram/pages/merchant/group/join/index.wxml')
const pageJson = read('weapp/miniprogram/pages/merchant/group/join/index.json')
const registerMerchantTs = read('weapp/miniprogram/pages/register/merchant/index.ts')
const registerJoinTs = read('weapp/miniprogram/pages/register/merchant/join-group/index.ts')
const confirmApplySection = extractSection(pageTs, 'async confirmApply()', '\n  }\n})')
const refreshSection = extractSection(pageTs, 'async refreshJoinRequests', '\n\n  async onSearchSubmit')
const duplicateErrorSection = extractSection(pageTs, 'function getErrorCode', '\n\nPage({')

assertContains(
  apiTs,
  'export function listMyGroupJoinRequests()',
  'group join API wrapper must expose applicant-scoped merchant history'
)
assertContains(
  apiTs,
  "url: '/v1/merchants/me/group-join-requests'",
  'group join API wrapper must call the merchant-scoped backend truth endpoint'
)
assertContains(
  apiTs,
  'GROUP_JOIN_REQUEST_ALREADY_PENDING_CODE',
  'group join API wrapper must expose the backend duplicate-pending error code'
)
assertContains(
  adapterTs,
  'export function getGroupJoinRequestStatusDisplay',
  'group join status display must be owned by a shared adapter, not the page'
)
assertContains(
  adapterTs,
  'export function isGroupJoinRequestPending',
  'group join pending checks must be owned by a shared adapter, not the page'
)

assertContains(
  pageTs,
  'listMyGroupJoinRequests',
  'group join page must import the applicant-scoped history wrapper'
)
assertContains(
  pageTs,
  "from '../../_main_shared/adapters/group-join-request'",
  'group join page must consume shared status helpers instead of hardcoding backend status literals'
)
assertContains(
  pageTs,
  'GROUP_JOIN_REQUEST_ALREADY_PENDING_CODE',
  'group join page must branch duplicate-pending recovery on stable backend error code'
)
assert(
  !pageTs.includes('getErrorDebugMessage'),
  'group join page must not classify duplicate-pending recovery by backend error message text'
)
assertContains(
  pageTs,
  'pendingJoinRequest: null as GroupJoinRequestView | null',
  'group join page must model current pending request state for re-entry recovery'
)
assertContains(
  pageTs,
  'joinRequestsErrorMessage',
  'group join page must model history refresh failures as visible page state'
)
assertContains(
  pageTs,
  'applying: false',
  'group join page must keep a duplicate-submit guard in page state'
)

assertContains(
  refreshSection,
  'const requests = await listMyGroupJoinRequests()',
  'refreshJoinRequests must reload durable backend request history'
)
assertContains(
  refreshSection,
  'pendingJoinRequest: findLatestPendingJoinRequest(viewRequests)',
  'refreshJoinRequests must derive current pending state from backend truth'
)
assertContains(
  refreshSection,
  'groups: buildGroupItems(this.data.rawGroups, viewRequests, this.data.applyingGroupId)',
  'history refresh must reconcile existing search results with pending request state'
)

assertContains(
  pageTs,
  'await this.refreshJoinRequests({ initial: true })',
  'onLoad must hydrate join request status after merchant access succeeds'
)
assertContains(
  pageTs,
  'await this.refreshJoinRequests({ silent: true })',
  'onShow or submit recovery must silently refresh durable join request status'
)

assertContains(
  confirmApplySection,
  'if (this.data.applying) return',
  'confirmApply must ignore duplicate taps while submit is in flight'
)
assertContains(
  confirmApplySection,
  'applying: true',
  'confirmApply must set submitting state before the POST'
)
assertContains(
  confirmApplySection,
  'const created = await applyToJoinGroup',
  'confirmApply must await the backend join request creation'
)
assertContains(
  confirmApplySection,
  'await this.refreshJoinRequests({ silent: true })',
  'confirmApply must re-read durable request status after success or duplicate conflict'
)
assertContains(
  confirmApplySection,
  'isDuplicateJoinRequestError(e)',
  'confirmApply must treat backend duplicate-pending conflicts as state recovery, not a generic failure'
)
assertContains(
  duplicateErrorSection,
  'const code = typeof knownError.code',
  'duplicate-pending recovery must inspect backend API error code'
)
assertContains(
  duplicateErrorSection,
  'return getErrorCode(error) === GROUP_JOIN_REQUEST_ALREADY_PENDING_CODE',
  'duplicate-pending recovery must compare the exact backend API error code'
)
assertContains(
  confirmApplySection,
  'finally',
  'confirmApply must clear submitting state in a finally block'
)
assertContains(
  confirmApplySection,
  'applying: false',
  'confirmApply must clear submitting state after any outcome'
)

assertContains(
  pageWxml,
  'wx:if="{{pendingJoinRequest}}"',
  'group join page must render current pending join request status on re-entry'
)
assertContains(
  pageWxml,
  'wx:if="{{joinRequestsErrorMessage}}"',
  'group join page must render request-history refresh failure visibly'
)
assertContains(
  pageWxml,
  'loading="{{item.applying}}"',
  'group list apply button must expose per-row submitting state'
)
assertContains(
  pageWxml,
  'disabled="{{item.actionDisabled || applying}}"',
  'group list apply button must disable duplicate submissions while submitting or already pending'
)
assertContains(
  pageJson,
  '"t-tag": "tdesign-miniprogram/tag/tag"',
  'group join page must declare TDesign tag for durable request status display'
)

assertContains(
  registerMerchantTs,
  "wx.navigateTo({ url: '/pages/merchant/group/join/index' })",
  'register merchant entry must route to the single merchant group-join page owner'
)
assertContains(
  registerJoinTs,
  "wx.redirectTo({ url: '/pages/merchant/group/join/index' })",
  'legacy register group-join page must redirect to the single merchant group-join page owner'
)
assert(
  !registerJoinTs.includes('applyToJoinGroup') && !registerJoinTs.includes('searchGroups'),
  'legacy register group-join page must not keep a second submit/search implementation'
)

console.log('check-merchant-group-join-recovery: group join page recovers duplicate submit and re-entry state from backend truth')
