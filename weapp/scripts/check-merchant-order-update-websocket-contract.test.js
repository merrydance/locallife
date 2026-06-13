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
const merchantKitchenBoardWxml = read('weapp/miniprogram/pages/merchant/kitchen/index.wxml')
const merchantKitchenBoardJson = read('weapp/miniprogram/pages/merchant/kitchen/index.json')
const merchantKitchenDetail = read('weapp/miniprogram/pages/merchant/kitchen/detail/index.ts')

function extractSection(source, startMarker, endMarker) {
  const start = source.indexOf(startMarker)
  assert(start >= 0, `Expected source to contain ${startMarker}`)
  const end = source.indexOf(endMarker, start)
  assert(end > start, `Expected source to contain ${endMarker} after ${startMarker}`)
  return source.slice(start, end)
}

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
  merchantKitchenBoardJson.includes('"t-notice-bar": "tdesign-miniprogram/notice-bar/notice-bar"'),
  'merchant kitchen board must register a notice bar for visible realtime degradation'
)
assert(
  merchantKitchenBoardWxml.includes('boardRealtimeErrorMessage') &&
    merchantKitchenBoardWxml.includes('<t-notice-bar') &&
    merchantKitchenBoardWxml.includes('content="{{boardRealtimeErrorMessage}}"'),
  'merchant kitchen board must render realtime degradation in a visible notice bar'
)

const kitchenRefreshRealtimeRuntime = extractSection(
  merchantKitchenBoard,
  'async refreshRealtimeRuntime()',
  '\n\n  handleRealtimeOrderUpdate'
)
const kitchenApplyMerchantOpenStatus = extractSection(
  merchantKitchenBoard,
  'applyMerchantOpenStatus(isOpen: boolean)',
  '\n\n  async refreshRealtimeRuntime'
)
const kitchenLoadKitchenOrders = extractSection(
  merchantKitchenBoard,
  'async loadKitchenOrders(showLoading = true)',
  '\n\n  onRetryBoard'
)
const kitchenRunKitchenAction = extractSection(
  merchantKitchenBoard,
  'async runKitchenAction(',
  '\n  }\n})'
)
assert(
  kitchenRefreshRealtimeRuntime.includes('const status = await getMyMerchantOpenStatus()'),
  'merchant kitchen board must refresh backend open-status truth before starting realtime'
)
assert(
  kitchenRefreshRealtimeRuntime.includes("logger.warn('Load merchant open status for kitchen realtime failed', err)"),
  'merchant kitchen board must log open-status refresh failure before degrading realtime'
)
assert(
  kitchenRefreshRealtimeRuntime.includes("boardRealtimeErrorMessage: '后厨实时同步暂不可用，请下拉刷新或稍后重试'"),
  'merchant kitchen board must expose a visible realtime degradation message when open-status refresh fails'
)
assert(
  kitchenRefreshRealtimeRuntime.includes('this.stopRealtimeRuntime({ disconnect: true })'),
  'merchant kitchen board must disconnect realtime when open-status refresh cannot be trusted'
)
assert(
  kitchenApplyMerchantOpenStatus.includes('boardRealtimeErrorMessage: isOpen ?') &&
    kitchenApplyMerchantOpenStatus.includes("'当前门店已打烊，后厨实时订单已暂停'"),
  'merchant kitchen board must clear realtime degradation only after trusted open-status recovery or replace it with closed-state copy'
)
assert(
  !kitchenLoadKitchenOrders.includes('boardRealtimeErrorMessage'),
  'merchant kitchen order refresh must not clear realtime degradation state'
)
assert(
  !kitchenRunKitchenAction.includes('boardRealtimeErrorMessage'),
  'merchant kitchen order actions must not clear realtime degradation state'
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
