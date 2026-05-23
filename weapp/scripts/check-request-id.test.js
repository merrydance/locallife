const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const sourcePath = path.join(__dirname, '..', 'miniprogram', 'utils', 'request-id.ts')

function loadRequestIdModule() {
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
    Date
  }

  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const originalDateNow = Date.now

try {
  const { buildDefaultRequestId } = loadRequestIdModule()

  Date.now = () => 1760000000000

  const requestIds = Array.from({ length: 8 }, () => buildDefaultRequestId('POST', '/v1/cart/calculate'))

  assert.strictEqual(new Set(requestIds).size, requestIds.length)
  requestIds.forEach((requestId) => {
    assert(requestId.startsWith('POST_/v1/cart/calculate_1760000000000_'))
  })
} finally {
  Date.now = originalDateNow
}

console.log('check-request-id tests passed')
