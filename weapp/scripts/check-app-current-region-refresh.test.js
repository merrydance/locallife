const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const rootDir = path.join(__dirname, '..')
const appPath = path.join(rootDir, 'miniprogram', 'app.ts')
const appSource = fs.readFileSync(appPath, 'utf8')

const compiled = ts.transpileModule(appSource, {
  compilerOptions: {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2018,
    esModuleInterop: true
  }
}).outputText

let appConfig = null
let currentRegionCallCount = 0
const currentRegionUpdates = []
const locationUpdates = []

function plain(value) {
  return JSON.parse(JSON.stringify(value))
}

const sandbox = {
  exports: {},
  module: { exports: {} },
  require: (request) => {
    if (request === './utils/tracker') {
      return { tracker: { log() {} }, EventType: { APP_OPEN: 'APP_OPEN' } }
    }
    if (request === './utils/logger') {
      return { logger: { debug() {}, info() {}, warn() {}, error() {} } }
    }
    if (request === './utils/error-handler') {
      return { AppError: class AppError extends Error {}, ErrorHandler: {}, ErrorType: {} }
    }
    if (request === './utils/user-facing') {
      return { getErrorDebugMessage: () => '', isRetryableNetworkError: () => false }
    }
    if (request === './utils/prompt-feedback') {
      return { installPromptFeedbackGuards() {} }
    }
    if (request === './utils/wechat-login-session') {
      return { ensureWechatLoginSession: async () => {} }
    }
    if (request === './utils/native-diagnostics') {
      return {
        getNativeOperationDiagnostics: () => ({}),
        markNativeOperationStart: () => () => {}
      }
    }
    if (request === './utils/geo') {
      return { haversineDistance: () => 0.01 }
    }
    if (request === './utils/auth') {
      return { getToken: () => 'token' }
    }
    if (request === './api/location') {
      return {
        getCurrentRegion: async () => {
          currentRegionCallCount += 1
          return { region_id: 2185, region_name: '测试区县' }
        }
      }
    }
    if (request === './utils/location') {
      return {
        locationService: {
          reverseGeocode: async () => ({
            address: '测试地址',
            formatted_address: '测试地址',
            district: '测试区县'
          })
        }
      }
    }
    if (request === './utils/global-store') {
      return {
        globalStore: {
          set(key, value) {
            if (key === 'currentRegion') currentRegionUpdates.push(value)
          },
          updateLocation(latitude, longitude, name, address) {
            locationUpdates.push({ latitude, longitude, name, address })
          }
        }
      }
    }
    if (request === './utils/responsive') {
      return { getGlobalLayoutData: () => ({ navBarHeight: 88 }), getStableBarHeights: () => ({ navBarHeight: 88 }) }
    }
    return require(request)
  },
  App(config) {
    appConfig = config
  },
  wx: {
    canIUse: () => false,
    getStorageSync: () => null,
    setStorageSync() {},
    getLocation() {},
    showModal() {},
    openSetting() {}
  },
  setTimeout,
  clearTimeout,
  Date,
  Math,
  JSON,
  console
}

sandbox.exports = sandbox.module.exports
vm.runInNewContext(compiled, sandbox, { filename: appPath })
assert(appConfig, 'app.ts should register App config')

appConfig.globalData.latitude = 37.6371
appConfig.globalData.longitude = 114.9141
appConfig.globalData.currentRegion = undefined
appConfig.globalData._lastLocationContext = {
  lat: 37.63709,
  lng: 114.91409,
  time: Date.now(),
  name: '缓存位置',
  address: '缓存地址'
}

appConfig.reverseGeocodeWhenReady.call(appConfig).then(() => {
  assert.strictEqual(currentRegionCallCount, 1, 'current region should refresh before small-move cache shortcut')
  assert.deepStrictEqual(plain(currentRegionUpdates), [{ id: 2185, name: '测试区县' }])
  assert.strictEqual(locationUpdates.length, 1, 'location name can still use small-move cache shortcut')
  console.log('check-app-current-region-refresh tests passed')
}).catch((error) => {
  console.error(error)
  process.exit(1)
})
