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
const appeals_customer_service_1 = require("../../../../api/appeals-customer-service");
Page({
    data: {
        appeals: [],
        loading: false,
        status: 'pending'
    },
    onLoad() {
        this.loadAppeals();
    },
    onShow() {
        // Refresh when returning from detail
        this.loadAppeals();
    },
    loadAppeals() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                // Using operator service for operator page
                const appeals = yield appeals_customer_service_1.operatorAppealReviewService.getPendingAppeals({
                    status: this.data.status === 'pending' ? 'pending' : 'approved', // Simplified status mapping
                    page_id: 1,
                    page_size: 20
                });
                this.setData({
                    appeals: appeals.appeals || [],
                    loading: false
                });
            }
            catch (error) {
                console.error(error);
                this.setData({ loading: false });
            }
        });
    },
    onTabChange(e) {
        this.setData({ status: e.detail.value });
        this.loadAppeals();
    },
    onDetail(e) {
        const id = e.currentTarget.dataset.id;
        wx.navigateTo({
            url: `/pages/operator/appeal/detail/index?id=${id}`
        });
    }
});
