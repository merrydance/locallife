const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const sourcePath = path.join(__dirname, '..', 'miniprogram', 'utils', 'restaurant-detail-header.ts')
const headerWxmlPath = path.join(__dirname, '..', 'miniprogram', 'pages', 'takeout', 'restaurant-detail', 'index.wxml')
const headerWxssPath = path.join(__dirname, '..', 'miniprogram', 'pages', 'takeout', 'restaurant-detail', 'index.wxss')
const headerSectionsWxssPath = path.join(__dirname, '..', 'miniprogram', 'styles', 'takeout-restaurant-detail-sections.wxss')

function loadModule() {
  const source = fs.readFileSync(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018,
      strict: true
    }
  }).outputText

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const { resolveRestaurantHeaderCollapsed } = loadModule()
const headerWxml = fs.readFileSync(headerWxmlPath, 'utf8')
const headerWxss = `${fs.readFileSync(headerWxssPath, 'utf8')}\n${fs.readFileSync(headerSectionsWxssPath, 'utf8')}`
const titleRowMatch = headerWxml.match(/<view class="restaurant-title-row"[\s\S]*?<\/view>\s*<\/view>/)
const tabsStartTag = headerWxml.match(/<view[\s\S]*?class="tabs-section[\s\S]*?>/)
const contentStartTag = headerWxml.match(/<view[\s\S]*?class="content-area[\s\S]*?>/)

assert(titleRowMatch, 'restaurant detail header must render a title row')
assert(
  titleRowMatch[0].includes('open-type="share"'),
  'share button should be placed in the same row as the restaurant name'
)
assert(
  titleRowMatch[0].includes('catchtap="stopPropagation"'),
  'share button should not bubble into the restaurant info tap target'
)
assert(
  !headerWxml.includes('class="header-share-row"'),
  'share button should no longer render as a separate row below the restaurant title'
)
assert(
  resolveRestaurantHeaderCollapsed({
    scrollTop: 0,
    headerCollapsed: false,
    scrollDirection: 'up'
  }) === true,
  'header should collapse from an upward page gesture even before an inner scroll-view reports scrollTop'
)
assert(
  resolveRestaurantHeaderCollapsed({
    scrollTop: 88,
    headerCollapsed: false,
    scrollDirection: 'none'
  }) === true,
  'header should collapse from scrollTop even when scroll-view touchmove is not delivered'
)
assert(
  resolveRestaurantHeaderCollapsed({
    scrollTop: 88,
    headerCollapsed: false,
    scrollDirection: 'up'
  }) === true,
  'header should collapse once menu content is clearly scrolled upward'
)
assert(
  resolveRestaurantHeaderCollapsed({
    scrollTop: 0,
    headerCollapsed: true,
    scrollDirection: 'none'
  }) === true,
  'header should ignore low scrollTop rebounds caused by the layout transition'
)
assert(
  resolveRestaurantHeaderCollapsed({
    scrollTop: 0,
    headerCollapsed: true,
    scrollDirection: 'up'
  }) === true,
  'header should not expand from a low scrollTop while the active gesture is still upward'
)
assert(
  resolveRestaurantHeaderCollapsed({
    scrollTop: 0,
    headerCollapsed: true,
    scrollDirection: 'down'
  }) === false,
  'header should expand again when the user scrolls downward back to the top'
)
assert(
  headerWxml.includes('bindscroll="onScroll"'),
  'restaurant detail menu scroll should still drive the visual header collapse'
)
assert(
  headerWxml.includes('bindtouchmove="onMenuTouchMove"'),
  'restaurant detail menu scroll should track touch direction to avoid layout-feedback jumps'
)
assert(
  headerWxml.includes('capture-bind:touchmove="onMenuTouchMove"'),
  'restaurant detail content area should capture touch direction before scroll-view consumes it'
)
assert(
  /class="header-section[\s\S]*?capture-bind:touchmove="onMenuTouchMove"/.test(headerWxml),
  'restaurant detail header area should also collapse from an upward page gesture'
)
assert(
  /class="tabs-section[\s\S]*?capture-bind:touchmove="onMenuTouchMove"/.test(headerWxml),
  'restaurant detail tabs should also collapse from an upward page gesture'
)
assert(tabsStartTag, 'restaurant detail page must render tabs section')
assert(contentStartTag, 'restaurant detail page must render content area')
assert(
  tabsStartTag[0].includes("headerCollapsed ? '100rpx' : '320rpx'"),
  'tabs section should follow the guarded collapsed header height'
)
assert(
  contentStartTag[0].includes("headerCollapsed ? '100rpx' : '320rpx'"),
  'content area should follow the guarded collapsed header height'
)
assert(
  headerWxml.includes('headerCollapsed') && headerWxss.includes('.header-section.collapsed'),
  'restaurant detail header should keep a visual collapsed state while scrolling upward'
)

console.log('restaurant detail header layout tests passed')
