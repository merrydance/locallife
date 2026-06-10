const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const ROOT = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(ROOT, relativePath), 'utf8')
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

function clone(value) {
  return JSON.parse(JSON.stringify(value))
}

function applySetData(target, patch) {
  for (const [key, value] of Object.entries(patch)) {
    if (!key.includes('.')) {
      target.data[key] = value
      continue
    }

    const parts = key.split('.')
    let cursor = target.data
    for (let index = 0; index < parts.length - 1; index += 1) {
      const part = parts[index]
      if (!cursor[part] || typeof cursor[part] !== 'object') {
        cursor[part] = {}
      }
      cursor = cursor[part]
    }
    cursor[parts[parts.length - 1]] = value
  }
}

function createPageContext(page, overrides = {}) {
  return {
    ...page,
    data: {
      ...clone(page.data),
      ...overrides
    },
    setData(patch) {
      applySetData(this, patch)
    }
  }
}

async function flushMicrotasks() {
  await Promise.resolve()
  await Promise.resolve()
}

const apiSource = read('miniprogram/pages/merchant/_main_shared/api/baofu-withdrawal.ts')
assert(!apiSource.includes('owner_type'), 'Baofu withdrawal API must not expose owner_type')
assert(!apiSource.includes('owner_id'), 'Baofu withdrawal API must not expose owner_id')

const capturedRequests = []
const api = loadTsModule('miniprogram/pages/merchant/_main_shared/api/baofu-withdrawal.ts', (id) => {
  if (id === '../../../../utils/request') {
    return {
      request(options) {
        capturedRequests.push(options)
        return Promise.resolve(options)
      }
    }
  }
  return {}
})

assert.strictEqual(api.baofuWithdrawalEndpoint('merchant'), '/v1/merchant/finance/baofu-withdrawal')
assert.strictEqual(api.baofuWithdrawalEndpoint('platform'), '/v1/platform/finance/baofu-withdrawal')
assert.strictEqual(api.baofuWithdrawalEndpoint('operator'), '/v1/operators/me/finance/baofu-withdrawal')
assert.strictEqual(api.baofuWithdrawalEndpoint('rider'), '/v1/rider/income/baofu-withdrawal')

api.getBaofuWithdrawalBalance('merchant')
api.listBaofuWithdrawals('operator', { page: 2, limit: 20 })
api.getBaofuWithdrawal('platform', 18)
assert.throws(
  () => api.createBaofuWithdrawal('rider', { amount: 1200, remark: '提现' }),
  /缺少提现请求幂等键/,
  'Create withdrawal wrapper must require a caller-provided idempotency key'
)
api.createBaofuWithdrawal('rider', { amount: 1200, remark: '提现' }, { idempotencyKey: 'withdraw-draft-key-1' })

assert.deepStrictEqual(capturedRequests.map((request) => request.url), [
  '/v1/merchant/finance/baofu-withdrawal/balance',
  '/v1/operators/me/finance/baofu-withdrawal/withdrawals',
  '/v1/platform/finance/baofu-withdrawal/withdrawals/18',
  '/v1/rider/income/baofu-withdrawal/withdraw'
])
assert.strictEqual(capturedRequests[3].data.amount, 1200)
assert.strictEqual(capturedRequests[3].header['Idempotency-Key'], 'withdraw-draft-key-1')
assert(!('owner_type' in capturedRequests[3].data), 'Create request must not include owner_type')
assert(!('owner_id' in capturedRequests[3].data), 'Create request must not include owner_id')

const workflow = loadTsModule('miniprogram/pages/merchant/_main_shared/services/baofu-withdrawal-workflow.ts')

assert.strictEqual(workflow.formatFenToYuanText(12345), '¥123.45')
assert.strictEqual(workflow.parseYuanInputToFen('12.30').amount, 1230)
assert.strictEqual(workflow.parseYuanInputToFen('12.345').errorMessage, '金额最多保留两位小数')
assert.strictEqual(workflow.parseYuanInputToFen('abc').errorMessage, '请输入有效金额')
assert.strictEqual(workflow.buildBaofuWithdrawalStatusView('processing').text, '提现处理中')
assert.strictEqual(workflow.buildBaofuWithdrawalStatusView('succeeded').theme, 'success')
assert.strictEqual(workflow.buildBaofuWithdrawalStatusView('returned').text, '提现退票')
assert.strictEqual(workflow.buildBaofuWithdrawalBalanceView({
  available_amount: 99,
  pending_amount: 0,
  ledger_amount: 99,
  frozen_amount: 0,
  min_withdraw_amount: 100,
  max_withdraw_amount: 500000000,
  can_withdraw: false,
  disabled_reason: ''
}).disabledReason, '可提现金额不足')
assert.strictEqual(workflow.buildBaofuWithdrawalSubmitCheck('200.00', workflow.buildBaofuWithdrawalBalanceView({
  available_amount: 30000,
  pending_amount: 0,
  ledger_amount: 30000,
  frozen_amount: 0,
  min_withdraw_amount: 100,
  max_withdraw_amount: 10000,
  can_withdraw: true,
  disabled_reason: ''
})).errorMessage, '提现金额最多 ¥100.00')

function loadMerchantWithdrawalCreatePage(events, apiStub) {
  let pageConfig

  loadTsModule('miniprogram/pages/merchant/finance/withdrawals/create/index.ts', (id) => {
    if (id === '../../../_main_shared/api/baofu-withdrawal') {
      return apiStub
    }
    if (id === '../../../_main_shared/services/baofu-withdrawal-workflow') {
      return workflow
    }
    if (id === '../../../../../utils/logger') {
      return { logger: { warn() {}, error() {}, info() {} } }
    }
    if (id === '../../../../../utils/responsive') {
      return { getStableBarHeights: () => ({ navBarHeight: 88 }) }
    }
    if (id === '../../../../../utils/user-facing') {
      return { getErrorUserMessage: (_error, fallback) => fallback }
    }
    throw new Error(`unexpected merchant withdrawal create require: ${id}`)
  }, {
    Page(config) {
      pageConfig = config
    },
    wx: {
      showToast(payload) {
        events.toasts.push(payload)
      },
      redirectTo(payload) {
        events.redirects.push(payload)
      }
    },
    Date,
    Math,
    Promise
  })

  assert(pageConfig, 'merchant withdrawal create page should register itself')
  return pageConfig
}

function loadMerchantWithdrawalDetailPage(events, apiStub) {
  let pageConfig

  loadTsModule('miniprogram/pages/merchant/finance/withdrawals/detail/index.ts', (id) => {
    if (id === '../../../_main_shared/api/baofu-withdrawal') {
      return apiStub
    }
    if (id === '../../../_main_shared/services/baofu-withdrawal-workflow') {
      return workflow
    }
    if (id === '../../../../../utils/logger') {
      return { logger: { warn() {}, error() {}, info() {} } }
    }
    if (id === '../../../../../utils/responsive') {
      return { getStableBarHeights: () => ({ navBarHeight: 88 }) }
    }
    if (id === '../../../../../utils/user-facing') {
      return { getErrorUserMessage: (_error, fallback) => fallback }
    }
    throw new Error(`unexpected merchant withdrawal detail require: ${id}`)
  }, {
    Page(config) {
      pageConfig = config
    },
    wx: {
      redirectTo(payload) {
        events.redirects.push(payload)
      }
    },
    setInterval(callback, intervalMs) {
      const timerID = events.nextTimerID
      events.nextTimerID += 1
      events.intervals.set(timerID, { callback, intervalMs })
      return timerID
    },
    clearInterval(timerID) {
      events.clearedTimers.push(timerID)
      events.intervals.delete(timerID)
    },
    Date,
    Promise
  })

  assert(pageConfig, 'merchant withdrawal detail page should register itself')
  return pageConfig
}

async function assertMerchantCreateRedirectsToDurableDetailAndBlocksDuplicateSubmit() {
  const events = {
    toasts: [],
    redirects: []
  }
  const createCalls = []
  let releaseCreate
  const createPromise = new Promise((resolve) => {
    releaseCreate = resolve
  })
  const page = createPageContext(loadMerchantWithdrawalCreatePage(events, {
    getBaofuWithdrawalBalance: async () => ({
      available_amount: 30000,
      pending_amount: 0,
      ledger_amount: 30000,
      frozen_amount: 0,
      min_withdraw_amount: 100,
      max_withdraw_amount: 500000000,
      can_withdraw: true,
      disabled_reason: ''
    }),
    createBaofuWithdrawal: async (role, payload, options) => {
      createCalls.push({ role, payload, options })
      return createPromise
    }
  }), {
    initialLoading: false,
    balanceView: workflow.buildBaofuWithdrawalBalanceView({
      available_amount: 30000,
      pending_amount: 0,
      ledger_amount: 30000,
      frozen_amount: 0,
      min_withdraw_amount: 100,
      max_withdraw_amount: 500000000,
      can_withdraw: true,
      disabled_reason: ''
    }),
    amountInput: '123.45',
    withdrawalIdempotencyKey: 'merchant-withdrawal:test-key'
  })

  const firstSubmit = page.onSubmit.call(page)
  const secondSubmit = page.onSubmit.call(page)

  assert.strictEqual(createCalls.length, 1, 'merchant withdrawal create page must ignore duplicate submit while request is in flight')
  assert.strictEqual(createCalls[0].role, 'merchant')
  assert.strictEqual(createCalls[0].payload.amount, 12345)
  assert.strictEqual(
    createCalls[0].options.idempotencyKey,
    'merchant-withdrawal:test-key',
    'create page must submit with the stable draft idempotency key'
  )

  releaseCreate({
    withdrawal: {
      id: 2468,
      out_request_no: 'W20260610001',
      amount: 12345,
      status: 'processing',
      sync_state: 'accepted',
      sync_message: '银行处理中',
      created_at: '2026-06-10T10:00:00Z',
      updated_at: '2026-06-10T10:00:00Z'
    },
    message: '提现申请已受理'
  })

  await firstSubmit
  await secondSubmit

  assert.strictEqual(events.toasts[0].title, '提现申请已受理')
  assert.strictEqual(events.toasts[0].icon, 'none')
  assert.strictEqual(events.redirects.length, 1)
  assert.strictEqual(
    events.redirects[0].url,
    '/pages/merchant/finance/withdrawals/detail/index?id=2468&created=1',
    'create page must redirect to durable withdrawal detail returned by backend'
  )
  assert.strictEqual(
    page.data.submitting,
    true,
    'create page must keep submitting state through redirect instead of enabling a second request'
  )
}

async function assertMerchantDetailLoadsDurableRecordAndPollsUntilTerminal() {
  const events = {
    redirects: [],
    intervals: new Map(),
    clearedTimers: [],
    nextTimerID: 1
  }
  const detailCalls = []
  const withdrawals = [
    {
      id: 2468,
      out_request_no: 'W20260610001',
      amount: 12345,
      status: 'processing',
      sync_state: 'accepted',
      sync_message: '银行处理中',
      created_at: '2026-06-10T10:00:00Z',
      updated_at: '2026-06-10T10:00:00Z'
    },
    {
      id: 2468,
      out_request_no: 'W20260610001',
      amount: 12345,
      status: 'succeeded',
      sync_state: 'confirmed',
      sync_message: '提现已到账',
      created_at: '2026-06-10T10:00:00Z',
      updated_at: '2026-06-10T10:01:00Z'
    }
  ]

  const page = createPageContext(loadMerchantWithdrawalDetailPage(events, {
    getBaofuWithdrawal: async (role, id) => {
      detailCalls.push({ role, id })
      return { withdrawal: withdrawals[Math.min(detailCalls.length - 1, withdrawals.length - 1)] }
    }
  }))

  page._pageVisible = true
  await page.onLoad.call(page, { id: '2468', created: '1' })

  assert.deepStrictEqual(detailCalls[0], { role: 'merchant', id: 2468 })
  assert.strictEqual(page.data.id, 2468)
  assert.strictEqual(page.data.createdNotice, true)
  assert.strictEqual(page.data.item.statusView.isTerminal, false)
  assert.strictEqual(page.data.item.statusView.text, '提现处理中')
  assert.strictEqual(events.intervals.size, 1, 'detail page must poll non-terminal withdrawal detail')

  const [{ callback, intervalMs }] = Array.from(events.intervals.values())
  assert.strictEqual(intervalMs, 15000)
  callback()
  await flushMicrotasks()

  assert.strictEqual(detailCalls.length, 2, 'detail polling must re-read durable withdrawal detail')
  assert.deepStrictEqual(detailCalls[1], { role: 'merchant', id: 2468 })
  assert.strictEqual(page.data.item.statusView.isTerminal, true)
  assert.strictEqual(page.data.item.statusView.text, '提现成功')
  assert.deepStrictEqual(events.clearedTimers, [1], 'detail page must stop polling after terminal status')
  assert.strictEqual(events.intervals.size, 0)
}

async function assertMerchantWithdrawalPageRecoveryRuntimeContract() {
  await assertMerchantCreateRedirectsToDurableDetailAndBlocksDuplicateSubmit()
  await assertMerchantDetailLoadsDurableRecordAndPollsUntilTerminal()
}

const merchantWithdrawalSources = [
  read('miniprogram/pages/merchant/finance/withdrawals/index.ts'),
  read('miniprogram/pages/merchant/finance/withdrawals/create/index.ts'),
  read('miniprogram/pages/merchant/finance/withdrawals/detail/index.ts')
].join('\n')

const rolePageGroups = {
  merchant: {
    label: 'Merchant',
    listPagePath: 'miniprogram/pages/merchant/finance/withdrawals/index.ts',
    createPagePath: 'miniprogram/pages/merchant/finance/withdrawals/create/index.ts',
    source: merchantWithdrawalSources
  },
  platform: {
    label: 'Platform',
    listPagePath: 'miniprogram/pages/platform/finance/withdrawals/index.ts',
    createPagePath: 'miniprogram/pages/platform/finance/withdrawals/create/index.ts',
    source: [
      read('miniprogram/pages/platform/finance/withdrawals/index.ts'),
      read('miniprogram/pages/platform/finance/withdrawals/create/index.ts'),
      read('miniprogram/pages/platform/finance/withdrawals/detail/index.ts')
    ].join('\n')
  },
  operator: {
    label: 'Operator',
    listPagePath: 'miniprogram/pages/operator/finance/withdrawals/index.ts',
    createPagePath: 'miniprogram/pages/operator/finance/withdrawals/create/index.ts',
    source: [
      read('miniprogram/pages/operator/finance/withdrawals/index.ts'),
      read('miniprogram/pages/operator/finance/withdrawals/create/index.ts'),
      read('miniprogram/pages/operator/finance/withdrawals/detail/index.ts')
    ].join('\n')
  },
  rider: {
    label: 'Rider income',
    listPagePath: 'miniprogram/pages/rider/income/withdrawals/index.ts',
    createPagePath: 'miniprogram/pages/rider/income/withdrawals/create/index.ts',
    source: [
      read('miniprogram/pages/rider/income/withdrawals/index.ts'),
      read('miniprogram/pages/rider/income/withdrawals/create/index.ts'),
      read('miniprogram/pages/rider/income/withdrawals/detail/index.ts')
    ].join('\n')
  }
}

for (const [role, group] of Object.entries(rolePageGroups)) {
  for (const required of [
    `getBaofuWithdrawalBalance('${role}')`,
    `listBaofuWithdrawals('${role}'`,
    `createBaofuWithdrawal('${role}'`,
    `getBaofuWithdrawal('${role}'`
  ]) {
    assert(group.source.includes(required), `${group.label} withdrawal pages must call ${required}`)
  }

  assert(!group.source.includes('owner_type'), `${group.label} withdrawal pages must not pass owner_type`)
  assert(!group.source.includes('owner_id'), `${group.label} withdrawal pages must not pass owner_id`)
  assert(!group.source.includes('/account/withdraw'), `${group.label} withdrawal pages must not call legacy WeChat withdraw routes`)
  assert(!group.source.includes('/v1/rider/withdraw'), `${group.label} withdrawal pages must not call rider deposit withdraw routes`)

  const listPage = read(group.listPagePath)
  assert(!listPage.includes('Promise.all([\n      getBaofuWithdrawalBalance'), `${group.label} withdrawal list must not couple balance and records with raw Promise.all`)
  assert(
    listPage.includes('Promise.allSettled') || listPage.includes('settleBaofuWithdrawalRequest'),
    `${group.label} withdrawal list must isolate balance and records failures`
  )
  assert(listPage.includes('balanceErrorMessage'), `${group.label} withdrawal list must expose balance error state`)
  assert(listPage.includes('recordsErrorMessage'), `${group.label} withdrawal list must expose records error state`)
  assert(listPage.includes('withdrawalBalanceUnavailableView'), `${group.label} withdrawal list must disable create when balance is unavailable`)

  const createPage = read(group.createPagePath)
  assert(createPage.includes('result.withdrawal.id'), `${group.label} create page must navigate from returned withdrawal id`)
  assert(createPage.includes('result.message') || createPage.includes('sync_message'), `${group.label} create page must surface accepted/unknown create message`)
  assert(createPage.includes('buildWithdrawalIdempotencyKey'), `${group.label} create page must create a withdrawal idempotency draft key`)
  assert(createPage.includes('withdrawalIdempotencyKey'), `${group.label} create page must keep a stable withdrawal idempotency draft key`)
  assert(createPage.includes('idempotencyKey'), `${group.label} create page must pass the draft idempotency key into createBaofuWithdrawal`)
}

const userFacingSource = read('miniprogram/utils/user-facing.ts')
assert(userFacingSource.includes("'提现'"), 'User-facing mapper must allow safe withdrawal copy')
assert(userFacingSource.includes("'可提现'"), 'User-facing mapper must allow safe withdrawable-balance copy')
assert(userFacingSource.includes("'结算账户'"), 'User-facing mapper must allow safe settlement account copy')
assert(userFacingSource.includes("'余额'"), 'User-facing mapper must allow safe balance copy')

const appConfig = JSON.parse(read('miniprogram/app.json'))

function findSubPackage(root) {
  return appConfig.subPackages.find((item) => item.root === root)
}

for (const { root, routes } of [
  {
    root: 'pages/operator',
    routes: [
      'finance/withdrawals/index',
      'finance/withdrawals/create/index',
      'finance/withdrawals/detail/index'
    ]
  },
  {
    root: 'pages/platform',
    routes: [
      'finance/withdrawals/index',
      'finance/withdrawals/create/index',
      'finance/withdrawals/detail/index'
    ]
  },
  {
    root: 'pages/rider',
    routes: [
      'income/withdrawals/index',
      'income/withdrawals/create/index',
      'income/withdrawals/detail/index'
    ]
  }
]) {
  const subPackage = findSubPackage(root)
  assert(subPackage, `app.json must include subpackage ${root}`)
  for (const route of routes) {
    assert(subPackage.pages.includes(route), `app.json ${root} must register ${route}`)
  }
}

const operatorFinanceOverview = read('miniprogram/pages/operator/finance/withdraw/index.wxml')
assert(operatorFinanceOverview.includes('title="结算账户"'), 'Operator finance overview must expose user-facing settlement account wording')
assert(operatorFinanceOverview.includes('title="提现"'), 'Operator finance overview must expose withdrawal entry')
assert(!operatorFinanceOverview.includes('宝付结算账户'), 'Operator finance overview must not expose Baofoo provider wording')

const platformDashboard = [
  read('miniprogram/pages/platform/dashboard/dashboard.ts'),
  read('miniprogram/pages/platform/_services/platform-dashboard-view.ts')
].join('\n')
assert(platformDashboard.includes("title: '结算账户'"), 'Platform dashboard must expose user-facing settlement account wording')
assert(platformDashboard.includes("title: '提现'"), 'Platform dashboard must expose withdrawal entry')
assert(platformDashboard.includes("url: '/pages/platform/finance/withdrawals/index'"), 'Platform dashboard must link to platform withdrawals')
assert(!platformDashboard.includes('宝付结算账户'), 'Platform dashboard must not expose Baofoo provider wording')

const riderIncomePage = read('miniprogram/pages/rider/income/index.ts')
const riderIncomeWxml = read('miniprogram/pages/rider/income/index.wxml')
const riderIncomeService = read('miniprogram/pages/rider/_services/rider-income.ts')
assert(riderIncomePage.includes('loadRiderIncomePageData'), 'Rider income page must load withdrawal entry through the rider income task-domain service')
assert(riderIncomeWxml.includes('title="收入提现"'), 'Rider income page must expose income withdrawal entry')
assert(!riderIncomeWxml.includes('wx:if="{{withdrawalBalanceReady}}"'), 'Rider income page must not hide withdrawal entry when balance check fails')
assert(riderIncomeService.includes("getBaofuWithdrawalBalance('rider')"), 'Rider income service must use backend withdrawal balance for income withdrawal entry')
assert(riderIncomeService.includes('可提现余额暂不可确认'), 'Rider income service must provide unavailable-balance copy for the withdrawal entry')
assert(!riderIncomePage.includes('summary.totalRiderIncome'), 'Rider income page must not infer withdrawable balance from cumulative income')

const riderDepositSources = [
  read('miniprogram/pages/rider/deposit/index.ts'),
  read('miniprogram/pages/rider/deposit/index.wxml')
].join('\n')
assert(!riderDepositSources.includes('baofu-withdrawal'), 'Rider deposit refund pages must stay separate from Baofoo income withdrawal')

assertMerchantWithdrawalPageRecoveryRuntimeContract()
  .then(() => {
    console.log('check-baofu-withdrawal-workflow: server-scoped Baofoo withdrawal workflow keeps durable detail recovery and terminal polling contracts')
  })
  .catch((error) => {
    console.error(error)
    process.exit(1)
  })
