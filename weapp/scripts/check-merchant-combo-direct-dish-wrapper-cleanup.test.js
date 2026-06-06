const assert = require('assert')
const fs = require('fs')
const path = require('path')

const repoRoot = path.join(__dirname, '..')

const dishApiFiles = [
  'miniprogram/api/dish.ts',
  'miniprogram/pages/dine-in/_main_shared/api/dish.ts',
  'miniprogram/pages/merchant/_main_shared/api/dish.ts',
  'miniprogram/pages/payment/_main_shared/api/dish.ts',
  'miniprogram/pages/platform/_main_shared/api/dish.ts',
  'miniprogram/pages/takeout/combo-detail/_main_shared/api/dish.ts',
  'miniprogram/pages/takeout/dish-detail/_main_shared/api/dish.ts'
]

for (const relativePath of dishApiFiles) {
  const source = fs.readFileSync(path.join(repoRoot, relativePath), 'utf8')

  assert(
    source.includes('export interface ComboDishInput') &&
      source.includes('dishes?: ComboDishInput[]'),
    `${relativePath} must keep the supported full combo create/update dishes[] contract`
  )

  for (const symbol of [
    'AddComboDishBodyRequest',
    'addDishToCombo(',
    'removeDishFromCombo(',
    '/v1/combos/{id}/dishes',
    '/v1/combos/{id}/dishes/{dish_id}',
    '`/v1/combos/${comboId}/dishes`',
    '`/v1/combos/${comboId}/dishes/${dishId}`'
  ]) {
    assert(
      !source.includes(symbol),
      `${relativePath} should not expose legacy direct combo-dish wrapper symbol ${symbol}`
    )
  }
}

const runtimeCallSearchRoots = [
  'miniprogram/pages',
  'miniprogram/components',
  'miniprogram/utils'
]

for (const root of runtimeCallSearchRoots) {
  const absoluteRoot = path.join(repoRoot, root)
  const stack = [absoluteRoot]
  while (stack.length) {
    const current = stack.pop()
    const stat = fs.statSync(current)
    if (stat.isDirectory()) {
      for (const entry of fs.readdirSync(current)) {
        stack.push(path.join(current, entry))
      }
      continue
    }
    if (!/\.(ts|js)$/.test(current)) {
      continue
    }
    const source = fs.readFileSync(current, 'utf8')
    assert(
      !/\.(addDishToCombo|removeDishFromCombo)\s*\(/.test(source),
      `${path.relative(repoRoot, current)} should not call legacy direct combo-dish wrappers`
    )
  }
}

console.log('check-merchant-combo-direct-dish-wrapper-cleanup: legacy direct combo-dish wrappers are not exposed')
