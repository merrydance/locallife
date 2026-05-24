const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const ROOT = path.join(__dirname, '..')

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

const apiSource = read('miniprogram/api/baofu-withdrawal.ts')
assert(!apiSource.includes('owner_type'), 'Baofu withdrawal API must not expose owner_type')
assert(!apiSource.includes('owner_id'), 'Baofu withdrawal API must not expose owner_id')

const capturedRequests = []
const api = loadTsModule('miniprogram/api/baofu-withdrawal.ts', (id) => {
  if (id === '../utils/request') {
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
api.createBaofuWithdrawal('rider', { amount: 1200, remark: '提现' })

assert.deepStrictEqual(capturedRequests.map((request) => request.url), [
  '/v1/merchant/finance/baofu-withdrawal/balance',
  '/v1/operators/me/finance/baofu-withdrawal/withdrawals',
  '/v1/platform/finance/baofu-withdrawal/withdrawals/18',
  '/v1/rider/income/baofu-withdrawal/withdraw'
])
assert.strictEqual(capturedRequests[3].data.amount, 1200)
assert(!('owner_type' in capturedRequests[3].data), 'Create request must not include owner_type')
assert(!('owner_id' in capturedRequests[3].data), 'Create request must not include owner_id')

const workflow = loadTsModule('miniprogram/services/baofu-withdrawal-workflow.ts')

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

const platformDashboard = read('miniprogram/pages/platform/dashboard/dashboard.ts')
assert(platformDashboard.includes("title: '结算账户'"), 'Platform dashboard must expose user-facing settlement account wording')
assert(platformDashboard.includes("title: '提现'"), 'Platform dashboard must expose withdrawal entry')
assert(platformDashboard.includes("url: '/pages/platform/finance/withdrawals/index'"), 'Platform dashboard must link to platform withdrawals')
assert(!platformDashboard.includes('宝付结算账户'), 'Platform dashboard must not expose Baofoo provider wording')

const riderIncomePage = read('miniprogram/pages/rider/income/index.ts')
const riderIncomeService = read('miniprogram/services/rider-income.ts')
assert(riderIncomePage.includes('loadRiderIncomePageData'), 'Rider income page must load withdrawal entry through the rider income task-domain service')
assert(riderIncomeService.includes("getBaofuWithdrawalBalance('rider')"), 'Rider income service must use backend withdrawal balance for income withdrawal entry')
assert(!riderIncomePage.includes('summary.totalRiderIncome'), 'Rider income page must not infer withdrawable balance from cumulative income')

const riderDepositSources = [
  read('miniprogram/pages/rider/deposit/index.ts'),
  read('miniprogram/pages/rider/deposit/index.wxml')
].join('\n')
assert(!riderDepositSources.includes('baofu-withdrawal'), 'Rider deposit refund pages must stay separate from Baofoo income withdrawal')

console.log('Baofu withdrawal workflow contract check passed')
