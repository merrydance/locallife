const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const repoRoot = path.resolve(__dirname, '..')
const sourcePath = path.join(repoRoot, 'miniprogram/pages/orders/_utils/customer-order-status-view.ts')
const orderCardPath = path.join(repoRoot, 'miniprogram/pages/orders/_adapters/order-card.ts')
const orderAdapterPath = path.join(repoRoot, 'miniprogram/pages/orders/_adapters/order.ts')
const timelinePath = path.join(repoRoot, 'miniprogram/pages/orders/_utils/timeline.ts')
const detailPath = path.join(repoRoot, 'miniprogram/pages/orders/detail/index.ts')
const trackingPath = path.join(repoRoot, 'miniprogram/pages/orders/tracking/index.ts')
const listWxssPath = path.join(repoRoot, 'miniprogram/pages/orders/list/index.wxss')

function read(filePath) {
  return fs.readFileSync(filePath, 'utf8')
}

function loadStatusViewModule() {
  const source = read(sourcePath)
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
      if (modulePath === '../_main_shared/api/order') {
        return {}
      }
      if (modulePath === '../_main_shared/api/delivery') {
        return {}
      }
      throw new Error(`unexpected require: ${modulePath}`)
    }
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

function baseTakeoutOrder(status) {
  return {
    id: 101,
    order_no: 'ORD101',
    user_id: 1,
    merchant_id: 2,
    merchant_name: '测试商户',
    status,
    fulfillment_status: status === 'ready' ? 'ready' : 'preparing',
    order_type: 'takeout',
    subtotal: 1800,
    total_amount: 2000,
    delivery_fee: 200,
    delivery_fee_discount: 0,
    discount_amount: 0,
    created_at: '2026-06-01T10:00:00+08:00'
  }
}

function baseDelivery(status, orderStatus) {
  return {
    id: 301,
    order_id: 101,
    status,
    order_status: orderStatus,
    fulfillment_status: orderStatus === 'ready' ? 'ready' : 'preparing',
    pickup_address: '商家地址',
    pickup_longitude: 120,
    pickup_latitude: 30,
    delivery_address: '顾客地址',
    delivery_longitude: 120.01,
    delivery_latitude: 30.01,
    delivery_fee: 200,
    rider_earnings: 180
  }
}

function assertSourceImportsSharedView(filePath) {
  const source = read(filePath)
  assert(
    source.includes('customer-order-status-view'),
    `${path.relative(repoRoot, filePath)} must import the shared customer order status view`
  )
}

function main() {
  const {
    buildCustomerOrderStatusView,
    buildCustomerDeliveryTrackingStatusView
  } = loadStatusViewModule()

  const readyView = buildCustomerOrderStatusView(baseTakeoutOrder('ready'))
  assert.strictEqual(readyView.label, '等待跑腿接单')
  assert.strictEqual(readyView.group, 'ready')
  assert.strictEqual(readyView.className, 'class-ready')
  assert.strictEqual(readyView.canTrack, false)

  const acceptedView = buildCustomerOrderStatusView(baseTakeoutOrder('courier_accepted'))
  assert.strictEqual(acceptedView.label, '骑手已接单')
  assert.strictEqual(acceptedView.group, 'delivering')
  assert.strictEqual(acceptedView.canTrack, true)

  const pickedView = buildCustomerOrderStatusView(baseTakeoutOrder('picked'))
  assert.strictEqual(pickedView.label, '骑手已取餐')
  assert.strictEqual(pickedView.group, 'delivering')
  assert.strictEqual(pickedView.canTrack, true)

  const riderDeliveredView = buildCustomerOrderStatusView(baseTakeoutOrder('rider_delivered'))
  assert.strictEqual(riderDeliveredView.label, '已送达待确认')
  assert.strictEqual(riderDeliveredView.group, 'delivering')
  assert.strictEqual(riderDeliveredView.canTrack, true)

  const trackingAssigned = buildCustomerDeliveryTrackingStatusView(baseDelivery('assigned', 'courier_accepted'))
  assert.strictEqual(trackingAssigned.label, acceptedView.label)
  assert.strictEqual(trackingAssigned.group, acceptedView.group)
  assert.strictEqual(trackingAssigned.shouldPoll, true)

  const trackingDelivered = buildCustomerDeliveryTrackingStatusView(baseDelivery('delivered', 'rider_delivered'))
  assert.strictEqual(trackingDelivered.label, riderDeliveredView.label)
  assert.strictEqual(trackingDelivered.canConfirmReceipt, true)
  assert.strictEqual(trackingDelivered.shouldPoll, false)

  assertSourceImportsSharedView(orderCardPath)
  assertSourceImportsSharedView(orderAdapterPath)
  assertSourceImportsSharedView(timelinePath)
  assertSourceImportsSharedView(detailPath)
  assertSourceImportsSharedView(trackingPath)
  assert(
    read(orderCardPath).includes('order.status_hint') && read(orderCardPath).includes('order.table_id'),
    'order list highlight must preserve backend hints and non-takeout context'
  )
  assert(
    read(timelinePath).includes("case 'user_delivered'"),
    'order detail timeline must cover user_delivered status'
  )
  assert(
    read(listWxssPath).includes('.status-tag.class-ready'),
    'order list must style the shared ready status class'
  )

  console.log('check-customer-order-status-view: shared customer order status view is consistent')
}

main()
