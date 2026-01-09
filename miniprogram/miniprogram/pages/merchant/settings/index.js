"use strict";
/**
 * 设置中心 - 桌面级 SaaS 实现
 * 对齐后端 API：
 * - GET/PATCH /v1/merchants/me - 商户信息
 * - CRUD /v1/merchant/devices - 打印机
 * - GET/PUT /v1/merchant/display-config - 显示配置
 */
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
const request_1 = require("../../../utils/request");
const auth_1 = require("../../../utils/auth");
const image_security_1 = require("../../../utils/image-security");
const logger_1 = require("../../../utils/logger");
const app = getApp();
Page({
    data: {
        // SaaS 布局
        sidebarCollapsed: false,
        merchantName: '',
        isOpen: true,
        // 导航
        activeTab: 'profile',
        // 商户信息
        merchant: {},
        originalMerchant: {},
        saving: false,
        descriptionLength: 0,
        // 打印机
        printers: [],
        showPrinterModal: false,
        editingPrinter: {
            printer_name: '',
            printer_sn: '',
            printer_key: '',
            printer_type: 'feieyun',
            print_takeout: true,
            print_dine_in: true,
            print_reservation: true
        },
        savingPrinter: false,
        // 显示配置
        displayConfig: {
            enable_print: true,
            print_takeout: true,
            print_dine_in: true,
            print_reservation: true,
            enable_voice: false,
            voice_takeout: true,
            voice_dine_in: true,
            enable_kds: false,
            kds_url: ''
        },
        savingConfig: false
    },
    onLoad() {
        this.loadMerchantInfo();
        this.loadPrinters();
        this.loadDisplayConfig();
    },
    // 切换标签
    switchTab(e) {
        const tab = e.currentTarget.dataset.tab;
        this.setData({ activeTab: tab });
    },
    // ========== 商户信息 ==========
    loadMerchantInfo() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const res = yield (0, request_1.request)({
                    url: '/v1/merchants/me',
                    method: 'GET'
                });
                // 处理 logo_url 确保能正确显示
                if (res.logo_url) {
                    res.logo_url = yield (0, image_security_1.resolveImageURL)(res.logo_url);
                }
                this.setData({
                    merchant: res,
                    originalMerchant: Object.assign({}, res),
                    merchantName: res.name,
                    isOpen: res.is_open,
                    descriptionLength: (res.description || '').length
                });
            }
            catch (error) {
                logger_1.logger.error('加载商户信息失败', error, 'Settings');
            }
        });
    },
    onInput(e) {
        const field = e.currentTarget.dataset.field;
        const value = e.detail.value;
        const updates = {
            [`merchant.${field}`]: value
        };
        if (field === 'description') {
            updates.descriptionLength = value.length;
        }
        this.setData(updates);
    },
    uploadLogo() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const res = yield wx.chooseMedia({
                    count: 1,
                    mediaType: ['image'],
                    sourceType: ['album', 'camera']
                });
                const tempFilePath = res.tempFiles[0].tempFilePath;
                wx.showLoading({ title: '上传中...' });
                // 上传图片到服务器
                const uploadRes = yield new Promise((resolve, reject) => {
                    wx.uploadFile({
                        url: `${request_1.API_BASE}/v1/merchants/images/upload`,
                        filePath: tempFilePath,
                        name: 'image',
                        formData: {
                            category: 'logo'
                        },
                        header: {
                            Authorization: `Bearer ${(0, auth_1.getToken)()}`
                        },
                        success: (res) => {
                            try {
                                const data = JSON.parse(res.data);
                                if (data.image_url) {
                                    resolve(data.image_url);
                                }
                                else if (data.url) {
                                    resolve(data.url);
                                }
                                else {
                                    reject(new Error(data.error || '上传失败'));
                                }
                            }
                            catch (e) {
                                reject(new Error('解析响应失败'));
                            }
                        },
                        fail: reject
                    });
                });
                // 使用项目标准方法处理图片URL（公共路径直接拼接，私有路径会签名）
                const logoUrl = yield (0, image_security_1.resolveImageURL)(uploadRes);
                this.setData({ 'merchant.logo_url': logoUrl });
                wx.hideLoading();
                wx.showToast({ title: '上传成功', icon: 'success' });
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('上传 Logo 失败', error, 'Settings');
                wx.showToast({ title: '上传失败', icon: 'error' });
            }
        });
    },
    chooseLocation() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const res = yield wx.chooseLocation({});
                this.setData({
                    'merchant.address': res.address,
                    'merchant.latitude': res.latitude.toString(),
                    'merchant.longitude': res.longitude.toString()
                });
            }
            catch (error) {
                logger_1.logger.warn('选择位置取消', error, 'Settings');
            }
        });
    },
    resetProfile() {
        this.setData({
            merchant: Object.assign({}, this.data.originalMerchant)
        });
        wx.showToast({ title: '已重置', icon: 'success' });
    },
    saveProfile() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a, _b;
            const { merchant } = this.data;
            // 验证
            if (!merchant.name || merchant.name.length < 2) {
                wx.showToast({ title: '店铺名称至少2个字符', icon: 'none' });
                return;
            }
            if (!merchant.phone || merchant.phone.length !== 11) {
                wx.showToast({ title: '请输入11位手机号', icon: 'none' });
                return;
            }
            if (!merchant.address || merchant.address.length < 5) {
                wx.showToast({ title: '地址至少5个字符', icon: 'none' });
                return;
            }
            this.setData({ saving: true });
            try {
                const res = yield (0, request_1.request)({
                    url: '/v1/merchants/me',
                    method: 'PATCH',
                    data: {
                        name: merchant.name,
                        description: merchant.description,
                        logo_url: merchant.logo_url,
                        phone: merchant.phone,
                        address: merchant.address,
                        latitude: merchant.latitude,
                        longitude: merchant.longitude,
                        version: merchant.version || 1
                    }
                });
                // 处理 logo_url 确保能正确显示
                if (res.logo_url) {
                    res.logo_url = yield (0, image_security_1.resolveImageURL)(res.logo_url);
                }
                this.setData({
                    merchant: res,
                    originalMerchant: Object.assign({}, res),
                    merchantName: res.name
                });
                wx.showToast({ title: '保存成功', icon: 'success' });
            }
            catch (error) {
                logger_1.logger.error('保存商户信息失败', error, 'Settings');
                if (((_a = error.message) === null || _a === void 0 ? void 0 : _a.includes('version')) || ((_b = error.message) === null || _b === void 0 ? void 0 : _b.includes('conflict'))) {
                    wx.showToast({ title: '数据已被修改，请刷新', icon: 'none' });
                    this.loadMerchantInfo();
                }
                else {
                    wx.showToast({ title: '保存失败', icon: 'error' });
                }
            }
            finally {
                this.setData({ saving: false });
            }
        });
    },
    // ========== 打印机管理 ==========
    loadPrinters() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const res = yield (0, request_1.request)({
                    url: '/v1/merchant/devices',
                    method: 'GET'
                });
                this.setData({ printers: res || [] });
            }
            catch (error) {
                logger_1.logger.error('加载打印机列表失败', error, 'Settings');
            }
        });
    },
    addPrinter() {
        this.setData({
            showPrinterModal: true,
            editingPrinter: {
                printer_name: '',
                printer_sn: '',
                printer_key: '',
                printer_type: 'feieyun',
                print_takeout: true,
                print_dine_in: true,
                print_reservation: true,
                is_active: true
            }
        });
    },
    editPrinter(e) {
        const id = e.currentTarget.dataset.id;
        const printer = this.data.printers.find(p => p.id === id);
        if (printer) {
            this.setData({
                showPrinterModal: true,
                editingPrinter: Object.assign({}, printer)
            });
        }
    },
    closePrinterModal() {
        this.setData({ showPrinterModal: false });
    },
    onPrinterInput(e) {
        const field = e.currentTarget.dataset.field;
        this.setData({
            [`editingPrinter.${field}`]: e.detail.value
        });
    },
    selectPrinterType(e) {
        const type = e.currentTarget.dataset.type;
        this.setData({ 'editingPrinter.printer_type': type });
    },
    togglePrinterScene(e) {
        const field = e.currentTarget.dataset.field;
        const current = this.data.editingPrinter[field];
        this.setData({
            [`editingPrinter.${field}`]: !current
        });
    },
    savePrinter() {
        return __awaiter(this, void 0, void 0, function* () {
            const { editingPrinter } = this.data;
            // 验证
            if (!editingPrinter.printer_name) {
                wx.showToast({ title: '请输入打印机名称', icon: 'none' });
                return;
            }
            if (!editingPrinter.id) {
                // 新增时需要验证更多字段
                if (!editingPrinter.printer_sn) {
                    wx.showToast({ title: '请输入打印机序列号', icon: 'none' });
                    return;
                }
                if (!editingPrinter.printer_key) {
                    wx.showToast({ title: '请输入打印机密钥', icon: 'none' });
                    return;
                }
            }
            this.setData({ savingPrinter: true });
            try {
                if (editingPrinter.id) {
                    // 更新
                    yield (0, request_1.request)({
                        url: `/v1/merchant/devices/${editingPrinter.id}`,
                        method: 'PATCH',
                        data: {
                            printer_name: editingPrinter.printer_name,
                            print_takeout: editingPrinter.print_takeout,
                            print_dine_in: editingPrinter.print_dine_in,
                            print_reservation: editingPrinter.print_reservation
                        }
                    });
                }
                else {
                    // 新增
                    yield (0, request_1.request)({
                        url: '/v1/merchant/devices',
                        method: 'POST',
                        data: {
                            printer_name: editingPrinter.printer_name,
                            printer_sn: editingPrinter.printer_sn,
                            printer_key: editingPrinter.printer_key,
                            printer_type: editingPrinter.printer_type,
                            print_takeout: editingPrinter.print_takeout,
                            print_dine_in: editingPrinter.print_dine_in,
                            print_reservation: editingPrinter.print_reservation
                        }
                    });
                }
                wx.showToast({ title: '保存成功', icon: 'success' });
                this.closePrinterModal();
                this.loadPrinters();
            }
            catch (error) {
                logger_1.logger.error('保存打印机失败', error, 'Settings');
                wx.showToast({ title: '保存失败', icon: 'error' });
            }
            finally {
                this.setData({ savingPrinter: false });
            }
        });
    },
    togglePrinter(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const id = e.currentTarget.dataset.id;
            const printer = this.data.printers.find(p => p.id === id);
            if (!printer)
                return;
            try {
                yield (0, request_1.request)({
                    url: `/v1/merchant/devices/${id}`,
                    method: 'PATCH',
                    data: {
                        is_active: !printer.is_active
                    }
                });
                this.loadPrinters();
                wx.showToast({ title: printer.is_active ? '已禁用' : '已启用', icon: 'success' });
            }
            catch (error) {
                logger_1.logger.error('切换打印机状态失败', error, 'Settings');
                wx.showToast({ title: '操作失败', icon: 'error' });
            }
        });
    },
    deletePrinter(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const id = e.currentTarget.dataset.id;
            wx.showModal({
                title: '确认删除',
                content: '删除后无法恢复，确定要删除这台打印机吗？',
                success: (res) => __awaiter(this, void 0, void 0, function* () {
                    if (res.confirm) {
                        try {
                            yield (0, request_1.request)({
                                url: `/v1/merchant/devices/${id}`,
                                method: 'DELETE'
                            });
                            this.loadPrinters();
                            wx.showToast({ title: '已删除', icon: 'success' });
                        }
                        catch (error) {
                            logger_1.logger.error('删除打印机失败', error, 'Settings');
                            wx.showToast({ title: '删除失败', icon: 'error' });
                        }
                    }
                })
            });
        });
    },
    // ========== 显示配置 ==========
    loadDisplayConfig() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const res = yield (0, request_1.request)({
                    url: '/v1/merchant/display-config',
                    method: 'GET'
                });
                this.setData({ displayConfig: res });
            }
            catch (error) {
                logger_1.logger.error('加载显示配置失败', error, 'Settings');
            }
        });
    },
    toggleConfig(e) {
        const field = e.currentTarget.dataset.field;
        const current = this.data.displayConfig[field];
        this.setData({
            [`displayConfig.${field}`]: !current
        });
    },
    onConfigInput(e) {
        const field = e.currentTarget.dataset.field;
        this.setData({
            [`displayConfig.${field}`]: e.detail.value
        });
    },
    saveDisplayConfig() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ savingConfig: true });
            try {
                const { displayConfig } = this.data;
                yield (0, request_1.request)({
                    url: '/v1/merchant/display-config',
                    method: 'PUT',
                    data: {
                        enable_print: displayConfig.enable_print,
                        print_takeout: displayConfig.print_takeout,
                        print_dine_in: displayConfig.print_dine_in,
                        print_reservation: displayConfig.print_reservation,
                        enable_voice: displayConfig.enable_voice,
                        voice_takeout: displayConfig.voice_takeout,
                        voice_dine_in: displayConfig.voice_dine_in,
                        enable_kds: displayConfig.enable_kds,
                        kds_url: displayConfig.kds_url || null
                    }
                });
                wx.showToast({ title: '保存成功', icon: 'success' });
            }
            catch (error) {
                logger_1.logger.error('保存显示配置失败', error, 'Settings');
                wx.showToast({ title: '保存失败', icon: 'error' });
            }
            finally {
                this.setData({ savingConfig: false });
            }
        });
    },
    // ========== 通用方法 ==========
    onSidebarCollapse(e) {
        this.setData({ sidebarCollapsed: e.detail.collapsed });
    },
    goBack() {
        wx.navigateBack({
            fail: () => {
                wx.redirectTo({ url: '/pages/merchant/dashboard/index' });
            }
        });
    }
});
