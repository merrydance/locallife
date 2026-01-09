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
        id: 0,
        appeal: null,
        replyContent: '',
        showRejectDialog: false
    },
    onLoad(options) {
        if (options.id) {
            this.setData({ id: parseInt(options.id) });
            this.loadDetail(parseInt(options.id));
        }
    },
    loadDetail(id) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const appeal = yield appeals_customer_service_1.operatorAppealReviewService.getAppealDetailForReview(id);
                this.setData({ appeal });
            }
            catch (error) {
                console.error(error);
                wx.showToast({ title: '加载失败', icon: 'none' });
            }
        });
    },
    onInput(e) {
        this.setData({ replyContent: e.detail.value });
    },
    onApprove() {
        return __awaiter(this, void 0, void 0, function* () {
            // Assume 'resolved' is the success status or we have a specific approve action
            yield this.handleAppeal('approved');
        });
    },
    onReject() {
        this.setData({ showRejectDialog: true });
    },
    onRejectConfirm() {
        return __awaiter(this, void 0, void 0, function* () {
            yield this.handleAppeal('rejected');
            this.setData({ showRejectDialog: false });
        });
    },
    onRejectCancel() {
        this.setData({ showRejectDialog: false });
    },
    handleAppeal(status) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id, replyContent } = this.data;
            try {
                wx.showLoading({ title: '处理中' });
                yield appeals_customer_service_1.operatorAppealReviewService.reviewAppeal(id, {
                    status: status,
                    review_notes: replyContent // Using replyContent as note
                });
                wx.showToast({ title: '处理成功', icon: 'success' });
                setTimeout(() => wx.navigateBack(), 1500);
            }
            catch (error) {
                wx.showToast({ title: error.message || '处理失败', icon: 'none' });
            }
            finally {
                wx.hideLoading();
            }
        });
    }
});
