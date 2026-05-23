const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const sourcePath = path.join(__dirname, '..', 'miniprogram', 'services', 'map.ts')

function loadMapService() {
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
      if (modulePath === '../utils/logger') {
        return {
          logger: {
            warn() {},
            error() {},
            info() {},
            debug() {}
          }
        }
      }
      if (modulePath === '../utils/request') {
        return { request: () => Promise.resolve({}) }
      }
      if (modulePath === '../utils/location') {
        return { locationService: {} }
      }
      throw new Error(`unexpected require: ${modulePath}`)
    },
    wx: {
      createMapContext() {
        return { includePoints() {} }
      }
    },
    Promise,
    Error,
    Number,
    Math
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports.mapService
}

function testDurationNeverUsesSecondsForEtaCopy() {
  const mapService = loadMapService()

  assert.strictEqual(mapService.formatDuration(0), '不足1分钟')
  assert.strictEqual(mapService.formatDuration(29), '不足1分钟')
  assert.strictEqual(mapService.formatDuration(59), '不足1分钟')
  assert.strictEqual(mapService.formatDuration(60), '1分钟')
  assert.strictEqual(mapService.formatDuration(89), '1分钟')
  assert.strictEqual(mapService.formatDuration(90), '2分钟')
  assert.strictEqual(mapService.formatDuration(3600), '1小时')
  assert.strictEqual(mapService.formatDuration(3660), '1小时1分钟')
}

testDurationNeverUsesSecondsForEtaCopy()
console.log('check-map-duration-format tests passed')
