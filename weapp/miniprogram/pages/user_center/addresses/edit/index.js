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
const address_1 = require("../../../../api/address");
const logger_1 = require("../../../../utils/logger");
Page({
    data: {
        addressId: '',
        name: '',
        gender: 1, // 1: Male, 0: Female
        mobile: '',
        address: '', // Location name
        complete_address: '', // Detailed part (building/room)
        latitude: 0,
        longitude: 0,
        tag: 'home',
        is_default: false,
        saving: false
    },
    onLoad(options) {
        if (options.id) {
            this.setData({ addressId: options.id });
            this.loadAddress(options.id);
            wx.setNavigationBarTitle({ title: '编辑地址' });
        }
        else {
            wx.setNavigationBarTitle({ title: '新增地址' });
        }
    },
    loadAddress(id) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const detail = yield (0, address_1.getAddressDetail)(id);
                this.setData({
                    name: detail.name,
                    gender: detail.gender,
                    mobile: detail.mobile,
                    address: detail.address,
                    complete_address: detail.complete_address,
                    latitude: detail.latitude,
                    longitude: detail.longitude,
                    tag: detail.tag,
                    is_default: detail.is_default
                });
            }
            catch (error) {
                logger_1.logger.error('Load address failed:', error, 'Edit');
                wx.showToast({ title: '加载失败', icon: 'error' });
            }
        });
    },
    onNameChange(e) {
        this.setData({ name: e.detail.value });
    },
    onGenderChange(e) {
        this.setData({ gender: Number(e.detail.value) });
    },
    onMobileChange(e) {
        this.setData({ mobile: e.detail.value });
    },
    onDetailChange(e) {
        this.setData({ complete_address: e.detail.value });
    },
    onTagChange(e) {
        this.setData({ tag: e.detail.value });
    },
    onDefaultChange(e) {
        this.setData({ is_default: e.detail.value });
    },
    onChooseLocation() {
        wx.chooseLocation({
            success: (res) => {
                this.setData({
                    address: res.name || res.address,
                    latitude: res.latitude,
                    longitude: res.longitude
                });
            },
            fail: (err) => {
                logger_1.logger.error('Choose location failed:', err, 'Edit');
                // Maybe show setting modal if auth denied
            }
        });
    },
    onSave() {
        return __awaiter(this, void 0, void 0, function* () {
            if (!this.validate())
                return;
            this.setData({ saving: true });
            const data = {
                name: this.data.name,
                gender: this.data.gender,
                mobile: this.data.mobile,
                address: this.data.address,
                complete_address: this.data.complete_address,
                latitude: this.data.latitude,
                longitude: this.data.longitude,
                tag: this.data.tag,
                is_default: this.data.is_default
            };
            try {
                if (this.data.addressId) {
                    yield (0, address_1.updateAddress)(this.data.addressId, data);
                }
                else {
                    yield (0, address_1.createAddress)(data);
                }
                wx.showToast({ title: '保存成功', icon: 'success' });
                setTimeout(() => wx.navigateBack(), 1500);
            }
            catch (error) {
                logger_1.logger.error('Save address failed:', error, 'Edit');
                wx.showToast({ title: '保存失败', icon: 'error' });
            }
            finally {
                this.setData({ saving: false });
            }
        });
    },
    onDelete() {
        return __awaiter(this, void 0, void 0, function* () {
            if (!this.data.addressId)
                return;
            wx.showModal({
                title: '删除地址',
                content: '确认删除此地址?',
                success: (res) => __awaiter(this, void 0, void 0, function* () {
                    if (res.confirm) {
                        try {
                            yield (0, address_1.deleteAddress)(this.data.addressId);
                            wx.showToast({ title: '已删除', icon: 'success' });
                            setTimeout(() => wx.navigateBack(), 1500);
                        }
                        catch (error) {
                            logger_1.logger.error('Delete address failed:', error, 'Edit');
                            wx.showToast({ title: '删除失败', icon: 'error' });
                        }
                    }
                })
            });
        });
    },
    validate() {
        const { name, mobile, address, complete_address } = this.data;
        if (!name.trim()) {
            wx.showToast({ title: '请填写联系人', icon: 'none' });
            return false;
        }
        if (!mobile.trim() || mobile.length < 11) {
            wx.showToast({ title: '请填写正确手机号', icon: 'none' });
            return false;
        }
        if (!address) {
            wx.showToast({ title: '请选择收货地址', icon: 'none' });
            return false;
        }
        if (!complete_address.trim()) {
            wx.showToast({ title: '请填写门牌号', icon: 'none' });
            return false;
        }
        return true;
    }
});
