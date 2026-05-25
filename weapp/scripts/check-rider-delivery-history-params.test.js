const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const sourcePath = path.join(__dirname, '..', 'miniprogram', 'api', 'delivery-task-management.ts')
const pageTsPath = path.join(__dirname, '..', 'miniprogram', 'pages', 'rider', 'tasks', 'index.ts')
const pageWxmlPath = path.join(__dirname, '..', 'miniprogram', 'pages', 'rider', 'tasks', 'index.wxml')
const pageJsonPath = path.join(__dirname, '..', 'miniprogram', 'pages', 'rider', 'tasks', 'index.json')

function plain(value) {
  return JSON.parse(JSON.stringify(value))
}

function loadModule() {
  const source = fs.readFileSync(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018
    }
  }).outputText

  const requests = []
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (modulePath === '../utils/request') {
        return {
          request(options) {
            requests.push(options)
            return Promise.resolve({
              deliveries: [],
              total: 0,
              completed_total: 0,
              total_earnings: 0,
              page_id: options.data.page,
              page_size: options.data.limit
            })
          }
        }
      }
      throw new Error(`unexpected require: ${modulePath}`)
    },
    Promise,
    Error,
    Number,
    String
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })

  return {
    module: sandbox.module.exports,
    requests
  }
}

(async () => {
  const loaded = loadModule()

  await loaded.module.deliveryTaskManagementService.getDeliveryHistory({
    page_id: 3,
    page_size: 20,
    status: 'picked'
  })

  assert.strictEqual(loaded.requests.length, 1)
  assert.strictEqual(loaded.requests[0].url, '/v1/delivery/history')
  assert.deepStrictEqual(plain(loaded.requests[0].data), {
    page: 3,
    limit: 20,
    status: 'picked'
  })

  const processService = loaded.module.deliveryProcessService
  assert.strictEqual(processService.getNextAction({ status: 'picking' }).action, 'confirm_pickup')
  assert.strictEqual(processService.getNextAction({ status: 'picked' }).action, 'start_delivery')
  assert.strictEqual(processService.getNextAction({ status: 'delivering' }).action, 'confirm_delivery')
  assert.strictEqual(processService.getNextAction({ status: 'delivered' }).action, 'none')

  await loaded.module.deliveryTaskManagementService.getDeliveryHistory({
    page_id: 1,
    page_size: 20
  })

  assert.strictEqual(loaded.requests.length, 2)
  assert.deepStrictEqual(plain(loaded.requests[1].data), {
    page: 1,
    limit: 20
  })

  await loaded.module.deliveryTaskManagementService.getDeliveryHistory({
    page_id: 2,
    page_size: 20,
    start_date: '2026-05-01',
    end_date: '2026-05-03'
  })

  assert.strictEqual(loaded.requests.length, 3)
  assert.deepStrictEqual(plain(loaded.requests[2].data), {
    page: 2,
    limit: 20,
    start_date: '2026-05-01',
    end_date: '2026-05-03'
  })

  const pageTs = fs.readFileSync(pageTsPath, 'utf8')
  const pageWxml = fs.readFileSync(pageWxmlPath, 'utf8')
  const pageJson = fs.readFileSync(pageJsonPath, 'utf8')

  assert(pageTs.includes('DEFAULT_HISTORY_DAYS = 30'), 'history page should use a bounded default range')
  assert(pageTs.includes('buildDefaultDateRange'), 'history page should build a default date range')
  assert(pageTs.includes('buildHistoryParams'), 'history page should build params from current date range')
  assert(pageTs.includes('const DEFAULT_DATE_RANGE = buildDefaultDateRange()'), 'history page should initialize from the bounded date range')
  assert(!pageTs.includes('EMPTY_DATE_RANGE'), 'history page must not default to an unbounded all-date request')
  assert(!pageTs.includes('dateRangeLabel'), 'history page should not keep the legacy date range label state')
  assert(!pageTs.includes('onClearDateRange'), 'history page should not allow unbounded all-date requests')
  assert(pageWxml.includes('bindtap="onOpenRangePicker"'), 'history page should open date range picker')
  assert(pageWxml.includes('range-card__body'), 'history page should use the unified range card layout')
  assert(pageWxml.includes('{{dateRange.start_date}}'), 'history page should render start date directly')
  assert(pageWxml.includes('{{dateRange.end_date}}'), 'history page should render end date directly')
  assert(pageWxml.includes('<t-calendar'), 'history page should render a date range calendar')
  assert(pageWxml.includes('bind:confirm="onConfirmRangePicker"'), 'history page should handle calendar confirmation')
  assert(!pageWxml.includes('日期范围'), 'history page should not render the legacy date range title')
  assert(!pageWxml.includes('全部日期'), 'history page should not render unbounded all-date copy')
  assert(!pageWxml.includes('onClearDateRange'), 'history page should not expose the legacy clear date action')
  assert(!pageWxml.includes('chevron-right'), 'history page should not render the legacy cell arrow')
  assert(!pageWxml.includes('已显示最近半年的记录'), 'history page should not describe the list as recent half-year only')
  assert(pageWxml.includes('没有更多任务了'), 'history page should use all-history no-more copy')
  assert(pageJson.includes('"t-calendar"'), 'history page should declare t-calendar')
})().then(() => {
  console.log('check-rider-delivery-history-params tests passed')
}, (error) => {
  console.error(error)
  process.exit(1)
})
