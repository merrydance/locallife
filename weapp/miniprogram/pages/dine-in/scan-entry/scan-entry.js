"use strict";
/**
 * 扫码点餐入口页面
 * 处理微信扫一扫跳转到小程序的场景
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
const table_1 = require("../../../api/table");
const dining_session_1 = require("../../../api/dining-session");
Page({
    data: {
        loading: true,
        tableInfo: null,
        error: null,
        merchantInfo: null,
        showTransferDialog: false,
        transferCode: '',
        transferSubmitting: false,
        activeSession: null
    },
    onLoad(options) {
        console.log('扫码点餐页面加载，参数:', options);
        // 从扫码参数中获取桌台信息
        const { scene, q } = options;
        if (scene) {
            // 通过scene参数获取桌台ID（小程序码场景）
            this.handleSceneParam(scene);
        }
        else if (q) {
            // 通过q参数获取完整URL（二维码场景）
            this.handleQRCodeParam(decodeURIComponent(q));
        }
        else {
            // 直接传入table_id参数（测试场景）
            const tableId = options.table_id;
            if (tableId) {
                this.loadTableInfo(parseInt(tableId));
            }
            else {
                this.setData({
                    loading: false,
                    error: '无效的扫码参数'
                });
            }
        }
    },
    /**
     * 处理小程序码scene参数
     */
    handleSceneParam(scene) {
        try {
            const decoded = decodeURIComponent(scene);
            const mMatch = decoded.match(/m_(\d+)/);
            const tMatch = decoded.match(/t_([^-]+)/);
            const tidMatch = decoded.match(/tid_(\d+)/);
            if (mMatch && tMatch) {
                const merchantId = parseInt(mMatch[1]);
                const tableNo = tMatch[1];
                this.loadTableInfoByNo(merchantId, tableNo);
                return;
            }
            if (tidMatch) {
                this.loadTableInfo(parseInt(tidMatch[1]));
                return;
            }
            // 兼容旧格式: table_123 或 t123
            const tableId = decoded.replace(/^(table_|t)/, '');
            if (tableId && !isNaN(parseInt(tableId))) {
                this.loadTableInfo(parseInt(tableId));
                return;
            }
            throw new Error('无效的桌台ID');
        }
        catch (error) {
            console.error('解析scene参数失败:', error);
            this.setData({
                loading: false,
                error: '无效的扫码信息'
            });
        }
    },
    /**
     * 处理二维码URL参数
     */
    handleQRCodeParam(url) {
        var _a;
        try {
            // 解析URL获取桌台ID
            const urlObj = new URL(url);
            const tableId = urlObj.searchParams.get('table_id') ||
                ((_a = urlObj.pathname.match(/\/table\/(\d+)/)) === null || _a === void 0 ? void 0 : _a[1]);
            if (tableId && !isNaN(parseInt(tableId))) {
                this.loadTableInfo(parseInt(tableId));
            }
            else {
                throw new Error('URL中未找到桌台ID');
            }
        }
        catch (error) {
            console.error('解析二维码URL失败:', error);
            this.setData({
                loading: false,
                error: '无效的二维码信息'
            });
        }
    },
    /**
     * 加载桌台信息
     */
    loadTableInfo(tableId) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                this.setData({ loading: true, error: null });
                const detail = yield (0, table_1.getTableDetail)(tableId);
                const scanResult = yield (0, table_1.scanTable)(detail.merchant_id, detail.table_no);
                this.setData({
                    loading: false,
                    tableInfo: {
                        id: detail.id,
                        table_no: detail.table_no,
                        merchant_id: detail.merchant_id,
                        capacity: detail.capacity,
                        status: detail.status
                    },
                    merchantInfo: scanResult.merchant
                });
                this.trackScanBehavior(tableId, detail.merchant_id);
                this.checkActiveSessionAndPrompt();
            }
            catch (error) {
                console.error('加载桌台信息失败:', error);
                this.setData({
                    loading: false,
                    error: error.message || '获取桌台信息失败，请重试'
                });
            }
        });
    },
    loadTableInfoByNo(merchantId, tableNo) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                this.setData({ loading: true, error: null });
                const scanResult = yield (0, table_1.scanTable)(merchantId, tableNo);
                const table = scanResult.table;
                this.setData({
                    loading: false,
                    tableInfo: {
                        id: table.id,
                        table_no: table.table_no,
                        merchant_id: merchantId,
                        capacity: table.capacity,
                        status: table.status
                    },
                    merchantInfo: scanResult.merchant
                });
                this.trackScanBehavior(table.id, merchantId);
                this.checkActiveSessionAndPrompt();
            }
            catch (error) {
                console.error('加载桌台信息失败:', error);
                this.setData({
                    loading: false,
                    error: error.message || '获取桌台信息失败，请重试'
                });
            }
        });
    },
    checkActiveSessionAndPrompt() {
        const { tableInfo } = this.data;
        if (!tableInfo)
            return;
        let activeSession = null;
        try {
            activeSession = wx.getStorageSync('activeDiningSession');
        }
        catch (error) {
            console.warn('读取用餐会话缓存失败:', error);
        }
        if (!activeSession || activeSession.status !== 'open')
            return;
        if (activeSession.merchant_id !== tableInfo.merchant_id)
            return;
        if (activeSession.table_id === tableInfo.id)
            return;
        this.setData({
            showTransferDialog: true,
            transferCode: '',
            activeSession
        });
    },
    onTransferCodeInput(e) {
        this.setData({ transferCode: e.detail.value });
    },
    confirmTransfer() {
        return __awaiter(this, void 0, void 0, function* () {
            const { tableInfo, activeSession, transferCode, transferSubmitting } = this.data;
            if (!tableInfo || !activeSession || transferSubmitting)
                return;
            if (!activeSession.reservation_id && (!transferCode || transferCode.trim() === '')) {
                wx.showToast({ title: '请输入桌台验证码', icon: 'error' });
                return;
            }
            this.setData({ transferSubmitting: true });
            try {
                yield (0, dining_session_1.transferDiningSessionTable)(activeSession.id, {
                    to_table_id: tableInfo.id,
                    table_code: activeSession.reservation_id ? undefined : transferCode.trim(),
                    reason: '扫码换桌'
                });
                try {
                    wx.setStorageSync('activeDiningSession', Object.assign(Object.assign({}, activeSession), { table_id: tableInfo.id, updated_at: new Date().toISOString() }));
                }
                catch (error) {
                    console.warn('更新用餐会话缓存失败:', error);
                }
                wx.showToast({ title: '换桌成功', icon: 'success' });
                this.setData({ showTransferDialog: false, transferSubmitting: false, transferCode: '' });
                this.startDining();
            }
            catch (error) {
                console.error('转台失败:', error);
                wx.showToast({ title: error.message || '换桌失败', icon: 'error' });
                this.setData({ transferSubmitting: false });
            }
        });
    },
    cancelTransfer() {
        this.setData({ showTransferDialog: false, transferCode: '' });
    },
    /**
     * 记录扫码行为
     */
    trackScanBehavior(tableId, merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                // 记录用户行为用于数据分析
                wx.reportAnalytics('scan_table_qr', {
                    table_id: tableId,
                    merchant_id: merchantId,
                    scan_time: new Date().toISOString()
                });
            }
            catch (error) {
                console.warn('记录扫码行为失败:', error);
            }
        });
    },
    /**
     * 开始点餐
     */
    startDining() {
        const { tableInfo } = this.data;
        if (!tableInfo)
            return;
        // 跳转到堂食点餐页面
        wx.navigateTo({
            url: `/pages/dine-in/menu/menu?table_id=${tableInfo.id}&merchant_id=${tableInfo.merchant_id}`
        });
    },
    /**
     * 重新扫码
     */
    rescan() {
        wx.navigateBack();
    },
    /**
     * 查看商户信息
     */
    viewMerchantInfo() {
        const { merchantInfo } = this.data;
        if (!merchantInfo)
            return;
        wx.navigateTo({
            url: `/pages/merchant/detail/detail?id=${merchantInfo.id}`
        });
    },
    /**
     * 分享给好友
     */
    onShareAppMessage() {
        const { tableInfo, merchantInfo } = this.data;
        return {
            title: `${(merchantInfo === null || merchantInfo === void 0 ? void 0 : merchantInfo.name) || '餐厅'}的${(tableInfo === null || tableInfo === void 0 ? void 0 : tableInfo.table_no) || ''}号桌`,
            path: `/pages/dine-in/scan-entry/scan-entry?table_id=${tableInfo === null || tableInfo === void 0 ? void 0 : tableInfo.id}`,
            imageUrl: (merchantInfo === null || merchantInfo === void 0 ? void 0 : merchantInfo.cover_image) || (merchantInfo === null || merchantInfo === void 0 ? void 0 : merchantInfo.logo_url)
        };
    },
    /**
     * 分享到朋友圈
     */
    onShareTimeline() {
        const { tableInfo, merchantInfo } = this.data;
        return {
            title: `在${(merchantInfo === null || merchantInfo === void 0 ? void 0 : merchantInfo.name) || '餐厅'}用餐`,
            query: `table_id=${tableInfo === null || tableInfo === void 0 ? void 0 : tableInfo.id}`,
            imageUrl: (merchantInfo === null || merchantInfo === void 0 ? void 0 : merchantInfo.cover_image) || (merchantInfo === null || merchantInfo === void 0 ? void 0 : merchantInfo.logo_url)
        };
    },
    /**
     * 跳转到外卖页面
     */
    goToTakeout() {
        const { merchantInfo } = this.data;
        wx.switchTab({
            url: '/pages/takeout/index',
            success: () => {
                // 可以通过全局变量或事件传递商户信息
                getApp().globalData.recommendMerchant = merchantInfo;
            }
        });
    },
    /**
     * 跳转到包间预定页面
     */
    goToReservation() {
        const { merchantInfo } = this.data;
        wx.switchTab({
            url: '/pages/reservation/index',
            success: () => {
                // 可以通过全局变量或事件传递商户信息
                getApp().globalData.recommendMerchant = merchantInfo;
            }
        });
    }
});
