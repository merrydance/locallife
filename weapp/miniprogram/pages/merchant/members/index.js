"use strict";
/**
 * 会员管理页面
 * 使用新的 /merchants/:id/members API 展示会员余额和交易记录
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
const request_1 = require("@/utils/request");
// 会员管理服务
const MemberService = {
    // 获取会员列表
    listMembers(merchantId, pageId, pageSize) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/members`,
                method: 'GET',
                data: { page_id: pageId, page_size: pageSize }
            });
        });
    },
    // 获取会员详情
    getMemberDetail(merchantId, userId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/members/${userId}`,
                method: 'GET'
            });
        });
    },
    // 调整余额
    adjustBalance(merchantId, userId, amount, notes) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/members/${userId}/balance`,
                method: 'POST',
                data: { amount, notes }
            });
        });
    }
};
Page({
    data: {
        merchantId: 0,
        sidebarCollapsed: false,
        // 会员列表
        members: [],
        loading: true,
        pageId: 1,
        pageSize: 20,
        hasMore: true,
        // 选中的会员
        selectedMember: null,
        showDetailModal: false,
        detailLoading: false,
        // 余额调整
        showAdjustModal: false,
        adjustForm: {
            amount: '',
            notes: '',
            type: 'add'
        },
        adjusting: false
    },
    onLoad() {
        this.initData();
    },
    initData() {
        return __awaiter(this, void 0, void 0, function* () {
            const app = getApp();
            const merchantId = app.globalData.merchantId;
            if (merchantId) {
                this.setData({ merchantId: Number(merchantId) });
                yield this.loadMembers();
            }
            else {
                app.userInfoReadyCallback = () => __awaiter(this, void 0, void 0, function* () {
                    if (app.globalData.merchantId) {
                        this.setData({ merchantId: Number(app.globalData.merchantId) });
                        yield this.loadMembers();
                    }
                });
            }
        });
    },
    onSidebarCollapse(e) {
        this.setData({ sidebarCollapsed: e.detail.collapsed });
    },
    // 加载会员列表
    loadMembers() {
        return __awaiter(this, arguments, void 0, function* (loadMore = false) {
            const { merchantId, pageId, pageSize, members } = this.data;
            if (!loadMore) {
                this.setData({ loading: true, pageId: 1, members: [] });
            }
            try {
                const result = yield MemberService.listMembers(merchantId, loadMore ? pageId : 1, pageSize);
                this.setData({
                    members: loadMore ? [...members, ...result] : result,
                    hasMore: result.length === pageSize,
                    pageId: loadMore ? pageId + 1 : 2,
                    loading: false
                });
            }
            catch (error) {
                console.error('加载会员列表失败:', error);
                wx.showToast({ title: error.message || '加载失败', icon: 'none' });
                this.setData({ loading: false });
            }
        });
    },
    // 加载更多
    onLoadMore() {
        if (this.data.hasMore && !this.data.loading) {
            this.loadMembers(true);
        }
    },
    // 查看会员详情
    onViewMember(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const userId = e.currentTarget.dataset.userId;
            const { merchantId } = this.data;
            this.setData({ showDetailModal: true, detailLoading: true, selectedMember: null });
            try {
                const detail = yield MemberService.getMemberDetail(merchantId, userId);
                this.setData({ selectedMember: detail, detailLoading: false });
            }
            catch (error) {
                console.error('加载会员详情失败:', error);
                wx.showToast({ title: error.message || '加载失败', icon: 'none' });
                this.setData({ detailLoading: false });
            }
        });
    },
    // 关闭详情弹窗
    onCloseDetail() {
        this.setData({ showDetailModal: false, selectedMember: null });
    },
    // 打开余额调整弹窗
    onOpenAdjust(e) {
        const userId = e.currentTarget.dataset.userId;
        // 先关闭详情弹窗
        this.setData({
            showDetailModal: false,
            showAdjustModal: true,
            adjustForm: { amount: '', notes: '', type: 'add' }
        });
    },
    // 关闭余额调整弹窗
    onCloseAdjust() {
        this.setData({ showAdjustModal: false });
    },
    // 调整类型切换
    onAdjustTypeChange(e) {
        const type = e.currentTarget.dataset.type;
        this.setData({ 'adjustForm.type': type });
    },
    // 输入金额
    onAmountInput(e) {
        this.setData({ 'adjustForm.amount': e.detail.value });
    },
    // 输入备注
    onNotesInput(e) {
        this.setData({ 'adjustForm.notes': e.detail.value });
    },
    // 提交余额调整
    onSubmitAdjust() {
        return __awaiter(this, void 0, void 0, function* () {
            const { merchantId, selectedMember, adjustForm } = this.data;
            if (!selectedMember)
                return;
            const amountYuan = parseFloat(adjustForm.amount);
            if (isNaN(amountYuan) || amountYuan <= 0) {
                wx.showToast({ title: '请输入有效金额', icon: 'none' });
                return;
            }
            if (!adjustForm.notes.trim()) {
                wx.showToast({ title: '请输入调整原因', icon: 'none' });
                return;
            }
            const amountFen = Math.round(amountYuan * 100);
            const finalAmount = adjustForm.type === 'add' ? amountFen : -amountFen;
            this.setData({ adjusting: true });
            try {
                yield MemberService.adjustBalance(merchantId, selectedMember.user_id, finalAmount, adjustForm.notes);
                wx.showToast({ title: '调整成功', icon: 'success' });
                this.setData({ showAdjustModal: false });
                this.loadMembers(); // 刷新列表
            }
            catch (error) {
                console.error('余额调整失败:', error);
                wx.showToast({ title: error.message || '调整失败', icon: 'none' });
            }
            finally {
                this.setData({ adjusting: false });
            }
        });
    },
    // 格式化金额
    formatAmount(fen) {
        return (fen / 100).toFixed(2);
    },
    // 格式化日期
    formatDate(dateStr) {
        if (!dateStr)
            return '-';
        return dateStr.slice(0, 10);
    },
    // 格式化交易类型
    formatTxType(type) {
        const map = {
            'recharge': '充值',
            'consume': '消费',
            'refund': '退款',
            'adjustment_credit': '余额增加',
            'adjustment_debit': '余额扣减'
        };
        return map[type] || type;
    }
});
