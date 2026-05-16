import { getPlatformBaofuSettlementAccount } from '../../../../api/baofu-account'
import { baofuSettlementStatusBehavior } from '../../../../behaviors/baofu-settlement-status'

Page({
  behaviors: [
    baofuSettlementStatusBehavior({
      role: 'platform',
      submitPagePath: '/pages/platform/finance/settlement-account/submit/index',
      getAccount: getPlatformBaofuSettlementAccount,
      supportPaymentRecovery: false,
      logTag: 'platform-baofu-settlement-account',
      loadErrorFallback: '平台宝付开户状态加载失败，请稍后重试',
      refreshErrorFallback: '平台宝付开户状态刷新失败，请稍后重试'
    })
  ]
})
