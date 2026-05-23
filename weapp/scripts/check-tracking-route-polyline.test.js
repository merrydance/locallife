const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const sourcePath = path.join(__dirname, '..', 'miniprogram', 'pages', 'orders', 'tracking', 'index.ts')

function loadPage({ directionResponse }) {
  const source = fs.readFileSync(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018
    }
  }).outputText

  let pageConfig
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (modulePath === '../../../api/delivery') {
        return {
          __esModule: true,
          default: {
            getRiderLocation() {
              return Promise.resolve(null)
            }
          },
          buildDeliveryProgress: () => [],
          getDeliveryStatusDisplay: () => ({})
        }
      }
      if (modulePath === '../../../api/location') {
        return {
          getBicyclingDirection() {
            return Promise.resolve(directionResponse)
          }
        }
      }
      if (modulePath === '../../../api/order') {
        return { getOrderDetail: () => Promise.resolve({}) }
      }
      if (modulePath === '../../../utils/logger') {
        return {
          logger: {
            warn() {},
            error() {},
            info() {},
            debug() {}
          }
        }
      }
      if (modulePath === '../../../utils/user-facing') {
        return { getErrorUserMessage: (_error, fallback) => fallback }
      }
      if (modulePath === '../../../services/order-receipt-confirmation') {
        return { confirmReceiptWithRecovery: () => Promise.resolve({ status: 'confirmed' }) }
      }
      throw new Error(`unexpected require: ${modulePath}`)
    },
    Page(config) {
      pageConfig = config
    },
    wx: {
      showToast() {},
      showLoading() {},
      hideLoading() {},
      navigateBack() {},
      makePhoneCall() {}
    },
    setInterval,
    clearInterval,
    Promise,
    Error,
    Number,
    Date
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })

  const page = {
    ...pageConfig,
    data: JSON.parse(JSON.stringify(pageConfig.data)),
    setData(patch) {
      this.data = { ...this.data, ...patch }
    }
  }

  return page
}

function plain(value) {
  return JSON.parse(JSON.stringify(value))
}

async function testUsesRoutePointsFromEnvelopeDirectionContract() {
  const merchantPoint = { latitude: 39.908722, longitude: 116.397499 }
  const customerPoint = { latitude: 39.914722, longitude: 116.404499 }
  const routePoint = { latitude: 39.910200, longitude: 116.400100 }
  const page = loadPage({
    directionResponse: {
      code: 0,
      data: {
        distance: 1200,
        duration: 500,
        points: [merchantPoint, routePoint, customerPoint]
      }
    }
  })

  await page.planRoute(merchantPoint, customerPoint)

  assert.strictEqual(page.data.polyline.length, 1)
  assert.deepStrictEqual(plain(page.data.polyline[0].points), [merchantPoint, routePoint, customerPoint])
  assert.strictEqual(page.data.polyline[0].dottedLine, false)
  assert.strictEqual(page.data.polyline[0].arrowLine, true)
}

async function testUsesRoutePointsFromUnwrappedDirectionData() {
  const merchantPoint = { latitude: 39.908722, longitude: 116.397499 }
  const customerPoint = { latitude: 39.914722, longitude: 116.404499 }
  const routePoint = { lat: 39.910200, lng: 116.400100 }
  const page = loadPage({
    directionResponse: {
      distance: 1200,
      duration: 500,
      points: [merchantPoint, routePoint, customerPoint]
    }
  })

  await page.planRoute(merchantPoint, customerPoint)

  assert.strictEqual(page.data.polyline.length, 1)
  assert.deepStrictEqual(plain(page.data.polyline[0].points), [
    merchantPoint,
    { latitude: routePoint.lat, longitude: routePoint.lng },
    customerPoint
  ])
  assert.strictEqual(page.data.polyline[0].dottedLine, false)
  assert.strictEqual(page.data.polyline[0].arrowLine, true)
}

async function testFallsBackWhenDirectionHasNoRoutePoints() {
  const merchantPoint = { latitude: 39.908722, longitude: 116.397499 }
  const customerPoint = { latitude: 39.914722, longitude: 116.404499 }
  const page = loadPage({
    directionResponse: {
      code: 0,
      data: {
        distance: 1200,
        duration: 500
      }
    }
  })

  await page.planRoute(merchantPoint, customerPoint)

  assert.deepStrictEqual(plain(page.data.polyline[0].points), [merchantPoint, customerPoint])
  assert.strictEqual(page.data.polyline[0].dottedLine, true)
}

async function testRouteProgressFollowsRiderLocation() {
  const merchantPoint = { latitude: 39.908722, longitude: 116.397499 }
  const routePoint = { latitude: 39.910200, longitude: 116.400100 }
  const customerPoint = { latitude: 39.914722, longitude: 116.404499 }
  const riderPoint = { latitude: 39.910190, longitude: 116.400090 }
  const page = loadPage({
    directionResponse: {
      code: 0,
      data: {
        distance: 1200,
        duration: 500,
        points: [merchantPoint, routePoint, customerPoint]
      }
    }
  })

  await page.planRoute(merchantPoint, customerPoint)
  page.renderRoutePolyline(page.data.routePoints, merchantPoint, customerPoint, riderPoint)

  assert.strictEqual(page.data.polyline.length, 1)
  assert.deepStrictEqual(plain(page.data.polyline[0].points), [riderPoint, customerPoint])
  assert.strictEqual(page.data.polyline[0].dottedLine, false)
  assert.strictEqual(page.data.polyline[0].arrowLine, true)
}

async function testRouteProgressWaitsUntilDeliveryIsTracked() {
  const merchantPoint = { latitude: 39.908722, longitude: 116.397499 }
  const routePoint = { latitude: 39.910200, longitude: 116.400100 }
  const customerPoint = { latitude: 39.914722, longitude: 116.404499 }
  const riderPoint = { latitude: 39.910190, longitude: 116.400090 }
  const page = loadPage({
    directionResponse: {
      code: 0,
      data: {
        distance: 1200,
        duration: 500,
        points: [merchantPoint, routePoint, customerPoint]
      }
    }
  })
  page.data.delivery = { status: 'assigned' }
  page.data.riderPoint = riderPoint

  await page.planRoute(merchantPoint, customerPoint)

  assert.deepStrictEqual(plain(page.data.polyline[0].points), [merchantPoint, routePoint, customerPoint])
}

(async () => {
  await testUsesRoutePointsFromEnvelopeDirectionContract()
  await testUsesRoutePointsFromUnwrappedDirectionData()
  await testFallsBackWhenDirectionHasNoRoutePoints()
  await testRouteProgressFollowsRiderLocation()
  await testRouteProgressWaitsUntilDeliveryIsTracked()
})().then(() => {
  console.log('check-tracking-route-polyline tests passed')
}, (error) => {
  console.error(error)
  process.exit(1)
})
