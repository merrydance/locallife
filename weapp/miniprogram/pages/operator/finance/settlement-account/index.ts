import { getOperatorBaofuSettlementAccount } from '../../_main_shared/api/baofu-account'
import { baofuSettlementStatusBehavior } from '../../_main_shared/behaviors/baofu-settlement-status'

Page({
  behaviors: [
    baofuSettlementStatusBehavior({
      role: 'operator',
      submitPagePath: '/pages/operator/finance/settlement-account/submit/index',
      getAccount: getOperatorBaofuSettlementAccount,
      supportPaymentRecovery: true,
      logTag: 'operator-baofu-settlement-account',
      loadErrorFallback: '运营商宝付开户状态加载失败，请稍后重试',
      refreshErrorFallback: '运营商宝付开户状态同步失败，请稍后重试'
    })
  ]
})
