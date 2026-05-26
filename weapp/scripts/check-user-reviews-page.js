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
    ts.includes("from '../../../utils/image'") &&
      /getPublicImageUrl\(url\)/.test(ts),
    'user reviews image URLs must pass through getPublicImageUrl before rendering'
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
    /<t-image[^>]*class="review-image"/.test(wxml) &&
      !/<t-image[^>]*width="100%"[^>]*height="100%"/.test(wxml) &&
      !/<t-image[^>]*\slazy(?:\s|\/|>)/.test(wxml),
    'user review thumbnails must give t-image a stable class size and avoid lazy placeholder-only rendering in scroll-view'
  )
  const mediaItemRule = getRule(wxss, '.media-item')
  const reviewImageRule = getRule(wxss, '.review-image')
  assert(
    /position:\s*relative/.test(mediaItemRule) &&
      /width:\s*100%/.test(reviewImageRule) &&
      /height:\s*100%/.test(reviewImageRule) &&
      /display:\s*block/.test(reviewImageRule),
    'user review thumbnail styles must size both the media wrapper and t-image root deterministically'
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

  assert(/orderNo:\s*string/.test(ts), 'user reviews view model should expose the backend order_no')
  assert(/orderNo\?:\s*string/.test(reviewApi), 'review API contract must expose backend order_no when available')
  assert(/orderNo:\s*r\.order_no\s*\|\|/.test(ts), 'user reviews view model must prefer backend order_no over internal order_id')
  assert(!/订单 #\$\{r\.order_id\}/.test(ts), 'user reviews page must not display internal order_id as the order number')
  assert(/orderId:\s*number/.test(ts), 'user reviews view model should expose orderId for supported management actions')
  assert(/orderIdLabel:\s*string/.test(ts), 'user reviews view model should expose a display order label')
  assert(!/bindtap="onOrderDetail"/.test(wxml), 'user reviews card footer must not expose an order detail button')
  assert(!/icon="shop"[\s\S]*bindtap="onMerchantClick"/.test(wxml), 'user reviews card footer must not expose a merchant button')
  assert(!/<t-image[^>]*item\.logoUrl/.test(wxml), 'user reviews cards must not render merchant logo')
  assert(!/<t-tag[^>]*visibilityLabel/.test(wxml), 'user reviews cards must not render public visibility tags')

  assert(!/\bt-rate\b|item\.rating|r\.rating/.test(wxml + ts + json), 'user reviews page must not render unsupported rating fields')
  assert(!/item\.tags|r\.tags|tag-row/.test(wxml + ts + wxss), 'user reviews page must not render unsupported review tags')
  assert(
    /static async updateReview\(id: number/.test(reviewApi) &&
      /method:\s*'PATCH'/.test(reviewApi) &&
      /static async deleteReview\(id: number/.test(reviewApi) &&
      /method:\s*'DELETE'/.test(reviewApi),
    'review API must expose backend-supported owner update/delete methods'
  )
  assert(
    /bindtap="onEditReview"/.test(wxml) &&
      /bindtap="onDeleteReview"/.test(wxml) &&
      /onEditReview/.test(ts) &&
      /onConfirmDeleteReview/.test(ts),
    'user reviews page must expose edit/delete management actions for owner reviews'
  )
  assert(
    /class="footer-tabs"/.test(wxml) &&
      /class="footer-tab footer-tab--delete/.test(wxml) &&
      /grid-template-columns:\s*repeat\(2,\s*1fr\)/.test(wxss),
    'user reviews edit/delete actions must render as two bottom tab buttons'
  )
  assert(
    /<t-dialog[\s\S]*confirm-btn="\{\{ \{ content: '确认删除', theme: 'danger', loading: deleteDialogSubmitting \} \}\}"/.test(wxml) &&
      /ReviewService\.deleteReview\(id\)/.test(ts),
    'delete review must use a TDesign confirmation dialog and call the real backend delete API'
  )
  assert(
    /\/pages\/user_center\/reviews\/create\/index\?reviewId=\$\{id\}/.test(ts),
    'edit review action must navigate to the review editor with reviewId'
  )

  assert(
    json.includes('"t-image"') && json.includes('"t-button"') && !json.includes('"t-tag"') && json.includes('"t-dialog"'),
    'user reviews page must declare the TDesign components it renders'
  )

  console.log('check-user-reviews-page: validated user review gutter, image rendering, preview, and backend-supported management actions')
}

main()
