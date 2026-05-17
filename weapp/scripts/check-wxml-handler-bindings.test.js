const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')

const {
  collectScriptMethodNames,
  collectWxmlHandlerBindingFailures
} = require('./check-wxml-handler-bindings')

function writeFile(filePath, content) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true })
  fs.writeFileSync(filePath, content)
}

function withFixture(fn) {
  const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'weapp-handler-bindings-'))

  try {
    return fn(repoRoot)
  } finally {
    fs.rmSync(repoRoot, { recursive: true, force: true })
  }
}

withFixture((repoRoot) => {
  writeFile(
    path.join(repoRoot, 'weapp/miniprogram/pages/demo/index.wxml'),
    '<view bindtap="onExisting" bind:change="onMissing" />\n'
  )
  writeFile(
    path.join(repoRoot, 'weapp/miniprogram/pages/demo/index.ts'),
    'Page({\n  onExisting() {}\n})\n'
  )

  const failures = collectWxmlHandlerBindingFailures({
    repoRoot,
    wxmlFiles: ['weapp/miniprogram/pages/demo/index.wxml'],
    allowlist: {}
  })

  assert.strictEqual(failures.length, 1)
  assert.strictEqual(failures[0].handlerName, 'onMissing')
})

withFixture((repoRoot) => {
  writeFile(
    path.join(repoRoot, 'weapp/miniprogram/pages/demo/index.wxml'),
    '<view bindtap="{{disabled ? \'\' : \'onRuntimeTap\'}}" />\n'
  )
  writeFile(
    path.join(repoRoot, 'weapp/miniprogram/pages/demo/index.ts'),
    [
      "import { runtimeMethods } from '../../utils/runtime'",
      'Page({',
      '  data: {},',
      '  ...runtimeMethods',
      '})'
    ].join('\n')
  )
  writeFile(
    path.join(repoRoot, 'weapp/miniprogram/utils/runtime.ts'),
    'export const runtimeMethods = {\n  onRuntimeTap() {}\n}\n'
  )

  const failures = collectWxmlHandlerBindingFailures({
    repoRoot,
    wxmlFiles: ['weapp/miniprogram/pages/demo/index.wxml'],
    allowlist: {}
  })

  assert.deepStrictEqual(failures, [])
})

withFixture((repoRoot) => {
  const scriptPath = path.join(repoRoot, 'weapp/miniprogram/pages/demo/index.ts')
  writeFile(
    path.join(repoRoot, 'weapp/miniprogram/pages/demo/index.wxml'),
    '<custom-navbar bind:navheight="onNavHeight" />\n<t-upload bind:add="onAddImage" />\n'
  )
  writeFile(
    scriptPath,
    [
      'Page({',
      '  data: {',
      "    // Initialize with tomorrow's date by default",
      '    title: "demo"',
      '  },',
      '  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {',
      '    this.setData({ navBarHeight: e.detail.navBarHeight })',
      '  },',
      '  // 图片添加回调',
      '  async onAddImage(e: WechatMiniprogram.CustomEvent<{ files: Array<{ url: string }> }>) {',
      '    const { files } = e.detail',
      '    this.setData({ files })',
      '  }',
      '})'
    ].join('\n')
  )

  const methods = collectScriptMethodNames(repoRoot, scriptPath)
  assert(methods.has('onNavHeight'))
  assert(methods.has('onAddImage'))

  const failures = collectWxmlHandlerBindingFailures({
    repoRoot,
    wxmlFiles: ['weapp/miniprogram/pages/demo/index.wxml'],
    allowlist: {}
  })

  assert.deepStrictEqual(failures, [])
})

withFixture((repoRoot) => {
  const scriptPath = path.join(repoRoot, 'weapp/miniprogram/pages/demo/index.ts')
  writeFile(
    path.join(repoRoot, 'weapp/miniprogram/pages/demo/index.wxml'),
    '<t-button bindtap="onCompleteComplaint">确认完结</t-button>\n'
  )
  writeFile(
    scriptPath,
    [
      'Page({',
      '  onSubmitResponse() {',
      "    const jumpUrl = 'https://example.com'",
      '    if (jumpUrl && !/^https?:\\/\\//.test(jumpUrl)) {',
      '      wx.showToast({ title: "链接格式错误", icon: "none" })',
      '    }',
      '  },',
      '  async onCompleteComplaint() {',
      '    this.setData({ completing: true })',
      '  }',
      '})'
    ].join('\n')
  )

  const methods = collectScriptMethodNames(repoRoot, scriptPath)
  assert(methods.has('onSubmitResponse'))
  assert(methods.has('onCompleteComplaint'))

  const failures = collectWxmlHandlerBindingFailures({
    repoRoot,
    wxmlFiles: ['weapp/miniprogram/pages/demo/index.wxml'],
    allowlist: {}
  })

  assert.deepStrictEqual(failures, [])
})

console.log('check-wxml-handler-bindings tests passed')
