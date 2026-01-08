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
const address_1 = __importDefault(require("../../../api/address"));
const logger_1 = require("../../../utils/logger");
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
                const addresses = yield address_1.default.getAddresses();
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
        wx.navigateTo({
            url: '/pages/user_center/addresses/edit/index'
        });
    },
    /**
     * 从微信导入地址
     */
    onImportWechatAddress() {
        wx.chooseAddress({
            success: (res) => {
                // 跳转到编辑页，预填微信地址数据
                const params = encodeURIComponent(JSON.stringify({
                    contact_name: res.userName,
                    contact_phone: res.telNumber,
                    detail_address: `${res.provinceName}${res.cityName}${res.countyName}${res.detailInfo}`
                }));
                wx.navigateTo({
                    url: `/pages/user_center/addresses/edit/index?wechat_data=${params}`
                });
            },
            fail: (err) => {
                if (err.errMsg.includes('cancel'))
                    return;
                logger_1.logger.error('Choose address failed:', err, 'Addresses');
                wx.showToast({ title: '获取微信地址失败', icon: 'none' });
            }
        });
    },
    onEditAddress(e) {
        const { id } = e.currentTarget.dataset;
        wx.navigateTo({
            url: `/pages/user_center/addresses/edit/index?id=${id}`
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
                        yield address_1.default.deleteAddress(id);
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
            try {
                yield address_1.default.setDefaultAddress(id);
                wx.showToast({ title: '已设为默认', icon: 'success' });
                this.loadAddresses();
            }
            catch (error) {
                error_handler_1.ErrorHandler.handle(error, 'Addresses.setDefault');
            }
        });
    }
});
