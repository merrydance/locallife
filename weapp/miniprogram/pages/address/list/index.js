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
const address_1 = require("../../../api/address");
Page({
    data: {
        addresses: [],
        loading: true
    },
    onShow() {
        this.loadAddresses();
    },
    loadAddresses() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const addresses = yield address_1.AddressService.getAddresses();
                this.setData({ addresses, loading: false });
            }
            catch (error) {
                console.error(error);
                this.setData({ loading: false });
                wx.showToast({ title: '加载失败', icon: 'none' });
            }
        });
    },
    onSetDefault(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const id = e.currentTarget.dataset.id;
            try {
                wx.showLoading({ title: '设置中' });
                yield address_1.AddressService.setDefaultAddress(id);
                this.loadAddresses(); // Reload to reflect changes
                wx.showToast({ title: '已设置默认', icon: 'success' });
            }
            catch (error) {
                wx.showToast({ title: '设置失败', icon: 'none' });
            }
            finally {
                wx.hideLoading();
            }
        });
    },
    onEdit(e) {
        const id = e.currentTarget.dataset.id;
        wx.navigateTo({ url: `/pages/address/edit/index?id=${id}` });
    },
    onAdd() {
        wx.navigateTo({ url: '/pages/address/edit/index' });
    }
});
