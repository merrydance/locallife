const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

function loadModule(relativePath, stubs = {}) {
  const sourcePath = path.join(__dirname, '..', 'miniprogram', ...relativePath.split('/'))
  const source = fs.readFileSync(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018,
      esModuleInterop: true
    }
  }).outputText

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (stubs[modulePath]) {
        return stubs[modulePath]
      }
      throw new Error(`unexpected require: ${modulePath}`)
    },
    Math,
    Number,
    String,
    Array,
    Set
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const view = loadModule('pages/merchant/_utils/merchant-dish-edit-view.ts', {
  '../_main_shared/api/dish': {
    TagService: {
      listCustomizationTags: async () => [],
      createTag: async ({ name }) => ({ id: 1, name })
    }
  },
  '../../../utils/image': {
    getPublicImageUrl: (url) => url
  }
})

assert.strictEqual(
  typeof view.buildDishPartialSaveRecoveryPatch,
  'function',
  'dish edit view should expose a patch builder for partial-save recovery state'
)
assert.strictEqual(
  typeof view.buildDishSaveRecoveredPatch,
  'function',
  'dish edit view should expose a patch builder that clears partial-save recovery state'
)

const tagRecoveryPatch = view.buildDishPartialSaveRecoveryPatch({
  dishId: 42,
  step: 'featured_tags'
})

assert.deepStrictEqual(
  {
    dishId: tagRecoveryPatch.dishId,
    isEdit: tagRecoveryPatch.isEdit,
    partialSavePending: tagRecoveryPatch.partialSavePending
  },
  {
    dishId: 42,
    isEdit: true,
    partialSavePending: true
  },
  'partial-save recovery should keep the newly saved dish editable and mark recovery as pending'
)
assert(
  tagRecoveryPatch.partialSaveMessage.includes('标签') &&
    tagRecoveryPatch.partialSaveMessage.includes('重试保存'),
  'featured-tag failures should leave a visible retry message about tag sync'
)

const customizationRecoveryPatch = view.buildDishPartialSaveRecoveryPatch({
  dishId: 42,
  step: 'customizations'
})
assert(
  customizationRecoveryPatch.partialSaveMessage.includes('规格') &&
    customizationRecoveryPatch.partialSaveMessage.includes('重试保存'),
  'customization failures should leave a visible retry message about customization sync'
)

const recoveredPatch = view.buildDishSaveRecoveredPatch()
assert.strictEqual(
  recoveredPatch.partialSavePending,
  false,
  'successful retry should clear the partial-save pending flag'
)
assert.strictEqual(
  recoveredPatch.partialSaveMessage,
  '',
  'successful retry should clear the partial-save recovery message'
)

const pageTsPath = path.join(__dirname, '..', 'miniprogram/pages/merchant/dishes/edit/index.ts')
const pageSource = fs.readFileSync(pageTsPath, 'utf8')
assert(
  pageSource.includes('partialSavePending') &&
    pageSource.includes('partialSaveMessage'),
  'dish edit page data should include partial-save recovery state'
)
assert(
  pageSource.includes('buildDishPartialSaveRecoveryPatch') &&
    pageSource.includes('buildDishSaveRecoveredPatch'),
  'dish edit submit flow should wire partial-save recovery patch builders'
)
assert(
  pageSource.includes("syncStep = 'featured_tags'") &&
    pageSource.includes("syncStep = 'customizations'"),
  'dish edit submit flow should track which post-base-save step failed'
)

const wxmlPath = path.join(__dirname, '..', 'miniprogram/pages/merchant/dishes/edit/index.wxml')
const wxmlSource = fs.readFileSync(wxmlPath, 'utf8')
assert(
  wxmlSource.includes('partialSavePending') &&
    wxmlSource.includes('partialSaveMessage') &&
    wxmlSource.includes('<t-notice-bar'),
  'dish edit page should render a persistent TDesign notice for partial-save recovery'
)

console.log('check-merchant-dish-edit-partial-save-recovery: partial save recovery state is visible and retryable')
