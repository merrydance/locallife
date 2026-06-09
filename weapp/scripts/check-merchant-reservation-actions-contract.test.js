const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const ROOT = path.resolve(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(ROOT, relativePath), 'utf8')
}

function assertContains(source, expected, message) {
  assert(
    source.includes(expected),
    message || `Expected source to contain ${expected}`
  )
}

function loadTsModule(relativePath, requireStub = () => ({}), extraSandbox = {}) {
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
    console,
    ...extraSandbox
  }

  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const reservationApi = loadTsModule('miniprogram/pages/merchant/_main_shared/api/reservation.ts', (id) => {
  if (id === '../miniprogram_npm/dayjs/index') {
    return () => ({
      isValid: () => true,
      subtract: () => ({ isBefore: () => false }),
      add: () => ({ isAfter: () => false }),
      isBefore: () => false,
      isAfter: () => false
    })
  }
  if (id === '../../../../utils/request') {
    return {
      request: () => Promise.resolve({}),
      API_BASE: ''
    }
  }
  throw new Error(`unexpected reservation api require: ${id}`)
})

const reservationView = loadTsModule('miniprogram/pages/merchant/_utils/merchant-reservations-view.ts', (id) => {
  if (id === '../_main_shared/miniprogram_npm/dayjs/index') {
    return () => ({
      isValid: () => true,
      format: () => '12:00'
    })
  }
  if (id === '../_main_shared/api/reservation') {
    return reservationApi
  }
  throw new Error(`unexpected reservation view require: ${id}`)
})

function loadReservationPage(reservationService) {
  let pageConfig

  loadTsModule('miniprogram/pages/merchant/reservations/index.ts', (id) => {
    if (id === '../_main_shared/miniprogram_npm/dayjs/index') {
      return () => ({ format: () => '2026-06-09' })
    }
    if (id === '../../../utils/responsive') {
      return { getStableBarHeights: () => ({ navBarHeight: 88 }) }
    }
    if (id === '../_main_shared/api/reservation') {
      return {
        ...reservationApi,
        ReservationService: reservationService,
        cancelReservation: async () => ({}),
        checkInReservation: async () => ({}),
        completeReservationByMerchant: async () => ({}),
        confirmReservationByMerchant: async () => ({}),
        markReservationNoShow: async () => ({}),
        startCookingReservation: async () => ({})
      }
    }
    if (id === '../../../utils/logger') {
      return { logger: { error() {}, warn() {}, info() {} } }
    }
    if (id === '../../../utils/user-facing') {
      return { getErrorUserMessage: (_err, fallback) => fallback }
    }
    if (id === '../../../utils/console-access') {
      return {
        ensureMerchantConsoleAccess: async () => 'granted',
        getMerchantConsoleAccessErrorMessage: () => '',
        isMerchantConsoleAccessDenied: () => false,
        isMerchantConsoleAccessGranted: () => true
      }
    }
    if (id === '../_utils/merchant-reservations-view') {
      return reservationView
    }
    throw new Error(`unexpected reservation page require: ${id}`)
  }, {
    Page(config) {
      pageConfig = config
    },
    wx: {
      stopPullDownRefresh() {},
      showToast() {},
      showLoading() {},
      hideLoading() {},
      navigateTo() {}
    },
    Promise,
    Date,
    setTimeout,
    clearTimeout
  })

  assert(pageConfig, 'reservation page should register itself')
  return pageConfig
}

const backendAllowed = {
  can_edit: false,
  can_cancel: false,
  can_confirm: false,
  can_check_in: false,
  can_start_cooking: false,
  can_no_show: true,
  can_complete: false,
  primary_action_key: 'no_show',
  show_more_actions: true
}

const blockedByLocalStatus = reservationApi.getMerchantReservationActionState({
  status: 'completed',
  cooking_started_at: '',
  reservation_date: '2026-06-09',
  reservation_time: '12:00',
  merchant_action_state: backendAllowed
})

assert.strictEqual(blockedByLocalStatus.canEdit, false)
assert.strictEqual(blockedByLocalStatus.canConfirm, false)
assert.strictEqual(blockedByLocalStatus.canNoShow, true)
assert.strictEqual(blockedByLocalStatus.primaryActionKey, 'no_show')
assert.strictEqual(
  blockedByLocalStatus.primaryActionLabel,
  '标记未到店',
  'frontend action label must be derived from backend primary_action_key'
)

const card = reservationView.buildReservationCard({
  id: 1,
  table_id: 2,
  user_id: 3,
  merchant_id: 4,
  reservation_date: '2026-06-09',
  reservation_time: '12:00',
  guest_count: 2,
  contact_name: '王先生',
  contact_phone: '13800000000',
  source: 'merchant',
  payment_mode: 'deposit',
  deposit_amount: 0,
  prepaid_amount: 0,
  refund_deadline: '',
  payment_deadline: '',
  status: 'completed',
  merchant_action_state: backendAllowed,
  created_at: '2026-06-09T12:00:00Z'
})

assert.strictEqual(card.canNoShow, true)
assert.strictEqual(card.primaryActionKey, 'no_show')
assert.strictEqual(card.showMoreActions, true)

const moreItems = reservationView.buildReservationMoreActionItems(card)
const moreItemKeys = Array.from(moreItems, (item) => item.key)
assert.deepStrictEqual(
  moreItemKeys,
  ['no_show'],
  'more-action sheet must be built from backend merchant_action_state-derived flags'
)
assert.strictEqual(moreItems[0].color, 'danger')

assert.strictEqual(
  reservationView.getReservationListStatusFilter('all'),
  undefined,
  'all tab must not send a status filter'
)
assert.strictEqual(
  reservationView.getReservationListStatusFilter('checked_in'),
  'checked_in',
  'checked-in tab must send backend status filter'
)
assert.strictEqual(
  reservationView.getReservationListStatusFilter('exception'),
  'exception',
  'exception tab must send backend exception status filter'
)

const pageSource = read('miniprogram/pages/merchant/reservations/index.ts')
const pageWxml = read('miniprogram/pages/merchant/reservations/index.wxml')
const apiSource = read('miniprogram/pages/merchant/_main_shared/api/reservation.ts')
const backendReservation = fs.readFileSync(path.join(ROOT, '../locallife/api/table_reservation.go'), 'utf8')
const backendActionState = fs.readFileSync(path.join(ROOT, '../locallife/api/table_reservation_action_state.go'), 'utf8')

assertContains(
  backendReservation,
  'MerchantActionState',
  'backend merchant reservation response must expose merchant action permission state'
)
assertContains(
  backendActionState,
  'func newMerchantListReservationResponse',
  'backend merchant reservation list must use the action-state-aware response builder'
)
assertContains(
  backendActionState,
  'MerchantActionState: buildMerchantReservationActionStateResponse(',
  'backend merchant reservation list response builder must populate merchant_action_state'
)
assertContains(
  backendActionState,
  'logic.ResolveMerchantReservationActionState',
  'backend merchant reservation action payload must come from the shared resolver'
)
assertContains(
  backendReservation,
  'resp[i] = newMerchantListReservationResponse(r, staffRole, now)',
  'backend merchant reservation list branches must use newMerchantListReservationResponse'
)
assertContains(
  apiSource,
  'merchant_action_state?: MerchantReservationActionStatePayload',
  'Mini Program reservation DTO must carry backend merchant_action_state'
)
assertContains(
  apiSource,
  'if (reservation.merchant_action_state)',
  'Mini Program action state must prefer backend merchant_action_state when present'
)
assertContains(
  apiSource,
  'primary_action_key',
  'Mini Program action state must consume backend primary_action_key'
)

assertContains(
  pageSource,
  'status: getReservationListStatusFilter(this.data.currentTab)',
  'reservation list must send the selected status tab to the backend'
)
assertContains(
  pageSource,
  'date: this.data.date',
  'reservation list must send the selected date to the backend'
)
assertContains(
  pageSource,
  'const total = typeof response.total === \'number\' ? response.total : nextReservations.length',
  'reservation list must read backend total before falling back for compatibility'
)
assertContains(
  pageSource,
  'listHasMore: page * this.data.listPageSize < total',
  'reservation list must compute pagination from backend total'
)
assertContains(
  pageSource,
  'listSummaryText: buildListSummaryText(this.data.currentTab, total)',
  'reservation list summary must use backend total for date+status tabs'
)
assertContains(
  pageSource,
  'buildReservationMoreActionItems(reservation)',
  'more actions must be built from card action-state flags'
)
assertContains(
  pageWxml,
  'wx:if="{{item.primaryActionKey}}"',
  'reservation card must only render primary action when backend/view state exposes one'
)
assertContains(
  pageWxml,
  'data-action="{{item.primaryActionKey}}"',
  'reservation card primary action must use backend/view-state primaryActionKey'
)
assertContains(
  pageWxml,
  'wx:if="{{item.showMoreActions}}"',
  'reservation card must only render more-actions entry when backend/view state exposes one'
)

async function assertReservationListConsumesBackendTotal() {
  const calls = []
  const page = loadReservationPage({
    getMerchantReservations: async (params) => {
      calls.push(params)
      return {
        reservations: [{
          id: 101,
          table_id: 2,
          user_id: 3,
          merchant_id: 4,
          reservation_date: '2026-06-09',
          reservation_time: '12:00',
          guest_count: 2,
          contact_name: '王先生',
          contact_phone: '13800000000',
          source: 'merchant',
          payment_mode: 'deposit',
          deposit_amount: 0,
          prepaid_amount: 0,
          refund_deadline: '',
          payment_deadline: '',
          status: 'completed',
          merchant_action_state: backendAllowed,
          created_at: '2026-06-09T12:00:00Z'
        }],
        total: 42,
        page_id: 1,
        page_size: 20
      }
    }
  })

  const ctx = {
    ...page,
    data: {
      ...JSON.parse(JSON.stringify(page.data)),
      accessReady: true,
      accessDenied: false,
      accessErrorMessage: '',
      date: '2026-06-09',
      currentTab: 'exception',
      listPageSize: 20,
      listHasMore: true
    },
    setData(patch) {
      Object.assign(this.data, patch)
    }
  }

  const loaded = await page.loadReservationList.call(ctx, true, { showLoading: false })

  assert.strictEqual(loaded, true)
  assert.strictEqual(calls.length, 1, 'reservation list should make exactly one backend request')
  assert.deepStrictEqual(
    {
      page_id: calls[0].page_id,
      page_size: calls[0].page_size,
      date: calls[0].date,
      status: calls[0].status
    },
    {
      page_id: 1,
      page_size: 20,
      date: '2026-06-09',
      status: 'exception'
    },
    'reservation list request must send selected date and status tab to backend'
  )
  assert.strictEqual(ctx.data.listTotal, 42, 'reservation list must store backend total')
  assert.strictEqual(ctx.data.listHasMore, true, 'reservation list pagination must use backend total')
  assert.strictEqual(
    ctx.data.listSummaryText,
    '异常预订共 42 条',
    'reservation list summary must use backend total for the selected tab'
  )
  assert.strictEqual(
    ctx.data.reservations[0].primaryActionKey,
    'no_show',
    'reservation list cards must preserve backend-derived primary action state'
  )
}

assertReservationListConsumesBackendTotal()
  .then(() => {
    console.log('check-merchant-reservation-actions-contract: action permissions and date+status totals are backend-truth aligned')
  })
  .catch((error) => {
    console.error(error)
    process.exit(1)
  })
