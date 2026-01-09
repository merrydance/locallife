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
const platform_dashboard_1 = require("../../../api/platform-dashboard");
const platformDashboardService = new platform_dashboard_1.PlatformDashboardService();
Page({
    data: {
        stats: {
            gmv: '0.00',
            active_merchants: 0,
            active_users: 0,
            total_orders: 0
        },
        loading: false
    },
    onLoad() {
        this.loadStats();
    },
    loadStats() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const today = new Date().toISOString().split('T')[0];
                const result = yield platformDashboardService.getPlatformOverview({
                    start_date: today,
                    end_date: today
                });
                this.setData({
                    stats: {
                        gmv: (result.total_gmv / 100).toFixed(2),
                        active_merchants: result.active_merchants,
                        active_users: result.active_users,
                        total_orders: result.total_orders
                    },
                    loading: false
                });
            }
            catch (error) {
                console.error(error);
                this.setData({ loading: false });
            }
        });
    }
});
