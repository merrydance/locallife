const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const rootDir = path.join(__dirname, '..')
const pageRelativePath = 'miniprogram/pages/takeout/wanted-merchants/index.ts'
const pagePath = path.join(rootDir, pageRelativePath)

const pageSource = fs.readFileSync(pagePath, 'utf8')

assert(
  !pageSource.includes('onRefresh') &&
    !pageSource.includes('refreshing:') &&
    !pageSource.includes('icon="refresh"'),
  'wanted merchant leaderboard must not expose manual refresh controls; use polling, pull-down refresh, or action-driven reloads'
)

function loadWantedMerchantPage(listWantedMerchants) {
  const compiled = ts.transpileModule(pageSource, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018,
      esModuleInterop: true,
      strict: true
    }
  }).outputText

  let capturedPage = null
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require: (request) => {
      if (request === '../../../api/wanted-merchant') {
        return {
          listWantedMerchants,
          submitWantedMerchant: async () => ({ result: 'created', wanted_merchant_id: 1 }),
          voteWantedMerchant: async () => ({ result: 'voted' })
        }
      }
      if (request === '../../../api/location') {
        return { getCurrentRegion: async () => ({ region_id: 2185, region_name: '测试区县' }) }
      }
      if (request === '../../../utils/navigation') {
        return { __esModule: true, default: { toRestaurantDetail() {} } }
      }
      if (request === '../../../utils/logger') {
        return { logger: { debug() {}, info() {}, warn() {}, error() {} } }
      }
      if (request === '../../../utils/responsive') {
        return { getStableBarHeights: () => ({ navBarHeight: 88 }) }
      }
      if (request === '../../../utils/global-store') {
        return { globalStore: { set() {} } }
      }
      return require(request)
    },
    Page(config) {
      capturedPage = config
    },
    getApp() {
      return {
        globalData: {
          currentRegion: { id: 2185, name: '测试区县' },
          latitude: 37.637098,
          longitude: 114.914107
        }
      }
    },
    wx: {
      showToast() {},
      chooseLocation() {},
      stopPullDownRefresh() {}
    },
    setTimeout,
    clearTimeout,
    setInterval,
    clearInterval,
    console
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: pagePath })
  assert(capturedPage, 'wanted merchant page should register itself')
  return capturedPage
}

let resolveLeaderboard
let listCallCount = 0
const page = loadWantedMerchantPage(() => {
  listCallCount += 1
  return new Promise((resolve) => {
    resolveLeaderboard = () => resolve({
      items: [],
      total: 0,
      page_id: 1,
      page_size: 50,
      total_pages: 0
    })
  })
})

const ctx = {
  ...page,
  data: {
    ...JSON.parse(JSON.stringify(page.data)),
    regionId: 2185,
    items: []
  },
  setData(patch) {
    Object.assign(this.data, patch)
  },
  focusWantedMerchant() {}
}

const firstLoad = page.loadLeaderboard.call(ctx)
const secondLoad = page.loadLeaderboard.call(ctx)

assert.strictEqual(listCallCount, 1, 'leaderboard should single-flight duplicate page-level loads')

resolveLeaderboard()

Promise.all([firstLoad, secondLoad]).then(() => {
  assert.strictEqual(listCallCount, 1, 'duplicate leaderboard loads should stay coalesced until completion')
  console.log('check-wanted-merchant-request-budget tests passed')
}).catch((error) => {
  console.error(error)
  process.exit(1)
})
