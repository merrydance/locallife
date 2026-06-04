const assert = require('assert')
const fs = require('fs')
const path = require('path')

const ROOT = path.resolve(__dirname, '..')
const apiSource = fs.readFileSync(
  path.join(ROOT, 'miniprogram/pages/register/rider/_api/rider-application.ts'),
  'utf8'
)
const pageSource = fs.readFileSync(
  path.join(ROOT, 'miniprogram/pages/register/rider/index.ts'),
  'utf8'
)
const pageWxml = fs.readFileSync(
  path.join(ROOT, 'miniprogram/pages/register/rider/index.wxml'),
  'utf8'
)

assert(
  apiSource.includes('patchRiderHealthCertOCRFields'),
  'rider application API must expose health cert OCR correction'
)
assert(
  apiSource.includes('/v1/rider/application/documents/health_cert/ocr-fields'),
  'health cert correction must call the backend correction endpoint'
)
assert(
  pageSource.includes('saveHealthCertOCRCorrection'),
  'rider register page must save corrected health cert OCR fields before advancing'
)
assert(
  pageSource.includes('await this.saveHealthCertOCRCorrection()'),
  'rider register page must await health cert correction persistence'
)
assert(
  pageWxml.includes('data-field="healthCertNo"') &&
    pageWxml.includes('data-field="healthCertDate"'),
  'health cert number and valid end must be editable form fields'
)
assert(
  !pageWxml.includes('<t-cell title="健康证号"') &&
    !pageWxml.includes('<t-cell title="有效期至"'),
  'health cert OCR fields must not remain read-only cells'
)
