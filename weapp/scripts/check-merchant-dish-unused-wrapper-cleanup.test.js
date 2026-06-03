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

const removedSymbols = [
  'BatchUpdateDishStatusRequest',
  'BatchDishStatusResponse',
  'CheckInventoryRequest',
  'CheckInventoryResponse',
  'InventoryStatsResponse',
  'batchUpdateDishStatus(',
  'checkInventory(',
  'getInventoryStats('
]

for (const relativePath of dishApiFiles) {
  const source = fs.readFileSync(path.join(repoRoot, relativePath), 'utf8')
  for (const symbol of removedSymbols) {
    assert(
      !source.includes(symbol),
      `${relativePath} should not expose unused dish/inventory wrapper symbol ${symbol}`
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
      !/\.(batchUpdateDishStatus|checkInventory|getInventoryStats)\s*\(/.test(source),
      `${path.relative(repoRoot, current)} should not call removed unused dish/inventory wrappers`
    )
  }
}

const staleContractPath = path.join(repoRoot, 'scripts/check-inventory-stats-contract.test.js')
assert(
  !fs.existsSync(staleContractPath),
  'stale inventory stats wrapper contract script should be removed with the unused wrapper'
)

console.log('check-merchant-dish-unused-wrapper-cleanup: unused dish/inventory wrappers are not exposed')
