const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const rootDir = path.join(__dirname, '..')
const merchantApiPath = path.join(rootDir, 'miniprogram/api/merchant.ts')

function loadTsModule(relativePath, stubs = {}, globals = {}) {
  const sourcePath = path.join(rootDir, relativePath)
  const source = fs.readFileSync(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018,
      esModuleInterop: true,
      strict: true
    }
  }).outputText

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require: (request) => {
      if (Object.prototype.hasOwnProperty.call(stubs, request)) {
        return stubs[request]
      }
      return require(request)
    },
    console,
    ...globals
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const merchantLabelStub = {
  buildMerchantDisplayTags(systemLabels = [], tags = [], limit = 3) {
    return [...systemLabels, ...tags].slice(0, limit)
  }
}

const consumerDiscovery = loadTsModule('miniprogram/adapters/consumer-discovery.ts', {
  '../utils/image': { getPublicImageUrl: (url) => url || '' },
  './dish': { DishAdapter: { formatDistance: (distance) => `${distance}m` } },
  './merchant-labels': merchantLabelStub
})

const { ConsumerDiscoveryAdapter } = consumerDiscovery
const consumerMerchantDetail = loadTsModule('miniprogram/pages/takeout/restaurant-detail/_adapters/consumer-merchant-detail.ts', {
  '../../../../utils/image': { getPublicImageUrl: (url) => url || '' },
  '../../../../adapters/merchant-labels': merchantLabelStub
})

const { default: ConsumerMerchantDetailAdapter } = consumerMerchantDetail

function merchantSummary(overrides) {
  return {
    id: 1,
    name: '测试商户',
    logo_url: 'logo.jpg',
    estimated_delivery_fee: 300,
    total_orders: 12,
    tags: [],
    system_labels: [],
    ...overrides
  }
}

assert.strictEqual(
  ConsumerDiscoveryAdapter.toMerchantSummaryViewModel(merchantSummary({ is_open: true })).isOpen,
  true,
  'search summaries should render open only when backend explicitly says is_open=true'
)
assert.strictEqual(
  ConsumerDiscoveryAdapter.toMerchantSummaryViewModel(merchantSummary({ is_open: false })).isOpen,
  false,
  'search summaries should preserve backend is_open=false'
)
assert.strictEqual(
  ConsumerDiscoveryAdapter.toMerchantSummaryViewModel(merchantSummary({})).isOpen,
  false,
  'search summaries must not treat a missing is_open contract field as open'
)

const takeoutSupport = loadTsModule('miniprogram/utils/takeout-index-support.ts', {
  '../adapters/consumer-discovery': { __esModule: true, default: ConsumerDiscoveryAdapter },
  '../adapters/merchant-labels': merchantLabelStub,
  '../adapters/takeout-categories': { buildTakeoutCategoryGridItems: () => [] },
  '../api/location': { getActiveCategories: async () => [] },
  './logger': { logger: { info() {}, warn() {}, error() {}, debug() {} } },
  './global-store': { globalStore: { updateLocation() {} } },
  './image': { getPublicImageUrl: (url) => url || '' },
  './util': { formatPrice: (value) => `¥${(value / 100).toFixed(2)}` }
})

function merchantDetail(isOpen) {
  return {
    id: 1,
    name: '测试商户',
    phone: '13800000000',
    address: '测试地址',
    latitude: 39.9,
    longitude: 116.4,
    region_id: 1,
    is_open: isOpen,
    is_ordering_suspended: false,
    tags: [],
    monthly_sales: 12,
    avg_prep_minutes: 15
  }
}

assert.strictEqual(
  takeoutSupport.buildTakeoutMerchantMetaPatch(merchantDetail(false)).isOpen,
  false,
  'feed lite detail hydration should overwrite stale list isOpen=true when detail says closed'
)
assert.strictEqual(
  takeoutSupport.buildTakeoutMerchantMetaPatch(merchantDetail(true)).isOpen,
  true,
  'feed lite detail hydration should keep list isOpen=true when detail says open'
)

const closedMerchantDetailView = ConsumerMerchantDetailAdapter.toViewModel(merchantDetail(false))
assert.strictEqual(
  closedMerchantDetailView.ordering_status_label,
  '暂停接单',
  'merchant info should not display normal ordering while the merchant is closed'
)
assert.strictEqual(
  closedMerchantDetailView.ordering_status_tone,
  'closed',
  'closed merchants should render ordering status with the closed tone'
)

let capturedComponent = null
const toastCalls = []
const navigateCalls = []
loadTsModule('miniprogram/components/merchant-feed-card/index.ts', {
  '../../utils/image': {
    formatImageUrl: (url) => url,
    ImageSize: { CARD: 'card' }
  }
}, {
  Component(config) {
    capturedComponent = config
  },
  wx: {
    showToast(payload) {
      toastCalls.push(payload)
    },
    navigateTo(payload) {
      navigateCalls.push(payload)
    }
  }
})

assert(capturedComponent, 'merchant feed card component should register itself')

let triggered = false
const closedCardContext = {
  data: {
    merchant: {
      id: 1,
      name: '测试商户',
      isOpen: false,
      isOrderingSuspended: false,
      featuredDishes: [{ id: 99, merchantId: 1 }]
    }
  },
  triggerEvent() {
    triggered = true
  },
  ...capturedComponent.methods
}

capturedComponent.methods.onDishTap.call(closedCardContext, { currentTarget: { dataset: { index: 0 } } })
capturedComponent.methods.onDishAdd.call(closedCardContext, { currentTarget: { dataset: { index: 0 } } })
capturedComponent.methods.onSelectSpec.call(closedCardContext, { currentTarget: { dataset: { index: 0 } } })

assert.strictEqual(navigateCalls.length, 0, 'closed merchants should not navigate from feed dish taps')
assert.strictEqual(triggered, false, 'closed merchants should not emit feed add-cart events')
assert(
  toastCalls.some((payload) => payload.title === '商户休息中～'),
  'closed feed dish actions should explain that the merchant is resting'
)

const feedCardWxml = fs.readFileSync(path.join(rootDir, 'miniprogram/components/merchant-feed-card/index.wxml'), 'utf8')
assert(
  feedCardWxml.includes('merchant.isOpen === false') && feedCardWxml.includes('休息中'),
  'feed card should keep the closed-state tag when merchant is closed'
)
assert(
  !feedCardWxml.includes('该商户当前暂未营业，可进入店铺查看信息。'),
  'feed card should not repeat closed-state explanatory copy below the merchant header'
)
assert(
  !feedCardWxml.includes('当前暂停接单') && !feedCardWxml.includes('该商户暂时无法下外卖单'),
  'feed card should keep paused ordering as a compact tag instead of a repeated explanatory panel'
)
assert(
  feedCardWxml.includes('class="dish-preview-section" wx:if="{{merchant.isOpen !== false && !merchant.isOrderingSuspended}}"'),
  'feed card should hide dish previews when the merchant is closed or ordering is suspended'
)

const restaurantDetailWxml = fs.readFileSync(path.join(rootDir, 'miniprogram/pages/takeout/restaurant-detail/index.wxml'), 'utf8')
assert(
  restaurantDetailWxml.includes('商家休息中，暂不可下单'),
  'restaurant detail should use concise closed-state copy'
)
assert(
  !restaurantDetailWxml.includes('该店铺当前支持浏览门店信息与经营内容，暂不支持下单或预订。'),
  'restaurant detail should not render long closed-state explanatory copy'
)

const merchantApiSource = fs.readFileSync(merchantApiPath, 'utf8')
const searchMerchantBlock = merchantApiSource.match(/export async function searchMerchantsWithMeta[\s\S]*?\n}\n/)
const detailMerchantBlock = merchantApiSource.match(/export async function getPublicMerchantDetail[\s\S]*?\n}/)
assert(searchMerchantBlock, 'merchant API should expose searchMerchantsWithMeta')
assert(detailMerchantBlock, 'merchant API should expose getPublicMerchantDetail')
assert(
  !/useCache:\s*true/.test(searchMerchantBlock[0]),
  'merchant search should not cache dynamic open-state data'
)
assert(
  !/useCache:\s*true/.test(detailMerchantBlock[0]),
  'public merchant detail should not cache dynamic open-state data'
)

console.log('takeout merchant open-state tests passed')
