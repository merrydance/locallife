const fs = require('fs')
const path = require('path')

const repoRoot = path.resolve(__dirname, '..')
const pageDir = path.join(repoRoot, 'miniprogram/pages/user_center/reviews/create')
const wxmlPath = path.join(pageDir, 'index.wxml')
const wxssPath = path.join(pageDir, 'index.wxss')
const tsPath = path.join(pageDir, 'index.ts')
const jsonPath = path.join(pageDir, 'index.json')
const reviewApiPath = path.join(repoRoot, 'miniprogram/api/review.ts')
const userReviewsWxmlPath = path.join(repoRoot, 'miniprogram/pages/user_center/reviews/index.wxml')
const userReviewsTsPath = path.join(repoRoot, 'miniprogram/pages/user_center/reviews/index.ts')
const userReviewsJsonPath = path.join(repoRoot, 'miniprogram/pages/user_center/reviews/index.json')
const sharedReviewCardWxmlPath = path.join(repoRoot, 'miniprogram/components/review-card/index.wxml')
const sharedReviewCardJsonPath = path.join(repoRoot, 'miniprogram/components/review-card/index.json')

function read(filePath) {
  return fs.readFileSync(filePath, 'utf8')
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}

function getRule(css, selector) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const match = css.match(new RegExp(`${escaped}\\s*\\{([\\s\\S]*?)\\}`))
  return match ? match[1] : ''
}

function hasNonZeroShorthandPadding(rule) {
  const match = rule.match(/(?:^|;)\s*padding\s*:\s*([^;]+)/)
  if (!match) {
    return false
  }

  const value = match[1].trim()
  return value !== '0'
}

function main() {
  const wxml = read(wxmlPath)
  const wxss = read(wxssPath)
  const ts = read(tsPath)
  const json = read(jsonPath)
  const reviewApi = read(reviewApiPath)
  const userReviewsWxml = read(userReviewsWxmlPath)
  const userReviewsTs = read(userReviewsTsPath)
  const userReviewsJson = read(userReviewsJsonPath)
  const sharedReviewCardWxml = read(sharedReviewCardWxmlPath)
  const sharedReviewCardJson = read(sharedReviewCardJsonPath)

  assert(
    wxml.includes('page-shell--page-gutter'),
    'review create page must use the shared page gutter'
  )

  const wrapRule = getRule(wxss, '.review-create-wrap')
  assert(
    !hasNonZeroShorthandPadding(wrapRule),
    'review create wrapper must not add another horizontal padding over page-shell gutter'
  )
  assert(
    /gap:\s*var\(--spacer-md\)/.test(wrapRule),
    'review create wrapper should use shell rhythm instead of margins for section spacing'
  )

  assert(
    !wxml.includes('<t-check-tag') && !wxml.includes('快捷评价'),
    'review create page must not render unsupported quick review tags'
  )
  assert(
    !wxml.includes('selectedTags.indexOf'),
    'quick review tags must not compute selected state with indexOf in WXML'
  )
  assert(
    !wxml.includes('class="tag-chip'),
    'quick review tags must not use the old hand-written chip control'
  )
  assert(
    !/\.tag-chip\b/.test(wxss),
    'review create styles must remove old tag-chip outline styling'
  )
  assert(
    !json.includes('"t-check-tag"'),
    'review create page must not declare unsupported t-check-tag'
  )
  assert(
    !json.includes('"t-rate"'),
    'review create page must not declare rating components because backend reviews do not support score'
  )

  assert(
    !ts.includes('QuickReviewTag') && !ts.includes('quickTags') && !ts.includes('onTagChange'),
    'review create script must not keep unsupported quick tag state or handlers'
  )
  assert(!/\brating\b/.test(wxml), 'review create WXML must not render unsupported rating fields')
  assert(!/\brating\b/.test(ts), 'review create script must not keep unsupported rating state or payload')
  assert(!/\btags\b/.test(ts), 'review create submit payload must not send unsupported tags')
  assert(!/canSubmit/.test(ts), 'review create page does not need derived submit availability without rating gating')
  assert(!/\brating\??:/.test(reviewApi), 'review API contract must not expose unsupported rating')
  assert(!/\btags\??:/.test(reviewApi), 'review API contract must not expose unsupported tags')
  assert(!/r\.rating|r\.tags/.test(userReviewsTs), 'user reviews page must not derive fake rating or tag fields')
  assert(!/item\.rating|item\.tags|<t-rate/.test(userReviewsWxml), 'user reviews page must not render unsupported rating or tag fields')
  assert(!userReviewsJson.includes('"t-rate"'), 'user reviews page must not declare t-rate for reviews')
  assert(!/review\.rating|<t-rate/.test(sharedReviewCardWxml), 'shared review card must not render unsupported review score')
  assert(!sharedReviewCardJson.includes('"t-rate"'), 'shared review card must not declare t-rate for reviews')

  assert(
    /class="bottom-submit-bar/.test(wxml),
    'review create page must keep the submit action in a fixed bottom bar'
  )
  assert(
    /disabled="\{\{submitting\}\}"/.test(wxml),
    'submit button should stay tappable for validation feedback and only disable while submitting'
  )
  assert(
    !/disabled="\{\{!canSubmit\}\}"/.test(wxml),
    'submit button must not look unusable before validation; onSubmit owns validation feedback'
  )
  assert(
    !ts.includes('请选择总体评分'),
    'review create submit handler must not ask for unsupported rating'
  )

  const pageRule = getRule(wxss, '.page-container')
  assert(
    /--page-shell-bottom-offset:\s*220rpx/.test(pageRule),
    'page container must reserve space for the fixed submit bar'
  )

  const bottomRule = getRule(wxss, '.bottom-submit-bar')
  assert(/position:\s*fixed/.test(bottomRule), 'submit bar must be fixed at the bottom')
  assert(/padding-bottom:\s*calc\(var\(--spacer-sm\) \+ var\(--safe-area-bottom\)\)/.test(bottomRule), 'submit bar must include safe-area padding')

  console.log('check-review-create-page: validated review create backend-truth fields, gutter, and submit action')
}

main()
