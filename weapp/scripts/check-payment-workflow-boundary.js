const fs = require('fs')
const path = require('path')
const {
  repoRoot,
  getGateScope,
  getScopedFiles
} = require('./gate-utils')

const SURFACE_ROOTS = ['weapp/miniprogram/pages/', 'weapp/miniprogram/components/']
const WORKFLOW_ROOTS = ['weapp/miniprogram/services/']
const LEGACY_NAV_ROOTS = ['weapp/miniprogram/pages/', 'weapp/miniprogram/components/', 'weapp/miniprogram/services/', 'weapp/miniprogram/utils/']

const ALLOWED_INVOKE_WECHAT_PAY_FILES = new Set([
  'weapp/miniprogram/services/payment-workflow.ts',
  'weapp/miniprogram/services/rider-deposit-payment.ts',
  'weapp/miniprogram/services/claim-recovery-payment.ts'
])

const PAYMENT_WORKFLOW_OWNER_SEGMENT = /\/_(?:api|main_shared\/api|main_shared\/services|services)\//
const INVOKE_WECHAT_PAY = /\binvokeWechatPay\b/
const PROCESS_PAYMENT_CALL = /\bprocessPayment\s*\(/
const PROCESS_PAYMENT_IMPORT = /import\s*\{[^}]*\bprocessPayment\b[^}]*\}\s*from\s*['"][^'"]*api\/payment['"]/
const LEGACY_PAYMENT_SUCCESS_NAV = /\bNavigation\.toPaymentSuccess\s*\(|\/pages\/orders\/success\//

function main() {
  const failures = []
  checkSurfacePaymentCalls(failures)
  checkWorkflowInvokeBoundary(failures)
  checkLegacySuccessNavigation(failures)

  if (failures.length > 0) {
    console.error('Payment workflow boundary gate failed. Payment pages must use the workflow layer and backend terminal truth before showing business success.')
    console.error('')

    for (const failure of failures) {
      console.error(failure.relativePath)
      for (const reason of failure.reasons) {
        console.error(`  - ${reason}`)
      }
    }

    process.exit(1)
  }

  console.log(`check-payment-workflow-boundary: validated Mini Program payment boundaries (${getGateScope()} scope)`)
}

function checkSurfacePaymentCalls(failures) {
  const files = getScopedFiles({ roots: SURFACE_ROOTS, extensions: ['.ts', '.js'] })
    .filter((relativePath) => !PAYMENT_WORKFLOW_OWNER_SEGMENT.test(relativePath))

  for (const relativePath of files) {
    const content = readRelativeFile(relativePath)
    const reasons = []

    if (INVOKE_WECHAT_PAY.test(content)) {
      reasons.push('pages/components must not call or import invokeWechatPay directly; use services/payment-workflow or a domain workflow')
    }

    if (PROCESS_PAYMENT_IMPORT.test(content) || PROCESS_PAYMENT_CALL.test(content)) {
      reasons.push('pages/components must not use legacy processPayment directly; use services/payment-workflow and payment result view models')
    }

    if (reasons.length > 0) {
      failures.push({ relativePath, reasons })
    }
  }
}

function checkWorkflowInvokeBoundary(failures) {
  const files = getScopedFiles({ roots: WORKFLOW_ROOTS, extensions: ['.ts', '.js'] })

  for (const relativePath of files) {
    if (ALLOWED_INVOKE_WECHAT_PAY_FILES.has(relativePath)) {
      continue
    }

    const content = readRelativeFile(relativePath)
    if (INVOKE_WECHAT_PAY.test(content)) {
      failures.push({
        relativePath,
        reasons: ['invokeWechatPay is only allowed in payment-workflow, rider-deposit-payment, or the temporary claim-recovery workflow boundary']
      })
    }
  }
}

function checkLegacySuccessNavigation(failures) {
  const files = getScopedFiles({ roots: LEGACY_NAV_ROOTS, extensions: ['.ts', '.js', '.wxml'] })

  for (const relativePath of files) {
    const content = readRelativeFile(relativePath)
    if (LEGACY_PAYMENT_SUCCESS_NAV.test(content)) {
      failures.push({
        relativePath,
        reasons: ['legacy payment success navigation is forbidden; route payment outcomes through pages/payment/result after backend truth checks']
      })
    }
  }
}

function readRelativeFile(relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), 'utf8')
}

main()
