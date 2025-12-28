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
/**
 * 桌台管理页面
 * 两栏布局 + 两步向导：
 * 第一步：基本信息 -> 保存创建桌台
 * 第二步：图片上传、标签管理、二维码
 */
const table_device_management_1 = require("../../../api/table-device-management");
const dish_1 = require("../../../api/dish");
const logger_1 = require("../../../utils/logger");
const image_security_1 = require("../../../utils/image-security");
const request_1 = require("../../../utils/request");
const app = getApp();
// 空桌台模板
const emptyTable = () => ({
    table_no: '',
    table_type: 'table',
    capacity: 0,
    description: '',
    minimum_spend: undefined,
    status: 'available'
});
Page({
    data: {
        // 侧边栏状态
        sidebarCollapsed: false,
        merchantName: '',
        isOpen: true,
        // 加载状态
        loading: true,
        saving: false,
        // 桌台数据
        tables: [],
        filteredTables: [],
        // 筛选
        activeType: '',
        // 编辑状态
        selectedTable: null,
        isAdding: false,
        currentStep: 1,
        // 最低消费（元）
        minimumSpendYuan: '',
        // 第二步数据
        tableImages: [],
        qrCodeUrl: '',
        // 标签管理
        availableTableTags: [], // 可用标签列表
        selectedTagIds: [], // 已选标签ID
        showTagManager: false, // 显示标签管理弹窗
        newTagName: '' // 新标签名称
    },
    onLoad() {
        this.initData();
    },
    onShow() {
        if (this.data.tables.length > 0) {
            this.loadTables();
        }
    },
    initData() {
        return __awaiter(this, void 0, void 0, function* () {
            const merchantId = app.globalData.merchantId;
            if (merchantId) {
                this.setData({ merchantName: app.globalData.merchantName || '' });
                yield Promise.all([
                    this.loadTables(),
                    this.loadAvailableTableTags()
                ]);
            }
            else {
                app.userInfoReadyCallback = () => __awaiter(this, void 0, void 0, function* () {
                    if (app.globalData.merchantId) {
                        this.setData({ merchantName: app.globalData.merchantName || '' });
                        yield Promise.all([
                            this.loadTables(),
                            this.loadAvailableTableTags()
                        ]);
                    }
                });
            }
        });
    },
    // ========== 数据加载 ==========
    loadTables() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const response = yield table_device_management_1.tableManagementService.listTables();
                const tables = response.tables || [];
                this.setData({ tables, loading: false });
                this.applyFilter();
            }
            catch (error) {
                logger_1.logger.error('加载桌台列表失败', error, 'Tables');
                this.setData({ loading: false });
                wx.showToast({ title: '加载失败', icon: 'none' });
            }
        });
    },
    // ========== 筛选 ==========
    onTypeFilter(e) {
        const type = e.currentTarget.dataset.type || '';
        this.setData({ activeType: type });
        this.applyFilter();
    },
    applyFilter() {
        const { tables, activeType } = this.data;
        let filtered = [...tables];
        if (activeType) {
            filtered = filtered.filter(t => t.table_type === activeType);
        }
        this.setData({ filteredTables: filtered });
    },
    // ========== 选择/添加 ==========
    onSelectTable(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const item = e.currentTarget.dataset.item;
            const minSpend = item.minimum_spend ? String(item.minimum_spend / 100) : '';
            // 提取已选标签 ID
            const selectedTagIds = (item.tags || []).map((t) => t.id);
            this.setData({
                selectedTable: Object.assign({}, item),
                isAdding: false,
                currentStep: 1,
                minimumSpendYuan: minSpend,
                tableImages: [],
                selectedTagIds,
                qrCodeUrl: ''
            });
            // 加载图片和二维码
            yield this.loadTableExtras(item.id);
        });
    },
    loadTableExtras(tableId) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const [imagesRes, qrRes] = yield Promise.all([
                    table_device_management_1.tableManagementService.getTableImages(tableId).catch(() => ({ images: [] })),
                    table_device_management_1.tableManagementService.getTableQRCode(tableId).catch(() => ({ qr_code_url: '' }))
                ]);
                const images = [];
                for (const img of (imagesRes.images || [])) {
                    const resolvedUrl = yield (0, image_security_1.resolveImageURL)(img.image_url || '');
                    images.push(Object.assign(Object.assign({}, img), { image_url: resolvedUrl }));
                }
                // 解析二维码URL为完整路径
                let qrCodeUrl = '';
                if (qrRes.qr_code_url) {
                    qrCodeUrl = yield (0, image_security_1.resolveImageURL)(qrRes.qr_code_url);
                }
                this.setData({
                    tableImages: images,
                    qrCodeUrl
                });
            }
            catch (error) {
                logger_1.logger.error('加载桌台附加信息失败', error, 'Tables');
            }
        });
    },
    onAddTable() {
        this.setData({
            selectedTable: emptyTable(),
            isAdding: true,
            currentStep: 1,
            minimumSpendYuan: '',
            tableImages: [],
            tableTags: [],
            newTagName: '',
            qrCodeUrl: ''
        });
    },
    onCancelEdit() {
        this.setData({
            selectedTable: null,
            isAdding: false,
            currentStep: 1
        });
    },
    // ========== 表单输入 ==========
    onFieldChange(e) {
        const field = e.currentTarget.dataset.field;
        this.setData({ [`selectedTable.${field}`]: e.detail.value });
    },
    onNumberFieldChange(e) {
        const field = e.currentTarget.dataset.field;
        const value = e.detail.value ? parseInt(e.detail.value) : undefined;
        this.setData({ [`selectedTable.${field}`]: value });
    },
    onMinSpendChange(e) {
        const yuan = e.detail.value;
        this.setData({ minimumSpendYuan: yuan });
        const fen = yuan ? Math.round(parseFloat(yuan) * 100) : undefined;
        this.setData({ 'selectedTable.minimum_spend': fen });
    },
    onSelectType(e) {
        const type = e.currentTarget.dataset.type;
        this.setData({ 'selectedTable.table_type': type });
    },
    onSelectStatus(e) {
        const status = e.currentTarget.dataset.status;
        this.setData({ 'selectedTable.status': status });
    },
    // ========== 两步向导 ==========
    onNextStep() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a, _b;
            const { selectedTable } = this.data;
            if (!selectedTable)
                return;
            if (!((_a = selectedTable.table_no) === null || _a === void 0 ? void 0 : _a.trim())) {
                wx.showToast({ title: '请输入桌号', icon: 'none' });
                return;
            }
            if (!selectedTable.capacity || selectedTable.capacity < 1) {
                wx.showToast({ title: '请输入有效人数', icon: 'none' });
                return;
            }
            this.setData({ saving: true });
            try {
                const createData = {
                    table_no: selectedTable.table_no.trim(),
                    table_type: selectedTable.table_type,
                    capacity: selectedTable.capacity,
                    description: ((_b = selectedTable.description) === null || _b === void 0 ? void 0 : _b.trim()) || undefined,
                    minimum_spend: selectedTable.minimum_spend || undefined
                };
                const newTable = yield table_device_management_1.tableManagementService.createTable(createData);
                this.setData({
                    saving: false,
                    currentStep: 2,
                    selectedTable: newTable
                });
                wx.showToast({ title: '桌台已创建', icon: 'success' });
                this.loadTables();
            }
            catch (error) {
                logger_1.logger.error('创建桌台失败', error, 'Tables');
                this.setData({ saving: false });
                wx.showToast({ title: (error === null || error === void 0 ? void 0 : error.userMessage) || '创建失败', icon: 'none' });
            }
        });
    },
    onFinishAdd() {
        this.setData({
            selectedTable: null,
            isAdding: false,
            currentStep: 1
        });
        this.loadTables();
    },
    // ========== 保存（编辑模式） ==========
    onSaveTable() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a, _b, _c;
            const { selectedTable } = this.data;
            if (!(selectedTable === null || selectedTable === void 0 ? void 0 : selectedTable.id))
                return;
            if (!((_a = selectedTable.table_no) === null || _a === void 0 ? void 0 : _a.trim())) {
                wx.showToast({ title: '请输入桌号', icon: 'none' });
                return;
            }
            if (!selectedTable.capacity || selectedTable.capacity < 1) {
                wx.showToast({ title: '请输入有效人数', icon: 'none' });
                return;
            }
            this.setData({ saving: true });
            try {
                const updateData = {
                    table_no: (_b = selectedTable.table_no) === null || _b === void 0 ? void 0 : _b.trim(),
                    capacity: selectedTable.capacity,
                    description: ((_c = selectedTable.description) === null || _c === void 0 ? void 0 : _c.trim()) || undefined,
                    minimum_spend: selectedTable.minimum_spend || undefined,
                    status: selectedTable.status
                };
                yield table_device_management_1.tableManagementService.updateTable(selectedTable.id, updateData);
                this.setData({ saving: false });
                wx.showToast({ title: '保存成功', icon: 'success' });
                yield this.loadTables();
            }
            catch (error) {
                logger_1.logger.error('保存桌台失败', error, 'Tables');
                this.setData({ saving: false });
                wx.showToast({ title: (error === null || error === void 0 ? void 0 : error.userMessage) || '保存失败', icon: 'none' });
            }
        });
    },
    // ========== 删除 ==========
    onDeleteTable() {
        const { selectedTable } = this.data;
        if (!(selectedTable === null || selectedTable === void 0 ? void 0 : selectedTable.id))
            return;
        const tableNo = selectedTable.table_no || '';
        wx.showModal({
            title: '确认删除',
            content: '确定要删除桌台 ' + tableNo + ' 吗？',
            confirmColor: '#ff4d4f',
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                if (res.confirm) {
                    try {
                        yield table_device_management_1.tableManagementService.deleteTable(selectedTable.id);
                        wx.showToast({ title: '已删除', icon: 'success' });
                        this.setData({ selectedTable: null, isAdding: false });
                        yield this.loadTables();
                    }
                    catch (error) {
                        logger_1.logger.error('删除失败', error, 'Tables');
                        wx.showToast({ title: (error === null || error === void 0 ? void 0 : error.userMessage) || '删除失败', icon: 'none' });
                    }
                }
            })
        });
    },
    // ========== 图片管理 ==========
    onUploadImage() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            const tableId = (_a = this.data.selectedTable) === null || _a === void 0 ? void 0 : _a.id;
            if (!tableId)
                return;
            try {
                const res = yield wx.chooseMedia({
                    count: 1,
                    mediaType: ['image'],
                    sourceType: ['album', 'camera']
                });
                const tempPath = res.tempFiles[0].tempFilePath;
                wx.showLoading({ title: '上传中...' });
                // 上传图片到服务器
                const { getToken } = require('../../../utils/auth');
                const token = getToken();
                const uploadRes = yield new Promise((resolve, reject) => {
                    wx.uploadFile({
                        url: request_1.API_BASE + '/v1/tables/images/upload',
                        filePath: tempPath,
                        name: 'image',
                        header: { 'Authorization': 'Bearer ' + token },
                        success: (uploadResult) => {
                            // 200 OK 或 201 Created 都表示成功
                            if (uploadResult.statusCode === 200 || uploadResult.statusCode === 201) {
                                const data = JSON.parse(uploadResult.data);
                                resolve(data.image_url || data.url || '');
                            }
                            else {
                                reject(new Error('HTTP ' + uploadResult.statusCode));
                            }
                        },
                        fail: (err) => {
                            reject(err);
                        }
                    });
                });
                // 添加到桌台
                yield table_device_management_1.tableManagementService.uploadTableImage(tableId, { image_url: uploadRes });
                wx.hideLoading();
                wx.showToast({ title: '上传成功', icon: 'success' });
                yield this.loadTableExtras(tableId);
            }
            catch (error) {
                wx.hideLoading();
                const errMsg = (error === null || error === void 0 ? void 0 : error.message) || (error === null || error === void 0 ? void 0 : error.errMsg) || String(error);
                logger_1.logger.error('上传图片失败', error, 'Tables');
                wx.showToast({ title: errMsg.substring(0, 15) || '上传失败', icon: 'none' });
            }
        });
    },
    onSetPrimaryImage(e) {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            const imageId = e.currentTarget.dataset.id;
            const tableId = (_a = this.data.selectedTable) === null || _a === void 0 ? void 0 : _a.id;
            if (!tableId || !imageId)
                return;
            try {
                yield table_device_management_1.tableManagementService.setPrimaryTableImage(tableId, imageId);
                wx.showToast({ title: '已设为主图', icon: 'success' });
                yield this.loadTableExtras(tableId);
            }
            catch (error) {
                logger_1.logger.error('设置主图失败', error, 'Tables');
                wx.showToast({ title: '操作失败', icon: 'none' });
            }
        });
    },
    onDeleteImage(e) {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            const imageId = e.currentTarget.dataset.id;
            const tableId = (_a = this.data.selectedTable) === null || _a === void 0 ? void 0 : _a.id;
            if (!tableId || !imageId)
                return;
            wx.showModal({
                title: '确认删除',
                content: '确定要删除这张图片吗？',
                success: (res) => __awaiter(this, void 0, void 0, function* () {
                    if (res.confirm) {
                        try {
                            yield table_device_management_1.tableManagementService.deleteTableImage(tableId, imageId);
                            wx.showToast({ title: '已删除', icon: 'success' });
                            yield this.loadTableExtras(tableId);
                        }
                        catch (error) {
                            logger_1.logger.error('删除图片失败', error, 'Tables');
                            wx.showToast({ title: '删除失败', icon: 'none' });
                        }
                    }
                })
            });
        });
    },
    // ========== 标签管理 ==========
    loadAvailableTableTags() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const tags = yield dish_1.TagService.listTags('table');
                this.setData({ availableTableTags: tags });
            }
            catch (error) {
                logger_1.logger.error('加载标签列表失败', error, 'Tables');
            }
        });
    },
    // 切换标签选中状态
    onTagToggle(e) {
        const tagId = e.currentTarget.dataset.id;
        const { selectedTagIds } = this.data;
        const index = selectedTagIds.indexOf(tagId);
        let newIds;
        if (index === -1) {
            newIds = [...selectedTagIds, tagId];
        }
        else {
            newIds = selectedTagIds.filter(id => id !== tagId);
        }
        this.setData({ selectedTagIds: newIds });
    },
    // 打开标签管理弹窗
    onOpenTagManager() {
        this.setData({ showTagManager: true });
    },
    // 关闭标签管理弹窗
    onCloseTagManager() {
        this.setData({ showTagManager: false, newTagName: '' });
    },
    // 输入新标签名
    onTagNameInput(e) {
        this.setData({ newTagName: e.detail.value });
    },
    // 创建新标签
    onCreateTag() {
        return __awaiter(this, void 0, void 0, function* () {
            const { newTagName } = this.data;
            if (!newTagName.trim()) {
                wx.showToast({ title: '请输入标签名称', icon: 'none' });
                return;
            }
            try {
                const newTag = yield dish_1.TagService.createTag({ name: newTagName.trim(), type: 'table' });
                this.setData({
                    availableTableTags: [...this.data.availableTableTags, newTag],
                    newTagName: ''
                });
                wx.showToast({ title: '标签已创建', icon: 'success' });
            }
            catch (error) {
                logger_1.logger.error('创建标签失败', error, 'Tables');
                wx.showToast({ title: '创建失败', icon: 'none' });
            }
        });
    },
    // 删除标签
    onDeleteTag(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const tagId = e.currentTarget.dataset.id;
            const tagName = e.currentTarget.dataset.name;
            const res = yield new Promise(resolve => {
                wx.showModal({
                    title: '确认删除',
                    content: `确定要删除标签"${tagName}"吗？`,
                    success: resolve
                });
            });
            if (!res.confirm)
                return;
            try {
                yield dish_1.TagService.deleteTag(tagId);
                this.setData({
                    availableTableTags: this.data.availableTableTags.filter(t => t.id !== tagId),
                    selectedTagIds: this.data.selectedTagIds.filter(id => id !== tagId)
                });
                wx.showToast({ title: '标签已删除', icon: 'success' });
            }
            catch (error) {
                logger_1.logger.error('删除标签失败', error, 'Tables');
                wx.showToast({ title: '删除失败', icon: 'none' });
            }
        });
    },
    // ========== 二维码 ==========
    onGenerateQRCode() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            const tableId = (_a = this.data.selectedTable) === null || _a === void 0 ? void 0 : _a.id;
            if (!tableId)
                return;
            try {
                wx.showLoading({ title: '生成中...' });
                const res = yield table_device_management_1.tableManagementService.getTableQRCode(tableId);
                wx.hideLoading();
                this.setData({ qrCodeUrl: res.qr_code_url });
                wx.showToast({ title: '二维码已生成', icon: 'success' });
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('生成二维码失败', error, 'Tables');
                wx.showToast({ title: '生成失败', icon: 'none' });
            }
        });
    },
    onPreviewQRCode() {
        const { qrCodeUrl } = this.data;
        if (!qrCodeUrl)
            return;
        wx.previewImage({
            urls: [qrCodeUrl],
            current: qrCodeUrl
        });
    },
    onDownloadQRCode() {
        const { qrCodeUrl } = this.data;
        if (!qrCodeUrl) {
            wx.showToast({ title: '无二维码', icon: 'none' });
            return;
        }
        wx.showLoading({ title: '下载中...' });
        wx.downloadFile({
            url: qrCodeUrl,
            success: (res) => {
                wx.hideLoading();
                if (res.statusCode === 200 && res.tempFilePath) {
                    // 打开文件让用户选择保存位置
                    wx.openDocument({
                        filePath: res.tempFilePath,
                        showMenu: true, // 显示右上角菜单可保存
                        success: () => { },
                        fail: () => {
                            // openDocument 失败则预览
                            wx.previewImage({
                                urls: [qrCodeUrl],
                                current: qrCodeUrl
                            });
                            wx.showToast({ title: '请右键图片保存', icon: 'none', duration: 2000 });
                        }
                    });
                }
                else {
                    wx.showToast({ title: '下载失败', icon: 'none' });
                }
            },
            fail: () => {
                wx.hideLoading();
                // 下载失败，直接预览
                wx.previewImage({
                    urls: [qrCodeUrl],
                    current: qrCodeUrl
                });
                wx.showToast({ title: '请右键图片保存', icon: 'none', duration: 2000 });
            }
        });
    },
    // ========== 侧边栏 ==========
    onSidebarCollapse(e) {
        this.setData({ sidebarCollapsed: e.detail.collapsed });
    },
    goBack() {
        wx.navigateBack();
    }
});
