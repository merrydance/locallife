const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const repoRoot = path.join(__dirname, '..')
const sourcePath = path.join(repoRoot, 'miniprogram', 'utils', 'request-core.ts')
const requestPath = path.join(repoRoot, 'miniprogram', 'utils', 'request.ts')

function plain(value) {
  return JSON.parse(JSON.stringify(value))
}

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
    JSON,
    Number,
    String
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const core = loadModule()

assert.deepStrictEqual(plain(core.sanitizeGetParams({
  keepNumber: 0,
  keepBoolean: false,
  keepText: ' 上海 ',
  keepArray: [1, 2],
  dropEmpty: '',
  dropBlank: '   ',
  dropUndefinedText: 'undefined',
  dropNullText: 'NULL',
  dropNanText: 'NaN',
  dropUndefined: undefined,
  dropNull: null,
  dropInfinity: Infinity
})), {
  keepNumber: 0,
  keepBoolean: false,
  keepText: '上海',
  keepArray: [1, 2]
})
assert.strictEqual(core.sanitizeGetParams(undefined), undefined)
assert.deepStrictEqual(plain(core.sanitizeGetParams(['keep'])), ['keep'])

assert.strictEqual(
  core.buildGetSingleFlightKey('/v1/orders', { status: 'paid' }, false),
  'GET|/v1/orders|{"status":"paid"}|skipAuth:0'
)
assert.strictEqual(
  core.buildGetSingleFlightKey('/v1/orders', { status: 'paid' }, true),
  'GET|/v1/orders|{"status":"paid"}|skipAuth:1'
)
const circular = {}
circular.self = circular
assert.strictEqual(
  core.buildGetSingleFlightKey('/v1/circular', circular, false),
  'GET|/v1/circular|[object Object]|skipAuth:0'
)

assert.strictEqual(core.isRecord({}), true)
assert.strictEqual(core.isRecord([]), true)
assert.strictEqual(core.isRecord(null), false)
assert.strictEqual(core.isRecord('text'), false)

const requestSource = fs.readFileSync(requestPath, 'utf8')
assert(requestSource.includes("from './request-core'"), 'request.ts must consume request-core owner')
for (const pattern of [
  'function sanitizeGetParams',
  'function buildGetSingleFlightKey',
  'function isRecord'
]) {
  assert(!requestSource.includes(pattern), `request.ts must not own ${pattern}`)
}

console.log('check-request-core tests passed')
