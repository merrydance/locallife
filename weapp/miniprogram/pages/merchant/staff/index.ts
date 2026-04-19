import { getUserInfo, type UserResponse } from '../../../api/auth'
import {
  getMerchantStaffRoleMeta,
  getMerchantStaffStatusMeta,
  generateMerchantStaffInviteCode,
  isMerchantStaffActiveStatus,
  isMerchantStaffManagerRole,
  isMerchantStaffOwnerRole,
  isMerchantStaffPendingRole,
  listMerchantStaff,
  MerchantStaffItem,
  MerchantStaffRole,
  removeMerchantStaff,
  updateMerchantStaffRole
} from '../../../api/merchant-staff'
import { ensureMerchantConsoleAccess } from '../../../utils/console-access'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'

type EditableMerchantStaffRole = Exclude<MerchantStaffRole, 'owner' | 'pending'>

interface StaffView extends MerchantStaffItem {
  displayName: string
  displayInitial: string
  roleLabel: string
  roleTheme: string
  statusLabel: string
  statusTheme: string
  joinedAtLabel: string
  canEditRole: boolean
  canRemove: boolean
}

const ROLE_OPTIONS: Array<{ value: EditableMerchantStaffRole, label: string, desc: string }> = [
  { value: 'manager', label: '店长', desc: '可查看员工列表并邀请店员加入' },
  { value: 'chef', label: '后厨', desc: '用于厨房和出餐相关协作' },
  { value: 'cashier', label: '收银', desc: '用于前台和核销相关协作' }
]

function buildInviteQRCodeValue(inviteCode: string) {
  return inviteCode ? `invite-merchant:${inviteCode}` : ''
}

function normalizeUserRole(staff: MerchantStaffItem[], currentUserId: number, roles: string[]) {
  const matched = staff.find((item) => item.user_id === currentUserId && isMerchantStaffActiveStatus(item.status))
  if (matched && isMerchantStaffManagerRole(matched.role)) {
    return {
      currentUserRoleLabel: '店长',
      canGenerateInvite: true,
      canManageRoles: false
    }
  }

  const normalizedRoles = roles.map((role) => String(role).toLowerCase())
  const isOwner = normalizedRoles.some((role) => ['merchant', 'merchant_owner'].includes(role))
  if (isOwner) {
    return {
      currentUserRoleLabel: '老板',
      canGenerateInvite: true,
      canManageRoles: true
    }
  }

  if (matched) {
    return {
      currentUserRoleLabel: getMerchantStaffRoleMeta(matched.role).label,
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
  const roleMeta = getMerchantStaffRoleMeta(item.role)
  const statusMeta = getMerchantStaffStatusMeta(item.status)
  return {
    ...item,
    displayName: item.full_name || `用户 #${item.user_id}`,
    displayInitial: (item.full_name || `用户 #${item.user_id}`).slice(0, 1),
    roleLabel: roleMeta.label,
    roleTheme: roleMeta.theme,
    statusLabel: statusMeta.label,
    statusTheme: statusMeta.theme,
    joinedAtLabel: item.created_at ? item.created_at.replace('T', ' ').slice(0, 16) : '--',
    canEditRole: canManageRoles && !isMerchantStaffOwnerRole(item.role) && isMerchantStaffActiveStatus(item.status),
    canRemove: canManageRoles && !isMerchantStaffOwnerRole(item.role) && isMerchantStaffActiveStatus(item.status)
  }
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    roleOptions: ROLE_OPTIONS,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    hasLoadedOnce: false,
    loading: false,
    staff: [] as StaffView[],
    staffCount: 0,
    pendingCount: 0,
    currentUserId: 0,
    currentUserRoles: [] as string[],
    currentUserRoleLabel: '--',
    canGenerateInvite: false,
    canManageRoles: false,
    inviteVisible: false,
    inviteLoading: false,
    inviteError: false,
    inviteErrorMessage: '',
    inviteCode: '',
    inviteQRCodeValue: '',
    inviteExpiresAtLabel: '--',
    rolePopupVisible: false,
    roleSubmitting: false,
    editingStaffId: 0,
    editingStaffName: '',
    editingRole: 'manager' as EditableMerchantStaffRole,
    removingStaffId: 0
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })

    const accessResult = await ensureMerchantConsoleAccess()
    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : '',
      currentUserId: accessResult.status === 'granted' ? accessResult.user?.id || 0 : 0,
      currentUserRoles: accessResult.status === 'granted' ? accessResult.user?.roles || [] : []
    })
    if (accessResult.status !== 'granted') return

    this.loadStaff(true, accessResult.user || null)
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      wx.stopPullDownRefresh()
      return
    }

    this.loadStaff(false)
  },

  async loadStaff(showLoading = true, currentUser: UserResponse | null = null) {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      wx.stopPullDownRefresh()
      return
    }
    if (this.data.loading) return

    const hasLoadedOnce = this.data.hasLoadedOnce
    const isSilentRefresh = !showLoading && hasLoadedOnce

    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
        : isSilentRefresh
          ? { refreshErrorMessage: '' }
          : {})
    })

    try {
      const response = await listMerchantStaff()
      const hasCachedUser = this.data.currentUserId > 0 || this.data.currentUserRoles.length > 0
      const user = currentUser
        || (hasCachedUser
          ? { id: this.data.currentUserId, roles: [...this.data.currentUserRoles] } as UserResponse
          : await getUserInfo().catch(() => null))
      const currentUserId = user?.id || 0
      const currentUserRoles = user?.roles || []
      const roleState = normalizeUserRole(response.staff || [], currentUserId, currentUserRoles)
      const staff = buildStaffView(response.staff || [], roleState.canManageRoles)
      this.setData({
        staff,
        staffCount: response.count || staff.length,
        pendingCount: staff.filter((item) => isMerchantStaffPendingRole(item.role) && isMerchantStaffActiveStatus(item.status)).length,
        currentUserId,
        currentUserRoles,
        currentUserRoleLabel: roleState.currentUserRoleLabel,
        canGenerateInvite: roleState.canGenerateInvite,
        canManageRoles: roleState.canManageRoles,
        hasLoadedOnce: true,
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
      } else if (hasLoadedOnce) {
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
      inviteErrorMessage: '',
      inviteCode: '',
      inviteQRCodeValue: '',
      inviteExpiresAtLabel: '--'
    })

    try {
      const response = await generateMerchantStaffInviteCode()
      this.setData({
        inviteCode: response.invite_code,
        inviteQRCodeValue: buildInviteQRCodeValue(response.invite_code),
        inviteExpiresAtLabel: response.expires_at ? response.expires_at.replace('T', ' ').slice(0, 16) : '--'
      })
    } catch (err: unknown) {
      logger.error('Generate merchant invite code failed', err)
      const message = getErrorMessage(err, '生成邀请码失败，请重试')
      this.setData({
        inviteError: true,
        inviteErrorMessage: message,
        inviteCode: '',
        inviteQRCodeValue: '',
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
        wx.showToast({ title: '备用邀请码已复制', icon: 'success' })
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
      editingRole: (isMerchantStaffPendingRole(target.role) ? 'manager' : target.role) as EditableMerchantStaffRole
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

  resetRolePopup() {
    this.setData({
      rolePopupVisible: false,
      editingStaffId: 0,
      editingStaffName: '',
      editingRole: 'manager'
    })
  },

  async submitRoleChange() {
    if (this.data.roleSubmitting || !this.data.editingStaffId) {
      return
    }

    this.setData({ roleSubmitting: true })
    wx.showLoading({ title: '保存中...' })

    try {
      const updatedStaff = await updateMerchantStaffRole(this.data.editingStaffId, { role: this.data.editingRole })
      this.patchStaffItem(this.data.editingStaffId, (item) => toStaffView({
        ...item,
        role: updatedStaff.role,
        status: updatedStaff.status
      }, this.data.canManageRoles))
      this.resetRolePopup()
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

  onSubmitRole() {
    if (this.data.roleSubmitting || !this.data.editingStaffId) return

    const target = this.data.staff.find((item) => item.id === this.data.editingStaffId)
    if (!target) return

    if (target.role === this.data.editingRole) {
      this.onCloseRolePopup()
      return
    }

    const fromLabel = getMerchantStaffRoleMeta(target.role).label
    const toLabel = getMerchantStaffRoleMeta(this.data.editingRole).label

    wx.showModal({
      title: '确认调整岗位',
      content: `确认将 ${target.displayName} 的岗位从“${fromLabel}”调整为“${toLabel}”吗？`,
      confirmText: '确认调整',
      cancelText: '取消',
      success: async (res) => {
        if (!res.confirm) return
        await this.submitRoleChange()
      }
    })
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

  onRetryAccess() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      initialLoading: true,
      currentUserId: 0,
      currentUserRoles: [],
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: '',
      hasLoadedOnce: false
    })
    this.onLoad()
  },

  onRetryRefresh() {
    this.loadStaff(false)
  },

  applyStaffState(staff: StaffView[]) {
    this.setData({
      staff,
      staffCount: staff.length,
      pendingCount: staff.filter((item) => isMerchantStaffPendingRole(item.role) && isMerchantStaffActiveStatus(item.status)).length
    })
  },

  patchStaffItem(staffId: number, updater: (item: StaffView) => StaffView) {
    const nextStaff = this.data.staff.map((item) => item.id === staffId ? updater(item) : item)
    this.applyStaffState(nextStaff)
  }
})