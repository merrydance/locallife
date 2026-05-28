const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const repoRoot = path.join(__dirname, '..')
const ownerPath = path.join(repoRoot, 'miniprogram', 'utils', 'merchant-store-registration-view.ts')
const runtimePath = path.join(repoRoot, 'miniprogram', 'utils', 'merchant-store-registration-runtime.ts')

function plain(value) {
  return JSON.parse(JSON.stringify(value))
}

function loadOwnerModule() {
  const source = fs.readFileSync(ownerPath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018
    }
  }).outputText

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (modulePath === '../api/onboarding') {
        return {
          buildMerchantApplicationOCRStatusView(status) {
            const normalizedStatus = String(status || '').trim().toLowerCase()
            return {
              statusCode: normalizedStatus,
              text: '',
              isPending: normalizedStatus === 'processing' || normalizedStatus === 'pending',
              isReady: normalizedStatus === 'done',
              isFailed: normalizedStatus === 'failed'
            }
          }
        }
      }
      if (modulePath === '../api/location') {
        return {}
      }
      throw new Error(`unexpected require: ${modulePath}`)
    },
    Array,
    Boolean,
    JSON,
    Math,
    Number,
    RegExp,
    Set,
    String
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: ownerPath })
  return sandbox.module.exports
}

const owner = loadOwnerModule()

assert.strictEqual(owner.buildLegalBusinessAddress({
  business_address: '  上海市徐汇区  ',
  business_license_ocr: { address: '营业执照地址' }
}), '上海市徐汇区')
assert.strictEqual(owner.buildLegalBusinessAddress({
  business_license_ocr: { address: '  营业执照地址  ' }
}), '营业执照地址')

assert.strictEqual(owner.buildMapLocationLabel({
  geocodedAddress: '上海市黄浦区中山东一路1号',
  chosenAddress: '用户选择地址',
  chosenName: '门店'
}), '上海市黄浦区中山东一路1号')
assert.strictEqual(owner.buildMapLocationLabel({
  chosenAddress: '上海市黄浦区中山东一路',
  chosenName: '外滩店'
}), '上海市黄浦区中山东一路 外滩店')
assert.strictEqual(owner.buildMapLocationLabel({
  chosenAddress: '上海市黄浦区外滩店',
  chosenName: '外滩店'
}), '上海市黄浦区外滩店')
assert.strictEqual(owner.buildMapLocationLabel({
  latitude: 31.2304,
  longitude: 121.4737
}), '已选位置：31.230400, 121.473700')

const images = [
  { url: 'https://cdn.local/a.jpg', rawUrl: ' raw/a.jpg ', assetId: 1 },
  { url: 'https://cdn.local/b.jpg', rawUrl: 'raw/b.jpg', assetId: 2 },
  { url: 'https://cdn.local/a-copy.jpg', rawUrl: 'raw/a.jpg', assetId: 3 },
  { url: 'local://temp.jpg', localFileUrl: 'local://temp.jpg', pendingSync: true }
]
assert.deepStrictEqual(owner.toPersistedImageUrls(images), ['raw/a.jpg', 'raw/b.jpg'])

const previousFiles = [{ url: 'cached://signed-a.jpg', rawUrl: 'raw/a.jpg', assetId: 1, status: 'done' }]
const renderImages = owner.buildUploadRenderImages(images, previousFiles)
assert.strictEqual(renderImages[0], previousFiles[0], 'existing signed file object should be reused when identity and status match')
assert.deepStrictEqual(plain(renderImages[3]), {
  url: 'local://temp.jpg',
  status: 'loading',
  localFileUrl: 'local://temp.jpg',
  pendingSync: true
})

const persisted = owner.markImagesPersisted([{
  url: 'https://cdn.local/a.jpg',
  rawUrl: 'raw/a.jpg',
  localFileUrl: 'local://temp.jpg',
  pendingSync: true,
  status: 'loading'
}])
assert.deepStrictEqual(plain(persisted[0]), {
  url: 'https://cdn.local/a.jpg',
  rawUrl: 'raw/a.jpg',
  pendingSync: false,
  status: 'done'
})

assert.strictEqual(owner.toSafeNumber('42'), 42)
assert.strictEqual(owner.toSafeNumber('bad'), 0)
assert.strictEqual(owner.toSafeNumber(Infinity), 0)

assert.deepStrictEqual(plain(owner.parseRegionAddress('上海市浦东新区世纪大道1号')), {
  province: '上海市',
  city: '',
  district: '浦东新区'
})
assert.deepStrictEqual(plain(owner.buildRegionSearchKeywords('广东省深圳市南山区科技园')), ['南山区', '南山', '深圳市', '深圳'])

const matched = owner.pickBestRegionSearchResult([
  { id: 1, code: 'city', name: '深圳市', level: 2 },
  { id: 2, code: 'district', name: '南山', level: 3 },
  { id: 3, code: 'district2', name: '福田区', level: 3 }
], '广东省深圳市南山区科技园')
assert.strictEqual(matched.id, 2)

assert.strictEqual(owner.hasMerchantBusinessLicenseResult({
  business_license_ocr: { enterprise_name: '本地生活餐饮店' }
}), true)
assert.strictEqual(owner.hasMerchantBusinessLicenseResult({
  business_license_ocr: { credit_code: '91310000MA1LOCAL1X' }
}), true)
assert.strictEqual(owner.hasMerchantBusinessLicenseResult({
  business_license_ocr: { address: '上海市徐汇区' }
}), true)
assert.strictEqual(owner.hasMerchantBusinessLicenseResult({
  business_license_number: '91310000MA1LOCAL1X'
}), true)
assert.strictEqual(owner.hasMerchantBusinessLicenseResult({
  business_license_ocr: { status: 'done' }
}), false)

assert.strictEqual(owner.buildMerchantOcrProgressMessage({
  data: { business_license_ocr: { status: 'processing' } },
  hasBusinessLicenseImage: true,
  hasFoodPermitImage: false,
  hasIdCardFrontImage: false,
  hasIdCardBackImage: false
}), '证照已上传，系统正在自动识别，完成后会自动回填。你可以先继续填写后续信息。')
assert.strictEqual(owner.buildMerchantOcrProgressMessage({
  data: { business_license_ocr: { status: 'done' } },
  hasBusinessLicenseImage: true,
  hasFoodPermitImage: false,
  hasIdCardFrontImage: false,
  hasIdCardBackImage: false
}), '')

assert.deepStrictEqual(plain(owner.buildMerchantOcrDisplayState({
  data: { business_license_ocr: { status: 'processing' } },
  hasBusinessLicenseImage: true,
  hasFoodPermitImage: false,
  hasIdCardFrontImage: false,
  hasIdCardBackImage: false
})), {
  businessLicenseReady: false,
  businessLicenseProcessing: true,
  businessLicenseFailed: false,
  foodPermitReady: false,
  foodPermitProcessing: false,
  foodPermitFailed: false,
  idCardReady: false,
  idCardProcessing: false,
  idCardFailed: false
})

assert.strictEqual(owner.buildMerchantOcrDisplayState({
  data: { business_license_ocr: { status: 'failed' } },
  hasBusinessLicenseImage: true,
  hasFoodPermitImage: false,
  hasIdCardFrontImage: false,
  hasIdCardBackImage: false
}).businessLicenseFailed, true)
assert.strictEqual(owner.buildMerchantOcrDisplayState({
  data: { business_license_ocr: { status: 'done' } },
  hasBusinessLicenseImage: true,
  hasFoodPermitImage: false,
  hasIdCardFrontImage: false,
  hasIdCardBackImage: false
}).businessLicenseReady, true)

const uploadFeedback = owner.buildMerchantUploadFeedback({
  data: {
    business_license_ocr: { status: 'done' },
    food_permit_ocr: { status: 'failed', error: '图片模糊' },
    id_card_front_ocr: { status: 'processing' }
  },
  hasBusinessLicenseImage: true,
  hasFoodPermitImage: true,
  hasIdCardFrontImage: true,
  hasIdCardBackImage: false
})
assert.deepStrictEqual(plain(uploadFeedback.license), {
  state: 'success',
  title: '识别成功',
  description: '已回填主体名称、统一信用代码和经营范围'
})
assert.deepStrictEqual(plain(uploadFeedback.foodPermit), {
  state: 'error',
  title: '识别失败',
  description: '图片模糊'
})
assert.deepStrictEqual(plain(uploadFeedback.idCardFront), {
  state: 'processing',
  title: '证照识别中',
  description: '正在识别身份证人像面信息'
})
assert.deepStrictEqual(plain(uploadFeedback.idCardBack), {
  state: 'idle',
  title: '',
  description: ''
})

const runtimeSource = fs.readFileSync(runtimePath, 'utf8')
const forbiddenRuntimePatterns = [
  'function buildLegalBusinessAddress',
  'function buildMapLocationLabel',
  'function normalizeImageRawUrl',
  'function toPersistedImageUrls',
  'function isImagePendingPersistence',
  'function isSameImageIdentity',
  'function buildUploadRenderImages',
  'function markImagesPersisted',
  'function toSafeNumber',
  'function parseRegionAddress',
  'function buildRegionSearchKeywords',
  'function pickBestRegionSearchResult',
  'function createUploadFeedback',
  'function hasMerchantBusinessLicenseResult',
  'function hasMerchantFoodPermitResult',
  'function hasMerchantIDCardFrontResult',
  'function hasMerchantIDCardBackResult'
]

for (const pattern of forbiddenRuntimePatterns) {
  assert(!runtimeSource.includes(pattern), `merchant-store-registration-runtime.ts must not own ${pattern}`)
}

assert(
  runtimeSource.includes("from './merchant-store-registration-view'"),
  'merchant-store-registration-runtime.ts must consume merchant-store-registration-view owner'
)

console.log('check-merchant-store-registration-view-owner tests passed')
