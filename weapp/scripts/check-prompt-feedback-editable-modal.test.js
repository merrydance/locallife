const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const repoRoot = path.join(__dirname, '..')
const sourcePath = path.join(repoRoot, 'miniprogram/utils/prompt-feedback.ts')
const source = fs.readFileSync(sourcePath, 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2018,
    esModuleInterop: true
  }
}).outputText

let modalOptions = null
let hiddenToastCount = 0

const sandbox = {
  exports: {},
  module: { exports: {} },
  wx: {
    showToast(options) {
      return options
    },
    showModal(options) {
      modalOptions = options
      return options
    },
    hideToast() {
      hiddenToastCount += 1
    }
  },
  Date,
  String
}
sandbox.exports = sandbox.module.exports

vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
sandbox.module.exports.installPromptFeedbackGuards()

sandbox.wx.showModal({
  title: '新建品类',
  editable: true,
  placeholderText: '例如 家常菜'
})

assert.strictEqual(hiddenToastCount, 1, 'modal guard should still hide stale toast before showing modal')
assert.strictEqual(modalOptions.title, '新建品类')
assert.strictEqual(modalOptions.editable, true)
assert.strictEqual(modalOptions.placeholderText, '例如 家常菜')
assert(
  !Object.prototype.hasOwnProperty.call(modalOptions, 'content') || modalOptions.content === undefined,
  'editable modal without content must not receive fallback content'
)
assert.notStrictEqual(modalOptions.content, '请稍后再试', 'editable modal input must not be polluted by fallback copy')

console.log('prompt feedback editable modal checks passed')
