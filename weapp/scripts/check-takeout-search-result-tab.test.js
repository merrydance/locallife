const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const rootDir = path.join(__dirname, '..')

function loadTsModule(relativePath, stubs = {}) {
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
    require: (requestPath) => {
      if (Object.prototype.hasOwnProperty.call(stubs, requestPath)) {
        return stubs[requestPath]
      }
      return require(requestPath)
    },
    console
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const pageSource = fs.readFileSync(path.join(rootDir, 'miniprogram/pages/takeout/search/index.ts'), 'utf8')
const { chooseTakeoutSearchResultTab } = loadTsModule('miniprogram/utils/takeout-search-result-tab.ts')

assert.strictEqual(
  chooseTakeoutSearchResultTab({ dishCount: 0, merchantCount: 2 }),
  'merchants',
  'restaurant-name search should show merchant results when dish results are empty'
)
assert.strictEqual(
  chooseTakeoutSearchResultTab({ dishCount: 3, merchantCount: 1 }),
  'dishes',
  'dish results remain the first result tab when present'
)
assert.strictEqual(
  chooseTakeoutSearchResultTab({ dishCount: 0, merchantCount: 0 }),
  'dishes',
  'empty searches should keep the default dish result tab'
)
assert(
  pageSource.includes("import { chooseTakeoutSearchResultTab } from '../../../utils/takeout-search-result-tab'"),
  'takeout search page should use the shared result-tab helper'
)
assert(
  pageSource.includes('activeResultTab: chooseTakeoutSearchResultTab({'),
  'takeout search page should set activeResultTab from result counts after each search'
)
assert(
  pageSource.includes('debounceTimer = setTimeout(() => this.doSearch(keyword.trim()), DEBOUNCE_MS)'),
  'typing in takeout search should debounce the real combined search instead of stopping in suggestions state'
)
assert(
  !pageSource.includes('showSuggestions: true'),
  'takeout search should not switch typed input into a suggestions-only state'
)

;(async () => {
  let merchantSearchParams = null
  const merchant = {
    id: 18,
    name: '棉香小厨',
    address: '棉花巷 18 号',
    logo_url: '',
    status: 'approved',
    is_open: true,
    region_id: 1,
    tags: []
  }

  const { unifiedSearch } = loadTsModule('miniprogram/api/search.ts', {
    '../utils/request': {
      request(options) {
        if (options.url === '/v1/search/dishes') {
          throw new Error('dish search unavailable')
        }
        if (options.url === '/v1/search/merchants') {
          throw new Error('unifiedSearch should use the merchant search adapter')
        }
        throw new Error(`unexpected request: ${options.url}`)
      }
    },
    '../utils/logger': {
      logger: { debug() {}, error() {} }
    },
    '../utils/promise': {
      settleAll: (promises) => Promise.all(promises.map((promise) => Promise.resolve(promise).then(
        (value) => ({ status: 'fulfilled', value }),
        (reason) => ({ status: 'rejected', reason })
      )))
    },
    './merchant': {
      getRecommendedMerchantsWithMeta: async () => ({ merchants: [], total: 0, page: 1, pageSize: 20, hasMore: false }),
      searchMerchantsWithMeta: async (params) => {
        merchantSearchParams = params
        return {
          merchants: [merchant],
          items: [merchant],
          total: 1,
          page: 1,
          pageSize: params.page_size,
          hasMore: false
        }
      }
    },
    './types': {
      normalizePaginatedResult: (items, response, fallback) => ({
        items,
        total: response?.total ?? items.length,
        page: response?.page ?? response?.page_id ?? fallback.page,
        pageSize: response?.page_size ?? fallback.pageSize,
        hasMore: false
      })
    }
  })

  const result = await unifiedSearch('棉', {
    dish_limit: 20,
    merchant_limit: 20,
    user_latitude: 39.9,
    user_longitude: 116.4
  })

  assert.strictEqual(result.dishes.length, 0, 'dish search failure should not hide matching restaurants')
  assert.strictEqual(result.merchants.length, 1, 'restaurant-name search should still show merchant results')
  assert.strictEqual(result.merchants[0].name, '棉香小厨')
  assert.strictEqual(result.total_merchants, 1)
  assert(merchantSearchParams, 'unifiedSearch should share the same merchant search adapter path as reservation search')
  assert.strictEqual(merchantSearchParams.keyword, '棉')
  assert.strictEqual(merchantSearchParams.page_id, 1)
  assert.strictEqual(merchantSearchParams.page_size, 20)
  assert.strictEqual(merchantSearchParams.user_latitude, 39.9)
  assert.strictEqual(merchantSearchParams.user_longitude, 116.4)

  console.log('takeout search result tab tests passed')
})().catch((error) => {
  console.error(error)
  process.exit(1)
})
