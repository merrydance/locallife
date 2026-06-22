const fs = require('fs')
const path = require('path')
const assert = require('assert')

const repoRoot = path.resolve(__dirname, '..')
const read = (file) => fs.readFileSync(path.join(repoRoot, file), 'utf8')

const pages = [
  {
    name: 'operator finance bills',
    wxml: 'miniprogram/pages/operator/finance/bills/index.wxml',
    ts: 'miniprogram/pages/operator/finance/bills/index.ts',
    json: 'miniprogram/pages/operator/finance/bills/index.json',
    scrollClass: 'bill-scroll',
    loadingFlag: 'loadingMore'
  },
  {
    name: 'operator withdrawals',
    wxml: 'miniprogram/pages/operator/finance/withdrawals/index.wxml',
    ts: 'miniprogram/pages/operator/finance/withdrawals/index.ts',
    json: 'miniprogram/pages/operator/finance/withdrawals/index.json',
    scrollClass: 'withdrawal-scroll',
    loadingFlag: 'loadingMore'
  },
  {
    name: 'operator safety report',
    wxml: 'miniprogram/pages/operator/safety/report/index.wxml',
    ts: 'miniprogram/pages/operator/safety/report/index.ts',
    json: 'miniprogram/pages/operator/safety/report/index.json',
    scrollClass: 'safety-scroll',
    loadingFlag: 'loadingMore'
  }
]

for (const page of pages) {
  const wxml = read(page.wxml)
  const ts = read(page.ts)
  const json = read(page.json)

  assert(
    wxml.includes('<scroll-view') &&
      wxml.includes(`class="${page.scrollClass}"`) &&
      wxml.includes('bindscrolltolower="onLoadMore"'),
    `${page.name} must use scroll-view touch-bottom loading instead of a manual load-more button`
  )
  assert(
    wxml.includes('enable-back-to-top') &&
      wxml.includes('scroll-top="{{scrollTop}}"'),
    `${page.name} must expose a reliable back-to-top path for the scroll container`
  )
  assert(
    wxml.includes('refresher-enabled="{{true}}"') &&
      wxml.includes('refresher-triggered="{{refreshing}}"') &&
      wxml.includes('bindrefresherrefresh="onPullDownRefresh"'),
    `${page.name} must use scroll-view pull-down refresh`
  )
  assert(
    !/加载更多/.test(wxml) && !/<t-button[^>]*bind(?::|)tap="onLoadMore"/.test(wxml),
    `${page.name} must not keep a visible load-more button`
  )
  assert(
    wxml.includes(`wx:if="{{${page.loadingFlag}}}"`) &&
      wxml.includes('没有更多'),
    `${page.name} must keep passive bottom loading and no-more feedback`
  )
  assert(
    ts.includes('scrollTop: 0') &&
      ts.includes('loadingMore: false') &&
      ts.includes('refreshing: false') &&
      ts.includes('onPullDownRefresh()') &&
      ts.includes('this.setData({ refreshing: true') &&
      ts.includes('refreshing: false') &&
      ts.includes('wx.stopPullDownRefresh()'),
    `${page.name} page state must drive scroll-view refresher completion`
  )
  assert(
    !json.includes('"enablePullDownRefresh": true'),
    `${page.name} must not use page-level pull-down refresh after adopting scroll-view refresher`
  )
}

console.log('check-operator-scroll-loadmore: operator legacy load-more buttons use scroll-view refresh and touch-bottom loading')
