const assert = require('assert')
const fs = require('fs')
const path = require('path')

const repoRoot = path.join(__dirname, '..')

const merchantTagApiPath = path.join(repoRoot, 'miniprogram/pages/merchant/_main_shared/api/dish.ts')
const platformTagApiPath = path.join(repoRoot, 'miniprogram/pages/platform/_main_shared/api/dish.ts')
const comboEditTsPath = path.join(repoRoot, 'miniprogram/pages/merchant/combos/edit/index.ts')
const comboEditWxmlPath = path.join(repoRoot, 'miniprogram/pages/merchant/combos/edit/index.wxml')
const dishEditTsPath = path.join(repoRoot, 'miniprogram/pages/merchant/dishes/edit/index.ts')
const tableEditTsPath = path.join(repoRoot, 'miniprogram/pages/merchant/tables/edit/index.ts')

const merchantTagApi = fs.readFileSync(merchantTagApiPath, 'utf8')
const platformTagApi = fs.readFileSync(platformTagApiPath, 'utf8')
const comboEditTs = fs.readFileSync(comboEditTsPath, 'utf8')
const comboEditWxml = fs.readFileSync(comboEditWxmlPath, 'utf8')
const dishEditTs = fs.readFileSync(dishEditTsPath, 'utf8')
const tableEditTs = fs.readFileSync(tableEditTsPath, 'utf8')

assert(
  /static async listTags[\s\S]*url:\s*['"]\/v1\/merchant\/tags['"]/.test(merchantTagApi),
  'merchant TagService.listTags must read merchant selectable tags from /v1/merchant/tags'
)

assert(
  /static async createTag[\s\S]*url:\s*['"]\/v1\/merchant\/tags['"]/.test(merchantTagApi),
  'merchant TagService.createTag must create/link merchant selectable tags through /v1/merchant/tags'
)

assert(
  /static async listTags[\s\S]*url:\s*['"]\/v1\/tags['"]/.test(platformTagApi) &&
    /static async createTag[\s\S]*url:\s*['"]\/v1\/tags['"]/.test(platformTagApi),
  'platform TagService must keep using admin tag endpoints under /v1/tags'
)

assert(
  dishEditTs.includes("TagService.createTag({ name, type: 'dish' })"),
  'merchant dish edit page should keep inline dish-tag creation wired through merchant TagService'
)

assert(
  tableEditTs.includes("TagService.createTag({ name, type: 'table' })"),
  'merchant table edit page should keep inline table-tag creation wired through merchant TagService'
)

assert(
  comboEditWxml.includes('新增套餐标签') &&
    /bind:tap="onCreateComboTag"/.test(comboEditWxml),
  'merchant combo edit page must expose combo-tag creation from the tag section'
)

assert(
  /title="新增套餐标签"/.test(comboEditWxml) &&
    /bind:confirm="onConfirmCreateTagDialog"/.test(comboEditWxml),
  'merchant combo edit page must render a combo-tag creation dialog'
)

assert(
  /TagService\.createTag\s*\(\s*\{[^}]*name[^}]*type:\s*['"]combo['"]/s.test(comboEditTs),
  'merchant combo edit page must create combo tags through merchant TagService'
)

assert(
  /selectedTagIds:[\s\S]*created\.id/.test(comboEditTs) || /nextTagIds[\s\S]*created\.id/.test(comboEditTs),
  'merchant combo edit page should select a newly created combo tag immediately'
)

console.log('check-merchant-combo-tag-permission-contract: merchant selectable tags use merchant endpoints and inline creation')
