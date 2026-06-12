const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const sourcePath = path.join(
  __dirname,
  '..',
  'miniprogram',
  'pages',
  'takeout',
  'cart',
  '_utils',
  'takeout-cart-view.ts'
)

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

const {
  buildRecalculatedGroup,
  buildUpdatedGroupWithDeliveryFee,
  getPackagingCheckoutBlocker
} = loadModule()

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

const updatedWithDeliveryDiscount = buildUpdatedGroupWithDeliveryFee({
  cartId: 2,
  orderType: 'takeout',
  subtotal: 2000,
  deliveryFee: 0,
  deliveryFeeDiscount: 0,
  items: [],
  packagingRequired: false
}, 520, 200)

assert.strictEqual(updatedWithDeliveryDiscount.deliveryFee, 520)
assert.strictEqual(updatedWithDeliveryDiscount.deliveryFeeDiscount, 200)
assert.strictEqual(updatedWithDeliveryDiscount.deliveryFeeDisplay, '¥3.20')
assert.strictEqual(updatedWithDeliveryDiscount.totalAmount, 2320)
assert.strictEqual(updatedWithDeliveryDiscount.totalAmountDisplay, '¥23.20')

const recalculatedWithDeliveryDiscount = buildRecalculatedGroup({
  ...updatedWithDeliveryDiscount,
  items: [
    { unitPrice: 1000, quantity: 3 }
  ]
})

assert.strictEqual(recalculatedWithDeliveryDiscount.subtotal, 3000)
assert.strictEqual(recalculatedWithDeliveryDiscount.totalAmount, 3320)

console.log('check-takeout-cart-packaging-checkout tests passed')
