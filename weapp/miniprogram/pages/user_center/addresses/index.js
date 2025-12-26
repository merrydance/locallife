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
const error_handler_1 = require("../../../utils/error-handler");
Page({
    data: {
        addresses: [],
        navBarHeight: 88,
        loading: false,
        isSelectMode: false
    },
    onLoad(options) {
        if (options.select === 'true') {
            this.setData({ isSelectMode: true });
        }
    },
    onShow() {
        this.loadAddresses();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadAddresses() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const addresses = yield (0, address_1.getAddressList)();
                this.setData({
                    addresses,
                    loading: false
                });
            }
            catch (error) {
                error_handler_1.ErrorHandler.handle(error, 'Addresses.loadAddresses');
                this.setData({ loading: false });
            }
        });
    },
    onAddAddress() {
        // Ensure edit page exists or create one. For now just a toast if not exist
        wx.navigateTo({
            url: '/pages/user_center/addresses/edit/index', fail: () => {
                wx.showToast({ title: '编辑页开发中', icon: 'none' });
            }
        });
    },
    onEditAddress(e) {
        const { id } = e.currentTarget.dataset;
        wx.navigateTo({
            url: `/pages/user_center/addresses/edit/index?id=${id}`,
            fail: () => {
                wx.showToast({ title: '编辑页开发中', icon: 'none' });
            }
        });
    },
    onDeleteAddress(e) {
        const { id } = e.currentTarget.dataset;
        wx.showModal({
            title: '删除地址',
            content: '确认删除此地址?',
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                if (res.confirm) {
                    try {
                        yield (0, address_1.deleteAddress)(id);
                        wx.showToast({ title: '已删除', icon: 'success' });
                        this.loadAddresses();
                    }
                    catch (error) {
                        error_handler_1.ErrorHandler.handle(error, 'Addresses.deleteAddress');
                    }
                }
            })
        });
    },
    onSelectAddress(e) {
        const { id } = e.currentTarget.dataset;
        if (this.data.isSelectMode) {
            const pages = getCurrentPages();
            const prevPage = pages[pages.length - 2];
            if (prevPage) {
                prevPage.setData({ selectedAddressId: id });
            }
            wx.navigateBack();
        }
    },
    onSetDefault(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id } = e.currentTarget.dataset;
            const address = this.data.addresses.find((a) => a.id === id);
            if (!address)
                return;
            try {
                // Convert AddressDTO back to CreateAddressRequest-like object for update
                // Note: In a real app, maybe a specific set_default endpoint exists or we just update is_default
                yield (0, address_1.updateAddress)(id, Object.assign(Object.assign({}, address), { is_default: true }));
                wx.showToast({ title: '已设为默认', icon: 'success' });
                this.loadAddresses();
            }
            catch (error) {
                error_handler_1.ErrorHandler.handle(error, 'Addresses.setDefault');
            }
        });
    }
});
