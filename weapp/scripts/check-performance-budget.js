const assert = require('assert')
const fs = require('fs')
const path = require('path')

const rootDir = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(rootDir, relativePath), 'utf8')
}

const appTs = read('miniprogram/app.ts')
const appJson = JSON.parse(read('miniprogram/app.json'))
const takeoutIndexTs = read('miniprogram/pages/takeout/index.ts')
const navbarTs = read('miniprogram/components/custom-navbar/index.ts')
const userCenterTs = read('miniprogram/pages/user_center/index.ts')

assert(
  !appTs.includes('this.clearApiCache()'),
  'App.onLaunch must not clear API cache on every cold start'
)

const takeoutPreload = appJson.preloadRule?.['pages/takeout/index']?.packages || []
assert(
  !takeoutPreload.includes('__APP__'),
  'takeout home preloadRule should not preload __APP__; it should target likely next-step subpackages'
)

for (const expectedPackage of [
  'pages/takeout/restaurant-detail',
  'pages/takeout/dish-detail',
  'pages/takeout/cart'
]) {
  assert(
    takeoutPreload.includes(expectedPackage),
    `takeout home should preload ${expectedPackage}`
  )
}

assert(
  /TAKEOUT_HYDRATION_MERCHANT_LIMIT\s*=\s*3/.test(takeoutIndexTs),
  'takeout feed hydration should be bounded to the first three merchants'
)

assert(
  /shouldRefreshCartDisplay/.test(takeoutIndexTs),
  'takeout home cart refresh should be guarded by a freshness budget'
)

assert(
  !/console\.(log|debug|info|warn|error)\(/.test(navbarTs),
  'custom navbar should not use direct console logging on location updates'
)

assert(
  !/console\.log\('\[UserCenter\]/.test(userCenterTs),
  'user center should not log full user profile objects on show'
)

console.log('Performance budget checks passed')
