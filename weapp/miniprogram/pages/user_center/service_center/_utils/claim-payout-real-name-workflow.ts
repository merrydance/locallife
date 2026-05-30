import { fetchUserProfile, updateUserProfile } from '../../../../services/user-profile'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import { isClaimPayoutRealNameReady, promptClaimPayoutRealName } from './claim-payout-real-name'

export async function ensureClaimPayoutRealName(logScope: string): Promise<boolean> {
  const app = getApp<IAppOption>()
  let fullName = ''

  try {
    const profile = await fetchUserProfile()
    fullName = String(profile.full_name || '').trim()
    app.globalData.userInfo = {
      ...(app.globalData.userInfo || {}),
      nickName: fullName || app.globalData.userInfo?.nickName || '微信用户',
      avatarUrl: profile.avatar_url || app.globalData.userInfo?.avatarUrl || ''
    } as WechatMiniprogram.UserInfo
  } catch (err) {
    logger.warn(`[${logScope}] fetch user profile before payout real name failed`, err)
  }

  if (isClaimPayoutRealNameReady(fullName)) {
    return true
  }

  const realName = await promptClaimPayoutRealName()
  if (!realName) {
    return false
  }
  if (!isClaimPayoutRealNameReady(realName)) {
    wx.showToast({ title: '请填写真实姓名', icon: 'none' })
    return false
  }

  try {
    await updateUserProfile({ full_name: realName })
    app.globalData.userInfo = {
      ...(app.globalData.userInfo || {}),
      nickName: realName
    } as WechatMiniprogram.UserInfo
    return true
  } catch (err) {
    logger.error(`[${logScope}] update payout real name failed`, err)
    wx.showToast({ title: getErrorUserMessage(err, '姓名保存失败，请稍后重试'), icon: 'none' })
    return false
  }
}
