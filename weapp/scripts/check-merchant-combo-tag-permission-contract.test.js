const assert = require('assert')
const fs = require('fs')
const path = require('path')

const repoRoot = path.join(__dirname, '..')

const comboEditTsPath = path.join(repoRoot, 'miniprogram/pages/merchant/combos/edit/index.ts')
const comboEditWxmlPath = path.join(repoRoot, 'miniprogram/pages/merchant/combos/edit/index.wxml')

const comboEditTs = fs.readFileSync(comboEditTsPath, 'utf8')
const comboEditWxml = fs.readFileSync(comboEditWxmlPath, 'utf8')

assert(
  !comboEditWxml.includes('新增标签'),
  'merchant combo edit page must not expose combo-tag creation because backend POST /v1/tags is admin-only'
)

assert(
  !/bind:tap="onCreateTag"/.test(comboEditWxml),
  'merchant combo edit page must not bind an unsupported combo-tag creation action'
)

assert(
  !/title="新增套餐标签"/.test(comboEditWxml),
  'merchant combo edit page must not render a combo-tag creation dialog'
)

assert(
  !/TagService\.createTag\s*\(\s*\{[^}]*type:\s*['"]combo['"]/s.test(comboEditTs),
  'merchant combo edit page must not call admin-only TagService.createTag for combo tags'
)

assert(
  !/TagService\.createTag\s*\(/.test(comboEditTs),
  'merchant combo edit page must not call admin-only TagService.createTag through any payload shape'
)

assert(
  !/\btype:\s*['"]combo['"]/.test(comboEditTs),
  'merchant combo edit page must not keep combo tag creation payload fragments'
)

assert(
  !/\bonConfirmCreatePopup\s*\(/.test(comboEditTs),
  'merchant combo edit page should not keep dead combo-tag creation handlers'
)

console.log('check-merchant-combo-tag-permission-contract: combo tags are selection-only for merchant combo edit')
