const fs = require('fs')
const path = require('path')
const assert = require('assert')

const repoRoot = path.resolve(__dirname, '..')
const read = (file) => fs.readFileSync(path.join(repoRoot, file), 'utf8')

function assertIncludes(source, needle, message) {
  assert(source.includes(needle), message)
}

function assertRegex(source, pattern, message) {
  assert(pattern.test(source), message)
}

function assertNotRegex(source, pattern, message) {
  assert(!pattern.test(source), message)
}

function checkScrollablePagedList({
  pageName,
  wxml,
  wxss,
  page,
  service,
  pageClass,
  scrollClass,
  itemsName,
  summaryClass,
  totalLabelName,
  totalLabelHelperName,
  resetMethodName
}) {
  assertIncludes(
    wxml,
    '<scroll-view',
    `${pageName} must render a scroll-view for the paged list`
  )
  assertIncludes(
    wxml,
    `class="${scrollClass}`,
    `${pageName} scroll-view must use the expected stable scroll class`
  )
  assertIncludes(
    wxml,
    'scroll-top="{{scrollTop}}"',
    `${pageName} scroll-view must bind scroll-top so refresh, search, and filter changes can return to the top`
  )
  assertIncludes(
    wxml,
    `class="${summaryClass}"`,
    `${pageName} must expose a backend-total summary before the list`
  )
  assertIncludes(
    wxml,
    '{{total}}',
    `${pageName} summary must render backend total`
  )
  assertIncludes(
    wxml,
    `{{${totalLabelName}}}`,
    `${pageName} summary must render a filter-aware total label`
  )
  assertIncludes(
    wxml,
    `已加载 {{${itemsName}.length}}/{{total}}`,
    `${pageName} summary must render loaded count against backend total`
  )
  assertRegex(
    wxss,
    new RegExp(`\\.${pageClass}\\s*\\{[\\s\\S]*?height:\\s*100vh;[\\s\\S]*?overflow:\\s*hidden;[\\s\\S]*?\\}`),
    `${pageName} page container must bound the internal scroll-view to the viewport instead of letting the page body scroll`
  )
  assertRegex(
    wxss,
    /\.content\s*\{[\s\S]*?min-height:\s*0;[\s\S]*?\}/,
    `${pageName} content container must allow its flex child scroll-view to shrink and fill remaining height`
  )
  assertRegex(
    wxss,
    new RegExp(`\\.${scrollClass}\\s*\\{[\\s\\S]*?height:\\s*100%;[\\s\\S]*?min-height:\\s*0;[\\s\\S]*?\\}`),
    `${pageName} scroll-view must use a stable visible height and allow flex shrink`
  )
  assertNotRegex(
    wxss,
    new RegExp(`\\.${scrollClass}\\s*\\{[\\s\\S]*?(^|\\n)\\s*height:\\s*0;[\\s\\S]*?\\}`, 'm'),
    `${pageName} scroll-view must not set height: 0 because it hides list, empty, and retry content`
  )
  assertIncludes(
    page,
    'scrollTop: 0',
    `${pageName} page must own scroll-top view state`
  )
  assertIncludes(
    page,
    resetMethodName,
    `${pageName} page must expose an explicit scroll-top reset method`
  )
  assertIncludes(
    page,
    'wx.nextTick',
    `${pageName} scroll reset must toggle scrollTop through nextTick so repeated resets work`
  )
  assertIncludes(
    service,
    totalLabelHelperName,
    `${pageName} service must own filter-aware total-label status mapping`
  )
  assertIncludes(
    page,
    `${totalLabelName}: ${totalLabelHelperName}(this.data.statusFilter)`,
    `${pageName} page must consume the service total-label helper instead of switching on status literals`
  )
  assertIncludes(
    page,
    'hasMore: result.hasMore',
    `${pageName} page must use service pagination contract instead of deriving load-more state from local list length`
  )
  assertRegex(
    page,
    /this\.setData\(\{\s*loading:\s*true,\s*loadingMore:\s*false,[\s\S]*?page:\s*1/,
    `${pageName} refresh branch must clear loadingMore so a filter refresh cannot leave stale bottom loading state`
  )
  assertIncludes(
    page,
    'page: result.nextPage',
    `${pageName} page must advance from the service returned nextPage`
  )
  assertIncludes(
    service,
    'result.page_id',
    `${pageName} service must inspect backend page_id metadata`
  )
  assertIncludes(
    service,
    'result.page_size',
    `${pageName} service must inspect backend page_size metadata`
  )
  assertIncludes(
    service,
    'pageId * pageSize < total',
    `${pageName} service must derive fallback hasMore from backend page metadata and total`
  )
}

function checkMerchantList() {
  checkScrollablePagedList({
    pageName: 'operator merchant list',
    wxml: read('miniprogram/pages/operator/merchants/index.wxml'),
    wxss: read('miniprogram/pages/operator/merchants/index.wxss'),
    page: read('miniprogram/pages/operator/merchants/index.ts'),
    service: read('miniprogram/pages/operator/_services/operator-merchant-management.ts'),
    pageClass: 'page-container',
    scrollClass: 'merchants-scroll',
    itemsName: 'merchants',
    summaryClass: 'merchant-summary',
    totalLabelName: 'totalLabel',
    totalLabelHelperName: 'buildOperatorMerchantTotalLabel',
    resetMethodName: 'resetMerchantScrollTop'
  })
}

function checkRiderList() {
  checkScrollablePagedList({
    pageName: 'operator rider list',
    wxml: read('miniprogram/pages/operator/riders/index.wxml'),
    wxss: read('miniprogram/pages/operator/riders/index.wxss'),
    page: read('miniprogram/pages/operator/riders/index.ts'),
    service: read('miniprogram/pages/operator/_services/operator-rider-management.ts'),
    pageClass: 'page',
    scrollClass: 'riders-scroll',
    itemsName: 'riders',
    summaryClass: 'rider-summary',
    totalLabelName: 'totalLabel',
    totalLabelHelperName: 'buildOperatorRiderTotalLabel',
    resetMethodName: 'resetRiderScrollTop'
  })
}

function checkNotificationList() {
  const wxml = read('miniprogram/pages/operator/notifications/index.wxml')
  const wxss = read('miniprogram/pages/operator/notifications/index.wxss')
  const page = read('miniprogram/pages/operator/notifications/index.ts')
  const service = read('miniprogram/pages/operator/_services/operator-notification-center.ts')

  assertIncludes(
    wxml,
    'scroll-top="{{scrollTop}}"',
    'operator notification list scroll-view must bind scroll-top for category and manual refresh recovery'
  )
  assertIncludes(
    wxml,
    '已加载 {{notifications.length}}/{{total}}',
    'operator notification list must show loaded count against backend total'
  )
  assertIncludes(
    page,
    'let notificationListRequestSeq = 0',
    'operator notification list must use a request sequence guard so stale category responses cannot overwrite current results'
  )
  assertIncludes(
    page,
    'const requestSeq = ++notificationListRequestSeq',
    'operator notification list must increment request sequence before each list request'
  )
  assertRegex(
    page,
    /if\s*\(\s*!refresh\s*&&\s*\(\s*this\.data\.loading\s*\|\|\s*this\.data\.loadingMore\s*\)\s*\)\s*return/,
    'operator notification list must allow a new refresh request while an older refresh is in flight so category changes can invalidate stale responses'
  )
  assertIncludes(
    page,
    'requestSeq !== notificationListRequestSeq',
    'operator notification list must drop stale list responses'
  )
  assertIncludes(
    page,
    'scrollTop: 0',
    'operator notification list must own scroll-top view state'
  )
  assertIncludes(
    page,
    'resetNotificationScrollTop',
    'operator notification list must expose an explicit scroll reset method'
  )
  assertIncludes(
    page,
    'hasMore: result.hasMore',
    'operator notification list page must use service pagination contract'
  )
  assertRegex(
    page,
    /this\.setData\(\{\s*loading:\s*true,\s*loadingMore:\s*false,[\s\S]*?page:\s*1/,
    'operator notification list refresh branch must clear loadingMore so a category refresh cannot leave stale bottom loading state'
  )
  assertIncludes(
    page,
    'page: result.nextPage',
    'operator notification list page must advance from service returned nextPage'
  )
  assertIncludes(
    service,
    'nextPage: result.page + 1',
    'operator notification list service must use normalized result.page for nextPage'
  )
  assertIncludes(
    service,
    'hasMore: result.hasMore',
    'operator notification list service must preserve normalizePaginatedResult hasMore instead of deriving from local list length'
  )
  assertRegex(
    wxss,
    /\.content\s*\{[\s\S]*?min-height:\s*0;[\s\S]*?\}/,
    'operator notification list content must allow the scroll-view to shrink inside the page shell'
  )
}

function checkDispatchHall() {
  const page = read('miniprogram/pages/operator/dispatch-hall/index.ts')

  assertIncludes(
    page,
    'let dispatchHallRequestSeq = 0',
    'operator dispatch hall must use a request sequence guard so stale region responses cannot overwrite current results'
  )
  assertIncludes(
    page,
    'const requestSeq = ++dispatchHallRequestSeq',
    'operator dispatch hall must increment request sequence before each region list request'
  )
  assertRegex(
    page,
    /if\s*\(\s*!refresh\s*&&\s*\(\s*this\.data\.loading\s*\|\|\s*this\.data\.loadingMore\s*\)\s*\)\s*\{\s*return\s*\}/,
    'operator dispatch hall must allow a new refresh request while an older refresh is in flight so region switches can invalidate stale responses'
  )
  assertIncludes(
    page,
    'requestSeq !== dispatchHallRequestSeq',
    'operator dispatch hall must drop stale region list responses'
  )
  assertIncludes(
    page,
    'page: result.page + 1',
    'operator dispatch hall must advance from backend-normalized result.page instead of stale local currentPage'
  )
  assertNotRegex(
    page,
    /page:\s*currentPage\s*\+\s*1/,
    'operator dispatch hall must not advance pagination from the stale local currentPage'
  )
  assertRegex(
    page,
    /this\.setData\(\{\s*loading:\s*true,\s*loadingMore:\s*false,[\s\S]*?page:\s*1/,
    'operator dispatch hall refresh branch must clear loadingMore so a region refresh cannot leave stale bottom loading state'
  )
}

checkMerchantList()
checkRiderList()
checkNotificationList()
checkDispatchHall()

console.log('check-operator-list-viewstate: operator list pagination, totals, scroll, and stale-request guards are covered')
