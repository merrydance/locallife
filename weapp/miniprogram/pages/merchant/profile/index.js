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
const merchant_1 = require("../../../api/merchant");
const responsive_1 = require("@/utils/responsive");
Page({
    data: {
        formData: {
            name: '',
            description: '',
            phone: '',
            address: '',
            logo_url: '',
            images: [], // Not in API, simplified to just logo for now or check upload
            start_time: '09:00', // Separate API for hours usually
            end_time: '22:00'
        },
        isLargeScreen: false,
        version: 0,
        fileList: []
    },
    onLoad() {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
        this.loadProfile();
    },
    loadProfile() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const data = yield merchant_1.MerchantManagementService.getMerchantInfo();
                const hours = yield merchant_1.MerchantManagementService.getBusinessHours();
                // Simplified hours logic: take first day or default
                let start = '09:00';
                let end = '22:00';
                if (hours.hours && hours.hours.length > 0) {
                    start = hours.hours[0].open_time;
                    end = hours.hours[0].close_time;
                }
                this.setData({
                    formData: {
                        name: data.name,
                        description: data.description,
                        phone: data.phone,
                        address: data.address,
                        logo_url: data.logo_url || '',
                        images: data.logo_url ? [data.logo_url] : [],
                        start_time: start,
                        end_time: end
                    },
                    version: data.version,
                    fileList: data.logo_url ? [{ url: data.logo_url, status: 'done' }] : []
                });
            }
            catch (error) {
                console.error(error);
                wx.showToast({ title: '加载失败', icon: 'none' });
            }
        });
    },
    onInput(e) {
        const field = e.currentTarget.dataset.field;
        this.setData({
            [`formData.${field}`]: e.detail.value
        });
    },
    onTimeChange(e) {
        const field = e.currentTarget.dataset.field;
        this.setData({
            [`formData.${field}`]: e.detail.value
        });
    },
    onAddFile(e) {
        const { fileList } = this.data;
        const { files } = e.detail;
        const newFiles = files.map((file) => (Object.assign(Object.assign({}, file), { url: file.url, status: 'done' })));
        this.setData({ fileList: [...fileList, ...newFiles] });
    },
    onRemoveFile(e) {
        const { index } = e.detail;
        const { fileList } = this.data;
        fileList.splice(index, 1);
        this.setData({ fileList });
    },
    onSave() {
        return __awaiter(this, void 0, void 0, function* () {
            const { formData, fileList, version } = this.data;
            const logo_url = fileList.length > 0 ? fileList[0].url : '';
            try {
                wx.showLoading({ title: '保存中' });
                // Update Basic Info
                const updateData = {
                    name: formData.name,
                    description: formData.description,
                    phone: formData.phone,
                    address: formData.address,
                    logo_url: logo_url,
                    version: version
                };
                yield merchant_1.MerchantManagementService.updateMerchantInfo(updateData);
                // Update Hours (Simplified: set same hours for all days)
                const hours = [];
                for (let i = 0; i < 7; i++) {
                    hours.push({
                        day_of_week: i,
                        open_time: formData.start_time,
                        close_time: formData.end_time,
                        is_closed: false
                    });
                }
                yield merchant_1.MerchantManagementService.setBusinessHours({ hours });
                wx.showToast({ title: '保存成功', icon: 'success' });
                this.loadProfile(); // Reload to get new version
            }
            catch (error) {
                wx.showToast({ title: error.message || '保存失败', icon: 'none' });
            }
            finally {
                wx.hideLoading();
            }
        });
    }
});
