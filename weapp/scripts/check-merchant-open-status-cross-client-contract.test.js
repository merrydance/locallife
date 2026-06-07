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

const backendServer = read('locallife/api/server.go')
const backendMerchant = read('locallife/api/merchant.go')
const miniProgramApi = read('weapp/miniprogram/api/merchant.ts')
const miniProgramService = read('weapp/miniprogram/pages/merchant/_services/merchant-open-status.ts')
const miniProgramDashboard = read('weapp/miniprogram/pages/merchant/dashboard/index.ts')
const miniProgramDashboardWxml = read('weapp/miniprogram/pages/merchant/dashboard/index.wxml')
const flutterEnv = read('merchant_app/lib/config/env.dart')
const flutterStatusProvider = read('merchant_app/lib/features/order/working_status_provider.dart')
const flutterOrderList = read('merchant_app/lib/features/order/order_list_page.dart')
const flutterStatusTest = read('merchant_app/test/working_status_provider_test.dart')
const flutterSyncManagerTest = read('merchant_app/test/working_status_sync_manager_test.dart')

assertContains(
  backendServer,
  'authGroup.GET("/merchants/me/status", server.getMerchantOpenStatus)',
  'backend must expose the merchant open-status read route'
)
assertContains(
  backendServer,
  'merchantProfileWriteGroup.PATCH("/status", server.updateMerchantOpenStatus)',
  'backend must expose the merchant open-status write route under the protected profile write group'
)
assertContains(
  backendMerchant,
  'IsOpen      *bool  `json:"is_open" binding:"required"`',
  'backend PATCH request must require the is_open truth field'
)
assertContains(
  backendMerchant,
  'IsOpen            bool                              `json:"is_open"`',
  'backend status response must return is_open for clients to rehydrate from server truth'
)
assertContains(
  backendMerchant,
  '@Router /v1/merchants/me/status [patch]',
  'backend Swagger contract must document the PATCH status route'
)
assertContains(
  backendMerchant,
  '@Router /v1/merchants/me/status [get]',
  'backend Swagger contract must document the GET status route'
)

assertContains(
  miniProgramApi,
  'url: \'/v1/merchants/me/status\',\n    method: \'GET\'',
  'Mini Program status reader must call the backend GET status route'
)
assertContains(
  miniProgramApi,
  'const data: Record<string, unknown> = { is_open: isOpen }',
  'Mini Program status writer must send the backend is_open field'
)
assertContains(
  miniProgramApi,
  'url: \'/v1/merchants/me/status\',\n    method: \'PATCH\'',
  'Mini Program status writer must call the backend PATCH status route'
)
assertContains(
  miniProgramService,
  'return getMyMerchantOpenStatus()',
  'Mini Program merchant service must delegate status reads to the shared API wrapper'
)
assertContains(
  miniProgramService,
  'return updateMyMerchantOpenStatus(nextIsOpen)',
  'Mini Program merchant service must delegate status writes to the shared API wrapper'
)
assertContains(
  miniProgramDashboard,
  'captureDashboardRequest(fetchMerchantStorefrontOpenStatus())',
  'Mini Program dashboard must load status from the backend status route'
)
assertContains(
  miniProgramDashboard,
  'openStatusResult.value.is_open',
  'Mini Program dashboard must read is_open from the backend status response'
)
assertContains(
  miniProgramDashboard,
  'updateMerchantStorefrontOpenStatus(nextIsOpen)',
  'Mini Program dashboard status switch must write through the merchant status service'
)
assertContains(
  miniProgramDashboard,
  'isOpen: response.is_open',
  'Mini Program dashboard must rehydrate the switch from the PATCH response'
)
assertContains(
  miniProgramDashboard,
  'openStatusSubmitting',
  'Mini Program dashboard must keep duplicate status writes guarded'
)
assertContains(
  miniProgramDashboardWxml,
  'value="{{isOpen}}"',
  'Mini Program dashboard switch must render backend-derived isOpen'
)
assertContains(
  miniProgramDashboardWxml,
  'bind:change="onOpenStatusSwitchChange"',
  'Mini Program dashboard switch must call the status change handler'
)

assertContains(
  flutterEnv,
  'defaultValue: \'https://llapi.merrydance.cn/v1\'',
  'Flutter API base URL must include the backend v1 prefix used by its relative status route'
)
assertContains(
  flutterStatusProvider,
  '_apiClient.get(\'/merchants/me/status\')',
  'Flutter App status sync must read the same backend status route'
)
assertContains(
  flutterStatusProvider,
  '_apiClient.patch(\n        \'/merchants/me/status\'',
  'Flutter App status writer must patch the same backend status route'
)
assertContains(
  flutterStatusProvider,
  'data: <String, dynamic>{\'is_open\': isOnline}',
  'Flutter App status writer must send the backend is_open field'
)
assertContains(
  flutterStatusProvider,
  'final isOpen = data[\'is_open\'];',
  'Flutter App status provider must read is_open from the backend response'
)
assertContains(
  flutterStatusProvider,
  'isOnline: isOpen is bool ? isOpen : fallback.isOnline',
  'Flutter App status provider must rehydrate state from backend truth instead of optimistic input'
)
assertContains(
  flutterOrderList,
  'ref.listen(workingStatusProvider.select((state) => state.isOnline)',
  'Flutter order list must react to backend-confirmed working status changes'
)
assertContains(
  flutterOrderList,
  '.setStatus(val)',
  'Flutter order-list switch must write through WorkingStatusNotifier.setStatus'
)
assertContains(
  flutterOrderList,
  '.setStatus(true)',
  'Flutter offline empty-state action must write through WorkingStatusNotifier.setStatus'
)

assertContains(
  flutterStatusTest,
  'expect(apiClient.lastGetPath, \'/merchants/me/status\')',
  'Flutter status unit test must cover the backend GET status route'
)
assertContains(
  flutterStatusTest,
  'expect(apiClient.lastPatchPath, \'/merchants/me/status\')',
  'Flutter status unit test must cover the backend PATCH status route'
)
assertContains(
  flutterStatusTest,
  'expect(apiClient.lastPatchData, <String, dynamic>{\'is_open\': true})',
  'Flutter status unit test must cover the backend is_open write payload'
)
assertContains(
  flutterSyncManagerTest,
  'expect(apiClient.lastMerchantStatusPath, \'/merchants/me/status\')',
  'Flutter sync-manager widget test must prove login sync reads backend status truth'
)

console.log('check-merchant-open-status-cross-client-contract: Mini Program and Flutter App share backend merchant open-status truth')
