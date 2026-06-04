const assert = require('assert')
const fs = require('fs')
const path = require('path')

const ROOT = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(ROOT, relativePath), 'utf8')
}

function assertInOrder(source, tokens, message) {
  let cursor = -1
  for (const token of tokens) {
    const next = source.indexOf(token, cursor + 1)
    assert(next > cursor, message)
    cursor = next
  }
}

const onboardingService = read('miniprogram/pages/merchant/_main_shared/services/baofu-account-onboarding.ts')
const statusHelpers = read('miniprogram/pages/merchant/_main_shared/api/baofu-account-status.ts')
const statusBehavior = read('miniprogram/pages/merchant/_main_shared/behaviors/baofu-settlement-status.ts')
const submitBehavior = read('miniprogram/pages/merchant/_main_shared/behaviors/baofu-settlement-submit.ts')
const submitBehaviorPaths = [
  'miniprogram/pages/merchant/_main_shared/behaviors/baofu-settlement-submit.ts',
  'miniprogram/pages/operator/_main_shared/behaviors/baofu-settlement-submit.ts',
  'miniprogram/pages/rider/_main_shared/behaviors/baofu-settlement-submit.ts'
]
const submitPagePaths = [
  'miniprogram/pages/merchant/finance/settlement-account/submit/index.ts',
  'miniprogram/pages/operator/finance/settlement-account/submit/index.ts',
  'miniprogram/pages/rider/settlement-account/submit/index.ts'
]
const waitPatchConsumers = [
  'miniprogram/pages/merchant/_main_shared/behaviors/baofu-settlement-status.ts',
  'miniprogram/pages/merchant/_main_shared/behaviors/baofu-settlement-submit.ts',
  'miniprogram/pages/merchant/finance/settlement-account/submit/index.ts',
  'miniprogram/pages/operator/_main_shared/behaviors/baofu-settlement-status.ts',
  'miniprogram/pages/operator/_main_shared/behaviors/baofu-settlement-submit.ts',
  'miniprogram/pages/operator/finance/settlement-account/submit/index.ts',
  'miniprogram/pages/platform/_main_shared/behaviors/baofu-settlement-status.ts',
  'miniprogram/pages/rider/_main_shared/behaviors/baofu-settlement-status.ts',
  'miniprogram/pages/rider/_main_shared/behaviors/baofu-settlement-submit.ts',
  'miniprogram/pages/rider/settlement-account/submit/index.ts'
]
const waitComponentTs = read('miniprogram/pages/merchant/_components/baofu-onboarding-wait/index.ts')
const waitComponentWxml = read('miniprogram/pages/merchant/_components/baofu-onboarding-wait/index.wxml')

assert(
  onboardingService.includes('isBaofuSettlementOpeningProcessingStatus(normalizedAccountStatus)'),
  'Baofoo onboarding without payment params must keep polling opening/report/auth processing states'
)
assert(
  onboardingService.includes('delayWithPollProgress'),
  'Baofoo onboarding polling must update the waiting countdown between backend queries'
)
assert(
  onboardingService.includes('BAOFU_STATUS_POLL_UNTIL_TERMINAL') &&
    onboardingService.includes('maxAttempts === BAOFU_STATUS_POLL_UNTIL_TERMINAL || attempt < maxAttempts'),
  'Baofoo onboarding must keep polling while the page is active until backend terminal state is returned'
)
assert(
  statusHelpers.includes('return isBaofuSettlementTerminalStatus(status)'),
  'Baofoo submit/payment polling must keep waiting for the final backend terminal state'
)
assert(
  onboardingService.includes('account?: BaofuSettlementAccountResponse') &&
    onboardingService.includes('buildBaofuOnboardingWaitViewFromAccount') &&
    onboardingService.includes('emitPollProgress(options, attempt, maxAttempts, interval, attempt * interval, account)'),
  'Baofoo onboarding polling must publish each backend status snapshot to the wait UI'
)
assert(
  submitBehavior.includes('buildBaofuOnboardingWaitViewFromAccount') &&
    statusBehavior.includes('buildBaofuOnboardingWaitViewFromAccount'),
  'Baofoo wait UI consumers must update title and description when polling observes a new backend status'
)
assert(
  onboardingService.includes('已等待'),
  'Baofoo onboarding wait progress must expose elapsed seconds copy'
)

for (const relativePath of submitBehaviorPaths) {
  const source = read(relativePath)
  for (const token of [
    '_startBaofuSubmitPendingTick',
    '_stopBaofuSubmitPendingTick',
    'setInterval(tick, 1000)',
    'clearInterval(timer)',
    'waitElapsedSeconds',
    'waitTimerVisible'
  ]) {
    assert(
      source.includes(token),
      `${relativePath} must maintain local submit-pending countdown while POST has not returned`
    )
  }
  assertInOrder(
    source,
    ['_beginBaofuLongWaitSession(): number {', 'this._stopBaofuSubmitPendingTick()', 'const nextSessionId'],
    `${relativePath} must stop any previous submit-pending countdown before starting a new wait session`
  )
  assertInOrder(
    source,
    ['_cancelBaofuLongWaitSession() {', 'this._stopBaofuSubmitPendingTick()', 'this.data._waitSessionId'],
    `${relativePath} must stop the submit-pending countdown when the wait session is cancelled`
  )
  assertInOrder(
    source,
    ['_handleBaofuOnboardingProgress', 'this._stopBaofuSubmitPendingTick()', 'waitProgressText'],
    `${relativePath} must stop local submit-pending countdown before backend polling progress owns the timer`
  )
  assert(
    (source.match(/this\.setData\(\{ syncing: false, submitting: false \}\)/g) || []).length >= 2,
    `${relativePath} must clear submitting state on hide and unload so submit-pending countdown cannot leak`
  )
}

for (const relativePath of submitPagePaths) {
  const source = read(relativePath)
  assert(
    source.includes('const waitSessionId = (this as any)._beginBaofuLongWaitSession()') &&
      source.includes('(this as any)._startBaofuSubmitPendingTick(waitSessionId)'),
    `${relativePath} must start the submit-pending countdown immediately after opening the wait session`
  )
  assertInOrder(
    source,
    [
      'const waitSessionId = (this as any)._beginBaofuLongWaitSession()',
      '(this as any)._startBaofuSubmitPendingTick(waitSessionId)',
      'await startBaofuAccountOnboarding'
    ],
    `${relativePath} must start the submit-pending countdown before awaiting the submit request`
  )
}

for (const [label, source] of [
  ['status behavior', statusBehavior],
  ['submit behavior', submitBehavior]
]) {
  assert(
    !source.includes('maxAttempts: 1'),
    `${label} must not degrade Baofoo long waiting into a single manual refresh query`
  )
  assert(
    source.includes('waitElapsedSeconds') && source.includes('waitUntilTerminal') && source.includes('waitTimerVisible'),
    `${label} must pass countdown state to the wait UI`
  )
}

for (const relativePath of waitPatchConsumers) {
  const source = read(relativePath)
  assert(
    !source.includes('...buildBaofuOnboardingWaitViewFromText({'),
    `${relativePath} must map Baofoo wait views into wait* page data fields before setData`
  )
}

assert(
  statusBehavior.includes('_startLongWaitForProcessing') && statusBehavior.includes('pageView.statusView.isWaiting'),
  'Baofoo status pages must automatically enter long waiting when backend state is still processing'
)

assert(
  waitComponentTs.includes('elapsedSeconds') &&
    waitComponentTs.includes('waitingUntilTerminal') &&
    waitComponentTs.includes('timerVisible'),
  'Baofoo wait component must accept countdown properties'
)
assert(
  waitComponentWxml.includes('onboarding-wait__timer') &&
    waitComponentWxml.includes('{{elapsedSeconds}}') &&
    waitComponentWxml.includes('持续确认'),
  'Baofoo wait component must render a visible countdown timer'
)

const waitConsumers = [
  'miniprogram/pages/merchant/finance/settlement-account/index.wxml',
  'miniprogram/pages/operator/finance/settlement-account/index.wxml',
  'miniprogram/pages/platform/finance/settlement-account/index.wxml',
  'miniprogram/pages/rider/settlement-account/index.wxml',
  'miniprogram/pages/merchant/finance/settlement-account/submit/index.wxml',
  'miniprogram/pages/operator/finance/settlement-account/submit/index.wxml',
  'miniprogram/pages/rider/settlement-account/submit/index.wxml'
]

for (const relativePath of waitConsumers) {
  const source = read(relativePath)
  assert(
    source.includes('elapsedSeconds="{{waitElapsedSeconds}}"') &&
      source.includes('waitingUntilTerminal="{{waitUntilTerminal}}"') &&
      source.includes('timerVisible="{{waitTimerVisible}}"'),
    `${relativePath} must wire countdown state into baofu-onboarding-wait`
  )
}

console.log('check-baofu-onboarding-long-wait: validated Baofoo onboarding long-wait UI contract')
