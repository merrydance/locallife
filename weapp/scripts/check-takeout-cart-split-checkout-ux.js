const fs = require('fs')
const path = require('path')

const repoRoot = path.resolve(__dirname, '..')
const cartWxmlPath = path.join(repoRoot, 'miniprogram/pages/takeout/cart/index.wxml')
const cartTsPath = path.join(repoRoot, 'miniprogram/pages/takeout/cart/index.ts')
const checkoutServicePath = path.join(repoRoot, 'miniprogram/services/takeout-checkout.ts')
const orderConfirmTsPath = path.join(repoRoot, 'miniprogram/pages/takeout/order-confirm/index.ts')

function read(relativePath) {
  return fs.readFileSync(relativePath, 'utf8')
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}

function main() {
  const cartWxml = read(cartWxmlPath)
  const cartTs = read(cartTsPath)
  const checkoutService = read(checkoutServicePath)
  const orderConfirmTs = read(orderConfirmTsPath)

  assert(
    !cartWxml.includes('split-checkout-notice'),
    'cart page should not show a persistent split-checkout notice before checkout intent'
  )
  assert(
    !cartTs.includes('this.data.splitCheckoutRequired) {'),
    'cart merchant selection should allow multi-select and defer split-checkout feedback to checkout'
  )
  assert(
    /wx\.showModal\(\{[\s\S]*title:\s*'暂不支持合单支付'/.test(cartTs),
    'cart checkout should use a modal when split-checkout is required and multiple merchants are selected'
  )
  assert(
    checkoutService.includes("'暂不支持合单支付，请一次选择一家商户下单。'"),
    'split-checkout fallback copy should be user-facing and provider-neutral'
  )
  assert(
    orderConfirmTs.includes("'暂不支持合单支付'"),
    'order confirm fallback should keep the split-checkout modal title provider-neutral'
  )

  const visibleCopy = `${cartWxml}\n${cartTs}\n${checkoutService}\n${orderConfirmTs}`
  assert(
    !visibleCopy.includes('宝付支付暂不支持合单支付'),
    'split-checkout visible copy should not expose provider implementation wording'
  )
  assert(
    !visibleCopy.includes('当前支付通道需按商户分别下单支付'),
    'split-checkout visible copy should not describe internal payment-channel mechanics'
  )

  console.log('check-takeout-cart-split-checkout-ux: validated cart split-checkout interaction copy')
}

main()
