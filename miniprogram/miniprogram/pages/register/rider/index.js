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
const ocr_1 = require("../../../api/ocr");
const logger_1 = require("../../../utils/logger");
const draft_storage_1 = require("../../../utils/draft-storage");
const onboarding_1 = require("../../../api/onboarding");
const DRAFT_KEY = 'rider_register_draft';
Page({
    data: {
        navBarHeight: 88,
        formData: {
            // 基本信息
            name: '',
            phone: '',
            idCard: '',
            address: '',
            addressDetail: '',
            latitude: 0,
            longitude: 0,
            vehicle: '',
            availableTime: '',
            // 身份信息
            gender: '',
            hometown: '',
            currentAddress: '',
            idCardValidity: ''
        },
        // 图片
        idCardFrontImages: [],
        idCardBackImages: [],
        healthCertImages: [],
        // 选择器状态
        vehiclePickerVisible: false,
        vehiclePickerValue: [],
        vehicleOptions: [
            { label: '电动车', value: 'electric_bike' },
            { label: '摩托车', value: 'motorcycle' },
            { label: '自行车', value: 'bicycle' },
            { label: '汽车', value: 'car' },
            { label: '步行', value: 'walk' }
        ],
        timePickerVisible: false,
        timePickerValue: [],
        timeOptions: [
            { label: '全天', value: 'all_day' },
            { label: '仅白天', value: 'day_only' },
            { label: '仅晚上', value: 'night_only' },
            { label: '周末', value: 'weekend' },
            { label: '工作日', value: 'workday' }
        ]
    },
    onLoad() {
        this.loadDraft();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    // ==================== 草稿管理 ====================
    saveDraft() {
        const data = {
            formData: this.data.formData,
            idCardFrontImages: this.data.idCardFrontImages,
            idCardBackImages: this.data.idCardBackImages,
            healthCertImages: this.data.healthCertImages
        };
        draft_storage_1.DraftStorage.save(DRAFT_KEY, data);
    },
    loadDraft() {
        const draft = draft_storage_1.DraftStorage.load(DRAFT_KEY);
        if (draft) {
            this.setData(draft);
        }
    },
    // ==================== 表单输入 ====================
    updateFormData(key, value) {
        this.setData({ [`formData.${key}`]: value });
        this.saveDraft();
    },
    onNameInput(e) { this.updateFormData('name', e.detail.value); },
    onPhoneInput(e) { this.updateFormData('phone', e.detail.value); },
    onIdCardInput(e) { this.updateFormData('idCard', e.detail.value); },
    onAddressDetailInput(e) { this.updateFormData('addressDetail', e.detail.value); },
    // 身份信息输入
    onGenderInput(e) { this.updateFormData('gender', e.detail.value); },
    onHometownInput(e) { this.updateFormData('hometown', e.detail.value); },
    onCurrentAddressInput(e) { this.updateFormData('currentAddress', e.detail.value); },
    onIdCardValidityInput(e) { this.updateFormData('idCardValidity', e.detail.value); },
    // ==================== 地址选择 ====================
    onChooseAddress() {
        wx.chooseLocation({
            success: (res) => {
                this.setData({
                    'formData.address': res.address || res.name,
                    'formData.latitude': res.latitude,
                    'formData.longitude': res.longitude
                });
                this.saveDraft();
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
    onChooseVehicle() { this.setData({ vehiclePickerVisible: true }); },
    onVehicleConfirm(e) {
        const { value } = e.detail;
        const selectedOption = this.data.vehicleOptions.find((opt) => opt.value === value[0]);
        if (selectedOption) {
            this.updateFormData('vehicle', selectedOption.label);
            this.setData({ vehiclePickerVisible: false });
        }
    },
    onVehicleCancel() { this.setData({ vehiclePickerVisible: false }); },
    onChooseTime() { this.setData({ timePickerVisible: true }); },
    onTimeConfirm(e) {
        const { value } = e.detail;
        const selectedOption = this.data.timeOptions.find((opt) => opt.value === value[0]);
        if (selectedOption) {
            this.updateFormData('availableTime', selectedOption.label);
            this.setData({ timePickerVisible: false });
        }
    },
    onTimeCancel() { this.setData({ timePickerVisible: false }); },
    // ==================== 图片上传与OCR ====================
    // 身份证正面
    onIdCardFrontUpload(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { files } = e.detail;
            this.setData({ idCardFrontImages: files });
            this.saveDraft();
            if (files.length > 0) {
                wx.showLoading({ title: '识别中...' });
                try {
                    const res = yield (0, ocr_1.ocrRiderIdCard)(files[0].url, 'front');
                    const info = res.ocrData;
                    this.setData({
                        'formData.name': info.name || '',
                        'formData.idCard': info.id || info.id_num || '',
                        'formData.gender': info.gender || '',
                        'formData.hometown': info.addr || info.address || ''
                    });
                    this.saveDraft();
                    wx.showToast({ title: '识别成功', icon: 'success' });
                }
                catch (error) {
                    logger_1.logger.error('OCR failed', error, 'Rider');
                    wx.showToast({ title: '识别失败', icon: 'none' });
                }
                finally {
                    wx.hideLoading();
                }
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
            const { files } = e.detail;
            this.setData({ idCardBackImages: files });
            this.saveDraft();
            if (files.length > 0) {
                wx.showLoading({ title: '识别中...' });
                try {
                    const res = yield (0, ocr_1.ocrRiderIdCard)(files[0].url, 'back');
                    const info = res.ocrData;
                    this.setData({
                        'formData.idCardValidity': info.valid_date || info.valid_period || ''
                    });
                    this.saveDraft();
                    wx.showToast({ title: '识别成功', icon: 'success' });
                }
                catch (error) {
                    logger_1.logger.error('OCR failed', error, 'Rider');
                    wx.showToast({ title: '识别失败', icon: 'none' });
                }
                finally {
                    wx.hideLoading();
                }
            }
        });
    },
    onIdCardBackRemove() {
        this.setData({ idCardBackImages: [] });
        this.saveDraft();
    },
    // 健康证
    onHealthCertUpload(e) {
        const { files } = e.detail;
        this.setData({ healthCertImages: files });
        this.saveDraft();
    },
    onHealthCertRemove() {
        this.setData({ healthCertImages: [] });
        this.saveDraft();
    },
    // ==================== 提交申请 ====================
    onSubmit() {
        return __awaiter(this, void 0, void 0, function* () {
            const { formData, idCardFrontImages, idCardBackImages, healthCertImages } = this.data;
            // 验证必填字段
            if (!idCardFrontImages.length)
                return wx.showToast({ title: '请上传身份证正面', icon: 'none' });
            if (!idCardBackImages.length)
                return wx.showToast({ title: '请上传身份证反面', icon: 'none' });
            if (!healthCertImages.length)
                return wx.showToast({ title: '请上传健康证', icon: 'none' });
            if (!formData.name)
                return wx.showToast({ title: '请输入真实姓名', icon: 'none' });
            if (!formData.phone)
                return wx.showToast({ title: '请输入联系电话', icon: 'none' });
            if (!formData.idCard)
                return wx.showToast({ title: '请输入身份证号', icon: 'none' });
            if (!formData.address)
                return wx.showToast({ title: '请选择常驻地址', icon: 'none' });
            if (!formData.gender)
                return wx.showToast({ title: '缺少身份信息', icon: 'none' });
            // 构建提交数据
            const submitData = {
                name: formData.name,
                phone: formData.phone,
                id_card: formData.idCard,
                vehicle_type: formData.vehicle,
                id_card_front_images: idCardFrontImages.map((img) => img.url),
                id_card_back_images: idCardBackImages.map((img) => img.url),
                health_certificate_images: healthCertImages.map((img) => img.url)
            };
            wx.showLoading({ title: '提交中...' });
            try {
                yield (0, onboarding_1.submitRiderApplication)(submitData);
                wx.showToast({
                    title: '申请提交成功',
                    icon: 'success',
                    duration: 2000,
                    success: () => {
                        draft_storage_1.DraftStorage.clear(DRAFT_KEY);
                        setTimeout(() => {
                            wx.navigateBack();
                        }, 2000);
                    }
                });
            }
            catch (error) {
                logger_1.logger.error('Apply rider failed:', error, 'Rider');
                wx.showToast({ title: '提交失败', icon: 'error' });
            }
            finally {
                wx.hideLoading();
            }
        });
    }
});
