const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const sourcePath = path.join(__dirname, '..', 'miniprogram/pages/dine-in/scan-entry/entry-params.ts')

assert(fs.existsSync(sourcePath), 'scan-entry params parser module must exist')

const source = fs.readFileSync(sourcePath, 'utf8')
const transpiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2020
  }
})

const moduleScope = { exports: {} }
vm.runInNewContext(transpiled.outputText, {
  module: moduleScope,
  exports: moduleScope.exports,
  URL,
  decodeURIComponent,
  parseInt,
  Number
}, { filename: 'entry-params.js' })

const { parseScene } = moduleScope.exports
assert.strictEqual(typeof parseScene, 'function', 'parseScene must be exported')

function normalize(value) {
  return JSON.parse(JSON.stringify(value))
}

assert.deepStrictEqual(
  normalize(parseScene('m_123-t_A-01')),
  { merchant_id: 123, table_no: 'A-01' },
  'scan-entry parser must preserve hyphenated table_no from backend scene'
)

assert.deepStrictEqual(
  normalize(parseScene(encodeURIComponent('m_123-t_A-01'))),
  { merchant_id: 123, table_no: 'A-01' },
  'scan-entry parser must preserve hyphenated table_no from encoded backend scene'
)

assert.deepStrictEqual(
  normalize(parseScene('tid_456')),
  { table_id: 456 },
  'scan-entry parser must keep table_id fallback scene support'
)

assert.deepStrictEqual(
  normalize(parseScene('t789')),
  { table_id: 789 },
  'scan-entry parser must keep legacy table-id scene support'
)

assert.deepStrictEqual(
  normalize(parseScene('table_790')),
  { table_id: 790 },
  'scan-entry parser must keep table_ legacy scene support'
)

assert.strictEqual(parseScene('t789-extra'), null, 'partial legacy scene must stay rejected')
assert.strictEqual(parseScene('%E0%A4%A'), null, 'malformed encoded scene must stay rejected')
assert.strictEqual(parseScene('not-a-table-scene'), null, 'invalid scene must stay rejected')

console.log('check-dine-in-scan-entry-scene: scan-entry scene parsing is aligned')
