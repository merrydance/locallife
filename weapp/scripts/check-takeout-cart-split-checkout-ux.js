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
  const orderConfirmTs = read(orderConfirmTsPath)

  assert(
    !cartWxml.includes('split-checkout-notice'),
    'cart page should not show a persistent split-checkout notice before checkout intent'
  )
  assert(
    cartTs.includes('SINGLE_MERCHANT_CHECKOUT_NOTICE'),
    'cart should own the single-merchant checkout notice'
  )
  assert(
    /wx\.showModal\(\{[\s\S]*title:\s*'暂不支持多商户一起支付'/.test(cartTs),
    'cart checkout should use a modal when multiple merchants are selected'
  )
  assert(
    cartTs.includes('hasMultiMerchantSelection'),
    'cart checkout must compute multi-merchant selection directly instead of relying on capability loading'
  )
  assert(
    orderConfirmTs.includes("'暂不支持多商户一起支付'"),
    'order confirm fallback should keep a provider-neutral multi-merchant guard'
  )

  const visibleCopy = `${cartWxml}\n${cartTs}\n${orderConfirmTs}`
  assert(
    !visibleCopy.includes('宝付支付暂不支持合单支付'),
    'multi-merchant checkout visible copy should not expose provider implementation wording'
  )
  assert(
    !visibleCopy.includes('当前支付通道需按商户分别下单支付'),
    'multi-merchant checkout visible copy should not describe internal payment-channel mechanics'
  )
  assert(
    !visibleCopy.includes('合单支付'),
    'multi-merchant checkout visible copy should not mention combined-payment business'
  )

  console.log('check-takeout-cart-split-checkout-ux: validated cart single-merchant checkout interaction copy')
}

main()
