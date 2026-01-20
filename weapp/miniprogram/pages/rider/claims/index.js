"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const rider_exception_handling_1 = require("../../../api/rider-exception-handling");
const appeals_customer_service_1 = require("../../../api/appeals-customer-service");
Page({
    data: {
        taskId: '',
        claims: [],
        form: {
            type: '',
            typeLabel: '',
            description: '',
            images: []
        },
        types: [
            { label: '商家出餐慢', value: 'MERCHANT_DELAY' },
            { label: '顾客联系不上', value: 'CUSTOMER_UNREACHABLE' },
            { label: '餐品损坏', value: 'DAMAGED' },
            { label: '其他', value: 'OTHER' }
        ],
        showTypePicker: false,
        navBarHeight: 88,
        loading: false,
        submitting: false
    },
    onLoad(options) {
        if (options.taskId) {
            this.setData({ taskId: options.taskId });
        }
        this.loadClaims();
    },
    onShow() {
        this.loadClaims();
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    async loadClaims() {
        this.setData({ loading: true });
        try {
            const response = await rider_exception_handling_1.riderExceptionHandlingService.getRiderClaims({
                page_id: 1,
                page_size: 20
            });
            const claims = response.claims.map((c) => {
                var _a;
                return ({
                    id: c.id,
                    task_id: (_a = c.order_id) === null || _a === void 0 ? void 0 : _a.toString(),
                    type: c.claim_type,
                    description: c.description,
                    status: c.status,
                    created_at: c.created_at
                });
            });
            this.setData({
                claims,
                loading: false
            });
        }
        catch (error) {
            console.error('加载申诉列表失败:', error);
            wx.showToast({ title: '加载失败', icon: 'error' });
            this.setData({ loading: false, claims: [] });
        }
    },
    onTypeClick() {
        this.setData({ showTypePicker: true });
    },
    onTypeChange(e) {
        const { value } = e.detail;
        const selected = this.data.types.find((t) => t.value === value[0]);
        this.setData({
            'form.type': value[0],
            'form.typeLabel': (selected === null || selected === void 0 ? void 0 : selected.label) || '',
            showTypePicker: false
        });
    },
    onTypeCancel() {
        this.setData({ showTypePicker: false });
    },
    onDescChange(e) {
        this.setData({ 'form.description': e.detail.value });
    },
    onAddImage() {
        wx.chooseMedia({
            count: 1,
            mediaType: ['image'],
            success: (res) => {
                const { images } = this.data.form;
                this.setData({
                    'form.images': [...images, res.tempFiles[0].tempFilePath]
                });
            }
        });
    },
    async onSubmit() {
        const { form, taskId } = this.data;
        if (!form.type || !form.description) {
            wx.showToast({ title: '请填写完整信息', icon: 'none' });
            return;
        }
        this.setData({ submitting: true });
        try {
            // 创建申诉
            const appealData = {
                claim_id: taskId ? Number(taskId) : 0,
                evidence_urls: form.images,
                reason: `[${form.type}] ${form.description}`
            };
            await appeals_customer_service_1.appealManagementService.createRiderAppeal(appealData);
            wx.showToast({ title: '提交成功', icon: 'success' });
            this.setData({
                form: { type: '', typeLabel: '', description: '', images: [] },
                submitting: false
            });
            this.loadClaims();
        }
        catch (error) {
            console.error('提交申诉失败:', error);
            wx.showToast({ title: '提交失败', icon: 'error' });
            this.setData({ submitting: false });
        }
    }
});
