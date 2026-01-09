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
exports.UploadService = void 0;
const request_1 = require("../utils/request");
const auth_1 = require("../utils/auth");
/**
 * 通用文件上传服务
 * 由于后端目前没有专门的"用户头像上传"接口，
 * 我们暂时使用最通用的图片上传接口 (Review Images)
 * 该接口对所有登录用户开放
 */
class UploadService {
    /**
     * 上传图片
     * @param filePath 本地文件路径
     * @param type 图片用途 (access, avatar, etc. - 目前后端可能忽略此参数，但保留扩展性)
     */
    static uploadImage(filePath_1) {
        return __awaiter(this, arguments, void 0, function* (filePath, type = 'common') {
            return new Promise((resolve, reject) => {
                const token = (0, auth_1.getToken)();
                // 使用 /v1/reviews/images/upload 作为通用上传通道
                // 如果后端后续提供了 /v1/users/avatar/upload，只需要修改这里的 URl
                const url = `${request_1.API_BASE}/v1/reviews/images/upload`;
                wx.uploadFile({
                    url,
                    filePath,
                    name: 'image',
                    header: {
                        'Authorization': `Bearer ${token}`
                    },
                    formData: {
                        type // 传递用途参数，以防后端支持
                    },
                    success: (res) => {
                        var _a;
                        if (res.statusCode === 200) {
                            try {
                                const data = JSON.parse(res.data);
                                if (data.code === 0 && data.data && data.data.image_url) {
                                    resolve(data.data.image_url);
                                }
                                else if (data.image_url) {
                                    resolve(data.image_url);
                                }
                                else {
                                    resolve(((_a = data.data) === null || _a === void 0 ? void 0 : _a.image_url) || data.image_url);
                                }
                            }
                            catch (e) {
                                reject(new Error('Parse response failed'));
                            }
                        }
                        else {
                            reject(new Error(`HTTP ${res.statusCode}`));
                        }
                    },
                    fail: (err) => {
                        reject(err);
                    }
                });
            });
        });
    }
}
exports.UploadService = UploadService;
