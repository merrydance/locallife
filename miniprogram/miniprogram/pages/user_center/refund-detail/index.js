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
const payment_refund_1 = require("../../../api/payment-refund");
const logger_1 = require("../../../utils/logger");
Page({
    data: {
        refundId: 0,
        refund: null,
        navBarHeight: 88,
        loading: true,
        // 显示字段
        amountDisplay: '',
        statusText: '',
        statusClass: '',
        refundTypeText: '',
        progress: []
    },
    onLoad(options) {
        if (options.id) {
            this.setData({ refundId: parseInt(options.id) });
            this.loadRefundDetail();
        }
    },
    loadRefundDetail() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const refund = yield (0, payment_refund_1.getRefundById)(this.data.refundId);
                this.processRefund(refund);
            }
            catch (error) {
                logger_1.logger.error('加载退款详情失败', error, 'refund-detail.loadRefundDetail');
                wx.showToast({ title: '加载失败', icon: 'error' });
            }
            finally {
                this.setData({ loading: false });
            }
        });
    },
    processRefund(refund) {
        const amountDisplay = `¥${(refund.amount / 100).toFixed(2)}`;
        const statusText = this.getStatusText(refund.status);
        const statusClass = refund.status;
        const refundTypeText = refund.refund_type === 'full' ? '全额退款' : '部分退款';
        const progress = this.generateProgress(refund);
        this.setData({
            refund,
            amountDisplay,
            statusText,
            statusClass,
            refundTypeText,
            progress
        });
    },
    getStatusText(status) {
        const statusMap = {
            'pending': '退款申请中',
            'processing': '退款处理中',
            'success': '退款成功',
            'failed': '退款失败'
        };
        return statusMap[status] || status;
    },
    generateProgress(refund) {
        const progress = [
            {
                title: '提交申请',
                time: this.formatTime(refund.created_at),
                done: true,
                active: refund.status === 'pending'
            },
            {
                title: '审核中',
                time: '',
                done: ['processing', 'success', 'failed'].includes(refund.status),
                active: refund.status === 'processing'
            },
            {
                title: '退款处理',
                time: '',
                done: ['success', 'failed'].includes(refund.status),
                active: false
            },
            {
                title: refund.status === 'failed' ? '退款失败' : '退款完成',
                time: refund.processed_at ? this.formatTime(refund.processed_at) : '',
                done: ['success', 'failed'].includes(refund.status),
                active: ['success', 'failed'].includes(refund.status)
            }
        ];
        return progress;
    },
    formatTime(timeStr) {
        if (!timeStr)
            return '';
        try {
            const date = new Date(timeStr);
            const m = ('0' + (date.getMonth() + 1)).slice(-2);
            const d = ('0' + date.getDate()).slice(-2);
            const h = ('0' + date.getHours()).slice(-2);
            const min = ('0' + date.getMinutes()).slice(-2);
            return `${m}-${d} ${h}:${min}`;
        }
        catch (_a) {
            return timeStr;
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    }
});
