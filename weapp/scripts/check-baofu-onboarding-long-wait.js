const assert = require('assert')
const fs = require('fs')
const path = require('path')

const ROOT = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(ROOT, relativePath), 'utf8')
}

const onboardingService = read('miniprogram/services/baofu-account-onboarding.ts')
const statusBehavior = read('miniprogram/behaviors/baofu-settlement-status.ts')
const submitBehavior = read('miniprogram/behaviors/baofu-settlement-submit.ts')
const waitComponentTs = read('miniprogram/components/baofu-onboarding-wait/index.ts')
const waitComponentWxml = read('miniprogram/components/baofu-onboarding-wait/index.wxml')

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
  onboardingService.includes('已等待'),
  'Baofoo onboarding wait progress must expose elapsed seconds copy'
)

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
  'miniprogram/pages/platform/finance/settlement-account/submit/index.wxml',
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
