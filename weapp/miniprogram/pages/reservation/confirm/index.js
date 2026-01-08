"use strict";
/**
 * 预约确认页面
 * 支持定金模式和全款模式
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
const reservation_1 = require("../../../api/reservation");
const room_1 = require("../../../api/room");
Page({
    data: {
        roomId: '',
        tableId: 0,
        merchantId: 0,
        roomName: '',
        capacity: 10,
        deposit: 0,
        paymentMode: 'deposit',
        form: {
            date: '',
            time: '',
            guestCount: 1,
            name: '',
            phone: '',
            remark: ''
        },
        minDate: new Date().getTime(),
        navBarHeight: 88,
        submitting: false,
        dateVisible: false,
        // 时段选择
        timeSlots: [],
        availableTimeSlots: [], // 可用时段列表（picker格式）
        timePickerVisible: false,
        loadingSlots: false
    },
    onLoad(options) {
        if (options.roomId) {
            this.setData({
                roomId: options.roomId,
                tableId: parseInt(options.roomId) || 0,
                merchantId: parseInt(options.merchantId) || 0,
                roomName: decodeURIComponent(options.roomName || ''),
                capacity: parseInt(options.capacity) || 10,
                deposit: Number(options.deposit) || 10000
            });
        }
        // 默认日期为明天
        const tomorrow = new Date();
        tomorrow.setDate(tomorrow.getDate() + 1);
        // 格式化日期为 YYYY-MM-DD（后端需要）
        const pad = (n) => n < 10 ? '0' + n : String(n);
        const dateStr = `${tomorrow.getFullYear()}-${pad(tomorrow.getMonth() + 1)}-${pad(tomorrow.getDate())}`;
        this.setData({ 'form.date': dateStr });
        // 加载明天的可用时段
        if (options.roomId) {
            this.loadAvailability(dateStr);
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    onInputChange(e) {
        const { field } = e.currentTarget.dataset;
        this.setData({
            [`form.${field}`]: e.detail.value
        });
    },
    onGuestCountChange(e) {
        this.setData({ 'form.guestCount': e.detail.value });
    },
    showDatePicker() {
        this.setData({ dateVisible: true });
    },
    hideDatePicker() {
        this.setData({ dateVisible: false });
    },
    onDateConfirm(e) {
        const date = e.detail.value;
        this.setData({
            'form.date': date,
            'form.time': '', // 重置时间选择
            dateVisible: false
        });
        // 加载新日期的可用时段
        this.loadAvailability(date);
    },
    // 加载可用时段
    loadAvailability(date) {
        return __awaiter(this, void 0, void 0, function* () {
            const { tableId } = this.data;
            if (!tableId)
                return;
            this.setData({ loadingSlots: true });
            try {
                const response = yield (0, room_1.checkRoomAvailability)(tableId, { date });
                const timeSlots = response.time_slots || [];
                // 转换为picker需要的格式：{label, value}
                const availableTimeSlots = timeSlots
                    .filter(slot => slot.available)
                    .map(slot => ({ label: slot.time, value: slot.time }));
                this.setData({
                    timeSlots,
                    availableTimeSlots,
                    loadingSlots: false
                });
                if (availableTimeSlots.length === 0) {
                    wx.showToast({ title: '该日期暂无可用时段', icon: 'none' });
                }
            }
            catch (error) {
                console.error('获取可用时段失败:', error);
                this.setData({ loadingSlots: false });
                wx.showToast({ title: '获取时段失败', icon: 'error' });
            }
        });
    },
    showTimePicker() {
        if (this.data.availableTimeSlots.length === 0) {
            wx.showToast({ title: '请先选择日期', icon: 'none' });
            return;
        }
        this.setData({ timePickerVisible: true });
    },
    hideTimePicker() {
        this.setData({ timePickerVisible: false });
    },
    onTimeSelect(e) {
        const { value } = e.detail;
        // value 是选中项的label数组，如 ["17:30"]
        const selectedTime = Array.isArray(value) ? value[0] : value;
        if (selectedTime) {
            this.setData({
                'form.time': selectedTime,
                timePickerVisible: false
            });
        }
    },
    onPaymentModeChange(e) {
        this.setData({ paymentMode: e.detail.value });
    },
    onSubmit() {
        return __awaiter(this, void 0, void 0, function* () {
            const { form, tableId, paymentMode, merchantId } = this.data;
            // 表单验证
            if (!form.date || !form.time || !form.name || !form.phone) {
                wx.showToast({ title: '请填写完整信息', icon: 'none' });
                return;
            }
            if (!/^1\d{10}$/.test(form.phone)) {
                wx.showToast({ title: '手机号格式不正确', icon: 'none' });
                return;
            }
            if (!tableId) {
                wx.showToast({ title: '请选择包间', icon: 'none' });
                return;
            }
            this.setData({ submitting: true });
            try {
                // 无论哪种模式，都先创建预订（锁定房间）
                const reservationData = {
                    table_id: tableId,
                    date: form.date,
                    time: form.time,
                    guest_count: form.guestCount,
                    contact_name: form.name,
                    contact_phone: form.phone,
                    payment_mode: paymentMode,
                    notes: form.remark || undefined
                };
                const reservation = yield (0, reservation_1.createReservation)(reservationData);
                if (paymentMode === 'full') {
                    // 全款模式：跳转到点菜页面（传入 reservation_id）
                    wx.redirectTo({
                        url: `/pages/dine-in/menu/menu?reservation_id=${reservation.id}&merchant_id=${merchantId}`
                    });
                }
                else {
                    // 定金模式：跳转到支付页面
                    wx.showToast({ title: '预定创建成功', icon: 'success' });
                    setTimeout(() => {
                        // 跳转到预订列表页（用户可在列表中找到待支付预订并支付）
                        wx.redirectTo({
                            url: `/pages/user_center/reservations/index`
                        });
                    }, 1000);
                }
            }
            catch (error) {
                console.error('预定提交失败:', error);
                wx.showToast({ title: (error === null || error === void 0 ? void 0 : error.message) || '提交失败', icon: 'none' });
            }
            finally {
                this.setData({ submitting: false });
            }
        });
    }
});
