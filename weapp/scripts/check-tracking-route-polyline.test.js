const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const sourcePath = path.join(__dirname, '..', 'miniprogram', 'pages', 'orders', 'tracking', 'index.ts')

function getDeliveryStatusDisplay(status) {
  const trackedStatuses = new Set(['assigned', 'picking', 'picked', 'delivering'])
  const isAssignedStage = status === 'assigned' || status === 'picking'
  const isPickedStage = status === 'picked'
  const isDeliveringStage = status === 'delivering'
  return {
    isAssignedStage,
    isPickedStage,
    isDeliveringStage,
    isDeliveredStage: status === 'delivered' || status === 'completed',
    isLocationTracked: trackedStatuses.has(status),
    canConfirmReceipt: status === 'delivered',
    text: status || ''
  }
}

function loadPage({ directionResponse, riderLocation = null }) {
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
              return Promise.resolve(riderLocation)
            }
          },
          buildDeliveryProgress: () => [],
          getDeliveryStatusDisplay
        }
      }
      if (modulePath === '../../../api/location') {
        return {
          getBicyclingDirection() {
            return Promise.resolve(directionResponse)
          }
        }
      }
      if (modulePath === '../../../services/map') {
        return {
          mapService: {
            formatDistance(meters) {
              return meters < 1000 ? `${meters}米` : `${(meters / 1000).toFixed(1)}公里`
            },
            formatDuration(seconds) {
              if (!Number.isFinite(seconds) || seconds <= 0 || seconds < 60) return '不足1分钟'
              const minutes = Math.max(1, Math.round(seconds / 60))
              if (minutes < 60) return `${minutes}分钟`
              const hours = Math.floor(minutes / 60)
              const remainMinutes = minutes % 60
              return remainMinutes === 0 ? `${hours}小时` : `${hours}小时${remainMinutes}分钟`
            }
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
  page.data.delivery = { status: 'pending' }
  page.data.riderPoint = riderPoint

  await page.planRoute(merchantPoint, customerPoint)

  assert.deepStrictEqual(plain(page.data.polyline[0].points), [merchantPoint, routePoint, customerPoint])
}

async function testDeliveringShowsRiderMarkerAndRemainingToCustomer() {
  const riderPoint = {
    latitude: 39.906000,
    longitude: 116.396000,
    recorded_at: '2026-05-24T08:00:00.000Z'
  }
  const pickupPoint = { latitude: 39.908722, longitude: 116.397499 }
  const deliveryPoint = { latitude: 39.914722, longitude: 116.404499 }
  const routePoint = { latitude: 39.907100, longitude: 116.396800 }
  const page = loadPage({
    riderLocation: riderPoint,
    directionResponse: {
      code: 0,
      data: {
        distance: 1200,
        duration: 500,
        points: [riderPoint, routePoint, deliveryPoint]
      }
    }
  })
  page.data.deliveryId = 88
  page.data.delivery = {
    id: 88,
    status: 'delivering',
    pickup_latitude: pickupPoint.latitude,
    pickup_longitude: pickupPoint.longitude,
    delivery_latitude: deliveryPoint.latitude,
    delivery_longitude: deliveryPoint.longitude
  }
  page.data.markers = [
    page.buildMarker(1, pickupPoint, '商家', '/assets/merchant.png'),
    page.buildMarker(3, deliveryPoint, '我', '/assets/customer.png')
  ]
  page.data.merchantPoint = pickupPoint
  page.data.customerPoint = deliveryPoint

  await page.updateRiderLocation()

  assert(page.data.markers.some((marker) => marker.id === 2), 'rider marker should be visible while delivering')
  assert.strictEqual(page.data.remainingStageText, '距送达点')
  assert.strictEqual(page.data.remainingDistanceText, '1.2公里')
  assert.strictEqual(page.data.remainingDurationText, '8分钟')
  assert.deepStrictEqual(plain(page.data.polyline[0].points), [
    { latitude: riderPoint.latitude, longitude: riderPoint.longitude },
    routePoint,
    deliveryPoint
  ])
}

(async () => {
  await testUsesRoutePointsFromEnvelopeDirectionContract()
  await testUsesRoutePointsFromUnwrappedDirectionData()
  await testFallsBackWhenDirectionHasNoRoutePoints()
  await testRouteProgressFollowsRiderLocation()
  await testRouteProgressWaitsUntilDeliveryIsTracked()
  await testDeliveringShowsRiderMarkerAndRemainingToCustomer()
})().then(() => {
  console.log('check-tracking-route-polyline tests passed')
}, (error) => {
  console.error(error)
  process.exit(1)
})
