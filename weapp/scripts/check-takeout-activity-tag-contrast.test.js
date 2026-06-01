const assert = require('assert')
const fs = require('fs')
const path = require('path')

const rootDir = path.join(__dirname, '..')

const takeoutHomeWxml = fs.readFileSync(
  path.join(rootDir, 'miniprogram/pages/takeout/index.wxml'),
  'utf8'
)
const takeoutHomeWxss = fs.readFileSync(
  path.join(rootDir, 'miniprogram/pages/takeout/index.wxss'),
  'utf8'
)
const wantedWxml = fs.readFileSync(
  path.join(rootDir, 'miniprogram/pages/takeout/wanted-merchants/index.wxml'),
  'utf8'
)
const wantedWxss = fs.readFileSync(
  path.join(rootDir, 'miniprogram/pages/takeout/wanted-merchants/index.wxss'),
  'utf8'
)

function getCssRuleBlock(content, selector) {
  const start = content.indexOf(`${selector} {`)
  assert(start >= 0, `${selector} rule should exist`)
  const openBrace = content.indexOf('{', start)
  let depth = 0

  for (let index = openBrace; index < content.length; index += 1) {
    const char = content[index]
    if (char === '{') depth += 1
    if (char === '}') depth -= 1
    if (depth === 0) {
      return content.slice(openBrace + 1, index)
    }
  }

  assert.fail(`${selector} rule should close`)
}

const activityBadgeRule = getCssRuleBlock(takeoutHomeWxss, '.activity-card-badge')
const regionChipRule = getCssRuleBlock(wantedWxss, '.region-chip')

assert(
  !/background:\s*var\(--td-brand-color-light\)/.test(activityBadgeRule) &&
    !/color:\s*var\(--td-brand-color\)/.test(activityBadgeRule),
  'takeout activity badge must not use brand-light background with brand text; marketing badges need stronger contrast'
)

assert(
  /background:\s*#1f1714/.test(activityBadgeRule) &&
    /color:\s*#ffffff/.test(activityBadgeRule),
  'takeout activity badge should render as a high-contrast badge'
)

assert(
  !/<t-tag[^>]*theme=["']primary["'][^>]*variant=["']light["'][^>]*>\{\{regionName\}\}<\/t-tag>/.test(wantedWxml),
  'wanted merchant region label must not use primary light tag; region is metadata, not a brand-color marketing badge'
)

assert(
  /class=["']region-chip["']/.test(wantedWxml),
  'wanted merchant page should render regionName with the neutral region chip'
)

assert(
  /background:\s*#ffffff/.test(regionChipRule) &&
    /color:\s*#4b5563/.test(regionChipRule),
  'wanted merchant region chip should use neutral high-contrast foreground/background colors'
)

console.log('check-takeout-activity-tag-contrast tests passed')
