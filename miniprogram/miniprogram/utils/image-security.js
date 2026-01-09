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
exports.isPublicUploads = isPublicUploads;
exports.resolveImageURL = resolveImageURL;
const request_1 = require("./request");
/**
 * Checks if a path is a public upload that does not require signing.
 * Based on backend rules:
 * - /uploads/public/...
 * - /uploads/reviews/...
 * - /uploads/merchants/{id}/logo/...
 * - /uploads/merchants/{id}/storefront/...
 * - /uploads/merchants/{id}/environment/...
 */
function isPublicUploads(urlOrPath) {
    if (!urlOrPath)
        return false;
    // Remove leading slash for easier checking ensuring consistency
    const path = urlOrPath.startsWith('/') ? urlOrPath.slice(1) : urlOrPath;
    return (path.startsWith('uploads/public/') ||
        path.startsWith('uploads/reviews/') ||
        // regex for uploads/merchants/{id}/logo|storefront|environment/
        /^uploads\/merchants\/\d+\/(logo|storefront|environment)\//.test(path));
}
/**
 * Normalizes a path to the stored relative path format (e.g., uploads/...)
 * Returns null if it's an external URL.
 */
function toStoredPath(urlOrPath) {
    if (!urlOrPath)
        return null;
    if (/^https?:\/\//.test(urlOrPath))
        return null; // External URL
    // Remove leading slash
    return urlOrPath.startsWith('/') ? urlOrPath.slice(1) : urlOrPath;
}
// In-memory cache for signed URLs: path -> { url, expires }
const signedUrlCache = new Map();
/**
 * Resolves an image URL.
 * - If public: returns absolute URL directly.
 * - If private: calls backend to sign and returns signed URL.
 * - If external: returns as is.
 */
function resolveImageURL(urlOrPath) {
    return __awaiter(this, void 0, void 0, function* () {
        var _a;
        if (!urlOrPath)
            return '';
        // 1. External URLs returned as is
        if (/^https?:\/\//.test(urlOrPath))
            return urlOrPath;
        // 2. Public paths: append API_BASE
        if (isPublicUploads(urlOrPath)) {
            // Ensure strictly one slash between base and path
            const baseUrl = request_1.API_BASE.endsWith('/') ? request_1.API_BASE.slice(0, -1) : request_1.API_BASE;
            const path = urlOrPath.startsWith('/') ? urlOrPath : `/${urlOrPath}`;
            return `${baseUrl}${path}`;
        }
        // 3. Private paths: Sign
        const storedPath = toStoredPath(urlOrPath);
        if (!storedPath)
            return urlOrPath; // Should verify logic here, but if not stored path and not external, what is it? Return as is.
        // Check cache
        const now = Math.floor(Date.now() / 1000);
        const cached = signedUrlCache.get(storedPath);
        // Refresh if expiring in less than 60s
        if (cached && cached.expires > now + 60) {
            return cached.url;
        }
        try {
            const res = yield (0, request_1.request)({
                url: '/v1/uploads/sign',
                method: 'POST',
                data: { path: '/' + storedPath }
            });
            // Cache it
            signedUrlCache.set(storedPath, res);
            return res.url;
        }
        catch (e) {
            // 如果是abort错误，等待短暂时间后检查缓存（另一个并发请求可能成功了）
            if ((_a = e === null || e === void 0 ? void 0 : e.errMsg) === null || _a === void 0 ? void 0 : _a.includes('abort')) {
                yield new Promise(resolve => setTimeout(resolve, 100));
                const cached = signedUrlCache.get(storedPath);
                if (cached && cached.expires > Math.floor(Date.now() / 1000) + 60) {
                    return cached.url;
                }
            }
            // 静默处理签名失败，返回备用路径
            const baseUrl = request_1.API_BASE.endsWith('/') ? request_1.API_BASE.slice(0, -1) : request_1.API_BASE;
            const path = urlOrPath.startsWith('/') ? urlOrPath : `/${urlOrPath}`;
            return `${baseUrl}${path}`;
        }
    });
}
