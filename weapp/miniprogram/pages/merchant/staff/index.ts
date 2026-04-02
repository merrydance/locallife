import { getUserInfo } from '../../../api/auth'
import {
  generateMerchantStaffInviteCode,
  listMerchantStaff,
  MerchantStaffItem,
  MerchantStaffRole,
  removeMerchantStaff,
  updateMerchantStaffRole
} from '../../../api/merchant-staff'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'

type EditableMerchantStaffRole = Exclude<MerchantStaffRole, 'owner' | 'pending'>

interface StaffView extends MerchantStaffItem {
  displayName: string
  roleLabel: string
  roleTheme: string
  statusLabel: string
  statusTheme: string
  joinedAtLabel: string
  canEditRole: boolean
  canRemove: boolean
}

const ROLE_OPTIONS: Array<{ value: EditableMerchantStaffRole, label: string, desc: string }> = [
  { value: 'manager', label: '店长', desc: '可查看员工列表并生成邀请码' },
  { value: 'chef', label: '后厨', desc: '用于厨房和出餐相关协作' },
  { value: 'cashier', label: '收银', desc: '用于前台和核销相关协作' }
]

function getRoleMeta(role: MerchantStaffRole) {
  switch (role) {
    case 'owner':
      return { label: '老板', theme: 'primary' }
    case 'manager':
      return { label: '店长', theme: 'success' }
    case 'chef':
      return { label: '后厨', theme: 'warning' }
    case 'cashier':
      return { label: '收银', theme: 'primary' }
    case 'pending':
      return { label: '待分配', theme: 'danger' }
    default:
      return { label: role, theme: 'default' }
  }
}

function getStatusMeta(status: string) {
  switch (status) {
    case 'active':
      return { label: '在职', theme: 'success' }
    case 'disabled':
      return { label: '已移除', theme: 'default' }
    default:
      return { label: status || '未知', theme: 'default' }
  }
}

function normalizeUserRole(staff: MerchantStaffItem[], currentUserId: number, roles: string[]) {
  const matched = staff.find((item) => item.user_id === currentUserId && item.status === 'active')
  if (matched?.role === 'manager') {
    return {
      currentUserRoleLabel: '店长',
      canGenerateInvite: true,
      canManageRoles: false
    }
  }

  const normalizedRoles = roles.map((role) => String(role).toLowerCase())
  const isOwner = normalizedRoles.some((role) => ['merchant', 'merchant_owner', 'merchant_boss'].includes(role))
  if (isOwner) {
    return {
      currentUserRoleLabel: '老板',
      canGenerateInvite: true,
      canManageRoles: true
    }
  }

  if (matched) {
    return {
      currentUserRoleLabel: getRoleMeta(matched.role).label,
      canGenerateInvite: false,
      canManageRoles: false
    }
  }

  return {
    currentUserRoleLabel: '员工',
    canGenerateInvite: false,
    canManageRoles: false
  }
}

function buildStaffView(items: MerchantStaffItem[], canManageRoles: boolean): StaffView[] {
  return items.map((item) => toStaffView(item, canManageRoles))
}

function toStaffView(item: MerchantStaffItem, canManageRoles: boolean): StaffView {
  const roleMeta = getRoleMeta(item.role)
  const statusMeta = getStatusMeta(item.status)
  return {
    ...item,
    displayName: item.full_name || `用户 #${item.user_id}`,
    roleLabel: roleMeta.label,
    roleTheme: roleMeta.theme,
    statusLabel: statusMeta.label,
    statusTheme: statusMeta.theme,
    joinedAtLabel: item.created_at ? item.created_at.replace('T', ' ').slice(0, 16) : '--',
    canEditRole: canManageRoles && item.role !== 'owner' && item.status === 'active',
    canRemove: canManageRoles && item.role !== 'owner' && item.status === 'active'
  }
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    roleOptions: ROLE_OPTIONS,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: false,
    staff: [] as StaffView[],
    staffCount: 0,
    pendingCount: 0,
    currentUserId: 0,
    currentUserRoleLabel: '--',
    canGenerateInvite: false,
    canManageRoles: false,
    inviteVisible: false,
    inviteLoading: false,
    inviteError: false,
    inviteErrorMessage: '',
    inviteCode: '',
    inviteExpiresAtLabel: '--',
    rolePopupVisible: false,
    roleSubmitting: false,
    editingStaffId: 0,
    editingStaffName: '',
    editingRole: 'manager' as EditableMerchantStaffRole,
    removingStaffId: 0
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadStaff()
  },

  onPullDownRefresh() {
    this.loadStaff(false)
  },

  async loadStaff(showLoading = true) {
    if (this.data.loading) return

    const hasExistingStaff = this.data.staff.length > 0
    const isSilentRefresh = !showLoading && hasExistingStaff

    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
        : isSilentRefresh
          ? { refreshErrorMessage: '' }
          : {})
    })

    try {
      const [response, user] = await Promise.all([
        listMerchantStaff(),
        getUserInfo().catch(() => null)
      ])
      const currentUserId = user?.id || 0
      const roleState = normalizeUserRole(response.staff || [], currentUserId, user?.roles || [])
      const staff = buildStaffView(response.staff || [], roleState.canManageRoles)
      this.setData({
        staff,
        staffCount: response.count || staff.length,
        pendingCount: staff.filter((item) => item.role === 'pending' && item.status === 'active').length,
        currentUserId,
        currentUserRoleLabel: roleState.currentUserRoleLabel,
        canGenerateInvite: roleState.canGenerateInvite,
        canManageRoles: roleState.canManageRoles,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } catch (err: unknown) {
      logger.error('Load merchant staff failed', err)
      const message = getErrorMessage(err, '员工列表加载失败，请重试')

      if (this.data.initialLoading) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else if (hasExistingStaff) {
        this.setData({
          refreshErrorMessage: `${message}，当前已保留上次同步结果`
        })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  async onOpenInvitePopup() {
    if (this.data.inviteLoading) return

    this.setData({
      inviteVisible: true,
      inviteLoading: true,
      inviteError: false,
      inviteErrorMessage: ''
    })

    try {
      const response = await generateMerchantStaffInviteCode()
      this.setData({
        inviteCode: response.invite_code,
        inviteExpiresAtLabel: response.expires_at ? response.expires_at.replace('T', ' ').slice(0, 16) : '--'
      })
    } catch (err: unknown) {
      logger.error('Generate merchant invite code failed', err)
      const message = getErrorMessage(err, '生成邀请码失败，请重试')
      this.setData({
        inviteError: true,
        inviteErrorMessage: message,
        inviteCode: '',
        inviteExpiresAtLabel: '--'
      })
    } finally {
      this.setData({ inviteLoading: false })
    }
  },

  onCloseInvitePopup() {
    this.setData({ inviteVisible: false })
  },

  onCopyInviteCode() {
    if (!this.data.inviteCode) return
    wx.setClipboardData({
      data: this.data.inviteCode,
      success: () => {
        wx.showToast({ title: '邀请码已复制', icon: 'success' })
      }
    })
  },

  onRetryInviteCode() {
    this.onOpenInvitePopup()
  },

  onOpenRolePopup(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    const target = this.data.staff.find((item) => item.id === id)
    if (!target) return

    this.setData({
      rolePopupVisible: true,
      editingStaffId: id,
      editingStaffName: target.displayName,
      editingRole: (target.role === 'pending' ? 'manager' : target.role) as EditableMerchantStaffRole
    })
  },

  onCloseRolePopup() {
    if (this.data.roleSubmitting) return
    this.setData({
      rolePopupVisible: false,
      editingStaffId: 0,
      editingStaffName: '',
      editingRole: 'manager'
    })
  },

  onRoleChange(e: WechatMiniprogram.CustomEvent) {
    const value = (e.detail?.value || e.detail) as EditableMerchantStaffRole
    this.setData({ editingRole: value })
  },

  async onSubmitRole() {
    if (this.data.roleSubmitting || !this.data.editingStaffId) return

    this.setData({ roleSubmitting: true })
    wx.showLoading({ title: '保存中...' })

    try {
      const updatedStaff = await updateMerchantStaffRole(this.data.editingStaffId, { role: this.data.editingRole })
      this.patchStaffItem(this.data.editingStaffId, (item) => toStaffView({
        ...item,
        role: updatedStaff.role,
        status: updatedStaff.status
      }, this.data.canManageRoles))
      this.onCloseRolePopup()
      await this.loadStaff(false)
    } catch (err: unknown) {
      logger.error('Update merchant staff role failed', err)
      const message = getErrorMessage(err, '更新角色失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ roleSubmitting: false })
    }
  },

  onRemoveStaff(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    const target = this.data.staff.find((item) => item.id === id)
    if (!target) return

    wx.showModal({
      title: '移除员工',
      content: `确认移除 ${target.displayName} 吗？移除后对方将无法继续以当前商户身份工作。`,
      confirmText: '确认移除',
      cancelText: '取消',
      success: async (res) => {
        if (!res.confirm || this.data.removingStaffId) return
        this.setData({ removingStaffId: id })
        try {
          await removeMerchantStaff(id)
          this.patchStaffItem(id, (item) => toStaffView({
            ...item,
            status: 'disabled'
          }, this.data.canManageRoles))
          await this.loadStaff(false)
        } catch (err: unknown) {
          logger.error('Remove merchant staff failed', err)
          const message = getErrorMessage(err, '移除员工失败，请稍后重试')
          wx.showToast({ title: message, icon: 'none' })
        } finally {
          this.setData({ removingStaffId: 0 })
        }
      }
    })
  },

  onRetry() {
    this.loadStaff()
  },

  onRetryRefresh() {
    this.loadStaff(false)
  },

  applyStaffState(staff: StaffView[]) {
    this.setData({
      staff,
      staffCount: staff.length,
      pendingCount: staff.filter((item) => item.role === 'pending' && item.status === 'active').length
    })
  },

  patchStaffItem(staffId: number, updater: (item: StaffView) => StaffView) {
    const nextStaff = this.data.staff.map((item) => item.id === staffId ? updater(item) : item)
    this.applyStaffState(nextStaff)
  }
})