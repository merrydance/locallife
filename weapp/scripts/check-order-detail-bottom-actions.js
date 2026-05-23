const fs = require('fs')
const path = require('path')

const repoRoot = path.resolve(__dirname, '..')
const detailWxmlPath = path.join(repoRoot, 'miniprogram/pages/orders/detail/index.wxml')
const detailWxssPath = path.join(repoRoot, 'miniprogram/pages/orders/detail/index.wxss')

function assert(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}

function read(filePath) {
  return fs.readFileSync(filePath, 'utf8')
}

function getRule(css, selector) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const match = css.match(new RegExp(`${escaped}\\s*\\{([\\s\\S]*?)\\}`))
  return match ? match[1] : ''
}

function main() {
  const wxml = read(detailWxmlPath)
  const wxss = read(detailWxssPath)

  assert(
    /<scroll-view[^>]*class="bottom-actions"[^>]*scroll-x="\{\{true\}\}"/.test(wxml),
    'order detail bottom actions must use a horizontal scroll-view'
  )
  assert(
    /<view\s+class="bottom-actions-row">/.test(wxml),
    'order detail bottom actions must keep button groups inside a scrollable row wrapper'
  )

  const bottomActionsRule = getRule(wxss, '.bottom-actions')
  assert(/white-space:\s*nowrap/.test(bottomActionsRule), '.bottom-actions must prevent row wrapping inside horizontal scroll')

  const rowRule = getRule(wxss, '.bottom-actions-row')
  assert(/display:\s*inline-flex/.test(rowRule), '.bottom-actions-row must use inline-flex so the content can exceed viewport width')
  assert(/min-width:\s*100%/.test(rowRule), '.bottom-actions-row must keep the normal toolbar width when actions do not overflow')

  const secondaryRule = getRule(wxss, '.secondary-actions')
  const primaryRule = getRule(wxss, '.primary-actions')
  assert(/flex:\s*0\s+0\s+auto/.test(secondaryRule), '.secondary-actions must not shrink in the scroll row')
  assert(/flex:\s*0\s+0\s+auto/.test(primaryRule), '.primary-actions must not shrink in the scroll row')
  assert(!/margin-left:\s*auto/.test(primaryRule), '.primary-actions must not force overflow with margin-left:auto')

  const buttonRule = getRule(wxss, '.bottom-action-button')
  assert(/flex:\s*0\s+0\s+auto/.test(buttonRule), '.bottom action buttons must not shrink or collapse labels')

  console.log('check-order-detail-bottom-actions: validated scrollable order detail bottom actions')
}

main()
