const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const repoRoot = path.join(__dirname, '..')
const sourcePath = path.join(repoRoot, 'miniprogram', 'utils', 'request-auth-refresh.ts')
const requestPath = path.join(repoRoot, 'miniprogram', 'utils', 'request.ts')

function loadModule(options = {}) {
  const source = fs.readFileSync(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018
    }
  }).outputText

  const refreshToken = options.refreshToken || ''
  let requestCalls = 0
  let loginCalls = 0
  const tokenWrites = []
  const clearedTokens = []
  const timeoutHandles = []
  const clearedTimeoutHandles = []
  let nextTimeoutId = 1

  function trackedSetTimeout(callback, delay) {
    if (delay === 25000) {
      const handle = { id: nextTimeoutId += 1, callback, delay }
      timeoutHandles.push(handle)
      return handle
    }
    return setTimeout(callback, delay)
  }

  function trackedClearTimeout(handle) {
    if (handle && typeof handle === 'object' && handle.delay === 25000) {
      clearedTimeoutHandles.push(handle)
      return
    }
    clearTimeout(handle)
  }

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (modulePath === './auth') {
        return {
          clearToken() { clearedTokens.push(true) },
          getRefreshToken() { return refreshToken },
          hasToken() { return options.hasToken !== undefined ? options.hasToken : true },
          isTokenNearExpiry() { return options.nearExpiry !== undefined ? options.nearExpiry : true },
          setToken(token, expiresAt, nextRefreshToken) {
            tokenWrites.push({ token, expiresAt, refreshToken: nextRefreshToken })
          }
        }
      }
      if (modulePath === './wechat-login-session') {
        return {
          ensureWechatLoginSession() {
            loginCalls += 1
            return Promise.resolve(options.loginData || {
              access_token: 'login-access-token',
              refresh_token: 'login-refresh-token'
            })
          }
        }
      }
      if (modulePath === './logger') {
        return { logger: { debug() {}, info() {}, warn() {}, error() {} } }
      }
      if (modulePath === './error-handler') {
        class AppError extends Error {
          constructor(config) {
            super(config.userMessage || config.message)
            this.type = config.type
            this.userMessage = config.userMessage || config.message
            this.detailMessage = config.message
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
      request(requestOptions) {
        requestCalls += 1
        assert.strictEqual(requestOptions.url, 'https://api.example.test/v1/auth/refresh')
        assert.strictEqual(requestOptions.method, 'POST')
        assert.strictEqual(requestOptions.data.refresh_token, refreshToken)
        assert.strictEqual(requestOptions.timeout, 10000)
        setTimeout(() => {
          if (options.refreshFails) {
            requestOptions.fail({ errMsg: 'request:fail' })
            return
          }
          requestOptions.success({
            statusCode: 200,
            data: {
              code: options.refreshCode === undefined ? 0 : options.refreshCode,
              data: {
                access_token: 'refreshed-access-token',
                refresh_token: 'refreshed-refresh-token',
                access_token_expires_at: '2026-05-28T08:00:00Z'
              }
            }
          })
        }, 0)
      }
    },
    setTimeout: trackedSetTimeout,
    clearTimeout: trackedClearTimeout,
    Promise,
    Date,
    Error
  }

  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return {
    module: sandbox.module.exports,
    getRequestCalls: () => requestCalls,
    getLoginCalls: () => loginCalls,
    getTokenWrites: () => tokenWrites,
    getClearedTokens: () => clearedTokens,
    getRefreshTimeoutHandles: () => timeoutHandles,
    getClearedRefreshTimeoutHandles: () => clearedTimeoutHandles
  }
}

(async () => {
  const loaded = loadModule({ refreshToken: 'refresh-token-001' })
  await Promise.all([
    loaded.module.ensureValidToken(),
    loaded.module.ensureValidToken()
  ])

  assert.strictEqual(loaded.getRequestCalls(), 1)
  assert.strictEqual(loaded.getLoginCalls(), 0)
  assert.deepStrictEqual(loaded.getTokenWrites(), [{
    token: 'refreshed-access-token',
    expiresAt: new Date('2026-05-28T08:00:00Z').getTime(),
    refreshToken: 'refreshed-refresh-token'
  }])
  assert.strictEqual(loaded.getRefreshTimeoutHandles().length, 1)
  assert.strictEqual(loaded.getClearedRefreshTimeoutHandles().length, 1)

  const fallback = loadModule({ refreshToken: 'refresh-token-001', refreshCode: 401 })
  await fallback.module.refreshAuthToken(true)
  assert.strictEqual(fallback.getRequestCalls(), 1)
  assert.strictEqual(fallback.getLoginCalls(), 1)

  const noToken = loadModule({ hasToken: false })
  await noToken.module.ensureValidToken()
  assert.strictEqual(noToken.getRequestCalls(), 0)
  assert.strictEqual(noToken.getLoginCalls(), 1)

  const fresh = loadModule({ nearExpiry: false })
  await fresh.module.ensureValidToken()
  assert.strictEqual(fresh.getRequestCalls(), 0)
  assert.strictEqual(fresh.getLoginCalls(), 0)

  const requestSource = fs.readFileSync(requestPath, 'utf8')
  assert(requestSource.includes("from './request-auth-refresh'"), 'request.ts must consume request-auth-refresh owner')
  for (const pattern of [
    'let _refreshingPromise',
    'function performTokenRefresh',
    'function refreshTokenWithTimeout',
    'function refreshTokenOnce',
    'const REFRESH_TIMEOUT',
    'const TOKEN_REFRESH_REQUEST_TIMEOUT'
  ]) {
    assert(!requestSource.includes(pattern), `request.ts must not own ${pattern}`)
  }
})().then(() => {
  console.log('check-request-auth-refresh tests passed')
}, (error) => {
  console.error(error)
  process.exit(1)
})
