const assert = require('assert')
const fs = require('fs')
const path = require('path')

const ROOT = path.resolve(__dirname, '..')
const REPO_ROOT = path.resolve(ROOT, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8')
}

const backendMessageTypes = read('locallife/websocket/message_types.go')
const merchantWebsocket = read('weapp/miniprogram/pages/merchant/_main_shared/utils/websocket.ts')
const merchantOrderList = read('weapp/miniprogram/pages/merchant/orders/list/index.ts')
const merchantOrderDetail = read('weapp/miniprogram/pages/merchant/orders/detail/index.ts')
const merchantKitchenBoard = read('weapp/miniprogram/pages/merchant/kitchen/index.ts')
const merchantKitchenDetail = read('weapp/miniprogram/pages/merchant/kitchen/detail/index.ts')

assert(
  backendMessageTypes.includes('MessageTypeOrderUpdate') && backendMessageTypes.includes('"order_update"'),
  'backend websocket message type constants must declare order_update'
)

assert(
  merchantWebsocket.includes("ORDER_UPDATE = 'order_update'"),
  'merchant Mini Program websocket enum must expose backend order_update'
)

assert(
  merchantOrderList.includes('WSMessageType.ORDER_UPDATE'),
  'merchant order list must subscribe to order_update websocket messages'
)
assert(
  merchantOrderList.includes('handleRealtimeOrderUpdate'),
  'merchant order list must route order_update through a dedicated refresh handler'
)

assert(
  merchantOrderDetail.includes('WSMessageType.ORDER_UPDATE'),
  'merchant order detail must subscribe to order_update websocket messages'
)
assert(
  merchantOrderDetail.includes('handleRealtimeOrderUpdate'),
  'merchant order detail must route matching order_update through a dedicated refresh handler'
)

assert(
  merchantKitchenBoard.includes('WSMessageType.ORDER_UPDATE'),
  'merchant kitchen board must subscribe to order_update websocket messages'
)
assert(
  merchantKitchenBoard.includes('handleRealtimeOrderUpdate'),
  'merchant kitchen board must route order_update through a dedicated refresh handler'
)

assert(
  merchantKitchenDetail.includes('WSMessageType.ORDER_UPDATE'),
  'merchant kitchen detail must subscribe to order_update websocket messages'
)
assert(
  merchantKitchenDetail.includes('handleRealtimeOrderUpdate'),
  'merchant kitchen detail must route matching order_update through a dedicated refresh handler'
)

console.log('check-merchant-order-update-websocket-contract: merchant order_update websocket contract is wired')
