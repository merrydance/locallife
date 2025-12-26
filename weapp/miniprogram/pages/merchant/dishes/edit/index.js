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
const dish_1 = require("../../../../api/dish");
const responsive_1 = require("@/utils/responsive");
const logger_1 = require("../../../../utils/logger");
const app = getApp();
Page({
    data: {
        dishId: 0,
        isEdit: false,
        merchantId: '',
        form: {
            name: '',
            category_id: 0,
            price: '',
            stock: '',
            description: '',
            image_url: ''
        },
        categories: [],
        isLargeScreen: false,
        navBarHeight: 88,
        loading: false,
        submitting: false
    },
    onLoad(options) {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
        if (options.id) {
            this.setData({ dishId: parseInt(options.id), isEdit: true });
            this.init(parseInt(options.id));
        }
        else {
            this.init();
        }
    },
    init(id) {
        this.loadCategoriesAndDish(id);
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadCategoriesAndDish(dishId) {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const categories = yield dish_1.DishManagementService.getDishCategories();
                this.setData({ categories });
                if (dishId) {
                    const dish = yield dish_1.DishManagementService.getDishDetail(dishId);
                    if (dish) {
                        this.setData({
                            form: {
                                name: dish.name,
                                category_id: dish.category_id,
                                price: (dish.price / 100).toFixed(2),
                                stock: '0', // Adjust if you have daily_limit
                                description: dish.description,
                                image_url: dish.image_url
                            }
                        });
                    }
                }
            }
            catch (error) {
                logger_1.logger.error('Load failed', error, 'DishEdit');
                wx.showToast({ title: '加载失败', icon: 'none' });
            }
            finally {
                this.setData({ loading: false });
            }
        });
    },
    onInputChange(e) {
        const { field } = e.currentTarget.dataset;
        this.setData({
            [`form.${field}`]: e.detail.value
        });
    },
    onCategoryChange(e) {
        const index = Number(e.detail.value);
        if (index >= 0 && index < this.data.categories.length) {
            const selectedCategory = this.data.categories[index];
            this.setData({ 'form.category_id': selectedCategory.id });
        }
    },
    onChooseImage() {
        wx.chooseImage({
            count: 1,
            success: (res) => {
                const filePath = res.tempFilePaths[0];
                wx.showLoading({ title: '上传中...' });
                dish_1.DishManagementService.uploadDishImage(filePath)
                    .then((url) => {
                    this.setData({ 'form.image_url': url });
                    wx.hideLoading();
                })
                    .catch((err) => {
                    logger_1.logger.error('上传图片失败', err, 'DishEdit.uploadImage');
                    wx.hideLoading();
                    wx.showToast({ title: '上传失败', icon: 'none' });
                });
            }
        });
    },
    onSubmit() {
        return __awaiter(this, void 0, void 0, function* () {
            const { form, isEdit, dishId } = this.data;
            if (!form.name || !form.category_id || !form.price) {
                wx.showToast({ title: '请填写必填项', icon: 'none' });
                return;
            }
            this.setData({ submitting: true });
            try {
                const payload = {
                    name: form.name,
                    category_id: form.category_id,
                    price: Math.round(parseFloat(form.price) * 100),
                    description: form.description,
                    image_url: form.image_url
                };
                if (isEdit) {
                    yield dish_1.DishManagementService.updateDish(dishId, payload);
                }
                else {
                    yield dish_1.DishManagementService.createDish(payload);
                }
                wx.showToast({ title: isEdit ? '保存成功' : '创建成功', icon: 'success' });
                setTimeout(() => wx.navigateBack(), 1500);
            }
            catch (error) {
                logger_1.logger.error('Submit failed', error, 'DishEdit');
                wx.showToast({ title: '保存失败', icon: 'none' });
                this.setData({ submitting: false });
            }
        });
    }
});
