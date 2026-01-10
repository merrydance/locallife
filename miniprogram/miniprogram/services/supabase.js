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
exports.SUPABASE_KEY = exports.SUPABASE_URL = void 0;
exports.supabaseRequest = supabaseRequest;
exports.SUPABASE_URL = 'https://ls.merrydance.cn';
exports.SUPABASE_KEY = 'sb_publishable_ACJWlzQHlZjBrEguHvfOxg_3BJgxAaH';
/**
 * Native Supabase Request Wrapper
 * Replaces supabase-wechat-stable to avoid compatibility issues.
 */
function supabaseRequest(options) {
    return __awaiter(this, void 0, void 0, function* () {
        return new Promise((resolve) => {
            var _a;
            wx.request({
                url: `${exports.SUPABASE_URL}${options.url}`,
                method: (options.method || 'GET'),
                data: options.data,
                header: Object.assign({ 'Content-Type': 'application/json', 'apikey': exports.SUPABASE_KEY, 'Authorization': ((_a = options.headers) === null || _a === void 0 ? void 0 : _a['Authorization']) || `Bearer ${exports.SUPABASE_KEY}` }, options.headers),
                success: (res) => {
                    if (res.statusCode >= 200 && res.statusCode < 300) {
                        resolve({ data: res.data, error: null });
                    }
                    else {
                        resolve({ data: null, error: res.data || { message: `HTTP ${res.statusCode}` } });
                    }
                },
                fail: (err) => {
                    resolve({ data: null, error: err });
                }
            });
        });
    });
}
