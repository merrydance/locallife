const fs = require('fs')
const path = require('path')
const assert = require('assert')

const repoRoot = path.resolve(__dirname, '..')
const read = (file) => fs.readFileSync(path.join(repoRoot, file), 'utf8')

const wxmlSource = read('miniprogram/pages/operator/merchants/index.wxml')
const wxssSource = read('miniprogram/pages/operator/merchants/index.wxss')

assert(
  wxmlSource.includes('<scroll-view') && wxmlSource.includes('wx:else') && wxmlSource.includes('class="merchants-scroll animate-fade-in"'),
  'operator merchant list must render the scroll-view directly in the success branch so list, empty, and load-more states have visible height'
)
assert(
  wxmlSource.includes('wx:if="{{merchants.length === 0}}"') && wxmlSource.includes('description="暂无商户数据"'),
  'operator merchant list must expose a visible empty state after successful loads with no merchants'
)
assert(
  /\.content\s*\{[\s\S]*?min-height:\s*0;[\s\S]*?\}/.test(wxssSource),
  'operator merchant list content container must allow its flex child scroll-view to shrink and fill remaining height'
)
assert(
  /\.merchants-scroll\s*\{[\s\S]*?height:\s*100%;[\s\S]*?\}/.test(wxssSource),
  'operator merchant list scroll-view must use a visible height instead of collapsing to zero'
)
assert(
  !/\.merchants-scroll\s*\{[\s\S]*?height:\s*0;[\s\S]*?\}/.test(wxssSource),
  'operator merchant list scroll-view must not set height: 0 because it hides the list, empty, and retry content'
)

console.log('check-operator-merchant-list-viewstate: merchant list view states keep visible scroll height')
