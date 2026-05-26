const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const sourcePath = path.join(__dirname, '..', 'miniprogram', 'utils', 'takeout-cart-view.ts')

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
    require(modulePath) {
      if (modulePath === '@/utils/image') {
        return { getPublicImageUrl: (value) => value }
      }
      throw new Error(`unexpected require: ${modulePath}`)
    },
    Math,
    Number,
    String
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const { getPackagingCheckoutBlocker } = loadModule()

const baseGroup = {
  cartId: 1,
  packagingRequired: true,
  items: [
    { id: 10, quantity: 2, isPackaging: false }
  ]
}

assert.strictEqual(getPackagingCheckoutBlocker([baseGroup], [1]), '请先选择包装方式')

assert.strictEqual(getPackagingCheckoutBlocker([
  {
    ...baseGroup,
    items: [
      { id: 10, quantity: 2, isPackaging: false },
      { id: 11, quantity: 1, isPackaging: true }
    ]
  }
], [1]), '')

assert.strictEqual(getPackagingCheckoutBlocker([
  {
    ...baseGroup,
    items: [
      { id: 11, quantity: 2, isPackaging: true }
    ]
  }
], [1]), '只能选择一种包装方式')

assert.strictEqual(getPackagingCheckoutBlocker([
  {
    ...baseGroup,
    packagingRequired: false
  }
], [1]), '')

console.log('check-takeout-cart-packaging-checkout tests passed')
