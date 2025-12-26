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
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const reservation_1 = require("../../../api/reservation");
const reservation_2 = __importDefault(require("../../../adapters/reservation"));
Page({
    data: {
        id: 0,
        reservation: null,
        loading: true,
        showCancelDialog: false,
        cancelReason: '',
        cancelReasons: ['行程改变', '订错了', '不想去了', '其他原因']
    },
    onLoad(options) {
        if (options.id) {
            this.setData({ id: parseInt(options.id) });
            this.loadDetail();
        }
    },
    loadDetail() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const res = yield reservation_1.ReservationService.getReservationDetail(this.data.id);
                const formatted = Object.assign(Object.assign({}, res), { _statusText: reservation_2.default.formatStatus(res.status), _statusTheme: reservation_2.default.getStatusTheme(res.status), _timeText: reservation_2.default.formatFullDateTime(res.reservation_time) });
                this.setData({ reservation: formatted, loading: false });
            }
            catch (error) {
                console.error(error);
                wx.showToast({ title: '加载失败', icon: 'none' });
                this.setData({ loading: false });
            }
        });
    },
    onCancel() {
        this.setData({ showCancelDialog: true });
    },
    closeCancelDialog() {
        this.setData({ showCancelDialog: false });
    },
    onReasonChange(e) {
        this.setData({ cancelReason: e.detail.value });
    },
    confirmCancel() {
        return __awaiter(this, void 0, void 0, function* () {
            if (!this.data.cancelReason) {
                wx.showToast({ title: '请选择取消原因', icon: 'none' });
                return;
            }
            try {
                wx.showLoading({ title: '提交中...' });
                yield reservation_1.ReservationService.cancelReservation(this.data.id, this.data.cancelReason);
                wx.showToast({ title: '已取消', icon: 'success' });
                this.closeCancelDialog();
                this.loadDetail();
            }
            catch (error) {
                wx.showToast({ title: error.message || '取消失败', icon: 'none' });
            }
            finally {
                wx.hideLoading();
            }
        });
    },
    onCallMerchant() {
        // Placeholder for calling merchant
        wx.makePhoneCall({ phoneNumber: '13800000000' });
    }
});
