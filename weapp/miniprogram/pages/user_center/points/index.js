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
const points_1 = require("../../../api/points");
const logger_1 = require("../../../utils/logger");
Page({
    data: {
        points: 0,
        history: [],
        navBarHeight: 88,
        loading: false
    },
    onLoad() {
        this.loadData();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadData() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const [summary, historyRes] = yield Promise.all([
                    points_1.PointsService.getSummary(),
                    points_1.PointsService.getHistory(1, 20)
                ]);
                this.setData({
                    points: summary.balance,
                    history: historyRes.list,
                    loading: false
                });
            }
            catch (error) {
                logger_1.logger.error('Load points failed', error);
                // Fallback or error state
                this.setData({ loading: false });
            }
        });
    },
    onExchange() {
        wx.showToast({ title: '积分商城开发中', icon: 'none' });
    }
});
