import { getPlatformBaofuSettlementAccount } from '../../_main_shared/api/baofu-account'
import { baofuSettlementStatusBehavior } from '../../_main_shared/behaviors/baofu-settlement-status'

Page({
  behaviors: [
    baofuSettlementStatusBehavior({
      role: 'platform',
      submitPagePath: '',
      disableSubmitProfile: true,
      getAccount: getPlatformBaofuSettlementAccount,
      supportPaymentRecovery: false,
      logTag: 'platform-baofu-settlement-account',
      loadErrorFallback: '平台宝付开户状态加载失败，请稍后重试',
      refreshErrorFallback: '平台宝付开户状态同步失败，请稍后重试'
    })
  ]
})
