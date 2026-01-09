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
        // Form Data
        merchantId: 0,
        merchantName: '',
        date: '',
        time: '',
        partySize: 2,
        contactName: '',
        contactPhone: '',
        notes: '',
        // UI State
        showDatePicker: false,
        showTimePicker: false,
        showPartySizePicker: false,
        minDate: new Date().getTime(),
        maxDate: new Date().getTime() + 30 * 24 * 60 * 60 * 1000,
        // Picker Ranges
        timeOptions: [
            { label: '11:00', value: '11:00' },
            { label: '11:30', value: '11:30' },
            { label: '12:00', value: '12:00' },
            { label: '12:30', value: '12:30' },
            { label: '13:00', value: '13:00' },
            { label: '13:30', value: '13:30' },
            { label: '17:00', value: '17:00' },
            { label: '17:30', value: '17:30' },
            { label: '18:00', value: '18:00' },
            { label: '18:30', value: '18:30' },
            { label: '19:00', value: '19:00' },
            { label: '19:30', value: '19:30' },
            { label: '20:00', value: '20:00' }
        ],
        partySizeOptions: [
            { label: '1人', value: 1 },
            { label: '2人', value: 2 },
            { label: '3人', value: 3 },
            { label: '4人', value: 4 },
            { label: '5-6人', value: 6 },
            { label: '7-8人', value: 8 },
            { label: '9-10人', value: 10 },
            { label: '10人以上', value: 12 }
        ]
    },
    onLoad(options) {
        if (options.merchantId) {
            this.setData({
                merchantId: parseInt(options.merchantId),
                merchantName: options.merchantName || '餐厅'
            });
        }
        // Initialize with tomorrow's date by default
        const tomorrow = new Date();
        tomorrow.setDate(tomorrow.getDate() + 1);
        this.setData({
            date: `${tomorrow.getFullYear()}-${tomorrow.getMonth() + 1}-${tomorrow.getDate()}`
        });
    },
    // Date Picker
    showDatePicker() {
        this.setData({ showDatePicker: true });
    },
    hideDatePicker() {
        this.setData({ showDatePicker: false });
    },
    onDateChange(e) {
        this.setData({ date: e.detail.value });
    },
    onDateConfirm(e) {
        this.setData({ date: e.detail.value });
        this.hideDatePicker();
    },
    // Time Picker
    showTimePicker() {
        this.setData({ showTimePicker: true });
    },
    hideTimePicker() {
        this.setData({ showTimePicker: false });
    },
    onTimeConfirm(e) {
        const value = e.detail.value[0];
        this.setData({ time: value });
        this.hideTimePicker();
    },
    // Party Size Picker
    showPartySizePicker() {
        this.setData({ showPartySizePicker: true });
    },
    hidePartySizePicker() {
        this.setData({ showPartySizePicker: false });
    },
    onPartySizeConfirm(e) {
        const value = e.detail.value[0];
        this.setData({ partySize: value });
        this.hidePartySizePicker();
    },
    // Input Handlers
    onNameInput(e) {
        this.setData({ contactName: e.detail.value });
    },
    onPhoneInput(e) {
        this.setData({ contactPhone: e.detail.value });
    },
    onNotesInput(e) {
        this.setData({ notes: e.detail.value });
    },
    // Submit
    onSubmit() {
        return __awaiter(this, void 0, void 0, function* () {
            const { merchantId, date, time, partySize, contactName, contactPhone, notes } = this.data;
            const reservationTime = `${date} ${time}:00`;
            const validation = reservation_2.default.validateReservation({
                reservation_time: reservationTime,
                party_size: partySize,
                contact_name: contactName,
                contact_phone: contactPhone
            });
            if (!validation.valid) {
                wx.showToast({ title: validation.message || '信息不完整', icon: 'none' });
                return;
            }
            try {
                wx.showLoading({ title: '提交中...' });
                yield reservation_1.ReservationService.createReservation({
                    merchant_id: merchantId,
                    reservation_time: reservationTime,
                    party_size: partySize,
                    contact_name: contactName,
                    contact_phone: contactPhone,
                    notes: notes
                });
                wx.showToast({ title: '预订成功', icon: 'success' });
                setTimeout(() => {
                    wx.redirectTo({
                        url: '/pages/reservation/list/index'
                    });
                }, 1500);
            }
            catch (error) {
                wx.showToast({ title: error.message || '预订失败', icon: 'none' });
            }
            finally {
                wx.hideLoading();
            }
        });
    }
});
