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
const request_1 = require("@/utils/request");
Page({
    data: {
        bindCode: '',
        loading: false,
        claimed: false,
        merchantName: ''
    },
    onInputChange(e) {
        this.setData({ bindCode: e.detail.value });
    },
    onScan() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const result = yield wx.scanCode({ onlyFromCamera: false });
                // 认领码可能直接是码值或包含路径
                let code = result.result;
                if (code.includes('code=')) {
                    code = code.split('code=')[1].split('&')[0];
                }
                this.setData({ bindCode: code });
                this.claimBoss();
            }
            catch (err) {
                if (err.errMsg && !err.errMsg.includes('cancel')) {
                    wx.showToast({ title: '扫码失败', icon: 'none' });
                }
            }
        });
    },
    claimBoss() {
        return __awaiter(this, void 0, void 0, function* () {
            var _a;
            const { bindCode } = this.data;
            if (!bindCode || bindCode.length < 6) {
                wx.showToast({ title: '请输入有效的认领码', icon: 'none' });
                return;
            }
            this.setData({ loading: true });
            try {
                const res = yield (0, request_1.request)({
                    url: '/v1/claim-boss',
                    method: 'POST',
                    data: { bind_code: bindCode }
                });
                this.setData({
                    loading: false,
                    claimed: true,
                    merchantName: res.merchant_name
                });
                wx.showToast({ title: '认领成功', icon: 'success' });
            }
            catch (err) {
                this.setData({ loading: false });
                if ((err === null || err === void 0 ? void 0 : err.statusCode) === 409) {
                    wx.showModal({
                        title: '已认领',
                        content: '您已经认领过该店铺，无需重复认领',
                        showCancel: false
                    });
                    return;
                }
                if ((err === null || err === void 0 ? void 0 : err.statusCode) === 400) {
                    wx.showToast({ title: ((_a = err.data) === null || _a === void 0 ? void 0 : _a.message) || '认领码无效或已过期', icon: 'none' });
                    return;
                }
                wx.showToast({ title: '认领失败，请重试', icon: 'none' });
            }
        });
    },
    onGoToWorkspace() {
        // 跳转到 Boss 工作台
        wx.reLaunch({ url: '/pages/boss/workspace/index' });
    },
    onGoBack() {
        wx.navigateBack();
    }
});
