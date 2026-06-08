const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const sourcePath = path.join(__dirname, '..', 'miniprogram', 'pages', 'rider', '_main_shared', 'api', 'appeals-customer-service.ts')
const allAppealServicePaths = [
  sourcePath,
  path.join(__dirname, '..', 'miniprogram', 'pages', 'merchant', '_main_shared', 'api', 'appeals-customer-service.ts'),
  path.join(__dirname, '..', 'miniprogram', 'pages', 'user_center', 'service_center', '_main_shared', 'api', 'appeals-customer-service.ts')
]

function plain(value) {
  return JSON.parse(JSON.stringify(value))
}

function loadModule() {
  for (const candidatePath of allAppealServicePaths) {
    const candidate = fs.readFileSync(candidatePath, 'utf8')
    assert(!candidate.includes('/v1/rider/appeals'), `${candidatePath} must not call removed /v1/rider/appeals routes`)
  }

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
      if (modulePath === '../../../../utils/request') {
        return {
          request(options) {
            requests.push(options)
            if (options.method === 'GET' && options.url === '/v1/rider/recovery-disputes') {
              return Promise.resolve({
                disputes: [{
                  id: 501,
                  claim_id: 301,
                  claim_type: 'damage',
                  claim_amount: 1200,
                  claim_description: '配送破损',
                  order_no: 'ORD301',
                  reason: '配送异常追偿申诉说明',
                  status: 'submitted',
                  created_at: '2026-06-08T00:00:00Z'
                }],
                total: 1,
                page_id: options.data.page_id,
                page_size: options.data.page_size,
                has_more: false
              })
            }
            if (options.method === 'GET' && options.url === '/v1/rider/recovery-disputes/501') {
              return Promise.resolve({
                id: 501,
                claim_id: 301,
                appellant_type: 'rider',
                claim_type: 'damage',
                claim_amount: 1200,
                claim_description: '配送破损',
                order_no: 'ORD301',
                order_amount: 3000,
                reason: '配送异常追偿申诉说明',
                status: 'submitted',
                created_at: '2026-06-08T00:00:00Z'
              })
            }
            if (options.method === 'POST' && options.url === '/v1/rider/recovery-disputes') {
              return Promise.resolve({
                id: 502,
                claim_id: options.data.claim_id,
                appellant_id: 201,
                appellant_type: 'rider',
                reason: options.data.reason,
                status: 'submitted',
                region_id: 1,
                created_at: '2026-06-08T00:00:00Z'
              })
            }
            throw new Error(`unexpected request: ${options.method} ${options.url}`)
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
  const service = loaded.module.appealManagementService

  const list = await service.getRiderAppeals({ page_id: 2, page_size: 20, status: 'submitted' })
  assert.strictEqual(loaded.requests[0].url, '/v1/rider/recovery-disputes')
  assert.strictEqual(loaded.requests[0].method, 'GET')
  assert.deepStrictEqual(plain(loaded.requests[0].data), {
    page_id: 2,
    page_size: 20,
    status: 'submitted'
  })
  assert.strictEqual(list.total, 1)
  assert.strictEqual(list.page_id, 2)
  assert.strictEqual(list.page_size, 20)
  assert.strictEqual(list.has_more, false)
  assert.strictEqual(list.appeals.length, 1)
  assert.strictEqual(list.appeals[0].id, 501)

  const detail = await service.getRiderAppealDetail(501)
  assert.strictEqual(loaded.requests[1].url, '/v1/rider/recovery-disputes/501')
  assert.strictEqual(detail.id, 501)

  const created = await service.createRiderAppeal({
    claim_id: 301,
    reason: '配送异常追偿申诉说明'
  })
  assert.strictEqual(loaded.requests[2].url, '/v1/rider/recovery-disputes')
  assert.strictEqual(loaded.requests[2].method, 'POST')
  assert.strictEqual(created.claim_id, 301)
})().then(() => {
  console.log('check-rider-claims-recovery-disputes-contract tests passed')
}, (error) => {
  console.error(error)
  process.exit(1)
})
