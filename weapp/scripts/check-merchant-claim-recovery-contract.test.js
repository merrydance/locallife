const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const ROOT = path.resolve(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(ROOT, relativePath), 'utf8')
}

function loadTsModule(relativePath, requireStub = () => ({})) {
  const sourcePath = path.join(ROOT, relativePath)
  const source = fs.readFileSync(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018,
      esModuleInterop: true
    }
  }).outputText

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require: requireStub,
    console
  }

  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const capturedRequests = []
const api = loadTsModule('miniprogram/pages/merchant/_main_shared/api/appeals-customer-service.ts', (id) => {
  if (id === '../../../../utils/request') {
    return {
      request(options) {
        capturedRequests.push(options)
        return Promise.resolve(options)
      }
    }
  }
  if (id === './payment') {
    return {}
  }
  return {}
})

const { appealManagementService, claimManagementService } = api

claimManagementService.getMerchantClaimRecovery(77)
claimManagementService.payMerchantClaimRecovery(77)
appealManagementService.getMerchantAppeals({ page_id: 1, page_size: 20, status: 'submitted' })
appealManagementService.getMerchantAppealsSummary()
appealManagementService.getMerchantAppealDetail(55)
appealManagementService.createAppeal({ claim_id: 12, reason: '顾客已当面确认，申请复核责任归属' })

assert.deepStrictEqual(capturedRequests.map((request) => `${request.method} ${request.url}`), [
  'GET /v1/merchant/recoveries/77',
  'POST /v1/merchant/recoveries/77/pay',
  'GET /v1/merchant/recovery-disputes',
  'GET /v1/merchant/recovery-disputes/summary',
  'GET /v1/merchant/recovery-disputes/55',
  'POST /v1/merchant/recovery-disputes'
])

const apiSource = read('miniprogram/pages/merchant/_main_shared/api/appeals-customer-service.ts')
const claimListSource = read('miniprogram/pages/merchant/claims/index.ts')
const claimDetailSource = read('miniprogram/pages/merchant/claims/detail/index.ts')
const claimDetailWxml = read('miniprogram/pages/merchant/claims/detail/index.wxml')
const appealListSource = read('miniprogram/pages/merchant/appeals/index.ts')
const appealDetailSource = read('miniprogram/pages/merchant/appeals/detail/index.ts')

assert(apiSource.includes("export type AppealStatus = 'submitted' | 'approved' | 'rejected'"), 'merchant dispute status must use backend recovery-dispute statuses')
assert(apiSource.includes("export type ClaimRecoveryStatus = 'pending' | 'paid' | 'overdue' | 'waived' | 'disputed'"), 'merchant recovery status must use backend disputed status')
assert(apiSource.includes("export type ClaimRecoveryReleaseStatus = 'pending' | 'released' | 'retrying' | 'syncing'"), 'merchant recovery contract must expose backend release visibility status')
assert(apiSource.includes('recovery_id?: number'), 'merchant claim contract must expose backend recovery_id')
assert(apiSource.includes('release_status?: ClaimRecoveryReleaseStatus'), 'merchant recovery response must expose release_status')
assert(apiSource.includes('release_message?: string'), 'merchant recovery response must expose release_message')
assert(!apiSource.includes('/v1/merchant/claims/${claimId}/recovery'), 'merchant recovery API must not use claim-id recovery path')
assert(!apiSource.includes('/v1/merchant/appeals'), 'merchant appeal API must not use stale appeals route')

assert(claimListSource.includes("type ClaimFilterTab = 'all' | 'pending_action' | 'disputed' | 'closed'"), 'claim list tab contract must use disputed bucket')
assert(claimListSource.includes('recovery_dispute_id'), 'claim list must read backend recovery_dispute_id')
assert(claimDetailSource.includes('claim.recovery_id'), 'claim detail must read backend recovery_id')
assert(claimDetailSource.includes('payMerchantClaimRecovery(this.data.recoveryId'), 'claim detail payment must use recovery id')
assert(claimDetailSource.includes('getMerchantClaimRecovery(this.data.recoveryId'), 'claim detail recovery read must use recovery id')
assert(claimDetailSource.includes('recovery?.release_message'), 'claim detail must map backend release message into view state')
assert(claimDetailSource.includes("'detail.recoveryReleaseMessage': recovery.release_message"), 'claim detail retry must refresh release message')
assert(claimDetailWxml.includes('detail.recoveryReleaseMessage'), 'claim detail must render release visibility message')
assert(!claimDetailSource.includes('payMerchantClaimRecovery(this.data.claimId)'), 'claim detail must not pay recovery with claim id')
assert(!claimDetailSource.includes('getMerchantClaimRecovery(this.data.claimId)'), 'claim detail must not load recovery with claim id')
assert(!appealListSource.includes('/v1/merchant/appeals'), 'appeal list page must rely on recovery-dispute service routes')
assert(!appealDetailSource.includes('/v1/merchant/appeals'), 'appeal detail page must rely on recovery-dispute service routes')

console.log('check-merchant-claim-recovery-contract: validated merchant claim recovery route contract')
