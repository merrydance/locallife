import Navigation from '../../utils/navigation'
import { updateUserInfo, getUserInfo } from '../../api/auth'
import { logger } from '../../utils/logger'
import { UploadService } from '../../api/upload'

const app = getApp<IAppOption>()

Page({
  data: {
    userInfo: {
      nickName: '微信用户',
      avatarUrl: ''
    },
    userRole: 'guest' as 'guest' | 'merchant' | 'rider' | 'operator',
    workbenches: [] as Array<{ id: string, name: string, path: string }>,
    registrationOptions: [
      { id: 'merchant', name: '餐厅入驻', desc: '开通商家账号', path: '/pages/register/merchant/index' },
      { id: 'rider', name: '骑手入驻', desc: '成为配送骑手', path: '/pages/register/rider/index' },
      { id: 'operator', name: '运营商入驻', desc: '区域运营合作', path: '/pages/register/operator/index' }
    ],
    navBarHeight: 88,
    scrollViewHeight: 600
  },

  onLoad() {
    // 计算导航栏高度和滚动区域高度
    const windowInfo = wx.getWindowInfo()
    const menuButton = wx.getMenuButtonBoundingClientRect()
    const statusBarHeight = windowInfo.statusBarHeight || 0
    const navBarContentHeight = menuButton.height + (menuButton.top - statusBarHeight) * 2
    const navBarHeight = statusBarHeight + navBarContentHeight
    // windowHeight 已扣除原生 tabBar，只需扣除自定义导航栏
    const scrollViewHeight = windowInfo.windowHeight - navBarHeight

    this.setData({ navBarHeight, scrollViewHeight })
    this.initUserInfo()
  },

  onShow() {
    // Refresh role in case it changed
    if (app.globalData.userInfo) {
      this.updateUser(app.globalData.userInfo, app.globalData.userRole)
    }
    // Always try to fetch fresh data on show to ensure persistence check
    this.refreshUserInfo()
  },

  updateUser(info: any, roles: string[] | string) {
    const role = (Array.isArray(roles) ? roles[0] : roles) as 'guest' | 'merchant' | 'rider' | 'operator' // Primary role for display
    this.setData({
      userInfo: {
        nickName: info.nickName || info.full_name || info.nickname || '微信用户',
        avatarUrl: info.avatarUrl || info.avatar_url || info.avatar || ''
      },
      userRole: role // Keep for compatibility
    })

    // Normalize to array for workbench check
    const roleList = Array.isArray(roles) ? roles : [roles]
    this.loadWorkbenches(roleList)
  },

  async initUserInfo() {
    if (app.globalData.userInfo) {
      // Use cached role as fallback
      this.updateUser(app.globalData.userInfo, app.globalData.userRole)
    }
    await this.refreshUserInfo()
  },

  async refreshUserInfo() {
    try {
      const user = await getUserInfo()
      if (user) {
        logger.debug('Refreshed User Info from Backend', user) // Debug log

        // Recover avatar from local storage if backend returns empty
        const localAvatar = wx.getStorageSync('user_avatar')
        const finalAvatar = user.avatar_url || localAvatar || ''
        console.log('[UserCenter] Refresh Info - Avatar:', finalAvatar, 'User:', user)

        // Update Global Data
        app.globalData.userInfo = {
          nickName: user.full_name || '微信用户',
          avatarUrl: finalAvatar,
        } as WechatMiniprogram.UserInfo

        // Update Local Data
        this.updateUser(app.globalData.userInfo, user.roles || [])
      }
    } catch (err) {
      logger.error('Failed to refresh user info', err)
    }
  },

  // ==================== 导航方法 ====================

  onMyOrders() {
    Navigation.toOrderList()
  },

  onAddresses() {
    Navigation.toAddressList()
  },

  onPoints() {
    Navigation.toPoints()
  },

  onCoupons() {
    Navigation.toCoupons()
  },

  onFavorites() {
    Navigation.toFavorites()
  },

  onMembership() {
    Navigation.toMembership()
  },

  onMyReviews() {
    Navigation.toMyReviews()
  },

  onMyReservations() {
    wx.navigateTo({ url: '/pages/user_center/reservations/index' })
  },

  onWallet() {
    Navigation.toWallet()
  },

  onCredit() {
    Navigation.toCredit()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  loadWorkbenches(roles: string[]) {
    const workbenches = []

    if (roles.includes('merchant') || roles.includes('operator')) {
      workbenches.push({
        id: 'merchant',
        name: '商家工作台',
        icon: 'shop',
        path: '/pages/merchant/dashboard/index'
      })
    }

    if (roles.includes('rider') || roles.includes('operator')) {
      workbenches.push({
        id: 'rider',
        name: '骑手工作台',
        icon: 'user-circle', // generic user icon for rider
        path: '/pages/rider/dashboard/index'
      })
    }

    // Admin Entrance
    if (roles.includes('admin')) {
      workbenches.push({
        id: 'admin',
        name: '平台管理',
        desc: '系统管理控制台',
        icon: 'control-platform', // Safe bet or 'desktop'
        path: '/pages/platform/dashboard/dashboard' // Corrected path
      })
    }

    this.setData({ workbenches })
  },

  onWorkbenchTap(e: WechatMiniprogram.TouchEvent) {
    const { path } = e.currentTarget.dataset
    if (path) {
      wx.navigateTo({ url: path })
    }
  },

  onRegisterTap(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset
    const pathMap: Record<string, string> = {
      merchant: '/pages/register/merchant/index',
      rider: '/pages/register/rider/index',
      operator: '/pages/register/operator/index'
    }
    const path = pathMap[id]
    if (path) {
      wx.navigateTo({ url: path })
    }
  },

  onContact() {
    wx.makePhoneCall({ phoneNumber: '400-800-8888' })
  },

  async onChooseAvatar(e: any) {
    const { avatarUrl } = e.detail

    // Optimistic Update
    this.setData({
      'userInfo.avatarUrl': avatarUrl
    })

    wx.showLoading({ title: '上传中...' })

    try {
      // 1. Upload to Server
      const imageUrl = await UploadService.uploadImage(avatarUrl, 'avatar')
      const remoteUrl = imageUrl

      // 2. Persist locally with remote URL
      wx.setStorageSync('user_avatar', remoteUrl)

      // 3. Update Global Data
      app.globalData.userInfo = {
        ...(app.globalData.userInfo || {}),
        avatarUrl: remoteUrl
      } as WechatMiniprogram.UserInfo

      this.setData({
        'userInfo.avatarUrl': remoteUrl
      })

      // 4. Update Backend Profile
      await updateUserInfo({ avatar_url: remoteUrl })

    } catch (error) {
      console.error('Failed to update avatar on backend', error)
      wx.showToast({ title: '头像上传失败', icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  async onNicknameChange(e: any) {
    const nickName = e.detail.value
    if (!nickName) return

    this.setData({
      'userInfo.nickName': nickName
    })

    // Update Global Data
    app.globalData.userInfo = {
      ...(app.globalData.userInfo || {}),
      nickName: nickName
    } as WechatMiniprogram.UserInfo

    // Call Backend API
    try {
      await updateUserInfo({ full_name: nickName })
    } catch (error) {
      console.error('Failed to update nickname on backend', error)
    }
  }
})
