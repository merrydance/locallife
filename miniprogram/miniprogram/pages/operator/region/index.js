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
const operator_basic_management_1 = require("../../../api/operator-basic-management");
Page({
    data: {
        regions: [],
        loading: false,
        page: 1,
        pageSize: 20,
        hasMore: true
    },
    onLoad() {
        this.loadRegions(true);
    },
    onPullDownRefresh() {
        this.loadRegions(true);
    },
    onReachBottom() {
        if (this.data.hasMore && !this.data.loading) {
            this.loadRegions(false);
        }
    },
    loadRegions() {
        return __awaiter(this, arguments, void 0, function* (reset = false) {
            if (this.data.loading)
                return;
            this.setData({ loading: true });
            if (reset) {
                this.setData({ page: 1, regions: [], hasMore: true });
            }
            try {
                const res = yield operator_basic_management_1.operatorBasicManagementService.getOperatorRegions({
                    page: this.data.page,
                    limit: this.data.pageSize
                });
                const newRegions = res.regions.map(r => operator_basic_management_1.OperatorBasicManagementAdapter.adaptRegionResponse(r));
                this.setData({
                    regions: reset ? newRegions : [...this.data.regions, ...newRegions],
                    page: this.data.page + 1,
                    hasMore: res.has_more,
                    loading: false
                });
            }
            catch (err) {
                console.error(err);
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
            finally {
                if (reset)
                    wx.stopPullDownRefresh();
            }
        });
    },
    // 跳转到详细配置页
    onRegionClick(e) {
        const id = e.currentTarget.dataset.id;
        wx.navigateTo({
            url: `/pages/operator/region/config?id=${id}`
        });
    },
    onAddRegion() {
        wx.showToast({ title: '添加功能暂未开放', icon: 'none' });
    }
});
