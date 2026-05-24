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
    status: 'completed'
  })

  assert.strictEqual(loaded.requests.length, 1)
  assert.strictEqual(loaded.requests[0].url, '/v1/delivery/history')
  assert.deepStrictEqual(plain(loaded.requests[0].data), {
    page: 3,
    limit: 20,
    status: 'completed'
  })

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

  assert(pageTs.includes('buildHistoryParams'), 'history page should build params from current date range')
  assert(pageTs.includes('dateRangeLabel'), 'history page should expose a date range label')
  assert(pageTs.includes('onClearDateRange'), 'history page should allow returning to all records')
  assert(pageWxml.includes('bindtap="onOpenRangePicker"'), 'history page should open date range picker')
  assert(pageWxml.includes('<t-calendar'), 'history page should render a date range calendar')
  assert(pageWxml.includes('bind:confirm="onConfirmRangePicker"'), 'history page should handle calendar confirmation')
  assert(!pageWxml.includes('已显示最近半年的记录'), 'history page should not describe the list as recent half-year only')
  assert(pageWxml.includes('没有更多任务了'), 'history page should use all-history no-more copy')
  assert(pageJson.includes('"t-calendar"'), 'history page should declare t-calendar')
})().then(() => {
  console.log('check-rider-delivery-history-params tests passed')
}, (error) => {
  console.error(error)
  process.exit(1)
})
