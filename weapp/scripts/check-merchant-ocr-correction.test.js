const assert = require('assert')
const fs = require('fs')
const path = require('path')

const ROOT = path.resolve(__dirname, '..')
const apiSource = fs.readFileSync(
  path.join(ROOT, 'miniprogram/pages/register/_main_shared/api/onboarding.ts'),
  'utf8'
)
const runtimeSource = fs.readFileSync(
  path.join(ROOT, 'miniprogram/pages/register/merchant/store/_utils/merchant-store-registration-runtime.ts'),
  'utf8'
)
const pageWxml = fs.readFileSync(
  path.join(ROOT, 'miniprogram/pages/register/merchant/store/index.wxml'),
  'utf8'
)
const step2Start = pageWxml.indexOf('currentStep === 2')
const step3Start = pageWxml.indexOf('currentStep === 3')
const step2Wxml = pageWxml.slice(step2Start, step3Start)

assert(
  apiSource.includes('patchMerchantApplicationOCRFields'),
  'merchant onboarding API must expose OCR correction'
)
assert(
  apiSource.includes('/v1/merchant/application/documents/${documentType}/ocr-fields'),
  'merchant OCR correction must call the backend correction endpoint'
)
assert(
  runtimeSource.includes('saveMerchantOCRCorrections'),
  'merchant store registration runtime must persist corrected OCR fields'
)
assert(
  runtimeSource.includes('await this.saveMerchantOCRCorrections()'),
  'merchant store registration runtime must await OCR correction before advancing/submitting'
)
assert(
  runtimeSource.includes('function formOCRFieldValue') &&
    runtimeSource.includes('touchedFields[fieldName] || formValue') &&
    runtimeSource.includes('ocrCorrectionTouchedFields.${key}'),
  'merchant OCR correction must preserve intentionally cleared editable values instead of falling back to stale OCR data'
)
assert(
  runtimeSource.includes("'licenseLegalRepresentative'") &&
    runtimeSource.includes("formOCRFieldValue(formData, touchedFields, 'licenseLegalRepresentative'"),
  'merchant OCR correction must save business license legal representative separately from read-only ID card name'
)
assert(
  runtimeSource.includes("patchMerchantApplicationOCRFields('business_license'") &&
    runtimeSource.includes("patchMerchantApplicationOCRFields('food_permit'"),
  'merchant OCR correction must save both business license and food permit fields'
)
assert(
  runtimeSource.includes('businessLicenseOCRConfirmed') &&
    runtimeSource.includes('foodPermitOCRConfirmed') &&
    runtimeSource.includes('confirmed: true'),
  'merchant OCR correction must require explicit merchant confirmation and persist confirmation snapshots'
)
assert(
  !runtimeSource.includes("foodPermitOCR.company_name || currentDraft.business_license_ocr?.enterprise_name") &&
    runtimeSource.includes("formOCRFieldValue(formData, touchedFields, 'foodLicenseCompanyName', foodPermitOCR.company_name || formData.foodLicenseCompanyName)"),
  'food permit subject name must not fall back to the business license OCR name'
)
assert(
  runtimeSource.includes('hasMerchantBusinessLicenseResult(latestDraft)') &&
    runtimeSource.includes('hasMerchantFoodPermitResult(currentDraft)'),
  'merchant OCR correction must save displayed legacy OCR results even when old payloads have no status field'
)
assert(
  runtimeSource.includes('latestDraft?.business_license_number || latestDraft?.business_license_ocr?.credit_code || latestDraft?.business_license_ocr?.reg_num'),
  'merchant OCR correction must prefer credit_code over reg_num when merging business license number'
)
assert(
    step2Wxml.includes('data-field="licenseName"') &&
    step2Wxml.includes('data-field="creditCode"') &&
    step2Wxml.includes('data-field="licenseLegalRepresentative"') &&
    step2Wxml.includes('data-field="registerAddress"') &&
    step2Wxml.includes('data-field="licenseValidity"') &&
    step2Wxml.includes('data-field="businessScope"') &&
    step2Wxml.includes('data-field="foodLicensePermitNo"') &&
    step2Wxml.includes('data-field="foodLicenseCompanyName"') &&
    step2Wxml.includes('data-field="foodLicenseOperatorName"') &&
    step2Wxml.includes('data-field="foodLicenseValidFrom"') &&
    step2Wxml.includes('data-field="foodLicenseValidity"'),
  'business license and food permit OCR fields must be editable form fields'
)
assert(
  step2Wxml.includes('onBusinessLicenseOCRConfirmChange') &&
    step2Wxml.includes('onFoodPermitOCRConfirmChange') &&
    step2Wxml.includes('我已核对营业执照名称和统一信用代码与原件一致') &&
    step2Wxml.includes('我已核对食品经营许可证主体名称和许可证编号与原件一致'),
  'Step 2 must show explicit OCR confirmation checkboxes for business license and food permit'
)
assert(
  !step2Wxml.includes('<t-cell title="营业执照名"') &&
    !step2Wxml.includes('<t-cell title="统一信用代码"') &&
    !step2Wxml.includes('<t-cell title="许可证编号"') &&
    !step2Wxml.includes('<t-cell title="主体名称"') &&
    !step2Wxml.includes('<t-cell title="许可证有效期"'),
  'merchant OCR correction fields must not remain read-only cells in Step 2'
)
assert(
  step2Wxml.includes('<t-cell title="法人姓名"') &&
    step2Wxml.includes('<t-cell title="身份证号"') &&
    step2Wxml.includes('<t-cell title="身份证有效期"'),
  'merchant ID card OCR fields must remain read-only'
)
assert(
  !step2Wxml.includes('data-field="idCard"') &&
    !step2Wxml.includes('data-field="legalPerson"') &&
    !step2Wxml.includes('data-field="idCardValidity"'),
  'merchant ID card OCR fields must not expose editable field bindings'
)
