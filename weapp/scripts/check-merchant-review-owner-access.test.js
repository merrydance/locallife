const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const repoRoot = path.join(__dirname, '..')
const consoleAccessPath = path.join(repoRoot, 'miniprogram', 'utils', 'console-access.ts')
const pagePath = path.join(repoRoot, 'miniprogram', 'pages', 'merchant', 'reviews', 'index.ts')
const wxmlPath = path.join(repoRoot, 'miniprogram', 'pages', 'merchant', 'reviews', 'index.wxml')

const ownerDeniedMessage = '评价管理仅支持老板账号处理，请联系老板回复顾客评价。'

function loadConsoleAccessModule(userProfile) {
  const source = fs.readFileSync(consoleAccessPath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018
    }
  }).outputText

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (modulePath === '../api/auth') {
        return {
          getUserInfo: async () => JSON.parse(JSON.stringify(userProfile))
        }
      }
      if (modulePath === '../api/table-device-management') {
        return {
          getMerchantDeviceAccess: async () => ({
            merchant_id: 1,
            merchant_name: '本地生活小店',
            staff_role: 'staff',
            can_manage: false,
            allowed_roles: ['owner', 'manager']
          })
        }
      }
      throw new Error(`unexpected require: ${modulePath}`)
    },
    Array,
    Boolean,
    Date,
    JSON,
    Number,
    Promise,
    Set,
    String
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: consoleAccessPath })
  return sandbox.module.exports
}

async function main() {
  const consoleAccessSource = fs.readFileSync(consoleAccessPath, 'utf8')
  const pageSource = fs.readFileSync(pagePath, 'utf8')
  const wxmlSource = fs.readFileSync(wxmlPath, 'utf8')

  assert(
    consoleAccessSource.includes('canManageMerchantReviews'),
    'console-access.ts must expose an owner-aware merchant review permission helper'
  )
  assert(
    consoleAccessSource.includes('ensureMerchantReviewManagementAccess'),
    'console-access.ts must expose an owner-aware merchant review page gate'
  )
  assert(
    consoleAccessSource.includes(ownerDeniedMessage),
    'console-access.ts must keep owner-only review denied copy near the permission helper'
  )

  assert(
    pageSource.includes('ensureMerchantReviewManagementAccess'),
    'merchant reviews page must use the owner-aware review access gate'
  )
  assert(
    !pageSource.includes('ensureMerchantConsoleAccess'),
    'merchant reviews page must not rely on generic merchant console access'
  )
  assert(
    pageSource.includes('accessDeniedMessage'),
    'merchant reviews page must preserve denied copy from the owner-aware gate'
  )

  assert(
    wxmlSource.includes('accessDeniedMessage'),
    'merchant reviews denied state must render the owner-aware denied message'
  )
  assert(
    !wxmlSource.includes('当前账号无商户权限，请返回“我的”切换身份'),
    'merchant reviews denied state must not show generic merchant-console copy'
  )

  const staffModule = loadConsoleAccessModule({
    roles: ['merchant_staff'],
    workbenches: [{ id: 'merchant', status: 'granted' }]
  })
  assert.strictEqual(staffModule.canManageMerchantReviews(['merchant_staff']), false)
  const staffResult = await staffModule.ensureMerchantReviewManagementAccess()
  assert.strictEqual(staffResult.status, 'denied')
  assert.strictEqual(staffResult.message, ownerDeniedMessage)

  const customerModule = loadConsoleAccessModule({
    roles: ['customer'],
    workbenches: []
  })
  const customerResult = await customerModule.ensureMerchantReviewManagementAccess()
  assert.strictEqual(customerResult.status, 'denied')
  assert.strictEqual(customerResult.message, '当前账号无商户权限，请返回“我的”切换身份')

  const ownerModule = loadConsoleAccessModule({
    roles: ['merchant_owner'],
    workbenches: [{ id: 'merchant', status: 'granted' }]
  })
  assert.strictEqual(ownerModule.canManageMerchantReviews(['merchant_owner']), true)
  assert.strictEqual(ownerModule.canManageMerchantReviews(['merchant']), true)
  const ownerResult = await ownerModule.ensureMerchantReviewManagementAccess()
  assert.strictEqual(ownerResult.status, 'granted')

  console.log('check-merchant-review-owner-access: merchant reviews page uses owner-aware access gate')
}

main().catch((error) => {
  console.error(error)
  process.exit(1)
})
