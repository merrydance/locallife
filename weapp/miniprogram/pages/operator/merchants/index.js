"use strict";
/**
 * 运营商户管理页面
 * 使用真实后端API
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
const responsive_1 = require("@/utils/responsive");
const operator_merchant_management_1 = require("../../../api/operator-merchant-management");
Page({
    data: {
        merchants: [],
        isLargeScreen: false,
        navBarHeight: 88,
        loading: false,
        page: 1,
        hasMore: true
    },
    onLoad() {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
        this.loadMerchants();
    },
    onShow() {
        // 返回时刷新
        if (this.data.merchants.length > 0) {
            this.loadMerchants();
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadMerchants() {
        return __awaiter(this, arguments, void 0, function* (reset = true) {
            if (reset) {
                this.setData({ page: 1, merchants: [], hasMore: true });
            }
            this.setData({ loading: true });
            try {
                const result = yield operator_merchant_management_1.operatorMerchantManagementService.getMerchantList({
                    page_id: this.data.page,
                    page_size: 20
                });
                const merchants = (result.merchants || []).map((m) => {
                    var _a;
                    return ({
                        id: m.id,
                        name: m.name,
                        phone: m.phone,
                        status: ((_a = m.status) === null || _a === void 0 ? void 0 : _a.toUpperCase()) || 'UNKNOWN',
                        region_id: m.region_id,
                        created_at: m.created_at
                    });
                });
                const newMerchants = reset ? merchants : [...this.data.merchants, ...merchants];
                this.setData({
                    merchants: newMerchants,
                    hasMore: merchants.length === 20,
                    loading: false
                });
            }
            catch (error) {
                console.error('加载商户列表失败:', error);
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    onReachBottom() {
        if (this.data.hasMore && !this.data.loading) {
            this.setData({ page: this.data.page + 1 });
            this.loadMerchants(false);
        }
    },
    onToggleStatus(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id, status } = e.currentTarget.dataset;
            const isActive = status === 'ACTIVE';
            const action = isActive ? '封禁' : '解封';
            wx.showModal({
                title: '确认操作',
                content: `确认${action}该商户?`,
                success: (res) => __awaiter(this, void 0, void 0, function* () {
                    if (res.confirm) {
                        try {
                            if (isActive) {
                                yield operator_merchant_management_1.operatorMerchantManagementService.suspendMerchant(id, { reason: '运营封禁' });
                            }
                            else {
                                yield operator_merchant_management_1.operatorMerchantManagementService.resumeMerchant(id, { reason: '运营解封' });
                            }
                            wx.showToast({ title: '操作成功', icon: 'success' });
                            this.loadMerchants();
                        }
                        catch (error) {
                            console.error('操作失败:', error);
                            wx.showToast({ title: '操作失败', icon: 'error' });
                        }
                    }
                })
            });
        });
    },
    onViewDetail(e) {
        const { id } = e.currentTarget.dataset;
        wx.navigateTo({ url: `/pages/takeout/restaurant-detail/index?id=${id}` });
    }
});
