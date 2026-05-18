const fs = require('fs')
const path = require('path')
const vm = require('vm')
const ts = require('typescript')

const repoRoot = path.resolve(__dirname, '..')
const helperPath = path.join(repoRoot, 'miniprogram/utils/takeout-payment-error-copy.ts')

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
    require
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: helperPath })
  return sandbox.module.exports
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}

function main() {
  const {
    getTakeoutPaymentCreateFailedContent
  } = loadHelper()

  const settlementMissing = getTakeoutPaymentCreateFailedContent({
    userMessage: '商户结算账户未开通，暂不能创建支付订单'
  })
  assert(
    settlementMissing === '该商户资质不完整，暂不支持下单',
    'merchant settlement-account payment failure should use consumer-facing merchant qualification copy'
  )

  const channelPending = getTakeoutPaymentCreateFailedContent({
    message: '商户微信支付通道待开通，暂不能创建微信生态支付订单'
  })
  assert(
    channelPending === '该商户资质不完整，暂不支持下单',
    'merchant payment-channel pending failure should use the same consumer-facing qualification copy'
  )

  const unknown = getTakeoutPaymentCreateFailedContent(new Error('network timeout'))
  assert(
    unknown === '支付创建失败，请在订单详情页重新发起支付。',
    'unknown payment failures should keep the safe existing fallback'
  )

  console.log('check-takeout-payment-error-copy: validated takeout payment failure copy')
}

main()
