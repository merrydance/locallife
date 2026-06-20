const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const root = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(root, ...relativePath.split('/')), 'utf8')
}

function loadModule(relativePath, stubs = {}) {
  const sourcePath = path.join(root, 'miniprogram', ...relativePath.split('/'))
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
    Set,
    Date,
    JSON,
    Promise
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const appJson = JSON.parse(read('miniprogram/app.json'))
const merchantPackage = appJson.subPackages.find((item) => item.root === 'pages/merchant')
assert(merchantPackage, 'merchant subpackage should be registered')
assert(
  merchantPackage.pages.includes('packaging/index'),
  'merchant packaging settings page route should be registered'
)

const packagingPageTs = read('miniprogram/pages/merchant/packaging/index.ts')
assert(
  packagingPageTs.includes('../_main_shared/api/packaging') ||
    packagingPageTs.includes('../../_main_shared/api/packaging'),
  'packaging page should import the merchant packaging API wrapper'
)
assert(
  packagingPageTs.includes('ensureMerchantPackagingManagementAccess') &&
    !packagingPageTs.includes('ensureMerchantConsoleAccess'),
  'packaging page should use the owner/manager packaging permission gate instead of broad merchant console access'
)
assert(
  packagingPageTs.includes('syncCurrentMerchantContext') &&
    packagingPageTs.includes('merchantId'),
  'packaging page should resolve and retain the selected merchant context before calling packaging APIs'
)
assert(
  packagingPageTs.includes('validatePackagingDraft'),
  'packaging page should validate duplicate option names locally before save'
)
assert(
  packagingPageTs.includes('buildPackagingSaveFailurePatch'),
  'packaging page should preserve draft through the extracted save-failure patch helper'
)
assert(
  packagingPageTs.includes('../_utils/merchant-packaging-settings-view'),
  'packaging page should keep packaging draft validation and save ordering in the task-domain helper'
)

const packagingApi = read('miniprogram/pages/merchant/_main_shared/api/packaging.ts')
for (const expected of [
  '/v1/merchant/packaging-settings',
  '/v1/merchant/packaging-options'
]) {
  assert(packagingApi.includes(expected), `packaging API wrapper should call ${expected}`)
}
assert(
  packagingApi.includes('X-Merchant-ID') &&
    packagingApi.includes('merchant_id'),
  'packaging API wrapper should send selected merchant context through header/query'
)

const consoleAccessTs = read('miniprogram/utils/console-access.ts')
assert(
  consoleAccessTs.includes('ensureMerchantPackagingManagementAccess') &&
    consoleAccessTs.includes('包装设置仅支持老板或店长管理'),
  'console access helpers should expose a packaging-specific owner/manager permission gate'
)

const configPageTs = read('miniprogram/pages/merchant/config/index.ts')
assert(
  configPageTs.includes('canManageMerchantPackaging') &&
    configPageTs.includes('PACKAGING_MANAGE_ITEM_IDS') &&
    configPageTs.includes('syncCurrentMerchantContext'),
  'merchant config page should hide packaging settings for staff roles that backend packaging routes reject'
)

async function verifyPackagingApiCarriesMerchantContext() {
  const calls = []
  const packagingApiModule = loadModule('pages/merchant/_main_shared/api/packaging.ts', {
    '../../../../utils/request': {
      request: async (options) => {
        calls.push(options)
        if (options.url === '/v1/merchant/packaging-options' && options.method === 'GET') {
          return { options: [], total: 0, page: 1, limit: 20, total_pages: 0 }
        }
        return {
          merchant_id: 42,
          enabled: false,
          required: true,
          applicable_order_types: ['takeout'],
          id: 7,
          name: '餐盒',
          description: '',
          price: 100,
          is_enabled: true,
          sort_order: 0
        }
      }
    }
  })

  await packagingApiModule.MerchantPackagingService.getSettings({ merchantId: 42 })
  await packagingApiModule.MerchantPackagingService.listOptions({ merchantId: 42 })
  await packagingApiModule.MerchantPackagingService.updateSettings({
    enabled: true,
    required: true,
    applicable_order_types: ['takeout'],
    default_option_id: null
  }, { merchantId: 42 })
  await packagingApiModule.MerchantPackagingService.createOption({
    name: '餐盒',
    description: '',
    price: 100,
    is_enabled: true,
    sort_order: 0
  }, { merchantId: 42 })
  await packagingApiModule.MerchantPackagingService.updateOption(7, {
    name: '餐盒',
    description: '',
    price: 100,
    is_enabled: true,
    sort_order: 0
  }, { merchantId: 42 })
  await packagingApiModule.MerchantPackagingService.deleteOption(7, { merchantId: 42 })

  assert.strictEqual(calls.length, 6, 'packaging API contract check should cover all packaging routes')
  for (const call of calls) {
    assert.strictEqual(
      call.header && call.header['X-Merchant-ID'],
      '42',
      `${call.method} ${call.url} should include X-Merchant-ID`
    )
  }
  assert.strictEqual(calls[0].data.merchant_id, 42, 'settings GET should include merchant_id query for request single-flight isolation')
  assert.strictEqual(calls[1].data.merchant_id, 42, 'options GET should include merchant_id query for request single-flight isolation')
  assert.strictEqual(
    Object.prototype.hasOwnProperty.call(calls[2].data, 'merchant_id'),
    false,
    'settings PUT should keep merchant context out of the request body'
  )
}

const dishEditView = loadModule('pages/merchant/_utils/merchant-dish-edit-view.ts', {
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

const payload = dishEditView.buildDishSubmitPayload({
  formData: {
    name: '牛肉饭',
    description: '',
    category_id: 2,
    price: 1800,
    member_price: 0,
    is_online: true,
    is_available: true,
    is_packaging: true,
    sort_order: 0,
    prepare_time: 12,
    image_asset_id: 0,
    image_preview_url: ''
  },
  selectedDishTagIds: [],
  isEdit: true
})
assert.strictEqual(
  Object.prototype.hasOwnProperty.call(payload, 'is_packaging'),
  false,
  'dish edit submit payload should not send legacy is_packaging'
)

const dishEditWxml = read('miniprogram/pages/merchant/dishes/edit/index.wxml')
assert(
  !dishEditWxml.includes('data-field="is_packaging"') &&
    !dishEditWxml.includes('包装菜品'),
  'dish edit page should no longer render the legacy packaging switch'
)

const packagingView = loadModule('pages/merchant/_utils/merchant-packaging-settings-view.ts', {
  '../_main_shared/api/packaging': {
    MerchantPackagingService: {}
  },
  '../../../utils/user-facing': {
    getErrorUserMessage: (_err, fallback) => fallback
  }
})

assert.strictEqual(
  typeof packagingView.validatePackagingDraft,
  'function',
  'packaging page should export validatePackagingDraft for contract verification'
)
assert.strictEqual(
  typeof packagingView.buildPackagingSaveFailurePatch,
  'function',
  'packaging page should export buildPackagingSaveFailurePatch for draft-preservation verification'
)

assert.throws(
  () => packagingView.validatePackagingDraft({
    enabled: true,
    required: true,
    applicable_order_types: ['takeout'],
    default_option_id: 0,
    options: [
      { local_id: 'a', id: 1, name: '餐盒', description: '', price_yuan: '1', is_enabled: true, sort_order: 0, deleted: false },
      { local_id: 'b', id: 2, name: ' 餐盒 ', description: '', price_yuan: '2', is_enabled: true, sort_order: 1, deleted: false }
    ]
  }),
  /包装名称不能重复/,
  'packaging page should block duplicate active option names locally'
)

assert.throws(
  () => packagingView.validatePackagingDraft({
    enabled: true,
    required: true,
    applicable_order_types: ['takeout'],
    default_option_id: 0,
    options: [
      { local_id: 'a', id: 1, name: 'Box', description: '', price_yuan: '1', is_enabled: true, sort_order: 0, deleted: false },
      { local_id: 'b', id: 2, name: 'box', description: '', price_yuan: '2', is_enabled: true, sort_order: 1, deleted: false }
    ]
  }),
  /包装名称不能重复/,
  'packaging page should match backend lower(name) uniqueness when validating option names'
)

const draft = {
  enabled: true,
  required: true,
  applicable_order_types: ['takeout'],
  default_option_id: 0,
  options: [
    { local_id: 'a', id: 1, name: '餐盒', description: '', price_yuan: '1', is_enabled: true, sort_order: 0, deleted: false }
  ]
}
const failurePatch = packagingView.buildPackagingSaveFailurePatch(new Error('network'), draft)
assert.strictEqual(failurePatch.saving, false, 'save failure should clear saving state')
assert.strictEqual(failurePatch.hasChanges, true, 'save failure should keep dirty state')
assert.deepStrictEqual(failurePatch.form, draft, 'save failure should preserve the current draft')
assert(
  String(failurePatch.saveErrorMessage || '').includes('保存失败'),
  'save failure should expose an inline retry message'
)

async function verifySaveSequenceClearsDefaultBeforeDeletingOption() {
  const calls = []
  const sequencedView = loadModule('pages/merchant/_utils/merchant-packaging-settings-view.ts', {
    '../_main_shared/api/packaging': {
      MerchantPackagingService: {
        updateSettings: async (data) => {
          calls.push(['updateSettings', data])
          return { merchant_id: 1, ...data }
        },
        updateOption: async (id, data) => {
          calls.push(['updateOption', id, data])
          return { id, merchant_id: 1, ...data }
        },
        createOption: async (data) => {
          calls.push(['createOption', data])
          return { id: 9, merchant_id: 1, ...data }
        },
        deleteOption: async (id) => {
          calls.push(['deleteOption', id])
          return { id, merchant_id: 1, name: '餐盒', description: '', price: 100, is_enabled: false, sort_order: 0 }
        }
      }
    },
    '../../../utils/user-facing': {
      getErrorUserMessage: (_err, fallback) => fallback
    }
  })

  assert.strictEqual(
    typeof sequencedView.savePackagingDraft,
    'function',
    'packaging helper should export savePackagingDraft for save-order verification'
  )

  await sequencedView.savePackagingDraft({
    enabled: true,
    required: true,
    applicable_order_types: ['takeout'],
    default_option_id: 1,
    options: [
      { local_id: 'a', id: 1, name: '餐盒', description: '', price_yuan: '1', is_enabled: true, sort_order: 0, deleted: true },
      { local_id: 'b', id: 2, name: '纸袋', description: '', price_yuan: '0.5', is_enabled: true, sort_order: 1, deleted: false }
    ]
  })

  assert.deepStrictEqual(
    calls.map((item) => item[0]),
    ['updateSettings', 'deleteOption', 'updateOption'],
    'saving should clear default settings before deleting/updating packaging options'
  )
  assert.strictEqual(
    calls[0][1].default_option_id,
    null,
    'first settings save should clear default option when the selected default is being deleted'
  )
}

async function verifyPartialCreateFailureKeepsSavedOptionIdForRetry() {
  const calls = []
  let failFinalSettings = true
  const sequencedView = loadModule('pages/merchant/_utils/merchant-packaging-settings-view.ts', {
    '../_main_shared/api/packaging': {
      MerchantPackagingService: {
        updateSettings: async (data) => {
          calls.push(['updateSettings', data])
          if (data.enabled && failFinalSettings) {
            throw new Error('final settings failed')
          }
          return { merchant_id: 1, ...data }
        },
        createOption: async (data) => {
          calls.push(['createOption', data])
          return { id: 9, merchant_id: 1, ...data }
        },
        updateOption: async (id, data) => {
          calls.push(['updateOption', id, data])
          return { id, merchant_id: 1, ...data }
        },
        deleteOption: async (id) => {
          calls.push(['deleteOption', id])
          return { id, merchant_id: 1, name: '餐盒', description: '', price: 100, is_enabled: false, sort_order: 0 }
        }
      }
    },
    '../../../utils/user-facing': {
      getErrorUserMessage: (_err, fallback) => fallback
    }
  })

  const form = {
    enabled: true,
    required: true,
    applicable_order_types: ['takeout'],
    default_option_id: 0,
    options: [
      { local_id: 'new-a', id: 0, name: '餐盒', description: '', price_yuan: '1', is_enabled: true, sort_order: 0, deleted: false }
    ]
  }

  await assert.rejects(
    () => sequencedView.savePackagingDraft(form),
    /final settings failed/,
    'save should surface final settings failure after creating an option'
  )
  assert.strictEqual(
    form.options[0].id,
    9,
    'partial save failure should retain the created option id in the retry draft'
  )

  failFinalSettings = false
  await sequencedView.savePackagingDraft(form)

  assert.strictEqual(
    calls.filter((item) => item[0] === 'createOption').length,
    1,
    'retry after partial create success should not create the same option again'
  )
  assert(
    calls.some((item) => item[0] === 'updateOption' && item[1] === 9),
    'retry after partial create success should update the saved option id'
  )
}

async function verifySaveSequenceDefersEnableUntilNewOptionExists() {
  const calls = []
  const sequencedView = loadModule('pages/merchant/_utils/merchant-packaging-settings-view.ts', {
    '../_main_shared/api/packaging': {
      MerchantPackagingService: {
        updateSettings: async (data) => {
          calls.push(['updateSettings', data])
          return { merchant_id: 1, ...data }
        },
        createOption: async (data) => {
          calls.push(['createOption', data])
          return { id: 9, merchant_id: 1, ...data }
        },
        updateOption: async (id, data) => {
          calls.push(['updateOption', id, data])
          return { id, merchant_id: 1, ...data }
        },
        deleteOption: async (id) => {
          calls.push(['deleteOption', id])
          return { id, merchant_id: 1, name: '餐盒', description: '', price: 100, is_enabled: false, sort_order: 0 }
        }
      }
    },
    '../../../utils/user-facing': {
      getErrorUserMessage: (_err, fallback) => fallback
    }
  })

  await sequencedView.savePackagingDraft({
    enabled: true,
    required: true,
    applicable_order_types: ['takeout'],
    default_option_id: 0,
    options: [
      { local_id: 'new-a', id: 0, name: '餐盒', description: '', price_yuan: '1', is_enabled: true, sort_order: 0, deleted: false }
    ]
  })

  assert.deepStrictEqual(
    calls.map((item) => item[0]),
    ['updateSettings', 'createOption', 'updateSettings'],
    'saving a new enabled packaging setup should write safe settings, create option, then enable settings'
  )
  assert.strictEqual(
    calls[0][1].enabled,
    false,
    'first settings save should not enable packaging before the first enabled option exists'
  )
  assert.strictEqual(
    calls[2][1].enabled,
    true,
    'final settings save should enable packaging after the option is persisted'
  )
}

async function verifySaveSequencePassesMerchantContext() {
  const contexts = []
  const sequencedView = loadModule('pages/merchant/_utils/merchant-packaging-settings-view.ts', {
    '../_main_shared/api/packaging': {
      MerchantPackagingService: {
        updateSettings: async (data, context) => {
          contexts.push(['updateSettings', context])
          return { merchant_id: 42, ...data }
        },
        createOption: async (data, context) => {
          contexts.push(['createOption', context])
          return { id: 9, merchant_id: 42, ...data }
        },
        updateOption: async (id, data, context) => {
          contexts.push(['updateOption', context])
          return { id, merchant_id: 42, ...data }
        },
        deleteOption: async (id, context) => {
          contexts.push(['deleteOption', context])
          return { id, merchant_id: 42, name: '餐盒', description: '', price: 100, is_enabled: false, sort_order: 0 }
        },
        listOptions: async (context) => {
          contexts.push(['listOptions', context])
          return []
        }
      }
    },
    '../../../utils/user-facing': {
      getErrorUserMessage: (_err, fallback) => fallback
    }
  })

  await sequencedView.savePackagingDraft({
    enabled: true,
    required: true,
    applicable_order_types: ['takeout'],
    default_option_id: 0,
    options: [
      { local_id: 'new-a', id: 0, name: '餐盒', description: '', price_yuan: '1', is_enabled: true, sort_order: 0, deleted: false }
    ]
  }, { merchantId: 42 })

  assert(
    contexts.length >= 3,
    'save flow should call settings and option APIs while preserving merchant context'
  )
  for (const [name, context] of contexts) {
    assert.deepStrictEqual(context, { merchantId: 42 }, `${name} should receive selected merchant context`)
  }
}

const packagingWxml = read('miniprogram/pages/merchant/packaging/index.wxml')
assert(
  packagingWxml.includes('item.id > 0 && form.default_option_id === item.id') &&
    packagingWxml.includes('disabled="{{saving || !item.is_enabled || item.id <= 0}}"'),
  'packaging page should only render/select a default option after it has a persisted id'
)
assert(
  packagingWxml.includes('disabled="{{saving}}"') &&
    packagingPageTs.includes('if (this.data.saving) return'),
  'packaging page should block mutating draft controls while save is in flight'
)
assert(
  packagingPageTs.includes('const savedForm = await savePackagingDraft(currentForm, { merchantId: this.data.merchantId })') &&
    packagingPageTs.includes('refreshErrorMessage: \'包装设置已保存，但最新状态同步失败，请稍后重新进入确认\''),
  'packaging page should save with merchant context, keep the saved draft snapshot, and surface a safe re-entry state when reload fails'
)

verifySaveSequenceClearsDefaultBeforeDeletingOption()
  .then(verifySaveSequenceDefersEnableUntilNewOptionExists)
  .then(verifyPartialCreateFailureKeepsSavedOptionIdForRetry)
  .then(verifyPackagingApiCarriesMerchantContext)
  .then(verifySaveSequencePassesMerchantContext)
  .then(() => {
    console.log('check-merchant-packaging-settings-contract: merchant packaging page contract is wired')
  })
  .catch((err) => {
    console.error(err)
    process.exit(1)
  })
