const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const rootDir = path.join(__dirname, '..')

function loadTsModule(relativePath) {
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
    require,
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

console.log('takeout search result tab tests passed')
