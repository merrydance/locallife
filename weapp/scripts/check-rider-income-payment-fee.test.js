const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const ROOT = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(ROOT, relativePath), 'utf8')
}

function loadRiderIncomeModule() {
  const sourcePath = path.join(ROOT, 'miniprogram', 'pages', 'rider', '_services', 'rider-income.ts')
  const source = fs.readFileSync(sourcePath, 'utf8')
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
      if (modulePath === '../../../utils/logger') {
        return { logger: { warn() {}, error() {} } }
      }
      if (modulePath === '../_api/rider-income') {
        return { riderIncomeApi: {} }
      }
      if (modulePath === '../_main_shared/api/baofu-withdrawal') {
        return { getBaofuWithdrawalBalance: async () => null }
      }
      if (modulePath === '../_main_shared/api/rider') {
        return { default: { getStatus: async () => ({}) } }
      }
      if (modulePath === '../_main_shared/services/baofu-withdrawal-workflow') {
        return {
          buildBaofuWithdrawalBalanceView() {
            return {
              canSubmit: false,
              availableAmountText: '¥0.00',
              disabledReason: '',
              statusDesc: ''
            }
          }
        }
      }
      throw new Error(`unexpected require: ${modulePath}`)
    },
    Date,
    Math,
    Number,
    String
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const riderIncomeApi = read('miniprogram/pages/rider/_api/rider-income.ts')
const riderIncomeService = read('miniprogram/pages/rider/_services/rider-income.ts')
const riderIncomeWxml = read('miniprogram/pages/rider/income/index.wxml')

assert.match(riderIncomeApi, /rider_payment_fee\s*:\s*number/, 'rider income ledger API type must expose rider_payment_fee')
assert.match(riderIncomeService, /riderPaymentFeeDisplay\s*:\s*string/, 'rider income ledger view must expose formatted rider payment fee')
assert.ok(riderIncomeWxml.includes('支付通道费'), 'rider income ledger card must render payment channel fee label')
assert.ok(riderIncomeWxml.includes('{{item.riderPaymentFeeDisplay}}'), 'rider income ledger card must render formatted rider payment fee value')

const { buildRiderIncomeLedgerItemView } = loadRiderIncomeModule()
const view = buildRiderIncomeLedgerItemView({
  id: 1,
  payment_order_id: 2,
  merchant_id: 3,
  order_id: 4,
  order_no: 'ORDER202605270001',
  merchant_name: '测试商户',
  status: 'finished',
  total_amount: 5000,
  delivery_fee: 800,
  rider_gross_amount: 800,
  rider_payment_fee: 5,
  rider_amount: 795,
  distributable_amount: 4200,
  out_order_no: 'PS202605270001',
  sharing_order_id: 'BCTPS001',
  finished_at: '2026-05-27T11:30:00Z',
  created_at: '2026-05-27T10:30:00Z'
})

assert.strictEqual(view.riderPaymentFeeDisplay, '¥0.05')
assert.strictEqual(view.deliveryFeeDisplay, '¥8.00')
assert.strictEqual(view.riderAmountDisplay, '¥7.95')

console.log('check-rider-income-payment-fee tests passed')
