const fs = require('fs')
const path = require('path')

const repoRoot = path.resolve(__dirname, '..')
const pageDir = path.join(repoRoot, 'miniprogram/pages/user_center/reviews')
const wxmlPath = path.join(pageDir, 'index.wxml')
const wxssPath = path.join(pageDir, 'index.wxss')
const tsPath = path.join(pageDir, 'index.ts')
const jsonPath = path.join(pageDir, 'index.json')
const reviewApiPath = path.join(repoRoot, 'miniprogram/api/review.ts')

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

function hasNonZeroPadding(rule) {
  const shorthand = rule.match(/(?:^|;)\s*padding\s*:\s*([^;]+)/)
  if (shorthand && shorthand[1].trim() !== '0') {
    return true
  }

  return /padding-(?:left|right)\s*:\s*(?!0(?:;|$))/.test(rule)
}

function main() {
  const wxml = read(wxmlPath)
  const wxss = read(wxssPath)
  const ts = read(tsPath)
  const json = read(jsonPath)
  const reviewApi = read(reviewApiPath)

  assert(
    /<custom-navbar[^>]*bind:navheight="onNavHeight"/.test(wxml),
    'user reviews page must bind custom-navbar navheight into the page shell'
  )
  assert(
    wxml.includes('page-shell--with-nav') && wxml.includes('page-shell--bottom-safe') && wxml.includes('page-shell--page-gutter'),
    'user reviews page must keep one shared page shell with nav, bottom safe area, and page gutter'
  )

  const pageRule = getRule(wxss, '.page-container')
  assert(
    /--page-shell-nav-gap:\s*var\(--spacer-xs\)/.test(pageRule),
    'user reviews page should use the compact shared nav gap instead of compensating with list padding'
  )
  assert(
    /box-sizing:\s*border-box/.test(pageRule),
    'user reviews page container must include shell padding inside the viewport height'
  )

  const listRule = getRule(wxss, '.list-inner')
  assert(
    listRule && !hasNonZeroPadding(listRule),
    'user reviews list-inner must not add a second horizontal gutter over page-shell--page-gutter'
  )
  assert(
    /gap:\s*var\(--spacer-md\)/.test(listRule),
    'user reviews list should preserve normal card spacing with flex gap'
  )

  const reviewCardRule = getRule(wxss, '.review-card')
  assert(
    !/margin-bottom\s*:/.test(reviewCardRule),
    'user reviews cards should not use per-card margins when the list owns spacing'
  )
  assert(
    /box-shadow:\s*var\(--shadow-sm\)/.test(reviewCardRule),
    'user reviews cards should use the shared shadow token'
  )

  assert(
    /function normalizeReviewImages/.test(ts) &&
      /review\.image_urls/.test(ts) &&
      /review\.imageUrls/.test(ts) &&
      /review\.images/.test(ts),
    'user reviews page must normalize backend image_urls, imageUrls, and legacy images before rendering'
  )
  assert(
    /review\.image_urls/.test(reviewApi) &&
      /review\.imageUrls/.test(reviewApi) &&
      /review\.images/.test(reviewApi),
    'review API normalization must preserve image URLs from backend-supported response shapes'
  )
  assert(
    /images:\s*normalizeReviewImages\(r\)/.test(ts),
    'user reviews view model must use the image normalization helper'
  )
  assert(
    !/data-urls="\{\{item\.images\}\}"/.test(wxml),
    'user reviews image preview must not pass image arrays through WXML dataset'
  )
  assert(
    /data-review-id="\{\{item\.id\}\}"/.test(wxml) &&
      /data-image-index="\{\{imageIndex\}\}"/.test(wxml) &&
      /wx:for-index="imageIndex"/.test(wxml),
    'user reviews image preview should look up images from page state by review id and image index'
  )
  assert(
    /find\(\(item\) => item\.id === reviewId\)/.test(ts) &&
      /wx\.previewImage/.test(ts),
    'user reviews preview handler must resolve the image list from page state'
  )

  assert(/orderId:\s*number/.test(ts), 'user reviews view model should expose orderId for supported management actions')
  assert(/orderIdLabel:\s*string/.test(ts), 'user reviews view model should expose a display order label')
  assert(/bindtap="onOrderDetail"/.test(wxml), 'user reviews page should expose a backend-supported order detail action')
  assert(/Navigation\.toOrderDetail\(String\(id\)\)/.test(ts), 'user reviews order action must navigate to the existing order detail page')
  assert(/icon="shop"[\s\S]*bindtap="onMerchantClick"/.test(wxml), 'user reviews page should expose a backend-supported merchant action')

  assert(!/\bt-rate\b|item\.rating|r\.rating/.test(wxml + ts + json), 'user reviews page must not render unsupported rating fields')
  assert(!/item\.tags|r\.tags|tag-row/.test(wxml + ts + wxss), 'user reviews page must not render unsupported review tags')
  assert(!/deleteReview|updateReview|removeReview|DELETE|PATCH|PUT/.test(ts + reviewApi), 'consumer review management must not expose unsupported edit/delete review API calls')

  assert(
    json.includes('"t-image"') && json.includes('"t-button"') && json.includes('"t-tag"'),
    'user reviews page must declare the TDesign components it renders'
  )

  console.log('check-user-reviews-page: validated user review gutter, image rendering, preview, and backend-supported management actions')
}

main()
