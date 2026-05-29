import { getMerchantBaofuSettlementAccount } from '../../_main_shared/api/baofu-account'
import {
  baofuSettlementStatusBehavior,
  type AccessCheckResult
} from '../../_main_shared/behaviors/baofu-settlement-status'
import {
  ensureMerchantApplymentAccess,
  getMerchantConsoleAccessErrorMessage,
  isMerchantConsoleAccessDenied,
  isMerchantConsoleAccessGranted
} from '../../../../utils/console-access'
import { logger } from '../../../../utils/logger'

async function merchantAccessGuard(): Promise<AccessCheckResult> {
  const accessResult = await ensureMerchantApplymentAccess()
  if (isMerchantConsoleAccessGranted(accessResult)) {
    return { granted: true, denied: false, deniedMessage: '', errorMessage: '' }
  }

  if (accessResult.status === 'error') {
    logger.error('Merchant baofu settlement access check failed action=check_access role=merchant', accessResult.message, 'merchant-baofu-settlement-account')
  }

  return {
    granted: false,
    denied: isMerchantConsoleAccessDenied(accessResult),
    deniedMessage: accessResult.status === 'denied' ? accessResult.message : '',
    errorMessage: getMerchantConsoleAccessErrorMessage(accessResult)
  }
}

Page({
  behaviors: [
    baofuSettlementStatusBehavior({
      role: 'merchant',
      submitPagePath: '/pages/merchant/finance/settlement-account/submit/index',
      getAccount: getMerchantBaofuSettlementAccount,
      accessGuard: merchantAccessGuard,
      supportPaymentRecovery: false,
      logTag: 'merchant-baofu-settlement-account',
      loadErrorFallback: '商户宝付开户状态加载失败，请稍后重试',
      refreshErrorFallback: '商户宝付开户状态刷新失败，请稍后重试'
    })
  ]
})
