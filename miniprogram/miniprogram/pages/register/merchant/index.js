"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
const logger_1 = require("../../../utils/logger");
const error_handler_1 = require("../../../utils/error-handler");
const draft_storage_1 = require("../../../utils/draft-storage");
const onboarding_1 = require("../../../api/onboarding");
const DRAFT_KEY = 'merchant_register_draft';
Page({
    data: {
        navBarHeight: 88,
        currentStep: 0, // 0: Intro, 1: Upload, 2: Info, 3: Location, 4: Review, 5: Polling
        isSubmitting: false, // 防止重复提交
        applicationInitialized: false, // 标记申请草稿是否已成功创建
        formData: {
            // 基本信息
            name: '',
            phone: '',
            address: '',
            addressDetail: '',
            regionId: 0,
            latitude: 0,
            longitude: 0,
            // 证照信息
            licenseName: '',
            creditCode: '',
            registerAddress: '',
            licenseValidity: '',
            businessScope: '',
            foodLicenseValidity: '',
            // 法人信息
            legalPerson: '',
            idCard: '',
            gender: '',
            hometown: '',
            currentAddress: '',
            idCardValidity: '',
            // 结算信息
            bankName: '',
            bankAccount: '',
            accountName: ''
        },
        // 图片 (包含 url 和 rawUrl)
        licenseImages: [],
        foodLicenseImages: [],
        idCardFrontImages: [],
        idCardBackImages: [],
        accountPermitImages: [],
        storefrontImages: [], // 门头照，最多3张
        environmentImages: [], // 环境照，最多5张
        // OCR原始结果 (用于一致性校验)
        ocrResults: {
            license: null,
            idCard: null
        },
        // 选择器状态
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
        ]
    },
    onLoad() {
        return __awaiter(this, void 0, void 0, function* () {
            // 先从后端加载草稿数据（后端优先）
            yield this.initApplication();
            // 后端数据加载后，再尝试从本地草稿恢复（仅补充后端没有返回的数据）
            // 注意：如果后端已返回数据，loadDraft 可能会覆盖，需要谨慎处理
            // 暂时禁用 loadDraft，完全依赖后端数据
            // this.loadDraft()
        });
    },
    initApplication() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a, _b, _c, _d, _e, _f, _g, _h, _j, _k, _l, _m, _o;
            wx.showLoading({ title: '加载中...' });
            console.log('[DEBUG] initApplication 开始');
            try {
                const res = yield (0, onboarding_1.getMerchantApplication)();
                console.log('[DEBUG] getMerchantApplication 返回:', res);
                if (!res) {
                    logger_1.logger.warn('[MerchantRegister] getMerchantApplication returned null', undefined, 'initApplication');
                    wx.hideLoading();
                    wx.showToast({ title: '无法创建申请，请重试', icon: 'none' });
                    return;
                }
                const data = res;
                console.log('[DEBUG] 后端返回的原始数据:', JSON.stringify(data, null, 2));
                logger_1.logger.info('[MerchantRegister] 加载申请数据', data, 'initApplication');
                // 标记申请已初始化成功
                this.setData({ applicationInitialized: true });
                // 检查申请状态 - 如果已提交或已通过，直接跳转
                if (data.status === 'approved') {
                    wx.showToast({ title: '您已是商户', icon: 'success' });
                    setTimeout(() => {
                        wx.reLaunch({ url: '/pages/merchant/dashboard/index' });
                    }, 1000);
                    return;
                }
                if (data.status === 'submitted') {
                    wx.showToast({ title: '申请审核中', icon: 'none' });
                    this.setData({ currentStep: 5 });
                    this.startPollingStatus();
                    return;
                }
                const safeStr = (val) => {
                    if (val === null || val === undefined || val === true || val === 'true')
                        return '';
                    return String(val);
                };
                // 映射 formData
                const formData = Object.assign(Object.assign({}, this.data.formData), { name: safeStr(data.merchant_name), phone: safeStr(data.contact_phone), address: safeStr(data.business_address), addressDetail: safeStr(data.business_address_detail), latitude: data.latitude ? parseFloat(data.latitude) : 0, longitude: data.longitude ? parseFloat(data.longitude) : 0, 
                    // OCR 回填
                    licenseName: safeStr((_a = data.business_license_ocr) === null || _a === void 0 ? void 0 : _a.enterprise_name), creditCode: safeStr(data.business_license_number || ((_b = data.business_license_ocr) === null || _b === void 0 ? void 0 : _b.reg_num) || ((_c = data.business_license_ocr) === null || _c === void 0 ? void 0 : _c.credit_code)), registerAddress: safeStr((_d = data.business_license_ocr) === null || _d === void 0 ? void 0 : _d.address), licenseValidity: safeStr((_e = data.business_license_ocr) === null || _e === void 0 ? void 0 : _e.valid_period), businessScope: safeStr(data.business_scope || ((_f = data.business_license_ocr) === null || _f === void 0 ? void 0 : _f.business_scope)), foodLicenseValidity: safeStr((_g = data.food_permit_ocr) === null || _g === void 0 ? void 0 : _g.valid_to), legalPerson: safeStr(((_h = data.id_card_front_ocr) === null || _h === void 0 ? void 0 : _h.name) || data.legal_person_name || ((_j = data.business_license_ocr) === null || _j === void 0 ? void 0 : _j.legal_representative)), idCard: safeStr(((_k = data.id_card_front_ocr) === null || _k === void 0 ? void 0 : _k.id_number) || data.legal_person_id_number), gender: safeStr((_l = data.id_card_front_ocr) === null || _l === void 0 ? void 0 : _l.gender), hometown: safeStr((_m = data.id_card_front_ocr) === null || _m === void 0 ? void 0 : _m.address), idCardValidity: safeStr((_o = data.id_card_back_ocr) === null || _o === void 0 ? void 0 : _o.valid_date), currentAddress: safeStr(data.legal_person_contact_address), bankName: safeStr(data.bank_name), bankAccount: safeStr(data.bank_account), accountName: safeStr(data.bank_account_name) });
                // OCR 原始数据
                const ocrResults = {
                    license: data.business_license_ocr || null,
                    idCard: data.id_card_front_ocr || null
                };
                // 解析图片 URL
                const { resolveImageURL } = require('../../../utils/image-security');
                const safeResolve = (url) => __awaiter(this, void 0, void 0, function* () {
                    if (!url)
                        return '';
                    try {
                        return yield resolveImageURL(url);
                    }
                    catch (_a) {
                        return '';
                    }
                });
                const licenseUrl = yield safeResolve(data.business_license_image_url || '');
                const foodLicenseUrl = yield safeResolve(data.food_permit_url || '');
                const idCardFrontUrl = yield safeResolve(data.legal_person_id_front_url || '');
                const idCardBackUrl = yield safeResolve(data.legal_person_id_back_url || '');
                // 确保所有数组都是数组，绝不为 null，同时保存 rawUrl 用于重试
                const licenseImages = licenseUrl ? [{ url: licenseUrl, rawUrl: data.business_license_image_url }] : [];
                const foodLicenseImages = foodLicenseUrl ? [{ url: foodLicenseUrl, rawUrl: data.food_permit_url }] : [];
                const idCardFrontImages = idCardFrontUrl ? [{ url: idCardFrontUrl, rawUrl: data.legal_person_id_front_url }] : [];
                const idCardBackImages = idCardBackUrl ? [{ url: idCardBackUrl, rawUrl: data.legal_person_id_back_url }] : [];
                const accountPermitImages = [];
                // 门头照
                const storefrontRaw = Array.isArray(data.storefront_images) ? data.storefront_images : [];
                const storefrontImages = [];
                for (const url of storefrontRaw) {
                    const resolved = yield safeResolve(url);
                    if (resolved)
                        storefrontImages.push({ url: resolved, rawUrl: url });
                }
                // 环境照
                const environmentRaw = Array.isArray(data.environment_images) ? data.environment_images : [];
                const environmentImages = [];
                for (const url of environmentRaw) {
                    const resolved = yield safeResolve(url);
                    if (resolved)
                        environmentImages.push({ url: resolved, rawUrl: url });
                }
                console.log('[DEBUG] setData payload:', { formData, licenseImages: licenseImages.length, storefrontImages: storefrontImages.length, environmentImages: environmentImages.length });
                // 关键：一次性设置所有数据
                this.setData({
                    formData,
                    ocrResults,
                    licenseImages,
                    foodLicenseImages,
                    idCardFrontImages,
                    idCardBackImages,
                    accountPermitImages,
                    storefrontImages,
                    environmentImages
                }, () => {
                    // 数据加载后立即进行法人一致性校验
                    this.checkLegalPersonConsistency();
                });
                logger_1.logger.debug('[MerchantRegister] initApplication 完成', formData, 'initApplication');
                wx.hideLoading();
            }
            catch (e) {
                wx.hideLoading();
                console.error('[MerchantRegister] initApplication Error:', e);
                // 如果初始化失败，提示用户刷新页面
                wx.showModal({
                    title: '加载失败',
                    content: (e === null || e === void 0 ? void 0 : e.userMessage) || '无法加载申请数据，请检查网络后重试',
                    confirmText: '重试',
                    cancelText: '返回',
                    success: (res) => {
                        if (res.confirm) {
                            // 重试初始化
                            this.initApplication();
                        }
                        else {
                            wx.navigateBack();
                        }
                    }
                });
            }
        });
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    // ==================== 草稿管理 ====================
    saveDraft() {
        const data = {
            formData: this.data.formData,
            licenseImages: this.data.licenseImages,
            foodLicenseImages: this.data.foodLicenseImages,
            idCardFrontImages: this.data.idCardFrontImages,
            idCardBackImages: this.data.idCardBackImages,
            accountPermitImages: this.data.accountPermitImages,
            storefrontImages: this.data.storefrontImages,
            environmentImages: this.data.environmentImages,
            ocrResults: this.data.ocrResults
        };
        draft_storage_1.DraftStorage.save(DRAFT_KEY, data);
    },
    syncToBackend() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a, _b, _c, _d;
            if (!this.data.applicationInitialized)
                return;
            const { formData } = this.data;
            // Only sync if minimum required fields are present
            if (!formData.name) {
                logger_1.logger.debug('[MerchantRegister] syncToBackend skipped: no name', { name: formData.name });
                return;
            }
            try {
                // 构造完整的数据对象
                // 注意：字段映射要与后端 UpdateMerchantBasicInfoRequest 一致
                const payload = {
                    // Basic Info
                    merchant_name: formData.name,
                    contact_phone: formData.phone,
                    business_address: formData.addressDetail || formData.address,
                    longitude: formData.longitude ? String(formData.longitude) : undefined,
                    latitude: formData.latitude ? String(formData.latitude) : undefined,
                    region_id: formData.regionId,
                    // License Info
                    business_license_number: formData.creditCode,
                    business_license_image_url: (_a = this.data.licenseImages[0]) === null || _a === void 0 ? void 0 : _a.url,
                    business_scope: formData.businessScope,
                    // Legal Person Info
                    legal_person_name: formData.legalPerson,
                    legal_person_id_number: formData.idCard,
                    legal_person_id_front_url: (_b = this.data.idCardFrontImages[0]) === null || _b === void 0 ? void 0 : _b.url,
                    legal_person_id_back_url: (_c = this.data.idCardBackImages[0]) === null || _c === void 0 ? void 0 : _c.url,
                    // Food Permit
                    food_permit_url: (_d = this.data.foodLicenseImages[0]) === null || _d === void 0 ? void 0 : _d.url,
                    // Images (Assuming API supports it, otherwise separate call needed)
                    storefront_images: (this.data.storefrontImages || []).map(i => i.url),
                    environment_images: (this.data.environmentImages || []).map(i => i.url)
                };
                console.log('[MerchantRegister] syncToBackend payload:', JSON.stringify(payload, null, 2));
                yield (0, onboarding_1.updateMerchantBasicInfo)(payload);
                console.log('[MerchantRegister] Sync to backend success');
            }
            catch (err) {
                console.error('[MerchantRegister] Sync to backend failed', err);
                console.error('[MerchantRegister] Error details:', (err === null || err === void 0 ? void 0 : err.originalError) || (err === null || err === void 0 ? void 0 : err.data) || (err === null || err === void 0 ? void 0 : err.message));
                // Silent fail to not interrupt user flow, unless final submit
            }
        });
    },
    loadDraft() {
        const draft = draft_storage_1.DraftStorage.load(DRAFT_KEY);
        if (draft) {
            // 深度清洗 formData (防止 null/true/undefined 导致崩溃)
            const safeFormData = Object.assign({}, this.data.formData);
            if (draft.formData) {
                // No special sanitization needed for removed fields
                Object.keys(safeFormData).forEach(key => {
                    let val = draft.formData[key];
                    // 1. 强制转为空字符串的情况
                    if (val === null || val === undefined) {
                        val = '';
                    }
                    // 2. 修复错误的 "true" 值 (字符串或布尔)
                    if (val === true || val === 'true') {
                        val = '';
                    }
                    // 3. 类型赋值
                    if (key === 'latitude' || key === 'longitude') {
                        safeFormData[key] = Number(val) || 0;
                    }
                    else {
                        safeFormData[key] = String(val);
                    }
                });
            }
            this.setData({
                formData: safeFormData,
                licenseImages: draft.licenseImages || [],
                foodLicenseImages: draft.foodLicenseImages || [],
                idCardFrontImages: draft.idCardFrontImages || [],
                idCardBackImages: draft.idCardBackImages || [],
                accountPermitImages: draft.accountPermitImages || [],
                shopImages: draft.shopImages || [],
                ocrResults: draft.ocrResults || { license: null, idCard: null }
            });
        }
    },
    // ==================== 表单输入 ====================
    updateFormData(key, value) {
        this.setData({ [`formData.${key}`]: value });
        this.saveDraft();
    },
    onNameInput(e) { this.updateFormData('name', e.detail.value); },
    onPhoneInput(e) { this.updateFormData('phone', e.detail.value); },
    onAddressDetailInput(e) { this.updateFormData('addressDetail', e.detail.value); },
    // 证照信息输入
    onLicenseNameInput(e) { this.updateFormData('licenseName', e.detail.value); },
    onCreditCodeInput(e) { this.updateFormData('creditCode', e.detail.value); },
    onRegisterAddressInput(e) { this.updateFormData('registerAddress', e.detail.value); },
    onLicenseValidityInput(e) { this.updateFormData('licenseValidity', e.detail.value); },
    onBusinessScopeInput(e) { this.updateFormData('businessScope', e.detail.value); },
    onFoodLicenseValidityInput(e) { this.updateFormData('foodLicenseValidity', e.detail.value); },
    // 法人信息输入
    onLegalPersonInput(e) { this.updateFormData('legalPerson', e.detail.value); },
    onIdCardInput(e) { this.updateFormData('idCard', e.detail.value); },
    onGenderInput(e) { this.updateFormData('gender', e.detail.value); },
    onHometownInput(e) { this.updateFormData('hometown', e.detail.value); },
    onCurrentAddressInput(e) { this.updateFormData('currentAddress', e.detail.value); },
    onIdCardValidityInput(e) { this.updateFormData('idCardValidity', e.detail.value); },
    // 结算信息输入
    onBankNameInput(e) { this.updateFormData('bankName', e.detail.value); },
    onBankAccountInput(e) { this.updateFormData('bankAccount', e.detail.value); },
    onAccountNameInput(e) { this.updateFormData('accountName', e.detail.value); },
    // ==================== 地址选择 ====================
    onAddressInput(e) { this.updateFormData('address', e.detail.value); },
    // ==================== 地址选择 ====================
    onChooseAddress() {
        wx.chooseLocation({
            success: (res) => {
                // 用户强调需要显示详细地址 (省市区+街道+门牌+名称)
                // 无论返回结构如何，都要尽可能组合出完整地址
                const addr = res.address || '';
                const name = res.name || '';
                // 组合地址: 优先全部信息，确保不遗漏
                let fullAddress = '';
                if (addr && name) {
                    // 如果地址已包含名称，不重复
                    fullAddress = addr.includes(name) ? addr : `${addr} ${name}`;
                }
                else if (addr) {
                    fullAddress = addr;
                }
                else if (name) {
                    fullAddress = name;
                }
                // 如果 fullAddress 还是空的，尝试用经纬度提示
                if (!fullAddress && (res.latitude || res.longitude)) {
                    fullAddress = `位置: ${res.latitude.toFixed(6)}, ${res.longitude.toFixed(6)}`;
                }
                console.log('[ChooseLocation] Final Address:', fullAddress, { res });
                // 使用 setData 的回调确保视图更新
                this.setData({
                    'formData.address': fullAddress,
                    'formData.addressDetail': fullAddress, // 同时设置 addressDetail 保持一致性
                    'formData.latitude': res.latitude,
                    'formData.longitude': res.longitude
                }, () => {
                    this.saveDraft();
                });
            },
            fail: (err) => {
                if (err.errMsg.includes('auth deny')) {
                    wx.showModal({
                        title: '需要位置权限',
                        content: '请在设置中开启位置权限',
                        confirmText: '去设置',
                        success: (modalRes) => {
                            if (modalRes.confirm) {
                                wx.openSetting();
                            }
                        }
                    });
                }
            }
        });
    },
    // ==================== 选择器 ====================
    // ==================== 步骤导航 ====================
    nextStep() {
        const { currentStep, licenseImages, foodLicenseImages, idCardFrontImages, idCardBackImages } = this.data;
        // Step 1 check (Intro) - No validation
        if (currentStep === 0) {
            this.syncToBackend();
            this.setData({ currentStep: 1 });
            return;
        }
        // Step 2 check (Upload) - Require all images
        if (currentStep === 1) {
            if (licenseImages.length === 0 || foodLicenseImages.length === 0 || idCardFrontImages.length === 0 || idCardBackImages.length === 0) {
                wx.showToast({ title: '请上传所有必需的证照', icon: 'none' });
                return;
            }
            // Transition to Step 3: Populate data from OCR results if needed
            // The OCR results are already in `formData` courtesy of `onUpload` handlers.
            // We just move to next step.
            this.setData({ currentStep: 2 });
            return;
        }
        // Step 3 check (Info) - 必填字段校验
        if (currentStep === 2) {
            const { name, phone, creditCode, legalPerson, idCard } = this.data.formData;
            const missingFields = [];
            if (!(name === null || name === void 0 ? void 0 : name.trim()))
                missingFields.push('店铺名称');
            if (!(phone === null || phone === void 0 ? void 0 : phone.trim()))
                missingFields.push('联系电话');
            if (!(creditCode === null || creditCode === void 0 ? void 0 : creditCode.trim()))
                missingFields.push('统一信用代码');
            if (!(legalPerson === null || legalPerson === void 0 ? void 0 : legalPerson.trim()))
                missingFields.push('法人姓名');
            if (!(idCard === null || idCard === void 0 ? void 0 : idCard.trim()))
                missingFields.push('身份证号');
            if (missingFields.length > 0) {
                const message = missingFields.length <= 3
                    ? `请填写: ${missingFields.join('、')}`
                    : `还有 ${missingFields.length} 项必填信息未完善`;
                wx.showToast({ title: message, icon: 'none', duration: 3000 });
                return;
            }
            // 进入 Step 3 前刷新门头照/环境照签名
            this.refreshShopImageUrls();
            this.setData({ currentStep: 3 });
            return;
        }
        // Step 4 check (Location)
        if (currentStep === 3) {
            // 使用 address 字段（地图选择时设置的）而不是 addressDetail
            const { address, latitude, longitude } = this.data.formData;
            if (!address) {
                wx.showToast({ title: '请选择店铺地址', icon: 'none' });
                return;
            }
            // regionId 由后端根据经纬度自动匹配，不强制前端校验
            if (!latitude || !longitude) {
                wx.showToast({ title: '请通过地图选择精确位置', icon: 'none' });
                return;
            }
            this.syncToBackend();
            this.setData({ currentStep: 4 });
            return;
        }
    },
    prevStep() {
        const { currentStep } = this.data;
        if (currentStep > 0) {
            this.setData({ currentStep: currentStep - 1 });
        }
    },
    // ==================== 图片上传与OCR ====================
    // 营业执照
    onLicenseUpload(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { path } = e.detail;
            if (!path)
                return;
            // 检查申请是否已初始化
            if (!this.data.applicationInitialized) {
                wx.showToast({ title: '请等待页面加载完成', icon: 'none' });
                return;
            }
            // 先显示本地图片
            this.setData({ licenseImages: [{ url: path }] });
            this.saveDraft();
            wx.showLoading({ title: '上传中...' });
            try {
                // 上传并触发异步 OCR
                const result = yield (0, onboarding_1.ocrBusinessLicense)(path);
                logger_1.logger.info('[MerchantRegister] 营业执照上传成功，OCR已入队', result, 'onLicenseUpload');
                wx.hideLoading();
                wx.showToast({ title: '上传成功，识别中...', icon: 'none' });
                // 开始轮询 OCR 结果
                this.pollOcrStatus('business_license_ocr', (ocr) => {
                    if (ocr) {
                        this.setData({
                            'formData.licenseName': ocr.enterprise_name || '',
                            'formData.creditCode': ocr.reg_num || ocr.credit_code || '',
                            'formData.registerAddress': ocr.address || '',
                            'formData.legalPerson': ocr.legal_representative || '',
                            'formData.licenseValidity': ocr.valid_period || '',
                            'formData.businessScope': ocr.business_scope || '',
                            'ocrResults.license': ocr
                        });
                        this.saveDraft();
                        this.checkLegalPersonConsistency();
                        wx.showToast({ title: '识别成功', icon: 'success' });
                    }
                });
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('[MerchantRegister] 营业执照上传失败', error, 'onLicenseUpload');
                // 显示更具体的错误信息
                const errMsg = (error === null || error === void 0 ? void 0 : error.userMessage) || (error === null || error === void 0 ? void 0 : error.message) || '上传失败，请重试';
                wx.showToast({ title: errMsg, icon: 'none', duration: 3000 });
            }
        });
    },
    onLicenseRemove() {
        this.setData({ licenseImages: [] });
        this.saveDraft();
    },
    // 食品经营许可证
    onFoodLicenseUpload(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { path } = e.detail;
            if (!path)
                return;
            if (!this.data.applicationInitialized) {
                wx.showToast({ title: '请等待页面加载完成', icon: 'none' });
                return;
            }
            this.setData({ foodLicenseImages: [{ url: path }] });
            this.saveDraft();
            wx.showLoading({ title: '上传中...' });
            try {
                const result = yield (0, onboarding_1.ocrFoodPermit)(path);
                logger_1.logger.info('[MerchantRegister] 食品许可证上传成功', result, 'onFoodLicenseUpload');
                wx.hideLoading();
                wx.showToast({ title: '上传成功，识别中...', icon: 'none' });
                this.pollOcrStatus('food_permit_ocr', (ocr) => {
                    if (ocr) {
                        this.setData({
                            'formData.foodLicenseValidity': ocr.valid_to || ''
                        });
                        this.saveDraft();
                        wx.showToast({ title: '识别成功', icon: 'success' });
                    }
                });
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('[MerchantRegister] 食品许可证上传失败', error, 'onFoodLicenseUpload');
                const errMsg = (error === null || error === void 0 ? void 0 : error.userMessage) || (error === null || error === void 0 ? void 0 : error.message) || '上传失败，请重试';
                wx.showToast({ title: errMsg, icon: 'none', duration: 3000 });
            }
        });
    },
    onFoodLicenseRemove() {
        this.setData({ foodLicenseImages: [] });
        this.saveDraft();
    },
    // 身份证正面
    onIdCardFrontUpload(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { path } = e.detail;
            if (!path)
                return;
            if (!this.data.applicationInitialized) {
                wx.showToast({ title: '请等待页面加载完成', icon: 'none' });
                return;
            }
            this.setData({ idCardFrontImages: [{ url: path }] });
            this.saveDraft();
            wx.showLoading({ title: '上传中...' });
            try {
                const result = yield (0, onboarding_1.ocrIdCard)(path, 'Front');
                logger_1.logger.info('[MerchantRegister] 身份证正面上传成功', result, 'onIdCardFrontUpload');
                wx.hideLoading();
                wx.showToast({ title: '上传成功，识别中...', icon: 'none' });
                this.pollOcrStatus('id_card_front_ocr', (ocr) => {
                    if (ocr) {
                        this.setData({
                            'formData.legalPerson': ocr.name || '',
                            'formData.idCard': ocr.id_number || '',
                            'formData.gender': ocr.gender || '',
                            'formData.hometown': ocr.address || '',
                            'ocrResults.idCard': ocr
                        });
                        this.saveDraft();
                        this.checkLegalPersonConsistency();
                        wx.showToast({ title: '识别成功', icon: 'success' });
                    }
                });
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('[MerchantRegister] 身份证正面上传失败', error, 'onIdCardFrontUpload');
                const errMsg = (error === null || error === void 0 ? void 0 : error.userMessage) || (error === null || error === void 0 ? void 0 : error.message) || '上传失败，请重试';
                wx.showToast({ title: errMsg, icon: 'none', duration: 3000 });
            }
        });
    },
    onIdCardFrontRemove() {
        this.setData({ idCardFrontImages: [] });
        this.saveDraft();
    },
    // 身份证反面
    onIdCardBackUpload(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { path } = e.detail;
            if (!path)
                return;
            if (!this.data.applicationInitialized) {
                wx.showToast({ title: '请等待页面加载完成', icon: 'none' });
                return;
            }
            this.setData({ idCardBackImages: [{ url: path }] });
            this.saveDraft();
            wx.showLoading({ title: '上传中...' });
            try {
                const result = yield (0, onboarding_1.ocrIdCard)(path, 'Back');
                logger_1.logger.info('[MerchantRegister] 身份证反面上传成功', result, 'onIdCardBackUpload');
                wx.hideLoading();
                wx.showToast({ title: '上传成功，识别中...', icon: 'none' });
                this.pollOcrStatus('id_card_back_ocr', (ocr) => {
                    if (ocr) {
                        this.setData({
                            'formData.idCardValidity': ocr.valid_date || ''
                        });
                        this.saveDraft();
                        wx.showToast({ title: '识别成功', icon: 'success' });
                    }
                });
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('[MerchantRegister] 身份证反面上传失败', error, 'onIdCardBackUpload');
                const errMsg = (error === null || error === void 0 ? void 0 : error.userMessage) || (error === null || error === void 0 ? void 0 : error.message) || '上传失败，请重试';
                wx.showToast({ title: errMsg, icon: 'none', duration: 3000 });
            }
        });
    },
    onIdCardBackRemove() {
        this.setData({ idCardBackImages: [] });
        this.saveDraft();
    },
    // 开户许可证
    onAccountPermitUpload(e) {
        const { path } = e.detail;
        const files = path ? [{ url: path }] : [];
        this.setData({ accountPermitImages: files });
        this.saveDraft();
    },
    onAccountPermitRemove() {
        this.setData({ accountPermitImages: [] });
        this.saveDraft();
    },
    // ==================== 图片加载错误重试 ====================
    onImageError(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { rawUrl, retryCount } = e.detail;
            if (!rawUrl)
                return;
            console.log('[MerchantRegister] 图片加载失败，重新签名:', rawUrl, 'retryCount:', retryCount);
            try {
                const { resolveImageURL } = require('../../../utils/image-security');
                const newSignedUrl = yield resolveImageURL(rawUrl);
                // 根据 rawUrl 找到对应的图片数组并更新
                const imageArrays = ['licenseImages', 'foodLicenseImages', 'idCardFrontImages', 'idCardBackImages'];
                for (const arrayName of imageArrays) {
                    const arr = this.data[arrayName];
                    for (let i = 0; i < arr.length; i++) {
                        if (arr[i].rawUrl === rawUrl) {
                            const newArr = [...arr];
                            newArr[i] = Object.assign(Object.assign({}, newArr[i]), { url: newSignedUrl });
                            this.setData({ [arrayName]: newArr });
                            console.log('[MerchantRegister] 已更新签名 URL:', arrayName, i);
                            return;
                        }
                    }
                }
            }
            catch (error) {
                logger_1.logger.error('[MerchantRegister] 重新签名失败', error);
            }
        });
    },
    // 刷新门头照/环境照签名 URL
    refreshShopImageUrls() {
        return __awaiter(this, void 0, void 0, function* () {
            const { resolveImageURL } = require('../../../utils/image-security');
            // 刷新门头照
            const storefrontImages = [...this.data.storefrontImages];
            for (let i = 0; i < storefrontImages.length; i++) {
                const img = storefrontImages[i];
                if (img.rawUrl) {
                    try {
                        const newUrl = yield resolveImageURL(img.rawUrl);
                        storefrontImages[i] = Object.assign(Object.assign({}, img), { url: newUrl });
                    }
                    catch (e) {
                        console.warn('[MerchantRegister] 刷新门头照签名失败:', img.rawUrl);
                    }
                }
            }
            // 刷新环境照
            const environmentImages = [...this.data.environmentImages];
            for (let i = 0; i < environmentImages.length; i++) {
                const img = environmentImages[i];
                if (img.rawUrl) {
                    try {
                        const newUrl = yield resolveImageURL(img.rawUrl);
                        environmentImages[i] = Object.assign(Object.assign({}, img), { url: newUrl });
                    }
                    catch (e) {
                        console.warn('[MerchantRegister] 刷新环境照签名失败:', img.rawUrl);
                    }
                }
            }
            this.setData({ storefrontImages, environmentImages });
            console.log('[MerchantRegister] 已刷新门头照/环境照签名');
        });
    },
    // 字段失去焦点时保存
    onFieldBlur(e) {
        const field = e.currentTarget.dataset.field;
        const value = e.detail.value;
        if (field && typeof field === 'string') {
            // 使用 setData 回调确保数据更新后再同步到后端
            this.setData({ [`formData.${field}`]: value }, () => {
                this.saveDraft();
                this.syncToBackend();
            });
        }
    },
    // ==================== 门头照/环境照上传 ====================
    onStorefrontImageUpload(e) {
        return __awaiter(this, void 0, void 0, function* () {
            // t-upload bind:add 传递的是 files 数组
            const files = e.detail.files;
            if (!files || files.length === 0) {
                console.warn('[MerchantRegister] 门头照上传: 无文件', e.detail);
                return;
            }
            // 取最后一个新添加的文件
            const newFile = files[files.length - 1];
            if (!(newFile === null || newFile === void 0 ? void 0 : newFile.url)) {
                console.warn('[MerchantRegister] 门头照上传: 文件无 URL', newFile);
                return;
            }
            const currentImages = [...this.data.storefrontImages];
            if (currentImages.length >= 3) {
                wx.showToast({ title: '最多上传3张门头照', icon: 'none' });
                return;
            }
            console.log('[MerchantRegister] 门头照上传开始:', newFile.url);
            wx.showLoading({ title: '上传中...' });
            try {
                const { uploadMerchantImage, updateMerchantImages } = require('../../../api/onboarding');
                const { API_CONFIG } = require('../../../config/index');
                const result = yield uploadMerchantImage(newFile.url, 'storefront');
                console.log('[MerchantRegister] 门头照上传响应:', JSON.stringify(result));
                const rawUrl = result.image_url; // 后端返回的相对路径
                if (!rawUrl) {
                    console.error('[MerchantRegister] 响应中没有 image_url 字段:', result);
                    throw new Error('上传响应格式错误');
                }
                // 调用签名接口获取可访问的 URL
                const { resolveImageURL } = require('../../../utils/image-security');
                const displayUrl = yield resolveImageURL(rawUrl);
                console.log('[MerchantRegister] 门头照显示 URL:', displayUrl);
                currentImages.push({ url: displayUrl, rawUrl });
                this.setData({ storefrontImages: currentImages });
                // 保存到后端（使用相对路径）
                yield updateMerchantImages({
                    storefront_images: currentImages.map((img) => img.rawUrl || img.url)
                });
                wx.hideLoading();
                wx.showToast({ title: '上传成功', icon: 'success' });
                this.saveDraft();
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('[MerchantRegister] 门头照上传失败', error);
                wx.showToast({ title: '上传失败', icon: 'none' });
            }
        });
    },
    onStorefrontImageRemove(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { index } = e.detail;
            const images = [...this.data.storefrontImages];
            images.splice(index, 1);
            this.setData({ storefrontImages: images });
            try {
                const { updateMerchantImages } = require('../../../api/onboarding');
                yield updateMerchantImages({
                    storefront_images: images.map((img) => img.url)
                });
                this.saveDraft();
            }
            catch (error) {
                logger_1.logger.error('[MerchantRegister] 删除门头照失败', error);
            }
        });
    },
    onEnvironmentImageUpload(e) {
        return __awaiter(this, void 0, void 0, function* () {
            // t-upload bind:add 传递的是 files 数组
            const files = e.detail.files;
            if (!files || files.length === 0) {
                console.warn('[MerchantRegister] 环境照上传: 无文件', e.detail);
                return;
            }
            // 取最后一个新添加的文件
            const newFile = files[files.length - 1];
            if (!(newFile === null || newFile === void 0 ? void 0 : newFile.url)) {
                console.warn('[MerchantRegister] 环境照上传: 文件无 URL', newFile);
                return;
            }
            const currentImages = [...this.data.environmentImages];
            if (currentImages.length >= 5) {
                wx.showToast({ title: '最多上传5张环境照', icon: 'none' });
                return;
            }
            console.log('[MerchantRegister] 环境照上传开始:', newFile.url);
            wx.showLoading({ title: '上传中...' });
            try {
                const { uploadMerchantImage, updateMerchantImages } = require('../../../api/onboarding');
                const { API_CONFIG } = require('../../../config/index');
                const result = yield uploadMerchantImage(newFile.url, 'environment');
                console.log('[MerchantRegister] 环境照上传响应:', JSON.stringify(result));
                const rawUrl = result.image_url; // 后端返回的相对路径
                if (!rawUrl) {
                    console.error('[MerchantRegister] 环境照响应中没有 image_url 字段:', result);
                    throw new Error('上传响应格式错误');
                }
                // 调用签名接口获取可访问的 URL
                const { resolveImageURL } = require('../../../utils/image-security');
                const displayUrl = yield resolveImageURL(rawUrl);
                console.log('[MerchantRegister] 环境照显示 URL:', displayUrl);
                currentImages.push({ url: displayUrl, rawUrl });
                this.setData({ environmentImages: currentImages });
                // 保存到后端（使用相对路径）
                yield updateMerchantImages({
                    environment_images: currentImages.map((img) => img.rawUrl || img.url)
                });
                wx.hideLoading();
                wx.showToast({ title: '上传成功', icon: 'success' });
                this.saveDraft();
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('[MerchantRegister] 环境照上传失败', error);
                wx.showToast({ title: '上传失败', icon: 'none' });
            }
        });
    },
    onEnvironmentImageRemove(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { index } = e.detail;
            const images = [...this.data.environmentImages];
            images.splice(index, 1);
            this.setData({ environmentImages: images });
            try {
                const { updateMerchantImages } = require('../../../api/onboarding');
                yield updateMerchantImages({
                    environment_images: images.map((img) => img.url)
                });
                this.saveDraft();
            }
            catch (error) {
                logger_1.logger.error('[MerchantRegister] 删除环境照失败', error);
            }
        });
    },
    // ==================== 提交申请 ====================
    // ==================== 校验逻辑 ====================
    checkLegalPersonConsistency() {
        var _a, _b, _c;
        const { ocrResults, formData } = this.data;
        const licenseName = ((_a = ocrResults.license) === null || _a === void 0 ? void 0 : _a.legal_representative) || ((_b = ocrResults.license) === null || _b === void 0 ? void 0 : _b.person);
        const idName = ((_c = ocrResults.idCard) === null || _c === void 0 ? void 0 : _c.name) || formData.legalPerson;
        console.log('[DEBUG] Consistency Check:', {
            licenseName,
            idName,
            licenseSource: ocrResults.license,
            idSource: ocrResults.idCard,
            formDataLegel: formData.legalPerson
        });
        if (licenseName && idName && licenseName !== idName) {
            wx.showModal({
                title: '法人信息不一致',
                content: `营业执照法人(${licenseName})与身份证/填写姓名(${idName})不一致，可能导致审核不通过，请确认。`,
                showCancel: false,
                confirmText: '我知道了'
            });
        }
    },
    // ==================== OCR 轮询 ====================
    pollOcrStatus(fieldKey, callback) {
        let attempts = 0;
        const maxAttempts = 30; // 30 * 2s = 60s max
        const intervalId = setInterval(() => __awaiter(this, void 0, void 0, function* () {
            attempts++;
            try {
                const res = yield (0, onboarding_1.getMerchantApplication)();
                const ocrData = res[fieldKey];
                if ((ocrData === null || ocrData === void 0 ? void 0 : ocrData.status) === 'done') {
                    clearInterval(intervalId);
                    callback(ocrData);
                }
                else if ((ocrData === null || ocrData === void 0 ? void 0 : ocrData.status) === 'failed') {
                    clearInterval(intervalId);
                    wx.showToast({ title: `识别失败: ${ocrData.error || '未知错误'}`, icon: 'none' });
                }
            }
            catch (e) {
                console.error('[pollOcrStatus] Error:', e);
            }
            if (attempts >= maxAttempts) {
                clearInterval(intervalId);
                wx.showToast({ title: '识别超时，请重试', icon: 'none' });
            }
        }), 2000);
    },
    // ==================== 提交申请 ====================
    onSubmit() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a, _b, _c, _d, _e, _f;
            const { formData, isSubmitting, licenseImages, idCardFrontImages, idCardBackImages } = this.data;
            // 防止重复提交
            if (isSubmitting) {
                logger_1.logger.warn('[MerchantRegister] 重复提交，已忽略');
                return;
            }
            // ========== 前端校验 ==========
            const missingFields = [];
            // 基本信息
            if (!((_a = formData.name) === null || _a === void 0 ? void 0 : _a.trim()))
                missingFields.push('店铺名称');
            if (!((_b = formData.phone) === null || _b === void 0 ? void 0 : _b.trim()))
                missingFields.push('联系电话');
            if (!((_c = formData.address) === null || _c === void 0 ? void 0 : _c.trim()))
                missingFields.push('店铺地址');
            // 证照图片
            if (!licenseImages || licenseImages.length === 0)
                missingFields.push('营业执照');
            if (!idCardFrontImages || idCardFrontImages.length === 0)
                missingFields.push('身份证正面');
            if (!idCardBackImages || idCardBackImages.length === 0)
                missingFields.push('身份证背面');
            // OCR 识别的信息（可选但建议有）
            if (!((_d = formData.creditCode) === null || _d === void 0 ? void 0 : _d.trim()))
                missingFields.push('统一信用代码(OCR)');
            if (!((_e = formData.legalPerson) === null || _e === void 0 ? void 0 : _e.trim()))
                missingFields.push('法人姓名(OCR)');
            if (!((_f = formData.idCard) === null || _f === void 0 ? void 0 : _f.trim()))
                missingFields.push('身份证号(OCR)');
            if (missingFields.length > 0) {
                const message = missingFields.length <= 3
                    ? `请填写: ${missingFields.join('、')}`
                    : `还有 ${missingFields.length} 项必填信息未完善`;
                wx.showToast({ title: message, icon: 'none', duration: 3000 });
                logger_1.logger.warn('[MerchantRegister] 校验失败', { missingFields });
                return;
            }
            this.setData({ isSubmitting: true });
            try {
                // 1. Sync latest data to backend (prevent "empty merchant name" error)
                yield this.syncToBackend();
                // 2. Enter Polling State UI
                this.setData({ currentStep: 5 });
                // 3. Submit Application (自动审核)
                const result = yield (0, onboarding_1.submitMerchantApplication)();
                logger_1.logger.info('[MerchantRegister] 提交结果', result);
                // 4. 检查审核结果
                if (result.status === 'approved') {
                    draft_storage_1.DraftStorage.clear(DRAFT_KEY);
                    // 更新用户角色为商户（后端已授予 MERCHANT 角色）
                    const app = getApp();
                    app.globalData.userRole = 'merchant';
                    // 商户ID将在商户后台页面加载时从API获取
                    wx.showToast({ title: '审核通过', icon: 'success' });
                    setTimeout(() => {
                        wx.reLaunch({ url: '/pages/merchant/dashboard/index' });
                    }, 1000);
                    return; // 立即返回，不重置 isSubmitting
                }
                else if (result.status === 'rejected') {
                    this.setData({
                        currentStep: 4,
                        isSubmitting: false,
                        'formData.rejectReason': result.reject_reason || ''
                    });
                    wx.showModal({
                        title: '审核未通过',
                        content: result.reject_reason || '请检查提交信息',
                        showCancel: false
                    });
                }
                else {
                    // submitted - 开始轮询
                    this.startPollingStatus();
                }
            }
            catch (err) {
                logger_1.logger.error('[MerchantRegister] Submit failed', err);
                const errMsg = (err === null || err === void 0 ? void 0 : err.userMessage) || (err === null || err === void 0 ? void 0 : err.message) || '提交失败，请重试';
                wx.showToast({ title: errMsg, icon: 'none', duration: 3000 });
                this.setData({ isSubmitting: false });
            }
        });
    },
    startPollingStatus() {
        let attempts = 0;
        const maxAttempts = 20;
        const intervalId = setInterval(() => __awaiter(this, void 0, void 0, function* () {
            attempts++;
            try {
                const res = yield (0, onboarding_1.getMyApplication)();
                if (res.status === 'approved') {
                    clearInterval(intervalId);
                    draft_storage_1.DraftStorage.clear(DRAFT_KEY);
                    // 更新用户角色为商户
                    const app = getApp();
                    app.globalData.userRole = 'merchant';
                    wx.showToast({ title: '审核通过', icon: 'success' });
                    setTimeout(() => {
                        wx.reLaunch({ url: '/pages/merchant/dashboard/index' });
                    }, 1500);
                }
                else if (res.status === 'rejected') {
                    clearInterval(intervalId);
                    this.setData({
                        currentStep: 4,
                        'formData.rejectReason': res.reject_reason || ''
                    });
                    wx.showModal({
                        title: '审核未通过',
                        content: res.reject_reason || '请检查提交信息',
                        showCancel: false
                    });
                }
            }
            catch (e) {
                console.error('Polling error', e);
            }
            if (attempts >= maxAttempts) {
                clearInterval(intervalId);
                wx.showToast({ title: '提交成功，请稍后查看审核结果', icon: 'none' });
                setTimeout(() => {
                    wx.reLaunch({ url: '/pages/merchant/dashboard/index' });
                }, 1500);
            }
        }), 2000);
    },
    // 重置被拒绝的申请
    onResetApplication() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                wx.showLoading({ title: '重置中...' });
                yield (0, onboarding_1.resetMerchantApplication)();
                wx.hideLoading();
                wx.showToast({ title: '已重置，可重新编辑', icon: 'success' });
                this.setData({ currentStep: 1 });
                this.initApplication();
            }
            catch (e) {
                wx.hideLoading();
                logger_1.logger.error('[MerchantRegister] Reset failed', e);
                error_handler_1.ErrorHandler.handle(e);
            }
        });
    }
});
