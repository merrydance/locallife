const assert = require('assert')
const fs = require('fs')
const path = require('path')

const ROOT = path.resolve(__dirname, '..')
const pageSource = fs.readFileSync(
  path.join(ROOT, 'miniprogram/pages/register/rider/index.ts'),
  'utf8'
)
const viewSource = fs.readFileSync(
  path.join(ROOT, 'miniprogram/pages/register/rider/_utils/rider-register-view.ts'),
  'utf8'
)
const workflowSource = fs.readFileSync(
  path.join(ROOT, 'miniprogram/pages/register/rider/_utils/rider-application-document-workflow.ts'),
  'utf8'
)

assert(
  pageSource.includes('hasRiderUploadAssetId(idFront.assetId)') &&
    pageSource.includes('hasRiderUploadAssetId(idBack.assetId)') &&
    pageSource.includes('hasRiderUploadAssetId(healthCert.assetId)'),
  'rider registration document gate must require all backend media asset ids, not preview URLs'
)

assert(
  !pageSource.includes('!idFront.url || !idBack.url || !healthCert.url'),
  'rider registration document gate must not use preview URLs as upload truth'
)

assert(
  !viewSource.includes('uploads.idFront?.url') &&
    !viewSource.includes('uploads.idBack?.url') &&
    !viewSource.includes('uploads.healthCert?.url'),
  'rider registration upload status must not treat preview URLs as uploaded documents'
)

assert(
  workflowSource.includes("'idFront.assetId': undefined") &&
    workflowSource.includes("'idBack.assetId': undefined") &&
    workflowSource.includes("'healthCert.assetId': undefined"),
  'rider registration replacement preview must clear stale asset ids until the backend upload completes'
)
