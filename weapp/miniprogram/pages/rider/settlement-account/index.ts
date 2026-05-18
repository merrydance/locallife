import { getRiderBaofuSettlementAccount } from '../../../api/baofu-account'
import { baofuSettlementStatusBehavior } from '../../../behaviors/baofu-settlement-status'

Page({
  behaviors: [
    baofuSettlementStatusBehavior({
      role: 'rider',
      submitPagePath: '/pages/rider/settlement-account/submit/index',
      getAccount: getRiderBaofuSettlementAccount,
      supportPaymentRecovery: true,
      logTag: 'rider-settlement-account',
      loadErrorFallback: '结算账户加载失败，请稍后重试',
      refreshErrorFallback: '开户状态刷新失败，请稍后重试'
    })
  ]
})
