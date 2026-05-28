const fs = require('fs')
const path = require('path')

const repoRoot = path.resolve(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), 'utf8')
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}

function assertPlatformDetailFeedback(pagePath, handlerName) {
  const ts = read(`${pagePath}.ts`)
  const wxml = read(`${pagePath}.wxml`)

  assert(
    ts.includes('actionResultText') && ts.includes('actionResultNote'),
    `${pagePath}.ts must keep durable action result state`
  )
  assert(
    wxml.includes('actionResultText') && wxml.includes('actionResultNote'),
    `${pagePath}.wxml must render durable action result state`
  )
  assert(
    !new RegExp(`${handlerName}[\\s\\S]*wx\\.showToast\\(\\{ title: [^\\n]*已`).test(ts),
    `${pagePath}.ts must not use success Toast as the main post-action feedback`
  )
}

function main() {
  assertPlatformDetailFeedback('miniprogram/pages/platform/merchants/detail', 'onToggleStatus')
  assertPlatformDetailFeedback('miniprogram/pages/platform/operators/detail', 'onToggleStatus')
  assertPlatformDetailFeedback('miniprogram/pages/platform/riders/detail', 'onToggleAccepting')

  const printersTs = read('miniprogram/pages/merchant/printers/index.ts')
  const printersWxml = read('miniprogram/pages/merchant/printers/index.wxml')

  assert(
    printersTs.includes('commandResultText') && printersTs.includes('commandResultNote'),
    'merchant printers page must keep durable command result state'
  )
  assert(
    printersWxml.includes('commandResultText') && printersWxml.includes('commandResultNote'),
    'merchant printers page must render durable command result state'
  )
  assert(
    !printersTs.includes("wx.showToast({ title: '测试命令已发送'"),
    'printer test command must not use final-sounding success Toast as the main feedback'
  )

  console.log('check-action-feedback: validated durable platform action and printer command feedback')
}

main()
