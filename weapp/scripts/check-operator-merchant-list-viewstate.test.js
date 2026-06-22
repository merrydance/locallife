const fs = require('fs')
const path = require('path')
const assert = require('assert')

const repoRoot = path.resolve(__dirname, '..')
const read = (file) => fs.readFileSync(path.join(repoRoot, file), 'utf8')

const wxmlSource = read('miniprogram/pages/operator/merchants/index.wxml')
const wxssSource = read('miniprogram/pages/operator/merchants/index.wxss')
const pageSource = read('miniprogram/pages/operator/merchants/index.ts')
const serviceSource = read('miniprogram/pages/operator/_services/operator-merchant-management.ts')

assert(
  wxmlSource.includes('<scroll-view') && wxmlSource.includes('wx:else') && wxmlSource.includes('class="merchants-scroll animate-fade-in"'),
  'operator merchant list must render the scroll-view directly in the success branch so list, empty, and load-more states have visible height'
)
assert(
  wxmlSource.includes('wx:if="{{merchants.length === 0}}"') && wxmlSource.includes('description="暂无商户数据"'),
  'operator merchant list must expose a visible empty state after successful loads with no merchants'
)
assert(
  wxmlSource.includes('class="merchant-summary"') &&
    wxmlSource.includes('{{total}}') &&
    wxmlSource.includes('{{totalLabel}}') &&
    wxmlSource.includes('已加载 {{merchants.length}}/{{total}}'),
  'operator merchant list must expose backend total and loaded count above the list'
)
assert(
  wxmlSource.includes('scroll-top="{{scrollTop}}"'),
  'operator merchant list scroll-view must bind scroll-top so refresh, search, and filter changes can return to the top'
)
assert(
  /\.content\s*\{[\s\S]*?min-height:\s*0;[\s\S]*?\}/.test(wxssSource),
  'operator merchant list content container must allow its flex child scroll-view to shrink and fill remaining height'
)
assert(
  /\.page-container\s*\{[\s\S]*?height:\s*100vh;[\s\S]*?overflow:\s*hidden;[\s\S]*?\}/.test(wxssSource),
  'operator merchant list page container must bound the internal scroll-view to the viewport instead of letting the page body scroll'
)
assert(
  /\.merchants-scroll\s*\{[\s\S]*?height:\s*100%;[\s\S]*?min-height:\s*0;[\s\S]*?\}/.test(wxssSource),
  'operator merchant list scroll-view must use a stable visible height and allow flex shrink'
)
assert(
  !/\.merchants-scroll\s*\{[\s\S]*?(^|\n)\s*height:\s*0;[\s\S]*?\}/m.test(wxssSource),
  'operator merchant list scroll-view must not set height: 0 because it hides the list, empty, and retry content'
)
assert(
  pageSource.includes('scrollTop: 0') &&
    pageSource.includes('resetMerchantScrollTop') &&
    pageSource.includes('wx.nextTick') &&
    pageSource.includes('totalLabel'),
  'operator merchant list page must own scroll-top reset state and total-label view state'
)
assert(
  pageSource.includes('hasMore: result.hasMore'),
  'operator merchant list page must use service pagination contract instead of deriving load-more state only from local list length'
)
assert(
  serviceSource.includes('result.page_id') &&
    serviceSource.includes('result.page_size') &&
    serviceSource.includes('pageId * pageSize < total'),
  'operator merchant list service must derive hasMore from backend page metadata and total'
)

console.log('check-operator-merchant-list-viewstate: merchant list view states keep visible scroll height')
