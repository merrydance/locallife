"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const merchant_1 = require("../../../api/merchant");
const image_1 = require("../../../utils/image");
const image_security_1 = require("../../../utils/image-security");
Page({
    data: {
        restaurantId: '',
        restaurant: null,
        navBarHeight: 88,
        loading: true
    },
    onLoad(options) {
        const restaurantId = options.id;
        if (!restaurantId) {
            wx.showToast({ title: '商家ID缺失', icon: 'error' });
            setTimeout(() => wx.navigateBack(), 1500);
            return;
        }
        this.setData({ restaurantId });
        this.loadMerchantInfo();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    async loadMerchantInfo() {
        this.setData({ loading: true });
        try {
            const merchantId = parseInt(this.data.restaurantId);
            const merchant = await (0, merchant_1.getPublicMerchantDetail)(merchantId);
            if (!merchant) {
                this.setData({ restaurant: null, loading: false });
                return;
            }
            const [coverImage, businessLicense, foodPermit] = await Promise.all([
                (0, image_security_1.resolveImageURL)(merchant.cover_image || merchant.logo_url || ''),
                (0, image_security_1.resolveImageURL)(merchant.business_license_image_url || ''),
                (0, image_security_1.resolveImageURL)(merchant.food_permit_url || '')
            ]);
            const dayNames = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
            const formattedHours = (merchant.business_hours || []).map((h) => ({
                ...h,
                day_name: dayNames[h.day_of_week]
            }));
            this.setData({
                restaurant: {
                    ...merchant,
                    cover_image: coverImage,
                    logo_url: (0, image_1.getPublicImageUrl)(merchant.logo_url || ''),
                    business_license_image_url: businessLicense,
                    food_permit_url: foodPermit,
                    business_hours: formattedHours,
                    biz_status: merchant.is_open ? 'OPEN' : 'CLOSED',
                    tags: merchant.tags || [],
                    discount_rules: merchant.discount_rules || [],
                    vouchers: merchant.vouchers || [],
                    delivery_promotions: merchant.delivery_promotions || []
                },
                loading: false
            });
        }
        catch (error) {
            console.error('加载商户信息失败:', error);
            wx.showToast({ title: '加载失败', icon: 'error' });
            this.setData({ loading: false });
        }
    },
    onCall() {
        var _a;
        const phone = (_a = this.data.restaurant) === null || _a === void 0 ? void 0 : _a.phone;
        if (!phone)
            return;
        wx.makePhoneCall({ phoneNumber: phone });
    },
    onMapTap() {
        const restaurant = this.data.restaurant;
        if (!restaurant || !restaurant.latitude || !restaurant.longitude)
            return;
        wx.openLocation({
            latitude: restaurant.latitude,
            longitude: restaurant.longitude,
            name: restaurant.name,
            address: restaurant.address
        });
    },
    onPreviewLicense(e) {
        const { src } = e.currentTarget.dataset;
        if (!src)
            return;
        wx.previewImage({ urls: [src] });
    }
});
