"use strict";
/**
 * 商户打印机管理页面
 * 使用真实后端API
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
const responsive_1 = require("../../../utils/responsive");
const table_device_management_1 = require("../../../api/table-device-management");
Page({
    data: {
        printers: [],
        isLargeScreen: false,
        navBarHeight: 88,
        loading: false
    },
    onLoad() {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
        this.loadPrinters();
    },
    onShow() {
        // 返回时刷新
        if (this.data.printers.length > 0) {
            this.loadPrinters();
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadPrinters() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const result = yield table_device_management_1.deviceManagementService.listPrinters();
                const printers = (result.printers || []).map((printer) => {
                    var _a;
                    return ({
                        id: printer.id,
                        name: printer.printer_name,
                        type: ((_a = printer.printer_type) === null || _a === void 0 ? void 0 : _a.toUpperCase()) || 'UNKNOWN',
                        sn: printer.printer_sn,
                        status: printer.is_active ? 'ONLINE' : 'OFFLINE',
                        auto_print: true,
                        print_takeout: printer.print_takeout,
                        print_dine_in: printer.print_dine_in,
                        print_reservation: printer.print_reservation
                    });
                });
                this.setData({
                    printers,
                    loading: false
                });
            }
            catch (error) {
                console.error('加载打印机失败:', error);
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    onAddPrinter() {
        wx.navigateTo({ url: '/pages/merchant/printers/add/index' });
    },
    onTestPrint(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id } = e.currentTarget.dataset;
            try {
                wx.showLoading({ title: '测试打印中...' });
                yield table_device_management_1.deviceManagementService.testPrinter(id);
                wx.hideLoading();
                wx.showToast({ title: '打印成功', icon: 'success' });
            }
            catch (error) {
                wx.hideLoading();
                console.error('测试打印失败:', error);
                wx.showToast({ title: '打印失败', icon: 'error' });
            }
        });
    },
    onEditPrinter(e) {
        const { id } = e.currentTarget.dataset;
        wx.navigateTo({ url: `/pages/merchant/printers/edit/index?id=${id}` });
    },
    onDeletePrinter(e) {
        const { id } = e.currentTarget.dataset;
        wx.showModal({
            title: '删除确认',
            content: '确认删除此打印机?',
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                if (res.confirm) {
                    try {
                        yield table_device_management_1.deviceManagementService.deletePrinter(id);
                        wx.showToast({ title: '已删除', icon: 'success' });
                        this.loadPrinters();
                    }
                    catch (error) {
                        console.error('删除打印机失败:', error);
                        wx.showToast({ title: '删除失败', icon: 'error' });
                    }
                }
            })
        });
    }
});
