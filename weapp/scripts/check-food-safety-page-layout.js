const fs = require('fs')
const path = require('path')

const repoRoot = path.resolve(__dirname, '..')
const pageDir = path.join(repoRoot, 'miniprogram/pages/user_center/service_center/food-safety')
const wxmlPath = path.join(pageDir, 'index.wxml')
const wxssPath = path.join(pageDir, 'index.wxss')
const tsPath = path.join(pageDir, 'index.ts')
const apiPath = path.join(repoRoot, 'miniprogram/pages/user_center/service_center/_api/food-safety.ts')
const appWxssPath = path.join(repoRoot, 'miniprogram/app.wxss')

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

function getLastRule(css, selector) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const matches = [...css.matchAll(new RegExp(`${escaped}\\s*\\{([\\s\\S]*?)\\}`, 'g'))]
  const match = matches[matches.length - 1]
  return match ? match[1] : ''
}

function assertNotMatches(source, checks) {
  for (const [pattern, message] of checks) {
    assert(!pattern.test(source), message)
  }
}

function main() {
  const wxml = read(wxmlPath)
  const wxss = read(wxssPath)
  const ts = read(tsPath)
  const api = read(apiPath)
  const appWxss = read(appWxssPath)

  assert(
    !wxml.includes('page-shell--page-gutter'),
    'food safety report form should follow service_center/submit and avoid an extra outer page gutter'
  )
  assert(
    /page-shell--with-nav/.test(wxml) && /page-shell--bottom-safe/.test(wxml),
    'food safety page must keep shared nav and bottom safe-area shell classes'
  )
  assertNotMatches(wxml + '\n' + wxss, [
    [/style="[^"]*--td-/, 'food safety page must not keep inline TDesign style overrides'],
    [/color="#/, 'food safety page WXML must not hard-code icon colors'],
    [/background(?:-color)?:\s*#/, 'food safety page must use semantic background tokens'],
    [/(^|\s)color:\s*#/, 'food safety page must use semantic text color tokens'],
    [/box-shadow:\s*0 /, 'food safety page must use shared shadow tokens'],
    [/border-radius:\s*\d/, 'food safety page must use shared radius tokens'],
    [/margin-bottom:\s*\d/, 'food safety page must not keep ad hoc vertical spacing']
  ])
  assert(
    /page-shell--with-nav\s*\{[\s\S]*?var\(--page-shell-nav-gap,\s*var\(--spacer-sm\)\)/.test(appWxss),
    'shared page shell must expose --page-shell-nav-gap with spacer-sm fallback'
  )
  assert(
    /page-shell--page-gutter\s*\{[\s\S]*?var\(--page-shell-horizontal-gutter,\s*var\(--spacer-md\)\)/.test(appWxss),
    'shared page shell must expose --page-shell-horizontal-gutter with spacer-md fallback'
  )

  const pageRule = getRule(wxss, '.page-container')
  assert(
    /--page-shell-bottom-offset:\s*var\(--spacer-md\)/.test(pageRule),
    'food safety page uses an in-flow submit button and should keep only rhythmic bottom spacing'
  )
  assert(
    /--page-shell-nav-gap:\s*var\(--spacer-xs\)/.test(pageRule),
    'food safety page should set one compact 8rpx nav gap through the page shell variable'
  )
  assert(
    !/--page-shell-horizontal-gutter/.test(pageRule),
    'food safety report form should not define a second horizontal gutter variable'
  )
  assert(!/160rpx/.test(pageRule), 'food safety page must not keep fixed-action bottom reserve spacing')
  assert(
    !/padding(?:-top|-bottom|-left|-right)?\s*:/.test(pageRule),
    'food safety page container must not override page-shell padding directly'
  )
  assert(
    !/background\s*:|min-height\s*:/.test(pageRule),
    'food safety page container should not duplicate page-shell background or min-height'
  )

  const formRule = getRule(wxss, '.form-container')
  assert(
    /margin-top:\s*calc\(-1 \* var\(--spacer-sm\)\)/.test(formRule),
    'food safety form should follow the submit form top rhythm and remove the visible extra top gutter'
  )
  assert(
    /gap:\s*var\(--spacer-sm\)/.test(formRule),
    'food safety form should use the compact submit-form field rhythm'
  )
  assert(
    !/padding\s*:/.test(formRule) && !/padding-left\s*:|padding-right\s*:/.test(formRule),
    'food safety form must not add another horizontal padding over page-shell gutter'
  )

  assert(
    /class="field-group"/.test(wxml) && /class="field-group field-group--textarea"/.test(wxml),
    'food safety form should follow service_center/submit field-group structure'
  )
  assert(
    /class="[^"]*\border-summary\b[^"]*"[\s\S]*class="order-main"[\s\S]*class="order-title"[\s\S]*class="order-subtitle"/.test(wxml),
    'food safety order summary must keep merchant name on the first line and order number on the second line'
  )
  assert(
    !/<t-cell[\s\S]*title="\{\{merchantName\}\}"[\s\S]*note="订单 \{\{orderNo\}\}"/.test(wxml),
    'food safety order summary must not use a t-cell title/note row because that renders horizontally'
  )
  assert(!/class="order-card"|class="form-card"|class="form-panel"/.test(wxml), 'food safety page must not keep the old card-wall form structure')

  const fieldRule = getRule(wxss, '.field-group')
  assert(
    /background:\s*var\(--td-bg-color-container\)/.test(fieldRule) &&
      /overflow:\s*hidden/.test(fieldRule),
    'food safety field groups should use the TDesign form surface without extra page padding'
  )
  const orderSummaryRule = getRule(wxss, '.order-summary')
  assert(
    /display:\s*flex/.test(orderSummaryRule) &&
      /align-items:\s*center/.test(orderSummaryRule) &&
      /justify-content:\s*space-between/.test(orderSummaryRule) &&
      /padding:\s*var\(--spacer-sm\) var\(--spacer-md\)/.test(orderSummaryRule),
    'food safety order summary should preserve vertical order text within the field-group rhythm'
  )
  const orderMainRule = getRule(wxss, '.order-main')
  assert(
    /display:\s*flex/.test(orderMainRule) &&
      /flex-direction:\s*column/.test(orderMainRule),
    'food safety order summary text must remain stacked vertically'
  )
  const resultCardRule = getLastRule(wxss, '.result-card')
  assert(
    /padding:\s*var\(--spacer-xl\) var\(--spacer-lg\)/.test(resultCardRule),
    'food safety result card should own its larger result-state padding separately'
  )

  const optionRule = getRule(wxss, '.option-group')
  assert(
    /display:\s*flex/.test(optionRule) && /flex-direction:\s*column/.test(optionRule),
    'food safety radio options must be vertical full-width rows on small screens'
  )
  assert(
    !/grid-template-columns/.test(optionRule),
    'food safety radio options must not use multi-column grid layout'
  )
  assert(
    !/\.severity-group\s*\{[\s\S]*?grid-template-columns/.test(wxss),
    'food safety severity options must not reintroduce a multi-column grid'
  )
  assert(!/grid-template-columns/.test(wxss), 'food safety page must not keep unused grid layout styles')
  assert(!/gap\s*:/.test(optionRule), 'food safety radio group should not add another gap beyond radio row padding')

  assert(
    /block="\{\{true\}\}"/.test(wxml) && /max-label-row="\{\{1\}\}"/.test(wxml) && /t-class="food-radio"/.test(wxml),
    'food safety radio controls should render as compact full-width single-line TDesign rows'
  )
  const radioRule = getRule(wxss, '.food-radio')
  assert(
    /--td-radio-vertical-padding:\s*var\(--spacer-xs\) var\(--spacer-md\)/.test(radioRule),
    'food safety radio rows should use the same horizontal rhythm as TDesign form fields'
  )
  assert(/--td-radio-icon-size:\s*40rpx/.test(radioRule), 'food safety radio rows should use compact icon sizing')

  assert(
    /t-class="food-textarea"/.test(wxml) && /disableDefaultPadding="\{\{true\}\}"/.test(wxml),
    'food safety textarea should use a page-owned external class and disable native default padding'
  )
  assert(
    !/style="--td-textarea/.test(wxml),
    'food safety textarea must not keep inline TDesign style overrides'
  )
  const textareaRule = getRule(wxss, '.food-textarea')
  assert(
    /--td-textarea-padding:\s*var\(--spacer-xs\) var\(--spacer-md\) var\(--spacer-sm\)/.test(textareaRule) &&
      /--td-textarea-background-color:\s*var\(--td-bg-color-container\)/.test(textareaRule),
    'food safety textarea should follow the field-group surface without adding another inner card'
  )

  assert(!/submit-area/.test(wxml), 'food safety submit button should not keep an empty spacing wrapper')
  assert(!/\.submit-area\b/.test(wxss), 'food safety styles should not keep submit-area padding on top of flex gap')

  assert(
    /incident_type:\s*FoodSafetyIncidentType/.test(api) &&
      /description:\s*string/.test(api) &&
      /severity_level:\s*number/.test(api),
    'food safety API contract must keep backend-supported report fields'
  )
  assert(
    !/\btags\b|\brating\b/.test(ts) && !/\btags\b|\brating\b/.test(api),
    'food safety page must not keep unsupported rating or tag fields'
  )

  console.log('check-food-safety-page-layout: validated food safety gutter, radio layout, and backend-truth fields')
}

main()
