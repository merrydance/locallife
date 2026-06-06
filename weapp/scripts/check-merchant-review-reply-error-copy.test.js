const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const repoRoot = path.join(__dirname, '..')
const helperPath = path.join(repoRoot, 'miniprogram', 'pages', 'merchant', '_utils', 'merchant-review-reply-error.ts')
const pagePath = path.join(repoRoot, 'miniprogram', 'pages', 'merchant', 'reviews', 'index.ts')

function loadHelper() {
  const source = fs.readFileSync(helperPath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018,
      strict: true
    }
  }).outputText

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (modulePath === '../../../utils/user-facing') {
        return {
          getErrorDebugMessage(error) {
            if (typeof error === 'string') return error
            if (!error || typeof error !== 'object') return ''
            return [
              error.detailMessage,
              error.message,
              error.data && error.data.message,
              error.data && error.data.error,
              error.body && error.body.message,
              error.originalError && error.originalError.message,
              error.errMsg
            ].find((value) => typeof value === 'string' && value.trim()) || ''
          },
          getErrorUserMessage(error, fallback) {
            if (error && typeof error === 'object' && typeof error.userMessage === 'string' && error.userMessage.trim()) {
              return error.userMessage.trim()
            }
            if (typeof error === 'string' && /[\u4e00-\u9fff]/.test(error)) {
              return error.trim()
            }
            return fallback
          }
        }
      }
      throw new Error(`unexpected require: ${modulePath}`)
    },
    Array,
    JSON,
    Number,
    Object,
    RegExp,
    String
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: helperPath })
  return sandbox.module.exports
}

function main() {
  const pageSource = fs.readFileSync(pagePath, 'utf8')
  assert(
    pageSource.includes('getMerchantReviewReplyErrorMessage'),
    'merchant reviews page must route reply submission failures through the review-reply copy mapper'
  )

  const { getMerchantReviewReplyErrorMessage } = loadHelper()

  assert.strictEqual(
    getMerchantReviewReplyErrorMessage({
      statusCode: 400,
      userMessage: '请求参数错误',
      detailMessage: '参数错误(400): missing wechat openid'
    }),
    '当前微信登录信息不完整，请重新登录后再回复',
    'missing WeChat OpenID must not leak backend identity terminology'
  )

  assert.strictEqual(
    getMerchantReviewReplyErrorMessage({
      statusCode: 400,
      code: 40011,
      userMessage: '请求参数错误',
      detailMessage: '参数错误(40011): text content safety check failed'
    }),
    '回复内容未通过安全检查，请调整后再提交',
    'text content-safety failures must explain the merchant reply recovery action'
  )

  assert.strictEqual(
    getMerchantReviewReplyErrorMessage({
      statusCode: 502,
      userMessage: '服务暂时不可用,请稍后重试',
      detailMessage: '网关错误(502): wechat msg sec check: upstream timeout'
    }),
    '内容安全检查暂不可用，请稍后重试',
    'WeChat content-safety provider failures must not expose provider diagnostics'
  )

  assert.strictEqual(
    getMerchantReviewReplyErrorMessage({
      statusCode: 502,
      userMessage: '服务暂时不可用,请稍后重试',
      detailMessage: '网关错误(502)'
    }),
    '内容安全检查暂不可用，请稍后重试',
    'reply API gateway failures should keep content-safety unavailable copy even when the request wrapper hides provider detail'
  )

  assert.strictEqual(
    getMerchantReviewReplyErrorMessage({
      statusCode: 403,
      userMessage: '当前无权限执行该操作',
      detailMessage: '权限不足(403): review does not belong to your merchant'
    }),
    '当前无权限执行该操作',
    'unrelated safe user messages should continue to pass through the shared copy mapper'
  )

  console.log('check-merchant-review-reply-error-copy: validated merchant review reply failure copy')
}

main()
