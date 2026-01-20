"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const favorite_1 = require("../../api/favorite");
Page({
    data: {
        currentTab: 'merchant',
        tabs: [
            { value: 'merchant', label: '收藏店铺' },
            { value: 'dish', label: '收藏菜品' }
        ],
        favorites: [],
        page: 1,
        pageSize: 10,
        hasMore: true,
        loading: false,
        refreshing: false
    },
    onLoad() {
        this.loadFavorites(true);
    },
    onPullDownRefresh() {
        this.setData({ refreshing: true });
        this.loadFavorites(true).then(() => {
            this.setData({ refreshing: false });
            wx.stopPullDownRefresh();
        });
    },
    onReachBottom() {
        if (this.data.hasMore && !this.data.loading) {
            this.loadFavorites(false);
        }
    },
    onTabChange(e) {
        this.setData({
            currentTab: e.detail.value,
            favorites: [],
            page: 1,
            hasMore: true
        });
        this.loadFavorites(true);
    },
    async loadFavorites(reset) {
        if (this.data.loading && !reset)
            return;
        this.setData({ loading: true });
        try {
            const page = reset ? 1 : this.data.page;
            const res = await favorite_1.FavoriteService.getFavorites({
                page_id: page,
                page_size: this.data.pageSize,
                type: this.data.currentTab
            });
            this.setData({
                favorites: reset ? res.items : [...this.data.favorites, ...res.items],
                page: page + 1,
                hasMore: res.items.length === this.data.pageSize,
                loading: false
            });
        }
        catch (error) {
            console.error(error);
            this.setData({ loading: false });
        }
    },
    onItemClick(e) {
        const id = e.currentTarget.dataset.id;
        const type = this.data.currentTab;
        if (type === 'merchant') {
            wx.navigateTo({ url: `/pages/merchant/detail/index?id=${id}` });
        }
        else {
            // Navigate to dish detail or merchant page with dish anchor
            // Simplified: just go to merchant for now or show toast
            wx.showToast({ title: '跳转到菜品详情', icon: 'none' });
        }
    },
    async onRemove(e) {
        const { id, index } = e.currentTarget.dataset;
        const type = this.data.currentTab;
        try {
            await favorite_1.FavoriteService.removeFavorite(type, id);
            // Remove from list locally
            const favorites = [...this.data.favorites];
            favorites.splice(index, 1);
            this.setData({ favorites });
            wx.showToast({ title: '已取消收藏', icon: 'success' });
        }
        catch (error) {
            wx.showToast({ title: '操作失败', icon: 'none' });
        }
    }
});
