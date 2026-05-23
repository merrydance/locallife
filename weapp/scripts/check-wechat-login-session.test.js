const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const sourcePath = path.join(__dirname, '..', 'miniprogram', 'utils', 'wechat-login-session.ts')

function loadModule() {
  const source = fs.readFileSync(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018
    }
  }).outputText

  const tokenWrites = []
  let loginCalls = 0
  let requestCalls = 0

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (modulePath === './auth') {
        return {
          clearToken() {},
          setToken(token, expiresAt, refreshToken) {
            tokenWrites.push({ token, expiresAt, refreshToken })
          }
        }
      }
      if (modulePath === './device-id') {
        return { getDeviceId: () => 'device-001' }
      }
      if (modulePath === './logger') {
        return { logger: { debug() {}, info() {}, warn() {}, error() {} } }
      }
      if (modulePath === './error-handler') {
        class AppError extends Error {
          constructor(config, originalError) {
            super(config.userMessage || config.message)
            this.type = config.type
            this.userMessage = config.userMessage || config.message
            this.detailMessage = config.message
            this.originalError = originalError
          }
        }
        return { AppError, ErrorType: { AUTH: 'AUTH', NETWORK: 'NETWORK' } }
      }
      if (modulePath === '../api/types') {
        return { ErrorCode: { SUCCESS: 0 } }
      }
      if (modulePath === '../config/index') {
        return { API_CONFIG: { BASE_URL: 'https://api.example.test' } }
      }
      throw new Error(`unexpected require: ${modulePath}`)
    },
    wx: {
      login(options) {
        loginCalls += 1
        assert.strictEqual(options.timeout, 10000)
        setTimeout(() => options.success({ code: `wx-code-${loginCalls}` }), 0)
      },
      request(options) {
        requestCalls += 1
        assert.strictEqual(options.url, 'https://api.example.test/v1/auth/wechat-login')
        assert.strictEqual(options.data.code, 'wx-code-1')
        assert.strictEqual(options.data.device_id, 'device-001')
        assert.strictEqual(options.data.device_type, 'miniprogram')
        assert.strictEqual(options.timeout, 10000)
        setTimeout(() => options.success({
          statusCode: 200,
          data: {
            code: 0,
            message: 'ok',
            data: {
              access_token: 'access-token-001',
              refresh_token: 'refresh-token-001',
              access_token_expires_at: '2026-05-19T07:00:00Z',
              refresh_token_expires_at: '2026-05-19T08:00:00Z',
              session_id: 7,
              user: { id: 12, full_name: '微信用户', roles: ['customer'] }
            }
          }
        }), 0)
      }
    },
    setTimeout,
    clearTimeout,
    Date,
    Error
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return {
    module: sandbox.module.exports,
    getLoginCalls: () => loginCalls,
    getRequestCalls: () => requestCalls,
    getTokenWrites: () => tokenWrites
  }
}

(async () => {
  const loaded = loadModule()
  const { ensureWechatLoginSession } = loaded.module

  const [first, second] = await Promise.all([
    ensureWechatLoginSession(),
    ensureWechatLoginSession()
  ])

  assert.strictEqual(first, second)
  assert.strictEqual(loaded.getLoginCalls(), 1)
  assert.strictEqual(loaded.getRequestCalls(), 1)
  assert.deepStrictEqual(loaded.getTokenWrites(), [{
    token: 'access-token-001',
    expiresAt: new Date('2026-05-19T07:00:00Z').getTime(),
    refreshToken: 'refresh-token-001'
  }])
})().then(() => {
  console.log('check-wechat-login-session tests passed')
}, (error) => {
  console.error(error)
  process.exit(1)
})
