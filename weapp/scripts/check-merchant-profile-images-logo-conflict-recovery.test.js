const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const repoRoot = path.join(__dirname, '..')
const helperPath = path.join(repoRoot, 'miniprogram', 'pages', 'merchant', '_utils', 'merchant-profile-images-view.ts')
const pagePath = path.join(repoRoot, 'miniprogram', 'pages', 'merchant', 'profile-images', 'index.ts')

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
      if (modulePath === '../../../utils/logger') {
        return {
          logger: {
            warn() {},
            error() {}
          }
        }
      }
      if (modulePath === '../../../utils/image') {
        return {
          getPublicImageUrl(value) {
            return typeof value === 'string' && value.trim() ? value.trim() : ''
          }
        }
      }
      if (modulePath === '../../../utils/user-facing') {
        return {
          getErrorUserMessage(error, fallback) {
            if (error && typeof error === 'object' && typeof error.userMessage === 'string' && error.userMessage.trim()) {
              return error.userMessage.trim()
            }
            return fallback
          }
        }
      }
      throw new Error(`unexpected require: ${modulePath}`)
    },
    wx: {
      getStorageSync() {
        return null
      }
    },
    Array,
    Date,
    JSON,
    Math,
    Number,
    Object,
    RegExp,
    Set,
    String
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: helperPath })
  return sandbox.module.exports
}

function main() {
  const pageSource = fs.readFileSync(pagePath, 'utf8')

  assert(
    pageSource.includes('getLogoPersistErrorMessage'),
    'profile-images page must route Logo persistence failures through a dedicated copy mapper'
  )
  assert(
    pageSource.includes('isLogoVersionConflictError'),
    'profile-images page must distinguish Logo PATCH version conflicts from generic upload failures'
  )
  assert(
    /isLogoVersionConflictError\(err\)[\s\S]+this\.loadData\(false,\s*true,\s*false,\s*\{\s*preferServerLogo:\s*true,\s*suppressRefreshErrorToast:\s*true\s*\}\)/.test(pageSource),
    'Logo version conflicts must trigger a forced server-truth profile refresh after local rollback'
  )
  assert(
    /async loadData\([^)]*options:\s*\{\s*preferServerLogo\?:\s*boolean,\s*suppressRefreshErrorToast\?:\s*boolean\s*\}/.test(pageSource) &&
      /const \{\s*preferServerLogo = false,\s*suppressRefreshErrorToast = false\s*\} = options/.test(pageSource) &&
      /const currentLogoImage = preferServerLogo \? null : this\.data\.logoImage/.test(pageSource),
    'Logo conflict refresh must bypass local Logo merge so backend truth can clear or replace the Logo'
  )
  assert(
    /if \(suppressRefreshErrorToast\) \{\s*this\.setData\(\{ loading: false \}\)\s*return\s*\}/.test(pageSource),
    'Logo conflict recovery refresh failures should be silent to avoid duplicate Toast feedback for one user action'
  )
  assert(
    /logoImage:\s*previousLogoImage/.test(pageSource) &&
      /logoFiles:\s*buildUploadRenderImages\(previousLogoImage \? \[previousLogoImage\] : \[\]/.test(pageSource),
    'Logo failure handling must restore the previously trusted Logo instead of keeping the uploaded local file'
  )

  const {
    getLogoPersistErrorMessage,
    isLogoVersionConflictError
  } = loadHelper()

  assert.strictEqual(
    isLogoVersionConflictError({ statusCode: 409 }),
    true,
    'HTTP 409 must be treated as a Logo version conflict'
  )
  assert.strictEqual(
    isLogoVersionConflictError({ code: 40901 }),
    true,
    'domain conflict codes in the 409xx range must be treated as Logo version conflicts'
  )
  assert.strictEqual(
    isLogoVersionConflictError({ statusCode: 500 }),
    false,
    'server errors must stay on the generic upload failure path'
  )

  assert.strictEqual(
    getLogoPersistErrorMessage({ statusCode: 409, userMessage: '操作冲突，请稍后重试' }),
    '资料已被其他操作更新，已恢复原 Logo，请重新上传',
    'Logo version conflict copy must explain rollback and safe retry without leaking raw conflict text'
  )
  assert.strictEqual(
    getLogoPersistErrorMessage({ statusCode: 500, userMessage: '服务暂时不可用，请稍后再试' }),
    '服务暂时不可用，请稍后再试',
    'non-conflict errors should continue to use the shared user-facing mapper'
  )

  console.log('check-merchant-profile-images-logo-conflict-recovery: validated Logo conflict rollback and recovery guard')
}

main()
