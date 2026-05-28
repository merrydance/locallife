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

assert.deepStrictEqual(plain(owner.getMerchantStoreRegistrationDocumentRemovalTarget('license')), {
  documentType: 'business_license',
  data: {
    licenseImages: [],
    'formData.licenseName': '',
    'formData.creditCode': '',
    'formData.registerAddress': '',
    'formData.licenseValidity': '',
    'formData.businessScope': '',
    'ocrResults.license': null
  }
})
assert.deepStrictEqual(plain(owner.getMerchantStoreRegistrationDocumentRemovalTarget('foodPermit')), {
  documentType: 'food_permit',
  data: {
    foodLicenseImages: [],
    'formData.foodLicenseValidity': ''
  }
})
assert.deepStrictEqual(plain(owner.getMerchantStoreRegistrationDocumentRemovalTarget('idCardFront')), {
  documentType: 'id_card_front',
  data: {
    idCardFrontImages: [],
    'formData.legalPerson': '',
    'formData.idCard': '',
    'formData.gender': '',
    'formData.hometown': '',
    'ocrResults.idCard': null
  }
})
assert.deepStrictEqual(plain(owner.getMerchantStoreRegistrationDocumentRemovalTarget('idCardBack')), {
  documentType: 'id_card_back',
  data: {
    idCardBackImages: [],
    'formData.idCardValidity': ''
  }
})

assert.deepStrictEqual(plain(owner.buildMerchantUploadedImagePatch('idCardBack', 'local://id-back.jpg', 42)), {
  idCardBackImages: [{ url: 'local://id-back.jpg', assetId: 42 }]
})
assert.deepStrictEqual(plain(owner.buildMerchantUploadProcessingFeedback()), {
  state: 'processing',
  title: '证照识别中',
  description: '请稍候，识别结果会显示在当前卡片中'
})
assert.deepStrictEqual(plain(owner.buildMerchantUploadErrorFeedback('上传失败，请重试')), {
  state: 'error',
  title: '识别失败',
  description: '上传失败，请重试'
})

assert.deepStrictEqual(plain(owner.buildMerchantLatestOcrFormPatch({
  business_address: '',
  business_license_number: '91310000MA1LOCAL1X',
  business_scope: '热食类食品制售',
  business_license_ocr: {
    enterprise_name: '本地生活餐饮店',
    address: '上海市徐汇区',
    valid_period: '2026-01-01 至 2036-01-01',
    legal_representative: '张三'
  },
  food_permit_ocr: {
    valid_to: '2030-01-01'
  },
  legal_person_name: '',
  legal_person_id_number: '310101199001010011',
  id_card_front_ocr: {
    name: '',
    id_number: '',
    gender: '男',
    address: '上海市黄浦区'
  },
  id_card_back_ocr: {
    valid_date: '2026.01.01-2036.01.01'
  }
}, '保留地址')), {
  licenseName: '本地生活餐饮店',
  creditCode: '91310000MA1LOCAL1X',
  address: '上海市徐汇区',
  registerAddress: '上海市徐汇区',
  licenseValidity: '2026-01-01 至 2036-01-01',
  businessScope: '热食类食品制售',
  foodLicenseValidity: '2030-01-01',
  legalPerson: '张三',
  idCard: '310101199001010011',
  gender: '男',
  hometown: '上海市黄浦区',
  idCardValidity: '2026.01.01-2036.01.01'
})
assert.strictEqual(owner.buildMerchantLatestOcrFormPatch({
  business_license_ocr: {}
}, '保留地址').address, '保留地址')

assert.deepStrictEqual(plain(owner.buildMerchantInitialDraftFormPatch({
  merchant_name: '本地生活餐饮店',
  contact_phone: '13800138000',
  business_address: '',
  business_address_detail: '静安寺店',
  region_id: 310106,
  latitude: '31.223',
  longitude: '121.445',
  business_license_number: '',
  business_scope: '',
  business_license_ocr: {
    enterprise_name: '营业执照主体',
    reg_num: '91310000MA1LOCAL1X',
    address: '上海市静安区',
    valid_period: '2026-01-01 至 2036-01-01',
    business_scope: '热食类食品制售',
    legal_representative: '李四'
  },
  food_permit_ocr: {
    valid_to: '2030-06-01'
  },
  legal_person_name: '',
  legal_person_id_number: '',
  id_card_front_ocr: {
    name: '李四',
    id_number: '310106199001010011',
    gender: '男',
    address: '上海市静安区'
  },
  id_card_back_ocr: {
    valid_date: '2026.01.01-2036.01.01'
  },
  legal_person_contact_address: '上海市静安区现住址',
  bank_name: '招商银行',
  bank_account: '6225880000000000',
  bank_account_name: '李四'
})), {
  name: '本地生活餐饮店',
  phone: '13800138000',
  address: '上海市静安区',
  addressDetail: '',
  regionId: 310106,
  latitude: 31.223,
  longitude: 121.445,
  licenseName: '营业执照主体',
  creditCode: '91310000MA1LOCAL1X',
  registerAddress: '上海市静安区',
  licenseValidity: '2026-01-01 至 2036-01-01',
  businessScope: '热食类食品制售',
  foodLicenseValidity: '2030-06-01',
  legalPerson: '李四',
  idCard: '310106199001010011',
  gender: '男',
  hometown: '上海市静安区',
  idCardValidity: '2026.01.01-2036.01.01',
  currentAddress: '上海市静安区现住址',
  bankName: '招商银行',
  bankAccount: '6225880000000000',
  accountName: '李四'
})
assert.deepStrictEqual(plain(owner.buildMerchantInitialDraftOcrResults({
  business_license_ocr: { enterprise_name: '营业执照主体' },
  id_card_front_ocr: { name: '李四' }
})), {
  license: { enterprise_name: '营业执照主体' },
  idCard: { name: '李四' }
})
assert.deepStrictEqual(plain(owner.buildMerchantInitialDraftOcrResults({})), {
  license: null,
  idCard: null
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
  'function hasMerchantIDCardBackResult',
  'const documentMap',
  '请稍候，识别结果会显示在当前卡片中',
  'licenseName: toSafeString(data.business_license_ocr',
  'foodLicenseValidity: toSafeString(data.food_permit_ocr',
  'idCardValidity: toSafeString(data.id_card_back_ocr',
  'name: safeStr(data.merchant_name)',
  'regionId: Number(data.region_id || 0)',
  'licenseName: safeStr(data.business_license_ocr',
  'const ocrResults = {'
]

for (const pattern of forbiddenRuntimePatterns) {
  assert(!runtimeSource.includes(pattern), `merchant-store-registration-runtime.ts must not own ${pattern}`)
}

assert(
  runtimeSource.includes("from './merchant-store-registration-view'"),
  'merchant-store-registration-runtime.ts must consume merchant-store-registration-view owner'
)

console.log('check-merchant-store-registration-view-owner tests passed')
