const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const repoRoot = path.join(__dirname, '..')
const viewPath = path.join(repoRoot, 'miniprogram', 'pages', 'merchant', '_utils', 'merchant-profile-images-view.ts')
const recoveryPath = path.join(repoRoot, 'miniprogram', 'pages', 'merchant', '_utils', 'merchant-profile-images-recovery.ts')
const lifecyclePath = path.join(repoRoot, 'miniprogram', 'pages', 'merchant', '_utils', 'merchant-profile-images-lifecycle.ts')
const pagePath = path.join(repoRoot, 'miniprogram', 'pages', 'merchant', 'profile-images', 'index.ts')

function transpile(source, filename) {
  return ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018,
      strict: true
    },
    fileName: filename
  }).outputText
}

function loadViewHelper() {
  const source = fs.readFileSync(viewPath, 'utf8')
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (modulePath === '../../../utils/logger') {
        return { logger: { warn() {}, error() {} } }
      }
      if (modulePath === '../../../utils/image') {
        return {
          getPublicImageUrl(value) {
            return typeof value === 'string' && value.trim() ? value.trim() : ''
          }
        }
      }
      if (modulePath === '../../../utils/user-facing') {
        return {
          getErrorUserMessage(error, fallback) {
            if (error && typeof error === 'object' && typeof error.userMessage === 'string' && error.userMessage.trim()) {
              return error.userMessage.trim()
            }
            return fallback
          }
        }
      }
      throw new Error(`unexpected require in view helper: ${modulePath}`)
    },
    wx: {
      getStorageSync() {
        return null
      }
    },
    Array,
    Date,
    JSON,
    Math,
    Number,
    Object,
    RegExp,
    Set,
    String
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(transpile(source, viewPath), sandbox, { filename: viewPath })
  return sandbox.module.exports
}

function loadRecoveryHelper(viewExports) {
  const source = fs.readFileSync(recoveryPath, 'utf8')
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require(modulePath) {
      if (modulePath === '../_main_shared/api/onboarding') {
        return {
          async waitForPublicMediaDisplayUrl(mediaId) {
            return `https://cdn.example.test/media/${mediaId}.jpg`
          }
        }
      }
      if (modulePath === '../../../utils/logger') {
        return { logger: { warn() {}, error() {} } }
      }
      if (modulePath === './merchant-profile-images-view') {
        return viewExports
      }
      throw new Error(`unexpected require in recovery helper: ${modulePath}`)
    },
    Promise,
    Array,
    Date,
    JSON,
    Number,
    Object,
    Set,
    String
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(transpile(source, recoveryPath), sandbox, { filename: recoveryPath })
  return sandbox.module.exports.merchantProfileImagesRecoveryMethods
}

function createPage(viewExports, storefrontImages = [], environmentImages = []) {
  return {
    data: {
      logoImage: null,
      storefrontImages,
      storefrontFiles: [],
      environmentImages,
      environmentFiles: []
    },
    _shopImagesGeneration: 0,
    _shopImagesPersistRetryPending: false,
    _deletedStorefrontImageRawUrls: new Set(),
    _deletedEnvironmentImageRawUrls: new Set(),
    resetRetryCount: 0,
    retryScheduledCount: 0,
    finalizedShopImages: [],
    bumpShopImagesGeneration() {
      this._shopImagesGeneration += 1
    },
    setData(patch) {
      this.data = { ...this.data, ...patch }
    },
    reconcilePendingDeletedShopImages(storefrontRawUrls, environmentRawUrls) {
      return {
        storefrontRawUrls: viewExports.toNormalizedRawUrls(storefrontRawUrls),
        environmentRawUrls: viewExports.toNormalizedRawUrls(environmentRawUrls)
      }
    },
    resetPendingShopImagesPersistenceRetryState() {
      this.resetRetryCount += 1
    },
    schedulePendingShopImagesPersistence() {
      this.retryScheduledCount += 1
      this._shopImagesPersistRetryPending = true
    },
    retryPendingShopImagePersistence() {
      this.schedulePendingShopImagesPersistence()
    },
    finalizePendingShopImage(kind, mediaId) {
      this.finalizedShopImages.push({ kind, mediaId })
    },
    finalizePendingLogo() {}
  }
}

function assertRuntimeRecovery(viewExports, recoveryMethods) {
  assert.strictEqual(
    typeof viewExports.resolveShopImagesServerTruth,
    'function',
    'view helper must export resolveShopImagesServerTruth for live merchant image truth selection'
  )

  const liveTruth = viewExports.resolveShopImagesServerTruth({
    storefront_images: ['media/live-storefront.jpg'],
    environment_images: []
  }, {
    storefront_images: ['media/application-storefront.jpg'],
    environment_images: ['media/application-environment.jpg']
  })
  assert.deepStrictEqual(
    liveTruth.storefrontRawUrls,
    ['media/live-storefront.jpg'],
    'merchant live storefront images must take precedence over application images'
  )
  assert.deepStrictEqual(
    liveTruth.environmentRawUrls,
    [],
    'empty merchant live environment images must not resurrect application images'
  )
  assert.strictEqual(
    liveTruth.hasMerchantStorefrontTruth,
    true,
    'merchant live storefront array must mark storefront server truth available even if application fallback fails'
  )
  assert.strictEqual(
    liveTruth.hasMerchantEnvironmentTruth,
    true,
    'empty merchant live environment array must still mark environment server truth available'
  )

  const fallbackTruth = viewExports.resolveShopImagesServerTruth({
    storefront_images: null,
    environment_images: undefined
  }, {
    storefront_images: ['media/application-storefront.jpg'],
    environment_images: ['media/application-environment.jpg']
  })
  assert.deepStrictEqual(
    fallbackTruth.storefrontRawUrls,
    ['media/application-storefront.jpg'],
    'null merchant storefront images should fall back to application images for compatibility'
  )
  assert.deepStrictEqual(
    fallbackTruth.environmentRawUrls,
    ['media/application-environment.jpg'],
    'undefined merchant environment images should fall back to application images for compatibility'
  )
  assert.strictEqual(
    fallbackTruth.hasMerchantStorefrontTruth,
    false,
    'null merchant storefront images must mark merchant storefront truth absent'
  )
  assert.strictEqual(
    fallbackTruth.hasMerchantEnvironmentTruth,
    false,
    'undefined merchant environment images must mark merchant environment truth absent'
  )

  const pendingStorefront = {
    url: 'https://cdn.example.test/storefront-a.jpg',
    rawUrl: 'media/storefront-a.jpg',
    assetId: 101,
    localFileUrl: 'wxfile://storefront-a.jpg',
    pendingSync: true
  }
  const page = createPage(viewExports, [pendingStorefront], [])

  recoveryMethods.applyShopImagesResponse.call(page, {
    storefront_images: [],
    environment_images: []
  }, [pendingStorefront], [])

  assert.strictEqual(page.data.storefrontImages.length, 1, 'missing server echo must keep the local pending storefront image visible')
  assert.strictEqual(page.data.storefrontImages[0].pendingSync, true, 'missing server echo must keep pendingSync=true')
  assert.strictEqual(page.data.storefrontImages[0].localFileUrl, 'wxfile://storefront-a.jpg', 'missing server echo must keep local preview URL')
  assert.strictEqual(page.data.storefrontFiles[0].status, 'loading', 'pending storefront upload should render as loading')
  assert.strictEqual(page._shopImagesPersistRetryPending, true, 'missing server echo must mark shop-image persistence retry pending')

  const confirmedPage = createPage(viewExports, [pendingStorefront], [])
  recoveryMethods.applyShopImagesResponse.call(confirmedPage, {
    storefront_images: ['media/storefront-a.jpg'],
    environment_images: []
  }, viewExports.clearPendingSyncFromImages([pendingStorefront]), [])

  assert.strictEqual(confirmedPage.data.storefrontImages.length, 1, 'confirmed server echo should keep one storefront image')
  assert.strictEqual(confirmedPage.data.storefrontImages[0].rawUrl, 'media/storefront-a.jpg', 'confirmed server echo should use persisted raw URL')
  assert.strictEqual(confirmedPage.data.storefrontImages[0].pendingSync, undefined, 'confirmed server echo should clear pendingSync')
  assert.strictEqual(confirmedPage.data.storefrontImages[0].localFileUrl, undefined, 'confirmed server echo should clear local preview URL')
  assert.strictEqual(confirmedPage.data.storefrontFiles[0].status, 'done', 'confirmed storefront upload should render as done')
  assert.strictEqual(confirmedPage._shopImagesPersistRetryPending, false, 'confirmed server echo must clear retry pending')
  assert.strictEqual(confirmedPage.resetRetryCount, 1, 'confirmed server echo should reset retry state')

  const pendingEnvironment = {
    url: 'https://cdn.example.test/environment-a.jpg',
    rawUrl: 'media/environment-a.jpg',
    assetId: 202,
    localFileUrl: 'wxfile://environment-a.jpg',
    pendingSync: true
  }
  const environmentPage = createPage(viewExports, [], [pendingEnvironment])
  recoveryMethods.applyShopImagesResponse.call(environmentPage, {
    storefront_images: [],
    environment_images: []
  }, [], [pendingEnvironment])

  assert.strictEqual(environmentPage.data.environmentImages.length, 1, 'missing server echo must keep the local pending environment image visible')
  assert.strictEqual(environmentPage.data.environmentImages[0].pendingSync, true, 'missing environment server echo must keep pendingSync=true')
  assert.strictEqual(environmentPage.data.environmentFiles[0].status, 'loading', 'pending environment upload should render as loading')
  assert.strictEqual(environmentPage._shopImagesPersistRetryPending, true, 'missing environment server echo must mark retry pending')

  const confirmedEnvironmentPage = createPage(viewExports, [], [pendingEnvironment])
  recoveryMethods.applyShopImagesResponse.call(confirmedEnvironmentPage, {
    storefront_images: [],
    environment_images: ['media/environment-a.jpg']
  }, [], viewExports.clearPendingSyncFromImages([pendingEnvironment]))

  assert.strictEqual(confirmedEnvironmentPage.data.environmentImages.length, 1, 'confirmed environment server echo should keep one image')
  assert.strictEqual(confirmedEnvironmentPage.data.environmentImages[0].pendingSync, undefined, 'confirmed environment server echo should clear pendingSync')
  assert.strictEqual(confirmedEnvironmentPage.data.environmentImages[0].localFileUrl, undefined, 'confirmed environment server echo should clear local preview URL')
  assert.strictEqual(confirmedEnvironmentPage.data.environmentFiles[0].status, 'done', 'confirmed environment upload should render as done')
  assert.strictEqual(confirmedEnvironmentPage._shopImagesPersistRetryPending, false, 'confirmed environment server echo must clear retry pending')

  const reloadPage = createPage(viewExports, [{
    url: 'https://cdn.example.test/storefront-reload.jpg',
    rawUrl: 'media/storefront-reload.jpg',
    assetId: 203,
    pendingSync: true
  }], [])
  recoveryMethods.resumePendingImageRecovery.call(reloadPage, null, reloadPage.data.storefrontImages, [], [], [])
  assert.strictEqual(reloadPage.retryScheduledCount, 1, 'reload with local asset absent from server truth must schedule persistence retry')

  const environmentReloadPage = createPage(viewExports, [], [{
    url: 'https://cdn.example.test/environment-reload.jpg',
    rawUrl: 'media/environment-reload.jpg',
    assetId: 204,
    pendingSync: true
  }])
  recoveryMethods.resumePendingImageRecovery.call(environmentReloadPage, null, [], environmentReloadPage.data.environmentImages, [], [])
  assert.strictEqual(environmentReloadPage.retryScheduledCount, 1, 'environment reload with local asset absent from server truth must schedule persistence retry')

  const waitingForMediaPage = createPage(viewExports, [{
    url: 'wxfile://storefront-b.jpg',
    assetId: 303,
    localFileUrl: 'wxfile://storefront-b.jpg',
    pendingSync: true
  }], [])
  recoveryMethods.resumePendingImageRecovery.call(waitingForMediaPage, null, waitingForMediaPage.data.storefrontImages, [], [], [])
  assert.deepStrictEqual(
    waitingForMediaPage.finalizedShopImages,
    [{ kind: 'storefront', mediaId: 303 }],
    'reload with asset id but no public raw URL must resume media public-URL polling'
  )

  const environmentWaitingForMediaPage = createPage(viewExports, [], [{
    url: 'wxfile://environment-b.jpg',
    assetId: 304,
    localFileUrl: 'wxfile://environment-b.jpg',
    pendingSync: true
  }])
  recoveryMethods.resumePendingImageRecovery.call(environmentWaitingForMediaPage, null, [], environmentWaitingForMediaPage.data.environmentImages, [], [])
  assert.deepStrictEqual(
    environmentWaitingForMediaPage.finalizedShopImages,
    [{ kind: 'environment', mediaId: 304 }],
    'environment reload with asset id but no public raw URL must resume media public-URL polling'
  )
}

function assertStaticRecoveryContract() {
  const pageSource = fs.readFileSync(pagePath, 'utf8')
  const lifecycleSource = fs.readFileSync(lifecyclePath, 'utf8')
  const recoverySource = fs.readFileSync(recoveryPath, 'utf8')

  assert(
    /pendingSync:\s*true/.test(pageSource) &&
      /isAmbiguousSyncFailure\(persistErr\)[\s\S]+schedulePendingShopImagesPersistence\(\)/.test(pageSource) &&
      pageSource.includes('CONTINUE_SYNC_TOAST'),
    'upload page must keep ambiguous shop-image persistence visible and schedule retry with user copy'
  )
  assert(
    pageSource.includes('hasMerchantLiveStorefrontTruth') &&
      pageSource.includes('hasMerchantLiveEnvironmentTruth') &&
      /applicationUnavailable[\s\S]+!hasMerchantLiveStorefrontTruth[\s\S]+toPersistedImageUrls\(latestStorefrontImages\)/.test(pageSource) &&
      /applicationUnavailable[\s\S]+!hasMerchantLiveEnvironmentTruth[\s\S]+toPersistedImageUrls\(latestEnvironmentImages\)/.test(pageSource),
    'loadData must still adopt merchant live shop images when the application fallback request fails'
  )
  assert(
    /while \(this\._shopImagesPersistRetryPending\)/.test(lifecycleSource) &&
      /updateShopImages\(\{[\s\S]+storefront_images:[\s\S]+environment_images:/.test(lifecycleSource) &&
      /isAmbiguousSyncFailure\(err\)[\s\S]+schedulePendingShopImagesPersistenceWithBackoff\(\)/.test(lifecycleSource),
    'lifecycle retry must retry both image arrays and back off after ambiguous failures'
  )
  assert(
    /applyShopImagesResponse\(updated[\s\S]+clearPendingSyncFromImages\(storefrontImages\)[\s\S]+clearPendingSyncFromImages\(environmentImages\)/.test(lifecycleSource),
    'successful retry must clear pending-sync markers from both shop-image lists'
  )
  assert(
    /resumePendingImageRecovery\([\s\S]+shouldRetryShopPersistence[\s\S]+retryPendingShopImagePersistence\(\)/.test(recoverySource),
    'application reload must resume persistence when local asset ids are not in server truth'
  )
}

function main() {
  const viewExports = loadViewHelper()
  const recoveryMethods = loadRecoveryHelper(viewExports)
  assertRuntimeRecovery(viewExports, recoveryMethods)
  assertStaticRecoveryContract()
  console.log('check-merchant-profile-images-pending-sync-recovery: validated storefront/environment pending-sync recovery')
}

main()
