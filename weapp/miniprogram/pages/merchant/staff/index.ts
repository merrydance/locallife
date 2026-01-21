/**
 * 员工管理页面
 * 对接后端 /v1/merchant/staff 接口
 */

import { request, API_BASE } from '@/utils/request'

// 员工响应类型
interface StaffResponse {
    id: number
    merchant_id: number
    user_id: number
    role: string
    status: string
    full_name: string
    avatar_url: string
    created_at: string
}

// 邀请码响应类型
interface InviteCodeResponse {
    invite_code: string
    expires_at: string
}

// 员工管理服务
const StaffService = {
    // 获取员工列表
    async listStaff(): Promise<{ staff: StaffResponse[], count: number }> {
        return request<{ staff: StaffResponse[], count: number }>({
            url: '/v1/merchant/staff',
            method: 'GET'
        })
    },

    // 添加员工
    async addStaff(userId: number, role: string): Promise<StaffResponse> {
        return request<StaffResponse>({
            url: '/v1/merchant/staff',
            method: 'POST',
            data: { user_id: userId, role }
        })
    },

    // 更新员工角色
    async updateStaffRole(staffId: number, role: string): Promise<StaffResponse> {
        return request<StaffResponse>({
            url: `/v1/merchant/staff/${staffId}/role`,
            method: 'PATCH',
            data: { role }
        })
    },

    // 删除员工
    async deleteStaff(staffId: number): Promise<void> {
        return request<void>({
            url: `/v1/merchant/staff/${staffId}`,
            method: 'DELETE'
        })
    },

    // 生成邀请码
    async generateInviteCode(): Promise<InviteCodeResponse> {
        return request<InviteCodeResponse>({
            url: '/v1/merchant/staff/invite-code',
            method: 'POST'
        })
    }
}

// 角色配置
const ROLE_CONFIG: Record<string, { name: string, color: string, icon: string }> = {
    'owner': { name: '老板', color: '#722ed1', icon: '👑' },
    'manager': { name: '店长', color: '#1890ff', icon: '👔' },
    'chef': { name: '厨师长', color: '#fa8c16', icon: '👨‍🍳' },
    'cashier': { name: '收银员', color: '#52c41a', icon: '💰' }
}

Page({
    data: {
        // 员工列表
        staffList: [] as StaffResponse[],
        loading: true,

        // 邀请码弹窗
        showInviteModal: false,
        inviteCode: '',
        inviteCodeUrl: '', // 包含页面路径的完整URL，用于二维码
        inviteExpiresAt: '',
        generating: false,

        // 编辑角色弹窗
        showEditModal: false,
        editingStaff: null as StaffResponse | null,
        selectedRole: '',
        updating: false,

        // 删除确认弹窗
        showDeleteModal: false,
        deletingStaff: null as StaffResponse | null,
        deleting: false,

        // 角色配置
        roleConfig: ROLE_CONFIG,
        roleOptions: [
            { value: 'manager', label: '店长' },
            { value: 'chef', label: '厨师长' },
            { value: 'cashier', label: '收银员' }
        ]
    },

    onLoad() {
        this.loadStaffList()
    },

    onShow() {
        this.loadStaffList()
    },

    // 加载员工列表
    async loadStaffList() {
        this.setData({ loading: true })
        try {
            const result = await StaffService.listStaff()
            this.setData({
                staffList: result.staff || [],
                loading: false
            })
        } catch (error: any) {
            console.error('加载员工列表失败:', error)
            wx.showToast({ title: error.message || '加载失败', icon: 'none' })
            this.setData({ loading: false })
        }
    },

    // 刷新员工列表
    async onRefresh() {
        wx.showLoading({ title: '刷新中...', mask: true })
        await this.loadStaffList()
        wx.hideLoading()
        wx.showToast({ title: '已刷新', icon: 'success', duration: 1000 })
    },

    // 打开邀请码弹窗
    async onGenerateInviteCode() {
        this.setData({ showInviteModal: true, generating: true, inviteCode: '', inviteCodeUrl: '' })
        try {
            const result = await StaffService.generateInviteCode()
            // 生成完整 URL，保证扫码可识别
            const base = API_BASE.endsWith('/') ? API_BASE.slice(0, -1) : API_BASE
            const inviteCodeUrl = `${base}/bind-merchant?code=${result.invite_code}`
            this.setData({
                inviteCode: result.invite_code,
                inviteCodeUrl: inviteCodeUrl,
                inviteExpiresAt: result.expires_at,
                generating: false
            })
        } catch (error: any) {
            console.error('生成邀请码失败:', error)
            wx.showToast({ title: error.message || '生成失败', icon: 'none' })
            this.setData({ generating: false })
        }
    },

    // 关闭邀请码弹窗
    onCloseInviteModal() {
        this.setData({ showInviteModal: false })
    },

    // 复制邀请码
    onCopyInviteCode() {
        wx.setClipboardData({
            data: this.data.inviteCode,
            success: () => {
                wx.showToast({ title: '已复制', icon: 'success' })
            }
        })
    },

    // 保存二维码到相册
    onSaveQRCode() {
        // 获取 t-qrcode 组件的 canvas 并保存
        const query = wx.createSelectorQuery().in(this)
        query.select('t-qrcode >>> canvas')
            .fields({ node: true, size: true })
            .exec((res: any) => {
                if (res[0]?.node) {
                    const canvas = res[0].node
                    wx.canvasToTempFilePath({
                        canvas,
                        success: (result) => {
                            wx.saveImageToPhotosAlbum({
                                filePath: result.tempFilePath,
                                success: () => {
                                    wx.showToast({ title: '已保存到相册', icon: 'success' })
                                },
                                fail: () => {
                                    wx.showToast({ title: '保存失败', icon: 'none' })
                                }
                            })
                        },
                        fail: () => {
                            wx.showToast({ title: '获取图片失败', icon: 'none' })
                        }
                    })
                } else {
                    wx.showToast({ title: '请长按二维码保存', icon: 'none' })
                }
            })
    },


    // 打开编辑角色弹窗
    onEditRole(e: any) {
        const staffId = e.currentTarget.dataset.id
        const staff = this.data.staffList.find(s => s.id === staffId)
        if (staff && staff.role !== 'owner') {
            this.setData({
                showEditModal: true,
                editingStaff: staff,
                selectedRole: staff.role
            })
        }
    },

    // 关闭编辑弹窗
    onCloseEditModal() {
        this.setData({ showEditModal: false, editingStaff: null })
    },

    // 选择角色
    onSelectRole(e: any) {
        const role = e.currentTarget.dataset.role
        this.setData({ selectedRole: role })
    },

    // 提交角色修改
    async onSubmitRoleChange() {
        const { editingStaff, selectedRole } = this.data
        if (!editingStaff) return

        this.setData({ updating: true })
        try {
            await StaffService.updateStaffRole(editingStaff.id, selectedRole)
            wx.showToast({ title: '修改成功', icon: 'success' })
            this.setData({ showEditModal: false, editingStaff: null })
            this.loadStaffList()
        } catch (error: any) {
            console.error('修改角色失败:', error)
            wx.showToast({ title: error.message || '修改失败', icon: 'none' })
        } finally {
            this.setData({ updating: false })
        }
    },

    // 打开删除确认弹窗
    onDeleteStaff(e: any) {
        const staffId = e.currentTarget.dataset.id
        const staff = this.data.staffList.find(s => s.id === staffId)
        if (staff && staff.role !== 'owner') {
            this.setData({
                showDeleteModal: true,
                deletingStaff: staff
            })
        }
    },

    // 关闭删除弹窗
    onCloseDeleteModal() {
        this.setData({ showDeleteModal: false, deletingStaff: null })
    },

    // 确认删除
    async onConfirmDelete() {
        const { deletingStaff } = this.data
        if (!deletingStaff) return

        this.setData({ deleting: true })
        try {
            await StaffService.deleteStaff(deletingStaff.id)
            wx.showToast({ title: '已移除', icon: 'success' })
            this.setData({ showDeleteModal: false, deletingStaff: null })
            this.loadStaffList()
        } catch (error: any) {
            console.error('移除员工失败:', error)
            wx.showToast({ title: error.message || '移除失败', icon: 'none' })
        } finally {
            this.setData({ deleting: false })
        }
    },

    // 格式化日期
    formatDate(dateStr: string): string {
        if (!dateStr) return '-'
        return dateStr.slice(0, 10)
    },

    // 获取角色名称
    getRoleName(role: string): string {
        return ROLE_CONFIG[role]?.name || role
    },

    // 获取角色颜色
    getRoleColor(role: string): string {
        return ROLE_CONFIG[role]?.color || '#666'
    }
})
