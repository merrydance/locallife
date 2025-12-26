"use strict";
/**
 * 图片处理工具
 * 支持七牛云、阿里云OSS等常见图片服务的URL参数优化
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.imageLazyLoader = exports.ImageSize = void 0;
exports.formatImageUrl = formatImageUrl;
exports.getImageWithFallback = getImageWithFallback;
exports.formatImageUrls = formatImageUrls;
exports.preloadImages = preloadImages;
exports.getPlaceholder = getPlaceholder;
exports.getPublicImageUrl = getPublicImageUrl;
const image_lazy_load_1 = require("./image-lazy-load");
Object.defineProperty(exports, "imageLazyLoader", { enumerable: true, get: function () { return image_lazy_load_1.imageLazyLoader; } });
/**
 * 格式化图片URL，添加裁剪和压缩参数
 * @param url 原始图片URL
 * @param size 图片尺寸（宽高相同时）
 * @param options 详细配置
 * @returns 优化后的图片URL
 */
function formatImageUrl(url, size, options) {
    // 空值处理
    if (!url) {
        return '/assets/placeholder.png';
    }
    // 预处理Url：如果以 / 开头，拼接域名
    url = getPublicImageUrl(url);
    // 本地图片或已处理图片，直接返回
    if (url.startsWith('/assets') || url.startsWith('data:')) {
        return url;
    }
    // 合并配置
    const config = {
        width: size || (options === null || options === void 0 ? void 0 : options.width) || 300,
        height: size || (options === null || options === void 0 ? void 0 : options.height) || 300,
        quality: (options === null || options === void 0 ? void 0 : options.quality) || 80,
        format: (options === null || options === void 0 ? void 0 : options.format) || 'jpg',
        mode: (options === null || options === void 0 ? void 0 : options.mode) || 'crop'
    };
    // 检测图片服务商类型
    if (url.includes('qiniucdn.com') || url.includes('qnssl.com')) {
        // 七牛云
        return formatQiniuUrl(url, config);
    }
    else if (url.includes('aliyuncs.com')) {
        // 阿里云 OSS
        return formatAliyunUrl(url, config);
    }
    else if (url.includes('myqcloud.com')) {
        // 腾讯云 COS
        return formatTencentUrl(url, config);
    }
    else {
        // 未知服务商，返回原URL
        return url;
    }
}
/**
 * 七牛云图片处理
 * 文档: https://developer.qiniu.com/dora/manual/1279/basic-processing-images-imageview2
 */
function formatQiniuUrl(url, options) {
    const { width, height, quality, mode } = options;
    const modeCode = mode === 'crop' ? 1 : 2;
    // imageView2/<mode>/w/<Width>/h/<Height>/q/<Quality>
    const params = `imageView2/${modeCode}/w/${width}/h/${height}/q/${quality}`;
    // 处理已有参数的情况
    const separator = url.includes('?') ? '&' : '?';
    return `${url}${separator}${params}`;
}
/**
 * 阿里云OSS图片处理
 * 文档: https://help.aliyun.com/document_detail/44688.html
 */
function formatAliyunUrl(url, options) {
    const { width, height, quality, mode } = options;
    const modeParam = mode === 'crop' ? 'c' : 'm';
    // image/resize,m_fill,w_300,h_300/quality,q_80
    const params = `image/resize,${modeParam}_fill,w_${width},h_${height}/quality,q_${quality}`;
    const separator = url.includes('?') ? '&' : '?';
    return `${url}${separator}x-oss-process=${params}`;
}
/**
 * 腾讯云COS图片处理
 * 文档: https://cloud.tencent.com/document/product/460/36540
 */
function formatTencentUrl(url, options) {
    const { width, height, quality } = options;
    // imageMogr2/thumbnail/300x300/quality/80
    const params = `imageMogr2/thumbnail/${width}x${height}/quality/${quality}`;
    const separator = url.includes('?') ? '&' : '?';
    return `${url}${separator}${params}`;
}
/**
 * 预设尺寸常量
 */
exports.ImageSize = {
    THUMB: 100, // 缩略图
    SMALL: 200, // 小图
    MEDIUM: 400, // 中图
    LARGE: 800, // 大图
    CARD: 300, // 卡片图
    AVATAR: 120, // 头像
    BANNER: 750 // Banner
};
/**
 * 获取带错误占位符的图片URL
 */
function getImageWithFallback(url, size) {
    return formatImageUrl(url, size);
}
/**
 * 批量格式化图片URL（用于列表）
 */
function formatImageUrls(urls, size) {
    return urls.map((url) => formatImageUrl(url, size));
}
/**
 * 预加载图片列表（用于列表页）
 * @param urls 图片URL数组
 * @param priority 是否高优先级
 */
function preloadImages(urls, priority = false) {
    const formattedUrls = urls.filter(Boolean).map((url) => formatImageUrl(url));
    image_lazy_load_1.imageLazyLoader.preload(formattedUrls, priority);
}
/**
 * 获取占位图
 */
function getPlaceholder() {
    return image_lazy_load_1.PLACEHOLDER_IMAGE;
}
const request_1 = require("./request");
/**
 * 获取公共图片完整URL
 * 如果是相对路径（以/开头），则拼接API_BASE
 */
function getPublicImageUrl(path) {
    if (!path)
        return '';
    if (/^https?:\/\//.test(path))
        return path;
    if (path.startsWith('data:'))
        return path;
    if (path.startsWith('/')) {
        const baseUrl = request_1.API_BASE.endsWith('/') ? request_1.API_BASE.slice(0, -1) : request_1.API_BASE;
        return `${baseUrl}${path}`;
    }
    return path;
}
