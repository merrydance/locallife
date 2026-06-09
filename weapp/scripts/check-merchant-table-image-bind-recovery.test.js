const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const ROOT = path.resolve(__dirname, '..')

function loadTsModule(relativePath, requireStub = () => ({}), extraSandbox = {}) {
  const sourcePath = path.join(ROOT, relativePath)
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
    require: requireStub,
    console,
    ...extraSandbox
  }

  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

function clone(value) {
  return JSON.parse(JSON.stringify(value))
}

function applySetData(target, patch) {
  for (const [key, value] of Object.entries(patch)) {
    if (!key.includes('.')) {
      target.data[key] = value
      continue
    }

    const parts = key.split('.')
    let cursor = target.data
    for (let index = 0; index < parts.length - 1; index += 1) {
      const part = parts[index]
      if (!cursor[part] || typeof cursor[part] !== 'object') {
        cursor[part] = {}
      }
      cursor = cursor[part]
    }
    cursor[parts[parts.length - 1]] = value
  }
}

function loadSharedTableHelpers() {
  return loadTsModule('miniprogram/pages/merchant/_utils/merchant-tables-shared.ts', (id) => {
    if (id === '../../../api/table-device-management') {
      return {}
    }
    if (id === '../_main_shared/api/table') {
      return {
        getTableStatusDisplay(status) {
          return {
            normalizedStatus: status || 'available',
            label: '空闲',
            theme: 'success',
            canRelease: false,
            canShowCode: true,
            isAvailableLike: true
          }
        }
      }
    }
    if (id === '../../../utils/image') {
      return {
        getPublicImageUrl(value) {
          return typeof value === 'string' && value.trim() ? value.trim() : ''
        }
      }
    }
    throw new Error(`unexpected shared table helper require: ${id}`)
  }, {
    Promise,
    Array,
    Date,
    JSON,
    Math,
    Number,
    Object,
    RegExp,
    Set,
    String
  })
}

function loadTableEditView(sharedHelpers) {
  return loadTsModule('miniprogram/pages/merchant/tables/shared/table-edit-view.ts', (id) => {
    if (id === '../../_main_shared/api/table') {
      return {}
    }
    if (id === '../../../../api/table-device-management') {
      return {}
    }
    if (id === '../../_utils/merchant-tables-shared') {
      return sharedHelpers
    }
    throw new Error(`unexpected table edit view require: ${id}`)
  }, {
    Promise,
    Array,
    JSON,
    Math,
    Number,
    Object,
    Set,
    String
  })
}

function loadTableEditPage(tableManagementService, events) {
  const sharedHelpers = loadSharedTableHelpers()
  const tableEditView = loadTableEditView(sharedHelpers)
  let pageConfig

  loadTsModule('miniprogram/pages/merchant/tables/edit/index.ts', (id) => {
    if (id === '../../../../api/table-device-management') {
      return { tableManagementService }
    }
    if (id === '../../_main_shared/api/table') {
      return {}
    }
    if (id === '../../_main_shared/api/dish') {
      return { TagService: { listTags: async () => [], createTag: async ({ name }) => ({ id: 1, name }) } }
    }
    if (id === '../../../../utils/responsive') {
      return { getStableBarHeights: () => ({ navBarHeight: 88 }) }
    }
    if (id === '../../../../utils/logger') {
      return { logger: { error() {}, warn() {}, info() {} } }
    }
    if (id === '../../../../utils/user-facing') {
      return { getErrorUserMessage: (_err, fallback) => fallback }
    }
    if (id === '../../../../utils/promise') {
      return {
        settleAll: (promises) => Promise.allSettled(promises),
        isSettledFulfilled: (result) => result && result.status === 'fulfilled'
      }
    }
    if (id === '../../_utils/merchant-tables-shared') {
      return sharedHelpers
    }
    if (id === '../shared/table-edit-view') {
      return tableEditView
    }
    throw new Error(`unexpected table edit page require: ${id}`)
  }, {
    Page(config) {
      pageConfig = config
    },
    getCurrentPages() {
      return [{}, { refreshAll: async () => events.previousRefreshes.push(true) }]
    },
    wx: {
      showToast(payload) {
        events.toasts.push(payload)
      },
      showLoading(payload) {
        events.loadings.push(payload)
      },
      hideLoading() {
        events.hideLoadingCount += 1
      },
      navigateBack() {
        events.navigateBackCount += 1
      },
      showModal() {},
      openSetting() {}
    },
    Promise,
    Array,
    Date,
    JSON,
    Math,
    Number,
    Object,
    RegExp,
    Set,
    String,
    setTimeout,
    clearTimeout
  })

  assert(pageConfig, 'table edit page should register itself')
  return pageConfig
}

function createPageContext(page, overrides = {}) {
  const data = {
    ...clone(page.data),
    initialLoading: false,
    bootstrapped: true,
    isEdit: false,
    tableId: 0,
    formData: {
      table_no: 'A01',
      table_type: 'table',
      capacity: 4,
      description: '',
      minimum_spend_yuan: '',
      status: 'available',
      tag_ids: []
    },
    coverUploadFiles: [{
      url: 'https://cdn.example.test/table-cover.jpg',
      localPath: 'wxfile://cover.jpg',
      mediaId: 501,
      status: 'done'
    }],
    galleryUploadFiles: [{
      url: 'https://cdn.example.test/table-gallery.jpg',
      localPath: 'wxfile://gallery.jpg',
      mediaId: 502,
      status: 'done'
    }],
    ...overrides
  }

  return {
    ...page,
    data,
    setData(patch) {
      applySetData(this, patch)
    }
  }
}

async function assertCreateFailureStaysOnEditableRecoveryPage() {
  const events = {
    toasts: [],
    loadings: [],
    hideLoadingCount: 0,
    navigateBackCount: 0,
    previousRefreshes: []
  }
  const calls = []
  const failingMediaIds = new Set([501])
  const tableManagementService = {
    createTable: async (payload) => {
      calls.push({ type: 'createTable', payload })
      return {
        id: 88,
        merchant_id: 7,
        table_no: payload.table_no,
        table_type: payload.table_type,
        capacity: payload.capacity,
        status: 'available',
        created_at: '2026-06-09T00:00:00Z',
        updated_at: '2026-06-09T00:00:00Z'
      }
    },
    updateTable: async (tableId, payload) => {
      calls.push({ type: 'updateTable', tableId, payload })
      return { id: tableId, ...payload }
    },
    uploadTableImage: async (tableId, payload) => {
      calls.push({ type: 'uploadTableImage', tableId, payload })
      if (failingMediaIds.has(payload.media_asset_id)) {
        throw new Error('provider rejected table image binding')
      }
      return {
        id: payload.media_asset_id === 501 ? 9901 : 9902,
        table_id: tableId,
        media_asset_id: payload.media_asset_id,
        image_url: payload.media_asset_id === 501
          ? 'https://cdn.example.test/table-cover-bound.jpg'
          : 'https://cdn.example.test/table-gallery-bound.jpg',
        is_primary: !!payload.is_primary
      }
    },
    getTableDetail: async () => ({}),
    getTableImages: async () => ({ images: [] })
  }
  const page = loadTableEditPage(tableManagementService, events)
  const ctx = createPageContext(page)

  await page.onSubmit.call(ctx)

  assert.strictEqual(events.navigateBackCount, 0, 'partial image binding failure must keep the newly created table page open')
  assert.strictEqual(ctx.data.isEdit, true, 'partial image binding failure should convert the page into edit mode')
  assert.strictEqual(ctx.data.tableId, 88, 'partial image binding failure should keep the created table id for retry')
  assert.strictEqual(ctx.data.imageBindRecoveryPending, true, 'partial image binding failure should mark recovery as pending')
  assert(
    typeof ctx.data.imageBindRecoveryMessage === 'string' &&
      ctx.data.imageBindRecoveryMessage.includes('部分图片') &&
      ctx.data.imageBindRecoveryMessage.includes('再次保存'),
    'partial image binding failure should leave a persistent retry message'
  )
  assert.strictEqual(
    ctx.data.coverUploadFiles[0].mediaId,
    501,
    'pending cover media id must remain on the page for a later binding retry'
  )
  assert.strictEqual(
    ctx.data.coverUploadFiles[0].imageId,
    undefined,
    'failed binding must remain pending rather than pretending the image was persisted'
  )
  assert.strictEqual(
    ctx.data.galleryUploadFiles[0].imageId,
    9902,
    'successful gallery binding should be marked persisted before retry'
  )

  failingMediaIds.clear()
  await page.onSubmit.call(ctx)

  assert.strictEqual(events.navigateBackCount, 1, 'successful retry should return to the table list')
  assert.strictEqual(ctx.data.imageBindRecoveryPending, false, 'successful retry should clear the recovery pending flag')
  assert.strictEqual(ctx.data.imageBindRecoveryMessage, '', 'successful retry should clear the recovery message')
  assert.strictEqual(ctx.data.coverUploadFiles[0].imageId, 9901, 'successful retry should mark the image as persisted')
  assert.strictEqual(ctx.data.galleryUploadFiles[0].imageId, 9902, 'retry should keep the already persisted gallery image')
  assert(
    calls.some((call) => call.type === 'updateTable' && call.tableId === 88),
    'retry should still save current table metadata through the edit endpoint before returning'
  )
  assert.deepStrictEqual(
    calls.filter((call) => call.type === 'uploadTableImage').map((call) => ({
      tableId: call.tableId,
      media_asset_id: call.payload.media_asset_id,
      is_primary: call.payload.is_primary
    })),
    [
      { tableId: 88, media_asset_id: 501, is_primary: true },
      { tableId: 88, media_asset_id: 502, is_primary: undefined },
      { tableId: 88, media_asset_id: 501, is_primary: true }
    ],
    'retry should bind only the failed pending cover after the gallery image was persisted'
  )
}

async function main() {
  await assertCreateFailureStaysOnEditableRecoveryPage()

  const pageSource = fs.readFileSync(path.join(ROOT, 'miniprogram/pages/merchant/tables/edit/index.ts'), 'utf8')
  assert(
    pageSource.includes('imageBindRecoveryPending') && pageSource.includes('imageBindRecoveryMessage'),
    'table edit page state should include persistent image binding recovery fields'
  )

  const wxmlSource = fs.readFileSync(path.join(ROOT, 'miniprogram/pages/merchant/tables/edit/index.wxml'), 'utf8')
  assert(
    wxmlSource.includes('imageBindRecoveryPending') &&
      wxmlSource.includes('imageBindRecoveryMessage') &&
      wxmlSource.includes('<t-notice-bar'),
    'table edit page should render a persistent TDesign notice for image binding recovery'
  )

  console.log('check-merchant-table-image-bind-recovery: table image binding recovery is visible and retryable')
}

main().catch((error) => {
  console.error(error)
  process.exit(1)
})
