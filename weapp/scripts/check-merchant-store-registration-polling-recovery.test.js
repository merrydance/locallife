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

function extractMethod(source, methodName) {
  const startPattern = new RegExp(`\\n  (?:async )?${methodName}\\(`, 'g')
  const match = startPattern.exec(source)
  const start = match ? match.index + 1 : -1
  assert(start >= 0, `Expected source to contain method ${methodName}`)

  const bodyStart = source.indexOf('{', start)
  assert(bodyStart > start, `Expected method ${methodName} to have a body`)

  let depth = 0
  for (let index = bodyStart; index < source.length; index += 1) {
    if (source[index] === '{') {
      depth += 1
    } else if (source[index] === '}') {
      depth -= 1
      if (depth === 0) {
        return source.slice(start, index + 1)
      }
    }
  }

  throw new Error(`Expected method ${methodName} body to close`)
}

const runtimeTs = read('weapp/miniprogram/pages/register/merchant/store/_utils/merchant-store-registration-runtime.ts')
const pageTs = read('weapp/miniprogram/pages/register/merchant/store/index.ts')

assertContains(
  runtimeTs,
  'completeMerchantOnboardingApproval()',
  'merchant store onboarding must use one terminal approval handler for direct-submit and polling outcomes'
)
assertContains(
  runtimeTs,
  'stopPollingStatus()',
  'merchant store onboarding must expose a polling cleanup owner'
)
assertContains(
  runtimeTs,
  'merchantApplicationPollingIntervalId',
  'merchant store onboarding must keep the polling interval id on the page context'
)
assertContains(
  runtimeTs,
  'merchantApplicationPollingSessionId',
  'merchant store onboarding must ignore stale polling results from previous page sessions'
)
assertContains(
  runtimeTs,
  'completeMerchantOnboardingApproval()',
  'approved status from polling must not leave the page on the spinner'
)
assertContains(
  pageTs,
  'merchantApplicationPollingIntervalId: null',
  'page data must initialize polling interval ownership'
)
assertContains(
  pageTs,
  'merchantApplicationPollingSessionId: 0',
  'page data must initialize polling session ownership'
)

const startPollingMethod = extractMethod(runtimeTs, 'startPollingStatus')
assertContains(
  startPollingMethod,
  'this.stopPollingStatus()',
  'starting a new merchant onboarding poll must clear any previous interval'
)
assertContains(
  startPollingMethod,
  'merchantApplicationPollingIntervalId',
  'startPollingStatus must persist interval id for cleanup on terminal status and page leave'
)
assertContains(
  startPollingMethod,
  'merchantApplicationPollingSessionId',
  'startPollingStatus must gate async polling results by session id'
)
assertContains(
  startPollingMethod,
  'this.completeMerchantOnboardingApproval()',
  'approved polling status must enter the same terminal approval handler used by direct submit'
)

const approvedPollingIndex = startPollingMethod.indexOf('pollingStatusView.isApproved')
const approvalHandlerIndex = startPollingMethod.indexOf('this.completeMerchantOnboardingApproval()', approvedPollingIndex)
const modalIndexAfterApproved = startPollingMethod.indexOf('wx.showModal', approvedPollingIndex)
assert(
  approvalHandlerIndex > approvedPollingIndex,
  'approved polling branch must call the terminal approval handler'
)
assert(
  modalIndexAfterApproved === -1 || approvalHandlerIndex < modalIndexAfterApproved,
  'approved polling branch must not depend on wx.showModal success before leaving the spinner'
)

const directSubmitMethod = extractMethod(runtimeTs, 'onSubmit')
assertContains(
  directSubmitMethod,
  'this.completeMerchantOnboardingApproval()',
  'direct approved submit result must use the shared terminal approval handler'
)

assertContains(
  runtimeTs,
  'onShow()',
  'merchant store registration page must resume polling when returning to a processing step'
)
assertContains(
  runtimeTs,
  'onHide()',
  'merchant store registration page must clean polling when hidden'
)
assertContains(
  runtimeTs,
  'onUnload()',
  'merchant store registration page must clean polling when unloaded'
)
