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
