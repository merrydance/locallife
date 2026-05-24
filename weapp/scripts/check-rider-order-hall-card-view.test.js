const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const sourcePath = path.join(__dirname, '..', 'miniprogram', 'utils', 'rider-order-hall-view.ts')

function loadModule() {
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

const { buildRecommendedOrderCardView } = loadModule()
const now = Date.parse('2026-05-24T02:00:00.000Z')

const card = buildRecommendedOrderCardView({
  order_id: 1001,
  merchant_id: 12,
  merchant_name: '朝阳小馆',
  merchant_address: '朝阳路 88 号',
  customer_address: '幸福小区 3 号楼',
  total_score: 80,
  distance_to_pickup: 840,
  real_distance: 960,
  estimated_minutes: 18,
  delivery_fee: 600,
  distance: 3200,
  pickup_longitude: 116.4,
  pickup_latitude: 39.9,
  delivery_longitude: 116.42,
  delivery_latitude: 39.92,
  expires_at: '2027-05-24T02:00:00.000Z',
  expected_delivery_at: '2026-05-24T02:20:00.000Z'
}, now)

assert.strictEqual(card.pickup_address, '朝阳路 88 号')
assert.strictEqual(card.delivery_address, '幸福小区 3 号楼')
assert.strictEqual(card.deadline_desc, '剩 20 分钟')
assert.strictEqual(card.estimated_duration, 18)
assert.strictEqual(card.pickup_distance_km, '0.8')
assert.strictEqual(card.route_distance_km, '3.2')
assert.notStrictEqual(card.deadline_desc, '剩 525600 分钟')

const fallbackCard = buildRecommendedOrderCardView({
  order_id: 1002,
  merchant_id: 13,
  total_score: 60,
  distance_to_pickup: 1000,
  estimated_minutes: 12,
  delivery_fee: 500,
  distance: 1500,
  pickup_longitude: 116.4,
  pickup_latitude: 39.9,
  delivery_longitude: 116.41,
  delivery_latitude: 39.91,
  expires_at: '2027-05-24T02:00:00.000Z'
}, now)

assert.strictEqual(fallbackCard.deadline_desc, '尽快接单')
assert.strictEqual(fallbackCard.pickup_address, '取餐地址待同步')
assert.strictEqual(fallbackCard.delivery_address, '送达地址待同步')

const zeroTimeCard = buildRecommendedOrderCardView({
  order_id: 1003,
  merchant_id: 14,
  total_score: 60,
  distance_to_pickup: 1000,
  estimated_minutes: 12,
  delivery_fee: 500,
  distance: 1500,
  pickup_longitude: 116.4,
  pickup_latitude: 39.9,
  delivery_longitude: 116.41,
  delivery_latitude: 39.91,
  expires_at: '2027-05-24T02:00:00.000Z',
  expected_delivery_at: '0001-01-01T00:00:00Z'
}, now)

assert.strictEqual(zeroTimeCard.deadline_desc, '尽快接单')

console.log('check-rider-order-hall-card-view tests passed')
