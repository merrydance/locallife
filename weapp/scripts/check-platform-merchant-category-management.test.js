const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const repoRoot = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), 'utf8')
}

function loadTsModule(relativePath) {
  const sourcePath = path.join(repoRoot, relativePath)
  const source = fs.readFileSync(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018,
      esModuleInterop: true
    }
  }).outputText

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(id) {
      if (id === '../api/location') return {}
      throw new Error(`unexpected require: ${id}`)
    }
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const appJson = JSON.parse(read('miniprogram/app.json'))
const platformPackage = appJson.subPackages.find((item) => item.root === 'pages/platform')

assert(platformPackage, 'platform subpackage must exist')
assert(
  platformPackage.pages.includes('categories/index'),
  'platform package must register the merchant category management page'
)

const dashboardViewSource = read('miniprogram/pages/platform/_services/platform-dashboard-view.ts')
assert(
  dashboardViewSource.includes("id: 'merchant-categories'"),
  'platform dashboard view must include a merchant category management entry'
)
assert(
  dashboardViewSource.includes("url: '/pages/platform/categories/index'"),
  'merchant category management entry must point to the platform category page'
)
assert(
  dashboardViewSource.includes("title: '经营品类'"),
  'merchant category management entry should use product-facing category copy'
)

const pageTs = read('miniprogram/pages/platform/categories/index.ts')
const pageWxml = read('miniprogram/pages/platform/categories/index.wxml')
const pageWxss = read('miniprogram/pages/platform/categories/index.wxss')
const pageJson = JSON.parse(read('miniprogram/pages/platform/categories/index.json'))
const packageJson = JSON.parse(read('package.json'))
const takeoutCategories = loadTsModule('miniprogram/adapters/takeout-categories.ts')

assert(pageTs.includes("TagService.listTags('merchant')"), 'category page must list merchant tags from backend')
assert(pageTs.includes("type: 'merchant'"), 'category page must create merchant tags only')
assert(pageTs.includes('TagService.updateTag'), 'category page must support updating merchant category icons')
assert(pageTs.includes('TagService.deleteTag'), 'category page must support deleting backend merchant tags')
assert(pageTs.includes('buildCategoryIconEmoji'), 'category page must reuse the takeout category emoji matcher')
assert(pageTs.includes('CATEGORY_ICON_OPTIONS'), 'category page must expose selectable category icon options')
assert(pageTs.includes('selectedIcon'), 'category page must track the selected category icon instead of only guessing from name')
assert(pageTs.includes('selectedIconManuallyChanged'), 'category page must preserve manually selected icons when category names change')
assert(pageTs.includes('buildCategoryIconEmoji(value)') || pageTs.includes('buildCategoryIconEmoji(name)'), 'category page must auto-match a default emoji from the entered category name')
assert(!pageTs.includes("type: 'table'"), 'category page must not manage table tags')
assert(!/wx\.showModal\(\{[\s\S]*editable:\s*true/.test(pageTs), 'category page must not use native editable modal because global prompt guards can pollute editable content')
assert(pageWxml.includes('经营品类管理'), 'category page must render the management title')
assert(pageWxml.includes('bind:navheight="onNavHeight"'), 'category page navbar must feed the measured nav height into the page shell')
assert(pageWxml.includes('<scroll-view'), 'category page must use an internal scroll-view so content does not slide under the fixed navbar')
assert(pageWxml.includes('refresher-triggered="{{refreshing}}"'), 'category page scroll-view must expose explicit refresh state')
assert(pageWxml.includes('bindrefresherrefresh="onRefresh"'), 'category page scroll-view must handle pull-to-refresh internally')
assert(pageTs.includes('onNavHeight'), 'category page must handle custom-navbar nav height events')
assert(pageTs.includes('refreshing'), 'category page must track scroll-view refresh state')
assert(!pageTs.includes('onPullDownRefresh'), 'category page must not rely on native page pull-down refresh with a fixed custom navbar')
assert(pageWxss.includes('.category-scroll'), 'category page must style the internal scroll-view')
assert(pageWxss.includes('height: calc(100vh - var(--page-shell-nav-height, 88px)'), 'category page scroll height must subtract the custom navbar height')
assert(pageWxml.includes('新建品类'), 'category page must expose the create action as category copy')
assert(pageWxml.includes('影响首页筛选与商户资料选择'), 'category page must keep category impact visible once in the list header')
assert(pageWxml.includes('首页图标预览'), 'category page must show how the takeout category emoji will be matched')
assert(pageWxml.includes('icon-picker'), 'category create/edit dialog must provide a selectable icon picker')
assert(pageWxml.includes('icon-picker-scroll'), 'category icon picker must scroll inside the dialog when many food icons are available')
assert(pageWxml.includes('data-icon="{{icon}}"'), 'category icon picker must bind selected icon data')
assert(!pageWxml.includes('tag-meta-row'), 'category rows must not repeat the same explanatory meta copy for every short tag')
assert(!pageWxml.includes('用于商户经营资料与首页筛选'), 'category rows must not duplicate static impact copy per item')
assert(!pageWxml.includes('平台统一维护</t-tag>'), 'category rows must not repeat static platform-maintained tags per item')
assert(!pageWxml.includes('首页图标预览</t-tag>'), 'category rows must not repeat static icon-preview tags per item')
assert(!pageWxss.includes('.tag-meta-row'), 'category styles must not retain per-row repeated meta layout')
assert(!pageWxss.includes('.tag-row {'), 'category rows must not be implemented as full-width card rows for two-character labels')
assert(!pageWxss.includes('.category-grid'), 'category page must not use card/grid cells for short two-character category labels')
assert(pageWxss.includes('max-height: 360rpx'), 'category icon picker scroll area must keep the dialog compact on small screens')
assert(!pageWxml.includes('<t-fab'), 'category page must not duplicate the create action with a floating button')
assert(pageWxml.includes('class="category-chip-list"'), 'category page must render categories as a compact tag flow')
assert(pageWxml.includes('<t-tag'), 'category page must use TDesign tags for short category labels')
assert(pageWxml.includes('bind:close="onDeleteCategory"'), 'category tag close affordance must delete the backend category')
assert(pageWxml.includes('closable="{{true}}"'), 'category tags must expose delete as a compact tag-level action')
assert(pageWxml.includes('item.iconText'), 'category list must render the matched category emoji')
assert(pageJson.usingComponents['t-button'], 'category page must declare TDesign button')
assert(pageJson.usingComponents['t-empty'], 'category page must declare TDesign empty')
assert(pageJson.usingComponents['t-input'], 'category page must declare TDesign input')
assert(pageJson.usingComponents['t-loading'], 'category page must declare TDesign loading')
assert(pageJson.usingComponents['t-notice-bar'], 'category page must declare TDesign notice bar')
assert(pageJson.usingComponents['t-tag'], 'category page must declare TDesign tag')
assert(!pageJson.usingComponents['t-fab'], 'category page must not declare unused TDesign fab after using a header create action')
assert(pageJson.usingComponents['t-dialog'], 'category page must declare TDesign dialog')
assert(packageJson.scripts['check:prompt-feedback-editable-modal'], 'prompt feedback editable modal regression must be exposed as an npm script')
assert(packageJson.scripts['quality:check'].includes('check:prompt-feedback-editable-modal'), 'prompt feedback editable modal regression must be part of quality:check')

const expectedFoodIconOptions = [
  '🍽️', '🍴', '🥢', '🥡', '🍱', '🍲', '🥘', '🍚', '🍜', '🍝', '🥟', '🍣',
  '🍔', '🍟', '🍕', '🌭', '🥪', '🌮', '🌯', '🥙', '🧆', '🍗', '🥩', '🥓',
  '🦐', '🦞', '🦀', '🦪', '🦑', '🥐', '🥯', '🍞', '🥖', '🥨', '🥞', '🧇',
  '🥗', '🥬', '🥦', '🍅', '🥑', '🫒', '🍰', '🧁', '🍮', '🍦', '🍩', '🍪',
  '🍿', '🧈', '🧂', '🥫', '🧋', '☕', '🍵', '🥤', '🧃', '🥛', '🍺', '🍻',
  '🍷', '🍹', '🍾', '🧉'
]

for (const icon of expectedFoodIconOptions) {
  assert(
    pageTs.includes(`'${icon}'`),
    `category icon options should include food and drink emoji ${icon}`
  )
}

assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('家常菜'), '🥘')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('面条'), '🍜')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('米饭'), '🍚')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('饺子'), '🥟')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('馄饨'), '🥟')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('寿司'), '🍣')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('披萨'), '🍕')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('汉堡'), '🍔')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('炸鸡'), '🍗')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('牛排'), '🥩')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('小龙虾'), '🦞')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('螃蟹'), '🦀')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('沙拉'), '🥗')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('面包'), '🍞')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('蛋糕'), '🍰')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('甜品'), '🧁')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('奶茶'), '🧋')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('咖啡'), '☕')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('茶饮'), '🍵')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('烧烤'), '🔥')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('麻辣烫'), '🌶️')
assert.notStrictEqual(takeoutCategories.buildCategoryIconEmoji('砂锅'), '🍴', 'seeded category 砂锅 should have a specific emoji')
assert.notStrictEqual(takeoutCategories.buildCategoryIconEmoji('炒饼'), '🍴', 'seeded category 炒饼 should have a specific emoji')
assert.notStrictEqual(takeoutCategories.buildCategoryIconEmoji('粥'), '🍴', 'seeded category 粥 should have a specific emoji')
assert.notStrictEqual(takeoutCategories.buildCategoryIconEmoji('鱼'), '🍴', 'seeded category 鱼 should have a specific emoji')
assert.strictEqual(takeoutCategories.buildCategoryIconEmoji('未匹配新品类'), '🍴', 'unknown categories should fall back to the default utensil emoji')

console.log('platform merchant category management checks passed')
