const assert = require('assert')
const fs = require('fs')
const path = require('path')

const weappRoot = path.join(__dirname, '..')
const repoRoot = path.join(weappRoot, '..')

const backendInventorySource = fs.readFileSync(path.join(repoRoot, 'locallife/api/inventory.go'), 'utf8')
assert(
  /type getInventoryStatsRequest[\s\S]*Date string `form:"date" binding:"required"`/.test(backendInventorySource),
  'backend inventory stats contract should require a single date query parameter'
)

const swaggerSource = fs.readFileSync(path.join(repoRoot, 'locallife/docs/swagger.yaml'), 'utf8')
const swaggerPathStart = swaggerSource.indexOf('  /v1/inventory/stats:')
assert.notStrictEqual(swaggerPathStart, -1, 'swagger should document /v1/inventory/stats')
const swaggerPathEnd = swaggerSource.indexOf('\n  /v1/', swaggerPathStart + 1)
const swaggerPathSection = swaggerSource.slice(swaggerPathStart, swaggerPathEnd === -1 ? undefined : swaggerPathEnd)
assert(/name:\s*date/.test(swaggerPathSection), 'swagger should expose the inventory stats date query')
assert(!/name:\s*start_date/.test(swaggerPathSection), 'swagger should not expose start_date for inventory stats')
assert(!/name:\s*end_date/.test(swaggerPathSection), 'swagger should not expose end_date for inventory stats')

const dishApiFiles = [
  'miniprogram/api/dish.ts',
  'miniprogram/pages/dine-in/_main_shared/api/dish.ts',
  'miniprogram/pages/merchant/_main_shared/api/dish.ts',
  'miniprogram/pages/payment/_main_shared/api/dish.ts',
  'miniprogram/pages/platform/_main_shared/api/dish.ts',
  'miniprogram/pages/takeout/combo-detail/_main_shared/api/dish.ts',
  'miniprogram/pages/takeout/dish-detail/_main_shared/api/dish.ts'
]

function inventoryStatsBlock(source, relativePath) {
  const start = source.indexOf('static async getInventoryStats(params:')
  assert.notStrictEqual(start, -1, `${relativePath} should expose getInventoryStats`)

  const endMarker = source.indexOf('\n// ==================== 顾客端菜品接口', start)
  return source.slice(start, endMarker === -1 ? start + 1600 : endMarker)
}

for (const relativePath of dishApiFiles) {
  const source = fs.readFileSync(path.join(weappRoot, relativePath), 'utf8')
  const block = inventoryStatsBlock(source, relativePath)

  assert(
    /getInventoryStats\(params:\s*\{\s*date:\s*string\s*\}/.test(block),
    `${relativePath} should accept only { date } for inventory stats`
  )
  assert(
    /Promise<InventoryStatsResponse>/.test(block),
    `${relativePath} should return the backend-aligned InventoryStatsResponse`
  )
  assert(!/\bstart_date\b/.test(block), `${relativePath} should not send start_date to inventory stats`)
  assert(!/\bend_date\b/.test(block), `${relativePath} should not send end_date to inventory stats`)

  for (const field of ['total_dishes', 'unlimited_dishes', 'sold_out_dishes', 'available_dishes']) {
    assert(source.includes(`${field}: number`), `${relativePath} should type ${field} in inventory stats response`)
  }

  for (const staleField of ['low_stock_dishes', 'out_of_stock_dishes', 'avg_daily_sales', 'total_sales']) {
    assert(!block.includes(staleField), `${relativePath} should not keep stale ${staleField} inventory stats response fields`)
  }
}

console.log('check-inventory-stats-contract tests passed')
