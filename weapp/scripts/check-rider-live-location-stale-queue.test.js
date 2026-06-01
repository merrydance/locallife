const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const sourcePath = path.join(__dirname, '..', 'miniprogram', 'pages', 'rider', '_utils', 'rider-live-location.ts')
const storageKey = 'rider_live_location_queue_v1'
const fixedNowMs = Date.parse('2026-05-27T09:15:00.000Z')

class FixedDate extends Date {
  constructor(...args) {
    super(...(args.length > 0 ? args : [fixedNowMs]))
  }

  static now() {
    return fixedNowMs
  }

  static parse(value) {
    return Date.parse(value)
  }

  static UTC(...args) {
    return Date.UTC(...args)
  }
}

function plain(value) {
  return JSON.parse(JSON.stringify(value))
}

function minutesAgo(minutes) {
  return new Date(fixedNowMs - minutes * 60 * 1000).toISOString()
}

function loadSession(storedQueue) {
  const source = fs.readFileSync(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018
    }
  }).outputText

  const storage = { [storageKey]: plain(storedQueue) }
  const updateLocationCalls = []

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (modulePath === '../_main_shared/api/rider') {
        return {
          __esModule: true,
          default: {
            async updateLocation(regionId, locations) {
              updateLocationCalls.push({ regionId, locations: plain(locations) })
              return { message: '位置更新成功' }
            }
          }
        }
      }
      if (modulePath === '../../../utils/network-monitor') {
        return {
          networkMonitor: {
            subscribe() {},
            isOnline() {
              return true
            }
          }
        }
      }
      if (modulePath === '../../../utils/location') {
        return { locationService: { getCurrentLocation: async () => ({ latitude: 1, longitude: 2 }) } }
      }
      if (modulePath === '../../../utils/logger') {
        return {
          logger: {
            warn() {},
            info() {},
            error() {},
            debug() {}
          }
        }
      }
      if (modulePath === '../_main_shared/utils/rider-location') {
        return { normalizeLocationError: (error) => error instanceof Error ? error : new Error(String(error)) }
      }
      if (modulePath === '../_main_shared/utils/current-region') {
        return { resolveCurrentRegionId: async () => 596 }
      }
      throw new Error(`unexpected require: ${modulePath}`)
    },
    wx: {
      getStorageSync(key) {
        return storage[key]
      },
      setStorageSync(key, value) {
        storage[key] = plain(value)
      }
    },
    setInterval() {
      return 1
    },
    clearInterval() {},
    Date: FixedDate,
    Math,
    Number,
    Promise,
    Error,
    Set
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })

  return {
    session: sandbox.module.exports.riderLiveLocationSession,
    updateLocationCalls,
    storage
  }
}

async function testFlushDropsStalePointsBeforeUpload() {
  const stalePoint = {
    delivery_id: 101,
    latitude: 39.9,
    longitude: 116.4,
    recorded_at: minutesAgo(61),
    source: 'live_tracking'
  }
  const freshPoint = {
    delivery_id: 101,
    latitude: 39.91,
    longitude: 116.41,
    recorded_at: minutesAgo(2),
    source: 'live_tracking'
  }
  const { session, updateLocationCalls } = loadSession([stalePoint, freshPoint])
  session.state.activeDeliveryId = 101
  session.state.isRunning = true

  await session.flushNow()

  assert.strictEqual(updateLocationCalls.length, 1)
  assert.strictEqual(updateLocationCalls[0].regionId, 596)
  assert.deepStrictEqual(updateLocationCalls[0].locations, [freshPoint])
}

async function testFlushClearsAllStalePointsWithoutUploading() {
  const stalePoint = {
    delivery_id: 101,
    latitude: 39.9,
    longitude: 116.4,
    recorded_at: minutesAgo(90),
    source: 'live_tracking'
  }
  const { session, updateLocationCalls, storage } = loadSession([stalePoint])
  session.state.activeDeliveryId = 101
  session.state.isRunning = true

  await session.flushNow()

  assert.strictEqual(updateLocationCalls.length, 0)
  assert.deepStrictEqual(storage[storageKey], [])
  assert.strictEqual(session.state.pendingCount, 0)
}

async function main() {
  await testFlushDropsStalePointsBeforeUpload()
  await testFlushClearsAllStalePointsWithoutUploading()
  console.log('check-rider-live-location-stale-queue tests passed')
}

main().catch((error) => {
  console.error(error)
  process.exit(1)
})
