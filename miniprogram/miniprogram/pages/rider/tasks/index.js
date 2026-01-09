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
const rider_1 = require("../../api/rider");
const logger_1 = require("../../../utils/logger");
Page({
    data: {
        tasks: [],
        loading: false,
        navBarHeight: 88,
        hasMore: true,
        page: 1
    },
    onLoad() {
        this.loadTasks(true);
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    onPullDownRefresh() {
        this.loadTasks(true).then(() => {
            wx.stopPullDownRefresh();
        });
    },
    onReachBottom() {
        if (this.data.hasMore && !this.data.loading) {
            this.setData({ page: this.data.page + 1 });
            this.loadTasks(false);
        }
    },
    loadTasks() {
        return __awaiter(this, arguments, void 0, function* (reset = false) {
            if (this.data.loading)
                return;
            this.setData({ loading: true });
            if (reset) {
                this.setData({ page: 1, tasks: [], hasMore: true });
            }
            try {
                const res = yield (0, rider_1.getAvailableOrders)(this.data.page);
                const newTasks = res.items;
                this.setData({
                    tasks: reset ? newTasks : [...this.data.tasks, ...newTasks],
                    hasMore: newTasks.length > 0, // Simple check
                    loading: false
                });
            }
            catch (error) {
                logger_1.logger.error('Load tasks failed', error, 'Tasks');
                this.setData({ loading: false });
                wx.showToast({ title: '加载失败', icon: 'none' });
            }
        });
    },
    onTaskAction(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id, action } = e.detail;
            if (action === 'accept') {
                try {
                    yield (0, rider_1.acceptOrder)(id);
                    wx.showToast({ title: '抢单成功', icon: 'success' });
                    this.loadTasks(true); // Refresh list
                }
                catch (error) {
                    wx.showToast({ title: '抢单失败', icon: 'none' });
                }
            }
        });
    }
});
