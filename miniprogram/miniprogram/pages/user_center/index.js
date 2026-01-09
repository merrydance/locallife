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
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const navigation_1 = __importDefault(require("../../utils/navigation"));
const auth_1 = require("../../api/auth");
const logger_1 = require("../../utils/logger");
const upload_1 = require("../../api/upload");
const app = getApp();
Page({
    data: {
        userInfo: {
            nickName: '微信用户',
            avatarUrl: ''
        },
        userRole: 'guest',
        workbenches: [],
        registrationOptions: [
            { id: 'merchant', name: '餐厅入驻', desc: '开通商家账号', path: '/pages/register/merchant/index' },
            { id: 'rider', name: '骑手入驻', desc: '成为配送骑手', path: '/pages/register/rider/index' },
            { id: 'operator', name: '运营商入驻', desc: '区域运营合作', path: '/pages/register/operator/index' }
        ],
        navBarHeight: 88,
        scrollViewHeight: 600
    },
    onLoad() {
        // 计算导航栏高度和滚动区域高度
        const windowInfo = wx.getWindowInfo();
        const menuButton = wx.getMenuButtonBoundingClientRect();
        const statusBarHeight = windowInfo.statusBarHeight || 0;
        const navBarContentHeight = menuButton.height + (menuButton.top - statusBarHeight) * 2;
        const navBarHeight = statusBarHeight + navBarContentHeight;
        // windowHeight 已扣除原生 tabBar，只需扣除自定义导航栏
        const scrollViewHeight = windowInfo.windowHeight - navBarHeight;
        this.setData({ navBarHeight, scrollViewHeight });
        this.initUserInfo();
    },
    onShow() {
        // Refresh role in case it changed
        if (app.globalData.userInfo) {
            this.updateUser(app.globalData.userInfo, app.globalData.userRole);
        }
        // Always try to fetch fresh data on show to ensure persistence check
        this.refreshUserInfo();
    },
    updateUser(info, roles) {
        const role = (Array.isArray(roles) ? roles[0] : roles); // Primary role for display
        this.setData({
            userInfo: {
                nickName: info.nickName || info.full_name || info.nickname || '微信用户',
                avatarUrl: info.avatarUrl || info.avatar_url || info.avatar || ''
            },
            userRole: role // Keep for compatibility
        });
        // Normalize to array for workbench check
        const roleList = Array.isArray(roles) ? roles : [roles];
        this.loadWorkbenches(roleList);
    },
    initUserInfo() {
        return __awaiter(this, void 0, void 0, function* () {
            if (app.globalData.userInfo) {
                // Use cached role as fallback
                this.updateUser(app.globalData.userInfo, app.globalData.userRole);
            }
            yield this.refreshUserInfo();
        });
    },
    refreshUserInfo() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const user = yield (0, auth_1.getUserInfo)();
                if (user) {
                    logger_1.logger.debug('Refreshed User Info from Backend', user); // Debug log
                    // Recover avatar from local storage if backend returns empty
                    const localAvatar = wx.getStorageSync('user_avatar');
                    const finalAvatar = user.avatar_url || localAvatar || '';
                    console.log('[UserCenter] Refresh Info - Avatar:', finalAvatar, 'User:', user);
                    // Update Global Data
                    app.globalData.userInfo = {
                        nickName: user.full_name || '微信用户',
                        avatarUrl: finalAvatar,
                    };
                    // Update Local Data
                    this.updateUser(app.globalData.userInfo, user.roles || []);
                }
            }
            catch (err) {
                logger_1.logger.error('Failed to refresh user info', err);
            }
        });
    },
    // ==================== 导航方法 ====================
    onMyOrders() {
        navigation_1.default.toOrderList();
    },
    onAddresses() {
        navigation_1.default.toAddressList();
    },
    onPoints() {
        navigation_1.default.toPoints();
    },
    onCoupons() {
        navigation_1.default.toCoupons();
    },
    onFavorites() {
        navigation_1.default.toFavorites();
    },
    onMembership() {
        navigation_1.default.toMembership();
    },
    onMyReviews() {
        navigation_1.default.toMyReviews();
    },
    onMyReservations() {
        wx.navigateTo({ url: '/pages/user_center/reservations/index' });
    },
    onWallet() {
        navigation_1.default.toWallet();
    },
    onCredit() {
        navigation_1.default.toCredit();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadWorkbenches(roles) {
        const workbenches = [];
        if (roles.includes('merchant') || roles.includes('operator')) {
            workbenches.push({
                id: 'merchant',
                name: '商家工作台',
                icon: 'shop',
                path: '/pages/merchant/dashboard/index'
            });
        }
        if (roles.includes('rider') || roles.includes('operator')) {
            workbenches.push({
                id: 'rider',
                name: '骑手工作台',
                icon: 'user-circle', // generic user icon for rider
                path: '/pages/rider/dashboard/index'
            });
        }
        // Admin Entrance
        if (roles.includes('admin')) {
            workbenches.push({
                id: 'admin',
                name: '平台管理',
                desc: '系统管理控制台',
                icon: 'control-platform', // Safe bet or 'desktop'
                path: '/pages/platform/dashboard/dashboard' // Corrected path
            });
        }
        this.setData({ workbenches });
    },
    onWorkbenchTap(e) {
        const { path } = e.currentTarget.dataset;
        if (path) {
            wx.navigateTo({ url: path });
        }
    },
    onRegisterTap(e) {
        const { id } = e.currentTarget.dataset;
        const pathMap = {
            merchant: '/pages/register/merchant/index',
            rider: '/pages/register/rider/index',
            operator: '/pages/register/operator/index'
        };
        const path = pathMap[id];
        if (path) {
            wx.navigateTo({ url: path });
        }
    },
    onContact() {
        wx.makePhoneCall({ phoneNumber: '400-800-8888' });
    },
    // 扫码入职 - 跳转到员工绑定页面
    onScanToJoin() {
        wx.navigateTo({ url: '/pages/user/bind-merchant/index' });
    },
    // 扫码认领 - 跳转到 Boss 认领页面
    onScanToClaim() {
        wx.navigateTo({ url: '/pages/user/claim-boss/index' });
    },
    onChooseAvatar(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { avatarUrl } = e.detail;
            // Optimistic Update
            this.setData({
                'userInfo.avatarUrl': avatarUrl
            });
            wx.showLoading({ title: '上传中...' });
            try {
                // 1. Upload to Server
                const imageUrl = yield upload_1.UploadService.uploadImage(avatarUrl, 'avatar');
                const remoteUrl = imageUrl;
                // 2. Persist locally with remote URL
                wx.setStorageSync('user_avatar', remoteUrl);
                // 3. Update Global Data
                app.globalData.userInfo = Object.assign(Object.assign({}, (app.globalData.userInfo || {})), { avatarUrl: remoteUrl });
                this.setData({
                    'userInfo.avatarUrl': remoteUrl
                });
                // 4. Update Backend Profile
                yield (0, auth_1.updateUserInfo)({ avatar_url: remoteUrl });
            }
            catch (error) {
                console.error('Failed to update avatar on backend', error);
                wx.showToast({ title: '头像上传失败', icon: 'none' });
            }
            finally {
                wx.hideLoading();
            }
        });
    },
    onNicknameChange(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const nickName = e.detail.value;
            if (!nickName)
                return;
            this.setData({
                'userInfo.nickName': nickName
            });
            // Update Global Data
            app.globalData.userInfo = Object.assign(Object.assign({}, (app.globalData.userInfo || {})), { nickName: nickName });
            // Call Backend API
            try {
                yield (0, auth_1.updateUserInfo)({ full_name: nickName });
            }
            catch (error) {
                console.error('Failed to update nickname on backend', error);
            }
        });
    }
});
