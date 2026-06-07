const fs = require('fs')
const path = require('path')

const root = path.resolve(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(root, relativePath), 'utf8')
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}

const rolePages = [
  'miniprogram/pages/merchant/finance/settlement-account/index',
  'miniprogram/pages/operator/finance/settlement-account/index',
  'miniprogram/pages/platform/finance/settlement-account/index',
  'miniprogram/pages/rider/settlement-account/index'
]

const roleComponents = [
  'miniprogram/pages/merchant/_components/baofu-account-status-summary/index',
  'miniprogram/pages/operator/_components/baofu-account-status-summary/index',
  'miniprogram/pages/platform/_components/baofu-account-status-summary/index',
  'miniprogram/pages/rider/_components/baofu-account-status-summary/index'
]

const roleViewModels = [
  'miniprogram/pages/merchant/_main_shared/api/baofu-account-view.ts',
  'miniprogram/pages/operator/_main_shared/api/baofu-account-view.ts',
  'miniprogram/pages/platform/_main_shared/api/baofu-account-view.ts',
  'miniprogram/pages/rider/_main_shared/api/baofu-account-view.ts'
]

const operatorContactUtil = read('miniprogram/utils/operator-contact.ts')
assert(operatorContactUtil.includes('checkRegionAvailability'), 'operator contact utility must reuse the region availability API')
assert(operatorContactUtil.includes('operator_contact_phone'), 'operator contact utility must read operator_contact_phone from backend truth')

const merchantRegisterIndex = read('miniprogram/pages/register/merchant/index.ts')
const merchantRegisterWxml = read('miniprogram/pages/register/merchant/index.wxml')
assert(merchantRegisterIndex.includes('getLocalOperatorContactPhone'), 'merchant registration page must reuse the shared operator contact utility')
assert(!merchantRegisterIndex.includes("from '../../../api/location'"), 'merchant registration page must not call checkRegionAvailability directly')
assert(
  merchantRegisterWxml.includes('open-type="contact"') && merchantRegisterWxml.includes('联系平台客服'),
  'merchant registration page must expose a WeChat customer-service button near the operator contact entry'
)
assert(
  merchantRegisterWxml.includes('<t-button theme="primary" block open-type="contact">'),
  'merchant registration customer-service button must use the primary solid button style'
)

const merchantStatusWxml = read('miniprogram/pages/merchant/finance/settlement-account/index.wxml')
const merchantStatusJson = read('miniprogram/pages/merchant/finance/settlement-account/index.json')
assert(
  merchantStatusJson.includes('baofu-intent-confirmation-hint'),
  'merchant Baofoo status page must declare the intent confirmation hint component'
)
assert(
  merchantStatusWxml.includes('wx:if="{{pageView.statusView.isReady}}"') &&
    merchantStatusWxml.includes('<baofu-intent-confirmation-hint'),
  'merchant Baofoo ready terminal state must render the WeChat Pay intent confirmation hint'
)

const intentHintTs = read('miniprogram/pages/merchant/_components/baofu-intent-confirmation-hint/index.ts')
const intentHintWxml = read('miniprogram/pages/merchant/_components/baofu-intent-confirmation-hint/index.wxml')
assert(
  intentHintWxml.includes('请联系客服完成微信支付商户开户意愿确认。这是最后一步。'),
  'intent confirmation hint must show the required static next-step copy'
)
assert(intentHintWxml.includes('运营商电话'), 'intent confirmation hint must show the operator phone label')
assert(intentHintWxml.includes('open-type="contact"'), 'intent confirmation hint must expose a WeChat customer-service contact button')
assert(intentHintTs.includes('getLocalOperatorContactPhone'), 'intent confirmation hint must reuse the shared operator contact utility')
assert(intentHintTs.includes('wx.makePhoneCall'), 'intent confirmation hint must provide a call action when an operator phone exists')

for (const pagePath of rolePages) {
  const wxml = read(`${pagePath}.wxml`)
  const json = read(`${pagePath}.json`)

  assert(!wxml.includes('<t-result'), `${pagePath}.wxml must not make the result icon the status page hero`)
  assert(
    wxml.includes('feedbackTitle="{{pageView.statusView.statusFeedbackTitle}}"'),
    `${pagePath}.wxml must pass shared status feedback view fields into the status summary view`
  )
  assert(!wxml.includes('status="{{pageView.statusView.normalizedStatus}}"'), `${pagePath}.wxml must not pass raw normalized status into the component`)
  assert(!json.includes('"t-result"'), `${pagePath}.json must not declare unused t-result after the status page relayout`)
}

for (const componentPath of roleComponents) {
  const wxml = read(`${componentPath}.wxml`)

  assert(wxml.includes('{{feedbackTitle}}'), `${componentPath}.wxml must render the status-specific primary title`)
  assert(wxml.includes('{{reasonTitle}}'), `${componentPath}.wxml must render the status-specific reason label`)
  assert(!wxml.includes('当前进度'), `${componentPath}.wxml must not render a generic progress banner`)
  assert(!wxml.includes('summary-fact__title">开户资料'), `${componentPath}.wxml must not turn the status page into an onboarding checklist`)
  assert(!wxml.includes('{{profileHint}}'), `${componentPath}.wxml must not repeat profile onboarding hints on the status page`)
}

for (const viewModelPath of roleViewModels) {
  const logic = read(viewModelPath)

  assert(logic.includes("statusFeedbackTitle: isFailedStatus ? '错误信息反馈'"), `${viewModelPath} must make error feedback the failed-state primary content`)
  assert(logic.includes("statusReasonTitle: isFailedStatus ? '失败原因'"), `${viewModelPath} must label the backend failure reason clearly`)
  assert(logic.includes('showVerifyFeePrompt'), `${viewModelPath} must own status-specific verify-fee visibility`)
}

console.log('check-baofu-account-status-page: status summary is focused on account state and failure feedback')
