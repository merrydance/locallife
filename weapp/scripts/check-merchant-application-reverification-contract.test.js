const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const repoRoot = path.join(__dirname, '..')
const apiPath = path.join(repoRoot, 'miniprogram', 'pages', 'merchant', '_main_shared', 'api', 'onboarding.ts')
const pageWxmlPath = path.join(repoRoot, 'miniprogram', 'pages', 'merchant', 'settings', 'application', 'index.wxml')

function loadOnboardingModule() {
  const source = fs.readFileSync(apiPath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018
    },
    fileName: apiPath
  }).outputText

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (modulePath === '../../../../config/config') {
        return { API_BASE_URL: 'https://api.example.test' }
      }
      if (modulePath === '../../../../utils/request') {
        return { request: async () => ({}) }
      }
      if (modulePath === '../../../../utils/media') {
        return { uploadMedia: async () => ({}) }
      }
      if (modulePath === './ocr-jobs') {
        return {
          waitForOCRJobTerminal: async () => ({}),
          waitForOCRWriteback: async () => ({})
        }
      }
      if (modulePath === '../../../../utils/error-handler') {
        return {
          AppError: class AppError extends Error {},
          ErrorType: {}
        }
      }
      if (modulePath === '../../../../utils/user-facing') {
        return {
          getErrorUserMessage(_error, fallback) {
            return fallback
          }
        }
      }
      throw new Error(`unexpected require: ${modulePath}`)
    },
    Array,
    Boolean,
    Date,
    JSON,
    Math,
    Number,
    Object,
    String
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: apiPath })
  return sandbox.module.exports
}

const onboarding = loadOnboardingModule()
const approvedStatusView = onboarding.buildMerchantApplicationStatusView('approved')

assert.strictEqual(approvedStatusView.canEdit, true, 'approved applications must remain editable for reverification')
assert.strictEqual(approvedStatusView.canSubmit, true, 'approved applications must remain submittable after edits create a draft')
assert(
  approvedStatusView.editTip.includes('线上店铺') &&
    approvedStatusView.editTip.includes('当前资料') &&
    approvedStatusView.editTip.includes('重新审核通过后更新'),
  'approved edit copy must explain that live merchant truth stays unchanged until reapproval'
)

const pageWxml = fs.readFileSync(pageWxmlPath, 'utf8')
assert(
  pageWxml.includes('wx:if="{{statusView.isApproved}}"') &&
    pageWxml.includes('title="重新认证"') &&
    pageWxml.includes('description="{{statusView.editTip}}"'),
  'merchant application page must render the approved reverification boundary near the status summary'
)
