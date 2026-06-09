const fs = require('fs')
const path = require('path')

const repoRoot = path.resolve(__dirname, '..')
const pageRoot = path.join(repoRoot, 'miniprogram/pages/takeout/restaurant-detail')
const wxml = fs.readFileSync(path.join(pageRoot, 'index.wxml'), 'utf8')
const wxss = fs.readFileSync(path.join(pageRoot, 'index.wxss'), 'utf8')
const json = fs.readFileSync(path.join(pageRoot, 'index.json'), 'utf8')

function assert(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}

assert(
  !wxml.includes('<t-radio') && !wxml.includes('<t-radio-group'),
  'restaurant detail order type switch should not use radio rows'
)
assert(
  !json.includes('tdesign-miniprogram/radio'),
  'restaurant detail should drop unused radio component declarations'
)
assert(
  wxml.includes('class="order-type-segmented"'),
  'restaurant detail should render a segmented order type switch'
)
assert(
  /class="order-type-option[\s\S]*data-value="takeout"[\s\S]*外卖代取/.test(wxml),
  'segmented switch should expose the takeout option'
)
assert(
  /class="order-type-option[\s\S]*data-value="takeaway"[\s\S]*到店自取/.test(wxml),
  'segmented switch should expose the takeaway option'
)
assert(
  wxml.includes('bindtap="onOrderTypeTap"'),
  'segmented switch should use the page order type handler'
)
assert(
  wxss.includes('grid-template-columns: repeat(2, minmax(0, 1fr))'),
  'segmented switch should keep takeout and takeaway in an equal-width horizontal layout'
)
assert(
  wxml.includes("feeTip=\"{{orderType === 'takeaway' ? '到店自取，无代取费' : ''}}\""),
  'takeaway cart bar should not continue to show a delivery fee tip'
)

console.log('takeout restaurant order type switch tests passed')
