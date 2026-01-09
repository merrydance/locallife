"use strict";
/**
 * 员工管理页面
 * 对接后端 /v1/merchant/staff 接口
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
const request_1 = require("@/utils/request");
// 员工管理服务
const StaffService = {
    // 获取员工列表
    listStaff() {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/staff',
                method: 'GET'
            });
        });
    },
    // 添加员工
    addStaff(userId, role) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/staff',
                method: 'POST',
                data: { user_id: userId, role }
            });
        });
    },
    // 更新员工角色
    updateStaffRole(staffId, role) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchant/staff/${staffId}/role`,
                method: 'PATCH',
                data: { role }
            });
        });
    },
    // 删除员工
    deleteStaff(staffId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchant/staff/${staffId}`,
                method: 'DELETE'
            });
        });
    },
    // 生成邀请码
    generateInviteCode() {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/staff/invite-code',
                method: 'POST'
            });
        });
    },
    // 生成 Boss 认领码
    generateBossBindCode() {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/boss-bind-code',
                method: 'POST'
            });
        });
    }
};
// 角色配置
const ROLE_CONFIG = {
    'owner': { name: '老板', color: '#722ed1', icon: '👑' },
    'manager': { name: '店长', color: '#1890ff', icon: '👔' },
    'chef': { name: '厨师长', color: '#fa8c16', icon: '👨‍🍳' },
    'cashier': { name: '收银员', color: '#52c41a', icon: '💰' }
};
Page({
    data: {
        // 员工列表
        staffList: [],
        loading: true,
        // 邀请码弹窗
        showInviteModal: false,
        inviteCode: '',
        inviteCodeUrl: '', // 包含页面路径的完整URL，用于二维码
        inviteExpiresAt: '',
        generating: false,
        // 编辑角色弹窗
        showEditModal: false,
        editingStaff: null,
        selectedRole: '',
        updating: false,
        // 删除确认弹窗
        showDeleteModal: false,
        deletingStaff: null,
        deleting: false,
        // Boss 认领码弹窗
        showBossCodeModal: false,
        bossBindCode: '',
        bossCodeUrl: '',
        bossCodeExpiresAt: '',
        generatingBossCode: false,
        // 角色配置
        roleConfig: ROLE_CONFIG,
        roleOptions: [
            { value: 'manager', label: '店长' },
            { value: 'chef', label: '厨师长' },
            { value: 'cashier', label: '收银员' }
        ]
    },
    onLoad() {
        this.loadStaffList();
    },
    onShow() {
        this.loadStaffList();
    },
    // 加载员工列表
    loadStaffList() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const result = yield StaffService.listStaff();
                this.setData({
                    staffList: result.staff || [],
                    loading: false
                });
            }
            catch (error) {
                console.error('加载员工列表失败:', error);
                wx.showToast({ title: error.message || '加载失败', icon: 'none' });
                this.setData({ loading: false });
            }
        });
    },
    // 刷新员工列表
    onRefresh() {
        return __awaiter(this, void 0, void 0, function* () {
            wx.showLoading({ title: '刷新中...', mask: true });
            yield this.loadStaffList();
            wx.hideLoading();
            wx.showToast({ title: '已刷新', icon: 'success', duration: 1000 });
        });
    },
    // 打开邀请码弹窗
    onGenerateInviteCode() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ showInviteModal: true, generating: true, inviteCode: '', inviteCodeUrl: '' });
            try {
                const result = yield StaffService.generateInviteCode();
                // 生成包含页面路径的完整URL，扫码后直接跳转
                const inviteCodeUrl = `/pages/user/bind-merchant/index?code=${result.invite_code}`;
                this.setData({
                    inviteCode: result.invite_code,
                    inviteCodeUrl: inviteCodeUrl,
                    inviteExpiresAt: result.expires_at,
                    generating: false
                });
            }
            catch (error) {
                console.error('生成邀请码失败:', error);
                wx.showToast({ title: error.message || '生成失败', icon: 'none' });
                this.setData({ generating: false });
            }
        });
    },
    // 关闭邀请码弹窗
    onCloseInviteModal() {
        this.setData({ showInviteModal: false });
    },
    // 复制邀请码
    onCopyInviteCode() {
        wx.setClipboardData({
            data: this.data.inviteCode,
            success: () => {
                wx.showToast({ title: '已复制', icon: 'success' });
            }
        });
    },
    // 保存二维码到相册
    onSaveQRCode() {
        // 获取 t-qrcode 组件的 canvas 并保存
        const query = wx.createSelectorQuery().in(this);
        query.select('t-qrcode >>> canvas')
            .fields({ node: true, size: true })
            .exec((res) => {
            var _a;
            if ((_a = res[0]) === null || _a === void 0 ? void 0 : _a.node) {
                const canvas = res[0].node;
                wx.canvasToTempFilePath({
                    canvas,
                    success: (result) => {
                        wx.saveImageToPhotosAlbum({
                            filePath: result.tempFilePath,
                            success: () => {
                                wx.showToast({ title: '已保存到相册', icon: 'success' });
                            },
                            fail: () => {
                                wx.showToast({ title: '保存失败', icon: 'none' });
                            }
                        });
                    },
                    fail: () => {
                        wx.showToast({ title: '获取图片失败', icon: 'none' });
                    }
                });
            }
            else {
                wx.showToast({ title: '请长按二维码保存', icon: 'none' });
            }
        });
    },
    // ==================== Boss 认领码 ====================
    // 生成 Boss 认领码
    onGenerateBossCode() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ showBossCodeModal: true, generatingBossCode: true });
            try {
                const result = yield StaffService.generateBossBindCode();
                const bossCodeUrl = `/pages/user/claim-boss/index?code=${result.bind_code}`;
                this.setData({
                    bossBindCode: result.bind_code,
                    bossCodeUrl: bossCodeUrl,
                    bossCodeExpiresAt: result.expires_at,
                    generatingBossCode: false
                });
            }
            catch (error) {
                console.error('生成 Boss 认领码失败:', error);
                wx.showToast({ title: error.message || '生成失败', icon: 'none' });
                this.setData({ generatingBossCode: false, showBossCodeModal: false });
            }
        });
    },
    // 关闭 Boss 认领码弹窗
    onCloseBossCodeModal() {
        this.setData({ showBossCodeModal: false });
    },
    // 复制 Boss 认领码
    onCopyBossCode() {
        wx.setClipboardData({
            data: this.data.bossBindCode,
            success: () => {
                wx.showToast({ title: '已复制', icon: 'success' });
            }
        });
    },
    // 保存 Boss 二维码
    onSaveBossQRCode() {
        wx.showToast({ title: '请长按二维码保存', icon: 'none' });
    },
    // 打开编辑角色弹窗
    onEditRole(e) {
        const staffId = e.currentTarget.dataset.id;
        const staff = this.data.staffList.find(s => s.id === staffId);
        if (staff && staff.role !== 'owner') {
            this.setData({
                showEditModal: true,
                editingStaff: staff,
                selectedRole: staff.role
            });
        }
    },
    // 关闭编辑弹窗
    onCloseEditModal() {
        this.setData({ showEditModal: false, editingStaff: null });
    },
    // 选择角色
    onSelectRole(e) {
        const role = e.currentTarget.dataset.role;
        this.setData({ selectedRole: role });
    },
    // 提交角色修改
    onSubmitRoleChange() {
        return __awaiter(this, void 0, void 0, function* () {
            const { editingStaff, selectedRole } = this.data;
            if (!editingStaff)
                return;
            this.setData({ updating: true });
            try {
                yield StaffService.updateStaffRole(editingStaff.id, selectedRole);
                wx.showToast({ title: '修改成功', icon: 'success' });
                this.setData({ showEditModal: false, editingStaff: null });
                this.loadStaffList();
            }
            catch (error) {
                console.error('修改角色失败:', error);
                wx.showToast({ title: error.message || '修改失败', icon: 'none' });
            }
            finally {
                this.setData({ updating: false });
            }
        });
    },
    // 打开删除确认弹窗
    onDeleteStaff(e) {
        const staffId = e.currentTarget.dataset.id;
        const staff = this.data.staffList.find(s => s.id === staffId);
        if (staff && staff.role !== 'owner') {
            this.setData({
                showDeleteModal: true,
                deletingStaff: staff
            });
        }
    },
    // 关闭删除弹窗
    onCloseDeleteModal() {
        this.setData({ showDeleteModal: false, deletingStaff: null });
    },
    // 确认删除
    onConfirmDelete() {
        return __awaiter(this, void 0, void 0, function* () {
            const { deletingStaff } = this.data;
            if (!deletingStaff)
                return;
            this.setData({ deleting: true });
            try {
                yield StaffService.deleteStaff(deletingStaff.id);
                wx.showToast({ title: '已移除', icon: 'success' });
                this.setData({ showDeleteModal: false, deletingStaff: null });
                this.loadStaffList();
            }
            catch (error) {
                console.error('移除员工失败:', error);
                wx.showToast({ title: error.message || '移除失败', icon: 'none' });
            }
            finally {
                this.setData({ deleting: false });
            }
        });
    },
    // 格式化日期
    formatDate(dateStr) {
        if (!dateStr)
            return '-';
        return dateStr.slice(0, 10);
    },
    // 获取角色名称
    getRoleName(role) {
        var _a;
        return ((_a = ROLE_CONFIG[role]) === null || _a === void 0 ? void 0 : _a.name) || role;
    },
    // 获取角色颜色
    getRoleColor(role) {
        var _a;
        return ((_a = ROLE_CONFIG[role]) === null || _a === void 0 ? void 0 : _a.color) || '#666';
    }
});
