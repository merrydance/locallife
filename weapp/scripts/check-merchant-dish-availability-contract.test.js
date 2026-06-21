const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const root = path.join(__dirname, '..')

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

const unavailablePatch = view.buildDishEditLoadPatch({
  isEdit: true,
  detail: {
    id: 11,
    name: '不可售菜品',
    category_id: 2,
    category_name: '热菜',
    price: 1200,
    member_price: 0,
    is_online: true,
    is_available: false,
    prepare_time: 15,
    tags: []
  },
  categoryOptions: [{ label: '热菜', value: '2' }],
  currentCategoryId: 0,
  currentCategoryName: '',
  availableDishTags: [],
  selectedDishTagIds: [],
  customizationGroups: [],
  warningMessages: []
})

assert.strictEqual(
  unavailablePatch.formData.is_available,
  false,
  'dish edit load should preserve backend is_available=false'
)

const payload = view.buildDishSubmitPayload({
  formData: {
    name: '不可售菜品',
    description: '',
    category_id: 2,
    price: 1200,
    member_price: 0,
    is_online: true,
    is_available: false,
    sort_order: 0,
    prepare_time: 15,
    image_asset_id: 0,
    image_preview_url: ''
  },
  selectedDishTagIds: [],
  isEdit: true
})

assert.strictEqual(
  payload.is_available,
  false,
  'dish edit submit should send the merchant-selected is_available value'
)

assert.strictEqual(
  Object.prototype.hasOwnProperty.call(payload, 'is_packaging'),
  false,
  'dish edit submit should not send legacy is_packaging'
)

const wxmlPath = path.join(__dirname, '..', 'miniprogram/pages/merchant/dishes/edit/index.wxml')
const wxmlSource = fs.readFileSync(wxmlPath, 'utf8')
assert(
  wxmlSource.includes('title="是否可售"') &&
    wxmlSource.includes('value="{{formData.is_available}}"') &&
    wxmlSource.includes('data-field="is_available"'),
  'dish edit page should render a TDesign switch bound to is_available'
)

const dishEditTs = fs.readFileSync(path.join(root, 'miniprogram/pages/merchant/dishes/edit/index.ts'), 'utf8')
assert(
  dishEditTs.includes('包装菜品请在包装设置中维护') &&
    dishEditTs.includes("wx.redirectTo({ url: '/pages/merchant/packaging/index' })"),
  'dish edit page should redirect legacy packaging dishes to packaging settings'
)

const dishListTs = fs.readFileSync(path.join(root, 'miniprogram/pages/merchant/dishes/index.ts'), 'utf8')
const dishListWxml = fs.readFileSync(path.join(root, 'miniprogram/pages/merchant/dishes/index.wxml'), 'utf8')
assert(
  dishListTs.includes('if (targetDish?.is_packaging) {') &&
    dishListTs.includes('/pages/merchant/packaging/index'),
  'dish list should route legacy packaging dishes to packaging settings instead of legacy dish edit'
)
assert(
  dishListTs.includes('targetDish.is_packaging') &&
    dishListTs.includes('onRequestDeleteDish'),
  'dish list should guard legacy packaging dishes before opening the delete flow'
)
assert(
    dishListWxml.includes("item.is_packaging ? '包装设置' : '编辑'") &&
    dishListWxml.includes('disabled="{{item.deletePending || item.statusPending || item.is_packaging}}"'),
  'dish list should not expose edit/delete affordances that send legacy packaging dishes into dish management'
)

console.log('check-merchant-dish-availability-contract: merchant dish availability is preserved and editable')
