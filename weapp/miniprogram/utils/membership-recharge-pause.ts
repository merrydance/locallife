export const MEMBERSHIP_RECHARGE_PAUSED_MESSAGE = '会员线上充值已暂停，请联系商户线下充值后入账。'

export function showMembershipRechargePausedMessage() {
  wx.showModal({
    title: '线上充值已暂停',
    content: MEMBERSHIP_RECHARGE_PAUSED_MESSAGE,
    showCancel: false,
    confirmText: '我知道了'
  })
}