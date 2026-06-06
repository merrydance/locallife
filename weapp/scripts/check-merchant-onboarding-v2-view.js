const assert = require('assert')
const fs = require('fs')
const path = require('path')

const ROOT = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(ROOT, relativePath), 'utf8')
}

function exists(relativePath) {
  return fs.existsSync(path.join(ROOT, relativePath))
}

function assertIncludes(source, token, message) {
  assert(source.includes(token), message)
}

function assertNotIncludes(source, token, message) {
  assert(!source.includes(token), message)
}

const appJson = JSON.parse(read('miniprogram/app.json'))
const merchantPackage = appJson.subPackages.find((pkg) => pkg.root === 'pages/merchant')

assert(merchantPackage, 'app.json must keep the merchant subpackage')
for (const route of [
  'onboarding-v2/index',
  'onboarding-v2/baofu-submit/index',
  'onboarding-v2/intent/index'
]) {
  assert(
    merchantPackage.pages.includes(route),
    `merchant subpackage must register ${route}`
  )
}

for (const relativePath of [
  'miniprogram/pages/merchant/_main_shared/config/merchant-onboarding-v2.ts',
  'miniprogram/pages/merchant/_main_shared/services/merchant-onboarding-v2-view.ts',
  'miniprogram/pages/merchant/_main_shared/services/merchant-onboarding-v2-runtime.ts',
  'miniprogram/pages/merchant/onboarding-v2/index.ts',
  'miniprogram/pages/merchant/onboarding-v2/index.wxml',
  'miniprogram/pages/merchant/onboarding-v2/index.wxss',
  'miniprogram/pages/merchant/onboarding-v2/index.json',
  'miniprogram/pages/merchant/onboarding-v2/baofu-submit/index.ts',
  'miniprogram/pages/merchant/onboarding-v2/baofu-submit/index.wxml',
  'miniprogram/pages/merchant/onboarding-v2/baofu-submit/index.wxss',
  'miniprogram/pages/merchant/onboarding-v2/baofu-submit/index.json',
  'miniprogram/pages/merchant/onboarding-v2/intent/index.ts',
  'miniprogram/pages/merchant/onboarding-v2/intent/index.wxml',
  'miniprogram/pages/merchant/onboarding-v2/intent/index.wxss',
  'miniprogram/pages/merchant/onboarding-v2/intent/index.json'
]) {
  assert(exists(relativePath), `${relativePath} must exist`)
}

const config = read('miniprogram/pages/merchant/_main_shared/config/merchant-onboarding-v2.ts')
assertIncludes(config, 'MERCHANT_ONBOARDING_V2_INTENT_QR_URL', 'QR config must export the intent QR URL')
assertIncludes(config, 'MERCHANT_ONBOARDING_V2_ALLOWED_QR_HOSTS', 'QR config must define an explicit allowed host list')
assertNotIncludes(config, "MERCHANT_ONBOARDING_V2_INTENT_QR_URL = ''", 'QR URL must not be empty for release validation')
assertNotIncludes(config, "MERCHANT_ONBOARDING_V2_INTENT_QR_URL = 'https://...'", 'QR URL must not be the placeholder value')
assertNotIncludes(config, 'merchant_confirm', 'V2 config must not wire Baofoo account-willingness APIs')
assertNotIncludes(config, 'confirm_apply_state_query', 'V2 config must not wire Baofoo account-willingness query APIs')

const view = read('miniprogram/pages/merchant/_main_shared/services/merchant-onboarding-v2-view.ts')
for (const token of [
  'platform_not_started',
  'platform_draft',
  'platform_submitted',
  'platform_rejected',
  'platform_approved',
  'owner_not_ready_after_approval',
  'baofu_profile_pending',
  'baofu_verify_fee_pending',
  'baofu_verify_fee_processing',
  'baofu_opening_processing',
  'baofu_merchant_report_processing',
  'baofu_applet_auth_pending',
  'baofu_failed',
  'baofu_voided',
  'baofu_ready',
  'intent_qr_pending',
  'intent_qr_unavailable',
  'locked_until_baofu_ready'
]) {
  assertIncludes(view, token, `ViewState must cover ${token}`)
}
assertIncludes(view, 'buildMerchantOnboardingV2ViewState', 'ViewState builder must be exported')
assertIncludes(view, 'buildMerchantOnboardingV2IntentViewState', 'Intent ViewState builder must be exported')
assertIncludes(view, 'getBaofuAccountStatusText', 'Baofoo status text must come from existing Baofoo helpers')
assertIncludes(view, 'buildBaofuSettlementAccountView', 'Baofoo action capability must come from existing Baofoo view helpers')
assertIncludes(view, '查看确认流程', 'Hub ready action must open the QR guide flow')
assertNotIncludes(view, '去确认开户意愿', 'Hub must not imply LocalLife performs the external account-willingness confirmation')
assertNotIncludes(view, '开户意愿已确认', 'ViewState must not invent account-willingness terminal truth')

const runtime = read('miniprogram/pages/merchant/_main_shared/services/merchant-onboarding-v2-runtime.ts')
assertIncludes(runtime, 'getUserInfo', 'Runtime must use the user/workbench truth before entering merchant-only Baofoo APIs')
assertIncludes(runtime, 'hasMerchantOnboardingV2PlatformAccess', 'Runtime must gate Baofoo reads on merchant workbench or merchant role access')
assertNotIncludes(runtime, 'getMyApplication', 'Runtime must not call the non-existent latest merchant application endpoint')
assertNotIncludes(runtime, 'getMerchantApplication(', 'Runtime must not create or reset application drafts from hub load')
assertIncludes(runtime, 'getMerchantBaofuSettlementAccount', 'Runtime must compose the existing merchant Baofoo account API')
assertIncludes(runtime, 'if (!hasMerchantOnboardingV2PlatformAccess(user))', 'Runtime must avoid Baofoo reads until merchant workbench or role access is granted')
assertIncludes(runtime, "const application = { status: 'approved' } as MerchantApplicationDraftResponse", 'Runtime may synthesize platform-approved ViewState only after merchant workbench or role access is granted')
assertIncludes(runtime, 'lastTrustedViewState', 'Runtime state must preserve last trusted ViewState')
assertIncludes(runtime, 'requestSeq', 'Runtime state must guard out-of-order refresh responses')
assertIncludes(runtime, 'isKnownNoApplicationError', 'Runtime must classify known no-application errors explicitly')
assertIncludes(runtime, 'isOwnerNotReadyAfterApprovalError', 'Runtime must classify owner-not-ready errors explicitly')
assertNotIncludes(runtime, 'merchant_confirm', 'Runtime must not call Baofoo account-willingness APIs')

const hubTs = read('miniprogram/pages/merchant/onboarding-v2/index.ts')
const hubWxml = read('miniprogram/pages/merchant/onboarding-v2/index.wxml')
const hubJson = read('miniprogram/pages/merchant/onboarding-v2/index.json')
assertIncludes(hubTs, '/pages/register/merchant/store/index', 'Hub must route platform actions to the registration onboarding page before merchant access exists')
assertIncludes(hubTs, '/pages/merchant/onboarding-v2/baofu-submit/index', 'Hub must route Baofoo profile submission to the v2 submit adapter')
assertNotIncludes(hubTs, '/pages/merchant/finance/settlement-account/submit/index', 'Hub must not route v2 users to the legacy finance submit page')
assertIncludes(hubTs, 'baofu_submit', 'Hub must recognize v2 Baofoo submit return markers')
assertIncludes(hubTs, 'requestSeq', 'Hub must guard stale refresh responses')
assertIncludes(hubWxml, 't-steps', 'Hub must render the three-stage flow with TDesign steps')
assertIncludes(hubWxml, '平台入驻', 'Hub must show platform stage')
assertIncludes(hubWxml, '宝付开户', 'Hub must show Baofoo stage')
assertIncludes(hubWxml, '开户意愿确认', 'Hub must show external intent stage')
assertIncludes(hubJson, 't-steps', 'Hub JSON must declare TDesign steps')

const submitTs = read('miniprogram/pages/merchant/onboarding-v2/baofu-submit/index.ts')
const submitWxml = read('miniprogram/pages/merchant/onboarding-v2/baofu-submit/index.wxml')
const submitWxss = read('miniprogram/pages/merchant/onboarding-v2/baofu-submit/index.wxss')
const submitJson = read('miniprogram/pages/merchant/onboarding-v2/baofu-submit/index.json')
assertIncludes(submitTs, 'ensureMerchantApplymentAccess', 'V2 submit adapter must re-check merchant-owner access on direct entry')
assertNotIncludes(submitTs, 'getMyApplication', 'V2 submit adapter must not call the non-existent latest merchant application endpoint')
assertIncludes(submitTs, 'baofuSettlementSubmitBehavior', 'V2 submit adapter must reuse the existing Baofoo submit behavior')
assertIncludes(submitTs, 'startBaofuAccountOnboarding', 'V2 submit adapter must reuse the existing Baofoo onboarding service')
assertIncludes(submitTs, '_startBaofuSubmitPendingTick', 'V2 submit adapter must start the existing submit-pending countdown')
assertIncludes(submitTs, 'onProgress', 'V2 submit adapter must feed backend polling progress into the wait panel')
assertIncludes(submitTs, '/pages/merchant/onboarding-v2/index?from=baofu_submit', 'V2 submit adapter terminal path must return to the v2 hub')
assertNotIncludes(submitTs, '/pages/merchant/finance/settlement-account/index', 'V2 submit adapter must not return to the legacy finance status page')
assertIncludes(submitWxml, 'baofu-onboarding-wait', 'V2 submit adapter must render the existing Baofoo wait component')
assertIncludes(submitWxml, 'elapsedSeconds="{{waitElapsedSeconds}}"', 'V2 submit adapter must wire wait elapsed seconds')
assertIncludes(submitWxml, 'timerVisible="{{waitTimerVisible}}"', 'V2 submit adapter must wire wait timer visibility')
assertIncludes(submitWxss, '@import "../../../../styles/baofu-settlement-account.wxss";', 'V2 submit adapter must import the shared Baofoo WXSS with a resolvable path')
assertIncludes(submitJson, 'baofu-onboarding-wait', 'V2 submit adapter must declare the existing Baofoo wait component')
assertNotIncludes(submitWxml, '开户意愿', 'V2 submit form must not mix in the external account-willingness step')

const intentTs = read('miniprogram/pages/merchant/onboarding-v2/intent/index.ts')
const intentWxml = read('miniprogram/pages/merchant/onboarding-v2/intent/index.wxml')
const intentJson = read('miniprogram/pages/merchant/onboarding-v2/intent/index.json')
assertIncludes(intentTs, 'wx.downloadFile', 'Intent page must download the configured QR before saving')
assertIncludes(intentTs, 'wx.saveImageToPhotosAlbum', 'Intent page must save the QR to the phone album')
assertIncludes(intentTs, 'wx.openSetting', 'Intent page must guide album permission recovery')
assertIncludes(intentTs, 'qrSaveState', 'Intent page must expose QR save state')
assertIncludes(intentTs, 'permission_denied', 'Intent page must distinguish album permission denial')
assertIncludes(intentWxml, '保存二维码', 'Intent page must expose save QR action')
assertIncludes(intentWxml, '法人使用本人微信', 'Intent page copy must require the legal representative WeChat account')
assertIncludes(intentWxml, '从相册识别二维码', 'Intent page must explain album recognition')
assertNotIncludes(intentWxml, '我已完成确认', 'Intent page must not create a local confirmation-complete action')
assertIncludes(intentJson, 't-image', 'Intent page should use TDesign image rendering')

console.log('check-merchant-onboarding-v2-view: validated merchant onboarding v2 frontend contract')
