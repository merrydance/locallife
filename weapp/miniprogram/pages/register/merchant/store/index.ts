import {
  DEFAULT_MERCHANT_OCR_DISPLAY_STATE,
  DEFAULT_MERCHANT_UPLOAD_FEEDBACK,
  type ImageFieldItem,
  type MerchantOCRDisplayState,
  type MerchantUploadFeedback,
  type OCRResult,
  merchantStoreRegistrationRuntimeMethods
} from './_utils/merchant-store-registration-runtime'

Page({
  data: {
    navBarHeight: 88,
    currentStep: 0,
    isSubmitting: false,
    applicationInitialized: false,
    ocrProgressMessage: '',
    ocrDisplayState: DEFAULT_MERCHANT_OCR_DISPLAY_STATE as MerchantOCRDisplayState,
    uploadFeedback: DEFAULT_MERCHANT_UPLOAD_FEEDBACK as MerchantUploadFeedback,
    phoneError: '',
    formData: {
      name: '',
      phone: '',
      address: '',
      addressDetail: '',
      regionId: 0,
      latitude: 0,
      longitude: 0,
      licenseName: '',
      creditCode: '',
      licenseLegalRepresentative: '',
      registerAddress: '',
      licenseValidity: '',
      businessScope: '',
      foodLicensePermitNo: '',
      foodLicenseCompanyName: '',
      foodLicenseOperatorName: '',
      foodLicenseValidFrom: '',
      foodLicenseValidity: '',
      legalPerson: '',
      idCard: '',
      gender: '',
      hometown: '',
      currentAddress: '',
      idCardValidity: '',
      bankName: '',
      bankAccount: '',
      accountName: ''
    },
    businessLicenseOCRConfirmed: false,
    foodPermitOCRConfirmed: false,
    licenseImages: [] as ImageFieldItem[],
    foodLicenseImages: [] as ImageFieldItem[],
    idCardFrontImages: [] as ImageFieldItem[],
    idCardBackImages: [] as ImageFieldItem[],
    accountPermitImages: [] as ImageFieldItem[],
    storefrontImages: [] as ImageFieldItem[],
    storefrontFiles: [] as ImageFieldItem[],
    storefrontSaving: false,
    environmentImages: [] as ImageFieldItem[],
    environmentFiles: [] as ImageFieldItem[],
    environmentSaving: false,
    ocrResults: {
      license: null as OCRResult | null,
      idCard: null as OCRResult | null
    },
    typePickerVisible: false,
    typePickerValue: [],
    typeOptions: [
      { label: '中餐', value: 'chinese' },
      { label: '西餐', value: 'western' },
      { label: '日韩料理', value: 'japanese_korean' },
      { label: '快餐', value: 'fast_food' },
      { label: '小吃', value: 'snack' },
      { label: '甜品饮品', value: 'dessert' },
      { label: '其他', value: 'other' }
    ],
    timePickerVisible: false,
    timePickerValue: [],
    timeOptions: [
      { label: '全天营业 (00:00-24:00)', value: 'all_day' },
      { label: '早餐时段 (06:00-10:00)', value: 'breakfast' },
      { label: '午餐时段 (11:00-14:00)', value: 'lunch' },
      { label: '晚餐时段 (17:00-21:00)', value: 'dinner' },
      { label: '自定义时间', value: 'custom' }
    ],
    consentChecked: false,
    consentPopupVisible: false
  },

  ...merchantStoreRegistrationRuntimeMethods
})
